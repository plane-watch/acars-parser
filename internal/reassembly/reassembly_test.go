package reassembly

import (
	"strings"
	"testing"
	"time"

	"acars_parser/internal/acars"
)

var t0 = time.Date(2026, 10, 23, 15, 5, 0, 0, time.UTC)

const (
	settle = 30 * time.Second
	idle   = 2 * time.Minute
)

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

func newAssembler() *Assembler {
	return New(settle, idle, 100)
}

// flushOne flushes at at and returns the one message joined, failing the
// test if there is not exactly one.
func flushOne(t *testing.T, a *Assembler, at time.Time) *acars.Message {
	t.Helper()
	got := a.Flush(at)
	if len(got) != 1 {
		t.Fatalf("Flush joined %d messages, want 1", len(got))
	}
	return got[0]
}

func expectNone(t *testing.T, a *Assembler, at time.Time) {
	t.Helper()
	if got := a.Flush(at); len(got) != 0 {
		t.Errorf("Flush joined %d messages (%q), want none", len(got), got[0].Text)
	}
}

func TestJoinsCompleteMessageOnceSettled(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(11, "s1", "D47B", "END"), t0.Add(6*time.Second))
	// Not before the message has been quiet for the settling time.
	expectNone(t, a, t0.Add(20*time.Second))
	got := flushOne(t, a, t0.Add(6*time.Second+settle))
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
	// A message is joined once.
	expectNone(t, a, t0.Add(time.Minute))
}

func TestJoinsBlocksOutOfOrder(t *testing.T) {
	a := newAssembler()
	a.Add(block(11, "s1", "D47B", "END"), t0)
	a.Add(block(10, "s1", "D47A", full), t0)
	if got := flushOne(t, a, t0.Add(settle)); got.Text != full+"END" {
		t.Errorf("Text = %q, want the blocks in letter order", got.Text)
	}
}

// A last block of full length may be followed by another, so the message
// is not joined until a shorter block ends it.
func TestWaitsForShortLastBlock(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(11, "s1", "D47B", full), t0)
	expectNone(t, a, t0.Add(settle))
	a.Add(block(12, "s1", "D47C", "END"), t0.Add(settle))
	if got := flushOne(t, a, t0.Add(2*settle)); got.Text != full+full+"END" {
		t.Errorf("Text = %q, want three blocks joined", got.Text)
	}
}

func TestDoesNotJoinAroundMissingBlock(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(12, "s1", "D47C", "END"), t0)
	expectNone(t, a, t0.Add(settle))
}

// A block that arrives after a shorter one shows the shorter one was not
// the last: the blocks are inconsistent and nothing is joined. Before the
// settling time, the shorter block looked like the end of the message.
func TestDoesNotJoinWhenALaterBlockFollowsTheShortOne(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(11, "s1", "D47B", full[:150]), t0)
	a.Add(block(12, "s1", "D47C", "END"), t0.Add(10*time.Second))
	expectNone(t, a, t0.Add(10*time.Second+settle))
}

// Two receptions of one block with different text cannot both be right,
// so the message is not joined.
func TestDoesNotJoinConflictingReceptions(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(20, "s1", "D47A", strings.Repeat("Y", 206)), t0)
	a.Add(block(21, "s1", "D47B", "END"), t0)
	expectNone(t, a, t0.Add(settle))
}

// The same block received twice is one block.
func TestToleratesRepeatedIdenticalBlock(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(20, "s1", "D47A", full), t0)
	a.Add(block(21, "s1", "D47B", "END"), t0)
	got := flushOne(t, a, t0.Add(settle))
	if got.AssembledFrom[0] != 10 {
		t.Errorf("AssembledFrom = %v, want the first reception of block A", got.AssembledFrom)
	}
}

// Some feeders repeat the first block's H1 preamble on later blocks.
func TestDropsRepeatedH1Preamble(t *testing.T) {
	for _, preamble := range []string{"#DFB", "- #DFB"} {
		a := newAssembler()
		first := preamble + full[len(preamble):]
		a.Add(block(10, "s1", "D47A", first), t0)
		a.Add(block(11, "s1", "D47B", preamble+"2380-0033"), t0)
		if got := flushOne(t, a, t0.Add(settle)); got.Text != first+"2380-0033" {
			t.Errorf("preamble %q: Text = %q, want the later block's preamble dropped", preamble, got.Text)
		}
	}
}

// Text that only looks like a preamble, on another label, is kept.
func TestKeepsTextOfOtherLabels(t *testing.T) {
	a := newAssembler()
	first := block(10, "s1", "D47A", "#ABC"+full[4:])
	second := block(11, "s1", "D47B", "#ABCEND")
	first.Label, second.Label = "RA", "RA"
	a.Add(first, t0)
	a.Add(second, t0)
	if got := flushOne(t, a, t0.Add(settle)); got.Text != first.Text+"#ABCEND" {
		t.Errorf("Text = %q, want both blocks whole", got.Text)
	}
}

// Some feeders also send the whole message, reassembled, as one long
// block; that copy is parsed as received, so nothing is joined.
func TestSkipsMessageAlreadyReassembled(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(11, "s1", "D47B", "END"), t0)
	a.Add(block(12, "s1", "D47B", full+"END"), t0.Add(5*time.Second))
	expectNone(t, a, t0.Add(time.Minute))
}

// Blocks received by different stations are different receptions.
func TestKeepsStationsApart(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(11, "s2", "D47B", "END"), t0)
	expectNone(t, a, t0.Add(settle))
}

func TestForgetsIdleMessages(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Flush(t0.Add(idle))
	if a.Pending() != 0 {
		t.Fatalf("%d pending after the idle time, want 0", a.Pending())
	}
}

// A block that arrives after its message's idle time starts a new message,
// even before a flush has forgotten the old one.
func TestStaleBlocksAreNotJoined(t *testing.T) {
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", full), t0)
	a.Add(block(11, "s1", "D47B", "END"), t0.Add(10*time.Minute))
	expectNone(t, a, t0.Add(10*time.Minute+settle))
}

func TestBoundsPendingMessages(t *testing.T) {
	a := New(settle, idle, 2)
	a.Add(block(10, "s1", "D01A", full), t0)
	a.Add(block(20, "s1", "D02A", full), t0.Add(time.Second))
	a.Add(block(30, "s1", "D03A", full), t0.Add(2*time.Second))
	if a.Pending() != 2 {
		t.Fatalf("Pending = %d, want 2", a.Pending())
	}
	// The least recently updated message (D01) was forgotten.
	a.Add(block(11, "s1", "D01B", "END"), t0.Add(3*time.Second))
	expectNone(t, a, t0.Add(3*time.Second+settle))
}

func TestIgnoresOtherMessages(t *testing.T) {
	a := newAssembler()
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
	a := newAssembler()
	a.Add(block(10, "s1", "D47A", "SHORT"), t0)
	expectNone(t, a, t0.Add(settle))
}
