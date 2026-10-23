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
// Some feeders also send the whole message, reassembled, as one long block
// (longer than an ACARS block can be); that copy is parsed as received, and
// its blocks are not joined again.
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
	done   bool                    // Joined, or already reassembled by the feeder.
}

// Assembler collects blocks and joins complete messages.
type Assembler struct {
	idle      time.Duration
	maxGroups int
	groups    map[key]*list.Element // Values are *group.
	order     *list.List            // Least recently updated first.
}

// New returns an Assembler that forgets a message when no block of it has
// arrived for idle, and keeps at most maxGroups messages, forgetting the
// least recently updated when it would exceed that.
func New(idle time.Duration, maxGroups int) *Assembler {
	return &Assembler{
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

// Add records a block received at now, and returns the joined message when
// the block completes one; otherwise nil. Messages that are not blocks of a
// downlink (no tail, no text, or no block letter) are ignored.
func (a *Assembler) Add(msg *acars.Message, now time.Time) *acars.Message {
	k, letter, ok := blockKey(msg)
	if !ok {
		return nil
	}
	g := a.group(k, now)
	if g.done {
		return nil
	}
	if len(msg.Text) > maxBlockLen {
		g.done = true
		return nil
	}
	if _, dup := g.blocks[letter]; !dup {
		g.blocks[letter] = msg
	}
	blocks, complete := g.complete()
	if !complete {
		return nil
	}
	g.done = true
	return join(k, blocks)
}

// Expire forgets the messages that have had no block since idle before
// now, and returns how many it forgot.
func (a *Assembler) Expire(now time.Time) int {
	n := 0
	for e := a.order.Front(); e != nil; e = a.order.Front() {
		g := e.Value.(*group)
		if now.Sub(g.seen) < a.idle {
			break
		}
		a.remove(e)
		n++
	}
	return n
}

// group returns the message's group, created if need be, marked as updated
// at now.
func (a *Assembler) group(k key, now time.Time) *group {
	if e, ok := a.groups[k]; ok {
		g := e.Value.(*group)
		g.seen = now
		a.order.MoveToBack(e)
		return g
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

// complete returns the blocks in letter order and true if they make a
// complete message (see the package documentation).
func (g *group) complete() ([]*acars.Message, bool) {
	n := len(g.blocks)
	if n < 2 {
		return nil, false
	}
	blocks := make([]*acars.Message, n)
	for i := 0; i < n; i++ {
		b, ok := g.blocks['A'+byte(i)]
		if !ok {
			return nil, false
		}
		blocks[i] = b
	}
	fullLen := len(blocks[0].Text)
	for _, b := range blocks[1 : n-1] {
		if len(b.Text) != fullLen {
			return nil, false
		}
	}
	return blocks, len(blocks[n-1].Text) < fullLen
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

// preambleLen is the length of a block's preamble, "#" and the sub-label
// and a letter ("#DFB"), which some feeders repeat on every block.
const preambleLen = 4

// join returns the message the blocks were split from. It takes its fields
// from the first block, and its flight from the first block that has one.
func join(k key, blocks []*acars.Message) *acars.Message {
	first := blocks[0]
	preamble := ""
	if len(first.Text) >= preambleLen && first.Text[0] == '#' {
		preamble = first.Text[:preambleLen]
	}
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
