package takeoff

import (
	"strings"
	"testing"

	"acars_parser/internal/acars"
)

func TestParser_Parse(t *testing.T) {
	parser := &Parser{}

	text := `TAKEOFF DATA
FLT      RLS/WB     TIME
0813      10/ 1    1808Z
WIND      OAT C      QNH
000/00       0     30.15
------------------------
GTOW /CG             PAX
409.3/25.1           226
FUEL               CARGO
 82.9              11878
ZFW  /CG
326.4/26.6         FINAL

REMARKS
MEL/CDL WT PENALTY 1213

     KPDX 10R NOTES
NOTAMS A1117/25 A1124/25

LENGTH    KPDX     SHIFT
11000     10R          0
`

	msg := &acars.Message{ID: 12345, Label: "RA", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	tr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}

	if tr.Time != "1808Z" {
		t.Errorf("Time = %q, want %q", tr.Time, "1808Z")
	}
	if tr.Wind != "000/00" {
		t.Errorf("Wind = %q, want %q", tr.Wind, "000/00")
	}
	if tr.QNH != 30.15 {
		t.Errorf("QNH = %f, want 30.15", tr.QNH)
	}
	if tr.GTOW != 409.3 {
		t.Errorf("GTOW = %f, want 409.3", tr.GTOW)
	}
	if tr.CG != 25.1 {
		t.Errorf("CG = %f, want 25.1", tr.CG)
	}
	if tr.PAX != 226 {
		t.Errorf("PAX = %d, want 226", tr.PAX)
	}
	if tr.Fuel != 82.9 {
		t.Errorf("Fuel = %f, want 82.9", tr.Fuel)
	}
	if tr.Cargo != 11878 {
		t.Errorf("Cargo = %d, want 11878", tr.Cargo)
	}
	if tr.ZFW != 326.4 {
		t.Errorf("ZFW = %f, want 326.4", tr.ZFW)
	}

	if len(tr.Runways) != 1 {
		t.Fatalf("expected 1 runway, got %d", len(tr.Runways))
	}
	if tr.Runways[0].Airport != "KPDX" {
		t.Errorf("Runway airport = %q, want %q", tr.Runways[0].Airport, "KPDX")
	}
	if tr.Runways[0].Runway != "10R" {
		t.Errorf("Runway = %q, want %q", tr.Runways[0].Runway, "10R")
	}
	if tr.Runways[0].Length != 11000 {
		t.Errorf("Runway length = %d, want 11000", tr.Runways[0].Length)
	}
}

func TestParser_SimpleFormat(t *testing.T) {
	parser := &Parser{}

	text := `TAKEOFF DATA
** PART 01 OF 01 **
************************
T/O SFO 01R *T PROC*
8650 FT
A320-232 V2527-A5
TEMP 10C       ALT 30.09
WIND 346/0 MAG
`

	msg := &acars.Message{ID: 12345, Label: "RA", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	tr := result.(*Result)

	if tr.AircraftType != "A320-232" {
		t.Errorf("AircraftType = %q, want %q", tr.AircraftType, "A320-232")
	}
	if tr.EngineType != "V2527-A5" {
		t.Errorf("EngineType = %q, want %q", tr.EngineType, "V2527-A5")
	}

	if len(tr.Runways) != 1 {
		t.Fatalf("expected 1 runway, got %d", len(tr.Runways))
	}
	if tr.Runways[0].Airport != "SFO" {
		t.Errorf("Airport = %q, want %q", tr.Runways[0].Airport, "SFO")
	}
	if tr.Runways[0].Runway != "01R" {
		t.Errorf("Runway = %q, want %q", tr.Runways[0].Runway, "01R")
	}
}

func TestParser_QuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"TAKEOFF DATA", true},
		{"T/O DATA", true},
		{"Some other message", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

// TestParser_UnitedModelLine checks that the model line of United's takeoff
// data, which gives the Boeing model without the "B" prefix, is captured.
func TestParser_UnitedModelLine(t *testing.T) {
	text := "QUNDCULUA~1TAKEOFF DATA\n** PART 01 OF 01 **\n************************\n" +
		"T/O FSD 21 \n8999 FT\n737-900ER CFM56-7B27\nTEMP 10C       ALT 30.33\nWIND 355/0 MAG\n"
	result := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "RA", Text: text})
	tr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}
	if tr.AircraftType != "737-900ER" || tr.EngineType != "CFM56-7B27" {
		t.Errorf("AircraftType, EngineType = %q, %q, want %q, %q", tr.AircraftType, tr.EngineType, "737-900ER", "CFM56-7B27")
	}
}

