// Package patterns provides shared regex patterns and helper functions for ACARS parsing.
package patterns

import (
	"regexp"
	"strings"
)

// Core patterns used across multiple parsers.
var (
	// FlightNumPattern matches flight numbers: 2-3 letter ICAO code + 1-4 digits + optional letter.
	FlightNumPattern = regexp.MustCompile(`\b([A-Z]{2,3})(\d{1,4}[A-Z]?)\b`)

	// FlightNumLabeledPatterns for explicit flight number markers.
	FlightNumFltPattern = regexp.MustCompile(`(?:FLT|FLIGHT)\s+(\d+)(?:/\d+)?`)
	// FlightNumCtxPattern matches ICAO callsigns followed by aircraft type, clearance, or airport.
	// e.g., "ASA329 B738", "UAL123 CLRD", "ASA329 KORD"
	FlightNumCtxPattern = regexp.MustCompile(`\b([A-Z]{2,3}\d{1,4}[A-Z]?)\s+(?:A\d{3}|B7\d{2}|CLRD|XPNDR|[KCELPYZRVOSWUABDFGHMNT][A-Z]{3})\b`)
	FlightNumTrailPattern = regexp.MustCompile(`([A-Z]{2,3}\d{3,4}[A-Z]?)$`)

	// ICAOPattern matches 4-letter ICAO airport codes with valid prefixes.
	ICAOPattern = regexp.MustCompile(`\b([KCELPYZRVOSWUABDFGHMNT][A-Z]{3})\b`)

	// RoutePattern matches ICAO-ICAO route format with dash separator.
	RoutePattern = regexp.MustCompile(`\b([A-Z]{4})\s*-\s*([A-Z]{4})\b`)

	// routeSlashPattern matches ICAO/ICAO route format with slash separator.
	routeSlashPattern = regexp.MustCompile(`\b([A-Z]{4})/([A-Z]{4})\b`)
)

// PDC / Clearance patterns.
var (
	RunwayOffPattern = regexp.MustCompile(`\bOFF\s+(\d{1,2}[LRC]?)\b`)
	RunwayRwyPattern = regexp.MustCompile(`\b(?:RWY|RUNWAY)\s+(\d{1,2}[LRC]?)\b`)
	RunwayDepPattern = regexp.MustCompile(`\bDEP[:\s]+[A-Z0-9]+.*?(\d{1,2}[LRC])\b`)

	SIDViaPattern     = regexp.MustCompile(`\bVIA\s+([A-Z]{2,}[0-9][A-Z0-9]*)\b`)
	SIDDepPattern     = regexp.MustCompile(`\bDEP[:\s]+([A-Z]{2,}[0-9][A-Z0-9]*)\b`)
	SIDNamedPattern   = regexp.MustCompile(`\b([A-Z]{2,}[0-9][A-Z0-9]*)\s+DEP(?:ARTURE)?\b`)
	SIDClearedPattern = regexp.MustCompile(`\bCLEARED\s+([A-Z]{2,}[0-9][A-Z0-9]*)\s+DEP`)
	// SIDWordPattern matches SIDs with word numbers, e.g., "SANEG TWO DEP" -> captures "SANEG" and "TWO".
	SIDWordPattern = regexp.MustCompile(`\b([A-Z]{3,})\s+(ONE|TWO|THREE|FOUR|FIVE|SIX|SEVEN|EIGHT|NINE)\s+DEP(?:ARTURE)?\b`)

	SquawkPattern = regexp.MustCompile(`\b(?:SQUAWK|XPNDR|XPDR|TRANSPONDER)[/:\s]+([0-7]{4})\b`)

	FreqDepPattern = regexp.MustCompile(`\b(?:DEP\s*FREQ|DPFRQ|NEXT\s*FREQ)[:\s]+(\d{3}\.\d{1,3})\b`)
	FreqPattern    = regexp.MustCompile(`\b(?:FREQ)[:\s]+(\d{3}\.\d{1,3})\b`)

	FLPattern        = regexp.MustCompile(`\bFL\s*(\d{2,3})\b`)
	FLExpPattern     = regexp.MustCompile(`\bEXP\s+(\d{2,3})\s+(?:\d+\s+)?MIN\b`)
	FLRequestPattern = regexp.MustCompile(`\bREQUESTED\s+FL\s*(\d{2,3})\b`)

	AltClimbPattern   = regexp.MustCompile(`\bCLIMB\s+(?:VIA\s+SID\s+)?TO[:\s]+(\d{3,5})\b`)
	AltPattern        = regexp.MustCompile(`\bALT\s*(\d{3,5})\b`)
	AltMaintPattern   = regexp.MustCompile(`\bMAINT(?:AIN)?\s+(\d{3,5})\s*(?:FT)?\b`)
	AltInitialPattern = regexp.MustCompile(`\bINITIAL\s+(?:ALT(?:ITUDE)?)\s+(\d{3,5})\s*(?:FT)?`)

	// AircraftDirectPattern matches ICAO aircraft type codes generically.
	// Matches 4-5 alphanumeric chars starting with a letter.
	// Post-filter in ExtractAircraftType ensures at least one digit is present.
	// Examples: A21N, B738, A320, E190, ATR72, CRJ9
	AircraftDirectPattern = regexp.MustCompile(`\b([A-Z][A-Z0-9]{3,4})\b`)
	AircraftFAAPattern    = regexp.MustCompile(`\b[HLMS]?/?([A-Z]\d{2,3}[A-Z]?)/[A-Z]\b`)

	ATISPattern = regexp.MustCompile(`\bATIS\s+([A-Z])\b`)
)

