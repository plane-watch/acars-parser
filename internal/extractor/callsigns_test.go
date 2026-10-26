package extractor

import (
	"testing"
	"time"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

func TestCallsignMemory(t *testing.T) {
	t0 := time.Date(2026, 10, 5, 19, 0, 0, 0, time.UTC)
	withCallsign := []registry.Result{&mockResult{typeStr: "progress_report", FlightNumber: "QTR869"}}
	position := []registry.Result{&mockResult{typeStr: "position", Origin: "ZSPD", Destination: "OTHH"}}

	// apply extracts a message and applies the memory to it, returning the
	// flight's key.
	apply := func(c *CallsignMemory, tail, flight string, results []registry.Result, at time.Time) string {
		msg := &acars.Message{ID: 1, Label: "H1", Tail: tail, FlightNumber: flight}
		data := Extract(msg, results)
		c.Apply(msg, &data, at)
		return data.Flight.FlightNumber
	}

	t.Run("a message without a callsign takes the one remembered", func(t *testing.T) {
		c := NewCallsignMemory(12 * time.Hour)
		if got := apply(c, "A7-BAE", "VA0869", position, t0); got != "VA869" {
			t.Errorf("before any callsign: %q, want VA869", got)
		}
		apply(c, "A7-BAE", "VA0869", withCallsign, t0.Add(time.Minute))
		if got := apply(c, "A7-BAE", "VA0869", position, t0.Add(2*time.Hour)); got != "QTR869" {
			t.Errorf("after the callsign: %q, want QTR869", got)
		}
	})

	t.Run("only for the same aircraft and transmitted flight", func(t *testing.T) {
		c := NewCallsignMemory(12 * time.Hour)
		apply(c, "A7-BAE", "VA0869", withCallsign, t0)
		if got := apply(c, "VH-8VG", "VA0869", position, t0.Add(time.Hour)); got != "VA869" {
			t.Errorf("another aircraft: %q, want VA869", got)
		}
		if got := apply(c, "A7-BAE", "VA0870", position, t0.Add(time.Hour)); got != "VA870" {
			t.Errorf("another flight: %q, want VA870", got)
		}
	})

	t.Run("forgotten after the TTL", func(t *testing.T) {
		c := NewCallsignMemory(12 * time.Hour)
		apply(c, "A7-BAE", "VA0869", withCallsign, t0)
		if got := apply(c, "A7-BAE", "VA0869", position, t0.Add(13*time.Hour)); got != "VA869" {
			t.Errorf("after the TTL: %q, want VA869", got)
		}
	})

	t.Run("a message out of order uses and keeps the newest callsign", func(t *testing.T) {
		// A joined message is archived after its blocks' neighbours (see
		// reassembly), so backfill reads it out of order.
		c := NewCallsignMemory(12 * time.Hour)
		apply(c, "A7-BAE", "VA0869", withCallsign, t0)
		if got := apply(c, "A7-BAE", "VA0869", position, t0.Add(-time.Hour)); got != "QTR869" {
			t.Errorf("earlier message: %q, want QTR869", got)
		}
		older := []registry.Result{&mockResult{typeStr: "progress_report", FlightNumber: "VOZ869"}}
		apply(c, "A7-BAE", "VA0869", older, t0.Add(-2*time.Hour))
		if got := apply(c, "A7-BAE", "VA0869", position, t0.Add(time.Hour)); got != "QTR869" {
			t.Errorf("after an older callsign: %q, want QTR869", got)
		}
	})

	t.Run("expired entries are removed", func(t *testing.T) {
		c := NewCallsignMemory(time.Hour)
		apply(c, "A7-BAE", "VA0869", withCallsign, t0)
		apply(c, "A7-BEF", "VA009A", []registry.Result{&mockResult{typeStr: "progress_report", FlightNumber: "QTR9A"}}, t0.Add(3*time.Hour))
		if n := c.Len(); n != 1 {
			t.Errorf("%d entries, want 1", n)
		}
	})
}
