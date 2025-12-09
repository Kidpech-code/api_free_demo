package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kidpech/api_free_demo/pkg/response"
)

// Handler wires HTTP routes to the Service.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mounts auth + user routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMW gin.HandlerFunc, adminMW gin.HandlerFunc) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register", h.register)
		auth.POST("/login", h.login)
		auth.POST("/refresh", h.refresh)
	}

	me := rg.Group("/users/me", authMW)
	{
		me.GET("", h.getMe)
		me.PUT("", h.updateMe)
	}

	admin := rg.Group("/admin", authMW, adminMW)
	{
		admin.GET("/users", h.listUsers)
	}
}

func (h *Handler) register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	ctx := c.Request.Context()
	res, err := h.service.Register(ctx, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.Header("Location", "/api/v1/users/me")
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	ctx := c.Request.Context()
	res, err := h.service.Login(ctx, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) refresh(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, err)
		return
	}
	ctx := c.Request.Context()
	res, err := h.service.Refresh(ctx, body.RefreshToken)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) getMe(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		response.Unauthorized(c, "missing auth context")
		return
	}
	ctx := c.Request.Context()
	usr, err := h.service.GetMe(ctx, userID.(uuid.UUID))
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, usr)
}

func (h *Handler) updateMe(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	userID, ok := c.Get("user_id")
	if !ok {
		response.Unauthorized(c, "missing auth context")
		return
	}
	ctx := c.Request.Context()
	usr, err := h.service.UpdateMe(ctx, userID.(uuid.UUID), req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, usr)
}

func (h *Handler) listUsers(c *gin.Context) {
	filter := UserFilter{
		Search: c.Query("search"),
		Limit:  response.GetLimit(c, 50, 200),
		Offset: response.GetOffset(c),
		Sort:   c.DefaultQuery("sort", "created_at_desc"),
	}
	ctx := c.Request.Context()
	users, total, err := h.service.List(ctx, filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Paginated(c, users, total, filter.Offset, filter.Limit)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrDuplicateEmail):
		response.Conflict(c, "duplicate_email", "email already registered")
	case errors.Is(err, ErrInvalidCreds):
		response.Unauthorized(c, "invalid credentials")
	case errors.Is(err, ErrRegistrationDisabled):
		response.Forbidden(c, "registration disabled")
	case errors.Is(err, ErrForbidden):
		response.Forbidden(c, "forbidden")
	case errors.Is(err, ErrUserNotFound):
		response.NotFound(c, "user")
	case errors.Is(err, ErrInvalidToken):
		response.Unauthorized(c, "invalid token")
	default:
		response.InternalServerError(c, err)
	}
}
