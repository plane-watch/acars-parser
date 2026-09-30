# Airframes NATS Payload

This document describes the JSON messages published on the Airframes NATS bus (subject `v1.aircraft.ingest.<source>.message.<message id>.created`), as observed in a 2,000-message capture on 30 September 2026. It lists every field that was populated in that capture and notes which ones the code currently reads.

`internal/acars/message.go` models only part of this payload (`NATSWrapper`, `NATSInner`, `Airframe`, `Flight`, `Station`). Go's JSON decoder ignores the rest, so the other fields are discarded at ingest.

## Delivery behaviour

- **Every message is published twice.** In the capture, all 1,000 distinct message IDs arrived exactly twice, as byte-identical payloads on the same subject, one or two messages apart. The relay's message-ID deduplication and `live`'s subject-ID check drop the second copy.
- **There is no sub-label field.** H1 sub-labels such as `#MD/AA` exist only inside `message.text`, so they must be recognised in the text.
- **Most messages have no text.** Only about 35% of messages carry `message.text`. The rest are link-layer frames (mostly VDL and HFDL) with metadata only.

## Top-level objects

| Object | Present | Contents |
|---|---|---|
| `message` | always | The ACARS message and its reception metadata |
| `messageReport` | always | The reception report from the feeding station |
| `source` | always | The decoder software and protocol that produced the message |
| `station` | always | The receiving station |
| `airframe` | ~85% | The aircraft record held by Airframes |
| `flight` | ~73% | The flight record held by Airframes |

## `message`

| Field | Present | Read by the code | Notes |
|---|---|---|---|
| `id` | always | yes | The Airframes message ID; also in the subject |
| `timestamp` | always | yes | Reception time (RFC 3339, UTC) |
| `label` | ~52% | yes | ACARS label |
| `text` | ~35% | yes | Message text |
| `tail` | ~50% | yes | Registration as transmitted |
| `flight` | ~35% | yes | Flight number as transmitted, sometimes space-padded |
| `frequency` | always | yes | MHz; `0.0` for satellite sources |
| `block_id` | ~50% | yes | ACARS block ID (digits are downlink, letters are uplink) |
| `link_direction` | ~11% | yes | `uplink` or `downlink` |
| `from_hex`, `to_hex` | ~72% | modelled, not used | ICAO addresses of the sender and recipient (VDL) |
| `level` | always | no | Signal level (dB) |
| `error` | always | no | Decoder error count |
| `channel` | always | no | Receiver channel |
| `mode` | ~52% | no | ACARS mode character |
| `ack` | ~52% | no | Acknowledgement character |
| `message_number` | ~34% | no | ACARS message sequence number (e.g. `M94`) |
| `block_end` | always | no | Whether this block ends the message |
| `source`, `source_type` | always | no | Decoder name, and one of `vdl`, `acars`, `hfdl`, `aero-acars`, `aero-adsc`, `iridium-acars` |
| `station_id`, `airframe_id`, `flight_id` | always | no | Airframes internal IDs |
| `latitude`, `longitude`, `altitude` | always | no | Aircraft position when the decoder supplies it, otherwise 0 |
| `ar_uuid`, `ar_version` | ~39% | no | Reporting agent ID and version |
| `created_at`, `updated_at` | always | no | Airframes record times |

## `airframe`

| Field | Present | Read by the code | Notes |
|---|---|---|---|
| `icao` | ~83% | yes | 24-bit ICAO address (hex) |
| `tail` | ~81% | yes | Registration |
| `manufacturer`, `manufacturer_model` | ~8% | yes | Aircraft type, when known |
| `owner` | ~10% | yes | Registered owner |
| `military` | ~85% | yes | |
| `id`, `airline_id` | ~85% / ~77% | no | Airframes internal IDs |
| `faa_ladd`, `faa_pia` | ~85% | no | FAA privacy programme flags |
| `created_at`, `updated_at` | ~85% | no | Airframes record times |

## `flight`

| Field | Present | Read by the code | Notes |
|---|---|---|---|
| `flight` | ~73% | yes | Flight number |
| `status` | ~73% | yes | e.g. `in-flight` |
| `latitude`, `longitude`, `altitude` | ~73% | yes | Last known position (0 when unknown) |
| `flight_iata` | ~73% | no | The IATA form of the flight number |
| `flight_icao` | ~34% | no | The ICAO form of the flight number (e.g. `MQ3782` and `ENY3782` for the same flight) |
| `track` | ~73% | no | Last known track |
| `id`, `uuid`, `airframe_id`, `airline_id` | ~66–73% | no | Airframes internal IDs |
| `messages_count`, `created_at`, `updated_at` | ~73% | no | Airframes record data |

## `messageReport`

Present on every message. Fields:
- `id` and `message_id` (the latter equals `message.id`)
- `first_to_report` (always true in the capture)
- `station_id`, `frequency` and `channel`
- the source's application, format, name, network protocol, protocol and type
- **`source_remote_ip`:** the feeding station's public IP address
- `created_at` and `updated_at`

## `source`

Present on every message. Fields:
- `application` and `applicationVersion`
- `dataProtocol` and `dataProtocolFormat`
- `name` and `networkProtocol`
- `transmissionType` and `transportType`

## `station`

| Field | Read by the code | Notes |
|---|---|---|
| `id`, `ident` | yes | Station ID and name (e.g. `XX-XXXX-VDL`) |
| `latitude`, `longitude` | yes | **The station's exact position** |
| `fuzzed_latitude`, `fuzzed_longitude` | no | A deliberately imprecise position |
| `ip_address` | no | **The station's public IP address** |
| `user_id` | no | **The Airframes user who operates the station** |
| `country_code`, `country_name`, `timezone` | no | |
| `type`, `source_type`, `source_application`, `source_protocol` | no | Station and decoder type |
| `status`, `messages_count`, `last_report_at`, `uuid`, `created_at`, `updated_at` | no | Airframes record data |

The fields in bold identify the volunteers who feed Airframes.
