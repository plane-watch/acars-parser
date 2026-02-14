// Package main provides the nats-relay binary.
//
// nats-relay connects to the Airframes public NATS bus, deduplicates messages
// using a two-layer strategy (subject message ID + content hash), and
// republishes unique messages to an internal NATS server. This allows multiple
// internal consumers to share a single upstream connection.
//
// All configuration is via environment variables:
//
//   - AIRFRAMES_NATS_URL:     Airframes NATS server (default: nats://157.90.242.138:4222)
//   - AIRFRAMES_NATS_CREDS:   Path to .creds file or inline credential content (required)
//   - AIRFRAMES_NATS_SUBJECT: Subject to subscribe to (default: v1.aircraft.ingest.*.message.*.created)
//   - INTERNAL_NATS_URL:      Internal NATS server (default: nats://localhost:4222)
//   - INTERNAL_NATS_SUBJECT:  Subject to publish on (default: acars.messages)
//   - DEDUP_TTL:              Content hash expiry window (default: 10s)
//   - DEDUP_MAX_SIZE:         Max dedup cache entries (default: 100000)
//   - METRICS_ADDR:           HTTP address for /healthz and /metrics (default: :9090)
//   - LOG_LEVEL:              Log verbosity: debug, info, warn, error (default: info)
package main

import (
	"context"
	"fmt"
	"os"

	"acars_parser/internal/relay"
)

func main() {
	cfg := relay.LoadConfig()

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	r := relay.New(cfg)

	if err := r.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "relay error: %v\n", err)
		os.Exit(1)
	}
}
