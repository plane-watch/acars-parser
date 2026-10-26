package storage

import (
	"context"
	"errors"
	"fmt"
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

// TestBlockColumns checks that a message's block fields (its message
// number, block ID and block_end flag, and for a joined message the IDs of
// its blocks) are stored and streamed back, and that a message without
// them reads back empty.
func TestBlockColumns(t *testing.T) {
	db := setupTestClickHouse(t)
	ctx := context.Background()
	const table = "block_columns_test"
	_ = db.DropTable(ctx, table)
	if err := db.CreateMessagesTable(ctx, table); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.DropTable(context.Background(), table) })

	ts := time.Date(2026, 10, 23, 15, 5, 0, 0, time.UTC)
	no := false
	rows := []CHInsertParams{
		{ID: 10, Timestamp: ts, Label: "H1", ParserType: "unparsed", RawText: "A", ParsedData: map[string]string{},
			BlockFields: BlockFields{MessageNumber: "D47A", BlockID: "5", BlockEnd: &no, LinkDirection: "downlink"}},
		{ID: 10 | 1<<62, Timestamp: ts, Label: "H1", ParserType: "unparsed", RawText: "AB", ParsedData: map[string]string{},
			BlockFields: BlockFields{MessageNumber: "D47", BlockID: "5", AssembledFrom: []uint64{10, 11}}},
		{ID: 12, Timestamp: ts, Label: "H1", ParserType: "unparsed", RawText: "C", ParsedData: map[string]string{}},
		// A message stored before the link direction was, whose CPDLC
		// result recorded the direction: its other row has none.
		{ID: 13, Timestamp: ts, Label: "H1", ParserType: "envelope", RawText: "D", ParsedData: map[string]string{}},
		{ID: 13, Timestamp: ts, Label: "H1", ParserType: "cpdlc", RawText: "D", ParsedData: map[string]string{"direction": "uplink"}},
		// Rows of one ID with different text: the direction is taken only
		// from the rows of the text streamed (the first in order, "E").
		{ID: 14, Timestamp: ts, Label: "H1", ParserType: "unparsed", RawText: "E", ParsedData: map[string]string{}},
		{ID: 14, Timestamp: ts, Label: "H1", ParserType: "cpdlc", RawText: "F", ParsedData: map[string]string{"direction": "downlink"}},
	}
	if err := db.InsertBatchInto(ctx, table, rows); err != nil {
		t.Fatal(err)
	}
	got := map[uint64]RawMessage{}
	if err := db.StreamRawMessages(ctx, table, 10, func(m RawMessage) error { got[m.ID] = m; return nil }); err != nil {
		t.Fatal(err)
	}
	if b := got[10]; b.MessageNumber != "D47A" || b.BlockID != "5" || b.BlockEnd == nil || *b.BlockEnd || len(b.AssembledFrom) != 0 ||
		b.LinkDirection != "downlink" {
		t.Errorf("block: %+v", b)
	}
	if j := got[10|1<<62]; j.MessageNumber != "D47" || j.BlockEnd != nil || len(j.AssembledFrom) != 2 || j.AssembledFrom[1] != 11 {
		t.Errorf("joined message: %+v", j)
	}
	if m := got[12]; m.MessageNumber != "" || m.BlockID != "" || m.BlockEnd != nil || len(m.AssembledFrom) != 0 || m.StoredDirection != "" {
		t.Errorf("message without block fields: %+v", m)
	}
	if m := got[14]; m.RawText != "E" || m.StoredDirection != "" {
		t.Errorf("message with two texts: %+v, want text E without F's direction", m)
	}
	if m := got[13]; m.StoredDirection != "uplink" || m.RawText != "D" {
		t.Errorf("message with a stored direction: %+v", m)
	}
}

