package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresConfig holds PostgreSQL connection settings.
type PostgresConfig struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
}

// PostgresDB wraps a PostgreSQL connection pool for state storage.
type PostgresDB struct {
	pool *pgxpool.Pool
}

// OpenPostgres opens a connection pool to PostgreSQL.
func OpenPostgres(ctx context.Context, cfg PostgresConfig) (*PostgresDB, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	poolCfg.MaxConns = 10
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// Test the connection.
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresDB{pool: pool}, nil
}

// Close closes the PostgreSQL connection pool.
func (d *PostgresDB) Close() {
	d.pool.Close()
}

// CreateSchema creates the PostgreSQL tables.
func (d *PostgresDB) CreateSchema(ctx context.Context) error {
	schema := `
	-- Reference data: Aircraft
	CREATE TABLE IF NOT EXISTS aircraft (
		icao_hex        TEXT PRIMARY KEY,
		registration    TEXT NOT NULL,
		type_code       TEXT,
		operator        TEXT,
		first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		msg_count       INTEGER NOT NULL DEFAULT 1,
		synced_at       TIMESTAMPTZ
	);

	CREATE INDEX IF NOT EXISTS idx_aircraft_registration ON aircraft(registration);
	CREATE INDEX IF NOT EXISTS idx_aircraft_synced ON aircraft(synced_at);

	-- Reference data: Waypoints
	CREATE TABLE IF NOT EXISTS waypoints (
		name            TEXT PRIMARY KEY,
		latitude        DOUBLE PRECISION NOT NULL,
		longitude       DOUBLE PRECISION NOT NULL,
		source_count    INTEGER NOT NULL DEFAULT 1,
		first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		synced_at       TIMESTAMPTZ
	);

	CREATE INDEX IF NOT EXISTS idx_waypoints_synced ON waypoints(synced_at);

	-- Reference data: Routes
	CREATE TABLE IF NOT EXISTS routes (
		id                  SERIAL PRIMARY KEY,
		flight_pattern      TEXT NOT NULL,
		origin_icao         TEXT NOT NULL,
		dest_icao           TEXT NOT NULL,
		is_multi_stop       BOOLEAN NOT NULL DEFAULT FALSE,
		observation_count   INTEGER NOT NULL DEFAULT 1,
		first_seen          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		synced_at           TIMESTAMPTZ,
		UNIQUE(flight_pattern, origin_icao, dest_icao)
	);

	CREATE INDEX IF NOT EXISTS idx_routes_pattern ON routes(flight_pattern);
	CREATE INDEX IF NOT EXISTS idx_routes_synced ON routes(synced_at);

	-- Reference data: Route legs
	CREATE TABLE IF NOT EXISTS route_legs (
		route_id            INTEGER NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
		sequence            INTEGER NOT NULL,
		origin_icao         TEXT NOT NULL,
		dest_icao           TEXT NOT NULL,
		observation_count   INTEGER NOT NULL DEFAULT 1,
		first_seen          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (route_id, sequence)
	);

	CREATE INDEX IF NOT EXISTS idx_route_legs_airports ON route_legs(origin_icao, dest_icao);

	-- Reference data: Aircraft on routes
	CREATE TABLE IF NOT EXISTS route_aircraft (
		route_id            INTEGER NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
		registration        TEXT NOT NULL,
		observation_count   INTEGER NOT NULL DEFAULT 1,
		first_seen          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (route_id, registration)
	);

	CREATE INDEX IF NOT EXISTS idx_route_aircraft_registration ON route_aircraft(registration);

	-- Reference data: Callsign prefixes
	CREATE TABLE IF NOT EXISTS aircraft_callsigns (
		registration        TEXT PRIMARY KEY,
		iata_prefix         TEXT NOT NULL,
		icao_prefix         TEXT NOT NULL,
		observation_count   INTEGER NOT NULL DEFAULT 1,
		first_seen          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen           TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_aircraft_callsigns_iata ON aircraft_callsigns(iata_prefix);

	-- Operational: Current ATIS
	CREATE TABLE IF NOT EXISTS atis_current (
		airport_icao    TEXT PRIMARY KEY,
		letter          TEXT NOT NULL,
		atis_type       TEXT,
		atis_time       TEXT,
		raw_text        TEXT,
		runways         JSONB,
		approaches      JSONB,
		wind            TEXT,
		visibility      TEXT,
		clouds          TEXT,
		temperature     TEXT,
		dew_point       TEXT,
		qnh             TEXT,
		remarks         JSONB,
		updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		synced_at       TIMESTAMPTZ
	);

	CREATE INDEX IF NOT EXISTS idx_atis_current_synced ON atis_current(synced_at);

	-- Ephemeral: Flight state
	CREATE TABLE IF NOT EXISTS flight_state (
		key             TEXT PRIMARY KEY,
		icao_hex        TEXT,
		registration    TEXT,
		flight_number   TEXT,
		origin          TEXT,
		destination     TEXT,
		latitude        DOUBLE PRECISION,
		longitude       DOUBLE PRECISION,
		altitude        INTEGER,
		ground_speed    INTEGER,
		track           INTEGER,
		waypoints       JSONB,
		first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		msg_count       INTEGER NOT NULL DEFAULT 1
	);

	CREATE INDEX IF NOT EXISTS idx_flight_state_flight ON flight_state(flight_number);
	CREATE INDEX IF NOT EXISTS idx_flight_state_last_seen ON flight_state(last_seen);

	-- Golden annotations (references ClickHouse message IDs)
	CREATE TABLE IF NOT EXISTS golden_annotations (
		message_id      BIGINT PRIMARY KEY,
		is_golden       BOOLEAN NOT NULL DEFAULT FALSE,
		annotation      TEXT,
		expected_json   JSONB,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`

	_, err := d.pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	// Create partial index separately (IF NOT EXISTS syntax differs).
	_, _ = d.pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_golden_is_golden ON golden_annotations(is_golden) WHERE is_golden = TRUE`)

	return nil
}

// Aircraft represents an aircraft record.
type Aircraft struct {
	ICAOHex      string
	Registration string
	TypeCode     string
	Operator     string
	FirstSeen    time.Time
	LastSeen     time.Time
	MsgCount     int
	SyncedAt     *time.Time
}

// UpsertAircraft inserts or updates an aircraft record.
func (d *PostgresDB) UpsertAircraft(ctx context.Context, a Aircraft) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO aircraft (icao_hex, registration, type_code, operator, first_seen, last_seen, msg_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (icao_hex) DO UPDATE SET
			registration = EXCLUDED.registration,
			type_code = COALESCE(EXCLUDED.type_code, aircraft.type_code),
			operator = COALESCE(EXCLUDED.operator, aircraft.operator),
			last_seen = EXCLUDED.last_seen,
			msg_count = aircraft.msg_count + 1
	`, a.ICAOHex, a.Registration, a.TypeCode, a.Operator, a.FirstSeen, a.LastSeen, a.MsgCount)
	return err
}

