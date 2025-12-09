package user

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the persistence boundary for users.
type Repository interface {
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	List(ctx context.Context, filter UserFilter) ([]User, int, error)
}
