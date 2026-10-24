package flightroutes

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"
)

var (
	t1 = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	t2 = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
)

// The pairs and aliases are shaped like those learned from the archive in
// October 2026.
var testPairs = []Pair{
	{IATA: "QF", ICAO: "QFA", Observations: 2724, SameNumber: 2670},
	{IATA: "NW", ICAO: "DAL", Observations: 15683, SameNumber: 15683},
	{IATA: "DL", ICAO: "DAL", Observations: 8428, SameNumber: 8300},
	// LOT's callsigns are not its flight numbers.
	{IATA: "LO", ICAO: "LOT", Observations: 2294, SameNumber: 1193},
	// An IATA code used by two airlines, one of them seldom.
	{IATA: "XY", ICAO: "KNE", Observations: 95, SameNumber: 95},
	{IATA: "XY", ICAO: "NAS", Observations: 5, SameNumber: 5},
	// Too few observations to trust.
	{IATA: "ZZ", ICAO: "ZZA", Observations: 5, SameNumber: 5},
}

// store is an in-memory Store.
type store struct {
	aliases []Alias
	routes  []Route
}

func (s *store) AirlinePairs(context.Context) ([]Pair, error) { return testPairs, nil }

func (s *store) FlightAliases(_ context.Context, ids []string) ([]Alias, error) {
	var out []Alias
	for _, a := range s.aliases {
		for _, id := range ids {
			if a.IATA == id || a.ICAO == id {
				out = append(out, a)
				break
			}
		}
	}
	return out, nil
}

