package state

import "time"

// Aircraft represents an aircraft with ICAO hex to registration mapping.
type Aircraft struct {
	ICAOHex      string    `json:"icao_hex"`
	Registration string    `json:"registration"`
	TypeCode     string    `json:"type_code,omitempty"`
	Operator     string    `json:"operator,omitempty"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	MsgCount     int       `json:"msg_count"`
	SyncedAt     *time.Time `json:"synced_at,omitempty"`
}

// Waypoint represents a navigation waypoint with coordinates.
type Waypoint struct {
	Name        string    `json:"name"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	SourceCount int       `json:"source_count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	SyncedAt    *time.Time `json:"synced_at,omitempty"`
}

// Route represents a learned flight route pattern.
type Route struct {
	ID               int64      `json:"id"`
	FlightPattern    string     `json:"flight_pattern"`
	OriginICAO       string     `json:"origin_icao"`
	DestICAO         string     `json:"dest_icao"`
	ObservationCount int        `json:"observation_count"`
	FirstSeen        time.Time  `json:"first_seen"`
	LastSeen         time.Time  `json:"last_seen"`
	SyncedAt         *time.Time `json:"synced_at,omitempty"`
}

// RouteAircraft represents an aircraft observed on a specific route.
type RouteAircraft struct {
	RouteID          int64     `json:"route_id"`
	Registration     string    `json:"registration"`
	ObservationCount int       `json:"observation_count"`
	FirstSeen        time.Time `json:"first_seen"`
	LastSeen         time.Time `json:"last_seen"`
}

// ATIS represents airport terminal information.
type ATIS struct {
	AirportICAO string     `json:"airport_icao"`
	Letter      string     `json:"letter"`
	ATISType    string     `json:"atis_type,omitempty"` // ARR, DEP, or empty for combined.
	ATISTime    string     `json:"atis_time,omitempty"`
	RawText     string     `json:"raw_text,omitempty"` // Full raw ATIS text.
	Runways     []string   `json:"runways,omitempty"`
	Approaches  []string   `json:"approaches,omitempty"`
	Wind        string     `json:"wind,omitempty"`
	Visibility  string     `json:"visibility,omitempty"`
	Clouds      string     `json:"clouds,omitempty"`
	Temperature string     `json:"temperature,omitempty"`
	DewPoint    string     `json:"dew_point,omitempty"`
	QNH         string     `json:"qnh,omitempty"`
	Remarks     []string   `json:"remarks,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	SyncedAt    *time.Time `json:"synced_at,omitempty"`
}

// FlightState represents the current known state of an aircraft/flight.
type FlightState struct {
	Key          string    `json:"key"` // Primary key: ICAO hex or registration.
	ICAOHex      string    `json:"icao_hex,omitempty"`
	Registration string    `json:"registration,omitempty"`
	FlightNumber string    `json:"flight_number,omitempty"`
	Origin       string    `json:"origin,omitempty"`
	Destination  string    `json:"destination,omitempty"`
	Latitude     float64   `json:"latitude,omitempty"`
	Longitude    float64   `json:"longitude,omitempty"`
	Altitude     int       `json:"altitude,omitempty"`
	GroundSpeed  int       `json:"ground_speed,omitempty"`
	Track        int       `json:"track,omitempty"`
	Waypoints    []string  `json:"waypoints,omitempty"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	MsgCount     int       `json:"msg_count"`
}

// HasPosition returns true if the flight state has valid position data.
func (f *FlightState) HasPosition() bool {
	return f.Latitude != 0 || f.Longitude != 0
}

// HasRoute returns true if the flight state has origin and destination.
func (f *FlightState) HasRoute() bool {
	return f.Origin != "" && f.Destination != ""
}

// AddWaypoint adds a waypoint to the list if not already present.
func (f *FlightState) AddWaypoint(wp string) {
	if wp == "" {
		return
	}
	for _, existing := range f.Waypoints {
		if existing == wp {
			return
		}
	}
	f.Waypoints = append(f.Waypoints, wp)
}
