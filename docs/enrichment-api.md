# Flight Enrichment API

The enrichment API is a standalone REST server that exposes flight operational data extracted from ACARS messages (route, departure runway, SID, squawk and passenger count). It is intended for ADS-B tracking applications that want to add this data to aircraft positions. The data is read from the `flight_enrichment` table in PostgreSQL.

## Quick Start

```bash
# Build the API server
go build -o bin/enrichment-api ./cmd/enrichment-api

# Run with defaults (connects to PostgreSQL on localhost, database acars_state)
./bin/enrichment-api

# Run with custom settings
./bin/enrichment-api -port 8081 -pg-host db.example.com -pg-password secret
```

## Configuration

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `-port` | - | `8081` | HTTP port |
| `-pg-host` | `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `-pg-port` | `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `-pg-database` | `POSTGRES_DATABASE` | `acars_state` | PostgreSQL database |
| `-pg-user` | `POSTGRES_USER` | `acars` | PostgreSQL user |
| `-pg-password` | `POSTGRES_PASSWORD` | `acars` | PostgreSQL password |
| `-auth` | - | `false` | Enable API key authentication |
| `-api-keys` | - | _(empty)_ | Comma-separated API keys (whitespace around each key is trimmed) |

A flag takes precedence over its environment variable. A non-numeric `POSTGRES_PORT` is ignored and the default is used. API keys can only be supplied via the `-api-keys` flag.

## Authentication

Authentication is optional and disabled by default. When `-auth` is enabled, it applies to **every** endpoint, including `/api/v1/health`. The key is read from the first of these that is present:

1. The `X-API-Key` header
2. The `Authorization: Bearer <key>` header
3. The `api_key` query parameter

| Situation | Status | Body |
|-----------|--------|------|
| No key supplied | `401` | `{"error": "API key required"}` |
| Key supplied but not in `-api-keys` | `403` | `{"error": "Invalid API key"}` |

```bash
./bin/enrichment-api -auth -api-keys "key1,key2,key3"

curl -H "X-API-Key: key1" http://localhost:8081/api/v1/enrichment/7C6CA3
```

CORS headers (`Access-Control-Allow-Origin: *`) are added to every response. An `OPTIONS` preflight request is answered with `200` before authentication is checked.

## Errors

Errors raised by the API handlers use a single JSON format:

```json
{"error": "No enrichment data found"}
```

| Status | When |
|--------|------|
| `400` | Invalid date format, invalid batch JSON, an empty batch, or more than 100 aircraft in a batch |
| `401` | Authentication is enabled and no API key was supplied |
| `403` | Authentication is enabled and the API key is not valid |
| `404` | No enrichment data exists for the requested aircraft or flight |
| `500` | A database error occurred; the message contains the underlying database error text |

Requests that match no route (or use an unsupported method) receive the router's plain-text `404`/`405` response, not the JSON format above.

## API Endpoints

All paths are prefixed with `/api/v1`. "Today" means the current date in UTC. The ICAO hex and callsign path values are converted to upper case before lookup; they are not otherwise validated.

### Health Check

```
GET /api/v1/health
```

```json
{"status": "ok", "time": "2026-01-31T10:00:00Z"}
```

`time` is the current server time in UTC (RFC 3339). The health check does not query the database.

### Get Enrichments by Aircraft

```
GET /api/v1/enrichment/{icao_hex}
```

Returns an **array** of all enrichment records for the aircraft on today's date, ordered by most recently updated first. An aircraft has more than one record if it operated more than one flight that day. If there are no records, the response is `404` with `{"error": "No enrichment data found for aircraft"}` rather than an empty array.

```bash
curl http://localhost:8081/api/v1/enrichment/7C6CA3
```

```json
[
  {
    "icao_hex": "7C6CA3",
    "callsign": "QFA9",
    "flight_date": "2026-01-31",
    "origin": "YPPH",
    "destination": "EGLL",
    "departure_runway": "03",
    "sid": "JULIM6",
    "squawk": "4521",
    "last_updated": "2026-01-31T08:45:00Z"
  }
]
```

### Get Enrichment by Callsign

```
GET /api/v1/enrichment/{icao_hex}/{callsign}
```

Returns a single enrichment **object** for the flight on today's date, or `404` with `{"error": "No enrichment data found"}`. The callsign is matched on its numeric suffix (see [Callsign Matching](#callsign-matching)), so `QF9` and `QFA9` find the same record.

```bash
curl http://localhost:8081/api/v1/enrichment/7C6CA3/QFA9
```

### Get Enrichment by Date

```
GET /api/v1/enrichment/{icao_hex}/{callsign}/{date}
```

