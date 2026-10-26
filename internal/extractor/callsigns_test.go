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

	// apply extracts a message with the memory, returning the flight's key.
	apply := func(c *CallsignMemory, tail, flight string, results []registry.Result, at time.Time) string {
		msg := &acars.Message{ID: 1, Label: "H1", Tail: tail, FlightNumber: flight}
		return c.Extract(msg, results, at).Flight.FlightNumber
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

	t.Run("the aircraft is the transmitted one", func(t *testing.T) {
		// A message without a tail is not matched by a registration a
		// result names, which backfill cannot see when it assigns workers.
		c := NewCallsignMemory(12 * time.Hour)
		apply(c, "A7-BAE", "VA0869", withCallsign, t0)
		named := []registry.Result{&reportLike{Registration: "A7-BAE", Origin: "ZSPD", Destination: "OTHH"}}
		if got := apply(c, "", "VA0869", named, t0.Add(time.Hour)); got != "VA869" {
			t.Errorf("no tail: %q, want VA869", got)
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

	t.Run("a report that may be about an earlier leg is not learned", func(t *testing.T) {
		// LO495 flies as LOT49H; a CMC report stored on an earlier LO495
		// leg flown as LOT495 keys only its own message.
		c := NewCallsignMemory(12 * time.Hour)
		apply(c, "SP-LWA", "LO495", []registry.Result{&mockResult{typeStr: "progress_report", FlightNumber: "LOT49H"}}, t0)
		if got := apply(c, "SP-LWA", "LO495", []registry.Result{&mockResult{typeStr: "cmc_report", FlightNumber: "LOT495"}}, t0.Add(time.Hour)); got != "LOT495" {
			t.Errorf("the report: %q, want LOT495", got)
		}
		if got := apply(c, "SP-LWA", "LO495", position, t0.Add(2*time.Hour)); got != "LOT49H" {
			t.Errorf("after the report: %q, want LOT49H", got)
		}
	})

	t.Run("routes are checked against the remembered callsign", func(t *testing.T) {
		// A report naming LOT49H describes LO495's flight once the memory
		// has it as LOT49H, though the numbers differ.
		c := NewCallsignMemory(12 * time.Hour)
		apply(c, "SP-LWA", "LO495", []registry.Result{&mockResult{typeStr: "progress_report", FlightNumber: "LOT49H"}}, t0)
		msg := &acars.Message{ID: 1, Label: "H1", Tail: "SP-LWA", FlightNumber: "LO495"}
		f := c.Extract(msg, []registry.Result{&reportLike{Flight: "LOT49H", Origin: "EPWA", Destination: "ENGM"}}, t0.Add(time.Hour)).Flight
		if f.FlightNumber != "LOT49H" || f.Origin != "EPWA" || f.Destination != "ENGM" {
			t.Errorf("flight %q, route %q-%q; want LOT49H, EPWA-ENGM", f.FlightNumber, f.Origin, f.Destination)
		}
	})

	t.Run("forgotten after the TTL", func(t *testing.T) {
		c := NewCallsignMemory(12 * time.Hour)
		apply(c, "A7-BAE", "VA0869", withCallsign, t0)
		if got := apply(c, "A7-BAE", "VA0869", position, t0.Add(13*time.Hour)); got != "VA869" {
			t.Errorf("after the TTL: %q, want VA869", got)
		}
	})

	t.Run("a callsign applies only to later messages", func(t *testing.T) {
		c := NewCallsignMemory(12 * time.Hour)
		apply(c, "A7-BAE", "VA0869", withCallsign, t0)
		// An earlier message may be of another operation of the flight.
		if got := apply(c, "A7-BAE", "VA0869", position, t0.Add(-time.Hour)); got != "VA869" {
			t.Errorf("an hour earlier: %q, want VA869", got)
		}
		// A joined message carries its first block's time (see
		// callsignGrace).
		if got := apply(c, "A7-BAE", "VA0869", position, t0.Add(-time.Minute)); got != "QTR869" {
			t.Errorf("a minute earlier: %q, want QTR869", got)
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
