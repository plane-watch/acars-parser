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