// envoyTakeoffData is a real Envoy takeoff data uplink (message 10450337),
// cut after the second runway block.
const envoyTakeoffData = ".PERFFMQ 191523\nAGM\nAN N337MR/FI MQ3845\n-  TAKEOFF DATA\n3845/19  KORD-KVPS 1523Z\n" +
	"337/N337MR   DISP RLS  1\nWX 284/16   -18C   A3012\n------------------------\n BOW 50867      25.7\n" +
	" ZFW 56664  12  16.5  30\n FOB 11200\nGTOW 67164  12  13.8  29\n FBO  6342\nPLDW 60822\n" +
	"REMARKS\nAUTO CLOSEOUT\n \nKORD 22L      TORA  8075\nEO-D224        FRA  1672\n      T/O-2 ECS ON      \n" +
	"A/I  OFF          V1 129\nAT   32           VR 129\nN1   80.5         V2 133\nMRTW 82685/O     VFS 183\n" +
	"MTOW 81299      FLAP   2\nGTOW 67164      STAB 4.8\n \nKORD 28RZ     TORA  9750\nEO-D273        FRA  1672\n" +
	"A/I  OFF          V1 131\n"

// TestParser_EnvoyLayout checks the Envoy layout: the flight, route and
// tail from its header lines, the weather line, and one entry per runway
// block with its own V1.
func TestParser_EnvoyLayout(t *testing.T) {
	result := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "C1", Text: envoyTakeoffData})
	tr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}
	if tr.FlightNumber != "MQ3845" || tr.Origin != "KORD" || tr.Destination != "KVPS" || tr.Tail != "N337MR" {
		t.Errorf("flight, origin, destination, tail = %q, %q, %q, %q, want MQ3845, KORD, KVPS, N337MR",
			tr.FlightNumber, tr.Origin, tr.Destination, tr.Tail)
	}
	if tr.Time != "1523Z" || tr.Wind != "284/16" || tr.OAT != -18 || tr.QNH != 30.12 || tr.GTOW != 67164 {
		t.Errorf("time, wind, OAT, QNH, GTOW = %q, %q, %d, %v, %v, want 1523Z, 284/16, -18, 30.12, 67164",
			tr.Time, tr.Wind, tr.OAT, tr.QNH, tr.GTOW)
	}
	want := []RunwayData{
		{Airport: "KORD", Runway: "22L", Length: 8075, Flaps: 2, V1: 129, VR: 129, V2: 133},
		{Airport: "KORD", Runway: "28R", Length: 9750, V1: 131},
	}
	if len(tr.Runways) != len(want) {
		t.Fatalf("runways = %+v, want %+v", tr.Runways, want)
	}
	for i := range want {
		if tr.Runways[i] != want[i] {
			t.Errorf("runway %d = %+v, want %+v", i, tr.Runways[i], want[i])
		}
	}
}

// TestParser_EnvoyFlightNeedsMatchingDigits checks that the envelope's
// flight is used only when its number is the one in the header line.
func TestParser_EnvoyFlightNeedsMatchingDigits(t *testing.T) {
	text := strings.Replace(envoyTakeoffData, "/FI MQ3845", "/FI MQ3846", 1)
	tr, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "C1", Text: text}).(*Result)
	if !ok {
		t.Fatal("expected a result")
	}
	if tr.FlightNumber != "" {
		t.Errorf("FlightNumber = %q, want empty", tr.FlightNumber)
	}
}
