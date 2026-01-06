// Package labelb3 provides grok-style pattern definitions for Label B3 message parsing.
package labelb3

import "acars_parser/internal/patterns"

// Formats defines the known Label B3 message formats.
var Formats = []patterns.Format{
	// Gate info header format.
	// Example: QFA123-YSSY-GATE A12-YMML
	// Groups: flight, origin, gate, dest
	{
		Name: "gate_info",
		Pattern: `(?P<flight>[A-Z0-9]+)-(?P<origin>{ICAO})-GATE\s+(?P<gate>\S+)-(?P<dest>{ICAO})`,
		Fields: []string{"flight", "origin", "gate", "dest"},
	},
	// ATIS extraction pattern.
	{
		Name:    "atis",
		Pattern: `ATIS\s+(?P<atis>{ATIS})`,
		Fields:  []string{"atis"},
	},
	// Aircraft type extraction pattern.
	{
		Name:    "aircraft_type",
		Pattern: `-TYP/(?P<aircraft>[A-Z0-9]+)`,
		Fields:  []string{"aircraft"},
	},
}
