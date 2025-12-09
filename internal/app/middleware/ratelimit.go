package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kidpech/api_free_demo/internal/infrastructure/ratelimit"
	"github.com/kidpech/api_free_demo/pkg/response"
)

// RateLimit enforces per-IP and per-user throttles.
func RateLimit(ipLimiter, userLimiter ratelimit.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		clientIP := c.ClientIP()
		if ipLimiter != nil {
			info, err := ipLimiter.Allow(ctx, "ip:"+clientIP)
			if err == nil {
				setHeaders(c, info)
				if !info.Allowed {
					response.TooManyRequests(c, info.Reset)
					c.Abort()
					return
				}
			}
		}
		userID := response.UserIDFromContext(c)
		if userLimiter != nil && userID != "" {
			info, err := userLimiter.Allow(ctx, "user:"+userID)
			if err == nil {
				setHeaders(c, info)
				if !info.Allowed {
					response.TooManyRequests(c, info.Reset)
					c.Abort()
					return
				}
			}
		}
		c.Next()
	}
}

func setHeaders(c *gin.Context, info ratelimit.RateLimitInfo) {
	c.Writer.Header().Set("X-RateLimit-Limit", intToString(info.Limit))
	c.Writer.Header().Set("X-RateLimit-Remaining", intToString(info.Remaining))
	reset := time.Until(info.Reset)
	if reset < 0 {
		reset = 0
	}
	c.Writer.Header().Set("X-RateLimit-Reset", strconv.FormatInt(int64(info.Reset.Unix()), 10))
	if !info.Allowed {
		c.Writer.Header().Set("Retry-After", strconv.Itoa(int(reset.Seconds())))
	}
}

func intToString(val int) string {
	return strconv.Itoa(val)
}
