package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"acars_parser/internal/flightroutes"
)

// routeStore is an in-memory flightroutes.Store.
type routeStore struct{}

func (routeStore) AirlinePairs(context.Context) ([]flightroutes.Pair, error) {
	return []flightroutes.Pair{{IATA: "QF", ICAO: "QFA", Observations: 100, SameNumber: 100}}, nil
}
func (routeStore) FlightAliases(context.Context, []string) ([]flightroutes.Alias, error) {
	return nil, nil
}
func (routeStore) Routes(_ context.Context, flights []string) ([]flightroutes.Route, error) {
	t := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var out []flightroutes.Route
	for _, f := range flights {
		if f == "QF1" || f == "QFA1" {
			out = append(out, flightroutes.Route{Flight: f, Origin: "YSSY", Destination: "WSSS", Observations: 3, FirstSeen: t, LastSeen: t})
		}
	}
	return out, nil
}

func TestRoutesEndpoint(t *testing.T) {
	server := &EnrichmentServer{routes: routeStore{}}
	router := server.Router()
	for _, query := range []string{"QF1", "qfa1", "QF0001"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/routes/"+query, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d, want 200", query, rec.Code)
		}
		var got RoutesResponse
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.Callsign != "QFA1" || len(got.Routes) != 1 || got.Routes[0].Observations != 6 {
			t.Errorf("%s: got %+v, want QFA1 with one route seen 6 times", query, got)
		}
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/routes/ZZZ999", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown flight: status %d, want 404", rec.Code)
	}
}
