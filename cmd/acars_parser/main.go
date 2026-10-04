// Package main provides the acars_parser command-line tool for parsing ACARS messages.
package main

import (
	"fmt"
	"os"

	"acars_parser/internal/registry"

	// Import all parsers for side effects (registration).
	_ "acars_parser/internal/parsers"
)

func main() {
	// Initialise the registry.
	reg := registry.Default()
	reg.Sort()

	// Check for subcommands.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "extract":
			runExtractCmd(os.Args[2:])
			return
		case "live":
			runLiveCmd(os.Args[2:])
			return
		case "query":
			runQueryCmd(os.Args[2:])
			return
		case "reparse":
			runReparseCmd(os.Args[2:])
			return
		case "debug":
			runDebugCmd(os.Args[2:])
			return
		case "review":
			runReviewCmd(os.Args[2:])
			return
		case "templates":
			runTemplatesCmd(os.Args[2:])
			return
		case "backfill":
			runBackfillCmd(os.Args[2:])
			return
		case "migrate":
			runMigrateCmd(os.Args[2:])
			return
		case "baseline":
			runBaselineCmd(os.Args[2:])
			return
		case "unparse":
			runUnparseCmd(os.Args[2:])
			return
		case "help", "-h", "--help":
			printHelp()
			return
		}
	}

	// Default: show help.
	printHelp()
}

