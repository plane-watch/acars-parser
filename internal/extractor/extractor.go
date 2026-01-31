// Package extractor provides functions for extracting flight data from parsed ACARS messages.
// This package is database-agnostic and can be used with any storage backend.
package extractor

import (
	"encoding/json"
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// flightNumRe captures airline code and flight number for normalisation.
var flightNumRe = regexp.MustCompile(`^([A-Z]{2,3})(\d{1,4})$`)

// FlightUpdate contains flight data extracted from messages.
type FlightUpdate struct {
	ICAOHex      string  `json:"icao_hex,omitempty"`
	Registration string  `json:"registration,omitempty"`
	FlightNumber string  `json:"flight_number,omitempty"`
	Origin       string  `json:"origin,omitempty"`
	Destination  string  `json:"destination,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
	Altitude     int     `json:"altitude,omitempty"`
	GroundSpeed  int     `json:"ground_speed,omitempty"`
	Track        int     `json:"track,omitempty"`
	Waypoint     string  `json:"waypoint,omitempty"`
	TypeCode     string  `json:"type_code,omitempty"`
	Operator     string  `json:"operator,omitempty"`
}

// WaypointUpdate contains waypoint data extracted from messages.
type WaypointUpdate struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ATISUpdate contains ATIS data extracted from messages.
type ATISUpdate struct {
	AirportICAO string   `json:"airport_icao"`
	Letter      string   `json:"letter"`
	ATISType    string   `json:"atis_type,omitempty"` // ARR, DEP, or empty for combined.
	ATISTime    string   `json:"atis_time,omitempty"`
	RawText     string   `json:"raw_text,omitempty"`
	Runways     []string `json:"runways,omitempty"`
	Approaches  []string `json:"approaches,omitempty"`
	Wind        string   `json:"wind,omitempty"`
	Visibility  string   `json:"visibility,omitempty"`
	Clouds      string   `json:"clouds,omitempty"`
	Temperature string   `json:"temperature,omitempty"`
	DewPoint    string   `json:"dew_point,omitempty"`
	QNH         string   `json:"qnh,omitempty"`
	Remarks     []string `json:"remarks,omitempty"`
}

// ExtractedData is a container for all data extracted from a message.
type ExtractedData struct {
	Flight    *FlightUpdate     `json:"flight,omitempty"`
	Waypoints []*WaypointUpdate `json:"waypoints,omitempty"`
	ATIS      *ATISUpdate       `json:"atis,omitempty"`
}

// Extract extracts relevant data from a message and its parsed results.
// This function is database-agnostic and returns all extracted data for the
// caller to process as needed.
func Extract(msg *acars.Message, results []registry.Result) ExtractedData {
	data := ExtractedData{}

	// Build the base flight update from the message metadata.
	update := &FlightUpdate{}

	// Extract identity from the message/airframe.
	if msg.Airframe != nil {
		update.ICAOHex = msg.Airframe.ICAO
		update.Registration = msg.Airframe.Tail
		update.TypeCode = msg.Airframe.ManufacturerModel
		update.Operator = msg.Airframe.Owner
	}
	if update.Registration == "" {
		update.Registration = msg.Tail
	}

	// Extract flight info from the message.
	if msg.Flight != nil {
		update.FlightNumber = strings.TrimSpace(msg.Flight.Flight)
		if update.Origin == "" && isValidAirportCode(msg.Flight.DepartingAirport) {
			update.Origin = strings.TrimSpace(msg.Flight.DepartingAirport)
		}
		if update.Destination == "" && isValidAirportCode(msg.Flight.DestinationAirport) {
			update.Destination = strings.TrimSpace(msg.Flight.DestinationAirport)
		}
	}

	// Process each parsed result to extract additional data.
	for _, result := range results {
		extractFromResult(update, &data, result)
	}

	// Normalise flight number to strip leading zeros.
	if update.FlightNumber != "" {
		update.FlightNumber = NormaliseFlightNumber(update.FlightNumber)
	}

	// Only include the flight update if we have identity info.
	if update.ICAOHex != "" || update.Registration != "" {
		data.Flight = update
	}

	return data
}

// NormaliseFlightNumber strips leading zeros from flight numbers for consistent matching.
// For example, "QF001" becomes "QF1", "QFA001" becomes "QFA1", "UAL0042" becomes "UAL42".
// Handles any alphabetic prefix followed by digits, not just standard 2-3 letter airline codes.
// Note: This function does not attempt IATA-to-ICAO airline code conversion as this
// requires per-aircraft learned mappings (handled by the state tracker).
func NormaliseFlightNumber(flightNum string) string {
	flightNum = strings.TrimSpace(flightNum)
	if flightNum == "" {
		return ""
	}

	// Find where the numeric part starts.
	for i, r := range flightNum {
		if r >= '0' && r <= '9' {
			prefix := flightNum[:i]
			numPart := strings.TrimLeft(flightNum[i:], "0")
			if numPart == "" {
				numPart = "0" // Preserve at least one zero for flight "000".
			}
			return prefix + numPart
		}
	}

	// No numeric part found, return as-is.
	return flightNum
}

// IsICAOCallsign checks if a flight number uses ICAO format (3-letter airline prefix).
func IsICAOCallsign(flightNum string) bool {
	match := flightNumRe.FindStringSubmatch(flightNum)
	if match == nil {
		return false
	}
	return len(match[1]) == 3
}

// isValidAirportCode checks if a string is a valid ICAO airport code.
// Uses proper ICAO prefix validation and blocklist to reject common words
// that look like airport codes (e.g., ASAT, MINS, MUST).
func isValidAirportCode(code string) bool {
	code = strings.TrimSpace(code)
	// Use the proper ICAO validation which checks prefix patterns and blocklist.
	return patterns.IsValidICAO(code)
}

// extractFromResult extracts data from a parsed result into the update struct.
func extractFromResult(update *FlightUpdate, data *ExtractedData, result registry.Result) {
	// Convert result to a map for generic field access.
	b, err := json.Marshal(result)
	if err != nil {
		return
	}

	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return
	}

	// Extract registration/tail.
	if v, ok := m["tail"].(string); ok && v != "" && update.Registration == "" {
		update.Registration = v
	}
	if v, ok := m["registration"].(string); ok && v != "" && update.Registration == "" {
		update.Registration = v
	}

	// Extract flight number (trimmed).
	if v, ok := m["flight_number"].(string); ok && v != "" {
		update.FlightNumber = strings.TrimSpace(v)
	}
	if v, ok := m["flight_num"].(string); ok && v != "" {
		update.FlightNumber = strings.TrimSpace(v)
	}
	if v, ok := m["callsign"].(string); ok && v != "" && update.FlightNumber == "" {
		update.FlightNumber = strings.TrimSpace(v)
	}

	// Extract route (with validation to reject corrupted codes).
	if v, ok := m["origin"].(string); ok && v != "" && isValidAirportCode(v) {
		update.Origin = strings.TrimSpace(v)
	}
	if v, ok := m["origin_icao"].(string); ok && v != "" && isValidAirportCode(v) {
		update.Origin = strings.TrimSpace(v)
	}
	if v, ok := m["destination"].(string); ok && v != "" && isValidAirportCode(v) {
		update.Destination = strings.TrimSpace(v)
	}
	if v, ok := m["dest_icao"].(string); ok && v != "" && isValidAirportCode(v) {
		update.Destination = strings.TrimSpace(v)
	}

	// Extract position.
	// Note: We accept zero values here since 0,0 is a valid position (Gulf of Guinea).
	// However, if BOTH lat and lon are exactly zero, it's likely unset data, not Null Island.
	lat, hasLat := m["latitude"].(float64)
	lon, hasLon := m["longitude"].(float64)
	if hasLat && hasLon {
		// Only skip if both are exactly zero (likely unset), otherwise accept the position.
		if lat != 0 || lon != 0 {
			update.Latitude = lat
			update.Longitude = lon
		}
	} else {
		// Handle case where only one coordinate is present.
		if hasLat && lat != 0 {
			update.Latitude = lat
		}
		if hasLon && lon != 0 {
			update.Longitude = lon
		}
	}

	// Extract altitude (could be int or float64 in JSON).
	if v, ok := m["altitude"].(float64); ok && v != 0 {
		update.Altitude = int(v)
	}
	if v, ok := m["flight_level"].(float64); ok && v != 0 {
		update.Altitude = int(v) * 100 // Convert FL to feet.
	}

	// Extract ground speed and track.
	if v, ok := m["ground_speed"].(float64); ok && v != 0 {
		update.GroundSpeed = int(v)
	}
	if v, ok := m["track"].(float64); ok && v != 0 {
		update.Track = int(v)
	}
	if v, ok := m["heading"].(float64); ok && v != 0 && update.Track == 0 {
		update.Track = int(v)
	}

	// Extract aircraft type.
	if v, ok := m["aircraft_type"].(string); ok && v != "" {
		update.TypeCode = v
	}

	// Extract ICAO hex address (Mode-S transponder code).
	// This is the 6-character hex identifier for the aircraft (e.g., "7C4EF3").
	if v, ok := m["aircraft_icao"].(string); ok && v != "" && update.ICAOHex == "" {
		update.ICAOHex = v
	}

	// Extract waypoint information.
	if v, ok := m["waypoint"].(string); ok && v != "" {
		update.Waypoint = v
		// If we have coordinates with the waypoint, record it.
		if update.Latitude != 0 && update.Longitude != 0 {
			data.Waypoints = append(data.Waypoints, &WaypointUpdate{
				Name:      v,
				Latitude:  update.Latitude,
				Longitude: update.Longitude,
			})
		}
	}
	if v, ok := m["current_waypoint"].(string); ok && v != "" {
		update.Waypoint = v
	}
	if v, ok := m["next_waypoint"].(string); ok && v != "" && update.Waypoint == "" {
		update.Waypoint = v
	}

	// Handle waypoints array (from label10, pdc, etc.).
	if waypoints, ok := m["waypoints"].([]interface{}); ok {
		for _, wp := range waypoints {
			if wpMap, ok := wp.(map[string]interface{}); ok {
				if name, ok := wpMap["name"].(string); ok && name != "" {
					// Try to get coordinates if available.
					lat, _ := wpMap["latitude"].(float64)
					lon, _ := wpMap["longitude"].(float64)
					if lat != 0 && lon != 0 {
						data.Waypoints = append(data.Waypoints, &WaypointUpdate{
							Name:      name,
							Latitude:  lat,
							Longitude: lon,
						})
					}
					// Set the first waypoint name if not already set.
					if update.Waypoint == "" {
						update.Waypoint = name
					}
				}
			} else if wpStr, ok := wp.(string); ok && wpStr != "" {
				// Just a waypoint name, no coordinates.
				// Only set if not already set (preserve first waypoint).
				if update.Waypoint == "" {
					update.Waypoint = wpStr
				}
			}
		}
	}

	// Handle route waypoints array (from pdc).
	if waypoints, ok := m["route_waypoints"].([]interface{}); ok {
		for _, wp := range waypoints {
			if wpStr, ok := wp.(string); ok && wpStr != "" {
				// Only set if not already set (preserve first waypoint).
				if update.Waypoint == "" {
					update.Waypoint = wpStr
				}
			}
		}
	}

	// Handle ATIS results.
	if result.Type() == "atis" {
		if atis := extractATIS(m); atis != nil {
			data.ATIS = atis
		}
	}
}

// extractATIS extracts ATIS data from a parsed result map.
func extractATIS(m map[string]interface{}) *ATISUpdate {
	airport, _ := m["airport"].(string)
	letter, _ := m["atis_letter"].(string)

	if airport == "" || letter == "" {
		return nil
	}

	// Validate airport is a valid ICAO code.
	if !patterns.IsValidICAO(airport) {
		return nil
	}

	// Validate ATIS letter is a single uppercase letter A-Z.
	if len(letter) != 1 || letter[0] < 'A' || letter[0] > 'Z' {
		return nil
	}

	atis := &ATISUpdate{
		AirportICAO: airport,
		Letter:      letter,
	}

	if v, ok := m["atis_type"].(string); ok {
		atis.ATISType = v
	}
	if v, ok := m["atis_time"].(string); ok {
		atis.ATISTime = v
	}
	// Extract raw ATIS text (try common field names).
	if v, ok := m["raw_text"].(string); ok {
		atis.RawText = v
	} else if v, ok := m["raw"].(string); ok {
		atis.RawText = v
	} else if v, ok := m["text"].(string); ok {
		atis.RawText = v
	}
	if v, ok := m["wind"].(string); ok {
		atis.Wind = v
	}
	if v, ok := m["visibility"].(string); ok {
		atis.Visibility = v
	}
	if v, ok := m["clouds"].(string); ok {
		atis.Clouds = v
	}
	if v, ok := m["temperature"].(string); ok {
		atis.Temperature = v
	}
	if v, ok := m["dew_point"].(string); ok {
		atis.DewPoint = v
	}
	if v, ok := m["qnh"].(string); ok {
		atis.QNH = v
	}

	// Handle arrays.
	if runways, ok := m["runways"].([]interface{}); ok {
		for _, r := range runways {
			if s, ok := r.(string); ok {
				atis.Runways = append(atis.Runways, s)
			}
		}
	}
	if approaches, ok := m["approaches"].([]interface{}); ok {
		for _, a := range approaches {
			if s, ok := a.(string); ok {
				atis.Approaches = append(atis.Approaches, s)
			}
		}
	}
	if remarks, ok := m["remarks"].([]interface{}); ok {
		for _, r := range remarks {
			if s, ok := r.(string); ok {
				atis.Remarks = append(atis.Remarks, s)
			}
		}
	}

	return atis
}