// Label 80 (Position) patterns.
var (
	Label80HeaderPattern = regexp.MustCompile(`\d+\s+(\w+)\s+\S+\s+([A-Z]{4})/([A-Z]{4})\s+\.?(\S+)`)
	Label80AltPattern    = regexp.MustCompile(`^([A-Z0-9]+),([A-Z]{4}),([A-Z]{4})`)
	PosPattern           = regexp.MustCompile(`/POS\s+([NS])(\d+\.?\d*)[/\s]*([EW])(\d+\.?\d*)`)
	AltSlashPattern      = regexp.MustCompile(`/(?:ALT|FL)\s+\+?(\d+)`)
	MachPattern          = regexp.MustCompile(`/MCH\s+(\d+)`)
	TASPattern           = regexp.MustCompile(`/TAS\s+(\d+)`)
	FOBPattern           = regexp.MustCompile(`/FOB\s+[N]?(\d+)`)
	ETAPattern           = regexp.MustCompile(`/ETA\s+(\d+[:\.]?\d*)`)
	OutPattern           = regexp.MustCompile(`/OUT\s+(\d+)`)
	OffPattern           = regexp.MustCompile(`/OFF\s+(\d+)`)
	OnPattern            = regexp.MustCompile(`/ON\s+(\d+)`)
	InPattern            = regexp.MustCompile(`/IN\s+(\d+)`)
)

// Label 16 (Waypoint Position) patterns.
var Label16Pattern = regexp.MustCompile(`^(\w+)\s*,([NS])\s*([\d.]+),([EW])\s*([\d.]+),(\d+),(\d+),(\d+),?(\d*)`)

// Label 21 (Position Report) patterns.
var Label21Pattern = regexp.MustCompile(`POSN\s+([-\d.]+)([EW])([\d.]+),\s*(\d+),(\d+),(\d+),(\d+),\s*([-\d ]+),\s*([-\d ]+),(\d+),([A-Z]{4})`)

// H1 (Flight Plan / Position / PWI) patterns.
var (
	FPNRoutePattern    = regexp.MustCompile(`:DA:([A-Z]{4}):AA:([A-Z]{4})`)
	FPNFlightPattern   = regexp.MustCompile(`FPN/FN([A-Z0-9]+)`)
	FPNWaypointPattern = regexp.MustCompile(`:F:([^/]+)`)
	FPNDepPattern      = regexp.MustCompile(`:D:([A-Z0-9]+)`)
	FPNArrPattern      = regexp.MustCompile(`:A:([A-Z0-9]+)`)
	H1PosPattern       = regexp.MustCompile(`^POS([NS])(\d{5})([EW])(\d{6}),([A-Z]+),(\d+),(\d+),([A-Z]+),(\d+),([A-Z]+),([MP]\d+)`)
)

