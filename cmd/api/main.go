package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/kidpech/api_free_demo/internal/app"
	"github.com/kidpech/api_free_demo/internal/app/diagnostics"
	"github.com/kidpech/api_free_demo/internal/config"
	"github.com/kidpech/api_free_demo/internal/domain/profile"
	"github.com/kidpech/api_free_demo/internal/domain/user"
	"github.com/kidpech/api_free_demo/internal/infrastructure/auth"
	cacheinfra "github.com/kidpech/api_free_demo/internal/infrastructure/cache"
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
		maskedDSN := maskDSN(cfg.Database.DSN)
		logger.Error("db connect failed - app will start without database",
			zap.Error(err),
			zap.String("dsn_host", maskedDSN),
			zap.String("driver", cfg.Database.Driver),
		)
		// Continue without DB - some endpoints will return 503
		// Background reconnector will keep trying
	} else {
		logger.Info("database connected successfully on startup")
	}
	if dbManager != nil {
		defer dbManager.Close()
	}

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

	// Initialize integrated cache layer (inspired by Uber's CacheFront)
	// Cache-aside pattern with singleflight, circuit breaker, negative caching
	var cacheLayer *cacheinfra.Layer
	if redisClient != nil {
		cacheLayer = cacheinfra.NewLayer(redisClient.Native, logger, cacheinfra.Config{
			Prefix:       "api",
			TTL:          cacheinfra.DefaultTTL,
			CBThreshold:  5,
			CBResetAfter: 30 * time.Second,
		})
		logger.Info("integrated cache layer initialized (CacheFront pattern)")
	} else {
		logger.Warn("cache layer disabled — no Redis connection")
	}

	// Initialize repositories with cache decorators if DB is available
	var userRepo user.Repository
	var profileRepo profile.Repository
	if dbManager != nil {
		rawUserRepo := dbinfra.NewUserRepository(dbManager.Write)
		rawProfileRepo := dbinfra.NewProfileRepository(dbManager.Write)
		// Wrap with cache-aside decorator
		userRepo = cacheinfra.NewUserRepository(rawUserRepo, cacheLayer)
		profileRepo = cacheinfra.NewProfileRepository(rawProfileRepo, cacheLayer)
	}

	userService := user.NewService(userRepo, authManager, logger, cfg.Security.AllowRegistration)
	profileService := profile.NewService(profileRepo)

	// Background DB reconnector — if initial connection failed, keep trying
	if dbManager == nil {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					logger.Info("attempting background DB reconnection...")
					mgr, err := dbinfra.Connect(ctx, cfg.Database, logger)
					if err != nil {
						logger.Warn("background DB reconnect failed", zap.Error(err))
						continue
					}
					dbManager = mgr
					rawUserRepo := dbinfra.NewUserRepository(mgr.Write)
					rawProfileRepo := dbinfra.NewProfileRepository(mgr.Write)
					userService.SetRepository(cacheinfra.NewUserRepository(rawUserRepo, cacheLayer))
					profileService.SetRepository(cacheinfra.NewProfileRepository(rawProfileRepo, cacheLayer))
					logger.Info("database connected via background reconnector — all endpoints available")
					return
				}
			}
		}()
	}

	logBuffer := diagnostics.NewLogBuffer(cfg.Diagnostics.MaxLogLines)
	diagHandler := diagnostics.NewHandler(logBuffer)

	// Register dependency health checkers for readiness endpoint
	if dbManager != nil {
		diagHandler.RegisterDependency("database", dbinfra.NewHealthChecker(dbManager))
	}
	if redisClient != nil {
		diagHandler.RegisterDependency("redis", redisintra.NewHealthChecker(redisClient))
	}

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

// maskDSN extracts host from DSN for safe logging (no credentials).
func maskDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		if len(dsn) > 20 {
			return dsn[:20] + "..."
		}
		return dsn
	}
	return u.Hostname() + ":" + u.Port()
}
