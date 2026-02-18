package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"api_free_demo/internal/domain/model"
	domainRepo "api_free_demo/internal/domain/repository"
	"api_free_demo/internal/infrastructure/metrics"
)

// Redis Key Schema (Uber DocStore - Partition Key Design)
//
//   Data key :  sandbox:{user_id}:product:{id}         -> JSON string
//   Index key:  sandbox:{user_id}:product:_index        -> ZSET (score=timestamp)
//   Cache key:  sandbox:{user_id}:product:_cache:{id}   -> JSON string (hot-cache)

const (
	keyPrefix    = "sandbox"
	resourceName = "product"
	cacheTTL     = 5 * time.Minute
	defaultLimit = 20
	maxLimit     = 100
)

type productRepository struct {
	rdb     *redis.Client
	dataTTL time.Duration
	metrics *metrics.Metrics
	logger  *zap.Logger
}

var _ domainRepo.ProductRepository = (*productRepository)(nil)

func NewProductRepository(
	rdb *redis.Client,
	dataTTL time.Duration,
	m *metrics.Metrics,
	logger *zap.Logger,
) domainRepo.ProductRepository {
	return &productRepository{
		rdb:     rdb,
		dataTTL: dataTTL,
		metrics: m,
		logger:  logger.Named("docstore.product"),
	}
}

// Key helpers

func (r *productRepository) dataKey(userID, id string) string {
	return fmt.Sprintf("%s:%s:%s:%s", keyPrefix, userID, resourceName, id)
}

func (r *productRepository) indexKey(userID string) string {
	return fmt.Sprintf("%s:%s:%s:_index", keyPrefix, userID, resourceName)
}

func (r *productRepository) cacheKey(userID, id string) string {
	return fmt.Sprintf("%s:%s:%s:_cache:%s", keyPrefix, userID, resourceName, id)
}

// Create stores a product and adds it to the ZSET index using a pipeline.
func (r *productRepository) Create(ctx context.Context, product *model.Product) error {
	if product.ID == "" {
		product.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	product.CreatedAt = now
	product.UpdatedAt = now
	product.Deleted = false

	data, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("marshal product: %w", err)
	}

	dk := r.dataKey(product.UserID, product.ID)
	ik := r.indexKey(product.UserID)
	score := float64(now.UnixNano())

	pipe := r.rdb.Pipeline()
	pipe.Set(ctx, dk, data, r.dataTTL)
	pipe.ZAdd(ctx, ik, redis.Z{Score: score, Member: product.ID})
	pipe.Expire(ctx, ik, r.dataTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("pipeline create: %w", err)
	}

	r.metrics.StorageOps.WithLabelValues("create").Inc()
	r.logger.Debug("product created",
		zap.String("user_id", product.UserID),
		zap.String("product_id", product.ID),
	)
	return nil
}

// FindByID implements Integrated Cache read path:
//  1. Hot-cache lookup (_cache key)   -> cache hit
//  2. Miss -> data key lookup         -> storage read
//  3. Back-fill hot-cache             -> write-through
func (r *productRepository) FindByID(ctx context.Context, userID, id string) (*model.Product, error) {
	ck := r.cacheKey(userID, id)

	// Step 1: hot-cache
	cached, err := r.rdb.Get(ctx, ck).Bytes()
	if err == nil {
		var p model.Product
		if err := json.Unmarshal(cached, &p); err == nil && !p.Deleted {
			r.metrics.CacheHits.Inc()
			r.logger.Debug("cache hit", zap.String("id", id))
			return &p, nil
		}
	}

	// Step 2: storage
	r.metrics.CacheMisses.Inc()
	dk := r.dataKey(userID, id)
	raw, err := r.rdb.Get(ctx, dk).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var p model.Product
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal product: %w", err)
	}
	if p.Deleted {
		return nil, nil
	}

	// Step 3: back-fill cache
	r.rdb.Set(ctx, ck, raw, cacheTTL)

	r.metrics.StorageOps.WithLabelValues("read").Inc()
	r.logger.Debug("storage read + cache backfill", zap.String("id", id))
	return &p, nil
}

