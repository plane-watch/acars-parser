// Package label17 parses Label 17 position/weather messages in compact CSV-like format.
package label17

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents a decoded Label 17 report.
//
// Example input:
//   031324,37995,0413, 7360,N 46.943,E 18.634,06OCT25,25680, 19,- 47
//
// Notes:
// - track_code and wind_dir_code are encoded with 2 decimals (value/100)
// - ground_speed and wind_speed are in knots; *_kmh fields provide km/h conversion
// - timestamp is the ACARS envelope timestamp; report_timestamp is derived from date+time in the payload (UTC)
type Result struct {
	MsgID           int64   `json:"message_id"`
	Timestamp       string  `json:"timestamp"`
	Tail            string  `json:"tail,omitempty"`
	ReportTime      string  `json:"time,omitempty"`
	ReportDate      string  `json:"date,omitempty"`
	ReportTimestamp string  `json:"report_timestamp,omitempty"`

	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`

	AltitudeFt int `json:"altitude_ft,omitempty"`

	GroundSpeedKts int     `json:"ground_speed_kts,omitempty"`
	GroundSpeedKmh float64 `json:"ground_speed_kmh,omitempty"`

	TrackDeg float64 `json:"track_deg,omitempty"`

	WindDirectionDeg float64 `json:"wind_direction_deg,omitempty"`
	WindSpeedKts     int     `json:"wind_speed_kts,omitempty"`
	WindSpeedKmh     float64 `json:"wind_speed_kmh,omitempty"`

	TemperatureC int `json:"temperature_c,omitempty"`
}

func (r *Result) Type() string     { return "label17" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 17 messages.
type Parser struct{}

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

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "label17" }
func (p *Parser) Labels() []string { return []string{"17"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	// Label-based dispatch is already strong; keep this cheap.
	return strings.Contains(text, ",") && (strings.Contains(text, "N ") || strings.Contains(text, "S "))
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg == nil || msg.Text == "" {
		return nil
	}

	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(msg.Text)
	if match == nil {
		return nil
	}

	// Only one format currently
	if match.FormatName != "label17_csv" {
		return nil
	}

	cap := match.Captures

	res := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	res.ReportTime = strings.TrimSpace(cap["time"])
	res.ReportDate = strings.ToUpper(strings.TrimSpace(cap["date"]))

	// Coords
	res.Latitude = patterns.ParseDecimalCoord(cap["lat"], cap["lat_dir"])
	res.Longitude = patterns.ParseDecimalCoord(cap["lon"], cap["lon_dir"])

	// Altitude
	if v, err := strconv.Atoi(strings.TrimSpace(cap["altitude_ft"])); err == nil {
		res.AltitudeFt = v
	}

	// Ground speed
	if v, err := strconv.Atoi(strings.TrimSpace(cap["ground_speed_kts"])); err == nil {
		res.GroundSpeedKts = v
		res.GroundSpeedKmh = float64(v) * 1.852
	}

	// Track code: deg * 100
	if v, err := parseCode2dp(cap["track_code"]); err == nil {
		res.TrackDeg = v
	}

	// Wind direction: deg * 100
	if v, err := parseCode2dp(cap["wind_dir_code"]); err == nil {
		res.WindDirectionDeg = v
	}

	// Wind speed
	if v, err := strconv.Atoi(strings.TrimSpace(cap["wind_speed_kts"])); err == nil {
		res.WindSpeedKts = v
		res.WindSpeedKmh = float64(v) * 1.852
	}

	// Temp
	if t, err := strconv.Atoi(strings.TrimSpace(cap["temperature_c"])); err == nil {
		if strings.TrimSpace(cap["temp_sign"]) == "-" {
			t = -t
		}
		res.TemperatureC = t
	}

	// Derive report timestamp (UTC) from payload date+time if parseable
	if ts, err := parseReportTimestampUTC(res.ReportDate, res.ReportTime); err == nil {
		res.ReportTimestamp = ts.Format(time.RFC3339)
	}

	// Validate coords
	if res.Latitude == 0 && res.Longitude == 0 {
		return nil
	}

	return res
}

func parseCode2dp(code string) (float64, error) {
	code = strings.TrimSpace(code)
	i, err := strconv.Atoi(code)
	if err != nil {
		return 0, err
	}
	return float64(i) / 100.0, nil
}

func parseReportTimestampUTC(dateDDMMMYY, hhmmss string) (time.Time, error) {
	dateDDMMMYY = strings.ToUpper(strings.TrimSpace(dateDDMMMYY))
	hhmmss = strings.TrimSpace(hhmmss)
	if len(dateDDMMMYY) != 7 || len(hhmmss) != 6 {
		return time.Time{}, strconv.ErrSyntax
	}
	// Convert "06OCT25" -> "06Oct25" for Go's Jan parsing
	dd := dateDDMMMYY[0:2]
	mon := strings.ToLower(dateDDMMMYY[2:5])
	mon = strings.ToUpper(mon[0:1]) + mon[1:]
	yy := dateDDMMMYY[5:7]

	dt, err := time.Parse("02Jan06", dd+mon+yy)
	if err != nil {
		return time.Time{}, err
	}

	hh, err := strconv.Atoi(hhmmss[0:2])
	if err != nil {
		return time.Time{}, err
	}
	mm, err := strconv.Atoi(hhmmss[2:4])
	if err != nil {
		return time.Time{}, err
	}
	ss, err := strconv.Atoi(hhmmss[4:6])
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(dt.Year(), dt.Month(), dt.Day(), hh, mm, ss, 0, time.UTC), nil
}
