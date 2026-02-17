package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/kidpech/api_free_demo/internal/config"
)

// Client wraps the redis connection plus metadata.
type Client struct {
	Native *redis.Client
}

// Connect instantiates redis client.
func Connect(cfg config.RedisConfig, logger *zap.Logger) (*Client, error) {
	var options *redis.Options

	// Railway provides REDIS_URL as redis://... — parse it automatically
	if strings.HasPrefix(cfg.Addr, "redis://") || strings.HasPrefix(cfg.Addr, "rediss://") {
		var err error
		options, err = redis.ParseURL(cfg.Addr)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
	} else {
		options = &redis.Options{
			Addr:     cfg.Addr,
			Username: cfg.Username,
			Password: cfg.Password,
			DB:       cfg.DB,
		}
	}

	if cfg.TLS {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		if logger != nil {
			logger.Warn("redis ping failed", zap.Error(err))
		}
		return nil, err
	}

	return &Client{Native: client}, nil
}

// Close redis connection.
func (c *Client) Close() error {
	if c == nil || c.Native == nil {
		return nil
	}
	return c.Native.Close()
}
