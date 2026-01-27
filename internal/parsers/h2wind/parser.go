// Package h2wind parses H2 wind/weather messages.
package h2wind

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

// WindLayer represents wind data at a specific flight level.
type WindLayer struct {
	FlightLevel int  `json:"flight_level"`
	Temperature int  `json:"temperature"`        // Celsius (could be SAT or ISA deviation)
	WindDir     int  `json:"wind_dir,omitempty"`
	WindSpeed   int  `json:"wind_speed,omitempty"`
	Gusting     bool `json:"gusting,omitempty"`
}

// Result represents a parsed H2 wind/weather report.
type Result struct {
	MsgID       int64       `json:"message_id"`
	Timestamp   string      `json:"timestamp"`
	Tail        string      `json:"tail,omitempty"`
	Origin      string      `json:"origin,omitempty"`
	Destination string      `json:"destination,omitempty"`
	Latitude    float64     `json:"latitude,omitempty"`
	Longitude   float64     `json:"longitude,omitempty"`
	ReportTime  string      `json:"report_time,omitempty"`
	WindLayers  []WindLayer `json:"wind_layers,omitempty"`
	RawData     string      `json:"raw_data,omitempty"`
}

func (r *Result) Type() string     { return "h2_wind" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses H2 wind/weather messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "h2_wind" }
func (p *Parser) Labels() []string { return []string{"H2"} }
func (p *Parser) Priority() int    { return 100 }


func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "02A") && len(text) > 40
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

	// Check for encoded/binary messages (not parseable).
	if !strings.HasPrefix(text, "02A") {
		return nil
	}

	match := compiler.Parse(text)
	if match == nil || match.FormatName != "h2_header" {
		return nil
	}

	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Tail:        msg.Tail,
		Origin:      match.Captures["origin"],
		Destination: match.Captures["dest"],
		ReportTime:  match.Captures["time"],
	}

	// Parse latitude (format: DDMMD - 2 degree digits, tenths of minutes).
	result.Latitude = patterns.ParseLatitude(match.Captures["lat"], match.Captures["lat_dir"])

	// Parse longitude (format: DDDMMD - 3 degree digits, tenths of minutes).
	result.Longitude = patterns.ParseLongitude(match.Captures["lon"], match.Captures["lon_dir"])

	// Extract the wind layer data (everything after the header).
	// Calculate header length based on known format: 02A + time(6) + origin(4) + dest(4) + lat_dir(1) + lat(5) + lon_dir(1) + lon(6) + datetime(6) = 36 chars.
	headerLen := 36
	if headerLen < len(text) {
		rest := text[headerLen:]
		result.RawData = strings.TrimSpace(rest)
		result.WindLayers = parseWindLayers(compiler, rest)
	}

	return result
}

// parseWindLayers extracts wind layer data from the message body using grok patterns.
func parseWindLayers(compiler *patterns.Compiler, data string) []WindLayer {
	var layers []WindLayer

	matches := compiler.FindAllMatches(data, "wind_layer")
	for _, m := range matches {
		layer := WindLayer{}

		// Flight level.
		if fl, err := strconv.Atoi(m["fl"]); err == nil {
			layer.FlightLevel = fl
		}

		// Temperature (M = minus, P = plus).
		if temp, err := strconv.Atoi(m["temp"]); err == nil {
			if m["temp_sign"] == "M" {
				layer.Temperature = -temp
			} else {
				layer.Temperature = temp
			}
		}

		// Optional wind direction.
		if m["wind_dir"] != "" {
			if dir, err := strconv.Atoi(m["wind_dir"]); err == nil {
				layer.WindDir = dir
			}
		}

		// Optional wind speed.
		if m["wind_spd"] != "" {
			if spd, err := strconv.Atoi(m["wind_spd"]); err == nil {
				layer.WindSpeed = spd
			}
		}

		// Gusting flag.
		layer.Gusting = m["gust"] == "G"

		layers = append(layers, layer)
	}

	return layers
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
		trace.QuickCheck.Reason = "Doesn't start with 02A or message too short"
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

	trace.Matched = compilerTrace.Match != nil && compilerTrace.Match.FormatName == "h2_header"
	return trace
}
