# Parser Specification

This document defines the requirements and best practices for writing ACARS message parsers in this codebase.

## Architecture Overview

Parsers are registered with a central registry (`internal/registry`) that dispatches incoming ACARS messages based on label and content. Each parser:

1. Declares which ACARS labels it handles (or empty for content-based matching)
2. Performs a fast `QuickCheck` to filter messages before expensive processing
3. Parses the message and returns a structured result
4. Provides debug tracing via `ParseWithTrace`

A parser package registers its parsers in `init()` and must be blank-imported in `internal/parsers/parsers.go`; a package that is not imported there is never registered. See [parsers.md](parsers.md) for the dispatch tiers and the list of current parsers.

## Required Interfaces

Every parser must implement the `registry.Parser` interface:

```go
type Parser interface {
    Name() string           // Unique identifier (e.g., "label21", "pdc")
    Labels() []string       // ACARS labels handled (empty = content-based)
    QuickCheck(text string) bool  // Fast filter using strings.Contains/HasPrefix
    Priority() int          // Lower = checked first
    Parse(msg *acars.Message) Result
}
```

Every parser **must also implement** the `registry.Traceable` interface:

```go
type Traceable interface {
    ParseWithTrace(msg *acars.Message) *TraceResult
}
```

This is a hard requirement. Parsers without `ParseWithTrace` are incomplete and will fail code review.

## Grok-Style Patterns

### When to Use Grok Patterns

Use grok patterns when the message has a **fixed structure** where a single pattern can match the whole message (or its primary content):

- **Position reports**: Fixed field order (coords, altitude, speed, etc.)
- **Clearances**: Structured format (flight, origin, dest, runway, SID, squawk)
- **Status messages**: Fixed positional encoding (e.g., media advisory)

**Indicators that grok is appropriate:**
- The message format is consistent across all examples
- Fields appear in a predictable order
- One pattern (or a small set of format variants) covers all cases

### When NOT to Use Grok Patterns

Use independent field extractors when the message is **free-form** with fields that appear in variable order or quantity:

- **Weather reports**: Multiple METAR/TAF/SIGMET per message, each with optional sub-fields
- **ATIS broadcasts**: Envelope header + body with fields in any order
- **Advisory messages**: Independent fields (TYPE, ID, SEVERITY, etc.) in varying order
- **Binary protocols**: Byte-level or TLV parsing (ADS-C, CPDLC)

**Indicators that grok is NOT appropriate:**
- Fields can appear in any order
- Multiple independent records per message
- Highly variable structure with many optional fields
- Binary/encoded payloads

### Why Grok Patterns?

When applicable, grok patterns provide:

1. **Declarative**: Pattern definitions are readable and self-documenting
2. **Reusable**: Common patterns (coordinates, flight numbers, times) are defined once
3. **Debuggable**: `ParseWithTrace` shows exactly which patterns matched
4. **Testable**: Each format can be tested in isolation
5. **Consistent**: All parsers follow the same structure

### Pattern Anatomy

A grok pattern consists of:

1. **Format definitions** in `grok.go` - declare the message structures
2. **Base patterns** in `internal/patterns/base_patterns.go` - reusable components
3. **Compiler** in the parser - compiles and executes patterns

### Base Patterns

The following placeholders are available for use in format patterns:

| Placeholder | Description | Example Match |
|------------|-------------|---------------|
| `{ICAO}` | 4-letter airport code | `KJFK`, `EGLL` |
| `{IATA}` | 3-letter airport code | `JFK`, `LHR` |
| `{FLIGHT}` | Flight number | `UAL123`, `QFA5` |
| `{TIME4}` | 4-digit time (HHMM) | `1430` |
| `{TIME6}` | 6-digit time (HHMMSS) | `143022` |
| `{LAT_DIR}` | Latitude direction | `N`, `S` |
| `{LAT_5D}` | 5-digit latitude (DDMMD) | `33456` |
| `{LAT_DEC}` | Decimal latitude | `-33.456` |
| `{LON_DIR}` | Longitude direction | `E`, `W` |
| `{LON_6D}` | 6-digit longitude (DDDMMD) | `151123` |
| `{LON_DEC}` | Decimal longitude | `151.123` |
| `{FL}` | Flight level | `350`, `41` |
| `{ALT}` | Altitude in feet | `35000` |
| `{HEADING}` | 3-digit heading | `270` |
| `{SPEED}` | Ground/air speed | `450` |
| `{WAYPOINT}` | Navigation waypoint | `SHARK`, `VOR1` |
| `{SQUAWK}` | Transponder code | `1234` |
| `{RUNWAY}` | Runway designator | `27L`, `09` |
| `{FREQ}` | Radio frequency | `124.850` |
| `{AIRCRAFT}` | Aircraft type | `A320`, `B738` |
| `{SID}` | SID/STAR procedure | `BUZAD2` |

