// Package pdc provides Grok-like pattern composition for PDC message parsing.
package pdc

import (
	"regexp"
	"strings"
)

// BasePatterns defines reusable regex components for PDC parsing.
// These are referenced in format patterns using {PATTERN_NAME} syntax.
var BasePatterns = map[string]string{
	// Airport codes.
	"ICAO": `[KYEPCZLRVOSWUABDFGHMNT][A-Z]{3}`,
	"IATA": `[A-Z]{3}`,

	// Flight identifiers.
	// Allows 2-5 letter airline code + 1-4 digit flight number + 0-2 optional letter suffix.
	// e.g., JST501, DAL1260, FIN5LA, QTR58U, MEDVC4C
	"FLIGHT": `[A-Z]{2,5}\d{1,4}[A-Z]{0,2}`,

	// Aircraft registrations used as flight identifiers (private/corporate flights).
	// e.g., 9HBCT (Malta), N123AB (US), GBXYZ (UK), VHABC (Australia)
	"REGISTRATION": `[A-Z0-9]{5,6}`,

	// Clearance data.
	"SQUAWK":   `[0-7]{4}`,
	"RUNWAY":   `\d{1,2}[LRC]?`,
	"ALTITUDE": `\d{3,5}`,
	"FREQ":     `\d{3}\.\d{1,3}`,

	// Aircraft types - various formats like A319, B738, E145, CRJ2, CRJ7, A21N, B38M.
	// Pattern: 1-3 letters + 1-3 digits + optional letter suffix.
	"AIRCRAFT": `[A-Z]{1,3}\d{1,3}[A-Z]?`,

	// SID/STAR names - letters followed by digit and optional alphanumerics.
	// Some SIDs have single letter prefix (e.g., P17Y9 in China).
	"SID": `[A-Z]{1,}[0-9][A-Z0-9]*`,

	// Time formats.
	"TIME4": `\d{4}`,            // HHMM
	"DATE":  `\d{6}`,            // DDMMYY or similar
	"DDHH":  `\d{2}[A-Z]{3}\s+\d{4}Z`, // 29DEC 1827Z

	// Misc.
	"ATIS":   `[A-Z]`,
	"PDCNUM": `\d{1,6}`,
}

// PDCFormat represents a specific PDC message format with named capture groups.
type PDCFormat struct {
	Name     string
	Pattern  string         // Pattern with {PLACEHOLDER} syntax
	Compiled *regexp.Regexp // Compiled regex (populated by Compile)
	Fields   []string       // Field names in capture order
}

