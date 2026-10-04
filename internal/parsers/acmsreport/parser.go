package acmsreport

import (
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the header and CC block of an Airbus ACMS report.
type Result struct {
	MsgID     int64  `json:"message_id"`
	Timestamp string `json:"timestamp"`

	// AircraftSeries is the series as the report gives it, e.g. "A321". It
	// is not an ICAO type designator: the same value is sent by the ceo
	// and neo models (A321 and A21N), so it is deliberately not reported
	// as "aircraft_type".
	AircraftSeries string `json:"aircraft_series"`
	Report         string `json:"report"` // The report number, e.g. "019".

	// Registration is reported only when it is the transmitted tail.
	Registration string `json:"registration,omitempty"`
	ReportDate   string `json:"report_date"` // MMMDD, e.g. JAN20; no year.
	ReportTime   string `json:"report_time"` // HHMMSS.
	Origin       string `json:"origin"`
	Destination  string `json:"destination"`

	// FlightNumberDigits is the flight number without its airline code,
	// e.g. "0816". The extractor uses the route only for a transmitted
	// flight with this number.
	FlightNumberDigits string `json:"flight_number_digits"`
}

func (r *Result) Type() string     { return "acms_report" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Airbus ACMS report headers.
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

func (p *Parser) Name() string     { return "acmsreport" }
func (p *Parser) Labels() []string { return []string{"H1"} }
func (p *Parser) Priority() int    { return 60 }

// QuickCheck looks for the series prefix ("A3xx,") and a CC block.
func (p *Parser) QuickCheck(text string) bool {
	return len(text) > 5 && strings.HasPrefix(text, "A3") && text[4] == ',' && strings.Contains(text, "/CC")
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
	// The pattern checks the airports' shape only; reject codes that are
	// not plausible ICAO airport codes.
	if !patterns.IsValidICAO(c["origin"]) || !patterns.IsValidICAO(c["dest"]) {
		return nil
	}

	result := &Result{
		MsgID:              int64(msg.ID),
		Timestamp:          msg.Timestamp,
		AircraftSeries:     c["series"],
		Report:             c["report"],
		ReportDate:         c["date"],
		ReportTime:         c["time"],
		Origin:             c["origin"],
		Destination:        c["dest"],
		FlightNumberDigits: c["flight_digits"],
	}
	reg := strings.TrimPrefix(c["reg_field"], ".")
	if sameRegistration(reg, msg.Tail) {
		result.Registration = reg
	}
	return result
}

// sameRegistration reports whether the report's registration is the
// transmitted tail, ignoring dashes and a leading "." (as some messages
// transmit the tail). An empty tail matches nothing.
func sameRegistration(reg, tail string) bool {
	norm := func(s string) string {
		return strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(s), "."), "-", "")
	}
	t := norm(tail)
	return t != "" && norm(reg) == t
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{ParserName: p.Name()}
	passed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{Passed: passed}
	if !passed {
		trace.QuickCheck.Reason = "text does not start with an A3xx series and contain a CC block"
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