// B2 (Oceanic Clearance) patterns.
var (
	OceanicDestPattern   = regexp.MustCompile(`CLRD TO (\w{4})`)
	OceanicFixPattern    = regexp.MustCompile(`(\d{2}[NS]\d{3}[EW])`)
	OceanicFLPattern     = regexp.MustCompile(`F(\d{3})`)
	OceanicMachPattern   = regexp.MustCompile(`M0?(\d{2,3})`)
	OceanicFlightPattern = regexp.MustCompile(`^([A-Z]{3}\d+)`)
)

// B3 (Gate Info) patterns.
var (
	B3HeaderPattern = regexp.MustCompile(`([A-Z0-9]+)-([A-Z]{4})-GATE\s+(\S+)-([A-Z]{4})`)
	B3AtisPattern   = regexp.MustCompile(`ATIS\s+([A-Z])`)
	B3TypePattern   = regexp.MustCompile(`-TYP/([A-Z0-9]+)`)
)

// 4J (Position + Weather) patterns.
var (
	Pos4JPattern = regexp.MustCompile(`/PSN(\d{5})([EW])(\d{6}),(\d{6}),(\d+),([A-Z0-9]+),(\d+),([A-Z0-9]+),([MP]\d+),(\d+)`)
	FB4JPattern  = regexp.MustCompile(`/FB(\d+)`)
)

// B6 (ADSC) patterns.
var ADSCFlightPattern = regexp.MustCompile(`([A-Z]{2,3}\d{3,4}[A-Z]?)$`)

// ICAOBlocklist contains common words and codes that look like ICAO airport codes but aren't.
var ICAOBlocklist = map[string]bool{
	// Common words.
	"WHEN": true, "WITH": true, "WILL": true, "WERE": true, "WHAT": true,
	"PUSH": true, "PULL": true, "ONLY": true, "OVER": true,
	"COPY": true, "CALL": true, "CLMB": true, "CONT": true,
	"EACH": true, "ELSE": true, "LEFT": true, "LIVE": true, "LOST": true,
	"YOUR": true, "YEAR": true, "ZERO": true, "ZONE": true,
	"READ": true, "RQST": true, "RPLY": true, "VOID": true, "VERY": true, "VFRQ": true,
	"OPER": true, "SEND": true, "STOP": true, "STAY": true, "UPDT": true, "UPON": true,
	"TIME": true, "TEST": true, "THEN": true, "TILL": true, "TAKE": true,
	"ATIS": true, "ACFT": true, "AFTN": true, "MAIN": true, "MNTN": true,
	"NEXT": true, "NONE": true, "HOLD": true, "HIGH": true, "FROM": true, "FREQ": true,
	"BACK": true, "BASE": true, "DOWN": true, "DCPC": true, "AREA": true, "ACAS": true, "APCH": true,
	"SKED": true, "SUPP": true, "WIND": true, "WIDE": true, "TAXI": true, "TURN": true, "TRSN": true,
	"PLAN": true, "PROC": true, "PDCL": true, "PDOP": true, "PFRQ": true,
	"CLRD": true, "CKPT": true, "CPDL": true, "TCAS": true, "SELC": true,
	"DATE": true, "DATA": true, "DPTS": true, "DEST": true,
	// Procedural terms often mistaken for airports.
	"STAR": true, "CODE": true, "GATE": true, "RSTR": true,
	// File/message markers.
	"FILE": true, "FULL": true, "NOSE": true, "PAGE": true,
	// British Airways internal messaging codes.
	"ASAT": true, "ALSO": true, "MINS": true, "MUST": true,
	"SIXI": true, "ECFG": true, "HELD": true, "CAPT": true, "TAIL": true,
	// Oceanic FIR/control centres (not airports).
	"EGGX": true, // Shanwick Oceanic
	"CZQX": true, // Gander Oceanic
	"BIRD": true, // Reykjavik
	"KZNY": true, // New York Oceanic
	"KZAK": true, // Oakland Oceanic
	"PAZA": true, // Anchorage Arctic
	"RJJJ": true, // Fukuoka FIR
	"VHHK": true, // Hong Kong FIR
	"WAAF": true, // Jakarta FIR
	"YBBB": true, // Brisbane Oceanic
}

