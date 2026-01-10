// Package label26 parses Label 26 ETA report messages.
package label26

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/airports"
	"acars_parser/internal/registry"
)

// Result represents an ETA report from label 26 messages.
type Result struct {
	MsgID       int64   `json:"message_id"`
	Timestamp   string  `json:"timestamp"`
	Tail        string  `json:"tail,omitempty"`
	MsgType     string  `json:"msg_type"`
	Format      string  `json:"format,omitempty"`         // ETA01, ETA02, etc.
	FlightNum   string  `json:"flight_num,omitempty"`     // Flight number (e.g., SU0245)
	FlightLevel int     `json:"flight_level,omitempty"`   // Flight level in feet (AFL1866 = 18660 ft)
	ReportTime  string  `json:"report_time,omitempty"`    // Time from /16180720 format
	OriginICAO  string  `json:"origin_icao,omitempty"`    // First 4 chars of UUEEUDYZ
	OriginName  string  `json:"origin_name,omitempty"`    // Airport name from ICAO
	DestICAO    string  `json:"dest_icao,omitempty"`      // Last 4 chars of UUEEUDYZ
	DestName    string  `json:"dest_name,omitempty"`      // Airport name from ICAO
	FuelOnBoard int     `json:"fuel_on_board,omitempty"`  // FUEL value
	Temperature int     `json:"temperature_c,omitempty"`  // TEMP value in Celsius
	WindDir     int     `json:"wind_dir,omitempty"`       // Wind direction in degrees
	WindSpeed   int     `json:"wind_speed_kts,omitempty"` // Wind speed in knots
	Latitude    float64 `json:"latitude,omitempty"`       // LATN/LATS
	Longitude   float64 `json:"longitude,omitempty"`      // LONE/LONW
	ETA         string  `json:"eta,omitempty"`            // ETA time
	Waypoint    string  `json:"waypoint,omitempty"`       // Waypoint identifier
	Altitude    int     `json:"altitude,omitempty"`       // ALT value in feet
}