// Update overwrites data key and invalidates cache key (Write-Invalidate).
func (r *productRepository) Update(ctx context.Context, product *model.Product) error {
	product.UpdatedAt = time.Now().UTC()

	data, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("marshal product: %w", err)
	}

	dk := r.dataKey(product.UserID, product.ID)
	ck := r.cacheKey(product.UserID, product.ID)

	pipe := r.rdb.Pipeline()
	pipe.Set(ctx, dk, data, r.dataTTL)
	pipe.Del(ctx, ck)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("pipeline update: %w", err)
	}

	r.metrics.StorageOps.WithLabelValues("update").Inc()
	return nil
}

// SoftDelete sets the Deleted flag and removes from index.
func (r *productRepository) SoftDelete(ctx context.Context, userID, id string) error {
	p, err := r.findRaw(ctx, userID, id)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("product %s not found", id)
	}

	p.Deleted = true
	p.UpdatedAt = time.Now().UTC()

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	dk := r.dataKey(userID, id)
	ck := r.cacheKey(userID, id)

	pipe := r.rdb.Pipeline()
	pipe.Set(ctx, dk, data, r.dataTTL)
	pipe.Del(ctx, ck)
	pipe.ZRem(ctx, r.indexKey(userID), id)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("pipeline soft-delete: %w", err)
	}

	r.metrics.StorageOps.WithLabelValues("delete").Inc()
	return nil
}

// HardDelete permanently removes data, cache, and index entry.
func (r *productRepository) HardDelete(ctx context.Context, userID, id string) error {
	dk := r.dataKey(userID, id)
	ck := r.cacheKey(userID, id)
	ik := r.indexKey(userID)

	pipe := r.rdb.Pipeline()
	pipe.Del(ctx, dk)
	pipe.Del(ctx, ck)
	pipe.ZRem(ctx, ik, id)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("pipeline hard-delete: %w", err)
	}

	r.metrics.StorageOps.WithLabelValues("delete").Inc()
	return nil
}

// BulkCreate inserts multiple products in a single Redis pipeline.
func (r *productRepository) BulkCreate(ctx context.Context, products []model.Product) (int, error) {
	if len(products) == 0 {
		return 0, nil
	}

	now := time.Now().UTC()
	pipe := r.rdb.Pipeline()

	for i := range products {
		p := &products[i]
		if p.ID == "" {
			p.ID = uuid.New().String()
		}
		p.CreatedAt = now
		p.UpdatedAt = now
		p.Deleted = false

		data, err := json.Marshal(p)
		if err != nil {
			return 0, fmt.Errorf("marshal product[%d]: %w", i, err)
		}

		dk := r.dataKey(p.UserID, p.ID)
		ik := r.indexKey(p.UserID)
		score := float64(now.UnixNano()) + float64(i)

		pipe.Set(ctx, dk, data, r.dataTTL)
		pipe.ZAdd(ctx, ik, redis.Z{Score: score, Member: p.ID})
		pipe.Expire(ctx, ik, r.dataTTL)
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, fmt.Errorf("pipeline bulk-create: %w", err)
	}

	created := 0
	for i := 0; i < len(cmds); i += 3 {
		if cmds[i].Err() == nil {
			created++
		}
	}

	r.metrics.StorageOps.WithLabelValues("create").Add(float64(created))
	r.logger.Info("bulk create completed",
		zap.Int("requested", len(products)),
		zap.Int("created", created),
	)
	return created, nil
}

