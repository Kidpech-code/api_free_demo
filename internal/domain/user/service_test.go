package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRegisterCreatesUser(t *testing.T) {
	repo := newFakeRepo()
	tokens := &fakeTokens{}
	service := NewService(repo, tokens, zap.NewNop(), true)

	resp, err := service.Register(context.Background(), RegisterRequest{
		Email:    "Demo@Example.com",
		Password: "Passw0rd!",
		Name:     "Demo User",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "demo@example.com", resp.User.Email)
	require.Equal(t, resp.User.ID, tokens.userID)
	require.Equal(t, 1, repo.count())
}

func TestRegisterDuplicateEmail(t *testing.T) {
	repo := newFakeRepo()
	tokens := &fakeTokens{}
	service := NewService(repo, tokens, zap.NewNop(), true)

	existing := &User{
		ID:           uuid.New(),
		Email:        "dup@example.com",
		Name:         "Dup User",
		PasswordHash: "hash",
		Role:         "user",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), existing))

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:    "dup@example.com",
		Password: "Passw0rd!",
		Name:     "Dup User",
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrDuplicateEmail))
}

func TestLoginInvalidPassword(t *testing.T) {
	repo := newFakeRepo()
	tokens := &fakeTokens{}
	service := NewService(repo, tokens, zap.NewNop(), true)

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:    "demo2@example.com",
		Password: "Passw0rd!",
		Name:     "Demo",
	})
	require.NoError(t, err)

	_, err = service.Login(context.Background(), LoginRequest{
		Email:    "demo2@example.com",
		Password: "wrongpass",
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidCreds))
}

func TestRefreshSuccess(t *testing.T) {
	repo := newFakeRepo()
	tokens := &fakeTokens{}
	service := NewService(repo, tokens, zap.NewNop(), true)

	registerResp, err := service.Register(context.Background(), RegisterRequest{
		Email:    "demo3@example.com",
		Password: "Passw0rd!",
		Name:     "Demo",
	})
	require.NoError(t, err)

	resp, err := service.Refresh(context.Background(), registerResp.Tokens.RefreshToken)

	require.NoError(t, err)
	require.Equal(t, registerResp.User.ID, resp.User.ID)
}

func TestRefreshInvalidToken(t *testing.T) {
	service := NewService(newFakeRepo(), &fakeTokens{}, zap.NewNop(), true)

	_, err := service.Refresh(context.Background(), "invalid")

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidToken))
}

func TestRegisterDisabled(t *testing.T) {
	service := NewService(newFakeRepo(), &fakeTokens{}, zap.NewNop(), false)

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:    "demo4@example.com",
		Password: "Passw0rd!",
		Name:     "Demo",
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrRegistrationDisabled))
}

type fakeTokens struct {
	userID uuid.UUID
}

func (f *fakeTokens) IssueTokens(ctx context.Context, user *User) (AuthTokens, error) {
	f.userID = user.ID
	return AuthTokens{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 60, TokenType: "Bearer"}, nil
}

func (f *fakeTokens) RefreshTokens(ctx context.Context, user *User, refreshToken string) (AuthTokens, error) {
	if refreshToken != "refresh" {
		return AuthTokens{}, ErrInvalidToken
	}
	f.userID = user.ID
	return AuthTokens{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 60, TokenType: "Bearer"}, nil
}

func (f *fakeTokens) ExtractUserID(refreshToken string) (uuid.UUID, error) {
	if refreshToken != "refresh" || f.userID == uuid.Nil {
		return uuid.Nil, ErrInvalidToken
	}
	return f.userID, nil
}

type fakeUserRepo struct {
	users      map[uuid.UUID]*User
	emailIndex map[string]uuid.UUID
}

func newFakeRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users:      make(map[uuid.UUID]*User),
		emailIndex: make(map[string]uuid.UUID),
	}
}

func (f *fakeUserRepo) Create(ctx context.Context, u *User) error {
	if _, exists := f.emailIndex[u.Email]; exists {
		return ErrDuplicateEmail
	}
	clone := *u
	f.users[u.ID] = &clone
	f.emailIndex[u.Email] = u.ID
	return nil
}

func (f *fakeUserRepo) Update(ctx context.Context, u *User) error {
	if _, ok := f.users[u.ID]; !ok {
		return ErrUserNotFound
	}
	clone := *u
	f.users[u.ID] = &clone
	f.emailIndex[u.Email] = u.ID
	return nil
}

func (f *fakeUserRepo) GetByEmail(ctx context.Context, email string) (*User, error) {
	if id, ok := f.emailIndex[email]; ok {
		clone := *f.users[id]
		return &clone, nil
	}
	return nil, ErrUserNotFound
}

func (f *fakeUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	if user, ok := f.users[id]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, ErrUserNotFound
}

func (f *fakeUserRepo) List(ctx context.Context, filter UserFilter) ([]User, int, error) {
	result := make([]User, 0, len(f.users))
	for _, user := range f.users {
		clone := *user
		result = append(result, clone)
	}
	return result, len(result), nil
}

func (f *fakeUserRepo) count() int {
	return len(f.users)
}
