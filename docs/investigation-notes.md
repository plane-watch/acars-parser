# Investigation Notes

Working notes on message patterns that have been investigated. Useful to avoid re-investigating patterns that have already been assessed.

## Unparsed Message Patterns - Not Worth Parsing

These patterns have been investigated and determined to contain no useful extractable data:

### Request/Acknowledgement Messages (no payload)
| Label | Pattern | Count | Notes |
|-------|---------|-------|-------|
| H1 | `- #MDREQPOS037B` | ~400k | Position request with checksum |
| H1 | `- #MDREQPRGC74C` | ~128k | Program request |
| H1 | `- #MDREQPRG,DT.PR...` | ~92k | Program request with params |
| H1 | `- #MDREQPER...` | ~34k | Performance request |
| H1 | `- #M1REQPRG...` | ~44k | Program request variant |
| H1 | `- #M1REQPOS...` | ~15k | Position request variant |
| H1 | `- #EIEM01R0` to `- #EIEM19R0` | ~150k total | EIEM protocol requests |
| H1 | `RESREQ/AK,...` | ~76k | Response acknowledgements |
| 15 | `REQPOS` | ~163k | Position request (no data) |
| 3P | `REQPOS037B` | ~28k | Position request variant |
| 2F | `PACEPOSREQ` | ~26k | PACE position request |
| 40 | `POS` | ~17k | Position acknowledgement only |
| 4C | `-FSQ CA01` | ~29k | Frequency squitter ack |

### Short/Empty Messages
| Label | Pattern | Count | Notes |
|-------|---------|-------|-------|
| SQ | `00XS` | ~122k | Short squitter, no position data |
| H1 | `0000000000...` | ~18k | Null/padding messages |
| 26 | `-` | ~9k | Single character |

### Notification/Status Messages (no actionable data)
| Label | Pattern | Count | Notes |
|-------|---------|-------|-------|
| 10, 11 | `New wind data now available` | ~3.8k | Just a notification to request wind |
| RA | `LOADCONTROL MESSAGE: FUEL FIGURES MESSAGE RECEIVED` | ~1.6k | Ack only |
| RA | `FMS WEIGHT DATA NOT UPDATED` | ~1.2k | Error message, no data |
| RA | `CI OPTIMIZER...TEST GROUP` | ~600 | Info message only |
| RA | `WIND RQST RCVD` | ~900 | Ack only |
| 31 | `*MESSAGE RECEIVED*` | ~850 | Generic ack |

## Patterns Worth Investigating

### CFIM Messages (Korean Air specific)
- ~4800 messages, exclusively Korean Air aircraft (HL-prefix)
- Label H1, sublabel CFD
- Address: `.BLVBOCR` (Korean SITA)
- Two observed formats:
  1. `#CFIM-L/VAR/D6620/20:R/VAR/D6720/20` - lateral offset data (L=Left, R=Right, VAR=Variable?)
  2. `#CFIM-E/35/221/10/E` - unknown structure (varies: 35/72/102 first field, 221/270/351 second)
- Not in libacars, no public documentation found
- Low priority given single-airline scope and small volume

### MIAM Encoded Messages (MA label)
- ~665k messages with MIAM encoding (T.0 prefix = Base85 + deflate)
- Some have `T-` prefix meaning uncompressed plain text
- A350 position reports visible in some: `REP081/POSDMU`
- Would need MIAM decoder implementation (reference: libacars/miam-core.c)

### A350 Position Reports
Example: `A350,000323,1,1,TB000000/REP081,01,01;A06/NX,KJFK WSSS/0,7,+069284,+329581,...`
- Contains origin/destination, coordinates, altitude, temperature
- Format needs more analysis to understand coordinate encoding

## Reference: AVICOM Japan
- VHF ground station network for domestic CPDLC
- All stations use 136.975 MHz
- Station IDs: YGJV (Yonago), IZOV (Izumo), KCZV (Kochi), FUKV (Fukuoka), etc.
- Message format: `02J` + type + IATA(3) + ICAO(4) + coords + freq + `/AVICOM`