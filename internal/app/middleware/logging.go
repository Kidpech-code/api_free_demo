package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kidpech/api_free_demo/internal/app/diagnostics"
	"github.com/kidpech/api_free_demo/internal/infrastructure/logging"
	"github.com/kidpech/api_free_demo/internal/infrastructure/monitoring"
)

// RequestLogger logs request info and records metrics.
func RequestLogger(logger *zap.Logger, buffer *diagnostics.LogBuffer) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		reqID, _ := c.Get("request_id")
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		id := ""
		if reqID != nil {
			if v, ok := reqID.(string); ok {
				id = v
			}
		}
		logging.WithRequestID(logger, id).Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
		)
		entry := time.Now().UTC().Format(time.RFC3339) + " " + c.Request.Method + " " + path + " -> " + httpStatus(status)
		if buffer != nil {
			buffer.Append(entry)
		}
		monitoring.ObserveRequest(path, c.Request.Method, httpStatus(status), latency.Seconds())
	}
}

func httpStatus(status int) string {
	return fmt.Sprintf("%d", status)
}
