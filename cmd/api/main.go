package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/kidpech/api_free_demo/internal/app"
	"github.com/kidpech/api_free_demo/internal/app/diagnostics"
	"github.com/kidpech/api_free_demo/internal/config"
	"github.com/kidpech/api_free_demo/internal/domain/profile"
	"github.com/kidpech/api_free_demo/internal/domain/user"
	"github.com/kidpech/api_free_demo/internal/infrastructure/auth"
	dbinfra "github.com/kidpech/api_free_demo/internal/infrastructure/db"
	"github.com/kidpech/api_free_demo/internal/infrastructure/logging"
	"github.com/kidpech/api_free_demo/internal/infrastructure/monitoring"
	"github.com/kidpech/api_free_demo/internal/infrastructure/ratelimit"
	redisintra "github.com/kidpech/api_free_demo/internal/infrastructure/redis"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger, err := logging.New(cfg.App.Env)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer logging.Sync(logger)

	if err := monitoring.InitSentry(cfg.Monitoring, cfg.App); err != nil {
		logger.Warn("sentry init failed", zap.Error(err))
	}
	monitoring.Init()
	defer monitoring.Flush()

	dbManager, err := dbinfra.Connect(ctx, cfg.Database, logger)
	if err != nil {
		logger.Fatal("db connect failed", zap.Error(err))
	}
	defer dbManager.Close()

	var redisClient *redisintra.Client
	if cfg.Redis.Addr != "" {
		client, err := redisintra.Connect(cfg.Redis, logger)
		if err == nil {
			redisClient = client
			defer client.Close()
		} else {
			logger.Warn("redis connect failed", zap.Error(err))
		}
	}

	authManager := auth.NewManager(cfg.Auth, nil)
	if redisClient != nil {
		authManager = auth.NewManager(cfg.Auth, redisClient.Native)
	}

	userRepo := dbinfra.NewUserRepository(dbManager.Write)
	profileRepo := dbinfra.NewProfileRepository(dbManager.Write)

	userService := user.NewService(userRepo, authManager, logger, cfg.Security.AllowRegistration)
	profileService := profile.NewService(profileRepo)

	logBuffer := diagnostics.NewLogBuffer(cfg.Diagnostics.MaxLogLines)
	diagHandler := diagnostics.NewHandler(logBuffer)
	userHandler := user.NewHandler(userService)
	profileHandler := profile.NewHandler(profileService)

	var ipLimiter, userLimiter ratelimit.Limiter
	if cfg.RateLimit.Enabled {
		if redisClient != nil {
			ipLimiter = ratelimit.NewRedisLimiter(redisClient.Native, cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.RedisPrefix+":ip")
			userLimiter = ratelimit.NewRedisLimiter(redisClient.Native, cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.RedisPrefix+":user")
		} else {
			ipLimiter = ratelimit.NewMemoryLimiter(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Burst)
			userLimiter = ratelimit.NewMemoryLimiter(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Burst)
		}
	}

	router := app.NewRouter(app.RouterDeps{
		Config:         cfg,
		UserHandler:    userHandler,
		ProfileHandler: profileHandler,
		Diagnostics:    diagHandler,
		AuthManager:    authManager,
		Logger:         logger,
		LogBuffer:      logBuffer,
		IPLimiter:      ipLimiter,
		UserLimiter:    userLimiter,
	})

	server := &app.Server{Engine: router, Addr: ":" + cfg.App.Port, Logger: logger}
	if err := server.Run(ctx); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}
