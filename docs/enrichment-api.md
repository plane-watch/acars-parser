# Flight Enrichment API

The enrichment API provides REST access to flight operational data extracted from ACARS messages. This data can be used to enhance ADS-B tracking with route, runway, squawk, and passenger information.

## Quick Start

```bash
# Build the API server
go build -o bin/enrichment-api ./cmd/enrichment-api

# Run with defaults (connects to localhost PostgreSQL)
./bin/enrichment-api

# Run with custom settings
./bin/enrichment-api -port 8081 -pg-host db.example.com -pg-password secret
```

## Configuration

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `-port` | - | 8081 | HTTP port |
| `-pg-host` | `POSTGRES_HOST` | localhost | PostgreSQL host |
| `-pg-port` | `POSTGRES_PORT` | 5432 | PostgreSQL port |
| `-pg-database` | `POSTGRES_DATABASE` | acars_state | PostgreSQL database |
| `-pg-user` | `POSTGRES_USER` | acars | PostgreSQL user |
| `-pg-password` | `POSTGRES_PASSWORD` | acars | PostgreSQL password |
| `-auth` | - | false | Enable API key authentication |
| `-api-keys` | - | - | Comma-separated API keys |

## API Endpoints

### Health Check

```
GET /api/v1/health
```

Returns server status.

```json
{"status": "ok", "time": "2026-01-31T10:00:00Z"}
```

### Get Enrichment by Aircraft

```
GET /api/v1/enrichment/{icao_hex}
```

Returns all enrichments for an aircraft on today's date (UTC). An aircraft may have multiple enrichments if it operated multiple flights.

**Parameters:**
- `icao_hex` - Aircraft ICAO 24-bit address (e.g., `7C6CA3`)

**Example:**
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

Returns enrichment for a specific flight on today's date.

**Example:**
```bash
curl http://localhost:8081/api/v1/enrichment/7C6CA3/QFA9
```

### Get Enrichment by Date

```
GET /api/v1/enrichment/{icao_hex}/{callsign}/{date}
```

Returns enrichment for a specific flight and date. Use for historical lookups.

**Parameters:**
- `date` - Flight date in `YYYY-MM-DD` format

**Example:**
```bash
curl http://localhost:8081/api/v1/enrichment/7C6CA3/QFA9/2026-01-30
```

### Batch Lookup

```
POST /api/v1/enrichment/batch
```

Look up enrichments for multiple aircraft in a single request. Maximum 100 aircraft per request. Returns today's data.

**Request Body:**
```json
{
  "aircraft": [
    {"icao_hex": "7C6CA3"},
    {"icao_hex": "780AB6", "callsign": "CPA844"}
  ]
}
```

**Response:**
```json
{
  "results": {
    "7C6CA3": [...],
    "780AB6": [...]
  }
}
```

## Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `icao_hex` | string | Aircraft ICAO 24-bit hex address |
| `callsign` | string | Flight callsign (ICAO format) |
| `flight_date` | string | Flight date (YYYY-MM-DD) |
| `origin` | string | Origin airport ICAO code |
| `destination` | string | Destination airport ICAO code |
| `route` | array | Route waypoints |
| `eta` | string | Estimated arrival time (HH:MM) |
| `departure_runway` | string | Departure runway |
| `arrival_runway` | string | Arrival runway (when known) |
| `sid` | string | Standard Instrument Departure |
| `squawk` | string | Assigned transponder code |
| `pax_count` | integer | Total passenger count |
| `pax_breakdown` | object | Passengers by cabin class |
| `last_updated` | string | Last update timestamp (RFC3339) |

## Authentication

When `-auth` is enabled, requests must include an API key via one of:

- `X-API-Key` header
- `Authorization: Bearer <key>` header
- `api_key` query parameter (for testing)

**Example:**
```bash
./bin/enrichment-api -auth -api-keys "key1,key2,key3"

curl -H "X-API-Key: key1" http://localhost:8081/api/v1/enrichment/7C6CA3
```

## OpenAPI Specification

A full OpenAPI 3.0 spec is available at `api/openapi.yaml`. Use it to generate client libraries:

```bash
# Generate TypeScript client
npx openapi-generator-cli generate -i api/openapi.yaml -g typescript-fetch -o clients/typescript

# Generate Python client
openapi-generator-cli generate -i api/openapi.yaml -g python -o clients/python
```

## Design

### Database Table

The `flight_enrichment` table is stored in PostgreSQL (`acars_state` database).

| Column | Description |
|--------|-------------|
| `icao_hex` | Aircraft Mode-S transponder code (unique per airframe) |
| `callsign` | Flight callsign (e.g., QTR411, UAL123) |
| `flight_date` | Date of the flight operation |
| `origin` | Departure airport (ICAO or IATA code) |
| `destination` | Arrival airport (ICAO or IATA code) |
| `route` | Array of waypoints |
| `eta` | Estimated time of arrival |
| `runway` | Assigned runway |
| `sid` | Standard Instrument Departure procedure |
| `squawk` | Transponder squawk code |
| `pax_count` | Passenger count |
| `pax_breakdown` | JSON breakdown of passenger classes |

### Callsign Matching Strategy

Airlines use both IATA (2-letter) and ICAO (3-letter) callsign formats interchangeably in ACARS messages:

| Airline | IATA | ICAO |
|---------|------|------|
| Qantas | QF1255 | QFA1255 |
| Qatar Airways | QR411 | QTR411 |
| Ethiopian | ET507 | ETH507 |
| EgyptAir | MS774 | MSR774 |

Different message types often use different formats:
- **Loadsheets** typically use IATA format (QF1255)
- **Position reports** typically use ICAO format (QFA1255)

Without special handling, a single flight would create two separate enrichment records - one for QF1255 with loadsheet data and one for QFA1255 with position data.

The enrichment system matches on the **numeric flight number suffix** rather than the exact callsign, combined with:
- `icao_hex` - unique aircraft identifier
- `flight_date` - date of operation

This is safe because the same physical aircraft cannot fly for two different airlines on the same day with the same flight number.

When upserting enrichment data:

1. Extract the numeric suffix from the callsign (e.g., "1255" from "QF1255")
2. Search for an existing row with matching `icao_hex`, `flight_date`, and callsign ending with that number
3. If found, update that row (merging new data with existing)
4. If not found, insert a new row

When a match is found between IATA and ICAO variants, the system prefers the longer (ICAO) format as it's more specific and standardised for ATC communications.

The regex pattern `callsign ~ (flight_num || '$')` matches callsigns ending with the flight number, allowing both QF1255 and QFA1255 to match when searching for "1255".

This approach was validated against the corpus: 1,096 duplicate rows (5% of total) were caused by IATA/ICAO format differences, with zero false positives detected.

### Data Sources

Enrichment data is populated from the following ACARS message types:

| Parser | Contributes |
|--------|-------------|
| `pdc` | squawk, runway, sid, origin, destination |
| `flight_plan` | route, origin, destination |
| `loadsheet` | pax_count, pax_breakdown, origin, destination |
| `eta` | eta |

### ICAO vs IATA Codes

The API standardises on ICAO codes (4-letter airport codes, 3-letter airline codes + flight number). IATA codes from source messages are stored separately and not returned in enrichment responses to maintain data consistency.
