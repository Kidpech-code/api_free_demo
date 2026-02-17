package diagnostics

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthChecker is implemented by any dependency that can report health.
type HealthChecker interface {
	IsHealthy(ctx context.Context) bool
}

// Handler exposes health + debug endpoints.
type Handler struct {
	buffer      *LogBuffer
	mu          sync.RWMutex
	deps        map[string]HealthChecker
	start       time.Time
	dbConnected bool
	dbError     string
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
	h.mu.Lock()
	h.deps[name] = checker
	h.mu.Unlock()
}

// SetDBStatus updates the database connection status for diagnostics.
func (h *Handler) SetDBStatus(connected bool, errMsg string) {
	h.mu.Lock()
	h.dbConnected = connected
	h.dbError = errMsg
	h.mu.Unlock()
}

// RegisterPublic attaches non-auth endpoints.
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/health", h.health)
	rg.GET("/health/ready", h.readiness)
	rg.GET("/health/debug", h.debugEnv)
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

	h.mu.RLock()
	for name, checker := range h.deps {
		if checker.IsHealthy(ctx) {
			deps[name] = "up"
		} else {
			deps[name] = "down"
			status = "degraded"
			code = http.StatusServiceUnavailable
		}
	}
	h.mu.RUnlock()

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
	h.mu.RLock()
	for name, checker := range h.deps {
		if checker.IsHealthy(ctx) {
			deps[name] = "up"
		} else {
			deps[name] = "down"
		}
	}
	h.mu.RUnlock()

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

// debugEnv shows which database env vars are available (no secrets).
// This is PUBLIC so we can diagnose Railway env var issues.
func (h *Handler) debugEnv(c *gin.Context) {
	// Check which DB-related env vars exist (never show values)
	envVars := []string{
		"DB_DSN", "DATABASE_URL", "DATABASE_PRIVATE_URL",
		"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE",
		"DATABASE_HOST", "DATABASE_PORT", "DATABASE_USER", "DATABASE_NAME",
		"REDIS_ADDR", "REDIS_URL", "REDIS_PRIVATE_URL",
		"PORT", "APP_ENV",
	}
	envStatus := make(map[string]interface{})
	for _, key := range envVars {
		val := os.Getenv(key)
		if val == "" {
			envStatus[key] = false
		} else {
			// Show length and masked prefix for debugging (no secrets)
			masked := maskValue(key, val)
			envStatus[key] = masked
		}
	}

	h.mu.RLock()
	dbStatus := "not connected"
	if h.dbConnected {
		dbStatus = "connected"
	}
	dbErr := h.dbError
	h.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"uptime":     time.Since(h.start).String(),
		"db_status":  dbStatus,
		"db_error":   dbErr,
		"env_vars":   envStatus,
		"go_version": runtime.Version(),
		"goroutines": runtime.NumGoroutine(),
	})
}

// maskValue safely masks sensitive env var values.
func maskValue(key, val string) string {
	// For safe keys, show the value
	safeKeys := map[string]bool{"PORT": true, "APP_ENV": true, "PGPORT": true, "DATABASE_PORT": true}
	if safeKeys[key] {
		return val
	}
	// For host keys, show the value (not secret)
	hostKeys := map[string]bool{"PGHOST": true, "DATABASE_HOST": true}
	if hostKeys[key] {
		return val
	}
	// For URL keys, show scheme + host only
	if strings.Contains(strings.ToLower(key), "url") || strings.Contains(strings.ToLower(key), "dsn") || strings.Contains(strings.ToLower(key), "addr") {
		// Show length and first chars
		if len(val) > 15 {
			return val[:15] + "...(" + strings.Repeat("*", 4) + ")"
		}
		return strings.Repeat("*", len(val))
	}
	// Everything else: just show it's set
	return "set (" + strings.Repeat("*", min(len(val), 8)) + ")"
}
