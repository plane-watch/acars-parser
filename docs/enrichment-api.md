# Flight Enrichment API

The enrichment API provides REST access to flight operational data extracted from ACARS messages. This data can be used to enhance ADS-B tracking with route, runway, squawk, and passenger information.

## Quick Start

```bash
# Build the API server
go build -o enrichment-api ./cmd/enrichment-api

# Run with defaults (connects to localhost PostgreSQL)
./enrichment-api

# Run with custom settings
./enrichment-api -port 8081 -pg-host db.example.com -pg-password secret
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
./enrichment-api -auth -api-keys "key1,key2,key3"

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

## Data Sources

Enrichment data is extracted from the following ACARS message types:

- **PDC (Pre-Departure Clearance)** - Runway, SID, squawk, route
- **Flight Plan (H1/FPN)** - Origin, destination, route waypoints
- **Loadsheet** - Passenger counts, cabin breakdown
- **ETA messages** - Estimated arrival times

## ICAO vs IATA Codes

The API standardises on ICAO codes (4-letter airport codes, 3-letter airline codes + flight number). IATA codes from source messages are stored separately and not returned in enrichment responses to maintain data consistency.