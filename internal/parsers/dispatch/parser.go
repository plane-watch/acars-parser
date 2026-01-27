// Package dispatch parses dispatcher messages from operations.
package dispatch

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Result represents a parsed dispatcher message.
type Result struct {
	MsgID        int64  `json:"message_id,omitempty"`
	FlightNumber string `json:"flight_number,omitempty"`
	Tail         string `json:"tail,omitempty"`
	AckRequired  bool   `json:"ack_required"`
	Category     string `json:"category,omitempty"`     // MEL, SIGMET, FUEL, AMEND, etc.
	MELRef       string `json:"mel_ref,omitempty"`      // MEL/CDL/SDL reference
	MDDRNumber   string `json:"mddr_number,omitempty"`  // Maintenance deferral number
	DispatcherID string `json:"dispatcher_id,omitempty"`
	Timestamp    string `json:"timestamp,omitempty"`
	Content      string `json:"content,omitempty"`      // Main message content
}

func (r *Result) Type() string     { return "dispatcher" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// Flight/tail: ASA849 N381HA or FLT: 991 ACFT: 391
	flightTailRe1 = regexp.MustCompile(`([A-Z]{2,3}\d+)\s+([A-Z0-9-]+)\s*\n`)
	flightTailRe2 = regexp.MustCompile(`FLT:\s*(\d+)\s*\n\s*ACFT:\s*(\d+)`)

	// MEL/CDL/SDL REF: 74-31-1A
	melRefRe = regexp.MustCompile(`MEL,?\s*CDL,?\s*SDL\s*REF:\s*([A-Z0-9-]+)`)

	// MDDR #: = 545476
	mddrRe = regexp.MustCompile(`MDDR\s*#?:?\s*=?\s*(\d+)`)

	// DISP/KS 18443/2303Z
	dispatcherRe = regexp.MustCompile(`DISP/([A-Z]{2})\s+(\d+)/(\d{4}Z)`)

	// SIGMET detection
	sigmetRe = regexp.MustCompile(`SIGMET\s+([A-Z]\d+)`)
)

// Parser parses dispatcher messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string           { return "dispatcher" }
func (p *Parser) Labels() []string       { return []string{"RA", "25", "H1"} }
func (p *Parser) Priority() int          { return 45 } // Lower than more specific parsers

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "DISPATCHER MSG")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := msg.Text

	if !p.QuickCheck(text) {
		return nil
	}

	result := &Result{
		MsgID:       int64(msg.ID),
		AckRequired: strings.Contains(text, "PLEASE ACK"),
	}

	// Parse flight/tail - format 1: ASA849 N381HA
	if m := flightTailRe1.FindStringSubmatch(text); m != nil {
		result.FlightNumber = m[1]
		result.Tail = m[2]
	}

	// Parse flight/tail - format 2: FLT: 991 ACFT: 391
	if m := flightTailRe2.FindStringSubmatch(text); m != nil {
		result.FlightNumber = m[1]
		result.Tail = m[2]
	}

	// Parse MEL reference.
	if m := melRefRe.FindStringSubmatch(text); m != nil {
		result.MELRef = m[1]
		result.Category = "MEL"
	}

	// Parse MDDR number.
	if m := mddrRe.FindStringSubmatch(text); m != nil {
		result.MDDRNumber = m[1]
	}

	// Parse dispatcher ID.
	if m := dispatcherRe.FindStringSubmatch(text); m != nil {
		result.DispatcherID = m[1]
		result.Timestamp = m[3]
	}

	// Detect SIGMET content.
	if m := sigmetRe.FindStringSubmatch(text); m != nil {
		result.Category = "SIGMET"
	}

	// Detect fuel amendments.
	if strings.Contains(text, "FUEL") && (strings.Contains(text, "MRF") || strings.Contains(text, "MIN TO FUEL")) {
		result.Category = "FUEL"
	}

	// Detect flight plan amendments.
	if strings.Contains(text, "AMND FLT PLAN") || strings.Contains(text, "AMEND") {
		if result.Category == "" {
			result.Category = "AMEND"
		}
	}

	// Extract content after "DISPATCHER MSG" header.
	idx := strings.Index(text, "DISPATCHER MSG")
	if idx >= 0 {
		content := strings.TrimSpace(text[idx+14:])
		// Limit content length for storage.
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		result.Content = content
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
		trace.QuickCheck.Reason = "No DISPATCHER MSG keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for key patterns.
	extractors := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"flight_tail_1", flightTailRe1},
		{"flight_tail_2", flightTailRe2},
		{"mel_ref", melRefRe},
		{"mddr", mddrRe},
		{"dispatcher", dispatcherRe},
		{"sigmet", sigmetRe},
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

	trace.Matched = true // If QuickCheck passed, the message is valid
	return trace
}