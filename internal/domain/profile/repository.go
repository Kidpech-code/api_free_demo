package profile

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines persistence needs for profiles.
type Repository interface {
	Create(ctx context.Context, profile *Profile) error
	BulkCreate(ctx context.Context, profiles []*Profile) error
	Update(ctx context.Context, profile *Profile) error
	Patch(ctx context.Context, profileID uuid.UUID, userID uuid.UUID, fields map[string]interface{}, version int) (*Profile, error)
	Delete(ctx context.Context, profileID uuid.UUID, userID uuid.UUID, hard bool, version int) error
	BulkDelete(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, hard bool) (int, error)
	GetByID(ctx context.Context, profileID uuid.UUID, userID uuid.UUID) (*Profile, error)
	List(ctx context.Context, filter Filter) ([]Profile, int, error)
}
