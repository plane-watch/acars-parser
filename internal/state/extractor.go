package state

import (
	"encoding/json"
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// airportCodeRe matches valid IATA (3 chars) or ICAO (4 chars) airport codes.
var airportCodeRe = regexp.MustCompile(`^[A-Z]{3,4}$`)

// isValidAirportCode checks if a string looks like a valid airport code.
// Returns false for codes containing special characters, whitespace, or wrong length.
func isValidAirportCode(code string) bool {
	code = strings.TrimSpace(code)
	return airportCodeRe.MatchString(code)
}

// ExtractAndUpdate extracts relevant data from a message and its parsed results,
// then updates the tracker accordingly.
func ExtractAndUpdate(t *Tracker, msg *acars.Message, results []registry.Result) {
	// Build the base flight update from the message metadata.
	update := FlightUpdate{}

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
		extractFromResult(&update, t, result)
	}

	// Update the flight state if we have identity info.
	if update.ICAOHex != "" || update.Registration != "" {
		t.UpdateFlight(update)
	}
}

// extractFromResult extracts data from a parsed result into the update struct.
func extractFromResult(update *FlightUpdate, t *Tracker, result registry.Result) {
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
	if v, ok := m["latitude"].(float64); ok && v != 0 {
		update.Latitude = v
	}
	if v, ok := m["longitude"].(float64); ok && v != 0 {
		update.Longitude = v
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
	if v, ok := m["aircraft_icao"].(string); ok && v != "" {
		update.TypeCode = v
	}

	// Extract waypoint information.
	if v, ok := m["waypoint"].(string); ok && v != "" {
		update.Waypoint = v
		// If we have coordinates with the waypoint, record it.
		if update.Latitude != 0 && update.Longitude != 0 {
			t.UpdateWaypoint(v, update.Latitude, update.Longitude)
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
						t.UpdateWaypoint(name, lat, lon)
					}
				}
			} else if wpStr, ok := wp.(string); ok && wpStr != "" {
				// Just a waypoint name, no coordinates.
				update.Waypoint = wpStr
			}
		}
	}

	// Handle route waypoints array (from pdc).
	if waypoints, ok := m["route_waypoints"].([]interface{}); ok {
		for _, wp := range waypoints {
			if wpStr, ok := wp.(string); ok && wpStr != "" {
				// Just record as a waypoint name without coordinates.
				update.Waypoint = wpStr
			}
		}
	}

	// Handle ATIS results.
	if result.Type() == "atis" {
		extractATIS(t, m)
	}
}

// extractATIS extracts ATIS data and updates the tracker.
func extractATIS(t *Tracker, m map[string]interface{}) {
	airport, _ := m["airport"].(string)
	letter, _ := m["atis_letter"].(string)

	if airport == "" || letter == "" {
		return
	}

	atis := &ATIS{
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

	t.UpdateATIS(atis)
}
