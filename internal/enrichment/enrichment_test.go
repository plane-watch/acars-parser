package enrichment

import (
	"testing"
	"time"

	"acars_parser/internal/extractor"
	"acars_parser/internal/registry"
)

// mockPDCResult implements registry.Result for testing PDC extraction.
type mockPDCResult struct {
	FlightNumber string   `json:"flight_number,omitempty"`
	Origin       string   `json:"origin,omitempty"`
	Destination  string   `json:"destination,omitempty"`
	Runway       string   `json:"runway,omitempty"`
	SID          string   `json:"sid,omitempty"`
	Squawk       string   `json:"squawk,omitempty"`
	RouteWpts    []string `json:"route_waypoints,omitempty"`
}

func (r *mockPDCResult) Type() string     { return "pdc" }
func (r *mockPDCResult) MessageID() int64 { return 0 }

// mockFPNResult implements registry.Result for testing flight plan extraction.
type mockFPNResult struct {
	FlightNum   string         `json:"flight_num,omitempty"`
	Origin      string         `json:"origin,omitempty"`
	Destination string         `json:"destination,omitempty"`
	Waypoints   []mockWaypoint `json:"waypoints,omitempty"`
}

type mockWaypoint struct {
	Name string `json:"name"`
}

func (r *mockFPNResult) Type() string     { return "flight_plan" }
func (r *mockFPNResult) MessageID() int64 { return 0 }

// mockLoadsheetResult implements registry.Result for testing loadsheet extraction.
type mockLoadsheetResult struct {
	Flight      string `json:"flight,omitempty"`
	Origin      string `json:"origin,omitempty"`
	Destination string `json:"destination,omitempty"`
	PAX         int    `json:"pax,omitempty"`
}

func (r *mockLoadsheetResult) Type() string     { return "loadsheet" }
func (r *mockLoadsheetResult) MessageID() int64 { return 0 }

func TestExtractFromPDC(t *testing.T) {
	timestamp := time.Date(2026, 1, 27, 14, 30, 0, 0, time.UTC)

	pdcResult := &mockPDCResult{
		FlightNumber: "QF008",
		Origin:       "YSSY",
		Destination:  "KLAX",
		Runway:       "34L",
		SID:          "RIC6",
		Squawk:       "4302",
	}

	update := ExtractEnrichment("7C6CA3", "", timestamp, []registry.Result{pdcResult})

	if update == nil {
		t.Fatal("expected update, got nil")
	}
	if update.ICAOHex != "7C6CA3" {
		t.Errorf("icao_hex = %q, want 7C6CA3", update.ICAOHex)
	}
	if update.Callsign != "QF8" {
		t.Errorf("callsign = %q, want QF8 (normalised)", update.Callsign)
	}
	if update.DepartureRunway == nil || *update.DepartureRunway != "34L" {
		t.Errorf("runway = %v, want 34L", update.DepartureRunway)
	}
	if update.SID == nil || *update.SID != "RIC6" {
		t.Errorf("sid = %v, want RIC6", update.SID)
	}
	if update.Squawk == nil || *update.Squawk != "4302" {
		t.Errorf("squawk = %v, want 4302", update.Squawk)
	}
	if update.Origin == nil || *update.Origin != "YSSY" {
		t.Errorf("origin = %v, want YSSY", update.Origin)
	}
}

func TestExtractFromFlightPlan(t *testing.T) {
	timestamp := time.Date(2026, 1, 27, 14, 30, 0, 0, time.UTC)

	fpnResult := &mockFPNResult{
		FlightNum:   "QR8157",
		Origin:      "SBGR",
		Destination: "SEQM",
		Waypoints: []mockWaypoint{
			{Name: "GERTU"},
			{Name: "EVRIV"},
			{Name: "BAGRE"},
		},
	}

	update := ExtractEnrichment("06A123", "", timestamp, []registry.Result{fpnResult})

	if update == nil {
		t.Fatal("expected update, got nil")
	}
	if update.Callsign != "QR8157" {
		t.Errorf("callsign = %q, want QR8157", update.Callsign)
	}
	if update.Origin == nil || *update.Origin != "SBGR" {
		t.Errorf("origin = %v, want SBGR", update.Origin)
	}
	if len(update.Route) != 3 || update.Route[0] != "GERTU" {
		t.Errorf("route = %v, want [GERTU EVRIV BAGRE]", update.Route)
	}
}

