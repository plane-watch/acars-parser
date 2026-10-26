# Storage

The project stores data in two databases:

- **ClickHouse** holds the append-only message archive (`messages`) and an ATIS history table.
- **PostgreSQL** holds mutable state derived from messages: aircraft, waypoints, routes, current ATIS, flight state, flight enrichment and golden annotations.

The schema definitions live in `internal/storage/clickhouse.go` (`ClickHouseDB.CreateSchema`) and `internal/storage/postgres.go` (`PostgresDB.CreateSchema`). This document describes those definitions and which commands read and write each table.

## Connection settings

Every subcommand of `acars_parser` takes its default connection settings from environment variables (see `cmd/acars_parser/config.go`). The command-line flags override them.

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

The enrichment API reads the same `POSTGRES_*` variables. The standalone tools in `tools/` do not read environment variables. Their `-pg-db` flag defaults to `acars`, and their password flags default to empty, so pass `-pg-db acars_state -pg-password acars` when using the default setup.

## Schema creation

| Schema | Created by |
|---|---|
| ClickHouse tables | `live` (which also adds columns missing from an existing `messages` table), `migrate` |
| PostgreSQL tables | `live`, `backfill`, `migrate` |

Parsers need the block ID or the link direction to tell an uplink from a downlink (CPDLC messages relayed on H1 cannot be decoded without it). Rows stored before October 2026 have neither, so for an H1 message `reparse -rebuild` and the `reparse` comparison take the direction that a parse result stored for the message recorded (`direction` in a CPDLC result). On H1 the CPDLC parser has no label fallback, so that direction came from the link fields or from the message itself. For AA and BA the label decides the direction, so an earlier parser version's stored guess is not used.

The commands that read `messages` (`query`, `debug`, `reparse`, `unparse` and `backfill`) check first that it has the block columns, and stop with an error naming the command to run if it does not; they do not change the table themselves.

Neither database is created automatically. The ClickHouse database named by `CLICKHOUSE_DATABASE` must already exist, because the connection selects it, and `Ping` fails otherwise.

On a fresh installation, create the ClickHouse tables without migrating any SQLite data:

```bash
./acars_parser migrate -skip-messages -skip-state
```

`review` does not create the PostgreSQL schema. Run one of the commands above first, or its annotation writes fail.

## ClickHouse

### `messages`

`live` writes one row **per parser result**, not one per message:

- When several parsers return a result for the same message, each result gets its own row with the same `id` and a different `parser_type`.
- A message with no results gets a single row with `parser_type = 'unparsed'`, and `parsed_json` contains only the label.

`id` is therefore not unique.

A message that `live` joins from its blocks (see [reassembly.md](reassembly.md)) is stored as well as its blocks, under the first block's ID with bit 62 set, and lists the blocks' IDs in `assembled_from`.

| Column | Type | Notes |
|---|---|---|
| `id` | `UInt64` | The Airframes message ID |
| `timestamp` | `DateTime64(3)` | The message timestamp |
| `label` | `LowCardinality(String)` | The ACARS label |
| `parser_type` | `LowCardinality(String)` | The result's `Type()`, or `unparsed` |
| `flight` | `LowCardinality(String)` | |
| `tail` | `LowCardinality(String)` | |
| `origin` | `LowCardinality(String)` | |
| `destination` | `LowCardinality(String)` | |
| `raw_text` | `String` | The original message text |
| `parsed_json` | `String` | The JSON-encoded parser result |
| `missing_fields` | `String` | Comma-separated list of expected fields that were empty |
| `confidence` | `Float32` | |
| `message_number` | `LowCardinality(String)` | The ACARS message number (`D47A`); for a message joined from its blocks, without the block letter (`D47`). Empty in rows stored before October 2026 |
| `block_id` | `LowCardinality(String)` | The ACARS block ID |
| `block_end` | `Nullable(Bool)` | The feed's `block_end` flag, which does not reliably mark a message's last block; NULL when not sent, and in rows stored before October 2026 |
| `assembled_from` | `Array(UInt64)` | For a message joined from its blocks, the blocks' IDs in order; otherwise empty |
| `link_direction` | `LowCardinality(String)` | The link direction the feed sent (`uplink`, `downlink`, or empty, as for most ACARS sources, whose direction follows from the block ID) |
| `created_at` | `DateTime64(3)` | Defaults to `now64(3)` |

