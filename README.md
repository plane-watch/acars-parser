# ACARS Parser

A Go tool for parsing ACARS (Aircraft Communications Addressing and Reporting System) messages. It extracts structured flight data from various message types including Pre-Departure Clearances, flight plans, position reports, and wind forecasts.

## Installation

```bash
go build -o acars_parser ./cmd/acars_parser
```

## Project Structure

```
acars_parser/
├── cmd/acars_parser/          # Command-line entry point
│   ├── main.go
│   ├── extract.go          # Extract command
│   └── live.go             # Live NATS command
├── internal/
│   ├── acars/              # ACARS message types
│   ├── registry/           # Parser registry
│   ├── patterns/           # Shared regex patterns and extractors
│   └── parsers/            # Individual parser implementations
│       ├── adsc/           # ADS-C (B6)
│       ├── agfsr/          # AGFSR flight status (4T)
│       ├── cpdlc/          # CPDLC FANS-1/A (AA)
│       ├── eta/            # ETA/timing (5Z)
│       ├── fst/            # FST reports (15)
│       ├── h1/             # H1 FPN/POS/PWI
│       ├── h2wind/         # Wind data (H2)
│       ├── label10/        # Rich position (10)
│       ├── label16/        # Waypoint position (16)
│       ├── label21/        # Position reports (21)
│       ├── label22/        # Detailed position (22)
│       ├── label44/        # Runway info (44)
│       ├── label4j/        # Position+weather (4J)
│       ├── label5l/        # Routes (5L)
│       ├── label80/        # Position (80)
│       ├── label83/        # Position reports (83)
│       ├── labelb2/        # Oceanic clearances (B2)
│       ├── labelb3/        # Gate info (B3)
│       ├── pdc/            # Pre-departure clearances
│       └── sq/             # ARINC position (SQ)
└── README.md
```

## Commands

### extract

Extracts structured data from JSONL files containing ACARS messages.

```bash
./acars_parser extract -input messages.jsonl [-output output.json] [-pretty] [-all]
```

**Options:**
- `-input FILE` - Input JSONL file (default: stdin)
- `-output FILE` - Output JSON file (default: stdout)
- `-pretty` - Pretty print JSON output
- `-all` - Include all parsed data types

### live

Connects to a live NATS feed and displays parsed messages in real-time.

```bash
./acars_parser live -creds credentials.creds [options]
```

**Options:**
- `-creds FILE` - Path to NATS credentials file (required)
- `-server URL` - NATS server URL (default: `nats://157.90.242.138:4222`)
- `-subject SUBJ` - NATS subject to subscribe to (default: `v1.aircraft.ingest.*.message.*.created`)
- `-output FILE` - Optional JSONL output file
- `-db FILE` - SQLite database for message storage (default: `messages.db`)
- `-state FILE` - SQLite database for flight state tracking (default: `state.db`)
- `-no-store` - Disable all database storage
- `-all` - Show all messages with text, not just parsed ones
- `-raw` - Show raw message text
- `-empty` - Show empty/missing fields to identify unparsed data
- `-exclude TYPES` - Exclude result types from display (default: `sq_position`). Use `-exclude ""` to show all.
- `-debug LABELS` - Debug specific labels (comma-separated, e.g. `80,B6,H1`)
- `-v` - Verbose output

### query

Query stored messages in SQLite database.

```bash
./acars_parser query [options]
```

**Options:**
- `-db FILE` - SQLite database file (default: `messages.db`)
- `-id N` - Fetch a specific message by database row ID
- `-msg-id N` - Fetch by ACARS message ID (from parsed JSON)
- `-type TYPE` - Filter by parser type (e.g. `h1_position`, `pdc`)
- `-label LABEL` - Filter by ACARS label (e.g. `H1`, `16`)
- `-flight TEXT` - Filter by flight number (partial match)
- `-missing FIELD` - Filter by specific missing field
- `-has-missing` - Only show messages with any missing fields
- `-search TEXT` - Full-text search on raw message text
- `-limit N` - Max results to return (default: 20)
- `-offset N` - Pagination offset
- `-order FIELD` - Sort by field: id, timestamp, parser_type, confidence (default: `id`)
- `-desc` - Sort descending (default: true)
- `-raw` - Show raw message text
- `-json` - Output as JSON
- `-stats` - Show database statistics only
- `-list-types` - List all parser types in the database
- `-list-missing` - List top missing fields across all messages

