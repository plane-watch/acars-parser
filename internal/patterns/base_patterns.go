// Package patterns provides shared regex patterns and helper functions for ACARS parsing.
// This file contains grok-style base patterns for use with the Compiler.

package patterns

// BasePatterns defines reusable regex components for grok-style pattern composition.
// These are referenced in format patterns using {PATTERN_NAME} syntax.
var BasePatterns = map[string]string{
	// Airport codes.
	"ICAO": `[KYEPCZLRVOSWUABDFGHMNT][A-Z]{3}`,
	"IATA": `[A-Z]{3}`,

	// Flight identifiers.
	// Allows 2-3 letter ICAO code + 1-4 digit flight number + 0-2 optional letter suffix.
	// e.g., JST501, DAL1260, FIN5LA, QTR58U
	"FLIGHT": `[A-Z]{2,3}\d{1,4}[A-Z]{0,2}`,

	// Time formats.
	"TIME4":  `\d{4}`,              // HHMM
	"TIME6":  `\d{6}`,              // HHMMSS
	"TIMEZ":  `\d{4}Z`,             // HHMM with Z suffix
	"DATE6":  `\d{6}`,              // DDMMYY or YYMMDD
	"DAYHH":  `\d{2}[A-Z]{3}\s+\d{4}Z`, // 29DEC 1827Z

	// Coordinates - latitude formats.
	"LAT_DIR":  `[NS]`,
	"LAT_2D":   `\d{2}`,            // DD (degrees only)
	"LAT_4D":   `\d{4}`,            // DDMM
	"LAT_5D":   `\d{5}`,            // DDMMD (tenths of minutes)
	"LAT_6D":   `\d{6}`,            // DDMMSS
	"LAT_DM":   `\d{4}\.\d`,        // DDMM.D (degrees minutes decimal)
	"LAT_DMS":  `\d{6}`,            // DDMMSS
	"LAT_DEC":  `[-\d.]+`,          // Decimal latitude

	// Coordinates - longitude formats.
	"LON_DIR":  `[EW]`,
	"LON_3D":   `\d{3}`,            // DDD (degrees only)
	"LON_5D":   `\d{5}`,            // DDDMM
	"LON_6D":   `\d{6}`,            // DDDMMD (tenths of minutes)
	"LON_7D":   `\d{7}`,            // DDDMMSS
	"LON_DM":   `\d{5}\.\d`,        // DDDMM.D (degrees minutes decimal)
	"LON_DMS":  `\d{6,7}`,          // DDMMSS or DDDMMSS
	"LON_DEC":  `[-\d.]+`,          // Decimal longitude

	// Altitude and flight level.
	"FL":       `\d{2,3}`,          // Flight level digits (e.g., 350, 41)
	"ALT":      `\d{3,6}`,          // Altitude in feet (e.g., 350, 35000)
	"ALTITUDE": `\d{3,5}`,          // General altitude

	// Navigation.
	"HEADING":  `\d{3}`,            // 3-digit heading (000-360)
	"SPEED":    `\d{2,4}`,          // Ground/air speed
	"MACH":     `\d{2,3}`,          // Mach number digits (e.g., 84, 840)
	"WAYPOINT": `[A-Z][A-Z0-9]{1,5}`, // Waypoint/navaid name

	// Weather.
	"TEMP_SIGN": `[MP]`,            // Minus/Plus for temperature
	"TEMP":      `\d{1,3}`,         // Temperature digits
	"WIND_DIR":  `\d{3}`,           // Wind direction (000-360)
	"WIND_SPD":  `\d{2,3}`,         // Wind speed

	// Clearance data.
	"SQUAWK":   `[0-7]{4}`,
	"RUNWAY":   `\d{1,2}[LRC]?`,
	"FREQ":     `\d{3}\.\d{1,3}`,

	// Aircraft types - ICAO format.
	// letter + 2-3 digits + optional letter suffix (A320, B738, A21N, E190).
	"AIRCRAFT": `[A-Z]\d{2,3}[A-Z]?`,

	// SID/STAR names - letters followed by digit and optional alphanumerics.
	"SID": `[A-Z]{2,}[0-9][A-Z0-9]*`,

	// Misc.
	"ATIS":     `[A-Z]`,
	"PDCNUM":   `\d{1,6}`,
	"CALLSIGN": `[A-Z0-9]{3,8}`,    // Generic callsign
	"TAIL":     `[A-Z0-9-]{4,8}`,   // Aircraft registration/tail
}
