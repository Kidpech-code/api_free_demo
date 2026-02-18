// Package middleware contains HTTP middleware for authentication and RBAC.
//
// Auth Middleware Design:
//
//	JWTAuth   → validates the Access Token signature + expiry
//	            → injects user_id and role into fiber.Locals
//	            → supports ?simulate_expiry=true for educational testing
//
//	RequireRole(role) → checks Locals["role"] against required role
//	                 → returns HTTP 403 Forbidden on mismatch (not 401)
//	                 → 401 = "you are not authenticated"
//	                 → 403 = "you are authenticated but not authorised"
package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// Context key constants — centralised here so handlers never use raw strings.
const (
	ContextKeyUserID = "user_id"
	ContextKeyRole   = "role" // populated from JWT "role" claim
)

// JWTAuth extracts and validates a Bearer JWT from the Authorization header.
//
// Behaviours:
//   - Skips /health, /metrics, /docs (public paths).
//   - No Authorization header → falls back to "demo-user" (sandbox convenience).
//   - Invalid/expired token → HTTP 401.
//   - Valid token → injects user_id and role into c.Locals for downstream use.
//
// ?simulate_expiry=true:
//
//	If this query param is present on ANY request that reaches JWTAuth,
//	the middleware pretends the token is expired and returns HTTP 401.
//	Purpose: lets frontend students test their auto-refresh interceptor
//	         without waiting for the real 15-minute TTL.
func JWTAuth(secret string, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()
		if path == "/health" || path == "/metrics" || path == "/docs" {
			return c.Next()
		}

		// ── 🎓 Simulation Mode ───────────────────────────────────────────
		// Allows UI / frontend tests to trigger a 401 on demand.
		if c.Query("simulate_expiry") == "true" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "token expired (simulated)",
				"hint":    "Call POST /auth/refresh with your refresh_token to get a new access_token.",
			})
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			// Sandbox convenience: no token → use a demo identity.
			c.Locals(ContextKeyUserID, "demo-user")
			c.Locals(ContextKeyRole, "user")
			logger.Debug("no auth header, using demo user")
			return c.Next()
		}

		// ── Parse "Bearer <token>" ────────────────────────────────────────
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "invalid Authorization header format — expected: Bearer <token>",
			})
		}
		tokenStr := parts[1]

		// ── Validate signature & expiry ───────────────────────────────────
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "unexpected signing method")
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "invalid or expired token",
				"hint":    "Call POST /auth/refresh with your refresh_token.",
			})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "invalid token claims",
			})
		}

		// ── Extract sub (user_id) ─────────────────────────────────────────
		sub, _ := claims["sub"].(string)
		if sub == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "token missing sub claim (user_id)",
			})
		}

		// ── Extract role (defaults to "user" for backward compat) ─────────
		role, _ := claims["role"].(string)
		if role == "" {
			role = "user"
		}

		c.Locals(ContextKeyUserID, sub)
		c.Locals(ContextKeyRole, role)
		logger.Debug("jwt authenticated",
			zap.String("user_id", sub),
			zap.String("role", role),
		)
		return c.Next()
	}
}

// RequireRole returns a middleware that enforces Role-Based Access Control.
//
// Usage (in router):
//
//	adminGroup := api.Group("/admin")
//	adminGroup.Use(middleware.RequireRole("admin"))
//
// HTTP responses:
//   - 401 Unauthorized  — if JWTAuth was bypassed (no user_id in Locals)
//   - 403 Forbidden     — if authenticated but role does not match
//
// The 401 vs 403 distinction is important:
//   - 401 = "I don't know who you are → go authenticate"
//   - 403 = "I know who you are → you don't have permission"
func RequireRole(requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Ensure JWTAuth has run first.
		userID, ok := c.Locals(ContextKeyUserID).(string)
		if !ok || userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "authentication required",
			})
		}

		role, _ := c.Locals(ContextKeyRole).(string)
		if role != requiredRole {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success":       false,
				"error":         "forbidden: insufficient role",
				"required_role": requiredRole,
				"your_role":     role,
				"hint":          "Login with POST /auth/login using role=\"" + requiredRole + "\" to access this resource.",
			})
		}

		return c.Next()
	}
}

// GenerateDemoToken creates a 72-hour JWT for the legacy GET /auth/token endpoint.
// New code should use the AuthUsecase.Login flow which issues 15-min access tokens.
func GenerateDemoToken(secret, userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": "user", // legacy tokens always carry "user" role
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(72 * time.Hour).Unix(),
		"iss":  "api_free_demo_sandbox",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
