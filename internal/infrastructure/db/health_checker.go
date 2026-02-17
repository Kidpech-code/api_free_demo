package db

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// TableHealthChecker verifies that critical tables exist and are queryable.
type TableHealthChecker struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewTableHealthChecker creates a checker that validates table accessibility.
func NewTableHealthChecker(db *sqlx.DB, logger *zap.Logger) *TableHealthChecker {
	return &TableHealthChecker{db: db, logger: logger}
}

// IsHealthy checks if the users table exists and is queryable.
func (t *TableHealthChecker) IsHealthy(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var count int
	query := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`

	err := t.db.GetContext(ctx, &count, query)
	if err != nil {
		if t.logger != nil {
			t.logger.Error("[HealthCheck] users table query failed - table may not exist!",
				zap.Error(err),
				zap.String("query", query),
			)
		}
		return false
	}

	if t.logger != nil {
		t.logger.Info("[HealthCheck] users table query OK",
			zap.Int("user_count", count),
		)
	}
	return true
}
