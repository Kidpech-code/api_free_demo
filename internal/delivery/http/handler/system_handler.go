package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"api_free_demo/internal/delivery/http/middleware"
	"api_free_demo/pkg/response"
)

// AuthHandler provides a convenience endpoint to generate sandbox JWT tokens.
type AuthHandler struct {
	secret string
}

func NewAuthHandler(secret string) *AuthHandler {
	return &AuthHandler{secret: secret}
}

// GenerateToken handles GET /auth/token?user_id=xxx
func (h *AuthHandler) GenerateToken(c *fiber.Ctx) error {
	userID := c.Query("user_id", "sandbox-user-1")

	token, err := middleware.GenerateDemoToken(h.secret, userID)
	if err != nil {
		return response.InternalError(c, "failed to generate token")
	}

	return response.OK(c, fiber.Map{
		"token":   token,
		"user_id": userID,
		"usage":   "Authorization: Bearer <token>",
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
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	status := "ok"
	redisStatus := "ok"

	if err := h.rdb.Ping(context.Background()).Err(); err != nil {
		redisStatus = "error: " + err.Error()
		status = "degraded"
	}

	return response.OK(c, fiber.Map{
		"status": status,
		"redis":  redisStatus,
	})
}
