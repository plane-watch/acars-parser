package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"acars_parser/internal/storage"
)

// fakeInserter records the rows stored, in order. It fails while down, and
// while block is set, each insert waits for a value on it.
type fakeInserter struct {
	mu       sync.Mutex
	down     bool
	block    chan struct{}
	attempts int
	ids      []uint64 // The rows stored, in order.
}

func (f *fakeInserter) InsertBatch(ctx context.Context, rows []storage.CHInsertParams) error {
	f.mu.Lock()
	f.attempts++
	block, down := f.block, f.down
	f.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if down {
		return errors.New("connection refused")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range rows {
		f.ids = append(f.ids, r.ID)
	}
	return nil
}

func (f *fakeInserter) set(down bool) {
	f.mu.Lock()
	f.down = down
	f.mu.Unlock()
}

func (f *fakeInserter) stored() []uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]uint64(nil), f.ids...)
}

// addRows adds rows with the IDs from..to-1.
func addRows(b *messageBuffer, from, to int) {
	for id := from; id < to; id++ {
		b.add(storage.CHInsertParams{ID: uint64(id)})
	}
}

// inOrder reports whether ids are from..to-1 in order.
func inOrder(ids []uint64, from, to int) bool {
	if len(ids) != to-from {
		return false
	}
	for i, id := range ids {
		if id != uint64(from+i) {
			return false
		}
	}
	return true
}

// TestMessageBufferRetries checks that a batch whose insert fails is kept
// and sent again before newer rows.
func TestMessageBufferRetries(t *testing.T) {
	db := &fakeInserter{down: true}
	b := newMessageBuffer(db, 1000, time.Second)
	ctx := context.Background()

	addRows(b, 0, 3)
	if err := b.flush(ctx); err == nil {
		t.Fatal("flush with ClickHouse down: no error")
	}
	if n := b.retained(); n != 3 {
		t.Fatalf("%d rows retained, want 3", n)
	}
	db.set(false)
	addRows(b, 3, 5)
	if err := b.flush(ctx); err != nil {
		t.Fatal(err)
	}
	if ids := db.stored(); !inOrder(ids, 0, 5) {
		t.Errorf("stored %v, want 0 to 4 in order", ids)
	}
	if stored, dropped := b.counts(); stored != 5 || dropped != 0 || b.retained() != 0 {
		t.Errorf("stored %d, dropped %d, retained %d; want 5, 0, 0", stored, dropped, b.retained())
	}
}

// TestMessageBufferDropsBeyondLimit checks that at most maxRetainedRows
// rows are kept, the oldest being dropped, even when they were added in
// one burst, larger than a batch.
func TestMessageBufferDropsBeyondLimit(t *testing.T) {
	db := &fakeInserter{down: true}
	b := newMessageBuffer(db, 1000, time.Second)
	ctx := context.Background()

	addRows(b, 0, maxRetainedRows+1500)
	_ = b.flush(ctx)
	if n := b.retained(); n > maxRetainedRows {
		t.Errorf("%d rows retained, want at most %d", n, maxRetainedRows)
	}
	if _, dropped := b.counts(); dropped != 2000 {
		t.Errorf("%d rows dropped, want 2000 (two batches)", dropped)
	}
	db.set(false)
	if err := b.flush(ctx); err != nil {
		t.Fatal(err)
	}
	if ids := db.stored(); !inOrder(ids, 2000, maxRetainedRows+1500) {
		t.Errorf("stored %d rows from %v; want the newest %d in order", len(ids), ids[:1], maxRetainedRows-500)
	}
}

// TestMessageBufferAddDoesNotInsert checks that a full buffer signals the
// flush loop instead of inserting while the message handler waits.
func TestMessageBufferAddDoesNotInsert(t *testing.T) {
	db := &fakeInserter{}
	b := newMessageBuffer(db, 2, time.Hour)
	addRows(b, 0, 5)
	if db.attempts != 0 {
		t.Errorf("add inserted %d batches, want none", db.attempts)
	}
	select {
	case <-b.flushNow:
	default:
		t.Error("a full buffer did not signal the flush loop")
	}
}

// TestMessageBufferDrainRetries checks that drain retries until ClickHouse
// recovers.
func TestMessageBufferDrainRetries(t *testing.T) {
	db := &fakeInserter{down: true}
	b := newMessageBuffer(db, 1000, time.Second)
	addRows(b, 0, 3)
	b.stop()
	go func() {
		time.Sleep(drainRetryEvery / 2)
		db.set(false)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*drainRetryEvery)
	defer cancel()
	if err := b.drain(ctx); err != nil {
		t.Fatal(err)
	}
	if ids := db.stored(); !inOrder(ids, 0, 3) {
		t.Errorf("stored %v, want 0 to 2", ids)
	}
}

// TestMessageBufferShutdown checks the shutdown sequence live uses: the
// flush loop, busy sending retained batches, stops after its current
// insert, and drain sends the rest, with the rows added meanwhile, in order.
func TestMessageBufferShutdown(t *testing.T) {
	db := &fakeInserter{block: make(chan struct{})}
	b := newMessageBuffer(db, 2, time.Millisecond)
	addRows(b, 0, 6) // Three batches.

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.flushLoop(context.Background())
	}()
	db.block <- struct{}{} // The first batch's insert completes.
	addRows(b, 6, 7)
	b.stop()
	db.block <- struct{}{} // The insert in progress completes.
	wg.Wait()

	db.mu.Lock()
	db.block = nil
	db.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := b.drain(ctx); err != nil {
		t.Fatal(err)
	}
	if ids := db.stored(); !inOrder(ids, 0, 7) {
		t.Errorf("stored %v, want 0 to 6 in order", ids)
	}
}
