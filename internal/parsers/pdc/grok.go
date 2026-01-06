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
	// Allows 2-3 letter ICAO code + 1-4 digit flight number + 0-2 optional letter suffix.
	// e.g., JST501, DAL1260, FIN5LA, QTR58U
	"FLIGHT": `[A-Z]{2,3}\d{1,4}[A-Z]{0,2}`,

	// Clearance data.
	"SQUAWK":   `[0-7]{4}`,
	"RUNWAY":   `\d{1,2}[LRC]?`,
	"ALTITUDE": `\d{3,5}`,
	"FREQ":     `\d{3}\.\d{1,3}`,

	// Aircraft types - letter + 2-3 digits + optional letter suffix.
	"AIRCRAFT": `[A-Z]\d{2,3}[A-Z]?`,

	// SID/STAR names - letters followed by digit and optional alphanumerics.
	"SID": `[A-Z]{2,}[0-9][A-Z0-9]*`,

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
	// Fields: flight, origin (IATA), destination (IATA), squawk(?), time
	{
		Name: "compact_apcdc",
		Pattern: `(?:C32)?PDC\s+\d*APCDC\s+` +
			`(?P<flight>[A-Z]{2}\d{3,4})/\d+/\d+\s+` +
			`(?P<origin>{IATA})\s+(?P<destination>{IATA})\s+` +
			`(?P<squawk>\d{3,4})?\s*(\d{4}Z)?`,
		Fields: []string{"flight", "origin", "destination", "squawk"},
	},

	// Format 1: Australian domestic (Jetstar, Qantas, Virgin Australia)
	// Two variants:
	// - "PDC 291826" (Jetstar/Qantas)
	// - "PDC UPLINK" (Virgin Australia)
	// Both followed by: FLIGHT AIRCRAFT ORIGIN TIME
	// SID can be:
	// - Digit form: "ABBEY3", "TANTA 3"
	// - Word form: "SANEG TWO"
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
			`(?:(?P<runway>{RUNWAY})\s*)?(?P<sid>[A-Z]+(?:\s*[0-9]|\s+(?:ONE|TWO|THREE|FOUR|FIVE|SIX|SEVEN|EIGHT|NINE))?)?\s*DEP`,
		Fields: []string{"flight", "aircraft", "origin", "dep_time", "destination", "runway", "sid"},
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
	// B738/L P1844 340
	{
		Name: "american",
		Pattern: `(?s)FLIGHT\s+\d+/\d+\s+{IATA}\s*-\s*{IATA}.*?PDC\s+` +
			`(?P<flight>{FLIGHT})\s+XPNDR\s+(?P<squawk>{SQUAWK})`,
		Fields: []string{"flight", "squawk"},
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

	// Format 4b: DC1 Clearance with HDG/VECTORS (Nordic/Finnish format)
	// Similar to dc1_clearance but uses HDG + VECTORS instead of VIA SID.
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
			`(?P<flight>{FLIGHT})\s+DEPART\s+(?P<origin>{IATA})\s+AT\s+\d{4}Z` +
			`.*?TRANSPONDER\s+(?P<squawk>{SQUAWK})` +
			`.*?ROUTE[:\s]+(?P<route>[A-Z0-9\s]+?)` +
			`(?:\s*REMARKS|\s*USE\s+SID)` +
			`.*?(?:USE\s+)?SID\s+(?P<sid>[A-Z0-9]+)` +
			`.*?(?:DEPARTURE\s+)?RUNWAY\s+(?P<runway>{RUNWAY})` +
			`.*?DESTINATION\s+(?P<destination>{ICAO})`,
		Fields: []string{"flight", "origin", "squawk", "route", "sid", "runway", "destination"},
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
	FormatName    string
	FlightNumber  string
	Origin        string
	Destination   string
	Aircraft      string
	Runway        string
	SID           string
	Route         string
	Squawk        string
	Altitude      string
	Frequency     string
	ATIS          string
	DepartureTime string
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
			case "destination":
				result.Destination = value
			case "aircraft":
				result.Aircraft = value
			case "runway":
				result.Runway = value
			case "sid":
				result.SID = normaliseSID(value)
			case "route":
				result.Route = cleanRoute(value)
			case "squawk":
				result.Squawk = value
			case "altitude":
				result.Altitude = value
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

		// Post-process: extract route if present.
		if result.Route == "" {
			result.Route = extractRoute(upperText)
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
				case "destination":
					trace.Result.Destination = value
				case "aircraft":
					trace.Result.Aircraft = value
				case "runway":
					trace.Result.Runway = value
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
	altitudeRe = regexp.MustCompile(`(?:CLIMB\s+(?:VIA\s+SID\s+)?TO[:\s]+|ALT\s*)(\d{3,5})`)
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