// validICAOPrefixes contains valid ICAO regional prefixes.
// K (USA) is handled separately as a single-letter prefix.
var validICAOPrefixes = map[string]bool{
	// A - South Pacific (limited).
	"AG": true, "AN": true, "AY": true,
	// B - Greenland, Iceland, Kosovo.
	"BG": true, "BI": true, "BK": true,
	// C - Canada.
	"CY": true, "CZ": true,
	// D - West Africa (DI = Ivory Coast).
	"DA": true, "DB": true, "DF": true, "DG": true, "DI": true, "DN": true, "DR": true, "DT": true, "DX": true,
	// E - Northern Europe.
	"EB": true, "ED": true, "EE": true, "EF": true, "EG": true, "EH": true, "EI": true, "EK": true, "EL": true, "EN": true, "EP": true, "ES": true, "ET": true, "EV": true, "EY": true,
	// F - Central/Southern Africa, Indian Ocean.
	"FA": true, "FB": true, "FC": true, "FD": true, "FE": true, "FG": true, "FH": true, "FI": true, "FJ": true, "FK": true, "FL": true, "FM": true, "FN": true, "FO": true, "FP": true, "FQ": true, "FS": true, "FT": true, "FV": true, "FW": true, "FX": true, "FY": true, "FZ": true,
	// G - Western Africa, Maghreb.
	"GA": true, "GB": true, "GC": true, "GE": true, "GF": true, "GG": true, "GL": true, "GM": true, "GO": true, "GQ": true, "GS": true, "GU": true, "GV": true,
	// H - East Africa.
	"HA": true, "HB": true, "HC": true, "HD": true, "HE": true, "HH": true, "HK": true, "HL": true, "HR": true, "HS": true, "HT": true, "HU": true,
	// L - Southern Europe (including LS Switzerland, LL Israel, LM Malta).
	"LA": true, "LB": true, "LC": true, "LD": true, "LE": true, "LF": true, "LG": true, "LH": true, "LI": true, "LJ": true, "LK": true, "LL": true, "LM": true, "LN": true, "LO": true, "LP": true, "LQ": true, "LR": true, "LS": true, "LT": true, "LU": true, "LV": true, "LW": true, "LX": true, "LY": true, "LZ": true,
	// M - Central America, Mexico, Caribbean.
	"MB": true, "MD": true, "MG": true, "MH": true, "MK": true, "MM": true, "MN": true, "MP": true, "MR": true, "MS": true, "MT": true, "MU": true, "MW": true, "MY": true, "MZ": true,
	// N - Pacific.
	"NC": true, "NF": true, "NG": true, "NI": true, "NL": true, "NS": true, "NT": true, "NV": true, "NW": true, "NZ": true,
	// O - Middle East.
	"OA": true, "OB": true, "OE": true, "OI": true, "OJ": true, "OK": true, "OL": true, "OM": true, "OO": true, "OP": true, "OR": true, "OS": true, "OT": true, "OY": true,
	// P - Pacific, Alaska, Hawaii.
	"PA": true, "PB": true, "PC": true, "PF": true, "PG": true, "PH": true, "PJ": true, "PK": true, "PL": true, "PM": true, "PO": true, "PP": true, "PT": true, "PW": true,
	// R - Far East (RO = Japan Ryukyu/Okinawa).
	"RC": true, "RJ": true, "RK": true, "RO": true, "RP": true,
	// S - South America.
	"SA": true, "SB": true, "SC": true, "SD": true, "SE": true, "SF": true, "SG": true, "SK": true, "SL": true, "SM": true, "SN": true, "SO": true, "SP": true, "SS": true, "SU": true, "SV": true, "SW": true, "SY": true,
	// T - Caribbean (TB = Barbados, TF = French Caribbean, TI = US Virgin Islands).
	"TA": true, "TB": true, "TC": true, "TD": true, "TF": true, "TG": true, "TI": true, "TJ": true, "TK": true, "TL": true, "TN": true, "TQ": true, "TR": true, "TT": true, "TU": true, "TV": true, "TX": true,
	// U - Russia, former USSR.
	"UA": true, "UB": true, "UC": true, "UD": true, "UE": true, "UG": true, "UH": true, "UI": true, "UK": true, "UL": true, "UM": true, "UN": true, "UO": true, "UR": true, "US": true, "UT": true, "UU": true, "UW": true,
	// V - South/Southeast Asia (VA/VI = India, VC = Sri Lanka, VD = Cambodia, VM = Vietnam/Macau).
	"VA": true, "VC": true, "VD": true, "VE": true, "VG": true, "VH": true, "VI": true, "VL": true, "VM": true, "VN": true, "VO": true, "VQ": true, "VR": true, "VT": true, "VV": true, "VY": true,
	// W - Indonesia, Malaysia.
	"WA": true, "WB": true, "WI": true, "WM": true, "WP": true, "WR": true, "WS": true,
	// Y - Australia.
	"YA": true, "YB": true, "YC": true, "YD": true, "YF": true, "YG": true, "YH": true, "YI": true, "YL": true, "YM": true, "YN": true, "YO": true, "YP": true, "YR": true, "YS": true, "YT": true, "YU": true, "YV": true, "YW": true, "YY": true,
	// Z - China.
	"ZA": true, "ZB": true, "ZG": true, "ZH": true, "ZJ": true, "ZK": true, "ZL": true, "ZM": true, "ZP": true, "ZS": true, "ZU": true, "ZW": true, "ZY": true,
}