### reparse

Re-parse stored messages to compare old vs new parsing results.

```bash
./acars_parser reparse [options]
```

**Options:**
- `-db FILE` - SQLite database file (default: `messages.db`)
- `-type TYPE` - Filter by parser type
- `-label LABEL` - Filter by ACARS label
- `-v` - Verbose output: show detailed diffs
- `-regressions-only` - Show only messages that regressed
- `-improvements-only` - Show only messages that improved
- `-limit N` - Limit number of messages to process (0 = all)
- `-json` - Output as JSON
- `-update` - Update database with new parsed results

### debug

Debug why a message didn't parse correctly.

```bash
./acars_parser debug -id N [options]
./acars_parser debug -text "MESSAGE TEXT" [-label LABEL] [options]
```

**Options:**
- `-db FILE` - SQLite database file (default: `messages.db`)
- `-id N` - Message ID to debug
- `-text TEXT` - Raw message text to debug (instead of -id)
- `-label LABEL` - ACARS label for raw text (e.g. `H1`, `16`)
- `-all` - Show all pattern attempts, not just matches
- `-type TYPE` - Only show trace for specific parser type (e.g. `pdc`)

### backfill

Populate state tracker from existing parsed messages.

```bash
./acars_parser backfill [options]
```

**Options:**
- `-db FILE` - SQLite database with parsed messages (default: `messages.db`)
- `-state FILE` - SQLite database for flight state (default: `state.db`)
- `-type TYPE` - Filter by parser type
- `-limit N` - Limit number of messages (0 = all)
- `-v` - Verbose output

### review

Launch web UI for reviewing and annotating messages.

```bash
./acars_parser review [options]
```

**Options:**
- `-db FILE` - SQLite database file (default: `messages.db`)
- `-port N` - HTTP port (default: 8080)
- `-type TYPE` - Pre-filter to specific parser type

### templates

Discover message format templates by normalising messages.

```bash
./acars_parser templates [options]
```

**Options:**
- `-db FILE` - SQLite database file (default: `messages.db`)
- `-type TYPE` - Filter by parser type
- `-label LABEL` - Filter by ACARS label
- `-limit N` - Limit number of messages (0 = all)
- `-min N` - Minimum messages per template to show (default: 2)
- `-examples N` - Number of example messages per template (default: 1)
- `-v` - Verbose output: show full template strings

## Supported Message Types

### PDC (Pre-Departure Clearance)
Extracts flight number, origin/destination, runway, SID, squawk code, and frequencies from pre-departure clearances.

### Route (5L)
Parses route messages containing callsign, origin/destination airports (IATA/ICAO), and scheduling data.

### Position (80)
Extracts current position (lat/lon), altitude, ground speed, and flight routing.

### ADS-C (B6)
Parses ADS-C (Automatic Dependent Surveillance - Contract) position reports using tag-based binary parsing based on libacars. Extracts:
- **Position data**: latitude, longitude, altitude, report timestamp, position accuracy (0-7)
- **Meteorological data** (tag 16): wind speed, wind direction, temperature
- **Earth reference** (tag 14): true track, ground speed, vertical speed
- **Air reference** (tag 15): true heading, mach number, vertical speed
- **Predicted route** (tag 13): next waypoint lat/lon/alt/ETA, next+1 waypoint coordinates
- **Flight ID** (tag 12): ISO5-encoded flight identifier
- **Airframe ID** (tag 17): ICAO hex address

### Flight Plan (H1 FPN)
Extracts flight plan data including waypoints, origin/destination, and route information.

### H1 Position (H1 POS)
Parses H1 position reports with current/next waypoint, altitude, and coordinates.

### PWI - Predicted Wind Information (H1)
Extracts wind and temperature forecasts along the route:
- **Climb winds (CB)**: Wind direction/speed at various altitudes during climb
- **Route winds (WD)**: Wind direction/speed/temperature at waypoints for each flight level
- **Descent winds (DD)**: Wind direction/speed at various altitudes during descent

