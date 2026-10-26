package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

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
	insertTimeout = 15 * time.Second
	// drainRetryEvery is how often drain retries while ClickHouse fails.
	drainRetryEvery = time.Second
)

// batchInserter stores a batch of messages (storage.ClickHouseDB).
type batchInserter interface {
	InsertBatch(ctx context.Context, messages []storage.CHInsertParams) error
}

// messageBuffer batches messages for ClickHouse. Messages arrive over NATS
// without replay, so a batch whose insert fails is kept and retried, in
// order, at the next flush. An insert that reached the server but reported
// a failure (a timeout) is stored again by its retry: the duplicate rows
// have the message's ID and time, and backfill reads them as one message.
// (ClickHouse's insert deduplication tokens would avoid the duplicates,
// but a server crash between recording a token and committing its rows
// would make the retry skip rows that were never stored.)
//
// Batches hold at most batchSize rows, and at most maxRetainedRows rows are
// kept, checked at each flush (so the rows that arrive during one flush can
// exceed it); beyond it the oldest batches are dropped and counted.
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
	messages []storage.CHInsertParams   // Not yet in a batch.
	pending  [][]storage.CHInsertParams // Batches not yet stored, oldest first.
	stored   int                        // Rows stored.
	dropped  int                        // Rows dropped beyond maxRetainedRows.
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

// flush makes batches of the buffered rows and sends the batches not yet
// stored, oldest first, stopping at the first that fails (it is kept for
// the next flush) or once the buffer is stopped (drain sends the rest). It
// returns that failure.
func (b *messageBuffer) flush(ctx context.Context) error {
	return b.send(ctx, true)
}

// drain sends every row not yet stored, retrying failures until ctx ends.
// It is used at shutdown, after stop. It returns the last failure.
func (b *messageBuffer) drain(ctx context.Context) error {
	for {
		err := b.send(ctx, false)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(drainRetryEvery):
		}
	}
}

// send makes batches of the buffered rows and sends the batches not yet
// stored, oldest first, until one fails or, with untilStopped, the buffer
// is stopped.
func (b *messageBuffer) send(ctx context.Context, untilStopped bool) error {
	b.sendMu.Lock()
	defer b.sendMu.Unlock()

	b.mu.Lock()
	for len(b.messages) > 0 {
		n := min(len(b.messages), b.batchSize)
		b.pending = append(b.pending, b.messages[:n:n])
		b.messages = b.messages[n:]
	}
	b.messages = make([]storage.CHInsertParams, 0, b.batchSize)
	b.dropBeyondLimit()
	batches := append([][]storage.CHInsertParams(nil), b.pending...)
	b.mu.Unlock()

	for _, batch := range batches {
		if untilStopped {
			select {
			case <-b.stopCh:
				return nil
			default:
			}
		}
		insertCtx, cancel := context.WithTimeout(ctx, insertTimeout)
		err := b.db.InsertBatch(insertCtx, batch)
		cancel()
		if err != nil {
			return fmt.Errorf("%w (%d rows kept to retry)", err, b.retained())
		}
		b.mu.Lock()
		b.pending = b.pending[1:]
		b.stored += len(batch)
		b.mu.Unlock()
	}
	return nil
}

// dropBeyondLimit drops the oldest batches while more than maxRetainedRows
// rows are kept. The caller holds b.mu.
func (b *messageBuffer) dropBeyondLimit() {
	retained := 0
	for _, batch := range b.pending {
		retained += len(batch)
	}
	for retained > maxRetainedRows && len(b.pending) > 0 {
		n := len(b.pending[0])
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
		n += len(batch)
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

// stop ends the flush loop; a flush in progress stops after its current
// insert. drain sends the rest.
func (b *messageBuffer) stop() {
	close(b.stopCh)
}
