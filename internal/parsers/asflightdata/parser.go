// Package asflightdata parses Alaska Airlines flight data reports (label
// H1): the route and a series of samples of position, altitude, outside air
// temperature and wind. For example:
//
//	D3M207KSANKATLN33216W11201120073500M052215102G0009
//	N33232W1115623500M052214100G0009
//	-0016
//
// The first line is "D3M", three digits, the origin and destination, and
// the first sample, which alone carries a time (HHMM). A sample is the
// latitude (degrees and minutes to a tenth), the longitude, the altitude in
// tens of feet, the temperature ("M" minus or "P" plus, in degrees Celsius),
// the wind direction and speed (knots), and a letter and four digits. The
// report ends with a line such as "-0016".
//
// The meanings were established from the January 2026 corpus: the
// temperature falls by 1.9 °C per 1,000 ft (the standard atmosphere gives
// 2.0), and the wind speed rises with altitude (median 17 kt below 10,000
// ft, 72 kt above 30,000 ft) and is mostly westerly. The three digits after
// "D3M" are not the flight number (none of 368 matched the flight) and, like
// the letter, the four digits after it and the last line, are not captured.
package asflightdata

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Sample is one sample of a report.
type Sample struct {
	Latitude      float64 `json:"lat"`
	Longitude     float64 `json:"lon"`
	AltitudeFt    int     `json:"altitude_ft"`
	TemperatureC  int     `json:"temperature_c"` // The outside air temperature.
	WindDirection int     `json:"wind_direction"`
	WindSpeedKt   int     `json:"wind_speed_kt"`
}

// Result is an Alaska Airlines flight data report.
type Result struct {
	MsgID       int64    `json:"message_id"`
	Timestamp   string   `json:"timestamp"`
	Origin      string   `json:"origin"`
	Destination string   `json:"destination"`
	Time        string   `json:"time"` // HHMM, of the first sample.
	Samples     []Sample `json:"samples"`
}

func (r *Result) Type() string     { return "alaska_flight_data" }
func (r *Result) MessageID() int64 { return r.MsgID }

const sampleFields = `([NS])(\d{2})(\d{3})([EW])(\d{3})(\d{3})`

var (
	// headerRe matches the first line: the route, the time and the first
	// sample's remaining fields.
	headerRe = regexp.MustCompile(`^D3M\d{3}([A-Z]{4})([A-Z]{4})` + sampleFields + `(\d{4})(\d{4})([MP])(\d{3})(\d{3})(\d{3})[A-Z]\d{4}$`)

	// sampleRe matches the following samples, which have no time.
	sampleRe = regexp.MustCompile(`^` + sampleFields + `(\d{4})([MP])(\d{3})(\d{3})(\d{3})[A-Z]\d{4}$`)
)

// Parser parses Alaska Airlines flight data reports.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "asflightdata" }
func (p *Parser) Labels() []string { return []string{"H1"} }
func (p *Parser) Priority() int    { return 60 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "D3M")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(msg.Text, "\r", ""), "\n")
	h := headerRe.FindStringSubmatch(lines[0])
	if h == nil || !patterns.IsValidICAO(h[1]) || !patterns.IsValidICAO(h[2]) {
		return nil
	}
	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Origin:      h[1],
		Destination: h[2],
		Time:        h[9],
	}
	// The first sample carries the report's only time; a report whose
	// first sample or time is invalid is not parsed.
	s, ok := sample(h[3:9], h[10], h[11], h[12], h[13], h[14])
	if !ok || !validTime(h[9]) {
		return nil
	}
	result.Samples = append(result.Samples, s)
	for _, line := range lines[1:] {
		if m := sampleRe.FindStringSubmatch(line); m != nil {
			if s, ok := sample(m[1:7], m[7], m[8], m[9], m[10], m[11]); ok {
				result.Samples = append(result.Samples, s)
			}
		}
	}
	return result
}

// sample builds a sample from its position fields (hemisphere, degrees and
// tenths of minutes for latitude, then for longitude) and its other fields,
// and returns false if the position or wind is out of range.
func sample(pos []string, alt, sign, temp, dir, speed string) (Sample, bool) {
	// Minutes are given in tenths, so 600 or more is not a valid value.
	if latMin, _ := strconv.Atoi(pos[2]); latMin >= 600 {
		return Sample{}, false
	}
	if lonMin, _ := strconv.Atoi(pos[5]); lonMin >= 600 {
		return Sample{}, false
	}
	s := Sample{
		Latitude:  degrees(pos[1], pos[2], pos[0] == "S"),
		Longitude: degrees(pos[4], pos[5], pos[3] == "W"),
	}
	a, _ := strconv.Atoi(alt)
	s.AltitudeFt = a * 10
	s.TemperatureC, _ = strconv.Atoi(temp)
	if sign == "M" {
		s.TemperatureC = -s.TemperatureC
	}
	s.WindDirection, _ = strconv.Atoi(dir)
	s.WindSpeedKt, _ = strconv.Atoi(speed)
	if s.Latitude < -90 || s.Latitude > 90 || s.Longitude < -180 || s.Longitude > 180 || s.WindDirection > 360 {
		return Sample{}, false
	}
	return s, true
}

// validTime reports whether an HHMM time is valid.
func validTime(hhmm string) bool {
	h, _ := strconv.Atoi(hhmm[:2])
	m, _ := strconv.Atoi(hhmm[2:])
	return h < 24 && m < 60
}

// degrees converts degrees and minutes in tenths ("216" is 21.6 minutes) to
// decimal degrees.
func degrees(deg, tenths string, negative bool) float64 {
	d, _ := strconv.ParseFloat(deg, 64)
	m, _ := strconv.ParseFloat(tenths, 64)
	v := d + m/600
	if negative {
		v = -v
	}
	return v
}
