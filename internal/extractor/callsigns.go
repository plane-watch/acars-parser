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

// callsignGrace is how long after a message a callsign may have been given
// and still apply to it: a joined message is processed after its blocks
// settle (see reassembly), carrying its first block's time.
const callsignGrace = 5 * time.Minute

// CallsignMemory remembers, per aircraft and transmitted flight, the
// callsign that the aircraft's messages gave for it (ExtractedData.Callsign),
// so that its messages that give none (position reports, for instance) are
// keyed by the same flight. A Qatar 777 transmitting "VA0869" while flying
// QTR869 sends most of its reports without the callsign; without the
// memory, their routes would be stored under VA869, which Virgin Australia
// also flies.
//
// Only a callsign given before a message (or within callsignGrace after
// it) is applied to it, so messages are expected in time order per
// aircraft: live receives them so, and backfill reads them so. A joined
// message carries its first block's time and is processed after its last
// block arrives, hence the grace. A message older than the callsign
// remembered does not replace it.
//
// The aircraft is identified as transmitted (the tail, or without one the
// link-layer address), not by a registration a result names, so that
// backfill can send each aircraft's messages to one worker before
// extracting them.
//
// A CallsignMemory is safe for concurrent use.
type CallsignMemory struct {
	ttl time.Duration

	mu      sync.Mutex
	entries map[callsignKey]callsignEntry
	newest  time.Time // The newest time applied.
	pruned  time.Time // When expired entries were last removed.
}

// callsignKey is an aircraft (its transmitted tail, or without one its
// link-layer address) and a transmitted flight, without leading zeros.
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
// gives for the transmitted flight; or, when data's flight is not keyed by
// a callsign, keys it by the callsign remembered for the aircraft's
// transmitted flight.
func (c *CallsignMemory) Apply(msg *acars.Message, data *ExtractedData, at time.Time) {
	if data.Flight == nil {
		return
	}
	k := callsignKey{aircraft: transmittedAircraft(msg), flight: NormaliseFlightNumber(msg.FlightNumber)}
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
	if ok && !data.keyedByCallsign && !at.Add(callsignGrace).Before(e.seen) && at.Sub(e.seen) < c.ttl {
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

// transmittedAircraft identifies the aircraft that sent a message: its
// tail (ignoring case and dashes), or without one its link-layer address.
func transmittedAircraft(msg *acars.Message) string {
	if reg := acars.NormaliseRegistration(msg.Tail); reg != "" {
		return reg
	}
	if addr, ok := msg.AircraftAddress(); ok {
		return "hex:" + addr
	}
	return ""
}