See `internal/patterns/base_patterns.go` for the complete list.

## Writing a New Parser

### Step 1: Create the Package Structure

```
internal/parsers/mynewparser/
    grok.go      # Pattern definitions
    parser.go    # Parser implementation
    parser_test.go  # Tests
```

### Step 2: Define Patterns (grok.go)

```go
package mynewparser

import "acars_parser/internal/patterns"

// Formats defines the known message formats for this parser.
var Formats = []patterns.Format{
    {
        Name: "my_format_v1",
        Pattern: `^HEADER\s+(?P<flight>{FLIGHT})\s+` +
            `(?P<lat_dir>{LAT_DIR})(?P<lat>{LAT_5D})\s*` +
            `(?P<lon_dir>{LON_DIR})(?P<lon>{LON_6D})\s+` +
            `FL(?P<fl>{FL})`,
        Fields: []string{"flight", "lat_dir", "lat", "lon_dir", "lon", "fl"},
    },
    // Add additional format variants as needed.
}
```

Key points:
- Use named capture groups: `(?P<name>pattern)`
- Reference base patterns with `{PLACEHOLDER}` syntax
- Document the `Fields` for clarity
- Add multiple formats if the message has variants

### Step 3: Implement the Parser (parser.go)

```go
package mynewparser

import (
    "strconv"
    "strings"
    "sync"

    "acars_parser/internal/acars"
    "acars_parser/internal/patterns"
    "acars_parser/internal/registry"
)

// Result represents the parsed data.
type Result struct {
    MsgID       int64   `json:"message_id"`
    Timestamp   string  `json:"timestamp"`
    Tail        string  `json:"tail,omitempty"`
    Flight      string  `json:"flight,omitempty"`
    Latitude    float64 `json:"latitude"`
    Longitude   float64 `json:"longitude"`
    FlightLevel int     `json:"flight_level,omitempty"`
}

func (r *Result) Type() string     { return "my_new_type" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Grok compiler singleton.
var (
    grokCompiler *patterns.Compiler
    grokOnce     sync.Once
    grokErr      error
)

func getCompiler() (*patterns.Compiler, error) {
    grokOnce.Do(func() {
        grokCompiler = patterns.NewCompiler(Formats, nil)
        grokErr = grokCompiler.Compile()
    })
    return grokCompiler, grokErr
}

// Parser parses the new message type.
type Parser struct{}

func init() {
    registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "mynewparser" }
func (p *Parser) Labels() []string { return []string{"XX"} }  // Replace with actual labels
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
    // Use strings.Contains or strings.HasPrefix - NO regex here.
    return strings.Contains(text, "HEADER")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
    if msg.Text == "" {
        return nil
    }

    compiler, err := getCompiler()
    if err != nil {
        return nil
    }

    match := compiler.Parse(msg.Text)
    if match == nil {
        return nil
    }

    result := &Result{
        MsgID:     int64(msg.ID),
        Timestamp: msg.Timestamp,
        Tail:      msg.Tail,
        Flight:    match.Captures["flight"],
    }

    // Parse coordinates using shared utilities.
    result.Latitude = patterns.ParseLatitude(
        match.Captures["lat"],
        match.Captures["lat_dir"],
    )
    result.Longitude = patterns.ParseLongitude(
        match.Captures["lon"],
        match.Captures["lon_dir"],
    )

    // Parse flight level.
    if fl, err := strconv.Atoi(match.Captures["fl"]); err == nil {
        result.FlightLevel = fl
    }

    return result
}
```

### Step 4: Implement ParseWithTrace (Required)

