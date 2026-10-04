package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"acars_parser/internal/acars"
	"acars_parser/internal/enrichment"
	"acars_parser/internal/extractor"
	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
)

func runLiveCmd(args []string) {
	fs := flag.NewFlagSet("live", flag.ExitOnError)
	natsServer := fs.String("server", "nats://157.90.242.138:4222", "NATS server URL")
	credsFile := fs.String("creds", "", "Path to NATS credentials file")
	subject := fs.String("subject", "v1.aircraft.ingest.*.message.*.created", "NATS subject to subscribe to")
	outputFile := fs.String("output", "", "Optional JSONL output file")

	// ClickHouse flags (defaults from environment variables).
	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPass := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")
	chDB := fs.String("ch-db", defaultCHDatabase(), "ClickHouse database")

	// PostgreSQL flags for state tracking (defaults from environment variables).
	pgHost := fs.String("pg-host", defaultPGHost(), "PostgreSQL host")
	pgPort := fs.Int("pg-port", defaultPGPort(), "PostgreSQL port")
	pgUser := fs.String("pg-user", defaultPGUser(), "PostgreSQL user")
	pgPass := fs.String("pg-password", defaultPGPassword(), "PostgreSQL password")
	pgDB := fs.String("pg-db", defaultPGDatabase(), "PostgreSQL database")
	noStore := fs.Bool("no-store", false, "Disable all database storage (messages and state)")
	showAll := fs.Bool("all", false, "Show all messages with text, not just parsed ones")
	showRaw := fs.Bool("raw", false, "Show raw message text")
	excludeTypes := fs.String("exclude", "sq_position", "Exclude result types (comma-separated, e.g. 'sq_position,route')")
	debugLabels := fs.String("debug", "", "Debug specific labels (comma-separated, e.g. '80,B6,H1')")
	showEmpty := fs.Bool("empty", false, "Show empty/missing fields to identify unparsed data")
	verbose := fs.Bool("v", false, "Verbose output: show message ID, label, parser type, and all parsed fields")
	_ = fs.Parse(args) // ExitOnError handles parse failures.

	if *credsFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -creds flag is required")
		os.Exit(1)
	}

	// Connect to NATS.
	opts := []nats.Option{
		nats.UserCredentials(*credsFile),
		nats.Name("acars_parser-live"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1),
	}

	nc, err := nats.Connect(*natsServer, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to NATS: %v\n", err)
		os.Exit(1)
	}
	defer nc.Close()

	fmt.Printf("Connected to %s\n", *natsServer)
	fmt.Printf("Subscribing to %s\n", *subject)

	// Optional file output.
	var outFile *os.File
	if *outputFile != "" {
		outFile, err = os.OpenFile(*outputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening output file: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = outFile.Close() }()
		fmt.Printf("Writing JSONL to %s\n", *outputFile)
	}

	// ClickHouse connection for message storage.
	var chDB_ *storage.ClickHouseDB
	var msgBuffer *messageBuffer
	var flushWg sync.WaitGroup

	// Create a cancellable context for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !*noStore {
		chDB_, err = storage.OpenClickHouse(ctx, storage.ClickHouseConfig{
			Host:     *chHost,
			Port:     *chPort,
			Database: *chDB,
			User:     *chUser,
			Password: *chPass,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to ClickHouse: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = chDB_.Close() }()
		fmt.Printf("ClickHouse: %s:%d/%s\n", *chHost, *chPort, *chDB)

		// Create message buffer for batched inserts.
		msgBuffer = newMessageBuffer(chDB_, 1000, 5*time.Second)
		flushWg.Add(1)
		go func() {
			defer flushWg.Done()
			msgBuffer.flushLoop(ctx)
		}()
	}

	// Initialise the registry.
	reg := registry.Default()
	reg.Sort()

	// PostgreSQL connection for state tracking.
	var pg *storage.PostgresDB
	stateStats := &liveStateStats{}
	if !*noStore {
		var err error
		pg, err = storage.OpenPostgres(ctx, storage.PostgresConfig{
			Host:     *pgHost,
			Port:     *pgPort,
			Database: *pgDB,
			User:     *pgUser,
			Password: *pgPass,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to PostgreSQL: %v\n", err)
			os.Exit(1)
		}
		defer pg.Close()
		fmt.Printf("PostgreSQL: %s:%d/%s\n", *pgHost, *pgPort, *pgDB)

		// Ensure the schema exists.
		if err := pg.CreateSchema(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating PostgreSQL schema: %v\n", err)
			os.Exit(1)
		}
	}

	// Use a bounded cache for deduplication to prevent unbounded memory growth.
	// 100,000 entries is sufficient for ~1 hour of messages at typical rates.
	const maxDedupeIDs = 100000
	stats := &liveStats{
		seenIDs:    newBoundedIDCache(maxDedupeIDs),
		typeCounts: make(map[string]int),
	}

	// Parse debug labels.
	debugMap := make(map[string]bool)
	if *debugLabels != "" {
		for _, l := range strings.Split(*debugLabels, ",") {
			debugMap[strings.TrimSpace(l)] = true
		}
	}

	// Parse excluded types.
	excludeMap := make(map[string]bool)
	if *excludeTypes != "" {
		for _, t := range strings.Split(*excludeTypes, ",") {
			excludeMap[strings.TrimSpace(t)] = true
		}
	}

	// Subscribe.
	_, err = nc.Subscribe(*subject, func(m *nats.Msg) {
		stats.totalMessages++

		// Extract message ID from subject for early deduplication.
		parts := strings.Split(m.Subject, ".")
		if len(parts) >= 7 {
			msgID := parts[5]
			if stats.seenIDs.Add(msgID) {
				stats.duplicates++
				return
			}
		}

		handleLiveMessage(ctx, m.Data, reg, stats, outFile, msgBuffer, pg, stateStats, *showAll, *showRaw, *showEmpty, *verbose, debugMap, excludeMap)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error subscribing: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Listening for messages... (Ctrl+C to quit)")
	fmt.Println()

	// Wait for interrupt.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Stop receiving signals.
	signal.Stop(sigChan)

	fmt.Println("\nShutting down gracefully...")

	// Flush remaining messages.
	if msgBuffer != nil {
		// Stop the flush loop goroutine.
		msgBuffer.stop()
		// Wait for the flush loop to exit.
		flushWg.Wait()

		// Do a final flush with a short timeout context.
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := msgBuffer.flush(flushCtx); err != nil {
			fmt.Fprintf(os.Stderr, "Error flushing final messages: %v\n", err)
		}
		flushCancel()
	}

	// Cancel the main context for any remaining operations.
	cancel()

	fmt.Println("\n--- Session Stats ---")
	fmt.Printf("Total: %d messages (%d unique, %d duplicates)\n",
		stats.totalMessages, stats.totalMessages-stats.duplicates, stats.duplicates)

	// Print type counts.
	for typeName, count := range stats.typeCounts {
		fmt.Printf("  %s: %d\n", typeName, count)
	}

	// Print storage stats.
	if msgBuffer != nil {
		fmt.Printf("  stored: %d\n", msgBuffer.stored)
	}

	// Print state tracker stats if enabled.
	if pg != nil {
		fmt.Println("\n--- State Tracker (Session) ---")
		fmt.Printf("Aircraft upserted: %d\n", stateStats.aircraftUpserted)
		fmt.Printf("Waypoints upserted: %d\n", stateStats.waypointsUpserted)
		fmt.Printf("Routes upserted: %d\n", stateStats.routesUpserted)
		fmt.Printf("ATIS updated: %d\n", stateStats.atisUpdated)
		fmt.Printf("Enrichments upserted: %d\n", stateStats.enrichmentsUpserted)
	}
}

// messageBuffer batches messages for efficient ClickHouse insertion.
type messageBuffer struct {
	db        *storage.ClickHouseDB
	messages  []storage.CHInsertParams
	mu        sync.Mutex
	batchSize int
	interval  time.Duration
	stopCh    chan struct{}
	stored    int
}

func newMessageBuffer(db *storage.ClickHouseDB, batchSize int, interval time.Duration) *messageBuffer {
	return &messageBuffer{
		db:        db,
		messages:  make([]storage.CHInsertParams, 0, batchSize),
		batchSize: batchSize,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

func (b *messageBuffer) add(msg storage.CHInsertParams) {
	b.mu.Lock()
	b.messages = append(b.messages, msg)
	shouldFlush := len(b.messages) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		if err := b.flush(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "Error auto-flushing messages: %v\n", err)
		}
	}
}

func (b *messageBuffer) flush(ctx context.Context) error {
	b.mu.Lock()
	if len(b.messages) == 0 {
		b.mu.Unlock()
		return nil
	}
	toFlush := b.messages
	b.messages = make([]storage.CHInsertParams, 0, b.batchSize)
	b.mu.Unlock()

	err := b.db.InsertBatch(ctx, toFlush)
	if err == nil {
		b.mu.Lock()
		b.stored += len(toFlush)
		b.mu.Unlock()
	}
	return err
}

func (b *messageBuffer) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := b.flush(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Error flushing messages: %v\n", err)
			}
		case <-b.stopCh:
			return
		}
	}
}

