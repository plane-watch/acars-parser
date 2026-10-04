package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"acars_parser/internal/baseline"
	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
	"acars_parser/internal/version"
)

// defaultBaselineDir is where the regression gate test reads its fixtures.
const defaultBaselineDir = "internal/parsers/testdata/baseline"

// baselineOrdering documents the deterministic ranking within each stratum.
const baselineOrdering = "cityHash64(id), id"

// baselineTimeFormat is the UTC format used for fixture timestamps.
const baselineTimeFormat = "2006-01-02T15:04:05.000Z"

func runBaselineCmd(args []string) {
	fs := flag.NewFlagSet("baseline", flag.ExitOnError)

	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPassword := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")
	chDB := fs.String("ch-db", defaultCHDatabase(), "ClickHouse database")
	outDir := fs.String("out", defaultBaselineDir, "Directory to write the baseline fixtures to")
	perStratum := fs.Int("per-stratum", 100, "Maximum messages per stratum (label and current result types)")
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

	reg := registry.Default()
	reg.Sort()
	sampler := baseline.NewSampler(*perStratum)

	stats, err := sampleBaseline(ctx, db, *cutoff, func(c baseline.Case, hash uint64) error {
		expected, err := baseline.Observe(reg.Dispatch(c.Message()))
		if err != nil {
			return fmt.Errorf("message %d: %w", c.ID, err)
		}
		c.Expected = expected
		c.Stratum = baseline.StratumOf(c.Label, expected)
		sampler.Offer(c, hash)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError sampling messages: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	cases := sampler.Cases()
	parserVersion := version.Parser()
	manifest := baseline.Manifest{
		Source:     "clickhouse " + *chDB + ".messages",
		Cutoff:     *cutoff,
		PerStratum: *perStratum,
		Ordering:   baselineOrdering,
		Strata:     sampler.Strata(),
		SampledBy:  parserVersion,
		RecordedBy: parserVersion,
	}
	if err := baseline.Save(*outDir, cases, manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing baseline: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scanned %d messages (%d duplicated IDs kept once, %d IDs excluded because their stored copies conflict).\n",
		stats.scanned, stats.duplicatesKept, stats.conflictsExcluded)
	fmt.Printf("Wrote %d cases from %d strata to %s\n", len(cases), sampler.Strata(), *outDir)
}

// sampleStats counts what the baseline scan saw.
type sampleStats struct {
	scanned           int
	duplicatesKept    int
	conflictsExcluded int
}

// sampleBaseline calls offer for every message stored at or before the cutoff,
// once per message ID, with the message's cityHash64(id) rank.
//
// An ID stored more than once is offered once if every copy has the same input
// fields, and is excluded (and counted) if the copies conflict, since no copy
// can be chosen as the input without guessing. Duplicates are resolved in a
// separate, small query so that the scan of every other message needs neither
// a GROUP BY nor a sort over the whole table; the sampler does not depend on
// the order messages are offered in.
func sampleBaseline(ctx context.Context, db *storage.ClickHouseDB, cutoff string, offer func(baseline.Case, uint64) error) (sampleStats, error) {
	var stats sampleStats

	// The connection's default 60-second max_execution_time is too short for a
	// scan that parses every message as it streams, so it is lifted for these
	// two queries only.
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{"max_execution_time": 0}))

	const window = `timestamp <= toDateTime64(?, 3, 'UTC')`
	const duplicateIDs = `SELECT id FROM messages WHERE ` + window + ` GROUP BY id HAVING count() > 1`
	const inputs = `tuple(timestamp, label, tail, flight, raw_text)`

	// Duplicated IDs: one canonical copy each (the earliest stored, chosen as
	// a whole tuple), unless the copies' inputs differ. Inputs are compared
	// exactly, not by hash.
	dupRows, err := db.Conn().Query(ctx, `
		SELECT id, t.1, t.2, t.3, t.4, t.5, cityHash64(id), variants
		FROM (
			SELECT id,
			       argMin(`+inputs+`, created_at) AS t,
			       uniqExact(`+inputs+`) AS variants
			FROM messages
			WHERE `+window+` AND id IN (`+duplicateIDs+`)
			GROUP BY id
		)`, cutoff, cutoff)
	if err != nil {
		return stats, fmt.Errorf("query duplicated IDs: %w", err)
	}
	defer func() { _ = dupRows.Close() }()
	for dupRows.Next() {
		c, hash, variants, err := scanBaselineRow(dupRows, true)
		if err != nil {
			return stats, err
		}
		if variants > 1 {
			stats.conflictsExcluded++
			continue
		}
		stats.duplicatesKept++
		stats.scanned++
		if err := offer(c, hash); err != nil {
			return stats, err
		}
	}
	if err := dupRows.Err(); err != nil {
		return stats, fmt.Errorf("iterate duplicated IDs: %w", err)
	}

	// Every other message, streamed in storage order.
	rows, err := db.Conn().Query(ctx, `
		SELECT id, timestamp, label, tail, flight, raw_text, cityHash64(id)
		FROM messages
		WHERE `+window+` AND id NOT IN (`+duplicateIDs+`)`, cutoff, cutoff)
	if err != nil {
		return stats, fmt.Errorf("query messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		c, hash, _, err := scanBaselineRow(rows, false)
		if err != nil {
			return stats, err
		}
		stats.scanned++
		if stats.scanned%1_000_000 == 0 {
			fmt.Printf("\rScanned %d messages...", stats.scanned)
		}
		if err := offer(c, hash); err != nil {
			return stats, err
		}
	}
	if err := rows.Err(); err != nil {
		return stats, fmt.Errorf("iterate messages: %w", err)
	}
	return stats, nil
}

// rowScanner is the part of the ClickHouse driver's rows used by the scan.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanBaselineRow reads one message row; withVariants reads the extra
// conflicting-copies column of the duplicated-IDs query.
func scanBaselineRow(rows rowScanner, withVariants bool) (baseline.Case, uint64, uint64, error) {
	var (
		id, hash, variants              uint64
		ts                              time.Time
		label, tail, flight, rawMessage string
	)
	dest := []any{&id, &ts, &label, &tail, &flight, &rawMessage, &hash}
	if withVariants {
		dest = append(dest, &variants)
	}
	if err := rows.Scan(dest...); err != nil {
		return baseline.Case{}, 0, 0, fmt.Errorf("scan: %w", err)
	}
	return baseline.Case{
		ID:        int64(id),
		Timestamp: ts.UTC().Format(baselineTimeFormat),
		Label:     label,
		Tail:      tail,
		Flight:    flight,
		Text:      rawMessage,
	}, hash, variants, nil
}
