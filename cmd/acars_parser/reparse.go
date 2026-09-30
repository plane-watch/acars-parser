// Package main provides the reparse command for comparing old vs new parsing results.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/enrichment"
	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
)

// ReparseResult tracks the outcome of re-parsing a single message.
type ReparseResult struct {
	ID         uint64
	ParserType string
	DiffType   string            // "unchanged", "improved", "regressed", "changed"
	OldFields  map[string]string // Fields from old parse
	NewFields  map[string]string // Fields from new parse
	Added      []string          // Fields gained
	Removed    []string          // Fields lost
	Changed    []string          // Fields with different values
}

func runReparseCmd(args []string) {
	fs := flag.NewFlagSet("reparse", flag.ExitOnError)

	// ClickHouse connection options.
	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chDatabase := fs.String("ch-database", defaultCHDatabase(), "ClickHouse database")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPassword := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")

	// PostgreSQL connection options (for enrichment).
	pgHost := fs.String("pg-host", defaultPGHost(), "PostgreSQL host")
	pgPort := fs.Int("pg-port", defaultPGPort(), "PostgreSQL port")
	pgDatabase := fs.String("pg-database", defaultPGDatabase(), "PostgreSQL database")
	pgUser := fs.String("pg-user", defaultPGUser(), "PostgreSQL user")
	pgPassword := fs.String("pg-password", defaultPGPassword(), "PostgreSQL password")

	msgID := fs.Uint64("id", 0, "Reparse a specific message by ID and show result")
	parserType := fs.String("type", "", "Filter by parser type")
	label := fs.String("label", "", "Filter by ACARS label")
	verbose := fs.Bool("v", false, "Verbose output: show detailed diffs")
	regressionsOnly := fs.Bool("regressions-only", false, "Show only regressions")
	improvementsOnly := fs.Bool("improvements-only", false, "Show only improvements")
	limit := fs.Int("limit", 0, "Limit number of messages (0 = all)")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	dumpFile := fs.String("dump", "", "Dump regressed messages to file (includes raw text)")
	updateDB := fs.Bool("update", false, "Update ClickHouse with new parse results")
	batchSize := fs.Int("batch", 10000, "Batch size for updates")
	enrichDB := fs.Bool("enrich", false, "Populate PostgreSQL flight_enrichment table")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Open ClickHouse connection.
	chDB, err := storage.OpenClickHouse(ctx, storage.ClickHouseConfig{
		Host:     *chHost,
		Port:     *chPort,
		Database: *chDatabase,
		User:     *chUser,
		Password: *chPassword,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to ClickHouse: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = chDB.Close() }()

	// Open PostgreSQL connection if enrichment is requested.
	var pgDB *storage.PostgresDB
	if *enrichDB {
		pgDB, err = storage.OpenPostgres(ctx, storage.PostgresConfig{
			Host:     *pgHost,
			Port:     *pgPort,
			Database: *pgDatabase,
			User:     *pgUser,
			Password: *pgPassword,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to PostgreSQL: %v\n", err)
			os.Exit(1)
		}
		defer pgDB.Close()

		// Ensure schema exists.
		if err := pgDB.CreateSchema(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating schema: %v\n", err)
			os.Exit(1)
		}
	}

	// Handle single message reparse.
	if *msgID != 0 {
		reparseSingleMessageCH(ctx, chDB, *msgID, *jsonOutput)
		return
	}

	// Get registry.
	reg := registry.Default()
	reg.Sort()

	// Query parameters.
	queryLimit := 10000000 // No practical limit.
	if *limit > 0 {
		queryLimit = *limit
	}

	// Get total count first for progress.
	totalCount, err := countMessages(ctx, chDB, *parserType, *label)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error counting messages: %v\n", err)
		os.Exit(1)
	}

	if totalCount == 0 {
		fmt.Println("No messages to reparse.")
		return
	}

	fmt.Printf("Found %d messages to reparse\n", totalCount)
	if queryLimit < int(totalCount) {
		fmt.Printf("Processing first %d messages (use -limit 0 for all)\n", queryLimit)
	}

	// Process messages in batches.
	var allResults []ReparseResult
	stats := struct {
		Total       int
		Unchanged   int
		Improved    int
		Regressed   int
		Changed     int
		Enrichments int
		FieldStats  map[string]int
	}{
		FieldStats: make(map[string]int),
	}

	// Build registration-to-hex lookup cache if enriching.
	var regToHex map[string]string
	if pgDB != nil {
		regToHex = make(map[string]string)
	}

	var updateBatch []storage.CHInsertParams
	offset := 0
	batchNum := 0

	for offset < queryLimit && offset < int(totalCount) {
		batchNum++
		currentBatch := *batchSize
		if offset+currentBatch > queryLimit {
			currentBatch = queryLimit - offset
		}

		fmt.Printf("\rProcessing batch %d (offset %d)...", batchNum, offset)

		messages, err := queryMessages(ctx, chDB, *parserType, *label, currentBatch, offset)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError querying messages: %v\n", err)
			os.Exit(1)
		}

		if len(messages) == 0 {
			break
		}

		for _, msg := range messages {
			stats.Total++

			// Parse old JSON to get old fields.
			oldFields := extractFields(msg.ParsedJSON)

			// Re-parse the message.
			acarsMsg := &acars.Message{
				ID:    acars.FlexInt64(msg.ID),
				Label: msg.Label,
				Text:  msg.RawText,
				Tail:  msg.Tail,
			}

			matches := reg.Dispatch(acarsMsg)
			newResults := registry.Results(matches)
			var newFields map[string]string
			var newParserType string
			var parsedData interface{}

			if match, ok := comparableMatch(msg.ParserType, matches); ok {
				newFields = resultToFields(match.Result)
				newParserType = match.Result.Type()
				parsedData = match.Result
			} else {
				newFields = make(map[string]string)
				newParserType = "unparsed"
				parsedData = nil
			}

			// Compare old vs new.
			result := compareFieldsCH(msg.ID, msg.ParserType, oldFields, newFields)

			// Track field statistics.
			for _, f := range result.Added {
				stats.FieldStats["+"+f]++
			}
			for _, f := range result.Removed {
				stats.FieldStats["-"+f]++
			}

			// Determine if parser type changed.
			if msg.ParserType != newParserType && newParserType != "" {
				result.Changed = append(result.Changed, fmt.Sprintf("parser_type: %s -> %s", msg.ParserType, newParserType))
			}

			// Categorise.
			switch result.DiffType {
			case "unchanged":
				stats.Unchanged++
			case "improved":
				stats.Improved++
			case "regressed":
				stats.Regressed++
			case "changed":
				stats.Changed++
			}

			// Extract enrichment data if requested.
			if pgDB != nil && len(newResults) > 0 {
				// Try to get ICAO hex from registration via PostgreSQL aircraft table.
				icaoHex := ""
				if msg.Tail != "" {
					if hex, ok := regToHex[msg.Tail]; ok {
						icaoHex = hex
					} else {
						// Look up in aircraft table.
						if aircraft, err := pgDB.GetAircraftByRegistration(ctx, msg.Tail); err == nil && aircraft != nil {
							icaoHex = aircraft.ICAOHex
							regToHex[msg.Tail] = icaoHex
						} else {
							regToHex[msg.Tail] = "" // Cache miss too
						}
					}
				}

				// Extract enrichment if we have an ICAO hex.
				if icaoHex != "" {
					if update := enrichment.ExtractEnrichment(icaoHex, msg.Flight, msg.Timestamp, newResults); update != nil {
						if err := pgDB.UpsertFlightEnrichment(ctx, *update); err == nil {
							stats.Enrichments++
						}
					}
				}
			}

			// Collect for update if requested and there was a change.
			if *updateDB && result.DiffType != "unchanged" {
				updateBatch = append(updateBatch, storage.CHInsertParams{
					ID:          msg.ID,
					Timestamp:   msg.Timestamp,
					Label:       msg.Label,
					ParserType:  newParserType,
					Flight:      msg.Flight,
					Tail:        msg.Tail,
					Origin:      msg.Origin,
					Destination: msg.Destination,
					RawText:     msg.RawText,
					ParsedData:  parsedData,
					Confidence:  msg.Confidence,
				})
			}

			// Filter based on flags.
			if *regressionsOnly && result.DiffType != "regressed" {
				continue
			}
			if *improvementsOnly && result.DiffType != "improved" {
				continue
			}

			allResults = append(allResults, result)
		}

		offset += len(messages)
	}

	fmt.Printf("\r                                                    \r")

	// Print enrichment stats if requested.
	if pgDB != nil {
		fmt.Printf("Enrichment: %d records written to PostgreSQL\n", stats.Enrichments)
	}

	// Dump regressions to file if requested.
	if *dumpFile != "" {
		dumpRegressionsCH(*dumpFile, allResults)
	}

	// Apply updates if requested.
	if *updateDB && len(updateBatch) > 0 {
		fmt.Printf("Updating %d messages in ClickHouse...\n", len(updateBatch))
		if err := applyUpdates(ctx, chDB, updateBatch); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating messages: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Updates applied successfully.")
	}

	// Output.
	if *jsonOutput {
		outputReparseJSON(allResults, stats)
	} else {
		outputSummary(allResults, stats, *verbose, *regressionsOnly, *improvementsOnly)
	}
}