// GetAircraft retrieves an aircraft by ICAO hex.
func (d *PostgresDB) GetAircraft(ctx context.Context, icaoHex string) (*Aircraft, error) {
	var a Aircraft
	var syncedAt *time.Time
	err := d.pool.QueryRow(ctx, `
		SELECT icao_hex, registration, type_code, operator, first_seen, last_seen, msg_count, synced_at
		FROM aircraft WHERE icao_hex = $1
	`, icaoHex).Scan(&a.ICAOHex, &a.Registration, &a.TypeCode, &a.Operator, &a.FirstSeen, &a.LastSeen, &a.MsgCount, &syncedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.SyncedAt = syncedAt
	return &a, nil
}

// GetAircraftByRegistration retrieves an aircraft by registration.
func (d *PostgresDB) GetAircraftByRegistration(ctx context.Context, registration string) (*Aircraft, error) {
	var a Aircraft
	var syncedAt *time.Time
	err := d.pool.QueryRow(ctx, `
		SELECT icao_hex, registration, type_code, operator, first_seen, last_seen, msg_count, synced_at
		FROM aircraft WHERE registration = $1
	`, registration).Scan(&a.ICAOHex, &a.Registration, &a.TypeCode, &a.Operator, &a.FirstSeen, &a.LastSeen, &a.MsgCount, &syncedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.SyncedAt = syncedAt
	return &a, nil
}

// Waypoint represents a waypoint record.
type Waypoint struct {
	Name        string
	Latitude    float64
	Longitude   float64
	SourceCount int
	FirstSeen   time.Time
	LastSeen    time.Time
	SyncedAt    *time.Time
}

// UpsertWaypoint inserts or updates a waypoint record.
func (d *PostgresDB) UpsertWaypoint(ctx context.Context, w Waypoint) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO waypoints (name, latitude, longitude, source_count, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name) DO UPDATE SET
			latitude = EXCLUDED.latitude,
			longitude = EXCLUDED.longitude,
			source_count = waypoints.source_count + 1,
			last_seen = EXCLUDED.last_seen
	`, w.Name, w.Latitude, w.Longitude, w.SourceCount, w.FirstSeen, w.LastSeen)
	return err
}

// GetWaypoint retrieves a waypoint by name.
func (d *PostgresDB) GetWaypoint(ctx context.Context, name string) (*Waypoint, error) {
	var w Waypoint
	var syncedAt *time.Time
	err := d.pool.QueryRow(ctx, `
		SELECT name, latitude, longitude, source_count, first_seen, last_seen, synced_at
		FROM waypoints WHERE name = $1
	`, name).Scan(&w.Name, &w.Latitude, &w.Longitude, &w.SourceCount, &w.FirstSeen, &w.LastSeen, &syncedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	w.SyncedAt = syncedAt
	return &w, nil
}

// Route represents a route record.
type Route struct {
	ID               int
	FlightPattern    string
	OriginICAO       string
	DestICAO         string
	IsMultiStop      bool
	ObservationCount int
	FirstSeen        time.Time
	LastSeen         time.Time
	SyncedAt         *time.Time
}

// UpsertRoute inserts or updates a route record, returning the route ID.
func (d *PostgresDB) UpsertRoute(ctx context.Context, r Route) (int, error) {
	var id int
	err := d.pool.QueryRow(ctx, `
		INSERT INTO routes (flight_pattern, origin_icao, dest_icao, is_multi_stop, observation_count, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (flight_pattern, origin_icao, dest_icao) DO UPDATE SET
			is_multi_stop = EXCLUDED.is_multi_stop,
			observation_count = routes.observation_count + 1,
			last_seen = EXCLUDED.last_seen
		RETURNING id
	`, r.FlightPattern, r.OriginICAO, r.DestICAO, r.IsMultiStop, r.ObservationCount, r.FirstSeen, r.LastSeen).Scan(&id)
	return id, err
}

// RouteLeg represents a leg of a route.
type RouteLeg struct {
	RouteID          int
	Sequence         int
	OriginICAO       string
	DestICAO         string
	ObservationCount int
	FirstSeen        time.Time
	LastSeen         time.Time
}

// UpsertRouteLeg inserts or updates a route leg.
func (d *PostgresDB) UpsertRouteLeg(ctx context.Context, leg RouteLeg) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO route_legs (route_id, sequence, origin_icao, dest_icao, observation_count, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (route_id, sequence) DO UPDATE SET
			origin_icao = EXCLUDED.origin_icao,
			dest_icao = EXCLUDED.dest_icao,
			observation_count = route_legs.observation_count + 1,
			last_seen = EXCLUDED.last_seen
	`, leg.RouteID, leg.Sequence, leg.OriginICAO, leg.DestICAO, leg.ObservationCount, leg.FirstSeen, leg.LastSeen)
	return err
}

// RouteAircraft represents an aircraft seen on a route.
type RouteAircraft struct {
	RouteID          int
	Registration     string
	ObservationCount int
	FirstSeen        time.Time
	LastSeen         time.Time
}

// UpsertRouteAircraft inserts or updates a route-aircraft association.
func (d *PostgresDB) UpsertRouteAircraft(ctx context.Context, ra RouteAircraft) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO route_aircraft (route_id, registration, observation_count, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (route_id, registration) DO UPDATE SET
			observation_count = route_aircraft.observation_count + 1,
			last_seen = EXCLUDED.last_seen
	`, ra.RouteID, ra.Registration, ra.ObservationCount, ra.FirstSeen, ra.LastSeen)
	return err
}

// AircraftCallsign represents a callsign mapping for an aircraft.
type AircraftCallsign struct {
	Registration     string
	IATAPrefix       string
	ICAOPrefix       string
	ObservationCount int
	FirstSeen        time.Time
	LastSeen         time.Time
}

// UpsertAircraftCallsign inserts or updates a callsign mapping.
func (d *PostgresDB) UpsertAircraftCallsign(ctx context.Context, cs AircraftCallsign) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO aircraft_callsigns (registration, iata_prefix, icao_prefix, observation_count, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (registration) DO UPDATE SET
			iata_prefix = EXCLUDED.iata_prefix,
			icao_prefix = EXCLUDED.icao_prefix,
			observation_count = aircraft_callsigns.observation_count + 1,
			last_seen = EXCLUDED.last_seen
	`, cs.Registration, cs.IATAPrefix, cs.ICAOPrefix, cs.ObservationCount, cs.FirstSeen, cs.LastSeen)
	return err
}

