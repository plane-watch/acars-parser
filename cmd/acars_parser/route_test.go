package main

import (
	"testing"
	"time"

	"acars_parser/internal/extractor"
)

// TestFlightRoute checks that a route is recorded only for a flight: the
// routes table maps flight numbers to origin and destination, so a route
// seen without a flight number (an aircraft's movement) is not recorded
// under the registration.
func TestFlightRoute(t *testing.T) {
	seen := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	icao := func(flight string) *extractor.FlightUpdate {
		return &extractor.FlightUpdate{FlightNumber: flight, Registration: "VH-OQA", Origin: "YSSY", Destination: "KLAX",
			AirportCodes: extractor.AirportCodesICAO}
	}

	r, ok := flightRoute(icao("QFA11"), seen)
	if !ok || r.FlightPattern != "QFA11" || r.OriginICAO != "YSSY" || r.DestICAO != "KLAX" || r.ObservationCount != 1 ||
		!r.FirstSeen.Equal(seen) || !r.LastSeen.Equal(seen) || r.IsMultiStop {
		t.Errorf("flightRoute = %+v, %v", r, ok)
	}
	if r, ok := flightRoute(icao(""), seen); ok {
		t.Errorf("a route without a flight number was recorded: %+v", r)
	}
	iata := icao("QF11")
	iata.AirportCodes = extractor.AirportCodesIATA
	if _, ok := flightRoute(iata, seen); ok {
		t.Error("an IATA route was recorded in the ICAO routes table")
	}
	if _, ok := flightRoute(nil, seen); ok {
		t.Error("a nil update gave a route")
	}
	// An aircraft that transmits its registration as the flight has no
	// flight number; "RPC7783" would read as an ICAO callsign.
	tail := icao("RPC7783")
	tail.Registration = "RP-C7783"
	if r, ok := flightRoute(tail, seen); ok {
		t.Errorf("a route under the registration was recorded: %+v", r)
	}
	// The extractor strips the flight's leading zeros, so "RPC0123" arrives
	// as "RPC123".
	tail = icao("RPC123")
	tail.Registration = "RP-C0123"
	if r, ok := flightRoute(tail, seen); ok {
		t.Errorf("a route under the normalised registration was recorded: %+v", r)
	}
}
