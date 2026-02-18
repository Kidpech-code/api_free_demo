package usecase

import (
	"context"

	"api_free_demo/internal/domain/model"
)

// ProductUsecase defines the business-logic contract.
// It is intentionally agnostic of Redis, HTTP, or any infrastructure detail.
type ProductUsecase interface {
	Create(ctx context.Context, userID, name, sku string, price float64) (*model.Product, error)
	GetByID(ctx context.Context, userID, productID string) (*model.Product, error)
	Update(ctx context.Context, userID, productID, name, sku string, price float64) (*model.Product, error)
	Delete(ctx context.Context, userID, productID string) error
	List(ctx context.Context, userID string, filter model.ProductFilter) (*model.ProductPage, error)

	// Bulk
	BulkCreate(ctx context.Context, userID string, items []BulkCreateItem) (int, error)
}

// BulkCreateItem carries input data for a single item in a bulk-create request.
type BulkCreateItem struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	SKU   string  `json:"sku"`
}
