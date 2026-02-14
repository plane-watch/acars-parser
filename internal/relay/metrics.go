package relay

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the relay.
type Metrics struct {
	MessagesReceived    prometheus.Counter
	MessagesPublished   prometheus.Counter
	DupesSubject        prometheus.Counter
	DupesContent        prometheus.Counter
	CacheSize           prometheus.Gauge
	UpstreamConnected   prometheus.Gauge
	DownstreamConnected prometheus.Gauge
	MessagesByLabel     *prometheus.CounterVec
	DupesByLabel        *prometheus.CounterVec

	// Registry is the Prometheus registry used for these metrics. This is
	// stored so the metrics HTTP handler can serve from the correct registry.
	Registry *prometheus.Registry
}

// NewMetrics creates and registers all Prometheus metrics with the default
// Prometheus registry. This is the standard constructor for production use.
func NewMetrics() *Metrics {
	return NewMetricsWithRegistry(prometheus.DefaultRegisterer.(*prometheus.Registry))
}

// NewMetricsWithRegistry creates and registers all Prometheus metrics with the
// given registry. This allows tests to use an isolated registry per instance,
// avoiding duplicate registration panics.
func NewMetricsWithRegistry(reg *prometheus.Registry) *Metrics {
	factory := promauto.With(reg)

	return &Metrics{
		Registry: reg,
		MessagesReceived: factory.NewCounter(prometheus.CounterOpts{
			Name: "relay_messages_received_total",
			Help: "Total messages received from the upstream NATS server.",
		}),
		MessagesPublished: factory.NewCounter(prometheus.CounterOpts{
			Name: "relay_messages_published_total",
			Help: "Messages successfully published to the internal NATS server.",
		}),
		DupesSubject: factory.NewCounter(prometheus.CounterOpts{
			Name: "relay_duplicates_subject_total",
			Help: "Duplicates caught by the subject message ID check (layer 1).",
		}),
		DupesContent: factory.NewCounter(prometheus.CounterOpts{
			Name: "relay_duplicates_content_total",
			Help: "Duplicates caught by the content hash check (layer 2).",
		}),
		CacheSize: factory.NewGauge(prometheus.GaugeOpts{
			Name: "relay_dedup_cache_size",
			Help: "Current number of entries in the dedup caches (subject + content combined).",
		}),
		UpstreamConnected: factory.NewGauge(prometheus.GaugeOpts{
			Name: "relay_upstream_connected",
			Help: "Whether the upstream (Airframes) NATS connection is active (1=connected, 0=disconnected).",
		}),
		DownstreamConnected: factory.NewGauge(prometheus.GaugeOpts{
			Name: "relay_downstream_connected",
			Help: "Whether the downstream (internal) NATS connection is active (1=connected, 0=disconnected).",
		}),
		MessagesByLabel: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "relay_messages_by_label_total",
			Help: "Messages received per ACARS label.",
		}, []string{"acars_label"}),
		DupesByLabel: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "relay_duplicates_by_label_total",
			Help: "Duplicates detected per ACARS label.",
		}, []string{"acars_label"}),
	}
}
