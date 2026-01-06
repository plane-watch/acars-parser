package adsc

import (
	"math"
	"testing"

	"acars_parser/internal/acars"
)

func TestADSCParser(t *testing.T) {
	// Test cases using real messages with valid CRCs.
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
			wantAlt:     13792,
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
			wantAlt:     14260,
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
			wantAlt:     19996,
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
			wantAlt:     17000,
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