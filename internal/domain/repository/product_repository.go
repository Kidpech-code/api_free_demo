package repository

import (
	"context"

	"api_free_demo/internal/domain/model"
)

// ProductRepository defines the contract for the "DocStore" data layer.
type ProductRepository interface {
	Create(ctx context.Context, product *model.Product) error
	FindByID(ctx context.Context, userID, productID string) (*model.Product, error)
	Update(ctx context.Context, product *model.Product) error
	SoftDelete(ctx context.Context, userID, productID string) error
	HardDelete(ctx context.Context, userID, productID string) error
	BulkCreate(ctx context.Context, products []model.Product) (int, error)
	BulkFindByIDs(ctx context.Context, userID string, ids []string) ([]model.Product, error)
	List(ctx context.Context, userID string, filter model.ProductFilter) (*model.ProductPage, error)
	Count(ctx context.Context, userID string) (int64, error)
}
