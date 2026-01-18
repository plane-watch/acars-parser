// Package fst provides grok-style pattern definitions for FST (Label 15) message parsing.
package fst

import "acars_parser/internal/patterns"

// Formats defines the known FST message formats.
var Formats = []patterns.Format{
	// FST format with 5-digit longitude (more common for European coordinates).
	// Example: FST01EGGDEGLL51420N00312W...
	// Groups: seq, origin, dest, lat_dir, lat, lon_dir, lon, rest
	{
		Name: "fst_5digit_lon",
		Pattern: `FST(?P<seq>\d{2})(?P<origin>{ICAO})(?P<dest>{ICAO})` +
			`(?P<lat_dir>[NS])(?P<lat>\d{5,7})` +
			`(?P<lon_dir>[EW])(?P<lon>\d{5,7})(?P<rest>\d.+)`,
		Fields: []string{"seq", "origin", "dest", "lat_dir", "lat", "lon_dir", "lon", "rest"},
	},
	// FST format with 6-digit longitude.
	// Groups: seq, origin, dest, lat_dir, lat, lon_dir, lon, rest
	{
		Name: "fst_6digit",
		Pattern: `FST(?P<seq>\d{2})(?P<origin>{ICAO})(?P<dest>{ICAO})` +
			`(?P<lat_dir>[NS])(?P<lat>{LON_6D})` +
			`(?P<lon_dir>[EW])(?P<lon>{LON_6D})(?P<rest>.+)`,
		Fields: []string{"seq", "origin", "dest", "lat_dir", "lat", "lon_dir", "lon", "rest"},
	},
}
