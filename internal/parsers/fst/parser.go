// Package fst parses FST (Label 15) flight status messages.
package fst

import (
	"strconv"
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents a parsed FST flight status report.
type Result struct {
	MsgID         int64   `json:"message_id"`
	Timestamp     string  `json:"timestamp"`
	Tail          string  `json:"tail,omitempty"`
	Sequence      string  `json:"sequence,omitempty"`
	Origin        string  `json:"origin,omitempty"`
	Destination   string  `json:"destination,omitempty"`
	Latitude      float64 `json:"latitude,omitempty"`
	Longitude     float64 `json:"longitude,omitempty"`
	FlightLevel   int     `json:"flight_level,omitempty"`
	Heading       int     `json:"heading,omitempty"`
	GroundSpeed   int     `json:"ground_speed,omitempty"`
	WindDirection int     `json:"wind_direction,omitempty"`
	WindSpeed     int     `json:"wind_speed,omitempty"`
	Track         int     `json:"track,omitempty"`    // Actual ground track
	Unknown1      int     `json:"unknown1,omitempty"` // Nepoznati parametar (možda TAS, IAS, Mach*100)
	Temperature   int     `json:"temperature,omitempty"`
	RawData       string  `json:"raw_data,omitempty"`
}

func (r *Result) Type() string     { return "fst" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses FST flight status messages.
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

func (p *Parser) Name() string     { return "fst" }
func (p *Parser) Labels() []string { return []string{"15"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "FST")
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
	if match == nil {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
		RawData:   msg.Text,
	}

	result.Sequence = match.Captures["seq"]
	result.Origin = match.Captures["origin"]
	result.Destination = match.Captures["dest"]

	// Parse coordinates
	latStr := match.Captures["lat"]
	lonStr := match.Captures["lon"]
	latDir := match.Captures["lat_dir"]
	lonDir := match.Captures["lon_dir"]
	lat := parseCoord(latStr, latDir)
	lon := parseCoord(lonStr, lonDir)
	result.Latitude = lat
	result.Longitude = lon

	// Parse rest of fields from the "rest" group if present
	rest := match.Captures["rest"]
	fields := strings.Fields(rest)

	// Field 0: Flight Level
	if len(fields) >= 1 {
		if fl, err := strconv.Atoi(fields[0]); err == nil {
			result.FlightLevel = fl
		}
	}

	// Field 1: Ground Speed, Heading, ili nepoznati parametar
	// Ako je > 360, onda je ground speed
	// Ako je u rasponu heading (120-200 ili 295-335), onda heading - ALI samo ako odgovara smeru leta
	// Inače je nepoznati parametar (možda TAS, IAS, Mach*100, itd.)
	if len(fields) >= 2 {
		val, err := strconv.Atoi(fields[1])
		if err == nil {
			if val > 360 {
				result.GroundSpeed = val
			} else if (val >= 120 && val <= 200) || (val >= 295 && val <= 335) {
				// Proveri da li heading ima smisla u kontekstu smera leta
				westbound := isWestbound(result.Origin, result.Destination)
				
				// Westbound letovi (Azija → Evropa) bi trebalo da imaju heading oko 295-335°
				// Eastbound letovi (Evropa → Azija) bi trebalo da imaju heading oko 120-200°
				if westbound && val >= 295 && val <= 335 {
					result.Heading = val
				} else if !westbound && val >= 120 && val <= 200 {
					result.Heading = val
				} else {
					// Heading je u validnom opsegu ali ne odgovara smeru leta
					result.Unknown1 = val
				}
			} else {
				// Nepoznati parametar
				result.Unknown1 = val
			}
		}
	}

	// Field 2: Ground Speed, Heading, Wind Direction, Wind Speed ili Track
	if len(fields) >= 3 {
		windField := fields[2]
		windField = strings.TrimSuffix(windField, "M")
		windField = strings.TrimSuffix(windField, "K")
		if val, err := strconv.Atoi(windField); err == nil {
			// Ako field 1 nije bio ground speed, proveri da li je field 2 ground speed
			if result.GroundSpeed == 0 && val > 360 {
				result.GroundSpeed = val
			} else if result.Heading == 0 && ((val >= 120 && val <= 200) || (val >= 295 && val <= 335)) {
				result.Heading = val
			} else if val >= 290 && val <= 360 {
				result.WindDirection = val
			} else if val < 200 {
				result.WindSpeed = val
			}
		}
	}

	// Parse temperature - find first field ending with 'C'
	var tempIdx int
	for i, field := range fields {
		if strings.HasSuffix(field, "C") && len(field) > 1 {
			tempField := strings.TrimSuffix(field, "C")
			if strings.HasPrefix(tempField, "M") {
				tempField = strings.TrimPrefix(tempField, "M")
				if temp, err := strconv.Atoi(tempField); err == nil {
					result.Temperature = -temp
					tempIdx = i
					break
				}
			} else {
				if temp, err := strconv.Atoi(tempField); err == nil {
					result.Temperature = temp
					tempIdx = i
					break
				}
			}
		}
	}

	// Parse dodatna polja posle temperature (wind direction/speed mogu biti tu)
	if tempIdx > 0 && tempIdx+1 < len(fields) {
		// Sledeći field posle temperature može sadržati dodatne podatke
		extraData := fields[tempIdx+1]
		// Format: SSWWW... (SS=wind speed, WWW=wind direction)
		if len(extraData) >= 5 {
			// Wind speed iz prvih 2 cifre
			if ws, err := strconv.Atoi(extraData[0:2]); err == nil {
				result.WindSpeed = ws
			}
			// Wind direction iz sledećih 3 cifre
			if wd, err := strconv.Atoi(extraData[2:5]); err == nil && wd <= 360 {
				result.WindDirection = wd
			}
		}
	}

	return result
}

// parseCoord parses a latitude or longitude string in various compact formats.
func parseCoord(coord, dir string) float64 {
	if coord == "" {
		return 0
	}

	// Format: decimalni stepeni bez decimalne tačke
	// Primer: 418071 = 41.8071°, 0214075 = 021.4075° = 21.4075°
	if val, err := strconv.ParseFloat(coord, 64); err == nil {
		result := val / 10000.0
		return applyDir(result, dir)
	}
	return 0
}

func applyDir(val float64, dir string) float64 {
	if dir == "S" || dir == "W" {
		return -val
	}
	return val
}

// isWestbound proverava da li je let westbound na osnovu origin i destination.
// Westbound letovi (Azija → Evropa) imaju heading oko 300°±30°
// Eastbound letovi (Evropa → Azija) imaju heading oko 140°±30°
func isWestbound(origin, dest string) bool {
	// Evropski aerodromi: EG** (UK), LP** (Portugal), LE** (Spain), LF** (France), itd.
	// Azijski aerodromi: VO** (India), ZS** (China), OM** (Middle East), OP** (Pakistan), itd.
	
	europeanPrefixes := []string{"EG", "LP", "LE", "LF", "EB", "EH", "ED", "LI", "LO", "LK", "LZ", "LR", "LY"}
	asianPrefixes := []string{"VO", "ZS", "ZG", "ZP", "OM", "OP", "OI", "VA", "VT", "VY"}
	
	isDestEuropean := false
	isOriginAsian := false
	
	for _, prefix := range europeanPrefixes {
		if strings.HasPrefix(dest, prefix) {
			isDestEuropean = true
		}
	}
	
	for _, prefix := range asianPrefixes {
		if strings.HasPrefix(origin, prefix) {
			isOriginAsian = true
		}
	}
	
	// Westbound: Origin je Azija, Destination je Evropa
	return isOriginAsian && isDestEuropean
}
