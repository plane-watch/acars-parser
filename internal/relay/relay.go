package relay

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Publisher defines the interface for publishing messages to NATS.
// *nats.Conn satisfies this interface.
type Publisher interface {
	Publish(subject string, data []byte) error
}

// Relay connects to an upstream NATS server, deduplicates messages,
// and republishes them to a downstream NATS server.
type Relay struct {
	cfg          Config
	upstream     *nats.Conn
	downstream   Publisher
	subjectCache *TTLCache[string]
	contentCache *TTLCache[uint64]
	metrics      *Metrics
	logger       *slog.Logger

	// Counters for the final stats summary on shutdown.
	received  atomic.Int64
	published atomic.Int64
}

// New creates a new Relay with the given configuration. It does not connect
// to NATS - call Run() to start the relay.
func New(cfg Config) *Relay {
	return &Relay{
		cfg:          cfg,
		subjectCache: NewTTLCache[string](cfg.DedupMaxSize, cfg.DedupTTL),
		contentCache: NewTTLCache[uint64](cfg.DedupMaxSize, cfg.DedupTTL),
		metrics:      NewMetrics(),
		logger:       newLogger(cfg.LogLevel),
	}
}

// Run starts the relay. It connects to both NATS servers, subscribes to the
// upstream subject, starts the metrics HTTP server, and blocks until a
// SIGINT or SIGTERM signal is received.
func (r *Relay) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Resolve credentials.
	credsPath, cleanup, err := ResolveCredsFile(r.cfg.AirframesCreds)
	if err != nil {
		return fmt.Errorf("resolve credentials: %w", err)
	}
	defer cleanup()

	// Connect to upstream (Airframes) NATS.
	r.logger.Info("connecting to upstream NATS", "url", r.cfg.AirframesURL)
	upOpts := []nats.Option{
		nats.UserCredentials(credsPath),
		nats.Name("nats-relay-upstream"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			r.logger.Warn("upstream disconnected", "error", err)
			r.metrics.UpstreamConnected.Set(0)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			r.logger.Info("upstream reconnected")
			r.metrics.UpstreamConnected.Set(1)
		}),
	}

	r.upstream, err = nats.Connect(r.cfg.AirframesURL, upOpts...)
	if err != nil {
		return fmt.Errorf("connect to upstream NATS: %w", err)
	}
	defer r.upstream.Close()
	r.metrics.UpstreamConnected.Set(1)
	r.logger.Info("connected to upstream NATS", "url", r.cfg.AirframesURL)

	// Connect to downstream (internal) NATS.
	r.logger.Info("connecting to downstream NATS", "url", r.cfg.InternalURL)
	downOpts := []nats.Option{
		nats.Name("nats-relay-downstream"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			r.logger.Warn("downstream disconnected", "error", err)
			r.metrics.DownstreamConnected.Set(0)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			r.logger.Info("downstream reconnected")
			r.metrics.DownstreamConnected.Set(1)
		}),
	}

	downConn, err := nats.Connect(r.cfg.InternalURL, downOpts...)
	if err != nil {
		return fmt.Errorf("connect to downstream NATS: %w", err)
	}
	defer downConn.Drain()
	r.downstream = downConn
	r.metrics.DownstreamConnected.Set(1)
	r.logger.Info("connected to downstream NATS", "url", r.cfg.InternalURL)

	// Subscribe to upstream messages.
	r.logger.Info("subscribing", "subject", r.cfg.AirframesSubject)
	sub, err := r.upstream.Subscribe(r.cfg.AirframesSubject, func(m *nats.Msg) {
		r.handleMessage(m.Subject, m.Data)
	})
	if err != nil {
		return fmt.Errorf("subscribe to upstream: %w", err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	// Start the dedup cache sweep loop.
	go r.sweepLoop(ctx)

	// Start the metrics HTTP server.
	metricsSrv := r.startMetricsServer()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	r.logger.Info("relay started",
		"upstream", r.cfg.AirframesURL,
		"downstream", r.cfg.InternalURL,
		"subject", r.cfg.AirframesSubject,
		"publish_to", r.cfg.InternalSubject,
		"dedup_ttl", r.cfg.DedupTTL,
		"metrics", r.cfg.MetricsAddr,
	)

	// Wait for shutdown signal.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		r.logger.Info("received signal, shutting down", "signal", sig)
	case <-ctx.Done():
		r.logger.Info("context cancelled, shutting down")
	}

	signal.Stop(sigChan)
	cancel()

	r.logger.Info("shutdown complete",
		"received", r.received.Load(),
		"published", r.published.Load(),
		"subject_cache", r.subjectCache.Len(),
		"content_cache", r.contentCache.Len(),
	)

	return nil
}

