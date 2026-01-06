// Package label22 provides grok-style pattern definitions for Label 22 message parsing.
package label22

import "acars_parser/internal/patterns"

// Formats defines the known Label 22 message formats.
var Formats = []patterns.Format{
	// DMS position format.
	// Example: N 325338W 971058,rest_of_data
	// Groups: lat_dir, lat, lon_dir, lon, rest
	{
		Name: "dms_position",
		Pattern: `^(?P<lat_dir>{LAT_DIR})\s*(?P<lat>{LAT_DMS})` +
			`(?P<lon_dir>{LON_DIR})\s*(?P<lon>{LON_DMS}),(?P<rest>.*)`,
		Fields: []string{"lat_dir", "lat", "lon_dir", "lon", "rest"},
	},
}
