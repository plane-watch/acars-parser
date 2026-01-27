package dispatch

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser_FuelAmendment(t *testing.T) {
	parser := &Parser{}

	text := `DISPATCHER MSG
ASA849 N381HA

AMND FLT PLAN RLS VER 4.
..

ADD 480 LBS TO MRF AND M
IN TO FUEL DUE OVER PTOW
.

NEW MRF 142542 LBS.
NEW MIN TO FUEL 141552 L
BS.

DISP/KS 18443/2303Z`

	msg := &acars.Message{ID: 12345, Label: "RA", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	dr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}

	if dr.FlightNumber != "ASA849" {
		t.Errorf("FlightNumber = %q, want %q", dr.FlightNumber, "ASA849")
	}
	if dr.Tail != "N381HA" {
		t.Errorf("Tail = %q, want %q", dr.Tail, "N381HA")
	}
	if dr.Category != "FUEL" {
		t.Errorf("Category = %q, want %q", dr.Category, "FUEL")
	}
	if dr.DispatcherID != "KS" {
		t.Errorf("DispatcherID = %q, want %q", dr.DispatcherID, "KS")
	}
	if dr.Timestamp != "2303Z" {
		t.Errorf("Timestamp = %q, want %q", dr.Timestamp, "2303Z")
	}
}

func TestParser_MEL(t *testing.T) {
	parser := &Parser{}

	text := `DISPATCHER MSG
	FLT: 991
	ACFT: 391
	MEL, CDL, SDL REF: 74-31-1A
	MDDR #: = 545476
	MOC NAME: DARREN OMOTO`

	msg := &acars.Message{ID: 12345, Label: "RA", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	dr := result.(*Result)

	if dr.MELRef != "74-31-1A" {
		t.Errorf("MELRef = %q, want %q", dr.MELRef, "74-31-1A")
	}
	if dr.MDDRNumber != "545476" {
		t.Errorf("MDDRNumber = %q, want %q", dr.MDDRNumber, "545476")
	}
	if dr.Category != "MEL" {
		t.Errorf("Category = %q, want %q", dr.Category, "MEL")
	}
}

func TestParser_SIGMET(t *testing.T) {
	parser := &Parser{}

	text := `DISPATCHER MSG
	RJJJ SIGMET P02 VALID 11
	0331/110731 RJTD-
	RJJJ FUKUOKA FIR SEV TUR
	B FCST`

	msg := &acars.Message{ID: 12345, Label: "RA", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	dr := result.(*Result)

	if dr.Category != "SIGMET" {
		t.Errorf("Category = %q, want %q", dr.Category, "SIGMET")
	}
}

func TestParser_QuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"DISPATCHER MSG", true},
		{"42 DISPATCHER MSG", true},
		{"Some other message", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}