Example PWI data structure:
```json
{
  "climb_winds": [
    {"flight_level": 100, "wind_dir": 252, "wind_speed": 39},
    {"flight_level": 310, "wind_dir": 261, "wind_speed": 84}
  ],
  "route_winds": [
    {
      "flight_level": 360,
      "waypoints": [
        {"waypoint": "DOLEV", "wind_dir": 321, "wind_speed": 74, "temperature": -57},
        {"waypoint": "ROTAR", "wind_dir": 303, "wind_speed": 85, "temperature": -63}
      ]
    }
  ],
  "descent_winds": [
    {"flight_level": 100, "wind_dir": 305, "wind_speed": 22},
    {"flight_level": 350, "wind_dir": 300, "wind_speed": 76}
  ]
}
```

### Waypoint Position (16)
Extracts waypoint crossing reports with position and timing.

### Position Report (21)
Parses position reports with coordinates, altitude, and destination.

### Oceanic Clearance (B2)
Extracts oceanic clearance data including track, flight level, and Mach number.

### Gate Info (B3)
Parses gate information messages with flight number and gate assignment.

### Position + Weather (4J)
Extracts combined position and weather data.

### SQ - ARINC Position (96k messages)
Parses squitter messages containing airport IATA/ICAO mapping and position data.
```
02XAORDKORD54158N08754WV136975/ARINC
```

### Label 10 - Rich Position/Route (10k messages)
Parses position reports with full route picture including waypoint timing.
```
/N40.024/W073.100/10/0.72/230/430/KISM/2057/0064/00015/ZIZZI/TBONN/1831/
```

### Label 4T - AGFSR Flight Status (2.6k messages)
Parses comprehensive flight status messages with route, position, fuel, wind, and ETA.
```
AGFSR AC1204/29/29/YULMIA/1829Z/110/3457.3N07711.0W/300/CRUISE/0067/0052/M37/248095/0300/202/02/1432/1640/
```

### Label 22 - Detailed Position (13k messages)
Parses detailed position reports in degrees/minutes/seconds format.
```
N 325338W 971058,-------,182836,9977, ,      , ,M  3,31104  41,  64,
```

### Label 5Z - ETA/Timing (21k messages)
Parses ETA and timing messages in various formats (ET, IR, B6, OS, C3).
```
/ET EXP TIME       / KSNA KIAH 29 182901/EON 1908 AUTO
```

### Label 15 - FST Reports (14k messages)
Parses flight status reports with route, position, and temperature.
```
FST01EGLCEIDWN51420W00049317803270072M020C014331258256370
```

### Label 83 - Position Reports (3.6k messages)
Parses PR and ZSPD position report formats.
```
001PR29182854N5106.0W11400.4035000----
```

### H2 - Wind Data
Parses wind/weather data with multiple altitude layers.
```
02A291829EDDKLSZHN50529E007101291809   6M005   48P002290008G
```

### Label 44 - Runway/Procedure Info (3k messages)
Parses runway takeoff information, FB positions, and POS reports.
```
KLGA T/O RWYS,04                  7002
```

### ATIS (A9)
Parses ATIS (Automatic Terminal Information Service) weather reports with runway, wind, visibility, and QNH data.

### Envelope (AA, A6)
Parses envelope-formatted messages containing aircraft position and status data.

### Gate Assignment (RA)
Parses gate assignment messages with flight and gate information.

### Landing Data (C1)
Parses landing performance data including runway, approach, and configuration.

### Loadsheet (C1)
Parses aircraft loadsheet messages with weight and balance information.

### Turbulence (C1)
Parses turbulence reports with severity and location data.

### Weather (RA, C1)
Parses general weather observation messages with temperature, wind, and conditions.

### Media Advisory (SA)
Parses data link status messages reporting which communication links (VHF, SATCOM, HF, VDL2, etc) are available or unavailable. Based on libacars media-adv format.
```
0EV095905V
```
Extracts: link status (established/lost), current link type, timestamp, available links.

### CPDLC - Controller-Pilot Data Link Communications (AA)
Parses FANS-1/A CPDLC messages using pure Go ASN.1 PER decoding (no libacars dependency). Supports:
- **Downlink messages** (dM0-dM80): Pilot responses/requests to ATC
- **Uplink messages** (uM0-uM182): ATC instructions/requests to aircraft
- **Connection management**: Connect requests (CR1), connect confirms (CC1), disconnect (DR1)

