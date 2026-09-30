package crew

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser_Parse(t *testing.T) {
	parser := &Parser{}

	// Crew names and employee IDs are redacted; the layout is from a real message.
	text := `QUNDCULUA~1CREW LIST
UA475/10 CYEG KDEN
SENT:  21:47:04z
GATE ETA 0054
-STD TAXI TIME ADDED-
COCKPIT:
1.CA REDACTED  PILOT A
  U000001
2.FO REDACTED  PILOT B
  U000002
CABIN:
FA REDACTED   CABIN A
   U000003
FA REDACTED   CABIN B
   U000004
FA REDACTED   CABIN C
   U000005
FM REDACTED   CABIN D
   U000006
FLIGHT ATTENDANT MIN:4`

	msg := &acars.Message{ID: 12345, Label: "RA", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	cr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}

	if cr.FlightNumber != "UA475" {
		t.Errorf("FlightNumber = %q, want %q", cr.FlightNumber, "UA475")
	}
	if cr.FlightDate != "10" {
		t.Errorf("FlightDate = %q, want %q", cr.FlightDate, "10")
	}
	if cr.Origin != "CYEG" {
		t.Errorf("Origin = %q, want %q", cr.Origin, "CYEG")
	}
	if cr.Destination != "KDEN" {
		t.Errorf("Destination = %q, want %q", cr.Destination, "KDEN")
	}
	if cr.SentTime != "21:47:04z" {
		t.Errorf("SentTime = %q, want %q", cr.SentTime, "21:47:04z")
	}
	if cr.GateETA != "0054" {
		t.Errorf("GateETA = %q, want %q", cr.GateETA, "0054")
	}
	if cr.MinCrew != 4 {
		t.Errorf("MinCrew = %d, want 4", cr.MinCrew)
	}

	if len(cr.CockpitCrew) != 2 {
		t.Errorf("expected 2 cockpit crew, got %d", len(cr.CockpitCrew))
	} else {
		if cr.CockpitCrew[0].Position != "CA" {
			t.Errorf("CockpitCrew[0].Position = %q, want %q", cr.CockpitCrew[0].Position, "CA")
		}
		if cr.CockpitCrew[0].EmployeeID != "U000001" {
			t.Errorf("CockpitCrew[0].EmployeeID = %q, want %q", cr.CockpitCrew[0].EmployeeID, "U000001")
		}
		if cr.CockpitCrew[0].Name != "REDACTED PILOT A" {
			t.Errorf("CockpitCrew[0].Name = %q, want %q", cr.CockpitCrew[0].Name, "REDACTED PILOT A")
		}
	}

	if len(cr.CabinCrew) != 4 {
		t.Errorf("expected 4 cabin crew, got %d", len(cr.CabinCrew))
	}
}

func TestParser_QuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"CREW LIST", true},
		{"~1CREW LIST", true},
		{"Some other message", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
