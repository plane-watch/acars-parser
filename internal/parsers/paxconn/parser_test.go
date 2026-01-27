package paxconn

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser_Parse(t *testing.T) {
	parser := &Parser{}

	text := `PAX CONN STATUS
	CURRENT FLIGHT:
	FLIGHT  ESTIMATED ARRIVAL TIME
	AY1434   EIBT  UTC

	CONNECTION INFO
	FLIGHT DATE     TIME  TO  GATE
	AY1075 20260120 14:30 RIX

	DECISION          CLASS PAX BAGS
	MISSEDCONNECTION  Y     1   0
	--------------------------------
	FLIGHT DATE     TIME  TO  GATE
	AY0315 20260120 14:45 VAA

	DECISION          CLASS PAX BAGS
	PENDING           Y     2   0
	--------------------------------
	FLIGHT DATE     TIME  TO  GATE
	AY0015 20260120 14:50 JFK

	DECISION          CLASS PAX BAGS
	WILLWAIT          Y     5   4
	--------------------------------`

	msg := &acars.Message{ID: 12345, Label: "3E", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	pr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}

	if pr.CurrentFlight != "AY1434" {
		t.Errorf("CurrentFlight = %q, want %q", pr.CurrentFlight, "AY1434")
	}

	if len(pr.Connections) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(pr.Connections))
	}

	// Check first connection.
	if pr.Connections[0].FlightNumber != "AY1075" {
		t.Errorf("Connections[0].FlightNumber = %q, want %q", pr.Connections[0].FlightNumber, "AY1075")
	}
	if pr.Connections[0].Decision != "MISSEDCONNECTION" {
		t.Errorf("Connections[0].Decision = %q, want %q", pr.Connections[0].Decision, "MISSEDCONNECTION")
	}
	if pr.Connections[0].Passengers != 1 {
		t.Errorf("Connections[0].Passengers = %d, want 1", pr.Connections[0].Passengers)
	}

	// Check second connection.
	if pr.Connections[1].Decision != "PENDING" {
		t.Errorf("Connections[1].Decision = %q, want %q", pr.Connections[1].Decision, "PENDING")
	}
	if pr.Connections[1].Passengers != 2 {
		t.Errorf("Connections[1].Passengers = %d, want 2", pr.Connections[1].Passengers)
	}

	// Check third connection.
	if pr.Connections[2].Decision != "WILLWAIT" {
		t.Errorf("Connections[2].Decision = %q, want %q", pr.Connections[2].Decision, "WILLWAIT")
	}
	if pr.Connections[2].Passengers != 5 {
		t.Errorf("Connections[2].Passengers = %d, want 5", pr.Connections[2].Passengers)
	}
	if pr.Connections[2].Bags != 4 {
		t.Errorf("Connections[2].Bags = %d, want 4", pr.Connections[2].Bags)
	}

	// Check counts.
	if pr.MissedCount != 1 {
		t.Errorf("MissedCount = %d, want 1", pr.MissedCount)
	}
	if pr.PendingCount != 2 {
		t.Errorf("PendingCount = %d, want 2", pr.PendingCount)
	}
	if pr.WillWaitCount != 5 {
		t.Errorf("WillWaitCount = %d, want 5", pr.WillWaitCount)
	}
	if pr.TotalConnecting != 8 {
		t.Errorf("TotalConnecting = %d, want 8", pr.TotalConnecting)
	}
}

func TestParser_QuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"PAX CONN STATUS", true},
		{"Some other message", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}