package paxbag

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser_Parse(t *testing.T) {
	parser := &Parser{}

	text := `QUHELASAY~1
CUSTOMER PAX AND BAG DETAILS FOR WEIGHT AND BALANCE
FLIGHT INFO  AT7       REG OH-ATG   68Y                     PW
AY1031   31DEC  HEL STD2120             BOARD 2050 GATE 9   AO
             *** ACCEPTANCE NOT FINALISED ***
DEST    CABIN    A    M    F    C    I   HOLD BAGS
TLL
 JOINING  Y      0    9    3    0    0     3@56
                          RUSH BAGS  -     0@0
                          CREW BAGS  -     0@0
----------------------------------------------------------------
TOTAL(TLL)       0    9    3    0    0     3@56
----------------------------------------------------------------
GRAND TOTAL (INCLUDING RUSH & CREW BAGS)
(EX HEL)         0    9    3    0    0     3@56
TOTAL PASSENGERS 12 PLUS INFANTS 0
----------------------------------------------------------------
AREA        A    M    F    C    I
ZONE A      0    1    0    0    0
ZONE B      0    1    0    0    0
ZONE C      0    7    3    0    0
----------------------------------------------------------------
TOTAL       0    9    3    0    0
----------------------------------------------------------------`

	msg := &acars.Message{ID: 12345, Label: "RA", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	pr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}

	if pr.AircraftType != "AT7" {
		t.Errorf("AircraftType = %q, want %q", pr.AircraftType, "AT7")
	}
	if pr.Registration != "OH-ATG" {
		t.Errorf("Registration = %q, want %q", pr.Registration, "OH-ATG")
	}
	if pr.Configuration != "68Y" {
		t.Errorf("Configuration = %q, want %q", pr.Configuration, "68Y")
	}
	if pr.FlightNumber != "AY1031" {
		t.Errorf("FlightNumber = %q, want %q", pr.FlightNumber, "AY1031")
	}
	if pr.Date != "31DEC" {
		t.Errorf("Date = %q, want %q", pr.Date, "31DEC")
	}
	if pr.Origin != "HEL" {
		t.Errorf("Origin = %q, want %q", pr.Origin, "HEL")
	}
	if pr.Destination != "TLL" {
		t.Errorf("Destination = %q, want %q", pr.Destination, "TLL")
	}
	if pr.TotalPax != 12 {
		t.Errorf("TotalPax = %d, want 12", pr.TotalPax)
	}
	if pr.BagCount != 3 {
		t.Errorf("BagCount = %d, want 3", pr.BagCount)
	}
	if pr.BagWeight != 56 {
		t.Errorf("BagWeight = %d, want 56", pr.BagWeight)
	}
	if pr.IsFinalised {
		t.Error("IsFinalised should be false")
	}
	if len(pr.Zones) != 3 {
		t.Errorf("expected 3 zones, got %d", len(pr.Zones))
	}
}

func TestParser_QuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"CUSTOMER PAX AND BAG DETAILS", true},
		{"PAX AND BAG DETAILS FOR", true},
		{"Some other message", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}