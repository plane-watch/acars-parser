package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"acars_parser/internal/storage"
)

// Limits of the message buffer.
const (
	// maxRetainedRows bounds the rows kept for a retry while ClickHouse is
	// unavailable: about an hour of traffic (October 2026: about 1,600
	// messages, some 2,000 rows, a minute), some 200 MB. Beyond it the
	// oldest batches are dropped.
	maxRetainedRows = 100000
	// insertTimeout bounds one batch's insert.
	insertTimeout = 30 * time.Second
)

// batchInserter stores a batch of messages with a deduplication token
// (storage.ClickHouseDB.InsertBatchWithToken).
type batchInserter interface {
	InsertBatchWithToken(ctx context.Context, token string, messages []storage.CHInsertParams) error
}

// messageBuffer batches messages for ClickHouse. Messages arrive over NATS
// without replay, so a batch whose insert fails is kept and retried, in
// order, at the next flush. Each batch keeps the deduplication token it was
// first sent with, so that a retry of an insert that did reach the server
// is not stored twice. At most maxRetainedRows rows are kept; beyond it the
// oldest batches are dropped and counted.
//
// add never inserts: a full buffer signals the flush loop, so that the
// message handler is not held up by ClickHouse.
type messageBuffer struct {
	db        batchInserter
	batchSize int
	interval  time.Duration
	stopCh    chan struct{}
	flushNow  chan struct{} // Signalled by add when the buffer is full.

	sendMu sync.Mutex // Serialises flushes, so that batches are sent in order.

	mu       sync.Mutex
	messages []storage.CHInsertParams // Not yet in a batch.
	pending  []pendingBatch           // Batches not yet stored, oldest first.
	stored   int                      // Rows stored.
	dropped  int                      // Rows dropped beyond maxRetainedRows.
}

// pendingBatch is a batch of rows with the deduplication token it is sent
// with.
type pendingBatch struct {
	token string
	rows  []storage.CHInsertParams
}

func newMessageBuffer(db batchInserter, batchSize int, interval time.Duration) *messageBuffer {
	return &messageBuffer{
		db:        db,
		messages:  make([]storage.CHInsertParams, 0, batchSize),
		batchSize: batchSize,
		interval:  interval,
		stopCh:    make(chan struct{}),
		flushNow:  make(chan struct{}, 1),
	}
}

// add buffers a row, and signals the flush loop when the buffer is full.
func (b *messageBuffer) add(msg storage.CHInsertParams) {
	b.mu.Lock()
	b.messages = append(b.messages, msg)
	full := len(b.messages) >= b.batchSize
	b.mu.Unlock()

	if full {
		select {
		case b.flushNow <- struct{}{}:
		default: // A flush is already signalled.
		}
	}
}

// flush makes a batch of the buffered rows and sends the batches not yet
// stored, oldest first, stopping at the first that fails; it is kept for
// the next flush. It returns that failure.
func (b *messageBuffer) flush(ctx context.Context) error {
	b.sendMu.Lock()
	defer b.sendMu.Unlock()

	b.mu.Lock()
	if len(b.messages) > 0 {
		b.pending = append(b.pending, pendingBatch{token: uuid.NewString(), rows: b.messages})
		b.messages = make([]storage.CHInsertParams, 0, b.batchSize)
	}
	b.dropBeyondLimit()
	batches := append([]pendingBatch(nil), b.pending...)
	b.mu.Unlock()

	for _, batch := range batches {
		insertCtx, cancel := context.WithTimeout(ctx, insertTimeout)
		err := b.db.InsertBatchWithToken(insertCtx, batch.token, batch.rows)
		cancel()
		if err != nil {
			return fmt.Errorf("%w (%d rows kept to retry)", err, b.retained())
		}
		b.mu.Lock()
		b.pending = b.pending[1:]
		b.stored += len(batch.rows)
		b.mu.Unlock()
	}
	return nil
}

// dropBeyondLimit drops the oldest batches while more than maxRetainedRows
// rows are kept. The caller holds b.mu.
func (b *messageBuffer) dropBeyondLimit() {
	retained := 0
	for _, batch := range b.pending {
		retained += len(batch.rows)
	}
	for retained > maxRetainedRows && len(b.pending) > 1 {
		n := len(b.pending[0].rows)
		b.pending = b.pending[1:]
		retained -= n
		b.dropped += n
		fmt.Fprintf(os.Stderr, "Dropped %d rows not stored: more than %d kept to retry\n", n, maxRetainedRows)
	}
}

// retained returns the number of rows not yet stored.
func (b *messageBuffer) retained() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(b.messages)
	for _, batch := range b.pending {
		n += len(batch.rows)
	}
	return n
}

// counts returns the rows stored and dropped.
func (b *messageBuffer) counts() (stored, dropped int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stored, b.dropped
}

// flushLoop flushes at each interval and when add signals a full buffer,
// until stop.
func (b *messageBuffer) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-b.flushNow:
		case <-b.stopCh:
			return
		}
		if err := b.flush(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error flushing messages: %v\n", err)
		}
	}
}

func (b *messageBuffer) stop() {
	close(b.stopCh)
}
