package diagnostics

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler exposes health + debug endpoints.
type Handler struct {
	buffer *LogBuffer
}

// NewHandler returns handler.
func NewHandler(buffer *LogBuffer) *Handler {
	return &Handler{buffer: buffer}
}

// RegisterPublic attaches non-auth endpoints.
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/health", h.health)
}

// RegisterProtected attaches debug endpoints requiring auth.
func (h *Handler) RegisterProtected(rg *gin.RouterGroup) {
	rg.GET("/debug/logs", h.logs)
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) logs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": h.buffer.Snapshot()})
}