// handleMessage processes a single message from the upstream NATS server.
// It applies two layers of deduplication before publishing to the downstream.
func (r *Relay) handleMessage(subject string, data []byte) {
	r.received.Add(1)
	r.metrics.MessagesReceived.Inc()

	// Layer 1: Subject message ID dedup (no parsing needed).
	if msgID, ok := ExtractSubjectMsgID(subject); ok {
		if r.subjectCache.Add(msgID) {
			r.metrics.DupesSubject.Inc()
			return
		}
	}

	// Layer 2: Content hash dedup (partial JSON parse).
	label, tail, text, err := ExtractDedupFields(data)
	if err != nil {
		// If we can't parse the JSON, skip content dedup but still relay the message.
		// Don't drop messages just because they have unexpected format.
		r.logger.Debug("skipping content dedup for unparseable message", "error", err)
	} else {
		hash := ContentHash(label, tail, text)
		if r.contentCache.Add(hash) {
			r.metrics.DupesContent.Inc()
			if label != "" {
				r.metrics.DupesByLabel.WithLabelValues(label).Inc()
			}
			return
		}
	}

	// Track per-label stats.
	if label != "" {
		r.metrics.MessagesByLabel.WithLabelValues(label).Inc()
	}

	// Publish the raw bytes to the downstream NATS server.
	if err := r.downstream.Publish(r.cfg.InternalSubject, data); err != nil {
		r.logger.Error("failed to publish to downstream", "error", err)
		return
	}

	r.published.Add(1)
	r.metrics.MessagesPublished.Inc()
}

// sweepLoop periodically removes expired entries from both dedup caches.
func (r *Relay) sweepLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			subjectSwept := r.subjectCache.Sweep()
			contentSwept := r.contentCache.Sweep()

			cacheSize := float64(r.subjectCache.Len() + r.contentCache.Len())
			r.metrics.CacheSize.Set(cacheSize)

			if subjectSwept > 0 || contentSwept > 0 {
				r.logger.Debug("cache sweep",
					"subject_swept", subjectSwept,
					"content_swept", contentSwept,
					"total_size", int(cacheSize),
				)
			}
		case <-ctx.Done():
			return
		}
	}
}

// startMetricsServer starts the HTTP server for health checks and Prometheus metrics.
func (r *Relay) startMetricsServer() *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		upOK := r.upstream != nil && r.upstream.IsConnected()
		// Check if the downstream is a *nats.Conn (it might be a mock in tests).
		downOK := true
		if nc, ok := r.downstream.(*nats.Conn); ok {
			downOK = nc.IsConnected()
		}

		if upOK && downOK {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "ok")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "upstream=%v downstream=%v\n", upOK, downOK)
		}
	})

	mux.Handle("/metrics", promhttp.HandlerFor(r.metrics.Registry, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:              r.cfg.MetricsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		r.logger.Info("metrics server listening", "addr", r.cfg.MetricsAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			r.logger.Error("metrics server error", "error", err)
		}
	}()

	return srv
}

// ExtractSubjectMsgID extracts the message ID from an Airframes NATS subject.
// The subject format is: v1.aircraft.ingest.{source}.message.{msgID}.created
// Returns the message ID and true if extraction succeeded.
func ExtractSubjectMsgID(subject string) (string, bool) {
	parts := strings.Split(subject, ".")
	if len(parts) < 7 {
		return "", false
	}
	return parts[5], true
}

// newLogger creates a slog.Logger with the given level string.
func newLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
}
