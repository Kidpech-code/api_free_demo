package tmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ──────────────────────────────────────────────────────────
// TMD Forecast – Redis Cache Repository
//
// Key patterns:
//   tmd:forecast:hourly:<location_code>
//   tmd:forecast:daily:<location_code>
//   tmd:forecast:area:<location_code>
// ──────────────────────────────────────────────────────────

// CacheRepository stores raw JSON forecast responses in Redis.
type CacheRepository struct {
	rdb    *redis.Client
	ttl    time.Duration
	logger *zap.Logger
}

// NewCacheRepository creates a repository with the given TTL.
func NewCacheRepository(rdb *redis.Client, ttl time.Duration, logger *zap.Logger) *CacheRepository {
	return &CacheRepository{
		rdb:    rdb,
		ttl:    ttl,
		logger: logger.Named("tmd.cache"),
	}
}

// key builds a Redis key following the pattern tmd:forecast:<type>:<code>.
func key(ft ForecastType, locationCode string) string {
	return fmt.Sprintf("tmd:forecast:%s:%s", ft, locationCode)
}

// Set stores raw JSON forecast data.
func (r *CacheRepository) Set(ctx context.Context, ft ForecastType, locationCode string, data json.RawMessage) error {
	k := key(ft, locationCode)
	if err := r.rdb.Set(ctx, k, []byte(data), r.ttl).Err(); err != nil {
		r.logger.Error("failed to cache forecast",
			zap.String("key", k),
			zap.Error(err),
		)
		return fmt.Errorf("redis SET %s: %w", k, err)
	}
	r.logger.Debug("cached forecast",
		zap.String("key", k),
		zap.Duration("ttl", r.ttl),
		zap.Int("bytes", len(data)),
	)
	return nil
}

// Get retrieves cached JSON for the given forecast type and location.
// Returns nil, nil when the key does not exist (cache miss).
func (r *CacheRepository) Get(ctx context.Context, ft ForecastType, locationCode string) (json.RawMessage, error) {
	k := key(ft, locationCode)
	val, err := r.rdb.Get(ctx, k).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis GET %s: %w", k, err)
	}
	return json.RawMessage(val), nil
}
