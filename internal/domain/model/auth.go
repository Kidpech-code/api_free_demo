package model

// TokenPair holds the dual-token pair returned on login/refresh.
//
// Auth Lifecycle (Educational Overview):
//
//	+----------+     POST /auth/login      +----------+
//	|  Client  | ─────────────────────────> |  Server  |
//	|          | <── AccessToken (15 min) ── |          |
//	|          | <── RefreshToken (7 days) ── |          |
//	+----------+                           +----------+
//
//	... 15 minutes later, AccessToken expires ...
//
//	+----------+  POST /auth/refresh        +----------+
//	|  Client  | ──── RefreshToken ────────> |  Server  |
//	|          | <── NEW AccessToken ──────── |  (Redis) |
//	+----------+                            +----------+
//
//	... user logs out ...
//
//	+----------+  POST /auth/logout (Auth)  +----------+
//	|  Client  | ────────────────────────── > |  Server  |
//	|          |                              | DEL key  |
//	+----------+                            +----------+
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"` // always "Bearer"
	ExpiresIn    int    `json:"expires_in"` // access token TTL in seconds
	Role         string `json:"role"`       // role embedded in access token
	UserID       string `json:"user_id"`
}

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	UserID string `json:"user_id"` // required: namespace identifier
	Role   string `json:"role"`    // optional: "admin" | "user" (default: "user")
}

// RefreshRequest is the payload for POST /auth/refresh.
//
// Why include user_id?
// The AccessToken is EXPIRED at refresh time, so the JWT middleware cannot
// inject user_id via Locals. The client is expected to store their own user_id
// locally alongside the refresh_token (it is not secret — it is their namespace ID).
type RefreshRequest struct {
	UserID       string `json:"user_id"`       // required: the user's namespace identifier
	RefreshToken string `json:"refresh_token"` // required: the long-lived token from login
}
