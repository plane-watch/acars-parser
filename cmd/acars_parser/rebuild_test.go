package main

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
)

// openTestClickHouse opens the ClickHouse test database (default
// acars_test, which must end in "_test"), creating it if needed, or skips
// the test if ClickHouse is not reachable.
func openTestClickHouse(t *testing.T) *storage.ClickHouseDB {
	t.Helper()
	env := func(k, d string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return d
	}
	database := env("CLICKHOUSE_TEST_DATABASE", "acars_test")
	if !testDatabaseRe.MatchString(database) {
		t.Fatalf("refusing to use database %q: it must be a plain name ending in _test", database)
	}
	port, _ := strconv.Atoi(env("CLICKHOUSE_PORT", "9000"))
	cfg := storage.ClickHouseConfig{Host: env("CLICKHOUSE_HOST", "localhost"), Port: port,
		User: env("CLICKHOUSE_USER", "default"), Password: env("CLICKHOUSE_PASSWORD", "acars"), Database: "default"}
	ctx := context.Background()
	admin, err := storage.OpenClickHouse(ctx, cfg)
	if err != nil {
		t.Skipf("ClickHouse not available: %v", err)
	}
	if err := admin.Conn().Exec(ctx, "CREATE DATABASE IF NOT EXISTS "+database); err != nil {
		t.Fatal(err)
	}
	_ = admin.Close()
	cfg.Database = database
	db, err := storage.OpenClickHouse(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// testDatabaseRe matches the names a test database may have: a plain
// identifier (it is interpolated into SQL) ending in "_test".
var testDatabaseRe = regexp.MustCompile(`^[a-z][a-z0-9_]*_test$`)

func TestRebuildArchive(t *testing.T) {
	db := openTestClickHouse(t)
	ctx := context.Background()
	for _, name := range []string{storage.MessagesTable, rebuildTable, previousTable} {
		if err := db.DropTable(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, name := range []string{storage.MessagesTable, rebuildTable, previousTable} {
			_ = db.DropTable(context.Background(), name)
		}
	})
	if err := db.CreateMessagesTable(ctx, storage.MessagesTable); err != nil {
		t.Fatal(err)
	}

	jan := time.Date(2026, 1, 5, 12, 0, 0, 123000000, time.UTC)
	oct := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	cpdlc := "/YEGE2YA.AT1B-1877224C8C0DE2B1624D9F3AA4F9C17A760F1D0"
	seed := []storage.CHInsertParams{
		// A January message stored twice (two old parse results), with
		// Airframes' flight in the flight column.
		{ID: 1, Timestamp: jan, Label: "AA", ParserType: "envelope", Flight: "CI0123", Tail: "B-18772", RawText: cpdlc, ParsedData: map[string]string{}},
		{ID: 1, Timestamp: jan, Label: "AA", ParserType: "unparsed", Flight: "CI0123", Tail: "B-18772", RawText: cpdlc, ParsedData: map[string]string{}},
		// A live-era message, whose flight was transmitted.
		{ID: 2, Timestamp: oct, Label: "H1", ParserType: "unparsed", Flight: "QF1", Tail: "VH-OQA", RawText: "NOTHING TO SEE", ParsedData: map[string]string{}},
		// A message without text keeps an unparsed row.
		{ID: 3, Timestamp: oct, Label: "_d", ParserType: "unparsed", Tail: "VH-OQA", ParsedData: map[string]string{}},
	}
	if err := db.InsertBatch(ctx, seed); err != nil {
		t.Fatal(err)
	}

	stats, err := rebuildArchive(ctx, db, registry.Default(), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), 2)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Messages != 3 || stats.Rows != 4 {
		t.Errorf("stats %+v, want 3 messages and 4 rows", stats)
	}

	type row struct {
		id                 uint64
		parserType, flight string
	}
	read := func(table string) []row {
		r, err := db.Conn().Query(ctx, "SELECT id, parser_type, flight FROM "+table+" ORDER BY id, parser_type")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = r.Close() }()
		var out []row
		for r.Next() {
			var x row
			if err := r.Scan(&x.id, &x.parserType, &x.flight); err != nil {
				t.Fatal(err)
			}
			out = append(out, x)
		}
		return out
	}
	got := read(storage.MessagesTable)
	want := []row{{1, "cpdlc", ""}, {1, "envelope", ""}, {2, "unparsed", "QF1"}, {3, "unparsed", ""}}
	if len(got) != len(want) {
		t.Fatalf("rebuilt rows %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rebuilt row %d = %v, want %v", i, got[i], want[i])
		}
	}
	if old := read(previousTable); len(old) != 4 {
		t.Errorf("%s has %d rows, want the 4 original rows", previousTable, len(old))
	}

	// The timestamp keeps its milliseconds.
	var got1 time.Time
	if err := db.Conn().QueryRow(ctx, "SELECT any(timestamp) FROM "+storage.MessagesTable+" WHERE id = 1").Scan(&got1); err != nil || !got1.Equal(jan) {
		t.Errorf("timestamp %v, %v; want %v", got1, err, jan)
	}

	// A second rebuild refuses to overwrite the kept previous archive.
	if _, err := rebuildArchive(ctx, db, registry.Default(), time.Time{}, 2); err == nil {
		t.Error("a second rebuild ran while messages_previous exists")
	}
}

// TestRebuildRefusesUnsafeStarts checks that a rebuild does not start with
// a bad batch size, or while a staging table exists (after a failure it may
// hold the only copy of the archive).
func TestRebuildRefusesUnsafeStarts(t *testing.T) {
	db := openTestClickHouse(t)
	ctx := context.Background()
	for _, name := range []string{storage.MessagesTable, rebuildTable, previousTable} {
		_ = db.DropTable(ctx, name)
	}
	if err := db.CreateMessagesTable(ctx, storage.MessagesTable); err != nil {
		t.Fatal(err)
	}
	if _, err := rebuildArchive(ctx, db, registry.Default(), time.Time{}, 0); err == nil {
		t.Error("batch size 0 accepted")
	}
	if exists, _ := db.TableExists(ctx, rebuildTable); exists {
		t.Error("a rejected rebuild created the staging table")
	}
	_ = db.DropTable(ctx, storage.MessagesTable)
	t.Cleanup(func() {
		for _, name := range []string{storage.MessagesTable, rebuildTable, previousTable} {
			_ = db.DropTable(context.Background(), name)
		}
	})
	for _, name := range []string{storage.MessagesTable, rebuildTable} {
		if err := db.CreateMessagesTable(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	keep := storage.CHInsertParams{ID: 1, Timestamp: time.Now(), Label: "H1", ParserType: "unparsed", RawText: "x", ParsedData: map[string]string{}}
	if err := db.InsertBatchInto(ctx, rebuildTable, []storage.CHInsertParams{keep}); err != nil {
		t.Fatal(err)
	}
	if _, err := rebuildArchive(ctx, db, registry.Default(), time.Time{}, 10); err == nil {
		t.Error("a rebuild started while the staging table exists")
	}
	if n, _ := db.CountDistinctIDs(ctx, rebuildTable); n != 1 {
		t.Errorf("the staging table was changed: %d messages", n)
	}
}

func TestTestDatabaseNames(t *testing.T) {
	for _, name := range []string{"foo --_test", "acars", "_test", "Acars_test"} {
		if testDatabaseRe.MatchString(name) {
			t.Errorf("%q accepted", name)
		}
	}
	if !testDatabaseRe.MatchString("acars_test") {
		t.Error("acars_test rejected")
	}
}
