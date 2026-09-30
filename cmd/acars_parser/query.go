package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"acars_parser/internal/storage"
)

func runQueryCmd(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)

	// ClickHouse connection flags.
	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPass := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")
	chDB := fs.String("ch-db", defaultCHDatabase(), "ClickHouse database")

	// Query flags.
	msgID := fs.Uint64("id", 0, "Fetch a specific message by database row ID")
	parserType := fs.String("type", "", "Filter by parser type (e.g. 'h1_position', 'pdc')")
	label := fs.String("label", "", "Filter by ACARS label (e.g. 'H1', '16')")
	flight := fs.String("flight", "", "Filter by flight number (partial match)")
	hasMissing := fs.Bool("has-missing", false, "Only show messages with any missing fields")
	fullText := fs.String("search", "", "Full-text search on raw message text")
	limit := fs.Int("limit", 20, "Max results to return")
	offset := fs.Int("offset", 0, "Pagination offset")
	orderBy := fs.String("order", "id", "Sort by field (id, timestamp, parser_type, confidence)")
	desc := fs.Bool("desc", true, "Sort descending")
	showRaw := fs.Bool("raw", false, "Show raw message text")
	showJSON := fs.Bool("json", false, "Output as JSON")
	statsOnly := fs.Bool("stats", false, "Show statistics only")
	listTypes := fs.Bool("list-types", false, "List all parser types in the database")
	listMissing := fs.Bool("list-missing", false, "List top missing fields across all messages")
	_ = fs.Parse(args) // ExitOnError handles parse failures.

	// Open the ClickHouse database.
	ctx := context.Background()
	db, err := storage.OpenClickHouse(ctx, storage.ClickHouseConfig{
		Host:     *chHost,
		Port:     *chPort,
		Database: *chDB,
		User:     *chUser,
		Password: *chPass,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening ClickHouse: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	// Handle special modes.
	if *statsOnly {
		showStats(ctx, db)
		return
	}

	if *listTypes {
		listDistinct(ctx, db, "parser_type")
		return
	}

	if *listMissing {
		showMissingFieldStats(ctx, db)
		return
	}

	// Handle single message by database row ID.
	if *msgID != 0 {
		showSingleMessage(ctx, db, *msgID, *showJSON)
		return
	}

	// Build and run the query.
	params := storage.CHQueryParams{
		ParserType: *parserType,
		Label:      *label,
		Flight:     *flight,
		HasMissing: *hasMissing,
		FullText:   *fullText,
		Limit:      *limit,
		Offset:     *offset,
		OrderBy:    *orderBy,
		OrderDesc:  *desc,
	}

	messages, err := db.Query(ctx, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying database: %v\n", err)
		os.Exit(1)
	}

	if len(messages) == 0 {
		fmt.Println("No messages found matching the query.")
		return
	}

	// TODO: This line is also printed in -json mode, so the output is not valid JSON.
	fmt.Printf("Found %d messages:\n\n", len(messages))

	if *showJSON {
		outputJSON(messages)
		return
	}

	for i, m := range messages {
		fmt.Printf("--- Message %d (ID: %d) ---\n", i+1, m.ID)
		fmt.Printf("Type: %s | Label: %s | Flight: %s | Tail: %s\n",
			m.ParserType, m.Label, m.Flight, m.Tail)

		if m.Origin != "" || m.Destination != "" {
			fmt.Printf("Route: %s -> %s\n", m.Origin, m.Destination)
		}

		if m.MissingFields != "" {
			fmt.Printf("Missing: %s\n", m.MissingFields)
		}

		if m.Confidence > 0 {
			fmt.Printf("Confidence: %.1f%%\n", m.Confidence*100)
		}

		if *showRaw {
			fmt.Printf("Raw: %s\n", m.RawText)
		}

		// Pretty print the parsed JSON.
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(m.ParsedJSON), &parsed); err == nil {
			// Remove some noisy fields for display.
			delete(parsed, "message_id")
			delete(parsed, "timestamp")
			delete(parsed, "raw_text")

			if prettyJSON, err := json.MarshalIndent(parsed, "", "  "); err == nil {
				fmt.Printf("Parsed:\n%s\n", string(prettyJSON))
			}
		}

		fmt.Println()
	}
}