// BulkFindByIDs fetches multiple products in a single pipeline.
func (r *productRepository) BulkFindByIDs(ctx context.Context, userID string, ids []string) ([]model.Product, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Pipeline 1: try hot-cache
	pipe := r.rdb.Pipeline()
	cmds := make([]*redis.StringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.Get(ctx, r.cacheKey(userID, id))
	}
	pipe.Exec(ctx) //nolint:errcheck

	products := make([]model.Product, 0, len(ids))
	missedIndices := make([]int, 0)

	for i, cmd := range cmds {
		raw, err := cmd.Bytes()
		if err == nil {
			var p model.Product
			if json.Unmarshal(raw, &p) == nil && !p.Deleted {
				r.metrics.CacheHits.Inc()
				products = append(products, p)
				continue
			}
		}
		missedIndices = append(missedIndices, i)
	}

	// Pipeline 2: storage read for misses
	if len(missedIndices) > 0 {
		pipe2 := r.rdb.Pipeline()
		storageCmds := make([]*redis.StringCmd, len(missedIndices))
		for j, idx := range missedIndices {
			storageCmds[j] = pipe2.Get(ctx, r.dataKey(userID, ids[idx]))
		}
		pipe2.Exec(ctx) //nolint:errcheck

		// Pipeline 3: back-fill cache
		pipe3 := r.rdb.Pipeline()
		for j, cmd := range storageCmds {
			raw, err := cmd.Bytes()
			if err != nil {
				continue
			}
			var p model.Product
			if json.Unmarshal(raw, &p) != nil || p.Deleted {
				continue
			}
			products = append(products, p)
			pipe3.Set(ctx, r.cacheKey(userID, ids[missedIndices[j]]), raw, cacheTTL)
			r.metrics.CacheMisses.Inc()
		}
		pipe3.Exec(ctx) //nolint:errcheck
	}

	r.metrics.StorageOps.WithLabelValues("read").Add(float64(len(missedIndices)))
	return products, nil
}

// List uses ZSET cursor-based pagination (score = unix nano timestamp).
func (r *productRepository) List(ctx context.Context, userID string, filter model.ProductFilter) (*model.ProductPage, error) {
	ik := r.indexKey(userID)
	limit := normalizeLimit(filter.Limit)

	maxScore := "+inf"
	if filter.Cursor != "" {
		maxScore = "(" + filter.Cursor // exclusive
	}

	ids, err := r.rdb.ZRevRangeByScore(ctx, ik, &redis.ZRangeBy{
		Max:    maxScore,
		Min:    "-inf",
		Offset: 0,
		Count:  int64(limit + 1),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrangebyscore: %w", err)
	}

	hasMore := len(ids) > limit
	if hasMore {
		ids = ids[:limit]
	}

	if len(ids) == 0 {
		return &model.ProductPage{Items: []model.Product{}, HasMore: false}, nil
	}

	products, err := r.BulkFindByIDs(ctx, userID, ids)
	if err != nil {
		return nil, err
	}

	var nextCursor string
	if hasMore && len(ids) > 0 {
		lastID := ids[len(ids)-1]
		score, err := r.rdb.ZScore(ctx, ik, lastID).Result()
		if err == nil {
			nextCursor = strconv.FormatFloat(score, 'f', -1, 64)
		}
	}

	r.metrics.StorageOps.WithLabelValues("list").Inc()
	return &model.ProductPage{
		Items:      products,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// Count returns the ZSET cardinality (active products).
func (r *productRepository) Count(ctx context.Context, userID string) (int64, error) {
	return r.rdb.ZCard(ctx, r.indexKey(userID)).Result()
}

// findRaw reads from data key directly (bypassing cache).
func (r *productRepository) findRaw(ctx context.Context, userID, id string) (*model.Product, error) {
	raw, err := r.rdb.Get(ctx, r.dataKey(userID, id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get raw: %w", err)
	}
	var p model.Product
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &p, nil
}

func normalizeLimit(l int) int {
	if l <= 0 {
		return defaultLimit
	}
	if l > maxLimit {
		return maxLimit
	}
	return l
}
