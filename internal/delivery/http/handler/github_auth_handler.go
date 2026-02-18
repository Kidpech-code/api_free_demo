package handler

// GitHubAuthHandler implements GitHub OAuth 2.0 login flow to protect the dashboard page.
//
// Flow:
//  1. GET /auth/github/login   → redirect to GitHub's authorization URL
//  2. GET /auth/github/callback → exchange code for token, fetch user, set signed cookie, redirect to /dashboard.html
//  3. GET /auth/github/logout   → clear session cookie, redirect to /
//
// Only the GitHub user in cfg.GitHub.AllowedLogin is allowed to proceed;
// everyone else gets HTTP 403.
//
// Session: stored as a signed HMAC-SHA256 cookie ("_gh_sess") containing the
// GitHub login name.  No external session store is needed.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"api_free_demo/internal/config"
)

const githubSessionCookie = "_gh_sess"

// GitHubAuthHandler handles GitHub OAuth login/callback/logout.
type GitHubAuthHandler struct {
	cfg    config.GitHubOAuthConfig
	logger *zap.Logger
}

// NewGitHubAuthHandler constructs a GitHubAuthHandler.
func NewGitHubAuthHandler(cfg config.GitHubOAuthConfig, logger *zap.Logger) *GitHubAuthHandler {
	return &GitHubAuthHandler{cfg: cfg, logger: logger}
}

// Login redirects the browser to GitHub's OAuth authorize page.
// GET /auth/github/login
func (h *GitHubAuthHandler) Login(c *fiber.Ctx) error {
	if h.cfg.ClientID == "" {
		return c.Status(fiber.StatusServiceUnavailable).SendString(
			"GitHub OAuth is not configured. Set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET environment variables.")
	}

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&scope=read:user&allow_signup=false",
		url.QueryEscape(h.cfg.ClientID),
	)
	return c.Redirect(authURL, fiber.StatusFound)
}

// Callback handles the redirect back from GitHub.
// GET /auth/github/callback?code=xxx
func (h *GitHubAuthHandler) Callback(c *fiber.Ctx) error {
	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing OAuth code")
	}

	// ── Exchange code for access_token ─────────────────────────────────────
	accessToken, err := h.exchangeCode(code)
	if err != nil {
		h.logger.Error("github oauth: code exchange failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("OAuth code exchange failed")
	}

	// ── Fetch authenticated user ────────────────────────────────────────────
	login, err := h.fetchLogin(accessToken)
	if err != nil {
		h.logger.Error("github oauth: failed to fetch user", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("failed to fetch GitHub user")
	}

	// ── Allow-list check ────────────────────────────────────────────────────
	if !strings.EqualFold(login, h.cfg.AllowedLogin) {
		h.logger.Warn("github oauth: unauthorized login attempt", zap.String("login", login))
		return c.Status(fiber.StatusForbidden).SendString(
			fmt.Sprintf("Access denied. Only @%s is allowed to access this dashboard.", h.cfg.AllowedLogin))
	}

	// ── Set signed session cookie ───────────────────────────────────────────
	signed := h.signValue(login)
	c.Cookie(&fiber.Cookie{
		Name:     githubSessionCookie,
		Value:    signed,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		SameSite: "Lax",
		// Secure: true in production (set by proxy / Railway)
	})

	h.logger.Info("github oauth: dashboard login success", zap.String("login", login))
	return c.Redirect("/dashboard.html", fiber.StatusFound)
}

// Logout clears the session cookie and redirects to the home page.
// GET /auth/github/logout
func (h *GitHubAuthHandler) Logout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     githubSessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: true,
		SameSite: "Lax",
	})
	return c.Redirect("/", fiber.StatusFound)
}

// ──────────────────────────── helpers ─────────────────────────────────────────

// exchangeCode swaps a GitHub OAuth code for an access token.
func (h *GitHubAuthHandler) exchangeCode(code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", h.cfg.ClientID)
	data.Set("client_secret", h.cfg.ClientSecret)
	data.Set("code", code)

	req, _ := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("github error: %s", result.Error)
	}
	return result.AccessToken, nil
}

// fetchLogin calls the GitHub API and returns the authenticated user's login name.
func (h *GitHubAuthHandler) fetchLogin(accessToken string) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var user struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}
	return user.Login, nil
}

// signValue returns "login:HMAC-SHA256-base64url" so the cookie cannot be forged.
func (h *GitHubAuthHandler) signValue(login string) string {
	mac := hmac.New(sha256.New, []byte(h.cfg.SessionSecret))
	mac.Write([]byte(login))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return login + ":" + sig
}

// VerifySession parses the signed session cookie and returns the GitHub login
// if the signature is valid.  Returns ("", false) on any failure.
func (h *GitHubAuthHandler) VerifySession(cookieValue string) (string, bool) {
	parts := strings.SplitN(cookieValue, ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	login, sig := parts[0], parts[1]
	expected := h.signValue(login) // re-compute
	// constant-time compare by comparing the full "login:sig" form
	if !hmac.Equal([]byte(login+":"+sig), []byte(expected)) {
		return "", false
	}
	return login, true
}
