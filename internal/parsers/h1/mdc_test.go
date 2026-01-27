package h1

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestMDCParser_EngineTrend(t *testing.T) {
	parser := &MDCParser{}

	text := `PAGE 00001
MDC REPORT: ENGINE TREND
WRITE OPTION: ACARS AUTO
FILENAME: AC15490Y.ETD
TIME: 21:23
DATE: 03Jan2026
MDC APPLICATION PN: 832-6207-420
MDC TABLES PN:      810-0042-115

LEG:28045  DATE:03JAN26 TIME:21:23

------------------------------------
L N1                      80.0 %
R N1                      80.0 %
L N2                      83.0 %
R N2                      83.4 %
L ITT                      706 C
R ITT                      694 C
L PS3                      142 PSI
R PS3                      143 PSI
L N1 VIBES                 0.2 MIL
R N1 VIBES                 0.0 MIL
L N2 VIBES                 0.2 MIL
R N2 VIBES                 0.3 MIL
L OIL TEMP                  87 C
R OIL TEMP                  91 C
L OIL PRESSURE              67 PSI
R OIL PRESSURE              69 PSI
L PLA                     35.4 DEG
R PLA                     35.4 DEG
L FUEL FLOW               1995 PPH
R FUEL FLOW               2031 PPH
L VG POSITION              2.7 DEG
R VG POSITION              2.1 DEG
FADEC IN CONTROL         LB AND RA
COMPUTED AIRSPEED        303.1 KT
ALTITUDE                 23996 FT
TOTAL AIR TEMP            -8.5 C
`

	msg := &acars.Message{ID: 12345, Label: "H1", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	mdc, ok := result.(*MDCResult)
	if !ok {
		t.Fatalf("expected *MDCResult, got %T", result)
	}

	// Check header fields.
	if mdc.ReportType != "ENGINE TREND" {
		t.Errorf("ReportType = %q, want %q", mdc.ReportType, "ENGINE TREND")
	}
	if mdc.WriteOption != "ACARS AUTO" {
		t.Errorf("WriteOption = %q, want %q", mdc.WriteOption, "ACARS AUTO")
	}
	if mdc.Filename != "AC15490Y.ETD" {
		t.Errorf("Filename = %q, want %q", mdc.Filename, "AC15490Y.ETD")
	}
	if mdc.Time != "21:23" {
		t.Errorf("Time = %q, want %q", mdc.Time, "21:23")
	}
	if mdc.Date != "03Jan2026" {
		t.Errorf("Date = %q, want %q", mdc.Date, "03Jan2026")
	}
	if mdc.ApplicationPN != "832-6207-420" {
		t.Errorf("ApplicationPN = %q, want %q", mdc.ApplicationPN, "832-6207-420")
	}
	if mdc.LegNumber != "28045" {
		t.Errorf("LegNumber = %q, want %q", mdc.LegNumber, "28045")
	}

	// Check engine trend data.
	if mdc.EngineTrend == nil {
		t.Fatal("expected EngineTrend, got nil")
	}
	et := mdc.EngineTrend

	if et.LeftN1 != 80.0 {
		t.Errorf("LeftN1 = %f, want 80.0", et.LeftN1)
	}
	if et.RightN1 != 80.0 {
		t.Errorf("RightN1 = %f, want 80.0", et.RightN1)
	}
	if et.LeftN2 != 83.0 {
		t.Errorf("LeftN2 = %f, want 83.0", et.LeftN2)
	}
	if et.RightN2 != 83.4 {
		t.Errorf("RightN2 = %f, want 83.4", et.RightN2)
	}
	if et.LeftITT != 706 {
		t.Errorf("LeftITT = %d, want 706", et.LeftITT)
	}
	if et.RightITT != 694 {
		t.Errorf("RightITT = %d, want 694", et.RightITT)
	}
	if et.LeftOilTemp != 87 {
		t.Errorf("LeftOilTemp = %d, want 87", et.LeftOilTemp)
	}
	if et.RightOilTemp != 91 {
		t.Errorf("RightOilTemp = %d, want 91", et.RightOilTemp)
	}
	if et.LeftFuelFlow != 1995 {
		t.Errorf("LeftFuelFlow = %d, want 1995", et.LeftFuelFlow)
	}
	if et.RightFuelFlow != 2031 {
		t.Errorf("RightFuelFlow = %d, want 2031", et.RightFuelFlow)
	}
	if et.FADECControl != "LB AND RA" {
		t.Errorf("FADECControl = %q, want %q", et.FADECControl, "LB AND RA")
	}
	if et.Airspeed != 303.1 {
		t.Errorf("Airspeed = %f, want 303.1", et.Airspeed)
	}
	if et.Altitude != 23996 {
		t.Errorf("Altitude = %d, want 23996", et.Altitude)
	}
	if et.TotalAirTemp != -8.5 {
		t.Errorf("TotalAirTemp = %f, want -8.5", et.TotalAirTemp)
	}
}

func TestMDCParser_CurrentFaults(t *testing.T) {
	parser := &MDCParser{}

	text := `PAGE 00001
MDC REPORT: CURRENT FAULTS
FILENAME: AC15351N.CFM
TIME: 15:20
DATE: 13Jan2026
MDC APPLICATION PN: 832-6207-420
MDC TABLES PN:      810-0042-115

   ATA/LRU/STATUS/FAULT MESSAGE
-----------------------------------
ATA32-44 ANTI-SKID CONTROL
 A/SKID CTRL UNIT   A165
  FAILED
  INTERNAL IB A/SKID FAULT
Equation ID: B1-006057

ATA23-22 COMMS
 VHF RADIO
  DEGRADED
Equation ID: C2-001234
`

	msg := &acars.Message{ID: 12345, Label: "H1", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	mdc, ok := result.(*MDCResult)
	if !ok {
		t.Fatalf("expected *MDCResult, got %T", result)
	}

	if mdc.ReportType != "CURRENT FAULTS" {
		t.Errorf("ReportType = %q, want %q", mdc.ReportType, "CURRENT FAULTS")
	}

	if len(mdc.Faults) != 2 {
		t.Fatalf("expected 2 faults, got %d", len(mdc.Faults))
	}

	if mdc.Faults[0].ATA != "32-44" {
		t.Errorf("Faults[0].ATA = %q, want %q", mdc.Faults[0].ATA, "32-44")
	}
	if mdc.Faults[0].System != "ANTI-SKID CONTROL" {
		t.Errorf("Faults[0].System = %q, want %q", mdc.Faults[0].System, "ANTI-SKID CONTROL")
	}
	if mdc.Faults[0].EquationID != "B1-006057" {
		t.Errorf("Faults[0].EquationID = %q, want %q", mdc.Faults[0].EquationID, "B1-006057")
	}

	if mdc.Faults[1].ATA != "23-22" {
		t.Errorf("Faults[1].ATA = %q, want %q", mdc.Faults[1].ATA, "23-22")
	}
	if mdc.Faults[1].EquationID != "C2-001234" {
		t.Errorf("Faults[1].EquationID = %q, want %q", mdc.Faults[1].EquationID, "C2-001234")
	}
}

func TestMDCParser_QuickCheck(t *testing.T) {
	parser := &MDCParser{}

	tests := []struct {
		text string
		want bool
	}{
		{"PAGE 00001\nMDC REPORT: ENGINE TREND", true},
		{"MDC REPORT: CURRENT FAULTS", true},
		{"Some other H1 message", false},
		{"++86501,N8747Q,B7378MAX", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q...) = %v, want %v", tt.text[:20], got, tt.want)
		}
	}
}