package label5l

import (
	"testing"

	"acars_parser/internal/acars"
)

// TestParseDesignator checks that the first field is reported as the
// designator with its flight number, not as a callsign (see the package
// documentation).
func TestParseDesignator(t *testing.T) {
	tests := []struct {
		text, designator, digits string
	}{
		{"JST0372,.VH-8NP,CCK,YPCC,PER,YPPH,048106,06OCT26,1126,1137,1501,1507", "JST0372", "0372"},
		{"JST1701,.VH-UVQ,ZNE,YNWN,PER,YPPH,056032,08OCT26,2357,0006,0135,0141", "JST1701", "1701"},
		{"JQ123A,.VH-VQE,SYD,YSSY,MEL,YMML", "JQ123A", "123"},
		// Not a flight: no flight number.
		{"ABCDEFGH,.VH-VQE,SYD,YSSY,MEL,YMML", "ABCDEFGH", ""},
	}
	p := &Parser{}
	for _, tt := range tests {
		r, ok := p.Parse(&acars.Message{ID: 1, Label: "5L", Text: tt.text}).(*Result)
		if !ok || r == nil {
			t.Fatalf("%s: not parsed", tt.text)
		}
		if r.Designator != tt.designator || r.FlightNumberDigits != tt.digits {
			t.Errorf("%s: designator %q, digits %q; want %q, %q", tt.text, r.Designator, r.FlightNumberDigits, tt.designator, tt.digits)
		}
	}
}
