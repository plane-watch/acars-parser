// Package label80 provides grok-style pattern definitions for Label 80 message parsing.
package label80

import "acars_parser/internal/patterns"

// Formats defines the known Label 80 message formats.
var Formats = []patterns.Format{
	// Main header format.
	// Example: 01 POS VH-ABC YSSY/YMML .VH-ABC
	// Groups: msg_type, origin, dest, tail
	{
		Name: "header_format",
		Pattern: `\d+\s+(?P<msg_type>\w+)\s+\S+\s+` +
			`(?P<origin>{ICAO})/(?P<dest>{ICAO})\s+\.?(?P<tail>\S+)`,
		Fields: []string{"msg_type", "origin", "dest", "tail"},
	},
	// Alternative header format.
	// Example: QFA123,YSSY,YMML
	// Groups: flight, origin, dest
	{
		Name: "alt_format",
		Pattern: `^(?P<flight>[A-Z0-9]+),(?P<origin>{ICAO}),(?P<dest>{ICAO})`,
		Fields: []string{"flight", "origin", "dest"},
	},
	// Position extraction pattern.
	// Example: /POS N33.5/W117.5
	// Groups: lat_dir, lat, lon_dir, lon
	{
		Name: "position",
		Pattern: `/POS\s+(?P<lat_dir>{LAT_DIR})(?P<lat>\d+\.?\d*)[/\s]*` +
			`(?P<lon_dir>{LON_DIR})(?P<lon>\d+\.?\d*)`,
		Fields: []string{"lat_dir", "lat", "lon_dir", "lon"},
	},
	// Altitude/FL extraction.
	{
		Name:    "altitude",
		Pattern: `/(?:ALT|FL)\s+\+?(?P<altitude>\d+)`,
		Fields:  []string{"altitude"},
	},
	// Mach number extraction.
	{
		Name:    "mach",
		Pattern: `/MCH\s+(?P<mach>\d+)`,
		Fields:  []string{"mach"},
	},
	// TAS extraction.
	{
		Name:    "tas",
		Pattern: `/TAS\s+(?P<tas>\d+)`,
		Fields:  []string{"tas"},
	},
	// Fuel on board extraction.
	{
		Name:    "fob",
		Pattern: `/FOB\s+[N]?(?P<fob>\d+)`,
		Fields:  []string{"fob"},
	},
	// ETA extraction.
	{
		Name:    "eta",
		Pattern: `/ETA\s+(?P<eta>\d+[:\.]?\d*)`,
		Fields:  []string{"eta"},
	},
	// OUT time extraction.
	{
		Name:    "out_time",
		Pattern: `/OUT\s+(?P<out>\d+)`,
		Fields:  []string{"out"},
	},
	// OFF time extraction.
	{
		Name:    "off_time",
		Pattern: `/OFF\s+(?P<off>\d+)`,
		Fields:  []string{"off"},
	},
	// ON time extraction.
	{
		Name:    "on_time",
		Pattern: `/ON\s+(?P<on>\d+)`,
		Fields:  []string{"on"},
	},
	// IN time extraction.
	{
		Name:    "in_time",
		Pattern: `/IN\s+(?P<in>\d+)`,
		Fields:  []string{"in"},
	},
}