// PDCFormats defines the known PDC message formats.
// Order matters - more specific patterns should come first.
var PDCFormats = []PDCFormat{
	// Format 0: Compact APCDC format (single line)
	// Example: C32PDC         1APCDC AC0564/31/31 YVR SFO 524 1804Z/0076/0000/  7
	// Fields: flight, origin_iata (IATA), dest_iata (IATA), squawk(?), time
	{
		Name: "compact_apcdc",
		Pattern: `(?:C32)?PDC\s+\d*APCDC\s+` +
			`(?P<flight>[A-Z]{2}\d{3,4})/\d+/\d+\s+` +
			`(?P<origin_iata>{IATA})\s+(?P<dest_iata>{IATA})\s+` +
			`(?P<squawk>\d{3,4})?\s*(\d{4}Z)?`,
		Fields: []string{"flight", "origin_iata", "dest_iata", "squawk"},
	},

	// Format 0b: Australian Regional (Bonza, Rex, etc)
	// Different format from Jetstar/Qantas - uses "CLEARED AS FILED" structure.
	// Example:
	// QUMLBSDCR~1PDC EVY82 B38M/M
	// ETD YSCB 0900UTC
	// FL100
	// CLEARED AS FILED
	// FILED ROUTE: CULIN Y59 RIVET DCT
	// CLEARED TO YSSY VIA CULIN 2 DEP: XXX
	// CLIMB VIA SID TO: 10000
	// ,DPFRQ 124.500
	// SQUAWK 2021
	{
		Name: "australian_regional",
		Pattern: `(?s)PDC\s+(?P<flight>{FLIGHT})\s+(?P<aircraft>{AIRCRAFT})/[A-Z]\s+` +
			`ETD\s+(?P<origin>{ICAO})\s+(?P<dep_time>\d{4})UTC\s+` +
			`FL(?P<flight_level>\d+)\s+` +
			`.*?CLEARED\s+TO\s+(?P<destination>{ICAO})\s+VIA\s+` +
			`(?P<sid>[A-Z]+\s*\d)\s+DEP` +
			`.*?SQUAWK\s+(?P<squawk>{SQUAWK})`,
		Fields: []string{"flight", "aircraft", "origin", "dep_time", "flight_level", "destination", "sid", "squawk"},
	},

	// Format 0c: Virgin Australia word-SID format
	// Uses word-based SID names like "GOLD COAST SEVEN DEP: G7"
	// Example:
	// PDC UPLINK
	// VOZ083 B738 YBCG 0750
	// CLEARED TO WADD VIA
	// GOLD COAST SEVEN DEP: G7
	// ROUTE:SCOTT Q47 IDRAS...
	// CLIMB VIA SID TO: 6000
	// DEP FREQ: 123.500
	// SQUAWK 1074
	{
		Name: "virgin_australia",
		Pattern: `(?s)PDC\s+UPLINK\s+` +
			`(?P<flight>{FLIGHT})\s+(?P<aircraft>{AIRCRAFT})\s+(?P<origin>{ICAO})\s+(?P<dep_time>\d{4})\s+` +
			`CLEARED\s+TO\s+(?P<destination>{ICAO})\s+VIA\s+` +
			`.*?DEP:\s*(?P<sid>[A-Z0-9]+)\s+` +
			`.*?SQUAWK\s+(?P<squawk>{SQUAWK})`,
		Fields: []string{"flight", "aircraft", "origin", "dep_time", "destination", "sid", "squawk"},
	},

	// Format 1: Australian domestic (Jetstar, Qantas, Virgin Australia)
	// Two variants:
	// - "PDC 291826" (Jetstar/Qantas)
	// - "PDC UPLINK" (Virgin Australia)
	// Both followed by: FLIGHT AIRCRAFT ORIGIN TIME
	// SID can be:
	// - Digit form: "ABBEY3", "TANTA 3"
	// - Word form: "SANEG TWO"
	// Runway can be:
	// - Before SID: "16L ABBEY3 DEP"
	// - At end: "XXX EXPECT RUNWAY 01R XXX"
	// Examples:
	// PDC 291826
	// JST501 A320 YSSY 1900
	// CLEARED TO YMML VIA
	// 16L ABBEY3 DEP: XXX
	//
	// PDC UPLINK
	// VOZ252 B738 YSCB 1935
	// CLEARED TO YMML VIA
	// TANTA 3 DEP: XXX
	{
		Name: "australian",
		Pattern: `(?s)PDC\s+(?:UPLINK|{PDCNUM})\s+` +
			`(?P<flight>{FLIGHT})\s+(?P<aircraft>{AIRCRAFT})\s+(?P<origin>{ICAO})\s+(?P<dep_time>{TIME4})\s+` +
			`CLEARED\s+TO\s+(?P<destination>{ICAO})\s+VIA\s+` +
			`(?:(?P<runway>{RUNWAY})\s*)?(?P<sid>[A-Z]+(?:\s*[0-9]|\s+(?:ONE|TWO|THREE|FOUR|FIVE|SIX|SEVEN|EIGHT|NINE))?)?\s*DEP` +
			`(?:.*?EXPECT\s+RUNWAY\s+(?P<runway2>{RUNWAY}))?`,
		Fields: []string{"flight", "aircraft", "origin", "dep_time", "destination", "runway", "sid", "runway2"},
	},

	// Format 2: US Delta
	// 42 PDC 1260 MSP HDN
	// ***DATE/TIME OF PDC RECEIPT: 29DEC 1827Z
	// **** PREDEPARTURE  CLEARANCE ****
	// DAL1260 DEPARTING KMSP  TRANSPONDER 2463
	// SKED DEP TIME 1857   EQUIP  A319/L
	{
		Name: "us_delta",
		Pattern: `(?s)\d+\s+PDC\s+\d+\s+{IATA}\s+{IATA}.*?` +
			`(?P<flight>{FLIGHT})\s+DEPARTING\s+(?P<origin>{ICAO})\s+TRANSPONDER\s+(?P<squawk>{SQUAWK})` +
			`.*?EQUIP\s+(?P<aircraft>{AIRCRAFT})/[A-Z]`,
		Fields: []string{"flight", "origin", "squawk", "aircraft"},
	},

	// Format 3: American Airlines
	// FLIGHT 1083/29 AUS - PHX
	// PDC
	// AAL1083 XPNDR 2555
	// A319/L P1839 350
	// KAUS ILEXY4 ... KPHL
	// CLEARED ILEXY4 DEPARTURE ...
	{
		Name: "american",
		Pattern: `(?s)FLIGHT\s+\d+/\d+\s+(?P<origin_iata>{IATA})\s*-\s*(?P<dest_iata>{IATA}).*?PDC\s+` +
			`(?P<flight>{FLIGHT})\s+XPNDR\s+(?P<squawk>{SQUAWK})\s+` +
			`(?P<aircraft>{AIRCRAFT})/[A-Z]\s+P\d{4}\s+(?P<altitude>\d{2,3})`,
		Fields: []string{"origin_iata", "dest_iata", "flight", "squawk", "aircraft", "altitude"},
	},

	// Format 3b: London Heathrow/UK CLD format (slightly different header)
	// Uses .CLD header instead of .DC1/CLD
	// Example:
	// /LHRDCXA.CLD 1620 260118 EGLL PDC 691
	// BAW99 CLRD TO CYYZ OFF 09R VIA ULTIB1J
	// SQUAWK 5166 ADT 1705 ATIS B
	{
		Name: "uk_cld",
		Pattern: `(?s)/[A-Z]+\.[A-Z]+\s+\d+\s+\d+\s+(?P<origin>{ICAO})\s+PDC\s+\d+\s+` +
			`(?P<flight>{FLIGHT})\s+CLRD\s+TO\s+(?P<destination>{ICAO})\s+` +
			`OFF\s*(?P<runway>{RUNWAY})\s+VIA\s+(?P<sid>{SID})`,
		Fields: []string{"origin", "flight", "destination", "runway", "sid"},
	},

	// Format 4: DC1 Clearance (global airport ground system format)
	// Used worldwide by airport clearance delivery systems.
	// Header format: /[IATA][SYSTEM].DC1/CLD [TIME] [DATE] [ICAO] PDC [NUM]
	// Examples:
	// /FRADFYA.DC1/CLD 0739 230201 EDDF PDC 847
	// /HKGCPYA.DC1/CLD 0809 230201 VHHH PDC 354
	// /SINCXYA.DC1/CLD 0909 230201 WSSS PDC 001
	// /RIOCGYA.DC1/CLD 0820 230201 SBGR PDC 551
	{
		Name: "dc1_clearance",
		Pattern: `(?s)/[A-Z]+\.[A-Z0-9]+/[A-Z]+\s+\d+\s+\d+\s+(?P<origin>{ICAO})\s*` +
			`PDC\s*{PDCNUM}?\s*` +
			`(?P<flight>{FLIGHT})\s+CLRD\s+TO\s+(?P<destination>{ICAO})\s+` +
			`OFF\s*(?P<runway>{RUNWAY})\s+` +
			`(?:(?P<altitude>{ALTITUDE})\s+FT\s+)?` +
			`VIA\s+(?P<sid>{SID})`,
		Fields: []string{"origin", "flight", "destination", "runway", "altitude", "sid"},
	},

	// Format 4a2: DC1 Clearance for private/corporate flights (registration as callsign)
	// Same as dc1_clearance but uses aircraft registration instead of airline callsign.
	// Example:
	// /GVADCYA.DC1/CLD 1423 260118 LSZH PDC
	// 174
	// 9HBCT CLRD TO LFPB OFF 28 VIA VEBIT4W ALT 5000
	{
		Name: "dc1_private",
		Pattern: `(?s)/[A-Z]+\.[A-Z0-9]+/[A-Z]+\s+\d+\s+\d+\s+(?P<origin>{ICAO})\s*` +
			`PDC\s*{PDCNUM}?\s*` +
			`(?P<flight>{REGISTRATION})\s+CLRD\s+TO\s+(?P<destination>{ICAO})\s+` +
			`OFF\s*(?P<runway>{RUNWAY})\s+` +
			`VIA\s+(?P<sid>[A-Z0-9]+)`,
		Fields: []string{"origin", "flight", "destination", "runway", "sid"},
	},

	// Format 4b: DC1 Clearance with waypoint-based departure (no numbered SID)
	// Similar to dc1_clearance but VIA is followed by a waypoint name, not a SID.
	// Example:
	// /GYDCEYA.DC1/CLD 1819 251231 UBBB PDC 093
	// CSN6024 CLRD TO ZWWW OFF 17 VIA NAMAS 1C
	{
		Name: "dc1_waypoint",
		Pattern: `(?s)/[A-Z]+\.[A-Z0-9]+/[A-Z]+\s+\d+\s+\d+\s+(?P<origin>{ICAO})\s*` +
			`PDC\s*{PDCNUM}?\s*` +
			`(?P<flight>{FLIGHT})\s+CLRD\s+TO\s+(?P<destination>{ICAO})\s+` +
			`OFF\s*(?P<runway>{RUNWAY})\s+` +
			`VIA\s+(?P<waypoint>[A-Z]{3,5})`,
		Fields: []string{"origin", "flight", "destination", "runway", "waypoint"},
	},

	// Format 4c: DC1 Clearance with HDG/VECTORS (Nordic/Finnish format)
	// Similar to dc1_clearance but uses HDG + VECTORS instead of VIA SID.
	// Handles both /CLD and /CDA headers.
	// Example:
	// /HELCLXA.DC1/CLD 1905 251231 EFHK PDC
	// 106
	// FIN4EL CLRD TO EETN OFF
	// 15 HDG 140 CLIMB TO 3000
	// FT VECTORS RENKU
	{
		Name: "dc1_nordic",
		Pattern: `(?s)/[A-Z]+\.[A-Z0-9]+/[A-Z]+\s+\d+\s+\d+\s+(?P<origin>{ICAO})\s*` +
			`PDC\s*{PDCNUM}?\s*` +
			`(?P<flight>{FLIGHT})\s+CLRD\s+TO\s+(?P<destination>{ICAO})\s+` +
			`OFF\s*(?P<runway>{RUNWAY})\s+` +
			`(?:HDG\s+\d+\s+)?` +
			`CLIMB\s+TO\s+(?P<altitude>{ALTITUDE})`,
		Fields: []string{"origin", "flight", "destination", "runway", "altitude"},
	},

	// Format 4b: ARINC MSG generic format
	// Standard format used by many airlines via ARINC/NAV Canada.
	// ARINC MSG
	// PDC <flight>
	// <origin>-<destination>
	// -ATC CLEARANCE-
	// CLR AS FILED
	// -FILED FLIGHT PLAN-
	// <route>
	// -REMARKS-
	// <SID, runway, altitude, squawk, etc.>
	{
		Name: "arinc_msg",
		Pattern: `(?s)ARINC\s+MSG\s+` +
			`PDC\s+(?P<flight>{FLIGHT}|{REGISTRATION})\s+` +
			`(?P<origin>{ICAO})-(?P<destination>{ICAO})\s+` +
			`-ATC\s+CLEARANCE-\s+` +
			`.*?-FILED\s+FLIGHT\s+PLAN-\s+` +
			`(?P<route>[A-Z0-9\s]+?)\s+` +
			`-REMARKS-` +
			`(?:.*?(?:USE\s+)?SID\s+(?P<sid>[A-Z0-9]+))?` +
			`(?:.*?(?:DEPARTURE\s+)?RUNWAY\s+(?P<runway>{RUNWAY}))?` +
			`(?:.*?(?:MAINTAIN|CLIMB\s+TO)\s+(?P<altitude>\d+)(?:FT)?)?` +
			`(?:.*?SQUAWK\s+(?P<squawk>{SQUAWK}))?` +
			`(?:.*?DPFRQ\s+(?P<frequency>{FREQ}))?`,
		Fields: []string{"flight", "origin", "destination", "route", "sid", "runway", "altitude", "squawk", "frequency"},
	},

	// Format 5: Canadian/WestJet style
	// PRE-DEPARTURE ATC CLEARANCE
	// WJA311  DEPART YEG AT 1716Z FL 400
	// M/B737/W TRANSPONDER 1731
	// ROUTE:
	// ANDIE Q860 MERYT BOOTH CANUC6
	// REMARKS:
	// USE SID JEVON1
	// DEPARTURE RUNWAY 12
	// DESTINATION CYVR
	{
		Name: "canadian_westjet",
		Pattern: `(?s)PRE-?DEPARTURE\s+(?:ATC\s+)?CLEARANCE\s+` +
			`(?P<flight>{FLIGHT})\s+DEPART\s+(?P<origin_iata>{IATA})\s+AT\s+\d{4}Z` +
			`.*?TRANSPONDER\s+(?P<squawk>{SQUAWK})` +
			`.*?ROUTE[:\s]+(?P<route>[A-Z0-9\s]+?)` +
			`(?:\s*REMARKS|\s*USE\s+SID)` +
			`.*?(?:USE\s+)?SID\s+(?P<sid>[A-Z0-9]+)` +
			`.*?(?:DEPARTURE\s+)?RUNWAY\s+(?P<runway>{RUNWAY})` +
			`.*?DESTINATION\s+(?P<destination>{ICAO})`,
		Fields: []string{"flight", "origin_iata", "squawk", "route", "sid", "runway", "destination"},
	},

	// Format 6: Canadian NAV Canada / Air Canada style
	// Uses -// ATC PA01 header with aircraft registration.
	// Example:
	// -// ATC PA01 YYZOWAC 03JAN/0637          C-FSIL/508/AC0348
	// TIMESTAMP 03JAN26 06:25
	// *PRE-DEPARTURE CLEARANCE*
	// FLT ACA348    CYVR
	// M/B38M/W FILED FL350
	// XPRD 0032
	//
	// USE SID FSR8
	// DEPARTURE RUNWAY 08R
	// DESTINATION CYOW
	{
		Name: "canadian_nav",
		Pattern: `(?s)-//\s+ATC\s+PA01\s+\w+\s+\d{2}[A-Z]{3}/\d{4}\s+` +
			`[A-Z]-[A-Z]{4}/\d+/[A-Z]{2}\d+\s+` +
			`TIMESTAMP\s+.+?\s+` +
			`\*PRE-DEPARTURE\s+CLEARANCE\*\s+` +
			`FLT\s+(?P<flight>{FLIGHT})\s+(?P<origin>{ICAO})\s+` +
			`(?P<aircraft>[A-Z]/[A-Z0-9]+/[A-Z])\s+FILED\s+FL(?P<flight_level>\d+)` +
			`.*?USE\s+SID\s+(?P<sid>[A-Z0-9]+)\s+` +
			`DEPARTURE\s+RUNWAY\s+(?P<runway>{RUNWAY})\s+` +
			`DESTINATION\s+(?P<destination>{ICAO})`,
		Fields: []string{"flight", "origin", "aircraft", "flight_level", "sid", "runway", "destination"},
	},

	// Format 6a2: Qantas/China Airlines Pacific format (tab-separated)
	// Uses tabs between fields, common for Pacific routes via Vancouver.
	// Example:
	// PDC
	// 197
	// QFA76	7006	CYVR
	// H/B789/W	P0420
	// 000	340
	// ...
	// USE SID GRG7
	// DEPARTURE RUNWAY 26L
	// DESTINATION YSSY
	{
		Name: "qantas_pacific",
		Pattern: `(?s)PDC\s+` +
			`\d+\s+` +
			`(?P<flight>[A-Z]{3}\d{1,4})[\t\s]+(?P<squawk>{SQUAWK})[\t\s]+(?P<origin>{ICAO})\s+` +
			`[A-Z]/(?P<aircraft>{AIRCRAFT})/[A-Z][\t\s]+P\d{4}\s+` +
			`\d+[\t\s]+(?P<altitude>\d{2,3})\s+` +
			`.*?USE\s+SID\s+(?P<sid>[A-Z0-9]+)\s+` +
			`DEPARTURE\s+RUNWAY\s+(?P<runway>{RUNWAY})\s+` +
			`DESTINATION\s+(?P<destination>{ICAO})`,
		Fields: []string{"flight", "squawk", "origin", "aircraft", "altitude", "sid", "runway", "destination"},
	},

	// Format 6b: Canadian Jazz/Cargojet format
	// Used by Jazz Aviation (JZA), Cargojet (CJT), and other Canadian carriers.
	// Example:
	// FLIGHT JZA810/17 CYVR
	// KSEA
	// PDC
	// JZA810 0031 CYVR
	// M/DH8D/R P0505
	// 150
	// YVR MARNR MARNR8
	// EDCT
	// USE SID GRG7
	// DEPARTURE RUNWAY 26L
	// DESTINATION KSEA
	{
		Name: "canadian_jazz",
		Pattern: `(?s)FLIGHT\s+(?P<flight>{FLIGHT})/\d+\s+(?P<origin>{ICAO})\s+` +
			`(?P<destination>{ICAO})\s+` +
			`PDC\s+` +
			`{FLIGHT}\s+(?P<squawk>{SQUAWK})\s+{ICAO}\s+` +
			`[A-Z]/(?P<aircraft>{AIRCRAFT})/[A-Z]\s+P\d{4}\s+` +
			`(?P<altitude>\d{2,3})` +
			`.*?USE\s+SID\s+(?P<sid>[A-Z0-9]+)\s+` +
			`DEPARTURE\s+RUNWAY\s+(?P<runway>{RUNWAY})`,
		Fields: []string{"flight", "origin", "destination", "squawk", "aircraft", "altitude", "sid", "runway"},
	},

	// Format 6c: Southwest Airlines PDC
	// Uses GEN01 header and similar structure to Canadian Jazz but with route to destination.
	// Example:
	// GEN01,00PRE DEPT CLR ,PDC MSG RECEIVED        ,
	// FLIGHT SWA1343/07 KALB KMCO
	// PDC
	// SWA1343 1454 KALB
	// B38M/L P1803
	// 360
	// KALB PONCT Q437 CRPLR EARZZ ... KMCO
	// EDCT
	// CLEARED ALB7 DEPARTURE
	{
		Name: "southwest",
		Pattern: `(?s)FLIGHT\s+(?P<flight>{FLIGHT})/\d+\s+(?P<origin>{ICAO})\s+(?P<destination>{ICAO})\s+` +
			`PDC\s+` +
			`{FLIGHT}\s+(?P<squawk>{SQUAWK})\s+{ICAO}\s+` +
			`(?P<aircraft>{AIRCRAFT})/[A-Z]\s+P\d{4}\s+` +
			`(?P<altitude>\d{2,3})`,
		Fields: []string{"flight", "origin", "destination", "squawk", "aircraft", "altitude"},
	},

	// Format 7: US Regional (Piedmont/PSA/etc) with destination in route line
	// PDC
	// 001
	// PDT5898 1772 KPHL
	// E145/L P1834
	// 145 310
	// -DITCH T416 JIMEE-
	// KPHL DITCH LUIGI HNNAH CYUL
	// The route line starts with origin ICAO and ends with destination ICAO.
	{
		Name: "us_regional_route",
		Pattern: `(?s)^PDC\s+` +
			`\d{3}\s+` +
			`(?P<flight>[A-Z]{3}\d{3,4})\s+(?P<squawk>{SQUAWK})\s+(?P<origin>{ICAO})\s+` +
			`(?P<aircraft>{AIRCRAFT})/[A-Z]\s+P\d{4}\s+` +
			`\d+\s+(?P<altitude>\d{2,3})\s+` +
			`-[A-Z0-9\s]+-\s+` +
			`{ICAO}\s+[A-Z0-9\s]+\s+(?P<destination>{ICAO})$`,
		Fields: []string{"flight", "squawk", "origin", "aircraft", "altitude", "destination"},
	},

	// Format 7b: US Regional (Piedmont/PSA/etc) without destination
	// PDC
	// 001
	// PDT5898 1772 KPHL
	// E145/L P1834
	// 145 310
	// -DITCH T416 JIMEE-
	// KPHL DITCH V312 JIMEE WAVEY
	{
		Name: "us_regional",
		Pattern: `(?s)^PDC\s+` +
			`\d{3}\s+` +
			`(?P<flight>[A-Z]{3}\d{3,4})\s+(?P<squawk>{SQUAWK})\s+(?P<origin>{ICAO})\s+` +
			`(?P<aircraft>{AIRCRAFT})/[A-Z]\s+P\d{4}\s+` +
			`\d+\s+(?P<altitude>\d{2,3})`,
		Fields: []string{"flight", "squawk", "origin", "aircraft", "altitude"},
	},

	// Format 8: Alaska/Hawaiian Airlines style
	// PDC MSG
	// RECEIVED ON 31 AT 1757UTC
	// PRE-DEPARTURE ATC CLEARANCE
	// ASA1033  DEPART HNL AT 1841Z FL 140
	// B712/L TRANSPONDER 3603
	// ROUTE:
	// PHNL KEOLA3 LIH PHLI
	{
		Name: "alaska_hawaiian",
		Pattern: `(?s)PRE-DEPARTURE\s+ATC\s+CLEARANCE\s+` +
			`(?P<flight>{FLIGHT})\s+DEPART\s+(?P<origin_iata>{IATA})\s+AT\s+(?P<dep_time>\d{4})Z\s+FL\s+(?P<flight_level>\d+)\s+` +
			`(?P<aircraft>{AIRCRAFT})/[A-Z]\s+TRANSPONDER\s+(?P<squawk>{SQUAWK})`,
		Fields: []string{"flight", "origin_iata", "dep_time", "flight_level", "aircraft", "squawk"},
	},

	// Format 9: Horizon/QXE format
	// PDC
	// **DEPARTURE CLEARANCE**
	// FLT 2022-31 KPDX-KBZN
	// QXE2022 KPDX
	// E75L/L P1830
	// REQUESTED FL 350
	// XPDR 1605
	{
		Name: "horizon_qxe",
		Pattern: `(?s)PDC\s+\*+DEPARTURE\s+CLEARANCE\*+\s+` +
			`FLT\s+\d+-\d+\s+(?P<origin>{ICAO})-(?P<destination>{ICAO})\s+` +
			`(?P<flight>[A-Z]{3}\d{3,4})\s+{ICAO}\s+` +
			`(?P<aircraft>{AIRCRAFT})/[A-Z]\s+P\d{4}\s+` +
			`REQUESTED\s+FL\s+(?P<flight_level>\d+)\s+` +
			`XPDR\s+(?P<squawk>{SQUAWK})`,
		Fields: []string{"origin", "destination", "flight", "aircraft", "flight_level", "squawk"},
	},

	// Format 10: SkyWest/regional pre-departure (full format with DESTINATION)
	// PRE-DEPARTURE CLEARANCE
	// SKW3630 DEPART YVR 1712Z
	// FL 130 E75L/W XPNDR 7014
	// USE SID GRG7 DEPARTURE
	// RUNWAY 26L DESTINATION
	// KSEA
	{
		Name: "skywest_full",
		Pattern: `(?s)PRE-DEPARTURE\s+CLEARANCE\s+` +

			`(?P<flight>{FLIGHT})\s+DEPART\s+(?P<origin_iata>{IATA})\s+(?P<dep_time>\d{4})Z\s+` +
			`FL\s+(?P<flight_level>\d+)\s+(?P<aircraft>{AIRCRAFT})/[A-Z]\s+XPNDR\s+(?P<squawk>{SQUAWK})` +
			`.*?USE\s+SID\s+(?P<sid>[A-Z0-9]+)\s+DEPARTURE\s+` +
			`RUNWAY\s+(?P<runway>{RUNWAY})\s+DESTINATION\s+` +
			`(?P<destination>{ICAO})`,
		Fields: []string{"flight", "origin_iata", "dep_time", "flight_level", "aircraft", "squawk", "sid", "runway", "destination"},
	},

	// Format 11: SkyWest/regional pre-departure (simple format)
	// PRE-DEPARTURE CLEARANCE
	// SKW5510 DEPART ORD 1845Z
	// FL 320 CRJ2/L XPNDR 6520
	// ROUTE KORD OLINN OREOS ...
	{
		Name: "skywest_simple",
		Pattern: `(?s)PRE-DEPARTURE\s+CLEARANCE\s+` +
			`(?P<flight>{FLIGHT})\s+DEPART\s+(?P<origin_iata>{IATA})\s+(?P<dep_time>\d{4})Z\s+` +
			`FL\s+(?P<flight_level>\d+)\s+(?P<aircraft>{AIRCRAFT})/[A-Z]\s+XPNDR\s+(?P<squawk>{SQUAWK})`,
		Fields: []string{"flight", "origin_iata", "dep_time", "flight_level", "aircraft", "squawk"},
	},

	// Format 12: Republic Airways PDC
	// Example:
	// QUHDQDDRP~1PDC SEQ 001
	// RPA4783
	// DEP/KDCA
	// SKD/1229Z
	// FL360
	// -REBLL5 OTTTO Q80 DEWAK GROAT PASLY5 KBNA-
	// KDCA REBLL5 OTTTO Q80./.KBNA
	// CLEARED REBLL5 DEPARTURE OTTTO TRSN
	// CLIMB VIA SID
	// SQUAWK/7050
	{
		Name: "republic_airways",
		Pattern: `(?s)PDC\s+SEQ\s+\d+\s+` +
			`(?P<flight>{FLIGHT})\s+` +
			`DEP/(?P<origin>{ICAO})\s+` +
			`SKD/(?P<dep_time>\d{4})Z\s+` +
			`FL(?P<flight_level>\d+)\s+` +
			`.*?CLEARED\s+(?P<sid>[A-Z0-9]+)\s+DEPARTURE` +
			`.*?SQUAWK/(?P<squawk>{SQUAWK})`,
		Fields: []string{"flight", "origin", "dep_time", "flight_level", "sid", "squawk"},
	},

	// Format 13: Private/Business jet PDC (Teterboro, etc)
	// Example:
	// KTEB PDC
	// PDC LXJ559 CL35/L (or PDC PSBCO GLEX/L for registrations)
	// ETD KTEB 1233UTC
	// FL20
	// CLEARED AS FILED
	// CLEARED TEB4 DEPARTURE
	// MAINTAIN 2000FT
	// EXP 20 10 MIN AFT DP,DPFRQ 119.2
	// SQUAWK 1234
	{
		Name: "private_jet",
		Pattern: `(?s){ICAO}\s+PDC\s+` +
			`PDC\s+(?P<flight>{FLIGHT}|{REGISTRATION})\s+(?P<aircraft>[A-Z0-9]+)/[A-Z]\s+` +
			`ETD\s+(?P<origin>{ICAO})\s+(?P<dep_time>\d{4})UTC\s+` +
			`FL(?P<flight_level>\d+)\s+` +
			`.*?CLEARED\s+(?P<sid>[A-Z0-9]+)\s+DEPARTURE` +
			`.*?MAINTAIN\s+(?P<init_alt>\d+)FT` +
			`.*?SQUAWK\s+(?P<squawk>{SQUAWK})`,
		Fields: []string{"flight", "aircraft", "origin", "dep_time", "flight_level", "sid", "init_alt", "squawk"},
	},

	// Format 14: UPS/Cargo PDC
	// Example:
	// PDC UP0081/03 ANC-SDF
	// ----ATC CLEARANCE----
	// CLEARED AS FILED
	// CLEARED ANC1 DEPARTURE
	// SQWK: 7271
	// MAINTAIN 4000FT EXP 350 10 MIN AFT DP,DPFRQ 118.6
	{
		Name: "ups_cargo",
		Pattern: `(?s)PDC\s+(?P<flight>[A-Z]{2,3}\d{1,4})/\d+\s+(?P<origin>{ICAO})-(?P<destination>{ICAO})\s+` +
			`.*?CLEARED\s+(?P<sid>[A-Z0-9]+)\s+DEPARTURE\s+` +
			`SQWK:\s*(?P<squawk>{SQUAWK})\s+` +
			`MAINTAIN\s+(?P<init_alt>\d+)`,
		Fields: []string{"flight", "origin", "destination", "sid", "squawk", "init_alt"},
	},

	// Format 15: Frontier extended AGM format
	// Example:
	// .ANPOCF9 051209
	// AGM
	// AN N625FR
	// -  CLD 1209 260105 PDC 001
	// FFT22 KPHL-MDPC
	// SQUAWK: 3354
	// ...
	// CLEARED PHL4 DEPARTURE
	// MAINTAIN 5000FT
	{
		Name: "frontier_extended",
		Pattern: `(?s)AGM\s+AN\s+[A-Z0-9-]+\s+-\s+CLD\s+\d+\s+\d+\s+PDC\s+\d+\s+` +
			`(?P<flight>[A-Z]{3}\d{1,4})\s+(?P<origin>{ICAO})-(?P<destination>{ICAO})\s+` +
			`SQUAWK:\s*(?P<squawk>{SQUAWK})` +
			`.*?CLEARED\s+(?P<sid>[A-Z0-9]+)\s+DEPARTURE\s+` +
			`MAINTAIN\s+(?P<init_alt>\d+)`,
		Fields: []string{"flight", "origin", "destination", "squawk", "sid", "init_alt"},
	},

	// Format 16: Frontier/Spirit AGM format
	// .ANPOCF9 311828
	// AGM
	// AN N708FR
	// -  CLD 1828 251231 PDC 002
	// FFT1577 KPHL-KRSW
	// SQUAWK: 7134
	{
		Name: "frontier_agm",
		Pattern: `(?s)AGM\s+AN\s+[A-Z0-9-]+\s+-\s+CLD\s+\d+\s+\d+\s+PDC\s+\d+\s+` +
			`(?P<flight>[A-Z]{3}\d{3,4})\s+(?P<origin>{ICAO})-(?P<destination>{ICAO})\s+` +
			`SQUAWK:\s*(?P<squawk>{SQUAWK})`,
		Fields: []string{"flight", "origin", "destination", "squawk"},
	},
}

