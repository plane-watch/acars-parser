// Package gateassign parses gate assignment messages from ACARS.
package gateassign

import (
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
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
	return strings.Contains(upper, "GATE ASSIGNMENT") || strings.Contains(upper, "GATE") && strings.Contains(upper, "ASSIGNED")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(msg.Text)
	if match == nil {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	// Extract fields based on which format matched.
	switch match.FormatName {
	case "simple_gate":
		result.Gate = match.Captures["gate"]

	case "in_range_gate":
		result.Gate = match.Captures["gate"]

	case "structured_gate":
		ppos := strings.TrimSpace(match.Captures["ppos"])
		if ppos != "" && ppos != "N/A" {
			result.PPOS = ppos
		}

		bagbelt := strings.TrimSpace(match.Captures["bagbelt"])
		if bagbelt != "" {
			result.BagBelt = bagbelt
		}

		if match.Captures["next_flight"] != "" {
			result.NextFlight = match.Captures["next_flight"]
			result.NextRoute = match.Captures["next_route"]
		}
	}

	// Only return if we got useful data.
	if result.Gate == "" && result.PPOS == "" && result.NextFlight == "" {
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
		trace.QuickCheck.Reason = "No GATE ASSIGNMENT or GATE...ASSIGNED keyword found"
		return trace
	}

	// Get the compiler trace.
	compiler, err := getCompiler()
	if err != nil {
		trace.QuickCheck.Reason = "Failed to get compiler: " + err.Error()
		return trace
	}

	// Get detailed trace from compiler.
	compilerTrace := compiler.ParseWithTrace(msg.Text)

	// Convert format traces to generic format traces.
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