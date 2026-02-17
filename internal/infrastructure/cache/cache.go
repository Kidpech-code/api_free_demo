// Package cache implements an integrated cache layer inspired by Uber's CacheFront.
//
// Architecture (from ByteByteGo / Uber Engineering):
//   - Cache-Aside Pattern: Check Redis first, DB on miss, populate cache
//   - Write-Through Invalidation: Invalidate cache synchronously on writes
//   - TTL Expiration: Default 5-minute TTL as safety backstop
//   - Negative Caching: Cache "not found" to prevent DB hammering
//   - Singleflight: Prevent cache stampede (thundering herd)
//   - Circuit Breaker: Degrade gracefully when Redis is unhealthy
//   - Metrics: Prometheus counters for hit/miss/error rates
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/kidpech/api_free_demo/internal/infrastructure/monitoring"
)

// Sentinel errors.
var (
	ErrCacheMiss = errors.New("cache: miss")
	ErrNegative  = errors.New("cache: negative entry (not found)")
)

// negativeEntry is stored in Redis to represent a "known not found" result.
const negativeEntry = "__NEGATIVE__"

// DefaultTTL mirrors Uber's recommended 5-minute default.
const DefaultTTL = 5 * time.Minute

// NegativeTTL is shorter to allow faster recovery from "not found".
const NegativeTTL = 30 * time.Second

// Layer provides an integrated caching abstraction over Redis.
// It implements the Cache-Aside pattern with singleflight dedup,
// negative caching, and circuit breaker integration.
type Layer struct {
	client *redis.Client
	logger *zap.Logger
	group  singleflight.Group
	prefix string
	ttl    time.Duration

	// Circuit breaker state (simple implementation)
	mu           sync.RWMutex
	failures     int
	lastFailure  time.Time
	cbOpen       bool
	cbThreshold  int
	cbResetAfter time.Duration
}

// Config for cache layer.
type Config struct {
	Prefix       string
	TTL          time.Duration
	CBThreshold  int           // failures before circuit opens
	CBResetAfter time.Duration // time before trying again
}

// NewLayer creates a new cache layer.
func NewLayer(client *redis.Client, logger *zap.Logger, cfg Config) *Layer {
	if cfg.TTL == 0 {
		cfg.TTL = DefaultTTL
	}
	if cfg.CBThreshold == 0 {
		cfg.CBThreshold = 5
	}
	if cfg.CBResetAfter == 0 {
		cfg.CBResetAfter = 30 * time.Second
	}
	return &Layer{
		client:       client,
		logger:       logger,
		prefix:       cfg.Prefix,
		ttl:          cfg.TTL,
		cbThreshold:  cfg.CBThreshold,
		cbResetAfter: cfg.CBResetAfter,
	}
}

// Get retrieves a value from cache.
// Returns ErrCacheMiss on miss, ErrNegative for negative entries.
func (l *Layer) Get(ctx context.Context, key string) ([]byte, error) {
	if l == nil || l.client == nil || l.isCircuitOpen() {
		monitoring.CacheMiss(l.prefix)
		return nil, ErrCacheMiss
	}

	fullKey := l.key(key)
	val, err := l.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			monitoring.CacheMiss(l.prefix)
			return nil, ErrCacheMiss
		}
		l.recordFailure()
		monitoring.CacheError(l.prefix)
		l.logger.Warn("cache get error", zap.String("key", fullKey), zap.Error(err))
		return nil, ErrCacheMiss
	}

	// Check for negative entry
	if string(val) == negativeEntry {
		monitoring.CacheHit(l.prefix)
		return nil, ErrNegative
	}

	monitoring.CacheHit(l.prefix)
	l.resetFailures()
	return val, nil
}

// Set stores a value in cache with TTL.
func (l *Layer) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
	if l == nil || l.client == nil || l.isCircuitOpen() {
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}

	expiry := l.ttl
	if len(ttl) > 0 {
		expiry = ttl[0]
	}

	fullKey := l.key(key)
	if err := l.client.Set(ctx, fullKey, data, expiry).Err(); err != nil {
		l.recordFailure()
		monitoring.CacheError(l.prefix)
		l.logger.Warn("cache set error", zap.String("key", fullKey), zap.Error(err))
		return err
	}

	l.resetFailures()
	return nil
}

