# Deployment

`deployments/docker-compose.yml` runs the stack with Docker Compose:

| Service | Container | Purpose |
|---|---|---|
| `clickhouse` | `acars-clickhouse` | The message archive (`acars.messages`). Data in the `clickhouse-data` volume. |
| `postgres` | `acars-postgres` | Mutable state (database `acars_state`). Data in the `postgres-data` volume. |
| `live` | `acars-live` | `acars_parser live`: subscribes to the Airframes NATS feed, parses each message and stores it. |
| `schema` | (one-off) | `acars_parser migrate -skip-messages -skip-state`: creates the ClickHouse and PostgreSQL tables. In the `tools` profile, so it runs only when asked for. |

The database ports are published on 127.0.0.1 only, so the CLI on the host can reach them with the default settings, and nothing else can.

## Image

`cmd/acars_parser/Dockerfile` builds the `acars_parser` binary. The ICAO Doc 8643 type list is not in the repository; the build fetches it from ICAO (`go generate ./internal/aircrafttype`), so it needs network access to `doc8643.icao.int`.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `NATS_CREDS` | none (required for `live`) | The path, on the host, of the Airframes NATS credentials file. It is mounted read-only and is never copied into the image or the repository. |
| `CLICKHOUSE_PASSWORD` | `acars` | The ClickHouse password, for the server and the clients. |
| `POSTGRES_PASSWORD` | `acars` | The PostgreSQL password, for the server and the clients. |

Compose reads these from the environment or from a `deployments/.env` file (gitignored).

## First start

From the repository root:

```bash
export NATS_CREDS=$HOME/airframes_nats.creds
docker compose -f deployments/docker-compose.yml up -d clickhouse postgres
docker compose -f deployments/docker-compose.yml run --rm schema
docker compose -f deployments/docker-compose.yml up -d --build live
```

## Moving the data from another host

The message archive is copied in ClickHouse's Native format, and the state with `pg_dump`. Run the schema step on the new host first.

```bash
# On the old host.
docker exec acars-clickhouse clickhouse-client --password acars \
    --query "SELECT * FROM acars.messages FORMAT Native" | zstd > messages.native.zst
docker exec acars-postgres pg_dump -U acars --data-only acars_state | zstd > state.sql.zst

# On the new host, after copying both files.
zstd -dc messages.native.zst | docker exec -i acars-clickhouse clickhouse-client --password acars \
    --query "INSERT INTO acars.messages FORMAT Native"
zstd -dc state.sql.zst | docker exec -i acars-postgres psql -U acars -d acars_state
```

Messages received between the copy and the switch to the new host can be copied afterwards with a `WHERE created_at > ...` condition.

## Operation

```bash
docker compose -f deployments/docker-compose.yml logs -f live   # the live output
docker compose -f deployments/docker-compose.yml restart live   # after a configuration change
docker compose -f deployments/docker-compose.yml up -d --build live   # after a code change
```

`live` restarts automatically unless it was stopped. Its logs are rotated (five files of 50 MB).