func printHelp() {
	fmt.Println(`ACARS Message Parser

Usage:
  acars_parser [command] [options]

Commands:
  extract   Extract data from JSONL files
  live      Connect to NATS bus and parse live messages
  query     Query stored messages in ClickHouse
  reparse   Re-parse stored messages to find improvements/regressions
  backfill  Extract state data from ClickHouse to PostgreSQL
  debug     Debug why a message didn't parse correctly
  review    Launch web UI for reviewing and annotating messages
  migrate   Migrate from SQLite to ClickHouse/PostgreSQL (legacy)
  unparse   Mark messages as unparsed (fix false positives)
  templates Analyse message templates and patterns
  help      Show this help

Extract Options:
  -input FILE      Input JSONL file (default: stdin)
  -output FILE     Output JSON file (default: stdout)
  -pretty          Pretty print JSON output
  -all             Include all parsed data types

Live Options:
  -server URL       NATS server URL (default: nats://157.90.242.138:4222)
  -creds FILE       Path to NATS credentials file (required)
  -subject SUBJ     NATS subject (default: v1.aircraft.ingest.*.message.*.created)
  -output FILE      Optional JSONL output file
  -ch-host HOST     ClickHouse host (default: localhost)
  -ch-port PORT     ClickHouse port (default: 9000)
  -ch-database DB   ClickHouse database (default: acars)
  -ch-user USER     ClickHouse user (default: default)
  -ch-password PASS ClickHouse password (default: acars)
  -pg-host HOST     PostgreSQL host for state (default: localhost)
  -pg-port PORT     PostgreSQL port (default: 5432)
  -pg-database DB   PostgreSQL database (default: acars_state)
  -pg-user USER     PostgreSQL user (default: acars)
  -pg-password PASS PostgreSQL password (default: acars)
  -no-store         Disable database storage
  -all              Show all messages, not just parsed ones
  -raw              Show raw message text
  -empty            Show missing/empty fields
  -v                Verbose output
  -debug LABELS     Debug specific labels (comma-separated)
  -exclude TYPES    Exclude result types (comma-separated)

Query Options:
  -ch-host HOST     ClickHouse host (default: localhost)
  -ch-port PORT     ClickHouse port (default: 9000)
  -ch-database DB   ClickHouse database (default: acars)
  -ch-user USER     ClickHouse user (default: default)
  -ch-password PASS ClickHouse password (default: acars)
  -type TYPE        Filter by parser type (e.g. 'flight_plan', 'pdc')
  -label LABEL      Filter by ACARS label (e.g. 'H1', '16')
  -flight FLIGHT    Filter by flight number
  -has-missing      Only show messages with missing fields
  -search TEXT      Full-text search on raw message text
  -limit N          Max results (default: 100)
  -offset N         Skip first N results
  -order-by FIELD   Order by field (default: id)
  -desc             Order descending
  -raw              Show raw message text
  -json             Output as JSON
  -stats            Show database statistics only
  -list-types       List all parser types

Reparse Options:
  -ch-host HOST         ClickHouse host (default: localhost)
  -ch-port PORT         ClickHouse port (default: 9000)
  -ch-database DB       ClickHouse database (default: acars)
  -ch-user USER         ClickHouse user (optional)
  -ch-password PASS     ClickHouse password (optional)
  -type TYPE            Filter by parser type
  -label LABEL          Filter by ACARS label
  -v                    Verbose output: show detailed diffs
  -regressions-only     Show only messages that regressed
  -improvements-only    Show only messages that improved
  -limit N              Limit number of messages to process
  -update               Update ClickHouse with new parse results
  -batch N              Batch size for updates (default: 10000)
  -rebuild              Reparse every message into a new table and swap it in
                        (stop live first; the old archive is kept as messages_previous)
  -drop-flight-before D With -rebuild: blank the stored flight before date D (YYYY-MM-DD)
  -json                 Output as JSON

Debug Options:
  -ch-host HOST     ClickHouse host (default: localhost)
  -ch-port PORT     ClickHouse port (default: 9000)
  -ch-database DB   ClickHouse database (default: acars)
  -ch-user USER     ClickHouse user (default: default)
  -ch-password PASS ClickHouse password (default: acars)
  -id N             Message ID to debug (from ClickHouse)
  -text TEXT        Raw message text to debug (instead of -id)
  -label LABEL      ACARS label for raw text (e.g. 'H1', '16')
  -all              Show all pattern attempts, not just matches
  -type TYPE        Only show trace for specific parser type (e.g. 'pdc')

Review Options:
  -ch-host HOST     ClickHouse host (default: localhost)
  -ch-port PORT     ClickHouse port (default: 9000)
  -ch-database DB   ClickHouse database (default: acars)
  -ch-user USER     ClickHouse user (default: default)
  -ch-password PASS ClickHouse password (default: acars)
  -pg-host HOST     PostgreSQL host (default: localhost)
  -pg-port PORT     PostgreSQL port (default: 5432)
  -pg-database DB   PostgreSQL database (default: acars_state)
  -pg-user USER     PostgreSQL user (default: acars)
  -pg-password PASS PostgreSQL password (default: acars)
  -port N           HTTP port (default: 8080)
  -type TYPE        Pre-filter to specific parser type

Backfill Options:
  -ch-host HOST       ClickHouse host (default: localhost)
  -ch-port PORT       ClickHouse port (default: 9000)
  -ch-database DB     ClickHouse database (default: acars)
  -ch-user USER       ClickHouse user (optional)
  -ch-password PASS   ClickHouse password (optional)
  -pg-host HOST       PostgreSQL host (default: localhost)
  -pg-port PORT       PostgreSQL port (default: 5432)
  -pg-database DB     PostgreSQL database (default: acars_state)
  -pg-user USER       PostgreSQL user (default: acars)
  -pg-password PASS   PostgreSQL password (default: acars)
  -type TYPE          Filter by parser type (e.g. 'flight_plan', 'pdc')
  -limit N            Limit number of messages (0 = all)
  -workers N          Number of worker goroutines (default: 8)
  -batch N            Batch size for querying messages (default: 10000)
  -v                  Verbose output

Migrate Options:
  -messages-db FILE   SQLite messages database (default: messages.db)
  -state-db FILE      SQLite state database (default: state.db)
  -ch-host HOST       ClickHouse host (default: localhost)
  -ch-port PORT       ClickHouse port (default: 9000)
  -ch-database DB     ClickHouse database (default: acars)
  -pg-host HOST       PostgreSQL host (default: localhost)
  -pg-port PORT       PostgreSQL port (default: 5432)
  -pg-database DB     PostgreSQL database (default: acars_state)
  -pg-user USER       PostgreSQL user (default: acars)
  -pg-password PASS   PostgreSQL password (default: acars)
  -batch N            Batch size for migration (default: 10000)
  -dry-run            Preview counts without migrating
  -skip-messages      Skip message migration
  -skip-state         Skip state migration
  -resume-from ID     Resume message migration from this ID

Unparse Options:
  -ch-host HOST       ClickHouse host (default: localhost)
  -ch-port PORT       ClickHouse port (default: 9000)
  -ch-database DB     ClickHouse database (default: acars)
  -ch-user USER       ClickHouse user (optional)
  -ch-password PASS   ClickHouse password (optional)
  -id N               Unparse a specific message by ID
  -type TYPE          Filter by parser type (e.g., 'pdc')
  -contains TEXT      Filter by text content (case-insensitive)
  -dry-run            Show what would be updated without changes
  -no-optimize        Skip OPTIMIZE TABLE after updates
  -limit N            Limit number of messages (default: 100 for safety)

Environment Variables:
  CLICKHOUSE_HOST       ClickHouse host (default: localhost)
  CLICKHOUSE_PORT       ClickHouse port (default: 9000)
  CLICKHOUSE_DATABASE   ClickHouse database (default: acars)
  CLICKHOUSE_USER       ClickHouse user (default: default)
  CLICKHOUSE_PASSWORD   ClickHouse password (default: acars)
  POSTGRES_HOST         PostgreSQL host (default: localhost)
  POSTGRES_PORT         PostgreSQL port (default: 5432)
  POSTGRES_DATABASE     PostgreSQL database (default: acars_state)
  POSTGRES_USER         PostgreSQL user (default: acars)
  POSTGRES_PASSWORD     PostgreSQL password (default: acars)

Examples:
  # Collect live messages (uses env vars or defaults for database)
  acars_parser live -creds ./my.creds

  # Query messages from ClickHouse
  acars_parser query -type pdc -limit 10

  # Search for specific text patterns
  acars_parser query -search "POSN" -raw

  # Show statistics about stored messages
  acars_parser query -stats

  # Find all flight_plan messages with missing fields
  acars_parser query -type flight_plan -has-missing

  # Re-parse all messages and show summary
  acars_parser reparse

  # Re-parse unparsed messages and show detailed diffs
  acars_parser reparse -type unparsed -v

  # Find regressions after pattern changes
  acars_parser reparse -regressions-only

  # Re-parse and update ClickHouse with new results
  acars_parser reparse -type unparsed -update

  # Debug why a specific message didn't parse correctly
  acars_parser debug -id 12345

  # Debug raw text with label
  acars_parser debug -text "PDC 291826 JST501 A320 YSSY..." -label 80

  # Show all pattern attempts (including failures)
  acars_parser debug -id 12345 -all

  # Launch review web UI
  acars_parser review

  # Launch review UI on custom port, filtered to PDC
  acars_parser review -port 9000 -type pdc

  # Backfill PostgreSQL state from ClickHouse messages
  acars_parser backfill

  # Backfill only PDC messages with verbose output
  acars_parser backfill -type pdc -v

  # Preview PDC messages containing "NO PDC AVAILABLE" (dry run)
  acars_parser unparse -type pdc -contains "NO PDC AVAILABLE" -dry-run

  # Mark PDC acknowledgements as unparsed
  acars_parser unparse -type pdc -contains "PDC/ASAT" -limit 0

  # Unparse a specific message by ID
  acars_parser unparse -id 12345`)
}
