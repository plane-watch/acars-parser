# ClickHouse + PostgreSQL Migration Design

## Overview

Migrate from SQLite to a two-database architecture:
- **ClickHouse**: Analytical workloads (messages, ATIS history)
- **PostgreSQL**: Mutable state and API-friendly data (aircraft, routes, waypoints, flight state)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      ACARS Parser                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌─────────────────┐         ┌─────────────────┐          │
│   │   ClickHouse    │         │   PostgreSQL    │          │
│   │                 │         │                 │          │
│   │  • messages     │         │  • aircraft     │          │
│   │  • atis_history │         │  • waypoints    │          │
│   │                 │         │  • routes       │          │
│   │  Bulk insert    │         │  • atis_current │          │
│   │  Fast scans     │         │  • flight_state │          │
│   │  Reprocessing   │         │  • golden_*     │          │
│   └─────────────────┘         └─────────────────┘          │
│          ▲                           ▲                      │
│          │                           │                      │
│     Parse/Store              State/Results                  │
│                                      │                      │
│                              ┌───────┴───────┐              │
│                              │  Frontend API │              │
│                              └───────────────┘              │
└─────────────────────────────────────────────────────────────┘
```

## ClickHouse Schema

```sql
-- Messages table - the core analytical store
CREATE TABLE messages (
    id              UInt64,
    timestamp       DateTime64(3),
    label           LowCardinality(String),
    parser_type     LowCardinality(String),
    flight          LowCardinality(String),
    tail            LowCardinality(String),
    origin          LowCardinality(String),
    destination     LowCardinality(String),
    raw_text        String,
    parsed_json     String,
    missing_fields  String,
    confidence      Float32,
    created_at      DateTime64(3) DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (parser_type, label, timestamp, id)
SETTINGS index_granularity = 8192;

-- Full-text search via tokenbf bloom filter
ALTER TABLE messages ADD INDEX idx_raw_text_bloom raw_text
    TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 1;

-- ATIS history - append-only historical record
CREATE TABLE atis_history (
    id              UInt64,
    airport_icao    LowCardinality(String),
    letter          LowCardinality(String),
    atis_type       LowCardinality(Nullable(String)),
    atis_time       String,
    raw_text        String,
    runways         String,
    approaches      String,
    wind            String,
    visibility      String,
    clouds          String,
    temperature     String,
    dew_point       String,
    qnh             String,
    remarks         String,
    recorded_at     DateTime64(3) DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (airport_icao, recorded_at, id);
```

## PostgreSQL Schema

```sql
-- Reference data: Aircraft
CREATE TABLE aircraft (
    icao_hex        TEXT PRIMARY KEY,
    registration    TEXT NOT NULL,
    type_code       TEXT,
    operator        TEXT,
    first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    msg_count       INTEGER NOT NULL DEFAULT 1,
    synced_at       TIMESTAMPTZ
);

CREATE INDEX idx_aircraft_registration ON aircraft(registration);
CREATE INDEX idx_aircraft_synced ON aircraft(synced_at);

-- Reference data: Waypoints
CREATE TABLE waypoints (
    name            TEXT PRIMARY KEY,
    latitude        DOUBLE PRECISION NOT NULL,
    longitude       DOUBLE PRECISION NOT NULL,
    source_count    INTEGER NOT NULL DEFAULT 1,
    first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    synced_at       TIMESTAMPTZ
);

CREATE INDEX idx_waypoints_synced ON waypoints(synced_at);

-- Reference data: Routes
CREATE TABLE routes (
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

CREATE INDEX idx_routes_pattern ON routes(flight_pattern);
CREATE INDEX idx_routes_synced ON routes(synced_at);

-- Reference data: Route legs
CREATE TABLE route_legs (
    route_id            INTEGER NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    sequence            INTEGER NOT NULL,
    origin_icao         TEXT NOT NULL,
    dest_icao           TEXT NOT NULL,
    observation_count   INTEGER NOT NULL DEFAULT 1,
    first_seen          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (route_id, sequence)
);

CREATE INDEX idx_route_legs_airports ON route_legs(origin_icao, dest_icao);

-- Reference data: Aircraft on routes
CREATE TABLE route_aircraft (
    route_id            INTEGER NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    registration        TEXT NOT NULL,
    observation_count   INTEGER NOT NULL DEFAULT 1,
    first_seen          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (route_id, registration)
);

CREATE INDEX idx_route_aircraft_registration ON route_aircraft(registration);

-- Reference data: Callsign prefixes
CREATE TABLE aircraft_callsigns (
    registration        TEXT PRIMARY KEY,
    iata_prefix         TEXT NOT NULL,
    icao_prefix         TEXT NOT NULL,
    observation_count   INTEGER NOT NULL DEFAULT 1,
    first_seen          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_aircraft_callsigns_iata ON aircraft_callsigns(iata_prefix);

-- Operational: Current ATIS
CREATE TABLE atis_current (
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

CREATE INDEX idx_atis_current_synced ON atis_current(synced_at);

-- Ephemeral: Flight state
CREATE TABLE flight_state (
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

CREATE INDEX idx_flight_state_flight ON flight_state(flight_number);
CREATE INDEX idx_flight_state_last_seen ON flight_state(last_seen);

-- Golden annotations (references ClickHouse message IDs)
CREATE TABLE golden_annotations (
    message_id      BIGINT PRIMARY KEY,
    is_golden       BOOLEAN NOT NULL DEFAULT FALSE,
    annotation      TEXT,
    expected_json   JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_golden_is_golden ON golden_annotations(is_golden) WHERE is_golden = TRUE;
```

## Go Driver Layer

Dependencies:
- ClickHouse: `github.com/ClickHouse/clickhouse-go/v2`
- PostgreSQL: `github.com/jackc/pgx/v5`

```go
type Config struct {
    ClickHouse struct {
        Host     string  // localhost
        Port     int     // 9000 (native protocol)
        Database string  // acars
        User     string  // default
        Password string
    }
    Postgres struct {
        Host     string  // localhost
        Port     int     // 5432
        Database string  // acars_state
        User     string
        Password string
    }
}

type DB struct {
    CH *clickhouse.Conn  // ClickHouse for messages
    PG *pgxpool.Pool     // PostgreSQL for state
}
```

## Migration Strategy

1. **Create schemas** - Run ClickHouse + PostgreSQL DDL
2. **Migrate messages** - Batch read from SQLite (10k rows), bulk insert to ClickHouse
3. **Migrate state** - aircraft, waypoints, routes to PostgreSQL
4. **Migrate golden annotations** - Extract and insert to PostgreSQL
5. **Verify counts**

CLI command: `acars migrate --sqlite-path ./state.db --clickhouse localhost:9000 --postgres localhost:5432`

Features:
- Batched reads (10k rows)
- Resumable from last ID
- Dry-run mode
- Keep SQLite as backup

## Error Handling

- **Connection failures**: Retry with backoff, fail fast if both DBs down
- **Bulk insert failures**: Retry batch, log failed rows
- **Transactions**: PostgreSQL for multi-table updates; ClickHouse append-only

## Testing

- Integration tests against Docker containers
- Unit tests with mocked interfaces
