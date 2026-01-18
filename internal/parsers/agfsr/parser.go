// Package agfsr parses AGFSR (Label 4T) flight status messages.
package agfsr

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
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

// Result represents a parsed AGFSR flight status report.
type Result struct {
	MsgID       int64   `json:"message_id"`
	Timestamp   string  `json:"timestamp"`
	Tail        string  `json:"tail,omitempty"`
	FlightNum   string  `json:"flight_number,omitempty"`
	DayOfMonth  int     `json:"day_of_month,omitempty"`
	Route       string  `json:"route,omitempty"`       // IATA pair like YULMIA
	Origin      string  `json:"origin,omitempty"`      // First 3 chars of route
	Destination string  `json:"destination,omitempty"` // Last 3 chars of route
	ReportTime  string  `json:"report_time,omitempty"`
	Unknown1    string  `json:"unknown1,omitempty"` // Field after time (110)
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	FlightLevel int     `json:"flight_level,omitempty"`
	Phase       string  `json:"phase,omitempty"`       // CRUISE, CLIMB, etc.
	FuelRemain  int     `json:"fuel_remain,omitempty"` // In hundreds of lbs or kg
	FuelUsed    int     `json:"fuel_used,omitempty"`
	Temperature int     `json:"temperature,omitempty"`
	WindDir     int     `json:"wind_dir,omitempty"`
	WindSpeed   int     `json:"wind_speed,omitempty"`
	Heading     int     `json:"heading,omitempty"`
	GroundSpeed int     `json:"ground_speed,omitempty"`
	Unknown2    string  `json:"unknown2,omitempty"` // Unknown field
	ETA         string  `json:"eta,omitempty"`
	Scheduled   string  `json:"scheduled,omitempty"`
}

func (r *Result) Type() string     { return "agfsr" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses AGFSR flight status messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "agfsr" }
func (p *Parser) Labels() []string { return []string{"4T"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "AGFSR")
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
	match := compiler.Parse(text)
	if match == nil || match.FormatName != "agfsr_status" {
		return nil
	}

	result := &Result{
		MsgID:      int64(msg.ID),
		Timestamp:  msg.Timestamp,
		Tail:       msg.Tail,
		FlightNum:  match.Captures["flight"],
		Route:      match.Captures["route"],
		ReportTime: match.Captures["time"],
		Unknown1:   match.Captures["unknown1"],
		Phase:      match.Captures["phase"],
	}

	// Extract origin/destination from route.
	route := match.Captures["route"]
	if len(route) == 6 {
		result.Origin = route[0:3]
		result.Destination = route[3:6]
	}

	// Parse day.
	if day, err := strconv.Atoi(match.Captures["day1"]); err == nil {
		result.DayOfMonth = day
	}

	// Parse position using the secondary pattern.
	if lat, lon, ok := parsePosition(compiler, match.Captures["position"]); ok {
		result.Latitude = lat
		result.Longitude = lon
	}

	// Parse flight level.
	if fl, err := strconv.Atoi(match.Captures["fl"]); err == nil {
		result.FlightLevel = fl
	}

	// Parse fuel remaining.
	if fuel, err := strconv.Atoi(match.Captures["fuel_remain"]); err == nil {
		result.FuelRemain = fuel
	}

	// Parse fuel used.
	if fuel, err := strconv.Atoi(match.Captures["fuel_used"]); err == nil {
		result.FuelUsed = fuel
	}

	// Parse temperature (M60 = -60°C)
	machStr := match.Captures["mach"]
	if strings.HasPrefix(machStr, "M") {
		if temp, err := strconv.Atoi(machStr[1:]); err == nil {
			result.Temperature = -temp
		}
	}

	// Parse wind (248095 = 248° at 95kt).
	windStr := match.Captures["wind"]
	if len(windStr) == 6 {
		if dir, err := strconv.Atoi(windStr[0:3]); err == nil {
			result.WindDir = dir
		}
		if spd, err := strconv.Atoi(windStr[3:6]); err == nil {
			result.WindSpeed = spd
		}
	}

	// Fix: heading should use field1, ground_speed should use field2
	field1 := match.Captures["field1"]
	field2 := match.Captures["field2"]
	if hdg, err := strconv.Atoi(field1); err == nil {
		result.Heading = hdg
	}
	if gs, err := strconv.Atoi(field2); err == nil {
		result.GroundSpeed = gs
	}

	// Parse ETA and scheduled.
	eta := match.Captures["eta"]
	sched := match.Captures["sched"]
	if eta != "----" && eta != "****" {
		result.ETA = eta
	}
	if sched != "----" && sched != "****" {
		result.Scheduled = sched
	}

	return result
}

// parsePosition parses position format like "3457.3N07711.0W" using grok.
func parsePosition(compiler *patterns.Compiler, s string) (lat, lon float64, ok bool) {
	match := compiler.Parse(s)
	if match == nil || match.FormatName != "position" {
		return 0, 0, false
	}

	// Parse latitude (format: DDMM.M - 2 degree digits, decimal minutes).
	lat = patterns.ParseLatitude(match.Captures["lat"], match.Captures["lat_dir"])

	// Parse longitude (format: DDDMM.M - 3 degree digits, decimal minutes).
	lon = patterns.ParseLongitude(match.Captures["lon"], match.Captures["lon_dir"])

	return lat, lon, true
}