// Compiler manages pattern compilation and caching.
type Compiler struct {
	basePatterns map[string]string
	formats      []PDCFormat
}

// NewCompiler creates a new pattern compiler with the default patterns.
func NewCompiler() *Compiler {
	c := &Compiler{
		basePatterns: make(map[string]string),
		formats:      make([]PDCFormat, len(PDCFormats)),
	}

	// Copy base patterns.
	for k, v := range BasePatterns {
		c.basePatterns[k] = v
	}

	// Copy formats.
	copy(c.formats, PDCFormats)

	return c
}

// Compile expands all {PLACEHOLDER} references and compiles regexes.
func (c *Compiler) Compile() error {
	for i := range c.formats {
		expanded := c.expand(c.formats[i].Pattern)
		re, err := regexp.Compile(expanded)
		if err != nil {
			return err
		}
		c.formats[i].Compiled = re
	}
	return nil
}

// expand replaces {PLACEHOLDER} with actual regex patterns.
func (c *Compiler) expand(pattern string) string {
	result := pattern
	for name, regex := range c.basePatterns {
		placeholder := "{" + name + "}"
		result = strings.ReplaceAll(result, placeholder, regex)
	}
	return result
}

// PDCResult contains the extracted fields from a PDC message.
type PDCResult struct {
	FormatName      string
	FlightNumber    string
	Origin          string // ICAO origin (4-letter code)
	OriginIATA      string // IATA origin (3-letter code) - not used for enrichment
	Destination     string // ICAO destination (4-letter code)
	DestIATA        string // IATA destination (3-letter code) - not used for enrichment
	Aircraft        string
	Runway          string
	SID             string
	Route           string
	Squawk          string
	Altitude        string // Initial climb altitude (e.g., MAINTAIN 5000FT)
	FlightLevel     string // Cruise flight level (e.g., FL410)
	Frequency       string
	ATIS            string
	DepartureTime   string
}

