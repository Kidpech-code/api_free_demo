package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kidpech/api_free_demo/internal/infrastructure/auth"
	"github.com/kidpech/api_free_demo/pkg/response"
)

// AuthMiddleware validates JWT bearer tokens.
func AuthMiddleware(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c.GetHeader("Authorization"))
		if token == "" {
			response.Unauthorized(c, "missing bearer token")
			c.Abort()
			return
		}
		claims, err := manager.ParseAccessToken(token)
		if err != nil {
			response.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("user_role", claims.Role)
		c.Next()
	}
}

// OptionalAuth attaches claims when available without enforcing auth.
func OptionalAuth(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c.GetHeader("Authorization"))
		if token == "" {
			c.Next()
			return
		}
		claims, err := manager.ParseAccessToken(token)
		if err == nil {
			c.Set("user_id", claims.UserID)
			c.Set("user_role", claims.Role)
		}
		c.Next()
	}
}

// AdminOnly ensures role based access.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("user_role")
		role, _ := roleVal.(string)
		if !exists || role != "admin" {
			response.Forbidden(c, "admin only")
			c.Abort()
			return
		}
		c.Next()
	}
}

func extractBearer(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
