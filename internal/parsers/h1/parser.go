// Package h1 parses H1 label messages including FPN (flight plans), POS (positions), and PWI (wind).
package h1

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/crc"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Grok compiler singleton.
var (
	grokCompiler *patterns.Compiler
	grokOnce     sync.Once
	grokErr      error
)

func getCompiler() (*patterns.Compiler, error) {
	grokOnce.Do(func() {
		grokCompiler = patterns.NewCompiler(Formats, nil)
		grokErr = grokCompiler.Compile()
	})
	return grokCompiler, grokErr
}

// =============================================================================
// FPN - Flight Plan Parser
// =============================================================================

// RouteWaypoint represents a waypoint with its geographic coordinates.
type RouteWaypoint struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

// FPNResult represents a parsed H1 FPN flight plan message.
type FPNResult struct {
	MsgID               int64           `json:"message_id"`
	Timestamp           string          `json:"timestamp"`
	Tail                string          `json:"tail,omitempty"`
	FlightNum           string          `json:"flight_num,omitempty"`
	Origin              string          `json:"origin"`
	Destination         string          `json:"destination"`
	Route               string          `json:"route,omitempty"`
	Waypoints           []RouteWaypoint `json:"waypoints,omitempty"`
	Departure           string          `json:"departure,omitempty"`
	DepartureTransition string          `json:"departure_transition,omitempty"`
	Arrival             string          `json:"arrival,omitempty"`
	ArrivalTransition   string          `json:"arrival_transition,omitempty"`
	Approach            string          `json:"approach,omitempty"`
	ApproachType        string          `json:"approach_type,omitempty"`
	ApproachRunway      string          `json:"approach_runway,omitempty"`
	ApproachRoute       string          `json:"approach_route,omitempty"`
	ApproachWaypoints   []RouteWaypoint `json:"approach_waypoints,omitempty"`
	Truncated           bool            `json:"truncated,omitempty"`
}

func (r *FPNResult) Type() string     { return "flight_plan" }
func (r *FPNResult) MessageID() int64 { return r.MsgID }

// FPNParser parses H1 FPN flight plan messages.
type FPNParser struct{}

func init() {
	registry.Register(&FPNParser{})
}

func (p *FPNParser) Name() string     { return "fpn" }
func (p *FPNParser) Labels() []string { return []string{"H1", "4A", "HX"} }
func (p *FPNParser) Priority() int    { return 10 }

func (p *FPNParser) QuickCheck(text string) bool {
	return strings.Contains(text, "FPN") && strings.Contains(text, ":DA:")
}

func (p *FPNParser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	// Use tokeniser-based parsing.
	tokens := TokeniseFPN(msg.Text)

	// Must have origin and destination to be a valid flight plan.
	origin := tokens.GetOrigin()
	dest := tokens.GetDestination()
	if origin == "" || dest == "" {
		return nil
	}

	// Parse departure (SID) and arrival (STAR) with transitions.
	// Format: "PROCEDURE.TRANSITION" e.g., "KATZZ2.BRHMA"
	departure, departureTransition := splitProcedureTransition(tokens.GetDeparture())
	arrival, arrivalTransition := splitProcedureTransition(tokens.GetArrival())

	fp := &FPNResult{
		MsgID:               int64(msg.ID),
		Timestamp:           msg.Timestamp,
		Tail:                msg.Tail,
		Origin:              origin,
		Destination:         dest,
		FlightNum:           tokens.FlightNum,
		Departure:           departure,
		DepartureTransition: departureTransition,
		Arrival:             arrival,
		ArrivalTransition:   arrivalTransition,
	}

	// Extract route waypoints from :F: section or inline after :AA:.
	route := tokens.GetRoute()
	if route == "" {
		route = tokens.GetInlineRoute()
	}

	if route != "" {
		fp.Route = route
		fp.Waypoints = parseRouteWaypoints(route)
	}

	// Extract approach from :AP: section.
	approach := tokens.GetApproach()
	if approach != "" {
		fp.ApproachRoute = approach
		fp.Approach, fp.ApproachType, fp.ApproachRunway, fp.ApproachWaypoints = parseApproachSection(approach)
	}

	// Detect truncated messages.
	fp.Truncated = detectTruncation(msg.Text, fp.Waypoints, route)

	return fp
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *FPNParser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	quickCheckPassed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{
		Passed: quickCheckPassed,
	}

	if !quickCheckPassed {
		trace.QuickCheck.Reason = "No FPN with :DA: section found"
		return trace
	}

	// Use tokeniser-based parsing for tracing.
	tokens := TokeniseFPN(msg.Text)

	// Add extractors for key tokens.
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "origin",
		Pattern: ":DA: section",
		Matched: tokens.GetOrigin() != "",
		Value:   tokens.GetOrigin(),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "destination",
		Pattern: ":AA: section",
		Matched: tokens.GetDestination() != "",
		Value:   tokens.GetDestination(),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "departure",
		Pattern: ":DP: section",
		Matched: tokens.GetDeparture() != "",
		Value:   tokens.GetDeparture(),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "arrival",
		Pattern: ":AR: section",
		Matched: tokens.GetArrival() != "",
		Value:   tokens.GetArrival(),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "route",
		Pattern: ":F: section or inline route",
		Matched: tokens.GetRoute() != "" || tokens.GetInlineRoute() != "",
		Value: func() string {
			if r := tokens.GetRoute(); r != "" {
				return r
			}
			return tokens.GetInlineRoute()
		}(),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "approach",
		Pattern: ":AP: section",
		Matched: tokens.GetApproach() != "",
		Value:   tokens.GetApproach(),
	})

	trace.Matched = tokens.GetOrigin() != "" && tokens.GetDestination() != ""
	return trace
}