func (b *messageBuffer) stop() {
	close(b.stopCh)
}

type liveStats struct {
	totalMessages int
	duplicates    int
	typeCounts    map[string]int
	seenIDs       *boundedIDCache
}

// boundedIDCache is a bounded cache for deduplication that prevents unbounded memory growth.
// It uses a simple eviction strategy: when full, it clears 25% of the oldest entries.
type boundedIDCache struct {
	ids       map[string]int64 // ID -> timestamp (unix nano)
	maxSize   int
	mu        sync.RWMutex
	evictions int
}

func newBoundedIDCache(maxSize int) *boundedIDCache {
	return &boundedIDCache{
		ids:     make(map[string]int64, maxSize),
		maxSize: maxSize,
	}
}

// Contains checks if an ID is in the cache.
func (c *boundedIDCache) Contains(id string) bool {
	c.mu.RLock()
	_, exists := c.ids[id]
	c.mu.RUnlock()
	return exists
}

// Add adds an ID to the cache. Returns true if the ID was already present.
func (c *boundedIDCache) Add(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.ids[id]; exists {
		return true
	}

	// Evict old entries if at capacity.
	if len(c.ids) >= c.maxSize {
		c.evictOldest()
	}

	c.ids[id] = time.Now().UnixNano()
	return false
}

