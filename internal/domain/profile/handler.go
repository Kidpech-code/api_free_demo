package profile

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kidpech/api_free_demo/pkg/response"
)

// Handler wires profile endpoints.
type Handler struct {
	service *Service
}

// NewHandler returns a profile Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes attaches routes onto router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMW gin.HandlerFunc) {
	authed := rg.Group("", authMW)
	authed.POST("/profiles", h.create)
	authed.POST("/profiles/bulk", h.bulkCreate)
	authed.GET("/profiles", h.list)
	authed.GET("/profiles/:id", h.get)
	authed.PUT("/profiles/:id", h.update)
	authed.PATCH("/profiles/:id", h.patch)
	authed.DELETE("/profiles/:id", h.delete)
	authed.DELETE("/profiles/bulk", h.bulkDelete)
}

func (h *Handler) create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	userID := response.MustUserID(c)
	if userID == uuid.Nil {
		return
	}
	profile, err := h.service.Create(c.Request.Context(), userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.Header("Location", "/api/v1/profiles/"+profile.ID.String())
	c.JSON(http.StatusCreated, profile)
}

func (h *Handler) bulkCreate(c *gin.Context) {
	var req BulkCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	userID := response.MustUserID(c)
	if userID == uuid.Nil {
		return
	}
	created, errs := h.service.BulkCreate(c.Request.Context(), userID, req)
	c.JSON(http.StatusMultiStatus, gin.H{
		"created": created,
		"errors":  response.SerializeErrors(errs),
	})
}

func (h *Handler) get(c *gin.Context) {
	userID := response.MustUserID(c)
	if userID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "profile")
		return
	}
	profile, err := h.service.Get(c.Request.Context(), id, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *Handler) list(c *gin.Context) {
	userID := response.MustUserID(c)
	if userID == uuid.Nil {
		return
	}
	cursor := c.Query("cursor")
	filter := Filter{
		Search: c.Query("search"),
		Limit:  response.GetLimit(c, 20, 100),
		Offset: response.GetOffset(c),
		Cursor: cursor,
		UserID: userID,
	}
	if cursor != "" {
		if ts, err := time.Parse(time.RFC3339, cursor); err == nil {
			filter.CursorTime = &ts
		}
	}
	profiles, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		h.handleError(c, err)
		return
	}
	nextCursor := ""
	if len(profiles) > 0 {
		nextCursor = profiles[len(profiles)-1].CreatedAt.Format(time.RFC3339)
	}
	c.JSON(http.StatusOK, gin.H{
		"data":        profiles,
		"total":       total,
		"offset":      filter.Offset,
		"limit":       filter.Limit,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) update(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	userID := response.MustUserID(c)
	if userID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "profile")
		return
	}
	profile, err := h.service.Update(c.Request.Context(), id, userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *Handler) patch(c *gin.Context) {
	var req PatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	userID := response.MustUserID(c)
	if userID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "profile")
		return
	}
	profile, err := h.service.Patch(c.Request.Context(), id, userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *Handler) delete(c *gin.Context) {
	userID := response.MustUserID(c)
	if userID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "profile")
		return
	}
	hard := c.Query("hard") == "true"
	version := parseVersion(c.Query("version"))
	if err := h.service.Delete(c.Request.Context(), id, userID, hard, version); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) bulkDelete(c *gin.Context) {
	var req BulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	userID := response.MustUserID(c)
	if userID == uuid.Nil {
		return
	}
	deleted, err := h.service.BulkDelete(c.Request.Context(), userID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

func parseVersion(val string) int {
	if val == "" {
		return 0
	}
	version, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return version
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.NotFound(c, "profile")
	case errors.Is(err, ErrForbidden):
		response.Forbidden(c, "forbidden")
	case errors.Is(err, ErrVersionConflict):
		response.Conflict(c, "version_conflict", "profile updated elsewhere")
	default:
		response.InternalServerError(c, err)
	}
}
