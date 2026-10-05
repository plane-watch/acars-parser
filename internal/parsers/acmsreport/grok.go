// Package acmsreport parses the header and CC block of Airbus aircraft
// condition monitoring system (ACMS) reports sent on label H1, and the
// fixed-width record of report 239.
package acmsreport

import "acars_parser/internal/patterns"

// shortHeader is the short report header, e.g. "A37/A31937,1,1".
const shortHeader = `[A-Z]\d{2}/(?P<series_short>A3\d{2})(?P<report_short>\d{2}),\d,\d`

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
// Reports also come with a short header, "A37/A31937,1,1/": a letter and
// the report number, then the series (A319) followed by the report number
// again (37). Its report number has two digits and is kept as sent.
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
		Pattern: `^(?:(?P<series>A3\d{2}),\d+,\d,\d,TB\d+/REP(?P<report>\d{3}),[^/]*|` + shortHeader + `)/` +
			`C[C1](?P<reg_field>\.?[A-Z0-9-]{2,8}),(?P<month>[A-Z]{3}),?(?P<day>\d{2}),` +
			`(?:(?:(?P<time>\d{6})|X{6}),)?(?P<origin>{ICAO}),(?P<dest>{ICAO})` +
			`(?:,(?P<flight_digits>\d{1,4}))?(?:/C2(?P<callsign>[A-Z]{3}\d{1,4}[A-Z]?),|/|\s|$)`,
		Fields: []string{"series", "report", "series_short", "report_short", "reg_field", "month", "day", "time", "origin", "dest", "flight_digits", "callsign"},
	},
	// The C1TRP block, sent with the short header (reports 38 and 39), starts
	// a trajectory report: the time (HHMMSS), the route, and fields not
	// parsed, followed by samples (C2, C3, ...) that are not parsed either.
	// Example:
	//
	//	A38/A32138,1,1/C1TRP,180234,KDFW,KRIC,08,8,94238/C2348139,-0904766,330,...
	{
		Name:    "acms_trp",
		Pattern: `^` + shortHeader + `/C1TRP,(?P<time>\d{6}),(?P<origin>{ICAO}),(?P<dest>{ICAO}),`,
		Fields:  []string{"series_short", "report_short", "time", "origin", "dest"},
	},
	// Report 281 gives the route after "//WX02EN04" and, on the next line,
	// usually a position in thousandths of a degree ("N42191W072884" is
	// 42.191, -72.884, just after leaving KBDL for KDTW). The rest of the
	// report is not parsed. Example:
	//
	//	A320,115883,1,1,TB000000/REP281,00,00,4//WX02EN04KBDLKDTW
	//	N42191W07288409361047P0222180140XXXX21003020)
	{
		Name: "acms_281",
		Pattern: `^(?P<series>A3\d{2}),\d+,\d,\d,TB\d+/REP(?P<report>281),[^/]*//WX\d{2}EN\d{2}` +
			`(?P<origin>{ICAO})(?P<dest>{ICAO})[ \t]*(?:\r?\n(?P<lat_hemi>[NS])(?P<lat>\d{5})(?P<lon_hemi>[EW])(?P<lon>\d{6}))?`,
		Fields: []string{"series", "report", "origin", "dest", "lat_hemi", "lat", "lon_hemi", "lon"},
	},
	// Report 291 is a trajectory report: the route on the "TRP" line, then
	// timed samples (A1, A2, ...) that are not parsed. Example:
	//
	//	A321,147316,1,1,TB000000/REP291,00,00,4/
	//	TRP KPHL KPBI  8 8
	//	/A1 175021, 32.3356,- 80.3872,339,167.0,437,0.001984,
	{
		Name: "acms_291",
		Pattern: `^(?P<series>A3\d{2}),\d+,\d,\d,TB\d+/REP(?P<report>291),[^/]*/\s*` +
			`TRP (?P<origin>{ICAO}) (?P<dest>{ICAO})\s`,
		Fields: []string{"series", "report", "origin", "dest"},
	},
}