// Parse attempts to parse a PDC message using all known formats.
// Returns the first successful match.
func (c *Compiler) Parse(text string) *PDCResult {
	upperText := strings.ToUpper(text)

	for _, format := range c.formats {
		if format.Compiled == nil {
			continue
		}

		match := format.Compiled.FindStringSubmatch(upperText)
		if match == nil {
			continue
		}

		result := &PDCResult{
			FormatName: format.Name,
		}

		// Extract named groups.
		for i, name := range format.Compiled.SubexpNames() {
			if i == 0 || name == "" {
				continue
			}
			value := match[i]
			switch name {
			case "flight":
				result.FlightNumber = value
			case "origin":
				result.Origin = value
			case "origin_iata":
				// IATA origin code - stored separately, not used for enrichment.
				result.OriginIATA = value
			case "destination":
				result.Destination = value
			case "dest_iata":
				// IATA destination code - stored separately, not used for enrichment.
				result.DestIATA = value
			case "waypoint":
				// Initial waypoint (for waypoint-based departures without named SID).
				if result.SID == "" {
					result.SID = value
				}
			case "aircraft":
				result.Aircraft = value
			case "runway":
				result.Runway = value
			case "runway2":
				// Alternate runway position (e.g., "EXPECT RUNWAY" at end of Australian format).
				if result.Runway == "" {
					result.Runway = value
				}
			case "sid":
				result.SID = normaliseSID(value)
			case "route":
				result.Route = cleanRoute(value)
			case "squawk":
				result.Squawk = value
			case "altitude", "init_alt":
				result.Altitude = value
			case "flight_level":
				result.FlightLevel = value
			case "freq":
				result.Frequency = value
			case "atis":
				result.ATIS = value
			case "dep_time":
				result.DepartureTime = value
			}
		}

		// Post-process: extract squawk if not in pattern.
		if result.Squawk == "" {
			result.Squawk = extractSquawk(upperText)
		}

		// Post-process: extract frequency if not in pattern.
		if result.Frequency == "" {
			result.Frequency = extractFrequency(upperText)
		}

		// Post-process: extract ATIS if not in pattern.
		if result.ATIS == "" {
			result.ATIS = extractATIS(upperText)
		}

		// Post-process: extract altitude if not in pattern.
		if result.Altitude == "" {
			result.Altitude = extractAltitude(upperText)
		}

		// Post-process: extract flight level if not in pattern.
		if result.FlightLevel == "" {
			result.FlightLevel = extractFlightLevel(upperText)
		}

		// Post-process: extract route if present.
		if result.Route == "" {
			result.Route = extractRoute(upperText)
		}

		// Post-process: remove origin from start and destination from end of route if duplicated.
		if result.Route != "" && result.Origin != "" {
			result.Route = strings.TrimPrefix(result.Route, result.Origin+" ")
		}
		if result.Route != "" && result.Destination != "" {
			result.Route = strings.TrimSuffix(result.Route, " "+result.Destination)
		}
		if result.Route != "" {
			result.Route = strings.TrimSpace(result.Route)
		}

		return result
	}

	return nil
}

