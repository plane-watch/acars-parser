package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"acars_parser/internal/storage"
)

// mockPostgresDB implements the database interface for testing.
type mockPostgresDB struct {
	enrichments map[string][]storage.FlightEnrichment
}

func newMockDB() *mockPostgresDB {
	return &mockPostgresDB{
		enrichments: make(map[string][]storage.FlightEnrichment),
	}
}

func (m *mockPostgresDB) addEnrichment(e storage.FlightEnrichment) {
	key := e.ICAOHex + "|" + e.FlightDate.Format("2006-01-02")
	m.enrichments[key] = append(m.enrichments[key], e)
}

// EnrichmentStore defines the interface for enrichment storage.
// This allows us to mock the database in tests.
type EnrichmentStore interface {
	GetFlightEnrichment(ctx context.Context, icaoHex, callsign string, flightDate time.Time) (*storage.FlightEnrichment, error)
	GetFlightEnrichmentsByAircraft(ctx context.Context, icaoHex string, flightDate time.Time) ([]storage.FlightEnrichment, error)
}

func TestHealthEndpoint(t *testing.T) {
	server := NewEnrichmentServer(nil, Config{Port: 8081})
	router := server.Router()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestAuthMiddleware(t *testing.T) {
	server := NewEnrichmentServer(nil, Config{
		Port:        8081,
		AuthEnabled: true,
		APIKeys:     []string{"test-key-123", "another-key"},
	})
	router := server.Router()

	tests := []struct {
		name       string
		apiKey     string
		keyHeader  string
		wantStatus int
	}{
		{
			name:       "no key",
			apiKey:     "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid key",
			apiKey:     "wrong-key",
			keyHeader:  "X-API-Key",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "valid key via X-API-Key",
			apiKey:     "test-key-123",
			keyHeader:  "X-API-Key",
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid key via Bearer",
			apiKey:     "another-key",
			keyHeader:  "Authorization",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			if tt.apiKey != "" {
				if tt.keyHeader == "Authorization" {
					req.Header.Set("Authorization", "Bearer "+tt.apiKey)
				} else {
					req.Header.Set(tt.keyHeader, tt.apiKey)
				}
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestAuthMiddlewareQueryParam(t *testing.T) {
	server := NewEnrichmentServer(nil, Config{
		Port:        8081,
		AuthEnabled: true,
		APIKeys:     []string{"query-key"},
	})
	router := server.Router()

	req := httptest.NewRequest(http.MethodGet, "/health?api_key=query-key", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestEnrichmentResponseFormat(t *testing.T) {
	now := time.Now().UTC()
	eta := now.Add(2 * time.Hour)
	paxCount := 150

	e := &storage.FlightEnrichment{
		ICAOHex:         "7C6CA3",
		Callsign:        "QFA9",
		FlightDate:      now.Truncate(24 * time.Hour),
		Origin:          "YPPH",
		Destination:     "EGLL",
		Route:           []string{"JULIM", "BEVLY", "ORRSU"},
		ETA:             &eta,
		DepartureRunway: "03",
		ArrivalRunway:   "27L",
		SID:             "JULIM6",
		Squawk:          "4521",
		PaxCount:        &paxCount,
		UpdatedAt:       now,
	}

	resp := enrichmentToResponse(e)

	if resp.ICAOHex != "7C6CA3" {
		t.Errorf("expected ICAOHex '7C6CA3', got %q", resp.ICAOHex)
	}
	if resp.Callsign != "QFA9" {
		t.Errorf("expected Callsign 'QFA9', got %q", resp.Callsign)
	}
	if resp.Origin != "YPPH" {
		t.Errorf("expected Origin 'YPPH', got %q", resp.Origin)
	}
	if resp.Destination != "EGLL" {
		t.Errorf("expected Destination 'EGLL', got %q", resp.Destination)
	}
	if len(resp.Route) != 3 {
		t.Errorf("expected 3 route waypoints, got %d", len(resp.Route))
	}
	if resp.ETA != eta.Format("15:04") {
		t.Errorf("expected ETA %q, got %q", eta.Format("15:04"), resp.ETA)
	}
	if resp.DepartureRunway != "03" {
		t.Errorf("expected DepartureRunway '03', got %q", resp.DepartureRunway)
	}
	if resp.ArrivalRunway != "27L" {
		t.Errorf("expected ArrivalRunway '27L', got %q", resp.ArrivalRunway)
	}
	if resp.SID != "JULIM6" {
		t.Errorf("expected SID 'JULIM6', got %q", resp.SID)
	}
	if resp.Squawk != "4521" {
		t.Errorf("expected Squawk '4521', got %q", resp.Squawk)
	}
	if resp.PaxCount != 150 {
		t.Errorf("expected PaxCount 150, got %d", resp.PaxCount)
	}
}

func TestBatchRequestValidation(t *testing.T) {
	server := NewEnrichmentServer(nil, Config{Port: 8081})
	router := chi.NewRouter()
	router.Post("/enrichment/batch", server.handleBatchEnrichment)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid JSON",
		},
		{
			name:       "invalid json",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid JSON",
		},
		{
			name:       "empty aircraft list",
			body:       `{"aircraft": []}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "No aircraft specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/enrichment/batch", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}

			var resp map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&resp); err == nil {
				if tt.wantError != "" && resp["error"] == "" {
					t.Errorf("expected error containing %q", tt.wantError)
				}
			}
		})
	}
}

func TestCORSHeaders(t *testing.T) {
	server := NewEnrichmentServer(nil, Config{Port: 8081})

	// Build a router with CORS middleware.
	r := chi.NewRouter()
	r.Use(corsMiddleware)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test OPTIONS request.
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for OPTIONS, got %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS Allow-Origin header")
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS Allow-Methods header")
	}

	// Suppress unused variable warning.
	_ = server
}

func TestDateParsing(t *testing.T) {
	server := NewEnrichmentServer(nil, Config{Port: 8081})
	router := chi.NewRouter()
	router.Get("/enrichment/{icao_hex}/{callsign}/{date}", server.handleGetEnrichmentByDate)

	// Note: Tests requiring database lookup are skipped when db is nil.
	// These tests only verify date parsing validation.
	tests := []struct {
		name       string
		date       string
		wantStatus int
	}{
		{
			name:       "invalid date format",
			date:       "30-01-2026",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid date",
			date:       "not-a-date",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/enrichment/7C6CA3/QFA9/"+tt.date, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}