package main

import (
	"math"
	"reflect"
	"testing"

	"acars_parser/internal/flightroutes"
)

func flight(callsign string, routes ...flightroutes.MergedRoute) flightroutes.Flight {
	return flightroutes.Flight{Callsign: callsign, Identifiers: []string{callsign}, Routes: routes}
}

func route(origin, dest string, obs int) flightroutes.MergedRoute {
	return flightroutes.MergedRoute{Origin: origin, Destination: dest, Observations: obs}
}

func TestSelectRoutes(t *testing.T) {
	flights := []flightroutes.Flight{
		// One route.
		flight("QFA1", route("YSSY", "WSSS", 100)),
		// A dominant route and a few reports of another.
		flight("DAL1368", route("KJFK", "KMSY", 90), route("KMSY", "KJFK", 10)),
		// A callsign flown both ways: neither route is the flight's.
		flight("DAL2", route("KATL", "KLAX", 60), route("KLAX", "KATL", 40)),
		// Too few observations.
		flight("UAE1", route("OMDB", "EGLL", 4)),
		// An identifier whose callsign is not known.
		{Identifiers: []string{"XY12"}, Routes: []flightroutes.MergedRoute{route("OERK", "OEJN", 50)}},
	}
	got, stats := selectRoutes(flights, 5, 0.8)
	want := []exportRow{
		{Callsign: "QFA1", Airports: []string{"YSSY", "WSSS"}},
		{Callsign: "DAL1368", Airports: []string{"KJFK", "KMSY"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rows %+v, want %+v", got, want)
	}
	wantStats := selectStats{Flights: 5, Exported: 2, NoCallsign: 1, TooFew: 1, Ambiguous: 1}
	if stats != wantStats {
		t.Errorf("stats %+v, want %+v", stats, wantStats)
	}
}

func TestValidShare(t *testing.T) {
	for share, want := range map[float64]bool{0.8: true, 1: true, 0.5: false, 1.1: false, math.NaN(): false} {
		if got := validShare(share); got != want {
			t.Errorf("validShare(%v) = %v, want %v", share, got, want)
		}
	}
}
