package domain

import "errors"

// Sentinel errors for the domain layer.
// These allow handlers to inspect error types without coupling to infrastructure details.
var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrDeleted       = errors.New("resource has been deleted")
)