The same as the callsign lookup, but for the given date. `date` must be in `YYYY-MM-DD` format and is interpreted as a UTC date. An invalid date returns `400` with `{"error": "Invalid date format (use YYYY-MM-DD)"}`.

```bash
curl http://localhost:8081/api/v1/enrichment/7C6CA3/QFA9/2026-01-30
```

### Batch Lookup

```
POST /api/v1/enrichment/batch
```

Looks up enrichments for up to 100 aircraft on today's date in a single request. There is no date parameter.

**Request body:**

```json
{
  "aircraft": [
    {"icao_hex": "7C6CA3"},
    {"icao_hex": "780AB6", "callsign": "CPA844"}
  ]
}
```

- An entry without a `callsign` returns all of that aircraft's records for today.
- An entry with a `callsign` returns at most one record, matched in the same way as the callsign lookup.
- Entries with an empty `icao_hex` are skipped silently.

**Response (`200`):**

```json
{
  "results": {
    "7C6CA3": [
      {
        "icao_hex": "7C6CA3",
        "callsign": "QFA9",
        "flight_date": "2026-01-31",
        "origin": "YPPH",
        "destination": "EGLL",
        "last_updated": "2026-01-31T08:45:00Z"
      }
    ]
  },
  "errors": {
    "780AB6": "<database error text>"
  }
}
```

- `results` is keyed by upper-case ICAO hex. An aircraft with no data is **omitted** from `results`; it does not appear with an empty array.
- `errors` is keyed by ICAO hex and holds the database error text for any lookup that failed. The key is omitted entirely when no lookup failed.
- The request fails with `400` if the body is not valid JSON (`"Invalid JSON: ..."`), if `aircraft` is empty (`"No aircraft specified"`), or if it has more than 100 entries (`"Maximum 100 aircraft per batch request"`).

## Response Fields

