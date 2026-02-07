# ACARS Parser Reference

This document provides a comprehensive overview of all message parsers in the system, their labels, pattern types, and what data they extract.

## Parser Architecture

Parsers implement the `registry.Parser` interface:

```go
type Parser interface {
    Name() string           // Unique identifier
    Labels() []string       // ACARS labels handled (empty = content-based)
    QuickCheck(text string) bool  // Fast string check before regex
    Priority() int          // Lower = checked first
    Parse(msg *acars.Message) Result
}
```

**Pattern Types:**
- **Grok-style** (`patterns.Compiler`): Uses `{PLACEHOLDER}` syntax for reusable patterns
- **Individual regex**: Uses standalone `regexp.MustCompile()` patterns
- **Custom**: Binary parsing, tokenisation, or other specialised approaches

**Traceable Interface:**
Parsers can optionally implement `registry.Traceable` to support the `debug` command:

```go
type Traceable interface {
    ParseWithTrace(msg *acars.Message) *TraceResult
}
```

---

## Parser Summary

| Parser | Label(s) | Pattern Type | Traceable |
|--------|----------|--------------|-----------|
| [pdc](#pdc) | All (content-based) | Grok (strict) | ✓ |
| [sq](#sq) | SQ | Grok | ✓ |
| [atis](#atis) | A9 | Individual regex | ✓ |
| [fpn](#fpn) | H1, 4A, HX | Tokeniser + grok | |
| [h1pos](#h1pos) | H1 | Grok | |
| [pwi](#pwi) | H1 | Custom parsing | |
| [weather](#weather) | RA, C1, 21, H1, 3W, 27, 31, 34, 3T, 23 | Individual regex | |
| [labelb3](#labelb3) | B3 | Grok | |
| [label5l](#label5l) | 5L | Grok | |
| [labelb2](#labelb2) | B2 | Grok | |
| [fst](#fst) | 15 | Grok + individual regex | |
| [label10](#label10) | 10 | Grok | |
| [eta](#eta) | 5Z | Grok | |
| [label4j](#label4j) | 4J | Grok + individual regex | |
| [label21](#label21) | 21 | Grok | |
| [label80](#label80) | 80 | Grok | |
| [turbulence](#turbulence) | C1 | Individual regex | |
| [landingdata](#landingdata) | C1 | Individual regex | |
| [gateassign](#gateassign) | RA | Individual regex | |
| [h2wind](#h2wind) | H2 | Grok | |
| [label44](#label44) | 44 | Grok | |
| [label83](#label83) | 83 | Grok | |
| [agfsr](#agfsr) | 4T | Grok | |
| [label22](#label22) | 22 | Grok | |
| [envelope](#envelope) | AA, A6 | Individual regex + binary TLV | |
| [mediaadv](#mediaadv) | SA | Individual regex + state parsing | |
| [label16](#label16) | 16 | Grok | |
| [adsc](#adsc) | B6 | Individual regex + binary tags | |
| [labelrf](#labelrf) | RF | Individual regex | |
| [cpdlc](#cpdlc) | AA, BA | ARINC layer + custom decoder | |
| [hazard](#hazard) | _, H1, SA | Individual regex | |
| [takeoff](#takeoff) | RA, H1, C1 | Individual regex | |
| [paxbag](#paxbag) | RA | Individual regex | |
| [dispatch](#dispatch) | RA, 25, H1 | Individual regex | |
| [crew](#crew) | RA | Individual regex | |
| [delay](#delay) | 3E, RA | Individual regex | |
| [parking](#parking) | 1E, RA | Individual regex | |
| [paxconn](#paxconn) | 3E, RA | Individual regex | |
| [fuel](#fuel) | 3E, RA | Individual regex | |
| [loadsheet](#loadsheet) | Multiple | Custom format matcher | |

---

## Parser Details

### pdc

**Package:** `internal/parsers/pdc`

**Labels:** Content-based (checks all messages)

**Priority:** 500 (runs after label-specific parsers)

**Pattern Type:** Grok-only (strict mode - no fallback extraction)

**Description:** Parses Pre-Departure Clearance messages from various airlines and airports worldwide. Uses strict grok pattern matching for accuracy over quantity.

**Extracted Fields:**
- Flight number, origin, destination
- Runway, SID (Standard Instrument Departure)
- Squawk code, departure frequency
- Aircraft type, ATIS letter
- Route waypoints, initial altitude

**Supported Formats:**
- Australian domestic (Jetstar, Qantas, Virgin)
- US carriers (Delta, American, Southwest, SkyWest, etc.)
- Canadian (WestJet, NAV Canada, Jazz)
- European DC1 clearance (Frankfurt, Heathrow, etc.)
- Private/corporate jets
- UPS/cargo

---

### sq

**Package:** `internal/parsers/sq`

**Labels:** SQ

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses SQ (Squitter) ARINC position messages containing airport IATA/ICAO mapping and position data.

**Extracted Fields:**
- IATA code, ICAO code
- Latitude, longitude
- Frequency (MHz), frequency band
- Message type (A or S)

**Formats:**
- `arinc_position`: Standard ARINC squitter
- `avicom_frequency`: Japanese AVICOM format

---

### atis

**Package:** `internal/parsers/atis`

**Labels:** A9

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses ATIS (Automatic Terminal Information Service) broadcast messages containing weather, runway, and approach information.

**Extracted Fields:**
- Airport, ATIS letter, ATIS type (ARR/DEP)
- Runways, approaches (ILS, RNAV, etc.)
- Wind, visibility, clouds
- Temperature, dew point, QNH
- Remarks (cautions, closures, etc.)

---

### fpn

**Package:** `internal/parsers/h1`

**Labels:** H1, 4A, HX

**Priority:** 10

**Pattern Type:** Tokeniser-based with grok support

**Description:** Parses FPN (Flight Plan) messages containing route information, SIDs, STARs, and approach procedures.

**Extracted Fields:**
- Origin, destination
- Flight number
- Route waypoints with coordinates
- Departure (SID) and transition
- Arrival (STAR) and transition
- Approach type and runway
- Truncation detection (CRC verification)

---

### h1pos

**Package:** `internal/parsers/h1`

**Labels:** H1

**Priority:** 20

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses H1 POS position messages with detailed flight data.

**Extracted Fields:**
- Latitude, longitude
- Flight level, ground speed
- Current/next/third waypoint
- ETA, temperature
- Wind direction and speed

---

### pwi

**Package:** `internal/parsers/h1`

**Labels:** H1

**Priority:** 30

**Pattern Type:** Custom section parsing

**Description:** Parses PWI (Predicted Wind Information) messages containing wind data along the route.

**Extracted Fields:**
- Climb winds by altitude
- Descent winds by altitude
- Route winds by waypoint and flight level
- Temperature at waypoints

---

### weather

**Package:** `internal/parsers/weather`

**Labels:** RA, C1, 21, H1, 3W, 27, 31, 34, 3T, 23

**Priority:** 50

**Pattern Type:** Individual regex

**Description:** Parses METAR, TAF, and SIGMET weather reports.

**Extracted Fields:**
- METAR: Airport, time, wind (direction/speed/gust), visibility, clouds, temperature, dew point, QNH
- TAF: Airport, issued time, validity period
- SIGMET: ID, validity, originator, FIR, phenomenon, altitude, movement

---

### labelb3

**Package:** `internal/parsers/labelb3`

**Labels:** B3

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses B3 gate information messages.

**Extracted Fields:**
- Flight number, origin, destination
- Gate, ATIS letter
- Aircraft type

---

### label5l

**Package:** `internal/parsers/label5l`

**Labels:** 5L

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses 5L route messages with ICAO/IATA codes and times.

**Extracted Fields:**
- Origin, destination (ICAO/IATA)
- Departure time, arrival time
- Flight number

---

### labelb2

**Package:** `internal/parsers/labelb2`

**Labels:** B2

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses B2 oceanic clearance messages.

**Extracted Fields:**
- Flight number, destination
- Oceanic fixes (lat/lon format)
- Flight levels, Mach number

---

### fst

**Package:** `internal/parsers/fst`

**Labels:** 15

**Priority:** 100

**Pattern Type:** Grok + individual regex

**Description:** Parses FST (Flight Status) messages.

**Extracted Fields:**
- Position (latitude/longitude)
- Temperature
- Flight status indicators

---

### label10

**Package:** `internal/parsers/label10`

**Labels:** 10

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses Label 10 rich position reports.

**Extracted Fields:**
- Position (latitude/longitude)
- Mach number, heading
- Flight level, ground speed
- Waypoints, ETA

---

### eta

**Package:** `internal/parsers/eta`

**Labels:** 5Z

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses ETA and timing messages in various formats.

**Formats:** ET, IR, B6, OS, C3

**Extracted Fields:**
- Estimated times
- Flight identifiers
- Route information

---

### label4j

**Package:** `internal/parsers/label4j`

**Labels:** 4J

**Priority:** 100

**Pattern Type:** Grok + individual regex

**Description:** Parses 4J position and weather messages.

**Extracted Fields:**
- Position (latitude/longitude)
- Fuel burn data

---

### label21

**Package:** `internal/parsers/label21`

**Labels:** 21

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses Label 21 POSN position reports.

**Extracted Fields:**
- Position (latitude/longitude)
- Heading, altitude
- Fuel, wind, temperature

---

### label80

**Package:** `internal/parsers/label80`

**Labels:** 80

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses Label 80 position messages with header format.

**Extracted Fields:**
- Position data
- Altitude, Mach, TAS
- Fuel on board
- OUT/OFF/ON/IN times

---

### turbulence

**Package:** `internal/parsers/turbulence`

**Labels:** C1

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses turbulence advisories and SIGMET alerts.

**Extracted Fields:**
- Turbulence severity
- Altitude range
- Movement direction

---

### landingdata

**Package:** `internal/parsers/landingdata`

**Labels:** C1

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses landing performance data.

**Extracted Fields:**
- Airport, runway
- Flap settings
- Weight limits, conditions

---

### gateassign

**Package:** `internal/parsers/gateassign`

**Labels:** RA

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses gate assignment messages.

**Extracted Fields:**
- Gate number
- PPOS (parking position)
- Baggage belt
- Next flight info

---

### h2wind

**Package:** `internal/parsers/h2wind`

**Labels:** H2

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses H2 wind and weather messages.

**Extracted Fields:**
- Wind layers at various altitudes
- Header information

---

### label44

**Package:** `internal/parsers/label44`

**Labels:** 44

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses Label 44 runway and position reports.

**Extracted Fields:**
- Runway information
- FB position data
- POS reports

---

### label83

**Package:** `internal/parsers/label83`

**Labels:** 83

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses Label 83 position reports.

**Formats:** PR, ZSPD

**Extracted Fields:**
- Position (latitude/longitude)
- Altitude, heading
- Ground speed

---

### agfsr

**Package:** `internal/parsers/agfsr`

**Labels:** 4T

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses AGFSR (Automated Ground-to-Flight Status Report) messages.

**Extracted Fields:**
- Route information
- Position, fuel
- Wind, heading, ground speed

---

### label22

**Package:** `internal/parsers/label22`

**Labels:** 22

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses Label 22 detailed position reports in DMS format.

**Extracted Fields:**
- Position (latitude/longitude in DMS)
- Altitude, Mach
- Flight level, ground speed, track

---

### envelope

**Package:** `internal/parsers/envelope`

**Labels:** AA, A6

**Priority:** 100

**Pattern Type:** Individual regex + binary TLV parsing

**Description:** Parses envelope messages containing aircraft registration and ADS-C data from binary payloads.

**Extracted Fields:**
- Aircraft registration
- ADS-C altitude and position

---

### mediaadv

**Package:** `internal/parsers/mediaadv`

**Labels:** SA

**Priority:** 100

**Pattern Type:** Individual regex + state parsing

**Description:** Parses Media Advisory messages indicating data link status.

**Extracted Fields:**
- Link status (VHF, SATCOM, HF, VDL2, etc.)
- Connection state

---

### label16

**Package:** `internal/parsers/label16`

**Labels:** 16

**Priority:** 100

**Pattern Type:** Grok (`patterns.Compiler`)

**Description:** Parses Label 16 waypoint position messages.

**Formats:** CSV, AUTPOS

**Extracted Fields:**
- Waypoint name
- Position (latitude/longitude)
- Flight identifier, track

---

### adsc

**Package:** `internal/parsers/adsc`

**Labels:** B6

**Priority:** 100

**Pattern Type:** Individual regex + binary tag parsing

**Description:** Parses ADS-C (Automatic Dependent Surveillance - Contract) messages.

**Extracted Fields:**
- Position, altitude
- Meteorological data
- Earth/air reference
- Predicted route

---

### labelrf

**Package:** `internal/parsers/labelrf`

**Labels:** RF

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses RF flight subscription messages.

**Formats:** FDASUB, FDACOM

**Extracted Fields:**
- Flight information
- IATA/ICAO conversion

---

### cpdlc

**Package:** `internal/parsers/cpdlc`

**Labels:** AA, BA

**Priority:** 100

**Pattern Type:** ARINC layer + custom decoder

**Description:** Parses CPDLC (Controller-Pilot Data Link Communications) messages.

**IMI Markers:** AT1, CR1, CC1, DR1

**Extracted Fields:**
- Message type and content
- Connection requests
- Clearances and instructions

---

### hazard

**Package:** `internal/parsers/hazard`

**Labels:** _ (blank), H1, SA

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses HAZARD ALERT messages for turbulence and wind warnings.

**Extracted Fields:**
- EDR (Eddy Dissipation Rate) turbulence
- Wind warnings
- Segment information

---

### takeoff

**Package:** `internal/parsers/takeoff`

**Labels:** RA, H1, C1

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses takeoff performance data.

**Extracted Fields:**
- Runway
- V-speeds (V1, VR, V2)
- Gross takeoff weight
- Fuel, weight limits

---

### paxbag

**Package:** `internal/parsers/paxbag`

**Labels:** RA

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses passenger and baggage details.

**Extracted Fields:**
- Passenger counts by zone
- Bag counts and weights
- Finalisation status

---

### dispatch

**Package:** `internal/parsers/dispatch`

**Labels:** RA, 25, H1

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses dispatcher messages.

**Extracted Fields:**
- MEL (Minimum Equipment List) references
- SIGMET alerts
- Fuel information
- MDDR references

---

### crew

**Package:** `internal/parsers/crew`

**Labels:** RA

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses crew list messages.

**Extracted Fields:**
- Cockpit crew (positions, names, IDs)
- Cabin crew (positions, names, IDs)
- Minimum crew requirements

---

### delay

**Package:** `internal/parsers/delay`

**Labels:** 3E, RA

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses delay summary messages.

**Extracted Fields:**
- Departure delay
- Arrival delay
- IATA delay codes

---

### parking

**Package:** `internal/parsers/parking`

**Labels:** 1E, RA

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses parking and gate information (includes French ESCALE format).

**Extracted Fields:**
- Stand/parking position
- Carousel information
- Gate details

---

### paxconn

**Package:** `internal/parsers/paxconn`

**Labels:** 3E, RA

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses passenger connection status messages.

**Extracted Fields:**
- Missed connections
- Pending connections
- Wait decisions
- Passenger counts

---

### fuel

**Package:** `internal/parsers/fuel`

**Labels:** 3E, RA

**Priority:** 100

**Pattern Type:** Individual regex

**Description:** Parses fuel delivery receipt messages.

**Extracted Fields:**
- Fuel truck ID
- Fuel company
- Fuel grade
- Density
- Delivery timestamps

---

### loadsheet

**Package:** `internal/parsers/loadsheet`

**Labels:** Multiple (format-dependent)

**Priority:** 100

**Pattern Type:** Custom format matcher

**Description:** Parses loadsheet data for weight and balance.

**Extracted Fields:**
- Zero Fuel Weight (ZFW)
- Takeoff Weight (TOW)
- Landing Weight (LAW)
- Fuel load
- Crew count
- MAC (Mean Aerodynamic Chord) percentage

---

## Adding Trace Support

To add debug tracing to a parser, implement the `Traceable` interface:

```go
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
    trace := &registry.TraceResult{
        ParserName: p.Name(),
    }

    // 1. Check QuickCheck
    quickCheckPassed := p.QuickCheck(msg.Text)
    trace.QuickCheck = &registry.QuickCheck{
        Passed: quickCheckPassed,
        Reason: "Reason if failed",
    }

    if !quickCheckPassed {
        return trace
    }

    // 2. For grok-based parsers, use compiler.ParseWithTrace()
    // 3. For regex-based parsers, add each pattern as an Extractor

    trace.Matched = // whether parser matched overall
    return trace
}
```

Use the `debug` command to test:

```bash
./acars_parser debug -id 12345
./acars_parser debug -id 12345 -type pdc -all
./acars_parser debug -text "YOUR MESSAGE" -label H1
```