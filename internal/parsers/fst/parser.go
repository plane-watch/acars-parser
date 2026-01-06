// Package fst parses FST (Label 15) flight status messages.
package fst

import (
	"regexp"
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents a parsed Label 15 FST (Flight Status) report.
type Result struct {
	MsgID       int64   `json:"message_id"`
	Timestamp   string  `json:"timestamp"`
	Tail        string  `json:"tail,omitempty"`
	Sequence    string  `json:"sequence,omitempty"`    // Usually "01"
	Origin      string  `json:"origin,omitempty"`      // ICAO code
	Destination string  `json:"destination,omitempty"` // ICAO code
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	FlightLevel int     `json:"flight_level,omitempty"`
	Heading     int     `json:"heading,omitempty"`
	GroundSpeed int     `json:"ground_speed,omitempty"`
	Temperature int     `json:"temperature,omitempty"` // Celsius, can be negative
	RawData     string  `json:"raw_data,omitempty"`    // Remaining unparsed data
}

func (r *Result) Type() string     { return "fst" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 15 FST messages.
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

func (p *Parser) Name() string     { return "fst" }
func (p *Parser) Labels() []string { return []string{"15"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "FST") ||
		strings.Contains(text, "FST01") ||
		strings.Contains(text, "FST02")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)

	// Strip any prefix before FST (like M51ABA0012).
	if idx := strings.Index(text, "FST"); idx > 0 {
		text = text[idx:]
	}

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
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Tail:        msg.Tail,
		Sequence:    match.Captures["seq"],
		Origin:      match.Captures["origin"],
		Destination: match.Captures["dest"],
	}

	// Parse latitude (DDMMD or DDMMDD format - degrees, minutes, tenths).
	lat := parseCoord(match.Captures["lat"])
	if match.Captures["lat_dir"] == "S" {
		lat = -lat
	}
	result.Latitude = lat

	// Parse longitude.
	lon := parseCoord(match.Captures["lon"])
	if match.Captures["lon_dir"] == "W" {
		lon = -lon
	}
	result.Longitude = lon

	// Parse the rest of the fields.
	rest := match.Captures["rest"]
	if len(rest) > 0 {
		result.RawData = rest
		parseFields(rest, result)
	}

	return result
}

// parseCoord parses FST coordinate format.
// 5-digit: DDMMD format (deg, min, tenths) - 51420 = 51 deg 42.0' = 51.7 deg
// 6-digit: DDMMTT format (deg, min, hundredths) - 175178 = 17 deg 51.78' = 17.863 deg
//
//	or DDDMMD for longitudes > 99 deg - 1043245 would be handled separately
func parseCoord(s string) float64 {
	if len(s) < 5 {
		return 0
	}

	var deg int
	var min float64
	var err error

	if len(s) == 5 {
		// DDMMD format: 2 digits degrees, 2 digits minutes, 1 digit tenths.
		deg, err = strconv.Atoi(s[0:2])
		if err != nil {
			return 0
		}
		minWhole, err := strconv.Atoi(s[2:4])
		if err != nil {
			return 0
		}
		minTenths, err := strconv.Atoi(s[4:5])
		if err != nil {
			return 0
		}
		min = float64(minWhole) + float64(minTenths)/10.0
	} else if len(s) == 6 {
		// Check if first 2 digits could be reasonable latitude (< 90) or longitude part.
		deg2, _ := strconv.Atoi(s[0:2])
		deg3, _ := strconv.Atoi(s[0:3])

		if deg2 <= 90 {
			// DDMMTT format: 2 digits degrees, 2 digits minutes, 2 digits hundredths.
			deg = deg2
			minWhole, err := strconv.Atoi(s[2:4])
			if err != nil {
				return 0
			}
			minHundredths, err := strconv.Atoi(s[4:6])
			if err != nil {
				return 0
			}
			min = float64(minWhole) + float64(minHundredths)/100.0
		} else if deg3 <= 180 {
			// DDDMMD format for longitude > 99 deg.
			deg = deg3
			minWhole, err := strconv.Atoi(s[3:5])
			if err != nil {
				return 0
			}
			minTenths, err := strconv.Atoi(s[5:6])
			if err != nil {
				return 0
			}
			min = float64(minWhole) + float64(minTenths)/10.0
		} else {
			return 0
		}
	} else {
		return 0
	}

	if min >= 60 {
		return 0
	}

	return float64(deg) + min/60.0
}

// parseFields tries to extract additional fields from the FST data.
func parseFields(data string, result *Result) {
	// Look for temperature pattern (M/P followed by digits, optionally ending with C).
	// M020 = -20 deg C, P5C = +5 deg C, M34C = -34 deg C.
	tempPattern := regexp.MustCompile(`([MP])\s*(\d{1,3})C?`)
	if m := tempPattern.FindStringSubmatch(data); m != nil {
		if temp, err := strconv.Atoi(m[2]); err == nil {
			if m[1] == "M" {
				result.Temperature = -temp
			} else {
				result.Temperature = temp
			}
		}
	}

	// Try to extract FL from the beginning of rest data.
	// Often format is 3-digit FL followed by other data.
	if len(data) >= 3 {
		if fl, err := strconv.Atoi(data[0:3]); err == nil && fl >= 0 && fl <= 600 {
			result.FlightLevel = fl
		}
	}

	// Try to find heading and ground speed patterns.
	// These are typically 3-digit values in specific positions.
	nums := regexp.MustCompile(`\d{3}`).FindAllString(data, -1)
	if len(nums) >= 2 {
		// Heuristic: heading is usually first 3-digit after FL, GS is second.
		if hdg, err := strconv.Atoi(nums[0]); err == nil && hdg <= 360 {
			result.Heading = hdg
		}
		if len(nums) >= 2 {
			if gs, err := strconv.Atoi(nums[1]); err == nil && gs > 0 && gs < 1000 {
				result.GroundSpeed = gs
			}
		}
	}
}
