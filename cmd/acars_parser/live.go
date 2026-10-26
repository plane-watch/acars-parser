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
	"acars_parser/internal/reassembly"
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

		// Ensure the schema exists and is current: the inserts write every
		// column, including those added to an existing archive.
		if err := chDB_.CreateSchema(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating ClickHouse schema: %v\n", err)
			os.Exit(1)
		}

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

	handler := &liveHandler{
		reg: reg, stats: stats, outFile: outFile, msgBuffer: msgBuffer, pg: pg, stateStats: stateStats,
		showAll: *showAll, showRaw: *showRaw, showEmpty: *showEmpty, verbose: *verbose,
		debugLabels: debugMap, excludeTypes: excludeMap,
		assembler: reassembly.New(reassemblySettle, reassemblyIdle, reassemblyMaxPending),
		callsigns: extractor.NewCallsignMemory(extractor.CallsignTTL),
		now:       time.Now,
	}

	// Judge the settled multi-block messages every few seconds.
	assemblyCtx, stopAssembly := context.WithCancel(ctx)
	var assemblyWg sync.WaitGroup
	assemblyTicker := time.NewTicker(reassemblyFlushEvery)
	assemblyWg.Add(1)
	go func() {
		defer assemblyWg.Done()
		handler.flushLoop(assemblyCtx, assemblyTicker.C)
	}()

	// Subscribe.
	sub, err := nc.Subscribe(*subject, func(m *nats.Msg) {
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

		handler.handle(ctx, m.Data)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error subscribing: %v\n", err)
		os.Exit(1)
	}
	// subClosed is closed once the subscription is closed, which nats.go
	// does after the callback goroutine has delivered the queued messages
	// and returned.
	subClosed := make(chan struct{})
	sub.SetClosedHandler(func(string) { close(subClosed) })

	fmt.Println("Listening for messages... (Ctrl+C to quit)")
	fmt.Println()

	// Wait for interrupt.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Stop receiving signals.
	signal.Stop(sigChan)

	fmt.Println("\nShutting down gracefully...")

	// Stop the subscription first: drain it (no new messages; the queued
	// ones are delivered) and wait for its callbacks to return, so that
	// nothing is processed after the final flushes below and the summary is
	// not read while a callback writes it. Each callback's database work is
	// bounded, so the wait ends; drainLimit only guards against a fault.
	if err := sub.Drain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error draining the subscription: %v\n", err)
	} else {
		select {
		case <-subClosed:
		case <-time.After(drainLimit):
			fmt.Fprintf(os.Stderr, "The subscription had not drained after %s; queued messages may be lost\n", drainLimit)
		}
	}
	nc.Close()

	// Stop the assembler's flush loop, and process the messages that have
	// settled since its last tick, before the final storage flush. Messages
	// whose blocks have not settled are not joined.
	assemblyTicker.Stop()
	stopAssembly()
	assemblyWg.Wait()
	handler.flush(ctx)

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
	fmt.Printf("Joined from blocks: %d messages\n", stats.assembled)

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

// autoFlushTimeout bounds the insert that add makes when the buffer is
// full, which runs while the handler's lock is held.
const autoFlushTimeout = 30 * time.Second

