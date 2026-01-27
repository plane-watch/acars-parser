// Package takeoff parses aircraft takeoff performance data messages.
package takeoff

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// RunwayData contains takeoff parameters for a specific runway.
type RunwayData struct {
	Airport    string  `json:"airport,omitempty"`
	Runway     string  `json:"runway"`
	Length     int     `json:"length,omitempty"`       // Runway length in feet
	Shift      int     `json:"shift,omitempty"`        // Displaced threshold
	Flaps      int     `json:"flaps,omitempty"`        // Flap setting
	EPR        float64 `json:"epr,omitempty"`          // Engine pressure ratio
	MRTW       float64 `json:"mrtw,omitempty"`         // Max recommended takeoff weight
	LimitCode  string  `json:"limit_code,omitempty"`   // O=obstacle, F=field, etc.
	V1         int     `json:"v1,omitempty"`           // Decision speed
	VR         int     `json:"vr,omitempty"`           // Rotation speed
	V2         int     `json:"v2,omitempty"`           // Takeoff safety speed
	FlexTemp   int     `json:"flex_temp,omitempty"`    // Flex/assumed temp
	FlexEPR    float64 `json:"flex_epr,omitempty"`     // Flex EPR
	MTOW       float64 `json:"mtow,omitempty"`         // Max takeoff weight
}

// Result represents parsed takeoff performance data.
type Result struct {
	MsgID        int64        `json:"message_id,omitempty"`
	FlightNumber string       `json:"flight_number,omitempty"`
	AircraftType string       `json:"aircraft_type,omitempty"`
	EngineType   string       `json:"engine_type,omitempty"`
	Time         string       `json:"time,omitempty"`
	Wind         string       `json:"wind,omitempty"`
	OAT          int          `json:"oat,omitempty"`          // Outside air temp (C)
	QNH          float64      `json:"qnh,omitempty"`          // Altimeter setting
	GTOW         float64      `json:"gtow,omitempty"`         // Gross takeoff weight (klbs)
	CG           float64      `json:"cg,omitempty"`           // Centre of gravity (%)
	PAX          int          `json:"pax,omitempty"`          // Passenger count
	Fuel         float64      `json:"fuel,omitempty"`         // Fuel (klbs)
	Cargo        int          `json:"cargo,omitempty"`        // Cargo weight (lbs)
	ZFW          float64      `json:"zfw,omitempty"`          // Zero fuel weight (klbs)
	Runways      []RunwayData `json:"runways,omitempty"`
	Remarks      string       `json:"remarks,omitempty"`
}

func (r *Result) Type() string     { return "takeoff_data" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// Aircraft type: A320-232 V2527-A5
	acTypeRe = regexp.MustCompile(`([AB]\d{3}-\d{3})\s+([A-Z0-9-]+)`)

	// Time: 1808Z
	timeRe = regexp.MustCompile(`(\d{4}Z)`)

	// Wind: 000/00 or 346/10 - on same or next line after WIND header
	windRe = regexp.MustCompile(`(\d{3}/\d+)`)

	// OAT: temperature value after the wind on same line or from TEMP header
	oatRe = regexp.MustCompile(`(?:TEMP|OAT)[^\d-]*([+-]?\d+)\s*C`)

	// QNH: 30.15 or similar altimeter setting
	qnhRe = regexp.MustCompile(`(?:QNH|ALT)[^\d]*(\d+\.\d+)`)

	// GTOW/CG: 409.3/25.1 - on line after header
	gtowRe = regexp.MustCompile(`([\d.]+)/([\d.]+)\s+\d+\s*\n`)

	// PAX: 226 - at end of line with GTOW
	paxRe = regexp.MustCompile(`PAX\s*\n\s*([\d.]+)/([\d.]+)\s+(\d+)`)

	// FUEL: 82.9 - on line after FUEL header
	fuelRe = regexp.MustCompile(`FUEL\s+CARGO\s*\n\s*([\d.]+)\s+(\d+)`)

	// CARGO: extracted from fuelRe above
	cargoRe = regexp.MustCompile(`CARGO\s*\n?\s*(\d+)`)

	// ZFW/CG: 326.4/26.6
	zfwRe = regexp.MustCompile(`ZFW\s*/\s*CG\s*\n?\s*([\d.]+)`)

	// Runway header: KPDX 10R or LENGTH KPDX
	runwayHeaderRe = regexp.MustCompile(`LENGTH\s+([A-Z]{4})\s+SHIFT\s*\n?\s*(\d+)\s+(\d+[LRC]?)\s+(\d+)`)

	// V speeds: V1 144, VR 146, V2 150
	v1Re = regexp.MustCompile(`V1\s*\n?\s*(\d+)`)
	vrRe = regexp.MustCompile(`VR\s*\n?\s*(\d+)`)
	v2Re = regexp.MustCompile(`V2\s*\n?\s*(\d+)`)

	// FLEX: 69
	flexRe = regexp.MustCompile(`FLEX\s+MAX\s*\n?\s*(\d+)`)

	// FLAP EPR MRTW: 2 1.39 411.7/O
	flapLineRe = regexp.MustCompile(`FLAP\s+EPR\s+MRTW/LIM\s+V1\s*\n?\s*(\d+)\s+([\d.]+)\s+([\d.]+)/([A-Z])`)

	// Simple runway format: T/O SFO 01R
	simpleRunwayRe = regexp.MustCompile(`T/O\s+([A-Z]{3,4})\s+(\d{2}[LRC]?)`)
)

