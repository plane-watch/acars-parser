// Package api provides REST API endpoints for flight enrichment data.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"acars_parser/internal/storage"
)

// EnrichmentServer provides REST API access to flight enrichment data.
type EnrichmentServer struct {
	pg          *storage.PostgresDB
	port        int
	authEnabled bool
	apiKeys     map[string]bool // Simple API key auth (when enabled).
}

// Config holds configuration for the enrichment API server.
type Config struct {
	Port        int
	AuthEnabled bool
	APIKeys     []string // List of valid API keys.
}

// NewEnrichmentServer creates a new enrichment API server.
func NewEnrichmentServer(pg *storage.PostgresDB, cfg Config) *EnrichmentServer {
	keys := make(map[string]bool)
	for _, k := range cfg.APIKeys {
		if k != "" {
			keys[k] = true
		}
	}

	return &EnrichmentServer{
		pg:          pg,
		port:        cfg.Port,
		authEnabled: cfg.AuthEnabled,
		apiKeys:     keys,
	}
}

// Run starts the HTTP server.
func (s *EnrichmentServer) Run() error {
	r := chi.NewRouter()

	// Standard middleware.
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS for browser access.
	r.Use(corsMiddleware)

	// Optional authentication.
	if s.authEnabled {
		r.Use(s.authMiddleware)
	}

	// API routes.
	r.Route("/api/v1", func(r chi.Router) {
		// Health check (no auth required).
		r.Get("/health", s.handleHealth)

		// Enrichment endpoints.
		r.Get("/enrichment/{icao_hex}", s.handleGetEnrichment)
		r.Get("/enrichment/{icao_hex}/{callsign}", s.handleGetEnrichmentByCallsign)
		r.Get("/enrichment/{icao_hex}/{callsign}/{date}", s.handleGetEnrichmentByDate)

		// Batch lookup for multiple aircraft.
		r.Post("/enrichment/batch", s.handleBatchEnrichment)
	})

	addr := ":" + itoa(s.port)
	log.Printf("Enrichment API starting at http://localhost%s", addr)
	if s.authEnabled {
		log.Printf("Authentication: ENABLED (API key required)")
	} else {
		log.Printf("Authentication: DISABLED (open access)")
	}

	return http.ListenAndServe(addr, r)
}

// Router returns the configured chi router for embedding in other servers.
func (s *EnrichmentServer) Router() chi.Router {
	r := chi.NewRouter()

	// Optional authentication.
	if s.authEnabled {
		r.Use(s.authMiddleware)
	}

	r.Get("/health", s.handleHealth)
	r.Get("/enrichment/{icao_hex}", s.handleGetEnrichment)
	r.Get("/enrichment/{icao_hex}/{callsign}", s.handleGetEnrichmentByCallsign)
	r.Get("/enrichment/{icao_hex}/{callsign}/{date}", s.handleGetEnrichmentByDate)
	r.Post("/enrichment/batch", s.handleBatchEnrichment)

	return r
}

