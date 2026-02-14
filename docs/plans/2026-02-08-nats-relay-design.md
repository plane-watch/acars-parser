# NATS Relay Design

## Purpose

A lightweight binary that maintains a single connection to the Airframes public NATS bus,
deduplicates messages, and republishes them to an internal NATS server. This allows multiple
internal consumers (acars_parser live, ADS-C parser, future tools) to share one upstream
connection without wasting Airframes bandwidth.

## Data Flow

```
Airframes NATS  -->  nats-relay  -->  Internal NATS
                   (dedup + relay)     |-- acars_parser live
                                       |-- ads-c parser
                                       |-- future consumers
```

## Key Decisions

- **Raw bytes passthrough**: No transformation of messages. The relay publishes the exact
  JSON bytes received from Airframes. Consumers parse however they like.
- **Flat internal subject**: Publishes to a single configurable subject (default:
  `acars.messages`) rather than preserving the Airframes subject hierarchy.
- **Single dedup cache**: One hash map with per-type stats counters for observability.
- **Location**: `cmd/nats-relay/` - shares `go.mod` with the rest of the project.

## Deduplication

### Analysis (from ClickHouse corpus, 2026-02-08)

- ~67k messages/day on the Airframes feed
- ~5,900 true multi-station duplicates per day (~8.8% of traffic)
- Same ACARS transmission picked up by multiple ground stations, each assigned a different
  Airframes message ID
- Most duplicates arrive within 0-2 seconds of each other

### Strategy: Two-Layer Dedup

**Layer 1 - Subject message ID (cheap):**
Extract the Airframes message ID from the NATS subject
(`v1.aircraft.ingest.{source}.message.{msgID}.created`). This catches exact re-deliveries
from the same source with zero parsing cost.

**Layer 2 - Content hash with TTL:**
Compute a hash of `label + tail + raw_text` from the JSON payload. Store in a hash map with
a configurable TTL (default 10 seconds). This catches multi-station duplicates where different
ground stations independently ingest the same transmission and assign different message IDs.

The content hash requires a partial JSON parse to extract the three fields, but avoids
hashing the entire message body (which includes variable metadata like station info and
timestamps that differ between stations).

### Cache Implementation

- Single `map[uint64]int64` (hash -> expiry timestamp)
- Max size: 100,000 entries (configurable)
- Eviction: periodic sweep of expired entries (every 5 seconds)
- Hash function: FNV-64a over the concatenated key fields
- Per-label stats counters for observability (total/duplicates per label)

## Configuration

All configuration via environment variables with sensible defaults.

| Variable | Default | Description |
|---|---|---|
| `AIRFRAMES_NATS_URL` | `nats://157.90.242.138:4222` | Airframes NATS server |
| `AIRFRAMES_NATS_CREDS` | _(required)_ | Path to .creds file, or inline creds content |
| `AIRFRAMES_NATS_SUBJECT` | `v1.aircraft.ingest.*.message.*.created` | Subject to subscribe |
| `INTERNAL_NATS_URL` | `nats://localhost:4222` | Internal NATS server |
| `INTERNAL_NATS_SUBJECT` | `acars.messages` | Subject to publish on |
| `DEDUP_TTL` | `10s` | Content hash expiry window |
| `DEDUP_MAX_SIZE` | `100000` | Max entries in dedup cache |
| `METRICS_ADDR` | `:9090` | HTTP address for /healthz and /metrics |
| `LOG_LEVEL` | `info` | Log verbosity (debug, info, warn, error) |

### Credentials Handling

`AIRFRAMES_NATS_CREDS` accepts either:
- A file path (local dev): `/path/to/airframes_nats.creds`
- Raw credential content as a string (Docker secrets/env injection)

The binary detects which by checking if the value is a valid file path.

## Architecture

### Package Structure

```
cmd/nats-relay/
    main.go          -- Entry point, config loading, signal handling
    config.go        -- Environment variable parsing with defaults
    relay.go         -- Core relay logic (subscribe, dedup, publish)
    dedup.go         -- Content hash dedup cache
    metrics.go       -- Prometheus metrics and health endpoint
```

### Components

**Config loader:** Reads environment variables, validates required fields (creds),
applies defaults. Handles the file-path-vs-inline detection for credentials.

**Upstream connection (Airframes):** NATS client with .creds auth, unlimited reconnect
(2-second backoff), client name `nats-relay-upstream`.

**Downstream connection (internal):** NATS client, no auth, unlimited reconnect,
client name `nats-relay-downstream`.

**Dedup cache:** Hash map with TTL-based expiry. Two checks per message:
1. Subject message ID lookup (string map, no parsing needed)
2. Content hash lookup (requires partial JSON parse of label/tail/raw_text)

**Message handler:** On each received message:
1. Extract message ID from subject -> check layer 1
2. If not a subject-level dupe, partial-parse JSON for label+tail+raw_text
3. Compute FNV-64a hash -> check layer 2
4. If unique, publish raw bytes to internal NATS on the configured subject
5. Update stats counters

**Metrics server:** HTTP server exposing:
- `GET /healthz` - Returns 200 if both NATS connections are healthy
- `GET /metrics` - Prometheus format metrics

### Metrics

| Metric | Type | Description |
|---|---|---|
| `relay_messages_received_total` | counter | Total messages from Airframes |
| `relay_messages_published_total` | counter | Messages published to internal NATS |
| `relay_duplicates_subject_total` | counter | Duplicates caught by subject ID |
| `relay_duplicates_content_total` | counter | Duplicates caught by content hash |
| `relay_dedup_cache_size` | gauge | Current entries in dedup cache |
| `relay_upstream_connected` | gauge | 1 if connected to Airframes, 0 if not |
| `relay_downstream_connected` | gauge | 1 if connected to internal NATS, 0 if not |
| `relay_messages_by_label_total` | counter (labelled) | Messages per ACARS label |
| `relay_duplicates_by_label_total` | counter (labelled) | Duplicates per ACARS label |

### Graceful Shutdown

On SIGINT/SIGTERM:
1. Unsubscribe from Airframes NATS
2. Drain internal NATS connection (flush pending publishes)
3. Stop metrics HTTP server
4. Log final stats summary
5. Exit

## Docker

Dockerfile for the relay binary:

```dockerfile
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /nats-relay ./cmd/nats-relay

FROM alpine:3.21
COPY --from=build /nats-relay /usr/local/bin/nats-relay
ENTRYPOINT ["nats-relay"]
```

Example docker-compose service:

```yaml
nats-relay:
  build:
    context: .
    dockerfile: cmd/nats-relay/Dockerfile
  environment:
    AIRFRAMES_NATS_URL: "nats://157.90.242.138:4222"
    AIRFRAMES_NATS_CREDS: "${AIRFRAMES_CREDS}"
    INTERNAL_NATS_URL: "nats://nats:4222"
    INTERNAL_NATS_SUBJECT: "acars.messages"
  ports:
    - "9090:9090"
  restart: unless-stopped
```

## Testing

- Unit tests for dedup cache (TTL expiry, max size eviction, hash correctness)
- Unit tests for config parsing (env vars, defaults, creds detection)
- Integration test: mock both NATS connections, verify dedup and passthrough
- Use sample messages from ClickHouse corpus for realistic test data

## Status

Implemented. See `cmd/nats-relay/` and `internal/relay/`.
