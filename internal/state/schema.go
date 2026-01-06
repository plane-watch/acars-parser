// Package state provides flight state tracking and reference data management.
package state

// schema contains the SQLite table definitions for state tracking.
const schema = `
-- Reference data: Aircraft (ICAO hex to registration mapping).
CREATE TABLE IF NOT EXISTS aircraft (
	icao_hex     TEXT PRIMARY KEY,
	registration TEXT NOT NULL,
	type_code    TEXT,
	operator     TEXT,
	first_seen   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	msg_count    INTEGER NOT NULL DEFAULT 1,
	synced_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_aircraft_registration ON aircraft(registration);
CREATE INDEX IF NOT EXISTS idx_aircraft_synced ON aircraft(synced_at);

-- Reference data: Waypoints with coordinates.
CREATE TABLE IF NOT EXISTS waypoints (
	name         TEXT PRIMARY KEY,
	latitude     REAL NOT NULL,
	longitude    REAL NOT NULL,
	source_count INTEGER NOT NULL DEFAULT 1,
	first_seen   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	synced_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_waypoints_synced ON waypoints(synced_at);

-- Reference data: Learned route patterns.
CREATE TABLE IF NOT EXISTS routes (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	flight_pattern    TEXT NOT NULL,
	origin_icao       TEXT NOT NULL,
	dest_icao         TEXT NOT NULL,
	observation_count INTEGER NOT NULL DEFAULT 1,
	first_seen        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	synced_at         DATETIME,
	UNIQUE(flight_pattern, origin_icao, dest_icao)
);

CREATE INDEX IF NOT EXISTS idx_routes_pattern ON routes(flight_pattern);
CREATE INDEX IF NOT EXISTS idx_routes_synced ON routes(synced_at);

-- Reference data: Aircraft seen on routes (junction table).
CREATE TABLE IF NOT EXISTS route_aircraft (
	route_id          INTEGER NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
	registration      TEXT NOT NULL,
	observation_count INTEGER NOT NULL DEFAULT 1,
	first_seen        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (route_id, registration)
);

CREATE INDEX IF NOT EXISTS idx_route_aircraft_registration ON route_aircraft(registration);

-- Operational: Current ATIS per airport.
CREATE TABLE IF NOT EXISTS atis_current (
	airport_icao TEXT PRIMARY KEY,
	letter       TEXT NOT NULL,
	atis_type    TEXT,  -- ARR, DEP, or NULL for combined.
	atis_time    TEXT,
	raw_text     TEXT,  -- Full raw ATIS text.
	runways      TEXT,  -- JSON array.
	approaches   TEXT,  -- JSON array.
	wind         TEXT,
	visibility   TEXT,
	clouds       TEXT,
	temperature  TEXT,
	dew_point    TEXT,
	qnh          TEXT,
	remarks      TEXT,  -- JSON array.
	updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	synced_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_atis_current_synced ON atis_current(synced_at);

-- Operational: ATIS history.
CREATE TABLE IF NOT EXISTS atis_history (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	airport_icao TEXT NOT NULL,
	letter       TEXT NOT NULL,
	atis_type    TEXT,
	atis_time    TEXT,
	raw_text     TEXT,
	runways      TEXT,
	approaches   TEXT,
	wind         TEXT,
	visibility   TEXT,
	clouds       TEXT,
	temperature  TEXT,
	dew_point    TEXT,
	qnh          TEXT,
	remarks      TEXT,
	recorded_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_atis_history_airport ON atis_history(airport_icao);
CREATE INDEX IF NOT EXISTS idx_atis_history_time ON atis_history(recorded_at);

-- Ephemeral: Current flight state (for live tracking).
CREATE TABLE IF NOT EXISTS flight_state (
	key           TEXT PRIMARY KEY,  -- ICAO hex or registration.
	icao_hex      TEXT,
	registration  TEXT,
	flight_number TEXT,
	origin        TEXT,
	destination   TEXT,
	latitude      REAL,
	longitude     REAL,
	altitude      INTEGER,
	ground_speed  INTEGER,
	track         INTEGER,
	waypoints     TEXT,  -- JSON array of waypoints seen.
	first_seen    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	msg_count     INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_flight_state_flight ON flight_state(flight_number);
CREATE INDEX IF NOT EXISTS idx_flight_state_last_seen ON flight_state(last_seen);
`
