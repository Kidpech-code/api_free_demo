package model

import "time"

// Product is the core domain entity.
type Product struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	SKU       string    `json:"sku"`
	Deleted   bool      `json:"deleted"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProductFilter holds query parameters for listing products.
type ProductFilter struct {
	Cursor string
	Limit  int
}

// ProductPage is the response envelope for cursor-based pagination.
type ProductPage struct {
	Items      []Product `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
}
