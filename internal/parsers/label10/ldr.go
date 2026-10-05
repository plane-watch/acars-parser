package label10

import (
	"regexp"
	"strconv"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// LDRResult is the start of Southwest's label 10 LDR report: the
// aircraft's position and altitude, and the route.
type LDRResult struct {
	MsgID       int64   `json:"message_id"`
	Timestamp   string  `json:"timestamp"`
	Report      string  `json:"report"` // e.g. LDR01.
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Altitude    int     `json:"altitude"` // Feet.
	Origin      string  `json:"origin"`
	Destination string  `json:"destination"`
}

func (r *LDRResult) Type() string     { return "ldr_report" }
func (r *LDRResult) MessageID() int64 { return r.MsgID }

// ldrRe matches the start of an LDR report, for example:
//
//	LDR01,189,D,SWA-2600-019,0,N 35.194,W119.735,34995,  9.4,KSFO,KLAX,KLAX,24R/,25L/,/,...
//
// After the report name come three fields that are not parsed, a system
// identifier (SWA-2600-019), a field not parsed, the latitude and longitude
// in decimal degrees, the altitude in feet, a number not parsed, the
// origin and the destination. The next field (the landing airport, the
// same as the destination in the archive) and the runways after it are not
// parsed: their meanings are not established.
var ldrRe = regexp.MustCompile(`^(LDR\d{2}),[^,]*,[^,]*,[A-Z0-9-]+,[^,]*,([NS]) ?(\d{1,2}\.\d+),([EW]) ?(\d{1,3}\.\d+),(\d+),[^,]*,([A-Z]{4}),([A-Z]{4}),`)

// LDRParser parses Southwest's LDR reports.
type LDRParser struct{}

func init() {
	registry.Register(&LDRParser{})
}

func (p *LDRParser) Name() string     { return "ldr" }
func (p *LDRParser) Labels() []string { return []string{"10"} }
func (p *LDRParser) Priority() int    { return 100 }

func (p *LDRParser) QuickCheck(text string) bool {
	return len(text) > 3 && text[:3] == "LDR"
}

func (p *LDRParser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	m := ldrRe.FindStringSubmatch(msg.Text)
	if m == nil || !patterns.IsValidICAO(m[7]) || !patterns.IsValidICAO(m[8]) {
		return nil
	}
	lat, errLat := strconv.ParseFloat(m[3], 64)
	lon, errLon := strconv.ParseFloat(m[5], 64)
	alt, errAlt := strconv.Atoi(m[6])
	if errLat != nil || errLon != nil || errAlt != nil || lat > 90 || lon > 180 {
		return nil
	}
	if m[2] == "S" {
		lat = -lat
	}
	if m[4] == "W" {
		lon = -lon
	}
	return &LDRResult{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Report:      m[1],
		Latitude:    lat,
		Longitude:   lon,
		Altitude:    alt,
		Origin:      m[7],
		Destination: m[8],
	}
}