Message format:
```
/BOMCAYA.AT1.A4O-SI005080204A
```
Structure: `/<station>.<type>.<registration><hex_data>`

**Decoded element types include:**
- Altitudes (flight level, feet, metres, QNH/QFE/GNSS)
- Speeds (knots, Mach, km/h)
- Positions (fix, navaid, airport, lat/lon, place-bearing-distance)
- Route clearances (departure/arrival airports, runways, SIDs/STARs, airways)
- Frequencies (VHF, UHF, HF, SATCOM)
- Free text messages
- Error information
- Vertical rates, beacon codes, ATIS codes, and more

Example decoded output:
```json
{
  "message_type": "cpdlc",
  "direction": "downlink",
  "header": {"msg_id": 0},
  "elements": [{
    "id": 80,
    "label": "DEVIATING [distanceoffset] [direction] OF ROUTE",
    "text": "DEVIATING 1 km south OF ROUTE"
  }]
}
```

**Limitations:**
- Multi-element messages (containing 2-5 elements) currently only decode the primary element
- Some complex route information types (placeBearingPlaceBearing, trackDetail, holdAtWaypoint) return placeholder text

## Output Format

All extract commands output JSON with a `stats` object summarising the parsing results:

```json
{
  "stats": {
    "total_messages": 794302,
    "parsed_pdcs": 1234,
    "parsed_pwi": 2706,
    ...
  },
  "pwi_reports": [...],
  "pdcs": [...]
}
```

The live command outputs human-readable summaries:
```
[UAL123 N12345 737-800] [PWI] CB:FL100-350 WD:FL360 (3 wpts) DD:FL100-390
[DAL456 N67890] [PDC] DAL456 KJFK->KLAX RWY 31L SID DEEZZ5 SQK 1234
```

---

## Developer Guide

