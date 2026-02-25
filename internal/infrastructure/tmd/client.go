package tmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// ──────────────────────────────────────────────────────────
// TMD NWP API – HTTP Client
//
// Features:
//   - Bearer token authentication
//   - Per-request throttling (configurable delay)
//   - Automatic retry with exponential back-off on 429 / 5xx
// ──────────────────────────────────────────────────────────

const (
	baseURL = "https://data.tmd.go.th/nwpapi/v1"

	// Endpoint patterns
	pathHourly = "/forecast/location/hourly/at?lat=%.4f&lon=%.4f&fields=tc,rh,slp,rain,ws10m,wd10m,cond&duration=48"
	pathDaily  = "/forecast/location/daily/at?lat=%.4f&lon=%.4f&fields=tc_max,tc_min,rh,rain,cond&duration=7"
	pathArea   = "/forecast/area/place" // Query params built dynamically
)

// ForecastType enumerates the three forecast categories.
type ForecastType string

const (
	ForecastHourly ForecastType = "hourly"
	ForecastDaily  ForecastType = "daily"
	ForecastArea   ForecastType = "area"
)

// ClientConfig holds tunables for the TMD API client.
type ClientConfig struct {
	Token         string        // Bearer token for Authorization header
	RequestDelay  time.Duration // Minimum gap between consecutive API calls (rate-limit safety)
	MaxRetries    int           // Max retry attempts on transient errors
	BaseBackoff   time.Duration // Initial back-off before first retry
	ClientTimeout time.Duration // HTTP client timeout per request
}

// DefaultClientConfig returns production-safe defaults.
func DefaultClientConfig(token string) ClientConfig {
	return ClientConfig{
		Token:         token,
		RequestDelay:  2 * time.Second, // 2 s between calls ⇒ max 30 req/min
		MaxRetries:    3,
		BaseBackoff:   5 * time.Second,
		ClientTimeout: 30 * time.Second,
	}
}

// Client talks to the TMD NWP API.
type Client struct {
	http   *http.Client
	cfg    ClientConfig
	logger *zap.Logger
}

// NewClient creates a TMD API client.
func NewClient(cfg ClientConfig, logger *zap.Logger) *Client {
	return &Client{
		http: &http.Client{
			Timeout: cfg.ClientTimeout,
		},
		cfg:    cfg,
		logger: logger.Named("tmd.client"),
	}
}

// FetchForecast fetches a single forecast type for the given location.
// It honours the configured delay, retry, and back-off policies.
func (c *Client) FetchForecast(ctx context.Context, ft ForecastType, loc TargetLocation) (json.RawMessage, error) {
	var path string
	switch ft {
	case ForecastHourly:
		path = fmt.Sprintf(pathHourly, loc.Lat, loc.Lon)
	case ForecastDaily:
		path = fmt.Sprintf(pathDaily, loc.Lat, loc.Lon)
	case ForecastArea:
		v := url.Values{}
		v.Set("domain", "2")
		// starttime is required by the TMD area endpoint; round down to the current hour.
		v.Set("starttime", time.Now().UTC().Format("2006-01-02T15:00:00"))
		if loc.Province != "" {
			v.Set("province", loc.Province)
		}
		if loc.Amphoe != "" {
			v.Set("amphoe", loc.Amphoe)
		}
		if loc.Tambon != "" {
			v.Set("tambon", loc.Tambon)
		}
		path = pathArea + "?" + v.Encode()
	default:
		return nil, fmt.Errorf("unknown forecast type: %s", ft)
	}

	fullURL := baseURL + path

	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential back-off: base * 2^(attempt-1)
			backoff := c.cfg.BaseBackoff * time.Duration(math.Pow(2, float64(attempt-1)))
			c.logger.Warn("retrying TMD request",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(lastErr),
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		body, err := c.doRequest(ctx, fullURL)
		if err == nil {
			return body, nil
		}
		lastErr = err

		// Only retry transient errors (network issues, 429, 5xx).
		// Non-retryable errors (4xx client errors) fail immediately.
		var re *retryableError
		if !errors.As(err, &re) {
			break
		}
	}

	return nil, fmt.Errorf("TMD request failed: %w", lastErr)
}

// Throttle pauses for the configured RequestDelay.
// Call this between successive FetchForecast invocations.
func (c *Client) Throttle(ctx context.Context) error {
	if c.cfg.RequestDelay <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(c.cfg.RequestDelay):
		return nil
	}
}

// ── internal ──────────────────────────────────────────────

// retryableError signals that the caller should retry.
// Unwrap exposes the inner error so errors.Is / errors.As can traverse the chain.
type retryableError struct{ error }

func (e *retryableError) Unwrap() error { return e.error }

func (c *Client) doRequest(ctx context.Context, url string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &retryableError{fmt.Errorf("http do: %w", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &retryableError{fmt.Errorf("read body: %w", err)}
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		c.logger.Warn("TMD API rate limited (429)", zap.String("url", url))
		return nil, &retryableError{fmt.Errorf("rate limited (HTTP 429)")}
	case resp.StatusCode >= 500:
		c.logger.Warn("TMD API server error", zap.Int("status", resp.StatusCode), zap.String("url", url))
		return nil, &retryableError{fmt.Errorf("server error (HTTP %d)", resp.StatusCode)}
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("TMD API client error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return json.RawMessage(body), nil
}