// countMessages returns the count of messages matching the filters.
func countMessages(ctx context.Context, db *storage.ClickHouseDB, parserType, label string) (uint64, error) {
	// Build query with both filters.
	query := "SELECT count() FROM messages WHERE 1=1"
	var args []interface{}

	if parserType != "" {
		query += " AND parser_type = ?"
		args = append(args, parserType)
	}
	if label != "" {
		query += " AND label = ?"
		args = append(args, label)
	}

	var count uint64
	row := db.Conn().QueryRow(ctx, query, args...)
	err := row.Scan(&count)
	return count, err
}

// queryMessages retrieves messages from ClickHouse with pagination.
func queryMessages(ctx context.Context, db *storage.ClickHouseDB, parserType, label string, limit, offset int) ([]storage.CHMessage, error) {
	return db.Query(ctx, storage.CHQueryParams{
		ParserType: parserType,
		Label:      label,
		Limit:      limit,
		Offset:     offset,
		OrderBy:    "id",
		OrderDesc:  false,
	})
}

// applyUpdates writes updated parse results back to ClickHouse by inserting new
// rows with the same ID.
//
// TODO: messages is a plain MergeTree, so these inserts never replace the
// original rows and every -update run creates permanent duplicates. The schema
// needs a ReplacingMergeTree (or the update needs ALTER TABLE ... DELETE first).
// missing_fields is also not carried over into the new rows.
func applyUpdates(ctx context.Context, db *storage.ClickHouseDB, updates []storage.CHInsertParams) error {
	// ClickHouse MergeTree doesn't support direct updates.
	// Options:
	// 1. Use ReplacingMergeTree (deduplicates on merge)
	// 2. Use ALTER TABLE DELETE + INSERT (slow for large datasets)
	// 3. Create a new table and swap
	//
	// For now, we'll insert with the same ID - if using ReplacingMergeTree this will
	// eventually deduplicate. Otherwise, this creates duplicates that need manual cleanup.

	fmt.Println("Note: ClickHouse updates work by inserting new rows with the same ID.")
	fmt.Println("If using ReplacingMergeTree, run OPTIMIZE TABLE to deduplicate.")

	return db.InsertBatch(ctx, updates)
}

