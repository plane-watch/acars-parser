// Package label16 provides grok-style pattern definitions for Label 16 message parsing.
package label16

import "acars_parser/internal/patterns"

// Formats defines the known Label 16 message formats.
var Formats = []patterns.Format{
	// CSV position format (most common).
	// Example: 221942,35989,2346, 118,N 47.983 E  9.626
	// Fields: time (HHMMSS), altitude (feet), speed, track, lat_dir, lat, lon_dir, lon
	{
		Name: "csv_position",
		Pattern: `^(?P<time>\d{6}),(?P<altitude>[+M]?\d+),(?P<speed>\d+),\s*(?P<track>\d+),` +
			`(?P<lat_dir>[NS])\s*(?P<lat>[\d.]+)[,\s]+(?P<lon_dir>[EW])\s*(?P<lon>[\d.]+)`,
		Fields: []string{"time", "altitude", "speed", "track", "lat_dir", "lat", "lon_dir", "lon"},
	},
	// CSV position with missing altitude (still has valid coords).
	// Example: 221641,,2249,  84,N 46.753 W122.356
	{
		Name: "csv_position_no_alt",
		Pattern: `^(?P<time>\d{6}),,(?P<speed>\d+),\s*(?P<track>\d+),` +
			`(?P<lat_dir>[NS])\s*(?P<lat>[\d.]+)[,\s]+(?P<lon_dir>[EW])\s*(?P<lon>[\d.]+)`,
		Fields: []string{"time", "speed", "track", "lat_dir", "lat", "lon_dir", "lon"},
	},
	// Extended CSV with flight number.
	// Example: 221737,+20995,2233,9160,N 50.0547,E 8.2408,SXS67A  ,5,7,4,925760,/,
	{
		Name: "csv_position_extended",
		Pattern: `^(?P<time>\d{6}),(?P<altitude>[+M]?\d+),(?P<speed>\d+),(?P<track>\d+),` +
			`(?P<lat_dir>[NS])\s*(?P<lat>[\d.]+),(?P<lon_dir>[EW])\s*(?P<lon>[\d.]+),` +
			`(?P<flight>\w+)`,
		Fields: []string{"time", "altitude", "speed", "track", "lat_dir", "lat", "lon_dir", "lon", "flight"},
	},
	// Waypoint position format with M##A prefix.
	// Format: M{seq:2}A{airline:2}{flight:4}{waypoint}  ,{coords}
	// The M## portion appears to be a message sequence number (purpose unknown).
	// Example: M47AQR8416NUPNI  ,N 34.901,E 100.595,41098,0477,2033,042\TS180219,311225
	//          M## = sequence, A = identifier, QR = airline, 8416 = flight, NUPNI = waypoint
	// Waypoints can be ICAO identifiers (2-5 chars) or lat/lon format (e.g., S15E090).
	{
		Name: "waypoint_position_prefixed",
		Pattern: `^M(?P<msg_seq>\d{2})A(?P<prefix_airline>[A-Z0-9]{2})(?P<prefix_flight>[A-Z0-9]{4})` +
			`(?P<waypoint>[A-Z][A-Z0-9]*)\s*,(?P<lat_dir>[NS])\s*(?P<lat>[\d.]+),` +
			`(?P<lon_dir>[EW])\s*(?P<lon>[\d.]+),(?P<altitude>\d+),\s*(?P<ground_speed>\d+),` +
			`(?P<eta>\d+),\s*(?P<track>\d+)`,
		Fields: []string{"msg_seq", "prefix_airline", "prefix_flight", "waypoint", "lat_dir", "lat", "lon_dir", "lon", "altitude", "ground_speed", "eta", "track"},
	},
	// Plain waypoint position format (no M##A prefix).
	// Example: BEGLA  ,N 47.555,E 18.028,40025,490,1934,030\TS180357,311225
	// Waypoints are typically 2-5 letter ICAO identifiers, but some are numeric (e.g., 10000 on T932 near HKJK).
	// Groups: waypoint, lat_dir, lat, lon_dir, lon, altitude, ground_speed, eta, track
	{
		Name: "waypoint_position",
		Pattern: `^(?P<waypoint>[A-Z0-9]{2,8})\s*,(?P<lat_dir>[NS])\s*(?P<lat>[\d.]+),` +
			`(?P<lon_dir>[EW])\s*(?P<lon>[\d.]+),(?P<altitude>\d+),\s*(?P<ground_speed>\d+),` +
			`(?P<eta>\d+),\s*(?P<track>\d+)`,
		Fields: []string{"waypoint", "lat_dir", "lat", "lon_dir", "lon", "altitude", "ground_speed", "eta", "track"},
	},
	// AUTPOS format.
	// Example: 035234/AUTPOS/LLD N440853 W0915239
	{
		Name:    "autpos",
		Pattern: `^(?P<time>\d{6})/AUTPOS/LLD\s+(?P<lat_dir>[NS])(?P<lat>\d{6})\s+(?P<lon_dir>[EW])(?P<lon>\d{7})`,
		Fields:  []string{"time", "lat_dir", "lat", "lon_dir", "lon"},
	},
}
