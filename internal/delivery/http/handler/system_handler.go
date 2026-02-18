package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"api_free_demo/internal/delivery/http/middleware"
	"api_free_demo/pkg/response"
)

// LegacyAuthHandler provides the original GET /auth/token convenience endpoint
// for backward-compatible sandbox token generation (72h demo token, no role).
// New clients should use POST /auth/login from AuthHandler instead.
type LegacyAuthHandler struct {
	secret string
}

func NewLegacyAuthHandler(secret string) *LegacyAuthHandler {
	return &LegacyAuthHandler{secret: secret}
}

// GenerateToken handles GET /auth/token?user_id=xxx
// Returns a 72h demo token — kept for UI backward compatibility.
// For the full dual-token flow use POST /auth/login.
func (h *LegacyAuthHandler) GenerateToken(c *fiber.Ctx) error {
	userID := c.Query("user_id", "sandbox-user-1")

	token, err := middleware.GenerateDemoToken(h.secret, userID)
	if err != nil {
		return response.InternalError(c, "failed to generate token")
	}

	return response.OK(c, fiber.Map{
		"token":   token,
		"user_id": userID,
		"usage":   "Authorization: Bearer <token>",
		"note":    "Legacy 72h demo token. Use POST /auth/login for full dual-token flow.",
	})
}

// HealthHandler provides a health-check endpoint.
type HealthHandler struct {
	rdb *redis.Client
}

func NewHealthHandler(rdb *redis.Client) *HealthHandler {
	return &HealthHandler{rdb: rdb}
}

// Health handles GET /health
// Returns HTTP 200 when healthy, HTTP 503 when Redis is unreachable.
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	status := "ok"
	redisStatus := "ok"
	healthy := true

	// Use request-scoped context so the health check respects client timeouts.
	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		redisStatus = "error: " + err.Error()
		status = "degraded"
		healthy = false
	}

	httpStatus := fiber.StatusOK
	if !healthy {
		httpStatus = fiber.StatusServiceUnavailable
	}

	return c.Status(httpStatus).JSON(fiber.Map{
		"success": healthy,
		"data": fiber.Map{
			"status": status,
			"redis":  redisStatus,
		},
	})
}
