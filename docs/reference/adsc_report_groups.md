# ADS-C Report Group Definitions

## Overview

ADS-C (Automatic Dependent Surveillance - Contract) uses various systems on board the aircraft to automatically provide aircraft position, altitude, speed, intent and meteorological data. Reports are generated in response to an ADS contract requested by the ground system.

## Contract Types

### 1. Periodic Contract
- Specifies time interval at which reports are sent (range: 64-4096 seconds)
- Specifies optional ADS-C groups to include with each report
- Each optional group may have a unique modulus (e.g., modulus of 5 = every 5th report)
- Only one periodic contract per ATSU per aircraft

### 2. Demand Contract
- Requests a single ADS-C periodic report
- Does not cancel or modify other ADS contracts

### 3. Event Contract
- Requests reports when specific events occur
- One event contract per ATSU, but can contain multiple event types
- Event types:
  - **Waypoint Change Event (WCE)**: Triggered when Next or Next+1 waypoint changes
  - **Level Range Deviation Event (LRDE)**: Triggered when aircraft exceeds defined level range
  - **Lateral Deviation Event (LDE)**: Triggered when lateral deviation exceeds threshold
  - **Vertical Rate Change Event (VRE)**: Triggered when climb/descent rate exceeds threshold

---

## ADS-C Report Groups

### 1. Basic Group (Always Included)

| Field | Description |
|-------|-------------|
| Latitude | Present position latitude |
| Longitude | Present position longitude |
| Altitude | Current flight level/altitude |
| Time Stamp | Seconds past the last hour |
| Figure of Merit (FOM) | Navigation accuracy indicator (0-7) |
| Redundancy Indicator | Navigation system redundancy status |

**Figure of Merit Values:**
| FOM | Accuracy | Remarks |
|-----|----------|---------|
| 0 | Loss of nav | Unable to determine position within 30 NM |
| 1 | < 30 nm | Long flight inertial nav without updates |
| 2 | < 15 nm | Intermediate flight inertial nav |
| 3 | < 8 nm | Short flight or beyond 50 nm from VOR |
| 4 | < 4 nm | VOR at 50 nm or less; GPS worldwide |
| 5 | < 1 nm | DME RNAV or multiple DME/GPS updates |
| 6 | < 0.25 nm | RNAV using GPS |
| 7 | < 0.05 nm | Augmented GPS |

---

### 2. Flight Identification Group

| Field | Description |
|-------|-------------|
| Aircraft Identification | ICAO aircraft identification (call sign) |

---

### 3. Earth Reference Group

| Field | Description |
|-------|-------------|
| True Track | Aircraft's true track over ground (degrees) |
| Ground Speed | Aircraft's ground speed |
| Vertical Rate | Rate of climb or descent |

---

### 4. Air Reference Group

| Field | Description |
|-------|-------------|
| Mach Number | True Mach number |
| True Heading | Aircraft's true heading |

---

### 5. Airframe Identification Group

| Field | Description |
|-------|-------------|
| Aircraft Address | 24-bit ICAO aircraft address |

---

### 6. Meteorological Group

| Field | Description |
|-------|-------------|
| Wind Speed | Speed of wind at aircraft position |
| Wind Direction | True direction from which wind is blowing |
| Temperature | Static air temperature |
| Turbulence (optional) | Current turbulence level |

---

### 7. Predicted Route Group

| Field | Description |
|-------|-------------|
| Next Waypoint Latitude | Latitude of next waypoint |
| Next Waypoint Longitude | Longitude of next waypoint |
| Next Waypoint Altitude | Predicted altitude at next waypoint |
| Next Waypoint ETA | Estimated time interval to next waypoint (seconds from timestamp) |
| Next+1 Waypoint Latitude | Latitude of waypoint after next |
| Next+1 Waypoint Longitude | Longitude of waypoint after next |
| Next+1 Waypoint Altitude | Predicted altitude at next+1 waypoint |

---

### 8. Fixed Projected Intent Group

| Field | Description |
|-------|-------------|
| Projected Position Latitude | Latitude of fixed projected point |
| Projected Position Longitude | Longitude of fixed projected point |
| Projected Altitude | Predicted altitude at projected point |
| Projected Time | Time interval to projected point (seconds from timestamp) |

---

### 9. Intermediate Projected Intent Group

Up to 10 points can be included, where each point:
- Is between current position and fixed projected point
- Is associated with a planned speed, altitude, or route change
- May include FMS-generated points (e.g., top of descent)

| Field | Description |
|-------|-------------|
| Bearing from Present Position | Direction to intermediate point |
| Distance from Present Position | Distance to intermediate point |
| Projected Altitude | Predicted altitude at point |
| Projected Time | Time interval to point (seconds) |

---

## Event Report Contents

### Waypoint Change Event (WCE) Report Contains:
- Basic group
- Predicted route group

### Level Range Deviation Event (LRDE) Report Contains:
- Basic group only

### Lateral Deviation Event (LDE) Report Contains:
- Basic group only

### Vertical Rate Change Event (VRE) Report Contains:
- Basic group
- Earth reference group

---

## ADS-C Emergency Reports

- Tagged as "emergency" reports for ATC highlighting
- Triggered by:
  - Manual selection of ADS-C emergency function
  - Triggering another emergency alerting system
  - Covert activation
- Continue until flight crew de-selects emergency function
- "Cancel ADS-C emergency" sent with next periodic report

---

## Reporting Interval Calculation

The reporting interval is calculated as:

```
Reporting Interval = (1 + Rate) × SF
```

Where:
- **Rate**: 6-bit value (0-63)
- **SF (Scaling Factor)**:
  - 00 = 0 seconds (demand contract)
  - 10 = 1 second
  - 01 = 8 seconds
  - 11 = 64 seconds

**Example**: 40-minute interval = (1 + 36) × 64 = 2368 seconds

---

## Key Notes

1. Positional information does NOT contain waypoint names
2. Time stamp expressed as seconds past the last hour
3. Estimates expressed as time intervals (seconds) from basic group timestamp
4. Maximum 5 ground systems can have contracts with a single aircraft
5. Minimum periodic interval for FANS 1/A: 64 seconds
6. Default periodic interval: 64 seconds (emergency), 304 seconds (normal)
