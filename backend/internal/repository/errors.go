package repository

import "errors"

// Common repository errors.
var (
	ErrNotFound = errors.New("resource not found")
	ErrConflict = errors.New("resource already exists")
)
