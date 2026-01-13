// Package h2wind provides grok-style pattern definitions for H2 wind message parsing.
package h2wind

import "acars_parser/internal/patterns"

// Formats defines the known H2 wind message formats.
var Formats = []patterns.Format{
	// Header: 02A (climb/descend)
	// Example (from logs):
	//   02A251038BKPRLOWWN42333E021013251018 ...
	// Groups: time, origin, dest, lat_dir, lat, lon_dir, lon, datetime
	{
		Name: "h2_header_02A",
		Pattern: `^02A(?P<time>{TIME6})(?P<origin>{ICAO})(?P<dest>{ICAO})` +
			`(?P<lat_dir>{LAT_DIR})(?P<lat>{LAT_5D})` +
			`(?P<lon_dir>{LON_DIR})(?P<lon>{LON_6D})(?P<datetime>{TIME6})`,
		Fields: []string{"time", "origin", "dest", "lat_dir", "lat", "lon_dir", "lon", "datetime"},
	},
	// Header: 02E (cruise)
	// Example (from logs):
	//   02E25LGAVLKPRN40359E02333410133599M522276069G ...
	// Groups: day, origin, dest, lat_dir, lat, lon_dir, lon, eta, fl, temp_sign, temp, wind_dir, wind_spd, gust
	{
		Name: "h2_header_02E",
		Pattern: `^02E(?P<day>\d{2})(?P<origin>{ICAO})(?P<dest>{ICAO})` +
			`(?P<lat_dir>{LAT_DIR})(?P<lat>{LAT_5D})` +
			`(?P<lon_dir>{LON_DIR})(?P<lon>{LON_6D})` +
			`(?P<eta>\d{4})(?P<fl>\d{3,4})(?P<temp_sign>{TEMP_SIGN})(?P<temp>\d{3})` +
			`(?P<wind_dir>\d{3})(?P<wind_spd>\d{3})(?P<gust>G?)`,
		Fields: []string{"day", "origin", "dest", "lat_dir", "lat", "lon_dir", "lon", "eta", "fl", "temp_sign", "temp", "wind_dir", "wind_spd", "gust"},
	},
	// Wind layer pattern (for repeated matching).
	// Example: 350M045270095G
	// Groups: fl, temp_sign, temp, wind_dir, wind_spd, gust
	{
		Name: "wind_layer",
		Pattern: `(?P<fl>\d{1,3})(?P<temp_sign>{TEMP_SIGN})(?P<temp>{TEMP})` +
			`(?:(?P<wind_dir>{WIND_DIR})(?P<wind_spd>{WIND_SPD})(?P<gust>G)?)?`,
		Fields: []string{"fl", "temp_sign", "temp", "wind_dir", "wind_spd", "gust"},
	},
}
