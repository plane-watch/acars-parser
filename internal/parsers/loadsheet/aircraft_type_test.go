package loadsheet

import (
	"strings"
	"testing"

	"acars_parser/internal/acars"
)

// Real loadsheets (messages 9478844 and 2532117) with personal names redacted.
const (
	labelledTypeLoadsheet = ".DPCCAMH 181122\n\tAGM\n\tAN 9M-MXE/FI MH0000/MA 795I\n\t-  LOADSHEET FINAL 1922 EDNO1\n\tMH616/18      18JAN26\n\tSIN KUL 9M-MXE   2/4\n\tZFW 53749  MAX 62731 \n\tTOF 8271\n\tTOW 62020  MAX 79015 \n\tTIF 2079\n\tLAW 59941  MAX 66360   L\n\tUNDLD 6419\n\tPAX/9/121 TTL 130\n\tPAX 130 PLUS 0\n\tDOI      41.6\n\tDLI      32.6\n\tMACZFW   17.1\n\tMACTOW   20.6\n\tPREPARED BY REDACTED/REDACTED \n\tLICENCE K8-119 E30NOV27\n\tAIRCRAFT TYPE : B737-800               \n\tPIC NAME: CAPT REDACTED \n\tSIGN & LIC NO.: ......................"

	dhlLoadsheet = ".DDLIRXA 062028\n\tAGM\n\tAN D-ACVG/MA 333A\n\t-  LOADSHEET  QY549   REF 2F6S5\n\t--- FOR INFORMATION ONLY ---\n\tFLT  HKG-BAH  A333-BCS3 / DACVG\n\tDTE  2026-01-06\n\tISSD 20260106T2028\n\tBY REDACTED (H/HKG\n\t\n\t      WEIGHT  INDEX  ALL WGHTS KG \n\tDOW   113142  63,32\n\tACM 1     95  -1,01\n\tPLD    51000  43,61\n\tZFW   164237 105,92 175000 Max\n\tTOF    68360   2,39\n\tTOW   232597 108,31 233000 Max\n\tTRIP   58416  10,17\n\tLW    174181  98,15 187000 Max\n\t\n\tEND LOADSHEET\n\t"
)

func TestParseAircraftType(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"labelled AIRCRAFT TYPE field", labelledTypeLoadsheet, "B737-800"},
		{"type and configuration on the DHL flight line", dhlLoadsheet, "A333-BCS3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "C1", Text: tt.text})
			if got == nil {
				t.Fatal("Parse returned nil")
			}
			if r := got.(*Result); r.AircraftType != tt.want {
				t.Errorf("AircraftType = %q, want %q", r.AircraftType, tt.want)
			}
		})
	}
}

// TestParseAircraftTypeLineIsStrict checks that the explicit type line is read
// only from a complete "AIRCRAFT TYPE :" line.
func TestParseAircraftTypeLineIsStrict(t *testing.T) {
	tests := []struct {
		name, line string
	}{
		{"empty value does not take the next line", "\tAIRCRAFT TYPE :      \n\tSIGN & LIC NO.: ......................"},
		{"a different field ending in AIRCRAFT TYPE", "\tPREVIOUS AIRCRAFT TYPE : B738"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := strings.Replace(labelledTypeLoadsheet, "\tAIRCRAFT TYPE : B737-800               ", tt.line, 1)
			got := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "C1", Text: text})
			if got == nil {
				t.Fatal("Parse returned nil")
			}
			if r := got.(*Result); r.AircraftType != "" {
				t.Errorf("AircraftType = %q, want empty", r.AircraftType)
			}
		})
	}
}
