# Day in the Life of an Aircraft: Message Flow Analysis

This document describes the typical sequence of ACARS messages seen during a flight, based on analysis of real traffic.

## Example: Qatar Cargo B777F (A7-BFN) - 13 January 2026

This aircraft operated three cargo flight legs:
1. **QR8430**: Transatlantic → São Paulo (SBGR)
2. **QR8157**: São Paulo (SBGR) → Quito (SEQM)
3. **QR8158**: Quito (SEQM) → Panama (MPTO)

---

## Message Sequence by Flight Phase

### 1. En Route (Previous Flight Completing)

**00:05-03:30 UTC - Oceanic Crossing (QR8430)**

| Time | Type | Description |
|------|------|-------------|
| 00:05 | CPDLC | Position reports, clearance requests over Atlantic |
| 00:38 | CPDLC | Connection handoff: GVSC (Sal) → DKRC (Dakar) |
| 00:48 | Position | Waypoint AKEMO: N48.277, W0.499, FL370 |
| 01:25 | Position | Updated position approaching South America |
| 02:08 | **ATIS** | SBGR Arrival ATIS A: RWY 10R, Wind 150/06kt, QNH 1017 |
| 02:21 | CPDLC | Connection to RECO (Recife oceanic) |

**Key insight**: Aircraft receives destination ATIS ~2 hours before arrival to begin planning the approach.

---

### 2. Turnaround at São Paulo

**~04:00-08:30 UTC - Ground Operations**

| Time | Type | Description |
|------|------|-------------|
| 04:06 | ATIS | Updated arrival ATIS D (CAVOK conditions) |
| 05:44 | ATIS | ATIS F: RWY 10L closed for works |
| ~06:00 | - | *Aircraft lands and proceeds to cargo ramp* |
| 08:38 | **ATIS** | Departure ATIS K: RWY 28L/28R, Wind 330/08kt |

---

### 3. Pre-Departure (QR8157: SBGR → SEQM)

**08:39-09:07 UTC - Flight Preparation**

| Time | Type | Description |
|------|------|-------------|
| 08:39 | **Flight Plan** | Route uploaded: GERTU.UL304..EVRIV.UM775..BAGRE..ANRIR.UL655..LANCE..EPLAG.UM665..IQT.UM776..QIT |
| 08:43 | **Loadsheet PRELIM** | ZFW 147,743kg, TOW 184,143kg, Fuel 36,400kg |
| 08:58 | **PWI** | Predicted winds at FL430/400/380/360 for all waypoints |
| 08:58 | **Loadsheet FINAL** | Same weights, Edition 1 |
| 09:07 | **PDC** | Cleared to SEQM off 28R via XOLUS1A, Squawk 4453, Freq 126.900 |

**Loadsheet Details (QR8157)**:
```
LOADSHEET FINAL 0657 EDNO1
QR8157/13    13JAN26
GRU UIO A7-BFN 2/0          ← Flight, Date, Tail, Crew
TYPE B777F
ZFW 147743 MAX 248115 L     ← Zero Fuel Weight
TOF 36400                    ← Takeoff Fuel
TOW 184143 MAX 334978        ← Takeoff Weight
TIF 28200                    ← Trip Fuel
LAW 155943 MAX 260815        ← Landing Weight
MACZFW   26.2                ← MAC at ZFW
MACTOW   27.6                ← MAC at TOW
LOAD IN CPTS MD/0 1/0        ← Cargo by compartment
 2/1045 3/2900 4/2155 5/87
PREPARED BY Alexander Gamboa
NOTOC: YES                   ← Dangerous goods onboard
```

---

### 4. En Route (QR8157)

**~09:30-15:00 UTC - Flight**

| Time | Type | Description |
|------|------|-------------|
| 10:06 | **SIGMET** | Volcanic ash warning: Mt Reventador (near destination!) |
| - | CPDLC | Ongoing position reports and clearance requests |
| - | Position | Waypoint reports along route |

**SIGMET received**:
```
SIGMET 2 VALID 130915/131515 SEGU-
SEFG GUAYAQUIL FIR VA ERUPTION MT REVENTADOR
PSN S0004 W07739
VA CLD OBS AT 0810Z SFC/FL150 MOV NW 5KT
```

---

### 5. Next Flight Preparation (QR8158: SEQM → MPTO)

**16:34 UTC - Quito Operations**

| Time | Type | Description |
|------|------|-------------|
| 16:34 | **Loadsheet PRELIM** | ZFW 230,207kg, TOW 253,107kg - much heavier cargo load! |

