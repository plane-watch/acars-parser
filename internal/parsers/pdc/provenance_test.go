package pdc

import (
	"testing"

	"acars_parser/internal/acars"
)

const jetstarPDC = `PDC 291826
JST501 A320 YSSY 1900
CLEARED TO YMML VIA
16L ABBEY3 DEP: XXX
ROUTE:DCT WOL H65 LEECE Q29 BOOIN DCT
CLIMB VIA SID TO: 5000
DEP FREQ: 129.700
SQUAWK 3670`

// TestParseDoesNotUseAirframesMetadata checks that the result is built only
// from what was transmitted: Airframes' airframe and flight records are not a
// source of facts.
func TestParseDoesNotUseAirframesMetadata(t *testing.T) {
	msg := &acars.Message{
		ID:       1,
		Label:    "RA",
		Text:     jetstarPDC,
		Airframe: &acars.Airframe{Tail: "VH-XXX", ICAO: "7C0000"},
		Flight:   &acars.Flight{Flight: "QFA999"},
	}

	r := (&Parser{}).Parse(msg).(*Result)

	if r.Tail != "" {
		t.Errorf("Tail = %q, want empty (not taken from the airframe record)", r.Tail)
	}
	if r.AircraftICAO != "" {
		t.Errorf("AircraftICAO = %q, want empty (not taken from the airframe record)", r.AircraftICAO)
	}
	if r.FlightNumber != "JST501" {
		t.Errorf("FlightNumber = %q, want JST501 from the text", r.FlightNumber)
	}
}

// TestParseUsesTransmittedIdentity checks that the transmitted tail and flight
// number, and the link-layer address of the uplink's recipient, are used.
func TestParseUsesTransmittedIdentity(t *testing.T) {
	msg := &acars.Message{
		ID:            1,
		Label:         "RA",
		Text:          jetstarPDC,
		Tail:          "VH-VFN",
		LinkDirection: "uplink",
		FromHex:       "11904A",
		ToHex:         "7C6CA3",
	}

	r := (&Parser{}).Parse(msg).(*Result)

	if r.Tail != "VH-VFN" {
		t.Errorf("Tail = %q, want VH-VFN", r.Tail)
	}
	if r.AircraftICAO != "7C6CA3" {
		t.Errorf("AircraftICAO = %q, want 7C6CA3 (the uplink's recipient)", r.AircraftICAO)
	}
}

// TestParseSplitsWakeCategoryFromType uses real message 2856433, where the
// aircraft field is the flight plan form wake category / type / equipment.
func TestParseSplitsWakeCategoryFromType(t *testing.T) {
	text := "-// ATC PA01 YYZOWAC 07JAN/0305          C-FCZF/420/AC0274\r\nTIMESTAMP 07JAN26 02:33\r\n*PRE-DEPARTURE CLEARANCE*\r\nFLT ACA274    CYVR \r\nM/A320/W FILED FL230 \r\nXPRD 5305 \r\n \r\nUSE SID YVR3\r\nDEPARTURE RUNWAY 26L\r\nDESTINATION CYLW\r\nCONTACT CLEARANCE DELIVERY 121.4 WITH\r\nIDENTIFIER 642T\r\n \r\nJANEK SEKAB SEKAB5\r\nEND\r\n"

	r := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "RA", Text: text}).(*Result)

	if r.AircraftType != "A320" {
		t.Errorf("AircraftType = %q, want A320", r.AircraftType)
	}
	if r.WakeCategory != "M" {
		t.Errorf("WakeCategory = %q, want M", r.WakeCategory)
	}
}
