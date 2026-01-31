# Flight Enrichment

The flight enrichment system aggregates data from multiple ACARS message types into a unified view per flight operation. This document covers the design decisions and matching strategies used.

## Table: `flight_enrichment`

Stored in PostgreSQL (`acars_state` database).

| Column | Description |
|--------|-------------|
| `icao_hex` | Aircraft Mode-S transponder code (unique per airframe) |
| `callsign` | Flight callsign (e.g., QTR411, UAL123) |
| `flight_date` | Date of the flight operation |
| `origin` | Departure airport (ICAO or IATA code) |
| `destination` | Arrival airport (ICAO or IATA code) |
| `route` | Array of waypoints |
| `eta` | Estimated time of arrival |
| `runway` | Assigned runway |
| `sid` | Standard Instrument Departure procedure |
| `squawk` | Transponder squawk code |
| `pax_count` | Passenger count |
| `pax_breakdown` | JSON breakdown of passenger classes |

## Callsign Matching Strategy

Airlines use both IATA (2-letter) and ICAO (3-letter) callsign formats interchangeably in ACARS messages:

| Airline | IATA | ICAO |
|---------|------|------|
| Qantas | QF1255 | QFA1255 |
| Qatar Airways | QR411 | QTR411 |
| Ethiopian | ET507 | ETH507 |
| EgyptAir | MS774 | MSR774 |

Different message types often use different formats:
- **Loadsheets** typically use IATA format (QF1255)
- **Position reports** typically use ICAO format (QFA1255)

### The Problem

Without special handling, a single flight would create two separate enrichment records:
- One for QF1255 with loadsheet data (pax count)
- One for QFA1255 with position data (route, ETA)

### The Solution

The enrichment system matches on the **numeric flight number suffix** rather than the exact callsign, combined with:
- `icao_hex` - unique aircraft identifier
- `flight_date` - date of operation

This is safe because the same physical aircraft cannot fly for two different airlines on the same day with the same flight number.

### Implementation

When upserting enrichment data:

1. Extract the numeric suffix from the callsign (e.g., "1255" from "QF1255")
2. Search for an existing row with matching `icao_hex`, `flight_date`, and callsign ending with that number
3. If found, update that row (merging new data with existing)
4. If not found, insert a new row

When a match is found between IATA and ICAO variants, the system prefers the longer (ICAO) format as it's more specific and standardised for ATC communications.

### Validation

This approach was validated against the corpus:
- **1,096 duplicate rows** (5% of total) were caused by IATA/ICAO format differences
- **Zero false positives** detected - no cases where different flights were incorrectly merged

The regex pattern `callsign ~ (flight_num || '$')` matches callsigns ending with the flight number, allowing both QF1255 and QFA1255 to match when searching for "1255".

## Data Sources

The enrichment table is populated from multiple parser types:

| Parser | Contributes |
|--------|-------------|
| `loadsheet` | pax_count, pax_breakdown, origin, destination |
| `flight_plan` | route, origin, destination |
| `pdc` | squawk, runway, sid, origin, destination |
| `eta` | eta |

## Future Improvements

- Normalise airport codes to ICAO format using a reference table
- Add carrier code reference table to map IATA/ICAO airline codes
- Track multiple legs for the same aircraft on the same day