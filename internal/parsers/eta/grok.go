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
	// B6 landing data request (United).
	// Example: /B6 LDG DATA REQ   / KIAD KBOS 31 182755 KBOS R22L/---- F30 G1460
	// Groups: origin, dest, day, time, runway (the requested landing runway).
	// The trailing fields (F30, G1460) are not extracted: their meaning is not
	// established from the traffic.
	{
		Name: "b6_ldg_data",
		Pattern: `/B6\s+LDG\s+DATA\s+REQ\s*/\s*(?P<origin>{ICAO})\s+(?P<dest>{ICAO})\s+` +
			`(?P<day>\d{2})\s+(?P<time>{TIME6})\s+{ICAO}\s+R(?P<runway>\d{1,2}[LRC]?)\s*/`,
		Fields: []string{"origin", "dest", "day", "time", "runway"},
	},
	// OS format.
	// Example: /OS YSSY/YMML 123456
	// Groups: origin, dest, time
	// TODO: No message in the January 2026 corpus (11.7M messages) contains
	// "/OS ", so this format is unverified against real traffic. Confirm it
	// from a real example, or remove it.
	{
		Name:    "os_format",
		Pattern: `/OS\s+(?P<origin>{ICAO})\s*/(?P<dest>{ICAO})\s*(?P<time>{TIME6})?`,
		Fields:  []string{"origin", "dest", "time"},
	},
	// C3 gate request (United).
	// Example: /C3 GATE REQ       / KEWR KATL 17 040207 1287 ---- ---- ---- ----
	// Groups: origin, dest, day, time. The four-digit number after the time is
	// not extracted: it is probably the flight number, but that is not proven.
	{
		Name: "c3_route",
		Pattern: `/C3\s+GATE\s+REQ\s*/\s*(?P<origin>{ICAO})\s+(?P<dest>{ICAO})\s+` +
			`(?P<day>\d{2})\s+(?P<time>{TIME6})`,
		Fields: []string{"origin", "dest", "day", "time"},
	},
	// Any other United downlink with the same header: a two-character code,
	// a title padded to the "/", the route, the day and the time. It comes
	// after the specific formats, which extract more.
	// Example: /R3 HOWGOZIT REQ   / KEWR KMCO 19 152736 1739 19 KEWR
	// Groups: code, origin, dest, day, time. The rest is not parsed.
	{
		Name: "united_header",
		Pattern: `^/(?P<code>[0-9A-Z]{2}) [A-Z0-9 ]{14,16}/ (?P<origin>{ICAO}) (?P<dest>{ICAO}) ` +
			`(?P<day>\d{2}) (?P<time>{TIME6})`,
		Fields: []string{"code", "origin", "dest", "day", "time"},
	},
}
