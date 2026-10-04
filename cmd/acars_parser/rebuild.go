package main

import (
	"context"
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

// rebuildStats counts what a rebuild wrote.
type rebuildStats struct {
	Messages int // Distinct messages read.
	Rows     int // Rows written (one per parse result, or one unparsed row).
	Empty    int // Messages without text, which have no rows.
}

// rebuildArchive reparses every message in the archive with the current
// parsers and replaces the archive with the result, without the duplicate
// rows that "reparse -update" leaves.
//
// Each distinct message is read once (its rows hold the same transmitted
// content), dispatched to the registry, and written as live writes it, into
// messages_rebuild. When the number of messages written matches, the tables
// are exchanged and the old archive is kept as messages_previous; nothing
// is swapped otherwise. A rebuild refuses to start while messages_previous
// exists, so that a kept archive is never overwritten.
//
// The stored flight of messages before dropFlightBefore is blanked: older
// code stored Airframes' flight record there, which is not transmitted data
// (zero keeps every flight). Messages written to the archive while the
// rebuild runs are lost at the swap, so live must be stopped first.
func rebuildArchive(ctx context.Context, db *storage.ClickHouseDB, reg *registry.Registry, dropFlightBefore time.Time, batchSize int) (rebuildStats, error) {
	var stats rebuildStats
	if exists, err := db.TableExists(ctx, previousTable); err != nil {
		return stats, err
	} else if exists {
		return stats, fmt.Errorf("%s exists (the archive before the last rebuild); drop it before rebuilding again", previousTable)
	}
	if err := db.DropTable(ctx, rebuildTable); err != nil {
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
	err := db.StreamRawMessages(ctx, storage.MessagesTable, func(m storage.RawMessage) error {
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
			stats.Empty++
		}
		stats.Rows += len(rows)
		batch = append(batch, rows...)
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
		if stats.Messages%500000 == 0 {
			fmt.Printf("  %d messages reparsed\n", stats.Messages)
		}
		return nil
	})
	if err == nil {
		err = flush()
	}
	if err != nil {
		return stats, fmt.Errorf("rebuild: %w (%s left in place, archive unchanged)", err, rebuildTable)
	}

	written, err := db.CountDistinctIDs(ctx, rebuildTable)
	if err != nil {
		return stats, err
	}
	if want := uint64(stats.Messages - stats.Empty); written != want {
		return stats, fmt.Errorf("%s holds %d messages, want %d; not swapped", rebuildTable, written, want)
	}
	if err := db.ExchangeTables(ctx, storage.MessagesTable, rebuildTable); err != nil {
		return stats, err
	}
	if err := db.RenameTable(ctx, rebuildTable, previousTable); err != nil {
		return stats, err
	}
	return stats, nil
}
