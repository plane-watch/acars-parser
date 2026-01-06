// Package label21 provides grok-style pattern definitions for Label 21 message parsing.
package label21

import "acars_parser/internal/patterns"

// Formats defines the known Label 21 message formats.
var Formats = []patterns.Format{
	// POSN position report format.
	// Example: POSN -33.123E151.456, 180,1234,35000,12345, 270 045,  -52,1530,YSSY
	// Groups: lat, lon_dir, lon, heading, time, altitude, fob, wind, temp, eta, dest
	{
		Name: "posn_report",
		Pattern: `POSN\s+(?P<lat>{LAT_DEC})(?P<lon_dir>{LON_DIR})(?P<lon>[\d.]+),\s*` +
			`(?P<heading>\d+),(?P<time>\d+),(?P<altitude>\d+),(?P<fob>\d+),\s*` +
			`(?P<wind>[-\d ]+),\s*(?P<temp>[-\d ]+),(?P<eta>\d+),(?P<dest>{ICAO})`,
		Fields: []string{"lat", "lon_dir", "lon", "heading", "time", "altitude", "fob", "wind", "temp", "eta", "dest"},
	},
}
