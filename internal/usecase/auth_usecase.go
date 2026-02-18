// Package usecase implements the Auth Lifecycle use-cases.
//
// Design notes (Clean Architecture):
//   - This layer owns ALL business logic. It knows nothing about HTTP.
//   - It depends on Redis (via the go-redis client) only for RefreshToken storage.
//   - JWT signing/validation lives here; middleware only verifies signatures.
//
// ╔══════════════════════════════════════════════════════════════════════╗
// ║                  AUTH LIFECYCLE DIAGRAM                              ║
// ║                                                                      ║
// ║  POST /auth/login                                                    ║
// ║  { user_id, role }  ──────►  Server                                 ║
// ║                    ◄──────  { access_token (15min),                 ║
// ║                               refresh_token (7 days) }              ║
// ║                                                                      ║
// ║  ... AccessToken expires after 15 minutes ...                       ║
// ║                                                                      ║
// ║  POST /auth/refresh                                                  ║
// ║  { user_id, refresh_token }  ──────►  Server → Redis HGETALL        ║
// ║                              ◄──────  { NEW access_token (15min) }  ║
// ║                                                                      ║
// ║  POST /auth/logout  (requires valid AccessToken)                     ║
// ║  Authorization: Bearer <access>  ──►  Server → Redis DEL            ║
// ║                                                                      ║
// ║  Redis Key Schema:                                                   ║
// ║    sandbox:{user_id}:auth:refresh  →  HASH{ token, role }           ║
// ║    TTL: 7 days                                                       ║
// ╚══════════════════════════════════════════════════════════════════════╝
package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"api_free_demo/internal/domain/model"
)

// ── Constants ──────────────────────────────────────────────────────────────

const (
	// AccessTokenTTL is intentionally short so students can observe token
	// expiry and practice implementing auto-refresh on the frontend.
	AccessTokenTTL = 15 * time.Minute

	// RefreshTokenTTL gives a comfortable window for sandbox sessions.
	RefreshTokenTTL = 7 * 24 * time.Hour

	jwtIssuer = "api_free_demo_sandbox"
)

// refreshRedisKey returns the Redis key that stores a user's refresh token data.
//
// Pattern: sandbox:{user_id}:auth:refresh
// Namespaced under sandbox:{user_id}: to keep all per-user data consistently
// grouped — same convention as product keys.
func refreshRedisKey(userID string) string {
	return fmt.Sprintf("sandbox:%s:auth:refresh", userID)
}

// ── Interface ──────────────────────────────────────────────────────────────

// AuthUsecase defines the contract for the auth lifecycle.
// Exporting an interface lets handlers depend on the
// abstraction, making unit testing trivial with a mock.
type AuthUsecase interface {
	Login(ctx context.Context, req *model.LoginRequest) (*model.TokenPair, error)
	Refresh(ctx context.Context, req *model.RefreshRequest) (*model.TokenPair, error)
	Logout(ctx context.Context, userID string) error
}

// ── Sentinel errors ────────────────────────────────────────────────────────

var (
	// ErrInvalidRefreshToken is returned when the provided refresh token does
	// not match the one stored in Redis (wrong value, expired, or revoked).
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")

	// ErrInvalidRole is returned when an unrecognised role is supplied.
	ErrInvalidRole = errors.New("invalid role: must be 'admin' or 'user'")
)

// ── Implementation ─────────────────────────────────────────────────────────

type authUsecase struct {
	rdb    *redis.Client
	secret string
	logger *zap.Logger
}

// NewAuthUsecase constructs the production AuthUsecase.
func NewAuthUsecase(rdb *redis.Client, secret string, logger *zap.Logger) AuthUsecase {
	return &authUsecase{rdb: rdb, secret: secret, logger: logger}
}