// hasValidICAOPrefix checks if a code starts with a valid regional prefix.
func hasValidICAOPrefix(code string) bool {
	if len(code) < 2 {
		return false
	}
	// K is a single-letter prefix for USA.
	if code[0] == 'K' {
		return true
	}
	return validICAOPrefixes[code[:2]]
}

// IsValidICAO checks if a potential ICAO code is likely valid.
// Validates length, character set, regional prefix, and blocklist.
func IsValidICAO(code string) bool {
	if len(code) != 4 {
		return false
	}
	if ICAOBlocklist[code] {
		return false
	}
	for _, c := range code {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	// Validate regional prefix to reject garbage like FSIL, FCQD, GJWO.
	return hasValidICAOPrefix(code)
}

// FindValidICAO finds the first valid ICAO code in text.
func FindValidICAO(text string) string {
	matches := ICAOPattern.FindAllString(text, -1)
	for _, m := range matches {
		if IsValidICAO(m) {
			return m
		}
	}
	return ""
}

// FindAllValidICAO finds all valid ICAO codes in text.
func FindAllValidICAO(text string) []string {
	matches := ICAOPattern.FindAllString(text, -1)
	var valid []string
	for _, m := range matches {
		if IsValidICAO(m) {
			valid = append(valid, m)
		}
	}
	return valid
}

// IATAToICAO maps common IATA codes to ICAO codes.
var IATAToICAO = map[string]string{
	"SFO": "KSFO", "LAX": "KLAX", "JFK": "KJFK", "ORD": "KORD",
	"DFW": "KDFW", "ATL": "KATL", "DEN": "KDEN", "SEA": "KSEA",
	"CLT": "KCLT", "PHX": "KPHX", "MIA": "KMIA", "BOS": "KBOS",
	"MSP": "KMSP", "DTW": "KDTW", "EWR": "KEWR", "LGA": "KLGA",
	"IAH": "KIAH", "ANC": "PANC", "HNL": "PHNL",
}

// IATAHint converts common IATA codes to ICAO.
func IATAHint(iata string) string {
	if icao, ok := IATAToICAO[iata]; ok {
		return icao
	}
	return iata
}

// tokenReplacer is used by Tokenize for efficient single-pass replacement.
var tokenReplacer = strings.NewReplacer("\r\n", " ", "\n", " ", "\t", " ", "/", " ")

// Tokenize splits text into tokens for efficient searching.
func Tokenize(text string) []string {
	text = tokenReplacer.Replace(text)

	fields := strings.Fields(text)
	tokens := make([]string, len(fields))
	for i, f := range fields {
		tokens[i] = strings.ToUpper(f)
	}
	return tokens
}

// wordToDigit converts a word number (ONE-NINE) to its digit equivalent.
var wordToDigit = map[string]string{
	"ONE": "1", "TWO": "2", "THREE": "3", "FOUR": "4", "FIVE": "5",
	"SIX": "6", "SEVEN": "7", "EIGHT": "8", "NINE": "9",
}

// WordToDigit converts a word number to a digit string. Returns empty if not a valid word number.
func WordToDigit(word string) string {
	return wordToDigit[strings.ToUpper(word)]
}
