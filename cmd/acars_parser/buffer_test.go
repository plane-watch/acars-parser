package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"acars_parser/internal/storage"
)

// fakeInserter records the batches inserted, and fails while down.
type fakeInserter struct {
	down    bool
	tokens  []string // The token of each insert attempted.
	storedN int      // Rows stored.
}

func (f *fakeInserter) InsertBatchWithToken(_ context.Context, token string, rows []storage.CHInsertParams) error {
	f.tokens = append(f.tokens, token)
	if f.down {
		return errors.New("connection refused")
	}
	f.storedN += len(rows)
	return nil
}

func rowsOf(n int) []storage.CHInsertParams {
	return make([]storage.CHInsertParams, n)
}

// TestMessageBufferRetries checks that a batch whose insert fails is kept
// and sent again, before newer batches, with its first token.
func TestMessageBufferRetries(t *testing.T) {
	db := &fakeInserter{down: true}
	b := newMessageBuffer(db, 1000, time.Second)
	ctx := context.Background()

	for _, r := range rowsOf(3) {
		b.add(r)
	}
	if err := b.flush(ctx); err == nil {
		t.Fatal("flush with ClickHouse down: no error")
	}
	if n := b.retained(); n != 3 {
		t.Fatalf("%d rows retained, want 3", n)
	}
	first := db.tokens[0]

	db.down = false
	for _, r := range rowsOf(2) {
		b.add(r)
	}
	if err := b.flush(ctx); err != nil {
		t.Fatal(err)
	}
	if len(db.tokens) != 3 || db.tokens[1] != first || db.tokens[2] == first {
		t.Errorf("tokens %v, want the first batch again with %s, then a new one", db.tokens, first)
	}
	if stored, dropped := b.counts(); stored != 5 || dropped != 0 || db.storedN != 5 || b.retained() != 0 {
		t.Errorf("stored %d (inserter %d), dropped %d, retained %d; want 5, 5, 0, 0", stored, db.storedN, dropped, b.retained())
	}
}

// TestMessageBufferDropsBeyondLimit checks that the oldest batches are
// dropped, and counted, once more than maxRetainedRows rows are kept.
func TestMessageBufferDropsBeyondLimit(t *testing.T) {
	db := &fakeInserter{down: true}
	b := newMessageBuffer(db, maxRetainedRows, time.Second)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		for _, r := range rowsOf(maxRetainedRows / 2) {
			b.add(r)
		}
		_ = b.flush(ctx)
	}
	if n := b.retained(); n != maxRetainedRows {
		t.Errorf("%d rows retained, want %d", n, maxRetainedRows)
	}
	if _, dropped := b.counts(); dropped != maxRetainedRows/2 {
		t.Errorf("%d rows dropped, want %d", dropped, maxRetainedRows/2)
	}
}

// TestMessageBufferAddDoesNotInsert checks that a full buffer signals the
// flush loop instead of inserting while the message handler waits.
func TestMessageBufferAddDoesNotInsert(t *testing.T) {
	db := &fakeInserter{}
	b := newMessageBuffer(db, 2, time.Hour)
	for _, r := range rowsOf(5) {
		b.add(r)
	}
	if len(db.tokens) != 0 {
		t.Errorf("add inserted %d batches, want none", len(db.tokens))
	}
	select {
	case <-b.flushNow:
	default:
		t.Error("a full buffer did not signal the flush loop")
	}
}
