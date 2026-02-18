package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"api_free_demo/internal/infrastructure/metrics"
)

// RateLimiter implements a Fixed Window rate-limiter per user_id using Redis.
//
// Algorithm: Fixed Window Counter
//   - Key:    ratelimit:{user_id}:{window_epoch}
//   - INCR on each request; if count > limit → reject 429.
//   - Key auto-expires after the window duration.
//
// This runs in Redis to ensure correctness across multiple API instances.
func RateLimiter(rdb *redis.Client, limit int, window time.Duration, m *metrics.Metrics, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// ── Skip non-authenticated endpoints ──
		path := c.Path()
		if path == "/health" || path == "/metrics" || path == "/docs" || path == "/auth/token" {
			return c.Next()
		}

		userID, _ := c.Locals(ContextKeyUserID).(string)
		if userID == "" {
			return c.Next() // no user_id → skip (shouldn't happen after JWT middleware)
		}

		ctx := context.Background()
		windowEpoch := time.Now().Unix() / int64(window.Seconds())
		key := fmt.Sprintf("ratelimit:%s:%d", userID, windowEpoch)

		// Atomic INCR + conditional EXPIRE in a pipeline.
		pipe := rdb.Pipeline()
		incrCmd := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, window+time.Second) // +1s grace
		if _, err := pipe.Exec(ctx); err != nil {
			logger.Error("rate limiter redis error", zap.Error(err))
			return c.Next() // fail-open: allow request on Redis errors
		}

		count := incrCmd.Val()
		remaining := int64(limit) - count
		if remaining < 0 {
			remaining = 0
		}

		// ── Set rate-limit headers (RFC draft) ──
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", (windowEpoch+1)*int64(window.Seconds())))

		if count > int64(limit) {
			m.RateLimitHits.Inc()
			logger.Warn("rate limit exceeded",
				zap.String("user_id", userID),
				zap.Int64("count", count),
			)
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":       "rate limit exceeded",
				"retry_after": fmt.Sprintf("%.0fs", window.Seconds()),
			})
		}

		return c.Next()
	}
}
