package db

import (
	"context"
	"time"
)

// HealthChecker wraps the DB manager as a health checker.
type HealthChecker struct {
	mgr *Manager
}

// NewHealthChecker creates a health checker for the database.
func NewHealthChecker(mgr *Manager) *HealthChecker {
	return &HealthChecker{mgr: mgr}
}

// IsHealthy returns true if the database is reachable.
func (h *HealthChecker) IsHealthy(ctx context.Context) bool {
	if h == nil || h.mgr == nil || h.mgr.Write == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return h.mgr.Write.PingContext(ctx) == nil
}
