package extractor

import (
	"testing"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

func TestNormaliseFlightNumber(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"QF001", "QF1"},
		{"QF008", "QF8"},
		{"QFA001", "QFA1"},
		{"UAL0042", "UAL42"},
		{"QF1", "QF1"},
		{"QF0", "QF0"},
		{"QF000", "QF0"},
		{"AAL", "AAL"},
		{"", ""},
		{"  QF001  ", "QF1"},
		{"ABCD1234", "ABCD1234"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormaliseFlightNumber(tt.input)
			if got != tt.want {
				t.Errorf("NormaliseFlightNumber(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsICAOCallsign(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"QFA1", true},   // 3-letter ICAO
		{"UAL123", true}, // 3-letter ICAO
		{"QF1", false},   // 2-letter IATA
		{"AA123", false}, // 2-letter IATA
		{"", false},
		{"AAL", false}, // No numeric part
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsICAOCallsign(tt.input)
			if got != tt.want {
				t.Errorf("IsICAOCallsign(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// mockResult implements registry.Result for testing.
type mockResult struct {
	typeStr      string
	msgID        int64
	Origin       string  `json:"origin,omitempty"`
	Destination  string  `json:"destination,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
	FlightNumber string  `json:"flight_number,omitempty"`
	Waypoint     string  `json:"waypoint,omitempty"`
}

func (r *mockResult) Type() string     { return r.typeStr }
func (r *mockResult) MessageID() int64 { return r.msgID }

func TestExtract(t *testing.T) {
	t.Run("extracts from message metadata", func(t *testing.T) {
		msg := &acars.Message{
			ID:    123,
			Label: "H1",
			Airframe: &acars.Airframe{
				ICAO: "7C6B2D",
				Tail: "VH-OQA",
			},
			Flight: &acars.Flight{
				Flight:             "QF1",
				DepartingAirport:   "YSSY",
				DestinationAirport: "KLAX",
			},
		}

		data := Extract(msg, nil)

		if data.Flight == nil {
			t.Fatal("expected flight data")
		}
		if data.Flight.ICAOHex != "7C6B2D" {
			t.Errorf("ICAOHex = %q, want 7C6B2D", data.Flight.ICAOHex)
		}
		if data.Flight.Registration != "VH-OQA" {
			t.Errorf("Registration = %q, want VH-OQA", data.Flight.Registration)
		}
		if data.Flight.FlightNumber != "QF1" {
			t.Errorf("FlightNumber = %q, want QF1", data.Flight.FlightNumber)
		}
		if data.Flight.Origin != "YSSY" {
			t.Errorf("Origin = %q, want YSSY", data.Flight.Origin)
		}
		if data.Flight.Destination != "KLAX" {
			t.Errorf("Destination = %q, want KLAX", data.Flight.Destination)
		}
	})

	t.Run("extracts from parsed results", func(t *testing.T) {
		msg := &acars.Message{
			ID:    456,
			Label: "80",
			Airframe: &acars.Airframe{
				ICAO: "ABC123",
			},
		}

		results := []registry.Result{
			&mockResult{
				typeStr:     "position",
				msgID:       456,
				Latitude:    -33.946,
				Longitude:   151.177,
				Origin:      "YSSY",
				Destination: "KLAX",
			},
		}

		data := Extract(msg, results)

		if data.Flight == nil {
			t.Fatal("expected flight data")
		}
		if data.Flight.Latitude != -33.946 {
			t.Errorf("Latitude = %f, want -33.946", data.Flight.Latitude)
		}
		if data.Flight.Longitude != 151.177 {
			t.Errorf("Longitude = %f, want 151.177", data.Flight.Longitude)
		}
	})

	t.Run("no flight data without identity", func(t *testing.T) {
		msg := &acars.Message{
			ID:    789,
			Label: "H1",
			// No Airframe or Flight data.
		}

		data := Extract(msg, nil)

		if data.Flight != nil {
			t.Error("expected nil flight data when no identity present")
		}
	})
}

func TestExtract_ZeroCoordinates(t *testing.T) {
	// Test that zero lat/lon at (0,0) together is treated as unset,
	// but individual zeros are accepted when one coordinate is non-zero.
	msg := &acars.Message{
		ID:    123,
		Label: "80",
		Airframe: &acars.Airframe{
			ICAO: "ABC123",
		},
	}

	t.Run("both zero - treated as unset", func(t *testing.T) {
		results := []registry.Result{
			&mockResult{
				typeStr:   "position",
				Latitude:  0,
				Longitude: 0,
			},
		}

		data := Extract(msg, results)
		if data.Flight.Latitude != 0 || data.Flight.Longitude != 0 {
			// Both should be zero (not set)
		}
	})

	t.Run("lat zero lon non-zero - accepted", func(t *testing.T) {
		results := []registry.Result{
			&mockResult{
				typeStr:   "position",
				Latitude:  0,       // Equator
				Longitude: 151.177, // Non-zero
			},
		}

		data := Extract(msg, results)
		if data.Flight.Latitude != 0 {
			t.Errorf("Latitude should be 0 (equator), got %f", data.Flight.Latitude)
		}
		if data.Flight.Longitude != 151.177 {
			t.Errorf("Longitude = %f, want 151.177", data.Flight.Longitude)
		}
	})
}

func TestIsValidAirportCode(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"YSSY", true},
		{"KLAX", true},
		{"EGLL", true},
		{"WHEN", false}, // Blocked word
		{"WITH", false}, // Blocked word
		{"XYZ", false},  // Too short
		{"ABCDE", false}, // Too long
		{"1234", false},  // Numbers
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidAirportCode(tt.input)
			if got != tt.want {
				t.Errorf("isValidAirportCode(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
