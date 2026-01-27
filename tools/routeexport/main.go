// Package main provides a tool to export routes from the PostgreSQL database to CSV format.
// The output is compatible with the planewatch-atc import_routes.rake task, which expects:
// callsign,ICAO1,ICAO2,ICAO3,...
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"sort"

	"acars_parser/internal/storage"
)

// RouteExport represents a flight route with its airport sequence.
type RouteExport struct {
	FlightPattern string
	Airports      []string // Ordered list of ICAO codes (origin, intermediate stops, destination).
}

func main() {
	// PostgreSQL connection flags.
	pgHost := flag.String("pg-host", "localhost", "PostgreSQL host")
	pgPort := flag.Int("pg-port", 5432, "PostgreSQL port")
	pgUser := flag.String("pg-user", "acars", "PostgreSQL user")
	pgPassword := flag.String("pg-password", "", "PostgreSQL password")
	pgDB := flag.String("pg-db", "acars", "PostgreSQL database")

	output := flag.String("output", "", "Output CSV file (default: stdout)")
	minObservations := flag.Int("min-obs", 1, "Minimum observation count to include a route")
	showStats := flag.Bool("stats", false, "Show statistics only, don't export")
	verbose := flag.Bool("v", false, "Verbose output")

	flag.Parse()

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

	// Show stats mode.
	if *showStats {
		showRouteStats(ctx, pg)
		return
	}

	// Query routes.
	routes, err := getRoutes(ctx, pg, *minObservations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying routes: %v\n", err)
		os.Exit(1)
	}

	if len(routes) == 0 {
		fmt.Fprintf(os.Stderr, "No routes found matching criteria\n")
		os.Exit(0)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Exporting %d routes to CSV\n", len(routes))
	}

	// Write output.
	var writer *csv.Writer
	if *output != "" {
		file, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = file.Close() }()
		writer = csv.NewWriter(file)
	} else {
		writer = csv.NewWriter(os.Stdout)
	}

	// Write CSV rows (no header, as the rake task expects headers: false).
	for _, route := range routes {
		// Build row: callsign followed by all airport ICAO codes.
		row := make([]string, 0, 1+len(route.Airports))
		row = append(row, route.FlightPattern)
		row = append(row, route.Airports...)

		if err := writer.Write(row); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing row: %v\n", err)
			os.Exit(1)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		fmt.Fprintf(os.Stderr, "Error flushing CSV: %v\n", err)
		os.Exit(1)
	}

	if *verbose && *output != "" {
		fmt.Fprintf(os.Stderr, "Wrote %d routes to %s\n", len(routes), *output)
	}
}

// getRoutes retrieves routes from the database with the specified minimum observation count.
// It reconstructs the airport sequence from the route_legs table.
func getRoutes(ctx context.Context, pg *storage.PostgresDB, minObservations int) ([]RouteExport, error) {
	// Query all routes meeting the observation threshold.
	dbRoutes, err := pg.ListRoutes(ctx, minObservations)
	if err != nil {
		return nil, fmt.Errorf("querying routes: %w", err)
	}

	if len(dbRoutes) == 0 {
		return nil, nil
	}

	// Build routes with airport sequences.
	routes := make([]RouteExport, 0, len(dbRoutes))
	for _, r := range dbRoutes {
		legs, err := pg.GetRouteLegs(ctx, r.ID)
		if err != nil {
			continue
		}
		airports := buildAirportSequence(legs)

		// Skip routes with fewer than 2 airports (the rake task requires at least 2).
		if len(airports) < 2 {
			continue
		}

		routes = append(routes, RouteExport{
			FlightPattern: r.FlightPattern,
			Airports:      airports,
		})
	}

	return routes, nil
}

// buildAirportSequence constructs the ordered list of airports from route legs.
// For a route A→B→C, the legs are: (1: A→B), (2: B→C).
// The result is: [A, B, C].
func buildAirportSequence(legs []storage.RouteLeg) []string {
	if len(legs) == 0 {
		return nil
	}

	// Sort legs by sequence (should already be ordered, but ensure it).
	sort.Slice(legs, func(i, j int) bool {
		return legs[i].Sequence < legs[j].Sequence
	})

	// Build airport sequence: first leg's origin, then each leg's destination.
	airports := make([]string, 0, len(legs)+1)
	airports = append(airports, legs[0].OriginICAO)
	for _, leg := range legs {
		airports = append(airports, leg.DestICAO)
	}

	return airports
}

// showRouteStats displays statistics about the routes in the database.
func showRouteStats(ctx context.Context, pg *storage.PostgresDB) {
	pool := pg.Pool()

	var total int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM routes").Scan(&total)

	var multiStop int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM routes WHERE is_multi_stop = TRUE").Scan(&multiStop)

	var avgObs float64
	_ = pool.QueryRow(ctx, "SELECT COALESCE(AVG(observation_count), 0) FROM routes").Scan(&avgObs)

	var maxObs int
	var maxPattern string
	_ = pool.QueryRow(ctx, "SELECT flight_pattern, observation_count FROM routes ORDER BY observation_count DESC LIMIT 1").Scan(&maxPattern, &maxObs)

	fmt.Println("Route Statistics")
	fmt.Println("────────────────")
	fmt.Printf("Total routes:        %d\n", total)
	fmt.Printf("Multi-stop routes:   %d\n", multiStop)
	fmt.Printf("Average observations: %.1f\n", avgObs)
	if maxPattern != "" {
		fmt.Printf("Most observed:       %s (%d observations)\n", maxPattern, maxObs)
	}

	// Observation count distribution.
	fmt.Println("\nObservation Count Distribution:")
	rows, err := pool.Query(ctx, `
		SELECT
			CASE
				WHEN observation_count = 1 THEN '1'
				WHEN observation_count <= 5 THEN '2-5'
				WHEN observation_count <= 10 THEN '6-10'
				WHEN observation_count <= 50 THEN '11-50'
				ELSE '50+'
			END as bucket,
			COUNT(*) as cnt
		FROM routes
		GROUP BY bucket
		ORDER BY MIN(observation_count)
	`)
	if err == nil {
		defer rows.Close()
		fmt.Printf("%-15s %10s\n", "Observations", "Count")
		for rows.Next() {
			var bucket string
			var cnt int
			_ = rows.Scan(&bucket, &cnt)
			fmt.Printf("%-15s %10d\n", bucket, cnt)
		}
	}

	// Top 10 most common flight patterns.
	fmt.Println("\nTop 10 Most Observed Routes:")
	topRows, err := pool.Query(ctx, `
		SELECT flight_pattern, origin_icao, dest_icao, observation_count, is_multi_stop
		FROM routes
		ORDER BY observation_count DESC
		LIMIT 10
	`)
	if err == nil {
		defer topRows.Close()
		fmt.Printf("%-12s %-6s %-6s %6s %s\n", "Flight", "Origin", "Dest", "Obs", "Multi")
		for topRows.Next() {
			var pattern, origin, dest string
			var obs int
			var isMultiStop bool
			_ = topRows.Scan(&pattern, &origin, &dest, &obs, &isMultiStop)
			multi := ""
			if isMultiStop {
				multi = "Yes"
			}
			fmt.Printf("%-12s %-6s %-6s %6d %s\n", pattern, origin, dest, obs, multi)
		}
	}
}
