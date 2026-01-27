// Package labelb2 parses Label B2 oceanic clearance messages.
package labelb2

import (
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents an oceanic clearance from label B2 messages.
type Result struct {
	MsgID       int64    `json:"message_id"`
	Timestamp   string   `json:"timestamp"`
	Tail        string   `json:"tail,omitempty"`
	FlightNum   string   `json:"flight_num,omitempty"`
	Destination string   `json:"destination,omitempty"`
	Route       []string `json:"route,omitempty"`
	OceanicFix  []string `json:"oceanic_fixes,omitempty"`
	FlightLevel string   `json:"flight_level,omitempty"`
	Mach        string   `json:"mach,omitempty"`
}

func (r *Result) Type() string     { return "oceanic_clearance" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label B2 oceanic clearance messages.
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

func (p *Parser) Name() string     { return "labelb2" }
func (p *Parser) Labels() []string { return []string{"B2"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return true // Label check is sufficient for B2.
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
		case "oceanic_dest":
			result.Destination = match.Captures["dest"]
		case "flight_level":
			result.FlightLevel = "FL" + match.Captures["fl"]
		case "mach":
			result.Mach = "M" + match.Captures["mach"]
		case "flight_num":
			result.FlightNum = match.Captures["flight"]
		}
	}

	// Extract all oceanic fixes (can match multiple times).
	fixes := compiler.FindAllMatches(text, "oceanic_fix")
	for _, fix := range fixes {
		if f := fix["fix"]; f != "" {
			result.OceanicFix = append(result.OceanicFix, f)
		}
	}

	// Fallback: try to find flight number from first line if not found.
	if result.FlightNum == "" {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Simple check for flight number at start of line.
			if len(line) >= 4 {
				// Check if starts with 3 letters followed by digits.
				if isAlpha(line[0]) && isAlpha(line[1]) && isAlpha(line[2]) && isDigit(line[3]) {
					// Find end of flight number.
					end := 3
					for end < len(line) && (isDigit(line[end]) || isAlpha(line[end])) {
						end++
					}
					result.FlightNum = line[:end]
					break
				}
			}
		}
	}

	// Only return if we found something useful.
	if result.Destination == "" && len(result.OceanicFix) == 0 {
		return nil
	}

	return result
}

func isAlpha(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	// QuickCheck always passes for B2.
	trace.QuickCheck = &registry.QuickCheck{
		Passed: true,
		Reason: "Label check sufficient for B2",
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

	// B2 matches if we found destination or oceanic fixes.
	hasDestination := false
	for _, ft := range compilerTrace.Formats {
		if ft.Matched && ft.Name == "oceanic_dest" {
			hasDestination = true
			break
		}
	}
	fixes := compiler.FindAllMatches(msg.Text, "oceanic_fix")
	trace.Matched = hasDestination || len(fixes) > 0

	return trace
}
