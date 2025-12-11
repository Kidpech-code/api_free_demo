package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors for deterministic HTTP mapping.
var (
	ErrDuplicateEmail       = errors.New("email already registered")
	ErrInvalidCreds         = errors.New("invalid credentials")
	ErrForbidden            = errors.New("forbidden")
	ErrUserNotFound         = errors.New("user not found")
	ErrInvalidToken         = errors.New("invalid token")
	ErrRegistrationDisabled = errors.New("registration disabled")
)

// TokenManager abstracts JWT/refresh issuance.
type TokenManager interface {
	IssueTokens(ctx context.Context, user *User) (AuthTokens, error)
	RefreshTokens(ctx context.Context, user *User, refreshToken string) (AuthTokens, error)
	ExtractUserID(refreshToken string) (uuid.UUID, error)
}

// Service encapsulates user orchestration.
type Service struct {
	repo        Repository
	tokens      TokenManager
	validator   *validator.Validate
	sanitizer   *bluemonday.Policy
	logger      *zap.Logger
	allowSignup bool
}

// NewService wires a Service.
func NewService(repo Repository, tokens TokenManager, logger *zap.Logger, allowSignup bool) *Service {
	return &Service{
		repo:        repo,
		tokens:      tokens,
		validator:   validator.New(),
		sanitizer:   bluemonday.UGCPolicy(),
		logger:      logger,
		allowSignup: allowSignup,
	}
}

// Register creates a new user and immediately issues tokens.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	if !s.allowSignup {
		return nil, ErrRegistrationDisabled
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)
	req.Name = strings.TrimSpace(s.sanitizer.Sanitize(req.Name))
	if err := s.validator.Struct(req); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByEmail(ctx, strings.ToLower(req.Email))
	if err == nil && existing != nil {
		return nil, ErrDuplicateEmail
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	user := &User{
		ID:             uuid.New(),
		Email:          strings.ToLower(req.Email),
		Name:           req.Name,
		PasswordHash:   string(hash),
		Role:           "user",
		RefreshVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if req.ProfileImage != "" {
		user.ProfileImage = &req.ProfileImage
	}

	if err := s.repo.Create(ctx, user); err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}

	tokens, err := s.tokens.IssueTokens(ctx, user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{User: user, Tokens: tokens}, nil
}

// Login authenticates by email/password.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)
	if err := s.validator.Struct(req); err != nil {
		return nil, err
	}

	user, err := s.repo.GetByEmail(ctx, strings.ToLower(req.Email))
	if err != nil || user == nil {
		return nil, ErrInvalidCreds
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCreds
	}

	now := time.Now().UTC()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	_ = s.repo.Update(ctx, user)

	tokens, err := s.tokens.IssueTokens(ctx, user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{User: user, Tokens: tokens}, nil
}

// Refresh issues new tokens based on a refresh token.

// Refresh uses refresh token to rotate credentials.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	userID, err := s.tokens.ExtractUserID(refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, ErrInvalidToken
	}
	tokens, err := s.tokens.RefreshTokens(ctx, user, refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return &AuthResponse{User: user, Tokens: tokens}, nil
}

// GetMe returns the authed profile.
func (s *Service) GetMe(ctx context.Context, userID uuid.UUID) (*User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// UpdateMe mutates the authed user.
func (s *Service) UpdateMe(ctx context.Context, userID uuid.UUID, req UpdateUserRequest) (*User, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, err
	}
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	user.Name = strings.TrimSpace(s.sanitizer.Sanitize(req.Name))
	if req.ProfileImage != nil {
		user.ProfileImage = req.ProfileImage
	}
	user.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// List returns paginated users for admin dashboards.
func (s *Service) List(ctx context.Context, filter UserFilter) ([]User, int, error) {
	return s.repo.List(ctx, filter)
}
