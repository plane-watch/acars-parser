// Package label22 parses Label 22 detailed position messages.
package label22

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents a parsed Label 22 position message.
type Result struct {
	MsgID       int64   `json:"message_id"`
	Timestamp   string  `json:"timestamp"`
	Tail        string  `json:"tail,omitempty"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	ReportTime  string  `json:"report_time,omitempty"`
	Altitude    int     `json:"altitude,omitempty"`    // Feet
	Mach        float64 `json:"mach,omitempty"`        // Mach number
	FlightLevel int     `json:"flight_level,omitempty"`
	GroundSpeed int     `json:"ground_speed,omitempty"`
	Track       int     `json:"track,omitempty"`
	RawData     string  `json:"raw_data,omitempty"`
}

func (r *Result) Type() string     { return "label22_position" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 22 detailed position messages.
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

func (p *Parser) Name() string     { return "label22" }
func (p *Parser) Labels() []string { return []string{"22"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "N ") || strings.HasPrefix(text, "S ")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)

	// Try grok-based parsing.
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(text)
	if match == nil || match.FormatName != "dms_position" {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
		RawData:   match.Captures["rest"],
	}

	// Parse latitude (DDMMSS format - 2 degree digits, seconds).
	result.Latitude = patterns.ParseLatitude(match.Captures["lat"], match.Captures["lat_dir"])

	// Parse longitude (DDDMMSS format - 3 degree digits, seconds).
	result.Longitude = patterns.ParseLongitude(match.Captures["lon"], match.Captures["lon_dir"])

	// Parse remaining comma-delimited fields.
	parseFields(match.Captures["rest"], result)

	return result
}

// parseFields extracts additional fields from the comma-delimited data.
func parseFields(data string, result *Result) {
	fields := strings.Split(data, ",")

	for i, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || field == "-------" || field == "-" {
			continue
		}

		switch i {
		case 1:
			// Field 1: Often timestamp (HHMMSS).
			if len(field) == 6 {
				if _, err := strconv.Atoi(field); err == nil {
					result.ReportTime = field
				}
			}
		case 2:
			// Field 2: Altitude.
			if alt, err := strconv.Atoi(field); err == nil {
				result.Altitude = alt
			}
		case 6:
			// Field 6: Often contains M followed by mach tenths.
			if strings.HasPrefix(field, "M") {
				machStr := strings.TrimPrefix(field, "M")
				machStr = strings.TrimSpace(machStr)
				if mach, err := strconv.Atoi(machStr); err == nil {
					result.Mach = 0.8 + float64(mach)/100.0
				}
			}
		case 7:
			// Field 7: Sometimes FL + something.
			// 31104 could be FL311 + 04.
			if len(field) >= 3 {
				if fl, err := strconv.Atoi(field[0:3]); err == nil && fl >= 100 && fl <= 500 {
					result.FlightLevel = fl
				}
			}
		case 8:
			// Field 8: Ground speed.
			if gs, err := strconv.Atoi(field); err == nil {
				result.GroundSpeed = gs
			}
		case 9:
			// Field 9: Track.
			if track, err := strconv.Atoi(field); err == nil && track >= 0 && track <= 360 {
				result.Track = track
			}
		}
	}
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
		trace.QuickCheck.Reason = "Doesn't start with N or S followed by space"
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

	trace.Matched = compilerTrace.Match != nil && compilerTrace.Match.FormatName == "dms_position"
	return trace
}
