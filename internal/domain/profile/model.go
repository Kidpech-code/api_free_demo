package profile

import (
	"time"

	"github.com/google/uuid"
)

// Profile models the user profile entity.
type Profile struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	UserID       uuid.UUID  `json:"user_id" db:"user_id"`
	FirstName    string     `json:"first_name" db:"first_name"`
	LastName     string     `json:"last_name" db:"last_name"`
	Bio          *string    `json:"bio,omitempty" db:"bio"`
	ProfileImage *string    `json:"profile_image,omitempty" db:"profile_image"`
	CoverImage   *string    `json:"cover_image,omitempty" db:"cover_image"`
	DateOfBirth  *time.Time `json:"date_of_birth,omitempty" db:"date_of_birth"`
	Phone        *string    `json:"phone,omitempty" db:"phone"`
	Website      *string    `json:"website,omitempty" db:"website"`
	Location     *string    `json:"location,omitempty" db:"location"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	Version      int        `json:"version" db:"version"`
}

// CreateRequest captures POST payloads.
type CreateRequest struct {
	FirstName    string  `json:"first_name" validate:"required"`
	LastName     string  `json:"last_name" validate:"required"`
	Bio          *string `json:"bio"`
	ProfileImage *string `json:"profile_image" validate:"omitempty,url"`
	CoverImage   *string `json:"cover_image" validate:"omitempty,url"`
	DateOfBirth  *string `json:"date_of_birth" validate:"omitempty,datetime=2006-01-02"`
	Phone        *string `json:"phone"`
	Website      *string `json:"website" validate:"omitempty,url"`
	Location     *string `json:"location"`
}

// UpdateRequest handles PUT semantics.
type UpdateRequest struct {
	CreateRequest
	Version int `json:"version" validate:"gte=0"`
}

// PatchRequest handles PATCH semantics.
type PatchRequest struct {
	Fields  map[string]interface{} `json:"fields" validate:"required"`
	Version int                    `json:"version" validate:"gte=0"`
}

// Filter for list endpoints.
type Filter struct {
	Search     string
	Limit      int
	Offset     int
	Cursor     string
	CursorTime *time.Time
	UserID     uuid.UUID
}

// BulkCreateRequest handles profile batch creation.
type BulkCreateRequest struct {
	Profiles []CreateRequest `json:"profiles" validate:"required,dive"`
}

// BulkDeleteRequest handles batch deletions.
type BulkDeleteRequest struct {
	IDs    []uuid.UUID `json:"ids" validate:"required,unique"`
	Hard   bool        `json:"hard_delete"`
	Reason string      `json:"reason"`
}
