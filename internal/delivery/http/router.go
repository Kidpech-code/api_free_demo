package http

import (
	"strings"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"api_free_demo/internal/config"
	"api_free_demo/internal/delivery/http/handler"
	"api_free_demo/internal/delivery/http/middleware"
	"api_free_demo/internal/domain/usecase"
	"api_free_demo/internal/infrastructure/metrics"
	appUsecase "api_free_demo/internal/usecase"
)

// NewRouter creates and configures the Fiber application with all routes.
func NewRouter(
	cfg *config.Config,
	rdb *redis.Client,
	productUC usecase.ProductUsecase,
	m *metrics.Metrics,
	logger *zap.Logger,
) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "API Free Demo Sandbox",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		},
	})

	// ── Global Middleware ──
	app.Use(recover.New())
	app.Use(requestid.New())

	// CORS: allow specific origins when configured, otherwise allow all (dev/sandbox mode)
	corsConfig := cors.Config{
		AllowMethods: strings.Join([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, ","),
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Request-ID",
	}
	if len(cfg.App.CORSOrigins) > 0 {
		corsConfig.AllowOrigins = strings.Join(cfg.App.CORSOrigins, ",")
	} else {
		corsConfig.AllowOrigins = "*"
	}
	app.Use(cors.New(corsConfig))
	app.Use(middleware.MetricsMiddleware(m))

	// ── Auth Usecase ──
	authUC := appUsecase.NewAuthUsecase(rdb, cfg.JWT.Secret, logger)

	// ── System Endpoints (no auth required) ──
	healthH := handler.NewHealthHandler(rdb)
	legacyAuthH := handler.NewLegacyAuthHandler(cfg.JWT.Secret)
	authH := handler.NewAuthHandler(authUC, logger)

	app.Get("/health", healthH.Health)

	// Prometheus metrics endpoint
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	)))

	// ── Auth Endpoints ────────────────────────────────────────────────────
	//
	//  GET  /auth/token            — legacy 72h demo token (backward compat)
	//  POST /auth/login            — dual-token: AccessToken(15m)+RefreshToken(7d)
	//  POST /auth/refresh          — exchange RefreshToken → new AccessToken
	//  POST /auth/logout           — revoke RefreshToken (requires AccessToken)
	//
	// All /auth/* routes intentionally bypass the rate-limiter so students
	// can freely experiment with the token lifecycle.
	app.Get("/auth/token", legacyAuthH.GenerateToken)
	app.Post("/auth/login", authH.Login)
	app.Post("/auth/refresh", authH.Refresh)

	// Logout requires a valid AccessToken so it sits behind JWTAuth.
	logoutGroup := app.Group("/auth")
	logoutGroup.Use(middleware.JWTAuth(cfg.JWT.Secret, logger))
	logoutGroup.Post("/logout", authH.Logout)

	// ── Authenticated API Routes ──────────────────────────────────────────
	api := app.Group("/api/v1")
	api.Use(middleware.JWTAuth(cfg.JWT.Secret, logger))
	api.Use(middleware.RateLimiter(rdb, cfg.App.RateLimit, cfg.App.RateWindow, m, logger))

	// Product routes (accessible by any authenticated user)
	productH := handler.NewProductHandler(productUC, logger)
	products := api.Group("/products")
	products.Post("/", productH.Create)
	products.Get("/", productH.List)
	products.Post("/bulk", productH.BulkCreate)
	products.Get("/:id", productH.GetByID)
	products.Put("/:id", productH.Update)
	products.Delete("/:id", productH.Delete)

	// ── Admin-only Routes (RBAC example) ─────────────────────────────────
	// Access requires a token obtained via:
	//   POST /auth/login { "user_id": "alice", "role": "admin" }
	//
	// Attempting to access with a "user"-role token returns HTTP 403.
	admin := api.Group("/admin")
	admin.Use(middleware.RequireRole("admin"))
	admin.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "welcome, admin 🚀",
			"user_id": c.Locals(middleware.ContextKeyUserID),
		})
	})

	return app
}