// TestCreateSchemaAddsBlockColumns checks that CreateSchema adds the block
// columns to a messages table created before they existed.
func TestCreateSchemaAddsBlockColumns(t *testing.T) {
	db := setupTestClickHouse(t)
	ctx := context.Background()
	_ = db.DropTable(ctx, MessagesTable)
	t.Cleanup(func() { _ = db.DropTable(context.Background(), MessagesTable) })
	// The table as it was before the block columns.
	if err := db.conn.Exec(ctx, `CREATE TABLE messages (
			id UInt64, timestamp DateTime64(3), label LowCardinality(String), parser_type LowCardinality(String),
			flight LowCardinality(String), tail LowCardinality(String), origin LowCardinality(String),
			destination LowCardinality(String), raw_text String, parsed_json String, missing_fields String,
			confidence Float32, created_at DateTime64(3) DEFAULT now64(3))
		ENGINE = MergeTree() ORDER BY (parser_type, label, timestamp, id)`); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckSchema(ctx); !errors.Is(err, ErrSchemaOutdated) {
		t.Errorf("CheckSchema before the update = %v, want ErrSchemaOutdated", err)
	}
	if err := db.CreateSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckSchema(ctx); err != nil {
		t.Errorf("CheckSchema after the update = %v, want nil", err)
	}
	var n uint64
	if err := db.conn.QueryRow(ctx, `SELECT count() FROM system.columns WHERE database = currentDatabase()
		AND table = 'messages' AND name IN ('message_number', 'block_id', 'block_end', 'assembled_from', 'link_direction')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("%d of the 5 block columns exist", n)
	}
}

// TestQueryTimeOrder checks the order backfill reads the archive in: by
// time, with a joined message (whose ID sorts after every received one) at
// its first block's time, and ties broken by ID so that a message's rows
// stay together.
func TestQueryTimeOrder(t *testing.T) {
	db := setupTestClickHouse(t)
	ctx := context.Background()
	if err := db.CreateSchema(ctx); err != nil {
		t.Fatal(err)
	}
	const label = "Q9" // Not a real label, so that only these rows are read.
	clear := func() {
		_ = db.conn.Exec(context.Background(), "ALTER TABLE messages DELETE WHERE label = ? SETTINGS mutations_sync = 1", label)
	}
	clear()
	t.Cleanup(clear)

	t0 := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	row := func(id uint64, at time.Time, parser string) CHInsertParams {
		return CHInsertParams{ID: id, Timestamp: at, Label: label, ParserType: parser, RawText: "x", ParsedData: map[string]string{}}
	}
	rows := []CHInsertParams{
		row(5, t0.Add(time.Minute), "b"),
		row(2, t0.Add(time.Second), "a"),
		row(3, t0.Add(time.Minute), "a"),
		row(5, t0.Add(time.Minute), "a"),
		row(1|1<<62, t0, "a"),
		row(3, t0.Add(time.Minute), "b"),
	}
	if err := db.InsertBatchInto(ctx, "messages", rows); err != nil {
		t.Fatal(err)
	}
	got, err := db.Query(ctx, CHQueryParams{Label: label, OrderBy: "timestamp", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var ids []uint64
	for _, m := range got {
		ids = append(ids, m.ID)
	}
	want := []uint64{1 | 1<<62, 2, 3, 3, 5, 5}
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Errorf("IDs %v, want %v", ids, want)
	}
}

// TestMessagesAfter checks the paging backfill reads the archive with:
// whole messages in time order, at millisecond precision (two messages may
// share a time), none repeated or skipped when a row is inserted before the
// key between pages (a message live joined from its blocks), within the
// row limit except for a message with more rows than it, and an empty page
// after the last message.
func TestMessagesAfter(t *testing.T) {
	db := setupTestClickHouse(t)
	ctx := context.Background()
	if err := db.CreateSchema(ctx); err != nil {
		t.Fatal(err)
	}
	// The test database's messages table holds only these rows.
	clear := func() {
		_ = db.conn.Exec(context.Background(), "TRUNCATE TABLE messages")
	}
	clear()
	t.Cleanup(clear)

	t0 := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	rows := func(id uint64, at time.Time, n int) []CHInsertParams {
		var out []CHInsertParams
		for i := 0; i < n; i++ {
			out = append(out, CHInsertParams{ID: id, Timestamp: at, Label: "Q8", ParserType: fmt.Sprintf("p%d", i), RawText: "x", ParsedData: map[string]string{}})
		}
		return out
	}
	var all []CHInsertParams
	all = append(all, rows(1, t0.Add(ms(10123)), 2)...)
	all = append(all, rows(2, t0.Add(ms(10124)), 2)...)
	all = append(all, rows(3, t0.Add(ms(30500)), 4)...) // More rows than the limit.
	all = append(all, rows(4, t0.Add(ms(40001)), 1)...)
	all = append(all, rows(6, t0.Add(ms(40001)), 1)...) // The same time as 4.
	if err := db.InsertBatchInto(ctx, "messages", all); err != nil {
		t.Fatal(err)
	}

	var got []string
	var key MessageKey
	for page := 0; ; page++ {
		if page == 10 {
			t.Fatalf("no empty page after %v", got)
		}
		msgs, next, err := db.MessagesAfter(ctx, key, "", 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) == 0 {
			break
		}
		ids := ""
		for _, m := range msgs {
			ids += fmt.Sprint(m.ID)
		}
		got = append(got, ids)
		key = next
		if page == 0 {
			// A joined message, inserted after the first page with an
			// earlier time than the key.
			if err := db.InsertBatchInto(ctx, "messages", rows(5|1<<62, t0.Add(ms(5)), 1)); err != nil {
				t.Fatal(err)
			}
		}
	}
	want := []string{"11", "22", "3333", "46"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("pages %v, want %v", got, want)
	}

	// With a parser type, only its rows are read.
	msgs, _, err := db.MessagesAfter(ctx, MessageKey{}, "p3", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].ID != 3 {
		t.Errorf("p3 rows: %+v, want message 3's", msgs)
	}
}

// TestPageEnd checks which rows a page returns: whole messages within the
// limit, the first message whole even beyond it, and none while the first
// message may continue past the rows read.
func TestPageEnd(t *testing.T) {
	t0 := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	msgs := func(ids ...uint64) []CHMessage {
		var out []CHMessage
		for _, id := range ids {
			out = append(out, CHMessage{ID: id, Timestamp: t0.Add(time.Duration(id) * time.Millisecond)})
		}
		return out
	}
	tests := []struct {
		name  string
		rows  []CHMessage
		atEnd bool
		limit int
		want  int
	}{
		{"the last message may continue", msgs(1, 1, 2), false, 3, 2},
		{"at the end, all whole", msgs(1, 1, 2), true, 3, 3},
		{"the first message may continue", msgs(1, 1, 1), false, 3, 0},
		{"the first message beyond the limit", msgs(1, 1, 1, 2), true, 2, 3},
		{"within the limit only", msgs(1, 1, 2, 3, 3), true, 1, 2},
		{"the same ID at another time is another message", append(msgs(1), CHMessage{ID: 1, Timestamp: t0}), true, 1, 1},
	}
	for _, tt := range tests {
		if got := pageEnd(tt.rows, tt.atEnd, tt.limit); got != tt.want {
			t.Errorf("%s: %d, want %d", tt.name, got, tt.want)
		}
	}
}
