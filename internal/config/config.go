package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config represents the full runtime configuration tree.
type Config struct {
	App         AppConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	Auth        AuthConfig
	RateLimit   RateLimitConfig
	Cors        CORSConfig
	Security    SecurityConfig
	Monitoring  MonitoringConfig
	Diagnostics DiagnosticsConfig
}

// AppConfig captures application-level settings.
type AppConfig struct {
	Name            string
	Env             string
	Version         string
	Port            string
	BaseURL         string
	AllowedHosts    []string
	AllowedOrigins  []string
	CloudflareHosts []string
}

// DatabaseConfig stores database connectivity info.
type DatabaseConfig struct {
	Driver           string
	DSN              string
	ReadOnlyDSN      string
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
	AutoMigrate      bool
	PreferSimpleProt bool
}

// RedisConfig stores redis connectivity info.
type RedisConfig struct {
	Addr     string
	Username string
	Password string
	DB       int
	TLS      bool
}

// AuthConfig stores JWT and auth related settings.
type AuthConfig struct {
	AccessSecret        string
	RefreshSecret       string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	TokenIssuer         string
	SecretVersion       string
	OAuthRedirectURL    string
	OAuthGoogleClientID string
}

// RateLimitConfig manages throttling parameters.
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerMinute int
	Burst             int
	RedisPrefix       string
}

// CORSConfig declares cross-origin policy.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// SecurityConfig covers app hardening toggles.
type SecurityConfig struct {
	AllowRegistration bool
	BcryptCost        int
}

// MonitoringConfig adds observability tunables.
type MonitoringConfig struct {
	PrometheusEnabled bool
	SentryDSN         string
	SentrySampleRate  float64
}

// DiagnosticsConfig governs debug helpers.
type DiagnosticsConfig struct {
	EnableDebugLogs bool
	MaxLogLines     int
}

// Load reads from environment (optionally .env) and builds Config.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		App: AppConfig{
			Name:            getenv("APP_NAME", "kidpech-demo-api"),
			Env:             getenv("APP_ENV", "development"),
			Version:         getenv("APP_VERSION", "0.1.0"),
			Port:            getenv("PORT", "8080"),
			BaseURL:         getenv("BASE_URL", "http://localhost:8080"),
			AllowedHosts:    splitAndTrim(getenv("ALLOWED_HOSTS", "api.kidpech.app,api.twentcode.com")),
			AllowedOrigins:  splitAndTrim(getenv("CORS_ORIGINS", "http://localhost:3000,http://localhost:5173,http://localhost:8080,https://dev.kidpech.app")),
			CloudflareHosts: splitAndTrim(getenv("CLOUDFLARE_HOSTS", "api.kidpech.app,api.twentcode.com")),
		},
		Database: DatabaseConfig{
			Driver:           strings.ToLower(getenv("DB_DRIVER", "postgres")),
			DSN:              getenv("DB_DSN", "postgres://postgres:postgres@db:5432/demo_db?sslmode=disable"),
			ReadOnlyDSN:      getenv("DB_READ_DSN", ""),
			MaxOpenConns:     getInt("DB_MAX_OPEN", 25),
			MaxIdleConns:     getInt("DB_MAX_IDLE", 10),
			ConnMaxLifetime:  time.Duration(getInt("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute,
			AutoMigrate:      getBool("DB_AUTO_MIGRATE", true),
			PreferSimpleProt: getBool("DB_PREFER_SIMPLE", true),
		},
		Redis: RedisConfig{
			Addr:     getenv("REDIS_ADDR", "redis:6379"),
			Username: getenv("REDIS_USER", ""),
			Password: getenv("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 0),
			TLS:      getBool("REDIS_TLS", false),
		},
		Auth: AuthConfig{
			AccessSecret:        getenv("JWT_ACCESS_SECRET", getenv("JWT_SECRET", "change-me")),
			RefreshSecret:       getenv("JWT_REFRESH_SECRET", getenv("JWT_SECRET", "change-me")),
			AccessTokenTTL:      time.Duration(getInt("JWT_ACCESS_EXP_MIN", 15)) * time.Minute,
			RefreshTokenTTL:     time.Duration(getInt("JWT_REFRESH_EXP_HOURS", 24)) * time.Hour,
			TokenIssuer:         getenv("JWT_ISSUER", "kidpech.app"),
			SecretVersion:       getenv("JWT_SECRET_VERSION", "v1"),
			OAuthRedirectURL:    getenv("OAUTH_REDIRECT_URL", ""),
			OAuthGoogleClientID: getenv("OAUTH_GOOGLE_CLIENT_ID", ""),
		},
		RateLimit: RateLimitConfig{
			Enabled:           getBool("RATE_LIMIT_ENABLED", true),
			RequestsPerMinute: getInt("RATE_LIMIT_PER_MIN", 60),
			Burst:             getInt("RATE_LIMIT_BURST", 5),
			RedisPrefix:       getenv("RATE_LIMIT_PREFIX", "ratelimit"),
		},
		Cors: CORSConfig{
			AllowedOrigins:   splitAndTrim(getenv("CORS_ORIGINS", "http://localhost:3000,http://localhost:5173,http://localhost:8080,https://dev.kidpech.app")),
			AllowedMethods:   splitAndTrim(getenv("CORS_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS")),
			AllowedHeaders:   splitAndTrim(getenv("CORS_HEADERS", "Authorization,Content-Type,Accept,X-Requested-With")),
			AllowCredentials: getBool("CORS_ALLOW_CREDENTIALS", true),
		},
		Security: SecurityConfig{
			AllowRegistration: getBool("ALLOW_REGISTRATION", true),
			BcryptCost:        getInt("BCRYPT_COST", 12),
		},
		Monitoring: MonitoringConfig{
			PrometheusEnabled: getBool("PROMETHEUS_ENABLED", true),
			SentryDSN:         getenv("SENTRY_DSN", ""),
			SentrySampleRate:  getFloat("SENTRY_SAMPLE_RATE", 0.2),
		},
		Diagnostics: DiagnosticsConfig{
			EnableDebugLogs: getBool("ENABLE_DEBUG_LOGS", false),
			MaxLogLines:     getInt("DEBUG_LOG_LIMIT", 200),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Auth.AccessSecret == "" || c.Auth.RefreshSecret == "" {
		return fmt.Errorf("jwt secrets must be provided")
	}
	switch c.Database.Driver {
	case "postgres", "mysql":
	default:
		return fmt.Errorf("unsupported db driver %s", c.Database.Driver)
	}
	return nil
}

func getenv(key, def string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	return val
}

func getInt(key string, def int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return i
}

func getBool(key string, def bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return def
	}
	return parsed
}

func getFloat(key string, def float64) float64 {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return def
	}
	return parsed
}

func splitAndTrim(val string) []string {
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trim := strings.TrimSpace(p)
		if trim != "" {
			out = append(out, trim)
		}
	}
	return out
}