func (b *messageBuffer) add(msg storage.CHInsertParams) {
	b.mu.Lock()
	b.messages = append(b.messages, msg)
	shouldFlush := len(b.messages) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		ctx, cancel := context.WithTimeout(context.Background(), autoFlushTimeout)
		defer cancel()
		if err := b.flush(ctx); err != nil {
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
	assembled     int // Messages joined from their blocks.
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

// updateStatePostgres extracts data from the message and upserts it to
// PostgreSQL. callsigns, if not nil, keys the flight of a message that gives
// no callsign (see extractor.CallsignMemory).
func updateStatePostgres(ctx context.Context, pg *storage.PostgresDB, msg *acars.Message, results []registry.Result, callsigns *extractor.CallsignMemory, stats *liveStateStats) {
	// TODO: This is local time, but enrichment.Extract builds flight_date from
	// its calendar fields and treats it as UTC. On a non-UTC host, messages near
	// midnight get the wrong flight_date and the API's UTC "today" misses them.
	now := time.Now()
	var data extractor.ExtractedData
	if callsigns != nil {
		// The message's own time: a joined message is processed after its
		// blocks settle (see extractor.CallsignMemory).
		data = callsigns.Extract(msg, results, parseTimestamp(msg.Timestamp))
	} else {
		data = extractor.Extract(msg, results)
	}

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

	// Upsert the flight's route (see flightRoute).
	if route, ok := flightRoute(data.Flight, now); ok {
		if _, err := pg.UpsertRoute(ctx, route); err == nil {
			stats.routesUpserted++
		}
	}

	// Record a flight named in both forms (see flightroutes).
	if a := data.FlightAlias; a != nil {
		_ = pg.UpsertFlightAlias(ctx, a.IATA, a.ICAO, now)
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

// liveHandler handles the messages of a live session. handle runs on the
// subscription's goroutine and flush on the flush loop's; mu serialises
// them, because the assembler, the stats and the buffers are not safe for
// concurrent use.
type liveHandler struct {
	mu sync.Mutex

	reg        *registry.Registry
	stats      *liveStats
	outFile    *os.File
	msgBuffer  *messageBuffer
	pg         *storage.PostgresDB
	stateStats *liveStateStats

	showAll, showRaw, showEmpty, verbose bool
	debugLabels, excludeTypes            map[string]bool

	// assembler joins multi-block downlinks.
	assembler *reassembly.Assembler

	// callsigns keys the flights of messages that give no callsign (see
	// extractor.CallsignMemory).
	callsigns *extractor.CallsignMemory

	// now returns the current time; tests replace it.
	now func() time.Time
}

// drainLimit bounds the wait, at shutdown, for the messages already
// received to be processed. Docker must allow longer before it kills the
// container (stop_grace_period in deployments/docker-compose.yml).
const drainLimit = 2 * time.Minute

// stateUpdateTimeout bounds a message's PostgreSQL state update, so that a
// blocked update cannot hold the handler's lock indefinitely.
const stateUpdateTimeout = 10 * time.Second

// Reassembly settings. Blocks of one message arrived up to about 25 seconds
// apart in October 2026: a message is judged once no block of it has
// arrived for 30 seconds (the flush loop runs every 5 seconds, so a joined
// message is processed 30 to 35 seconds after its last block), and
// forgotten after two minutes. Every downlink
// starts a group (its message number ends in a block letter), about 2,000
// a minute, so the bound is well above the groups held for two minutes.
const (
	reassemblySettle     = 30 * time.Second
	reassemblyIdle       = 2 * time.Minute
	reassemblyMaxPending = 50000
	reassemblyFlushEvery = 5 * time.Second
)

// handle decodes a feed message, processes it and gives it to the
// assembler.
func (h *liveHandler) handle(ctx context.Context, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	msg := parseMessage(data)
	if msg == nil {
		return
	}

	// Debug output for specific labels - dump full JSON.
	if len(h.debugLabels) > 0 && h.debugLabels[msg.Label] {
		fmt.Printf("[DEBUG %s] FULL JSON:\n%s\n\n", msg.Label, string(data))
	}

	h.process(ctx, msg)

	h.assembler.Add(msg, h.now())
}

// flush processes the messages the assembler has joined: those whose
// blocks have settled.
func (h *liveHandler) flush(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, joined := range h.assembler.Flush(h.now()) {
		h.stats.assembled++
		h.process(ctx, joined)
	}
}

// flushLoop flushes at each tick, until ctx is done. Flushing on a timer
// rather than on message arrival means a settled message is processed even
// when no further message arrives.
func (h *liveHandler) flushLoop(ctx context.Context, tick <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			h.flush(ctx)
		}
	}
}

// process parses a message, received or joined, updates the state, prints
// it and stores it.
func (h *liveHandler) process(ctx context.Context, msg *acars.Message) {
	// Dispatch to all matching parsers.
	results := registry.Results(h.reg.Dispatch(msg))

	// Update state in PostgreSQL with parsed data.
	if h.pg != nil {
		stateCtx, cancel := context.WithTimeout(ctx, stateUpdateTimeout)
		updateStatePostgres(stateCtx, h.pg, msg, results, h.callsigns, h.stateStats)
		cancel()
	}

	if len(results) > 0 {
		for _, result := range results {
			typeName := result.Type()
			h.stats.typeCounts[typeName]++

			// Skip excluded types.
			// TODO: This skips storage as well as display, so excluded types (by
			// default sq_position) are never written to ClickHouse or the JSONL
			// output. Exclusion should apply to console output only.
			if h.excludeTypes[typeName] {
				continue
			}

			// Format and print.
			if h.verbose {
				// Verbose output: show all details.
				printVerboseResult(msg, result, h.showRaw, h.showEmpty)
			} else {
				msgCtx := buildContext(msg)
				output := formatResult(result, h.showEmpty)
				if h.showRaw {
					fmt.Printf("[%s] %s %s\n  -> %s\n", msg.Label, msgCtx, output, msg.Text)
				} else {
					fmt.Printf("[%s] %s %s\n", msg.Label, msgCtx, output)
				}
			}

			// Write to JSONL file.
			if h.outFile != nil {
				entry := map[string]interface{}{
					"type": typeName,
					"data": result,
				}
				if b, err := json.Marshal(entry); err == nil {
					_, _ = h.outFile.Write(b)
					_, _ = h.outFile.WriteString("\n")
				}
			}

			// Store in ClickHouse.
			if h.msgBuffer != nil {
				h.msgBuffer.add(resultRow(msg, result))
			}
		}
	} else {
		// No parser matched.
		if h.showAll {
			if h.verbose {
				printVerboseUnparsed(msg, h.showRaw)
			} else {
				msgCtx := buildContext(msg)
				fmt.Printf("%s [%s] %s\n", msgCtx, msg.Label, truncate(msg.Text, 60))
			}
		}

		// Store unparsed messages too.
		if h.msgBuffer != nil && msg.Text != "" {
			h.msgBuffer.add(unparsedRow(msg))
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
