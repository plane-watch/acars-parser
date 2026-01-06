package mediaadv

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParse(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		name        string
		text        string
		wantMatch   bool
		wantEstab   bool
		wantLink    string
		wantTime    string
		wantAvail   int
	}{
		{
			name:      "VHF established",
			text:      "0EV095905V",
			wantMatch: true,
			wantEstab: true,
			wantLink:  "V",
			wantTime:  "09:59:05",
			wantAvail: 1,
		},
		{
			name:      "SATCOM lost with VDL2 available",
			text:      "0LS0959482",
			wantMatch: true,
			wantEstab: false,
			wantLink:  "S",
			wantTime:  "09:59:48",
			wantAvail: 1,
		},
		{
			name:      "VDL2 established",
			text:      "0E21000102",
			wantMatch: true,
			wantEstab: true,
			wantLink:  "2",
			wantTime:  "10:00:10",
			wantAvail: 1, // Just "2" at end.
		},
		{
			name:      "With text suffix and multiple links",
			text:      "0E21031582SH/test",
			wantMatch: true,
			wantEstab: true,
			wantLink:  "2",
			wantTime:  "10:31:58",
			wantAvail: 3, // "2SH" = VDL2, SATCOM, HF.
		},
		{
			name:      "Invalid version",
			text:      "1EV095905V",
			wantMatch: false,
		},
		{
			name:      "Invalid state",
			text:      "0XV095905V",
			wantMatch: false,
		},
		{
			name:      "Too short",
			text:      "0EV09590",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &acars.Message{Text: tt.text, Label: "SA"}

			if got := parser.QuickCheck(tt.text); got != tt.wantMatch {
				t.Errorf("QuickCheck() = %v, want %v", got, tt.wantMatch)
			}

			result := parser.Parse(msg)

			if tt.wantMatch {
				if result == nil {
					t.Fatal("Parse() returned nil, want result")
				}
				r := result.(*Result)
				if r.Established != tt.wantEstab {
					t.Errorf("Established = %v, want %v", r.Established, tt.wantEstab)
				}
				if r.CurrentLink.Code != tt.wantLink {
					t.Errorf("CurrentLink.Code = %v, want %v", r.CurrentLink.Code, tt.wantLink)
				}
				if r.LinkTime != tt.wantTime {
					t.Errorf("LinkTime = %v, want %v", r.LinkTime, tt.wantTime)
				}
				if len(r.AvailableLinks) != tt.wantAvail {
					t.Errorf("AvailableLinks count = %d, want %d", len(r.AvailableLinks), tt.wantAvail)
				}
			} else {
				if result != nil {
					t.Errorf("Parse() = %v, want nil", result)
				}
			}
		})
	}
}
