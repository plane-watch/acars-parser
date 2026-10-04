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
// ICAO callsign, and origin/destination.
var Formats = []patterns.Format{
	{
		Name: "cmc_header",
		Pattern: `^(?P<report_type>RTE|PLF|CFG)\s+(?P<seq>\d+)\s+(?P<date>\d{2}[A-Z]{3}\d{2})\s+(?P<time>\d{4})\s+` +
			`(?:(?P<airline>[A-Z0-9]{2})\s+)?(?P<reg_field>[A-Z0-9-]+)\s+(?P<flight>[A-Z]{3}[A-Z0-9]{1,5})\s+` +
			`(?P<origin>{ICAO})/(?P<dest>{ICAO})`,
		Fields: []string{"report_type", "seq", "date", "time", "airline", "reg_field", "flight", "origin", "dest"},
	},
}
