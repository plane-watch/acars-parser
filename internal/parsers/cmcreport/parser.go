package cmcreport

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the header of a Boeing CMC report.
type Result struct {
	MsgID        int64  `json:"message_id"`
	Timestamp    string `json:"timestamp"`
	ReportType   string `json:"report_type"`       // RTE, PLF or CFG.
	Sequence     int    `json:"sequence"`          // The report's sequence number.
	ReportDate   string `json:"report_date"`       // DDMMMYY, e.g. 04OCT26.
	ReportTime   string `json:"report_time"`       // HHMM.
	Airline      string `json:"airline,omitempty"` // IATA airline code, when transmitted.
	Registration string `json:"registration,omitempty"`
	Flight       string `json:"flight"` // ICAO callsign.
	Origin       string `json:"origin"`
	Destination  string `json:"destination"`
}

func (r *Result) Type() string     { return "cmc_report" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Boeing CMC report headers.
type Parser struct{}

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

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "cmcreport" }
func (p *Parser) Labels() []string { return []string{"H1"} }
func (p *Parser) Priority() int    { return 60 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "RTE ") || strings.HasPrefix(text, "PLF ") || strings.HasPrefix(text, "CFG ")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
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
	c := match.Captures

	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		ReportType:  c["report_type"],
		ReportDate:  c["date"],
		ReportTime:  c["time"],
		Airline:     c["airline"],
		Flight:      c["flight"],
		Origin:      c["origin"],
		Destination: c["dest"],
	}
	result.Sequence, _ = strconv.Atoi(c["seq"])

	if reg, glued, ok := splitRegistration(c["reg_field"], msg.Tail); ok {
		result.Registration = reg
		if glued != "" {
			result.Airline = glued
		}
	}
	return result
}

// splitRegistration finds the registration in the header's registration
// field using the tail transmitted in the ACARS header. The field may carry
// the IATA airline code glued to its front ("5YN703GT", "BRB-17807"), which
// cannot be told apart from the registration reliably on its own; the
// transmitted tail settles it. Dashes are ignored when comparing, since the
// header and the field sometimes differ only by one ("HP-9907" and "HP9907").
// The registration is returned in the field's form, with any glued prefix,
// which must be a 2-character airline code. ok is false when the field does
// not end with the tail, or the prefix is not 2 characters: the registration
// is then not reported rather than guessed.
func splitRegistration(field, tail string) (reg, glued string, ok bool) {
	tail = strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(tail), "."), "-", "")
	if tail == "" {
		return "", "", false
	}
	// Walk back from the end of the field until len(tail) non-dash
	// characters have been matched.
	matched, start := 0, len(field)
	for start > 0 && matched < len(tail) {
		start--
		if field[start] == '-' {
			continue
		}
		if field[start] != tail[len(tail)-1-matched] {
			return "", "", false
		}
		matched++
	}
	if matched < len(tail) {
		return "", "", false
	}
	prefix := field[:start]
	if prefix != "" && len(prefix) != 2 {
		return "", "", false
	}
	return field[start:], prefix, true
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{ParserName: p.Name()}
	passed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{Passed: passed}
	if !passed {
		trace.QuickCheck.Reason = "text does not start with RTE, PLF or CFG"
		return trace
	}
	compiler, err := getCompiler()
	if err != nil {
		trace.QuickCheck.Reason = "failed to get compiler: " + err.Error()
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
	trace.Matched = p.Parse(msg) != nil
	return trace
}
