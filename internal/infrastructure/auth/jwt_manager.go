package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"

	"github.com/kidpech/api_free_demo/internal/config"
	"github.com/kidpech/api_free_demo/internal/domain/user"
)

// Claims extends JWT registered claims with app metadata.
type Claims struct {
	UserID    uuid.UUID `json:"user_id"`
	Role      string    `json:"role"`
	SecretVer string    `json:"sv"`
	TokenType string    `json:"type"`
	jwt.RegisteredClaims
}

// Manager issues and validates JWT pairs.
type Manager struct {
	cfg          config.AuthConfig
	redis        *redis.Client
	memoryTokens sync.Map
}

// NewManager builds Manager.
func NewManager(cfg config.AuthConfig, redisClient *redis.Client) *Manager {
	return &Manager{cfg: cfg, redis: redisClient}
}

// IssueTokens issues access + refresh pair.
func (m *Manager) IssueTokens(ctx context.Context, u *user.User) (user.AuthTokens, error) {
	access, exp, err := m.issueAccess(u)
	if err != nil {
		return user.AuthTokens{}, err
	}
	refresh, refreshClaims, err := m.issueRefresh(u)
	if err != nil {
		return user.AuthTokens{}, err
	}
	if err := m.persistRefresh(ctx, refreshClaims, refresh); err != nil {
		return user.AuthTokens{}, err
	}
	return user.AuthTokens{AccessToken: access, RefreshToken: refresh, ExpiresIn: exp, TokenType: "Bearer"}, nil
}

// RefreshTokens rotates refresh tokens.
func (m *Manager) RefreshTokens(ctx context.Context, u *user.User, token string) (user.AuthTokens, error) {
	claims, err := m.parse(token, m.cfg.RefreshSecret)
	if err != nil {
		return user.AuthTokens{}, err
	}
	if claims.UserID != u.ID || claims.TokenType != "refresh" {
		return user.AuthTokens{}, errors.New("invalid refresh token")
	}
	if err := m.ensureRefreshValid(ctx, claims, token); err != nil {
		return user.AuthTokens{}, err
	}
	access, exp, err := m.issueAccess(u)
	if err != nil {
		return user.AuthTokens{}, err
	}
	newRefresh, refreshClaims, err := m.issueRefresh(u)
	if err != nil {
		return user.AuthTokens{}, err
	}
	if err := m.persistRefresh(ctx, refreshClaims, newRefresh); err != nil {
		return user.AuthTokens{}, err
	}
	_ = m.revoke(ctx, claims)
	return user.AuthTokens{AccessToken: access, RefreshToken: newRefresh, ExpiresIn: exp, TokenType: "Bearer"}, nil
}

// ParseAccessToken validates and extracts claims.
func (m *Manager) ParseAccessToken(token string) (*Claims, error) {
	return m.parse(token, m.cfg.AccessSecret)
}

// ExtractUserID parses refresh token and returns subject id.
func (m *Manager) ExtractUserID(refreshToken string) (uuid.UUID, error) {
	claims, err := m.parse(refreshToken, m.cfg.RefreshSecret)
	if err != nil {
		return uuid.Nil, err
	}
	return claims.UserID, nil
}

func (m *Manager) issueAccess(u *user.User) (string, int64, error) {
	now := time.Now().UTC()
	expires := now.Add(m.cfg.AccessTokenTTL)
	claims := Claims{
		UserID:    u.ID,
		Role:      u.Role,
		SecretVer: m.cfg.SecretVersion,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.cfg.TokenIssuer,
			Subject:   u.ID.String(),
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	encoded, err := tkn.SignedString([]byte(m.cfg.AccessSecret))
	if err != nil {
		return "", 0, err
	}
	return encoded, int64(m.cfg.AccessTokenTTL.Seconds()), nil
}

func (m *Manager) issueRefresh(u *user.User) (string, *Claims, error) {
	now := time.Now().UTC()
	expires := now.Add(m.cfg.RefreshTokenTTL)
	claims := &Claims{
		UserID:    u.ID,
		Role:      u.Role,
		SecretVer: m.cfg.SecretVersion,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.cfg.TokenIssuer,
			Subject:   u.ID.String(),
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	encoded, err := tkn.SignedString([]byte(m.cfg.RefreshSecret))
	if err != nil {
		return "", nil, err
	}
	return encoded, claims, nil
}

func (m *Manager) parse(token, secret string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.SecretVer != m.cfg.SecretVersion {
		return nil, errors.New("token version mismatch")
	}
	return claims, nil
}

func (m *Manager) persistRefresh(ctx context.Context, claims *Claims, token string) error {
	key := m.refreshKey(claims.RegisteredClaims.ID)
	if m.redis != nil {
		return m.redis.Set(ctx, key, token, m.cfg.RefreshTokenTTL).Err()
	}
	m.memoryTokens.Store(key, token)
	return nil
}

func (m *Manager) ensureRefreshValid(ctx context.Context, claims *Claims, token string) error {
	key := m.refreshKey(claims.RegisteredClaims.ID)
	if m.redis != nil {
		val, err := m.redis.Get(ctx, key).Result()
		if err != nil {
			return err
		}
		if val != token {
			return errors.New("refresh token revoked")
		}
		return nil
	}
	if val, ok := m.memoryTokens.Load(key); ok {
		if val.(string) != token {
			return errors.New("refresh token revoked")
		}
		return nil
	}
	return errors.New("refresh token missing")
}

func (m *Manager) revoke(ctx context.Context, claims *Claims) error {
	key := m.refreshKey(claims.RegisteredClaims.ID)
	if m.redis != nil {
		return m.redis.Del(ctx, key).Err()
	}
	m.memoryTokens.Delete(key)
	return nil
}

func (m *Manager) refreshKey(id string) string {
	return "refresh:" + id
}
