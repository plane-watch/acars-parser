// Package label39 parses Label 39 position/status report messages.
package label39

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/airports"
	"acars_parser/internal/registry"
)

// Result represents a position/status report from label 39 messages.
type Result struct {
	MsgID       int64   `json:"message_id"`
	Timestamp   string  `json:"timestamp"`
	Tail        string  `json:"tail,omitempty"`
	MsgType     string  `json:"msg_type"`
	Header      string  `json:"header,omitempty"`        // CDC01, ADC01, ODO01, PBS01, LDC01
	OriginICAO  string  `json:"origin_icao,omitempty"`   // First 4 chars of route
	OriginName  string  `json:"origin_name,omitempty"`   // Airport name from ICAO
	DestICAO    string  `json:"dest_icao,omitempty"`     // Last 4 chars of route
	DestName    string  `json:"dest_name,omitempty"`     // Airport name from ICAO
	ETA         string  `json:"eta,omitempty"`           // HH:MM:SS format
	FuelOnBoard int     `json:"fuel_on_board,omitempty"` // FOB value
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
}

func (r *Result) Type() string     { return "position_status" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 39 messages.
type Parser struct{}

var (
	// Match header: 3 letters + 2 digits + AFL + flight number
	// Example: CDC01AFL1334, ADC01AFL1334, ODO01AFL1383
	headerAFLRe = regexp.MustCompile(`(?i)^([A-Z]{3}\d{2})AFL(\d+)`)

	// Match route (8 letters) - UUEEULAA, USHHUUEE, UNTTUUEE
	routeRe = regexp.MustCompile(`(?i)([A-Z]{8})`)

	// Match ETA time (6 digits HHMMSS)
	etaTimeRe = regexp.MustCompile(`\s(\d{6})\s`)

	// Match FOB (Fuel On Board)
	fobRe = regexp.MustCompile(`(?i)FOB\s+(\d+)`)

	// Match LATN/LATS with decimal
	latRe = regexp.MustCompile(`(?i)LAT([NS])\s*([\d.]+)`)

	// Match LONE/LONW with decimal
	lonRe = regexp.MustCompile(`(?i)LON([EW])\s*([\d.]+)`)
)

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "label39" }
func (p *Parser) Labels() []string { return []string{"39"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)
	// Check for known header patterns followed by AFL
	return (strings.Contains(upper, "AFL") &&
		(strings.HasPrefix(upper, "CDC") || strings.HasPrefix(upper, "ADC") ||
			strings.HasPrefix(upper, "ODO") || strings.HasPrefix(upper, "PBS") ||
			strings.HasPrefix(upper, "LDC") || strings.HasPrefix(upper, "PBR") ||
			strings.HasPrefix(upper, "CDO")))
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)
	upper := strings.ToUpper(text)

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
		MsgType:   "position_status",
	}

	// Parse header and AFL flight number
	if m := headerAFLRe.FindStringSubmatch(upper); m != nil {
		result.Header = m[1]
	} else {
		// No valid header format found
		return nil
	}

	// Parse route (8 letters: origin + dest)
	if m := routeRe.FindStringSubmatch(upper); m != nil {
		route := m[1]
		if len(route) == 8 {
			result.OriginICAO = route[0:4]
			result.DestICAO = route[4:8]
			// Lookup airport names
			result.OriginName = airports.GetName(result.OriginICAO)
			result.DestName = airports.GetName(result.DestICAO)
		}
	}

	// Parse ETA time (HHMMSS)
	if m := etaTimeRe.FindStringSubmatch(text); m != nil {
		timeStr := m[1]
		if len(timeStr) == 6 {
			hh := timeStr[0:2]
			mm := timeStr[2:4]
			ss := timeStr[4:6]
			result.ETA = hh + ":" + mm + ":" + ss
		}
	}

	// Parse fuel on board
	if m := fobRe.FindStringSubmatch(upper); m != nil {
		if fob, err := strconv.Atoi(m[1]); err == nil {
			result.FuelOnBoard = fob
		}
	}

	// Parse latitude
	if m := latRe.FindStringSubmatch(upper); m != nil {
		if lat, err := strconv.ParseFloat(m[2], 64); err == nil {
			if m[1] == "S" {
				lat = -lat
			}
			result.Latitude = lat
		}
	}

	// Parse longitude
	if m := lonRe.FindStringSubmatch(upper); m != nil {
		if lon, err := strconv.ParseFloat(m[2], 64); err == nil {
			if m[1] == "W" {
				lon = -lon
			}
			result.Longitude = lon
		}
	}

	return result
}
