// Package main provides the unparse command for marking messages as unparsed.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"acars_parser/internal/storage"
)

func runUnparseCmd(args []string) {
	fs := flag.NewFlagSet("unparse", flag.ExitOnError)

	// ClickHouse connection options.
	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chDatabase := fs.String("ch-database", defaultCHDatabase(), "ClickHouse database")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPassword := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")

	// Filter options.
	msgID := fs.Uint64("id", 0, "Unparse a specific message by ID")
	parserType := fs.String("type", "", "Filter by parser type (e.g., 'pdc')")
	contains := fs.String("contains", "", "Filter by text content (case-insensitive)")
	dryRun := fs.Bool("dry-run", false, "Show what would be updated without making changes")
	noOptimize := fs.Bool("no-optimize", false, "Skip OPTIMIZE TABLE after updates")
	limit := fs.Int("limit", -1, "Limit number of messages (0 = unlimited, default 100 for safety)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Require at least one filter to prevent accidental mass updates.
	if *msgID == 0 && *parserType == "" && *contains == "" {
		fmt.Fprintln(os.Stderr, "Error: must specify at least one filter (-id, -type, or -contains)")
		os.Exit(1)
	}

	// Default limit of 100 for safety unless explicitly set.
	// -1 = not specified (use default 100), 0 = unlimited, >0 = explicit limit.
	effectiveLimit := *limit
	if effectiveLimit < 0 && *msgID == 0 {
		effectiveLimit = 100
		fmt.Printf("Note: Using default limit of %d. Use -limit 0 for unlimited.\n", effectiveLimit)
	} else if effectiveLimit < 0 {
		effectiveLimit = 0 // Single ID lookup, no limit needed.
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

	// Build the query.
	var conditions []string
	var queryArgs []interface{}

	if *msgID > 0 {
		conditions = append(conditions, "id = ?")
		queryArgs = append(queryArgs, *msgID)
	}
	if *parserType != "" {
		conditions = append(conditions, "parser_type = ?")
		queryArgs = append(queryArgs, *parserType)
	}
	if *contains != "" {
		conditions = append(conditions, "positionCaseInsensitive(raw_text, ?) > 0")
		queryArgs = append(queryArgs, *contains)
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count matching messages first.
	countQuery := fmt.Sprintf("SELECT count() FROM messages WHERE %s", whereClause)
	var totalCount uint64
	if err := chDB.Conn().QueryRow(ctx, countQuery, queryArgs...).Scan(&totalCount); err != nil {
		fmt.Fprintf(os.Stderr, "Error counting messages: %v\n", err)
		os.Exit(1)
	}

	if totalCount == 0 {
		fmt.Println("No matching messages found.")
		return
	}

	fmt.Printf("Found %d matching messages\n", totalCount)

	// Build the select query.
	selectQuery := fmt.Sprintf(`
		SELECT id, timestamp, label, flight, tail, origin, destination, raw_text
		FROM messages
		WHERE %s
		ORDER BY id
	`, whereClause)

	if effectiveLimit > 0 {
		selectQuery += fmt.Sprintf(" LIMIT %d", effectiveLimit)
	}

	// Query matching messages.
	rows, err := chDB.Conn().Query(ctx, selectQuery, queryArgs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying messages: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var updates []storage.CHInsertParams
	for rows.Next() {
		var (
			id          uint64
			timestamp   time.Time
			label       string
			flight      string
			tail        string
			origin      string
			destination string
			rawText     string
		)

		if err := rows.Scan(&id, &timestamp, &label, &flight, &tail, &origin, &destination, &rawText); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning row: %v\n", err)
			continue
		}

		updates = append(updates, storage.CHInsertParams{
			ID:          id,
			Timestamp:   timestamp,
			Label:       label,
			ParserType:  "unparsed",
			Flight:      flight,
			Tail:        tail,
			Origin:      origin,
			Destination: destination,
			RawText:     rawText,
			ParsedData:  map[string]interface{}{},
			Confidence:  0,
		})

		if *dryRun {
			// Show a preview of the message.
			preview := rawText
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			preview = strings.ReplaceAll(preview, "\n", " ")
			fmt.Printf("  ID %d: %s\n", id, preview)
		}
	}

	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error iterating rows: %v\n", err)
		os.Exit(1)
	}

	if len(updates) == 0 {
		fmt.Println("No messages to update.")
		return
	}

	if *dryRun {
		fmt.Printf("\nDry run: would mark %d messages as unparsed.\n", len(updates))
		fmt.Println("Run without -dry-run to apply changes.")
		return
	}

	// Apply updates.
	fmt.Printf("Marking %d messages as unparsed...\n", len(updates))
	if err := chDB.InsertBatch(ctx, updates); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating messages: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Updated %d messages.\n", len(updates))

	// Optimise table to deduplicate unless skipped.
	// TODO: messages is a plain MergeTree (see storage.ClickHouseDB.CreateSchema),
	// so OPTIMIZE ... FINAL does not remove the original parsed rows. Each unparse
	// leaves both rows in place. Deduplication needs a ReplacingMergeTree (keyed
	// on id with a version column) or ALTER TABLE ... DELETE before the insert.
	if !*noOptimize {
		fmt.Println("Running OPTIMIZE TABLE to deduplicate...")
		if err := chDB.Conn().Exec(ctx, "OPTIMIZE TABLE messages FINAL"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: OPTIMIZE TABLE failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "You may need to run it manually: OPTIMIZE TABLE messages FINAL")
		} else {
			fmt.Println("Done.")
		}
	} else {
		fmt.Println("Skipped OPTIMIZE TABLE. Run manually if needed: OPTIMIZE TABLE messages FINAL")
	}
}
