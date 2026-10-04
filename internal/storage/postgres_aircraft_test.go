package storage

import (
	"context"
	"testing"
	"time"
)

// TestUpsertAircraftKeepsValuesWhenNewOnesAreEmpty checks that an update with
// an empty registration, type or operator leaves the stored value in place.
func TestUpsertAircraftKeepsValuesWhenNewOnesAreEmpty(t *testing.T) {
	pg := setupTestPostgres(t)
	if pg == nil {
		t.Skip("No PostgreSQL connection available")
	}
	const hex = "7CFFF1"
	// Cleanups run last-registered first: delete the row, then close the pool.
	t.Cleanup(pg.Close)
	t.Cleanup(func() {
		if _, err := pg.pool.Exec(context.Background(), `DELETE FROM aircraft WHERE icao_hex = $1`, hex); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	ctx := context.Background()

	now := time.Now()
	full := Aircraft{ICAOHex: hex, Registration: "VH-TST", TypeCode: "A332", Operator: "Test", FirstSeen: now, LastSeen: now, MsgCount: 1}
	if err := pg.UpsertAircraft(ctx, full); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	empty := Aircraft{ICAOHex: hex, FirstSeen: now, LastSeen: now, MsgCount: 1}
	if err := pg.UpsertAircraft(ctx, empty); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := pg.GetAircraft(ctx, hex)
	if err != nil || got == nil {
		t.Fatalf("GetAircraft: %v, %v", got, err)
	}
	if got.Registration != "VH-TST" || got.TypeCode != "A332" || got.Operator != "Test" {
		t.Errorf("stored = %q, %q, %q, want VH-TST, A332, Test", got.Registration, got.TypeCode, got.Operator)
	}
	if got.MsgCount != 2 {
		t.Errorf("MsgCount = %d, want 2", got.MsgCount)
	}
}