// PDCFormatTrace contains debug information about a PDC format match attempt.
type PDCFormatTrace struct {
	Name     string            // Format name
	Matched  bool              // Whether the pattern matched
	Pattern  string            // The expanded regex pattern
	Captures map[string]string // Captured groups (if matched)
}

// PDCParseTrace contains complete trace information for a PDC parse attempt.
type PDCParseTrace struct {
	Formats    []PDCFormatTrace    // All format match attempts
	Result     *PDCResult          // The parse result (if any match succeeded)
	Extractors []PDCExtractorTrace // Post-processing extractor results
}

// PDCExtractorTrace contains debug info about a field extractor.
type PDCExtractorTrace struct {
	Name    string // Extractor name (e.g., "ExtractSquawk")
	Pattern string // The regex pattern used
	Matched bool   // Whether it matched
	Value   string // Extracted value (if matched)
}

// ParseWithTrace attempts to parse a PDC message and returns detailed trace information.
func (c *Compiler) ParseWithTrace(text string) *PDCParseTrace {
	upperText := strings.ToUpper(text)
	trace := &PDCParseTrace{
		Formats: make([]PDCFormatTrace, 0, len(c.formats)),
	}

	for _, format := range c.formats {
		ft := PDCFormatTrace{
			Name:    format.Name,
			Pattern: c.expand(format.Pattern),
		}

		if format.Compiled == nil {
			trace.Formats = append(trace.Formats, ft)
			continue
		}

		match := format.Compiled.FindStringSubmatch(upperText)
		if match == nil {
			ft.Matched = false
			trace.Formats = append(trace.Formats, ft)
			continue
		}

		// Pattern matched.
		ft.Matched = true
		ft.Captures = make(map[string]string)

		for i, name := range format.Compiled.SubexpNames() {
			if i == 0 || name == "" {
				continue
			}
			ft.Captures[name] = match[i]
		}

		trace.Formats = append(trace.Formats, ft)

		// Build result from first match.
		if trace.Result == nil {
			trace.Result = &PDCResult{FormatName: format.Name}
			for name, value := range ft.Captures {
				switch name {
				case "flight":
					trace.Result.FlightNumber = value
				case "origin":
					trace.Result.Origin = value
				case "origin_iata":
					trace.Result.OriginIATA = value
				case "destination":
					trace.Result.Destination = value
				case "dest_iata":
					trace.Result.DestIATA = value
				case "waypoint":
					if trace.Result.SID == "" {
						trace.Result.SID = value
					}
				case "aircraft":
					trace.Result.Aircraft = value
				case "runway":
					trace.Result.Runway = value
				case "runway2":
					if trace.Result.Runway == "" {
						trace.Result.Runway = value
					}
				case "sid":
					trace.Result.SID = normaliseSID(value)
				case "route":
					trace.Result.Route = cleanRoute(value)
				case "squawk":
					trace.Result.Squawk = value
				case "altitude":
					trace.Result.Altitude = value
				case "freq":
					trace.Result.Frequency = value
				case "atis":
					trace.Result.ATIS = value
				}
			}
		}
	}

	// Trace post-processing extractors.
	trace.Extractors = []PDCExtractorTrace{
		traceExtractor("ExtractSquawk", squawkRe.String(), squawkRe.FindStringSubmatch(upperText)),
		traceExtractor("ExtractFrequency", freqRe.String(), freqRe.FindStringSubmatch(upperText)),
		traceExtractor("ExtractATIS", atisRe.String(), atisRe.FindStringSubmatch(upperText)),
		traceExtractor("ExtractAltitude", altitudeRe.String(), altitudeRe.FindStringSubmatch(upperText)),
		traceExtractor("ExtractFlightLevel", flightLevelRe.String(), flightLevelRe.FindStringSubmatch(upperText)),
	}

	return trace
}