// ATISCurrent represents current ATIS for an airport.
type ATISCurrent struct {
	AirportICAO string
	Letter      string
	ATISType    string
	ATISTime    string
	RawText     string
	Runways     []string
	Approaches  []string
	Wind        string
	Visibility  string
	Clouds      string
	Temperature string
	DewPoint    string
	QNH         string
	Remarks     []string
	UpdatedAt   time.Time
	SyncedAt    *time.Time
}

// UpsertATISCurrent inserts or updates current ATIS for an airport.
func (d *PostgresDB) UpsertATISCurrent(ctx context.Context, a ATISCurrent) error {
	runwaysJSON, _ := json.Marshal(a.Runways)
	approachesJSON, _ := json.Marshal(a.Approaches)
	remarksJSON, _ := json.Marshal(a.Remarks)

	_, err := d.pool.Exec(ctx, `
		INSERT INTO atis_current (airport_icao, letter, atis_type, atis_time, raw_text, runways, approaches, wind, visibility, clouds, temperature, dew_point, qnh, remarks, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (airport_icao) DO UPDATE SET
			letter = EXCLUDED.letter,
			atis_type = EXCLUDED.atis_type,
			atis_time = EXCLUDED.atis_time,
			raw_text = EXCLUDED.raw_text,
			runways = EXCLUDED.runways,
			approaches = EXCLUDED.approaches,
			wind = EXCLUDED.wind,
			visibility = EXCLUDED.visibility,
			clouds = EXCLUDED.clouds,
			temperature = EXCLUDED.temperature,
			dew_point = EXCLUDED.dew_point,
			qnh = EXCLUDED.qnh,
			remarks = EXCLUDED.remarks,
			updated_at = EXCLUDED.updated_at
	`, a.AirportICAO, a.Letter, a.ATISType, a.ATISTime, a.RawText, runwaysJSON, approachesJSON, a.Wind, a.Visibility, a.Clouds, a.Temperature, a.DewPoint, a.QNH, remarksJSON, a.UpdatedAt)
	return err
}

