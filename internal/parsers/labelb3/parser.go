// Package labelb3 parses Label B3 gate info messages.
package labelb3

import (
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents gate info from label B3 messages.
type Result struct {
	MsgID        int64  `json:"message_id"`
	Timestamp    string `json:"timestamp"`
	Tail         string `json:"tail,omitempty"`
	FlightNum    string `json:"flight_num,omitempty"`
	Origin       string `json:"origin,omitempty"`
	Destination  string `json:"destination,omitempty"`
	Gate         string `json:"gate,omitempty"`
	ATIS         string `json:"atis,omitempty"`
	AircraftType string `json:"aircraft_type,omitempty"`
}

func (r *Result) Type() string     { return "gate_info" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label B3 gate info messages.
type Parser struct{}

// Grok compiler singleton.
var (
	grokCompiler *patterns.Compiler
	grokOnce     sync.Once
	grokErr      error
)

// getCompiler returns the singleton grok compiler.
func getCompiler() (*patterns.Compiler, error) {
	grokOnce.Do(func() {
		grokCompiler = patterns.NewCompiler(Formats, nil)
		grokErr = grokCompiler.Compile()
	})
	return grokCompiler, grokErr
}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "labelb3" }
func (p *Parser) Labels() []string { return []string{"B3"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return true // Label check is sufficient for B3.
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := msg.Text

	// Try grok-based parsing.
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	// Parse all formats to extract different fields.
	matches := compiler.ParseAll(text)
	for _, match := range matches {
		switch match.FormatName {
		case "gate_info":
			result.FlightNum = match.Captures["flight"]
			result.Origin = match.Captures["origin"]
			result.Gate = match.Captures["gate"]
			result.Destination = match.Captures["dest"]
		case "atis":
			result.ATIS = match.Captures["atis"]
		case "aircraft_type":
			result.AircraftType = match.Captures["aircraft"]
		}
	}

	// Only return if we got something useful.
	if result.FlightNum == "" && result.Gate == "" && result.ATIS == "" {
		return nil
	}

	return result
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	// QuickCheck always passes for B3.
	trace.QuickCheck = &registry.QuickCheck{
		Passed: true,
		Reason: "Label check sufficient for B3",
	}

	compiler, err := getCompiler()
	if err != nil {
		trace.QuickCheck.Reason = "Failed to get compiler: " + err.Error()
		return trace
	}

	// Get detailed trace from compiler.
	compilerTrace := compiler.ParseWithTrace(msg.Text)

	for _, ft := range compilerTrace.Formats {
		trace.Formats = append(trace.Formats, registry.FormatTrace{
			Name:     ft.Name,
			Matched:  ft.Matched,
			Pattern:  ft.Pattern,
			Captures: ft.Captures,
		})
	}

	trace.Matched = compilerTrace.Match != nil
	return trace
}