// evictOldest removes the oldest 25% of entries. Must be called with lock held.
func (c *boundedIDCache) evictOldest() {
	// Find entries to remove (oldest 25%).
	toRemove := c.maxSize / 4
	if toRemove < 1 {
		toRemove = 1
	}

	// Collect all entries with timestamps.
	type entry struct {
		id string
		ts int64
	}
	entries := make([]entry, 0, len(c.ids))
	for id, ts := range c.ids {
		entries = append(entries, entry{id, ts})
	}

	// Sort by timestamp (oldest first).
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].ts < entries[i].ts {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Remove oldest entries.
	for i := 0; i < toRemove && i < len(entries); i++ {
		delete(c.ids, entries[i].id)
	}
	c.evictions += toRemove
}

// liveStateStats tracks state upsert operations for the current session.
type liveStateStats struct {
	aircraftUpserted    int
	waypointsUpserted   int
	routesUpserted      int
	atisUpdated         int
	enrichmentsUpserted int
}

// updateStatePostgres extracts data from the message and upserts it to PostgreSQL.
func updateStatePostgres(ctx context.Context, pg *storage.PostgresDB, msg *acars.Message, results []registry.Result, stats *liveStateStats) {
	data := extractor.Extract(msg, results)
	// TODO: This is local time, but enrichment.Extract builds flight_date from
	// its calendar fields and treats it as UTC. On a non-UTC host, messages near
	// midnight get the wrong flight_date and the API's UTC "today" misses them.
	now := time.Now()

	// Upsert aircraft if we have identity information.
	if data.Flight != nil && data.Flight.ICAOHex != "" {
		err := pg.UpsertAircraft(ctx, storage.Aircraft{
			ICAOHex:      data.Flight.ICAOHex,
			Registration: data.Flight.Registration,
			TypeCode:     data.Flight.AircraftType, // ICAO designator, empty if not proven.
			FirstSeen:    now,
			LastSeen:     now,
			MsgCount:     1,
		})
		if err == nil {
			stats.aircraftUpserted++
		}
	}

	// Upsert waypoints with coordinates.
	for _, wp := range data.Waypoints {
		if wp.Name != "" && wp.Latitude != 0 && wp.Longitude != 0 {
			err := pg.UpsertWaypoint(ctx, storage.Waypoint{
				Name:        wp.Name,
				Latitude:    wp.Latitude,
				Longitude:   wp.Longitude,
				SourceCount: 1,
				FirstSeen:   now,
				LastSeen:    now,
			})
			if err == nil {
				stats.waypointsUpserted++
			}
		}
	}

	// Upsert ATIS if present.
	if data.ATIS != nil {
		err := pg.UpsertATISCurrent(ctx, storage.ATISCurrent{
			AirportICAO: data.ATIS.AirportICAO,
			Letter:      data.ATIS.Letter,
			ATISType:    data.ATIS.ATISType,
			ATISTime:    data.ATIS.ATISTime,
			RawText:     data.ATIS.RawText,
			Runways:     data.ATIS.Runways,
			Approaches:  data.ATIS.Approaches,
			Wind:        data.ATIS.Wind,
			Visibility:  data.ATIS.Visibility,
			Clouds:      data.ATIS.Clouds,
			Temperature: data.ATIS.Temperature,
			DewPoint:    data.ATIS.DewPoint,
			QNH:         data.ATIS.QNH,
			Remarks:     data.ATIS.Remarks,
			UpdatedAt:   now,
		})
		if err == nil {
			stats.atisUpdated++
		}
	}

	// Upsert route if we have origin and destination.
	// The routes table holds ICAO pairs; IATA pairs are kept by the extractor
	// for storage v2, which records the code type.
	if data.Flight != nil && data.Flight.AirportCodes == extractor.AirportCodesICAO {
		flightPattern := data.Flight.FlightNumber
		if flightPattern == "" {
			flightPattern = data.Flight.Registration
		}
		if flightPattern != "" {
			_, err := pg.UpsertRoute(ctx, storage.Route{
				FlightPattern:    flightPattern,
				OriginICAO:       data.Flight.Origin,
				DestICAO:         data.Flight.Destination,
				ObservationCount: 1,
				FirstSeen:        now,
				LastSeen:         now,
			})
			if err == nil {
				stats.routesUpserted++
			}
		}
	}

	// Upsert flight enrichment data for ADS-B integration, keyed only on
	// transmitted identity: the aircraft's link-layer address and the
	// transmitted flight number (not Airframes' airframe or flight records).
	icaoHex, _ := msg.AircraftAddress()
	callsign := msg.FlightNumber

	// Extract enrichment and upsert if we have the required fields.
	if icaoHex != "" && len(results) > 0 {
		if update := enrichment.ExtractEnrichment(icaoHex, callsign, now, results); update != nil {
			if err := pg.UpsertFlightEnrichment(ctx, *update); err == nil {
				stats.enrichmentsUpserted++
			}
		}
	}
}

