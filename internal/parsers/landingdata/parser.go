// Package landingdata parses aircraft landing performance data messages from ACARS.
package landingdata

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Result represents parsed landing data.
type Result struct {
	MsgID            int64   `json:"message_id"`
	Timestamp        string  `json:"timestamp"`
	Tail             string  `json:"tail,omitempty"`
	Airport          string  `json:"airport,omitempty"`
	Runway           string  `json:"runway,omitempty"`
	RunwayLength     int     `json:"runway_length,omitempty"`
	AircraftType     string  `json:"aircraft_type,omitempty"`
	FlapSetting      string  `json:"flap_setting,omitempty"`
	Temperature      int     `json:"temperature,omitempty"`
	Altimeter        float64 `json:"altimeter,omitempty"`
	Wind             string  `json:"wind,omitempty"`
	LandingWeight    float64 `json:"landing_weight,omitempty"`
	StructuralLimit  float64 `json:"structural_limit,omitempty"`
	PerformanceLimit float64 `json:"performance_limit,omitempty"`
	RunwayCondition  string  `json:"runway_condition,omitempty"`
}

func (r *Result) Type() string     { return "landing_data" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses landing data messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "landingdata" }
func (p *Parser) Labels() []string { return []string{"C1"} }
func (p *Parser) Priority() int    { return 70 } // High priority for specific messages.

// QuickCheck looks for landing data keywords.
func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)
	return strings.Contains(upper, "LANDING DATA")
}

// Pattern matchers.
var (
	// Airport/runway pattern: LANDING DATA HNL RW 08L
	airportRwyRe = regexp.MustCompile(`LANDING DATA\s+([A-Z]{3,4})\s+RW\s*(\d{2}[LRC]?)`)

	// Runway length: 12245 FT
	rwyLengthRe = regexp.MustCompile(`(\d{4,5})\s*FT`)

	// Aircraft type: 777-200 PW4077 or 787-10 GENX-1B76
	acTypeRe = regexp.MustCompile(`\n\s*([A-Z0-9]{3}-\d+)\s+([A-Z0-9-]+)`)

	// Flap setting: *FLAPS 30*
	flapRe = regexp.MustCompile(`\*FLAPS?\s*(\d+)\*`)

	// Temperature and altimeter: TEMP 25C       ALT 29.94
	tempAltRe = regexp.MustCompile(`TEMP\s+(M?\d+)C\s+ALT\s+([\d.]+)`)

	// Wind: WIND 089/5 MAG
	windRe = regexp.MustCompile(`WIND\s+(\d{3}/\d+)\s*(?:MAG)?`)

	// Landing weight: 421.6 - PLANNED LDG WT
	ldgWtRe = regexp.MustCompile(`([\d.]+)\s*-\s*PLANNED\s+LDG\s+WT`)

	// Structural limit: 445.0 - STRUCTURAL
	structRe = regexp.MustCompile(`([\d.]+)\s*-\s*STRUCTURAL`)

	// Performance limit: 580.0 - LM
	perfRe = regexp.MustCompile(`([\d.]+)\s*-\s*LM\s`)

	// Runway condition: RWY DRY or RWY WET etc.
	rwyCondRe = regexp.MustCompile(`\bRWY\s+(DRY|WET|CONTAMINATED|SLIPPERY)\b`)
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

	// Extract airport and runway.
	if m := airportRwyRe.FindStringSubmatch(text); len(m) > 2 {
		result.Airport = m[1]
		result.Runway = m[2]
	}

	// Extract runway length.
	if m := rwyLengthRe.FindStringSubmatch(text); len(m) > 1 {
		result.RunwayLength, _ = strconv.Atoi(m[1])
	}

	// Extract aircraft type.
	if m := acTypeRe.FindStringSubmatch(text); len(m) > 2 {
		result.AircraftType = m[1] + " " + m[2]
	}

	// Extract flap setting.
	if m := flapRe.FindStringSubmatch(text); len(m) > 1 {
		result.FlapSetting = m[1]
	}

	// Extract temperature and altimeter.
	if m := tempAltRe.FindStringSubmatch(text); len(m) > 2 {
		tempStr := m[1]
		if strings.HasPrefix(tempStr, "M") {
			temp, _ := strconv.Atoi(strings.TrimPrefix(tempStr, "M"))
			result.Temperature = -temp
		} else {
			result.Temperature, _ = strconv.Atoi(tempStr)
		}
		result.Altimeter, _ = strconv.ParseFloat(m[2], 64)
	}

	// Extract wind.
	if m := windRe.FindStringSubmatch(text); len(m) > 1 {
		result.Wind = m[1]
	}

	// Extract landing weight.
	if m := ldgWtRe.FindStringSubmatch(text); len(m) > 1 {
		result.LandingWeight, _ = strconv.ParseFloat(m[1], 64)
	}

	// Extract structural limit.
	if m := structRe.FindStringSubmatch(text); len(m) > 1 {
		result.StructuralLimit, _ = strconv.ParseFloat(m[1], 64)
	}

	// Extract performance limit.
	if m := perfRe.FindStringSubmatch(text); len(m) > 1 {
		result.PerformanceLimit, _ = strconv.ParseFloat(m[1], 64)
	}

	// Extract runway condition.
	if m := rwyCondRe.FindStringSubmatch(text); len(m) > 1 {
		result.RunwayCondition = m[1]
	}

	// Only return if we got useful data.
	if result.Airport == "" && result.Runway == "" && result.LandingWeight == 0 {
		return nil
	}

	return result
}