# Flight routes by callsign or flight number

Routes are stored in PostgreSQL `routes` under the flight identifier the messages gave, in either form: the IATA flight number (`QF1`, `B61` for B6 flight 1) or the ICAO callsign (`QFA1`), without leading zeros. One flight can therefore have routes under both. `internal/flightroutes` looks a flight up by either form and merges its routes under the ICAO callsign, which is unique where IATA codes are reused. The enrichment API serves it at `GET /api/v1/routes/{flight}` (see [enrichment-api.md](enrichment-api.md)).

## Learning the equivalences

A message often names its flight both ways: the transmitted flight (in the ACARS header, usually the IATA form) and a parser result's flight or callsign in the other. The extractor records such a pair (`ExtractedData.FlightAlias`), and `live` and `backfill` count it in the `flight_aliases` table. Some airlines' callsigns are not their flight numbers (LO495 flies as LOT49H), but a report can also be about an adjacent leg (ELY315 reported while LY316 was transmitted). So a pair whose numbers differ is recorded only from a result about the flight in progress: a departure clearance, an FMS progress report or a flight plan. A pair with the same number and suffix is recorded from any result. The `airline_codes` view sums the aliases per airline code pair (QF and QFA) and counts how many had the same number and suffix in both forms.

In October 2026, 208,124 messages named their flight both ways, for 500 airline code pairs; 97.6% had the same number in both forms. Most airlines' pairs always agree (UA and UAL, QR and QTR); some do not (LO and LOT 52%, FI and ICE 65%, DL and DAL 91%). Delta's header uses NW as well as DL.

## Using them

An equivalence is used only on enough evidence:

- **A flight alias** (an IATA flight and a callsign) is used when it was seen at least twice and is at least 60% of that IATA flight's aliases. Some reports are about an adjacent leg (ELY315 sent while transmitting LY316), which gives a few wrong aliases.
- **An airline code pair** converts flight numbers (QF1 to QFA1) when it was seen at least 20 times, had the same number at least 95% of the time, and is at least 90% of the IATA code's observations, and no other ICAO code was paired with the IATA code at least 5 times, so that an IATA code used by two airlines is not converted.

A lookup:

1. finds the flight's ICAO callsign: the queried callsign itself; for an IATA flight, the callsign of its alias, or else the one its airline's code pair converts it to;
2. finds the callsign's identifiers: the callsign, the IATA flights aliased to it (judged among all of each IATA flight's aliases), and the IATA flights its code pairs convert it to, unless aliased to another callsign (QF9 is QFA9X, so QFA9 is not QF9). An airline may have several IATA codes (DL and NW for DAL);
3. merges the identifiers' routes by origin and destination, summing the observations, and lists the identifiers each route was observed as.

A flight whose callsign is not known (an IATA code without a usable pair, and no alias) is returned under its own identifier, with its own routes only and no callsign.

The stored routes are not changed: the merge is made when routes are read, so an equivalence learned wrongly can be corrected without rewriting data.