func traceExtractor(name, pattern string, match []string) PDCExtractorTrace {
	t := PDCExtractorTrace{
		Name:    name,
		Pattern: pattern,
		Matched: len(match) > 1,
	}
	if t.Matched {
		t.Value = match[1]
	}
	return t
}

// Helper extractors for fields not captured by format patterns.
var (
	squawkRe   = regexp.MustCompile(`(?:SQUAWK|XPNDR|XPDR|TRANSPONDER)[/:\s]+([0-7]{4})`)
	freqRe     = regexp.MustCompile(`(?:DEP\s*FREQ|DPFRQ|NEXT\s*FREQ|AIRBORNE\s*FREQ)[:\s]+(\d{3}\.\d{1,3})`)
	atisRe     = regexp.MustCompile(`ATIS\s+([A-Z])\b`)
	altitudeRe     = regexp.MustCompile(`(?:CLIMB\s+(?:VIA\s+SID\s+)?TO[:\s]+|ALT\s*)(\d{3,5})`)
	flightLevelRe  = regexp.MustCompile(`(?:CRUISE\s+(?:FLT\s+)?LEVEL\s+|FL)(\d{2,3})\b`)
	// Runway patterns - various PDC formats use different keywords.
	runwayRe = regexp.MustCompile(`(?:EXPECT\s+RUNWAY|DEPARTURE\s+RUNWAY|DEP(?:ARTURE)?\s+RWY|RWY)\s+(\d{1,2}[LRC]?)`)
	// Departure time patterns:
	// - "SKED DEP TIME 1857" (Delta format)
	// - "AT 1716Z" (Canadian/WestJet format)
	// - "DEPART ... AT 1716Z" (alternative)
	// - "P1844" after aircraft type (American format - P prefix means planned/proposed time)
	depTimeRe = regexp.MustCompile(`(?:SKED\s*DEP\s*TIME|DEPART\s+\S+\s+AT|AT)\s+(\d{4})Z?`)
	// Route patterns - multiple formats exist:
	// 1. Australian: "ROUTE:" prefix
	// 2. US Delta: "ROUTING" section between asterisk lines
	// 3. DC1: Often inline or multi-line after "VIA [SID]"
	routeRe        = regexp.MustCompile(`(?s)ROUTE[:\s]+(.+?)(?:\n\s*(?:CLIMB|DEP|SQUAWK)|$)`)
	routeUSRe      = regexp.MustCompile(`(?s)ROUTING\s*\n\*+\s*\n(?:-[^\n]*\n)?(.+?)\n\*+`)
	routeDC1InlRe  = regexp.MustCompile(`VIA\s+[A-Z0-9]+\s+([A-Z]\d{1,4}[A-Z]?\s.+?)(?:\s+(?:ALT|FL)\d|$)`)
	routeDC1MultiRe = regexp.MustCompile(`(?s)VIA\s*\n\s*([A-Z0-9/]+.+?)(?:\n\s*SQUAWK|\n\s*$)`)
)

