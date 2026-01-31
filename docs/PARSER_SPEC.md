# Parser Specification

This document defines the requirements and best practices for writing ACARS message parsers in this codebase.

## Architecture Overview

Parsers are registered with a central registry that dispatches incoming ACARS messages based on label and content. Each parser:

1. Declares which ACARS labels it handles (or empty for content-based matching)
2. Performs a fast `QuickCheck` to filter messages before expensive processing
3. Parses the message and returns a structured result
4. Provides debug tracing via `ParseWithTrace`

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

| Priority Range | Use Case |
|---------------|----------|
| 1-20 | High-confidence, specific format parsers |
| 21-50 | Standard parsers with clear markers |
| 51-80 | Parsers that may conflict with others |
| 81-100 | Generic or catch-all parsers |

Lower priority = checked first. Use lower priority for parsers with cheap, specific checks.

## QuickCheck Best Practices

The `QuickCheck` method is called for every message before `Parse`. It must be:

1. **Fast** - use `strings.Contains` or `strings.HasPrefix`, never regex
2. **Conservative** - return `true` if the message *might* match
3. **Correct** - returning `false` means `Parse` will never be called

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

## When Token/Regex Parsers Are Acceptable

In rare cases, grok patterns may not be suitable:

1. **Binary protocols** - use byte-level parsing
2. **Highly variable formats** - where no pattern can reliably match
3. **Performance-critical paths** - where grok overhead is measurable

Even in these cases, `ParseWithTrace` must still be implemented using `registry.Extractor` entries to report what was matched.

### Parsers Using Field Extractors

The following parsers use independent field extractors rather than grok patterns, per the criteria above:

**Binary protocols:**
- `adsc` - ADS-C tag-based binary encoding
- `cpdlc` - ASN.1 PER binary encoding
- `envelope` - ARINC hex-encoded TLV with CRC

**Free-form multi-field messages:**
- `atis` - Envelope header + body with fields in variable order
- `weather` - Multiple METAR/TAF/SIGMET reports per message
- `turbulence` - Advisory with independent fields in varying order
- `landingdata` - Performance data with independent fields and tabular sections
- `takeoff` - Performance data with many fields and tabular runway sections
- `parking` - Sparse extraction from French-format messages
- `crew` - Multiple independent crew/schedule fields
- `delay` - Multiple delay code and timing fields
- `dispatch` - Multiple dispatch/MEL reference fields
- `fuel` - Multiple fuel-related fields
- `hazard` - Header + alert fields
- `paxbag` - Multiple flight line extractions
- `paxconn` - Multiple connection flight records

These parsers must still implement `ParseWithTrace` with appropriate extractor entries for debugging.

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
- [ ] Unit tests in `parser_test.go`
- [ ] Uses base patterns from `internal/patterns`
- [ ] `QuickCheck` uses string operations only (no regex)
- [ ] Coordinates parsed with shared utilities
- [ ] JSON field names documented with struct tags