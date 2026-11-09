package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
)

const (
	// rebuildTable receives the rebuilt archive before it is swapped in.
	rebuildTable = "messages_rebuild"
	// previousTable keeps the archive as it was before the last rebuild,
	// until it is dropped.
	previousTable = "messages_previous"
)

// rebuildChunk is the number of message IDs each read query covers.
const rebuildChunk = 200000

// insertPollEvery is how often a rebuild checks whether the inserts it
// waits for have ended.
const insertPollEvery = time.Second

// rebuildHooks are called at points of a rebuild, so that tests can store
// messages there as live would; they are nil otherwise.
var rebuildHooks struct {
	afterRead     func() // After the archive is read and the result checked.
	afterExchange func() // After the rebuilt archive is swapped in.
}

// rebuildStats counts what a rebuild wrote.
type rebuildStats struct {
	Messages int    // Distinct messages read.
	Rows     int    // Rows written (one per parse result, or one unparsed row).
	Copied   uint64 // Messages stored during the rebuild, copied as stored.
}

// rebuildArchive reparses every message in the archive with the current
// parsers and replaces the archive with the result, without the duplicate
// rows that "reparse -update" leaves.
//
// Each distinct message is read once (its rows hold the same transmitted
// content), in ranges of IDs, dispatched to the registry and written as
// live writes it, into messages_rebuild; a message no parser matches, even
// one without text, keeps an unparsed row, so that every message ID
// survives. When the rebuilt table holds exactly the archive's distinct
// IDs, an atomic EXCHANGE TABLES makes it the archive, and the old one is
// renamed to messages_previous; nothing is swapped otherwise.
//
// A rebuild refuses to start while messages_rebuild or messages_previous
// exists: either may hold the only copy of an archive (a failed rebuild
// leaves its staging table for inspection), so it is never dropped
// automatically.
//
// The stored flight of messages before dropFlightBefore is blanked: older
// code stored Airframes' flight record there, which is not transmitted data
// (zero keeps every flight).
//
// Live keeps storing messages meanwhile. The rebuild reads only the rows
// stored (created_at, the server's clock) before it started, once the
// server has finished every insert started before then (so that all of
// those rows are committed). The messages stored since are copied as
// stored: those in the archive before the swap, then, once the server has
// finished the inserts started before the exchange, those such an insert
// stored in the old archive. Live parsed them, so live must run the same
// parsers as the rebuild (deploy it first). A message is copied by its ID,
// whole and once: a retried insert's duplicates of a rebuilt message are
// not. A retry that live completes while the last copy runs can still
// store a message twice, as a retry after a lost acknowledgement can at any
// time (see messageBuffer); backfill reads the duplicate rows as one
// message.
func rebuildArchive(ctx context.Context, db *storage.ClickHouseDB, reg *registry.Registry, dropFlightBefore time.Time, batchSize int) (rebuildStats, error) {
	var stats rebuildStats
	if batchSize <= 0 {
		return stats, fmt.Errorf("batch size %d: must be positive", batchSize)
	}
	for _, name := range []string{rebuildTable, previousTable} {
		if exists, err := db.TableExists(ctx, name); err != nil {
			return stats, err
		} else if exists {
			return stats, fmt.Errorf("%s exists: it may hold an archive (a failed rebuild's staging table, or the archive before the last rebuild); check it and drop it before rebuilding", name)
		}
	}
	started, err := db.ServerTime(ctx)
	if err != nil {
		return stats, err
	}
	fmt.Printf("  reading the messages stored before %s\n", formatStored(started))
	if err := waitForInserts(ctx, db, started); err != nil {
		return stats, err
	}
	source, err := db.CountDistinctIDs(ctx, storage.MessagesTable, started)
	if err != nil {
		return stats, err
	}
	if err := db.CreateMessagesTable(ctx, rebuildTable); err != nil {
		return stats, err
	}

	batch := make([]storage.CHInsertParams, 0, batchSize)
	flush := func() error {
		if err := db.InsertBatchInto(ctx, rebuildTable, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}
	err = db.StreamRawMessages(ctx, storage.MessagesTable, rebuildChunk, started, func(m storage.RawMessage) error {
		stats.Messages++
		flight := m.Flight
		if !dropFlightBefore.IsZero() && m.Timestamp.Before(dropFlightBefore) {
			flight = ""
		}
		msg := &acars.Message{
			ID:           acars.FlexInt64(m.ID),
			Timestamp:    m.Timestamp.UTC().Format(time.RFC3339Nano),
			Label:        m.Label,
			Tail:         m.Tail,
			FlightNumber: flight,
			Text:         m.RawText,
		}
		setBlockFields(msg, m.BlockFields, m.StoredDirection)
		rows := messageRows(msg, registry.Results(reg.Dispatch(msg)))
		if len(rows) == 0 {
			rows = append(rows, unparsedRow(msg))
		}
		stats.Rows += len(rows)
		batch = append(batch, rows...)
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
		if stats.Messages%500000 == 0 {
			fmt.Printf("  %d of %d messages reparsed\n", stats.Messages, source)
		}
		return nil
	})
	if err == nil {
		err = flush()
	}
	if err != nil {
		return stats, fmt.Errorf("rebuild: %w (the archive is unchanged; %s holds the partial rebuild: drop it before retrying)", err, rebuildTable)
	}

	written, err := db.CountDistinctIDs(ctx, rebuildTable, time.Time{})
	if err != nil {
		return stats, err
	}
	if written != source || uint64(stats.Messages) != source {
		return stats, fmt.Errorf("the archive had %d messages, %d were read and %s holds %d; not swapped", source, stats.Messages, rebuildTable, written)
	}
	if rebuildHooks.afterRead != nil {
		rebuildHooks.afterRead()
	}

	// The messages stored since the start, up to now.
	// The error after the exchange says where messages stored meanwhile may
	// be; from here, every error does.
	lost := fmt.Sprintf("messages stored since %s (during the rebuild) may be only in the old archive: keep it until they are in %s", formatStored(started), storage.MessagesTable)
	if stats.Copied, err = copyStoredSince(ctx, db, storage.MessagesTable, rebuildTable, started); err != nil {
		return stats, fmt.Errorf("copy the messages stored during the rebuild: %w (the archive is unchanged; %s holds the partial rebuild: drop it before retrying)", err, rebuildTable)
	}
	// The UUIDs identify the tables whatever their names; row counts cannot,
	// since both hold the same messages.
	oldUUID, err := db.TableUUID(ctx, storage.MessagesTable)
	if err != nil {
		return stats, err
	}
	newUUID, err := db.TableUUID(ctx, rebuildTable)
	if err != nil {
		return stats, err
	}
	if err := db.SwapInTable(ctx, storage.MessagesTable, rebuildTable, previousTable); err != nil {
		if errors.Is(err, storage.ErrSwapRename) {
			return stats, fmt.Errorf("%w: %s holds the rebuilt archive (UUID %s); the old archive (UUID %s) is %s or %s, so rename %s to %s if it exists; %s", err, storage.MessagesTable, newUUID, oldUUID, rebuildTable, previousTable, rebuildTable, previousTable, lost)
		}
		return stats, fmt.Errorf("%w: the exchange is atomic, but if its reply was lost it may have happened; the rebuilt archive is the table with UUID %s and the old one UUID %s (SELECT name, uuid FROM system.tables WHERE database = currentDatabase()); check before dropping either; %s", err, newUUID, oldUUID, lost)
	}
	if rebuildHooks.afterExchange != nil {
		rebuildHooks.afterExchange()
	}

	// The messages stored in the old archive since the first copy: until
	// the exchange, and by an insert in progress at it.
	exchanged, err := db.ServerTime(ctx)
	if err == nil {
		err = waitForInserts(ctx, db, exchanged)
	}
	if err != nil {
		return stats, fmt.Errorf("%w: the rebuilt archive is in place, but %s", err, lost)
	}
	n, err := copyStoredSince(ctx, db, previousTable, storage.MessagesTable, started)
	stats.Copied += n
	if err != nil {
		// A copy that fails part-way may have stored some of a message's
		// rows, after which its ID counts as present.
		return stats, fmt.Errorf("%w: the rebuilt archive is in place, but %s (a message may be partly copied)", err, lost)
	}
	left, err := db.MessagesMissingSince(ctx, previousTable, storage.MessagesTable, started)
	if err != nil {
		return stats, fmt.Errorf("check the copy: %w: the rebuilt archive is in place, but %s", err, lost)
	}
	if left != 0 {
		return stats, fmt.Errorf("%d messages not copied: the rebuilt archive is in place, but %s", left, lost)
	}
	return stats, nil
}

// copyStoredSince copies to to the messages stored in from since the time
// that to lacks, and returns how many it found to copy.
func copyStoredSince(ctx context.Context, db *storage.ClickHouseDB, from, to string, since time.Time) (uint64, error) {
	n, err := db.MessagesMissingSince(ctx, from, to, since)
	if err != nil || n == 0 {
		return 0, err
	}
	return n, db.CopyMessagesMissingSince(ctx, from, to, since)
}

// waitForInserts waits until the server has finished every insert that
// started before t, or until ctx ends.
func waitForInserts(ctx context.Context, db *storage.ClickHouseDB, t time.Time) error {
	for {
		n, err := db.InsertsStartedBefore(ctx, t)
		if err != nil || n == 0 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(insertPollEvery):
		}
	}
}

// formatStored formats a created_at bound as the column stores it.
func formatStored(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05.000")
}
