// Package acmsreport parses the header and CC block of Airbus aircraft
// condition monitoring system (ACMS) reports sent on label H1, and the
// fixed-width record of report 239.
package acmsreport

import "acars_parser/internal/patterns"

// Formats defines the ACMS report header and its CC block.
//
// Examples:
//
//	A321,014057,1,1,TB000000/REP001,00,00,1/CCVH-VWT,JAN20,040543,YSSY,YBBN,0816/C0TIA05JST4R0000/...
//	A321,039021,1,1,TB000000/REP037,00,00,4/C1N74532,OCT02,042921,KDEN,KLAA,1460/C29999,...
//	A320,002230,1,1,TB000000/REP037,00,00,4/C1N491UA,OCT04,XXXXXX,KSFO,KAUS,0400/...
//	A320,035141,1,1,TB000000/REP050,00,00,4/CCN402FR,OCT02,KATL,KIAD,3692/C001,...
//	A321,000460,1,1,TB000000/REP032,00,00,4/C1N34562,JAN,03,055354,KIAH,KORR/C2UAL787,4300,09/...
//
// Groups: the aircraft series (A319, A320, A321, ...), the report number,
// and from the block (named CC or C1) the registration (sometimes with a
// leading "."), the report date (month and day, sometimes separated by a
// comma), the time (HHMMSS; blanked as XXXXXX or left out in some reports),
// origin, destination and the flight number's digits, without the airline
// code (left out in some reports). Reports without the digits may carry the
// ICAO callsign at the start of a C2 block.
//
// The fields between the series and the report number (a serial number and
// the "TB000000" block) are not captured: their meaning is not established.
var Formats = []patterns.Format{
	{
		Name: "acms_cc",
		Pattern: `^(?P<series>A3\d{2}),\d+,\d,\d,TB\d+/REP(?P<report>\d{3}),[^/]*/` +
			`C[C1](?P<reg_field>\.?[A-Z0-9-]{2,8}),(?P<month>[A-Z]{3}),?(?P<day>\d{2}),` +
			`(?:(?:(?P<time>\d{6})|X{6}),)?(?P<origin>{ICAO}),(?P<dest>{ICAO})` +
			`(?:,(?P<flight_digits>\d{1,4}))?(?:/C2(?P<callsign>[A-Z]{3}\d{1,4}[A-Z]?),|/|\s|$)`,
		Fields: []string{"series", "report", "reg_field", "month", "day", "time", "origin", "dest", "flight_digits", "callsign"},
	},
}
