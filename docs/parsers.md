# ACARS Parser Reference

This document lists every message parser registered with the parser registry: its name, the ACARS labels it handles, its priority, its result type, how it matches messages, and what it extracts.

For the rules that new parsers must follow, see [PARSER_SPEC.md](PARSER_SPEC.md).

---

## Parser Architecture

### The Parser Interface

Every parser implements `registry.Parser` (`internal/registry/registry.go`):

```go
type Parser interface {
    Name() string                    // Unique identifier
    Labels() []string                // ACARS labels handled (empty = global, checks all labels)
    QuickCheck(text string) bool     // Fast string check before the full parse
    Priority() int                   // Lower = checked first within a tier
    Parse(msg *acars.Message) Result // Returns nil if the message is not applicable
}
```

Each result implements `registry.Result`, which has two methods: `Type()` (the result type string stored as `parser_type`) and `MessageID()`.

### The Traceable Interface

Every registered parser also implements `registry.Traceable` (`internal/registry/trace.go`), which the `debug` command uses to show why a parser did or did not match a message:

```go
type Traceable interface {
    ParseWithTrace(msg *acars.Message) *TraceResult
}
```

### Registration

Each parser package registers its parsers in an `init()` function by calling `registry.Register`. The `internal/parsers/parsers.go` file blank-imports every parser package so that those `init()` functions run. A parser package that is not imported there is never registered.

The `internal/parsers/arinc` package is not a parser. It is a library that validates the ARINC 622 envelope and CRC, and the `cpdlc` parser uses it.

### Dispatch Order

`Registry.Dispatch` (`internal/registry/registry.go`) processes a message in three tiers:

1. **Label parsers**: parsers whose `Labels()` contains `msg.Label`.
2. **Global parsers**: parsers whose `Labels()` is empty. These run for every message, whatever its label. The `pdc` parser is the only global parser.
3. **Catch-all parsers**: parsers added with `registry.RegisterCatchAll`. This tier runs only when no earlier tier produced a result, and it does not call `QuickCheck`. The registry supports this tier, but no parser currently uses it.

Within each tier, a parser runs only if its `QuickCheck` returns true. Every parser that returns a non-nil result contributes that result, so one message can produce several results.

Global parsers run after label parsers because of the order of the tiers in `Dispatch`, not because of their priority. The `pdc` parser's priority of 500 has no effect on this.

### Priority

`Priority()` orders parsers within a tier: lower numbers run first. Every command calls `Registry.Sort()` before dispatching. Priority does not stop other parsers from running; it only decides the order of the results that `Dispatch` returns.

`Dispatch` returns a `registry.Match` per result, recording the producing parser's name. The `reparse` command compares each stored row with the new result of the same type, and falls back to the first match when that type is no longer produced.

`Sort()` breaks priority ties by parser name, so the order is deterministic and does not depend on registration order.

Most parsers that own a label use priority 100. Lower numbers are used where several parsers share a label. The shared labels are:

| Label | Parsers (priority) |
|-------|--------------------|
| RA | dispatcher (45), weather (50), delay_summary (50), parking_info (50), crew_list (55), pax_bag (55), pax_conn_status (55), takeoff_data (55), gateassign (60), loadsheet (60), fuel_delivery (100) |
| H1 | fpn (10), h1pos (20), pwi (30), mdc (40), dispatcher (45), cpdlc (50), trajectory (50), weather (50), takeoff_data (55), acmsreport (60), cmcreport (60), hazard_alert (60), loadsheet (60) |
| C1 | weather (50), takeoff_data (55), loadsheet (60), turbulence (65), landingdata (70) |
| 3E | delay_summary (50), pax_conn_status (55), fuel_delivery (100) |
| AA | cpdlc (50), envelope (100) |
| SA | hazard_alert (60), mediaadv (100) |
| 10 | loadsheet (60), label10 (100) |
| 21 | weather (50), label21 (100) |
| 22 | loadsheet (60), label22 (100) |

### Matching Techniques

Parsers use one of the following techniques:

- **Grok (`patterns.Compiler`)**: the formats are declared in the package's `grok.go` as `patterns.Format` values with `{PLACEHOLDER}` references to the base patterns in `internal/patterns/base_patterns.go`. The patterns are compiled by `patterns.NewCompiler`.
- **Own format engine**: `pdc` and `loadsheet` each define their own regex-based format list and compiler (`pdc/grok.go`, `loadsheet/grok.go`). They do not use `internal/patterns.Compiler` or its base patterns.
- **Hand-written regex**: standalone package-level `regexp.MustCompile` patterns, each extracting one field.
- **Binary decoding**: hex-encoded payloads decoded byte by byte or bit by bit (`adsc`, `cpdlc`, part of `envelope`).
- **Tokeniser**: the `fpn` parser splits flight plans into sections with `h1/tokeniser.go`.

---

## Parser Summary

The "Tests" column records whether the package has `_test.go` files that exercise that parser.

