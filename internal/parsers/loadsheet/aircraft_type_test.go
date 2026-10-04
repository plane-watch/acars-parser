package loadsheet

import (
	"strings"
	"testing"

	"acars_parser/internal/acars"
)

// Real loadsheets (messages 9478844, 2532117 and 10551395, the last cut
// after MACTOW) with personal names redacted.
const (
	labelledTypeLoadsheet = ".DPCCAMH 181122\n\tAGM\n\tAN 9M-MXE/FI MH0000/MA 795I\n\t-  LOADSHEET FINAL 1922 EDNO1\n\tMH616/18      18JAN26\n\tSIN KUL 9M-MXE   2/4\n\tZFW 53749  MAX 62731 \n\tTOF 8271\n\tTOW 62020  MAX 79015 \n\tTIF 2079\n\tLAW 59941  MAX 66360   L\n\tUNDLD 6419\n\tPAX/9/121 TTL 130\n\tPAX 130 PLUS 0\n\tDOI      41.6\n\tDLI      32.6\n\tMACZFW   17.1\n\tMACTOW   20.6\n\tPREPARED BY REDACTED/REDACTED \n\tLICENCE K8-119 E30NOV27\n\tAIRCRAFT TYPE : B737-800               \n\tPIC NAME: CAPT REDACTED \n\tSIGN & LIC NO.: ......................"

	etihadLoadsheet = "QUAUHASEY~1WAB01191942\n\tLOADSHEET FINAL   001 0242\n\tEY401/19 20JAN26 B78X\n\tBKK AUH A6BMA    2/11\n\tZFW 173124 MAX 192776  L\n\tTOF 45200\n\tTOW 218324 MAX 250836\n\tTIF 37700\n\tLAW 180624 MAX 201848\n\tUNDLD 19652\n\tPAX/31/264 TTL 295\n\tPAX 295 PLUS 0\n\tBI      430.7\n\tDOI     443.6\n\tLIZFW   621.0\n\tLITOW   672.8\n\tMACZFW   26.4\n\tMACTOW   27.2\n"

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

// TestParseTypeAfterTheFlightDate checks the standard format with the
// aircraft type after the flight date, as Etihad sends it (label RA).
func TestParseTypeAfterTheFlightDate(t *testing.T) {
	got := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "RA", Text: etihadLoadsheet})
	if got == nil {
		t.Fatal("Parse returned nil")
	}
	r := got.(*Result)
	if r.AircraftType != "B78X" || r.Tail != "A6BMA" || r.Flight != "EY401" || r.Origin != "BKK" || r.Destination != "AUH" || r.TOW != 218324 {
		t.Errorf("got type %q, tail %q, flight %q, route %q-%q, TOW %d", r.AircraftType, r.Tail, r.Flight, r.Origin, r.Destination, r.TOW)
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

// TestParseTokenAfterDateOnlyInEtihadLayout checks that a token after the
// flight date is taken as the type only in Etihad's layout (with the
// edition before the time): elsewhere it may be something else, such as a
// gate ("A10", which is also an ICAO designator).
func TestParseTokenAfterDateOnlyInEtihadLayout(t *testing.T) {
	text := "LOADSHEET FINAL 1736 EDNO1\nLX1376/21     21JAN26 A10\nZRH WRO HB-AZH   2/3\nZFW 39754  MAX 46700\nTOF 4800\nTOW 44554  MAX 54000\nTIF 2000\nLAW 42554  MAX 49050   L\nPAX/6/59 TTL 65\n"
	if r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "RA", Text: text}).(*Result); ok && r.AircraftType != "" {
		t.Errorf("AircraftType = %q, want none", r.AircraftType)
	}
	r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "3S", Text: strings.Replace(etihadLoadsheet, "B78X", "A320", 1)}).(*Result)
	if !ok || r.AircraftType != "A320" || r.FormatName != "etihad" {
		t.Errorf("Etihad on label 3S: got %+v", r)
	}
}

// TestParseEtihadDoesNotReachIntoLaterText checks that fields missing from
// the Etihad loadsheet are not taken from text after it.
func TestParseEtihadDoesNotReachIntoLaterText(t *testing.T) {
	text := "LOADSHEET FINAL 001 0242\nEY401/19 20JAN26 B78X\nBKK AUH A6BMA 2/11\nZFW 173124 MAX 192776\nTOF 45200\nTOW 218324 MAX 254011\nEND LOADSHEET\nPREVIOUS FLIGHT DATA\nLAW 99999 MAX 100000\nPAX/9/99 TTL 108\n"
	r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "3S", Text: text}).(*Result)
	if ok && (r.LAW == 99999 || r.PAX == 108) {
		t.Errorf("took LAW %d and PAX %d from text after the loadsheet", r.LAW, r.PAX)
	}
}

// TestParseQatarTypeLine checks Qatar's loadsheets, which give the type on
// its own line between the DOW and the service weight adjustment (a real
// message from A7-BEM, cut down).
func TestParseQatarTypeLine(t *testing.T) {
	text := "QUDOHEOQR~1DIS01010101\n\t\n\tLOADSHEET PRELIM 1625\n\tQR400/04 04JAN26\n\tDOH AMM A7-BEM 2/14\n\t*************************\n\tZFW 200597  MAX 237682  L\n\t*************************\n\tTOF  29085\n\t*************************\n\tTOW 229682  MAX 351534\n\t*************************\n\tTIF  18106\n\tLAW 211576  MAX 251290\n\tUNDLD  37085\n\tPAX/18/194 TTL 215\n\tPAX 212 PLUS 3\n\tLITOW       487.4\n\tSI DOI 477.4\n\tDOW 175738\n\t777-300ER\n\tSERVICE WEIGHT ADJUSTMENT WEIGHT/INDEX\n\tADD\n"
	r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "RA", Text: text}).(*Result)
	if !ok || r.AircraftType != "777-300ER" || r.Tail != "A7-BEM" {
		t.Errorf("got %+v", r)
	}
	// A line between other fields is not taken as the type.
	other := strings.Replace(text, "\tSERVICE WEIGHT ADJUSTMENT", "\tSOMETHING ELSE", 1)
	if r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "RA", Text: other}).(*Result); ok && r.AircraftType != "" {
		t.Errorf("type %q taken from an unanchored line", r.AircraftType)
	}
}