```go
// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
    trace := &registry.TraceResult{
        ParserName: p.Name(),
    }

    quickCheckPassed := p.QuickCheck(msg.Text)
    trace.QuickCheck = &registry.QuickCheck{
        Passed: quickCheckPassed,
    }

    if !quickCheckPassed {
        trace.QuickCheck.Reason = "No HEADER keyword found"
        return trace
    }

    compiler, err := getCompiler()
    if err != nil {
        trace.QuickCheck.Reason = "Failed to get compiler: " + err.Error()
        return trace
    }

    compilerTrace := compiler.ParseWithTrace(msg.Text)

    for _, ft := range compilerTrace.Formats {
        trace.Formats = append(trace.Formats, registry.FormatTrace{
            Name:     ft.Name,
            Matched:  ft.Matched,
            Pattern:  ft.Pattern,
            Captures: ft.Captures,
        })
    }

    trace.Matched = compilerTrace.Match != nil
    return trace
}
```

### Step 5: Write Tests

```go
package mynewparser

import (
    "fmt"
    "testing"

    "acars_parser/internal/acars"
)

func TestParse(t *testing.T) {
    tests := []struct {
        name    string
        text    string
        wantNil bool
        check   func(*Result) error
    }{
        {
            name: "valid message",
            text: "HEADER UAL123 N33456 W151123 FL350",
            check: func(r *Result) error {
                if r.Flight != "UAL123" {
                    return fmt.Errorf("flight = %q, want UAL123", r.Flight)
                }
                if r.FlightLevel != 350 {
                    return fmt.Errorf("fl = %d, want 350", r.FlightLevel)
                }
                return nil
            },
        },
        {
            name:    "no header keyword",
            text:    "SOMETHING ELSE",
            wantNil: true,
        },
    }

    p := &Parser{}
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            msg := &acars.Message{ID: 1, Text: tt.text}
            result := p.Parse(msg)

            if tt.wantNil {
                if result != nil {
                    t.Errorf("expected nil, got %+v", result)
                }
                return
            }

            if result == nil {
                t.Fatal("expected result, got nil")
            }

            r := result.(*Result)
            if err := tt.check(r); err != nil {
                t.Error(err)
            }
        })
    }
}
```

## ParseWithTrace Requirements

The `ParseWithTrace` method must:

1. **Always return a valid `TraceResult`** - never return nil
2. **Include QuickCheck status** - show whether the fast filter passed
3. **Report pattern match attempts** - for grok parsers, include all format traces
4. **Set `Matched` correctly** - true only if parsing would succeed

This enables the debug command to show exactly why a parser did or didn't match a message.

## Priority Guidelines

`Priority()` orders parsers within a dispatch tier; lower numbers run first. It does not stop other parsers from running: every parser whose `QuickCheck` passes and whose `Parse` returns a result contributes that result. `Dispatch` returns each result with the name of the parser that produced it (`registry.Match`), so callers do not need to rely on the order.

Priority does not move a parser between tiers. Label parsers always run before global parsers (those with an empty `Labels()`), whatever their priorities.

Use these values:

- **100** for a parser that is the only parser on its label, or the least specific parser on a shared label. Most label parsers use 100.
- **A lower number** (the current range is 10 to 70) when several parsers share a label, so that the parser with the most specific quick check and format comes first. For example, on H1: `fpn` (10), `h1pos` (20), `pwi` (30), `mdc` (40), `dispatcher` (45), `weather` (50), `trajectory` (50), `takeoff_data` (55), `hazard_alert` (60), `loadsheet` (60).
- **Parsers with equal priority on the same label are ordered by name,** so the dispatch order does not depend on package import order.

The only global parser, `pdc`, uses 500. That number orders it only against other global parsers.

The registry also has a catch-all tier (`registry.RegisterCatchAll`), which runs only when nothing else matched and does not call `QuickCheck`. No parser currently uses it.

## QuickCheck Best Practices

The `QuickCheck` method is called for every message before `Parse`. It must be:

1. **Fast** - use `strings.Contains` or `strings.HasPrefix`, never regex
2. **Conservative** - return `true` if the message *might* match
3. **Correct** - returning `false` means that the registry will not call `Parse` for that message

```go
// Good
func (p *Parser) QuickCheck(text string) bool {
    return strings.Contains(text, "LOADSHEET")
}

// Bad - uses regex
func (p *Parser) QuickCheck(text string) bool {
    return regexp.MustCompile(`LOADSHEET`).MatchString(text)  // Don't do this
}
```

## When Other Techniques Are Acceptable

In some cases, grok patterns are not suitable:

