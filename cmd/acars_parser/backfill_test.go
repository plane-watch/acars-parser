package main

import (
	"testing"
	"time"

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
