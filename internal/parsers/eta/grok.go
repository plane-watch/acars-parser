// Package eta provides grok-style pattern definitions for ETA/timing message parsing.
package eta

import "acars_parser/internal/patterns"

// Formats defines the known ETA message formats.
var Formats = []patterns.Format{
	// ET EXP TIME format.
	// Example: /ET EXP TIME / YSSY YMML 29 123456/EON 1530 AUTO
	// Groups: origin, dest, day, time, eta, mode
	{
		Name: "et_exp_time",
		Pattern: `/ET\s+EXP\s+TIME\s+/\s*(?P<origin>{ICAO})\s+(?P<dest>{ICAO})\s+` +
			`(?P<day>\d{2})\s+(?P<time>{TIME6})/EON\s+(?P<eta>{TIME4})(?:\s+(?P<mode>\w+))?`,
		Fields: []string{"origin", "dest", "day", "time", "eta", "mode"},
	},
	// IR format.
	// Example: /IR QFA123/.../ETA 1530
	// Groups: flight, eta
	{
		Name:    "ir_format",
		Pattern: `/IR\s+(?P<flight>[A-Z]{3}\d+)/.*?/ETA\s+(?P<eta>{TIME4})`,
		Fields:  []string{"flight", "eta"},
	},
	// B6 LDG DATA REQ format.
	// Example: /B6 LDG DATA REQ/YMML 1530 00/RWY 16R/GATE A12
	// Groups: dest, eta, runway, gate
	{
		Name: "b6_ldg_data",
		Pattern: `/B6\s+LDG\s+DATA\s+REQ/(?P<dest>{ICAO})\s+(?P<eta>{TIME4})` +
			`(?:\s+\d{2})?/RWY\s*(?P<runway>{RUNWAY})(?:/GATE\s*(?P<gate>[A-Z0-9]+))?`,
		Fields: []string{"dest", "eta", "runway", "gate"},
	},
	// OS format.
	// Example: /OS YSSY/YMML 123456
	// Groups: origin, dest, time
	{
		Name:    "os_format",
		Pattern: `/OS\s+(?P<origin>{ICAO})\s*/(?P<dest>{ICAO})\s*(?P<time>{TIME6})?`,
		Fields:  []string{"origin", "dest", "time"},
	},
	// C3 route format.
	// Example: /C3 YSSY.YMML
	// Groups: origin, dest
	{
		Name:    "c3_route",
		Pattern: `/C3\s+(?P<origin>{ICAO})\s*\.(?P<dest>{ICAO})`,
		Fields:  []string{"origin", "dest"},
	},
}
