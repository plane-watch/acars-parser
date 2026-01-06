// Package label5l provides grok-style pattern definitions for Label 5L message parsing.
package label5l

import "acars_parser/internal/patterns"

// Formats defines the known Label 5L message formats.
var Formats = []patterns.Format{
	// Route format (comma-delimited).
	// Example: .QFA123,.VH-ABC,SYD,YSSY,MEL,YMML,FLT001,311224,0800,0805,0930,0925
	// Groups: callsign, tail, origin_iata, origin_icao, dest_iata, dest_icao, flight_id, date, dep_sched, dep_actual, arr_sched, arr_actual
	{
		Name: "route",
		Pattern: `^\.?(?P<callsign>[A-Z0-9]+),\.?(?P<tail>[A-Z0-9-]+),` +
			`(?P<origin_iata>[A-Z]{3}),(?P<origin_icao>{ICAO}),` +
			`(?P<dest_iata>[A-Z-]{3}),(?P<dest_icao>{ICAO})` +
			`(?:,(?P<flight_id>[^,]*))?` +
			`(?:,(?P<date>[^,]*))?` +
			`(?:,(?P<dep_sched>[^,]*))?` +
			`(?:,(?P<dep_actual>[^,]*))?` +
			`(?:,(?P<arr_sched>[^,]*))?` +
			`(?:,(?P<arr_actual>[^,]*))?`,
		Fields: []string{"callsign", "tail", "origin_iata", "origin_icao", "dest_iata", "dest_icao",
			"flight_id", "date", "dep_sched", "dep_actual", "arr_sched", "arr_actual"},
	},
}
