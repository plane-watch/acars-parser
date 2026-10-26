package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"acars_parser/internal/acars"
	"acars_parser/internal/extractor"
	"acars_parser/internal/parsers/label5l"
	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
)

// backfillWorkItem represents a message to be processed by a worker.
// backfillWorkItem is a stored message with its parse results.
type backfillWorkItem struct {
	msg       *acars.Message
	results   []registry.Result
	timestamp time.Time
}

// messageGrouper joins the archive rows of a message (one per parse result,
// read together, in time order) into one work item, so that the extractor
// sees each message once with all its results and its transmitted flight,
// as it does in live. "unparsed" rows add nothing.
type messageGrouper struct {
	cur *backfillWorkItem
}

// add adds a row and returns the previous message once a row of another
// message arrives; otherwise nil.
func (g *messageGrouper) add(m storage.CHMessage) *backfillWorkItem {
	var done *backfillWorkItem
	if g.cur != nil && uint64(g.cur.msg.ID) != m.ID {
		done = g.flush()
	}
	if m.ParserType == "unparsed" {
		return done
	}
	parsed, ok := storedResult(m)
	if !ok {
		return done
	}
	if g.cur == nil {
		// The flight column holds the transmitted flight (rows stored
		// before February 2026, when it held Airframes' record, had it
		// blanked by the archive rebuild).
		g.cur = &backfillWorkItem{
			msg:       &acars.Message{ID: acars.FlexInt64(m.ID), Tail: m.Tail, Label: m.Label, FlightNumber: m.Flight},
			timestamp: m.Timestamp,
		}
	}
	g.cur.results = append(g.cur.results, &genericResult{typeName: m.ParserType, data: parsed})
	return done
}

