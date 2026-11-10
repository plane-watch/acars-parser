// Package main exports routes from the PostgreSQL database to CSV for the
// planewatch-atc routes:import rake task (lib/tasks/import_routes.rake),
// which reads lines of callsign,ICAO1,ICAO2,... with no header.
//
// The rake task takes the operator from the callsign's first three
// characters and upserts one route per callsign, so each line is an ICAO
// callsign, once. The stored routes are merged by flight as the routes API
// merges them (see flightroutes.Group): a flight's routes stored under its
// IATA flight number ("QF1", "NW1368") are exported under its ICAO callsign
// ("QFA1", "DAL1368") when the messages prove the equivalence. A flight
// whose callsign is not known is not exported.
//
// A callsign gets its most observed route only when that route accounts for
// at least -min-share of the flight's observations: a callsign flown on
// several routes (both directions, or different legs on different days)
// has no single route, and is not exported rather than given one of them.
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"

	"acars_parser/internal/flightroutes"
	"acars_parser/internal/storage"
)

// exportRow is one CSV line: a callsign and its route's airports.
type exportRow struct {
	Callsign string
	Airports []string // Origin, then destination.
}

// selectStats counts the flights by what selectRoutes did with them.
type selectStats struct {
	Flights    int // Flights considered.
	Exported   int
	NoCallsign int // The ICAO callsign is not known.
	TooFew     int // The most observed route has fewer than the minimum observations.
	Ambiguous  int // No route has the minimum share of the observations.
}

func main() {
	pgHost := flag.String("pg-host", "localhost", "PostgreSQL host")
	pgPort := flag.Int("pg-port", 5432, "PostgreSQL port")
	pgUser := flag.String("pg-user", "acars", "PostgreSQL user")
	pgPassword := flag.String("pg-password", "", "PostgreSQL password")
	pgDB := flag.String("pg-db", "acars", "PostgreSQL database")

	output := flag.String("output", "", "Output CSV file (default: stdout)")
	minObservations := flag.Int("min-obs", 1, "Minimum observations of a callsign's route")
	minShare := flag.Float64("min-share", 0.8, "Minimum share of a callsign's observations that its route must have")
	showStats := flag.Bool("stats", false, "Show what would be exported, without exporting")
	verbose := flag.Bool("v", false, "Verbose output")
	flag.Parse()

	if !validShare(*minShare) {
		fmt.Fprintln(os.Stderr, "-min-share must be above 0.5 and at most 1")
		os.Exit(2)
	}

	ctx := context.Background()
	pg, err := storage.OpenPostgres(ctx, storage.PostgresConfig{
		Host:     *pgHost,
		Port:     *pgPort,
		Database: *pgDB,
		User:     *pgUser,
		Password: *pgPassword,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	defer pg.Close()

	flights, err := groupedFlights(ctx, pg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading routes: %v\n", err)
		os.Exit(1)
	}
	rows, stats := selectRoutes(flights, *minObservations, *minShare)

	if *showStats || *verbose {
		printStats(os.Stderr, stats)
	}
	if *showStats {
		return
	}

	out := io.Writer(os.Stdout)
	if *output != "" {
		file, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = file.Close() }()
		out = file
	}
	if err := writeCSV(out, rows); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
		os.Exit(1)
	}
	if *verbose && *output != "" {
		fmt.Fprintf(os.Stderr, "Wrote %d routes to %s\n", len(rows), *output)
	}
}

// validShare reports whether a minimum share is above a half, so that at
// most one route qualifies, and at most 1. NaN is not valid: every
// comparison with it is false, so selectRoutes would accept any route.
func validShare(share float64) bool {
	return share > 0.5 && share <= 1
}

// groupedFlights reads every route, alias and airline code pair, and
// merges the routes by flight.
func groupedFlights(ctx context.Context, pg *storage.PostgresDB) ([]flightroutes.Flight, error) {
	routes, err := pg.AllRoutes(ctx)
	if err != nil {
		return nil, fmt.Errorf("routes: %w", err)
	}
	aliases, err := pg.AllFlightAliases(ctx)
	if err != nil {
		return nil, fmt.Errorf("flight aliases: %w", err)
	}
	pairs, err := pg.AirlinePairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("airline code pairs: %w", err)
	}
	return flightroutes.Group(routes, aliases, pairs), nil
}

// selectRoutes returns the route of each flight that has a known callsign
// and a route with at least minObservations and minShare of the flight's
// observations, in the flights' order. minShare must be above a half, so
// that at most one route qualifies.
func selectRoutes(flights []flightroutes.Flight, minObservations int, minShare float64) ([]exportRow, selectStats) {
	var rows []exportRow
	stats := selectStats{Flights: len(flights)}
	for _, f := range flights {
		if f.Callsign == "" {
			stats.NoCallsign++
			continue
		}
		if len(f.Routes) == 0 {
			continue // Group gives every flight a route; this is defensive.
		}
		total := 0
		for _, r := range f.Routes {
			total += r.Observations
		}
		top := f.Routes[0] // The routes are sorted by observations.
		switch {
		case top.Observations < minObservations:
			stats.TooFew++
		case float64(top.Observations) < minShare*float64(total):
			stats.Ambiguous++
		default:
			stats.Exported++
			rows = append(rows, exportRow{Callsign: f.Callsign, Airports: []string{top.Origin, top.Destination}})
		}
	}
	return rows, stats
}

// writeCSV writes the rows without a header, as the rake task reads them.
func writeCSV(w io.Writer, rows []exportRow) error {
	cw := csv.NewWriter(w)
	for _, r := range rows {
		if err := cw.Write(append([]string{r.Callsign}, r.Airports...)); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// printStats prints what selectRoutes did with the flights.
func printStats(w io.Writer, s selectStats) {
	_, _ = fmt.Fprintf(w, "Flights:               %d\n", s.Flights)
	_, _ = fmt.Fprintf(w, "Exported:              %d\n", s.Exported)
	_, _ = fmt.Fprintf(w, "Callsign not known:    %d\n", s.NoCallsign)
	_, _ = fmt.Fprintf(w, "Too few observations:  %d\n", s.TooFew)
	_, _ = fmt.Fprintf(w, "No dominant route:     %d\n", s.Ambiguous)
}
