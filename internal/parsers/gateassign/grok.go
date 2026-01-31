// Package gateassign provides grok-style pattern definitions for gate assignment parsing.
package gateassign

import "acars_parser/internal/patterns"

// Formats defines the known gate assignment message formats.
var Formats = []patterns.Format{
	// Simple gate assignment format.
	// Example: GATE ASSIGNMENT: 30
	// Example: GTA01061807\n\tGATE ASSIGNMENT: 30
	{
		Name:    "simple_gate",
		Pattern: `GATE\s+ASSIGNMENT[:\s]+(?P<gate>[A-Z]?\d+)`,
		Fields:  []string{"gate"},
	},
	// IN-RANGE gate assignment format.
	// Example: IWA GATE 6 ASSIGNED
	{
		Name:    "in_range_gate",
		Pattern: `(?P<station>[A-Z]{3})\s+GATE\s+(?P<gate>\d+)\s+ASSIGNED`,
		Fields:  []string{"station", "gate"},
	},
	// Structured multi-line format (LATAM style).
	// Example:
	//   GATE ASSIGNMENT
	//
	//   PPOS:124
	//   BAG BELT:205
	//   NEXT LEG: LA3309 CNF-GRU 05JAN 16:10Z
	{
		Name: "structured_gate",
		Pattern: `GATE\s+ASSIGNMENT\s*\n` +
			`\s*\n` +
			`PPOS:(?P<ppos>[^\n]*)\n` +
			`BAG\s+BELT:(?P<bagbelt>[^\n]*)\n` +
			`(?:NEXT\s+LEG:\s*(?P<next_flight>\w+)\s+(?P<next_route>[A-Z]{3}-[A-Z]{3}))?`,
		Fields: []string{"ppos", "bagbelt", "next_flight", "next_route"},
	},
}