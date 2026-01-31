// Package mediaadv provides grok-style pattern definitions for Media Advisory parsing.
package mediaadv

import "acars_parser/internal/patterns"

// Formats defines the known Media Advisory message formats.
// Based on libacars media-adv implementation.
var Formats = []patterns.Format{
	// Media Advisory format version 0.
	// Format: 0[E/L][link][HHMMSS][available_links][/optional_text]
	// Example: 0EV032922VS - Established, VHF, 03:29:22, VHF+SATCOM available
	// Example: 0LH080047VS/ - Lost, HF, 08:00:47, VHF+SATCOM available, no text
	{
		Name: "media_advisory_v0",
		Pattern: `^0(?P<state>[EL])(?P<current_link>[VSHGC2XI])` +
			`(?P<time>{TIME6})` +
			`(?P<available>[VSHGC2XI]*)` +
			`(?:/(?P<text>.*))?$`,
		Fields: []string{"state", "current_link", "time", "available", "text"},
	},
}