// parseRouteWaypoints extracts waypoints with coordinates from a route string.
// Route format: "WAYPOINT,N31490E035327.AIRWAY..NEXT,COORDS"
func parseRouteWaypoints(route string) []RouteWaypoint {
	var waypoints []RouteWaypoint

	// Parse route segments separated by ".." (waypoint separator).
	parts := strings.Split(route, "..")
	for _, part := range parts {
		if part == "" {
			continue
		}
		// Strip airway designator (after the period following coordinates).
		segment := strings.Split(part, ".")[0]
		if wpt := parseWaypointWithCoords(segment); wpt != nil {
			waypoints = append(waypoints, *wpt)
		}
	}

	return waypoints
}

// parseApproachSection extracts the approach details and waypoints from an AP section.
// Format: "ILS22L..ZIGEE,N37312W102468..STAMY,..." or "RNAV 07R..WAYPOINT"
// Returns: full approach string, approach type (ILS/RNAV/VOR/etc.), runway, waypoints.
func parseApproachSection(approach string) (string, string, string, []RouteWaypoint) {
	var waypoints []RouteWaypoint
	var approachFull, approachType, runway string

	parts := strings.Split(approach, "..")
	for i, part := range parts {
		if part == "" {
			continue
		}
		// First part is typically the approach procedure (e.g., "ILS22L", "RNAV 07R").
		if i == 0 {
			approachFull = strings.Split(part, ",")[0]
			approachType, runway = parseApproachProcedure(approachFull)
		}
		// Extract waypoint with coordinates (skip the approach procedure itself).
		if i > 0 {
			if wpt := parseWaypointWithCoords(part); wpt != nil {
				waypoints = append(waypoints, *wpt)
			}
		}
	}

	return approachFull, approachType, runway, waypoints
}

// parseApproachProcedure splits an approach procedure into type and runway.
// Examples:
//   - "ILS07R" -> ("ILS", "07R")
//   - "RNAV 22L" -> ("RNAV", "22L")
//   - "VOR-A" -> ("VOR", "A")
//   - "ILS 22" -> ("ILS", "22")
//   - "ILS 17L.RIVET" -> ("ILS", "17L") - strips transition
//   - "RNVY 22L" -> ("RNAV", "22L") - normalises RNVY to RNAV
func parseApproachProcedure(proc string) (approachType, runway string) {
	proc = strings.TrimSpace(proc)
	if proc == "" {
		return "", ""
	}

	// Handle space-separated format (e.g., "ILS 22L", "RNAV 07R", "ILS 17L.RIVET").
	if idx := strings.Index(proc, " "); idx > 0 {
		approachType = strings.TrimSpace(proc[:idx])
		runway = strings.TrimSpace(proc[idx+1:])
		approachType = normaliseApproachType(approachType)
		// Strip transition (e.g., "17L.RIVET" -> "17L").
		if dotIdx := strings.Index(runway, "."); dotIdx > 0 {
			runway = runway[:dotIdx]
		}
		return approachType, runway
	}

	// Handle concatenated format (e.g., "ILS07R", "RNAV22L").
	// Approach types: ILS, RNAV, RNVY, VOR, NDB, LOC, LDA, GPS, RNP.
	approachTypes := []string{"RNAV", "RNVY", "ILS", "VOR", "NDB", "LOC", "LDA", "GPS", "RNP"}
	for _, at := range approachTypes {
		if strings.HasPrefix(proc, at) {
			approachType = normaliseApproachType(at)
			runway = extractRunway(proc[len(at):])
			return approachType, runway
		}
	}

	// Couldn't parse - return full string as type.
	return proc, ""
}

