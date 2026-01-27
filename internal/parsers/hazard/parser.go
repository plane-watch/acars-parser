// Package hazard parses ARINC Direct HAZARD ALERT messages.
package hazard

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// HazardResult represents a parsed HAZARD ALERT message.
type HazardResult struct {
	MsgID       int64   `json:"message_id,omitempty"`
	Timestamp   string  `json:"timestamp"`              // e.g., "03-JAN-26 0500Z"
	From        string  `json:"from"`                   // e.g., "ARINCDIRECT"
	To          string  `json:"to"`                     // Aircraft address e.g., "9HMOON"
	Callsign    string  `json:"callsign,omitempty"`     // e.g., "KFE300"
	FlightID    string  `json:"flight_id,omitempty"`    // e.g., "H7104"
	Origin      string  `json:"origin,omitempty"`       // ICAO code
	Destination string  `json:"destination,omitempty"`  // ICAO code
	ETD         string  `json:"etd,omitempty"`          // Estimated departure time
	Segment     string  `json:"segment,omitempty"`      // e.g., "TIDKA-LSGG"
	ETO         string  `json:"eto,omitempty"`          // Estimated time over segment
	EDR         float64 `json:"edr,omitempty"`          // EDR turbulence value
	WindWarning string  `json:"wind_warning,omitempty"` // Wind warning text
	AlertLevel  string  `json:"alert_level,omitempty"`  // CRITICAL, WARNING, etc.
}

func (r *HazardResult) Type() string     { return "hazard_alert" }
func (r *HazardResult) MessageID() int64 { return r.MsgID }

var (
	// MSG/RX03-JAN-26 0500Z FR:ARINCDIRECT TO:9HMOON
	hazardHeaderRe = regexp.MustCompile(`MSG/RX(\d{2}-[A-Z]{3}-\d{2}\s+\d{4}Z)\s+FR:([A-Z0-9]+)\s+TO:([A-Z0-9]+)`)

	// HAZARD ALERT FOR KFE300 H7104: LMML-LSGG
	hazardAlertRe = regexp.MustCompile(`HAZARD ALERT FOR\s+([A-Z0-9]+)\s+([A-Z0-9]+):\s*([A-Z]{4})-([A-Z]{4})`)

	// (0520Z ETD)
	etdRe = regexp.MustCompile(`\((\d{4}Z)\s+ETD\)`)

	// EDR TURBULENCE IS 0.5 EDR
	edrRe = regexp.MustCompile(`EDR TURBULENCE IS\s+([\d.]+)\s+EDR`)

	// SEGMENT: TIDKA-LSGG ETO: 0637Z
	segmentRe = regexp.MustCompile(`SEGMENT:\s*([A-Z0-9-]+)\s+ETO:\s*(\d{4}Z)`)

	// WARNING: WINDS IS 20
	windRe = regexp.MustCompile(`WARNING:\s*WINDS\s+IS\s+(\d+)`)
)

// Parser parses HAZARD ALERT messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string           { return "hazard_alert" }
func (p *Parser) Labels() []string       { return []string{"_", "H1", "SA"} } // Various labels possible
func (p *Parser) Priority() int          { return 60 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "HAZARD ALERT")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := msg.Text

	// Must have HAZARD ALERT.
	if !strings.Contains(text, "HAZARD ALERT") {
		return nil
	}

	result := &HazardResult{
		MsgID: int64(msg.ID),
	}

	// Parse header.
	if m := hazardHeaderRe.FindStringSubmatch(text); m != nil {
		result.Timestamp = m[1]
		result.From = m[2]
		result.To = m[3]
	}

	// Parse alert details.
	if m := hazardAlertRe.FindStringSubmatch(text); m != nil {
		result.Callsign = m[1]
		result.FlightID = m[2]
		result.Origin = m[3]
		result.Destination = m[4]
	}

	// Parse ETD.
	if m := etdRe.FindStringSubmatch(text); m != nil {
		result.ETD = m[1]
	}

	// Parse EDR turbulence.
	if m := edrRe.FindStringSubmatch(text); m != nil {
		result.EDR, _ = strconv.ParseFloat(m[1], 64)
		result.AlertLevel = "CRITICAL"
	}

	// Parse segment.
	if m := segmentRe.FindStringSubmatch(text); m != nil {
		result.Segment = m[1]
		result.ETO = m[2]
	}

	// Parse wind warning.
	if m := windRe.FindStringSubmatch(text); m != nil {
		result.WindWarning = m[1] + " knots"
		if result.AlertLevel == "" {
			result.AlertLevel = "WARNING"
		}
	}

	// Only return if we parsed something meaningful.
	if result.Callsign == "" && result.EDR == 0 {
		return nil
	}

	return result
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	quickCheckPassed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{
		Passed: quickCheckPassed,
	}

	if !quickCheckPassed {
		trace.QuickCheck.Reason = "No HAZARD ALERT keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for each regex pattern.
	extractors := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"header", hazardHeaderRe},
		{"alert", hazardAlertRe},
		{"etd", etdRe},
		{"edr", edrRe},
		{"segment", segmentRe},
		{"wind", windRe},
	}

	for _, e := range extractors {
		ext := registry.Extractor{
			Name:    e.name,
			Pattern: e.pattern.String(),
		}
		if m := e.pattern.FindStringSubmatch(text); len(m) > 1 {
			ext.Matched = true
			ext.Value = m[1]
		}
		trace.Extractors = append(trace.Extractors, ext)
	}

	// Determine if overall match succeeds.
	hasCallsign := hazardAlertRe.MatchString(text)
	hasEDR := edrRe.MatchString(text)
	trace.Matched = hasCallsign || hasEDR

	return trace
}