// SetNegative caches a "not found" result to prevent DB hammering.
func (l *Layer) SetNegative(ctx context.Context, key string) error {
	if l == nil || l.client == nil || l.isCircuitOpen() {
		return nil
	}
	fullKey := l.key(key)
	return l.client.Set(ctx, fullKey, negativeEntry, NegativeTTL).Err()
}

// Invalidate removes one or more cache keys (synchronous write-through invalidation).
func (l *Layer) Invalidate(ctx context.Context, keys ...string) error {
	if l == nil || l.client == nil {
		return nil
	}

	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = l.key(k)
	}

	if err := l.client.Del(ctx, fullKeys...).Err(); err != nil {
		l.logger.Warn("cache invalidate error", zap.Strings("keys", fullKeys), zap.Error(err))
		return err
	}

	monitoring.CacheInvalidation(l.prefix, len(keys))
	return nil
}

// InvalidatePattern removes keys matching a glob pattern (e.g., "user:*").
func (l *Layer) InvalidatePattern(ctx context.Context, pattern string) error {
	if l == nil || l.client == nil {
		return nil
	}

	fullPattern := l.key(pattern)
	var cursor uint64
	for {
		keys, nextCursor, err := l.client.Scan(ctx, cursor, fullPattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			l.client.Del(ctx, keys...)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// GetOrLoad implements cache-aside with singleflight dedup.
// If the cache misses, it calls loader ONCE (even under concurrent requests)
// and populates the cache with the result.
func (l *Layer) GetOrLoad(ctx context.Context, key string, dest interface{}, loader func() (interface{}, error)) error {
	// Try cache first
	data, err := l.Get(ctx, key)
	if err == nil {
		return json.Unmarshal(data, dest)
	}
	if errors.Is(err, ErrNegative) {
		return ErrNegative
	}

	// Singleflight: coalesce concurrent requests for the same key
	result, sErr, _ := l.group.Do(key, func() (interface{}, error) {
		// Double-check cache (another goroutine may have populated it)
		if data2, err2 := l.Get(ctx, key); err2 == nil {
			return data2, nil
		}

		// Load from source (database)
		val, loadErr := loader()
		if loadErr != nil {
			return nil, loadErr
		}

		// Populate cache asynchronously (don't block the response)
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if val == nil {
				_ = l.SetNegative(cacheCtx, key)
			} else {
				_ = l.Set(cacheCtx, key, val)
			}
		}()

		// Marshal for return
		marshaled, mErr := json.Marshal(val)
		if mErr != nil {
			return nil, mErr
		}
		return marshaled, nil
	})
	if sErr != nil {
		return sErr
	}

	return json.Unmarshal(result.([]byte), dest)
}

// key prefixes the cache key.
func (l *Layer) key(k string) string {
	if l.prefix == "" {
		return k
	}
	return l.prefix + ":" + k
}

// Circuit breaker helpers (lightweight implementation).

func (l *Layer) isCircuitOpen() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if !l.cbOpen {
		return false
	}
	// Check if reset period has passed
	if time.Since(l.lastFailure) > l.cbResetAfter {
		return false // allow half-open attempt
	}
	return true
}

func (l *Layer) recordFailure() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures++
	l.lastFailure = time.Now()
	if l.failures >= l.cbThreshold {
		l.cbOpen = true
		if l.logger != nil {
			l.logger.Warn("cache circuit breaker OPEN",
				zap.Int("failures", l.failures),
				zap.Duration("reset_after", l.cbResetAfter))
		}
	}
}

func (l *Layer) resetFailures() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failures > 0 {
		l.failures = 0
		if l.cbOpen {
			l.cbOpen = false
			if l.logger != nil {
				l.logger.Info("cache circuit breaker CLOSED (recovered)")
			}
		}
	}
}

// IsHealthy returns whether the cache is operational.
func (l *Layer) IsHealthy(ctx context.Context) bool {
	if l == nil || l.client == nil {
		return false
	}
	return l.client.Ping(ctx).Err() == nil
}
