package label80

import (
	"testing"

	"acars_parser/internal/acars"
)

// TestParseTailStopsAtTheSlash checks that the tail does not include the
// field that follows it ("HK-5365/01F"). The message is real, from the
// January 2026 corpus.
func TestParseTailStopsAtTheSlash(t *testing.T) {
	text := "3N01 POSRPT 0254/04 SKBO/CYYZ HK-5365/01F 10:55\r\n/WYP VERKO"
	r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "80", Text: text}).(*Result)
	if !ok {
		t.Fatal("Parse returned no result")
	}
	if r.Tail != "HK-5365" {
		t.Errorf("Tail = %q, want HK-5365", r.Tail)
	}
}

// TestParseRejectsAMalformedTail checks that a malformed tail token is not
// cut down to a registration-like prefix.
func TestParseRejectsAMalformedTail(t *testing.T) {
	text := "3N01 POSRPT 0254/04 SKBO/CYYZ HK-5365.BAD/01F 10:55"
	if r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "80", Text: text}).(*Result); ok && r.Tail != "" {
		t.Errorf("Tail = %q, want none", r.Tail)
	}
}

// TestParseHeaderFlight checks that the header's flight field is reported,
// so that the extractor uses the route only for the transmitted flight: the
// number's digits, a suffix checked against the transmitted flight, or the
// ICAO callsign. A field that is not a flight gives no route.
func TestParseHeaderFlight(t *testing.T) {
	tests := []struct {
		name, flight, text string
		digits, callsign   string
		route              bool
	}{
		{"digits", "AV0254", "3N01 POSRPT 0254/04 SKBO/CYYZ .HK-5365", "0254", "", true},
		{"digits without the day", "FZ1574", "3N01 POSRPT 1574/ OMDB/OKBK .A6-FEC", "1574", "", true},
		{"callsign", "XABAH1", "3N01 POSRPT BAH1/03 OBBI/EGLL .A9C-HMH", "", "BAH1", true},
		{"suffix of the transmitted flight", "EY071M", "3J01 DSPTCH 071M/25 EGLL/OMAA .A6-APF", "071", "", true},
		{"suffix of another flight", "EY071N", "3J01 DSPTCH 071M/25 EGLL/OMAA .A6-APF", "", "", false},
		{"not a flight", "EY0071", "3J01 DSPTCH ABCDEFGH/25 EGLL/OMAA .A6-APF", "", "", false},
	}
	for _, tt := range tests {
		r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "80", FlightNumber: tt.flight, Text: tt.text}).(*Result)
		if !ok {
			t.Errorf("%s: Parse returned no result", tt.name)
			continue
		}
		if r.FlightNumberDigits != tt.digits || r.FlightNum != tt.callsign || (r.OriginICAO != "") != tt.route {
			t.Errorf("%s: digits %q, flight %q, route %q-%q; want %q, %q, route %v",
				tt.name, r.FlightNumberDigits, r.FlightNum, r.OriginICAO, r.DestICAO, tt.digits, tt.callsign, tt.route)
		}
	}
}