func handleLiveMessage(ctx context.Context, data []byte, reg *registry.Registry, stats *liveStats, outFile *os.File, msgBuffer *messageBuffer, pg *storage.PostgresDB, stateStats *liveStateStats, showAll, showRaw, showEmpty, verbose bool, debugLabels, excludeTypes map[string]bool) {
	// Parse the message.
	msg := parseMessage(data)
	if msg == nil {
		return
	}

	// Debug output for specific labels - dump full JSON.
	if len(debugLabels) > 0 && debugLabels[msg.Label] {
		fmt.Printf("[DEBUG %s] FULL JSON:\n%s\n\n", msg.Label, string(data))
	}

	// Dispatch to all matching parsers.
	results := registry.Results(reg.Dispatch(msg))

	// Update state in PostgreSQL with parsed data.
	if pg != nil {
		updateStatePostgres(ctx, pg, msg, results, stateStats)
	}

	if len(results) > 0 {
		for _, result := range results {
			typeName := result.Type()
			stats.typeCounts[typeName]++

			// Skip excluded types.
			// TODO: This skips storage as well as display, so excluded types (by
			// default sq_position) are never written to ClickHouse or the JSONL
			// output. Exclusion should apply to console output only.
			if excludeTypes[typeName] {
				continue
			}

			// Format and print.
			if verbose {
				// Verbose output: show all details.
				printVerboseResult(msg, result, showRaw, showEmpty)
			} else {
				msgCtx := buildContext(msg)
				output := formatResult(result, showEmpty)
				if showRaw {
					fmt.Printf("[%s] %s %s\n  -> %s\n", msg.Label, msgCtx, output, msg.Text)
				} else {
					fmt.Printf("[%s] %s %s\n", msg.Label, msgCtx, output)
				}
			}

			// Write to JSONL file.
			if outFile != nil {
				entry := map[string]interface{}{
					"type": typeName,
					"data": result,
				}
				if b, err := json.Marshal(entry); err == nil {
					_, _ = outFile.Write(b)
					_, _ = outFile.WriteString("\n")
				}
			}

			// Store in ClickHouse.
			if msgBuffer != nil {
				missingFields := getMissingFields(result)
				origin, dest := extractRouteFromResult(result)
				confidence := extractConfidenceFromResult(result)

				flight := msg.FlightNumber

				ts := parseTimestamp(msg.Timestamp)

				msgBuffer.add(storage.CHInsertParams{
					ID:            uint64(msg.ID),
					Timestamp:     ts,
					Label:         msg.Label,
					ParserType:    typeName,
					Flight:        flight,
					Tail:          msg.Tail,
					Origin:        origin,
					Destination:   dest,
					RawText:       msg.Text,
					ParsedData:    result,
					MissingFields: missingFields,
					Confidence:    float32(confidence),
				})
			}
		}
	} else {
		// No parser matched.
		if showAll {
			if verbose {
				printVerboseUnparsed(msg, showRaw)
			} else {
				msgCtx := buildContext(msg)
				fmt.Printf("%s [%s] %s\n", msgCtx, msg.Label, truncate(msg.Text, 60))
			}
		}

		// Store unparsed messages too.
		if msgBuffer != nil && msg.Text != "" {
			flight := msg.FlightNumber

			ts := parseTimestamp(msg.Timestamp)

			msgBuffer.add(storage.CHInsertParams{
				ID:         uint64(msg.ID),
				Timestamp:  ts,
				Label:      msg.Label,
				ParserType: "unparsed",
				Flight:     flight,
				Tail:       msg.Tail,
				RawText:    msg.Text,
				ParsedData: map[string]string{"label": msg.Label},
			})
		}
	}
}

