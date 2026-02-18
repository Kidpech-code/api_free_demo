package middleware

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"api_free_demo/internal/infrastructure/metrics"
)

// MetricsMiddleware records HTTP request count and latency for Prometheus.
func MetricsMiddleware(m *metrics.Metrics) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())
		method := c.Method()
		path := c.Route().Path // use route pattern, not raw path (avoids cardinality explosion)

		m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)

		return err
	}
}