// normaliseApproachType standardises approach type abbreviations.
func normaliseApproachType(t string) string {
	switch t {
	case "RNVY":
		return "RNAV"
	default:
		return t
	}
}

// extractRunway extracts a valid runway designator from a string.
// A runway is 2 digits (01-36) optionally followed by L, R, or C.
// Examples: "35R0EAC" -> "35R", "22L" -> "22L", "07" -> "07"
// Handles checksums and other trailing garbage.
func extractRunway(s string) string {
	if len(s) < 2 {
		return ""
	}

	// Find the end of the runway designator.
	// First, find 2 digits.
	if s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return ""
	}

	// Check for L/R/C suffix.
	if len(s) >= 3 && (s[2] == 'L' || s[2] == 'R' || s[2] == 'C') {
		return s[:3]
	}

	return s[:2]
}

// splitProcedureTransition splits a procedure string into procedure and transition.
// Format: "PROCEDURE.TRANSITION" e.g., "KATZZ2.BRHMA" -> ("KATZZ2", "BRHMA")
// If no transition, returns (procedure, "").
func splitProcedureTransition(s string) (procedure, transition string) {
	if s == "" {
		return "", ""
	}
	if idx := strings.Index(s, "."); idx > 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

// detectTruncation checks if an FPN message is truncated or corrupt by verifying
// the CRC checksum. For messages with /WD section, the last 4 hex characters are
// the CRC which should verify to 0x1D0F when appended as bytes.
// Returns true if the message is truncated/corrupt, false if valid or unknown.
func detectTruncation(text string, waypoints []RouteWaypoint, route string) bool {
	// Check for multi-part message markers without proper termination.
	if strings.Contains(text, "#M1") && !strings.Contains(text, "#MD") {
		return true
	}

	// For messages with /WD section, verify CRC.
	// Format: ...data/WD,,,,XXXX where XXXX is the 4-char hex checksum.
	if idx := strings.Index(text, "/WD"); idx >= 0 {
		// Find the checksum at the end (last 4 hex chars).
		text = strings.TrimSpace(text)
		if len(text) >= 4 {
			checksumHex := text[len(text)-4:]
			// Verify all 4 chars are hex digits.
			if crc.IsHexDigit(checksumHex[0]) && crc.IsHexDigit(checksumHex[1]) &&
				crc.IsHexDigit(checksumHex[2]) && crc.IsHexDigit(checksumHex[3]) {

				// Decode checksum hex to bytes.
				checksumBytes := []byte{
					crc.HexToByte(checksumHex[0], checksumHex[1]),
					crc.HexToByte(checksumHex[2], checksumHex[3]),
				}

				// Get message without the hex checksum string.
				msgWithoutCRC := text[:len(text)-4]

				// Verify using the shared CRC package.
				if !crc.Verify16Arinc([]byte(msgWithoutCRC), checksumBytes) {
					return true // CRC mismatch - message is corrupt/truncated.
				}
				return false // CRC valid - message is complete.
			}
		}
	}

	// For messages without /WD, use basic heuristics.
	text = strings.TrimSpace(text)

	// Message ends with section marker (incomplete).
	if strings.HasSuffix(text, ":") {
		return true
	}

	// Message ends mid-route (trailing comma or double period).
	if strings.HasSuffix(text, ",") || strings.HasSuffix(text, "..") {
		return true
	}

	// Check for incomplete coordinate after the route section.
	// Valid coordinates are like N33490E034050 (13 chars). If it ends with
	// a partial coordinate (direction + digits but too short), it's truncated.
	if route != "" {
		// Find last segment after comma (potential coordinate).
		if idx := strings.LastIndex(route, ","); idx >= 0 {
			lastPart := route[idx+1:]
			if len(lastPart) > 0 {
				first := lastPart[0]
				// Starts with direction indicator but too short to be a full coordinate.
				if (first == 'N' || first == 'S') && len(lastPart) < 13 {
					// Verify it has digits after the direction (not just a waypoint name).
					if len(lastPart) > 1 && lastPart[1] >= '0' && lastPart[1] <= '9' {
						return true
					}
				}
			}
		}
	}

	return false
}

// isValidWaypoint checks if a string looks like a valid waypoint name.
// Valid waypoints are 2-5 uppercase letters, or 3-5 uppercase letters followed by 1-2 digits.
// Filters out altitude restrictions (P155, L620), airways (N123), and garbage.
func isValidWaypoint(s string) bool {
	if len(s) < 2 || len(s) > 6 {
		return false
	}

	// Filter out common altitude restriction prefixes.
	if len(s) >= 2 {
		prefix := s[0]
		if prefix == 'P' || prefix == 'L' || prefix == 'N' || prefix == 'A' || prefix == 'B' {
			// Check if rest is all digits (altitude restriction).
			allDigits := true
			for _, c := range s[1:] {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits && len(s) > 1 {
				return false
			}
		}
	}

	// Count letters and digits.
	letters := 0
	digits := 0
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			letters++
		} else if c >= '0' && c <= '9' {
			// Digits should only appear at the end.
			if i < len(s)-2 && letters < 3 {
				return false
			}
			digits++
		} else {
			return false // Invalid character.
		}
	}

	// Must have at least 2 letters.
	if letters < 2 {
		return false
	}

	// Maximum 2 trailing digits for waypoints like "UNGAP" or "WPT01".
	if digits > 2 {
		return false
	}

	return true
}

