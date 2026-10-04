package storage

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// setupTestClickHouse connects to a ClickHouse test database, creating it
// if needed, or skips the test if ClickHouse is not reachable. It uses the
// CLICKHOUSE_HOST, CLICKHOUSE_PORT, CLICKHOUSE_USER and CLICKHOUSE_PASSWORD
// variables, and its own database, CLICKHOUSE_TEST_DATABASE (default
// acars_test), whose name must end in "_test", so that tests never touch
// real data.
func setupTestClickHouse(t *testing.T) *ClickHouseDB {
	t.Helper()
	database := envOr("CLICKHOUSE_TEST_DATABASE", "acars_test")
	if err := checkTestDatabaseName(database); err != nil {
		t.Fatal(err)
	}
	// The name is interpolated into SQL, so it must be a plain identifier.
	if !tableNameRe.MatchString(database) {
		t.Fatalf("refusing to use database %q: it must be a plain identifier", database)
	}
	port, err := strconv.Atoi(envOr("CLICKHOUSE_PORT", "9000"))
	if err != nil {
		t.Fatalf("CLICKHOUSE_PORT: %v", err)
	}
	cfg := ClickHouseConfig{
		Host:     envOr("CLICKHOUSE_HOST", "localhost"),
		Port:     port,
		User:     envOr("CLICKHOUSE_USER", "default"),
		Password: envOr("CLICKHOUSE_PASSWORD", "acars"),
		Database: "default",
	}
	ctx := context.Background()
	admin, err := OpenClickHouse(ctx, cfg)
	if err != nil {
		t.Skipf("ClickHouse not available: %v", err)
	}
	if err := admin.conn.Exec(ctx, "CREATE DATABASE IF NOT EXISTS "+database); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	_ = admin.Close()

	cfg.Database = database
	db, err := OpenClickHouse(ctx, cfg)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestMessagesTableRebuildPrimitives checks the functions a rebuild of the
// message archive uses: creating a messages table by name, inserting into
// it, streaming each message's content once (a message has a row per parse
// result), counting distinct IDs and swapping tables.
func TestMessagesTableRebuildPrimitives(t *testing.T) {
	db := setupTestClickHouse(t)
	ctx := context.Background()
	const a, b = "rebuild_test_a", "rebuild_test_b"
	for _, name := range []string{a, b} {
		if err := db.DropTable(ctx, name); err != nil {
			t.Fatal(err)
		}
		if err := db.CreateMessagesTable(ctx, name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.DropTable(context.Background(), name) })
	}

	ts := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	rows := []CHInsertParams{
		{ID: 1, Timestamp: ts, Label: "H1", ParserType: "pwi", Tail: "VH-EBO", RawText: "one", ParsedData: map[string]string{}},
		{ID: 1, Timestamp: ts, Label: "H1", ParserType: "fpn", Tail: "VH-EBO", RawText: "one", ParsedData: map[string]string{}},
		{ID: 2, Timestamp: ts, Label: "AA", ParserType: "unparsed", Flight: "QF1", Tail: "VH-OQA", RawText: "two", ParsedData: map[string]string{}},
	}
	if err := db.InsertBatchInto(ctx, a, rows); err != nil {
		t.Fatal(err)
	}

	n, err := db.CountDistinctIDs(ctx, a)
	if err != nil || n != 2 {
		t.Fatalf("CountDistinctIDs = %d, %v; want 2", n, err)
	}

	got := map[uint64]RawMessage{}
	// A chunk of one message reads each message in its own range.
	if err := db.StreamRawMessages(ctx, a, 1, func(m RawMessage) error {
		if _, dup := got[m.ID]; dup {
			t.Errorf("message %d streamed twice", m.ID)
		}
		got[m.ID] = m
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[2].Flight != "QF1" || got[2].RawText != "two" || got[1].Tail != "VH-EBO" || !got[1].Timestamp.Equal(ts) {
		t.Errorf("streamed %+v", got)
	}

	if err := db.ExchangeTables(ctx, a, b); err != nil {
		t.Fatal(err)
	}
	if n, _ := db.CountDistinctIDs(ctx, b); n != 2 {
		t.Errorf("after the exchange, %s has %d messages, want 2", b, n)
	}
	if n, _ := db.CountDistinctIDs(ctx, a); n != 0 {
		t.Errorf("after the exchange, %s has %d messages, want 0", a, n)
	}

	if exists, err := db.TableExists(ctx, b); err != nil || !exists {
		t.Errorf("TableExists(%s) = %v, %v", b, exists, err)
	}
	if err := db.RenameTable(ctx, b, "rebuild_test_c"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.DropTable(context.Background(), "rebuild_test_c") })
	if exists, _ := db.TableExists(ctx, b); exists {
		t.Errorf("%s still exists after the rename", b)
	}
}

func TestTableNamesAreChecked(t *testing.T) {
	for _, name := range []string{"messages; DROP TABLE x", "Messages", "", "a b"} {
		if err := checkTableName(name); err == nil {
			t.Errorf("checkTableName(%q) accepted", name)
		}
	}
}

// TestSwapInTable checks that the staging table becomes the current one and
// the current one is kept, in one step.
func TestSwapInTable(t *testing.T) {
	db := setupTestClickHouse(t)
	ctx := context.Background()
	const cur, stg, prev = "swap_test_current", "swap_test_staging", "swap_test_previous"
	for _, name := range []string{cur, stg, prev} {
		_ = db.DropTable(ctx, name)
		t.Cleanup(func() { _ = db.DropTable(context.Background(), name) })
	}
	for _, name := range []string{cur, stg} {
		if err := db.CreateMessagesTable(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	row := func(id uint64) []CHInsertParams {
		return []CHInsertParams{{ID: id, Timestamp: time.Now(), Label: "H1", ParserType: "unparsed", RawText: "x", ParsedData: map[string]string{}}}
	}
	if err := db.InsertBatchInto(ctx, cur, row(1)); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertBatchInto(ctx, stg, append(row(2), row(3)...)); err != nil {
		t.Fatal(err)
	}
	if err := db.SwapInTable(ctx, cur, stg, prev); err != nil {
		t.Fatal(err)
	}
	if n, _ := db.CountDistinctIDs(ctx, cur); n != 2 {
		t.Errorf("current has %d messages, want the staging table's 2", n)
	}
	if n, _ := db.CountDistinctIDs(ctx, prev); n != 1 {
		t.Errorf("previous has %d messages, want the old current table's 1", n)
	}
	if exists, _ := db.TableExists(ctx, stg); exists {
		t.Error("the staging table still exists")
	}
}

// TestTableUUIDFollowsTheTable checks that a table's UUID stays with it
// when the table is exchanged, so it identifies the table whatever its name.
func TestTableUUIDFollowsTheTable(t *testing.T) {
	db := setupTestClickHouse(t)
	ctx := context.Background()
	const a, b = "uuid_test_a", "uuid_test_b"
	for _, name := range []string{a, b} {
		_ = db.DropTable(ctx, name)
		if err := db.CreateMessagesTable(ctx, name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.DropTable(context.Background(), name) })
	}
	ua, err := db.TableUUID(ctx, a)
	if err != nil || ua == "" {
		t.Fatalf("TableUUID(%s) = %q, %v", a, ua, err)
	}
	if err := db.ExchangeTables(ctx, a, b); err != nil {
		t.Fatal(err)
	}
	if ub, _ := db.TableUUID(ctx, b); ub != ua {
		t.Errorf("after the exchange, %s has UUID %q, want %q", b, ub, ua)
	}
}
