package relay

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the NATS relay.
type Config struct {
	// Upstream (Airframes) NATS connection.
	AirframesURL     string
	AirframesCreds   string // Path to .creds file or inline credential content.
	AirframesSubject string

	// Downstream (internal) NATS connection.
	InternalURL     string
	InternalSubject string

	// Deduplication settings.
	DedupTTL     time.Duration
	DedupMaxSize int

	// Observability.
	MetricsAddr string
	LogLevel    string
}

// LoadConfig reads configuration from environment variables, falling back
// to sensible defaults for each field.
func LoadConfig() Config {
	return Config{
		AirframesURL:     envOrDefault("AIRFRAMES_NATS_URL", "nats://157.90.242.138:4222"),
		AirframesCreds:   os.Getenv("AIRFRAMES_NATS_CREDS"),
		AirframesSubject: envOrDefault("AIRFRAMES_NATS_SUBJECT", "v1.aircraft.ingest.*.message.*.created"),
		InternalURL:      envOrDefault("INTERNAL_NATS_URL", "nats://localhost:4222"),
		InternalSubject:  envOrDefault("INTERNAL_NATS_SUBJECT", "acars.messages"),
		DedupTTL:         envDurationOrDefault("DEDUP_TTL", 10*time.Second),
		DedupMaxSize:     envIntOrDefault("DEDUP_MAX_SIZE", 100000),
		MetricsAddr:      envOrDefault("METRICS_ADDR", ":9090"),
		LogLevel:         envOrDefault("LOG_LEVEL", "info"),
	}
}

// Validate checks that all required configuration fields are set.
func (c Config) Validate() error {
	if c.AirframesCreds == "" {
		return fmt.Errorf("AIRFRAMES_NATS_CREDS is required (set to a file path or inline credential content)")
	}
	return nil
}

// ResolveCredsFile takes a credentials value (file path or inline content) and
// returns a file path suitable for nats.UserCredentials(). If the value is
// inline content (not an existing file), it writes it to a temporary file.
//
// The returned cleanup function should be deferred to remove any temporary file.
func ResolveCredsFile(value string) (path string, cleanup func(), err error) {
	noop := func() {}

	// Check if the value is an existing file path.
	if _, err := os.Stat(value); err == nil {
		return value, noop, nil
	}

	// Treat as inline content - write to a temporary file.
	tmpFile, err := os.CreateTemp("", "nats-relay-creds-*")
	if err != nil {
		return "", noop, fmt.Errorf("create temp creds file: %w", err)
	}

	if _, err := tmpFile.WriteString(value); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", noop, fmt.Errorf("write temp creds file: %w", err)
	}
	tmpFile.Close()

	cleanup = func() { os.Remove(tmpFile.Name()) }
	return tmpFile.Name(), cleanup, nil
}

// envOrDefault returns the environment variable value if set, otherwise the default.
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// envIntOrDefault returns the environment variable as an int if set and valid,
// otherwise the default.
func envIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

// envDurationOrDefault returns the environment variable as a time.Duration if
// set and valid, otherwise the default.
func envDurationOrDefault(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
