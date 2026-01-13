// Package label17 provides grok-style pattern definitions for Label 17 messages.
package label17

import "acars_parser/internal/patterns"

// Formats defines the known Label 17 message formats.
//
// Example:
//   031324,37995,0413, 7360,N 46.943,E 18.634,06OCT25,25680, 19,- 47
//
// Fields:
//   time(HHMMSS), altitude_ft, ground_speed_kts, track_code(deg*100), lat_dir, lat, lon_dir, lon,
//   date(DDMMMYY), wind_dir_code(deg*100), wind_speed_kts, temp_sign, temp
var Formats = []patterns.Format{
	{
		Name: "label17_csv",
		Pattern: `^\s*(?P<time>\d{6})\s*,\s*` +
			`(?P<altitude_ft>\d{4,5})\s*,\s*` +
			`(?P<ground_speed_kts>\d{3,4})\s*,\s*` +
			`(?P<track_code>\d{3,5})\s*,\s*` +
			`(?P<lat_dir>[NS])\s*(?P<lat>\d{1,2}\.\d+)\s*,\s*` +
			`(?P<lon_dir>[EW])\s*(?P<lon>\d{1,3}\.\d+)\s*,\s*` +
			`(?P<date>\d{2}[A-Z]{3}\d{2})\s*` +
			`(?:,\s*(?P<wind_dir_code>\d{4,5})\s*,\s*` +
			`(?P<wind_speed_kts>\d{1,3})\s*,\s*` +
			`(?P<temp_sign>-)?\s*(?P<temperature_c>\d{1,2})\s*)?\s*$`,
		Fields: []string{
			"time", "altitude_ft", "ground_speed_kts", "track_code",
			"lat_dir", "lat", "lon_dir", "lon",
			"date", "wind_dir_code", "wind_speed_kts",
			"temp_sign", "temperature_c",
		},
	},
}