// extractFields parses JSON and extracts non-empty string fields.
func extractFields(jsonStr string) map[string]string {
	fields := make(map[string]string)
	if jsonStr == "" {
		return fields
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return fields
	}

	for k, v := range data {
		// Skip metadata fields that aren't extracted from message text.
		if k == "message_id" || k == "timestamp" || k == "parse_confidence" ||
			k == "label" || k == "tail" || k == "aircraft_icao" {
			continue
		}

		switch val := v.(type) {
		case string:
			if val != "" {
				fields[k] = val
			}
		case float64:
			if val != 0 {
				fields[k] = fmt.Sprintf("%v", val)
			}
		case []interface{}:
			if len(val) > 0 {
				var parts []string
				for _, item := range val {
					parts = append(parts, fmt.Sprintf("%v", item))
				}
				fields[k] = strings.Join(parts, ",")
			}
		}
	}

	return fields
}

// resultToFields converts a registry.Result to a field map using reflection.
func resultToFields(result registry.Result) map[string]string {
	fields := make(map[string]string)
	if result == nil {
		return fields
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fields
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return fields
	}

	for k, v := range m {
		if k == "message_id" || k == "timestamp" || k == "parse_confidence" ||
			k == "label" || k == "tail" || k == "aircraft_icao" {
			continue
		}

		switch val := v.(type) {
		case string:
			if val != "" {
				fields[k] = val
			}
		case float64:
			if val != 0 {
				fields[k] = fmt.Sprintf("%v", val)
			}
		case []interface{}:
			if len(val) > 0 {
				var parts []string
				for _, item := range val {
					parts = append(parts, fmt.Sprintf("%v", item))
				}
				fields[k] = strings.Join(parts, ",")
			}
		}
	}

	return fields
}