func TestExtractFromLoadsheet(t *testing.T) {
	timestamp := time.Date(2026, 1, 27, 14, 30, 0, 0, time.UTC)

	loadsheetResult := &mockLoadsheetResult{
		Flight:      "QF007",
		Origin:      "YSSY",
		Destination: "KDFW",
		PAX:         459,
	}

	update := ExtractEnrichment("7C6CA3", "", timestamp, []registry.Result{loadsheetResult})

	if update == nil {
		t.Fatal("expected update, got nil")
	}
	if update.Callsign != "QF7" {
		t.Errorf("callsign = %q, want QF7 (normalised)", update.Callsign)
	}
	if update.PaxCount == nil || *update.PaxCount != 459 {
		t.Errorf("pax_count = %v, want 459", update.PaxCount)
	}
}

func TestExtractMergesMultipleResults(t *testing.T) {
	timestamp := time.Date(2026, 1, 27, 14, 30, 0, 0, time.UTC)

	// PDC provides clearance data.
	pdcResult := &mockPDCResult{
		FlightNumber: "QF008",
		Runway:       "34L",
		SID:          "RIC6",
		Squawk:       "4302",
	}

	// FPN provides route data.
	fpnResult := &mockFPNResult{
		Origin:      "YSSY",
		Destination: "KLAX",
		Waypoints: []mockWaypoint{
			{Name: "ABARB"},
			{Name: "RIKNI"},
		},
	}

	update := ExtractEnrichment("7C6CA3", "QF008", timestamp, []registry.Result{pdcResult, fpnResult})

	if update == nil {
		t.Fatal("expected update, got nil")
	}

	// Should have PDC data.
	if update.DepartureRunway == nil || *update.DepartureRunway != "34L" {
		t.Errorf("runway = %v, want 34L", update.DepartureRunway)
	}

	// Should have FPN data.
	if update.Origin == nil || *update.Origin != "YSSY" {
		t.Errorf("origin = %v, want YSSY", update.Origin)
	}
	if len(update.Route) != 2 {
		t.Errorf("route = %v, want 2 waypoints", update.Route)
	}
}

func TestExtractNoCallsignReturnsNil(t *testing.T) {
	timestamp := time.Date(2026, 1, 27, 14, 30, 0, 0, time.UTC)

	// Result with no callsign field.
	result := &mockPDCResult{
		Origin: "YSSY",
		Runway: "34L",
	}

	// No callsign provided and none in result.
	update := ExtractEnrichment("7C6CA3", "", timestamp, []registry.Result{result})

	if update != nil {
		t.Errorf("expected nil for missing callsign, got %+v", update)
	}
}

func TestExtractNoICAOHexReturnsNil(t *testing.T) {
	timestamp := time.Date(2026, 1, 27, 14, 30, 0, 0, time.UTC)

	result := &mockPDCResult{
		FlightNumber: "QF008",
		Origin:       "YSSY",
	}

	update := ExtractEnrichment("", "QF008", timestamp, []registry.Result{result})

	if update != nil {
		t.Errorf("expected nil for missing icao_hex, got %+v", update)
	}
}

func TestNormaliseFlightNumber(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"QF008", "QF8"},
		{"QF001", "QF1"},
		{"UAL0042", "UAL42"},
		{"QF1", "QF1"},
		{"QF0", "QF0"},
		{"AAL", "AAL"},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractor.NormaliseFlightNumber(tt.input)
		if got != tt.want {
			t.Errorf("NormaliseFlightNumber(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}