# GOLD Document - Parser Implementation Guide

## Key Operational Information for Building a CPDLC/ADS-C Parser

---

## 1. Message Structure Overview

### CPDLC Message Components

```
┌─────────────────────────────────────────────────────────────┐
│                    CPDLC Message                             │
├─────────────────────────────────────────────────────────────┤
│  Message Identification Number (MIN)  │  0-63 integer       │
│  Message Reference Number (MRN)       │  Links to prior msg │
│  Message Element(s)                   │  UM/DM + parameters │
│  Response Attribute                   │  W/U, R, N, Y, A/N  │
│  Urgency Attribute                    │  Normal/Urgent/Dist │
│  Alert Attribute                      │  Low/Medium/High    │
└─────────────────────────────────────────────────────────────┘
```

### Message Identification

- **MIN (Message Identification Number)**: Integer 0-63, uniquely identifies messages per CPDLC connection
- **MRN (Message Reference Number)**: References a prior message to create dialogue chains

---

## 2. Response Attributes

| Code | Meaning | Valid Responses |
|------|---------|-----------------|
| **W/U** | WILCO or UNABLE required | DM0 (WILCO), DM1 (UNABLE), DM2 (STANDBY) |
| **A/N** | AFFIRM or NEGATIVE required | DM4 (AFFIRM), DM5 (NEGATIVE) |
| **R** | ROGER required | DM3 (ROGER) |
| **Y** | Yes - any response valid | Any appropriate DM message |
| **N** | No response required | None (message closes immediately) |
| **NE** | Not Enabled (FANS 1/A) | Flight crew cannot select standard responses |

### Response Precedence (when multi-element message)

1. UNABLE
2. STANDBY
3. WILCO/NEGATIVE
4. ROGER/AFFIRM

---

## 3. Data Link Systems

### FANS 1/A
- Defined by RTCA DO-258A/EUROCAE ED-100A
- Uses ACARS over VHF, SATCOM, or HF
- Supports CPDLC + ADS-C applications

### ATN B1
- Defined by RTCA DO-280B/EUROCAE ED-110B
- Includes Logical Acknowledgement (LACK) mechanism
- Context Management (CM) + CPDLC (ACM, ACL, AMC)

### FANS 1/A - ATN B1 (Interoperable)
- Ground systems supporting both aircraft types
- Message translation between formats

---

## 4. Variable Types in Messages

### Level Variables
```
[level]     - Flight level (FL350) or altitude (35000FT)
[altitude]  - FANS 1/A equivalent to [level]
```
**Note**: ICAO [level] can be single level OR vertical range. FANS 1/A treats [level] as single level only.

### Position Variables
```
[position]  - Waypoint identifier, lat/lon, bearing/distance
Format examples:
  - Named: "KATL", "EMFAB"
  - Lat/Lon: "N4523.5W12345.2" 
  - Bearing/Distance: "KATL090045" (090° at 45nm from KATL)
```

### Speed Variables
```
[speed]        - True airspeed or Mach number
[speed type]   - Indicates which speed type
  - True Airspeed
  - Indicated Airspeed  
  - Mach Number
  - Ground Speed
Format: "M082" (Mach 0.82) or "K450" (450 knots)
```

### Time Variables
```
[time]  - HHMM format in UTC (e.g., "1423")
```

### Other Variables
```
[frequency]           - Radio frequency (e.g., "128.325")
[unit name]           - ATS unit name (e.g., "OAKLAND CENTER")
[facility designation] - ICAO facility code (e.g., "KZOA")
[code]                - SSR transponder code (e.g., "1234")
[degrees]             - Heading/track in degrees (e.g., "275")
[vertical rate]       - Rate in feet per minute (e.g., "2000")
[specified distance]  - Distance in NM (e.g., "15")
[direction]           - LEFT or RIGHT
[route clearance]     - Full route string
[procedure name]      - Published procedure identifier
[free text]           - Unstructured text (max ~200 chars FANS 1/A)
[error information]   - Error code/description
```

---

## 5. ADS-C Encoding Details

### Reporting Interval Calculation
```python
def calculate_interval(byte_value):
    rate = byte_value & 0x3F  # bits 1-6 (0-63)
    sf_bits = (byte_value >> 6) & 0x03  # bits 7-8
    
    scaling_factors = {
        0b00: 0,    # Demand contract
        0b10: 1,    # 1 second
        0b01: 8,    # 8 seconds
        0b11: 64    # 64 seconds
    }
    
    sf = scaling_factors[sf_bits]
    return (1 + rate) * sf  # Returns seconds
```

### Time Stamp Format
- **FANS 1/A**: Seconds past the last hour (0-3599)
- Must derive full timestamp from context (hour/day/month/year)
- Invalid when FOM = 0

### Position Encoding
```
Latitude:  Decimal degrees (+ = North, - = South)
Longitude: Decimal degrees (+ = East, - = West)
Resolution: Varies by group (typically 1/128 degree ~ 0.5nm)
```

