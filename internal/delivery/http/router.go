package http

import (
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
	app.Use(cors.New())
	app.Use(middleware.MetricsMiddleware(m))

	// ── System Endpoints (no auth required) ──
	healthH := handler.NewHealthHandler(rdb)
	authH := handler.NewAuthHandler(cfg.JWT.Secret)

	app.Get("/health", healthH.Health)
	app.Get("/auth/token", authH.GenerateToken)

	// Prometheus metrics endpoint
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	)))

	// ── Authenticated API Routes ──
	api := app.Group("/api/v1")
	api.Use(middleware.JWTAuth(cfg.JWT.Secret, logger))
	api.Use(middleware.RateLimiter(rdb, cfg.App.RateLimit, cfg.App.RateWindow, m, logger))

	// Product routes
	productH := handler.NewProductHandler(productUC, logger)
	products := api.Group("/products")
	products.Post("/", productH.Create)
	products.Get("/", productH.List)
	products.Post("/bulk", productH.BulkCreate)
	products.Get("/:id", productH.GetByID)
	products.Put("/:id", productH.Update)
	products.Delete("/:id", productH.Delete)

	return app
}
