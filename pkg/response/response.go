package response

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// ErrorResponse standardizes API errors.
type ErrorResponse struct {
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// ValidationError writes 400 payloads.
func ValidationError(c *gin.Context, err error) {
	resp := ErrorResponse{Error: "validation_error", Message: "invalid request"}
	var verr validator.ValidationErrors
	if errors.As(err, &verr) {
		detail := make(map[string]string)
		for _, field := range verr {
			detail[strings.ToLower(field.Field())] = field.Tag()
		}
		resp.Details = detail
	}
	c.JSON(http.StatusBadRequest, resp)
}

// Unauthorized helper.
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: message})
}

// Forbidden helper.
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden", Message: message})
}

// NotFound helper.
func NotFound(c *gin.Context, resource string) {
	msg := "resource not found"
	if resource != "" {
		msg = resource + " not found"
	}
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "not_found", Message: msg})
}

// Conflict helper.
func Conflict(c *gin.Context, code, message string) {
	c.JSON(http.StatusConflict, ErrorResponse{Error: code, Message: message})
}

// TooManyRequests helper.
func TooManyRequests(c *gin.Context, reset time.Time) {
	resetSeconds := strconv.FormatInt(reset.Unix(), 10)
	retryAfter := int(time.Until(reset).Seconds())
	if retryAfter < 0 {
		retryAfter = 0
	}
	c.Writer.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	c.Writer.Header().Set("X-RateLimit-Reset", resetSeconds)
	c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: "rate_limited", Message: "slow down"})
}

// InternalServerError helper.
func InternalServerError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "unexpected error"})
	if gin.IsDebugging() {
		c.Error(err) // surface for logs
	}
}

// Paginated writes envelope.
func Paginated(c *gin.Context, data interface{}, total, offset, limit int) {
	c.JSON(http.StatusOK, gin.H{
		"data":   data,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

// GetLimit parses query limit.
func GetLimit(c *gin.Context, fallback, max int) int {
	limitStr := c.Query("limit")
	if limitStr == "" {
		return fallback
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return fallback
	}
	if max > 0 && limit > max {
		return max
	}
	return limit
}

// GetOffset parses offset query.
func GetOffset(c *gin.Context) int {
	offsetStr := c.Query("offset")
	if offsetStr == "" {
		return 0
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}

// SerializeErrors returns list of error strings for bulk APIs.
func SerializeErrors(errs []error) []string {
	if len(errs) == 0 {
		return nil
	}
	out := make([]string, len(errs))
	for i, err := range errs {
		if err != nil {
			out[i] = err.Error()
		}
	}
	return out
}

// MustUserID ensures contexts contain user_id.
func MustUserID(c *gin.Context) uuid.UUID {
	val, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "missing user context")
		c.Abort()
		return uuid.Nil
	}
	id, ok := val.(uuid.UUID)
	if !ok {
		Unauthorized(c, "invalid user context")
		c.Abort()
		return uuid.Nil
	}
	return id
}

// UserIDFromContext extracts string for rate limiting.
func UserIDFromContext(c *gin.Context) string {
	val, exists := c.Get("user_id")
	if !exists {
		return ""
	}
	if id, ok := val.(uuid.UUID); ok {
		return id.String()
	}
	return ""
}
