package patterns

import (
	"testing"
)

func TestExtractAirports(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantOrig string
		wantDest string
	}{
		{
			name:     "CLRD TO pattern extracts destination correctly",
			text:     "DKH1380 CLRD TO ZSPD\nEXPECT DUM81A ARRIVAL ILS Z APP RWY 16L TO ZSPD",
			wantOrig: "",
			wantDest: "ZSPD",
		},
		{
			name:     "CLRD TO not overwritten by later ICAO-like words",
			text:     "DKH1380 CLRD TO ZSPD\nREPORT DETAILED STAR AND RWY RECEIVED BY LID",
			wantOrig: "",
			wantDest: "ZSPD", // Should NOT be STAR.
		},
		{
			name:     "CLEARED TO pattern",
			text:     "UAL123 CLEARED TO KJFK VIA RADAR VECTORS",
			wantOrig: "",
			wantDest: "KJFK",
		},
		{
			name:     "Route format XXXX-XXXX",
			text:     "FLIGHT KLAX-KJFK CLEARED AS FILED",
			wantOrig: "KLAX",
			wantDest: "KJFK",
		},
		// NOTE: Route strings like "KPHL DITCH LUIGI HNNAH CYUL" are tested
		// via PDC grok patterns in parser_test.go (us_regional_route format).
		{
			name:     "BCN CODE should not be extracted as destination",
			text:     "DEPARTING PHNL ADVISE ATIS AND BCN CODE",
			wantOrig: "PHNL",
			wantDest: "", // CODE should not be extracted.
		},
		{
			name:     "NO PDC AVAILABLE should not extract airports",
			text:     "NO PDC AVAILABLE",
			wantOrig: "",
			wantDest: "",
		},
		{
			name:     "FILE/FULL should not be extracted as airports",
			text:     "NO DEPARTURE CLEARANCE MESSAGE ON FILE\nREQUEST FULL ROUTE CLEARANCE",
			wantOrig: "",
			wantDest: "",
		},
		{
			name:     "Single ICAO with CLRD TO context is destination",
			text:     "CLRD TO KSFO MAINTAIN FL350",
			wantOrig: "",
			wantDest: "KSFO",
		},
		{
			name:     "Single ICAO without clearance context is origin",
			text:     "DEPARTING KORD EXPECT DELAYS",
			wantOrig: "KORD",
			wantDest: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOrig, gotDest := ExtractAirports(tt.text, nil)
			if gotOrig != tt.wantOrig {
				t.Errorf("ExtractAirports() origin = %q, want %q", gotOrig, tt.wantOrig)
			}
			if gotDest != tt.wantDest {
				t.Errorf("ExtractAirports() destination = %q, want %q", gotDest, tt.wantDest)
			}
		})
	}
}

func TestIsValidICAO(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		// Valid ICAO codes.
		{"KJFK", true},
		{"EGLL", true},
		{"YSSY", true},
		{"RJTT", true},
		{"ZSPD", true},

		// Invalid - wrong length.
		{"JFK", false},
		{"KJFKA", false},

		// Invalid - contains numbers.
		{"K1FK", false},

		// Invalid - blocklisted words.
		{"STAR", false},
		{"CODE", false},
		{"FILE", false},
		{"FULL", false},
		{"ATIS", false},
		{"WHEN", false},
		{"WITH", false},
		{"CLRD", false},
		{"DEST", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := IsValidICAO(tt.code); got != tt.want {
				t.Errorf("IsValidICAO(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}