func extractSquawk(text string) string {
	if m := squawkRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractFrequency(text string) string {
	if m := freqRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractATIS(text string) string {
	if m := atisRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractAltitude(text string) string {
	if m := altitudeRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractFlightLevel(text string) string {
	if m := flightLevelRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

// ExtractDepartureTime extracts scheduled departure time from PDC text.
func ExtractDepartureTime(text string) string {
	upperText := strings.ToUpper(text)
	if m := depTimeRe.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractRoute(text string) string {
	// Try each route pattern in order of specificity.
	patterns := []*regexp.Regexp{routeRe, routeUSRe, routeDC1MultiRe, routeDC1InlRe}

	for _, re := range patterns {
		if m := re.FindStringSubmatch(text); len(m) > 1 {
			return cleanRoute(m[1])
		}
	}
	return ""
}

// cleanRoute normalises route strings by collapsing whitespace.
func cleanRoute(route string) string {
	route = strings.TrimSpace(route)
	route = strings.ReplaceAll(route, "\r\n", " ")
	route = strings.ReplaceAll(route, "\r", " ")
	route = strings.ReplaceAll(route, "\n", " ")
	route = strings.ReplaceAll(route, "\t", " ")
	// Collapse multiple spaces.
	for strings.Contains(route, "  ") {
		route = strings.ReplaceAll(route, "  ", " ")
	}
	return route
}

// wordToDigit maps number words to digits for SID normalisation.
var wordToDigit = map[string]string{
	"ONE":   "1",
	"TWO":   "2",
	"THREE": "3",
	"FOUR":  "4",
	"FIVE":  "5",
	"SIX":   "6",
	"SEVEN": "7",
	"EIGHT": "8",
	"NINE":  "9",
}

// normaliseSID converts word-form SIDs to digit form (e.g., "SANEG TWO" -> "SANEG2").
func normaliseSID(sid string) string {
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return ""
	}

	// Check if SID contains a word number.
	for word, digit := range wordToDigit {
		if strings.HasSuffix(sid, " "+word) {
			// Replace "NAME WORD" with "NAMEdigit".
			return strings.TrimSuffix(sid, " "+word) + digit
		}
	}

	return sid
}

// waypointRe matches valid waypoint/fix patterns:
// 2-5 uppercase letters, optionally followed by 1-2 digits (for STARs/SIDs).
var waypointRe = regexp.MustCompile(`^[A-Z]{2,5}[0-9]{0,2}$`)

// excludedWaypoints contains words that match waypoint pattern but are not waypoints.
// These are labels, headings, keywords, or common terms in PDC messages.
var excludedWaypoints = map[string]bool{
	// Section labels/headings.
	"ROUTE":   true,
	"CLIMB":   true,
	"SQUAWK":  true,
	"ATIS":    true,
	"QNH":     true,
	"TSAT":    true,
	"EDCT":    true, // Expect Departure Clearance Time
	"CTOT":    true, // Calculated Take-Off Time
	// Keywords.
	"USE":         true,
	"SID":         true,
	"VIA":         true,
	"OFF":         true,
	"HDG":         true,
	"END":         true,
	"WITH":        true,
	"NEXT":        true,
	"FREQ":        true,
	"DEP":         true,
	"ARR":         true,
	"ALT":         true,
	"CL":          true, // Clearance
	"RLS":         true, // Release
	"CTC":         true, // Contact
	"CD":          true, // Clearance Delivery
	"GND":         true, // Ground
	"TWR":         true, // Tower
	"APP":         true, // Approach
	"CTR":         true, // Center
	// Common instruction words.
	"DEPARTURE":   true,
	"DESTINATION": true,
	"CONTACT":     true,
	"DELIVERY":    true,
	"CLEARANCE":   true,
	"MESSAGE":     true,
	"IDENTIFIER":  true,
	"REMARKS":     true,
	"RUNWAY":      true,
	"CLEARED":     true,
	"MAINTAIN":    true,
	"EXPECT":      true,
	"VECTORS":     true,
	"DIRECT":      true,
	"DCT":         true, // Direct To (abbreviation)
	"THEN":        true,
	"AFTER":       true,
	"UNTIL":       true,
	"WHEN":        true,
	"FLIGHT":      true,
	"FULL":        true, // "FULL ROUTE" etc.
	"FILE":        true, // "ON FILE" etc.
}

// isValidWaypoint checks if a token is a valid waypoint (matches pattern and not excluded).
func isValidWaypoint(token string) bool {
	if !waypointRe.MatchString(token) {
		return false
	}
	return !excludedWaypoints[token]
}

// ExtractRouteWaypoints extracts waypoint identifiers from PDC text.
// Handles multiple PDC formats:
// 1. US format: waypoints between aircraft type line and CLEARED section
// 2. Australian format: waypoints in the ROUTE: line after CLEARED section
func ExtractRouteWaypoints(text string) []string {
	var waypoints []string
	seen := make(map[string]bool)

	upperText := strings.ToUpper(text)
	lines := strings.Split(upperText, "\n")

	// Track if we're in a route section.
	inPreClearedRoute := false
	inPostClearedRoute := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// US format: start capturing after aircraft type line (e.g., "B738/L P0302 360").
		if strings.Contains(line, "/L ") || strings.Contains(line, "/W ") {
			inPreClearedRoute = true
			continue
		}

		// Stop pre-CLEARED section at CLEARED or END.
		if strings.HasPrefix(line, "CLEARED") || line == "END" {
			inPreClearedRoute = false
			// Don't break - continue to look for ROUTE: section.
		}

		// Australian format: start capturing at ROUTE: line.
		if strings.HasPrefix(line, "ROUTE:") || strings.HasPrefix(line, "ROUTE ") {
			inPostClearedRoute = true
			// Extract waypoints from the same line after "ROUTE:".
			routePart := strings.TrimPrefix(line, "ROUTE:")
			routePart = strings.TrimPrefix(routePart, "ROUTE ")
			tokens := strings.Fields(routePart)
			for _, token := range tokens {
				if isValidWaypoint(token) && !seen[token] {
					waypoints = append(waypoints, token)
					seen[token] = true
				}
			}
			continue
		}

		// Stop post-CLEARED route section at various terminating keywords.
		if inPostClearedRoute {
			if strings.HasPrefix(line, "CLIMB") || strings.HasPrefix(line, "DEP FREQ") ||
				strings.HasPrefix(line, "SQUAWK") || strings.HasPrefix(line, "DPFRQ") ||
				strings.HasPrefix(line, "EDCT") || strings.HasPrefix(line, "USE SID") ||
				strings.HasPrefix(line, "DEPARTURE") || strings.HasPrefix(line, "DESTINATION") ||
				strings.HasPrefix(line, "CONTACT") || strings.HasPrefix(line, "REMARKS") ||
				strings.HasPrefix(line, "END OF") {
				inPostClearedRoute = false
				continue
			}
		}

		if !inPreClearedRoute && !inPostClearedRoute {
			continue
		}

		// Split line into tokens and check each for waypoint-like patterns.
		tokens := strings.Fields(line)
		for _, token := range tokens {
			// Check if it's a valid waypoint (matches pattern and not excluded).
			if isValidWaypoint(token) && !seen[token] {
				waypoints = append(waypoints, token)
				seen[token] = true
			}
		}
	}

	return waypoints
}