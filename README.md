# ACARS Parser

A Go toolkit for parsing ACARS (Aircraft Communications Addressing and Reporting System) messages. It extracts structured data from a range of message types: pre-departure clearances, flight plans, position reports, ADS-C, CPDLC, ATIS, wind forecasts, loadsheets, and others. The results are stored for analysis and served to other systems.

The repository contains:

- **`acars_parser`**, the main CLI. It consumes the Airframes NATS feed, parses messages, stores them in ClickHouse and PostgreSQL, and provides tools for querying, reparsing, debugging and reviewing parser output.
- **`enrichment-api`**, a REST API that serves per-flight data (route, runway, SID, squawk, passenger count) for ADS-B tracking integration.
- **`nats-relay`**, a service that deduplicates the Airframes NATS feed and republishes it to an internal NATS server.
- **Standalone tools** in `tools/` for corpus analysis and for exporting waypoints (KML) and routes (CSV).

## Building

```bash
go build -o bin/acars_parser ./cmd/acars_parser
go build -o bin/enrichment-api ./cmd/enrichment-api
go build -o bin/nats-relay ./cmd/nats-relay
go build -o bin/analyzer ./tools/analyzer
go build -o bin/kmlexport ./tools/kmlexport
go build -o bin/routeexport ./tools/routeexport
```

All binaries belong to the single Go module in the repository root.

