// Package label10 parses Label 10 rich position/route messages.
package label10

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/airports"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// WaypointETA represents a waypoint with its estimated time of arrival.
type WaypointETA struct {
	Name string `json:"name"`
	ETA  string `json:"eta,omitempty"`
}

// Result represents a parsed Label 10 position/route message.
type Result struct {
	MsgID           int64         `json:"message_id"`
	Timestamp       string        `json:"timestamp"`
	Tail            string        `json:"tail,omitempty"`
	Latitude        float64       `json:"latitude"`
	Longitude       float64       `json:"longitude"`
	Mach            float64       `json:"mach,omitempty"`
	Heading         int           `json:"heading,omitempty"`
	FlightLevel     int           `json:"flight_level,omitempty"`
	Destination     string        `json:"destination,omitempty"` // ICAO code
	DestinationName string        `json:"destination_name,omitempty"`
	ETA             string        `json:"eta,omitempty"`
	Fuel            int           `json:"fuel,omitempty"`     // Fuel remaining
	Distance        int           `json:"distance,omitempty"` // Distance to destination
	Waypoints       []WaypointETA `json:"waypoints,omitempty"`
}

func (r *Result) Type() string     { return "label10_position" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Label 10 position/route messages.
type Parser struct{}

// Grok compiler singleton.
var (
	grokCompiler *patterns.Compiler
	grokOnce     sync.Once
	grokErr      error
)

// getCompiler returns the singleton grok compiler.
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

func (p *Parser) Name() string     { return "label10" }
func (p *Parser) Labels() []string { return []string{"10"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "/N") || strings.HasPrefix(text, "/S")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)

	// Try grok-based parsing.
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(text)
	if match == nil || match.FormatName != "rich_position" {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	// Parse latitude.
	if lat, err := strconv.ParseFloat(match.Captures["lat"], 64); err == nil {
		if match.Captures["lat_dir"] == "S" {
			lat = -lat
		}
		result.Latitude = lat
	}

	// Parse longitude.
	if lon, err := strconv.ParseFloat(match.Captures["lon"], 64); err == nil {
		if match.Captures["lon_dir"] == "W" {
			lon = -lon
		}
		result.Longitude = lon
	}

	// Parse remaining slash-delimited fields.
	rest := match.Captures["rest"]
	fields := strings.Split(rest, "/")

	if len(fields) >= 5 {
		// Field 0: unknown (often "10").
		// Field 1: Mach number.
		if mach, err := strconv.ParseFloat(fields[1], 64); err == nil {
			result.Mach = mach
		}

		// Field 2: Heading.
		if hdg, err := strconv.Atoi(fields[2]); err == nil {
			result.Heading = hdg
		}

		// Field 3: Flight level.
		if fl, err := strconv.Atoi(fields[3]); err == nil {
			result.FlightLevel = fl
		}

		// Field 4: Destination ICAO.
		if len(fields[4]) == 4 {
			result.Destination = fields[4]
			result.DestinationName = airports.GetName(result.Destination)
		}

		// Field 5: ETA.
		if len(fields) > 5 && len(fields[5]) == 4 {
			result.ETA = fields[5]
		}

		// Field 6: Fuel.
		if len(fields) > 6 {
			if fuel, err := strconv.Atoi(fields[6]); err == nil {
				result.Fuel = fuel
			}
		}

		// Field 7: Distance.
		if len(fields) > 7 {
			if dist, err := strconv.Atoi(fields[7]); err == nil {
				result.Distance = dist
			}
		}

		// Remaining fields are waypoint/ETA pairs.
		if len(fields) > 8 {
			result.Waypoints = parseWaypoints(fields[8:])
		}
	}

	return result
}

// parseWaypoints extracts waypoint names and their ETAs from the field list.
func parseWaypoints(fields []string) []WaypointETA {
	var waypoints []WaypointETA

	i := 0
	for i < len(fields) {
		field := strings.TrimSpace(fields[i])
		if field == "" {
			i++
			continue
		}

		// Check if it looks like a waypoint name (letters only or mixed).
		isWaypoint := false
		for _, c := range field {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				isWaypoint = true
				break
			}
		}

		if isWaypoint && len(field) >= 2 && len(field) <= 7 {
			wp := WaypointETA{Name: field}

			// Check if next field is an ETA (4 digits).
			if i+1 < len(fields) {
				nextField := strings.TrimSpace(fields[i+1])
				if len(nextField) == 4 {
					if _, err := strconv.Atoi(nextField); err == nil {
						wp.ETA = nextField
						i++
					}
				}
			}

			waypoints = append(waypoints, wp)
		}
		i++
	}

	return waypoints
}
