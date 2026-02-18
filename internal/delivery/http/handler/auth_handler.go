package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"api_free_demo/internal/delivery/http/middleware"
	"api_free_demo/internal/domain/model"
	"api_free_demo/internal/usecase"
	"api_free_demo/pkg/response"
)

// AuthHandler handles the full Auth Lifecycle: Login, Refresh, Logout.
//
// Endpoints:
//
//	POST /auth/login    — issue AccessToken (15 min) + RefreshToken (7 days)
//	POST /auth/refresh  — exchange RefreshToken for a new AccessToken
//	POST /auth/logout   — revoke RefreshToken (requires valid AccessToken)
type AuthHandler struct {
	authUC usecase.AuthUsecase
	logger *zap.Logger
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(authUC usecase.AuthUsecase, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{authUC: authUC, logger: logger}
}

// Login handles POST /auth/login
//
// Request body:
//
//	{ "user_id": "alice", "role": "admin" }   (role is optional, default: "user")
//
// Response (200):
//
//	{
//	  "access_token":  "eyJ...",   // JWT, 15 minutes
//	  "refresh_token": "a3f9...",  // random hex, 7 days
//	  "token_type":    "Bearer",
//	  "expires_in":    900,        // seconds
//	  "role":          "admin",
//	  "user_id":       "alice"
//	}
//
// The client should:
//  1. Store access_token in memory (never localStorage — XSS risk).
//  2. Store refresh_token + user_id in httpOnly cookie or secure storage.
//  3. When any API call returns 401, call POST /auth/refresh to renew.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req model.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body: "+err.Error())
	}
	if req.UserID == "" {
		return response.BadRequest(c, "user_id is required")
	}

	pair, err := h.authUC.Login(c.UserContext(), &req)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidRole) {
			return response.BadRequest(c, err.Error())
		}
		h.logger.Error("login failed", zap.Error(err), zap.String("user_id", req.UserID))
		return response.InternalError(c, "login failed")
	}

	return response.OK(c, pair)
}

// Refresh handles POST /auth/refresh
//
// Request body:
//
//	{ "user_id": "alice", "refresh_token": "a3f9..." }
//
// Why does the client need to send user_id?
// Because the AccessToken is EXPIRED at this point — the JWT middleware cannot
// inject user_id into Locals. The user_id is not secret; it is the client's
// own namespace identifier they stored alongside the refresh_token.
//
// Response (200): same shape as Login response but only access_token is new.
//
// ?simulate_expiry=true:
// If this query param is present, the handler returns a 401 immediately,
// pretending the token is expired. This lets frontend developers test their
// "Auto-Refresh" logic without waiting 15 minutes.
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	// 🎓 Simulation Mode: return a 401 as if the access token just expired.
	// Use this to test your frontend's auto-refresh interceptor.
	if c.Query("simulate_expiry") == "true" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "token expired (simulated — use your refresh_token to call POST /auth/refresh)",
			"hint":    "This is a test mode. Call POST /auth/refresh with your refresh_token.",
		})
	}

	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "invalid request body: "+err.Error())
	}
	if req.UserID == "" || req.RefreshToken == "" {
		return response.BadRequest(c, "user_id and refresh_token are required")
	}

	pair, err := h.authUC.Refresh(c.UserContext(), &req)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidRefreshToken) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
				"hint":    "Your refresh token is invalid or has been revoked. Please call POST /auth/login again.",
			})
		}
		h.logger.Error("refresh failed", zap.Error(err))
		return response.InternalError(c, "refresh failed")
	}

	return response.OK(c, pair)
}

// Logout handles POST /auth/logout
//
// Requires: Authorization: Bearer <access_token>  (validated by JWTAuth middleware)
//
// Effect: Deletes sandbox:{user_id}:auth:refresh from Redis.
// After this call the refresh_token is permanently invalid — the client must
// call POST /auth/login to obtain a new pair.
//
// ?simulate_expiry=true: returns 401 to let students test logout→refresh edge cases.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// 🎓 Simulation Mode
	if c.Query("simulate_expiry") == "true" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "token expired (simulated)",
		})
	}

	userID, ok := c.Locals(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "authentication required",
		})
	}

	if err := h.authUC.Logout(c.UserContext(), userID); err != nil {
		h.logger.Error("logout failed", zap.Error(err), zap.String("user_id", userID))
		return response.InternalError(c, "logout failed")
	}

	return response.OK(c, fiber.Map{
		"message": "logged out successfully — refresh token has been revoked",
		"user_id": userID,
	})
}