**Loadsheet Details (QR8158)**:
```
LOADSHEET PRELIM 1134
QR8158/13    13JAN26
UIO PTY A7-BFN 3/0          ← 3 crew now (heavy cargo)
TYPE B777F
ZFW 230207 MAX 248115       ← Near max ZFW!
TOF 22900
TOW 253107 MAX 271403
TOTAL DEADLOAD 88566        ← 88 tonnes of cargo
LOAD IN CPTS MD/68900       ← 69t in main deck
 1/5058 2/5628 3/3040 4/5266 5/674
```

---

## Message Types Summary

| Phase | Message Types |
|-------|--------------|
| **Pre-departure** | ATIS (departure), Flight Plan, Loadsheet (prelim), PWI, Loadsheet (final), PDC |
| **Taxi/Takeoff** | OOOI (Out time), Position |
| **En Route** | CPDLC, Position reports, SIGMET/Weather, ATIS (destination) |
| **Approach** | ATIS updates, CPDLC descent clearances |
| **Landed** | OOOI (In time) |

---

## Data Capture Opportunities

Based on this analysis, we can capture:

### Pre-Flight
- **Route**: Origin, destination, waypoints, airways
- **Weight & Balance**: ZFW, TOW, fuel, cargo distribution, MAC
- **Clearance**: Runway, SID, squawk, frequencies
- **Winds**: Predicted winds at multiple flight levels for all waypoints

### En Route
- **Position**: Lat/lon, altitude, speed, heading
- **Weather**: SIGMETs, turbulence reports
- **ATC**: Clearance amendments, frequency changes

### Arrival
- **Weather**: Current ATIS, runway in use
- **Approach**: Expected approach type

---

## Parser Coverage

| Data Type | Parser | Status |
|-----------|--------|--------|
| Flight Plan | `h1` (FPN) | ✓ Captures route, waypoints |
| Loadsheet | `loadsheet` | ✓ Captures weights, cargo |
| PDC | `pdc` | ✓ Captures clearance, squawk |
| PWI | `h1` (PWI) | ✓ Captures winds by waypoint |
| ATIS | `atis` | ✓ Captures runway, weather |
| SIGMET | `weather` | ✓ Captures hazards |
| Position | Multiple | ✓ Various position formats |
| CPDLC | `cpdlc` | ✓ Clearances, reports |

---

## Appendix: Raw Message Examples

### Flight Plan
```
- #MDFPN/FNQTR8157/RI:DA:SBGR:AA:SEQM:F:GERTU.UL304..EVRIV.UM775..BAGRE,S18483W050480.UM775..ANRIR.UL655..LANCE,S06261W066204..EPLAG.UM665..IQT.UM776..QIT
```

### PDC
```
QTR8157 CLRD TO SEQM OFF 28R VIA
XOLUS1A/GERTU/F400/UL304 EVRIV UM775
BAGRE/N0470F430 UM775 ANRIR UL655 LANCE DCT
EPLAG UM665 IQT UM776 QIT DCT
SQUAWK 4453 ADT 0920 NEXT FREQ 126.900 ATIS K
```

### PWI (excerpt)
```
PWI/CB100020012.150002010.200003015.310288014.350270013/
WD430,GERTU,344003,430M62.OPNUS,313004,430M61.EVRIV,308007,430M62...
```
Format: `WD<FL>,<waypoint>,<wind_dir><wind_speed>,<FL><temp>.<next_waypoint>...`

---

## Example: Qantas A380 (VH-OQG) - 7-8 January 2026

Ultra-long-haul operations between Dallas and Sydney, showing oceanic CPDLC handoffs across the Pacific.

### QF008: Dallas (KDFW) → Sydney (YSSY)

**17-hour transpacific flight**

| Time (UTC) | Type | Description |
|------------|------|-------------|
| 01:11 | **Flight Plan** | Route via oceanic waypoints: KDFW..RZS.C1176..DINTY..oceanic..ABARB..MARLN5E5A |
| 01:12 | **ATIS** | KDFW Departure ATIS Z: Wind 320/07kt, RWY 36R/35L, QNH 2992 |
| 01:55 | ATIS | Updated ATIS A: Wind 330/10kt, QNH 2993 |
| 02:03 | CPDLC | Connection to USADCXA (US domestic) |
| 02:22 | **Loadsheet FINAL** | Captain Brown accepts. ZFW 334.5t, TOW 566.3t, Fuel 231.8t |
| ~02:30 | - | *Departure from Dallas* |
| 06:06 | CPDLC | Handoff to OAKODYA (Oakland Oceanic) |
| 06:08 | ATIS | PHNL (Honolulu) - checking alternate/overfly |
| 14:49 | CPDLC | Handoff to NANCDYA (Nadi Oceanic - Fiji) |
| 16:51 | CPDLC | Handoff to BNECAYA (Brisbane) |
| 18:59 | **PWI** | Predicted winds for approach |
| ~20:00 | - | *Arrival at Sydney* |

