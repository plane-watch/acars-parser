// Package sq parses SQ (Squitter) ARINC position messages.
package sq

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents a parsed SQ ARINC position message.
// These messages contain airport IATA/ICAO mapping and position data.
type Result struct {
	MsgID       int64   `json:"message_id"`
	Timestamp   string  `json:"timestamp"`
	Tail        string  `json:"tail,omitempty"`
	IATACode    string  `json:"iata_code"`              // 3-char IATA airport code
	ICAOCode    string  `json:"icao_code"`              // 4-char ICAO airport code
	Latitude    float64 `json:"latitude"`               // Decimal degrees, negative for south
	Longitude   float64 `json:"longitude"`              // Decimal degrees, negative for west
	FreqBand    string  `json:"freq_band,omitempty"`    // V=VHF, B=?
	FreqMHz     float64 `json:"freq_mhz,omitempty"`     // Frequency in MHz (e.g., 136.975)
	MessageType string  `json:"message_type,omitempty"` // A or S from prefix
}

func (r *Result) Type() string     { return "sq_position" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses SQ (Squitter) ARINC position messages.
// Format: 02XA/02XS + IATA(3) + ICAO(4) + lat(5) + N/S + lon(5) + E/W + band+freq + suffix
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

func (p *Parser) Name() string     { return "sq" }
func (p *Parser) Labels() []string { return []string{"SQ"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	// Fast check for the 02X prefix.
	return strings.HasPrefix(text, "02X")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)

	// Skip the short "00XS" messages which don't contain position data.
	if strings.HasPrefix(text, "00X") {
		return nil
	}

	// Try grok-based parsing.
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(text)
	if match == nil || match.FormatName != "arinc_position" {
		return nil
	}

	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Tail:        msg.Tail,
		MessageType: match.Captures["msg_type"],
		IATACode:    match.Captures["iata"],
		ICAOCode:    match.Captures["icao"],
		FreqBand:    match.Captures["freq_band"],
	}

	// Parse latitude: 5 digits where first is a check/flag digit, remaining 4 are DDMM.
	if lat, ok := parseLatitude(match.Captures["lat"], match.Captures["lat_dir"]); ok {
		result.Latitude = lat
	}

	// Parse longitude: 5 digits where first is hundreds digit, remaining 4 are DDMM.
	if lon, ok := parseLongitude(match.Captures["lon"], match.Captures["lon_dir"]); ok {
		result.Longitude = lon
	}

	// Parse frequency: 6 digits like 136975 = 136.975 MHz.
	if freq, err := strconv.ParseFloat(match.Captures["freq"], 64); err == nil {
		result.FreqMHz = freq / 1000.0
	}

	return result
}

// parseLatitude parses the 5-digit latitude format used in SQ messages.
// The first digit appears to be a check/flag digit; the remaining 4 are DDMM.
// Returns latitude in decimal degrees (negative for south).
func parseLatitude(digits string, hemisphere string) (float64, bool) {
	if len(digits) != 5 {
		return 0, false
	}

	// Skip first digit (check/flag), parse remaining DDMM.
	deg, err := strconv.Atoi(digits[1:3])
	if err != nil || deg > 90 {
		return 0, false
	}

	min, err := strconv.Atoi(digits[3:5])
	if err != nil || min > 59 {
		return 0, false
	}

	lat := float64(deg) + float64(min)/60.0

	if hemisphere == "S" {
		lat = -lat
	}

	return lat, true
}

// parseLongitude parses the 5-digit longitude format used in SQ messages.
// The first digit is the hundreds digit (0 or 1), remaining 4 are DDMM.
// Returns longitude in decimal degrees (negative for west).
func parseLongitude(digits string, hemisphere string) (float64, bool) {
	if len(digits) != 5 {
		return 0, false
	}

	// First digit is hundreds (0 or 1), next 2 are tens/units, last 2 are minutes.
	hundreds, err := strconv.Atoi(digits[0:1])
	if err != nil || hundreds > 1 {
		return 0, false
	}

	tensUnits, err := strconv.Atoi(digits[1:3])
	if err != nil {
		return 0, false
	}

	deg := hundreds*100 + tensUnits
	if deg > 180 {
		return 0, false
	}

	min, err := strconv.Atoi(digits[3:5])
	if err != nil || min > 59 {
		return 0, false
	}

	lon := float64(deg) + float64(min)/60.0

	if hemisphere == "W" {
		lon = -lon
	}

	return lon, true
}
