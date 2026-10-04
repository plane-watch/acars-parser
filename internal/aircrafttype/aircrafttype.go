// Package aircrafttype normalises aircraft types, as transmitted in ACARS
// messages, to ICAO type designators (ICAO Doc 8643).
//
// Messages carry the type in several forms: a designator ("B738"), a
// manufacturer model name ("B737-800", "787-9 GENX-1B76A", "A330-323"), a
// designator with a configuration ("A333-BCS3") or an IATA code ("388").
// Normalise returns a designator only when the form identifies exactly one;
// an ambiguous value ("AT7", "737") is not normalised, so that no type is
// published that the message does not prove. Callers keep the raw value too.
package aircrafttype

import (
	"regexp"
	"strings"
)

var (
	// designatorRe matches the shape of an ICAO type designator.
	designatorRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,3}$`)

	// configuredRe matches a designator followed by a configuration code,
	// e.g. "A333-BCS3". The configuration starts with a letter, which
	// distinguishes it from a model number ("B737-800").
	configuredRe = regexp.MustCompile(`^([A-Z][A-Z0-9]{3})-[A-Z][A-Z0-9]*$`)

	// boeingRe matches a Boeing model name: optional "B", the series, then
	// the variant, e.g. "B737-800", "737-800 CFM56-7B26", "787-9 GENX-1B76A",
	// "B7378MAX".
	boeingRe = regexp.MustCompile(`^B?(7[0-9]7)-?(\d{1,2}0{0,2})(?:\s*(MAX))?(?:\s|$)`)

	// boeingMaxRe matches the compact MAX form "B7378MAX".
	boeingMaxRe = regexp.MustCompile(`^B?737-?([789])\s*MAX`)

	// familyNameRe matches manufacturer family names that look like
	// designators but are not ones: Boeing "B707" to "B797", and Airbus
	// "A330", "A340", "A350" and "A380" (whereas A310, A319, A320 and A321
	// are designators).
	familyNameRe = regexp.MustCompile(`^(?:B7\d7|A3[3-8]0)$`)

	// airbusRe matches an Airbus model name, e.g. "A330-323", "A321-271N".
	airbusRe = regexp.MustCompile(`^A3(\d)(\d)-(\d)\d{1,2}(N?)(?:\s|$)`)
)

// boeingVariants maps a Boeing series and variant to its designator. Only
// variants with a single designator are listed: for example "777" alone, or
// a 777-200 freighter, is not.
var boeingVariants = map[string]string{
	"737-600": "B736", "737-700": "B737", "737-800": "B738", "737-900": "B739",
	"747-400": "B744", "747-8": "B748",
	"757-200": "B752", "757-300": "B753",
	"767-200": "B762", "767-300": "B763", "767-400": "B764",
	"777-200": "B772", "777-300": "B773",
	"787-8": "B788", "787-9": "B789", "787-10": "B78X",
}

// boeingMax maps a 737 MAX variant digit to its designator.
var boeingMax = map[string]string{"7": "B37M", "8": "B38M", "9": "B39M"}

// airbusSeries maps an Airbus series (A3XY) and the first digit of the
// model number to a designator, for series where that digit distinguishes
// variants; for the A320 family the model number does not, so the series
// alone gives the designator (with "N" giving the neo).
var airbusSeries = map[string]string{
	"330-2": "A332", "330-3": "A333", "330-8": "A338", "330-9": "A339",
	"340-2": "A342", "340-3": "A343", "340-5": "A345", "340-6": "A346",
	"350-9": "A359", "350-1": "A35K",
	"380-8": "A388",
}

// airbusFamily maps the A320 family to its designators (ceo, neo).
var airbusFamily = map[string][2]string{
	"319": {"A319", "A19N"},
	"320": {"A320", "A20N"},
	"321": {"A321", "A21N"},
}

// iataCodes maps IATA aircraft type codes that identify a single ICAO
// designator. Generic codes ("737", "330", "AT7") are deliberately absent.
var iataCodes = map[string]string{
	"388": "A388",
	"32N": "A20N",
	"32Q": "A21N",
	"359": "A359",
	"351": "A35K",
	"788": "B788",
	"789": "B789",
	"78X": "B78X",
	"77W": "B77W",
	"7M8": "B38M",
	"7M9": "B39M",
}

// Normalise returns the ICAO type designator for an aircraft type as
// transmitted, and false if the value does not identify exactly one.
func Normalise(raw string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s == "" {
		return "", false
	}

	if d, ok := iataCodes[s]; ok {
		return d, true
	}
	if m := configuredRe.FindStringSubmatch(s); m != nil {
		if familyNameRe.MatchString(m[1]) {
			return "", false
		}
		return m[1], true
	}
	if m := boeingMaxRe.FindStringSubmatch(s); m != nil {
		return boeingMax[m[1]], true
	}
	if m := boeingRe.FindStringSubmatch(s); m != nil {
		if d, ok := boeingVariants[m[1]+"-"+m[2]]; ok {
			return d, true
		}
		return "", false
	}
	if m := airbusRe.FindStringSubmatch(s); m != nil {
		series := m[1] + m[2]
		if fam, ok := airbusFamily["3"+series]; ok {
			if m[4] == "N" {
				return fam[1], true
			}
			return fam[0], true
		}
		if d, ok := airbusSeries["3"+series+"-"+m[3]]; ok {
			return d, true
		}
		return "", false
	}
	// A designator-shaped value with a letter first is taken as transmitted.
	// Purely numeric or ambiguous short codes (e.g. "737", "AT7") are not.
	if designatorRe.MatchString(s) && len(s) == 4 && !familyNameRe.MatchString(s) {
		return s, true
	}
	return "", false
}
