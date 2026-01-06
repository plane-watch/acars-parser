// Package label4j parses Label 4J position + weather messages.
package label4j

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents position + weather data from label 4J messages.
type Result struct {
	MsgID           int64   `json:"message_id"`
	Timestamp       string  `json:"timestamp"`
	Tail            string  `json:"tail,omitempty"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Heading         int     `json:"heading,omitempty"`
	Altitude        int     `json:"altitude,omitempty"`
	Temperature     string  `json:"temperature,omitempty"`
	CurrentWaypoint string  `json:"current_waypoint,omitempty"`
	NextWaypoint    string  `json:"next_waypoint,omitempty"`
	ETA             string  `json:"eta,omitempty"`
	FuelBurn        int     `json:"fuel_burn,omitempty"`
}

func (r *Result) Type() string     { return "pos_weather" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 4J position + weather messages.
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

func (p *Parser) Name() string     { return "label4j" }
func (p *Parser) Labels() []string { return []string{"4J"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "POS")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := msg.Text
	if !strings.HasPrefix(text, "POS") {
		return nil
	}

	// Try grok-based parsing.
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	// Parse all formats to get both position and fuel burn.
	matches := compiler.ParseAll(text)
	if len(matches) == 0 {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	for _, match := range matches {
		switch match.FormatName {
		case "pos_weather":
			// Parse latitude (DDMMD format - 5 digits, 2 degree digits).
			result.Latitude = patterns.ParseDMSCoord(match.Captures["lat"], 2, "N")

			// Parse longitude (DDDMMD format - 6 digits, 3 degree digits).
			result.Longitude = patterns.ParseDMSCoord(match.Captures["lon"], 3, match.Captures["lon_dir"])

			// Parse heading.
			if hdg, err := strconv.Atoi(match.Captures["heading"]); err == nil {
				result.Heading = hdg
			}

			result.CurrentWaypoint = match.Captures["curr_wpt"]
			result.ETA = match.Captures["eta"]
			result.NextWaypoint = match.Captures["next_wpt"]
			result.Temperature = match.Captures["temp"]

			// Parse altitude.
			if alt, err := strconv.Atoi(match.Captures["altitude"]); err == nil {
				result.Altitude = alt
			}

		case "fuel_burn":
			if fb, err := strconv.Atoi(match.Captures["fuel_burn"]); err == nil {
				result.FuelBurn = fb
			}
		}
	}

	// Check we got the main position data.
	if result.Latitude == 0 && result.Longitude == 0 {
		return nil
	}

	return result
}