### Figure of Merit (FOM)
```python
FOM_ACCURACY = {
    0: None,      # Navigation lost
    1: 30,        # < 30 nm
    2: 15,        # < 15 nm
    3: 8,         # < 8 nm
    4: 4,         # < 4 nm
    5: 1,         # < 1 nm
    6: 0.25,      # < 0.25 nm
    7: 0.05       # < 0.05 nm (augmented GPS)
}
```

---

## 6. Message Assurance (MAS)

### FANS 1/A Message Assurance
```
Uplink Path:
  Ground → CSP → Aircraft
  Aircraft sends MAS ACK back to Ground

Downlink Path:
  Aircraft → CSP → Ground
  CSP sends ACK back to Aircraft
```

### ATN B1 Logical Acknowledgement (LACK)
- UM227 LOGICAL ACKNOWLEDGEMENT (ground → aircraft)
- DM100 LOGICAL ACKNOWLEDGEMENT (aircraft → ground)
- Used to confirm message receipt and display to human

---

## 7. Dialogue Management

### Opening a Dialogue
- Initial message contains at least one element requiring response
- MIN assigned by sender
- Message is "open" until closed

### Closing a Dialogue
- Send closure response (WILCO, UNABLE, ROGER, AFFIRM, NEGATIVE)
- Or receive response to all elements requiring response
- Use MRN to link response to original message

### Multi-Element Messages
```
Example: UM20 CLIMB TO FL350 + UM106 MAINTAIN M082
- Treated as single message
- Single response covers all elements
- If any element cannot be complied with → UNABLE for entire message
```

---

## 8. Error Handling

### Common Error Codes
| Code | Description |
|------|-------------|
| ERROR | Generic error detected |
| NOT CURRENT DATA AUTHORITY | Message from non-CDA ground system |
| NOT AUTHORIZED NEXT DATA AUTHORITY | Unauthorized NDA connection attempt |
| SERVICE UNAVAILABLE | Ground system doesn't support message |
| FLIGHT PLAN NOT HELD | No flight plan for aircraft |

### Timer Values (Typical)
| Timer | FANS 1/A | ATN B1 |
|-------|----------|--------|
| MAS Timeout | 60-120s | N/A |
| LACK Timeout | N/A | 60s |
| Uplink Expiration | Variable | Variable |

---

## 9. Protocol Flow Examples

### Basic Clearance Exchange
```
1. Ground → Aircraft: UM20 CLIMB TO FL350 (MIN=5)
2. Aircraft → Ground: MAS ACK (for MIN=5)
3. Aircraft → Ground: DM0 WILCO (MRN=5)
4. Ground → Aircraft: MAS ACK (if required)
```

### Request-Response Flow
```
1. Aircraft → Ground: DM6 REQUEST FL370 (MIN=12)
2. Ground → Aircraft: UM1 STANDBY (MRN=12)
3. Ground → Aircraft: UM20 CLIMB TO FL370 (MRN=12, MIN=15)
4. Aircraft → Ground: DM0 WILCO (MRN=15)
```

### ADS-C Contract Setup
```
1. Ground → Aircraft: Periodic Contract Request (interval=1920s, groups=[basic, predicted_route])
2. Aircraft → Ground: Contract Acknowledgement
3. Aircraft → Ground: First ADS-C Report
4. Aircraft → Ground: ADS-C Report (every 1920 seconds)
...
5. Ground → Aircraft: Cancel Contract
```

---

## 10. Implementation Notes

### Character Sets
- FANS 1/A: Limited character set (uppercase, digits, some symbols)
- Free text: ASCII subset, approximately 200 character limit for FANS 1/A

### Byte Ordering
- Network byte order (big-endian) for multi-byte values

### Validation Recommendations
1. Validate MIN uniqueness within connection
2. Check MRN references exist in dialogue history
3. Verify response type matches required response attribute
4. Validate variable ranges (levels, speeds, coordinates)
5. Check timestamp validity against current time
6. Verify FOM before using position data

### Common Parsing Pitfalls
1. **Block levels**: FANS 1/A uses separate messages (UM30-32), ATN B1 uses [level] with range
2. **Time stamp rollover**: Seconds past hour can appear from previous hour
3. **Free text**: May contain embedded field separators
4. **Position formats**: Multiple valid formats for same data
5. **Duplicate reports**: Same waypoint may generate multiple WCE reports

---

## 11. Reference Documents

- **ICAO Doc 4444**: PANS-ATM (defines message elements)
- **ICAO Doc 9880**: ATN Manual (technical encoding)
- **ICAO Doc 9869**: RCP/RSP Manual (performance requirements)
- **RTCA DO-258A / EUROCAE ED-100A**: FANS 1/A MASPS
- **RTCA DO-280B / EUROCAE ED-110B**: ATN B1 MASPS
- **RTCA DO-306 / EUROCAE ED-122**: Oceanic SPR Standard
- **RTCA DO-290 / EUROCAE ED-120**: Continental SPR Standard
