// Package acmsreport parses the header and CC block of Airbus aircraft
// condition monitoring system (ACMS) reports sent on label H1.
package acmsreport

import "acars_parser/internal/patterns"

// Formats defines the ACMS report header and its CC block.
//
// Example: A321,014057,1,1,TB000000/REP001,00,00,1/CCVH-VWT,JAN20,040543,YSSY,YBBN,0816/C0TIA05JST4R0000/...
// Groups: the aircraft series (A319, A320, A321, ...), the report number,
// and from the CC block the registration (sometimes with a leading "."),
// the report date (MMMDD) and time (HHMMSS), origin, destination and the
// flight number's digits, without the airline code.
//
// The fields between the series and the report number (a serial number and
// the "TB000000" block) are not captured: their meaning is not established.
var Formats = []patterns.Format{
	{
		Name: "acms_cc",
		Pattern: `^(?P<series>A3\d{2}),\d+,\d,\d,TB\d+/REP(?P<report>\d{3}),[^/]*/` +
			`CC(?P<reg_field>\.?[A-Z0-9-]{2,8}),(?P<date>[A-Z]{3}\d{2}),(?P<time>\d{6}),` +
			`(?P<origin>{ICAO}),(?P<dest>{ICAO}),(?P<flight_digits>\d{1,4})(?:/|\s|$)`,
		Fields: []string{"series", "report", "reg_field", "date", "time", "origin", "dest", "flight_digits"},
	},
}
