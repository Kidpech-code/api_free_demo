package usecase

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"api_free_demo/internal/domain/model"
	"api_free_demo/internal/domain/repository"
	domainUC "api_free_demo/internal/domain/usecase"
)

type productUsecase struct {
	repo   repository.ProductRepository
	logger *zap.Logger
}

var _ domainUC.ProductUsecase = (*productUsecase)(nil)

func NewProductUsecase(repo repository.ProductRepository, logger *zap.Logger) domainUC.ProductUsecase {
	return &productUsecase{
		repo:   repo,
		logger: logger.Named("usecase.product"),
	}
}

func (uc *productUsecase) Create(ctx context.Context, userID, name, sku string, price float64) (*model.Product, error) {
	if name == "" {
		return nil, fmt.Errorf("product name is required")
	}
	if price <= 0 {
		return nil, fmt.Errorf("price must be greater than zero")
	}

	p := &model.Product{
		UserID: userID,
		Name:   name,
		Price:  price,
		SKU:    sku,
	}

	if err := uc.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}
	return p, nil
}

func (uc *productUsecase) GetByID(ctx context.Context, userID, productID string) (*model.Product, error) {
	p, err := uc.repo.FindByID(ctx, userID, productID)
	if err != nil {
		return nil, fmt.Errorf("find product: %w", err)
	}
	if p == nil {
		return nil, fmt.Errorf("product %s not found", productID)
	}
	return p, nil
}

func (uc *productUsecase) Update(ctx context.Context, userID, productID, name, sku string, price float64) (*model.Product, error) {
	existing, err := uc.repo.FindByID(ctx, userID, productID)
	if err != nil {
		return nil, fmt.Errorf("find product: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("product %s not found", productID)
	}

	if name != "" {
		existing.Name = name
	}
	if sku != "" {
		existing.SKU = sku
	}
	if price > 0 {
		existing.Price = price
	}
	existing.UpdatedAt = time.Now().UTC()

	if err := uc.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update product: %w", err)
	}
	return existing, nil
}

func (uc *productUsecase) Delete(ctx context.Context, userID, productID string) error {
	if err := uc.repo.SoftDelete(ctx, userID, productID); err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	return nil
}

func (uc *productUsecase) List(ctx context.Context, userID string, filter model.ProductFilter) (*model.ProductPage, error) {
	return uc.repo.List(ctx, userID, filter)
}

func (uc *productUsecase) BulkCreate(ctx context.Context, userID string, items []domainUC.BulkCreateItem) (int, error) {
	if len(items) == 0 {
		return 0, fmt.Errorf("at least one item is required")
	}
	if len(items) > 100 {
		return 0, fmt.Errorf("bulk create limited to 100 items per request")
	}

	products := make([]model.Product, len(items))
	for i, item := range items {
		if item.Name == "" || item.Price <= 0 {
			return 0, fmt.Errorf("item[%d]: name and positive price are required", i)
		}
		products[i] = model.Product{
			UserID: userID,
			Name:   item.Name,
			Price:  item.Price,
			SKU:    item.SKU,
		}
	}

	created, err := uc.repo.BulkCreate(ctx, products)
	if err != nil {
		return 0, fmt.Errorf("bulk create: %w", err)
	}
	return created, nil
}
