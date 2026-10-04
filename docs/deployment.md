# Deployment

`deployments/docker-compose.yml` runs the stack with Docker Compose:

| Service | Container | Purpose |
|---|---|---|
| `clickhouse` | `acars-clickhouse` | The message archive (`acars.messages`). Data in the `clickhouse-data` volume. |
| `postgres` | `acars-postgres` | Mutable state (database `acars_state`). Data in the `postgres-data` volume. |
| `live` | `acars-live` | `acars_parser live`: subscribes to the Airframes NATS feed, parses each message and stores it. |
| `schema` | (one-off) | `acars_parser migrate -skip-messages -skip-state`: creates the ClickHouse and PostgreSQL tables. In the `tools` profile, so it runs only when asked for. |
| `dbviewer` | `acars-dbviewer` | A read-only web interface to the PostgreSQL state database ([pgweb](https://github.com/sosedoff/pgweb)), on port 8081. See [State database viewer](#state-database-viewer). |
| `dbviewer-role` | (one-off) | Runs `deployments/postgres/dbviewer-role.sql`: creates or updates the read-only `acars_viewer` role that `dbviewer` connects as. In the `tools` profile. |

The database ports are published on 127.0.0.1 only, so the CLI on the host can reach them with the default settings, and nothing else can. The `dbviewer` port is also published on 127.0.0.1 unless `DBVIEWER_BIND` says otherwise.

## Image

`cmd/acars_parser/Dockerfile` builds the `acars_parser` binary. The ICAO Doc 8643 type list is not in the repository; the build fetches it from ICAO (`go generate ./internal/aircrafttype`), so it needs network access to `doc8643.icao.int`.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `NATS_CREDS` | none (required for `live`) | The path, on the host, of the Airframes NATS credentials file. It is mounted read-only and is never copied into the image or the repository. Without it, `up live` fails; other commands do not need it. |
| `CLICKHOUSE_PASSWORD` | `acars` | The ClickHouse password, for the server and the clients. |
| `POSTGRES_PASSWORD` | `acars` | The PostgreSQL password, for the server and the clients. |
| `DBVIEWER_PASSWORD` | `acars_viewer` | The password of the read-only `acars_viewer` role, set by `dbviewer-role` and used by `dbviewer`. |
| `DBVIEWER_BIND` | `127.0.0.1` | The host address that `dbviewer`'s port 8081 is published on. `0.0.0.0` opens it to the network (the collector sets this in `deployments/.env`). |

Compose reads these from the environment or from a `deployments/.env` file, which is excluded from git and from the image build context.

## First start

From the repository root:

```bash
export NATS_CREDS=$HOME/airframes_nats.creds
docker compose -f deployments/docker-compose.yml up -d clickhouse postgres
docker compose -f deployments/docker-compose.yml run --rm schema
docker compose -f deployments/docker-compose.yml up -d --build live
docker compose -f deployments/docker-compose.yml run --rm dbviewer-role
docker compose -f deployments/docker-compose.yml up -d dbviewer
```

## State database viewer

`dbviewer` runs pgweb, a single-binary PostgreSQL browser, which serves the tables, their rows and structure, and an SQL query box at `http://<host>:8081` (for the collector, http://acars-collector.local:8081). pgweb is used because it is a maintained tool that runs as one container and has a read-only mode, so the project does not need a web application of its own.

It cannot change stored data because of the role it connects as. The role script drops `acars_viewer` and creates it afresh on every run (refusing if the existing role owns anything, since dropping it would delete what it owns), so the role holds only `CONNECT` on `acars_state`, `USAGE` on the `public` schema and `SELECT` on its tables (default privileges extend that to tables the `acars` role creates later), and is a member of no other role. The role script also revokes from `PUBLIC`, which every role belongs to, the two privileges that need no table grant: creating temporary tables and creating large objects (`lo_create`, `lo_creat`, `lo_from_bytea`), since a large object persists. `acars_parser` uses neither, and its `acars` role is a superuser and unaffected.

Two further layers do not stop writes on their own:

- pgweb's `--readonly` rejects queries containing some write keywords (`UPDATE`, `CREATE` and so on), but not all writes: `BEGIN READ WRITE; SELECT lo_from_bytea(...)` passed it before the revoke above. `--lock-session` stops the interface from connecting to another server or database or as another user.
- The role's transactions start read-only, but the role can switch that off (`BEGIN READ WRITE`).

The protection covers stored data. Actions with only session effects remain open to the role, as to any database user: sending a notification (`pg_notify`), taking advisory locks, or changing its own session settings.

pgweb keeps one database connection. On the collector, a query that opened a transaction (`BEGIN ...`) and then failed left that connection in an aborted transaction: every later query failed with "current transaction is aborted", and a `ROLLBACK` sent through pgweb failed the same way. Restart the viewer to reset it: `docker compose -f deployments/docker-compose.yml restart dbviewer`.

The enrichment API's examples also use port 8081; on the same host, run the API on another port.

It has no authentication: anyone who can reach port 8081 can read every table. By default it is published on the host's loopback address only; reach it through an SSH tunnel, or set `DBVIEWER_BIND=0.0.0.0` (in the environment or `deployments/.env`) and recreate it with `up -d dbviewer` to open it to the local network. Do not publish the port to the internet. The tunnel:

```bash
ssh -N -L 8081:127.0.0.1:8081 acars-collector.local   # then open http://localhost:8081
```

### Setting up the role

The role is created by `deployments/postgres/dbviewer-role.sql`, run by the one-off `dbviewer-role` service. It is not an init script of the `postgres` service, because PostgreSQL runs those only when its volume is first initialised, and existing deployments already are. The script is idempotent: run it before the first `up dbviewer`, after changing `DBVIEWER_PASSWORD` (followed by `up -d --force-recreate dbviewer`), and at any other time without harm.

```bash
docker compose -f deployments/docker-compose.yml run --rm dbviewer-role
docker compose -f deployments/docker-compose.yml up -d dbviewer
```

## Moving the data from another host

The message archive is copied in ClickHouse's Native format, and the state with `pg_dump`. Nothing may write to either database while the copy is taken, or the copy misses those changes: `live` updates PostgreSQL (aircraft, routes, ATIS, flight state and enrichment) as well as ClickHouse, and a later message-only copy does not bring the PostgreSQL changes. The destinations must be empty: ClickHouse's `messages` table keeps duplicate rows, and PostgreSQL rejects existing keys.

1. On the old host, stop `live`.
2. On the old host, export both databases:

   ```bash
   set -euo pipefail
   docker exec acars-clickhouse clickhouse-client --password acars \
       --query "SELECT * FROM acars.messages FORMAT Native" | zstd > messages.native.zst
   docker exec acars-postgres pg_dump -U acars --data-only acars_state | zstd > state.sql.zst
   ```

   With `set -e`, a failed export stops the block instead of being followed by the next one.

3. Copy both files to the new host.
4. On the new host, start the databases and create the tables, without starting `live`:

   ```bash
   docker compose -f deployments/docker-compose.yml up -d clickhouse postgres
   docker compose -f deployments/docker-compose.yml run --rm schema
   ```

5. On the new host, check both archives, then import them, PostgreSQL in a single transaction that stops at the first error:

   ```bash
   set -euo pipefail
   zstd -t messages.native.zst state.sql.zst
   zstd -dc messages.native.zst | docker exec -i acars-clickhouse clickhouse-client --password acars \
       --query "INSERT INTO acars.messages FORMAT Native"
   zstd -dc state.sql.zst | docker exec -i acars-postgres \
       psql -X -U acars -d acars_state -v ON_ERROR_STOP=1 --single-transaction -f -
   ```

   `zstd -t` checks each archive in full first, so a damaged archive cannot end the input early and leave a partial import committed. `--single-transaction` needs `-f` (here `-f -`, standard input).

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
