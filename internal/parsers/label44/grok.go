// Package label44 provides grok-style pattern definitions for Label 44 message parsing.
package label44

import "acars_parser/internal/patterns"

// Formats defines the known Label 44 message formats.
var Formats = []patterns.Format{
	// Runway info header format.
	// Example: YSSY T/O RWY,16 12500
	// Groups: airport, runway
	{
		Name:    "runway_header",
		Pattern: `^(?P<airport>{ICAO})\s+T/O\s+RWYS?,(?P<runway>\d{2})`,
		Fields:  []string{"airport", "runway"},
	},
	// FB (Flight Brief) position format.
	// Example: /FB 01/AD YSSY/S 33.50,E 151.30,QFA123,INA03,YMML,1234
	// Groups: fb, airport, lat_dir, lat, lon_dir, lon, callsign, unknown, dest, time
	{
		Name: "fb_position",
		Pattern: `/FB\s*(?P<fb>\d+)/AD\s*(?P<airport>{ICAO})/` +
			`(?P<lat_dir>{LAT_DIR})\s*(?P<lat>[\d.]+),` +
			`(?P<lon_dir>{LON_DIR})\s*(?P<lon>[\d.]+),` +
			`(?P<callsign>[A-Z0-9]+),(?P<unknown>[^,]+),(?P<dest>{ICAO}),(?P<time>{TIME4})`,
		Fields: []string{"fb", "airport", "lat_dir", "lat", "lon_dir", "lon", "callsign", "unknown", "dest", "time"},
	},
	// POS position report format.
	// Example: POS01,S33561E151234,350,YSSY,YMML,1234,1530
	// Groups: unknown, lat_dir, lat, lon_dir, lon, fl, origin, dest, time1, time2
	{
		Name: "pos_report",
		Pattern: `^POS(?P<unknown>\d{2}),(?P<lat_dir>{LAT_DIR})(?P<lat>\d{5,6})` +
			`(?P<lon_dir>{LON_DIR})(?P<lon>\d{5,6}),(?P<fl>\d{1,3}),` +
			`(?P<origin>{ICAO}),(?P<dest>{ICAO}),(?P<time1>{TIME4}),(?P<time2>{TIME4})`,
		Fields: []string{"unknown", "lat_dir", "lat", "lon_dir", "lon", "fl", "origin", "dest", "time1", "time2"},
	},
	// Individual runway line pattern.
	// Groups: runway, suffix, distance
	{
		Name:    "runway_line",
		Pattern: `^(?P<runway>\d{2})(?:/(?P<suffix>[A-Z0-9]+))?\s+.*?(?P<distance>\d{4,5})\s*$`,
		Fields:  []string{"runway", "suffix", "distance"},
	},
}
