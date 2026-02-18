package handler

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"api_free_demo/internal/delivery/http/middleware"
	"api_free_demo/internal/domain"
	"api_free_demo/internal/domain/model"
	"api_free_demo/internal/domain/usecase"
	"api_free_demo/pkg/response"
)

// ProductHandler handles HTTP requests for the Product resource.
type ProductHandler struct {
	uc     usecase.ProductUsecase
	logger *zap.Logger
}

func NewProductHandler(uc usecase.ProductUsecase, logger *zap.Logger) *ProductHandler {
	return &ProductHandler{uc: uc, logger: logger.Named("handler.product")}
}

func userID(c *fiber.Ctx) string {
	uid, _ := c.Locals(middleware.ContextKeyUserID).(string)
	return uid
}

// Create handles POST /api/v1/products
func (h *ProductHandler) Create(c *fiber.Ctx) error {
	var body struct {
		Name  string  `json:"name"`
		Price float64 `json:"price"`
		SKU   string  `json:"sku"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	product, err := h.uc.Create(c.UserContext(), userID(c), body.Name, body.SKU, body.Price)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, product)
}

// GetByID handles GET /api/v1/products/:id
func (h *ProductHandler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	product, err := h.uc.GetByID(c.UserContext(), userID(c), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrDeleted) {
			return response.NotFound(c, err.Error())
		}
		return response.InternalError(c, err.Error())
	}
	return response.OK(c, product)
}

// Update handles PUT /api/v1/products/:id
func (h *ProductHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		Name  string  `json:"name"`
		Price float64 `json:"price"`
		SKU   string  `json:"sku"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	product, err := h.uc.Update(c.UserContext(), userID(c), id, body.Name, body.SKU, body.Price)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, product)
}

// Delete handles DELETE /api/v1/products/:id (soft delete)
func (h *ProductHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.uc.Delete(c.UserContext(), userID(c), id); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.OK(c, fiber.Map{"deleted": true})
}

// List handles GET /api/v1/products
func (h *ProductHandler) List(c *fiber.Ctx) error {
	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	page, err := h.uc.List(c.UserContext(), userID(c), model.ProductFilter{
		Cursor: cursor,
		Limit:  limit,
	})
	if err != nil {
		return response.InternalError(c, err.Error())
	}

	return response.OKWithMeta(c, page.Items, &response.Meta{
		Count:      len(page.Items),
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	})
}

// BulkCreate handles POST /api/v1/products/bulk
func (h *ProductHandler) BulkCreate(c *fiber.Ctx) error {
	var body struct {
		Items []usecase.BulkCreateItem `json:"items"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "invalid request body")
	}

	created, err := h.uc.BulkCreate(c.UserContext(), userID(c), body.Items)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, fiber.Map{"created": created})
}
