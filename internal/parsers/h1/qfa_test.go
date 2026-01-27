package h1

import (
	"encoding/json"
	"testing"

	"acars_parser/internal/acars"
)

func TestQFAFlightPlan(t *testing.T) {
	// Text with embedded newline (as stored in database).
	text := `FPN/SN2993/FNQFA401/RI:DA:YSSY:CR:SYDMEL001:AA:YMML..WOL,S34334E15
0474.H65.LEECE.Q29.BOOINC817`

	msg := &acars.Message{
		ID:    11736,
		Label: "H1",
		Text:  text,
	}

	parser := &FPNParser{}
	if !parser.QuickCheck(text) {
		t.Fatal("QuickCheck failed")
	}

	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("Parse returned nil")
	}

	fpn, ok := result.(*FPNResult)
	if !ok {
		t.Fatalf("Expected FPNResult, got %T", result)
	}

	t.Logf("Origin: %s, Destination: %s", fpn.Origin, fpn.Destination)
	t.Logf("Flight: %s", fpn.FlightNum)
	t.Logf("Waypoints: %d", len(fpn.Waypoints))

	b, _ := json.MarshalIndent(fpn, "", "  ")
	t.Logf("Full result:\n%s", string(b))

	if fpn.Origin != "YSSY" {
		t.Errorf("Expected origin YSSY, got %s", fpn.Origin)
	}
	if fpn.Destination != "YMML" {
		t.Errorf("Expected dest YMML, got %s", fpn.Destination)
	}
}
