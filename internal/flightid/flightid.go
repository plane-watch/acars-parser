// Package flightid splits flight identifiers into their airline code,
// flight number and suffix, in either form: the ICAO form, used in ATC
// callsigns ("QFA1", "LOT49H"), and the IATA form, used in airline flight
// numbers ("QF1", "B6123").
package flightid

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
)

// identifierRe splits a flight identifier: an ICAO code is three letters
// ("SWR4WF"), and an IATA code is two letters or digits ("B6123" is B6
// flight 123). The two forms cannot be confused, because an ICAO code's
// third character is a letter and an IATA flight number's first character
// is a digit.
var identifierRe = regexp.MustCompile(`^(?:([A-Z]{3})|([A-Z0-9]{2}))(\d{1,4})([A-Z]{0,2})$`)

// ID is a flight identifier split into its parts.
type ID struct {
	Airline string
	ICAO    bool   // The airline code is ICAO (three letters), not IATA.
	Number  string // Without leading zeros ("0" for flight 0).
	Suffix  string
}

// Parse splits a flight identifier, and returns false if it is not an
// airline code followed by a flight number. An IATA code must contain a
// letter. An identifier that is the aircraft's tail (ignoring dashes) is a
// registration used as the callsign, and is not split: "N123AB" would
// otherwise read as airline N1, flight 23, suffix AB. tail may be empty.
func Parse(s, tail string) (ID, bool) {
	s = strings.TrimSpace(s)
	if t := acars.NormaliseRegistration(tail); t != "" && acars.NormaliseRegistration(s) == t {
		return ID{}, false
	}
	m := identifierRe.FindStringSubmatch(s)
	if m == nil {
		return ID{}, false
	}
	number := strings.TrimLeft(m[3], "0")
	if number == "" {
		number = "0" // Flight 0, as in "QF000".
	}
	id := ID{Airline: m[1], ICAO: true, Number: number, Suffix: m[4]}
	if m[1] == "" {
		id.Airline, id.ICAO = m[2], false
		if !strings.ContainsAny(id.Airline, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			return ID{}, false
		}
	}
	return id, true
}

// Same reports whether two identifiers name the same flight: the same
// number and suffix, and the same airline when both use the same form of
// airline code. An IATA and an ICAO form with the same number and suffix
// ("QF1255" and "QFA1255") are taken to match, without a table of codes:
// callers compare identifiers of one aircraft, which does not fly two
// airlines' flights with the same number. The whole number is compared,
// so B61 (B6 flight 1) is not JBU61.
func Same(a, b ID) bool {
	if a.Number != b.Number || a.Suffix != b.Suffix {
		return false
	}
	return a.ICAO != b.ICAO || a.Airline == b.Airline
}

// String returns the identifier without leading zeros in its number, as
// routes are stored: "QFA1", "QF1".
func (id ID) String() string {
	return id.Airline + id.Number + id.Suffix
}

// headerDigitsRe matches a header flight field that is the flight number's
// digits, with an optional suffix.
var headerDigitsRe = regexp.MustCompile(`^(\d{1,4})([A-Z]{0,2})$`)

// headerCallsignRe matches a header flight field that is an ICAO callsign:
// three letters, a digit, and up to three more letters or digits ("UAE0069",
// "UAE8UW").
var headerCallsignRe = regexp.MustCompile(`^[A-Z]{3}\d[A-Z0-9]{0,3}$`)

// HeaderFlight reads the flight field of a downlink header ("0800",
// "038A", "UAE0069"): the flight number's digits or the ICAO callsign. It
// returns false when the field names a flight that cannot be compared with
// the transmitted one: a field of another shape, or a suffixed number
// ("038A") whose number and suffix are not the transmitted flight's (the
// extractor compares only the digits). transmitted and tail are the
// message's (see Parse).
func HeaderFlight(field, transmitted, tail string) (digits, callsign string, ok bool) {
	if headerCallsignRe.MatchString(field) {
		return "", field, true
	}
	d := headerDigitsRe.FindStringSubmatch(field)
	if d == nil {
		return "", "", false
	}
	if d[2] != "" {
		number := strings.TrimLeft(d[1], "0")
		if number == "" {
			number = "0" // Flight 0, as Parse writes it.
		}
		t, ok := Parse(transmitted, tail)
		if !ok || t.Suffix != d[2] || t.Number != number {
			return "", "", false
		}
	}
	return d[1], "", true
}
