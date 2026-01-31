// Package labelrf provides grok-style pattern definitions for Label RF flight subscription parsing.
package labelrf

import "acars_parser/internal/patterns"

// Formats defines the known Label RF message formats.
// These are SITA FDA (Flight Data Application) messages with comma-delimited fields.
var Formats = []patterns.Format{
	// Flight subscription format (FDASUB, FDACOM, FSTREQ, FDAACK).
	// Format: ,<msg_type><timestamp>,<origin>,,<dest>,,<flight>,<date>,<time>,<type>,,<reg>,/<data>
	// Example: ,FDASUB01260107143821,SIN,,LHR,,QF001,260107,1535,388,,VH-OQI,/358301,349,,1,4EC0
	{
		Name: "flight_subscription",
		Pattern: `^,(?P<msg_type>FD(?:ASUB|ACOM|AACK)|FSTREQ)\d+,` +
			`(?P<origin>[A-Z]{3}),,` +
			`(?P<dest>[A-Z]{3}),,` +
			`(?P<flight>{FLIGHT}),` +
			`(?P<date>\d{6}),` +
			`(?P<time>\d{4}),` +
			`(?P<actype>[^,]*),` +
			`,` +
			`(?P<reg>[^,]*),`,
		Fields: []string{"msg_type", "origin", "dest", "flight", "date", "time", "actype", "reg"},
	},
}
