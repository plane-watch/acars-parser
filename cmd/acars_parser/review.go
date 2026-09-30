// Package main provides the review command for launching the web UI.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"acars_parser/internal/review"
	"acars_parser/internal/storage"
)

func runReviewCmd(args []string) {
	fs := flag.NewFlagSet("review", flag.ExitOnError)

	// ClickHouse connection flags.
	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPassword := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")
	chDB := fs.String("ch-db", defaultCHDatabase(), "ClickHouse database")

	// PostgreSQL connection flags.
	pgHost := fs.String("pg-host", defaultPGHost(), "PostgreSQL host")
	pgPort := fs.Int("pg-port", defaultPGPort(), "PostgreSQL port")
	pgUser := fs.String("pg-user", defaultPGUser(), "PostgreSQL user")
	pgPassword := fs.String("pg-password", defaultPGPassword(), "PostgreSQL password")
	pgDB := fs.String("pg-db", defaultPGDatabase(), "PostgreSQL database")

	port := fs.Int("port", 8080, "HTTP port")
	parserType := fs.String("type", "", "Pre-filter to specific parser type")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Open ClickHouse database.
	ch, err := storage.OpenClickHouse(ctx, storage.ClickHouseConfig{
		Host:     *chHost,
		Port:     *chPort,
		Database: *chDB,
		User:     *chUser,
		Password: *chPassword,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening ClickHouse: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = ch.Close() }()

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

	// Create and run server.
	server := review.NewServer(ch, pg, *port, *parserType)
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
