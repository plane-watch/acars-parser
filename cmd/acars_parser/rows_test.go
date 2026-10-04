package main

import (
	"testing"

	"acars_parser/internal/acars"
	_ "acars_parser/internal/parsers"
	"acars_parser/internal/registry"
)

// TestMessageRows checks that a message gives one archive row per parse
// result, or one "unparsed" row, as live stores them.
func TestMessageRows(t *testing.T) {
	reg := registry.Default()

	parsed := &acars.Message{ID: 7, Label: "AA", Timestamp: "2026-01-05T12:00:00.000Z", Tail: "B-18772",
		Text: "/YEGE2YA.AT1B-1877224C8C0DE2B1624D9F3AA4F9C17A760F1D0"}
	rows := messageRows(parsed, registry.Results(reg.Dispatch(parsed)))
	types := map[string]bool{}
	for _, r := range rows {
		types[r.ParserType] = true
		if r.ID != 7 || r.Tail != "B-18772" || r.RawText != parsed.Text || r.Timestamp.Year() != 2026 {
			t.Errorf("row %+v", r)
		}
	}
	if !types["cpdlc"] || !types["envelope"] || len(rows) != 2 {
		t.Errorf("parser types %v, want cpdlc and envelope", types)
	}

	unparsed := &acars.Message{ID: 8, Label: "H1", Timestamp: "2026-01-05T12:00:00.000Z", FlightNumber: "QF1", Text: "NOTHING TO SEE"}
	rows = messageRows(unparsed, registry.Results(reg.Dispatch(unparsed)))
	if len(rows) != 1 || rows[0].ParserType != "unparsed" || rows[0].Flight != "QF1" {
		t.Errorf("unparsed rows %+v", rows)
	}

	if rows := messageRows(&acars.Message{ID: 9, Label: "H1"}, nil); len(rows) != 0 {
		t.Errorf("a message without text gave %d rows, want none", len(rows))
	}
}