// parseWaypointCoords parses a coordinate string in the format N31490E035327 or S12345W098765.
// Returns latitude and longitude in decimal degrees, or (0, 0) if parsing fails.
// Format: [N|S]DDMMT[E|W]DDDMMT where DD/DDD=degrees, MM=minutes, T=tenths of minutes.
func parseWaypointCoords(coordStr string) (lat, lon float64) {
	if len(coordStr) < 11 {
		return 0, 0
	}

	// Find the longitude direction marker (E or W) to split the string.
	// It should be after the latitude portion.
	lonDirIdx := -1
	for i := 5; i < len(coordStr); i++ {
		if coordStr[i] == 'E' || coordStr[i] == 'W' {
			lonDirIdx = i
			break
		}
	}

	if lonDirIdx < 1 {
		return 0, 0
	}

	// Split into latitude and longitude parts.
	latPart := coordStr[:lonDirIdx]
	lonPart := coordStr[lonDirIdx:]

	// Parse latitude: [N|S]DDMMT (direction + 5 digits).
	if len(latPart) < 2 {
		return 0, 0
	}
	latDir := string(latPart[0])
	latVal := latPart[1:]

	// Parse longitude: [E|W]DDDMMT (direction + 6 digits).
	if len(lonPart) < 2 {
		return 0, 0
	}
	lonDir := string(lonPart[0])
	lonVal := lonPart[1:]

	lat = patterns.ParseLatitude(latVal, latDir)
	lon = patterns.ParseLongitude(lonVal, lonDir)

	return lat, lon
}

// parseWaypointWithCoords extracts a waypoint name and its coordinates from a route segment.
// Input format: "WAYPOINT,N31490E035327" or just "WAYPOINT".
// Returns the RouteWaypoint with name and coordinates (if present).
func parseWaypointWithCoords(segment string) *RouteWaypoint {
	// Split on comma to separate waypoint name from coordinates.
	parts := strings.SplitN(segment, ",", 2)
	if len(parts) == 0 {
		return nil
	}

	name := parts[0]
	if !isValidWaypoint(name) {
		return nil
	}

	wpt := &RouteWaypoint{Name: name}

	// If there's a coordinate part, parse it.
	if len(parts) == 2 && len(parts[1]) >= 11 {
		wpt.Latitude, wpt.Longitude = parseWaypointCoords(parts[1])
	}

	return wpt
}

// =============================================================================
// H1 POS - Position Report Parser
// =============================================================================