func (s *store) Routes(_ context.Context, flights []string) ([]Route, error) {
	var out []Route
	for _, r := range s.routes {
		for _, f := range flights {
			if r.Flight == f {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

var testStore = &store{
	aliases: []Alias{
		{IATA: "LO495", ICAO: "LOT49H", Observations: 7},
		{IATA: "LO495", ICAO: "LOT495", Observations: 1},
		// LO777 is mostly LOT777: a lookup of LOT77A must not take it.
		{IATA: "LO777", ICAO: "LOT77A", Observations: 2},
		{IATA: "LO777", ICAO: "LOT777", Observations: 20},
		// One observation is not enough.
		{IATA: "LO123", ICAO: "LOT12A", Observations: 1},
		// A flight number whose callsign differs from the code's rule.
		{IATA: "QF9", ICAO: "QFA9X", Observations: 4},
	},
	routes: []Route{
		{Flight: "QFA1", Origin: "YSSY", Destination: "WSSS", Observations: 32, FirstSeen: t1, LastSeen: t2},
		{Flight: "QF1", Origin: "YSSY", Destination: "WSSS", Observations: 719, FirstSeen: t1, LastSeen: t2},
		{Flight: "QF1", Origin: "WSSS", Destination: "EGLL", Observations: 285, FirstSeen: t1, LastSeen: t2},
		{Flight: "QF9", Origin: "YMML", Destination: "YPPH", Observations: 10, FirstSeen: t1, LastSeen: t2},
		{Flight: "QFA9X", Origin: "YMML", Destination: "YPPH", Observations: 3, FirstSeen: t1, LastSeen: t2},
		{Flight: "LO495", Origin: "EPWA", Destination: "LFPG", Observations: 20, FirstSeen: t1, LastSeen: t2},
		{Flight: "LOT49H", Origin: "EPWA", Destination: "LFPG", Observations: 5, FirstSeen: t1, LastSeen: t2},
		{Flight: "DAL1", Origin: "KATL", Destination: "EGLL", Observations: 2, FirstSeen: t1, LastSeen: t2},
		{Flight: "DL1", Origin: "KATL", Destination: "EGLL", Observations: 9, FirstSeen: t1, LastSeen: t2},
		{Flight: "NW1", Origin: "KATL", Destination: "EGLL", Observations: 4, FirstSeen: t1, LastSeen: t2},
		{Flight: "XY12", Origin: "OERK", Destination: "OEJN", Observations: 6, FirstSeen: t1, LastSeen: t2},
		{Flight: "LO777", Origin: "EPWA", Destination: "KJFK", Observations: 9, FirstSeen: t1, LastSeen: t2},
	},
}

func TestLookup(t *testing.T) {
	tests := []struct {
		query       string
		callsign    string
		identifiers []string
		routes      map[string]int // origin-destination: observations
	}{
		// By callsign or flight number, the routes of every identifier of
		// the flight, merged.
		{"QFA1", "QFA1", []string{"QF1", "QFA1"}, map[string]int{"YSSY-WSSS": 751, "WSSS-EGLL": 285}},
		{"QF1", "QFA1", []string{"QF1", "QFA1"}, map[string]int{"YSSY-WSSS": 751, "WSSS-EGLL": 285}},
		{"QF0001", "QFA1", []string{"QF1", "QFA1"}, map[string]int{"YSSY-WSSS": 751, "WSSS-EGLL": 285}},
		// A flight aliased to another callsign than the code's rule gives.
		// (The code's rule also makes QF9X equivalent, which has no routes.)
		{"QF9", "QFA9X", []string{"QF9", "QF9X", "QFA9X"}, map[string]int{"YMML-YPPH": 13}},
		{"QFA9X", "QFA9X", []string{"QF9", "QF9X", "QFA9X"}, map[string]int{"YMML-YPPH": 13}},
		// So QFA9 is not QF9.
		{"QFA9", "QFA9", []string{"QFA9"}, map[string]int{}},
		// LOT's callsigns are learned flight by flight.
		{"LO495", "LOT49H", []string{"LO495", "LOT49H"}, map[string]int{"EPWA-LFPG": 25}},
		{"LOT49H", "LOT49H", []string{"LO495", "LOT49H"}, map[string]int{"EPWA-LFPG": 5 + 20}},
		// Two IATA codes for one airline.
		{"DAL1", "DAL1", []string{"DAL1", "DL1", "NW1"}, map[string]int{"KATL-EGLL": 15}},
		{"DL1", "DAL1", []string{"DAL1", "DL1", "NW1"}, map[string]int{"KATL-EGLL": 15}},
		// An IATA code of two airlines, and one with too little evidence,
		// are not converted: the flight keeps only its own routes.
		{"XY12", "", []string{"XY12"}, map[string]int{"OERK-OEJN": 6}},
		{"ZZ1", "", []string{"ZZ1"}, map[string]int{}},
		{"KNE12", "KNE12", []string{"KNE12"}, map[string]int{}},
		// An IATA flight is merged into a callsign only if that is its
		// trusted alias among all its aliases.
		{"LOT77A", "LOT77A", []string{"LOT77A"}, map[string]int{}},
		{"LO777", "LOT777", []string{"LO777", "LOT777"}, map[string]int{"EPWA-KJFK": 9}},
		// An IATA code with a digit.
		{"B60001", "", []string{"B61"}, map[string]int{}},
		{"LO123", "", []string{"LO123"}, map[string]int{}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := Lookup(context.Background(), testStore, tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if got.Callsign != tt.callsign || !reflect.DeepEqual(got.Identifiers, tt.identifiers) {
				t.Errorf("callsign %q, identifiers %v; want %q, %v", got.Callsign, got.Identifiers, tt.callsign, tt.identifiers)
			}
			routes := map[string]int{}
			for _, r := range got.Routes {
				routes[r.Origin+"-"+r.Destination] = r.Observations
			}
			if !reflect.DeepEqual(routes, tt.routes) {
				t.Errorf("routes %v, want %v", routes, tt.routes)
			}
		})
	}
}

// A merged route lists the identifiers it was observed as, and spans their
// first and last observations.
func TestLookupMergedRoute(t *testing.T) {
	got, err := Lookup(context.Background(), testStore, "QF1")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got.Routes {
		if r.Origin != "YSSY" {
			continue
		}
		sort.Strings(r.ObservedAs)
		if !reflect.DeepEqual(r.ObservedAs, []string{"QF1", "QFA1"}) || !r.FirstSeen.Equal(t1) || !r.LastSeen.Equal(t2) {
			t.Errorf("merged route %+v", r)
		}
		return
	}
	t.Fatal("no YSSY route")
}

// An identifier with no known equivalent keeps only its own routes.
func TestLookupUnknown(t *testing.T) {
	got, err := Lookup(context.Background(), testStore, "N123AB")
	if err != nil {
		t.Fatal(err)
	}
	if got.Callsign != "" || len(got.Routes) != 0 {
		t.Errorf("got %+v, want nothing", got)
	}
}