// getMissingFields returns a list of empty/missing optional fields for a result.
func getMissingFields(result registry.Result) []string {
	b, err := json.Marshal(result)
	if err != nil {
		return nil
	}

	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return nil
	}

	omitemptyFields := getOmitemptyFields(result)
	var missing []string
	for _, field := range omitemptyFields {
		if _, exists := m[field]; !exists {
			missing = append(missing, field)
		} else if v, ok := m[field].(string); ok && v == "" {
			missing = append(missing, field)
		} else if v, ok := m[field].(float64); ok && v == 0 {
			missing = append(missing, field)
		}
	}
	return missing
}

// extractRouteFromResult extracts origin and destination from a parsed result.
func extractRouteFromResult(result registry.Result) (origin, dest string) {
	b, err := json.Marshal(result)
	if err != nil {
		return "", ""
	}

	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return "", ""
	}

	if v, ok := m["origin"].(string); ok {
		origin = v
	} else if v, ok := m["origin_icao"].(string); ok {
		origin = v
	}

	if v, ok := m["destination"].(string); ok {
		dest = v
	} else if v, ok := m["dest_icao"].(string); ok {
		dest = v
	}

	return origin, dest
}

// extractConfidenceFromResult extracts the parse_confidence field if present.
func extractConfidenceFromResult(result registry.Result) float64 {
	b, err := json.Marshal(result)
	if err != nil {
		return 0
	}

	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return 0
	}

	if v, ok := m["parse_confidence"].(float64); ok {
		return v
	}
	return 0
}

