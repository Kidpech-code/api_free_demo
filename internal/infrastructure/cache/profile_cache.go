package cache

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/kidpech/api_free_demo/internal/domain/profile"
)

// ProfileRepository wraps a profile.Repository with cache-aside caching.
type ProfileRepository struct {
	inner profile.Repository
	cache *Layer
}

// NewProfileRepository creates a cached profile repository decorator.
func NewProfileRepository(inner profile.Repository, cache *Layer) profile.Repository {
	if cache == nil {
		return inner
	}
	return &ProfileRepository{inner: inner, cache: cache}
}

func (r *ProfileRepository) Create(ctx context.Context, p *profile.Profile) error {
	err := r.inner.Create(ctx, p)
	if err != nil {
		return err
	}
	_ = r.cache.Set(ctx, r.key(p.ID, p.UserID), p)
	_ = r.cache.InvalidatePattern(ctx, fmt.Sprintf("profile:list:%s:*", p.UserID))
	return nil
}

func (r *ProfileRepository) BulkCreate(ctx context.Context, profiles []*profile.Profile) error {
	err := r.inner.BulkCreate(ctx, profiles)
	if err != nil {
		return err
	}
	// Invalidate list caches for all affected users
	seen := make(map[uuid.UUID]bool)
	for _, p := range profiles {
		_ = r.cache.Set(ctx, r.key(p.ID, p.UserID), p)
		if !seen[p.UserID] {
			_ = r.cache.InvalidatePattern(ctx, fmt.Sprintf("profile:list:%s:*", p.UserID))
			seen[p.UserID] = true
		}
	}
	return nil
}

func (r *ProfileRepository) Update(ctx context.Context, p *profile.Profile) error {
	err := r.inner.Update(ctx, p)
	if err != nil {
		return err
	}
	_ = r.cache.Invalidate(ctx, r.key(p.ID, p.UserID))
	_ = r.cache.InvalidatePattern(ctx, fmt.Sprintf("profile:list:%s:*", p.UserID))
	return nil
}

func (r *ProfileRepository) Patch(ctx context.Context, profileID, userID uuid.UUID, fields map[string]interface{}, version int) (*profile.Profile, error) {
	result, err := r.inner.Patch(ctx, profileID, userID, fields, version)
	if err != nil {
		return nil, err
	}
	// Synchronous invalidation after successful write
	_ = r.cache.Invalidate(ctx, r.key(profileID, userID))
	_ = r.cache.InvalidatePattern(ctx, fmt.Sprintf("profile:list:%s:*", userID))
	return result, nil
}

func (r *ProfileRepository) Delete(ctx context.Context, profileID, userID uuid.UUID, hard bool, version int) error {
	err := r.inner.Delete(ctx, profileID, userID, hard, version)
	if err != nil {
		return err
	}
	_ = r.cache.Invalidate(ctx, r.key(profileID, userID))
	_ = r.cache.InvalidatePattern(ctx, fmt.Sprintf("profile:list:%s:*", userID))
	return nil
}

func (r *ProfileRepository) BulkDelete(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, hard bool) (int, error) {
	count, err := r.inner.BulkDelete(ctx, userID, ids, hard)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		_ = r.cache.Invalidate(ctx, r.key(id, userID))
	}
	_ = r.cache.InvalidatePattern(ctx, fmt.Sprintf("profile:list:%s:*", userID))
	return count, nil
}

func (r *ProfileRepository) GetByID(ctx context.Context, profileID, userID uuid.UUID) (*profile.Profile, error) {
	key := r.key(profileID, userID)
	var cached profile.Profile

	err := r.cache.GetOrLoad(ctx, key, &cached, func() (interface{}, error) {
		return r.inner.GetByID(ctx, profileID, userID)
	})
	if err != nil {
		if err == ErrNegative {
			return nil, profile.ErrNotFound
		}
		return nil, err
	}
	if cached.ID == uuid.Nil {
		return nil, profile.ErrNotFound
	}
	return &cached, nil
}

func (r *ProfileRepository) List(ctx context.Context, filter profile.Filter) ([]profile.Profile, int, error) {
	// List queries bypass individual cache — too many permutations
	return r.inner.List(ctx, filter)
}

func (r *ProfileRepository) key(profileID, userID uuid.UUID) string {
	return fmt.Sprintf("profile:%s:%s", userID, profileID)
}
