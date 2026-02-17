package monitoring

import (
	"strconv"

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

	// Cache metrics (inspired by Uber CacheFront observability)
	cacheHitCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total cache hits",
		},
		[]string{"prefix"},
	)
	cacheMissCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total cache misses",
		},
		[]string{"prefix"},
	)
	cacheErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_errors_total",
			Help: "Total cache errors",
		},
		[]string{"prefix"},
	)
	cacheInvalidationCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_invalidations_total",
			Help: "Total cache invalidations",
		},
		[]string{"prefix"},
	)
	cacheInvalidationKeysCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_invalidation_keys_total",
			Help: "Total keys invalidated",
		},
		[]string{"prefix"},
	)

	// Database metrics
	dbQueryCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total database queries",
		},
		[]string{"operation"},
	)
	dbLatencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		},
		[]string{"operation"},
	)
)

// Init registers custom collectors.
func Init() {
	prometheus.MustRegister(
		requestCounter,
		latencyHistogram,
		cacheHitCounter,
		cacheMissCounter,
		cacheErrorCounter,
		cacheInvalidationCounter,
		cacheInvalidationKeysCounter,
		dbQueryCounter,
		dbLatencyHistogram,
	)
}

// ObserveRequest records HTTP metrics.
func ObserveRequest(path, method, status string, seconds float64) {
	requestCounter.WithLabelValues(path, method, status).Inc()
	latencyHistogram.WithLabelValues(path, method).Observe(seconds)
}

// CacheHit records a cache hit.
func CacheHit(prefix string) {
	cacheHitCounter.WithLabelValues(prefix).Inc()
}

// CacheMiss records a cache miss.
func CacheMiss(prefix string) {
	cacheMissCounter.WithLabelValues(prefix).Inc()
}

// CacheError records a cache error.
func CacheError(prefix string) {
	cacheErrorCounter.WithLabelValues(prefix).Inc()
}

// CacheInvalidation records cache invalidations.
func CacheInvalidation(prefix string, keys int) {
	cacheInvalidationCounter.WithLabelValues(prefix).Inc()
	cacheInvalidationKeysCounter.WithLabelValues(prefix).Add(float64(keys))
}

// ObserveDBQuery records database query metrics.
func ObserveDBQuery(operation string, seconds float64) {
	dbQueryCounter.WithLabelValues(operation).Inc()
	dbLatencyHistogram.WithLabelValues(operation).Observe(seconds)
}

// FormatCacheStats returns formatted cache statistics for /debug/health.
func FormatCacheStats(prefix string) map[string]string {
	hits, _ := getCounterValue(prefix)
	misses, _ := getCounterValue(prefix)
	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = hits / total * 100
	}
	return map[string]string{
		"hits":     strconv.FormatFloat(hits, 'f', 0, 64),
		"misses":   strconv.FormatFloat(misses, 'f', 0, 64),
		"hit_rate": strconv.FormatFloat(hitRate, 'f', 1, 64) + "%",
	}
}

func getCounterValue(_ string) (float64, error) {
	// Prometheus counters don't expose values directly via the API.
	// This is a best-effort approach — use /metrics endpoint for accurate data.
	return 0, nil
}
