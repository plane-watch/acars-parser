// Package parking parses parking and gate information messages.
package parking

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Result represents a parsed parking/gate info message.
type Result struct {
	MsgID          int64  `json:"message_id,omitempty"`
	Airport        string `json:"airport,omitempty"`         // IATA airport code
	ParkingStand   string `json:"parking_stand,omitempty"`   // Predicted parking stand
	BaggageCarousel string `json:"baggage_carousel,omitempty"` // Carousel number/code
}

func (r *Result) Type() string     { return "parking_info" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// ESCALE  CDG
	airportRe = regexp.MustCompile(`ESCALE\s+([A-Z]{3})`)

	// PARKING PREVIS. A L ARRIVEE : K64
	parkingRe = regexp.MustCompile(`PARKING[^:]+:\s*([A-Z0-9]+)`)

	// TAPIS A BAGAGES :  330E69
	baggageRe = regexp.MustCompile(`TAPIS A BAGAGES\s*:\s*([A-Z0-9]+)`)
)

// Parser parses parking information messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string           { return "parking_info" }
func (p *Parser) Labels() []string       { return []string{"1E", "RA"} }
func (p *Parser) Priority() int          { return 50 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "PKG INFO MSG")
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

	// Parse airport.
	if m := airportRe.FindStringSubmatch(text); m != nil {
		result.Airport = m[1]
	}

	// Parse parking stand.
	if m := parkingRe.FindStringSubmatch(text); m != nil {
		result.ParkingStand = m[1]
	}

	// Parse baggage carousel.
	if m := baggageRe.FindStringSubmatch(text); m != nil {
		result.BaggageCarousel = m[1]
	}

	// Must have at least some data.
	if result.Airport == "" && result.ParkingStand == "" {
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
		trace.QuickCheck.Reason = "No PKG INFO MSG keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for key patterns.
	extractors := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"airport", airportRe},
		{"parking", parkingRe},
		{"baggage", baggageRe},
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

	hasAirport := airportRe.MatchString(text)
	hasParking := parkingRe.MatchString(text)
	trace.Matched = hasAirport || hasParking
	return trace
}