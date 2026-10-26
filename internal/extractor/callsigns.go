package extractor

import (
	"sync"
	"time"

	"acars_parser/internal/acars"
)

// CallsignTTL is how long a callsign is remembered for an aircraft's
// transmitted flight: longer than the longest flight, during which the
// callsign is given again (by FMS progress and position reports), and
// shorter than the time before an aircraft flies the same transmitted
// flight again.
const CallsignTTL = 12 * time.Hour

// CallsignMemory remembers, per aircraft and transmitted flight, the
// callsign that the aircraft's messages gave for it (ExtractedData.Callsign),
// so that its messages that give none (position reports, for instance) are
// keyed by the same flight. A Qatar 777 transmitting "VA0869" while flying
// QTR869 sends most of its reports without the callsign; without the
// memory, their routes would be stored under VA869, which Virgin Australia
// also flies.
//
// Messages are expected roughly in time order, per aircraft. A message out
// of order (a joined message, archived after its neighbours) uses the
// newest callsign within the TTL, and does not replace it.
//
// A CallsignMemory is safe for concurrent use.
type CallsignMemory struct {
	ttl time.Duration

	mu      sync.Mutex
	entries map[callsignKey]callsignEntry
	newest  time.Time // The newest time applied.
	pruned  time.Time // When expired entries were last removed.
}

// callsignKey is an aircraft (its registration, or without one its
// address) and a transmitted flight, without leading zeros.
type callsignKey struct {
	aircraft, flight string
}

// callsignEntry is a callsign and when it was last given.
type callsignEntry struct {
	callsign string
	seen     time.Time
}

// NewCallsignMemory returns a CallsignMemory that forgets a callsign once
// it has not been given for ttl.
func NewCallsignMemory(ttl time.Duration) *CallsignMemory {
	return &CallsignMemory{ttl: ttl, entries: make(map[callsignKey]callsignEntry)}
}

// Len returns the number of callsigns remembered.
func (c *CallsignMemory) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Apply records the callsign that data, extracted from msg at the time at,
// gives for the transmitted flight; or, when data gives none, keys data's
// flight by the callsign remembered for the aircraft's transmitted flight.
func (c *CallsignMemory) Apply(msg *acars.Message, data *ExtractedData, at time.Time) {
	if data.Flight == nil {
		return
	}
	k := callsignKey{aircraft: aircraftKey(data.Flight), flight: NormaliseFlightNumber(msg.FlightNumber)}
	if k.aircraft == "" || k.flight == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.prune(at)
	e, ok := c.entries[k]
	if data.Callsign != "" {
		if !ok || !at.Before(e.seen) {
			c.entries[k] = callsignEntry{callsign: data.Callsign, seen: at}
		}
		return
	}
	if ok && absDuration(at.Sub(e.seen)) < c.ttl {
		data.Flight.FlightNumber = e.callsign
	}
}

// prune removes the entries not given for the TTL before the newest time
// applied, at most once per TTL. The caller holds c.mu.
func (c *CallsignMemory) prune(at time.Time) {
	if at.After(c.newest) {
		c.newest = at
	}
	if c.newest.Sub(c.pruned) < c.ttl {
		return
	}
	for k, e := range c.entries {
		if c.newest.Sub(e.seen) >= c.ttl {
			delete(c.entries, k)
		}
	}
	c.pruned = c.newest
}

// aircraftKey identifies the aircraft of a flight update: its registration
// (ignoring case and dashes), or without one its address.
func aircraftKey(f *FlightUpdate) string {
	if reg := acars.NormaliseRegistration(f.Registration); reg != "" {
		return reg
	}
	if f.ICAOHex != "" {
		return "hex:" + f.ICAOHex
	}
	return ""
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
