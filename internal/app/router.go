package app

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/kidpech/api_free_demo/internal/app/diagnostics"
	"github.com/kidpech/api_free_demo/internal/app/middleware"
	"github.com/kidpech/api_free_demo/internal/config"
	"github.com/kidpech/api_free_demo/internal/domain/profile"
	"github.com/kidpech/api_free_demo/internal/domain/user"
	"github.com/kidpech/api_free_demo/internal/infrastructure/auth"
	"github.com/kidpech/api_free_demo/internal/infrastructure/ratelimit"
)

// RouterDeps aggregates HTTP dependencies.
type RouterDeps struct {
	Config         *config.Config
	UserHandler    *user.Handler
	ProfileHandler *profile.Handler
	Diagnostics    *diagnostics.Handler
	AuthManager    *auth.Manager
	Logger         *zap.Logger
	LogBuffer      *diagnostics.LogBuffer
	IPLimiter      ratelimit.Limiter
	UserLimiter    ratelimit.Limiter
}

// NewRouter builds the gin engine.
func NewRouter(deps RouterDeps) *gin.Engine {
	if deps.Config != nil && deps.Config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	if deps.Config != nil {
		r.Use(middleware.CORS(deps.Config.Cors))
	}
	if deps.AuthManager != nil {
		r.Use(middleware.OptionalAuth(deps.AuthManager))
	}
	if deps.Config == nil || deps.Config.RateLimit.Enabled {
		r.Use(middleware.RateLimit(deps.IPLimiter, deps.UserLimiter))
	}
	r.Use(middleware.RequestLogger(deps.Logger, deps.LogBuffer))

	var authMW gin.HandlerFunc = func(c *gin.Context) { c.Next() }
	if deps.AuthManager != nil {
		authMW = middleware.AuthMiddleware(deps.AuthManager)
	}
	adminMW := middleware.AdminOnly()

	api := r.Group("/api/v1")
	deps.Diagnostics.RegisterPublic(api)

	debug := r.Group("/api/v1")
	debug.Use(authMW, adminMW)
	deps.Diagnostics.RegisterProtected(debug)

	metrics := r.Group("/api/v1")
	metrics.GET("/metrics", gin.WrapH(promhttp.Handler()))

	deps.UserHandler.RegisterRoutes(api, authMW, adminMW)
	deps.ProfileHandler.RegisterRoutes(api, authMW)

	return r
}
