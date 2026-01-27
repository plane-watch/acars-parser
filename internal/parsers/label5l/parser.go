// Package label5l parses Label 5L route messages.
package label5l

import (
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents a parsed route from label 5L messages.
type Result struct {
	MsgID      int64  `json:"message_id"`
	Timestamp  string `json:"timestamp"`
	Callsign   string `json:"callsign"`
	Tail       string `json:"tail,omitempty"`
	OriginIATA string `json:"origin_iata,omitempty"`
	OriginICAO string `json:"origin_icao"`
	DestIATA   string `json:"dest_iata,omitempty"`
	DestICAO   string `json:"dest_icao"`
	FlightID   string `json:"flight_id,omitempty"`
	Date       string `json:"date,omitempty"`
	DepSched   string `json:"dep_sched,omitempty"`
	DepActual  string `json:"dep_actual,omitempty"`
	ArrSched   string `json:"arr_sched,omitempty"`
	ArrActual  string `json:"arr_actual,omitempty"`
}

func (r *Result) Type() string     { return "route" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 5L route messages.
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

func (p *Parser) Name() string     { return "label5l" }
func (p *Parser) Labels() []string { return []string{"5L"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	// 5L messages have comma-delimited format.
	return strings.Contains(text, ",")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}

	// First line contains the route info.
	firstLine := strings.TrimSpace(lines[0])

	// Try grok-based parsing.
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(firstLine)
	if match == nil || match.FormatName != "route" {
		return nil
	}

	result := &Result{
		MsgID:      int64(msg.ID),
		Timestamp:  msg.Timestamp,
		Callsign:   match.Captures["callsign"],
		Tail:       match.Captures["tail"],
		OriginIATA: strings.TrimSpace(match.Captures["origin_iata"]),
		OriginICAO: strings.TrimSpace(match.Captures["origin_icao"]),
		DestICAO:   strings.TrimSpace(match.Captures["dest_icao"]),
		FlightID:   strings.TrimSpace(match.Captures["flight_id"]),
		Date:       strings.TrimSpace(match.Captures["date"]),
		DepSched:   strings.TrimSpace(match.Captures["dep_sched"]),
		DepActual:  strings.TrimSpace(match.Captures["dep_actual"]),
		ArrSched:   strings.TrimSpace(match.Captures["arr_sched"]),
		ArrActual:  strings.TrimSpace(match.Captures["arr_actual"]),
	}

	// Handle destination IATA (may be "---").
	destIATA := strings.TrimSpace(match.Captures["dest_iata"])
	if destIATA != "---" {
		result.DestIATA = destIATA
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
		trace.QuickCheck.Reason = "No comma found in message"
		return trace
	}

	compiler, err := getCompiler()
	if err != nil {
		trace.QuickCheck.Reason = "Failed to get compiler: " + err.Error()
		return trace
	}

	// Parse first line only (as the parser does).
	text := strings.TrimSpace(msg.Text)
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return trace
	}
	firstLine := strings.TrimSpace(lines[0])

	compilerTrace := compiler.ParseWithTrace(firstLine)

	for _, ft := range compilerTrace.Formats {
		trace.Formats = append(trace.Formats, registry.FormatTrace{
			Name:     ft.Name,
			Matched:  ft.Matched,
			Pattern:  ft.Pattern,
			Captures: ft.Captures,
		})
	}

	trace.Matched = compilerTrace.Match != nil && compilerTrace.Match.FormatName == "route"
	return trace
}