func (r *Result) Type() string     { return "eta_report" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 26 ETA messages.
type Parser struct{}

var (
	// Match ETA01AFL1866 or ALT01AFL1866 format (with flight level)
	etaAFLRe = regexp.MustCompile(`(?i)^((?:ETA|ALT)\d+)AFL(\d+)`)

	// Match ETA01SU0245 or ALT01SU0245 format (with flight number, 2-3 letter airline code)
	etaFlightRe = regexp.MustCompile(`(?i)^((?:ETA|ALT)\d+)([A-Z]{2,3}\d+)`)

	// Match /16180720 time format (HHMMSSCC)
	timeRe = regexp.MustCompile(`/(\d{8})`)

	// Match UUEEUDYZ (8 consecutive uppercase letters)
	icaoRe = regexp.MustCompile(`(?i)([A-Z]{8})`)

	// Match FUEL 140
	fuelRe = regexp.MustCompile(`(?i)FUEL\s+(\d+)`)

	// Match TEMP- 32 or TEMP -32 or TEMP-32 or TEMP 1 (positive temp)
	tempRe = regexp.MustCompile(`(?i)TEMP\s*(-\s*\d+|\d+)`)

	// Match WDIR266 or WDIR 266
	wdirRe = regexp.MustCompile(`(?i)WDIR\s*(\d{3})(?:\s|\d)`)

	// Match WSPD 36 or WSPD36
	wspdRe = regexp.MustCompile(`(?i)WSPD\s*(\d+)`)

	// Match LATN 55.164 or LATS 55.164 or LATN55.164 (space optional)
	latRe = regexp.MustCompile(`(?i)LAT([NS])\s*([\d.]+)`)

	// Match LONE 38.545 or LONW 38.545 or LONE38.545 (space optional)
	lonRe = regexp.MustCompile(`(?i)LON([EW])\s*([\d.]+)`)

	// Match ETA1013
	etaTimeRe = regexp.MustCompile(`(?i)ETA(\d{4})`)

	// Match ALT 21728
	altRe = regexp.MustCompile(`(?i)ALT\s+(\d+)`)

	// Match waypoint (3-5 letter word that's not a keyword)
	waypointRe = regexp.MustCompile(`\b([A-Z]{3,5})\b`)
)

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "label26" }
func (p *Parser) Labels() []string { return []string{"26"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)
	// Check for ETA or ALT format identifier at start
	return strings.HasPrefix(upper, "ETA0") || strings.HasPrefix(upper, "ETA1") ||
		strings.HasPrefix(upper, "ALT0") || strings.HasPrefix(upper, "ALT1")
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
		MsgType:   "eta_report",
	}

	// Try AFL format first (ETA01AFL010 or ALT01AFL1866)
	if m := etaAFLRe.FindStringSubmatch(upper); m != nil {
		result.Format = m[1]
		if fl, err := strconv.Atoi(m[2]); err == nil {
			result.FlightLevel = fl // AFL010 = FL10, AFL1866 = FL1866
		}
	} else if m := etaFlightRe.FindStringSubmatch(upper); m != nil {
		// Try flight number format (ETA01SU0245)
		result.Format = m[1]
		result.FlightNum = m[2]
	} else {
		// No valid ETA format found
		return nil
	}

	// Parse time
	if m := timeRe.FindStringSubmatch(text); m != nil {
		timeStr := m[1]
		if len(timeStr) == 8 {
			hh := timeStr[0:2]
			mm := timeStr[2:4]
			ss := timeStr[4:6]
			result.ReportTime = hh + ":" + mm + ":" + ss
		}
	}

	// Parse ICAO codes (first 4 = origin, last 4 = dest)
	if m := icaoRe.FindStringSubmatch(upper); m != nil {
		icaoPair := m[1]
		if len(icaoPair) == 8 {
			result.OriginICAO = icaoPair[0:4]
			result.DestICAO = icaoPair[4:8]
			// Lookup airport names
			result.OriginName = airports.GetName(result.OriginICAO)
			result.DestName = airports.GetName(result.DestICAO)
		}
	}

	// Parse fuel
	if m := fuelRe.FindStringSubmatch(upper); m != nil {
		if fuel, err := strconv.Atoi(m[1]); err == nil {
			result.FuelOnBoard = fuel
		}
	}

	// Parse temperature
	if m := tempRe.FindStringSubmatch(upper); m != nil {
		tempStr := strings.ReplaceAll(m[1], " ", "")
		if temp, err := strconv.Atoi(tempStr); err == nil {
			result.Temperature = temp
		}
	}

	// Parse wind direction
	if m := wdirRe.FindStringSubmatch(upper); m != nil {
		if wdir, err := strconv.Atoi(m[1]); err == nil {
			result.WindDir = wdir % 360
		}
	}

	// Parse wind speed
	if m := wspdRe.FindStringSubmatch(upper); m != nil {
		if wspd, err := strconv.Atoi(m[1]); err == nil {
			result.WindSpeed = wspd
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

	// Parse ETA
	if m := etaTimeRe.FindStringSubmatch(upper); m != nil {
		etaStr := m[1]
		if len(etaStr) == 4 {
			hh := etaStr[0:2]
			mm := etaStr[2:4]
			result.ETA = hh + ":" + mm
		}
	}

	// Parse altitude
	if m := altRe.FindStringSubmatch(upper); m != nil {
		if alt, err := strconv.Atoi(m[1]); err == nil {
			result.Altitude = alt
			// Calculate flight level from altitude (altitude / 100)
			result.FlightLevel = alt / 100
		}
	}

	// Parse waypoint (find 3-5 letter word that's not a keyword)
	keywords := map[string]bool{
		"ETA": true, "AFL": true, "FUEL": true, "TEMP": true,
		"WDIR": true, "WSPD": true, "LATN": true, "LATS": true,
		"LONE": true, "LONW": true, "ALT": true, "TUR": true,
	}

	matches := waypointRe.FindAllString(upper, -1)
	for _, match := range matches {
		if !keywords[match] && len(match) >= 3 && len(match) <= 5 {
			// Take the first valid waypoint
			result.Waypoint = match
			break
		}
	}

	return result
}