// corsMiddleware adds CORS headers for browser access.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates API key authentication.
func (s *EnrichmentServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check X-API-Key header first.
		apiKey := r.Header.Get("X-API-Key")

		// Fall back to Authorization: Bearer <key>.
		if apiKey == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		// Fall back to query parameter (for simple testing).
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if apiKey == "" {
			writeError(w, http.StatusUnauthorized, "API key required")
			return
		}

		if !s.apiKeys[apiKey] {
			writeError(w, http.StatusForbidden, "Invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// EnrichmentResponse is the JSON response for enrichment queries.
type EnrichmentResponse struct {
	ICAOHex         string            `json:"icao_hex"`
	Callsign        string            `json:"callsign"`
	FlightDate      string            `json:"flight_date"`
	Origin          string            `json:"origin,omitempty"`
	Destination     string            `json:"destination,omitempty"`
	Route           []string          `json:"route,omitempty"`
	ETA             string            `json:"eta,omitempty"`
	DepartureRunway string            `json:"departure_runway,omitempty"`
	ArrivalRunway   string            `json:"arrival_runway,omitempty"`
	SID             string            `json:"sid,omitempty"`
	Squawk          string            `json:"squawk,omitempty"`
	PaxCount        int               `json:"pax_count,omitempty"`
	PaxBreakdown    map[string]int    `json:"pax_breakdown,omitempty"`
	LastUpdated     string            `json:"last_updated"`
}

func enrichmentToResponse(e *storage.FlightEnrichment) EnrichmentResponse {
	resp := EnrichmentResponse{
		ICAOHex:         e.ICAOHex,
		Callsign:        e.Callsign,
		FlightDate:      e.FlightDate.Format("2006-01-02"),
		Origin:          e.Origin,
		Destination:     e.Destination,
		Route:           e.Route,
		DepartureRunway: e.DepartureRunway,
		ArrivalRunway:   e.ArrivalRunway,
		SID:             e.SID,
		Squawk:          e.Squawk,
		LastUpdated:     e.UpdatedAt.Format(time.RFC3339),
	}

	if e.ETA != nil {
		resp.ETA = e.ETA.Format("15:04")
	}
	if e.PaxCount != nil {
		resp.PaxCount = *e.PaxCount
	}
	if len(e.PaxBreakdown) > 0 {
		resp.PaxBreakdown = e.PaxBreakdown
	}

	return resp
}

func (s *EnrichmentServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *EnrichmentServer) handleGetEnrichment(w http.ResponseWriter, r *http.Request) {
	icaoHex := strings.ToUpper(chi.URLParam(r, "icao_hex"))
	if icaoHex == "" {
		writeError(w, http.StatusBadRequest, "icao_hex is required")
		return
	}

	ctx := context.Background()

	// Get all enrichments for this aircraft on today's date.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	enrichments, err := s.pg.GetFlightEnrichmentsByAircraft(ctx, icaoHex, today)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(enrichments) == 0 {
		writeError(w, http.StatusNotFound, "No enrichment data found for aircraft")
		return
	}

	// Return the most recent enrichment (or all if multiple callsigns).
	var results []EnrichmentResponse
	for _, e := range enrichments {
		results = append(results, enrichmentToResponse(&e))
	}

	writeJSON(w, http.StatusOK, results)
}

func (s *EnrichmentServer) handleGetEnrichmentByCallsign(w http.ResponseWriter, r *http.Request) {
	icaoHex := strings.ToUpper(chi.URLParam(r, "icao_hex"))
	callsign := strings.ToUpper(chi.URLParam(r, "callsign"))

	if icaoHex == "" || callsign == "" {
		writeError(w, http.StatusBadRequest, "icao_hex and callsign are required")
		return
	}

	ctx := context.Background()

	// Default to today.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	enrichment, err := s.pg.GetFlightEnrichment(ctx, icaoHex, callsign, today)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if enrichment == nil {
		writeError(w, http.StatusNotFound, "No enrichment data found")
		return
	}

	writeJSON(w, http.StatusOK, enrichmentToResponse(enrichment))
}

func (s *EnrichmentServer) handleGetEnrichmentByDate(w http.ResponseWriter, r *http.Request) {
	icaoHex := strings.ToUpper(chi.URLParam(r, "icao_hex"))
	callsign := strings.ToUpper(chi.URLParam(r, "callsign"))
	dateStr := chi.URLParam(r, "date")

	if icaoHex == "" || callsign == "" || dateStr == "" {
		writeError(w, http.StatusBadRequest, "icao_hex, callsign, and date are required")
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid date format (use YYYY-MM-DD)")
		return
	}

	ctx := context.Background()
	enrichment, err := s.pg.GetFlightEnrichment(ctx, icaoHex, callsign, date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if enrichment == nil {
		writeError(w, http.StatusNotFound, "No enrichment data found")
		return
	}

	writeJSON(w, http.StatusOK, enrichmentToResponse(enrichment))
}

// BatchRequest is the request body for batch enrichment lookups.
type BatchRequest struct {
	Aircraft []BatchAircraftQuery `json:"aircraft"`
}

// BatchAircraftQuery represents a single aircraft query in a batch request.
type BatchAircraftQuery struct {
	ICAOHex  string `json:"icao_hex"`
	Callsign string `json:"callsign,omitempty"` // Optional: if provided, filters to specific callsign.
}

// BatchResponse is the response for batch enrichment lookups.
type BatchResponse struct {
	Results map[string][]EnrichmentResponse `json:"results"` // Keyed by icao_hex.
	Errors  map[string]string               `json:"errors,omitempty"`
}

func (s *EnrichmentServer) handleBatchEnrichment(w http.ResponseWriter, r *http.Request) {
	var req BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if len(req.Aircraft) == 0 {
		writeError(w, http.StatusBadRequest, "No aircraft specified")
		return
	}

	if len(req.Aircraft) > 100 {
		writeError(w, http.StatusBadRequest, "Maximum 100 aircraft per batch request")
		return
	}

	ctx := context.Background()
	today := time.Now().UTC().Truncate(24 * time.Hour)

	resp := BatchResponse{
		Results: make(map[string][]EnrichmentResponse),
		Errors:  make(map[string]string),
	}

	for _, q := range req.Aircraft {
		icaoHex := strings.ToUpper(q.ICAOHex)
		if icaoHex == "" {
			continue
		}

		if q.Callsign != "" {
			// Specific callsign lookup.
			callsign := strings.ToUpper(q.Callsign)
			enrichment, err := s.pg.GetFlightEnrichment(ctx, icaoHex, callsign, today)
			if err != nil {
				resp.Errors[icaoHex] = err.Error()
				continue
			}
			if enrichment != nil {
				resp.Results[icaoHex] = []EnrichmentResponse{enrichmentToResponse(enrichment)}
			}
		} else {
			// Get all enrichments for this aircraft.
			enrichments, err := s.pg.GetFlightEnrichmentsByAircraft(ctx, icaoHex, today)
			if err != nil {
				resp.Errors[icaoHex] = err.Error()
				continue
			}
			for _, e := range enrichments {
				resp.Results[icaoHex] = append(resp.Results[icaoHex], enrichmentToResponse(&e))
			}
		}
	}

	// Remove empty errors map for cleaner output.
	if len(resp.Errors) == 0 {
		resp.Errors = nil
	}

	writeJSON(w, http.StatusOK, resp)
}

// Helper functions.

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func itoa(i int) string {
	return strconv.Itoa(i)
}