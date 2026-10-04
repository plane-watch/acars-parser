// Package h1 contains parsers for H1 label messages.
package h1

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// TrajectoryResult represents a parsed Southwest Airlines flight data
// report (737 NG and 737 MAX): the aircraft's type, the flight and route,
// and a series of samples.
//
// The meanings of the sample fields were established from the January 2026
// corpus: the temperature falls by 1.8 °C per 1,000 ft (the standard
// atmosphere gives 2.0); the wind speed rises with altitude (median 17 kt
// below 10,000 ft, 82 kt above 30,000 ft) and the wind direction is 84%
// westerly; the phase codes match the altitude (TO and IC near the ground,
// CR at cruise); and the time is DDHHMM, since several samples ~1.8 nm apart
// share one value, and samples a minute apart are ~7.7 nm apart. The
// header's eighth field ("0196") is not captured: its meaning is not
// established (it is not the route distance).
type TrajectoryResult struct {
	MsgID int64 `json:"message_id,omitempty"`

	// Registration is reported only when the header's registration field
	// is the transmitted tail; the field also holds fleet numbers ("201"),
	// "XXX" and truncated registrations (" N8852" for N8852Q).
	Registration string     `json:"registration,omitempty"`
	AircraftType string     `json:"aircraft_type"` // As transmitted, e.g. B7378MAX.
	Date         string     `json:"date"`          // YYMMDD format
	FlightNumber string     `json:"flight_number,omitempty"`
	Origin       string     `json:"origin,omitempty"`
	Destination  string     `json:"destination,omitempty"`
	SystemID     string     `json:"system_id,omitempty"` // e.g. SMX34-2502-F320.
	Positions    []Position `json:"positions"`
}

// Position represents a single sample in a trajectory.
type Position struct {
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Time          string  `json:"time"`                     // DDHHMM: day of the month, hour and minute.
	Altitude      int     `json:"altitude"`                 // Feet
	Temperature   float64 `json:"temperature,omitempty"`    // Outside air temperature, Celsius.
	WindDirection int     `json:"wind_direction,omitempty"` // Degrees
	WindSpeed     int     `json:"wind_speed,omitempty"`     // Knots
	Phase         string  `json:"phase,omitempty"`          // Flight phase: TO, IC, CL, CR, ER, DC, AP
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

	if !patterns.IsValidICAO(headerMatch[5]) || !patterns.IsValidICAO(headerMatch[6]) {
		return nil
	}
	if reg := strings.TrimSpace(headerMatch[1]); isTransmittedTail(reg, msg.Tail) {
		result.Registration = reg
	}
	result.AircraftType = headerMatch[2]
	result.Date = headerMatch[3]
	result.FlightNumber = headerMatch[4]
	result.Origin = headerMatch[5]
	result.Destination = headerMatch[6]
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
		pos.WindDirection, _ = strconv.Atoi(m[10])
		pos.WindSpeed, _ = strconv.Atoi(m[11])
		pos.Phase = m[12]

		result.Positions = append(result.Positions, pos)
	}

	if len(result.Positions) == 0 {
		return nil
	}

	return result
}

// isTransmittedTail reports whether a registration field is the transmitted
// tail (see acars.NormaliseRegistration). An empty tail matches nothing.
func isTransmittedTail(reg, tail string) bool {
	t := acars.NormaliseRegistration(tail)
	return t != "" && acars.NormaliseRegistration(reg) == t
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
			if len(headerMatch) > 1 {
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
