## Rules. DO NOT break these rules.
- Be brutally honest, don't be a yes man.
- If I am wrong, point it out bluntly.
- I need honest feedback on my code.
- If I am not following best practices, tell me.
- If you're not sure or don't understand something, ALWAYS ask for clarification.
- Ensure comprehensive commenting and documentation, especially for complex logic-- and avoid the use of hyperbolic language.
- Ensure that all code is properly formatted and adheres to the project's style guide, if present-- else use a consistent style.
- Make sure to fix problems properly, not just superficially.
- Consistency is key, inspect what's already been done and think carefully before suggesting consistent options.
- follow the 'Principal of the least surprise'-- don't surprise anyone with unexpected behaviour. Implement one, obvious way to do things.
- Don't create "temp files" when testing or debugging, use proper logging instead.
- Leverage existing tools and libraries where possible, don't reinvent the wheel.
- Keep things simple. If a solution looks complex, let's evaluate if it can be simplified.
- Mark any unfinished or questionable code with `TODO` comments and explain why it is there.
- If you find any TODOs, address them or explain why they are still necessary.
- When updating documentation, treat it as the current state-- don't talk about improvements or plans unless explicitly stated.
- When writing comments-- ensure you include the article when referencing, i.e. 'the' or 'a'.
- Ensure all code is written to Australian/British English. it's 'colour' not 'color'.
- Think critically about what you're doing-- if it doesn't seem like a good idea or if it feels like a shortcut, don't do it.
- When doing any unit testing etc, redirect the output to a temp file then grep for your results, so you don't have to run it a second time to see the full output.
- Ensure all code generated is run through the appropriate language linters to ensure good quality code is being generated.
- Try and abide by all IDE warnings and errors, fixing them where feasible.
- Don't begin the names of interfaces/classes with the package name.
- If there's an issue, fix it properly, don't go for the 'quick fix'
- Go will not run files ending in `_test.go` with `go run` - use `go test` instead or name the file differently
- Avoid creating temp files for testing - write proper tests in the project's test infrastructure
- If you must create temp files, write them to /tmp/ and clean them up when you're done.

Project Information:
Read the README.md file (if present) for project details.

Maintain the project structure per googles recommendations: https://raw.githubusercontent.com/golang-standards/project-layout/refs/heads/master/README.md

When making changes, ensure the following:
- Document any new features or changes in the README.md file. As well as writing any feature specific documentation into docs/ (updating any existing ones if required)
- Use meaningful commit messages that clearly describe the changes made.

If there are any useful tools, resources or information I provide, update this file accordingly.

## Database Architecture

The project uses a two-database architecture:
- **ClickHouse**: Append-only message storage (`messages`, plus an unused `atis_history` table)
- **PostgreSQL**: Mutable state data (aircraft, waypoints, routes, ATIS, flight state, flight enrichment, golden annotations)

Full schema and the commands that read and write each table: `docs/storage.md`.

Connection defaults come from environment variables (`CLICKHOUSE_*`, `POSTGRES_*`, see `cmd/acars_parser/config.go`), so the connection flags are usually unnecessary. The location of the running environment is not recorded; confirm the host, container names and PostgreSQL database name with the user before querying.

### ClickHouse Database

Container name (as created by the README): `acars-clickhouse`

Connection:
- Host: `localhost`
- Port: `9000`
- User: `default`
- Password: `acars`
- Database: `acars`

Query example:
```bash
docker exec -i acars-clickhouse clickhouse-client --password acars --query "SELECT * FROM acars.messages LIMIT 1"
```

Reparse example (compares only; add `-update` to write, which creates duplicate rows):
```bash
./acars_parser reparse -type unparsed
```

Schema for `acars.messages` (`ENGINE = MergeTree`, `ORDER BY (parser_type, label, timestamp, id)`):
| Column | Type |
|--------|------|
| id | UInt64 |
| timestamp | DateTime64(3) |
| label | LowCardinality(String) |
| parser_type | LowCardinality(String) |
| flight | LowCardinality(String) |
| tail | LowCardinality(String) |
| origin | LowCardinality(String) |
| destination | LowCardinality(String) |
| raw_text | String |
| parsed_json | String |
| missing_fields | String |
| confidence | Float32 |
| created_at | DateTime64(3) |

Notes:
- Use `parser_type = 'unparsed'` to find unparsed messages
- Use `raw_text` for the message content (not `text`)
- `id` is not unique: `live` writes one row per parser result, and `unparse` / `reparse -update` add rows without removing the originals (plain MergeTree does not deduplicate)

### PostgreSQL Database

Connection:
- Host: `localhost`
- Port: `5432`
- User: `acars`
- Password: `acars`
- Database: `acars_state` (the code default; the standalone tools in `tools/` default to `acars`)

Tables:
- `aircraft` - Aircraft registry (icao_hex, registration, type_code, operator)
- `waypoints` - Navigation waypoints (name, lat/lon, source_count)
- `routes` - Flight routes (flight_pattern, origin, dest, observation_count)
- `route_legs` - Individual route segments (populated by `migrate` only)
- `route_aircraft` - Aircraft seen on routes (populated by `migrate` only)
- `aircraft_callsigns` - IATA/ICAO callsign mappings (populated by `migrate` only)
- `atis_current` - Current ATIS for airports
- `flight_state` - Ephemeral flight tracking state
- `flight_enrichment` - Per-flight data served by the enrichment API
- `golden_annotations` - Message annotations for parser testing

## Tooling

- Linting: `golangci-lint run ./...` (configuration in `.golangci.yml`); formatting: `gofmt`.
- `github.com/shaneshort/go-asn` is required by version from GitHub. A local, gitignored `go.work` may point it at `/Users/shanes/Documents/development/go-asn`; changes to go-asn must be tagged and pushed before acars_parser can require them. Use `GOWORK=off go build ./...` to check the build without the workspace.
- All binaries, including `tools/*`, belong to the root Go module. Build into `bin/` (gitignored), e.g. `go build -o bin/acars_parser ./cmd/acars_parser`.
- `.gitignore` patterns for build output must be anchored (e.g. `/acars_parser`). An unanchored `acars_parser` pattern previously matched `cmd/acars_parser/` and kept the CLI source out of git.