- **Engine:** `MergeTree()`, partitioned by `toYYYYMM(timestamp)`, ordered by `(parser_type, label, timestamp, id)`.
- **Index:** a token bloom filter index (`idx_raw_text_bloom`, `tokenbf_v1(32768, 3, 0)`) on `raw_text`. It supports `query -search`, which is a `LIKE '%text%'` match.

**Row replacement does not happen.** `MergeTree` never deduplicates, but two commands write a new row intended to replace an existing one:

- `reparse -update`
- `unparse`

The original row remains alongside the new one, including after `OPTIMIZE TABLE ... FINAL`. Queries on `id` can therefore return more than one row for a message that has been reparsed or unparsed. Both code paths carry a `TODO` describing the problem. `reparse -rebuild` reparses the whole archive without this problem: it writes a new table and makes it `messages` with an atomic `EXCHANGE TABLES`, keeping the old archive as `messages_previous` until it is dropped (see the README).

`live -exclude` (default `sq_position`) also skips storage for the excluded types, so those results are not written to `messages`.

### `atis_history`

This table has the same fields as `atis_current` (see below), stored as strings, with a `recorded_at` timestamp.

- **Engine:** `MergeTree()`, partitioned by `toYYYYMM(recorded_at)`, ordered by `(airport_icao, recorded_at, id)`.
- No command currently writes to this table.

## PostgreSQL

| Table | Purpose | Key | Written by |
|---|---|---|---|
| `aircraft` | ICAO hex to registration and type | `icao_hex` | `live`, `backfill`, `migrate` |
| `waypoints` | Named waypoints with coordinates and an observation count | `name` | `live`, `backfill`, `migrate` |
| `routes` | Observed flight number to origin/destination pairs | `(flight_pattern, origin_icao, dest_icao)` | `live`, `backfill`, `migrate` |
| `route_legs` | Legs of multi-stop routes | `(route_id, sequence)` | `migrate` only |
| `route_aircraft` | Registrations seen on each route | `(route_id, registration)` | `migrate` only |
| `aircraft_callsigns` | IATA/ICAO callsign prefix per registration | `registration` | `migrate` only |
| `atis_current` | The latest ATIS per airport | `airport_icao` | `live`, `backfill`, `migrate` |
| `flight_state` | Per-flight tracking state | `key` | `backfill`, `migrate` |
| `golden_annotations` | Review annotations keyed by ClickHouse message ID | `message_id` | `review`, `migrate` |
| `flight_enrichment` | Per-flight data served by the enrichment API | `(icao_hex, callsign, flight_date)` | `live` |
| `flight_aliases` | Flights named in both forms by a message: IATA flight number and ICAO callsign, with an observation count | `(iata_flight, icao_callsign)` | `live`, `backfill` |
| `airline_codes` (view) | Airline code pairs summed from `flight_aliases`, with how many had the same number in both forms | — | — |

Some tables have only one writer:

- `route_legs`, `route_aircraft` and `aircraft_callsigns` are populated only by the SQLite migration. `live` and `backfill` do not maintain them. `routeexport` builds each route from its legs, so it exports only migrated routes.
- `routes` stores a flight under the identifier the messages gave, in either form; `flight_aliases` and `airline_codes` relate the two forms (see [flight-routes.md](flight-routes.md)).
- `flight_state` is not written by `live`.

Only transmitted data is stored. `live` and `backfill` do not take facts from Airframes' `airframe`, `flight` or `station` records (see [airframes-payload.md](airframes-payload.md)):

- **ICAO hex.** It comes from one of four sources, strongest first:
  1. the aircraft's link-layer address;
  2. an ADS-C airframe ID;
  3. the aircraft address in an AFN logon header, used only for the registration it was reported with, and not when it contradicts an N-number derivation;
  4. derivation from a US N-number registration.

  The extractor records which, as `icao_hex_source`, ready for storage v2.