// compareFieldsCH compares old and new field maps.
func compareFieldsCH(id uint64, parserType string, old, new map[string]string) ReparseResult {
	result := ReparseResult{
		ID:         id,
		ParserType: parserType,
		OldFields:  old,
		NewFields:  new,
	}

	// Find added fields (in new but not old).
	for k, v := range new {
		if _, ok := old[k]; !ok {
			result.Added = append(result.Added, k)
		} else if old[k] != v {
			result.Changed = append(result.Changed, k)
		}
	}

	// Find removed fields (in old but not new).
	for k := range old {
		if _, ok := new[k]; !ok {
			result.Removed = append(result.Removed, k)
		}
	}

	// Determine diff type.
	if len(result.Added) == 0 && len(result.Removed) == 0 && len(result.Changed) == 0 {
		result.DiffType = "unchanged"
	} else if len(result.Added) > 0 && len(result.Removed) == 0 {
		result.DiffType = "improved"
	} else if len(result.Removed) > 0 && len(result.Added) == 0 {
		result.DiffType = "regressed"
	} else {
		result.DiffType = "changed"
	}

	sort.Strings(result.Added)
	sort.Strings(result.Removed)
	sort.Strings(result.Changed)

	return result
}

// outputSummary prints a human-readable summary.
func outputSummary(results []ReparseResult, stats struct {
	Total       int
	Unchanged   int
	Improved    int
	Regressed   int
	Changed     int
	Enrichments int
	FieldStats  map[string]int
}, verbose, regressionsOnly, improvementsOnly bool) {

	fmt.Println("Re-parse Summary")
	fmt.Println("================")
	fmt.Printf("Total: %d  Unchanged: %d (%.1f%%)  Improved: %d (%.1f%%)  Regressed: %d (%.1f%%)  Changed: %d (%.1f%%)\n",
		stats.Total,
		stats.Unchanged, percent(stats.Unchanged, stats.Total),
		stats.Improved, percent(stats.Improved, stats.Total),
		stats.Regressed, percent(stats.Regressed, stats.Total),
		stats.Changed, percent(stats.Changed, stats.Total),
	)

	if !regressionsOnly {
		fmt.Println("\nTop Improvements:")
		printTopFieldStats(stats.FieldStats, "+", 10)
	}

	if !improvementsOnly {
		fmt.Println("\nTop Regressions:")
		printTopFieldStats(stats.FieldStats, "-", 10)
	}

	if verbose && len(results) > 0 {
		fmt.Println("\n---\nDetailed Changes:")
		for _, r := range results {
			if r.DiffType == "unchanged" {
				continue
			}
			fmt.Printf("\n--- Message ID: %d (%s) [%s] ---\n", r.ID, r.ParserType, r.DiffType)
			if len(r.Added) > 0 {
				fmt.Printf("  + Added: %s\n", strings.Join(r.Added, ", "))
			}
			if len(r.Removed) > 0 {
				fmt.Printf("  - Removed: %s\n", strings.Join(r.Removed, ", "))
			}
			if len(r.Changed) > 0 {
				fmt.Printf("  ~ Changed: %s\n", strings.Join(r.Changed, ", "))
			}
		}
	}
}

