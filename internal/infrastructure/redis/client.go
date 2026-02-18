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
func NewClient(cfg config.RedisConfig, logger *zap.Logger) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     50,
		MinIdleConns: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	logger.Info("redis connected", zap.String("addr", cfg.Addr), zap.Int("db", cfg.DB))

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
