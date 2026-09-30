package adsc

import (
	"math"
	"testing"

	"acars_parser/internal/acars"
)

func TestADSCParser(t *testing.T) {
	// Test cases using real messages with valid CRCs.
	// Altitudes use 4 ft per bit (ICAO GOLD 2nd ed.; libacars la_adsc_altitude_parse;
	// JAERO alt_scaller). Each value is consistent with the aircraft's own predicted
	// route altitudes in the same message.
	tests := []struct {
		name        string
		text        string
		wantType    string
		wantReg     string
		wantStation string
		wantLat     float64
		wantLon     float64
		wantAlt     int
		tolerance   float64
	}{
		{
			name:        "Basic report (F-GXLI)",
			text:        "/XYTGL7X.ADS.F-GXLI0725BFC82D8D46BC46CC1D0D25B0182C2CC745807725965029EF880A40B791",
			wantType:    "basic",
			wantReg:     "F-GXLI",
			wantStation: "XYTGL7X",
			wantLat:     53.08,
			wantLon:     8.01,
			wantAlt:     27584,
			tolerance:   0.1,
		},
		{
			name:        "Basic report (G-ZBKO)",
			text:        "/QUKAXBA.ADS.G-ZBKO072495A7EE7786F6A4D21F7A5D",
			wantType:    "basic",
			wantReg:     "G-ZBKO",
			wantStation: "QUKAXBA",
			wantLat:     51.45,
			wantLon:     -3.08,
			wantAlt:     28520,
			tolerance:   0.1,
		},
		{
			name:        "Basic report with flight prefix (N760GT)",
			text:        "F67A5Y0700/FUKJJYA.ADS.N760GT0724F34BA86989C3C98D1D17231AE3868D09C408AB0D24B2D3A348C9C4013F23B1DB9071C9C4000E54A0E140040F54F1A0C004D45D",
			wantType:    "basic",
			wantReg:     "N760GT",
			wantStation: "FUKJJYA",
			wantLat:     51.96,
			wantLon:     164.60,
			wantAlt:     39996,
			tolerance:   0.1,
		},
		{
			name:        "Basic report (F-GXLO)",
			text:        "/XYTGL7X.ADS.F-GXLO0725A2E02967884D24581D0D25665826E6484D0110254F0025F2884D00815F",
			wantType:    "basic",
			wantReg:     "F-GXLO",
			wantStation: "XYTGL7X",
			wantLat:     52.93,
			wantLon:     7.28,
			wantAlt:     34000,
			tolerance:   0.1,
		},
	}

	p := &Parser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &acars.Message{
				Label: "B6",
				Text:  tt.text,
			}

			result := p.Parse(msg)
			if result == nil {
				t.Fatalf("Parse returned nil")
			}

			r, ok := result.(*Result)
			if !ok {
				t.Fatalf("Result is not *Result type")
			}

			if r.MessageType != tt.wantType {
				t.Errorf("MessageType = %q, want %q", r.MessageType, tt.wantType)
			}

			if r.Registration != tt.wantReg {
				t.Errorf("Registration = %q, want %q", r.Registration, tt.wantReg)
			}

			if r.GroundStation != tt.wantStation {
				t.Errorf("GroundStation = %q, want %q", r.GroundStation, tt.wantStation)
			}

			if tt.wantLat != 0 {
				if math.Abs(r.Latitude-tt.wantLat) > tt.tolerance {
					t.Errorf("Latitude = %f, want %f (±%f)", r.Latitude, tt.wantLat, tt.tolerance)
				}
			}

			if tt.wantLon != 0 {
				if math.Abs(r.Longitude-tt.wantLon) > tt.tolerance {
					t.Errorf("Longitude = %f, want %f (±%f)", r.Longitude, tt.wantLon, tt.tolerance)
				}
			}

			if tt.wantAlt != 0 {
				if r.Altitude < tt.wantAlt-100 || r.Altitude > tt.wantAlt+100 {
					t.Errorf("Altitude = %d, want %d (±100)", r.Altitude, tt.wantAlt)
				}
			}
		})
	}
}

func TestDecodeCoordinate(t *testing.T) {
	// 21-bit coordinate encoding: MSB weight is 90°, range is approximately ±180°.
	// Value 0x080000 (bit 19 set) = 90°, 0x100000 (bit 20 set) = -180° (sign bit).
	tests := []struct {
		name      string
		raw       uint32
		want      float64
		tolerance float64
	}{
		{"Zero", 0, 0, 0.001},
		{"Positive 90°", 0x080000, 90.0, 0.01},
		{"Negative 90°", 0x180000, -90.0, 0.01},
		{"Max positive ~180°", 0x0FFFFF, 180.0, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeCoordinate(tt.raw)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("decodeCoordinate(0x%X) = %f, want %f", tt.raw, got, tt.want)
			}
		})
	}
}

