// Package h2wind parses H2 wind/weather messages.
package h2wind

import (
	"regexp"
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

// WindLayer represents wind data at a specific flight level.
type WindLayer struct {
	FlightLevel int  `json:"flight_level"`
	Temperature int  `json:"temperature"` // Celsius (could be SAT or ISA deviation)
	WindDir     int  `json:"wind_dir,omitempty"`
	WindSpeed   int  `json:"wind_speed,omitempty"`
	Gusting     bool `json:"-"`
}

// WindPoint represents wind/weather at a specific (lat,lon) along a route.
//
// H2 examples:
//
//	N42540E0185553400M485278094G
//	QN44191E02136210483802M517273066G
type WindPoint struct {
	Marker      string  `json:"marker,omitempty"` // "N" or "Q" (when present in raw)
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	ETA         string  `json:"eta,omitempty"`          // HH:MM
	FlightLevel float64 `json:"flight_level,omitempty"` // e.g. 340.0
	Temperature float64 `json:"temperature,omitempty"`  // Celsius (can include tenths)
	WindDir     int     `json:"wind_dir,omitempty"`
	WindSpeed   int     `json:"wind_speed,omitempty"`
	Gusting     bool    `json:"-"`
}

// Result represents a parsed H2 wind/weather report.
type Result struct {
	MsgID           int64       `json:"message_id"`
	Timestamp       string      `json:"timestamp"`
	Tail            string      `json:"tail,omitempty"`
	Phase           string      `json:"phase,omitempty"` // 02A climb/descend, 02E cruise
	Day             int         `json:"day,omitempty"`   // only for 02E (2-digit day)
	Origin          string      `json:"origin,omitempty"`
	Destination     string      `json:"destination,omitempty"`
	OriginName      string      `json:"origin_name,omitempty"`
	DestinationName string      `json:"destination_name,omitempty"`
	Latitude        float64     `json:"latitude,omitempty"`
	Longitude       float64     `json:"longitude,omitempty"`
	ReportTime      string      `json:"report_time,omitempty"`
	WindLayers      []WindLayer `json:"wind_layers,omitempty"`
	Points          []WindPoint `json:"points,omitempty"`
	RawData         string      `json:"raw_data,omitempty"`
}

func (r *Result) Type() string     { return "h2_wind" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses H2 wind/weather messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "h2_wind" }
func (p *Parser) Labels() []string { return []string{"H2"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	text = strings.TrimSpace(text)
	return (strings.HasPrefix(text, "02A") || strings.HasPrefix(text, "02E")) && len(text) > 25
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	text := strings.TrimSpace(msg.Text)

	// Check for encoded/binary messages (not parseable).
	if !(strings.HasPrefix(text, "02A") || strings.HasPrefix(text, "02E")) {
		return nil
	}

	match := compiler.Parse(text)
	if match == nil || (match.FormatName != "h2_header_02A" && match.FormatName != "h2_header_02E") {
		return nil
	}

	result := &Result{
		MsgID:           int64(msg.ID),
		Timestamp:       msg.Timestamp,
		Tail:            msg.Tail,
		Origin:          match.Captures["origin"],
		Destination:     match.Captures["dest"],
		OriginName:      airports.GetName(match.Captures["origin"]),
		DestinationName: airports.GetName(match.Captures["dest"]),
		ReportTime:      match.Captures["time"],
	}

	// Parse lat/lon. For H2, the common encoding is degrees * 1000
	// (e.g. 42540 -> 42.540). This matches the examples in your logs.
	result.Latitude = parseDegThousandths(match.Captures["lat"], match.Captures["lat_dir"])
	result.Longitude = parseDegThousandths(match.Captures["lon"], match.Captures["lon_dir"])

	// Branch by header flavor.
	switch match.FormatName {
	case "h2_header_02A":
		result.Phase = "02A"
		// 02A header length: 02A + time(6) + origin(4) + dest(4) + lat_dir(1) + lat(5) + lon_dir(1) + lon(6) + datetime(6) = 36
		headerLen := 36
		if headerLen < len(text) {
			rest := text[headerLen:]
			result.RawData = strings.TrimSpace(rest)
			// Old-style vertical layers (kept for backwards compatibility)
			result.WindLayers = parseWindLayers(compiler, rest)
			// Newer route/point blocks beginning with N/S (no ETA)
			result.Points = append(result.Points, parseRoutePoints02A(rest)...)
		}

	case "h2_header_02E":
		result.Phase = "02E"
		if d, err := strconv.Atoi(match.Captures["day"]); err == nil {
			result.Day = d
		}
		// For 02E there is no 6-digit "time"; ETA is usually the most useful timestamp.
		if result.ReportTime == "" {
			result.ReportTime = fmtHHMM(match.Captures["eta"])
		}

		// First point is embedded in the header for 02E.
		if pt, ok := buildPointFrom02EHeader(match.Captures); ok {
			result.Points = append(result.Points, pt)
			// Prefer the point position as the "main" position too.
			result.Latitude = pt.Latitude
			result.Longitude = pt.Longitude
		}

		headerLen := headerLen02E(text)
		if headerLen > 0 && headerLen < len(text) {
			rest := text[headerLen:]
			result.RawData = strings.TrimSpace(rest)
			result.Points = append(result.Points, parseRoutePoints02E(rest)...)
		}
	}

	return result
}

// parseDegThousandths parses coordinates encoded as degrees * 1000.
// Example: "42540" + "N" -> 42.540
func parseDegThousandths(v string, dir string) float64 {
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	out := float64(n) / 1000.0
	switch strings.ToUpper(dir) {
	case "S", "W":
		out = -out
	}
	return out
}

func fmtHHMM(s string) string {
	if len(s) != 4 {
		return s
	}
	return s[0:2] + ":" + s[2:4]
}

func parseFL10(flStr string) float64 {
	if flStr == "" {
		return 0
	}
	fl, err := strconv.Atoi(flStr)
	if err != nil {
		return 0
	}
	return float64(fl) / 10.0
}

func parseTemp10(sign, tempStr string) float64 {
	if tempStr == "" {
		return 0
	}
	t, err := strconv.Atoi(tempStr)
	if err != nil {
		return 0
	}
	out := float64(t) / 10.0
	if strings.ToUpper(sign) == "M" {
		out = -out
	}
	return out
}

func headerLen02E(text string) int {
	// 02E + day(2) + origin(4) + dest(4) + lat_dir(1) + lat(5) + lon_dir(1) + lon(6)
	// + eta(4) + fl(3-4) + sign(1) + temp(3) + wind_dir(3) + wind_spd(3) + gust(optional)
	//
	// fl is either 3 or 4 digits. We can detect by looking for the temperature sign (M/P)
	// at the expected offset.
	if !strings.HasPrefix(text, "02E") {
		return 0
	}
	base := 3 + 2 + 4 + 4 + 1 + 5 + 1 + 6 + 4
	// base points to start of FL.
	if len(text) < base+3+1+3+3+3 {
		return 0
	}
	// Try FL=4 first.
	if len(text) >= base+4+1 && (text[base+4] == 'M' || text[base+4] == 'P') {
		l := base + 4 + 1 + 3 + 3 + 3
		if len(text) > l && text[l] == 'G' {
			l++
		}
		return l
	}
	// Fallback FL=3.
	if len(text) >= base+3+1 && (text[base+3] == 'M' || text[base+3] == 'P') {
		l := base + 3 + 1 + 3 + 3 + 3
		if len(text) > l && text[l] == 'G' {
			l++
		}
		return l
	}
	return 0
}

func buildPointFrom02EHeader(c map[string]string) (WindPoint, bool) {
	lat := parseDegThousandths(c["lat"], c["lat_dir"])
	lon := parseDegThousandths(c["lon"], c["lon_dir"])
	if lat == 0 && lon == 0 {
		return WindPoint{}, false
	}
	pt := WindPoint{
		Marker:      "H",
		Latitude:    lat,
		Longitude:   lon,
		ETA:         fmtHHMM(c["eta"]),
		FlightLevel: parseFL10(c["fl"]),
		Temperature: parseTemp10(c["temp_sign"], c["temp"]),
		Gusting:     c["gust"] == "G",
	}
	if d, err := strconv.Atoi(c["wind_dir"]); err == nil {
		pt.WindDir = d
	}
	if s, err := strconv.Atoi(c["wind_spd"]); err == nil {
		pt.WindSpeed = s
	}
	return pt, true
}

var (
	// 02A route points: N/S + lat5 + E/W + lon6 + FL(3-4) + temp_sign + temp3 + dir3 + spd3 + optional G
	rePoint02A = regexp.MustCompile(`([NS])(\d{5})([EW])(\d{6})\s*(\d{3,4})([MP])(\d{3})(\d{3})(\d{3})(G?)`)
	// 02E route points: Q + N/S + lat5 + E/W + lon6 + ETA4 + FL(3-4) + temp_sign + temp3 + dir3 + spd3 + optional G
	rePoint02E = regexp.MustCompile(`Q([NS])(\d{5})([EW])(\d{6})\s*(\d{4})\s*(\d{3,4})([MP])(\d{3})(\d{3})(\d{3})(G?)`)
)

func parseRoutePoints02A(data string) []WindPoint {
	var out []WindPoint
	for _, m := range rePoint02A.FindAllStringSubmatch(data, -1) {
		lat := parseDegThousandths(m[2], m[1])
		lon := parseDegThousandths(m[4], m[3])
		if lat == 0 && lon == 0 {
			continue
		}
		pt := WindPoint{
			Marker:      "N",
			Latitude:    lat,
			Longitude:   lon,
			FlightLevel: parseFL10(m[5]),
			Temperature: parseTemp10(m[6], m[7]),
			Gusting:     m[10] == "G",
		}
		if d, err := strconv.Atoi(m[8]); err == nil {
			pt.WindDir = d
		}
		if s, err := strconv.Atoi(m[9]); err == nil {
			pt.WindSpeed = s
		}
		out = append(out, pt)
	}
	return out
}

func parseRoutePoints02E(data string) []WindPoint {
	var out []WindPoint
	for _, m := range rePoint02E.FindAllStringSubmatch(data, -1) {
		lat := parseDegThousandths(m[2], m[1])
		lon := parseDegThousandths(m[4], m[3])
		if lat == 0 && lon == 0 {
			continue
		}
		pt := WindPoint{
			Marker:      "Q",
			Latitude:    lat,
			Longitude:   lon,
			ETA:         fmtHHMM(m[5]),
			FlightLevel: parseFL10(m[6]),
			Temperature: parseTemp10(m[7], m[8]),
			Gusting:     m[11] == "G",
		}
		if d, err := strconv.Atoi(m[9]); err == nil {
			pt.WindDir = d
		}
		if s, err := strconv.Atoi(m[10]); err == nil {
			pt.WindSpeed = s
		}
		out = append(out, pt)
	}
	return out
}

// parseWindLayers extracts wind layer data from the message body using grok patterns.
func parseWindLayers(compiler *patterns.Compiler, data string) []WindLayer {
	var layers []WindLayer

	matches := compiler.FindAllMatches(data, "wind_layer")
	for _, m := range matches {
		layer := WindLayer{}

		// Flight level.
		if fl, err := strconv.Atoi(m["fl"]); err == nil {
			layer.FlightLevel = fl
		}

		// Temperature (M = minus, P = plus).
		if temp, err := strconv.Atoi(m["temp"]); err == nil {
			if m["temp_sign"] == "M" {
				layer.Temperature = -temp
			} else {
				layer.Temperature = temp
			}
		}

		// Optional wind direction.
		if m["wind_dir"] != "" {
			if dir, err := strconv.Atoi(m["wind_dir"]); err == nil {
				layer.WindDir = dir
			}
		}

		// Optional wind speed.
		if m["wind_spd"] != "" {
			if spd, err := strconv.Atoi(m["wind_spd"]); err == nil {
				layer.WindSpeed = spd
			}
		}

		// Gusting flag.
		layer.Gusting = m["gust"] == "G"

		layers = append(layers, layer)
	}

	return layers
}
