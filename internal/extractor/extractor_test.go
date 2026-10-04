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
	AircraftType string  `json:"aircraft_type,omitempty"`
	AirframeID   string  `json:"airframe_id,omitempty"`
}

// loadsheetLike is a result that carries its flight number as "flight", as
// the loadsheet parser does.
type loadsheetLike struct {
	Flight string `json:"flight,omitempty"`
}

func (r *loadsheetLike) Type() string     { return "loadsheet" }
func (r *loadsheetLike) MessageID() int64 { return 0 }

// reportLike is a result that names its own flight and route, as a stored
// maintenance report does.
type reportLike struct {
	Flight      string `json:"flight"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}

func (r *reportLike) Type() string     { return "report" }
func (r *reportLike) MessageID() int64 { return 0 }

// digitsReport is a result that names its flight by number only, without
// the airline code, as an Airbus ACMS report does.
type digitsReport struct {
	FlightNumberDigits string `json:"flight_number_digits"`
	Origin             string `json:"origin"`
	Destination        string `json:"destination"`
}

func (r *digitsReport) Type() string     { return "digits_report" }
func (r *digitsReport) MessageID() int64 { return 0 }

func (r *mockResult) Type() string     { return r.typeStr }
func (r *mockResult) MessageID() int64 { return r.msgID }

func TestExtract(t *testing.T) {
	t.Run("uses only transmitted identity, never Airframes metadata", func(t *testing.T) {
		msg := &acars.Message{
			ID:            123,
			Label:         "H1",
			Tail:          "VH-OQA",
			FlightNumber:  "QFA001",
			LinkDirection: "downlink",
			FromHex:       "7C6B2D",
			// Airframes' records must be ignored.
			Airframe: &acars.Airframe{ICAO: "123456", Tail: "VH-XXX", ManufacturerModel: "A380-800", Owner: "Someone"},
			Flight:   &acars.Flight{Flight: "QF9", DepartingAirport: "YSSY", DestinationAirport: "KLAX"},
		}

		data := Extract(msg, nil)

		if data.Flight == nil {
			t.Fatal("expected flight data")
		}
		f := data.Flight
		if f.ICAOHex != "7C6B2D" || f.ICAOHexSource != HexFromLinkLayer {
			t.Errorf("ICAOHex = %q (%s), want 7C6B2D (link_layer)", f.ICAOHex, f.ICAOHexSource)
		}
		if f.Registration != "VH-OQA" {
			t.Errorf("Registration = %q, want VH-OQA", f.Registration)
		}
		if f.FlightNumber != "QFA1" {
			t.Errorf("FlightNumber = %q, want QFA1", f.FlightNumber)
		}
		if f.Origin != "" || f.Destination != "" || f.AircraftTypeRaw != "" {
			t.Errorf("route or type taken from Airframes: %q-%q, %q", f.Origin, f.Destination, f.AircraftTypeRaw)
		}
	})

	t.Run("a result's route is used only with its own flight", func(t *testing.T) {
		report := &reportLike{Flight: "THA482", Origin: "YPPH", Destination: "VTBS"}
		tests := []struct {
			name, transmitted, wantFlight, wantOrigin string
		}{
			{"same callsign", "THA482", "THA482", "YPPH"},
			{"same flight in IATA form", "TG482", "TG482", "YPPH"},
			{"no transmitted flight", "", "THA482", "YPPH"},
			// A report stored on an earlier flight and sent on this one.
			{"another flight", "THA661", "THA661", ""},
		}
		for _, tt := range tests {
			msg := &acars.Message{ID: 1, Label: "H1", Tail: "HS-TWC", FlightNumber: tt.transmitted}
			f := Extract(msg, []registry.Result{report}).Flight
			if f.FlightNumber != tt.wantFlight || f.Origin != tt.wantOrigin || (f.Destination != "") != (tt.wantOrigin != "") {
				t.Errorf("%s: flight %q, route %q-%q; want %q, origin %q", tt.name, f.FlightNumber, f.Origin, f.Destination, tt.wantFlight, tt.wantOrigin)
			}
		}
	})

	t.Run("a route named by flight digits needs a transmitted flight with them", func(t *testing.T) {
		report := &digitsReport{FlightNumberDigits: "0816", Origin: "YSSY", Destination: "YBBN"}
		tests := []struct{ transmitted, wantOrigin string }{
			{"JST816", "YSSY"},
			{"JQ816", "YSSY"},
			{"JST817", ""},
			{"", ""},
		}
		for _, tt := range tests {
			msg := &acars.Message{ID: 1, Label: "H1", Tail: "VH-VWT", FlightNumber: tt.transmitted}
			f := Extract(msg, []registry.Result{report}).Flight
			if f.Origin != tt.wantOrigin || (f.Destination != "") != (tt.wantOrigin != "") {
				t.Errorf("transmitted %q: route %q-%q, want origin %q", tt.transmitted, f.Origin, f.Destination, tt.wantOrigin)
			}
			if f.FlightNumber == "0816" || f.FlightNumber == "816" {
				t.Errorf("transmitted %q: the digits became the flight number", tt.transmitted)
			}
		}
	})

	t.Run("ADS-C airframe ID gives a transmitted address", func(t *testing.T) {
		msg := &acars.Message{ID: 1, Label: "B6", Tail: "VH-ZNA"}
		results := []registry.Result{&mockResult{typeStr: "adsc", AirframeID: "7C6CA3"}}

		f := Extract(msg, results).Flight
		if f == nil || f.ICAOHex != "7C6CA3" || f.ICAOHexSource != HexFromADSC {
			t.Fatalf("flight = %+v, want 7C6CA3 from adsc", f)
		}
	})

	t.Run("an invalid ADS-C airframe ID is not an address", func(t *testing.T) {
		for _, id := range []string{"000000", "FFFFFF", "XYZ"} {
			msg := &acars.Message{ID: 1, Label: "B6", Tail: "N1"}
			f := Extract(msg, []registry.Result{&mockResult{typeStr: "adsc", AirframeID: id}}).Flight
			if f.ICAOHex != "A00001" || f.ICAOHexSource != HexDerivedFromTail {
				t.Errorf("airframe ID %q: ICAOHex = %q (%s), want A00001 derived from the tail", id, f.ICAOHex, f.ICAOHexSource)
			}
		}
	})

	t.Run("loadsheet flight field gives the flight number", func(t *testing.T) {
		msg := &acars.Message{ID: 1, Label: "C1", Tail: "9M-MXE"}
		f := Extract(msg, []registry.Result{&loadsheetLike{Flight: "MH616"}}).Flight
		if f.FlightNumber != "MH616" {
			t.Errorf("FlightNumber = %q, want MH616", f.FlightNumber)
		}
	})

	t.Run("US N-number gives a derived address", func(t *testing.T) {
		msg := &acars.Message{ID: 1, Label: "H1", Tail: "N1"}

		f := Extract(msg, nil).Flight
		if f == nil || f.ICAOHex != "A00001" || f.ICAOHexSource != HexDerivedFromTail {
			t.Fatalf("flight = %+v, want A00001 derived from the tail", f)
		}
	})

	t.Run("a transmitted address wins over a derived one", func(t *testing.T) {
		msg := &acars.Message{ID: 1, Label: "H1", Tail: "N1", LinkDirection: "downlink", FromHex: "A12345"}

		f := Extract(msg, nil).Flight
		if f.ICAOHex != "A12345" || f.ICAOHexSource != HexFromLinkLayer {
			t.Errorf("ICAOHex = %q (%s), want A12345 (link_layer)", f.ICAOHex, f.ICAOHexSource)
		}
	})

	t.Run("aircraft type is kept raw and normalised", func(t *testing.T) {
		msg := &acars.Message{ID: 1, Label: "C1", Tail: "VH-VXA"}
		for _, tt := range []struct{ raw, want string }{
			{"B737-800", "B738"},
			{"AT7", ""}, // Ambiguous: kept raw, not normalised.
		} {
			f := Extract(msg, []registry.Result{&mockResult{typeStr: "loadsheet", AircraftType: tt.raw}}).Flight
			if f.AircraftTypeRaw != tt.raw || f.AircraftType != tt.want {
				t.Errorf("type = %q / %q, want %q / %q", f.AircraftTypeRaw, f.AircraftType, tt.raw, tt.want)
			}
		}
	})

	t.Run("routes are kept as transmitted, ICAO or IATA", func(t *testing.T) {
		msg := &acars.Message{ID: 1, Label: "C1", Tail: "VH-VXA"}
		tests := []struct {
			origin, dest, wantCodes string
		}{
			{"YSSY", "YMML", AirportCodesICAO},
			{"SYD", "MEL", AirportCodesIATA},
			{"SYD", "YMML", ""}, // Mixed: kept, but no single code type.
		}
		for _, tt := range tests {
			f := Extract(msg, []registry.Result{&mockResult{typeStr: "loadsheet", Origin: tt.origin, Destination: tt.dest}}).Flight
			if f.Origin != tt.origin || f.Destination != tt.dest || f.AirportCodes != tt.wantCodes {
				t.Errorf("route = %q-%q (%q), want %q-%q (%q)", f.Origin, f.Destination, f.AirportCodes, tt.origin, tt.dest, tt.wantCodes)
			}
		}
	})

	t.Run("extracts from parsed results", func(t *testing.T) {
		msg := &acars.Message{
			ID:    456,
			Label: "80",
			Tail:  "VH-ABC",
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
			// No transmitted tail, flight number or address.
			Airframe: &acars.Airframe{ICAO: "7C6B2D", Tail: "VH-OQA"},
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
		Tail:  "VH-ABC",
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
			t.Errorf("expected lat/lon to be unset, got %v,%v", data.Flight.Latitude, data.Flight.Longitude)
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
		{"WHEN", false},  // Blocked word
		{"WITH", false},  // Blocked word
		{"XYZ", false},   // Too short
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

func TestSameFlightNumber(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"THA482", "TG482", true},
		{"QFA001", "QF1", true},
		{"BAW990G", "BA990G", true},
		{"BAW990G", "BAW990", false},
		{"UAL2443", "UAL243", false},
		{"UAL", "UAL", false},
		{"", "QF1", false},
	}
	for _, tt := range tests {
		if got := sameFlightNumber(tt.a, tt.b); got != tt.want {
			t.Errorf("sameFlightNumber(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
