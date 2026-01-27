// Package h1 contains parsers for H1 label messages.
package h1

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// TrajectoryResult represents a parsed aircraft trajectory message.
// These messages contain position history data, typically from Boeing 737 MAX aircraft.
type TrajectoryResult struct {
	MsgID        int64      `json:"message_id,omitempty"`
	Registration string     `json:"registration"`
	AircraftType string     `json:"aircraft_type"`
	Date         string     `json:"date"`                   // YYMMDD format
	FlightNumber string     `json:"flight_number,omitempty"`
	Origin       string     `json:"origin,omitempty"`
	Destination  string     `json:"destination,omitempty"`
	Distance     int        `json:"distance,omitempty"`     // Nautical miles
	SystemID     string     `json:"system_id,omitempty"`
	Positions    []Position `json:"positions"`
}

// Position represents a single position report in a trajectory.
type Position struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Time        string  `json:"time"`                  // HHMMSS format
	Altitude    int     `json:"altitude"`              // Feet
	Temperature float64 `json:"temperature,omitempty"` // Celsius
	Heading     int     `json:"heading,omitempty"`     // Degrees
	Speed       int     `json:"speed,omitempty"`       // Knots (ground speed)
	Phase       string  `json:"phase,omitempty"`       // Flight phase: IC, CL, ER, DC, AP, TO
}

func (r *TrajectoryResult) Type() string     { return "trajectory" }
func (r *TrajectoryResult) MessageID() int64 { return r.MsgID }

// Header pattern: ++86501,N8967Q,B7378MAX,260112,WN2085,KLAS,KBNA,0261,SMX34-2502-F320
// Also handles: ++76502,XXX,B737-800,260111,WN0297,KMDW,KLAX,1175,SW2501
var trajectoryHeaderRe = regexp.MustCompile(`^\+\+\d+,\s*([A-Z0-9-]+),([A-Z0-9-]+),(\d{6}),([A-Z0-9]*),([A-Z]{4}),([A-Z]{4}),(\d+),([A-Z0-9-]+)`)

// Position pattern: N3702.1,W09921.8,120918,39000,-64.3,256,037,ER,00000,0,
// Note: Temperature may have leading space for positive values (e.g., " 05.3" vs "-48.3").
// Note: Altitude can be negative for ground-level readings (e.g., "-0270").
var positionRe = regexp.MustCompile(`([NS])(\d{2})(\d{2}\.\d),([EW])(\d{2,3})(\d{2}\.\d),(\d{6}),(-?\d+),\s*([+-]?\d+\.?\d*),(\d+),(\d+),([A-Z]{2}),`)

// TrajectoryParser parses aircraft trajectory/position history messages.
type TrajectoryParser struct{}

func init() {
	registry.Register(&TrajectoryParser{})
}

// Name returns the parser's unique identifier.
func (p *TrajectoryParser) Name() string { return "trajectory" }

// Labels returns which ACARS labels this parser handles.
func (p *TrajectoryParser) Labels() []string { return []string{"H1"} }

// Priority determines order when multiple parsers match.
func (p *TrajectoryParser) Priority() int { return 50 }

// QuickCheck performs a fast string check before expensive regex.
func (p *TrajectoryParser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "++86501") ||
		strings.HasPrefix(text, "++76502")
}

// Parse extracts trajectory data from the message.
func (p *TrajectoryParser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	result := &TrajectoryResult{
		MsgID: int64(msg.ID),
	}

	// Normalise line endings.
	text := strings.ReplaceAll(msg.Text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Parse header.
	headerMatch := trajectoryHeaderRe.FindStringSubmatch(text)
	if headerMatch == nil {
		return nil
	}

	result.Registration = strings.TrimSpace(headerMatch[1])
	result.AircraftType = headerMatch[2]
	result.Date = headerMatch[3]
	result.FlightNumber = headerMatch[4]
	result.Origin = headerMatch[5]
	result.Destination = headerMatch[6]
	if dist, err := strconv.Atoi(headerMatch[7]); err == nil {
		result.Distance = dist
	}
	result.SystemID = headerMatch[8]

	// Parse position entries.
	posMatches := positionRe.FindAllStringSubmatch(text, -1)
	for _, m := range posMatches {
		pos := Position{}

		// Parse latitude: N3702.1 -> 37.035
		latDir := m[1]
		latDeg, _ := strconv.ParseFloat(m[2], 64)
		latMin, _ := strconv.ParseFloat(m[3], 64)
		pos.Latitude = latDeg + latMin/60.0
		if latDir == "S" {
			pos.Latitude = -pos.Latitude
		}

		// Parse longitude: W09921.8 -> -99.363
		lonDir := m[4]
		lonDeg, _ := strconv.ParseFloat(m[5], 64)
		lonMin, _ := strconv.ParseFloat(m[6], 64)
		pos.Longitude = lonDeg + lonMin/60.0
		if lonDir == "W" {
			pos.Longitude = -pos.Longitude
		}

		pos.Time = m[7]
		pos.Altitude, _ = strconv.Atoi(m[8])
		pos.Temperature, _ = strconv.ParseFloat(m[9], 64)
		pos.Heading, _ = strconv.Atoi(m[10])
		pos.Speed, _ = strconv.Atoi(m[11])
		pos.Phase = m[12]

		result.Positions = append(result.Positions, pos)
	}

	if len(result.Positions) == 0 {
		return nil
	}

	return result
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *TrajectoryParser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	quickCheckPassed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{
		Passed: quickCheckPassed,
	}

	if !quickCheckPassed {
		trace.QuickCheck.Reason = "No ++86501 or ++76502 prefix found"
		return trace
	}

	text := msg.Text

	// Add extractor for header pattern.
	headerMatch := trajectoryHeaderRe.FindStringSubmatch(text)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "header",
		Pattern: trajectoryHeaderRe.String(),
		Matched: headerMatch != nil,
		Value: func() string {
			if headerMatch != nil && len(headerMatch) > 1 {
				return headerMatch[1] + " / " + headerMatch[2]
			}
			return ""
		}(),
	})

	// Add extractor for position entries.
	posMatches := positionRe.FindAllStringSubmatch(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "positions",
		Pattern: positionRe.String(),
		Matched: len(posMatches) > 0,
		Value:   strconv.Itoa(len(posMatches)) + " found",
	})

	trace.Matched = headerMatch != nil && len(posMatches) > 0
	return trace
}