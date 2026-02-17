package diagnostics

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthChecker is implemented by any dependency that can report health.
type HealthChecker interface {
	IsHealthy(ctx context.Context) bool
}

// Handler exposes health + debug endpoints.
type Handler struct {
	buffer *LogBuffer
	deps   map[string]HealthChecker
	start  time.Time
}

// NewHandler returns handler.
func NewHandler(buffer *LogBuffer) *Handler {
	return &Handler{
		buffer: buffer,
		deps:   make(map[string]HealthChecker),
		start:  time.Now(),
	}
}

// RegisterDependency adds a named dependency for health reporting.
func (h *Handler) RegisterDependency(name string, checker HealthChecker) {
	h.deps[name] = checker
}

// RegisterPublic attaches non-auth endpoints.
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/health", h.health)
	rg.GET("/health/ready", h.readiness)
}

// RegisterProtected attaches debug endpoints requiring auth.
func (h *Handler) RegisterProtected(rg *gin.RouterGroup) {
	rg.GET("/debug/logs", h.logs)
	rg.GET("/debug/health", h.detailedHealth)
}

// health is the lightweight liveness probe — always returns 200 (for Railway healthcheck).
func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readiness checks all dependencies; returns 503 if any critical dep is down.
func (h *Handler) readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	status := "ready"
	code := http.StatusOK
	deps := make(map[string]string)

	for name, checker := range h.deps {
		if checker.IsHealthy(ctx) {
			deps[name] = "up"
		} else {
			deps[name] = "down"
			status = "degraded"
			code = http.StatusServiceUnavailable
		}
	}

	c.JSON(code, gin.H{
		"status":       status,
		"dependencies": deps,
		"uptime":       time.Since(h.start).String(),
	})
}

// detailedHealth returns full system diagnostics (admin only).
func (h *Handler) detailedHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	deps := make(map[string]string)
	for name, checker := range h.deps {
		if checker.IsHealthy(ctx) {
			deps[name] = "up"
		} else {
			deps[name] = "down"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"uptime":       time.Since(h.start).String(),
		"goroutines":   runtime.NumGoroutine(),
		"dependencies": deps,
		"memory": gin.H{
			"alloc_mb":       mem.Alloc / 1024 / 1024,
			"total_alloc_mb": mem.TotalAlloc / 1024 / 1024,
			"sys_mb":         mem.Sys / 1024 / 1024,
			"num_gc":         mem.NumGC,
		},
	})
}

func (h *Handler) logs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": h.buffer.Snapshot()})
}
