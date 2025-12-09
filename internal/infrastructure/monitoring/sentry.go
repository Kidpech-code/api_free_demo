package monitoring

import (
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/kidpech/api_free_demo/internal/config"
)

// InitSentry configures sentry if DSN provided.
func InitSentry(cfg config.MonitoringConfig, app config.AppConfig) error {
	if cfg.SentryDSN == "" {
		return nil
	}
	return sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.SentryDSN,
		Release:          app.Version,
		Environment:      app.Env,
		TracesSampleRate: cfg.SentrySampleRate,
	})
}

// Flush ensures buffered events ship.
func Flush() {
	sentry.Flush(2 * time.Second)
}
