package main

import (
	"fmt"
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

	// ── 7. HTTP Router (Fiber) ──
	app := httpDelivery.NewRouter(cfg, rc.RDB(), productUC, m, logger)

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
