package storage

import (
	"context"
	"testing"
	"time"

	"acars_parser/internal/flightroutes"
)

// TestFlightAliasesLookup checks the flight aliases, the airline_codes view
// and the routes through flightroutes.Lookup.
func TestFlightAliasesLookup(t *testing.T) {
	pg := setupTestPostgres(t)
	if pg == nil {
		t.Skip("No PostgreSQL connection available")
	}
	defer pg.Close()
	ctx := context.Background()
	cleanup := func() {
		_, _ = pg.pool.Exec(ctx, "DELETE FROM flight_aliases WHERE iata_flight LIKE 'Q9%'")
		_, _ = pg.pool.Exec(ctx, "DELETE FROM routes WHERE flight_pattern LIKE 'Q9%' OR flight_pattern LIKE 'QQA%'")
	}
	cleanup()
	defer cleanup()

	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	// 25 observations of Q9 and QQA flights with the same numbers.
	for i := 0; i < 25; i++ {
		if err := pg.UpsertFlightAlias(ctx, "Q91", "QQA1", t0.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []Route{
		{FlightPattern: "Q91", OriginICAO: "YSSY", DestICAO: "WSSS", ObservationCount: 1, FirstSeen: t0, LastSeen: t0},
		{FlightPattern: "QQA1", OriginICAO: "YSSY", DestICAO: "WSSS", ObservationCount: 1, FirstSeen: t0, LastSeen: t0},
	} {
		if _, err := pg.UpsertRoute(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	pairs, err := pg.AirlinePairs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range pairs {
		if p.IATA == "Q9" && p.ICAO == "QQA" {
			found = p.Observations == 25 && p.SameNumber == 25
		}
	}
	if !found {
		t.Errorf("airline_codes has no Q9/QQA pair with 25 observations: %+v", pairs)
	}

	got, err := flightroutes.Lookup(ctx, pg, "Q91")
	if err != nil {
		t.Fatal(err)
	}
	if got.Callsign != "QQA1" || len(got.Routes) != 1 || got.Routes[0].Observations != 2 {
		t.Errorf("Lookup(Q91) = %+v, want QQA1 with one route seen twice", got)
	}
}