// TestADSCAirRefAndPredictedRoute checks the Mach and predicted route scaling
// against a real report in which the basic, predicted route and air reference
// groups must agree: N760GT cruising at FL400.
func TestADSCAirRefAndPredictedRoute(t *testing.T) {
	text := "F67A5Y0700/FUKJJYA.ADS.N760GT0724F34BA86989C3C98D1D17231AE3868D09C408AB0D24B2D3A348C9C4013F23B1DB9071C9C4000E54A0E140040F54F1A0C004D45D"

	result := (&Parser{}).Parse(&acars.Message{Label: "B6", Text: text})
	if result == nil {
		t.Fatal("Parse returned nil")
	}
	r := result.(*Result)

	if r.AirRef == nil {
		t.Fatal("AirRef is nil")
	}
	// Mach is 0.0005 per bit (libacars la_adsc_speed_parse then /1000; JAERO machspeed_scaller).
	if math.Abs(r.AirRef.Mach-0.8335) > 0.001 {
		t.Errorf("Mach = %.4f, want 0.8335", r.AirRef.Mach)
	}

	if r.PredictedRoute == nil || r.PredictedRoute.NextWaypoint == nil || r.PredictedRoute.NextNextWaypoint == nil {
		t.Fatal("PredictedRoute waypoints are missing")
	}
	if got := r.PredictedRoute.NextWaypoint.Altitude; got != 40000 {
		t.Errorf("NextWaypoint.Altitude = %d, want 40000", got)
	}
	if got := r.PredictedRoute.NextNextWaypoint.Altitude; got != 40000 {
		t.Errorf("NextNextWaypoint.Altitude = %d, want 40000", got)
	}
}

// TestParseTagNACKLength checks that reason codes 1, 2 and 7 carry an extended
// data byte (libacars la_adsc_nack_parse), so the following tag is not misaligned.
func TestParseTagNACKLength(t *testing.T) {
	tests := []struct {
		name   string
		reason byte
		want   int
	}{
		{"reason 1 has extended data", 1, 3},
		{"reason 2 has extended data", 2, 3},
		{"reason 7 has extended data", 7, 3},
		{"reason 3 has no extended data", 3, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Contract number, reason code, extended data byte, then a trailing byte.
			data := []byte{0x05, tt.reason, 0x09, 0x03}
			got := parseTag(&Result{}, 0x04, data, true)
			if got != tt.want {
				t.Errorf("parseTag consumed %d bytes, want %d", got, tt.want)
			}
		})
	}

	t.Run("truncated extended data is an error", func(t *testing.T) {
		if got := parseTag(&Result{}, 0x04, []byte{0x05, 0x01}, true); got != -1 {
			t.Errorf("parseTag consumed %d bytes, want -1", got)
		}
	})
}

// TestParseTagNoncomplianceLength checks the noncompliance notification length:
// each group is 2 bytes plus one byte per two non-compliant parameters, unless the
// group is flagged as unrecognised (0x80) or wholly unavailable (0x40)
// (libacars la_adsc_noncomp_group_parse).
func TestParseTagNoncomplianceLength(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{
			name: "no groups",
			data: []byte{0x01, 0x00},
			want: 2,
		},
		{
			name: "one group with three parameters",
			// Contract, group count, group tag, flags/param_cnt=3, two nibble bytes.
			data: []byte{0x01, 0x01, 0x0D, 0x03, 0x12, 0x30},
			want: 6,
		},
		{
			name: "unrecognised group has no parameter bytes",
			data: []byte{0x01, 0x01, 0x0E, 0x80},
			want: 4,
		},
		{
			name: "whole group unavailable has no parameter bytes",
			data: []byte{0x01, 0x01, 0x0E, 0x40},
			want: 4,
		},
		{
			name: "two groups of mixed kinds",
			data: []byte{0x01, 0x02, 0x0D, 0x03, 0x12, 0x30, 0x0E, 0x80},
			want: 8,
		},
		{
			name: "truncated parameter bytes is an error",
			data: []byte{0x01, 0x01, 0x0D, 0x03, 0x12},
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTag(&Result{}, 0x05, tt.data, true)
			if got != tt.want {
				t.Errorf("parseTag consumed %d bytes, want %d", got, tt.want)
			}
		})
	}
}