func showSingleMessage(ctx context.Context, db *storage.ClickHouseDB, id uint64, asJSON bool) {
	m, err := db.GetByID(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching message: %v\n", err)
		os.Exit(1)
	}
	if m == nil {
		fmt.Fprintf(os.Stderr, "Message ID %d not found\n", id)
		os.Exit(1)
	}

	if asJSON {
		outputJSON([]storage.CHMessage{*m})
		return
	}

	fmt.Printf("=== Message ID: %d ===\n\n", m.ID)
	fmt.Printf("Type:   %s\n", m.ParserType)
	fmt.Printf("Label:  %s\n", m.Label)
	fmt.Printf("Flight: %s\n", m.Flight)
	fmt.Printf("Tail:   %s\n", m.Tail)

	if m.Origin != "" || m.Destination != "" {
		fmt.Printf("Route:  %s -> %s\n", m.Origin, m.Destination)
	}

	if m.MissingFields != "" {
		fmt.Printf("Missing: %s\n", m.MissingFields)
	}

	fmt.Printf("\n--- Raw Text ---\n%s\n", m.RawText)

	// Pretty print parsed JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(m.ParsedJSON), &parsed); err == nil {
		delete(parsed, "message_id")
		delete(parsed, "timestamp")
		delete(parsed, "raw_text")

		if prettyJSON, err := json.MarshalIndent(parsed, "", "  "); err == nil {
			fmt.Printf("\n--- Parsed ---\n%s\n", string(prettyJSON))
		}
	}
}

func showStats(ctx context.Context, db *storage.ClickHouseDB) {
	stats, err := db.GetStats(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting stats: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Database Statistics ===")
	fmt.Printf("Total messages: %d\n", stats.TotalMessages)
	fmt.Printf("With missing fields: %d (%.1f%%)\n",
		stats.WithMissing, float64(stats.WithMissing)/float64(stats.TotalMessages)*100)

	fmt.Println("\n--- By Parser Type ---")
	// Sort by count descending.
	type kv struct {
		Key   string
		Value uint64
	}
	var sortedTypes []kv
	for k, v := range stats.ByParserType {
		sortedTypes = append(sortedTypes, kv{k, v})
	}
	sort.Slice(sortedTypes, func(i, j int) bool {
		return sortedTypes[i].Value > sortedTypes[j].Value
	})
	for _, kv := range sortedTypes {
		fmt.Printf("  %s: %d\n", kv.Key, kv.Value)
	}

	fmt.Println("\n--- By Label ---")
	var sortedLabels []kv
	for k, v := range stats.ByLabel {
		sortedLabels = append(sortedLabels, kv{k, v})
	}
	sort.Slice(sortedLabels, func(i, j int) bool {
		return sortedLabels[i].Value > sortedLabels[j].Value
	})
	for _, kv := range sortedLabels {
		fmt.Printf("  %s: %d\n", kv.Key, kv.Value)
	}
}

func showMissingFieldStats(ctx context.Context, db *storage.ClickHouseDB) {
	stats, err := db.GetStats(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting stats: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Top Missing Fields ===")
	fmt.Printf("(across %d messages with missing fields)\n\n", stats.WithMissing)

	// Sort by count descending.
	type kv struct {
		Key   string
		Value uint64
	}
	var sorted []kv
	for k, v := range stats.TopMissingFields {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	for i, kv := range sorted {
		if i >= 30 { // Limit to top 30.
			break
		}
		fmt.Printf("  %s: %d\n", kv.Key, kv.Value)
	}
}

func listDistinct(ctx context.Context, db *storage.ClickHouseDB, column string) {
	values, err := db.Distinct(ctx, column)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing %s: %v\n", column, err)
		os.Exit(1)
	}

	fmt.Printf("=== Distinct %s values ===\n", column)
	for _, v := range values {
		fmt.Printf("  %s\n", v)
	}
}

func outputJSON(messages []storage.CHMessage) {
	type jsonMessage struct {
		ID            uint64                 `json:"id"`
		ParserType    string                 `json:"parser_type"`
		Label         string                 `json:"label"`
		Flight        string                 `json:"flight,omitempty"`
		Tail          string                 `json:"tail,omitempty"`
		Origin        string                 `json:"origin,omitempty"`
		Destination   string                 `json:"destination,omitempty"`
		RawText       string                 `json:"raw_text"`
		Parsed        map[string]interface{} `json:"parsed"`
		MissingFields []string               `json:"missing_fields,omitempty"`
		Confidence    float32                `json:"confidence,omitempty"`
	}

	var output []jsonMessage
	for _, m := range messages {
		jm := jsonMessage{
			ID:          m.ID,
			ParserType:  m.ParserType,
			Label:       m.Label,
			Flight:      m.Flight,
			Tail:        m.Tail,
			Origin:      m.Origin,
			Destination: m.Destination,
			RawText:     m.RawText,
			Confidence:  m.Confidence,
		}

		if m.MissingFields != "" {
			jm.MissingFields = strings.Split(m.MissingFields, ",")
		}

		if err := json.Unmarshal([]byte(m.ParsedJSON), &jm.Parsed); err != nil {
			jm.Parsed = map[string]interface{}{"error": "failed to parse"}
		}

		output = append(output, jm)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output)
}
