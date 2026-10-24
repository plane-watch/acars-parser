package storage

import (
	"context"
	"time"

	"acars_parser/internal/flightroutes"
)

// flightAliasesSchema creates the flight_aliases table and the
// airline_codes view (see flightroutes). It is part of CreateSchema.
const flightAliasesSchema = `
	-- A flight named in both forms by a message: the IATA flight number
	-- ("QF1") and the ICAO callsign ("QFA1"), without leading zeros.
	CREATE TABLE IF NOT EXISTS flight_aliases (
		iata_flight         TEXT NOT NULL,
		icao_callsign       TEXT NOT NULL,
		observation_count   INTEGER NOT NULL DEFAULT 1,
		first_seen          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_seen           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (iata_flight, icao_callsign)
	);

	CREATE INDEX IF NOT EXISTS idx_flight_aliases_icao ON flight_aliases(icao_callsign);

	-- Airline code pairs, from the flight aliases: the IATA and ICAO codes,
	-- the observations, and how many had the same flight number and suffix
	-- in both forms.
	CREATE OR REPLACE VIEW airline_codes AS
	SELECT substring(iata_flight from '^[A-Z0-9]{2}') AS iata,
	       substring(icao_callsign from '^[A-Z]{3}') AS icao,
	       sum(observation_count)::integer AS observations,
	       coalesce(sum(observation_count) FILTER (
	           WHERE substring(iata_flight from 3) = substring(icao_callsign from 4)), 0)::integer AS same_number
	FROM flight_aliases
	GROUP BY 1, 2;
`

// UpsertFlightAlias records an observation of a flight named in both forms.
func (d *PostgresDB) UpsertFlightAlias(ctx context.Context, iataFlight, icaoCallsign string, seen time.Time) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO flight_aliases (iata_flight, icao_callsign, first_seen, last_seen)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (iata_flight, icao_callsign) DO UPDATE SET
			observation_count = flight_aliases.observation_count + 1,
			first_seen = LEAST(flight_aliases.first_seen, EXCLUDED.first_seen),
			last_seen = GREATEST(flight_aliases.last_seen, EXCLUDED.last_seen)
	`, iataFlight, icaoCallsign, seen)
	return err
}

// AirlinePairs returns the airline code pairs (flightroutes.Store).
func (d *PostgresDB) AirlinePairs(ctx context.Context) ([]flightroutes.Pair, error) {
	rows, err := d.pool.Query(ctx, `SELECT iata, icao, observations, same_number FROM airline_codes
		WHERE iata IS NOT NULL AND icao IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []flightroutes.Pair
	for rows.Next() {
		var p flightroutes.Pair
		if err := rows.Scan(&p.IATA, &p.ICAO, &p.Observations, &p.SameNumber); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// FlightAliases returns the flight aliases whose IATA flight or ICAO
// callsign is one of ids (flightroutes.Store).
func (d *PostgresDB) FlightAliases(ctx context.Context, ids []string) ([]flightroutes.Alias, error) {
	rows, err := d.pool.Query(ctx, `SELECT iata_flight, icao_callsign, observation_count FROM flight_aliases
		WHERE iata_flight = ANY($1) OR icao_callsign = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []flightroutes.Alias
	for rows.Next() {
		var a flightroutes.Alias
		if err := rows.Scan(&a.IATA, &a.ICAO, &a.Observations); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Routes returns the routes stored under the given flight identifiers
// (flightroutes.Store).
func (d *PostgresDB) Routes(ctx context.Context, flights []string) ([]flightroutes.Route, error) {
	rows, err := d.pool.Query(ctx, `SELECT flight_pattern, origin_icao, dest_icao, observation_count, first_seen, last_seen
		FROM routes WHERE flight_pattern = ANY($1)`, flights)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []flightroutes.Route
	for rows.Next() {
		var r flightroutes.Route
		if err := rows.Scan(&r.Flight, &r.Origin, &r.Destination, &r.Observations, &r.FirstSeen, &r.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