// parseTimestamp parses a timestamp string into time.Time.
// Supports RFC3339 and common variations.
func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Now()
	}

	// Try RFC3339 first (most common).
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}

	// Try RFC3339Nano.
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}

	// Try without timezone.
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t
	}

	// Fallback to now.
	return time.Now()
}

// Note: parseMessage is defined in extract.go and shared across commands.

func buildContext(msg *acars.Message) string {
	var parts []string

	if msg.Flight != nil && msg.Flight.Flight != "" {
		parts = append(parts, msg.Flight.Flight)
	}

	if msg.Tail != "" {
		parts = append(parts, msg.Tail)
	}

	if msg.Airframe != nil && msg.Airframe.ManufacturerModel != "" {
		parts = append(parts, msg.Airframe.ManufacturerModel)
	}

	if len(parts) == 0 {
		return "[???]"
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func formatResult(result registry.Result, showEmpty bool) string {
	typeName := result.Type()

	// Format based on type - extract key fields via JSON.
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("[%s]", typeName)
	}

	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return fmt.Sprintf("[%s]", typeName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s]", strings.ToUpper(typeName)))

	// Common fields to display.
	if v, ok := m["flight_number"].(string); ok && v != "" {
		sb.WriteString(fmt.Sprintf(" %s", v))
	}
	if v, ok := m["flight_num"].(string); ok && v != "" {
		sb.WriteString(fmt.Sprintf(" %s", v))
	}

	// Route info.
	origin := ""
	dest := ""
	if v, ok := m["origin"].(string); ok {
		origin = v
	}
	if v, ok := m["origin_icao"].(string); ok {
		origin = v
	}
	if v, ok := m["destination"].(string); ok {
		dest = v
	}
	if v, ok := m["dest_icao"].(string); ok {
		dest = v
	}
	if origin != "" || dest != "" {
		sb.WriteString(fmt.Sprintf(" %s->%s", origin, dest))
	}

	// Position info.
	lat := ""
	lon := ""
	if v, ok := m["latitude"].(string); ok {
		lat = v
	} else if v, ok := m["latitude"].(float64); ok && v != 0 {
		lat = fmt.Sprintf("%.3f", v)
	}
	if v, ok := m["longitude"].(string); ok {
		lon = v
	} else if v, ok := m["longitude"].(float64); ok && v != 0 {
		lon = fmt.Sprintf("%.3f", v)
	}
	if lat != "" && lon != "" {
		sb.WriteString(fmt.Sprintf(" @%s,%s", lat, lon))
	}

	// Flight level (preferred) or altitude.
	if v, ok := m["flight_level"].(float64); ok && v > 0 {
		sb.WriteString(fmt.Sprintf(" FL%d", int(v)))
	} else if v, ok := m["altitude"].(string); ok && v != "" {
		sb.WriteString(fmt.Sprintf(" FL%s", v))
	} else if v, ok := m["altitude"].(float64); ok && v > 0 {
		sb.WriteString(fmt.Sprintf(" FL%d", int(v)/100))
	}

	// Squawk.
	if v, ok := m["squawk"].(string); ok && v != "" {
		sb.WriteString(fmt.Sprintf(" SQK %s", v))
	}

	// SID.
	if v, ok := m["sid"].(string); ok && v != "" {
		sb.WriteString(fmt.Sprintf(" SID %s", v))
	}

	// Runway.
	if v, ok := m["runway"].(string); ok && v != "" {
		sb.WriteString(fmt.Sprintf(" RWY %s", v))
	}

	// ADS-C specific: show message type and payload size for analysis.
	if typeName == "adsc" {
		if v, ok := m["message_type"].(string); ok && v != "" {
			sb.WriteString(fmt.Sprintf(" [%s", v))
			if bytes, ok := m["payload_bytes"].(float64); ok {
				sb.WriteString(fmt.Sprintf("/%db", int(bytes)))
			}
			if hex, ok := m["raw_hex"].(string); ok && hex != "" {
				sb.WriteString(fmt.Sprintf(" %s", hex))
			}
			sb.WriteString("]")
		}
	}

	// Show empty fields if requested (helps identify unparsed data).
	// Uses reflection to find fields with omitempty tag - these are the optional/extractable fields.
	if showEmpty {
		omitemptyFields := getOmitemptyFields(result)
		var empty []string
		for _, field := range omitemptyFields {
			if _, exists := m[field]; !exists {
				empty = append(empty, field)
			} else if v, ok := m[field].(string); ok && v == "" {
				empty = append(empty, field)
			} else if v, ok := m[field].(float64); ok && v == 0 {
				empty = append(empty, field)
			}
		}
		if len(empty) > 0 {
			sb.WriteString(fmt.Sprintf(" [missing: %s]", strings.Join(empty, ", ")))
		}
	}

	return sb.String()
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// metadataFields are fields that represent metadata, not extracted data.
// These should not be reported as "missing" since they're not parsed from the message.
var metadataFields = map[string]bool{
	"raw_text":   true,
	"message_id": true,
	"timestamp":  true,
}

// getOmitemptyFields uses reflection to find all struct fields with the omitempty JSON tag.
// These represent optional/extractable fields that might be missing from parsed data.
func getOmitemptyFields(v interface{}) []string {
	var fields []string

	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return fields
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Check if the tag contains omitempty.
		if strings.Contains(jsonTag, "omitempty") {
			// Extract the field name (before the comma).
			name := strings.Split(jsonTag, ",")[0]
			// Skip metadata fields that aren't extracted from messages.
			if name != "" && !metadataFields[name] {
				fields = append(fields, name)
			}
		}
	}

	return fields
}

// printVerboseResult outputs detailed information about a parsed message.
func printVerboseResult(msg *acars.Message, result registry.Result, showRaw, showEmpty bool) {
	typeName := result.Type()
	timestamp := time.Now().Format("15:04:05")

	// Header line with timestamp, ID, label, and parser.
	fmt.Printf("=== [%s] [%s] Label=%s Parser=%s ===\n",
		timestamp, formatMsgID(result.MessageID()), msg.Label, strings.ToUpper(typeName))

	// Flight info line.
	var flightInfo []string
	if msg.Flight != nil && msg.Flight.Flight != "" {
		flightInfo = append(flightInfo, fmt.Sprintf("Flight: %s", msg.Flight.Flight))
	}
	if msg.Tail != "" {
		flightInfo = append(flightInfo, fmt.Sprintf("Tail: %s", msg.Tail))
	}
	if msg.Airframe != nil && msg.Airframe.ManufacturerModel != "" {
		flightInfo = append(flightInfo, fmt.Sprintf("Aircraft: %s", msg.Airframe.ManufacturerModel))
	}
	if len(flightInfo) > 0 {
		fmt.Printf("  %s\n", strings.Join(flightInfo, " | "))
	}

	// Parse result to map and display all non-empty fields.
	b, err := json.Marshal(result)
	if err != nil {
		fmt.Printf("  (error marshalling result)\n")
		return
	}

	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		fmt.Printf("  (error unmarshalling result)\n")
		return
	}

	// Group fields for display.
	var positionFields, routeFields, otherFields []string

	for k, v := range m {
		// Skip metadata fields in display.
		if k == "message_id" || k == "timestamp" {
			continue
		}

		// Format the value.
		valStr := formatValue(v)
		if valStr == "" {
			continue
		}

		// Categorise fields.
		switch k {
		case "latitude", "longitude", "altitude", "flight_level", "ground_speed", "track":
			positionFields = append(positionFields, fmt.Sprintf("%s=%s", k, valStr))
		case "origin", "destination", "origin_icao", "dest_icao", "waypoints":
			routeFields = append(routeFields, fmt.Sprintf("%s=%s", k, valStr))
		default:
			otherFields = append(otherFields, fmt.Sprintf("%s=%s", k, valStr))
		}
	}

	if len(positionFields) > 0 {
		fmt.Printf("  Position: %s\n", strings.Join(positionFields, ", "))
	}
	if len(routeFields) > 0 {
		fmt.Printf("  Route: %s\n", strings.Join(routeFields, ", "))
	}
	if len(otherFields) > 0 {
		fmt.Printf("  Data: %s\n", strings.Join(otherFields, ", "))
	}

	// Show missing fields if requested.
	if showEmpty {
		omitemptyFields := getOmitemptyFields(result)
		var empty []string
		for _, field := range omitemptyFields {
			if _, exists := m[field]; !exists {
				empty = append(empty, field)
			} else if v, ok := m[field].(string); ok && v == "" {
				empty = append(empty, field)
			} else if v, ok := m[field].(float64); ok && v == 0 {
				empty = append(empty, field)
			}
		}
		if len(empty) > 0 {
			fmt.Printf("  Missing: %s\n", strings.Join(empty, ", "))
		}
	}

	// Show raw text if requested.
	if showRaw && msg.Text != "" {
		fmt.Printf("  Raw: %s\n", truncate(msg.Text, 100))
	}

	fmt.Println()
}

