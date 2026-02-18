package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds all Prometheus counters and histograms for the sandbox.
// It simulates Uber's observability for "Cache Hits" vs "Storage Operations".
type Metrics struct {
	// DocStore / Cache Simulation
	CacheHits   prometheus.Counter
	CacheMisses prometheus.Counter
	StorageOps  *prometheus.CounterVec // labels: operation

	// HTTP
	HTTPRequestsTotal   *prometheus.CounterVec   // labels: method, path, status
	HTTPRequestDuration *prometheus.HistogramVec // labels: method, path

	// Rate Limiter
	RateLimitHits prometheus.Counter
}

// NewMetrics registers and returns all application metrics.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		CacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "sandbox",
			Subsystem: "docstore",
			Name:      "cache_hits_total",
			Help:      "Number of reads served from the integrated cache layer.",
		}),
		CacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "sandbox",
			Subsystem: "docstore",
			Name:      "cache_misses_total",
			Help:      "Number of reads that required a full storage lookup.",
		}),
		StorageOps: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "sandbox",
			Subsystem: "docstore",
			Name:      "storage_operations_total",
			Help:      "Total storage-layer operations by type.",
		}, []string{"operation"}),
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "sandbox",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests by method, path, and status.",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "sandbox",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),
		RateLimitHits: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "sandbox",
			Subsystem: "ratelimit",
			Name:      "rejected_total",
			Help:      "Total requests rejected by rate limiter (HTTP 429).",
		}),
	}

	reg.MustRegister(
		m.CacheHits,
		m.CacheMisses,
		m.StorageOps,
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.RateLimitHits,
	)

	return m
}
