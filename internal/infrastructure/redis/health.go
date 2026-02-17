package redis

import (
	"context"
	"time"
)

// HealthChecker wraps the Redis client as a health checker.
type HealthChecker struct {
	client *Client
}

// NewHealthChecker creates a health checker for Redis.
func NewHealthChecker(client *Client) *HealthChecker {
	return &HealthChecker{client: client}
}

// IsHealthy returns true if Redis is reachable.
func (h *HealthChecker) IsHealthy(ctx context.Context) bool {
	if h == nil || h.client == nil || h.client.Native == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return h.client.Native.Ping(ctx).Err() == nil
}
