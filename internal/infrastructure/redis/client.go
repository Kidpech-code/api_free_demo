package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"api_free_demo/internal/config"
)

// Client wraps *redis.Client with health-check and lifecycle helpers.
type Client struct {
	rdb    *redis.Client
	logger *zap.Logger
}

// NewClient creates and validates a Redis connection.
//
// Connection priority:
//  1. cfg.URL (REDIS_URL env var) — parsed via redis.ParseURL which handles
//     redis:// (plain TCP) and rediss:// (TLS) schemes, embedded credentials,
//     and database path.  This is the format provided by Railway, Render, Heroku, etc.
//  2. cfg.Addr + cfg.Password + cfg.DB — individual env vars for self-hosted Redis.
func NewClient(cfg config.RedisConfig, logger *zap.Logger) (*Client, error) {
	var opts *redis.Options

	if cfg.URL != "" {
		// ── URL-based connection (Railway / managed providers) ──────────────
		// redis.ParseURL handles all URL formats including rediss:// (TLS),
		// userinfo credentials, and numeric database path.
		parsed, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
		}
		opts = parsed
		logger.Info("redis: using REDIS_URL", zap.String("addr", opts.Addr), zap.Int("db", opts.DB))
	} else {
		// ── Individual-field connection (local / self-hosted) ────────────────
		opts = &redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		}
		logger.Info("redis: using REDIS_ADDR", zap.String("addr", cfg.Addr), zap.Int("db", cfg.DB))
	}

	// ── Apply production pool settings regardless of connection source ──────
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second
	opts.PoolSize = 50
	opts.MinIdleConns = 10

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Client{rdb: rdb, logger: logger}, nil
}

// RDB exposes the underlying *redis.Client for the repository layer.
func (c *Client) RDB() *redis.Client {
	return c.rdb
}

// Health returns nil if Redis is reachable.
func (c *Client) Health(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close gracefully shuts down the connection pool.
func (c *Client) Close() error {
	c.logger.Info("closing redis connection")
	return c.rdb.Close()
}