// GetATISCurrent retrieves current ATIS for an airport.
func (d *PostgresDB) GetATISCurrent(ctx context.Context, airportICAO string) (*ATISCurrent, error) {
	var a ATISCurrent
	var runwaysJSON, approachesJSON, remarksJSON []byte
	var syncedAt *time.Time

	err := d.pool.QueryRow(ctx, `
		SELECT airport_icao, letter, atis_type, atis_time, raw_text, runways, approaches, wind, visibility, clouds, temperature, dew_point, qnh, remarks, updated_at, synced_at
		FROM atis_current WHERE airport_icao = $1
	`, airportICAO).Scan(&a.AirportICAO, &a.Letter, &a.ATISType, &a.ATISTime, &a.RawText, &runwaysJSON, &approachesJSON, &a.Wind, &a.Visibility, &a.Clouds, &a.Temperature, &a.DewPoint, &a.QNH, &remarksJSON, &a.UpdatedAt, &syncedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(runwaysJSON, &a.Runways)
	_ = json.Unmarshal(approachesJSON, &a.Approaches)
	_ = json.Unmarshal(remarksJSON, &a.Remarks)
	a.SyncedAt = syncedAt
	return &a, nil
}

// FlightState represents current state of a flight.
type FlightState struct {
	Key          string
	ICAOHex      string
	Registration string
	FlightNumber string
	Origin       string
	Destination  string
	Latitude     *float64
	Longitude    *float64
	Altitude     *int
	GroundSpeed  *int
	Track        *int
	Waypoints    []string
	FirstSeen    time.Time
	LastSeen     time.Time
	MsgCount     int
}

// UpsertFlightState inserts or updates flight state.
func (d *PostgresDB) UpsertFlightState(ctx context.Context, fs FlightState) error {
	waypointsJSON, _ := json.Marshal(fs.Waypoints)

	_, err := d.pool.Exec(ctx, `
		INSERT INTO flight_state (key, icao_hex, registration, flight_number, origin, destination, latitude, longitude, altitude, ground_speed, track, waypoints, first_seen, last_seen, msg_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (key) DO UPDATE SET
			icao_hex = COALESCE(EXCLUDED.icao_hex, flight_state.icao_hex),
			registration = COALESCE(EXCLUDED.registration, flight_state.registration),
			flight_number = COALESCE(EXCLUDED.flight_number, flight_state.flight_number),
			origin = COALESCE(EXCLUDED.origin, flight_state.origin),
			destination = COALESCE(EXCLUDED.destination, flight_state.destination),
			latitude = COALESCE(EXCLUDED.latitude, flight_state.latitude),
			longitude = COALESCE(EXCLUDED.longitude, flight_state.longitude),
			altitude = COALESCE(EXCLUDED.altitude, flight_state.altitude),
			ground_speed = COALESCE(EXCLUDED.ground_speed, flight_state.ground_speed),
			track = COALESCE(EXCLUDED.track, flight_state.track),
			waypoints = EXCLUDED.waypoints,
			last_seen = EXCLUDED.last_seen,
			msg_count = flight_state.msg_count + 1
	`, fs.Key, fs.ICAOHex, fs.Registration, fs.FlightNumber, fs.Origin, fs.Destination, fs.Latitude, fs.Longitude, fs.Altitude, fs.GroundSpeed, fs.Track, waypointsJSON, fs.FirstSeen, fs.LastSeen, fs.MsgCount)
	return err
}

// GetFlightState retrieves flight state by key.
func (d *PostgresDB) GetFlightState(ctx context.Context, key string) (*FlightState, error) {
	var fs FlightState
	var waypointsJSON []byte

	err := d.pool.QueryRow(ctx, `
		SELECT key, icao_hex, registration, flight_number, origin, destination, latitude, longitude, altitude, ground_speed, track, waypoints, first_seen, last_seen, msg_count
		FROM flight_state WHERE key = $1
	`, key).Scan(&fs.Key, &fs.ICAOHex, &fs.Registration, &fs.FlightNumber, &fs.Origin, &fs.Destination, &fs.Latitude, &fs.Longitude, &fs.Altitude, &fs.GroundSpeed, &fs.Track, &waypointsJSON, &fs.FirstSeen, &fs.LastSeen, &fs.MsgCount)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(waypointsJSON, &fs.Waypoints)
	return &fs, nil
}

// GoldenAnnotation represents a golden message annotation.
type GoldenAnnotation struct {
	MessageID    int64
	IsGolden     bool
	Annotation   string
	ExpectedJSON map[string]interface{}
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UpsertGoldenAnnotation inserts or updates a golden annotation.
func (d *PostgresDB) UpsertGoldenAnnotation(ctx context.Context, g GoldenAnnotation) error {
	expectedJSON, _ := json.Marshal(g.ExpectedJSON)

	_, err := d.pool.Exec(ctx, `
		INSERT INTO golden_annotations (message_id, is_golden, annotation, expected_json, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (message_id) DO UPDATE SET
			is_golden = EXCLUDED.is_golden,
			annotation = EXCLUDED.annotation,
			expected_json = EXCLUDED.expected_json,
			updated_at = EXCLUDED.updated_at
	`, g.MessageID, g.IsGolden, g.Annotation, expectedJSON, g.CreatedAt, g.UpdatedAt)
	return err
}

// GetGoldenAnnotation retrieves a golden annotation by message ID.
func (d *PostgresDB) GetGoldenAnnotation(ctx context.Context, messageID int64) (*GoldenAnnotation, error) {
	var g GoldenAnnotation
	var expectedJSON []byte

	err := d.pool.QueryRow(ctx, `
		SELECT message_id, is_golden, annotation, expected_json, created_at, updated_at
		FROM golden_annotations WHERE message_id = $1
	`, messageID).Scan(&g.MessageID, &g.IsGolden, &g.Annotation, &expectedJSON, &g.CreatedAt, &g.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(expectedJSON, &g.ExpectedJSON)
	return &g, nil
}

// GetGoldenMessages retrieves all golden annotations.
func (d *PostgresDB) GetGoldenMessages(ctx context.Context) ([]GoldenAnnotation, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT message_id, is_golden, annotation, expected_json, created_at, updated_at
		FROM golden_annotations WHERE is_golden = TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var annotations []GoldenAnnotation
	for rows.Next() {
		var g GoldenAnnotation
		var expectedJSON []byte

		if err := rows.Scan(&g.MessageID, &g.IsGolden, &g.Annotation, &expectedJSON, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(expectedJSON, &g.ExpectedJSON)
		annotations = append(annotations, g)
	}

	return annotations, nil
}

// SetGolden marks or unmarks a message as golden.
func (d *PostgresDB) SetGolden(ctx context.Context, messageID int64, golden bool) error {
	now := time.Now()
	_, err := d.pool.Exec(ctx, `
		INSERT INTO golden_annotations (message_id, is_golden, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (message_id) DO UPDATE SET
			is_golden = EXCLUDED.is_golden,
			updated_at = EXCLUDED.updated_at
	`, messageID, golden, now)
	return err
}

// SetAnnotation sets the annotation for a message.
func (d *PostgresDB) SetAnnotation(ctx context.Context, messageID int64, annotation string) error {
	now := time.Now()
	_, err := d.pool.Exec(ctx, `
		INSERT INTO golden_annotations (message_id, annotation, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (message_id) DO UPDATE SET
			annotation = EXCLUDED.annotation,
			updated_at = EXCLUDED.updated_at
	`, messageID, annotation, now)
	return err
}

// Pool returns the underlying connection pool for advanced operations.
func (d *PostgresDB) Pool() *pgxpool.Pool {
	return d.pool
}

// ListWaypoints retrieves all waypoints with at least minSources observations.
func (d *PostgresDB) ListWaypoints(ctx context.Context, minSources int) ([]Waypoint, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT name, latitude, longitude, source_count, first_seen, last_seen, synced_at
		FROM waypoints
		WHERE source_count >= $1
		ORDER BY name
	`, minSources)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var waypoints []Waypoint
	for rows.Next() {
		var w Waypoint
		if err := rows.Scan(&w.Name, &w.Latitude, &w.Longitude, &w.SourceCount, &w.FirstSeen, &w.LastSeen, &w.SyncedAt); err != nil {
			return nil, err
		}
		waypoints = append(waypoints, w)
	}
	return waypoints, rows.Err()
}

// ListRoutes retrieves all routes with at least minObservations.
func (d *PostgresDB) ListRoutes(ctx context.Context, minObservations int) ([]Route, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, flight_pattern, origin_icao, dest_icao, is_multi_stop, observation_count, first_seen, last_seen, synced_at
		FROM routes
		WHERE observation_count >= $1
		ORDER BY flight_pattern
	`, minObservations)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		var r Route
		if err := rows.Scan(&r.ID, &r.FlightPattern, &r.OriginICAO, &r.DestICAO, &r.IsMultiStop, &r.ObservationCount, &r.FirstSeen, &r.LastSeen, &r.SyncedAt); err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	return routes, rows.Err()
}

// GetRouteLegs retrieves all legs for a route.
func (d *PostgresDB) GetRouteLegs(ctx context.Context, routeID int) ([]RouteLeg, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT route_id, sequence, origin_icao, dest_icao, observation_count, first_seen, last_seen
		FROM route_legs
		WHERE route_id = $1
		ORDER BY sequence
	`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var legs []RouteLeg
	for rows.Next() {
		var l RouteLeg
		if err := rows.Scan(&l.RouteID, &l.Sequence, &l.OriginICAO, &l.DestICAO, &l.ObservationCount, &l.FirstSeen, &l.LastSeen); err != nil {
			return nil, err
		}
		legs = append(legs, l)
	}
	return legs, rows.Err()
}
