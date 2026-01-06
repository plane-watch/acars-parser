// Package gateassign parses gate assignment messages from ACARS.
package gateassign

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Result represents parsed gate assignment data.
type Result struct {
	MsgID      int64  `json:"message_id"`
	Timestamp  string `json:"timestamp"`
	Tail       string `json:"tail,omitempty"`
	Gate       string `json:"gate,omitempty"`
	PPOS       string `json:"ppos,omitempty"`       // Parking position.
	BagBelt    string `json:"bag_belt,omitempty"`   // Baggage belt.
	NextFlight string `json:"next_flight,omitempty"`
	NextRoute  string `json:"next_route,omitempty"`
}

func (r *Result) Type() string     { return "gate_assignment" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses gate assignment messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "gateassign" }
func (p *Parser) Labels() []string { return []string{"RA"} }
func (p *Parser) Priority() int    { return 60 }

// QuickCheck looks for gate assignment keywords.
func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)
	return strings.Contains(upper, "GATE ASSIGNMENT")
}

// Pattern matchers.
var (
	// Simple gate: GATE ASSIGNMENT: A12 or GATE ASSIGNMENT: G5
	gateRe = regexp.MustCompile(`GATE\s+ASSIGNMENT[:\s]+([A-Z]?\d+)`)

	// IN-RANGE gate format: IWA GATE 6 ASSIGNED
	inRangeGateRe = regexp.MustCompile(`([A-Z]{3})\s+GATE\s+(\d+)\s+ASSIGNED`)

	// Parking position: PPOS:305 or PPOS:R3 (but not N/A).
	pposRe = regexp.MustCompile(`PPOS[:\s]+([A-Z]?\d+)`)

	// Bag belt: BAG BELT:206
	bagBeltRe = regexp.MustCompile(`BAG\s+BELT[:\s]+(\d+)`)

	// Next leg format: NEXT LEG: LA3709 BPS-BSB 31DEC 19:45Z
	nextLegRe = regexp.MustCompile(`NEXT\s+LEG[:\s]+(\w+)\s+([A-Z]{3}-[A-Z]{3})`)

	// IN-RANGE next flight: --NEXT FLIGHT           278 BLI-PSP 31-2002Z
	nextFlightRe = regexp.MustCompile(`NEXT\s+FLIGHT\s+(\d+)\s+([A-Z]{3}-[A-Z]{3})`)
)

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	text := msg.Text

	// Extract gate (try simple format first, then IN-RANGE format).
	if m := gateRe.FindStringSubmatch(text); len(m) > 1 {
		result.Gate = m[1]
	} else if m := inRangeGateRe.FindStringSubmatch(text); len(m) > 2 {
		// IN-RANGE format includes station code: IWA GATE 6 ASSIGNED
		result.Gate = m[2]
	}

	// Extract parking position.
	if m := pposRe.FindStringSubmatch(text); len(m) > 1 {
		result.PPOS = m[1]
	}

	// Extract bag belt.
	if m := bagBeltRe.FindStringSubmatch(text); len(m) > 1 {
		result.BagBelt = m[1]
	}

	// Extract next leg (try both formats).
	if m := nextLegRe.FindStringSubmatch(text); len(m) > 2 {
		result.NextFlight = m[1]
		result.NextRoute = m[2]
	} else if m := nextFlightRe.FindStringSubmatch(text); len(m) > 2 {
		result.NextFlight = m[1]
		result.NextRoute = m[2]
	}

	// Only return if we got useful data.
	if result.Gate == "" && result.PPOS == "" && result.NextFlight == "" {
		return nil
	}

	return result
}