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

// rebuildStats counts what a rebuild wrote.
type rebuildStats struct {
	Messages int // Distinct messages read.
	Rows     int // Rows written (one per parse result, or one unparsed row).
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
// (zero keeps every flight). Messages written to the archive while the
// rebuild runs are lost at the swap, so every writer (live) must be stopped
// first.
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
	source, err := db.CountDistinctIDs(ctx, storage.MessagesTable)
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
	err = db.StreamRawMessages(ctx, storage.MessagesTable, rebuildChunk, func(m storage.RawMessage) error {
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

	written, err := db.CountDistinctIDs(ctx, rebuildTable)
	if err != nil {
		return stats, err
	}
	if written != source || uint64(stats.Messages) != source {
		return stats, fmt.Errorf("the archive has %d messages, %d were read and %s holds %d; not swapped", source, stats.Messages, rebuildTable, written)
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
			return stats, fmt.Errorf("%w: %s holds the rebuilt archive (UUID %s); the old archive (UUID %s) is %s or %s, so rename %s to %s if it exists", err, storage.MessagesTable, newUUID, oldUUID, rebuildTable, previousTable, rebuildTable, previousTable)
		}
		return stats, fmt.Errorf("%w: the exchange is atomic, but if its reply was lost it may have happened; the rebuilt archive is the table with UUID %s and the old one UUID %s (SELECT name, uuid FROM system.tables WHERE database = currentDatabase()); check before dropping either", err, newUUID, oldUUID)
	}
	return stats, nil
}
