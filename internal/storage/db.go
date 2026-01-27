package storage

import (
	"context"
	"fmt"
)

// Config holds database connection settings for both ClickHouse and PostgreSQL.
type Config struct {
	ClickHouse ClickHouseConfig
	Postgres   PostgresConfig
}

// DefaultConfig returns a configuration with default local development settings.
func DefaultConfig() Config {
	return Config{
		ClickHouse: ClickHouseConfig{
			Host:     "localhost",
			Port:     9000,
			Database: "acars",
			User:     "default",
			Password: "",
		},
		Postgres: PostgresConfig{
			Host:     "localhost",
			Port:     5432,
			Database: "acars_state",
			User:     "acars",
			Password: "acars",
		},
	}
}

// DB wraps both ClickHouse and PostgreSQL connections.
type DB struct {
	CH *ClickHouseDB // ClickHouse for messages and analytics.
	PG *PostgresDB   // PostgreSQL for state and mutable data.
}

// Open opens connections to both ClickHouse and PostgreSQL.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	ch, err := OpenClickHouse(ctx, cfg.ClickHouse)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: %w", err)
	}

	pg, err := OpenPostgres(ctx, cfg.Postgres)
	if err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("postgres: %w", err)
	}

	return &DB{CH: ch, PG: pg}, nil
}

// Close closes both database connections.
func (d *DB) Close() error {
	var errs []error
	if d.CH != nil {
		if err := d.CH.Close(); err != nil {
			errs = append(errs, fmt.Errorf("clickhouse: %w", err))
		}
	}
	if d.PG != nil {
		d.PG.Close()
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// CreateSchemas creates the schemas in both databases.
func (d *DB) CreateSchemas(ctx context.Context) error {
	if err := d.CH.CreateSchema(ctx); err != nil {
		return fmt.Errorf("clickhouse schema: %w", err)
	}
	if err := d.PG.CreateSchema(ctx); err != nil {
		return fmt.Errorf("postgres schema: %w", err)
	}
	return nil
}
