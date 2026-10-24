package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"acars_parser/internal/flightroutes"
)

// RoutesResponse is the response of the routes endpoint: the flight's
// routes, merged across its identifiers (see flightroutes.Lookup).
type RoutesResponse struct {
	Query string `json:"query"`
	flightroutes.Flight
}

// handleGetRoutes returns the routes of a flight, by its ICAO callsign
// ("QFA1") or its IATA flight number ("QF1", "QF0001").
func (s *EnrichmentServer) handleGetRoutes(w http.ResponseWriter, r *http.Request) {
	query := strings.ToUpper(strings.TrimSpace(chi.URLParam(r, "flight")))
	if query == "" {
		writeError(w, http.StatusBadRequest, "flight is required")
		return
	}
	flight, err := flightroutes.Lookup(r.Context(), s.routes, query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(flight.Routes) == 0 {
		writeError(w, http.StatusNotFound, "No routes found")
		return
	}
	writeJSON(w, http.StatusOK, RoutesResponse{Query: query, Flight: flight})
}