| Field | Type | Always present | Description |
|-------|------|----------------|-------------|
| `icao_hex` | string | Yes | Aircraft ICAO 24-bit address, upper case |
| `callsign` | string | Yes | Flight callsign as stored; may be IATA (`QF9`) or ICAO (`QFA9`) format |
| `flight_date` | string | Yes | Flight date (`YYYY-MM-DD`); see [Data Population](#data-population) for how it is derived |
| `origin` | string | No | Origin airport code as reported by the source message; may be ICAO or IATA |
| `destination` | string | No | Destination airport code as reported by the source message; may be ICAO or IATA |
| `route` | array of string | No | Route waypoint names |
| `eta` | string | No | Estimated arrival time (`HH:MM`). Currently never populated |
| `departure_runway` | string | No | Departure runway |
| `arrival_runway` | string | No | Arrival runway. Currently never populated |
| `sid` | string | No | Standard Instrument Departure |
| `squawk` | string | No | Assigned transponder code |
| `pax_count` | integer | No | Total passenger count; omitted when zero or unknown |
| `pax_breakdown` | object | No | Passengers by cabin class. Currently never populated |
| `last_updated` | string | Yes | Time the record was last updated (RFC 3339) |

Optional fields are omitted from the JSON when they have no value. `last_updated` is formatted in the time zone that the PostgreSQL driver returns for `TIMESTAMPTZ` values, which is the local time zone of the host running the API, so it may carry an offset such as `+10:00` rather than `Z`.

## OpenAPI Specification

An OpenAPI 3.0 specification is available at `api/openapi.yaml`. It can be used to generate client libraries:

```bash
# Generate a TypeScript client
npx openapi-generator-cli generate -i api/openapi.yaml -g typescript-fetch -o clients/typescript

# Generate a Python client
openapi-generator-cli generate -i api/openapi.yaml -g python -o clients/python
```

## Design

### Database Table

The `flight_enrichment` table is created in PostgreSQL by `PostgresDB.CreateSchema` (`internal/storage/postgres.go`).

| Column | Type | Description |
|--------|------|-------------|
| `id` | `SERIAL` | Primary key |
| `icao_hex` | `VARCHAR(6)` | Aircraft ICAO 24-bit address |
| `callsign` | `VARCHAR(10)` | Flight callsign (IATA or ICAO format) |
| `flight_date` | `DATE` | Date of the flight operation |
| `origin` | `VARCHAR(4)` | Origin airport code |
| `destination` | `VARCHAR(4)` | Destination airport code |
| `route` | `JSONB` | Array of waypoint names |
| `eta` | `TIMESTAMPTZ` | Estimated time of arrival |
| `departure_runway` | `VARCHAR(6)` | Departure runway |
| `arrival_runway` | `VARCHAR(6)` | Arrival runway |
| `sid` | `VARCHAR(12)` | Standard Instrument Departure |
| `squawk` | `VARCHAR(4)` | Transponder code |
| `pax_count` | `INTEGER` | Passenger count |
| `pax_breakdown` | `JSONB` | Passenger count by cabin class |
| `created_at` | `TIMESTAMPTZ` | Time the row was created |
| `updated_at` | `TIMESTAMPTZ` | Time the row was last updated |

The table has a unique constraint on `(icao_hex, callsign, flight_date)` and indexes on `(icao_hex, callsign, flight_date)` and `(icao_hex, flight_date)`.

### Data Population

The API only reads the table. Only `acars_parser live` writes to it (unless `-no-store` is set), and only from transmitted identity:

| Field | Source |
|-------|--------|
| ICAO hex | The aircraft's link-layer address (`acars.Message.AircraftAddress`: the sender of a downlink, the recipient of an uplink) |
| Callsign | The flight number as transmitted (`message.flight`) |
| `flight_date` | The date the message was processed (see [Known Limitations](#known-limitations)) |

Airframes' `airframe` and `flight` records are not used: their accuracy is unknown, and Airframes may in future draw on data this project feeds downstream. Messages without a link-layer address (for example most ACARS and satellite messages) therefore produce no enrichment.

`acars_parser backfill` and `reparse` do **not** populate `flight_enrichment`. The stored corpus does not record transmitted identity, so enrichment cannot be rebuilt from it until storage v2.

A record is written only when the ICAO hex is known, a callsign is available (from the message or from a `flight_number`, `flight_num` or `flight` field in the parser result), and at least one enrichment field was extracted. Before storage, the callsign's numeric part has its leading zeros removed (`QFA008` becomes `QFA8`).

On update, fields that are absent from the new data keep their existing values. `route` and `pax_count` are replaced when new values are present.

### Data Sources

Enrichment fields are taken from the following parser result types (`internal/enrichment/enrichment.go`):

| Parser type | Contributes |
|-------------|-------------|
| `pdc` | `origin`, `destination`, `departure_runway` (from `runway`), `sid`, `squawk`, `route` (from `route_waypoints`) |
| `flight_plan` | `origin`, `destination`, `route` (from waypoint names) |
| `loadsheet` | `origin`, `destination`, `pax_count` (from `pax`) |
| `eta` | `origin`, `destination` |

No parser currently contributes `eta`, `arrival_runway` or `pax_breakdown`.

### Callsign Matching

Airlines use both IATA (2-letter) and ICAO (3-letter) prefixes in ACARS messages, often for the same flight:

| Airline | IATA | ICAO |
|---------|------|------|
| Qantas | QF1255 | QFA1255 |
| Qatar Airways | QR411 | QTR411 |
| Ethiopian | ET507 | ETH507 |
| EgyptAir | MS774 | MSR774 |

To avoid holding two records for one flight, both writes and reads match on the numeric suffix of the callsign together with `icao_hex` and `flight_date`:

1. The trailing digits are extracted from the callsign (for example, `1255` from `QF1255`).
2. A row is looked up with the same `icao_hex` and `flight_date` whose callsign matches the regular expression `<digits>$`.
3. On write, if such a row exists, it is updated and its callsign is replaced if the new callsign is longer (so an ICAO form replaces an IATA form). Otherwise a new row is inserted.
4. If the callsign has no trailing digits, an exact callsign match is used instead.

The stored callsign is therefore whichever form was seen, upgraded to the longer form when one arrives later. It is not guaranteed to be in ICAO format.

## Known Limitations

- **Suffix matching is not anchored to the airline prefix.** The pattern `<digits>$` matches any callsign that ends in those digits, so `123` matches `QF1123`, and `1` matches `QF11`, `QF21` and `QF101`. Two different flights by the same aircraft on the same day can therefore be merged into one record on write, or the wrong one returned on read.
- **Single-flight lookups are not deterministic when several rows match.** The callsign and date lookups (and batch entries with a callsign) take the first matching row without an ordering.
- **Requested callsigns are not normalised.** Stored callsigns have leading zeros removed, but the callsign in a request is only upper-cased. A request for `QFA008` searches for the suffix `008` and does not find the stored `QFA8`.
- **`flight_date` from `live` uses the host's local date.** `live` takes the date of the local clock and labels it as a UTC date. On a host that is not running in UTC, records written near midnight can be given a date that differs from the UTC date the API uses for "today".
- **`flight_date` is the date the data was received, not the scheduled departure date.** A flight whose messages span UTC midnight can be split across two dates, and lookups for "today" after midnight UTC do not return the previous day's record.
- **`eta`, `arrival_runway` and `pax_breakdown` are never populated.**
- **Timeouts do not cancel database queries.** The server applies a 30-second request timeout, but database queries do not observe it.
