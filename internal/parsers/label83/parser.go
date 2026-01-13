// Package label83 parses Label 83 position report messages.
package label83

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/airports"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents a parsed Label 83 position report.
type Result struct {
	MsgID           int64   `json:"message_id"`
	Timestamp       string  `json:"timestamp"`
	Tail            string  `json:"tail,omitempty"`
	MessageType     string  `json:"message_type"` // PR or ZSPD
	DayOfMonth      int     `json:"day_of_month,omitempty"`
	ReportTime      string  `json:"report_time,omitempty"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Altitude        int     `json:"altitude,omitempty"`
	Heading         int     `json:"heading,omitempty"`
	GroundSpeed     float64 `json:"ground_speed,omitempty"`
	Origin          string  `json:"origin,omitempty"`
	Destination     string  `json:"destination,omitempty"`
	OriginName      string  `json:"origin_name,omitempty"`
	DestinationName string  `json:"destination_name,omitempty"`
}

func (r *Result) Type() string     { return "label83_position" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 83 position report messages.
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

func (p *Parser) Name() string     { return "label83" }
func (p *Parser) Labels() []string { return []string{"83"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "PR") || strings.Contains(text, "ZSPD")
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
	if match == nil {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	switch match.FormatName {
	case "pr_position":
		result.MessageType = "PR"
		result.ReportTime = match.Captures["time"]

		if day, err := strconv.Atoi(match.Captures["day"]); err == nil {
			result.DayOfMonth = day
		}

		// Parse latitude (format: DDMM.D - 2 degree digits, decimal minutes).
		result.Latitude = patterns.ParseLatitude(match.Captures["lat"], match.Captures["lat_dir"])

		// Parse longitude (format: DDDMM.D - 3 degree digits, decimal minutes).
		result.Longitude = patterns.ParseLongitude(match.Captures["lon"], match.Captures["lon_dir"])

		// Parse altitude.
		if alt, err := strconv.Atoi(match.Captures["altitude"]); err == nil {
			result.Altitude = alt
		}

	case "zspd_position":
		result.MessageType = "ZSPD"
		result.Origin = match.Captures["origin"]
		result.Destination = match.Captures["dest"]
		result.OriginName = airports.GetName(result.Origin)
		result.DestinationName = airports.GetName(result.Destination)
		result.ReportTime = match.Captures["time"]

		// Parse latitude (decimal).
		if lat, err := strconv.ParseFloat(match.Captures["lat"], 64); err == nil {
			result.Latitude = lat
		}

		// Parse longitude (decimal).
		if lon, err := strconv.ParseFloat(match.Captures["lon"], 64); err == nil {
			result.Longitude = lon
		}

		// Parse altitude.
		if alt, err := strconv.Atoi(match.Captures["altitude"]); err == nil {
			result.Altitude = alt
		}

		// Parse heading.
		if hdg, err := strconv.Atoi(match.Captures["heading"]); err == nil {
			result.Heading = hdg
		}

		// Parse ground speed.
		if gs, err := strconv.ParseFloat(match.Captures["ground_speed"], 64); err == nil {
			result.GroundSpeed = gs
		}
	}

	return result
}
