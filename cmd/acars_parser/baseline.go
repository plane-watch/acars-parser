package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"acars_parser/internal/baseline"
	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
	"acars_parser/internal/version"
)

// defaultBaselineDir is where the regression gate test reads its fixtures.
const defaultBaselineDir = "internal/parsers/testdata/baseline"

// baselineOrdering documents the deterministic ordering within each stratum.
const baselineOrdering = "cityHash64(id), id"

func runBaselineCmd(args []string) {
	fs := flag.NewFlagSet("baseline", flag.ExitOnError)

	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPassword := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")
	chDB := fs.String("ch-db", defaultCHDatabase(), "ClickHouse database")
	outDir := fs.String("out", defaultBaselineDir, "Directory to write the baseline fixtures to")
	perStratum := fs.Int("per-stratum", 100, "Maximum messages per (label, parser type) stratum")
	cutoff := fs.String("cutoff", "", "Only sample messages at or before this UTC time (YYYY-MM-DD HH:MM:SS); required")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}
	if *cutoff == "" {
		fmt.Fprintln(os.Stderr, "Error: -cutoff is required so that the sample is reproducible")
		os.Exit(1)
	}
	if _, err := time.Parse(time.DateTime, *cutoff); err != nil {
		fmt.Fprintf(os.Stderr, "Error: -cutoff must be YYYY-MM-DD HH:MM:SS: %v\n", err)
		os.Exit(1)
	}
	if *perStratum < 1 {
		fmt.Fprintln(os.Stderr, "Error: -per-stratum must be at least 1")
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := storage.OpenClickHouse(ctx, storage.ClickHouseConfig{
		Host:     *chHost,
		Port:     *chPort,
		Database: *chDB,
		User:     *chUser,
		Password: *chPassword,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening ClickHouse: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	cases, strata, err := sampleBaseline(ctx, db, *cutoff, *perStratum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sampling messages: %v\n", err)
		os.Exit(1)
	}

	// Record what the current parsers produce for each sampled message.
	reg := registry.Default()
	reg.Sort()
	for i := range cases {
		expected, err := baseline.Observe(reg.Dispatch(cases[i].Message()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error recording message %d: %v\n", cases[i].ID, err)
			os.Exit(1)
		}
		cases[i].Expected = expected
	}

	if err := baseline.Write(*outDir, cases); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing baseline: %v\n", err)
		os.Exit(1)
	}
	manifest := baseline.Manifest{
		Source:        "clickhouse " + *chDB + ".messages",
		Cutoff:        *cutoff,
		PerStratum:    *perStratum,
		Ordering:      baselineOrdering,
		Strata:        strata,
		Cases:         len(cases),
		ParserVersion: version.Parser(),
	}
	if err := baseline.WriteManifest(*outDir, manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing manifest: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Wrote %d cases from %d strata to %s\n", len(cases), strata, *outDir)
}

// sampleBaseline draws up to perStratum messages from each (label, parser type)
// stratum at or before the cutoff. Messages are first deduplicated by ID (the
// earliest stored row wins), then ordered within each stratum by a hash of the
// ID with the ID as a tie-breaker, so the same data always yields the same sample.
func sampleBaseline(ctx context.Context, db *storage.ClickHouseDB, cutoff string, perStratum int) ([]baseline.Case, int, error) {
	query := `
		SELECT id, toString(timestamp) AS ts, label, tail, flight, raw_text, parser_type
		FROM (
			SELECT id, timestamp, label, tail, flight, raw_text, parser_type
			FROM messages
			WHERE timestamp <= toDateTime64(?, 3, 'UTC')
			ORDER BY id, created_at, parser_type
			LIMIT 1 BY id
		)
		ORDER BY label, parser_type, ` + baselineOrdering + `
		LIMIT ? BY label, parser_type`

	rows, err := db.Conn().Query(ctx, query, cutoff, perStratum)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cases []baseline.Case
	strata := make(map[string]bool)
	for rows.Next() {
		var (
			id                                 uint64
			ts, label, tail, flight, text, ptp string
		)
		if err := rows.Scan(&id, &ts, &label, &tail, &flight, &text, &ptp); err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		stratum := label + "/" + ptp
		strata[stratum] = true
		cases = append(cases, baseline.Case{
			ID:        int64(id),
			Timestamp: ts,
			Label:     label,
			Tail:      tail,
			Flight:    flight,
			Text:      text,
			Stratum:   stratum,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate: %w", err)
	}
	return cases, len(strata), nil
}