### Application Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│  cmd/acars_parser/main.go                                               │
│  - Entry point, imports internal/parsers for side-effect registration  │
│  - Calls registry.Default().Sort() to prepare parsers                  │
│  - Routes to extract.go or live.go based on subcommand                 │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
    ┌──────────────────────────┐    ┌──────────────────────────┐
    │  cmd/.../extract.go      │    │  cmd/.../live.go         │
    │  - Reads JSONL files     │    │  - Connects to NATS      │
    │  - Batch processing      │    │  - Real-time streaming   │
    │  - JSON output           │    │  - Console output        │
    └──────────────────────────┘    └──────────────────────────┘
                    │                               │
                    └───────────────┬───────────────┘
                                    ▼
    ┌─────────────────────────────────────────────────────────────────────┐
    │  internal/registry/registry.go                                      │
    │  - Dispatch(msg) routes messages to matching parsers                │
    │  - Parsers registered via init() in each parser package            │
    │  - Label-based routing (fast) + global parsers (content-based)     │
    └─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
    ┌─────────────────────────────────────────────────────────────────────┐
    │  internal/parsers/*/parser.go                                       │
    │  - Each parser implements: Name(), Labels(), QuickCheck(), Parse() │
    │  - Returns a Result struct with Type() and MessageID()             │
    └─────────────────────────────────────────────────────────────────────┘
```

### Key Files

| File | Purpose |
|------|---------|
| `cmd/acars_parser/main.go` | Entry point, subcommand routing |
| `cmd/acars_parser/extract.go` | Batch extraction from JSONL files |
| `cmd/acars_parser/live.go` | Real-time NATS streaming, console output |
| `internal/acars/message.go` | ACARS message types (`Message`, `NATSWrapper`, `Airframe`, `Flight`) |
| `internal/registry/registry.go` | Parser registry, `Dispatch()` routing logic |
| `internal/parsers/parsers.go` | Blank import to trigger all parser `init()` registrations |
| `internal/patterns/patterns.go` | Shared regex patterns (coordinates, flight numbers, etc.) |
| `internal/patterns/extractors.go` | Shared extraction functions |

### Parser Locations

Each parser lives in `internal/parsers/<name>/parser.go`:

| Parser | Label(s) | Result Type | File |
|--------|----------|-------------|------|
| ADS-C | `B6` | `adsc` | `internal/parsers/adsc/parser.go` |
| AGFSR | `4T` | `agfsr` | `internal/parsers/agfsr/parser.go` |
| ATIS | `A9` | `atis` | `internal/parsers/atis/parser.go` |
| CPDLC | `AA` | `cpdlc`, `connect_request`, `connect_confirm`, `disconnect` | `internal/parsers/cpdlc/parser.go` |
| Envelope | `AA`, `A6` | `envelope` | `internal/parsers/envelope/parser.go` |
| ETA | `5Z` | `eta` | `internal/parsers/eta/parser.go` |
| FST | `15` | `fst` | `internal/parsers/fst/parser.go` |
| Gate Assignment | `RA` | `gate_assignment` | `internal/parsers/gateassign/parser.go` |
| H1 FPN | `H1`, `4A`, `HX` | `flight_plan` | `internal/parsers/h1/parser.go` |
| H1 POS | `H1` | `h1_position` | `internal/parsers/h1/parser.go` |
| H1 PWI | `H1` | `pwi` | `internal/parsers/h1/parser.go` |
| H2 Wind | `H2` | `h2_wind` | `internal/parsers/h2wind/parser.go` |
| Label 10 | `10` | `label10_position` | `internal/parsers/label10/parser.go` |
| Label 16 | `16` | `waypoint_position` | `internal/parsers/label16/parser.go` |
| Label 21 | `21` | `position_report` | `internal/parsers/label21/parser.go` |
| Label 22 | `22` | `label22_position` | `internal/parsers/label22/parser.go` |
| Label 44 | `44` | `label44` | `internal/parsers/label44/parser.go` |
| Label 4J | `4J` | `pos_weather` | `internal/parsers/label4j/parser.go` |
| Label 5L | `5L` | `route` | `internal/parsers/label5l/parser.go` |
| Label 80 | `80` | `position` | `internal/parsers/label80/parser.go` |
| Label 83 | `83` | `label83_position` | `internal/parsers/label83/parser.go` |
| Label B2 | `B2` | `oceanic_clearance` | `internal/parsers/labelb2/parser.go` |
| Label B3 | `B3` | `gate_info` | `internal/parsers/labelb3/parser.go` |
| Landing Data | `C1` | `landing_data` | `internal/parsers/landingdata/parser.go` |
| Loadsheet | `C1` | `loadsheet` | `internal/parsers/loadsheet/parser.go` |
| Media Advisory | `SA` | `media_advisory` | `internal/parsers/mediaadv/parser.go` |
| PDC | *(content-based)* | `pdc` | `internal/parsers/pdc/parser.go` |
| SQ | `SQ` | `sq_position` | `internal/parsers/sq/parser.go` |
| Turbulence | `C1` | `turbulence` | `internal/parsers/turbulence/parser.go` |
| Weather | `RA`, `C1` | `weather` | `internal/parsers/weather/parser.go` |

### Adding a New Parser

1. Create directory: `internal/parsers/<name>/`
2. Create `parser.go` implementing the `registry.Parser` interface:

```go
package myparser

import (
    "acars_parser/internal/acars"
    "acars_parser/internal/registry"
)

type Result struct {
    MsgID     int64  `json:"message_id"`
    Timestamp string `json:"timestamp"`
    // ... your fields
}

func (r *Result) Type() string     { return "my_type" }
func (r *Result) MessageID() int64 { return r.MsgID }

type Parser struct{}

func init() {
    registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "myparser" }
func (p *Parser) Labels() []string { return []string{"XX"} } // or empty for content-based
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
    return strings.Contains(text, "MYPREFIX") // fast string check, no regex
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
    // Parse logic here
    // Return nil if message doesn't match
    return &Result{...}
}
```

3. Add import to `internal/parsers/parsers.go`:
```go
_ "acars_parser/internal/parsers/myparser"
```

### Parser Interface

```go
type Parser interface {
    Name() string           // Unique identifier
    Labels() []string       // ACARS labels to match (empty = content-based, checks all)
    QuickCheck(text string) bool  // Fast pre-filter (use strings.Contains, not regex)
    Priority() int          // Lower = checked first
    Parse(msg *acars.Message) Result  // Returns nil if not applicable
}
```

### Registry Dispatch Order

1. **Label-specific parsers** - Matched by `msg.Label`, sorted by priority
2. **Global parsers** - Content-based parsers (empty `Labels()`), check all messages
3. **Catch-all parsers** - Only run if nothing else matched

Multiple parsers can return results for the same message.
