// Package label80 parses Label 80 position messages.
package label80

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents position data from label 80 messages.
type Result struct {
	MsgID       int64   `json:"message_id"`
	Timestamp   string  `json:"timestamp"`
	Tail        string  `json:"tail,omitempty"`
	MsgType     string  `json:"msg_type"`
	FlightNum   string  `json:"flight_num,omitempty"`
	OriginICAO  string  `json:"origin_icao,omitempty"`
	DestICAO    string  `json:"dest_icao,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	Altitude    int     `json:"altitude,omitempty"`
	Mach        string  `json:"mach,omitempty"`
	TAS         int     `json:"tas,omitempty"`
	FuelOnBoard int     `json:"fuel_on_board,omitempty"`
	ETA         string  `json:"eta,omitempty"`
	OutTime     string  `json:"out_time,omitempty"`
	OffTime     string  `json:"off_time,omitempty"`
	OnTime      string  `json:"on_time,omitempty"`
	InTime      string  `json:"in_time,omitempty"`
}

func (r *Result) Type() string     { return "position" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 80 position messages.
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

func (p *Parser) Name() string     { return "label80" }
func (p *Parser) Labels() []string { return []string{"80"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return true // Label check is sufficient for 80.
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

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	// Parse all formats to extract different fields.
	matches := compiler.ParseAll(text)

	// Track if we found a header.
	foundHeader := false

	for _, match := range matches {
		switch match.FormatName {
		case "header_format":
			result.MsgType = match.Captures["msg_type"]
			result.OriginICAO = match.Captures["origin"]
			result.DestICAO = match.Captures["dest"]
			result.Tail = strings.TrimPrefix(match.Captures["tail"], ".")
			foundHeader = true

		case "alt_format":
			if !foundHeader {
				result.FlightNum = match.Captures["flight"]
				result.OriginICAO = match.Captures["origin"]
				result.DestICAO = match.Captures["dest"]
				result.MsgType = "FLT"
				foundHeader = true
			}

		case "position":
			result.Latitude = parseLabel80Coord(match.Captures["lat"], match.Captures["lat_dir"])
			result.Longitude = parseLabel80Coord(match.Captures["lon"], match.Captures["lon_dir"])

		case "altitude":
			if alt, err := strconv.Atoi(match.Captures["altitude"]); err == nil {
				result.Altitude = alt
			}

		case "mach":
			result.Mach = match.Captures["mach"]

		case "tas":
			if tas, err := strconv.Atoi(match.Captures["tas"]); err == nil {
				result.TAS = tas
			}

		case "fob":
			if fob, err := strconv.Atoi(match.Captures["fob"]); err == nil {
				result.FuelOnBoard = fob
			}

		case "eta":
			result.ETA = match.Captures["eta"]

		case "out_time":
			result.OutTime = match.Captures["out"]

		case "off_time":
			result.OffTime = match.Captures["off"]

		case "on_time":
			result.OnTime = match.Captures["on"]

		case "in_time":
			result.InTime = match.Captures["in"]
		}
	}

	// Return nil if we couldn't parse the header.
	if !foundHeader {
		return nil
	}

	return result
}


// parseLabel80Coord parses /POS coordinates that may be encoded as:
// - decimal degrees with a dot: "44.038"
// - compact decimal degrees without a dot: "44038" (=> 44.038), "019408" (=> 19.408)
// For compact form we insert a dot after degree digits (lat:2, lon:2 or 3 depending on length).
func parseLabel80Coord(s string, dir string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.Contains(s, ".") {
		return patterns.ParseDecimalCoord(s, dir)
	}
	// compact digits-only form
	if dir == "E" || dir == "W" {
		degDigits := 3
		if len(s) <= 5 { // e.g. "19408" => 19.408
			degDigits = 2
		}
		if len(s) > degDigits {
			s = s[:degDigits] + "." + s[degDigits:]
		}
	} else {
		degDigits := 2
		if len(s) > degDigits {
			s = s[:degDigits] + "." + s[degDigits:]
		}
	}
	return patterns.ParseDecimalCoord(s, dir)
}
