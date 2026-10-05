// Package takeoff parses aircraft takeoff performance data messages.
package takeoff

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// RunwayData contains takeoff parameters for a specific runway.
type RunwayData struct {
	Airport   string  `json:"airport,omitempty"`
	Runway    string  `json:"runway"`
	Length    int     `json:"length,omitempty"`     // Runway length in feet
	Shift     int     `json:"shift,omitempty"`      // Displaced threshold
	Flaps     int     `json:"flaps,omitempty"`      // Flap setting
	EPR       float64 `json:"epr,omitempty"`        // Engine pressure ratio
	MRTW      float64 `json:"mrtw,omitempty"`       // Max recommended takeoff weight
	LimitCode string  `json:"limit_code,omitempty"` // O=obstacle, F=field, etc.
	V1        int     `json:"v1,omitempty"`         // Decision speed
	VR        int     `json:"vr,omitempty"`         // Rotation speed
	V2        int     `json:"v2,omitempty"`         // Takeoff safety speed
	FlexTemp  int     `json:"flex_temp,omitempty"`  // Flex/assumed temp
	FlexEPR   float64 `json:"flex_epr,omitempty"`   // Flex EPR
	MTOW      float64 `json:"mtow,omitempty"`       // Max takeoff weight
}

// Result represents parsed takeoff performance data.
type Result struct {
	MsgID        int64        `json:"message_id,omitempty"`
	FlightNumber string       `json:"flight_number,omitempty"`
	Origin       string       `json:"origin,omitempty"`
	Destination  string       `json:"destination,omitempty"`
	Tail         string       `json:"tail,omitempty"`
	AircraftType string       `json:"aircraft_type,omitempty"`
	EngineType   string       `json:"engine_type,omitempty"`
	Time         string       `json:"time,omitempty"`
	Wind         string       `json:"wind,omitempty"`
	OAT          int          `json:"oat,omitempty"`   // Outside air temp (C)
	QNH          float64      `json:"qnh,omitempty"`   // Altimeter setting
	GTOW         float64      `json:"gtow,omitempty"`  // Gross takeoff weight (klbs)
	CG           float64      `json:"cg,omitempty"`    // Centre of gravity (%)
	PAX          int          `json:"pax,omitempty"`   // Passenger count
	Fuel         float64      `json:"fuel,omitempty"`  // Fuel (klbs)
	Cargo        int          `json:"cargo,omitempty"` // Cargo weight (lbs)
	ZFW          float64      `json:"zfw,omitempty"`   // Zero fuel weight (klbs)
	Runways      []RunwayData `json:"runways,omitempty"`
	Remarks      string       `json:"remarks,omitempty"`
}

func (r *Result) Type() string     { return "takeoff_data" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// Aircraft model and engine: "A320-232 V2527-A5", "777-300ER GE90-115BL".
	acTypeRe = patterns.ModelEngineLinePattern

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

	// GTOW/CG and PAX from the tabular format:
	// GTOW /CG             PAX
	// 409.3/25.1           226
	gtowPaxRe = regexp.MustCompile(`GTOW\s*/\s*CG\s+PAX\s*\n\s*([\d.]+)/([\d.]+)\s+(\d+)`)

	// FUEL and CARGO from the tabular format:
	// FUEL               CARGO
	//  82.9              11878
	fuelCargoRe = regexp.MustCompile(`FUEL\s+CARGO\s*\n\s*([\d.]+)\s+(\d+)`)

	// QNH formats, tried in order by Parse.
	qnhPatterns = []*regexp.Regexp{
		regexp.MustCompile(`QNH\s*\n[^\n]*\s(\d+\.\d+)`), // QNH on header, value on next line
		regexp.MustCompile(`ALT\s+(\d+\.\d+)`),           // ALT 30.09
		regexp.MustCompile(`(\d{2}\.\d{2})\s*$`),         // At end of line
	}

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

	// Simple runway format: T/O SFO 01R
	simpleRunwayRe = regexp.MustCompile(`T/O\s+([A-Z]{3,4})\s+(\d{2}[LRC]?)`)
)

