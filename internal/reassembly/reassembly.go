// Package reassembly joins the blocks of multi-block ACARS downlinks into
// the message they were split from.
//
// A downlink's message number ends with a block letter: the blocks of one
// message share the first three characters ("D47") and are lettered A, B,
// C, ... in order. Blocks are grouped by the receiving station, the tail,
// the label and those three characters. The feed's block_end flag does not
// reliably mark the last block (see acars.Message.BlockEnd), so a message is
// taken to be complete when it has a block for every letter from A, every
// block but the last has the same length (a full block), and the last block
// is shorter. A message whose last block happens to be full is therefore
// never joined: a truncated message would be worse than none.
//
// Completeness is judged only once no block of the message has arrived for
// a settling time, so that a block arriving later can still show that the
// message was not complete (a block after the short one), or was received
// twice differently (two blocks with one letter and different text), in
// which case it is not joined. A message whose blocks so far are consistent
// but which may have more (its last block is full, or a letter is missing)
// is judged again at each flush until it is forgotten. Some feeders also send the whole message,
// reassembled, as one long block (longer than an ACARS block can be); that
// copy is parsed as received, and the blocks are not joined.
//
// An Assembler is not safe for concurrent use.
package reassembly

import (
	"container/list"
	"strings"
	"time"

	"acars_parser/internal/acars"
)

// AssembledIDFlag is set in the ID of a joined message, which is otherwise
// the ID of its first block. Airframes' message IDs are far below it (about
// 8e9 in October 2026), so a joined message's ID is never a received one.
const AssembledIDFlag int64 = 1 << 62

// maxBlockLen is the most text an ACARS block carries; a longer "block" is
// a message that a feeder has already reassembled.
const maxBlockLen = 220

// IsAssembled reports whether msg was joined from blocks.
func IsAssembled(msg *acars.Message) bool {
	return int64(msg.ID)&AssembledIDFlag != 0
}

// key identifies the blocks of one message from one station.
type key struct {
	station, tail, label, number string
}

// group holds the blocks received of one message.
type group struct {
	key    key
	blocks map[byte]*acars.Message // By block letter; the first received is kept.
	seen   time.Time               // When the last block arrived.

	// done is set when the message is joined or will not be: it was
	// reassembled by the feeder, received twice differently, or judged
	// incomplete when it settled. Later blocks are ignored.
	done bool
}

// Assembler collects blocks and joins complete messages.
type Assembler struct {
	settle    time.Duration
	idle      time.Duration
	maxGroups int
	groups    map[key]*list.Element // Values are *group.
	order     *list.List            // Least recently updated first.
}

// New returns an Assembler that judges a message once no block of it has
// arrived for settle, forgets it once none has arrived for idle (which
// must be longer), and keeps at most maxGroups messages, forgetting the
// least recently updated when it would exceed that.
func New(settle, idle time.Duration, maxGroups int) *Assembler {
	return &Assembler{
		settle:    settle,
		idle:      idle,
		maxGroups: maxGroups,
		groups:    make(map[key]*list.Element),
		order:     list.New(),
	}
}

// Pending returns the number of messages whose blocks are held.
func (a *Assembler) Pending() int {
	return len(a.groups)
}

// Add records a block received at now. Messages that are not blocks of a
// downlink (no tail, no text, or no block letter) are ignored.
func (a *Assembler) Add(msg *acars.Message, now time.Time) {
	k, letter, ok := blockKey(msg)
	if !ok {
		return
	}
	g := a.group(k, now)
	if g.done {
		return
	}
	if len(msg.Text) > maxBlockLen {
		g.done = true
		return
	}
	if prev, dup := g.blocks[letter]; dup {
		if prev.Text != msg.Text {
			g.done = true
		}
		return
	}
	g.blocks[letter] = msg
}

// Flush judges the messages that have had no block for the settling time
// and returns those that are complete, joined, in the order their last
// blocks arrived; each message is returned once. It also forgets the
// messages that have had no block for the idle time.
func (a *Assembler) Flush(now time.Time) []*acars.Message {
	var joined []*acars.Message
	for e := a.order.Front(); e != nil; {
		g := e.Value.(*group)
		quiet := now.Sub(g.seen)
		if quiet < a.settle {
			break
		}
		next := e.Next()
		if !g.done {
			switch blocks, state := g.judge(); state {
			case complete:
				g.done = true
				joined = append(joined, join(g.key, blocks))
			case inconsistent:
				g.done = true
			}
			// A message still waiting for blocks is judged again at the next
			// flush, until it is forgotten.
		}
		if quiet >= a.idle {
			a.remove(e)
		}
		e = next
	}
	return joined
}

