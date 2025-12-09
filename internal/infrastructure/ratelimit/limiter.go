package ratelimit

import (
	"context"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// RateLimitInfo captures limiter response metadata.
type RateLimitInfo struct {
	Allowed   bool
	Limit     int
	Remaining int
	Reset     time.Time
}

// Limiter defines common interface.
type Limiter interface {
	Allow(ctx context.Context, key string) (RateLimitInfo, error)
}

// MemoryLimiter implements a leaky bucket per key.
type MemoryLimiter struct {
	limit int
	burst int
	store map[string]*bucket
	mu    sync.Mutex
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewMemoryLimiter builds RAM limiter.
func NewMemoryLimiter(limit, burst int) *MemoryLimiter {
	return &MemoryLimiter{
		limit: limit,
		burst: burst,
		store: make(map[string]*bucket),
	}
}

// Allow implements limiter.
func (m *MemoryLimiter) Allow(ctx context.Context, key string) (RateLimitInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	b, ok := m.store[key]
	if !ok {
		b = &bucket{tokens: float64(m.limit + m.burst - 1), last: now}
		m.store[key] = b
		return RateLimitInfo{Allowed: true, Limit: m.limit, Remaining: m.limit - 1, Reset: now.Add(time.Minute)}, nil
	}
	delta := now.Sub(b.last).Minutes()
	b.tokens = min(float64(m.limit+m.burst), b.tokens+delta*float64(m.limit))
	if b.tokens >= 1 {
		b.tokens -= 1
		b.last = now
		return RateLimitInfo{Allowed: true, Limit: m.limit, Remaining: int(b.tokens), Reset: now.Add(time.Minute)}, nil
	}
	return RateLimitInfo{Allowed: false, Limit: m.limit, Remaining: 0, Reset: now.Add(time.Minute)}, nil
}

// RedisLimiter coordinates distributed throttling.
type RedisLimiter struct {
	client *redis.Client
	limit  int
	prefix string
}

// NewRedisLimiter builds redis limiter.
func NewRedisLimiter(client *redis.Client, limit int, prefix string) *RedisLimiter {
	return &RedisLimiter{client: client, limit: limit, prefix: prefix}
}

// Allow implements limiter.
func (r *RedisLimiter) Allow(ctx context.Context, key string) (RateLimitInfo, error) {
	redisKey := r.prefix + ":" + key
	remaining, err := r.client.Decr(ctx, redisKey).Result()
	if err == redis.Nil {
		pipe := r.client.TxPipeline()
		pipe.Set(ctx, redisKey, r.limit-1, time.Minute)
		_, err = pipe.Exec(ctx)
		if err != nil {
			return RateLimitInfo{Allowed: false, Limit: r.limit, Remaining: 0, Reset: time.Now().Add(time.Minute)}, err
		}
		return RateLimitInfo{Allowed: true, Limit: r.limit, Remaining: r.limit - 1, Reset: time.Now().Add(time.Minute)}, nil
	}
	if err != nil {
		return RateLimitInfo{}, err
	}
	if remaining >= 0 {
		return RateLimitInfo{Allowed: true, Limit: r.limit, Remaining: int(remaining), Reset: time.Now().Add(time.Minute)}, nil
	}
	return RateLimitInfo{Allowed: false, Limit: r.limit, Remaining: 0, Reset: time.Now().Add(time.Minute)}, nil
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
