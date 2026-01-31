package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

// setupTestPostgres creates a test database connection.
// Returns nil if no PostgreSQL connection is available.
func setupTestPostgres(t *testing.T) *PostgresDB {
	t.Helper()

	// Check for environment variable or use defaults.
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "acars"
	}
	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		password = "acars"
	}
	database := os.Getenv("POSTGRES_DB")
	if database == "" {
		database = "acars_state"
	}

	ctx := context.Background()
	pg, err := OpenPostgres(ctx, PostgresConfig{
		Host:     host,
		Port:     5432,
		User:     user,
		Password: password,
		Database: database,
	})
	if err != nil {
		return nil
	}

	// Ensure schema exists.
	if err := pg.CreateSchema(ctx); err != nil {
		pg.Close()
		return nil
	}

	return pg
}

func stringPtr(s string) *string { return &s }
func intPtr(i int) *int          { return &i }

func TestUpsertFlightEnrichment(t *testing.T) {
	pg := setupTestPostgres(t)
	if pg == nil {
		t.Skip("No PostgreSQL connection available")
	}
	defer pg.Close()

	ctx := context.Background()
	flightDate := time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC)

	// Clean up test data before and after the test.
	cleanup := func() {
		_, _ = pg.pool.Exec(ctx, "DELETE FROM flight_enrichment WHERE icao_hex = '7C6CA3' AND callsign = 'QF008' AND flight_date = $1", flightDate)
	}
	cleanup()
	defer cleanup()

	// First upsert - PDC data.
	err := pg.UpsertFlightEnrichment(ctx, FlightEnrichmentUpdate{
		ICAOHex:    "7C6CA3",
		Callsign:   "QF008",
		FlightDate: flightDate,
		DepartureRunway: stringPtr("34L"),
		SID:             stringPtr("RIC6"),
		Squawk:     stringPtr("4302"),
	})
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	// Second upsert - FPN data (should merge, not overwrite).
	err = pg.UpsertFlightEnrichment(ctx, FlightEnrichmentUpdate{
		ICAOHex:     "7C6CA3",
		Callsign:    "QF008",
		FlightDate:  flightDate,
		Origin:      stringPtr("YSSY"),
		Destination: stringPtr("KLAX"),
		Route:       []string{"YSSY", "ABARB", "KLAX"},
	})
	if err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	// Query and verify both fields present.
	result, err := pg.GetFlightEnrichment(ctx, "7C6CA3", "QF008", flightDate)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Check PDC fields are preserved.
	if result.DepartureRunway != "34L" {
		t.Errorf("departure_runway = %q, want 34L", result.DepartureRunway)
	}
	if result.SID != "RIC6" {
		t.Errorf("sid = %q, want RIC6", result.SID)
	}
	if result.Squawk != "4302" {
		t.Errorf("squawk = %q, want 4302", result.Squawk)
	}

	// Check FPN fields are added.
	if result.Origin != "YSSY" {
		t.Errorf("origin = %q, want YSSY", result.Origin)
	}
	if result.Destination != "KLAX" {
		t.Errorf("destination = %q, want KLAX", result.Destination)
	}
	if len(result.Route) != 3 || result.Route[0] != "YSSY" {
		t.Errorf("route = %v, want [YSSY ABARB KLAX]", result.Route)
	}
}

func TestUpsertFlightEnrichment_PaxData(t *testing.T) {
	pg := setupTestPostgres(t)
	if pg == nil {
		t.Skip("No PostgreSQL connection available")
	}
	defer pg.Close()

	ctx := context.Background()
	flightDate := time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC)

	// Clean up test data.
	cleanup := func() {
		_, _ = pg.pool.Exec(ctx, "DELETE FROM flight_enrichment WHERE icao_hex = 'TESTPX' AND callsign = 'TEST1' AND flight_date = $1", flightDate)
	}
	cleanup()
	defer cleanup()

	// Upsert with PAX data.
	err := pg.UpsertFlightEnrichment(ctx, FlightEnrichmentUpdate{
		ICAOHex:    "TESTPX",
		Callsign:   "TEST1",
		FlightDate: flightDate,
		PaxCount:   intPtr(288),
		PaxBreakdown: map[string]int{
			"first":    12,
			"business": 42,
			"economy":  234,
		},
	})
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	result, err := pg.GetFlightEnrichment(ctx, "TESTPX", "TEST1", flightDate)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if result.PaxCount == nil || *result.PaxCount != 288 {
		t.Errorf("pax_count = %v, want 288", result.PaxCount)
	}
	if result.PaxBreakdown == nil || result.PaxBreakdown["first"] != 12 {
		t.Errorf("pax_breakdown = %v, want first=12", result.PaxBreakdown)
	}
}

func TestGetFlightEnrichment_NotFound(t *testing.T) {
	pg := setupTestPostgres(t)
	if pg == nil {
		t.Skip("No PostgreSQL connection available")
	}
	defer pg.Close()

	ctx := context.Background()
	flightDate := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)

	result, err := pg.GetFlightEnrichment(ctx, "NONEXISTENT", "FAKE999", flightDate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for non-existent record, got %+v", result)
	}
}

func TestUpsertFlightEnrichment_MissingKey(t *testing.T) {
	pg := setupTestPostgres(t)
	if pg == nil {
		t.Skip("No PostgreSQL connection available")
	}
	defer pg.Close()

	ctx := context.Background()

	// Missing icao_hex - should return nil without error.
	err := pg.UpsertFlightEnrichment(ctx, FlightEnrichmentUpdate{
		Callsign:        "QF008",
		FlightDate:      time.Now(),
		DepartureRunway: stringPtr("34L"),
	})
	if err != nil {
		t.Errorf("expected nil error for missing icao_hex, got: %v", err)
	}

	// Missing flight_date - should return nil without error.
	err = pg.UpsertFlightEnrichment(ctx, FlightEnrichmentUpdate{
		ICAOHex:         "7C6CA3",
		Callsign:        "QF008",
		DepartureRunway: stringPtr("34L"),
	})
	if err != nil {
		t.Errorf("expected nil error for missing flight_date, got: %v", err)
	}
}