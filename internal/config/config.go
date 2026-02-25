package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
// Values are loaded from environment variables with sensible defaults.
type Config struct {
	Server ServerConfig
	Redis  RedisConfig
	JWT    JWTConfig
	App    AppConfig
	GitHub GitHubOAuthConfig
	TMD    TMDConfig
}

// TMDConfig holds settings for the Thai Meteorological Department NWP API worker.
type TMDConfig struct {
	Enabled      bool          // TMD_ENABLED — set to "true" to activate the cron worker
	Token        string        // TMD_TOKEN — Bearer token for the TMD NWP API
	CronExpr     string        // TMD_CRON — cron expression (default: every 6 hours)
	CacheTTL     time.Duration // TMD_CACHE_TTL — Redis TTL for cached forecasts
	RequestDelay time.Duration // TMD_REQUEST_DELAY — throttle between API calls
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// RedisConfig holds connection parameters for the Redis client.
// When URL is non-empty it takes precedence over Addr/Password/DB —
// the Redis client will call redis.ParseURL(URL) which correctly handles
// redis:// (plain) and rediss:// (TLS) schemes, embedded credentials,
// and database path (e.g. redis://:pass@host:6379/0).
type RedisConfig struct {
	URL      string // Full URL — set from REDIS_URL env var (Railway, Render, etc.)
	Addr     string // host:port fallback when URL is empty
	Password string
	DB       int
}

type JWTConfig struct {
	Secret string
}

type AppConfig struct {
	DataTTL     time.Duration // Default TTL for all sandbox data
	RateLimit   int           // Requests per window per user
	RateWindow  time.Duration // Rate limit window duration
	Environment string
	LogLevel    string
	CORSOrigins []string // Allowed CORS origins; empty = allow all
}

// GitHubOAuthConfig holds credentials for GitHub OAuth login.
// Used to protect the /dashboard.html page (admin-only access).
type GitHubOAuthConfig struct {
	ClientID      string // GITHUB_CLIENT_ID env var
	ClientSecret  string // GITHUB_CLIENT_SECRET env var
	AllowedLogin  string // GITHUB_ALLOWED_LOGIN env var — only this GitHub username may access the dashboard
	SessionSecret string // SESSION_SECRET env var — used to sign the session cookie (min 32 chars)
}

// Load reads configuration from environment variables with defaults.
func Load() *Config {
	env := getEnv("ENVIRONMENT", "development")

	// ── JWT Secret validation ─────────────────────────────────────────────
	// In production, a weak/default secret is a critical security vulnerability.
	const defaultSecret = "sandbox-secret-change-me-in-production"
	jwtSecret := getEnv("JWT_SECRET", defaultSecret)
	if env == "production" && (jwtSecret == defaultSecret || len(jwtSecret) < 32) {
		log.Fatal("[FATAL] JWT_SECRET must be set to a secure random string (min 32 chars) in production. Generate one with: openssl rand -hex 32")
	}

	// ── Redis: prefer REDIS_URL (Railway / managed providers) ───────────────
	// When REDIS_URL is set the Redis client will call redis.ParseURL() which
	// handles plain redis:// and TLS rediss:// schemes, embedded credentials,
	// and database path automatically — no manual string parsing needed.
	redisURL := os.Getenv("REDIS_URL")

	var corsOrigins []string
	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				corsOrigins = append(corsOrigins, o)
			}
		}
	}

	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
		},
		Redis: RedisConfig{
			URL:      redisURL,
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret: jwtSecret,
		},
		App: AppConfig{
			DataTTL:     getDuration("DATA_TTL", 72*time.Hour),
			RateLimit:   getInt("RATE_LIMIT", 100),
			RateWindow:  getDuration("RATE_WINDOW", 1*time.Minute),
			Environment: env,
			LogLevel:    getEnv("LOG_LEVEL", "info"),
			CORSOrigins: corsOrigins,
		},
		GitHub: GitHubOAuthConfig{
			ClientID:      os.Getenv("GITHUB_CLIENT_ID"),
			ClientSecret:  os.Getenv("GITHUB_CLIENT_SECRET"),
			AllowedLogin:  getEnv("GITHUB_ALLOWED_LOGIN", "Kidpech-code"),
			SessionSecret: getEnv("SESSION_SECRET", "dashboard-session-secret-change-me"),
		},
		TMD: TMDConfig{
			Enabled:      getEnv("TMD_ENABLED", "false") == "true",
			Token:        os.Getenv("TMD_TOKEN"),
			CronExpr:     getEnv("TMD_CRON", "0 */6 * * *"),
			CacheTTL:     getDuration("TMD_CACHE_TTL", 12*time.Hour),
			RequestDelay: getDuration("TMD_REQUEST_DELAY", 2*time.Second),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
