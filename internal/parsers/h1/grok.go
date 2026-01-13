// Package h1 provides grok-style pattern definitions for H1 message parsing.
package h1

import "acars_parser/internal/patterns"

// Formats defines the known H1 message formats.
// Note: FPN (flight plan) parsing now uses the tokeniser (tokeniser.go) instead of grok patterns.
var Formats = []patterns.Format{
	// H1 POS position format with time (6-digit) - most common format.
	// Example: POSN53139W001524,RODOL,173054,320,MCT,173303,ASNIP,M56,29442,2092BA73
	// Fields: position, waypoint, time (HHMMSS), altitude (FL in hundreds), next waypoint, ETA, third waypoint, temp, extra fields
	// Note: Ground speed appears later in extended variants, not in this position.
	// Waypoint fields can be empty (empty string between commas) or contain coordinates.
	{
		Name: "h1_position_time",
		Pattern: `^POS(?P<lat_dir>{LAT_DIR})(?P<lat>\d{5})(?P<lon_dir>{LON_DIR})(?P<lon>\d{6}),` +
			`(?P<curr_wpt>[A-Z0-9/-]*),(?P<report_time>\d{6}),(?P<altitude>\d+),` +
			`(?P<next_wpt>[A-Z0-9/-]*),(?P<eta>\d+),(?P<wpt3>[A-Z0-9/-]*),(?P<temp>[MP]\d+)` +
			// Wind field is usually DDDSS (5 digits), but some feeds use DDDSSS
			// with a leading zero (e.g. 255044 -> 255/44kts). Accept 5 or 6 digits.
			// Allow multiple additional comma-separated fields after temp
			`(?:,(?P<wind>\d{5,6}))?(?:,(?P<extra>.+))?$`,
		Fields: []string{"lat_dir", "lat", "lon_dir", "lon", "curr_wpt", "report_time", "altitude", "next_wpt", "eta", "wpt3", "temp", "wind", "extra"},
	},
	// H1 POS position format with altitude (3-digit FL) - alternate format.
	// Example: POSN33520E151180,WAYP1,350,450,WAYP2,1234,WAYP3,M52
	// Fields: position, waypoint, altitude (FL), ground speed, next waypoint, ETA, third waypoint, temp
	// Waypoint fields can be empty (empty string between commas) or contain coordinates.
	{
		Name: "h1_position_alt",
		Pattern: `^POS(?P<lat_dir>{LAT_DIR})(?P<lat>\d{5})(?P<lon_dir>{LON_DIR})(?P<lon>\d{6}),` +
			`(?P<curr_wpt>[A-Z0-9/-]*),(?P<altitude>\d{3}),(?P<gs>\d+),` +
			`(?P<next_wpt>[A-Z0-9/-]*),(?P<eta>\d+),(?P<wpt3>[A-Z0-9/-]*),(?P<temp>[MP]\d+)`,
		Fields: []string{"lat_dir", "lat", "lon_dir", "lon", "curr_wpt", "altitude", "gs", "next_wpt", "eta", "wpt3", "temp"},
	},
}
