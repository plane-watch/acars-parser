package storage

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// TestUpsertFlightStateKeepsValuesWhenNewOnesAreEmpty checks that an update
// with empty text fields or no waypoints leaves the stored values in place.
func TestUpsertFlightStateKeepsValuesWhenNewOnesAreEmpty(t *testing.T) {
	pg := setupTestPostgres(t)
	if pg == nil {
		t.Skip("No PostgreSQL connection available")
	}
	const key = "test-flight-state-empty"
	t.Cleanup(pg.Close)
	t.Cleanup(func() {
		if _, err := pg.pool.Exec(context.Background(), `DELETE FROM flight_state WHERE key = $1`, key); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	ctx := context.Background()

	now := time.Now()
	full := FlightState{Key: key, ICAOHex: "7CFFF2", Registration: "VH-TST", FlightNumber: "QFA1",
		Origin: "YSSY", Destination: "YMML", Waypoints: []string{"WOL", "ELW"}, FirstSeen: now, LastSeen: now, MsgCount: 1}
	if err := pg.UpsertFlightState(ctx, full); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := pg.UpsertFlightState(ctx, FlightState{Key: key, FirstSeen: now, LastSeen: now, MsgCount: 1}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := pg.GetFlightState(ctx, key)
	if err != nil || got == nil {
		t.Fatalf("GetFlightState: %v, %v", got, err)
	}
	if got.ICAOHex != "7CFFF2" || got.Registration != "VH-TST" || got.FlightNumber != "QFA1" ||
		got.Origin != "YSSY" || got.Destination != "YMML" {
		t.Errorf("stored = %+v, want the first upsert's values", *got)
	}
	if !reflect.DeepEqual(got.Waypoints, []string{"WOL", "ELW"}) {
		t.Errorf("waypoints = %v, want [WOL ELW]", got.Waypoints)
	}
}

// TestFlightStateWithoutWaypointsCanBeRead checks that a row stored without
// waypoints (as NULL) reads back with none.
func TestFlightStateWithoutWaypointsCanBeRead(t *testing.T) {
	pg := setupTestPostgres(t)
	if pg == nil {
		t.Skip("No PostgreSQL connection available")
	}
	const key = "test-flight-state-no-waypoints"
	t.Cleanup(pg.Close)
	t.Cleanup(func() {
		if _, err := pg.pool.Exec(context.Background(), `DELETE FROM flight_state WHERE key = $1`, key); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	ctx := context.Background()

	now := time.Now()
	if err := pg.UpsertFlightState(ctx, FlightState{Key: key, Registration: "VH-TST", FirstSeen: now, LastSeen: now, MsgCount: 1}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := pg.GetFlightState(ctx, key)
	if err != nil || got == nil {
		t.Fatalf("GetFlightState: %v, %v", got, err)
	}
	if len(got.Waypoints) != 0 {
		t.Errorf("waypoints = %v, want none", got.Waypoints)
	}
}
