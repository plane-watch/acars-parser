package hazard

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser_Parse(t *testing.T) {
	parser := &Parser{}

	text := `MSG/RX03-JAN-26 0500Z FR:ARINCDIRECT TO:9HMOON HAZARD ALERT FOR KFE300 H7104: LMML-LSGG FLIGHT: LMML-LSGG (0520Z ETD)  CRITICAL: EDR TURBULENCE IS 0.5 EDR  SEGMENT: TIDKA-LSGG ETO: 0637Z (1+17)  WARNING: WINDS IS 20`

	msg := &acars.Message{ID: 12345, Label: "H1", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	hr, ok := result.(*HazardResult)
	if !ok {
		t.Fatalf("expected *HazardResult, got %T", result)
	}

	if hr.Timestamp != "03-JAN-26 0500Z" {
		t.Errorf("Timestamp = %q, want %q", hr.Timestamp, "03-JAN-26 0500Z")
	}
	if hr.From != "ARINCDIRECT" {
		t.Errorf("From = %q, want %q", hr.From, "ARINCDIRECT")
	}
	if hr.To != "9HMOON" {
		t.Errorf("To = %q, want %q", hr.To, "9HMOON")
	}
	if hr.Callsign != "KFE300" {
		t.Errorf("Callsign = %q, want %q", hr.Callsign, "KFE300")
	}
	if hr.FlightID != "H7104" {
		t.Errorf("FlightID = %q, want %q", hr.FlightID, "H7104")
	}
	if hr.Origin != "LMML" {
		t.Errorf("Origin = %q, want %q", hr.Origin, "LMML")
	}
	if hr.Destination != "LSGG" {
		t.Errorf("Destination = %q, want %q", hr.Destination, "LSGG")
	}
	if hr.ETD != "0520Z" {
		t.Errorf("ETD = %q, want %q", hr.ETD, "0520Z")
	}
	if hr.EDR != 0.5 {
		t.Errorf("EDR = %f, want 0.5", hr.EDR)
	}
	if hr.Segment != "TIDKA-LSGG" {
		t.Errorf("Segment = %q, want %q", hr.Segment, "TIDKA-LSGG")
	}
	if hr.ETO != "0637Z" {
		t.Errorf("ETO = %q, want %q", hr.ETO, "0637Z")
	}
	if hr.WindWarning != "20 knots" {
		t.Errorf("WindWarning = %q, want %q", hr.WindWarning, "20 knots")
	}
	if hr.AlertLevel != "CRITICAL" {
		t.Errorf("AlertLevel = %q, want %q", hr.AlertLevel, "CRITICAL")
	}
}

func TestParser_QuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"HAZARD ALERT FOR KFE300", true},
		{"MSG/RX03-JAN-26 HAZARD ALERT", true},
		{"Normal position report", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}