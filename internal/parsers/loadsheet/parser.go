// Package loadsheet parses aircraft loadsheet messages from ACARS.
// These contain weight and balance data for flight operations.
package loadsheet

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Result represents parsed loadsheet data.
type Result struct {
	MsgID        int64  `json:"message_id"`
	Timestamp    string `json:"timestamp"`
	Tail         string `json:"tail,omitempty"`
	Flight       string `json:"flight,omitempty"`
	Origin       string `json:"origin,omitempty"`
	Destination  string `json:"destination,omitempty"`
	AircraftType string `json:"aircraft_type,omitempty"`
	ZFW          int    `json:"zfw,omitempty"`          // Zero Fuel Weight
	TOW          int    `json:"tow,omitempty"`          // Take Off Weight
	LAW          int    `json:"law,omitempty"`          // Landing Weight
	TOF          int    `json:"tof,omitempty"`          // Take Off Fuel
	PAX          int    `json:"pax,omitempty"`          // Passenger count
	Crew         string `json:"crew,omitempty"`         // Crew configuration
	Trim         string `json:"trim,omitempty"`         // Stabiliser trim
	MACZFW       string `json:"mac_zfw,omitempty"`      // MAC at ZFW
	MACTOW       string `json:"mac_tow,omitempty"`      // MAC at TOW
	Cargo        int    `json:"cargo,omitempty"`        // Cargo weight
	Edition      string `json:"edition,omitempty"`      // Loadsheet edition
}

func (r *Result) Type() string     { return "loadsheet" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses loadsheet messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "loadsheet" }
func (p *Parser) Labels() []string { return []string{"C1"} }
func (p *Parser) Priority() int    { return 60 } // Higher priority than weather.

// QuickCheck looks for loadsheet keywords.
func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)
	return strings.Contains(upper, "LOADSHEET")
}

// Pattern matchers.
var (
	// ZFW patterns.
	zfwRe = regexp.MustCompile(`\bZFW\s+(\d+)`)

	// TOW patterns.
	towRe = regexp.MustCompile(`\bTOW\s+(\d+)`)

	// LAW/LDW patterns.
	lawRe = regexp.MustCompile(`\b(?:LAW|LDW)\s+(\d+)`)

	// TOF patterns.
	tofRe = regexp.MustCompile(`\bTOF\s+(\d+)`)

	// PAX patterns - various formats.
	paxRe = regexp.MustCompile(`\bPAX[/\s]+(\d+)[/\s]*(\d+)?[/\s]*(\d+)?`)

	// TTL (total) pattern.
	ttlRe = regexp.MustCompile(`\bTTL[:\s]+(\d+)`)

	// Crew pattern.
	crewRe = regexp.MustCompile(`\bCREW[:\s]+(\d+/\d+(?:/\d+)?)`)

	// Trim/stabiliser pattern.
	trimRe = regexp.MustCompile(`\bSTAB[:\s]+(?:FLAPS\s+\d+/\d+\s+\d+K)?\s*([\d.]+\s*(?:UP|DN|DOWN))`)

	// MAC patterns.
	maczfwRe = regexp.MustCompile(`\bMACZFW[:\s]+([\d.]+)`)
	mactowRe = regexp.MustCompile(`\bMACTOW[:\s]+([\d.]+)`)

	// Edition pattern.
	ednoRe = regexp.MustCompile(`\bEDNO?\s*(\d+)`)

	// Flight/route pattern.
	flightRouteRe = regexp.MustCompile(`\b([A-Z]{2}\d{3,4})/\d+\s+\d+[A-Z]{3}\d+\s+([A-Z]{3})\s*([A-Z]{3})`)

	// Aircraft type pattern.
	acTypeRe = regexp.MustCompile(`\bAIRCRAFT\s+TYPE\s*:\s*([A-Z0-9-]+)`)
)

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	text := msg.Text

	// Extract weights.
	if m := zfwRe.FindStringSubmatch(text); len(m) > 1 {
		result.ZFW, _ = strconv.Atoi(m[1])
	}
	if m := towRe.FindStringSubmatch(text); len(m) > 1 {
		result.TOW, _ = strconv.Atoi(m[1])
	}
	if m := lawRe.FindStringSubmatch(text); len(m) > 1 {
		result.LAW, _ = strconv.Atoi(m[1])
	}
	if m := tofRe.FindStringSubmatch(text); len(m) > 1 {
		result.TOF, _ = strconv.Atoi(m[1])
	}

	// Extract passenger count.
	if m := ttlRe.FindStringSubmatch(text); len(m) > 1 {
		result.PAX, _ = strconv.Atoi(m[1])
	} else if m := paxRe.FindStringSubmatch(text); len(m) > 1 {
		// Sum up PAX categories if present.
		total := 0
		for i := 1; i < len(m) && m[i] != ""; i++ {
			val, _ := strconv.Atoi(m[i])
			total += val
		}
		result.PAX = total
	}

	// Extract crew.
	if m := crewRe.FindStringSubmatch(text); len(m) > 1 {
		result.Crew = m[1]
	}

	// Extract trim.
	if m := trimRe.FindStringSubmatch(text); len(m) > 1 {
		result.Trim = strings.TrimSpace(m[1])
	}

	// Extract MAC values.
	if m := maczfwRe.FindStringSubmatch(text); len(m) > 1 {
		result.MACZFW = m[1]
	}
	if m := mactowRe.FindStringSubmatch(text); len(m) > 1 {
		result.MACTOW = m[1]
	}

	// Extract edition.
	if m := ednoRe.FindStringSubmatch(text); len(m) > 1 {
		result.Edition = m[1]
	}

	// Extract flight/route.
	if m := flightRouteRe.FindStringSubmatch(text); len(m) > 3 {
		result.Flight = m[1]
		result.Origin = m[2]
		result.Destination = m[3]
	}

	// Extract aircraft type.
	if m := acTypeRe.FindStringSubmatch(text); len(m) > 1 {
		result.AircraftType = strings.TrimSpace(m[1])
	}

	// Only return if we got useful weight data.
	if result.ZFW == 0 && result.TOW == 0 && result.PAX == 0 {
		return nil
	}

	return result
}