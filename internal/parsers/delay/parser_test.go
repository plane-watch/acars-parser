package delay

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser_Parse(t *testing.T) {
	parser := &Parser{}

	text := `ACARS MSG: DELAY SUMMARY
DELAY SUMMARY
AY1366/03JAN2026 MAN-HEL
STD 18:10
ATD 18:23
DEP DELAY 13 MIN
STA 21:05
ATA 21:25
ARR DELAY 20 MIN
ARR DELAY 7 MIN GREATER THAN DEP DELAY
DEP DELAY CODES
DL93/11MIN DL81/2MIN
MESSAGE CREATED: 2026-01-03T21:29:45Z`

	msg := &acars.Message{ID: 12345, Label: "3E", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	dr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}

	if dr.FlightNumber != "AY1366" {
		t.Errorf("FlightNumber = %q, want %q", dr.FlightNumber, "AY1366")
	}
	if dr.FlightDate != "03JAN2026" {
		t.Errorf("FlightDate = %q, want %q", dr.FlightDate, "03JAN2026")
	}
	if dr.Origin != "MAN" {
		t.Errorf("Origin = %q, want %q", dr.Origin, "MAN")
	}
	if dr.Destination != "HEL" {
		t.Errorf("Destination = %q, want %q", dr.Destination, "HEL")
	}
	if dr.STD != "18:10" {
		t.Errorf("STD = %q, want %q", dr.STD, "18:10")
	}
	if dr.ATD != "18:23" {
		t.Errorf("ATD = %q, want %q", dr.ATD, "18:23")
	}
	if dr.DepDelayMinutes != 13 {
		t.Errorf("DepDelayMinutes = %d, want 13", dr.DepDelayMinutes)
	}
	if dr.STA != "21:05" {
		t.Errorf("STA = %q, want %q", dr.STA, "21:05")
	}
	if dr.ATA != "21:25" {
		t.Errorf("ATA = %q, want %q", dr.ATA, "21:25")
	}
	if dr.ArrDelayMinutes != 20 {
		t.Errorf("ArrDelayMinutes = %d, want 20", dr.ArrDelayMinutes)
	}
	if len(dr.DelayCodes) != 2 {
		t.Errorf("expected 2 delay codes, got %d", len(dr.DelayCodes))
	} else {
		if dr.DelayCodes[0].Code != "DL93" || dr.DelayCodes[0].Minutes != 11 {
			t.Errorf("DelayCodes[0] = %+v, want DL93/11", dr.DelayCodes[0])
		}
		if dr.DelayCodes[1].Code != "DL81" || dr.DelayCodes[1].Minutes != 2 {
			t.Errorf("DelayCodes[1] = %+v, want DL81/2", dr.DelayCodes[1])
		}
	}
	if dr.MessageCreated != "2026-01-03T21:29:45Z" {
		t.Errorf("MessageCreated = %q, want %q", dr.MessageCreated, "2026-01-03T21:29:45Z")
	}
}

func TestParser_NoDelay(t *testing.T) {
	parser := &Parser{}

	text := `ACARS MSG: DELAY SUMMARY
DELAY SUMMARY
AY540/07JAN2026 RVN-HEL
STD 03:20
ATD 03:14
DEP DELAY 0 MIN
STA 04:40
ATA 04:39
ARR DELAY 0 MIN
MESSAGE CREATED: 2026-01-07T04:39:40Z`

	msg := &acars.Message{ID: 12346, Label: "3E", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	dr := result.(*Result)

	if dr.FlightNumber != "AY540" {
		t.Errorf("FlightNumber = %q, want %q", dr.FlightNumber, "AY540")
	}
	if dr.DepDelayMinutes != 0 {
		t.Errorf("DepDelayMinutes = %d, want 0", dr.DepDelayMinutes)
	}
	if dr.ArrDelayMinutes != 0 {
		t.Errorf("ArrDelayMinutes = %d, want 0", dr.ArrDelayMinutes)
	}
	if len(dr.DelayCodes) != 0 {
		t.Errorf("expected 0 delay codes, got %d", len(dr.DelayCodes))
	}
}

func TestParser_QuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"DELAY SUMMARY", true},
		{"ACARS MSG: DELAY SUMMARY", true},
		{"Some other message", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}