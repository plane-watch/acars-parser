package takeoff

import (
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