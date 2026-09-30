package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"acars_parser/internal/storage"

	_ "modernc.org/sqlite"
)

func runMigrateCmd(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)

	// SQLite source paths.
	messagesDB := fs.String("messages-db", "messages.db", "SQLite messages database path")
	stateDB := fs.String("state-db", "state.db", "SQLite state database path")

	// ClickHouse target.
	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chDatabase := fs.String("ch-database", defaultCHDatabase(), "ClickHouse database")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPassword := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")

	// PostgreSQL target.
	pgHost := fs.String("pg-host", defaultPGHost(), "PostgreSQL host")
	pgPort := fs.Int("pg-port", defaultPGPort(), "PostgreSQL port")
	pgDatabase := fs.String("pg-database", defaultPGDatabase(), "PostgreSQL database")
	pgUser := fs.String("pg-user", defaultPGUser(), "PostgreSQL user")
	pgPassword := fs.String("pg-password", defaultPGPassword(), "PostgreSQL password")

	// Options.
	batchSize := fs.Int("batch", 10000, "Batch size for message migration")
	dryRun := fs.Bool("dry-run", false, "Preview counts without migrating")
	skipMessages := fs.Bool("skip-messages", false, "Skip message migration")
	skipState := fs.Bool("skip-state", false, "Skip state migration")
	resumeFrom := fs.Int64("resume-from", 0, "Resume message migration from this ID")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Build config.
	cfg := storage.Config{
		ClickHouse: storage.ClickHouseConfig{
			Host:     *chHost,
			Port:     *chPort,
			Database: *chDatabase,
			User:     *chUser,
			Password: *chPassword,
		},
		Postgres: storage.PostgresConfig{
			Host:     *pgHost,
			Port:     *pgPort,
			Database: *pgDatabase,
			User:     *pgUser,
			Password: *pgPassword,
		},
	}

	// Open target databases.
	fmt.Println("Connecting to target databases...")
	db, err := storage.Open(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to databases: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create schemas.
	fmt.Println("Creating schemas...")
	if err := db.CreateSchemas(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating schemas: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Schemas created successfully.")

	if *dryRun {
		fmt.Println("\n=== DRY RUN ===")
	}

	// Migrate messages.
	if !*skipMessages {
		if err := migrateMessages(ctx, *messagesDB, db, *batchSize, *dryRun, *resumeFrom); err != nil {
			fmt.Fprintf(os.Stderr, "Error migrating messages: %v\n", err)
			os.Exit(1)
		}
	}

	// Migrate state.
	if !*skipState {
		if err := migrateState(ctx, *stateDB, db, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "Error migrating state: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("\nMigration complete!")
}

func migrateMessages(ctx context.Context, sqlitePath string, db *storage.DB, batchSize int, dryRun bool, resumeFrom int64) error {
	fmt.Printf("\n=== Migrating messages from %s ===\n", sqlitePath)

	// Open SQLite.
	sqliteDB, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer sqliteDB.Close()

	// Count messages.
	var totalCount int64
	err = sqliteDB.QueryRow("SELECT COUNT(*) FROM messages").Scan(&totalCount)
	if err != nil {
		return fmt.Errorf("count messages: %w", err)
	}
	fmt.Printf("Total messages in SQLite: %d\n", totalCount)

	// Count messages to migrate (from resume point).
	var migrateCount int64
	err = sqliteDB.QueryRow("SELECT COUNT(*) FROM messages WHERE id > ?", resumeFrom).Scan(&migrateCount)
	if err != nil {
		return fmt.Errorf("count messages to migrate: %w", err)
	}
	fmt.Printf("Messages to migrate (from ID %d): %d\n", resumeFrom, migrateCount)

	if dryRun {
		return nil
	}

	// Migrate in batches.
	var migrated int64
	lastID := resumeFrom
	startTime := time.Now()

	for {
		rows, err := sqliteDB.Query(`
			SELECT id, timestamp, label, parser_type, flight, tail, origin, destination,
				   raw_text, parsed_json, missing_fields, confidence, is_golden, annotation, expected_json
			FROM messages
			WHERE id > ?
			ORDER BY id
			LIMIT ?
		`, lastID, batchSize)
		if err != nil {
			return fmt.Errorf("query messages: %w", err)
		}

		var batch []storage.CHInsertParams
		var goldenAnnotations []storage.GoldenAnnotation

		for rows.Next() {
			var id int64
			var timestamp, label, parserType, flight, tail, origin, destination sql.NullString
			var rawText, parsedJSON, missingFields sql.NullString
			var confidence sql.NullFloat64
			var isGolden sql.NullInt64
			var annotation, expectedJSON sql.NullString

			err := rows.Scan(&id, &timestamp, &label, &parserType, &flight, &tail, &origin, &destination,
				&rawText, &parsedJSON, &missingFields, &confidence, &isGolden, &annotation, &expectedJSON)
			if err != nil {
				rows.Close()
				return fmt.Errorf("scan row: %w", err)
			}

			// Parse timestamp.
			ts := time.Now()
			if timestamp.Valid && timestamp.String != "" {
				if parsed, err := time.Parse(time.RFC3339, timestamp.String); err == nil {
					ts = parsed
				}
			}

			// Parse missing fields.
			var mf []string
			if missingFields.Valid && missingFields.String != "" {
				mf = strings.Split(missingFields.String, ",")
			}

			// Parse the parsed_json back to interface{}.
			var parsedData interface{}
			if parsedJSON.Valid && parsedJSON.String != "" {
				_ = json.Unmarshal([]byte(parsedJSON.String), &parsedData)
			}

			batch = append(batch, storage.CHInsertParams{
				ID:            uint64(id),
				Timestamp:     ts,
				Label:         label.String,
				ParserType:    parserType.String,
				Flight:        flight.String,
				Tail:          tail.String,
				Origin:        origin.String,
				Destination:   destination.String,
				RawText:       rawText.String,
				ParsedData:    parsedData,
				MissingFields: mf,
				Confidence:    float32(confidence.Float64),
			})

			// Track golden annotations separately.
			if isGolden.Valid && isGolden.Int64 == 1 {
				var expectedData map[string]interface{}
				if expectedJSON.Valid && expectedJSON.String != "" {
					_ = json.Unmarshal([]byte(expectedJSON.String), &expectedData)
				}
				goldenAnnotations = append(goldenAnnotations, storage.GoldenAnnotation{
					MessageID:    id,
					IsGolden:     true,
					Annotation:   annotation.String,
					ExpectedJSON: expectedData,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				})
			}

			lastID = id
		}
		rows.Close()

		if len(batch) == 0 {
			break // No more messages.
		}

		// Insert batch to ClickHouse.
		if err := db.CH.InsertBatch(ctx, batch); err != nil {
			return fmt.Errorf("insert batch: %w", err)
		}

		// Insert golden annotations to PostgreSQL.
		for _, g := range goldenAnnotations {
			if err := db.PG.UpsertGoldenAnnotation(ctx, g); err != nil {
				return fmt.Errorf("insert golden annotation: %w", err)
			}
		}

		migrated += int64(len(batch))
		elapsed := time.Since(startTime)
		rate := float64(migrated) / elapsed.Seconds()
		remaining := time.Duration(float64(migrateCount-migrated)/rate) * time.Second

		fmt.Printf("\rMigrated: %d/%d (%.1f%%) | %.0f msgs/sec | ETA: %s    ",
			migrated, migrateCount,
			float64(migrated)/float64(migrateCount)*100,
			rate, remaining.Round(time.Second))
	}

	fmt.Println()
	fmt.Printf("Messages migrated: %d\n", migrated)

	return nil
}

func migrateState(ctx context.Context, sqlitePath string, db *storage.DB, dryRun bool) error {
	fmt.Printf("\n=== Migrating state from %s ===\n", sqlitePath)

	// Check if state DB exists.
	if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
		fmt.Println("State database not found, skipping state migration.")
		return nil
	}

	// Open SQLite.
	sqliteDB, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer sqliteDB.Close()

	// Migrate each table.
	tables := []struct {
		name    string
		migrate func(context.Context, *sql.DB, *storage.DB, bool) (int64, error)
	}{
		{"aircraft", migrateAircraft},
		{"waypoints", migrateWaypoints},
		{"routes", migrateRoutes},
		{"aircraft_callsigns", migrateAircraftCallsigns},
		{"atis_current", migrateATISCurrent},
		{"flight_state", migrateFlightState},
	}

	for _, t := range tables {
		count, err := t.migrate(ctx, sqliteDB, db, dryRun)
		if err != nil {
			fmt.Printf("  %s: error - %v\n", t.name, err)
		} else {
			fmt.Printf("  %s: %d records\n", t.name, count)
		}
	}

	return nil
}

func migrateAircraft(ctx context.Context, sqliteDB *sql.DB, db *storage.DB, dryRun bool) (int64, error) {
	var count int64
	err := sqliteDB.QueryRow("SELECT COUNT(*) FROM aircraft").Scan(&count)
	if err != nil {
		// Table might not exist.
		return 0, nil
	}

	if dryRun {
		return count, nil
	}

	rows, err := sqliteDB.Query(`
		SELECT icao_hex, registration, type_code, operator, first_seen, last_seen, msg_count
		FROM aircraft
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var migrated int64
	for rows.Next() {
		var a storage.Aircraft
		var firstSeen, lastSeen string
		err := rows.Scan(&a.ICAOHex, &a.Registration, &a.TypeCode, &a.Operator, &firstSeen, &lastSeen, &a.MsgCount)
		if err != nil {
			return migrated, err
		}
		a.FirstSeen, _ = time.Parse("2006-01-02 15:04:05", firstSeen)
		a.LastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeen)

		if err := db.PG.UpsertAircraft(ctx, a); err != nil {
			return migrated, err
		}
		migrated++
	}

	return migrated, nil
}

func migrateWaypoints(ctx context.Context, sqliteDB *sql.DB, db *storage.DB, dryRun bool) (int64, error) {
	var count int64
	err := sqliteDB.QueryRow("SELECT COUNT(*) FROM waypoints").Scan(&count)
	if err != nil {
		return 0, nil
	}

	if dryRun {
		return count, nil
	}

	rows, err := sqliteDB.Query(`
		SELECT name, latitude, longitude, source_count, first_seen, last_seen
		FROM waypoints
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var migrated int64
	for rows.Next() {
		var w storage.Waypoint
		var firstSeen, lastSeen string
		err := rows.Scan(&w.Name, &w.Latitude, &w.Longitude, &w.SourceCount, &firstSeen, &lastSeen)
		if err != nil {
			return migrated, err
		}
		w.FirstSeen, _ = time.Parse("2006-01-02 15:04:05", firstSeen)
		w.LastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeen)

		if err := db.PG.UpsertWaypoint(ctx, w); err != nil {
			return migrated, err
		}
		migrated++
	}

	return migrated, nil
}

func migrateRoutes(ctx context.Context, sqliteDB *sql.DB, db *storage.DB, dryRun bool) (int64, error) {
	var count int64
	err := sqliteDB.QueryRow("SELECT COUNT(*) FROM routes").Scan(&count)
	if err != nil {
		return 0, nil
	}

	if dryRun {
		return count, nil
	}

	// Migrate routes.
	rows, err := sqliteDB.Query(`
		SELECT id, flight_pattern, origin_icao, dest_icao, is_multi_stop, observation_count, first_seen, last_seen
		FROM routes
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	// Map old IDs to new IDs.
	idMap := make(map[int]int)
	var migrated int64

	for rows.Next() {
		var oldID int
		var r storage.Route
		var firstSeen, lastSeen string
		var isMultiStop int
		err := rows.Scan(&oldID, &r.FlightPattern, &r.OriginICAO, &r.DestICAO, &isMultiStop, &r.ObservationCount, &firstSeen, &lastSeen)
		if err != nil {
			return migrated, err
		}
		r.IsMultiStop = isMultiStop == 1
		r.FirstSeen, _ = time.Parse("2006-01-02 15:04:05", firstSeen)
		r.LastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeen)

		newID, err := db.PG.UpsertRoute(ctx, r)
		if err != nil {
			return migrated, err
		}
		idMap[oldID] = newID
		migrated++
	}
	rows.Close()

	// Migrate route legs.
	rows, err = sqliteDB.Query(`
		SELECT route_id, sequence, origin_icao, dest_icao, observation_count, first_seen, last_seen
		FROM route_legs
	`)
	if err != nil {
		return migrated, nil // Table might not exist.
	}
	defer rows.Close()

	for rows.Next() {
		var oldRouteID int
		var leg storage.RouteLeg
		var firstSeen, lastSeen string
		err := rows.Scan(&oldRouteID, &leg.Sequence, &leg.OriginICAO, &leg.DestICAO, &leg.ObservationCount, &firstSeen, &lastSeen)
		if err != nil {
			continue
		}
		leg.RouteID = idMap[oldRouteID]
		leg.FirstSeen, _ = time.Parse("2006-01-02 15:04:05", firstSeen)
		leg.LastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeen)

		_ = db.PG.UpsertRouteLeg(ctx, leg)
	}
	rows.Close()

	// Migrate route aircraft.
	rows, err = sqliteDB.Query(`
		SELECT route_id, registration, observation_count, first_seen, last_seen
		FROM route_aircraft
	`)
	if err != nil {
		return migrated, nil
	}
	defer rows.Close()

	for rows.Next() {
		var oldRouteID int
		var ra storage.RouteAircraft
		var firstSeen, lastSeen string
		err := rows.Scan(&oldRouteID, &ra.Registration, &ra.ObservationCount, &firstSeen, &lastSeen)
		if err != nil {
			continue
		}
		ra.RouteID = idMap[oldRouteID]
		ra.FirstSeen, _ = time.Parse("2006-01-02 15:04:05", firstSeen)
		ra.LastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeen)

		_ = db.PG.UpsertRouteAircraft(ctx, ra)
	}

	return migrated, nil
}

func migrateAircraftCallsigns(ctx context.Context, sqliteDB *sql.DB, db *storage.DB, dryRun bool) (int64, error) {
	var count int64
	err := sqliteDB.QueryRow("SELECT COUNT(*) FROM aircraft_callsigns").Scan(&count)
	if err != nil {
		return 0, nil
	}

	if dryRun {
		return count, nil
	}

	rows, err := sqliteDB.Query(`
		SELECT registration, iata_prefix, icao_prefix, observation_count, first_seen, last_seen
		FROM aircraft_callsigns
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var migrated int64
	for rows.Next() {
		var cs storage.AircraftCallsign
		var firstSeen, lastSeen string
		err := rows.Scan(&cs.Registration, &cs.IATAPrefix, &cs.ICAOPrefix, &cs.ObservationCount, &firstSeen, &lastSeen)
		if err != nil {
			return migrated, err
		}
		cs.FirstSeen, _ = time.Parse("2006-01-02 15:04:05", firstSeen)
		cs.LastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeen)

		if err := db.PG.UpsertAircraftCallsign(ctx, cs); err != nil {
			return migrated, err
		}
		migrated++
	}

	return migrated, nil
}

func migrateATISCurrent(ctx context.Context, sqliteDB *sql.DB, db *storage.DB, dryRun bool) (int64, error) {
	var count int64
	err := sqliteDB.QueryRow("SELECT COUNT(*) FROM atis_current").Scan(&count)
	if err != nil {
		return 0, nil
	}

	if dryRun {
		return count, nil
	}

	rows, err := sqliteDB.Query(`
		SELECT airport_icao, letter, atis_type, atis_time, raw_text, runways, approaches,
			   wind, visibility, clouds, temperature, dew_point, qnh, remarks, updated_at
		FROM atis_current
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var migrated int64
	for rows.Next() {
		var a storage.ATISCurrent
		var runwaysJSON, approachesJSON, remarksJSON sql.NullString
		var updatedAt string
		err := rows.Scan(&a.AirportICAO, &a.Letter, &a.ATISType, &a.ATISTime, &a.RawText,
			&runwaysJSON, &approachesJSON, &a.Wind, &a.Visibility, &a.Clouds,
			&a.Temperature, &a.DewPoint, &a.QNH, &remarksJSON, &updatedAt)
		if err != nil {
			return migrated, err
		}

		if runwaysJSON.Valid {
			_ = json.Unmarshal([]byte(runwaysJSON.String), &a.Runways)
		}
		if approachesJSON.Valid {
			_ = json.Unmarshal([]byte(approachesJSON.String), &a.Approaches)
		}
		if remarksJSON.Valid {
			_ = json.Unmarshal([]byte(remarksJSON.String), &a.Remarks)
		}
		a.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		if err := db.PG.UpsertATISCurrent(ctx, a); err != nil {
			return migrated, err
		}
		migrated++
	}

	return migrated, nil
}

func migrateFlightState(ctx context.Context, sqliteDB *sql.DB, db *storage.DB, dryRun bool) (int64, error) {
	var count int64
	err := sqliteDB.QueryRow("SELECT COUNT(*) FROM flight_state").Scan(&count)
	if err != nil {
		return 0, nil
	}

	if dryRun {
		return count, nil
	}

	rows, err := sqliteDB.Query(`
		SELECT key, icao_hex, registration, flight_number, origin, destination,
			   latitude, longitude, altitude, ground_speed, track, waypoints,
			   first_seen, last_seen, msg_count
		FROM flight_state
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var migrated int64
	for rows.Next() {
		var fs storage.FlightState
		var waypointsJSON sql.NullString
		var firstSeen, lastSeen string
		err := rows.Scan(&fs.Key, &fs.ICAOHex, &fs.Registration, &fs.FlightNumber, &fs.Origin, &fs.Destination,
			&fs.Latitude, &fs.Longitude, &fs.Altitude, &fs.GroundSpeed, &fs.Track, &waypointsJSON,
			&firstSeen, &lastSeen, &fs.MsgCount)
		if err != nil {
			return migrated, err
		}

		if waypointsJSON.Valid {
			_ = json.Unmarshal([]byte(waypointsJSON.String), &fs.Waypoints)
		}
		fs.FirstSeen, _ = time.Parse("2006-01-02 15:04:05", firstSeen)
		fs.LastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeen)

		if err := db.PG.UpsertFlightState(ctx, fs); err != nil {
			return migrated, err
		}
		migrated++
	}

	return migrated, nil
}