- **Registration.** The transmitted tail. Without one, the first registration a parser result names, unless that result reports an aircraft address other than the link-layer one.
- **Other aircraft.** A parser result that names another registration than the message's, or reports an aircraft address other than the link-layer one, describes another aircraft (or is wrong): none of its data (flight, route, address) is used.
- **Type.** The ICAO designator of the transmitted type (`internal/aircrafttype`), only when the transmitted value identifies exactly one. The raw value is kept by the extractor. An A320 family model number whose engine digit is 5 (CFM LEAP-1A) or 7 (PW1100G) is a neo even without the "N" suffix: United sends its A321neo as `A321-271`.
- **Operator.** No longer written, because it came only from Airframes.
- **Routes.** Only pairs where both endpoints are ICAO codes are written, because the `routes` columns are ICAO codes. The extractor keeps IATA pairs as transmitted, for storage v2. A route is written only with a flight number: a route seen without one is an aircraft's movement, not a flight's route, and is not written (it used to be written with the registration as its flight pattern).
- **Flight.** When the transmitted flight is in the IATA form, the ATC callsign (ICAO form) a parser result gives for it: from a result about the flight in progress (a departure clearance, an FMS progress report or a flight plan), whatever its number (LO495 flies as LOT49H); otherwise from another result only when it has the transmitted flight's number and suffix. Otherwise the transmitted flight, and without one, the first flight a parser result names (e.g. a loadsheet's flight). The transmitted flight comes from the aircraft's ACARS unit and can carry another airline's code: Qatar Airways 777s that Virgin Australia wet leases keep transmitting `VA0869` while flying Qatar's QTR869, a flight number Virgin also flies as VOZ869. Keying by the callsign keeps the two flights' routes apart.
- **Flight of a message without a callsign.** It takes the callsign that a result about the flight in progress last gave for the same aircraft (the transmitted tail, or without one the link-layer address) and transmitted flight, within the 12 hours before the message (`extractor.CallsignMemory`, with 5 minutes' grace for a joined message, which carries its first block's time). A callsign from another report (such as a CMC report, which can be stored on an earlier leg flown under another callsign) keys only its own message. `backfill` reads the archive in time order and sends each aircraft's messages to one worker for this.
- **Route and flight pairing.** A parser result that names its own flight (such as a CMC report, which can be stored on one flight and sent on a later one) lends its route only to the same flight. Flights match when they have the same flight number and suffix, and the same airline when both use the same form of airline code (`UAL482` does not match `THA482`; `B6123` is B6 flight 123). There is no table of IATA and ICAO airline codes, so an IATA and an ICAO form with the same number (`TG482` and `THA482`) are taken to match. A callsign that is the transmitted tail (a registration used as the callsign, such as `N123AB`) is compared whole, not split into an airline and a number. A result that names its flight by number only (`flight_number_digits`, as an Airbus ACMS report does) lends its route only when the transmitted flight has that number; a flight named by another result is not enough.

Rows migrated from January may hold Airframes-derived values.

Upsert behaviour:

- Reference tables increment their `observation_count`, `source_count` or `msg_count` and update `last_seen` on conflict.
- An empty registration, type or operator does not overwrite a stored value.
- `synced_at` columns exist on `aircraft`, `waypoints`, `routes` and `atis_current`, but nothing in this repository sets them.

`golden_annotations` also has a partial index on `is_golden` where it is true.

See [enrichment-api.md](enrichment-api.md) for the `flight_enrichment` columns and how rows are matched and merged.

## Migrating from SQLite

`acars_parser migrate` copies the legacy SQLite databases into ClickHouse and PostgreSQL:

- `messages.db` goes to ClickHouse `messages`.
- `state.db` goes to the PostgreSQL state tables.

It is a one-off tool for older installations. It is also the only command that creates the ClickHouse schema, which is why a fresh installation needs `migrate -skip-messages -skip-state`.

`-dry-run` connects to both databases and creates the schemas before reporting counts. It does not copy any data.
