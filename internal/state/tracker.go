package state

import (
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Tracker manages flight state and reference data.
type Tracker struct {
	db *sql.DB
	mu sync.RWMutex

	// In-memory flight state cache for fast access.
	flights map[string]*FlightState

	// Callbacks for change notifications.
	onAircraftNew  func(*Aircraft)
	onWaypointNew  func(*Waypoint)
	onRouteNew     func(*Route)
	onATISChanged  func(*ATIS)
}

// NewTracker creates a new state tracker with the given database path.
// If dbPath is empty or ":memory:", uses an in-memory database.
func NewTracker(dbPath string) (*Tracker, error) {
	if dbPath == "" {
		dbPath = ":memory:"
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	// Initialise the schema.
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}

	t := &Tracker{
		db:      db,
		flights: make(map[string]*FlightState),
	}

	// Load existing flight states into memory.
	if err := t.loadFlightStates(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return t, nil
}

// Close closes the database connection.
func (t *Tracker) Close() error {
	return t.db.Close()
}

// OnAircraftNew sets a callback for when a new aircraft is seen.
func (t *Tracker) OnAircraftNew(fn func(*Aircraft)) {
	t.onAircraftNew = fn
}

// OnWaypointNew sets a callback for when a new waypoint is discovered.
func (t *Tracker) OnWaypointNew(fn func(*Waypoint)) {
	t.onWaypointNew = fn
}

// OnRouteNew sets a callback for when a new route pattern is learned.
func (t *Tracker) OnRouteNew(fn func(*Route)) {
	t.onRouteNew = fn
}

// OnATISChanged sets a callback for when an ATIS update is received.
func (t *Tracker) OnATISChanged(fn func(*ATIS)) {
	t.onATISChanged = fn
}

// loadFlightStates loads existing flight states from the database into memory.
func (t *Tracker) loadFlightStates() error {
	rows, err := t.db.Query(`
		SELECT key, icao_hex, registration, flight_number, origin, destination,
		       latitude, longitude, altitude, ground_speed, track, waypoints,
		       first_seen, last_seen, msg_count
		FROM flight_state
		WHERE last_seen > datetime('now', '-1 hour')
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var fs FlightState
		var icaoHex, reg, flight, origin, dest sql.NullString
		var lat, lon sql.NullFloat64
		var alt, gs, track sql.NullInt64
		var waypoints sql.NullString

		err := rows.Scan(
			&fs.Key, &icaoHex, &reg, &flight, &origin, &dest,
			&lat, &lon, &alt, &gs, &track, &waypoints,
			&fs.FirstSeen, &fs.LastSeen, &fs.MsgCount,
		)
		if err != nil {
			continue
		}

		fs.ICAOHex = icaoHex.String
		fs.Registration = reg.String
		fs.FlightNumber = flight.String
		fs.Origin = origin.String
		fs.Destination = dest.String
		fs.Latitude = lat.Float64
		fs.Longitude = lon.Float64
		fs.Altitude = int(alt.Int64)
		fs.GroundSpeed = int(gs.Int64)
		fs.Track = int(track.Int64)

		if waypoints.Valid && waypoints.String != "" {
			_ = json.Unmarshal([]byte(waypoints.String), &fs.Waypoints)
		}

		t.flights[fs.Key] = &fs
	}

	return rows.Err()
}

// UpdateFlight updates the flight state with new information.
// Returns true if this is a new flight (flight number changed).
func (t *Tracker) UpdateFlight(update FlightUpdate) (*FlightState, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Determine the key (prefer ICAO hex, fallback to registration).
	key := update.ICAOHex
	if key == "" {
		key = update.Registration
	}
	if key == "" {
		return nil, false
	}

	now := time.Now()
	isNewFlight := false

	fs, exists := t.flights[key]
	if !exists {
		fs = &FlightState{
			Key:       key,
			FirstSeen: now,
		}
		t.flights[key] = fs
		isNewFlight = true
	}

	// Check if the flight number changed (indicates new flight).
	if update.FlightNumber != "" && fs.FlightNumber != "" && update.FlightNumber != fs.FlightNumber {
		// Reset flight-specific data for the new flight.
		fs.Origin = ""
		fs.Destination = ""
		fs.Waypoints = nil
		fs.FirstSeen = now
		fs.MsgCount = 0
		isNewFlight = true
	}

	// Update identity.
	if update.ICAOHex != "" {
		fs.ICAOHex = update.ICAOHex
	}
	if update.Registration != "" {
		fs.Registration = update.Registration
	}
	if update.FlightNumber != "" {
		fs.FlightNumber = update.FlightNumber
	}

	// Update route.
	if update.Origin != "" {
		fs.Origin = update.Origin
	}
	if update.Destination != "" {
		fs.Destination = update.Destination
	}

	// Update position (only if non-zero).
	if update.Latitude != 0 || update.Longitude != 0 {
		fs.Latitude = update.Latitude
		fs.Longitude = update.Longitude
	}
	if update.Altitude != 0 {
		fs.Altitude = update.Altitude
	}
	if update.GroundSpeed != 0 {
		fs.GroundSpeed = update.GroundSpeed
	}
	if update.Track != 0 {
		fs.Track = update.Track
	}

	// Add waypoint if provided.
	if update.Waypoint != "" {
		fs.AddWaypoint(update.Waypoint)
	}

	fs.LastSeen = now
	fs.MsgCount++

	// Persist to database.
	t.saveFlightState(fs)

	// Update reference data if we have enough info.
	if fs.ICAOHex != "" && fs.Registration != "" {
		t.updateAircraft(fs.ICAOHex, fs.Registration, update.TypeCode, update.Operator)
	}

	// Update route patterns if we have full route info.
	// IMPORTANT: Only create routes when the current update includes a flight number.
	// This prevents stale flight numbers from being associated with new routes when
	// a message has origin/destination but no flight number (e.g., partial FPN messages).
	if update.FlightNumber != "" && fs.Origin != "" && fs.Destination != "" {
		t.updateRoute(fs.FlightNumber, fs.Origin, fs.Destination, fs.Registration)
	}

	return fs, isNewFlight
}

// FlightUpdate contains data to update a flight state.
type FlightUpdate struct {
	ICAOHex      string
	Registration string
	FlightNumber string
	Origin       string
	Destination  string
	Latitude     float64
	Longitude    float64
	Altitude     int
	GroundSpeed  int
	Track        int
	Waypoint     string
	TypeCode     string
	Operator     string
}

// saveFlightState persists a flight state to the database.
func (t *Tracker) saveFlightState(fs *FlightState) {
	waypoints, _ := json.Marshal(fs.Waypoints)

	_, err := t.db.Exec(`
		INSERT INTO flight_state (key, icao_hex, registration, flight_number, origin, destination,
		                          latitude, longitude, altitude, ground_speed, track, waypoints,
		                          first_seen, last_seen, msg_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			icao_hex = excluded.icao_hex,
			registration = excluded.registration,
			flight_number = excluded.flight_number,
			origin = excluded.origin,
			destination = excluded.destination,
			latitude = excluded.latitude,
			longitude = excluded.longitude,
			altitude = excluded.altitude,
			ground_speed = excluded.ground_speed,
			track = excluded.track,
			waypoints = excluded.waypoints,
			last_seen = excluded.last_seen,
			msg_count = excluded.msg_count
	`,
		fs.Key, fs.ICAOHex, fs.Registration, fs.FlightNumber, fs.Origin, fs.Destination,
		fs.Latitude, fs.Longitude, fs.Altitude, fs.GroundSpeed, fs.Track, string(waypoints),
		fs.FirstSeen, fs.LastSeen, fs.MsgCount,
	)
	// Silently ignore errors - flight state is best-effort.
	_ = err
}

// updateAircraft updates or inserts an aircraft record.
func (t *Tracker) updateAircraft(icaoHex, registration, typeCode, operator string) {
	// Check if this is a new aircraft.
	var exists bool
	_ = t.db.QueryRow("SELECT 1 FROM aircraft WHERE icao_hex = ?", icaoHex).Scan(&exists)

	if !exists {
		// New aircraft - insert and trigger callback.
		_, err := t.db.Exec(`
			INSERT INTO aircraft (icao_hex, registration, type_code, operator)
			VALUES (?, ?, ?, ?)
		`, icaoHex, registration, typeCode, operator)

		if err == nil && t.onAircraftNew != nil {
			t.onAircraftNew(&Aircraft{
				ICAOHex:      icaoHex,
				Registration: registration,
				TypeCode:     typeCode,
				Operator:     operator,
				FirstSeen:    time.Now(),
				LastSeen:     time.Now(),
				MsgCount:     1,
			})
		}
	} else {
		// Update existing aircraft.
		_, _ = t.db.Exec(`
			UPDATE aircraft SET
				registration = COALESCE(NULLIF(?, ''), registration),
				type_code = COALESCE(NULLIF(?, ''), type_code),
				operator = COALESCE(NULLIF(?, ''), operator),
				last_seen = CURRENT_TIMESTAMP,
				msg_count = msg_count + 1
			WHERE icao_hex = ?
		`, registration, typeCode, operator, icaoHex)
	}
}

// updateRoute updates or inserts a route pattern.
func (t *Tracker) updateRoute(flightNumber, origin, dest, registration string) {
	// Extract flight pattern (airline code + number, e.g., "QF1" from "QFA1").
	pattern := flightNumber

	// Check if this route exists.
	var id int64
	err := t.db.QueryRow(`
		SELECT id FROM routes
		WHERE flight_pattern = ? AND origin_icao = ? AND dest_icao = ?
	`, pattern, origin, dest).Scan(&id)

	switch err {
	case sql.ErrNoRows:
		// New route - insert and trigger callback.
		result, err := t.db.Exec(`
			INSERT INTO routes (flight_pattern, origin_icao, dest_icao)
			VALUES (?, ?, ?)
		`, pattern, origin, dest)

		if err == nil {
			newID, _ := result.LastInsertId()

			// Add the aircraft to the junction table.
			if registration != "" {
				t.updateRouteAircraft(newID, registration)
			}

			if t.onRouteNew != nil {
				t.onRouteNew(&Route{
					ID:               newID,
					FlightPattern:    pattern,
					OriginICAO:       origin,
					DestICAO:         dest,
					ObservationCount: 1,
					FirstSeen:        time.Now(),
					LastSeen:         time.Now(),
				})
			}
		}
	case nil:
		// Update existing route.
		_, _ = t.db.Exec(`
			UPDATE routes SET
				observation_count = observation_count + 1,
				last_seen = CURRENT_TIMESTAMP,
				synced_at = NULL
			WHERE id = ?
		`, id)

		// Update the aircraft junction table.
		if registration != "" {
			t.updateRouteAircraft(id, registration)
		}
	default:
		// Silently ignore query errors.
	}
}

// updateRouteAircraft updates or inserts an aircraft observation for a route.
func (t *Tracker) updateRouteAircraft(routeID int64, registration string) {
	_, err := t.db.Exec(`
		INSERT INTO route_aircraft (route_id, registration)
		VALUES (?, ?)
		ON CONFLICT(route_id, registration) DO UPDATE SET
			observation_count = observation_count + 1,
			last_seen = CURRENT_TIMESTAMP
	`, routeID, registration)
	// Silently ignore errors - route aircraft tracking is best-effort.
	_ = err
}

// UpdateWaypoint records a waypoint with coordinates.
func (t *Tracker) UpdateWaypoint(name string, lat, lon float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if name == "" || (lat == 0 && lon == 0) {
		return
	}

	// Check if waypoint exists.
	var exists bool
	_ = t.db.QueryRow("SELECT 1 FROM waypoints WHERE name = ?", name).Scan(&exists)

	if !exists {
		// New waypoint.
		_, err := t.db.Exec(`
			INSERT INTO waypoints (name, latitude, longitude)
			VALUES (?, ?, ?)
		`, name, lat, lon)

		if err == nil && t.onWaypointNew != nil {
			t.onWaypointNew(&Waypoint{
				Name:        name,
				Latitude:    lat,
				Longitude:   lon,
				SourceCount: 1,
				FirstSeen:   time.Now(),
				LastSeen:    time.Now(),
			})
		}
	} else {
		// Update existing waypoint count.
		_, _ = t.db.Exec(`
			UPDATE waypoints SET
				source_count = source_count + 1,
				last_seen = CURRENT_TIMESTAMP
			WHERE name = ?
		`, name)
	}
}

// UpdateATIS updates the current ATIS for an airport.
func (t *Tracker) UpdateATIS(atis *ATIS) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if atis.AirportICAO == "" || atis.Letter == "" {
		return
	}

	// Check if ATIS changed.
	var currentLetter sql.NullString
	_ = t.db.QueryRow("SELECT letter FROM atis_current WHERE airport_icao = ?", atis.AirportICAO).Scan(&currentLetter)

	runwaysJSON, _ := json.Marshal(atis.Runways)
	approachesJSON, _ := json.Marshal(atis.Approaches)
	remarksJSON, _ := json.Marshal(atis.Remarks)

	// Update or insert current ATIS.
	_, err := t.db.Exec(`
		INSERT INTO atis_current (airport_icao, letter, atis_type, atis_time, raw_text, runways, approaches,
		                          wind, visibility, clouds, temperature, dew_point, qnh, remarks)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(airport_icao) DO UPDATE SET
			letter = excluded.letter,
			atis_type = excluded.atis_type,
			atis_time = excluded.atis_time,
			raw_text = excluded.raw_text,
			runways = excluded.runways,
			approaches = excluded.approaches,
			wind = excluded.wind,
			visibility = excluded.visibility,
			clouds = excluded.clouds,
			temperature = excluded.temperature,
			dew_point = excluded.dew_point,
			qnh = excluded.qnh,
			remarks = excluded.remarks,
			updated_at = CURRENT_TIMESTAMP,
			synced_at = NULL
	`,
		atis.AirportICAO, atis.Letter, atis.ATISType, atis.ATISTime, atis.RawText,
		string(runwaysJSON), string(approachesJSON),
		atis.Wind, atis.Visibility, atis.Clouds, atis.Temperature, atis.DewPoint,
		atis.QNH, string(remarksJSON),
	)

	if err != nil {
		return
	}

	// If the letter changed, archive to history and trigger callback.
	if !currentLetter.Valid || currentLetter.String != atis.Letter {
		_, _ = t.db.Exec(`
			INSERT INTO atis_history (airport_icao, letter, atis_type, atis_time, raw_text, runways, approaches,
			                          wind, visibility, clouds, temperature, dew_point, qnh, remarks)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			atis.AirportICAO, atis.Letter, atis.ATISType, atis.ATISTime, atis.RawText,
			string(runwaysJSON), string(approachesJSON),
			atis.Wind, atis.Visibility, atis.Clouds, atis.Temperature, atis.DewPoint,
			atis.QNH, string(remarksJSON),
		)

		if t.onATISChanged != nil {
			t.onATISChanged(atis)
		}
	}
}

// GetFlight returns the current state of a flight by key.
func (t *Tracker) GetFlight(key string) *FlightState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.flights[key]
}

// GetAllFlights returns all current flight states.
func (t *Tracker) GetAllFlights() []*FlightState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*FlightState, 0, len(t.flights))
	for _, fs := range t.flights {
		result = append(result, fs)
	}
	return result
}

