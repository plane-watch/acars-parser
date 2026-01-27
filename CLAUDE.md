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

When making changes, ensure the following:
- Document any new features or changes in the README.md file. As well as writing any feature specific documentation into doc/ (updating any existing ones if required)
- Use meaningful commit messages that clearly describe the changes made.

If there are any useful tools, resources or information I provide, update this file accordingly.

## Database Architecture

The project uses a two-database architecture:
- **ClickHouse**: Immutable message storage (messages table)
- **PostgreSQL**: Mutable state data (aircraft, waypoints, routes, ATIS, flight state, golden annotations)

### ClickHouse Database

Container name: `acars-clickhouse`

Connection:
- Host: `localhost`
- Port: `9000`
- User: `default`
- Password: `acars`
- Database: `acars`

Query example:
```bash
docker exec -i acars-clickhouse clickhouse-client --query "SELECT * FROM acars.messages LIMIT 1"
```

Reparse example:
```bash
./acars_parser reparse -type unparsed -ch-user default -ch-password acars
```

Schema for `acars.messages`:
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

### PostgreSQL Database

Connection:
- Host: `localhost`
- Port: `5432`
- User: `acars`
- Password: `acars`
- Database: `acars`

Tables:
- `aircraft` - Aircraft registry (icao_hex, registration, type_code, operator)
- `waypoints` - Navigation waypoints (name, lat/lon, source_count)
- `routes` - Flight routes (flight_pattern, origin, dest, observation_count)
- `route_legs` - Individual route segments
- `route_aircraft` - Aircraft seen on routes
- `aircraft_callsigns` - IATA/ICAO callsign mappings
- `atis_current` - Current ATIS for airports
- `flight_state` - Ephemeral flight tracking state
- `golden_annotations` - Message annotations for parser testing