// group returns the message's group, created if need be, marked as updated
// at now. A group with no block for the idle time is replaced, so that a
// block arriving after it is not joined to the old ones.
func (a *Assembler) group(k key, now time.Time) *group {
	if e, ok := a.groups[k]; ok {
		g := e.Value.(*group)
		if now.Sub(g.seen) < a.idle {
			g.seen = now
			a.order.MoveToBack(e)
			return g
		}
		a.remove(e)
	}
	for len(a.groups) >= a.maxGroups && a.order.Len() > 0 {
		a.remove(a.order.Front())
	}
	g := &group{key: k, blocks: make(map[byte]*acars.Message), seen: now}
	a.groups[k] = a.order.PushBack(g)
	return g
}

func (a *Assembler) remove(e *list.Element) {
	delete(a.groups, e.Value.(*group).key)
	a.order.Remove(e)
}

// state is the judgement of a message's blocks.
type state int

const (
	// waiting: the blocks so far are consistent, but the message may have
	// more (the last is full, or a letter is missing).
	waiting state = iota
	// complete: the message is whole (see the package documentation).
	complete
	// inconsistent: the blocks cannot be one message (a block other than
	// the last is not full, or one is longer than block A), or the message
	// has a single block and needs no joining.
	inconsistent
)

// judge returns the state of the message's blocks and, when it is
// complete, the blocks in letter order.
func (g *group) judge() ([]*acars.Message, state) {
	first, ok := g.blocks['A']
	if !ok {
		return nil, waiting
	}
	fullLen := len(first.Text)
	var last byte = 'A'
	for letter := range g.blocks {
		last = max(last, letter)
	}
	for letter, b := range g.blocks {
		if letter != last && len(b.Text) != fullLen || len(b.Text) > fullLen {
			return nil, inconsistent
		}
	}
	if len(g.blocks[last].Text) == fullLen {
		return nil, waiting
	}
	if last == 'A' {
		return nil, inconsistent
	}
	blocks := make([]*acars.Message, 0, last-'A'+1)
	for letter := byte('A'); letter <= last; letter++ {
		b, ok := g.blocks[letter]
		if !ok {
			return nil, waiting
		}
		blocks = append(blocks, b)
	}
	return blocks, complete
}

// blockKey returns the group key and block letter of a downlink block.
func blockKey(msg *acars.Message) (key, byte, bool) {
	number := msg.MessageNumber
	if len(number) != 4 || msg.Tail == "" || msg.Text == "" {
		return key{}, 0, false
	}
	letter := number[3]
	if letter < 'A' || letter > 'Z' {
		return key{}, 0, false
	}
	station := ""
	if msg.Station != nil {
		station = msg.Station.ID
	}
	return key{station: station, tail: msg.Tail, label: msg.Label, number: number[:3]}, letter, true
}

// join returns the message the blocks were split from. It takes its fields
// from the first block, and its flight from the first block that has one.
// Some feeders repeat an H1 message's preamble ("#DFB", "- #DFB"; see
// acars.Message.H1Preamble) on every block; a later block's copy of the
// first block's preamble is dropped.
func join(k key, blocks []*acars.Message) *acars.Message {
	first := blocks[0]
	preamble := first.H1Preamble()
	var text strings.Builder
	ids := make([]int64, len(blocks))
	flight := ""
	for i, b := range blocks {
		t := b.Text
		if i > 0 && preamble != "" {
			t = strings.TrimPrefix(t, preamble)
		}
		text.WriteString(t)
		ids[i] = int64(b.ID)
		if flight == "" {
			flight = b.FlightNumber
		}
	}
	msg := *first
	msg.ID = acars.FlexInt64(int64(first.ID) | AssembledIDFlag)
	msg.Text = text.String()
	msg.FlightNumber = flight
	msg.MessageNumber = k.number
	msg.BlockEnd = nil
	msg.AssembledFrom = ids
	return &msg
}
