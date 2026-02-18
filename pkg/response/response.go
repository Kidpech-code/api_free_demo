package response

import "github.com/gofiber/fiber/v2"

// Envelope is the standard API response wrapper.
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta carries pagination or summary metadata.
type Meta struct {
	Total      int64  `json:"total,omitempty"`
	Count      int    `json:"count,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more,omitempty"`
}

// OK sends a 200 success response.
func OK(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Envelope{Success: true, Data: data})
}

// Created sends a 201 response.
func Created(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Envelope{Success: true, Data: data})
}

// OKWithMeta sends a 200 with pagination meta.
func OKWithMeta(c *fiber.Ctx, data interface{}, meta *Meta) error {
	return c.Status(fiber.StatusOK).JSON(Envelope{Success: true, Data: data, Meta: meta})
}

// Err sends an error response with the given status code.
func Err(c *fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(Envelope{Success: false, Error: msg})
}

// BadRequest is a 400 shorthand.
func BadRequest(c *fiber.Ctx, msg string) error {
	return Err(c, fiber.StatusBadRequest, msg)
}

// NotFound is a 404 shorthand.
func NotFound(c *fiber.Ctx, msg string) error {
	return Err(c, fiber.StatusNotFound, msg)
}

// InternalError is a 500 shorthand.
func InternalError(c *fiber.Ctx, msg string) error {
	return Err(c, fiber.StatusInternalServerError, msg)
}
