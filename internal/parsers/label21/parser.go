// Package label21 parses Label 21 position report messages.
package label21

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents a position report from label 21 messages.
type Result struct {
	MsgID       int64   `json:"message_id"`
	Timestamp   string  `json:"timestamp"`
	Tail        string  `json:"tail,omitempty"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Heading     int     `json:"heading,omitempty"`
	Altitude    int     `json:"altitude,omitempty"`
	FuelOnBoard int     `json:"fuel_on_board,omitempty"`
	Temperature string  `json:"temperature,omitempty"`
	Wind        string  `json:"wind,omitempty"`
	ETA         string  `json:"eta,omitempty"`
	Destination string  `json:"destination,omitempty"`
}

func (r *Result) Type() string     { return "position_report" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 21 position report messages.
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

func (p *Parser) Name() string     { return "label21" }
func (p *Parser) Labels() []string { return []string{"21"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "POSN")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	// Try grok-based parsing.
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(msg.Text)
	if match == nil || match.FormatName != "posn_report" {
		return nil
	}

	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Tail:        msg.Tail,
		Latitude:    patterns.ParseDecimalCoord(match.Captures["lat"], "N"),
		Longitude:   patterns.ParseDecimalCoord(strings.TrimSpace(match.Captures["lon"]), match.Captures["lon_dir"]),
		Wind:        strings.TrimSpace(match.Captures["wind"]),
		Temperature: strings.TrimSpace(match.Captures["temp"]),
		ETA:         match.Captures["eta"],
		Destination: match.Captures["dest"],
	}

	// Parse heading.
	if hdg, err := strconv.Atoi(match.Captures["heading"]); err == nil {
		result.Heading = hdg
	}

	// Parse altitude.
	if alt, err := strconv.Atoi(match.Captures["altitude"]); err == nil {
		result.Altitude = alt
	}

	// Parse fuel on board.
	if fob, err := strconv.Atoi(match.Captures["fob"]); err == nil {
		result.FuelOnBoard = fob
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
		trace.QuickCheck.Reason = "No POSN keyword found"
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

	trace.Matched = compilerTrace.Match != nil && compilerTrace.Match.FormatName == "posn_report"
	return trace
}
