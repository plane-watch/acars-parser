package envelope

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestADSCDecoding(t *testing.T) {
	// Test cases using real messages with valid CRCs.
	tests := []struct {
		name      string
		text      string
		wantAlt   string
		wantLat   float64
		wantLon   float64
		wantTail  string
		tolerance float64
	}{
		{
			name:     "Type 0x07 altitude FL202",
			text:     "/YEGE2YA.ADS.HL838207020BCA0C010D010F0110012AA9",
			wantTail: "HL8382",
			wantAlt:  "FL202",
		},
		{
			name:      "Type 0x08 with lat/lon",
			text:      "/YEGE2YA.ADS.HL838208010A2812B213217F20E914AC8B",
			wantTail:  "HL8382",
			tolerance: 0.1,
		},
		{
			name:      "B-number with double dot",
			text:      "/UPGCAYA.ADS..B-LQC080413274226DEF57F",
			wantTail:  "B-LQC",
			tolerance: 0.1,
		},
	}

	p := &Parser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &acars.Message{
				Label: "A6",
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

			if tt.wantTail != "" && r.Tail != tt.wantTail {
				t.Errorf("Tail = %q, want %q", r.Tail, tt.wantTail)
			}

			if tt.wantAlt != "" && r.Altitude != tt.wantAlt {
				t.Errorf("Altitude = %q, want %q", r.Altitude, tt.wantAlt)
			}

			if tt.wantLon != 0 {
				diff := r.Longitude - tt.wantLon
				if diff < 0 {
					diff = -diff
				}
				if diff > tt.tolerance {
					t.Errorf("Longitude = %f, want %f (±%f)", r.Longitude, tt.wantLon, tt.tolerance)
				}
			}

			if tt.wantLat != 0 {
				diff := r.Latitude - tt.wantLat
				if diff < 0 {
					diff = -diff
				}
				if diff > tt.tolerance {
					t.Errorf("Latitude = %f, want %f (±%f)", r.Latitude, tt.wantLat, tt.tolerance)
				}
			}
		})
	}
}

func TestEnvelopeParser(t *testing.T) {
	// Test cases using real messages with valid CRCs.
	tests := []struct {
		name     string
		label    string
		text     string
		wantTail string
		wantType string
	}{
		{
			name:     "French F-GSQC",
			label:    "AA",
			text:     "/PIKCPYA.AT1.F-GSQC214823E24092E7",
			wantTail: "F-GSQC",
			wantType: "AT1",
		},
		{
			name:     "French F-GSQN",
			label:    "AA",
			text:     "/NYCODYA.AT1.F-GSQN24C8232840DB54",
			wantTail: "F-GSQN",
			wantType: "AT1",
		},
		{
			name:     "US N-number",
			label:    "AA",
			text:     "/NYCODYA.AT1.N784AV22C823E840FBCE",
			wantTail: "N784AV",
			wantType: "AT1",
		},
		{
			name:     "Chinese B-number ADS (double dot)",
			label:    "A6",
			text:     "/UPGCAYA.ADS..B-LQC080413274226DEF57F",
			wantTail: "B-LQC",
			wantType: "ADS",
		},
		{
			name:     "Korean HL number ADS",
			label:    "A6",
			text:     "/YEGE2YA.ADS.HL838207020BCA0C010D010F0110012AA9",
			wantTail: "HL8382",
			wantType: "ADS",
		},
	}

	p := &Parser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &acars.Message{
				Label: tt.label,
				Text:  tt.text,
			}

			if !p.QuickCheck(tt.text) {
				t.Errorf("QuickCheck failed for %s", tt.text)
				return
			}

			result := p.Parse(msg)
			if result == nil {
				t.Errorf("Parse returned nil for %s", tt.text)
				return
			}

			r, ok := result.(*Result)
			if !ok {
				t.Errorf("Result is not *Result type")
				return
			}

			if r.Tail != tt.wantTail {
				t.Errorf("Tail = %q, want %q", r.Tail, tt.wantTail)
			}
			if r.MessageType != tt.wantType {
				t.Errorf("MessageType = %q, want %q", r.MessageType, tt.wantType)
			}
		})
	}
}
