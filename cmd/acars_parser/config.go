package main

import (
	"os"
	"strconv"
)

// envOrDefault returns the environment variable value if set, otherwise the default.
func envOrDefault(envVar, defaultVal string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return defaultVal
}

// envIntOrDefault returns the environment variable as int if set, otherwise the default.
func envIntOrDefault(envVar string, defaultVal int) int {
	if v := os.Getenv(envVar); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

// Default ClickHouse configuration from environment variables.
func defaultCHHost() string     { return envOrDefault("CLICKHOUSE_HOST", "localhost") }
func defaultCHPort() int        { return envIntOrDefault("CLICKHOUSE_PORT", 9000) }
func defaultCHDatabase() string { return envOrDefault("CLICKHOUSE_DATABASE", "acars") }
func defaultCHUser() string     { return envOrDefault("CLICKHOUSE_USER", "default") }
func defaultCHPassword() string { return envOrDefault("CLICKHOUSE_PASSWORD", "acars") }

// Default PostgreSQL configuration from environment variables.
func defaultPGHost() string     { return envOrDefault("POSTGRES_HOST", "localhost") }
func defaultPGPort() int        { return envIntOrDefault("POSTGRES_PORT", 5432) }
func defaultPGDatabase() string { return envOrDefault("POSTGRES_DATABASE", "acars_state") }
func defaultPGUser() string     { return envOrDefault("POSTGRES_USER", "acars") }
func defaultPGPassword() string { return envOrDefault("POSTGRES_PASSWORD", "acars") }
