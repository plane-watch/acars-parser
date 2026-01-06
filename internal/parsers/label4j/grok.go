// Package label4j provides grok-style pattern definitions for Label 4J message parsing.
package label4j

import "acars_parser/internal/patterns"

// Formats defines the known Label 4J message formats.
var Formats = []patterns.Format{
	// Position + weather format.
	// Example: POS/PSN50028W123456,123456,180,WAYP1,1234,WAYP2,M52,35000
	// Groups: lat, lon_dir, lon, time, heading, curr_wpt, eta, next_wpt, temp, altitude
	{
		Name: "pos_weather",
		Pattern: `/PSN(?P<lat>{LAT_5D})(?P<lon_dir>{LON_DIR})(?P<lon>{LON_6D}),` +
			`(?P<time>{TIME6}),(?P<heading>\d+),(?P<curr_wpt>[A-Z0-9]+),` +
			`(?P<eta>\d+),(?P<next_wpt>[A-Z0-9]+),(?P<temp>{TEMP_SIGN}\d+),(?P<altitude>\d+)`,
		Fields: []string{"lat", "lon_dir", "lon", "time", "heading", "curr_wpt", "eta", "next_wpt", "temp", "altitude"},
	},
	// Fuel burn extraction pattern.
	{
		Name:    "fuel_burn",
		Pattern: `/FB(?P<fuel_burn>\d+)`,
		Fields:  []string{"fuel_burn"},
	},
}