// printVerboseUnparsed outputs detailed information about an unparsed message.
func printVerboseUnparsed(msg *acars.Message, showRaw bool) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("=== [%s] [%d] Label=%s Parser=NONE ===\n", timestamp, msg.ID, msg.Label)

	// Flight info line.
	var flightInfo []string
	if msg.Flight != nil && msg.Flight.Flight != "" {
		flightInfo = append(flightInfo, fmt.Sprintf("Flight: %s", msg.Flight.Flight))
	}
	if msg.Tail != "" {
		flightInfo = append(flightInfo, fmt.Sprintf("Tail: %s", msg.Tail))
	}
	if msg.Airframe != nil && msg.Airframe.ManufacturerModel != "" {
		flightInfo = append(flightInfo, fmt.Sprintf("Aircraft: %s", msg.Airframe.ManufacturerModel))
	}
	if len(flightInfo) > 0 {
		fmt.Printf("  %s\n", strings.Join(flightInfo, " | "))
	}

	if showRaw && msg.Text != "" {
		fmt.Printf("  Raw: %s\n", msg.Text)
	} else if msg.Text != "" {
		fmt.Printf("  Text: %s\n", truncate(msg.Text, 80))
	}

	fmt.Println()
}

// formatMsgID formats a message ID for display.
func formatMsgID(id int64) string {
	if id == 0 {
		return "?"
	}
	return fmt.Sprintf("%d", id)
}

// formatValue converts an interface{} value to a display string.
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		if val == "" {
			return ""
		}
		return val
	case float64:
		if val == 0 {
			return ""
		}
		// Check if it's effectively an integer.
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%.4f", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case []interface{}:
		if len(val) == 0 {
			return ""
		}
		var parts []string
		for _, item := range val {
			parts = append(parts, formatValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		if len(val) == 0 {
			return ""
		}
		b, _ := json.Marshal(val)
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}
