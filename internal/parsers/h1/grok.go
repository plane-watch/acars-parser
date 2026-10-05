// Package h1 provides grok-style pattern definitions for H1 message parsing.
package h1

import "acars_parser/internal/patterns"

// h1PosWaypoint is a waypoint in an H1 position report: a named fix, a
// lat/lon point (N58548E016310) or a place, bearing and distance
// (NOLSU196-0022). It starts with a letter: an all-digit value means the
// fields are not where the format expects them. A waypoint may also be
// left empty.
const h1PosWaypoint = `[A-Z][A-Z0-9-]*`

// Formats defines the known H1 message formats.
// Note: FPN (flight plan) parsing now uses the tokeniser (tokeniser.go) instead of grok patterns.
var Formats = []patterns.Format{
	// H1 POS position format with the route, sent mostly by Southwest. It
	// adds a number (perhaps the distance to the next waypoint) before the
	// ETA, and after the wind two speeds ("380K,305K"), two numbers, the
	// origin and destination, and further fields that are not parsed.
	// Example:
	// POSN33005W096222,WIGIS,004904,143,JAYXX,27,005245,TRYTN,M4,281040,380K,305K,1429,162,KDAL,KBWI,,69,...
	// The first waypoint may also be a runway (RW36R) or an altitude point
	// (1000): unlike in h1_position_time, the speeds and route fix the
	// fields' positions, so an all-digit value there is not misaligned.
	{
		Name: "h1_position_route",
		Pattern: `^POS(?P<lat_dir>{LAT_DIR})(?P<lat>\d{5})(?P<lon_dir>{LON_DIR})(?P<lon>\d{6}),` +
			`(?P<curr_wpt>[A-Z0-9][A-Z0-9-]*)?,(?P<report_time>\d{6}),(?P<altitude>\d+),` +
			`(?P<next_wpt>` + h1PosWaypoint + `)?,\d*,(?P<eta>\d{6})?,(?P<wpt3>` + h1PosWaypoint + `)?,(?P<temp>[MP]\d+),` +
			`(?P<wind>\d{5,6}),\d+K,\d+K,\d+,\d+,(?P<origin>{ICAO}),(?P<dest>{ICAO}),`,
		Fields: []string{"lat_dir", "lat", "lon_dir", "lon", "curr_wpt", "report_time", "altitude", "next_wpt", "eta", "wpt3", "temp", "wind", "origin", "dest"},
	},
	// H1 POS position format with time (6-digit) - most common format.
	// Example: POSN53139W001524,RODOL,173054,320,MCT,173303,ASNIP,M56,29442,2092BA73
	// Fields: position, waypoint, time (HHMMSS), altitude (FL in hundreds), next waypoint, ETA, third waypoint, temp, wind, extra fields
	// The waypoints are described at h1PosWaypoint; any of them may be empty.
	// The wind is DDDSS or DDDSSS: three digits of direction, then the speed in knots.
	// Note: Ground speed appears later in extended variants, not in this position.
	{
		Name: "h1_position_time",
		Pattern: `^POS(?P<lat_dir>{LAT_DIR})(?P<lat>\d{5})(?P<lon_dir>{LON_DIR})(?P<lon>\d{6}),` +
			`(?P<curr_wpt>` + h1PosWaypoint + `)?,(?P<report_time>\d{6}),(?P<altitude>\d+),` +
			`(?P<next_wpt>` + h1PosWaypoint + `)?,(?P<eta>\d+)?,(?P<wpt3>` + h1PosWaypoint + `)?,(?P<temp>[MP]\d+)` +
			`(?:,(?P<wind>\d{5,6}))?(?:,(?P<extra>[A-Z0-9]+))?`,
		Fields: []string{"lat_dir", "lat", "lon_dir", "lon", "curr_wpt", "report_time", "altitude", "next_wpt", "eta", "wpt3", "temp", "wind", "extra"},
	},
	// H1 POS position format with altitude (3-digit FL) - alternate format.
	// Example: POSN33520E151180,WAYP1,350,450,WAYP2,1234,WAYP3,M52
	// Fields: position, waypoint, altitude (FL), ground speed, next waypoint, ETA, third waypoint, temp
	{
		Name: "h1_position_alt",
		Pattern: `^POS(?P<lat_dir>{LAT_DIR})(?P<lat>\d{5})(?P<lon_dir>{LON_DIR})(?P<lon>\d{6}),` +
			`(?P<curr_wpt>[A-Z]+),(?P<altitude>\d{3}),(?P<gs>\d+),` +
			`(?P<next_wpt>[A-Z]+),(?P<eta>\d+),(?P<wpt3>[A-Z]+),(?P<temp>[MP]\d+)`,
		Fields: []string{"lat_dir", "lat", "lon_dir", "lon", "curr_wpt", "altitude", "gs", "next_wpt", "eta", "wpt3", "temp"},
	},
}
