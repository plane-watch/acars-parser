package reassembly

import (
	"strings"
	"testing"
	"time"

	"acars_parser/internal/acars"
)

var t0 = time.Date(2026, 10, 23, 15, 5, 0, 0, time.UTC)

// full is the text of a full block: in October 2026, H1 blocks carried 206
// or 210 characters.
var full = strings.Repeat("X", 206)

func block(id int64, station, msn, text string) *acars.Message {
	return &acars.Message{
		ID:            acars.FlexInt64(id),
		Timestamp:     "2026-10-23T15:05:00Z",
		Label:         "H1",
		Tail:          "N8953Q",
		FlightNumber:  "WN1234",
		BlockID:       "5",
		MessageNumber: msn,
		Text:          text,
		Station:       &acars.Station{ID: station},
	}
}

func TestJoinsCompleteMessage(t *testing.T) {
	a := New(time.Minute, 100)
	if got := a.Add(block(10, "s1", "D47A", full), t0); got != nil {
		t.Fatalf("after block A: got %+v, want nothing yet", got)
	}
	got := a.Add(block(11, "s1", "D47B", "END"), t0.Add(6*time.Second))
	if got == nil {
		t.Fatal("after the short last block: got nothing, want the joined message")
	}
	if got.Text != full+"END" {
		t.Errorf("Text = %q, want block A then block B", got.Text)
	}
	if int64(got.ID) != 10|AssembledIDFlag || !IsAssembled(got) {
		t.Errorf("ID = %d, want block A's ID with AssembledIDFlag", got.ID)
	}
	if got.MessageNumber != "D47" || len(got.AssembledFrom) != 2 || got.AssembledFrom[0] != 10 || got.AssembledFrom[1] != 11 {
		t.Errorf("MessageNumber, AssembledFrom = %q, %v; want D47, [10 11]", got.MessageNumber, got.AssembledFrom)
	}
	if got.Tail != "N8953Q" || got.Label != "H1" || got.FlightNumber != "WN1234" || got.Timestamp != "2026-10-23T15:05:00Z" {
		t.Errorf("got %+v, want block A's tail, label, flight and time", got)
	}
}

func TestJoinsBlocksOutOfOrder(t *testing.T) {
	a := New(time.Minute, 100)
	a.Add(block(11, "s1", "D47B", "END"), t0)
	if got := a.Add(block(10, "s1", "D47A", full), t0); got == nil || got.Text != full+"END" {
		t.Errorf("got %+v, want the blocks joined in letter order", got)
	}
}

// A last block of full length may be followed by another, so the message
// is not joined until a shorter block ends it.
func TestWaitsForShortLastBlock(t *testing.T) {
	a := New(time.Minute, 100)
	a.Add(block(10, "s1", "D47A", full), t0)
	if got := a.Add(block(11, "s1", "D47B", full), t0); got != nil {
		t.Fatalf("got %+v after two full blocks, want nothing", got)
	}
	if got := a.Add(block(12, "s1", "D47C", "END"), t0); got == nil || got.Text != full+full+"END" {
		t.Errorf("got %+v, want three blocks joined", got)
	}
}

func TestDoesNotJoinAroundMissingBlock(t *testing.T) {
	a := New(time.Minute, 100)
	a.Add(block(10, "s1", "D47A", full), t0)
	if got := a.Add(block(12, "s1", "D47C", "END"), t0); got != nil {
		t.Errorf("got %+v with block B missing, want nothing", got)
	}
}

// Blocks whose lengths differ before the last are not all full, so the
// sequence is not trusted.
func TestDoesNotJoinUnequalFullBlocks(t *testing.T) {
	a := New(time.Minute, 100)
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(11, "s1", "D47B", full[:150]), t0)
	if got := a.Add(block(12, "s1", "D47C", "END"), t0); got != nil {
		t.Errorf("got %+v, want nothing", got)
	}
}

// Some feeders repeat the first block's preamble ("#DFB") on later blocks.
func TestDropsRepeatedPreamble(t *testing.T) {
	a := New(time.Minute, 100)
	first := "#DFB" + full[4:]
	a.Add(block(10, "s1", "D47A", first), t0)
	got := a.Add(block(11, "s1", "D47B", "#DFB2380-0033"), t0)
	if got == nil || got.Text != first+"2380-0033" {
		t.Errorf("got %+v, want the later block's preamble dropped", got)
	}
}

// Some feeders also send the whole message, reassembled, on its last
// block; that copy is parsed as received, so nothing is joined.
func TestSkipsMessageAlreadyReassembled(t *testing.T) {
	a := New(time.Minute, 100)
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(11, "s1", "D47B", strings.Repeat("Y", 400)), t0)
	if got := a.Add(block(12, "s1", "D47B", "END"), t0); got != nil {
		t.Errorf("got %+v, want nothing", got)
	}
}

func TestJoinsOnlyOnce(t *testing.T) {
	a := New(time.Minute, 100)
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(11, "s1", "D47B", "END"), t0)
	if got := a.Add(block(13, "s1", "D47B", "END"), t0); got != nil {
		t.Errorf("a repeated block gave %+v, want nothing", got)
	}
}

// Blocks received by different stations are different receptions.
func TestKeepsStationsApart(t *testing.T) {
	a := New(time.Minute, 100)
	a.Add(block(10, "s1", "D47A", full), t0)
	if got := a.Add(block(11, "s2", "D47B", "END"), t0); got != nil {
		t.Errorf("got %+v from blocks of two stations, want nothing", got)
	}
}

func TestExpiresIdleMessages(t *testing.T) {
	a := New(time.Minute, 100)
	a.Add(block(10, "s1", "D47A", full), t0)
	if n := a.Expire(t0.Add(2 * time.Minute)); n != 1 || a.Pending() != 0 {
		t.Fatalf("Expire dropped %d, %d pending; want 1, 0", n, a.Pending())
	}
	if got := a.Add(block(11, "s1", "D47B", "END"), t0.Add(2*time.Minute)); got != nil {
		t.Errorf("got %+v after block A expired, want nothing", got)
	}
}

func TestBoundsPendingMessages(t *testing.T) {
	a := New(time.Minute, 2)
	a.Add(block(10, "s1", "D01A", full), t0)
	a.Add(block(20, "s1", "D02A", full), t0.Add(time.Second))
	a.Add(block(30, "s1", "D03A", full), t0.Add(2*time.Second))
	if a.Pending() != 2 {
		t.Fatalf("Pending = %d, want 2", a.Pending())
	}
	// The oldest message (D01) was dropped.
	if got := a.Add(block(11, "s1", "D01B", "END"), t0); got != nil {
		t.Errorf("got %+v for the dropped message, want nothing", got)
	}
}

func TestIgnoresOtherMessages(t *testing.T) {
	a := New(time.Minute, 100)
	for name, m := range map[string]*acars.Message{
		"no message number":  block(10, "s1", "", full),
		"no block letter":    block(10, "s1", "D97", full),
		"digit block letter": block(10, "s1", "D971", full),
		"no tail":            func() *acars.Message { m := block(10, "s1", "D47A", full); m.Tail = ""; return m }(),
		"no text":            block(10, "s1", "D47A", ""),
	} {
		a.Add(m, t0)
		if a.Pending() != 0 {
			t.Errorf("%s: %d pending, want 0", name, a.Pending())
		}
	}
}

// A message of one block needs no joining.
func TestSingleBlockIsNotJoined(t *testing.T) {
	a := New(time.Minute, 100)
	if got := a.Add(block(10, "s1", "D47A", "SHORT"), t0); got != nil {
		t.Errorf("got %+v, want nothing", got)
	}
}
