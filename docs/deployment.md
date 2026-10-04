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
| `NATS_CREDS` | none (required for `live`) | The path, on the host, of the Airframes NATS credentials file. It is mounted read-only and is never copied into the image or the repository. Without it, `up live` fails; other commands do not need it. |
| `CLICKHOUSE_PASSWORD` | `acars` | The ClickHouse password, for the server and the clients. |
| `POSTGRES_PASSWORD` | `acars` | The PostgreSQL password, for the server and the clients. |

Compose reads these from the environment or from a `deployments/.env` file, which is excluded from git and from the image build context.

## First start

From the repository root:

```bash
export NATS_CREDS=$HOME/airframes_nats.creds
docker compose -f deployments/docker-compose.yml up -d clickhouse postgres
docker compose -f deployments/docker-compose.yml run --rm schema
docker compose -f deployments/docker-compose.yml up -d --build live
```

## Moving the data from another host

The message archive is copied in ClickHouse's Native format, and the state with `pg_dump`. Nothing may write to either database while the copy is taken, or the copy misses those changes: `live` updates PostgreSQL (aircraft, routes, ATIS, flight state and enrichment) as well as ClickHouse, and a later message-only copy does not bring the PostgreSQL changes. The destinations must be empty: ClickHouse's `messages` table keeps duplicate rows, and PostgreSQL rejects existing keys.

1. On the old host, stop `live`.
2. On the old host, export both databases:

   ```bash
   set -o pipefail
   docker exec acars-clickhouse clickhouse-client --password acars \
       --query "SELECT * FROM acars.messages FORMAT Native" | zstd > messages.native.zst
   docker exec acars-postgres pg_dump -U acars --data-only acars_state | zstd > state.sql.zst
   ```

3. Copy both files to the new host.
4. On the new host, start the databases and create the tables, without starting `live`:

   ```bash
   docker compose -f deployments/docker-compose.yml up -d clickhouse postgres
   docker compose -f deployments/docker-compose.yml run --rm schema
   ```

5. On the new host, import both, PostgreSQL in a single transaction that stops at the first error:

   ```bash
   set -o pipefail
   zstd -dc messages.native.zst | docker exec -i acars-clickhouse clickhouse-client --password acars \
       --query "INSERT INTO acars.messages FORMAT Native"
   zstd -dc state.sql.zst | docker exec -i acars-postgres \
       psql -U acars -d acars_state -v ON_ERROR_STOP=1 --single-transaction
   ```

6. Check the row counts against the old host, then start `live` on the new host.

If an import fails, empty the destination before retrying (`TRUNCATE TABLE acars.messages` in ClickHouse; for PostgreSQL, `docker compose -f deployments/docker-compose.yml down -v` removes both volumes, after which steps 4 and 5 start again).

## Operation

```bash
docker compose -f deployments/docker-compose.yml logs -f live   # the live output
docker compose -f deployments/docker-compose.yml up -d --force-recreate live   # after a configuration change
docker compose -f deployments/docker-compose.yml up -d --build live   # after a code change
```

`live` restarts automatically unless it was stopped. Its logs are rotated (five files of 50 MB).

`restart` would keep the container's old environment and mounts, so configuration changes need `--force-recreate`. Changing `POSTGRES_PASSWORD` or `CLICKHOUSE_PASSWORD` does not change the password of a database whose volume is already initialised; change it in the database too.
