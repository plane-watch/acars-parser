// Package main provides the enrichment-api server for flight enrichment data.
//
// This is a standalone REST API server that provides access to flight enrichment
// data stored in PostgreSQL. It's designed to be queried by ADS-B tracking
// applications to enrich aircraft positions with route, destination, and other
// operational data extracted from ACARS messages.
//
// Usage:
//
//	enrichment-api [options]
//
// Options:
//
//	-pg-host HOST       PostgreSQL host (default: localhost, env: POSTGRES_HOST)
//	-pg-port PORT       PostgreSQL port (default: 5432, env: POSTGRES_PORT)
//	-pg-database DB     PostgreSQL database (default: acars_state, env: POSTGRES_DATABASE)
//	-pg-user USER       PostgreSQL user (default: acars, env: POSTGRES_USER)
//	-pg-password PASS   PostgreSQL password (default: acars, env: POSTGRES_PASSWORD)
//	-port N             HTTP port (default: 8081)
//	-auth               Enable API key authentication
//	-api-keys KEYS      Comma-separated list of valid API keys
//
// API Endpoints:
//
//	GET /api/v1/health
//	    Health check endpoint.
//
//	GET /api/v1/enrichment/{icao_hex}
//	    Get all enrichments for an aircraft on today's date.
//
//	GET /api/v1/enrichment/{icao_hex}/{callsign}
//	    Get enrichment for a specific flight on today's date.
//
//	GET /api/v1/enrichment/{icao_hex}/{callsign}/{date}
//	    Get enrichment for a specific flight and date (YYYY-MM-DD).
//
//	POST /api/v1/enrichment/batch
//	    Batch lookup for multiple aircraft. Body: {"aircraft": [{"icao_hex": "..."}]}
//
// Authentication:
//
//	When -auth is enabled, requests must include an API key via:
//	  - X-API-Key header
//	  - Authorization: Bearer <key> header
//	  - ?api_key=<key> query parameter
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"acars_parser/internal/api"
	"acars_parser/internal/storage"
)

func main() {
	// PostgreSQL connection flags.
	pgHost := flag.String("pg-host", envOrDefault("POSTGRES_HOST", "localhost"), "PostgreSQL host")
	pgPort := flag.Int("pg-port", envOrDefaultInt("POSTGRES_PORT", 5432), "PostgreSQL port")
	pgUser := flag.String("pg-user", envOrDefault("POSTGRES_USER", "acars"), "PostgreSQL user")
	pgPassword := flag.String("pg-password", envOrDefault("POSTGRES_PASSWORD", "acars"), "PostgreSQL password")
	pgDB := flag.String("pg-database", envOrDefault("POSTGRES_DATABASE", "acars_state"), "PostgreSQL database")

	// API server flags.
	port := flag.Int("port", 8081, "HTTP port for API server")
	authEnabled := flag.Bool("auth", false, "Enable API key authentication")
	apiKeys := flag.String("api-keys", "", "Comma-separated list of valid API keys (when auth enabled)")

	flag.Parse()

	ctx := context.Background()

	// Open PostgreSQL database.
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

	// Parse API keys.
	var keys []string
	if *apiKeys != "" {
		keys = strings.Split(*apiKeys, ",")
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
		}
	}

	// Create and run server.
	server := api.NewEnrichmentServer(pg, api.Config{
		Port:        *port,
		AuthEnabled: *authEnabled,
		APIKeys:     keys,
	})

	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envOrDefaultInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}