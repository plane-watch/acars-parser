package fuel

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser_Parse(t *testing.T) {
	parser := &Parser{}

	text := `ACARS MSG: FUEL DELIVERY
	FUEL DELIVERY RECEIPT

	AY571 OHLVL 22.01.2026
	DEST: KTT
	FUEL COMPANY: NES
	FUEL GRADE: JETA1

	TRUCK TIMES
	START: 03:37
	END: 04:06
	AMOUNT: 6805 LTRS
	DENSITY: 807 KG/M3
	QTY BEFORE FUELING: 2900 KG

	TRUCK: S6315


	DATES AND TIMES IN UTC`

	msg := &acars.Message{ID: 12345, Label: "3E", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	fr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}

	if fr.FlightNumber != "AY571" {
		t.Errorf("FlightNumber = %q, want %q", fr.FlightNumber, "AY571")
	}
	if fr.Tail != "OHLVL" {
		t.Errorf("Tail = %q, want %q", fr.Tail, "OHLVL")
	}
	if fr.Date != "22.01.2026" {
		t.Errorf("Date = %q, want %q", fr.Date, "22.01.2026")
	}
	if fr.Destination != "KTT" {
		t.Errorf("Destination = %q, want %q", fr.Destination, "KTT")
	}
	if fr.FuelCompany != "NES" {
		t.Errorf("FuelCompany = %q, want %q", fr.FuelCompany, "NES")
	}
	if fr.FuelGrade != "JETA1" {
		t.Errorf("FuelGrade = %q, want %q", fr.FuelGrade, "JETA1")
	}
	if fr.StartTime != "03:37" {
		t.Errorf("StartTime = %q, want %q", fr.StartTime, "03:37")
	}
	if fr.EndTime != "04:06" {
		t.Errorf("EndTime = %q, want %q", fr.EndTime, "04:06")
	}
	if fr.AmountLitres != 6805 {
		t.Errorf("AmountLitres = %d, want %d", fr.AmountLitres, 6805)
	}
	if fr.DensityKgM3 != 807 {
		t.Errorf("DensityKgM3 = %d, want %d", fr.DensityKgM3, 807)
	}
	if fr.QtyBeforeKg != 2900 {
		t.Errorf("QtyBeforeKg = %d, want %d", fr.QtyBeforeKg, 2900)
	}
	if fr.TruckID != "S6315" {
		t.Errorf("TruckID = %q, want %q", fr.TruckID, "S6315")
	}
}

func TestParser_QuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"ACARS MSG: FUEL DELIVERY", true},
		{"FUEL DELIVERY RECEIPT", true},
		{"Some other message", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}