| Name() | Package | Labels | Priority | Type() | Technique | Tests |
|--------|---------|--------|----------|--------|-----------|-------|
| [acmsreport](#acmsreport) | acmsreport | H1 | 60 | `acms_report` | Grok | Yes |
| [adsc](#adsc) | adsc | B6 | 10 | `adsc` | Binary tag decoding | Yes |
| [agfsr](#agfsr) | agfsr | 4T | 100 | `agfsr` | Grok | No |
| [atis](#atis) | atis | A9 | 100 | `atis` | Hand-written regex | Yes |
| [cmcreport](#cmcreport) | cmcreport | H1 | 60 | `cmc_report` | Grok | Yes |
| [cpdlc](#cpdlc) | cpdlc | AA, BA, H1 | 50 | `cpdlc` | ARINC layer + ASN.1 UPER decoding | Yes |
| [crew_list](#crew_list) | crew | RA | 55 | `crew_list` | Hand-written regex | Yes |
| [delay_summary](#delay_summary) | delay | 3E, RA | 50 | `delay_summary` | Hand-written regex | Yes |
| [dispatcher](#dispatcher) | dispatch | RA, 25, H1 | 45 | `dispatcher` | Hand-written regex | Yes |
| [envelope](#envelope) | envelope | AA, A6 | 100 | `envelope` | Hand-written regex + binary decoding | Yes |
| [eta](#eta) | eta | 5Z | 100 | `eta` | Grok | No |
| [fst](#fst) | fst | 15 | 100 | `fst` | Grok + hand-written regex | No |
| [fuel_delivery](#fuel_delivery) | fuel | 3E, RA | 100 | `fuel_delivery` | Hand-written regex | Yes |
| [gateassign](#gateassign) | gateassign | RA | 60 | `gate_assignment` | Grok | No |
| [fpn](#fpn) | h1 | H1, 4A, HX | 10 | `flight_plan` | Tokeniser | Yes |
| [h1pos](#h1pos) | h1 | H1 | 20 | `h1_position` | Grok | No |
| [pwi](#pwi) | h1 | H1 | 30 | `pwi` | Custom section parsing | No |
| [mdc](#mdc) | h1 | H1 | 40 | `mdc` | Hand-written regex | Yes |
| [trajectory](#trajectory) | h1 | H1 | 50 | `trajectory` | Hand-written regex | Yes |
| [h2_wind](#h2_wind) | h2wind | H2 | 100 | `h2_wind` | Grok | No |
| [hazard_alert](#hazard_alert) | hazard | _, H1, SA | 60 | `hazard_alert` | Hand-written regex | Yes |
| [label10](#label10) | label10 | 10 | 100 | `label10_position` | Grok | No |
| [label16](#label16) | label16 | 16 | 100 | `waypoint_position` | Grok | No |
| [label21](#label21) | label21 | 21 | 100 | `position_report` | Grok | No |
| [label22](#label22) | label22 | 22 | 100 | `label22_position` | Grok | No |
| [label44](#label44) | label44 | 44 | 100 | `label44` | Grok | No |
| [label4j](#label4j) | label4j | 4J | 100 | `pos_weather` | Grok | No |
| [label5l](#label5l) | label5l | 5L | 100 | `route` | Grok | No |
| [label80](#label80) | label80 | 80 | 100 | `position` | Grok | No |
| [label83](#label83) | label83 | 83 | 100 | `label83_position` | Grok | No |
| [labelb2](#labelb2) | labelb2 | B2 | 100 | `oceanic_clearance` | Grok | No |
| [labelb3](#labelb3) | labelb3 | B3 | 100 | `gate_info` | Grok | No |
| [labelrf](#labelrf) | labelrf | RF | 100 | `flight_subscription` | Grok | No |
| [landingdata](#landingdata) | landingdata | C1 | 70 | `landing_data` | Hand-written regex | No |
| [loadsheet](#loadsheet) | loadsheet | 10, 13, 14, 22, 2A, 30, 31, 35, 3S, 42, 45, C1, H1, RA | 60 | `loadsheet` | Own format engine | Yes |
| [mediaadv](#mediaadv) | mediaadv | SA | 100 | `media_advisory` | Grok | Yes |
| [parking_info](#parking_info) | parking | 1E, RA | 50 | `parking_info` | Hand-written regex | Yes |
| [pax_bag](#pax_bag) | paxbag | RA | 55 | `pax_bag` | Hand-written regex | Yes |
| [pax_conn_status](#pax_conn_status) | paxconn | 3E, RA | 55 | `pax_conn_status` | Hand-written regex | Yes |
| [pdc](#pdc) | pdc | *(global)* | 500 | `pdc` | Own format engine | Yes |
| [sq](#sq) | sq | SQ | 100 | `sq_position` | Grok | Yes |
| [takeoff_data](#takeoff_data) | takeoff | RA, H1, C1 | 55 | `takeoff_data` | Hand-written regex | Yes |
| [turbulence](#turbulence) | turbulence | C1 | 65 | `turbulence` | Hand-written regex | No |
| [weather](#weather) | weather | RA, C1, 21, H1, 3W, 27, 31, 34, 3T, 23 | 50 | `weather` | Hand-written regex | No |

That is 44 parsers in 40 packages. The `h1` package registers five parsers: `fpn`, `h1pos`, `pwi`, `mdc` and `trajectory`.

---

## Parser Details

The parsers are listed in alphabetical order of package.

### acmsreport

**Package:** `internal/parsers/acmsreport` · **Labels:** H1 · **Priority:** 60 · **Type:** `acms_report`

**Technique:** Grok (format `acms_cc`).

**Description:** Parses the header and CC block of Airbus aircraft condition monitoring system (ACMS) reports. For example:

```
A321,014057,1,1,TB000000/REP001,00,00,1/CCVH-VWT,JAN20,040543,YSSY,YBBN,0816/C0TIA05JST4R0000/...
```

**Extracted fields:** the aircraft series (`A321`), the report number (`001`), and from the CC block the registration, report date (MMMDD, no year) and time (HHMMSS), origin, destination and the flight number's digits (`0816`). The serial number and the `TB000000` block are not captured, because their meaning is not established.

**Aircraft series, not type:** The series is reported as `aircraft_series`, not `aircraft_type`, so it is not normalised to an ICAO designator. In the January 2026 corpus, `A320` and `A321` were sent by aircraft that other messages identify as A20N (39 tails) and A21N (52 tails): the series does not distinguish the ceo from the neo.

**Registration:** Reported only when it is the tail transmitted in the ACARS header, ignoring dashes and a leading `.`.

**Route pairing:** The CC block gives the flight number without its airline code. The extractor uses the route only when the message transmits a flight with the same number (`JST816` for `0816`), since a report can be stored on one flight and sent on another (see [storage.md](storage.md)).

**Validation:** Both airports must pass `patterns.IsValidICAO`.

**Coverage (January 2026 corpus):** 49,603 of the 50,606 reports with a CC block parsed, giving a series for 2,623 registrations (none with two series) and 208 (flight, origin, destination) combinations. The rest are other layouts: blanked airports, padded IATA codes, and blocks without a time.

---

### adsc

**Package:** `internal/parsers/adsc` · **Labels:** B6 · **Priority:** 10 · **Type:** `adsc`

**Technique:** Binary tag decoding, based on the libacars ADS-C decoder. The flight identifier in the text prefix is matched with the shared `patterns.ADSCFlightPattern`.

**Description:** Parses ADS-C (Automatic Dependent Surveillance - Contract) downlink reports from the hex-encoded payload.

**Extracted fields:**
- Registration, ground station and flight ID from the text prefix
- Basic report: latitude, longitude, altitude, report time (seconds past the hour), position accuracy, navigation redundancy and TCAS availability
- Flight ID (tag 12), predicted route (tag 13), earth reference (tag 14), air reference (tag 15), meteorological data (tag 16) and airframe ID (tag 17)
- The message type for acknowledgement, negative acknowledgement, non-compliance and cancel-emergency messages

**Scaling:** ARINC 745 itself is not publicly available. The field scalings follow [ICAO GOLD](https://www.icao.int/sites/default/files/SAM/eDocuments/GOLD%202aEdicionInglesUnicamente.pdf) (2nd edition, 2013), [libacars `adsc.c`](https://github.com/szpajder/libacars/blob/master/libacars/adsc.c) and [JAERO `arincparse.h`](https://github.com/jontio/JAERO/blob/master/JAERO/arincparse.h):

| Field | Scaling |
|---|---|
| Latitude and longitude | 21-bit signed, 90/2^19 degrees per bit |
| Altitude | 16-bit signed, 4 ft per bit |
| Mach | 0.0005 per bit |
| Track and heading | 90/2^10 degrees per bit |
| Report time | 0.125 s per bit |

**Limitations:** The intermediate projection (tag 22, 8 bytes per point) and fixed projection (tag 23, 9 bytes) groups are skipped by length and not decoded; `adsc/parser.go` marks both with a `TODO`.

---

### agfsr

**Package:** `internal/parsers/agfsr` · **Labels:** 4T · **Priority:** 100 · **Type:** `agfsr`

**Technique:** Grok (formats `agfsr_status` and `position`).

**Description:** Parses AGFSR flight status reports. The quick check looks for `AGFSR`.

**Extracted fields:** flight number, day of month, route, origin, destination, report time, latitude, longitude, flight level, phase, fuel remaining, fuel used, Mach, wind direction and speed, heading, ground speed, ETA and scheduled time.

---

### atis

**Package:** `internal/parsers/atis` · **Labels:** A9 · **Priority:** 100 · **Type:** `atis`

**Technique:** Hand-written regex.

**Description:** Parses D-ATIS (digital Automatic Terminal Information Service) broadcasts.

**Extracted fields:** airport, ATIS letter, ATIS type (ARR/DEP), ATIS time, runways, approaches, wind, visibility, clouds, temperature, dew point, QNH and remarks.

---

### cmcreport

**Package:** `internal/parsers/cmcreport` · **Labels:** H1 · **Priority:** 60 · **Type:** `cmc_report`

**Technique:** Grok (format `cmc_header`).

**Description:** Parses the header line of Boeing central maintenance computer (CMC) reports: RTE (route), PLF (post-flight) and CFG (configuration). For example:

```
RTE 1 04OCT26 0930 TG HS-TWC THA482 YPPH/VTBS BCG4F-45LD-0077 C L 0915 04OCT26
```

**Extracted fields:** report type, sequence number, report date (DDMMMYY) and time (HHMM), IATA airline code, registration, ICAO callsign, origin and destination.

**Registration and airline code:** The registration field sometimes carries the IATA airline code, either separated by a space (`TG HS-TWC`) or glued to its front (`5YN703GT`, `BRB-17807`). A glued code cannot be told apart from the registration reliably on its own, so the tail transmitted in the ACARS header is used: the registration is the part of the field that matches the header tail (ignoring dashes, since `HP-9907` and `HP9907` both occur), and the remainder must be a 2-character code of letters and digits. No registration or glued airline code is reported if the field does not end with the header tail, if the tail is not a plausible registration (at least 2 characters, including a letter), or if the remainder is anything else. If a separately transmitted airline code and a glued one disagree, neither is reported.

**Validation:** The header must be on one line, with fields separated by spaces. The callsign must be three letters followed by a flight number that starts with a digit. Both airports must pass `patterns.IsValidICAO`, and the destination must be a whole token (`KJAXX` is not read as `KJAX`). A header that fails any of these checks is not parsed.

**Route pairing:** The result names its own flight. If the message transmits a flight with another flight number, the report was stored on an earlier flight, and the extractor does not use its route (see [storage.md](storage.md)).

**Coverage (January 2026 corpus):** 19,136 of 19,183 CMC reports parsed (99.75%), giving 1,977 aircraft, 3,848 (flight, origin, destination) combinations and 85 IATA-to-ICAO airline code pairs. The rest are other layouts (a weather CFG report, RTE lines without a sequence number) that carry no flight or route.

---

### cpdlc

**Package:** `internal/parsers/cpdlc` · **Labels:** AA, BA, H1 · **Priority:** 50 · **Type:** `cpdlc`

**Technique:** The ARINC layer (`internal/parsers/arinc`) validates the envelope and CRC and extracts the payload. The payload is then decoded as FANS-1/A ASN.1 unaligned PER with `github.com/shaneshort/go-asn/uper`, using the type definitions in `fans_uper_types.go`.

**Description:** Parses FANS-1/A CPDLC (Controller-Pilot Data Link Communications) messages. The quick check looks for the `.AT1.`, `.CR1.`, `.CC1.` and `.DR1.` IMI markers.

Label H1 carries CPDLC in two further forms, which `arinc.Unwrap` converts to the envelope form: relayed with its original label (`- #MD/AA PIKCPYA.AT1.N657UA...`) and without the leading `/` (`USADCXA.AT1.N200WN...`).

The result type is always `cpdlc`. The kind of message is recorded in the `message_type` field: `cpdlc`, `connect_request`, `connect_confirm` or `disconnect`. Connection messages carry no CPDLC payload.

**Direction:** The uplink and downlink message sets give different meanings to the same element numbers (element 0 is dM0 WILCO as a downlink and uM0 UNABLE as an uplink), so the direction decides what a message says. It is taken from the feed's link direction, then the ACARS block ID, then the label: AA is an uplink and BA a downlink (for a relayed H1 message, its original label). In ten minutes of live traffic (October 2026), all 118 AA messages were uplinks and all 77 BA messages downlinks. Label H1 carries both directions, so it does not give one.

The payload is decoded with both message sets, and a decode is valid when every element is defined:
- If only one message set gives valid elements, it is used, and the result reports that direction, whatever the feed indicated.
- If both do, the known direction decides. If the direction is not known, the elements are not reported and the error is `direction_unknown`.
- If neither does, the elements are not reported and the error is `no_valid_elements`.

**Extracted fields:** message type, direction, ground station, registration, header (message ID, optional reference number and optional timestamp), every message element (the primary element and any additional elements) with its label and formatted text, the formatted text of the whole message, the raw hex and any error (for example `crc_failed`).

**Limitations:**
- Route clearance data is not converted. For the elements that carry a `[routeclearance]` (UM79, UM80, UM83, UM85 and UM86), the payload is decoded into the UPER types, but the result contains only the element ID and label template. It carries no route data and no formatted text.
- Some element templates are not filled in from the decoded data: for example, dM6 `REQUEST [altitude]` keeps its placeholder in the formatted text, although the element's data holds the level. Some uplink elements, such as uM153 `ALTIMETER [altimeter]` and uM61, are decoded without their data.
- The `holdAtWaypoint` route information type and the RNP requirements field are defined in simplified form in `fans_uper_types.go`, not in full.

---

### crew_list

**Package:** `internal/parsers/crew` · **Labels:** RA · **Priority:** 55 · **Type:** `crew_list`

**Technique:** Hand-written regex.

**Description:** Parses crew list messages. The quick check looks for `CREW LIST`.

**Extracted fields:** flight number, flight date, origin, destination, sent time, gate ETA, cockpit crew and cabin crew (each member's position, name and employee ID), and minimum crew.

---

### delay_summary

**Package:** `internal/parsers/delay` · **Labels:** 3E, RA · **Priority:** 50 · **Type:** `delay_summary`

**Technique:** Hand-written regex.

**Description:** Parses delay summary messages. The quick check looks for `DELAY SUMMARY`.

**Extracted fields:** flight number, flight date, origin, destination, scheduled and actual departure times, departure delay, scheduled and actual arrival times, arrival delay, IATA delay codes with minutes, and the message creation time.

---

### dispatcher

**Package:** `internal/parsers/dispatch` · **Labels:** RA, 25, H1 · **Priority:** 45 · **Type:** `dispatcher`

**Technique:** Hand-written regex.

**Description:** Parses dispatcher messages from airline operations. The quick check looks for `DISPATCHER MSG`.

**Extracted fields:** flight number, tail, acknowledgement flag, category, MEL (Minimum Equipment List) reference, MDDR number, dispatcher ID, timestamp and message content.

---

### envelope

**Package:** `internal/parsers/envelope` · **Labels:** AA, A6 · **Priority:** 100 · **Type:** `envelope`

**Technique:** Hand-written regex for the header, and binary decoding for the ADS-C payload.

**Description:** Extracts the aircraft registration and ground station from ARINC envelope headers (`/<station>.AT1.`, `.CR1.` and `.ADS` messages). It does not decode the ADS-C payload: on label A6 it is a contract request from the ground station, which holds no position (aircraft reports are on B6, decoded by the `adsc` parser). On label AA, it runs after the `cpdlc` parser, and both can return a result for the same message.

**Extracted fields:** tail, station, message type (AT1, CR1 or ADS), payload size, and, for ADS-C, latitude, longitude and altitude.

---

### eta

**Package:** `internal/parsers/eta` · **Labels:** 5Z · **Priority:** 100 · **Type:** `eta`

**Technique:** Grok (formats `et_exp_time`, `ir_format`, `b6_ldg_data`, `os_format` and `c3_route`).

**Description:** Parses ETA and timing messages in the ET, IR, B6, OS and C3 formats.

**Extracted fields:** message type, origin, destination, day of month, report time, ETA, mode, runway and gate.

---

### fst

**Package:** `internal/parsers/fst` · **Labels:** 15 · **Priority:** 100 · **Type:** `fst`

**Technique:** Grok (formats `fst_5digit_lon` and `fst_6digit`), plus hand-written regex for the heading and ground speed.

**Description:** Parses FST flight status reports.

**Extracted fields:** sequence, origin, destination, latitude, longitude, flight level, heading, ground speed and temperature.

---

### fuel_delivery

**Package:** `internal/parsers/fuel` · **Labels:** 3E, RA · **Priority:** 100 · **Type:** `fuel_delivery`

**Technique:** Hand-written regex.

**Description:** Parses fuel delivery receipts. The quick check looks for `FUEL DELIVERY`.

**Extracted fields:** flight number, tail, date, destination, fuel company, fuel grade, truck ID, start and end times, amount in litres, density (kg/m³) and the quantity before fuelling (kg).

---

### gateassign

**Package:** `internal/parsers/gateassign` · **Labels:** RA · **Priority:** 60 · **Type:** `gate_assignment`

**Technique:** Grok (formats `simple_gate`, `in_range_gate` and `structured_gate`).

**Description:** Parses gate assignment messages. The quick check looks for `GATE ASSIGNMENT`, or for both `GATE` and `ASSIGNED`.

**Extracted fields:** gate, parking position (PPOS), baggage belt, next flight and next route.

---

### fpn

**Package:** `internal/parsers/h1` (`parser.go`, `tokeniser.go`) · **Labels:** H1, 4A, HX · **Priority:** 10 · **Type:** `flight_plan`

**Technique:** Tokeniser. `h1/tokeniser.go` splits the flight plan into its sections, and the shared `patterns` coordinate functions convert waypoint coordinates.

**Description:** Parses FPN flight plan messages, including SIDs, STARs and approaches.

**Extracted fields:** flight number, origin, destination, route, waypoints with coordinates, departure and departure transition, arrival and arrival transition, approach, approach type, approach runway, approach route and approach waypoints, and a truncation flag. The flag is set when a multi-part message (`#M1`) has no final part (`#MD`), or when the ARINC CRC at the end of a message with a `/WD` section does not verify.

---

### h1pos

**Package:** `internal/parsers/h1` (`parser.go`) · **Labels:** H1 · **Priority:** 20 · **Type:** `h1_position`

**Technique:** Grok (formats `h1_position_time` and `h1_position_alt`).

**Description:** Parses H1 POS position reports.

**Extracted fields:** latitude, longitude, report time, flight level, ground speed, current, next and third waypoints, ETA, temperature, and wind direction and speed.

---

### pwi

**Package:** `internal/parsers/h1` (`parser.go`) · **Labels:** H1 · **Priority:** 30 · **Type:** `pwi`

**Technique:** Custom section parsing.

**Description:** Parses PWI (Predicted Wind Information) messages.

**Extracted fields:** report time, climb winds (CB) by flight level, descent winds (DD) by flight level, and route winds (WD) by flight level and waypoint, including temperature.

---

### mdc

**Package:** `internal/parsers/h1` (`mdc.go`) · **Labels:** H1 · **Priority:** 40 · **Type:** `mdc`

**Technique:** Hand-written regex.

**Description:** Parses MDC (Maintenance Data Computer) reports. The quick check looks for `MDC REPORT:`.

**Extracted fields:** report type, write option, file name, time, date, application and table part numbers, leg number, engine trend data (left and right N1, N2, ITT, PS3, vibration, oil temperature and pressure, PLA, fuel flow, VG position, plus FADEC control, airspeed, altitude and total air temperature) and fault entries (ATA chapter, system, LRU, status, message and equation ID).

---

### trajectory

**Package:** `internal/parsers/h1` (`trajectory.go`) · **Labels:** H1 · **Priority:** 50 · **Type:** `trajectory`

**Technique:** Hand-written regex.

**Description:** Parses trajectory (position history) messages that start with `++86501` or `++76502`.

**Extracted fields:** registration, aircraft type, date, flight number, origin, destination, distance, system ID, and a list of positions, each with latitude, longitude, time, altitude, temperature, heading, ground speed and flight phase.

---

### h2_wind

**Package:** `internal/parsers/h2wind` · **Labels:** H2 · **Priority:** 100 · **Type:** `h2_wind`

**Technique:** Grok (formats `h2_header` and `wind_layer`).

**Description:** Parses H2 wind messages. Only messages that start with `02A` are parsed; encoded messages are skipped.

**Extracted fields:** origin, destination, latitude, longitude, report time, and wind layers (flight level, temperature, wind direction, wind speed and a gust flag).

---

### hazard_alert

**Package:** `internal/parsers/hazard` · **Labels:** _, H1, SA · **Priority:** 60 · **Type:** `hazard_alert`

**Technique:** Hand-written regex.

**Description:** Parses ARINC Direct HAZARD ALERT messages for turbulence and wind warnings. The quick check looks for `HAZARD ALERT`.

**Extracted fields:** sender, recipient, callsign, flight ID, origin, destination, ETD, segment, ETO, EDR (Eddy Dissipation Rate) turbulence value, wind warning and alert level.

**Limitations:** The registry matches labels exactly, and no real traffic uses the label `_`, so this parser effectively routes on H1 and SA only. The label was probably meant to be `_d`, the general response label; `hazard/parser.go` has a `TODO` to confirm this.

---

### label10

**Package:** `internal/parsers/label10` · **Labels:** 10 · **Priority:** 100 · **Type:** `label10_position`

**Technique:** Grok (format `rich_position`).

**Description:** Parses Label 10 position reports that include the route ahead.

**Extracted fields:** latitude, longitude, Mach, heading, flight level, destination, ETA, fuel, distance, and waypoints with ETAs.

---

### label16

**Package:** `internal/parsers/label16` · **Labels:** 16 · **Priority:** 100 · **Type:** `waypoint_position`

**Technique:** Grok (formats `csv_position`, `csv_position_no_alt`, `csv_position_extended`, `waypoint_position_prefixed`, `waypoint_position` and `autpos`).

**Description:** Parses Label 16 waypoint position reports in CSV and AUTPOS formats.

**Extracted fields:** time, flight, waypoint, latitude, longitude, flight level, ground speed, ETA and track.

---

### label21

**Package:** `internal/parsers/label21` · **Labels:** 21 · **Priority:** 100 · **Type:** `position_report`

**Technique:** Grok (format `posn_report`).

**Description:** Parses Label 21 POSN position reports.

**Extracted fields:** latitude, longitude, heading, altitude, fuel on board, temperature, wind, ETA and destination.

---

### label22

**Package:** `internal/parsers/label22` · **Labels:** 22 · **Priority:** 100 · **Type:** `label22_position`

**Technique:** Grok (format `dms_position`).

**Description:** Parses Label 22 position reports with coordinates in degrees, minutes and seconds.

**Extracted fields:** latitude, longitude, report time, altitude, Mach, flight level, ground speed and track.

---

### label44

**Package:** `internal/parsers/label44` · **Labels:** 44 · **Priority:** 100 · **Type:** `label44`

**Technique:** Grok (formats `runway_header`, `fb_position`, `pos_report` and `runway_line`).

**Description:** Parses Label 44 takeoff runway lists, FB position reports and POS reports. Messages that contain `|` or `\` are treated as encoded and skipped.

**Extracted fields:** message type, airport, runways (with suffix and distance), procedures, latitude, longitude, flight level, origin, destination, callsign and report time.

---

### label4j

**Package:** `internal/parsers/label4j` · **Labels:** 4J · **Priority:** 100 · **Type:** `pos_weather`

**Technique:** Grok (formats `pos_weather` and `fuel_burn`).

**Description:** Parses Label 4J combined position and weather reports.

**Extracted fields:** latitude, longitude, heading, altitude, temperature, current and next waypoints, ETA and fuel burn.

---

### label5l

**Package:** `internal/parsers/label5l` · **Labels:** 5L · **Priority:** 100 · **Type:** `route`

**Technique:** Grok (format `route`).

**Description:** Parses Label 5L route messages.

**Extracted fields:** callsign, origin and destination (IATA and ICAO), flight ID, date, and scheduled and actual departure and arrival times.

---

### label80

**Package:** `internal/parsers/label80` · **Labels:** 80 · **Priority:** 100 · **Type:** `position`

**Technique:** Grok (header formats `header_format` and `alt_format`, plus one format per field).

**Description:** Parses Label 80 position and OOOI (OUT/OFF/ON/IN) messages.

**Extracted fields:** message type, flight number, origin and destination (ICAO), latitude, longitude, altitude, Mach, TAS, fuel on board, ETA, and the OUT, OFF, ON and IN times.

---

### label83

**Package:** `internal/parsers/label83` · **Labels:** 83 · **Priority:** 100 · **Type:** `label83_position`

**Technique:** Grok (formats `pr_position` and `zspd_position`).

**Description:** Parses Label 83 position reports in the PR and ZSPD formats.

**Extracted fields:** message type, day of month, report time, latitude, longitude, altitude, heading, ground speed, origin and destination.

---

### labelb2

**Package:** `internal/parsers/labelb2` · **Labels:** B2 · **Priority:** 100 · **Type:** `oceanic_clearance`

**Technique:** Grok (formats `oceanic_dest`, `oceanic_fix`, `flight_level`, `mach` and `flight_num`).

**Description:** Parses Label B2 oceanic clearances.

**Extracted fields:** flight number, destination, route, oceanic fixes, flight level and Mach.

---

### labelb3

**Package:** `internal/parsers/labelb3` · **Labels:** B3 · **Priority:** 100 · **Type:** `gate_info`

**Technique:** Grok (formats `gate_info`, `atis` and `aircraft_type`).

**Description:** Parses Label B3 gate information messages.

**Extracted fields:** flight number, origin, destination, gate, ATIS letter and aircraft type.

---

### labelrf

**Package:** `internal/parsers/labelrf` · **Labels:** RF · **Priority:** 100 · **Type:** `flight_subscription`

**Technique:** Grok (format `flight_subscription`).

**Description:** Parses SITA FDA (Flight Data Application) flight subscription messages. The message types are FDASUB, FDACOM, FSTREQ and FDAACK.

**Extracted fields:** message type, origin and destination (IATA, and ICAO converted from IATA), flight number, date, time, aircraft type and registration.

---

### landingdata

**Package:** `internal/parsers/landingdata` · **Labels:** C1 · **Priority:** 70 · **Type:** `landing_data`

**Technique:** Hand-written regex.

**Description:** Parses landing performance data. The quick check looks for `LANDING DATA`.

**Extracted fields:** airport, runway, runway length, aircraft type, flap setting, temperature, altimeter, wind, landing weight, structural limit, performance limit and runway condition.

---

### loadsheet

**Package:** `internal/parsers/loadsheet` · **Labels:** 10, 13, 14, 22, 2A, 30, 31, 35, 3S, 42, 45, C1, H1, RA · **Priority:** 60 · **Type:** `loadsheet`

**Technique:** Own format engine. `loadsheet/grok.go` defines 18 `LoadsheetFormat` entries, each with its own labels, and its own regex compiler. `Labels()` returns the union of the labels of all formats.

**Description:** Parses weight and balance loadsheets. The formats are `standard_kg`, `standard_kg_minimal`, `qantas_tonnes`, `ba_full_names`, `jetsmart`, `jetsmart_minimal`, `chinese_airlines`, `jat_bw_dow`, `european_edn`, `cathay_act`, `tui_edn`, `eat_cargo_lb`, `ethiopian`, `kalitta_cargo_lb`, `kalitta_old_lb`, `dhl_cargo_kg`, `french_bee_short` and `vic_corsair`.

**Extracted fields:** format name, status, flight, origin, destination, aircraft type, ZFW and maximum ZFW, TOW and maximum TOW, LAW and maximum LAW, take-off fuel, trip fuel, passengers, crew, MAC at ZFW and TOW, and edition.

---

### mediaadv

**Package:** `internal/parsers/mediaadv` · **Labels:** SA · **Priority:** 100 · **Type:** `media_advisory`

**Technique:** Grok (format `media_advisory_v0`), based on the libacars media advisory format.

**Description:** Parses media advisory messages, which report the state of the data links (VHF, SATCOM, HF, VDL2 and others).

**Extracted fields:** version, current link, whether the link was established or lost, link time, available links and free text.

---

### parking_info

**Package:** `internal/parsers/parking` · **Labels:** 1E, RA · **Priority:** 50 · **Type:** `parking_info`

**Technique:** Hand-written regex.

**Description:** Parses parking information messages, including the French ESCALE format. The quick check looks for `PKG INFO MSG`.

**Extracted fields:** airport, parking stand and baggage carousel.

---

### pax_bag

**Package:** `internal/parsers/paxbag` · **Labels:** RA · **Priority:** 55 · **Type:** `pax_bag`

**Technique:** Hand-written regex.

**Description:** Parses passenger and baggage details for weight and balance. The quick check looks for `PAX AND BAG DETAILS`.

**Extracted fields:** aircraft type, registration, configuration, flight number, date, origin, destination, STD, boarding time, gate, passenger totals (adults, male, female, children and infants), passenger counts by zone, bag count, bag weight, and whether the figures are final.

---

### pax_conn_status

**Package:** `internal/parsers/paxconn` · **Labels:** 3E, RA · **Priority:** 55 · **Type:** `pax_conn_status`

**Technique:** Hand-written regex.

**Description:** Parses passenger connection status messages. The quick check looks for `PAX CONN STATUS`.

**Extracted fields:** current flight, connecting flights (flight number, date, time, destination, gate, wait decision, class, passengers and bags), and counts of missed, pending, will-wait and total connecting passengers.

---

### pdc

**Package:** `internal/parsers/pdc` · **Labels:** none (global) · **Priority:** 500 · **Type:** `pdc`

**Technique:** Own format engine. `pdc/grok.go` defines its own base patterns, `{PLACEHOLDER}` expansion and compiler, separate from `internal/patterns.Compiler`. Parsing is strict: a message that does not match one of the formats is not parsed.

**Description:** Parses Pre-Departure Clearances. As the only global parser, it checks messages on every label, after all label parsers have run. The 28 formats cover Australian domestic (Qantas, Jetstar, Virgin Australia and regional), US carriers (Delta, American, Southwest, SkyWest, Republic, Alaska/Hawaiian, Horizon, Frontier, UPS and regional operators), Canadian (WestJet, NAV CANADA, Jazz), UK and European DC1 clearances, ARINC-formatted clearances, Qantas Pacific and private jets.

**Extracted fields:** flight number, aircraft ICAO type, origin, destination, departure time, runway, SID, route and route waypoints, squawk, departure frequency, initial altitude, flight level, aircraft type, ATIS letter, the matched format name and a parse confidence value.

---

### sq

**Package:** `internal/parsers/sq` · **Labels:** SQ · **Priority:** 100 · **Type:** `sq_position`

**Technique:** Grok (formats `arinc_position` and `avicom_frequency`).

**Description:** Parses SQ (squitter) messages from ground stations. The quick check looks for the `02X` (ARINC) or `02J` (AVICOM Japan) prefix.

**Extracted fields:** IATA code, ICAO code, latitude, longitude, frequency band, frequency (MHz) and message type.

---

### takeoff_data

**Package:** `internal/parsers/takeoff` · **Labels:** RA, H1, C1 · **Priority:** 55 · **Type:** `takeoff_data`

**Technique:** Hand-written regex.

**Description:** Parses takeoff performance data. The quick check looks for `TAKEOFF DATA` or `T/O DATA`. A result is returned only if a gross takeoff weight or at least one runway is found.

**Extracted fields:** flight number, aircraft type, engine type, time, wind, OAT, QNH, gross takeoff weight, centre of gravity, passengers, fuel, cargo, ZFW, remarks, and runways (airport, runway, length and shift). V1 is assigned to the last runway found.

**Limitations:** `RunwayData` also declares VR, V2, flex temperature, flex EPR, flaps, EPR, MRTW, MTOW and the limit code, but `Parse` never populates them, so they are always absent from the output. The VR, V2 and flex patterns are used only by `ParseWithTrace`. `takeoff/parser.go` has a `TODO` explaining that more sample messages are needed to confirm which runway column each value belongs to.

---

### turbulence

**Package:** `internal/parsers/turbulence` · **Labels:** C1 · **Priority:** 65 · **Type:** `turbulence`

**Technique:** Hand-written regex.

**Description:** Parses turbulence advisories and SIGMETs. The quick check requires `TURB` together with `SIGMET`, `ADVISORY` or `WSI`.

**Extracted fields:** turbulence type, ID, severity, lower and upper altitude, validity period, movement, description, and entry and exit points.

---

### weather

**Package:** `internal/parsers/weather` · **Labels:** RA, C1, 21, H1, 3W, 27, 31, 34, 3T, 23 · **Priority:** 50 · **Type:** `weather`

**Technique:** Hand-written regex.

**Description:** Parses METAR, TAF and SIGMET reports. One message can contain several reports of each kind.

**Extracted fields:**
- METAR: airport, time, wind (direction, speed and gust), visibility, weather, clouds, temperature, dew point and QNH
- TAF: airport, issue time and validity period
- SIGMET: ID, validity period, originator, FIR, phenomenon, altitude and movement

---

## Adding Trace Support

Every parser must implement `registry.Traceable`. A minimal implementation looks like this:

```go
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
    trace := &registry.TraceResult{
        ParserName: p.Name(),
    }

    // 1. Record the QuickCheck result.
    quickCheckPassed := p.QuickCheck(msg.Text)
    trace.QuickCheck = &registry.QuickCheck{
        Passed: quickCheckPassed,
    }
    if !quickCheckPassed {
        trace.QuickCheck.Reason = "Reason the quick check failed"
        return trace
    }

    // 2. For grok parsers, copy the formats from compiler.ParseWithTrace() into trace.Formats.
    // 3. For regex parsers, add one registry.Extractor per pattern to trace.Extractors.

    trace.Matched = p.Parse(msg) != nil
    return trace
}
```

Use the `debug` command to test tracing:

```bash
./acars_parser debug -id 12345
./acars_parser debug -id 12345 -type pdc -all
./acars_parser debug -text "YOUR MESSAGE" -label H1
```
