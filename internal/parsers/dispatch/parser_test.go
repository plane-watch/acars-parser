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

	msg := &acars.Message{ID: 12345, Label: "RA", Tail: "N381HA", Text: text}
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
	MOC NAME: REDACTED NAME`

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

// TestParser_FlightTailLineNeedsTheTail checks that a "flight tail" line is
// read only when its second token is the transmitted tail: weather text
// such as "FEW050 BKN100" has the same shape.
func TestParser_FlightTailLineNeedsTheTail(t *testing.T) {
	for _, tc := range []struct{ tail, text string }{
		{"N506DN", "DISPATCHER MSG\nKATL 041852Z 27008KT 10SM FEW050 BKN100\nPLEASE ACK"},
		{"", "DISPATCHER MSG\nASA849 N381HA\nAMND FLT PLAN"},
		{"N381HB", "DISPATCHER MSG\nASA849 N381HA\nAMND FLT PLAN"},
	} {
		r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "RA", Tail: tc.tail, Text: tc.text}).(*Result)
		if !ok {
			t.Fatalf("tail %q: Parse returned no result", tc.tail)
		}
		if r.Tail != "" || r.FlightNumber != "" {
			t.Errorf("tail %q: Tail = %q, FlightNumber = %q; want none", tc.tail, r.Tail, r.FlightNumber)
		}
	}
}

// TestParser_FleetNumbers checks that "FLT: 991 / ACFT: 391" gives the flight
// number's digits and the airline's aircraft number, not a flight number
// and a registration: neither is in that form.
func TestParser_FleetNumbers(t *testing.T) {
	text := "DISPATCHER MSG\nFLT: 991\nACFT: 391\nPLEASE ACK"
	r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "RA", Text: text}).(*Result)
	if !ok {
		t.Fatal("Parse returned no result")
	}
	if r.Tail != "" || r.FlightNumber != "" || r.FlightNumberDigits != "991" || r.AircraftNumber != "391" {
		t.Errorf("got tail %q, flight %q, digits %q, aircraft number %q", r.Tail, r.FlightNumber, r.FlightNumberDigits, r.AircraftNumber)
	}
}
