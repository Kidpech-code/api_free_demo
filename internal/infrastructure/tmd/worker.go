package tmd

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// ──────────────────────────────────────────────────────────
// TMD Forecast – Cron Worker
//
// Periodically fetches forecasts from the TMD NWP API for a
// list of target locations and caches them in Redis.
//
// Recommended cron expressions (safe for rate limits):
//   "0 */6 * * *"   – every 6 hours (4 runs/day) ← recommended
//   "0 */4 * * *"   – every 4 hours (6 runs/day)
//   "0 0,12 * * *"  – twice a day at midnight & noon
//
// With 10 locations × 3 types × 2 s delay = ~60 s per run.
// 4 runs/day = 120 API calls/day — well within typical quotas.
// ──────────────────────────────────────────────────────────

// WorkerConfig holds cron schedule and runtime limits.
type WorkerConfig struct {
	CronExpr   string        // Cron expression for the scheduler
	RunTimeout time.Duration // Max duration for a single run
}

// DefaultWorkerConfig returns production-safe defaults.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		CronExpr:   "0 */6 * * *", // every 6 hours
		RunTimeout: 10 * time.Minute,
	}
}

// Worker orchestrates periodic forecast caching.
type Worker struct {
	client    *Client
	cache     *CacheRepository
	locations []TargetLocation
	cfg       WorkerConfig
	cron      *cron.Cron
	logger    *zap.Logger
	running   atomic.Bool
}

// NewWorker creates a TMD cron worker.
func NewWorker(
	client *Client,
	cache *CacheRepository,
	locations []TargetLocation,
	cfg WorkerConfig,
	logger *zap.Logger,
) *Worker {
	return &Worker{
		client:    client,
		cache:     cache,
		locations: locations,
		cfg:       cfg,
		logger:    logger.Named("tmd.worker"),
	}
}

// Start registers the cron job and begins the scheduler.
// It also runs the job once immediately so the cache is warm on startup.
func (w *Worker) Start() error {
	w.cron = cron.New()

	_, err := w.cron.AddFunc(w.cfg.CronExpr, func() {
		w.runOnce()
	})
	if err != nil {
		return err
	}

	w.cron.Start()
	w.logger.Info("TMD cron worker started",
		zap.String("schedule", w.cfg.CronExpr),
		zap.Int("locations", len(w.locations)),
	)

	// Warm cache immediately in a goroutine so Start() is non-blocking.
	go w.runOnce()

	return nil
}

// Stop gracefully shuts down the cron scheduler.
func (w *Worker) Stop() {
	if w.cron != nil {
		ctx := w.cron.Stop()
		<-ctx.Done()
		w.logger.Info("TMD cron worker stopped")
	}
}

// runOnce fetches all forecast types for every target location.
func (w *Worker) runOnce() {
	// Prevent overlapping runs.
	if !w.running.CompareAndSwap(false, true) {
		w.logger.Warn("skipping TMD run — previous run still in progress")
		return
	}
	defer w.running.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), w.cfg.RunTimeout)
	defer cancel()

	types := []ForecastType{ForecastHourly, ForecastDaily, ForecastArea}

	start := time.Now()
	var success, failed int

	w.logger.Info("TMD forecast sync started",
		zap.Int("locations", len(w.locations)),
		zap.Int("types", len(types)),
	)

	for _, loc := range w.locations {
		for _, ft := range types {
			if ctx.Err() != nil {
				w.logger.Error("TMD run cancelled — context expired")
				return
			}

			w.logger.Info("fetching forecast",
				zap.String("type", string(ft)),
				zap.String("location", loc.Code),
				zap.Float64("lat", loc.Lat),
				zap.Float64("lon", loc.Lon),
				zap.String("province", loc.Province),
			)

			data, err := w.client.FetchForecast(ctx, ft, loc)
			if err != nil {
				w.logger.Error("failed to fetch forecast",
					zap.String("type", string(ft)),
					zap.String("location", loc.Code),
					zap.Error(err),
				)
				failed++
			} else {
				if err := w.cache.Set(ctx, ft, loc.Code, data); err != nil {
					w.logger.Error("failed to cache forecast",
						zap.String("type", string(ft)),
						zap.String("location", loc.Code),
						zap.Error(err),
					)
					failed++
				} else {
					success++
				}
			}

			// Throttle between calls to respect rate limits.
			if err := w.client.Throttle(ctx); err != nil {
				w.logger.Error("throttle interrupted", zap.Error(err))
				return
			}
		}
	}

	w.logger.Info("TMD forecast sync completed",
		zap.Int("success", success),
		zap.Int("failed", failed),
		zap.Duration("elapsed", time.Since(start)),
	)
}
