package landingdata

import (
	"testing"

	"acars_parser/internal/acars"
)

// TestParser_ModelLine checks that the model and engine on United's landing
// data are captured separately, including an extended-range model.
func TestParser_ModelLine(t *testing.T) {
	text := ".NDCULUA 201418\n\tAGM\n\tAN N2534U\n\t-  /LANDING DATA\n\t ** PART 01 OF 01 **\n" +
		"\t************************\n\tLANDING DATA MNL RW 06\n\t11188 FT\n\t777-300ER GE90-115BL\n" +
		"\t       *FLAPS 25*\n\tTEMP 24C       ALT 29.88\n\tWIND 123/6 MAG\n"
	result := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "C1", Text: text})
	lr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}
	if lr.AircraftType != "777-300ER" || lr.EngineType != "GE90-115BL" {
		t.Errorf("AircraftType, EngineType = %q, %q, want %q, %q", lr.AircraftType, lr.EngineType, "777-300ER", "GE90-115BL")
	}
	if lr.Airport != "MNL" || lr.Runway != "06" {
		t.Errorf("Airport, Runway = %q, %q, want MNL, 06", lr.Airport, lr.Runway)
	}
}