func printTopFieldStats(stats map[string]int, prefix string, limit int) {
	type kv struct {
		Field string
		Count int
	}
	var items []kv
	for k, v := range stats {
		if strings.HasPrefix(k, prefix) {
			items = append(items, kv{strings.TrimPrefix(k, prefix), v})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})

	if len(items) == 0 {
		fmt.Println("  (none)")
		return
	}
	for i, item := range items {
		if i >= limit {
			break
		}
		fmt.Printf("  %s %s: %d\n", prefix, item.Field, item.Count)
	}
}

func percent(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func outputReparseJSON(results []ReparseResult, stats struct {
	Total       int
	Unchanged   int
	Improved    int
	Regressed   int
	Changed     int
	Enrichments int
	FieldStats  map[string]int
}) {
	output := map[string]interface{}{
		"stats":   stats,
		"results": results,
	}
	data, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(data))
}

// reparseSingleMessageCH fetches a message by ID, reparses it, and shows the result.
func reparseSingleMessageCH(ctx context.Context, db *storage.ClickHouseDB, id uint64, asJSON bool) {
	msg, err := db.GetByID(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching message: %v\n", err)
		os.Exit(1)
	}
	if msg == nil {
		fmt.Fprintf(os.Stderr, "Message ID %d not found\n", id)
		os.Exit(1)
	}

	reg := registry.Default()
	reg.Sort()

	acarsMsg := &acars.Message{
		ID:    acars.FlexInt64(msg.ID),
		Label: msg.Label,
		Text:  msg.RawText,
		Tail:  msg.Tail,
	}

	matches := reg.Dispatch(acarsMsg)

	if asJSON {
		out := make([]map[string]interface{}, 0, len(matches))
		for _, m := range matches {
			out = append(out, map[string]interface{}{
				"parser": m.Parser,
				"type":   m.Result.Type(),
				"result": m.Result,
			})
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("=== Reparse Message ID: %d ===\n\n", id)
	fmt.Printf("Label:  %s\n", msg.Label)
	fmt.Printf("Tail:   %s\n", msg.Tail)
	fmt.Printf("\n--- Raw Text ---\n%s\n", msg.RawText)

	if len(matches) == 0 {
		fmt.Println("\n--- New Parse ---\n(no parser matched)")
		return
	}

	for _, m := range matches {
		fmt.Printf("\n--- New Parse (parser: %s, type: %s) ---\n", m.Parser, m.Result.Type())
		data, err := json.MarshalIndent(m.Result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling result: %v\n", err)
			continue
		}
		fmt.Println(string(data))
	}
}

// comparableMatch selects the new match to compare with a stored row.
//
// The messages table stores one row per parse result, labelled with the result
// type, and every parser produces a single result type. A stored row is
// therefore compared with the new result of the same type. If that type is no
// longer produced (or the row is "unparsed"), it is compared with the first
// match in dispatch order, so the change of type is reported.
func comparableMatch(storedType string, matches []registry.Match) (registry.Match, bool) {
	if len(matches) == 0 {
		return registry.Match{}, false
	}
	for _, m := range matches {
		if m.Result.Type() == storedType {
			return m, true
		}
	}
	return matches[0], true
}

// dumpRegressionsCH writes all regressed messages to a file.
func dumpRegressionsCH(filename string, results []ReparseResult) {
	var regressions []ReparseResult
	for _, r := range results {
		if r.DiffType == "regressed" {
			regressions = append(regressions, r)
		}
	}

	if len(regressions) == 0 {
		fmt.Println("No regressions to dump.")
		return
	}

	f, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dump file: %v\n", err)
		return
	}
	defer func() { _ = f.Close() }()

	for _, r := range regressions {
		entry := map[string]interface{}{
			"id":          r.ID,
			"parser_type": r.ParserType,
			"old_fields":  r.OldFields,
			"removed":     r.Removed,
			"changed":     r.Changed,
		}

		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		_, _ = f.Write(data)
		_, _ = f.WriteString("\n")
	}

	fmt.Printf("Dumped %d regressions to %s\n", len(regressions), filename)
}
