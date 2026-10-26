package main

import (
	"testing"
	"time"

	"acars_parser/internal/extractor"
	"acars_parser/internal/storage"
)

// TestMessageGrouper checks that the backfill gives the extractor each
// message once, with all its parse results and its transmitted flight, as
// live does, including a message whose rows are split between two pages
// of the query.
func TestMessageGrouper(t *testing.T) {
	ts := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	row := func(id uint64, parser, flight, json string) storage.CHMessage {
		return storage.CHMessage{ID: id, Timestamp: ts, Label: "H1", ParserType: parser, Flight: flight, Tail: "VH-OQA", ParsedJSON: json}
	}
	var g messageGrouper
	var items []*backfillWorkItem
	add := func(m storage.CHMessage) {
		if item := g.add(m); item != nil {
			items = append(items, item)
		}
	}
	// The first page.
	add(row(1, "delta_header", "DL0447", `{"origin":"KATL","destination":"KIAD"}`))
	add(row(1, "acms_report", "DL0447", `{"origin":"KATL","destination":"KIAD"}`))
	add(row(2, "unparsed", "DL0447", `{"label":"H1"}`))
	add(row(3, "progress_report", "", `{"destination":"EGLL"}`))
	// The second page continues message 3.
	add(row(3, "h1_position", "", `{"latitude":51.5}`))
	add(row(4, "eta", "QF0001", `{"origin":"YSSY"}`))
	if item := g.flush(); item != nil {
		items = append(items, item)
	}

	want := []struct {
		id      int64
		flight  string
		results int
	}{{1, "DL0447", 2}, {3, "", 2}, {4, "QF0001", 1}}
	if len(items) != len(want) {
		t.Fatalf("got %d messages, want %d", len(items), len(want))
	}
	for i, w := range want {
		got := items[i]
		if int64(got.msg.ID) != w.id || got.msg.FlightNumber != w.flight || len(got.results) != w.results || got.msg.Tail != "VH-OQA" {
			t.Errorf("message %d: id %d, flight %q, %d results; want %d, %q, %d",
				i, got.msg.ID, got.msg.FlightNumber, len(got.results), w.id, w.flight, w.results)
		}
	}
}

// TestWorkerFor checks that an aircraft's messages go to one worker, so
// that its callsign memory sees them in order.
func TestWorkerFor(t *testing.T) {
	const n = 4
	if a, b := workerFor("A7-BAE", n), workerFor("a7bae", n); a != b {
		t.Errorf("A7-BAE to worker %d, a7bae to %d; want the same", a, b)
	}
	if w := workerFor("", n); w != 0 {
		t.Errorf("no tail to worker %d, want 0", w)
	}
	used := map[int]bool{}
	for _, tail := range []string{"A7-BAE", "A7-BEF", "VH-8VG", "VH-UVQ", "N123AB", "G-XLEA", "JA731A", "9V-SMA"} {
		w := workerFor(tail, n)
		if w < 0 || w >= n {
			t.Fatalf("%s to worker %d, out of range", tail, w)
		}
		used[w] = true
	}
	if len(used) < 2 {
		t.Errorf("8 tails used %d of %d workers", len(used), n)
	}
}

// TestBackfillArchived5L checks that an archived 5L route, stored with the
// 5L designator as its callsign, is parsed again from its raw text, so that
// it neither keys its flight as JST1701 nor teaches a QF1701/JST1701 alias.
func TestBackfillArchived5L(t *testing.T) {
	var g messageGrouper
	g.add(storage.CHMessage{
		ID: 1, Timestamp: time.Date(2026, 10, 8, 2, 0, 0, 0, time.UTC), Label: "5L", ParserType: "route",
		Flight: "QF1701", Tail: "VH-UVQ",
		RawText:    "JST1701,.VH-UVQ,ZNE,YNWN,PER,YPPH,056032,08OCT26,2357,0006,0135,0141",
		ParsedJSON: `{"callsign":"JST1701","tail":"VH-UVQ","origin_icao":"YNWN","dest_icao":"YPPH"}`,
	})
	item := g.flush()
	if item == nil || len(item.results) != 1 {
		t.Fatalf("got %+v, want one message with one result", item)
	}
	data := extractor.Extract(item.msg, item.results)
	if data.FlightAlias != nil {
		t.Errorf("FlightAlias = %+v, want none", data.FlightAlias)
	}
	f := data.Flight
	if f.FlightNumber != "QF1701" || f.Origin != "YNWN" || f.Destination != "YPPH" {
		t.Errorf("flight %q, route %q-%q; want QF1701, YNWN-YPPH", f.FlightNumber, f.Origin, f.Destination)
	}
}

// TestMessageGrouperTimestamp checks that rows sharing an ID but not a
// time are separate messages, as MessagesAfter pages them.
func TestMessageGrouperTimestamp(t *testing.T) {
	t0 := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	var g messageGrouper
	g.add(storage.CHMessage{ID: 7, Timestamp: t0, Label: "H1", ParserType: "a", Tail: "VH-OQA", ParsedJSON: `{}`})
	first := g.add(storage.CHMessage{ID: 7, Timestamp: t0.Add(time.Second), Label: "H1", ParserType: "b", Tail: "VH-OQA", ParsedJSON: `{}`})
	second := g.flush()
	if first == nil || second == nil || len(first.results) != 1 || len(second.results) != 1 || !second.timestamp.Equal(t0.Add(time.Second)) {
		t.Errorf("got %+v and %+v, want two messages of one result", first, second)
	}
}

// TestFirstMessages checks that -limit counts messages, not their rows.
func TestFirstMessages(t *testing.T) {
	t0 := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	rows := []storage.CHMessage{
		{ID: 1, Timestamp: t0, ParserType: "a"}, {ID: 1, Timestamp: t0, ParserType: "b"},
		{ID: 2, Timestamp: t0}, {ID: 2, Timestamp: t0.Add(time.Second)},
	}
	for _, tt := range []struct{ n, rows, count int }{{1, 2, 1}, {2, 3, 2}, {3, 4, 3}, {9, 4, 3}} {
		got, count := firstMessages(rows, tt.n)
		if len(got) != tt.rows || count != tt.count {
			t.Errorf("first %d: %d rows, %d messages; want %d, %d", tt.n, len(got), count, tt.rows, tt.count)
		}
	}
}
