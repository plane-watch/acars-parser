// Package labelb2 provides grok-style pattern definitions for Label B2 message parsing.
package labelb2

import "acars_parser/internal/patterns"

// Formats defines the known Label B2 message formats.
var Formats = []patterns.Format{
	// Oceanic clearance destination format.
	// Example: CLRD TO EGLL
	{
		Name:    "oceanic_dest",
		Pattern: `CLRD TO (?P<dest>\w{4})`,
		Fields:  []string{"dest"},
	},
	// Oceanic fix format (lat/lon waypoint).
	// Example: 50N030W, 51N040W
	{
		Name:    "oceanic_fix",
		Pattern: `(?P<fix>\d{2}[NS]\d{3}[EW])`,
		Fields:  []string{"fix"},
	},
	// Flight level format.
	// Example: F350
	{
		Name:    "flight_level",
		Pattern: `F(?P<fl>\d{3})`,
		Fields:  []string{"fl"},
	},
	// Mach number format.
	// Example: M84, M084
	{
		Name:    "mach",
		Pattern: `M0?(?P<mach>\d{2,3})`,
		Fields:  []string{"mach"},
	},
	// Flight number from beginning of line.
	// Example: DAL123
	{
		Name:    "flight_num",
		Pattern: `^(?P<flight>[A-Z]{3}\d+)`,
		Fields:  []string{"flight"},
	},
}
