package swareport

import (
	"testing"

	"acars_parser/internal/acars"
)

// The reports are real Southwest reports (label H1) from the January 2026
// corpus, cut after the header's first data field.
func TestParse(t *testing.T) {
	tests := []struct {
		tail, text string
		want       Result
	}{
		{"N228WN", "05201,0228,B737-700,260118,WN1301,KDTW,KBWI,0947,SW2501\r\n11.49.58,DC,0999,10743,261.0,.483,-13.5",
			Result{Report: "05201", AircraftType: "B737-700", Date: "260118", Flight: "WN1301", Origin: "KDTW", Destination: "KBWI", SystemID: "SW2501"}},
		{"N8767M", "82740,8767,B737-8MAX,260120,WN0428,KBDL,KMDW,0239,SM2502\r\n12.45.00,AP,2419,00514,134.3",
			Result{Report: "82740", AircraftType: "B737-8MAX", Date: "260120", Flight: "WN0428", Origin: "KBDL", Destination: "KMDW", SystemID: "SM2502"}},
		{"N500WR", "72740,8636,B737-800,260111,WN0374,KAUS,KPHX,1071,SW2501\r\n12.22.51,CR,1661,40002,235.7",
			Result{Report: "72740", AircraftType: "B737-800", Date: "260111", Flight: "WN0374", Origin: "KAUS", Destination: "KPHX", SystemID: "SW2501"}},
	}
	for _, tt := range tests {
		r, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: tt.tail, Text: tt.text}).(*Result)
		if !ok {
			t.Fatalf("%s: Parse returned no result", tt.tail)
		}
		r.MsgID, r.Timestamp = 0, ""
		if *r != tt.want {
			t.Errorf("%s:\n got %+v\nwant %+v", tt.tail, *r, tt.want)
		}
	}
}

func TestParseRejectsOtherText(t *testing.T) {
	for _, text := range []string{
		// The ++ flight data reports belong to the trajectory parser.
		"++86501,N8960L,B7378MAX,260109,WN0712,KABQ,KAUS,0196,SMX34-2502-F320\r\n1\r\n",
		// Not an ICAO route.
		"72740,8636,B737-800,260111,WN0374,KAUS,XXXX,1071,SW2501\r\n",
		// Not a type.
		"72740,8636,FOO,260111,WN0374,KAUS,KPHX,1071,SW2501\r\n",
		"POSN31211W097249,ACT,052904",
		// Short enough to need a bounds check in the quick check.
		"72740,8636,B7",
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Text: text}); r != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, r)
		}
	}
}
