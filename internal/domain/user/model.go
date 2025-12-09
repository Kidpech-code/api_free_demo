package user

import (
	"time"

	"github.com/google/uuid"
)

// User represents the persisted user entity.
type User struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	Email            string     `json:"email" db:"email"`
	Name             string     `json:"name" db:"name"`
	PasswordHash     string     `json:"-" db:"password_hash"`
	ProfileImage     *string    `json:"profile_image,omitempty" db:"profile_image"`
	Role             string     `json:"role" db:"role"`
	RefreshVersion   int        `json:"-" db:"refresh_version"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	PasswordResetAt  *time.Time `json:"-" db:"password_reset_at"`
	LastPasswordHash string     `json:"-" db:"last_password_hash"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt        *time.Time `json:"-" db:"deleted_at"`
}

// RegisterRequest captures incoming registration payloads.
type RegisterRequest struct {
	Email        string `json:"email" validate:"required,email"`
	Password     string `json:"password" validate:"required,min=8"`
	Name         string `json:"name" validate:"required,min=2"`
	ProfileImage string `json:"profile_image" validate:"omitempty,url"`
}

// LoginRequest models the login payload.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// UpdateUserRequest updates the current user profile.
type UpdateUserRequest struct {
	Name         string  `json:"name" validate:"required,min=2"`
	ProfileImage *string `json:"profile_image" validate:"omitempty,url"`
}

// UserFilter encapsulates pagination and filter params for administrative listings.
type UserFilter struct {
	Search   string
	Limit    int
	Offset   int
	Sort     string
	TenantID uuid.UUID
}

// AuthTokens groups issued tokens.
type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// AuthResponse returns user info plus tokens.
type AuthResponse struct {
	User   *User      `json:"user"`
	Tokens AuthTokens `json:"tokens"`
}
