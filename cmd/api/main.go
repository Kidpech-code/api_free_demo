package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"api_free_demo/internal/config"
	httpDelivery "api_free_demo/internal/delivery/http"
	"api_free_demo/internal/infrastructure/logging"
	"api_free_demo/internal/infrastructure/metrics"
	redisClient "api_free_demo/internal/infrastructure/redis"
	infraRepo "api_free_demo/internal/infrastructure/repository"
	"api_free_demo/internal/infrastructure/tmd"
	"api_free_demo/internal/usecase"
)

// @title        API Free Demo Sandbox
// @description  Educational REST API simulating Uber's DocStore Integrated Cache Pattern with Redis.
// @version      1.0.0
func main() {
	// ── 1. Configuration ──
	cfg := config.Load()

	// ── 2. Logger (Zap) ──
	logger := logging.NewLogger(cfg.App.LogLevel, cfg.App.Environment)
	defer logger.Sync() //nolint:errcheck
	logger.Info("starting api_free_demo sandbox",
		zap.String("env", cfg.App.Environment),
		zap.String("port", cfg.Server.Port),
	)

	// ── 3. Prometheus Metrics ──
	m := metrics.NewMetrics(prometheus.DefaultRegisterer)

	// ── 4. Redis ──
	rc, err := redisClient.NewClient(cfg.Redis, logger)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rc.Close()

	// ── 5. Repository (Uber DocStore Pattern) ──
	productRepo := infraRepo.NewProductRepository(rc.RDB(), cfg.App.DataTTL, m, logger)

	// ── 6. Usecase ──
	productUC := usecase.NewProductUsecase(productRepo, logger)

	// ── 6b. TMD Weather Cron Worker ──
	var tmdCache *tmd.CacheRepository
	var tmdWorker *tmd.Worker
	if cfg.TMD.Enabled && cfg.TMD.Token != "" {
		tmdCache = tmd.NewCacheRepository(rc.RDB(), cfg.TMD.CacheTTL, logger)

		clientCfg := tmd.DefaultClientConfig(cfg.TMD.Token)
		clientCfg.RequestDelay = cfg.TMD.RequestDelay
		tmdClient := tmd.NewClient(clientCfg, logger)

		workerCfg := tmd.DefaultWorkerConfig()
		workerCfg.CronExpr = cfg.TMD.CronExpr

		tmdWorker = tmd.NewWorker(tmdClient, tmdCache, tmd.DefaultLocations(), workerCfg, logger)
		if err := tmdWorker.Start(); err != nil {
			logger.Fatal("failed to start TMD worker", zap.Error(err))
		}
		defer tmdWorker.Stop()
		logger.Info("TMD weather cron worker enabled",
			zap.String("cron", cfg.TMD.CronExpr),
			zap.Duration("cache_ttl", cfg.TMD.CacheTTL),
		)
	} else {
		logger.Info("TMD weather cron worker disabled (set TMD_ENABLED=true and TMD_TOKEN to activate)")
	}

	// ── 7. HTTP Router (Fiber) ──
	// Strip the "web/" prefix so the embedded FS root maps directly to the URL root.
	webRoot, err := fs.Sub(WebFS, "web")
	if err != nil {
		logger.Fatal("failed to sub webFS", zap.Error(err))
	}
	app := httpDelivery.NewRouter(cfg, rc.RDB(), productUC, m, logger, webRoot, tmdCache)

	// ── 8. Graceful Shutdown ──
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	sig := <-quit
	logger.Info("shutting down", zap.String("signal", sig.String()))
	if err := app.Shutdown(); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
	fmt.Println("server stopped")
}