// storedResult returns a row's parse result as a field map. A 5L route is
// parsed again from the raw text, since archived ones report the 5L
// designator (always "JST" and the flight number) as their callsign (see
// label5l), which would teach false flight aliases.
// TODO: Read the stored result for 5L too once the archive has been rebuilt
// (reparse -rebuild) with the current label5l parser.
func storedResult(m storage.CHMessage) (map[string]interface{}, bool) {
	data := []byte(m.ParsedJSON)
	if m.Label == "5L" && m.ParserType == "route" {
		r := (&label5l.Parser{}).Parse(&acars.Message{ID: acars.FlexInt64(m.ID), Label: m.Label, Text: m.RawText})
		if r == nil {
			return nil, false
		}
		var err error
		if data, err = json.Marshal(r); err != nil {
			return nil, false
		}
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

// flush returns the message being collected, if any.
func (g *messageGrouper) flush() *backfillWorkItem {
	done := g.cur
	g.cur = nil
	return done
}

// workerFor returns the worker, of n, that processes an aircraft's
// messages: the same for every message with the tail, ignoring case and
// dashes. Messages without a tail all go to the first worker.
func workerFor(tail string, n int) int {
	reg := acars.NormaliseRegistration(tail)
	if reg == "" {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(reg))
	return int(h.Sum32() % uint32(n))
}

func runBackfillCmd(args []string) {
	fs := flag.NewFlagSet("backfill", flag.ExitOnError)

	// ClickHouse connection flags (source).
	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chDatabase := fs.String("ch-database", defaultCHDatabase(), "ClickHouse database")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPassword := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")

	// PostgreSQL connection flags (destination).
	pgHost := fs.String("pg-host", defaultPGHost(), "PostgreSQL host")
	pgPort := fs.Int("pg-port", defaultPGPort(), "PostgreSQL port")
	pgDatabase := fs.String("pg-database", defaultPGDatabase(), "PostgreSQL database")
	pgUser := fs.String("pg-user", defaultPGUser(), "PostgreSQL user")
	pgPassword := fs.String("pg-password", defaultPGPassword(), "PostgreSQL password")

	// Processing options.
	parserType := fs.String("type", "", "Filter by parser type (e.g. 'flight_plan', 'pdc')")
	limit := fs.Int("limit", 0, "Limit number of messages (0 = all)")
	workers := fs.Int("workers", 8, "Number of worker goroutines")
	batchSize := fs.Int("batch", 10000, "Batch size for querying messages")
	verbose := fs.Bool("v", false, "Verbose output")

	_ = fs.Parse(args) // ExitOnError handles parse failures.

	ctx := context.Background()

	// Open ClickHouse connection (source).
	chCfg := storage.ClickHouseConfig{
		Host:     *chHost,
		Port:     *chPort,
		Database: *chDatabase,
		User:     *chUser,
		Password: *chPassword,
	}
	ch, err := storage.OpenClickHouse(ctx, chCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to ClickHouse: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = ch.Close() }()
	// Fail before doing anything if the archive lacks columns that are read.
	if err := ch.CheckSchema(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Open PostgreSQL connection (destination).
	pgCfg := storage.PostgresConfig{
		Host:     *pgHost,
		Port:     *pgPort,
		Database: *pgDatabase,
		User:     *pgUser,
		Password: *pgPassword,
	}
	pg, err := storage.OpenPostgres(ctx, pgCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	defer pg.Close()

	// Ensure the PostgreSQL schema exists.
	if err := pg.CreateSchema(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating PostgreSQL schema: %v\n", err)
		os.Exit(1)
	}

	// Get total count for progress reporting.
	totalCount, err := ch.Count(ctx, *parserType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error counting messages: %v\n", err)
		os.Exit(1)
	}
	maxMessages := int(totalCount)
	if *limit > 0 && *limit < maxMessages {
		maxMessages = *limit
	}

	fmt.Printf("Backfilling up to %d messages with %d workers...\n", maxMessages, *workers)
	fmt.Printf("Source: ClickHouse @ %s:%d/%s\n", *chHost, *chPort, *chDatabase)
	fmt.Printf("Destination: PostgreSQL @ %s:%d/%s\n", *pgHost, *pgPort, *pgDatabase)

	// Track statistics.
	var (
		processed        int64
		aircraftUpserted int64
		routesUpserted   int64
		waypointsFound   int64
		atisUpdates      int64
		flightStates     int64
		errorCount       int64
	)

	// Start the workers. Each aircraft's messages go to one worker (see
	// workerFor), in time order, so that the worker's callsign memory sees
	// them in order (see extractor.CallsignMemory).
	if *workers < 1 {
		*workers = 1
	}
	workChans := make([]chan *backfillWorkItem, *workers)
	var wg sync.WaitGroup

	for i := range workChans {
		workChans[i] = make(chan *backfillWorkItem, *batchSize / *workers + 1)
		wg.Add(1)
		go func(work <-chan *backfillWorkItem) {
			defer wg.Done()
			callsigns := extractor.NewCallsignMemory(extractor.CallsignTTL)
			for item := range work {
				// Extract data from the message.
				data := extractor.Extract(item.msg, item.results)
				callsigns.Apply(item.msg, &data, item.timestamp)

				// Write extracted data to PostgreSQL.
				if err := writeExtractedData(ctx, pg, data, item.timestamp); err != nil {
					atomic.AddInt64(&errorCount, 1)
					if *verbose {
						fmt.Fprintf(os.Stderr, "Error writing data: %v\n", err)
					}
					continue
				}

				// Update counters.
				if data.Flight != nil && data.Flight.ICAOHex != "" {
					atomic.AddInt64(&aircraftUpserted, 1)
				}
				// Count only the routes that are written (see flightRoute).
				if _, ok := flightRoute(data.Flight, time.Time{}); ok {
					atomic.AddInt64(&routesUpserted, 1)
				}
				atomic.AddInt64(&waypointsFound, int64(len(data.Waypoints)))
				if data.ATIS != nil {
					atomic.AddInt64(&atisUpdates, 1)
				}
				if data.Flight != nil && (data.Flight.ICAOHex != "" || data.Flight.Registration != "") {
					atomic.AddInt64(&flightStates, 1)
				}

				count := atomic.AddInt64(&processed, 1)
				if *verbose && count%10000 == 0 {
					fmt.Printf("  Processed %d messages...\n", count)
				}
			}
		}(workChans[i])
	}

	// Query and process messages in batches, in time order, joining each
	// message's rows (see messageGrouper).
	offset := 0
	var grouper messageGrouper

	for offset < maxMessages {
		currentBatch := *batchSize
		if offset+currentBatch > maxMessages {
			currentBatch = maxMessages - offset
		}

		// In time order, so that each aircraft's callsign memory sees its
		// messages in order: a joined message's ID (see reassembly) sorts
		// after every received one, but its time is its first block's.
		params := storage.CHQueryParams{
			ParserType: *parserType,
			Limit:      currentBatch,
			Offset:     offset,
			OrderBy:    "timestamp",
			OrderDesc:  false,
		}

		messages, err := ch.Query(ctx, params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error querying messages at offset %d: %v\n", offset, err)
			break
		}

		if len(messages) == 0 {
			break
		}

		for _, m := range messages {
			if item := grouper.add(m); item != nil {
				workChans[workerFor(item.msg.Tail, len(workChans))] <- item
			}
		}

		offset += len(messages)
		if !*verbose {
			pct := float64(offset) / float64(maxMessages) * 100
			fmt.Printf("\r  Queued %d / %d messages (%.1f%%)...", offset, maxMessages, pct)
		}
	}

	if item := grouper.flush(); item != nil {
		workChans[workerFor(item.msg.Tail, len(workChans))] <- item
	}

	// Close the work channels and wait for the workers to finish.
	for _, c := range workChans {
		close(c)
	}
	wg.Wait()
	fmt.Println()

	// Print summary.
	fmt.Printf("\nBackfill complete!\n")
	fmt.Printf("  Messages processed: %d\n", processed)
	fmt.Printf("  Aircraft upserted:  %d\n", aircraftUpserted)
	fmt.Printf("  Routes upserted:    %d\n", routesUpserted)
	fmt.Printf("  Waypoints found:    %d\n", waypointsFound)
	fmt.Printf("  ATIS updates:       %d\n", atisUpdates)
	fmt.Printf("  Flight states:      %d\n", flightStates)
	if errorCount > 0 {
		fmt.Printf("  Errors:             %d\n", errorCount)
	}
}

// writeExtractedData writes extracted data to PostgreSQL.
func writeExtractedData(ctx context.Context, pg *storage.PostgresDB, data extractor.ExtractedData, msgTimestamp time.Time) error {
	now := time.Now()

	// Use the message timestamp for first_seen/last_seen if available,
	// otherwise fall back to current time.
	seenTime := msgTimestamp
	if seenTime.IsZero() {
		seenTime = now
	}

	// Write aircraft if we have an ICAO hex.
	if data.Flight != nil && data.Flight.ICAOHex != "" {
		err := pg.UpsertAircraft(ctx, storage.Aircraft{
			ICAOHex:      data.Flight.ICAOHex,
			Registration: data.Flight.Registration,
			TypeCode:     data.Flight.AircraftType, // ICAO designator, empty if not proven.
			FirstSeen:    seenTime,
			LastSeen:     seenTime,
			MsgCount:     1,
		})
		if err != nil {
			return fmt.Errorf("upsert aircraft: %w", err)
		}
	}

	// Write waypoints.
	for _, wp := range data.Waypoints {
		if wp.Name == "" || (wp.Latitude == 0 && wp.Longitude == 0) {
			continue
		}
		err := pg.UpsertWaypoint(ctx, storage.Waypoint{
			Name:        wp.Name,
			Latitude:    wp.Latitude,
			Longitude:   wp.Longitude,
			SourceCount: 1,
			FirstSeen:   seenTime,
			LastSeen:    seenTime,
		})
		if err != nil {
			return fmt.Errorf("upsert waypoint %s: %w", wp.Name, err)
		}
	}

	// Write ATIS.
	if data.ATIS != nil && data.ATIS.AirportICAO != "" && data.ATIS.Letter != "" {
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
			UpdatedAt:   seenTime,
		})
		if err != nil {
			return fmt.Errorf("upsert ATIS: %w", err)
		}
	}

	// Write the flight's route (see flightRoute).
	if route, ok := flightRoute(data.Flight, seenTime); ok {
		if _, err := pg.UpsertRoute(ctx, route); err != nil {
			return fmt.Errorf("upsert route: %w", err)
		}
	}

	// Record a flight named in both forms (see flightroutes).
	if a := data.FlightAlias; a != nil {
		if err := pg.UpsertFlightAlias(ctx, a.IATA, a.ICAO, seenTime); err != nil {
			return fmt.Errorf("upsert flight alias: %w", err)
		}
	}

	// Write flight state.
	if data.Flight != nil && (data.Flight.ICAOHex != "" || data.Flight.Registration != "") {
		// Use ICAO hex as the key if available, otherwise registration.
		key := data.Flight.ICAOHex
		if key == "" {
			key = data.Flight.Registration
		}

		// Build the flight state with optional fields.
		fs := storage.FlightState{
			Key:          key,
			ICAOHex:      data.Flight.ICAOHex,
			Registration: data.Flight.Registration,
			FlightNumber: data.Flight.FlightNumber,
			Origin:       data.Flight.Origin,
			Destination:  data.Flight.Destination,
			FirstSeen:    seenTime,
			LastSeen:     seenTime,
			MsgCount:     1,
		}

		// Add position if available.
		if data.Flight.Latitude != 0 || data.Flight.Longitude != 0 {
			lat := data.Flight.Latitude
			lon := data.Flight.Longitude
			fs.Latitude = &lat
			fs.Longitude = &lon
		}

		// Add altitude if available.
		if data.Flight.Altitude != 0 {
			alt := data.Flight.Altitude
			fs.Altitude = &alt
		}

		// Add ground speed if available.
		if data.Flight.GroundSpeed != 0 {
			gs := data.Flight.GroundSpeed
			fs.GroundSpeed = &gs
		}

		// Add track if available.
		if data.Flight.Track != 0 {
			trk := data.Flight.Track
			fs.Track = &trk
		}

		// Add waypoint if available.
		if data.Flight.Waypoint != "" {
			fs.Waypoints = []string{data.Flight.Waypoint}
		}

		err := pg.UpsertFlightState(ctx, fs)
		if err != nil {
			return fmt.Errorf("upsert flight state: %w", err)
		}
	}

	return nil
}

// genericResult wraps a parsed map to implement registry.Result.
type genericResult struct {
	typeName string
	data     map[string]interface{}
}

func (r *genericResult) Type() string {
	return r.typeName
}

func (r *genericResult) MessageID() int64 {
	if id, ok := r.data["message_id"].(float64); ok {
		return int64(id)
	}
	return 0
}

func (r *genericResult) Timestamp() string {
	if ts, ok := r.data["timestamp"].(string); ok {
		return ts
	}
	return ""
}

// MarshalJSON returns the underlying data map.
func (r *genericResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.data)
}
