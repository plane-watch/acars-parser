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
	FlightNumFltPattern   = regexp.MustCompile(`(?:FLT|FLIGHT)\s+(\d+)(?:/\d+)?`)
	FlightNumCtxPattern   = regexp.MustCompile(`\b([A-Z]{2,3}\d{1,4}[A-Z]?)\s+(?:A\d{3}|B7\d{2}|CLRD|XPNDR)`)
	FlightNumTrailPattern = regexp.MustCompile(`([A-Z]{2,3}\d{3,4}[A-Z]?)$`)

	// ICAOPattern matches 4-letter ICAO airport codes with valid prefixes.
	ICAOPattern = regexp.MustCompile(`\b([KCELPYZRVOSWUABDFGHMNT][A-Z]{3})\b`)

	// RoutePattern matches ICAO-ICAO route format.
	RoutePattern = regexp.MustCompile(`\b([A-Z]{4})\s*-\s*([A-Z]{4})\b`)
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

// IsValidICAO checks if a potential ICAO code is likely valid.
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
	return true
}

// FindValidICAO finds the first valid ICAO code in text./exit

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

// Tokenize splits text into tokens for efficient searching.
func Tokenize(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.ReplaceAll(text, "/", " ")

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