**Loadsheet Details (QF008)**:
```
FINAL LOADSHEET
REGO: VH-OQG
FLIGHT: QF008  06JAN DFW
4/22 crew                     ← 4 flight crew, 22 cabin crew
ZFW   334.5  tonnes
TOF   231.8  tonnes           ← 232 tonnes of fuel!
TOW   566.3  tonnes           ← Near A380 MTOW
TIF   216.9  tonnes           ← Trip fuel
LAW   349.3  tonnes
TOB   288    passengers
MACZFW  34.5
MACTOW  39.2
WING TANK    214513 kg
TRIM TANK     18329 kg
PREPARED BY NIKOL/DIDUR
CAPTAIN BROWN
```

### QF007: Sydney (YSSY) → Dallas (KDFW)

**Return flight, same aircraft**

| Time (UTC) | Type | Description |
|------------|------|-------------|
| 02:33 | **PDC** | Cleared via RWY 34L, RIC6 departure, route via B450 ABARB oceanic |
| 02:43 | **Flight Plan** | YSSY..NOBAR.B450..ABARB..oceanic..KDFW |
| 03:16 | **Loadsheet FINAL** | ZFW 349.9t, TOW 569.6t, **459 passengers!** |
| 03:51 | CPDLC | Connection to BNECAYA (Brisbane) |
| 04:11 | CPDLC | Handoff to NANCDYA (Nadi Oceanic) |
| 05:13 | CPDLC | Position reports en route |

**Key insight**: Return flight has 171 more passengers than outbound (459 vs 288), reflecting demand asymmetry.

---

## CPDLC Ground Station Handoffs

Oceanic flights show clear handoff patterns between FIRs:

### Pacific (Australia-Americas)
```
BNECAYA (Brisbane) → NANCDYA (Nadi) → OAKODYA (Oakland) → USADCXA (US domestic)
```

### Pacific (Australia-Asia)
```
BNECAYA (Brisbane) → POMCAYA (Port Moresby) → FUKJJYA (Fukuoka)
```

### Atlantic (Europe-Americas)
```
GVSC (Sal) → DKRC (Dakar) → RECO (Recife)
```

Each handoff generates:
1. **CR1** - Connection Request from aircraft
2. **AT1** - Logon/Current Data Authority acknowledgment
3. Subsequent position reports and clearances

---

## Example: Jetstar A320 (VH-VFD) - Domestic Sydney-Melbourne

Short-haul domestic flights have a simpler message flow, typically just PWI and PDC.

### JST501: Sydney (YSSY) → Melbourne (YMML)

**~1 hour domestic flight**

| Time (UTC) | Type | Description |
|------------|------|-------------|
| 18:20 | **PWI** | Winds at FL380/360/340/320 for all waypoints |
| 18:20 | **PDC** | Cleared via RWY 16R, GROOK1 departure, Squawk 4302 |
| ~19:00 | - | *Departure from Sydney* |
| ~20:00 | - | *Arrival at Melbourne* |

**PWI Details (JST501)**:
```
PWI/WD380,WOL,188040,380M57.LEECE,180040,380M58.TANTA,180043,380M58.
ANLID,180042,380M58.RUMIE,180040,380M58.NABBA,182037,380M58.
BULLA,187033,380M58.LUVAS,190033,380M58.BOOIN,191031,380M58
```
Format: `WD<FL>,<waypoint>,<wind_dir><wind_speed>,<FL><temp>`

Waypoints: WOL → LEECE → TANTA → ANLID → RUMIE → NABBA → BULLA → LUVAS → BOOIN

**PDC Details (JST501)**:
```
PDC 311819
JST501 A320 YSSY 1900
CLEARED TO YMML VIA
16R GROOK1 DEP WOL:TRAN
ROUTE:DCT WOL H65 LEECE Q29 BOOIN DCT
CLIMB VIA SID TO: 5000
DEP FREQ: 129.700
SQUAWK 4302
```

**Key differences from long-haul:**
- No CPDLC (domestic airspace uses VHF)
- No ADS-C position reports
- No ATIS uplink (pilots get via voice/D-ATIS)
- No loadsheet (handled by ground systems)
- Simpler routing via domestic airways (H65, Q29)