1. **Binary protocols** - use byte-level or bit-level decoding
2. **Free-form messages** - where fields appear in variable order or quantity, and no pattern can reliably match the whole message
3. **Performance-critical paths** - where the grok overhead is measurable

Even in these cases, `ParseWithTrace` must still be implemented, using `registry.Extractor` entries to report what was matched.

### Parsers That Do Not Use `patterns.Compiler`

The following parsers use techniques other than `internal/patterns.Compiler`.

**Binary decoding:**
- `adsc` - ADS-C tag-based binary encoding
- `cpdlc` - FANS-1/A ASN.1 unaligned PER, decoded with `github.com/shaneshort/go-asn/uper` after the ARINC layer (`internal/parsers/arinc`) validates the CRC
- `envelope` - hand-written regex for the ARINC envelope header, plus binary decoding of the A6 ADS-C payload

**Own format engines** (a list of named regex formats with their own compiler, separate from `internal/patterns` and its base patterns):
- `pdc` - `pdc/grok.go`, 28 formats
- `loadsheet` - `loadsheet/grok.go`, 18 formats, each with its own labels

**Tokeniser and custom section parsing:**
- `fpn` (`h1/tokeniser.go`) - flight plans split into sections
- `pwi` (`h1/parser.go`) - climb, route and descent wind sections

**Hand-written regex field extractors:**
- `atis` - envelope header and body with fields in variable order
- `weather` - multiple METAR, TAF and SIGMET reports per message
- `turbulence` - advisory with independent fields in varying order
- `landingdata` - performance data with independent fields and tabular sections
- `takeoff_data` - performance data with many fields and tabular runway sections
- `mdc` (`h1/mdc.go`) - maintenance reports with engine trend tables and fault lists
- `trajectory` (`h1/trajectory.go`) - a header line followed by repeated position records
- `parking_info` - sparse extraction from French-format messages
- `crew_list` - independent crew and schedule fields
- `delay_summary` - delay codes and timing fields
- `dispatcher` - dispatch and MEL reference fields
- `fuel_delivery` - fuel-related fields
- `hazard_alert` - header and alert fields
- `pax_bag` - flight line and zone count extraction
- `pax_conn_status` - repeated connecting flight records

The `fst` parser uses grok for its main formats and two hand-written regexes for heading and ground speed.

## Code Style

1. **Australian/British English** in comments and documentation
2. **No hyperbolic language** - be precise and factual
3. **Comprehensive comments** for complex logic
4. **JSON field names** use `snake_case`
5. **Result types** should include `message_id` and `timestamp`
6. **Use shared utilities** from `internal/patterns` for coordinate parsing

## Checklist for New Parsers

- [ ] Package created under `internal/parsers/`
- [ ] `grok.go` defines format patterns
- [ ] `parser.go` implements `registry.Parser`
- [ ] `ParseWithTrace` implemented (required)
- [ ] Registered in `init()` with `registry.Register`
- [ ] Package blank-imported in `internal/parsers/parsers.go`
- [ ] Unit tests in `parser_test.go`
- [ ] Uses base patterns from `internal/patterns`
- [ ] `QuickCheck` uses string operations only (no regex)
- [ ] Coordinates parsed with shared utilities
- [ ] JSON field names documented with struct tags
## Current Compliance

The checklist applies to new parsers. The existing parsers meet it except as follows:

- **Tests:** 20 packages have no `_test.go` files: `agfsr`, `eta`, `fst`, `gateassign`, `h2wind`, `label10`, `label16`, `label21`, `label22`, `label44`, `label4j`, `label5l`, `label80`, `label83`, `labelb2`, `labelb3`, `labelrf`, `landingdata`, `turbulence` and `weather`. In the `h1` package, the `h1pos` and `pwi` parsers have no tests; `fpn`, `mdc` and `trajectory` do.
- **Grok and base patterns:** the parsers listed under [Parsers That Do Not Use `patterns.Compiler`](#parsers-that-do-not-use-patternscompiler) have no `grok.go` built on `internal/patterns`. The `pdc` and `loadsheet` packages each have a `grok.go`, but it defines their own format engine rather than using `patterns.Compiler` and its base patterns.
- **Result fields:** the results of `crew_list`, `delay_summary`, `fuel_delivery`, `parking_info`, `pax_bag`, `pax_conn_status`, `takeoff_data`, `mdc` and `trajectory` have no `timestamp` field.
- **Tracing:** every registered parser implements `ParseWithTrace`.
