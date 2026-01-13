// Package label44 parses Label 44 messages (runway info, FB positions, POS reports).
package label44

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/airports"
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

// RunwayInfo represents a runway entry from takeoff info.
type RunwayInfo struct {
	Runway   string `json:"runway"`
	Suffix   string `json:"suffix,omitempty"`   // Y, AA, R, BHB, etc.
	Distance int    `json:"distance,omitempty"` // Runway length in feet
}

// Result represents a parsed Label 44 message.
type Result struct {
	MsgID           int64        `json:"message_id"`
	Timestamp       string       `json:"timestamp"`
	Tail            string       `json:"tail,omitempty"`
	MessageType     string       `json:"message_type"` // "runway", "fb", "pos"
	Airport         string       `json:"airport,omitempty"`
	AirportName     string       `json:"airport_name,omitempty"`
	Runways         []RunwayInfo `json:"runways,omitempty"`
	Procedures      []string     `json:"procedures,omitempty"`
	Latitude        float64      `json:"latitude,omitempty"`
	Longitude       float64      `json:"longitude,omitempty"`
	FlightLevel     int          `json:"flight_level,omitempty"`
	Origin          string       `json:"origin,omitempty"`
	Destination     string       `json:"destination,omitempty"`
	OriginName      string       `json:"origin_name,omitempty"`
	DestinationName string       `json:"destination_name,omitempty"`
	Callsign        string       `json:"callsign,omitempty"`
	ReportTime      string       `json:"report_time,omitempty"`
	RawData         string       `json:"raw_data,omitempty"`
}

func (r *Result) Type() string     { return "label44" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 44 messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "label44" }
func (p *Parser) Labels() []string { return []string{"44"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	// Skip encoded/binary messages
	if strings.Contains(text, "|") || strings.Contains(text, "\\") {
		return false
	}
	return strings.Contains(text, "T/O RWY") ||
		strings.Contains(text, "/FB ") ||
		strings.HasPrefix(text, "POS")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)

	// Skip encoded/binary messages
	if strings.Contains(text, "|") || strings.Contains(text, "\\") {
		return nil
	}

	// Try runway info format
	if result := p.parseRunwayInfo(msg, text); result != nil {
		return result
	}

	// Try FB (Flight Brief) position format
	if result := p.parseFBPosition(msg, text); result != nil {
		return result
	}

	// Try POS position format
	if result := p.parsePOSReport(msg, text); result != nil {
		return result
	}

	return nil
}

func (p *Parser) parseRunwayInfo(msg *acars.Message, text string) *Result {
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	// Try to match the runway header format.
	match := compiler.Parse(text)
	if match == nil || match.FormatName != "runway_header" {
		return nil
	}

	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Tail:        msg.Tail,
		MessageType: "runway",
		Airport:     match.Captures["airport"],
		AirportName: airports.GetName(match.Captures["airport"]),
		Runways:     []RunwayInfo{},
		Procedures:  []string{},
	}

	primaryRunway := match.Captures["runway"]

	// Parse each line.
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		// Remove carriage returns but keep leading spaces for now.
		line = strings.ReplaceAll(line, "\r", "")
		if strings.TrimSpace(line) == "" {
			continue
		}

		// First line contains the header with primary runway.
		if i == 0 {
			// Extract distance from first line if present.
			parts := strings.Fields(line)
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				if dist, err := strconv.Atoi(lastPart); err == nil && dist > 1000 {
					result.Runways = append(result.Runways, RunwayInfo{
						Runway:   primaryRunway,
						Distance: dist,
					})
				}
			}
			continue
		}

		// Check if it's a procedure/comment line (indented with spaces).
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(line, " ") && !strings.HasPrefix(trimmedLine, "0") {
			// Try to match as runway line first.
			lineMatch := compiler.Parse(trimmedLine)
			if lineMatch != nil && lineMatch.FormatName == "runway_line" {
				// It's a runway line, not a procedure.
			} else if trimmedLine != "" {
				result.Procedures = append(result.Procedures, trimmedLine)
			}
			continue
		}

		// Check if it's a runway line using grok.
		lineMatch := compiler.Parse(trimmedLine)
		if lineMatch != nil && lineMatch.FormatName == "runway_line" {
			rwy := RunwayInfo{
				Runway: lineMatch.Captures["runway"],
				Suffix: lineMatch.Captures["suffix"],
			}
			if dist, err := strconv.Atoi(lineMatch.Captures["distance"]); err == nil {
				rwy.Distance = dist
			}
			result.Runways = append(result.Runways, rwy)
		}
	}

	return result
}

func (p *Parser) parseFBPosition(msg *acars.Message, text string) *Result {
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(text)
	if match == nil || match.FormatName != "fb_position" {
		return nil
	}

	result := &Result{
		MsgID:           int64(msg.ID),
		Timestamp:       msg.Timestamp,
		Tail:            msg.Tail,
		MessageType:     "fb",
		Airport:         match.Captures["airport"],
		AirportName:     airports.GetName(match.Captures["airport"]),
		Callsign:        match.Captures["callsign"],
		Destination:     match.Captures["dest"],
		DestinationName: airports.GetName(match.Captures["dest"]),
		ReportTime:      match.Captures["time"],
		RawData:         match.Captures["unknown"], // Unknown field (INA03, INR03, etc.)
	}

	// Parse latitude.
	if lat, err := strconv.ParseFloat(match.Captures["lat"], 64); err == nil {
		if match.Captures["lat_dir"] == "S" {
			lat = -lat
		}
		result.Latitude = lat
	}

	// Parse longitude.
	if lon, err := strconv.ParseFloat(match.Captures["lon"], 64); err == nil {
		if match.Captures["lon_dir"] == "W" {
			lon = -lon
		}
		result.Longitude = lon
	}

	return result
}

func (p *Parser) parsePOSReport(msg *acars.Message, text string) *Result {
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(text)
	if match == nil || match.FormatName != "pos_report" {
		return nil
	}

	result := &Result{
		MsgID:           int64(msg.ID),
		Timestamp:       msg.Timestamp,
		Tail:            msg.Tail,
		MessageType:     "pos",
		Origin:          match.Captures["origin"],
		OriginName:      airports.GetName(match.Captures["origin"]),
		Destination:     match.Captures["dest"],
		DestinationName: airports.GetName(match.Captures["dest"]),
		ReportTime:      match.Captures["time1"],
	}

	// Parse latitude (format: DDMMD - 2 degree digits, tenths of minutes).
	result.Latitude = patterns.ParseLatitude(match.Captures["lat"], match.Captures["lat_dir"])

	// Parse longitude (format: DDDMMD - 3 degree digits, tenths of minutes).
	result.Longitude = patterns.ParseLongitude(match.Captures["lon"], match.Captures["lon_dir"])

	// Parse flight level.
	if fl, err := strconv.Atoi(match.Captures["fl"]); err == nil {
		result.FlightLevel = fl
	}

	return result
}