// Login creates a fresh TokenPair and persists the refresh token in Redis.
//
// Redis write:
//
//	HSET sandbox:{user_id}:auth:refresh token <random_hex> role <role>
//	EXPIRE sandbox:{user_id}:auth:refresh 604800   (7 days)
//
// Using a HASH (not a plain string) stores metadata (role) alongside the token
// value — single Redis key per user, no secondary indexes.
func (uc *authUsecase) Login(ctx context.Context, req *model.LoginRequest) (*model.TokenPair, error) {
	// ── 1. Validate & normalise role ──────────────────────────────────────
	role := req.Role
	if role == "" {
		role = "user"
	}
	if role != "admin" && role != "user" {
		return nil, ErrInvalidRole
	}

	// ── 2. Issue AccessToken (JWT, 15 min) ────────────────────────────────
	// Claims: sub (user_id), role, iss, iat, exp
	accessToken, err := uc.mintAccessToken(req.UserID, role)
	if err != nil {
		return nil, fmt.Errorf("mint access token: %w", err)
	}

	// ── 3. Issue RefreshToken (32 random bytes → hex string) ──────────────
	// Opaque random string (not JWT) means:
	//   • no decodable payload — zero information leakage
	//   • validation = O(1) Redis HGET comparison
	//   • revocation = single Redis DEL
	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	// ── 4. Persist refresh token in Redis ─────────────────────────────────
	// Each new login overwrites the previous refresh token, enforcing a
	// single-active-session-per-user policy in the sandbox.
	if err := uc.storeRefreshToken(ctx, req.UserID, role, refreshToken); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	uc.logger.Info("user logged in",
		zap.String("user_id", req.UserID),
		zap.String("role", role),
	)

	return &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(AccessTokenTTL.Seconds()), // 900s
		Role:         role,
		UserID:       req.UserID,
	}, nil
}

// Refresh validates the supplied refresh token and, if valid, mints a new
// AccessToken. The refresh token itself is NOT rotated (token rotation would
// require an atomic CAS — left as an extension exercise).
//
// Redis read:
//
//	HGETALL sandbox:{user_id}:auth:refresh
//	→ compare stored "token" field with submitted token (constant-time)
//	→ read "role" field so the new AccessToken carries the correct role
//
// The client submits both user_id and refresh_token because the AccessToken is
// expired at this point and the JWT middleware cannot supply user_id via Locals.
func (uc *authUsecase) Refresh(ctx context.Context, req *model.RefreshRequest) (*model.TokenPair, error) {
	if req.RefreshToken == "" || req.UserID == "" {
		return nil, ErrInvalidRefreshToken
	}

	// ── 1. Look up stored refresh data ────────────────────────────────────
	key := refreshRedisKey(req.UserID)
	fields, err := uc.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis hgetall: %w", err)
	}
	if len(fields) == 0 {
		// Key missing → token was never issued or was revoked via logout.
		return nil, ErrInvalidRefreshToken
	}

	storedToken := fields["token"]
	role := fields["role"]

	// ── 2. Constant-time comparison ───────────────────────────────────────
	// Timing-attack resistance: compare all bytes regardless of first diff.
	if !secureCompare(storedToken, req.RefreshToken) {
		return nil, ErrInvalidRefreshToken
	}

	// ── 3. Mint a brand-new AccessToken ───────────────────────────────────
	accessToken, err := uc.mintAccessToken(req.UserID, role)
	if err != nil {
		return nil, fmt.Errorf("mint access token: %w", err)
	}

	uc.logger.Info("token refreshed", zap.String("user_id", req.UserID))

	return &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: req.RefreshToken, // unchanged — client keeps the same RT
		TokenType:    "Bearer",
		ExpiresIn:    int(AccessTokenTTL.Seconds()),
		Role:         role,
		UserID:       req.UserID,
	}, nil
}

// Logout revokes the user's refresh token by deleting the Redis key.
// After this call any Refresh attempt returns ErrInvalidRefreshToken —
// the client must Login again to obtain a new pair.
//
// Redis write:
//
//	DEL sandbox:{user_id}:auth:refresh
func (uc *authUsecase) Logout(ctx context.Context, userID string) error {
	key := refreshRedisKey(userID)
	if err := uc.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	uc.logger.Info("user logged out", zap.String("user_id", userID))
	return nil
}

// ── Private helpers ────────────────────────────────────────────────────────

// mintAccessToken builds and signs a 15-minute JWT with sub + role claims.
func (uc *authUsecase) mintAccessToken(userID, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"iss":  jwtIssuer,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(AccessTokenTTL).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(uc.secret))
}

// generateRefreshToken produces a cryptographically secure random token.
// 32 bytes → 64-char hex → 256 bits of entropy. Practically unguessable.
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// storeRefreshToken persists the refresh token and role as a Redis HASH.
func (uc *authUsecase) storeRefreshToken(ctx context.Context, userID, role, token string) error {
	key := refreshRedisKey(userID)
	pipe := uc.rdb.Pipeline()
	pipe.HSet(ctx, key, map[string]interface{}{
		"token": token,
		"role":  role,
	})
	pipe.Expire(ctx, key, RefreshTokenTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// secureCompare does constant-time string comparison to prevent timing attacks.
func secureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
