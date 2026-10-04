// Package cmcreport parses the header of Boeing central maintenance computer
// (CMC) reports sent on label H1: RTE (route), PLF (post-flight) and CFG
// (configuration) reports.
package cmcreport

import "acars_parser/internal/patterns"

// Formats defines the CMC report header.
//
// Example: RTE 1 04OCT26 0930 TG HS-TWC THA482 YPPH/VTBS BCG4F-45LD-0077 C L 0915 04OCT26
// Groups: report type, sequence, date (DDMMMYY), time (HHMM), an optional IATA
// airline code followed by a space, the registration field (which may have
// the airline code glued to its front, e.g. "5YN703GT" or "BRB-17807"), the
// ICAO callsign (three letters, then a flight number starting with a digit),
// and origin/destination.
//
// The header is a single line, so fields are separated by spaces only; the
// destination must end the line or be followed by a space, so that a longer
// token is not cut short.
var Formats = []patterns.Format{
	{
		Name: "cmc_header",
		Pattern: `^(?P<report_type>RTE|PLF|CFG) +(?P<seq>\d+) +(?P<date>\d{2}[A-Z]{3}\d{2}) +(?P<time>\d{4}) +` +
			`(?:(?P<airline>[A-Z0-9]{2}) +)?(?P<reg_field>[A-Z0-9-]+) +(?P<flight>[A-Z]{3}\d[A-Z0-9]{0,4}) +` +
			`(?P<origin>{ICAO})/(?P<dest>{ICAO})(?:\s|$)`,
		Fields: []string{"report_type", "seq", "date", "time", "airline", "reg_field", "flight", "origin", "dest"},
	},
}
