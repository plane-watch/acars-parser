// Package agfsr provides grok-style pattern definitions for AGFSR message parsing.
package agfsr

import "acars_parser/internal/patterns"

// Formats defines the known AGFSR message formats.
var Formats = []patterns.Format{
	// AGFSR flight status format.
	// Example: AGFSR AC1234/29/29/YULMIA/1234Z/110/3457.3N07711.0W/350/CRUISE/1234/0567/M37/248095/1234/GS450/UNK/1530/1600
	// Groups: flight, day1, day2, route, time, unknown1, position, fl, phase, fuel_remain, fuel_used, mach, wind, heading, field1, field2, eta, sched
	{
		Name: "agfsr_status",
		Pattern: `^AGFSR\s+(?P<flight>[A-Z]{2}\d{4})/` +
			`(?P<day1>\d{2})/(?P<day2>\d{2})/(?P<route>[A-Z]{6})/(?P<time>\d{4}Z)/` +
			`(?P<unknown1>[^/]+)/(?P<position>[^/]+)/(?P<fl>{HEADING})/(?P<phase>[A-Z]+)/` +
			`(?P<fuel_remain>\d{4})/(?P<fuel_used>\d{4})/(?P<mach>M\d{2})/` +
			`(?P<wind>\d{6})/(?P<heading>{TIME4})/(?P<field1>[^/]+)/(?P<field2>[^/]+)/` +
			`(?P<eta>\d{4}|\-{4}|\*{4})/(?P<sched>\d{4}|\-{4}|\*{4})`,
		Fields: []string{"flight", "day1", "day2", "route", "time", "unknown1", "position",
			"fl", "phase", "fuel_remain", "fuel_used", "mach", "wind", "heading",
			"field1", "field2", "eta", "sched"},
	},
	// Position extraction pattern (for parsing position field).
	// Example: 3457.3N07711.0W
	// Groups: lat, lat_dir, lon, lon_dir
	{
		Name:    "position",
		Pattern: `(?P<lat>\d{4}\.\d)(?P<lat_dir>{LAT_DIR})(?P<lon>\d{5}\.\d)(?P<lon_dir>{LON_DIR})`,
		Fields:  []string{"lat", "lat_dir", "lon", "lon_dir"},
	},
}
