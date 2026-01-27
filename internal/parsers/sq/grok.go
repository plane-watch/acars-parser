// Package sq provides grok-style pattern definitions for SQ message parsing.
package sq

import "acars_parser/internal/patterns"

// Formats defines the known SQ message formats.
var Formats = []patterns.Format{
	// ARINC squitter position format.
	// Example: 02XASYDYSSY03341S14959EV136975...
	// Groups: msg_type, iata, icao, lat, lat_dir, lon, lon_dir, freq_band, freq
	{
		Name: "arinc_position",
		Pattern: `^02X(?P<msg_type>[AS])(?P<iata>{IATA})(?P<icao>{ICAO})` +
			`(?P<lat>{LAT_5D})(?P<lat_dir>{LAT_DIR})` +
			`(?P<lon>{LON_5D})(?P<lon_dir>{LON_DIR})` +
			`(?P<freq_band>[VB])(?P<freq>{LON_6D})`,
		Fields: []string{"msg_type", "iata", "icao", "lat", "lat_dir", "lon", "lon_dir", "freq_band", "freq"},
	},
	// AVICOM frequency announcement format.
	// Example: 02JDHNDRJTT23533N13946EV136975/AVICOM
	// D=type, HND=IATA, RJTT=ICAO, coordinates, V=VHF, 136975=136.975MHz
	// Groups: msg_type, iata, icao, lat, lat_dir, lon, lon_dir, freq_band, freq
	{
		Name: "avicom_frequency",
		Pattern: `^02J(?P<msg_type>[A-Z])(?P<iata>{IATA})(?P<icao>{ICAO})` +
			`(?P<lat>{LAT_5D})(?P<lat_dir>{LAT_DIR})` +
			`(?P<lon>{LON_5D})(?P<lon_dir>{LON_DIR})` +
			`(?P<freq_band>[VBT])(?P<freq>\d{6})`,
		Fields: []string{"msg_type", "iata", "icao", "lat", "lat_dir", "lon", "lon_dir", "freq_band", "freq"},
	},
}
