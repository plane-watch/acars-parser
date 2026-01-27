// Package eta parses ETA/timing messages (Label 5Z).
package eta

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Grok compiler singleton.
var (
	grokCompiler *patterns.Compiler
	grokOnce     sync.Once
	grokErr      error
)

func getCompiler() (*patterns.Compiler, error) {
	grokOnce.Do(func() {
		grokCompiler = patterns.NewCompiler(Formats, nil)
		grokErr = grokCompiler.Compile()
	})
	return grokCompiler, grokErr
}

// Result represents a parsed ETA message.
type Result struct {
	MsgID       int64  `json:"message_id"`
	Timestamp   string `json:"timestamp"`
	Tail        string `json:"tail,omitempty"`
	MessageType string `json:"message_type"` // ET, IR, B6, OS, C3
	Origin      string `json:"origin,omitempty"`
	Destination string `json:"destination,omitempty"`
	DayOfMonth  int    `json:"day_of_month,omitempty"`
	ReportTime  string `json:"report_time,omitempty"`
	ETA         string `json:"eta,omitempty"`
	Mode        string `json:"mode,omitempty"` // AUTO, etc.
	Runway      string `json:"runway,omitempty"`
	Gate        string `json:"gate,omitempty"`
	RawData     string `json:"raw_data,omitempty"`
}

func (r *Result) Type() string     { return "eta" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses ETA/timing messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "eta" }
func (p *Parser) Labels() []string { return []string{"5Z"} }
func (p *Parser) Priority() int    { return 100 }


func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "/ET ") ||
		strings.Contains(text, "/IR ") ||
		strings.Contains(text, "/B6 ") ||
		strings.Contains(text, "/OS ") ||
		strings.Contains(text, "/C3 ")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	text := strings.TrimSpace(msg.Text)
	match := compiler.Parse(text)
	if match == nil {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	switch match.FormatName {
	case "et_exp_time":
		result.MessageType = "ET"
		result.Origin = match.Captures["origin"]
		result.Destination = match.Captures["dest"]
		result.ReportTime = match.Captures["time"]
		result.ETA = match.Captures["eta"]
		result.Mode = match.Captures["mode"]
		if day, err := strconv.Atoi(match.Captures["day"]); err == nil {
			result.DayOfMonth = day
		}

	case "ir_format":
		result.MessageType = "IR"
		result.RawData = match.Captures["flight"]
		result.ETA = match.Captures["eta"]

	case "b6_ldg_data":
		result.MessageType = "B6"
		result.Destination = match.Captures["dest"]
		result.ETA = match.Captures["eta"]
		result.Runway = match.Captures["runway"]
		result.Gate = match.Captures["gate"]

	case "os_format":
		result.MessageType = "OS"
		result.Origin = match.Captures["origin"]
		result.Destination = match.Captures["dest"]
		result.ReportTime = match.Captures["time"]

	case "c3_route":
		result.MessageType = "C3"
		result.Origin = match.Captures["origin"]
		result.Destination = match.Captures["dest"]

	default:
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
		trace.QuickCheck.Reason = "No /ET, /IR, /B6, /OS, or /C3 prefix found"
		return trace
	}

	compiler, err := getCompiler()
	if err != nil {
		trace.QuickCheck.Reason = "Failed to get compiler: " + err.Error()
		return trace
	}

	text := strings.TrimSpace(msg.Text)
	compilerTrace := compiler.ParseWithTrace(text)

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
