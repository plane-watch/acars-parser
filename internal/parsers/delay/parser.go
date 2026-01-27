// Package delay parses delay summary messages.
package delay

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// DelayCode represents an IATA delay code with duration.
type DelayCode struct {
	Code    string `json:"code"`
	Minutes int    `json:"minutes"`
}

// Result represents a parsed delay summary.
type Result struct {
	MsgID           int64       `json:"message_id,omitempty"`
	FlightNumber    string      `json:"flight_number,omitempty"`
	FlightDate      string      `json:"flight_date,omitempty"`
	Origin          string      `json:"origin,omitempty"`
	Destination     string      `json:"destination,omitempty"`
	STD             string      `json:"std,omitempty"`              // Scheduled Time of Departure
	ATD             string      `json:"atd,omitempty"`              // Actual Time of Departure
	DepDelayMinutes int         `json:"dep_delay_minutes"`
	STA             string      `json:"sta,omitempty"`              // Scheduled Time of Arrival
	ATA             string      `json:"ata,omitempty"`              // Actual Time of Arrival
	ArrDelayMinutes int         `json:"arr_delay_minutes"`
	DelayCodes      []DelayCode `json:"delay_codes,omitempty"`
	MessageCreated  string      `json:"message_created,omitempty"`
}

func (r *Result) Type() string     { return "delay_summary" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// AY540/07JAN2026 RVN-HEL or AY1366/03JAN2026 MAN-HEL
	flightRouteRe = regexp.MustCompile(`([A-Z]{2}\d+)/(\d{2}[A-Z]{3}\d{4})\s+([A-Z]{3})-([A-Z]{3})`)

	// STD 03:20 or STD 18:10
	stdRe = regexp.MustCompile(`STD\s+(\d{2}:\d{2})`)

	// ATD 03:14 or ATD 18:23
	atdRe = regexp.MustCompile(`ATD\s+(\d{2}:\d{2})`)

	// DEP DELAY 5 MIN or DEP DELAY 0 MIN
	depDelayRe = regexp.MustCompile(`DEP DELAY\s+(\d+)\s*MIN`)

	// STA 04:40
	staRe = regexp.MustCompile(`STA\s+(\d{2}:\d{2})`)

	// ATA 04:39
	ataRe = regexp.MustCompile(`ATA\s+(\d{2}:\d{2})`)

	// ARR DELAY 5 MIN
	arrDelayRe = regexp.MustCompile(`ARR DELAY\s+(\d+)\s*MIN`)

	// DL89/5MIN or DL93/11MIN DL81/2MIN
	delayCodeRe = regexp.MustCompile(`DL(\d+)/(\d+)MIN`)

	// MESSAGE CREATED: 2026-01-07T04:39:40Z
	createdRe = regexp.MustCompile(`MESSAGE CREATED:\s*(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z?)`)
)

// Parser parses delay summary messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string           { return "delay_summary" }
func (p *Parser) Labels() []string       { return []string{"3E", "RA"} }
func (p *Parser) Priority() int          { return 50 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "DELAY SUMMARY")
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
		MsgID: int64(msg.ID),
	}

	// Parse flight/route.
	if m := flightRouteRe.FindStringSubmatch(text); m != nil {
		result.FlightNumber = m[1]
		result.FlightDate = m[2]
		result.Origin = m[3]
		result.Destination = m[4]
	}

	// Parse times.
	if m := stdRe.FindStringSubmatch(text); m != nil {
		result.STD = m[1]
	}
	if m := atdRe.FindStringSubmatch(text); m != nil {
		result.ATD = m[1]
	}
	if m := staRe.FindStringSubmatch(text); m != nil {
		result.STA = m[1]
	}
	if m := ataRe.FindStringSubmatch(text); m != nil {
		result.ATA = m[1]
	}

	// Parse delays.
	if m := depDelayRe.FindStringSubmatch(text); m != nil {
		result.DepDelayMinutes, _ = strconv.Atoi(m[1])
	}
	if m := arrDelayRe.FindStringSubmatch(text); m != nil {
		result.ArrDelayMinutes, _ = strconv.Atoi(m[1])
	}

	// Parse delay codes.
	codeMatches := delayCodeRe.FindAllStringSubmatch(text, -1)
	for _, m := range codeMatches {
		minutes, _ := strconv.Atoi(m[2])
		result.DelayCodes = append(result.DelayCodes, DelayCode{
			Code:    "DL" + m[1],
			Minutes: minutes,
		})
	}

	// Parse message created timestamp.
	if m := createdRe.FindStringSubmatch(text); m != nil {
		result.MessageCreated = m[1]
	}

	// Must have at least flight info or delay data.
	if result.FlightNumber == "" && result.DepDelayMinutes == 0 && result.ArrDelayMinutes == 0 {
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
		trace.QuickCheck.Reason = "No DELAY SUMMARY keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for key patterns.
	extractors := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"flight_route", flightRouteRe},
		{"std", stdRe},
		{"atd", atdRe},
		{"dep_delay", depDelayRe},
		{"sta", staRe},
		{"ata", ataRe},
		{"arr_delay", arrDelayRe},
		{"delay_code", delayCodeRe},
		{"created", createdRe},
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

	hasFlightNum := flightRouteRe.MatchString(text)
	hasDelay := depDelayRe.MatchString(text) || arrDelayRe.MatchString(text)
	trace.Matched = hasFlightNum || hasDelay
	return trace
}