// Parser parses takeoff performance data messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string           { return "takeoff_data" }
func (p *Parser) Labels() []string       { return []string{"RA", "H1", "C1"} }
func (p *Parser) Priority() int          { return 55 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "TAKEOFF DATA") || strings.Contains(text, "T/O DATA")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := msg.Text

	if !p.QuickCheck(text) {
		return nil
	}

	result := &Result{
		MsgID: int64(msg.ID),
	}

	// Parse aircraft type and engine.
	if m := acTypeRe.FindStringSubmatch(text); m != nil {
		result.AircraftType = m[1]
		result.EngineType = m[2]
	}

	// Parse time.
	if m := timeRe.FindStringSubmatch(text); m != nil {
		result.Time = m[1]
	}

	// Parse wind - look for pattern like 000/00 or 346/10
	if m := windRe.FindStringSubmatch(text); m != nil {
		result.Wind = m[1]
	}

	// Parse OAT - multiple formats
	if m := oatRe.FindStringSubmatch(text); m != nil {
		result.OAT, _ = strconv.Atoi(m[1])
	}

	// Parse QNH - multiple formats (look for decimal number like 30.15 or 29.92)
	qnhPatterns := []*regexp.Regexp{
		regexp.MustCompile(`QNH\s*\n[^\n]*\s(\d+\.\d+)`),           // QNH on header, value on next line
		regexp.MustCompile(`ALT\s+(\d+\.\d+)`),                      // ALT 30.09
		regexp.MustCompile(`(\d{2}\.\d{2})\s*$`),                    // At end of line
	}
	for _, re := range qnhPatterns {
		if m := re.FindStringSubmatch(text); m != nil {
			result.QNH, _ = strconv.ParseFloat(m[1], 64)
			break
		}
	}

	// Parse GTOW/CG and PAX from the tabular format:
	// GTOW /CG             PAX
	// 409.3/25.1           226
	gtowPaxRe := regexp.MustCompile(`GTOW\s*/\s*CG\s+PAX\s*\n\s*([\d.]+)/([\d.]+)\s+(\d+)`)
	if m := gtowPaxRe.FindStringSubmatch(text); m != nil {
		result.GTOW, _ = strconv.ParseFloat(m[1], 64)
		result.CG, _ = strconv.ParseFloat(m[2], 64)
		result.PAX, _ = strconv.Atoi(m[3])
	}

	// Parse FUEL and CARGO from tabular format:
	// FUEL               CARGO
	//  82.9              11878
	fuelCargoRe := regexp.MustCompile(`FUEL\s+CARGO\s*\n\s*([\d.]+)\s+(\d+)`)
	if m := fuelCargoRe.FindStringSubmatch(text); m != nil {
		result.Fuel, _ = strconv.ParseFloat(m[1], 64)
		result.Cargo, _ = strconv.Atoi(m[2])
	}

	// Parse ZFW.
	if m := zfwRe.FindStringSubmatch(text); m != nil {
		result.ZFW, _ = strconv.ParseFloat(m[1], 64)
	}

	// Parse runway data.
	runwayMatches := runwayHeaderRe.FindAllStringSubmatch(text, -1)
	for _, m := range runwayMatches {
		rwy := RunwayData{
			Airport: m[1],
			Runway:  m[3],
		}
		rwy.Length, _ = strconv.Atoi(m[2])
		rwy.Shift, _ = strconv.Atoi(m[4])

		result.Runways = append(result.Runways, rwy)
	}

	// Try simple runway format if no complex data.
	if len(result.Runways) == 0 {
		if m := simpleRunwayRe.FindStringSubmatch(text); m != nil {
			result.Runways = append(result.Runways, RunwayData{
				Airport: m[1],
				Runway:  m[2],
			})
		}
	}

	// Parse V speeds (apply to last runway or create default).
	if m := v1Re.FindStringSubmatch(text); m != nil {
		v1, _ := strconv.Atoi(m[1])
		if len(result.Runways) > 0 {
			result.Runways[len(result.Runways)-1].V1 = v1
		}
	}

	// Must have some meaningful data.
	if result.GTOW == 0 && len(result.Runways) == 0 {
		return nil
	}

	return result
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
		trace.QuickCheck.Reason = "No TAKEOFF DATA or T/O DATA keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for key patterns.
	extractors := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"aircraft_type", acTypeRe},
		{"time", timeRe},
		{"wind", windRe},
		{"oat", oatRe},
		{"qnh", qnhRe},
		{"zfw", zfwRe},
		{"runway_header", runwayHeaderRe},
		{"simple_runway", simpleRunwayRe},
		{"v1", v1Re},
		{"vr", vrRe},
		{"v2", v2Re},
		{"flex", flexRe},
	}

	for _, e := range extractors {
		ext := registry.Extractor{
			Name:    e.name,
			Pattern: e.pattern.String(),
		}
		if m := e.pattern.FindStringSubmatch(text); len(m) > 1 {
			ext.Matched = true
			ext.Value = m[1]
		}
		trace.Extractors = append(trace.Extractors, ext)
	}

	// Determine if overall match succeeds.
	hasGTOW := gtowRe.MatchString(text) || strings.Contains(text, "GTOW")
	hasRunway := runwayHeaderRe.MatchString(text) || simpleRunwayRe.MatchString(text)
	trace.Matched = hasGTOW || hasRunway

	return trace
}