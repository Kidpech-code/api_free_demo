package cache

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/kidpech/api_free_demo/internal/domain/user"
)

// UserRepository wraps a user.Repository with cache-aside caching.
// Mirrors Uber's CacheFront: read from cache, fallback to DB, invalidate on write.
type UserRepository struct {
	inner user.Repository
	cache *Layer
}

// NewUserRepository creates a cached user repository decorator.
func NewUserRepository(inner user.Repository, cache *Layer) user.Repository {
	if cache == nil {
		return inner // no cache available, passthrough
	}
	return &UserRepository{inner: inner, cache: cache}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	err := r.inner.Create(ctx, u)
	if err != nil {
		return err
	}
	// Write-through: populate cache immediately after successful write
	_ = r.cache.Set(ctx, r.keyByID(u.ID), u)
	_ = r.cache.Set(ctx, r.keyByEmail(u.Email), u)
	// Invalidate list cache since a new user was added
	_ = r.cache.InvalidatePattern(ctx, "user:list:*")
	return nil
}

func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	err := r.inner.Update(ctx, u)
	if err != nil {
		return err
	}
	// Synchronous invalidation on write (Uber's write-path invalidation)
	_ = r.cache.Invalidate(ctx, r.keyByID(u.ID), r.keyByEmail(u.Email))
	_ = r.cache.InvalidatePattern(ctx, "user:list:*")
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	key := r.keyByEmail(email)
	var cached user.User

	err := r.cache.GetOrLoad(ctx, key, &cached, func() (interface{}, error) {
		return r.inner.GetByEmail(ctx, email)
	})
	if err != nil {
		if err == ErrNegative {
			return nil, nil // negative cache: user not found
		}
		return nil, err
	}
	if cached.ID == uuid.Nil {
		return nil, nil
	}
	return &cached, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	key := r.keyByID(id)
	var cached user.User

	err := r.cache.GetOrLoad(ctx, key, &cached, func() (interface{}, error) {
		return r.inner.GetByID(ctx, id)
	})
	if err != nil {
		if err == ErrNegative {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}
	if cached.ID == uuid.Nil {
		return nil, user.ErrUserNotFound
	}
	return &cached, nil
}

func (r *UserRepository) List(ctx context.Context, filter user.UserFilter) ([]user.User, int, error) {
	// List queries bypass cache (complex filters, pagination)
	// Consistent with Uber's approach: cache point lookups, not range queries
	return r.inner.List(ctx, filter)
}

func (r *UserRepository) keyByID(id uuid.UUID) string {
	return fmt.Sprintf("user:id:%s", id.String())
}

func (r *UserRepository) keyByEmail(email string) string {
	return fmt.Sprintf("user:email:%s", email)
}