// Envoy's layout. Its header lines give the flight number, day, route and
// time ("3845/19  KORD-KVPS 1523Z") and the fleet number and tail
// ("337/N337MR   DISP RLS  1"); the weather line gives the wind, the
// temperature and the altimeter in inches ("WX 284/16   -18C   A3012"). Each
// runway block starts with the airport, runway and length ("KORD 22L
// TORA  8075"; a letter after the runway, as in "28RZ", is not part of it)
// and holds that runway's speeds and flap setting. The generic patterns
// above are not used for this layout: they match inside its other fields
// (windRe would read "845/19" from the header line).
var (
	envoyHeaderRe = regexp.MustCompile(`(?m)^[ \t]*(\d{1,4})/\d{2}[ \t]+([A-Z]{4})-([A-Z]{4})[ \t]+(\d{4}Z)[ \t]*\r?$`)
	envoyTailRe   = regexp.MustCompile(`(?m)^[ \t]*\d{1,4}/([A-Z0-9-]{3,8})[ \t]+DISP RLS\b`)
	envoyFlightRe = regexp.MustCompile(`(?m)^[ \t]*AN [A-Z0-9-]+/FI ([A-Z0-9]{2})(\d{1,4})(?:/|[ \t]*\r?$)`)
	envoyWxRe     = regexp.MustCompile(`(?m)^[ \t]*WX[ \t]+(\d{3}/\d{1,3})[ \t]+(-?\d{1,2})C[ \t]+A(\d{2})(\d{2})\b`)
	envoyGTOWRe   = regexp.MustCompile(`(?m)^[ \t]*GTOW[ \t]+(\d+)[ \t]`)
	envoyRunwayRe = regexp.MustCompile(`(?m)^[ \t]*([A-Z]{4})[ \t]+(\d{2}[LRC]?)[A-Z]?[ \t]+TORA[ \t]+(\d+)[ \t]*\r?$`)
	envoyV1Re     = regexp.MustCompile(`\bV1[ \t]+(\d{2,3})\b`)
	envoyVRRe     = regexp.MustCompile(`\bVR[ \t]+(\d{2,3})\b`)
	envoyV2Re     = regexp.MustCompile(`\bV2[ \t]+(\d{2,3})\b`)
	envoyFlapRe   = regexp.MustCompile(`\bFLAP[ \t]+(\d{1,2})\b`)
)

// Parser parses takeoff performance data messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "takeoff_data" }
func (p *Parser) Labels() []string { return []string{"RA", "H1", "C1"} }
func (p *Parser) Priority() int    { return 55 }

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

	if m := envoyHeaderRe.FindStringSubmatchIndex(text); m != nil {
		return parseEnvoy(msg, text, m)
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
	for _, re := range qnhPatterns {
		if m := re.FindStringSubmatch(text); m != nil {
			result.QNH, _ = strconv.ParseFloat(m[1], 64)
			break
		}
	}

	// Parse GTOW/CG and PAX.
	if m := gtowPaxRe.FindStringSubmatch(text); m != nil {
		result.GTOW, _ = strconv.ParseFloat(m[1], 64)
		result.CG, _ = strconv.ParseFloat(m[2], 64)
		result.PAX, _ = strconv.Atoi(m[3])
	}

	// Parse FUEL and CARGO.
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

	// Parse V speeds (apply to the last runway, if any).
	// TODO: VR, V2, flex temperature, flaps, EPR, MRTW and the limit code are
	// declared on RunwayData but never populated. vrRe, v2Re and flexRe are only
	// used by ParseWithTrace. Populating them needs more sample messages to confirm
	// which runway column each value belongs to in the tabular format.
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

// parseEnvoy parses Envoy's layout (see envoyHeaderRe). header is the
// submatch index of envoyHeaderRe in text.
func parseEnvoy(msg *acars.Message, text string, header []int) *Result {
	group := func(i int) string { return text[header[2*i]:header[2*i+1]] }
	digits := group(1)
	result := &Result{
		MsgID:       int64(msg.ID),
		Origin:      group(2),
		Destination: group(3),
		Time:        group(4),
	}

	// The envelope's flight ("AN N337MR/FI MQ3845") is taken only when its
	// number is the header's, so that the flight is confirmed by both.
	if m := envoyFlightRe.FindStringSubmatch(text); m != nil && strings.TrimLeft(m[2], "0") == strings.TrimLeft(digits, "0") {
		result.FlightNumber = m[1] + m[2]
	}
	if m := envoyTailRe.FindStringSubmatch(text); m != nil {
		result.Tail = m[1]
	}
	if m := envoyWxRe.FindStringSubmatch(text); m != nil {
		result.Wind = m[1]
		result.OAT, _ = strconv.Atoi(m[2])
		result.QNH, _ = strconv.ParseFloat(m[3]+"."+m[4], 64)
	}
	if m := envoyGTOWRe.FindStringSubmatch(text); m != nil {
		result.GTOW, _ = strconv.ParseFloat(m[1], 64)
	}

	// Each runway block runs to the next block's header or the end.
	blocks := envoyRunwayRe.FindAllStringSubmatchIndex(text, -1)
	for i, b := range blocks {
		end := len(text)
		if i+1 < len(blocks) {
			end = blocks[i+1][0]
		}
		body := text[b[1]:end]
		rwy := RunwayData{Airport: text[b[2]:b[3]], Runway: text[b[4]:b[5]]}
		rwy.Length, _ = strconv.Atoi(text[b[6]:b[7]])
		rwy.V1 = firstInt(envoyV1Re, body)
		rwy.VR = firstInt(envoyVRRe, body)
		rwy.V2 = firstInt(envoyV2Re, body)
		rwy.Flaps = firstInt(envoyFlapRe, body)
		result.Runways = append(result.Runways, rwy)
	}
	return result
}

// firstInt returns the first submatch of re in s as an integer, or zero.
func firstInt(re *regexp.Regexp, s string) int {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
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
