package envelope

import (
	"encoding/json"
	"testing"

	"acars_parser/internal/acars"
)

// TestADSCRequestHasNoPosition checks that an ADS-C message on label A6 is
// reported without a position or altitude. A6 carries contract requests from
// the ground to the aircraft (B6 carries the aircraft's reports), so its
// payload holds no position: "07 02 0B CA 0C 01 ..." is a periodic contract
// request (contract 2, then the data groups requested), and reading 0xCA as
// FL202 would invent an altitude. The messages are real, with valid CRCs.
func TestADSCRequestHasNoPosition(t *testing.T) {
	tests := []struct{ text, wantTail string }{
		{"/YEGE2YA.ADS.HL838207020BCA0C010D010F0110012AA9", "HL8382"},
		{"/YEGE2YA.ADS.HL838208010A2812B213217F20E914AC8B", "HL8382"},
		{"/UPGCAYA.ADS..B-LQC080413274226DEF57F", "B-LQC"},
	}
	for _, tt := range tests {
		r, ok := (&Parser{}).Parse(&acars.Message{Label: "A6", Text: tt.text}).(*Result)
		if !ok {
			t.Fatalf("%s: Parse returned no result", tt.text)
		}
		if r.Tail != tt.wantTail || r.MessageType != "ADS" {
			t.Errorf("%s: tail %q, type %q; want %q, ADS", tt.text, r.Tail, r.MessageType, tt.wantTail)
		}
		b, _ := json.Marshal(r)
		var m map[string]interface{}
		_ = json.Unmarshal(b, &m)
		for _, k := range []string{"latitude", "longitude", "altitude"} {
			if _, ok := m[k]; ok {
				t.Errorf("%s: result has %s = %v", tt.text, k, m[k])
			}
		}
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
