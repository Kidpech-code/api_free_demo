package http

import (
	"io/fs"
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
	"api_free_demo/internal/infrastructure/tmd"
	appUsecase "api_free_demo/internal/usecase"
)

// htmlPage returns a Fiber handler that reads the given filename from the
// embedded FS and sends it with the correct Content-Type. Using explicit
// ReadFile is more reliable than the filesystem middleware across Fiber versions.
func htmlPage(webUI fs.FS, name string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		data, err := fs.ReadFile(webUI, name)
		if err != nil {
			return fiber.ErrNotFound
		}
		c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
		return c.Send(data)
	}
}

// NewRouter creates and configures the Fiber application with all routes.
func NewRouter(
	cfg *config.Config,
	rdb *redis.Client,
	productUC usecase.ProductUsecase,
	m *metrics.Metrics,
	logger *zap.Logger,
	webUI fs.FS,
	tmdCache *tmd.CacheRepository, // nil when TMD worker is disabled
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

	// ── Error Sandbox (no auth required — for learning) ───────────────────
	// These endpoints intentionally return specific HTTP error codes so
	// students can explore error handling without manufacturing failures.
	errH := handler.NewErrorSandboxHandler()
	errSandbox := app.Group("/api/v1/sandbox/errors")
	errSandbox.Get("/400", errH.E400)
	errSandbox.Get("/401", errH.E401)
	errSandbox.Get("/403", errH.E403)
	errSandbox.Get("/404", errH.E404)
	errSandbox.Get("/405", errH.E405)
	errSandbox.Get("/409", errH.E409)
	errSandbox.Get("/410", errH.E410)
	errSandbox.Get("/422", errH.E422)
	errSandbox.Get("/429", errH.E429)
	errSandbox.Get("/500", errH.E500)
	errSandbox.Get("/502", errH.E502)
	errSandbox.Get("/503", errH.E503)
	errSandbox.Get("/504", errH.E504)

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

	// ── TMD Weather Endpoints (public — no JWT required) ───────────────────
	// Weather data is cached public information from TMD NWP.
	// Rate-limited to prevent abuse but does NOT require a JWT token.
	if tmdCache != nil {
		tmdH := handler.NewTMDHandler(tmdCache, logger)
		weather := app.Group("/api/v1/weather")
		weather.Use(middleware.RateLimiter(rdb, cfg.App.RateLimit, cfg.App.RateWindow, m, logger))
		weather.Get("/locations", tmdH.ListLocations)
		weather.Get("/:type/:location", tmdH.GetForecast)
	}

	// ── GitHub OAuth Routes (dashboard protection) ───────────────────────
	// Register these BEFORE the static Web UI routes.
	ghHandler := handler.NewGitHubAuthHandler(cfg.GitHub, logger)
	app.Get("/auth/github/login", ghHandler.Login)
	app.Get("/auth/github/callback", ghHandler.Callback)
	app.Get("/auth/github/logout", ghHandler.Logout)

	// ── Web UI ────────────────────────────────────────────────────────────
	// Files are compiled into the binary via go:embed (no volume needed).
	// Each page has an explicit route; unknown paths fall back to intro.html.
	app.Get("/", htmlPage(webUI, "intro.html"))
	app.Get("/login.html", htmlPage(webUI, "login.html"))
	// /dashboard.html is protected — only the owner's GitHub account may access it.
	app.Get("/dashboard.html", middleware.RequireGitHubSession(ghHandler.VerifySession), htmlPage(webUI, "dashboard.html"))
	app.Get("/docs.html", htmlPage(webUI, "docs.html"))
	app.Get("/playground.html", htmlPage(webUI, "playground.html"))
	app.Get("/code-viewer.html", htmlPage(webUI, "code-viewer.html"))
	app.Get("/architecture.html", htmlPage(webUI, "architecture.html"))
	app.Get("/learn-go.html", htmlPage(webUI, "learn-go.html"))
	app.Get("/learn-dart-flutter.html", htmlPage(webUI, "learn-dart-flutter.html"))
	app.Get("/learn-vuejs.html", htmlPage(webUI, "learn-vuejs.html"))
	app.Get("/snippets.json", func(c *fiber.Ctx) error {
		data, err := fs.ReadFile(webUI, "snippets.json")
		if err != nil {
			return fiber.ErrNotFound
		}
		c.Set(fiber.HeaderContentType, "application/json; charset=utf-8")
		return c.Send(data)
	})
	// Fallback: any unmatched path → intro.html (SPA-style)
	app.Use(htmlPage(webUI, "intro.html"))
	return app
}