// GetActiveFlights returns flights seen within the given duration.
func (t *Tracker) GetActiveFlights(within time.Duration) []*FlightState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-within)
	result := make([]*FlightState, 0)
	for _, fs := range t.flights {
		if fs.LastSeen.After(cutoff) {
			result = append(result, fs)
		}
	}
	return result
}

// CleanupStale removes flight states older than the given duration.
func (t *Tracker) CleanupStale(olderThan time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	removed := 0

	for key, fs := range t.flights {
		if fs.LastSeen.Before(cutoff) {
			delete(t.flights, key)
			removed++
		}
	}

	// Also cleanup database.
	_, _ = t.db.Exec("DELETE FROM flight_state WHERE last_seen < ?", cutoff)

	return removed
}

// GetUnsyncedAircraft returns aircraft that haven't been synced.
func (t *Tracker) GetUnsyncedAircraft() ([]*Aircraft, error) {
	rows, err := t.db.Query(`
		SELECT icao_hex, registration, type_code, operator, first_seen, last_seen, msg_count
		FROM aircraft WHERE synced_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []*Aircraft
	for rows.Next() {
		var a Aircraft
		var typeCode, operator sql.NullString
		if err := rows.Scan(&a.ICAOHex, &a.Registration, &typeCode, &operator,
			&a.FirstSeen, &a.LastSeen, &a.MsgCount); err != nil {
			continue
		}
		a.TypeCode = typeCode.String
		a.Operator = operator.String
		result = append(result, &a)
	}
	return result, rows.Err()
}

// GetUnsyncedWaypoints returns waypoints that haven't been synced.
func (t *Tracker) GetUnsyncedWaypoints() ([]*Waypoint, error) {
	rows, err := t.db.Query(`
		SELECT name, latitude, longitude, source_count, first_seen, last_seen
		FROM waypoints WHERE synced_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []*Waypoint
	for rows.Next() {
		var w Waypoint
		if err := rows.Scan(&w.Name, &w.Latitude, &w.Longitude, &w.SourceCount,
			&w.FirstSeen, &w.LastSeen); err != nil {
			continue
		}
		result = append(result, &w)
	}
	return result, rows.Err()
}

// GetUnsyncedRoutes returns routes that haven't been synced.
func (t *Tracker) GetUnsyncedRoutes() ([]*Route, error) {
	rows, err := t.db.Query(`
		SELECT id, flight_pattern, origin_icao, dest_icao,
		       observation_count, first_seen, last_seen
		FROM routes WHERE synced_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []*Route
	for rows.Next() {
		var r Route
		if err := rows.Scan(&r.ID, &r.FlightPattern, &r.OriginICAO, &r.DestICAO,
			&r.ObservationCount, &r.FirstSeen, &r.LastSeen); err != nil {
			continue
		}
		result = append(result, &r)
	}
	return result, rows.Err()
}

// GetRouteAircraft returns all aircraft seen on a specific route.
func (t *Tracker) GetRouteAircraft(routeID int64) ([]*RouteAircraft, error) {
	rows, err := t.db.Query(`
		SELECT route_id, registration, observation_count, first_seen, last_seen
		FROM route_aircraft WHERE route_id = ?
	`, routeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []*RouteAircraft
	for rows.Next() {
		var ra RouteAircraft
		if err := rows.Scan(&ra.RouteID, &ra.Registration, &ra.ObservationCount,
			&ra.FirstSeen, &ra.LastSeen); err != nil {
			continue
		}
		result = append(result, &ra)
	}
	return result, rows.Err()
}

// GetAircraftRoutes returns all routes an aircraft has been seen on.
func (t *Tracker) GetAircraftRoutes(registration string) ([]*Route, error) {
	rows, err := t.db.Query(`
		SELECT r.id, r.flight_pattern, r.origin_icao, r.dest_icao,
		       r.observation_count, r.first_seen, r.last_seen
		FROM routes r
		JOIN route_aircraft ra ON r.id = ra.route_id
		WHERE ra.registration = ?
	`, registration)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []*Route
	for rows.Next() {
		var r Route
		if err := rows.Scan(&r.ID, &r.FlightPattern, &r.OriginICAO, &r.DestICAO,
			&r.ObservationCount, &r.FirstSeen, &r.LastSeen); err != nil {
			continue
		}
		result = append(result, &r)
	}
	return result, rows.Err()
}

// MarkSynced marks records as synced.
func (t *Tracker) MarkAircraftSynced(icaoHex string) error {
	_, err := t.db.Exec("UPDATE aircraft SET synced_at = CURRENT_TIMESTAMP WHERE icao_hex = ?", icaoHex)
	return err
}

func (t *Tracker) MarkWaypointSynced(name string) error {
	_, err := t.db.Exec("UPDATE waypoints SET synced_at = CURRENT_TIMESTAMP WHERE name = ?", name)
	return err
}

func (t *Tracker) MarkRouteSynced(id int64) error {
	_, err := t.db.Exec("UPDATE routes SET synced_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}

// Stats returns statistics about tracked data.
type Stats struct {
	ActiveFlights   int
	TotalAircraft   int
	TotalWaypoints  int
	TotalRoutes     int
	UnsyncedCount   int
}

func (t *Tracker) GetStats() Stats {
	t.mu.RLock()
	activeFlights := len(t.flights)
	t.mu.RUnlock()

	var stats Stats
	stats.ActiveFlights = activeFlights

	_ = t.db.QueryRow("SELECT COUNT(*) FROM aircraft").Scan(&stats.TotalAircraft)
	_ = t.db.QueryRow("SELECT COUNT(*) FROM waypoints").Scan(&stats.TotalWaypoints)
	_ = t.db.QueryRow("SELECT COUNT(*) FROM routes").Scan(&stats.TotalRoutes)
	_ = t.db.QueryRow(`
		SELECT (SELECT COUNT(*) FROM aircraft WHERE synced_at IS NULL) +
		       (SELECT COUNT(*) FROM waypoints WHERE synced_at IS NULL) +
		       (SELECT COUNT(*) FROM routes WHERE synced_at IS NULL)
	`).Scan(&stats.UnsyncedCount)

	return stats
}