The CPDLC decoder depends on [`github.com/shaneshort/go-asn`](https://github.com/shaneshort/go-asn), which is fetched as a normal module. To develop against a local checkout of it, create a `go.work` (gitignored):

```bash
go work init . ../go-asn
```

## Database setup

The project uses a two-database architecture. See [docs/storage.md](docs/storage.md) for the full schema and for which commands read and write each table.

- **ClickHouse** stores the append-only message archive.
- **PostgreSQL** stores mutable state: aircraft, waypoints, routes, ATIS, flight state, flight enrichment and golden annotations.

```bash
# ClickHouse (the acars database must exist before any command connects)
docker run -d --name acars-clickhouse -p 9000:9000 -p 8123:8123 \
    -e CLICKHOUSE_DB=acars \
    -e CLICKHOUSE_PASSWORD=acars \
    clickhouse/clickhouse-server:latest

# PostgreSQL
docker run -d --name acars-postgres -p 5432:5432 \
    -e POSTGRES_USER=acars \
    -e POSTGRES_PASSWORD=acars \
    -e POSTGRES_DB=acars_state \
    postgres:16-alpine

# Create the ClickHouse and PostgreSQL tables (no SQLite data is copied).
./bin/acars_parser migrate -skip-messages -skip-state
```

`migrate` is the only command that creates the ClickHouse tables, so the last step is required on a fresh installation.

### Configuration

Every `acars_parser` subcommand takes its default connection settings from these environment variables. The command-line flags override them.

| Variable | Default |
|---|---|
| `CLICKHOUSE_HOST` | `localhost` |
| `CLICKHOUSE_PORT` | `9000` |
| `CLICKHOUSE_DATABASE` | `acars` |
| `CLICKHOUSE_USER` | `default` |
| `CLICKHOUSE_PASSWORD` | `acars` |
| `POSTGRES_HOST` | `localhost` |
| `POSTGRES_PORT` | `5432` |
| `POSTGRES_DATABASE` | `acars_state` |
| `POSTGRES_USER` | `acars` |
| `POSTGRES_PASSWORD` | `acars` |

## Project structure

```
.
├── api/openapi.yaml          # OpenAPI spec for the enrichment API
├── cmd/
│   ├── acars_parser/         # Main CLI (one file per subcommand, config.go for env defaults)
│   ├── enrichment-api/       # Enrichment REST API server
│   └── nats-relay/           # NATS relay entry point and Dockerfile
├── docs/                     # Documentation (see below)
├── internal/
│   ├── acars/                # ACARS message and NATS wrapper types
│   ├── api/                  # Enrichment API HTTP handlers
│   ├── crc/                  # CRC-16/ARINC checksum (used by the H1, envelope and ADS-C parsers)
│   ├── enrichment/           # Extracts flight enrichment data from parser results
│   ├── extractor/            # Extracts state data (aircraft, waypoints, routes, ATIS) from results
│   ├── parsers/              # Parser implementations; parsers.go registers them all
│   ├── patterns/             # Shared regex patterns, the grok-style format compiler and extractors
│   ├── registry/             # The parser registry, dispatch and trace types
│   ├── relay/                # NATS relay core (config, dedup, metrics)
│   ├── review/               # Review web UI (embedded static files)
│   └── storage/              # ClickHouse and PostgreSQL access
└── tools/
    ├── analyzer/             # Corpus analysis (ClickHouse)
    ├── kmlexport/            # Waypoint export to KML (PostgreSQL)
    └── routeexport/          # Route export to CSV (PostgreSQL)
```

## acars_parser commands

```
acars_parser <command> [options]
```

| Command | Purpose | Databases |
|---|---|---|
| `extract` | Parse a JSONL file and write the results as JSON | none |
| `live` | Consume the NATS feed, print, and store the results | ClickHouse, PostgreSQL |
| `query` | Query stored messages | ClickHouse |
| `reparse` | Re-run the current parsers over stored messages and compare the results | ClickHouse |
| `debug` | Trace the pattern matching for a single message | ClickHouse (with `-id`) |
| `review` | Web UI for browsing messages and setting golden annotations | ClickHouse, PostgreSQL |
| `templates` | Group messages into normalised format templates | ClickHouse |
| `backfill` | Rebuild the PostgreSQL state from stored messages | ClickHouse, PostgreSQL |
| `migrate` | Create the schemas and copy the legacy SQLite databases | SQLite, ClickHouse, PostgreSQL |
| `unparse` | Mark stored messages as unparsed | ClickHouse |
| `baseline` | Record the parser regression baseline from a sample of stored messages | ClickHouse |

`acars_parser help` prints a usage summary. The flag lists below are authoritative where the two differ.

### Connection flags

Commands that use a database accept connection flags whose defaults come from the environment variables above. The database-name flag is spelt differently depending on the command:

| Commands | Database name flags |
|---|---|
| `live`, `query`, `debug`, `review`, `templates` | `-ch-db`, `-pg-db` |
| `reparse`, `backfill`, `migrate`, `unparse` | `-ch-database`, `-pg-database` |

All of them also accept these flags, where the command uses that database:

- ClickHouse: `-ch-host`, `-ch-port`, `-ch-user`, `-ch-password`
- PostgreSQL: `-pg-host`, `-pg-port`, `-pg-user`, `-pg-password`

The per-command sections below list only the remaining flags.

### extract

Reads JSONL messages from a file or stdin, parses each with every matching parser, and writes a single JSON document. Each line may be an Airframes NATS wrapper or a bare message.

```bash
./bin/acars_parser extract -input messages.jsonl -output results.json -pretty
```

| Flag | Default | Description |
|---|---|---|
| `-input` | stdin | Input JSONL file |
| `-output` | stdout | Output JSON file |
| `-pretty` | `false` | Pretty-print the output |
| `-all` | `false` | Accepted but has no effect; all result types are always included |

The output has this shape:

```json
{
  "stats": {
    "total_messages": 1000,
    "parsed_by_type": {"pdc": 12, "h1_position": 87},
    "top_origins": [{"code": "YSSY", "count": 40}],
    "top_destinations": [{"code": "YMML", "count": 35}]
  },
  "results": {
    "pdc": [ ... ],
    "h1_position": [ ... ]
  }
}
```

`total_messages` counts every input line, including blank and unparseable ones.

### live

Subscribes to the NATS feed and parses each message.

- It prints one line per result.
- Unless `-no-store` is set, it batch-inserts results into ClickHouse `messages` (every 1,000 rows or 5 seconds).
- Unless `-no-store` is set, it also updates the PostgreSQL `aircraft`, `waypoints`, `routes`, `atis_current` and `flight_enrichment` tables.

```bash
./bin/acars_parser live -creds airframes_nats.creds
```

| Flag | Default | Description |
|---|---|---|
| `-creds` | *(required)* | NATS credentials file |
| `-server` | `nats://157.90.242.138:4222` | NATS server URL |
| `-subject` | `v1.aircraft.ingest.*.message.*.created` | NATS subject |
| `-output` | none | Append results to a JSONL file |
| `-no-store` | `false` | Disable all database storage |
| `-all` | `false` | Also print messages that no parser matched |
| `-raw` | `false` | Print the raw message text under each result |
| `-empty` | `false` | Show empty fields, to identify unparsed data |
| `-exclude` | `sq_position` | Comma-separated result types to exclude. Use `-exclude ""` to include everything. |
| `-debug` | none | Comma-separated labels to debug, e.g. `80,B6,H1` |
| `-v` | `false` | Verbose output: message ID, label, parser and all parsed fields |

`-exclude` removes the excluded types from storage and the JSONL output as well as from the console. With the default setting, `sq_position` results are not written to ClickHouse. PostgreSQL state is still updated from them.

Console output has the form `[<label>] [<flight> <tail> <aircraft model>] [<RESULT TYPE>] <summary>`. Parts of the context that are unknown are omitted. For example:

```
[H1] [QFA9 VH-ZNA] [PDC] QFA9 YPPH->EGLL SQK 4521 SID JULIM6 RWY 03
```

`live` requires a credentials file, so it cannot subscribe to an unauthenticated internal NATS server such as the one `nats-relay` publishes to.

### query

Queries ClickHouse `messages`.

```bash
./bin/acars_parser query -type pdc -limit 5 -raw
./bin/acars_parser query -stats
```

| Flag | Default | Description |
|---|---|---|
| `-id` | none | Fetch a message by ID |
| `-type` | none | Filter by parser type, e.g. `h1_position`, `pdc`, `unparsed` |
| `-label` | none | Filter by ACARS label |
| `-flight` | none | Filter by flight number (partial match) |
| `-has-missing` | `false` | Only messages with missing fields |
| `-search` | none | Substring match on the raw text |
| `-limit` | `20` | Maximum results |
| `-offset` | `0` | Pagination offset |
| `-order` | `id` | Sort field: `id`, `timestamp`, `parser_type`, `confidence`, `label`, `flight` |
| `-desc` | `true` | Sort descending. Use `-desc=false` for ascending. |
| `-raw` | `false` | Show the raw message text |
| `-json` | `false` | Output as JSON (preceded by a "Found N messages" line) |
| `-stats` | `false` | Show database statistics only |
| `-list-types` | `false` | List the parser types in the database |
| `-list-missing` | `false` | List the most common missing fields |

### reparse

Re-runs the current parsers over stored messages and compares each new result with the stored one. Each message is classified as unchanged, improved, regressed or changed. This is the main tool for checking parser changes against historical data.

```bash
./bin/acars_parser reparse -type unparsed
./bin/acars_parser reparse -label H1 -regressions-only -v
```

| Flag | Default | Description |
|---|---|---|
| `-id` | none | Reparse one message and show the result |
| `-type` | none | Filter by stored parser type |
| `-label` | none | Filter by ACARS label |
| `-limit` | `0` | Maximum messages (0 means all) |
| `-v` | `false` | Show detailed field diffs |
| `-regressions-only` | `false` | Only show regressions |
| `-improvements-only` | `false` | Only show improvements |
| `-json` | `false` | Output as JSON |
| `-dump` | none | Write regressed messages, with raw text, to a file |
| `-update` | `false` | Insert the new results into ClickHouse |
| `-batch` | `10000` | ClickHouse query page size |

Behaviour to be aware of:

- Each stored row is compared with the new result of the same type. If that type is no longer produced, the row is compared with the first match, so the change of type is reported.
- Without `-update`, nothing is written to ClickHouse.
- `-update` inserts new rows and does not remove the old ones, because `messages` is a plain `MergeTree`. It creates duplicate rows (see [docs/storage.md](docs/storage.md)).

### debug

Shows which parsers were tried for a message, whether each quick check passed, and which patterns and extractors matched.

```bash
./bin/acars_parser debug -id 123456789
./bin/acars_parser debug -text "MESSAGE TEXT" -label H1 -all
```

| Flag | Default | Description |
|---|---|---|
| `-id` | none | Message ID to load from ClickHouse |
| `-text` | none | Raw message text (instead of `-id`) |
| `-label` | none | ACARS label for `-text` |
| `-all` | `false` | Show every pattern attempt, not only matches |
| `-type` | none | Only show the trace for this parser type |

### review

Serves a web UI for browsing stored messages and marking golden messages and annotations. Annotations are stored in PostgreSQL `golden_annotations`. The UI can export golden messages as JSON or as a generated Go test file.

```bash
./bin/acars_parser review -port 8080
```

| Flag | Default | Description |
|---|---|---|
| `-port` | `8080` | HTTP port |
| `-type` | none | Pre-filter to a parser type |

- The server listens on all interfaces and has no authentication, so run it only on a trusted network.
- It does not create the PostgreSQL schema. Run `migrate`, `live` or `backfill` first.
- The "golden only" filter is applied after paging, so a page shows only the golden messages among its rows.

### templates

Normalises message text into token templates and groups messages by template. This helps find message formats that no parser handles yet.

```bash
./bin/acars_parser templates -label H1 -min 5 -examples 2
```

| Flag | Default | Description |
|---|---|---|
| `-type` | none | Filter by parser type |
| `-label` | none | Filter by ACARS label |
| `-limit` | `0` | Maximum messages. 0 is treated as 100,000. |
| `-min` | `2` | Minimum messages per template to show |
| `-examples` | `1` | Example messages per template |
| `-v` | `false` | Show the full template strings |

### backfill

Rebuilds the PostgreSQL state from the `parsed_json` stored in ClickHouse, using a pool of workers. It updates `aircraft`, `waypoints`, `routes`, `atis_current` and `flight_state`. It does not update `flight_enrichment`.

```bash
./bin/acars_parser backfill -type flight_plan -workers 16
```

| Flag | Default | Description |
|---|---|---|
| `-type` | none | Filter by parser type |
| `-limit` | `0` | Maximum messages (0 means all) |
| `-workers` | `8` | Worker goroutines |
| `-batch` | `10000` | ClickHouse query page size |
| `-v` | `false` | Verbose output |

### migrate

Creates the ClickHouse and PostgreSQL schemas, then copies the legacy SQLite databases (`messages.db` and `state.db`) into them.

```bash
./bin/acars_parser migrate -dry-run
./bin/acars_parser migrate -messages-db data/messages.db -state-db data/state.db
./bin/acars_parser migrate -resume-from 5000000
```

| Flag | Default | Description |
|---|---|---|
| `-messages-db` | `messages.db` | SQLite messages database |
| `-state-db` | `state.db` | SQLite state database |
| `-batch` | `10000` | Message batch size |
| `-resume-from` | `0` | Resume the message migration from this ID |
| `-skip-messages` | `false` | Skip the message migration |
| `-skip-state` | `false` | Skip the state migration |
| `-dry-run` | `false` | Report counts without copying data. The schemas are still created. |

### baseline

Draws a deterministic sample of stored messages and records what the current parsers produce for each one. The result is the regression baseline in `internal/parsers/testdata/baseline/`, which `go test` checks (see [Development](#development)).

```bash
./bin/acars_parser baseline -cutoff "2026-01-22 06:00:00"
```

| Flag | Default | Description |
|---|---|---|
| `-cutoff` | *(required)* | Only sample messages at or before this UTC time (`YYYY-MM-DD HH:MM:SS`), so that the sample is reproducible |
| `-per-stratum` | `100` | Maximum messages per stratum (label plus current result types) |
| `-out` | `internal/parsers/testdata/baseline` | Output directory |

How the sample is drawn:

1. Every message up to the cutoff is parsed with the current parsers.
2. Each message's stratum is its label plus the sorted result types it produces, for example `RA/takeoff_data+weather` or `H1/unparsed`. Grouping by current output, rather than by the stored parser type, gives every registered parser its own share of the sample.
3. Each stratum keeps up to `-per-stratum` messages, those with the lowest `cityHash64(id)`, with the ID as a tie-breaker. The same data therefore always produces the same sample.
4. Duplicates are handled per ID. An ID stored more than once is used once if its copies agree, and is excluded (and counted) if they conflict.

The output is one file per label, plus a `manifest.json` that records:
- the cutoff and the sampling rules
- the commit that drew the sample and the commit that recorded the expectations
- the number of cases in each file

The new baseline is written to a staging directory and swapped in, so a failed run leaves the previous baseline intact, and an interrupted run is recovered by the next one. A full scan of the 11.7M-message corpus takes a few minutes.

Generate the baseline against a corpus that is not receiving writes timestamped before the cutoff. Duplicate IDs are resolved in a separate query from the main scan, so rows inserted between the two can be handled inconsistently.

### unparse

Marks matching messages as unparsed by inserting copies with `parser_type = 'unparsed'` and an empty `parsed_json`, then runs `OPTIMIZE TABLE messages FINAL`. Use it to queue messages for `reparse -type unparsed`.

```bash
./bin/acars_parser unparse -type pdc -contains "EXPECT RUNWAY" -dry-run
```

| Flag | Default | Description |
|---|---|---|
| `-id` | none | Unparse one message |
| `-type` | none | Filter by parser type |
| `-contains` | none | Filter by text (case-insensitive) |
| `-limit` | `100` | Maximum messages (0 means unlimited) |
| `-dry-run` | `false` | Show what would change without writing |
| `-no-optimize` | `false` | Skip `OPTIMIZE TABLE` |

At least one of `-id`, `-type` or `-contains` is required.

Because `messages` is a plain `MergeTree`, `OPTIMIZE ... FINAL` does not remove the original rows. After an unparse, both the parsed and the unparsed rows exist.

## Enrichment API

A REST API that serves per-flight enrichment data from PostgreSQL `flight_enrichment`. That table is populated by `live`, keyed on the aircraft address and flight number as transmitted.

```bash
./bin/enrichment-api -port 8081
curl http://localhost:8081/api/v1/enrichment/7C6CA3
```

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/health` | Health check |
| `GET` | `/api/v1/enrichment/{icao_hex}` | All flights for an aircraft today (UTC) |
| `GET` | `/api/v1/enrichment/{icao_hex}/{callsign}` | One flight today (UTC) |
| `GET` | `/api/v1/enrichment/{icao_hex}/{callsign}/{date}` | One flight on a date (`YYYY-MM-DD`) |
| `POST` | `/api/v1/enrichment/batch` | Up to 100 aircraft in one request |

API key authentication is available with `-auth -api-keys key1,key2` and is off by default. See [docs/enrichment-api.md](docs/enrichment-api.md) for the flags, response format and known limitations, and [api/openapi.yaml](api/openapi.yaml) for the OpenAPI specification.

## NATS relay

A service that connects to the Airframes NATS feed and drops duplicate messages. It republishes the remaining payloads, unchanged, to a single subject on an internal NATS server, so several internal consumers can share one upstream connection. It is configured entirely through environment variables and exposes `/healthz` and Prometheus `/metrics`.

```bash
AIRFRAMES_NATS_CREDS=/path/to/airframes_nats.creds ./bin/nats-relay
```

Duplicates are detected in two ways:

- by the Airframes message ID in the NATS subject
- by a hash of the label, tail and text within a time window

See [docs/nats-relay.md](docs/nats-relay.md) for the configuration, deduplication details, metrics, the Docker build and known limitations.

## Parsers

Parsers are registered with the registry in each package's `init()`. `internal/parsers/parsers.go` imports every parser package so that they are all registered.

For each message, the registry runs:

1. every parser registered for the message's label
2. every content-based (label-independent) parser

A parser contributes a result when its quick check passes and `Parse` returns a result. A message can produce several results, one per matching parser.

| Result type | Parser | Labels |
|---|---|---|
| `adsc` | ADS-C reports (binary tag decoding) | B6 |
| `agfsr` | AGFSR flight status | 4T |
| `atis` | D-ATIS | A9 |
| `cpdlc` | FANS-1/A CPDLC (ASN.1 PER), including connection management | AA, BA |
| `crew_list` | Crew lists | RA |
| `delay_summary` | IATA delay codes | 3E, RA |
| `dispatcher` | Dispatcher and MEL messages | RA, 25, H1 |
| `envelope` | Tail and station from AT1/CR1/ADS headers; A6 ADS-C position | AA, A6 |
| `eta` | ETA and timing formats | 5Z |
| `fst` | FST flight status | 15 |
| `fuel_delivery` | Fuel delivery receipts | 3E, RA |
| `gate_assignment` | Gate assignment | RA |
| `flight_plan` | H1 flight plans (FPN) | H1, 4A, HX |
| `h1_position` | H1 position reports (POS) | H1 |
| `pwi` | Predicted wind information | H1 |
| `mdc` | Maintenance and fault reports | H1 |
| `trajectory` | `++` trajectory messages | H1 |
| `h2_wind` | Wind layers | H2 |
| `hazard_alert` | ARINC Direct HAZARD ALERT | H1, SA |
| `label10_position` | Position with route and waypoint timing | 10 |
| `waypoint_position` | Waypoint position reports | 16 |
| `position_report` | POSN position reports | 21 |
| `label22_position` | Position in degrees/minutes/seconds | 22 |
| `label44` | Runway, FB and POS reports | 44 |
| `pos_weather` | Position with weather and fuel burn | 4J |
| `route` | Routes | 5L |
| `position` | Position and OOOI | 80 |
| `label83_position` | PR and ZSPD position reports | 83 |
| `oceanic_clearance` | Oceanic clearances | B2 |
| `gate_info` | Gate information | B3 |
| `flight_subscription` | SITA FDA (FDASUB, FDACOM, FSTREQ, FDAACK) | RF |
| `landing_data` | Landing performance | C1 |
| `loadsheet` | Loadsheets (18 formats) | 10, 13, 14, 22, 2A, 30, 31, 35, 3S, 42, 45, C1, H1, RA |
| `media_advisory` | Data link media advisory | SA |
| `parking_info` | Parking and gate information | 1E, RA |
| `pax_bag` | Passenger and baggage details | RA |
| `pax_conn_status` | Passenger connection status | 3E, RA |
| `pdc` | Pre-departure clearances (28 formats) | any (content-based) |
| `sq_position` | SQ ARINC position and AVICOM frequency | SQ |
| `takeoff_data` | Takeoff performance | RA, H1, C1 |
| `turbulence` | Turbulence reports | C1 |
| `weather` | METAR, TAF and SIGMET | RA, C1, 21, H1, 3W, 27, 31, 34, 3T, 23 |

See [docs/parsers.md](docs/parsers.md) for each parser's formats, fields, priority and matching technique.

The CPDLC parser decodes uplink and downlink messages, including multi-element messages, using `github.com/shaneshort/go-asn`. Route clearance elements (for example UM79, UM80 and UM83) are decoded but not converted into route data, so they appear with their ID and label only.

The PWI result looks like this:

```json
{
  "climb_winds": [
    {"flight_level": 100, "wind_dir": 252, "wind_speed": 39}
  ],
  "route_winds": [
    {
      "flight_level": 360,
      "waypoints": [
        {"waypoint": "DOLEV", "wind_dir": 321, "wind_speed": 74, "temperature": -57}
      ]
    }
  ],
  "descent_winds": [
    {"flight_level": 100, "wind_dir": 305, "wind_speed": 22}
  ]
}
```

### Adding a parser

[docs/PARSER_SPEC.md](docs/PARSER_SPEC.md) is the specification for new parsers. It covers the format definitions, the grok-style patterns in `internal/patterns`, tracing and tests. In outline:

1. Create `internal/parsers/<name>/` with a `parser.go` that implements `registry.Parser` and calls `registry.Register` in `init()`:

   ```go
   type Parser interface {
       Name() string                     // Unique identifier
       Labels() []string                 // ACARS labels to match (empty = content-based, checks all)
       QuickCheck(text string) bool      // Fast pre-filter (strings.Contains, not regex)
       Priority() int                    // Lower values sort first within a label
       Parse(msg *acars.Message) Result  // Returns nil if not applicable
   }
   ```

2. Implement `registry.Traceable` (`ParseWithTrace`) so that `acars_parser debug` can explain matches.
3. Add the package import to `internal/parsers/parsers.go`.
4. Add tests, then run the baseline gate (see [Development](#development)). A new parser changes the baseline for any sampled message it matches. Review those changes, then re-record the baseline.

## Standalone tools

The tools are part of the root module. Build them with `go build ./tools/<name>` (see [Building](#building)). They do not read the environment variables used by `acars_parser`.

### analyzer

Analyses the message corpus in ClickHouse: label distribution, parser coverage and format patterns. It can also suggest and test regex patterns.

| Flag | Default | Description |
|---|---|---|
| `-ch-host`, `-ch-port`, `-ch-user`, `-ch-password`, `-ch-db` | `localhost`, `9000`, `default`, empty, `acars` | ClickHouse connection |
| `-format` | `text` | `text` or `json` |
| `-templates` | `false` | Include template analysis (slower) |
| `-top` | `20` | Items shown per category |
| `-label` | none | Analyse one label only |
| `-suggest` | `false` | Generate pattern suggestions (requires `-label`) |
| `-min-cluster` | `3` | Minimum cluster size for suggestions |
| `-test` | none | Test a regex against the corpus (requires `-label`) |

### kmlexport

Exports waypoints from PostgreSQL to KML, for Google Earth or other GIS applications.

```bash
./bin/kmlexport -pg-db acars_state -pg-password acars -min-sources 50 -output waypoints.kml
```

| Flag | Default | Description |
|---|---|---|
| `-pg-host`, `-pg-port`, `-pg-user`, `-pg-password`, `-pg-db` | `localhost`, `5432`, `acars`, empty, `acars` | PostgreSQL connection |
| `-output` | stdout | Output KML file |
| `-min-sources` | `1` | Minimum source count per waypoint |
| `-stats` | `false` | Show statistics only |
| `-v` | `false` | Verbose output |

### routeexport

Exports routes from PostgreSQL to CSV for the planewatch-atc `import_routes.rake` task.

```bash
./bin/routeexport -pg-db acars_state -pg-password acars -min-obs 100 -output routes.csv
```

| Flag | Default | Description |
|---|---|---|
| `-pg-host`, `-pg-port`, `-pg-user`, `-pg-password`, `-pg-db` | `localhost`, `5432`, `acars`, empty, `acars` | PostgreSQL connection |
| `-output` | stdout | Output CSV file |
| `-min-obs` | `1` | Minimum observation count per route |
| `-stats` | `false` | Show statistics only |
| `-v` | `false` | Verbose output |

The CSV has no header row. Each line is `callsign,ICAO1,ICAO2,...`, with multi-stop routes listing every airport in sequence:

```
QFA1,YSSY,WSSS,EGLL
JL300,RJTT,RJCC
```

Multi-stop legs come from the `route_legs` table, which only `migrate` populates.

## Documentation

| Document | Contents |
|---|---|
| [docs/storage.md](docs/storage.md) | ClickHouse and PostgreSQL schemas, and which commands use each table |
| [docs/parsers.md](docs/parsers.md) | Parser reference |
| [docs/PARSER_SPEC.md](docs/PARSER_SPEC.md) | Specification for writing parsers |
| [docs/enrichment-api.md](docs/enrichment-api.md) | Enrichment API reference |
| [docs/nats-relay.md](docs/nats-relay.md) | NATS relay reference |
| [docs/airframes-payload.md](docs/airframes-payload.md) | The upstream Airframes NATS payload: every field, and which ones the code reads |
| [docs/FLIGHT_MESSAGE_FLOW.md](docs/FLIGHT_MESSAGE_FLOW.md) | The ACARS messages observed across the phases of a flight |
| [docs/investigation-notes.md](docs/investigation-notes.md) | Working notes on unparsed message formats |
| [docs/reference/](docs/reference/) | ICAO GOLD, ADS-C and CPDLC reference material |

## Development

```bash
go build ./...
go vet ./...
golangci-lint run ./...   # configuration in .golangci.yml
go test ./...
```

The PostgreSQL integration tests in `internal/storage` are skipped when no database is reachable.

### Parser regression baseline

`go test` includes `TestBaseline` in `internal/parsers`.

- It re-parses every message in the recorded sample (`internal/parsers/testdata/baseline/`).
- It fails on **any** added, removed or changed result, and reports each difference by parser and field (as a JSON Pointer path).
- New results count as changes, so a parser that starts matching messages it should not match is caught too.
- It also fails if the fixture files do not match the manifest's per-file case counts, so a deleted or truncated file cannot quietly reduce coverage.

When every reported difference is intended, re-record the expectations and commit them with the parser change, so that the diff shows the effect of the change:

```bash
go test -buildvcs=true ./internal/parsers -run TestBaseline -update-baseline
```

Pass the package path as shown. Other packages do not define the flag, so `go test ./... -update-baseline` fails them. `-buildvcs=true` is required: without it the test binary does not know its commit, and the update refuses to run rather than record an unknown version.

Use `acars_parser baseline` to draw a new sample, for example after the stored corpus has grown.