// H1PosResult represents a parsed H1 position message.
type H1PosResult struct {
	MsgID           int64   `json:"message_id"`
	Timestamp       string  `json:"timestamp"`
	Tail            string  `json:"tail,omitempty"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	ReportTime      string  `json:"report_time,omitempty"`
	FlightLevel     int     `json:"flight_level,omitempty"`
	GroundSpeed     int     `json:"ground_speed,omitempty"`
	CurrentWaypoint string  `json:"current_waypoint,omitempty"`
	NextWaypoint    string  `json:"next_waypoint,omitempty"`
	ThirdWaypoint   string  `json:"third_waypoint,omitempty"`
	ETA             string  `json:"eta,omitempty"`
	Temperature     int     `json:"temperature,omitempty"`
	WindDir         int     `json:"wind_dir,omitempty"`
	WindSpeed       int     `json:"wind_speed,omitempty"`
	Origin          string  `json:"origin,omitempty"`      // From h1_position_route.
	Destination     string  `json:"destination,omitempty"` // From h1_position_route.
}

func (r *H1PosResult) Type() string     { return "h1_position" }
func (r *H1PosResult) MessageID() int64 { return r.MsgID }

// H1PosParser parses H1 POS position messages.
type H1PosParser struct{}

func init() {
	registry.Register(&H1PosParser{})
}

func (p *H1PosParser) Name() string     { return "h1pos" }
func (p *H1PosParser) Labels() []string { return []string{"H1"} }
func (p *H1PosParser) Priority() int    { return 20 }

func (p *H1PosParser) QuickCheck(text string) bool {
	// Starts with POS but not POS/ (which is part of other messages).
	return strings.HasPrefix(text, "POS") && !strings.HasPrefix(text, "POS/")
}

func (p *H1PosParser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	text := msg.Text
	match := compiler.Parse(text)
	if match == nil {
		return nil
	}

	// Check for valid H1 position format.
	switch match.FormatName {
	case "h1_position_time", "h1_position_alt":
	case "h1_position_route":
		if !patterns.IsValidICAO(match.Captures["origin"]) || !patterns.IsValidICAO(match.Captures["dest"]) {
			return nil
		}
	default:
		return nil
	}

	// Parse coordinates using shared utility.
	lat := patterns.ParseLatitude(match.Captures["lat"], match.Captures["lat_dir"])
	lon := patterns.ParseLongitude(match.Captures["lon"], match.Captures["lon_dir"])

	result := &H1PosResult{
		MsgID:           int64(msg.ID),
		Timestamp:       msg.Timestamp,
		Tail:            msg.Tail,
		Latitude:        lat,
		Longitude:       lon,
		CurrentWaypoint: match.Captures["curr_wpt"],
		NextWaypoint:    match.Captures["next_wpt"],
		ThirdWaypoint:   match.Captures["wpt3"],
		ETA:             match.Captures["eta"],
		Origin:          match.Captures["origin"],
		Destination:     match.Captures["dest"],
	}

	// Handle format-specific fields.
	if match.FormatName != "h1_position_alt" {
		// The time-based formats (h1_position_time and h1_position_route):
		// report_time, altitude and wind.
		result.ReportTime = match.Captures["report_time"]

		// Parse altitude (FL in hundreds).
		if alt, err := parseIntField(match.Captures["altitude"]); err == nil {
			result.FlightLevel = alt
		}

		// Parse wind data (5 digits: DDDSS = direction + speed).
		// The wind is DDDSS or DDDSSS: three digits of direction, then the speed.
		if windStr := match.Captures["wind"]; len(windStr) == 5 || len(windStr) == 6 {
			if dir, err := parseIntField(windStr[:3]); err == nil {
				result.WindDir = dir
			}
			if spd, err := parseIntField(windStr[3:]); err == nil {
				result.WindSpeed = spd
			}
		}
	} else {
		// Altitude-based format: has flight level and ground speed.
		if alt, err := parseIntField(match.Captures["altitude"]); err == nil {
			result.FlightLevel = alt
		}
		if gs, err := parseIntField(match.Captures["gs"]); err == nil {
			result.GroundSpeed = gs
		}
	}

	// Parse temperature (e.g., "M56" = -56°C, "P10" = +10°C).
	if tempStr := match.Captures["temp"]; len(tempStr) >= 2 {
		switch tempStr[0] {
		case 'M':
			if temp, err := parseIntField(tempStr[1:]); err == nil {
				result.Temperature = -temp
			}
		case 'P':
			if temp, err := parseIntField(tempStr[1:]); err == nil {
				result.Temperature = temp
			}
		}
	}

	return result
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *H1PosParser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	quickCheckPassed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{
		Passed: quickCheckPassed,
	}

	if !quickCheckPassed {
		trace.QuickCheck.Reason = "No POS prefix found (or has POS/ which is different)"
		return trace
	}

	compiler, err := getCompiler()
	if err != nil {
		trace.QuickCheck.Reason = "Failed to get compiler: " + err.Error()
		return trace
	}

	compilerTrace := compiler.ParseWithTrace(msg.Text)

	for _, ft := range compilerTrace.Formats {
		trace.Formats = append(trace.Formats, registry.FormatTrace{
			Name:     ft.Name,
			Matched:  ft.Matched,
			Pattern:  ft.Pattern,
			Captures: ft.Captures,
		})
	}

	// Check if a valid H1 position format was matched.
	validFormat := false
	if compilerTrace.Match != nil {
		if compilerTrace.Match.FormatName == "h1_position_time" ||
			compilerTrace.Match.FormatName == "h1_position_alt" {
			validFormat = true
		}
	}

	trace.Matched = validFormat
	return trace
}

// parseIntField is a helper to parse integer fields.
func parseIntField(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}

// =============================================================================
// PWI - Predicted Wind Information Parser
// =============================================================================

// PWIResult represents predicted wind information along a route.
type PWIResult struct {
	MsgID        int64            `json:"message_id"`
	Timestamp    string           `json:"timestamp"`
	Tail         string           `json:"tail,omitempty"`
	ReportTime   string           `json:"report_time,omitempty"`
	ClimbWinds   []AltitudeWind   `json:"climb_winds,omitempty"`
	DescentWinds []AltitudeWind   `json:"descent_winds,omitempty"`
	RouteWinds   []RouteWindLayer `json:"route_winds,omitempty"`
}

func (r *PWIResult) Type() string     { return "pwi" }
func (r *PWIResult) MessageID() int64 { return r.MsgID }

// AltitudeWind represents wind at a specific altitude.
type AltitudeWind struct {
	FlightLevel int `json:"flight_level"`
	WindDir     int `json:"wind_dir"`
	WindSpeed   int `json:"wind_speed"`
}

// RouteWindLayer represents wind data at waypoints for a flight level.
type RouteWindLayer struct {
	FlightLevel int            `json:"flight_level"`
	Waypoints   []WaypointWind `json:"waypoints"`
}

// WaypointWind represents wind and temperature at a waypoint.
type WaypointWind struct {
	Waypoint    string `json:"waypoint"`
	WindDir     int    `json:"wind_dir"`
	WindSpeed   int    `json:"wind_speed"`
	Temperature int    `json:"temperature,omitempty"`
}

// PWIParser parses H1 PWI predicted wind messages.
type PWIParser struct{}

func init() {
	registry.Register(&PWIParser{})
}

func (p *PWIParser) Name() string     { return "pwi" }
func (p *PWIParser) Labels() []string { return []string{"H1"} }
func (p *PWIParser) Priority() int    { return 30 }

func (p *PWIParser) QuickCheck(text string) bool {
	return strings.Contains(pwiNormaliser.Replace(text), "PWI/")
}

func (p *PWIParser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	// Normalise before looking for the header, which wrapping or a block
	// marker can also split.
	text, ok := pwiBody(msg.Text)
	if !ok {
		return nil
	}

	report := &PWIResult{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	// Split by section markers (TS, CB, WD, DD).
	sections := strings.Split(text, "/")
	for _, section := range sections {
		if len(section) < 2 {
			continue
		}

		prefix := section[:2]
		data := section[2:]

		switch prefix {
		case "TS":
			// Timestamp section (e.g., "TS080544,311225").
			if commaIdx := strings.Index(data, ","); commaIdx > 0 {
				report.ReportTime = data[:commaIdx]
			} else {
				report.ReportTime = data
			}
		case "CB":
			report.ClimbWinds = parseAltitudeWinds(beforeSubFields(data))
		case "DD":
			report.DescentWinds = parseAltitudeWinds(beforeSubFields(data))
		case "WD":
			if layer := parseRouteWindLayer(data); layer != nil {
				report.RouteWinds = append(report.RouteWinds, *layer)
			}
		}
	}

	// Only return if we found useful data.
	if len(report.ClimbWinds) == 0 && len(report.DescentWinds) == 0 && len(report.RouteWinds) == 0 {
		return nil
	}

	return report
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *PWIParser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	quickCheckPassed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{
		Passed: quickCheckPassed,
	}

	if !quickCheckPassed {
		trace.QuickCheck.Reason = "No PWI/ marker found"
		return trace
	}

	// Use the same normalisation as Parse.
	text, ok := pwiBody(msg.Text)
	if !ok {
		trace.QuickCheck.Reason = "PWI/ not found"
		return trace
	}

	// Add extractors for each section type.
	sections := strings.Split(text, "/")
	hasTS := false
	hasCB := false
	hasDD := false
	hasWD := false

	for _, section := range sections {
		if len(section) < 2 {
			continue
		}
		switch section[:2] {
		case "TS":
			hasTS = true
		case "CB":
			hasCB = true
		case "DD":
			hasDD = true
		case "WD":
			hasWD = true
		}
	}

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "timestamp",
		Pattern: "TS section",
		Matched: hasTS,
		Value:   fmt.Sprintf("found=%v", hasTS),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "climb_winds",
		Pattern: "CB section",
		Matched: hasCB,
		Value:   fmt.Sprintf("found=%v", hasCB),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "descent_winds",
		Pattern: "DD section",
		Matched: hasDD,
		Value:   fmt.Sprintf("found=%v", hasDD),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "route_winds",
		Pattern: "WD section",
		Matched: hasWD,
		Value:   fmt.Sprintf("found=%v", hasWD),
	})

	// A section being present does not mean it could be read: the trace
	// matches only when Parse produces a result.
	trace.Matched = p.Parse(msg) != nil
	return trace
}

// parseAltitudeWinds parses altitude wind data like "100252039.150251040.200246036".
// pwiNormaliser removes what transport adds inside a PWI message: line
// wrapping ("\r", "\n", "\t"), which can fall inside a wind group
// ("310\n\t270031"), and the block markers inserted where a multi-block
// message's blocks join ("- #MD", or "- #M1" to "- #M3", all four of which
// occur in the January 2026 corpus), which can fall inside a token
// ("VE- #MDLDT" is VELDT). It uses plain string replacement so that
// QuickCheck can use it.
var pwiNormaliser = strings.NewReplacer(
	"\r", "", "\n", "", "\t", "",
	"- #MD", "", "- #M1", "", "- #M2", "", "- #M3", "",
)

// pwiBody returns the normalised text after the "PWI/" header, with the
// trailing checksum removed, and false if there is no header.
func pwiBody(text string) (string, bool) {
	n := pwiNormaliser.Replace(text)
	i := strings.Index(n, "PWI/")
	if i < 0 {
		return "", false
	}
	return stripPWIChecksum(n[i+4:]), true
}

// parseAltitudeWinds parses dot-separated climb or descent wind groups.
// Groups are nine digits, FFFDDDSSS (flight level, direction, speed), or
// occasionally eight (727 in the January 2026 corpus). The caller removes any
// ":" sub-field and the message checksum first, so a group is used only if it
// is exactly eight or nine digits.
func parseAltitudeWinds(data string) []AltitudeWind {
	var winds []AltitudeWind
	parts := strings.Split(data, ".")

	for _, part := range parts {
		if !isDigits(part) || (len(part) != 8 && len(part) != 9) {
			continue
		}
		var fl, dir, speed int
		if len(part) == 9 {
			_, _ = fmt.Sscanf(part, "%3d%3d%3d", &fl, &dir, &speed)
		} else if len(part) == 8 {
			_, _ = fmt.Sscanf(part, "%2d%3d%3d", &fl, &dir, &speed)
			if speed > 200 {
				_, _ = fmt.Sscanf(part, "%3d%3d%2d", &fl, &dir, &speed)
			}
		}
		if fl > 0 && dir > 0 {
			winds = append(winds, AltitudeWind{
				FlightLevel: fl,
				WindDir:     dir,
				WindSpeed:   speed,
			})
		}
	}
	return winds
}

// parseRouteWindLayer parses one route wind layer: the flight level, then for
// each waypoint its wind and, optionally, its altitude and temperature.
// Handles formats like:
// - "410,EHGG,348048,410M69.SONEB,352048,410M69.OLDOD,..."
// - "380,N53089E019480,270014.N53196E019325,260015.LARMA,236027" (no temperatures)
//
// The layer is split on both "," and ".", since "." separates a temperature
// from the next waypoint (and, in the second form, a wind from the next
// waypoint). A token is taken as a waypoint only when the token after it is a
// valid wind; otherwise parsing moves on by one token, so an unreadable field
// never causes the following waypoints to be skipped.
func parseRouteWindLayer(data string) *RouteWindLayer {
	tokens := strings.FieldsFunc(data, func(r rune) bool { return r == ',' || r == '.' })
	if len(tokens) < 2 {
		return nil
	}

	var fl int
	_, _ = fmt.Sscanf(tokens[0], "%d", &fl)
	if fl == 0 {
		return nil
	}

	layer := &RouteWindLayer{FlightLevel: fl}
	for i := 1; i < len(tokens); {
		if !isRouteWaypoint(tokens[i]) || i+1 >= len(tokens) {
			i++
			continue
		}
		dir, speed, ok := parseRouteWind(tokens[i+1])
		if !ok {
			i++
			continue
		}
		ww := WaypointWind{Waypoint: tokens[i], WindDir: dir, WindSpeed: speed}
		next := i + 2
		if next < len(tokens) && routeTemperaturePattern.MatchString(tokens[next]) {
			ww.Temperature = parseRouteTemperature(tokens[next])
			next++
		}
		layer.Waypoints = append(layer.Waypoints, ww)
		i = next
	}

	if len(layer.Waypoints) == 0 {
		return nil
	}
	return layer
}

// routeWaypointPattern matches a route wind waypoint: a named fix ("LARMA",
// "VHP", "BO613"), a runway ("RW19C"), a lat/lon point ("N53089E019480",
// "N50W020"), or an ARINC 424 oceanic lat/lon waypoint ("5350N" is 53N 050W,
// "48N70" is 48N 170W; 30,240 and 3,914 tokens of these two forms in the
// January 2026 corpus). Wind and temperature fields never match: winds are
// all digits and route temperatures start with three digits.
var routeWaypointPattern = regexp.MustCompile(`^(?:[A-Z][A-Z0-9]{1,12}|\d{4}[NSEW]|\d{2}[NSEW]\d{2})$`)

// bareTemperaturePattern matches a temperature on its own ("M49", "P05"),
// which is never a waypoint.
var bareTemperaturePattern = regexp.MustCompile(`^[MP]\d{1,2}$`)

// isRouteWaypoint reports whether s looks like a route wind waypoint.
func isRouteWaypoint(s string) bool {
	return routeWaypointPattern.MatchString(s) && !bareTemperaturePattern.MatchString(s)
}

// routeTemperaturePattern matches a route wind altitude and temperature field,
// such as "400M58" (FL400, minus 58 C) or "080P5".
var routeTemperaturePattern = regexp.MustCompile(`^\d{3}([MP])(\d{1,2})$`)

// parseRouteTemperature reads the temperature from a route wind altitude
// field. The message checksum has already been removed by stripPWIChecksum, so
// the field must match exactly; otherwise no temperature is read.
func parseRouteTemperature(field string) int {
	m := routeTemperaturePattern.FindStringSubmatch(field)
	if m == nil {
		return 0
	}
	value := 0
	for _, c := range m[2] {
		value = value*10 + int(c-'0')
	}
	if m[1] == "M" {
		return -value
	}
	return value
}

// parseRouteWind reads a route wind token as DDDSSS or DDDSS (direction in
// degrees, then speed in knots). Five-digit winds occur in about 5% of route
// wind groups in the January 2026 corpus.
func parseRouteWind(token string) (dir, speed int, ok bool) {
	if !isDigits(token) || (len(token) != 5 && len(token) != 6) {
		return 0, 0, false
	}
	_, _ = fmt.Sscanf(token[:3], "%d", &dir)
	_, _ = fmt.Sscanf(token[3:], "%d", &speed)
	// A direction beyond 360 degrees or a speed of 400 kt or more is not a
	// wind, so the token is not read as one.
	if dir > 360 || speed >= 400 {
		return 0, 0, false
	}
	return dir, speed, true
}

// beforeSubFields returns s up to its first ":" or ",". In climb and descent
// sections these introduce further sub-fields (":,,,M16,1024" or ",,,,").
func beforeSubFields(s string) string {
	if i := strings.IndexAny(s, ":,"); i >= 0 {
		return s[:i]
	}
	return s
}

// pwiSectionTokens gives, for each PWI section, the pattern a complete final
// token of that section must match: climb and descent groups (eight or nine
// digits), a route wind (five or six digits) or route altitude and
// temperature, or a timestamp.
var pwiSectionTokens = map[string]*regexp.Regexp{
	"CB": regexp.MustCompile(`^\d{8,9}$`),
	"DD": regexp.MustCompile(`^\d{8,9}$`),
	"WD": regexp.MustCompile(`^(?:\d{5,6}|\d{3}[MP]\d{1,2})$`),
	"TS": regexp.MustCompile(`^\d{4,6}$`),
}

// isCompletePWIToken reports whether token is a complete final token for its
// section. An unknown section accepts any of the section patterns.
func isCompletePWIToken(token, section string) bool {
	if re, ok := pwiSectionTokens[section]; ok {
		return re.MatchString(token)
	}
	for _, re := range pwiSectionTokens {
		if re.MatchString(token) {
			return true
		}
	}
	return false
}

// stripPWIChecksum removes the 4-character hex checksum that complete PWI
// messages end with. The checksum is not always present (truncated and some
// single-block messages lack it), and it can begin with digits, so it is
// removed only when the message's final token, judged by the grammar of the
// section it is in, is not complete with it and is complete without it (or
// the text then ends with a separator). For example "100328073B13" in a DD
// section is the eight-digit group 10032807 followed by 3B13, and
// "242701234" in a WD section is the wind 24270 followed by 1234, while a
// message ending in the wind "258103" is left alone.
func stripPWIChecksum(text string) string {
	if len(text) < 4 {
		return text
	}
	sum := text[len(text)-4:]
	for i := 0; i < 4; i++ {
		if !crc.IsHexDigit(sum[i]) {
			return text
		}
	}
	if token, section := lastPWIToken(text); isCompletePWIToken(token, section) {
		return text
	}
	rest := text[:len(text)-4]
	token, section := lastPWIToken(rest)
	if token == "" || isCompletePWIToken(token, section) {
		return rest
	}
	return text
}

// lastPWIToken returns the text after the last PWI field separator, and the
// two-letter code of the section it is in (CB, DD, TS or WD; empty if
// unknown). When the token starts its section, the section code is not part
// of the token.
func lastPWIToken(s string) (token, section string) {
	if j := strings.LastIndexByte(s, '/'); j >= 0 && len(s) >= j+3 {
		section = s[j+1 : j+3]
	} else if len(s) >= 2 {
		section = s[:2]
	}
	i := strings.LastIndexAny(s, ".,/:")
	token = s[i+1:]
	if (i < 0 || s[i] == '/') && len(token) >= 2 {
		switch token[:2] {
		case "CB", "DD", "TS", "WD":
			token = token[2:]
		}
	}
	return token, section
}

// isDigits reports whether s is non-empty and consists only of ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
