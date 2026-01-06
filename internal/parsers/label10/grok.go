// Package label10 provides grok-style pattern definitions for Label 10 message parsing.
package label10

import "acars_parser/internal/patterns"

// Formats defines the known Label 10 message formats.
var Formats = []patterns.Format{
	// Rich position format with slash-delimited fields.
	// Example: /N33.123/W117.456/10/0.84/270/350/KLAX/1234/12000/500/WAYP1/1230/WAYP2/1245
	// Groups: lat_dir, lat, lon_dir, lon, rest
	{
		Name: "rich_position",
		Pattern: `^/(?P<lat_dir>{LAT_DIR})(?P<lat>[\d.]+)/` +
			`(?P<lon_dir>{LON_DIR})(?P<lon>[\d.]+)/(?P<rest>.*)`,
		Fields: []string{"lat_dir", "lat", "lon_dir", "lon", "rest"},
	},
}
