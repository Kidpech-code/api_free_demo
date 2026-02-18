package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

const (
	ContextKeyUserID = "user_id"
)

// JWTAuth extracts and validates a JWT from the Authorization header.
func JWTAuth(secret string, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()
		if path == "/health" || path == "/metrics" || path == "/docs" {
			return c.Next()
		}

		authHeader := c.Get("Authorization")

		if authHeader == "" {
			demoUserID := "demo-user"
			c.Locals(ContextKeyUserID, demoUserID)
			logger.Debug("no auth header, using demo user", zap.String("user_id", demoUserID))
			return c.Next()
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid authorization header format, expected: Bearer <token>",
			})
		}
		tokenStr := parts[1]

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "unexpected signing method")
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid or expired token",
			})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid token claims",
			})
		}

		sub, _ := claims["sub"].(string)
		if sub == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "token missing sub claim (user_id)",
			})
		}

		c.Locals(ContextKeyUserID, sub)
		logger.Debug("jwt authenticated", zap.String("user_id", sub))
		return c.Next()
	}
}

// GenerateDemoToken creates a signed JWT for quick sandbox testing.
func GenerateDemoToken(secret, userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(72 * time.Hour).Unix(),
		"iss": "api_free_demo_sandbox",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
