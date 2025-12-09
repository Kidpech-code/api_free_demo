package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"path", "method", "status"},
	)
	latencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Request latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	)
)

// Init registers custom collectors.
func Init() {
	prometheus.MustRegister(requestCounter, latencyHistogram)
}

// ObserveRequest records metrics.
func ObserveRequest(path, method, status string, seconds float64) {
	requestCounter.WithLabelValues(path, method, status).Inc()
	latencyHistogram.WithLabelValues(path, method).Observe(seconds)
}
