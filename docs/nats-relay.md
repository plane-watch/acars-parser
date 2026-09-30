# NATS Relay

`nats-relay` is a standalone binary that holds a single subscription to the Airframes public NATS bus, removes duplicate messages, and republishes the remaining messages to an internal NATS server. It allows several internal consumers to share one upstream connection.

- Entry point: `cmd/nats-relay/main.go`
- Implementation: `internal/relay/` (`config.go`, `dedup.go`, `metrics.go`, `relay.go`)
- Dockerfile: `cmd/nats-relay/Dockerfile`

```
Airframes NATS  -->  nats-relay (dedup)  -->  internal NATS, subject acars.messages
```

## Running

```bash
go build -o bin/nats-relay ./cmd/nats-relay
AIRFRAMES_NATS_CREDS=/path/to/airframes_nats.creds ./bin/nats-relay
```

## Configuration

All configuration is read from environment variables; there are no command-line flags.

| Variable | Default | Description |
|----------|---------|-------------|
| `AIRFRAMES_NATS_URL` | `nats://157.90.242.138:4222` | The upstream (Airframes) NATS server |
| `AIRFRAMES_NATS_CREDS` | _(required)_ | A path to a `.creds` file, or the credential content itself |
| `AIRFRAMES_NATS_SUBJECT` | `v1.aircraft.ingest.*.message.*.created` | The upstream subject to subscribe to |
| `INTERNAL_NATS_URL` | `nats://localhost:4222` | The internal NATS server (connected without authentication) |
| `INTERNAL_NATS_SUBJECT` | `acars.messages` | The subject that every relayed message is published on |
| `DEDUP_TTL` | `10s` | The expiry time for entries in **both** dedup caches (Go duration syntax) |
| `DEDUP_MAX_SIZE` | `100000` | The maximum number of entries in **each** dedup cache |
| `METRICS_ADDR` | `:9090` | The listen address for `/healthz` and `/metrics` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` (case-insensitive) |

Notes:

- An empty variable is treated as unset.
- A value for `DEDUP_TTL` or `DEDUP_MAX_SIZE` that cannot be parsed is ignored silently and the default is used. An unrecognised `LOG_LEVEL` is treated as `info`.
- A `DEDUP_TTL` of zero or less makes every entry expire immediately, which disables deduplication.
- A negative `DEDUP_MAX_SIZE` causes a panic at start-up.
- If `AIRFRAMES_NATS_CREDS` is empty, the relay exits with a configuration error.

### Credentials

If `AIRFRAMES_NATS_CREDS` names an existing file, that file is used directly. Otherwise, the value is treated as the credential content: it is written to a temporary file (created with mode `0600` in the system temporary directory), which is removed when the relay exits. This allows the credentials to be injected as an environment variable or a Docker secret.

## Message Handling

The upstream subscription callback runs for one message at a time. For each message:

1. The received counter is incremented.
2. **Layer 1 (subject ID):** the subject is split on `.`. If it has at least seven tokens, the sixth token is taken as the Airframes message ID (`v1.aircraft.ingest.{source}.message.{msgID}.created`). If that ID is present in the subject cache and has not expired, the message is dropped as a subject duplicate. Subjects with fewer than seven tokens skip this layer.
3. **Layer 2 (content hash):** the payload is partially decoded to read `message.label`, `message.tail` and `message.text`. An FNV-64a hash is computed over `label`, a zero byte, `tail`, a zero byte, and `text`. If the hash is present in the content cache and has not expired, the message is dropped as a content duplicate. If the payload is not valid JSON, this layer is skipped and the message is still relayed.
4. The payload is published to `INTERNAL_NATS_SUBJECT`.

The purpose of layer 2 is to remove the copies that arise when several ground stations receive the same transmission and Airframes gives each copy a different message ID. Station metadata and timestamps are excluded from the hash because they differ between those copies.

### Republished Payload

The payload is published **unchanged**: the exact bytes received from Airframes, with no wrapper and no added fields. All messages go to the single, flat subject `INTERNAL_NATS_SUBJECT`. The upstream subject, including the source name and message ID, is not preserved.

### Dedup Caches

There are two independent caches:

| Cache | Key | Expiry | Maximum size |
|-------|-----|--------|--------------|
| Subject cache | The message ID string | `DEDUP_TTL` | `DEDUP_MAX_SIZE` |
| Content cache | The 64-bit content hash | `DEDUP_TTL` | `DEDUP_MAX_SIZE` |

Each cache maps a key to its expiry time. A key that is seen again after it has expired is treated as new. The combined size of the two caches can therefore reach twice `DEDUP_MAX_SIZE`.

Expired entries are removed by a sweep every five seconds. When a cache is full at insertion time, expired entries are swept first; if the cache is still full, the entry with the earliest expiry is evicted.

Because the subject cache also uses `DEDUP_TTL`, a re-delivery of the same message ID more than `DEDUP_TTL` after the first is not caught by layer 1.

### Dedup Edge Cases

- **Payloads without a `message` object.** A valid JSON payload with no `message` object produces empty label, tail and text, so every such payload hashes to the same value. Only the first of them within each `DEDUP_TTL` window is relayed.
- **The tail comes only from `message.tail`.** The fallback to `airframe.tail` used elsewhere in the project is not applied. Messages from different aircraft with an empty `message.tail` and identical label and text are treated as duplicates.
- **Repeated identical messages.** Distinct transmissions with the same label, tail and text (for example, short or empty-text messages) within `DEDUP_TTL` of each other are treated as duplicates.

## Metrics

`GET /metrics` serves Prometheus metrics from the default registry, which also includes the standard Go runtime and process collectors.

| Metric | Type | Meaning |
|--------|------|---------|
| `relay_messages_received_total` | counter | Messages received from the upstream server, before deduplication |
| `relay_messages_published_total` | counter | Messages published to the internal server without error |
| `relay_duplicates_subject_total` | counter | Messages dropped by layer 1 |
| `relay_duplicates_content_total` | counter | Messages dropped by layer 2 |
| `relay_dedup_cache_size` | gauge | The combined number of entries in both caches, updated by the five-second sweep |
| `relay_upstream_connected` | gauge | `1` after the upstream connection is established or re-established, `0` after it disconnects |
| `relay_downstream_connected` | gauge | The same for the internal connection |
| `relay_messages_by_label_total{acars_label}` | counter | Messages that passed both dedup layers, by ACARS label (counted before publishing, so a failed publish is still counted) |
| `relay_duplicates_by_label_total{acars_label}` | counter | Layer 2 (content) duplicates by ACARS label; layer 1 duplicates are not included |

Messages with an empty label, or whose payload could not be decoded, are not counted in the per-label metrics.

## Health Check

`GET /healthz` returns:

- `200` with the body `ok` when both the upstream and the internal connection report that they are connected.
- `503` with a body such as `upstream=false downstream=true` otherwise, including while a connection is reconnecting.

The HTTP server for `/healthz` and `/metrics` is started only after both NATS connections and the upstream subscription have succeeded. There is no retry on the initial connection: if either server cannot be reached at start-up, the relay exits with an error and the endpoints never become available. After start-up, both connections reconnect indefinitely with a two-second wait.

If the HTTP server cannot bind to `METRICS_ADDR`, the error is logged and the relay continues to run without the health and metrics endpoints.

## Shutdown

On `SIGINT` or `SIGTERM`, the relay:

1. Logs a summary with the received and published counts and the size of each cache.
2. Shuts down the HTTP server (with a five-second timeout).
3. Unsubscribes from the upstream subject.
4. Starts draining the internal connection. The drain is asynchronous and the relay does not wait for it to finish, so publishes still in flight at this point may be lost. A drain error is logged as a warning.
5. Closes the upstream connection.
6. Removes the temporary credentials file, if one was created.

## Docker

```bash
docker build -f cmd/nats-relay/Dockerfile -t nats-relay .
docker run -e AIRFRAMES_NATS_CREDS="$CREDS" -e INTERNAL_NATS_URL=nats://nats:4222 -p 9090:9090 nats-relay
```

The Dockerfile is a two-stage build: `golang:1.25-alpine` compiles a static binary (`CGO_ENABLED=0`), and `alpine:3.21` with `ca-certificates` runs it. The repository's `.dockerignore` keeps local data, binaries, documentation, `.creds` files and any local `go.work` out of the build context.

## Known Limitations

- **No consumer in this repository reads `acars.messages`.** `acars_parser live` requires `-creds` and always authenticates with a credentials file, so it has no mode for subscribing to an unauthenticated internal server. The relayed payload is in the same format that `live` already parses.
- **Shutdown does not wait for the internal connection to drain**, so messages can be lost on shutdown.
- **No start-up retry.** The relay exits if either NATS server is unavailable when it starts.
- **Invalid configuration values are not reported.** They fall back to defaults silently, and a negative `DEDUP_MAX_SIZE` causes a panic.
- **Dedup edge cases.** Payloads without a `message` object, and messages with an empty `message.tail`, can be dropped incorrectly (see [Dedup Edge Cases](#dedup-edge-cases)).
