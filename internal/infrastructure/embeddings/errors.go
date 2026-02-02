// Package embeddings provides infrastructure for the embeddings service.
package embeddings

import "errors"

// Provider errors.
var (
	ErrUnsupportedProvider = errors.New("unsupported embedding provider")
	ErrAPIKeyRequired      = errors.New("API key is required")
	ErrEmptyInput          = errors.New("input text is empty")
	ErrBatchTooLarge       = errors.New("batch size exceeds maximum limit")
	ErrRequestFailed       = errors.New("embedding request failed")
	ErrRateLimited         = errors.New("rate limit exceeded")
	ErrTimeout             = errors.New("request timed out")
	ErrInvalidResponse     = errors.New("invalid API response")
	ErrProviderClosed      = errors.New("provider is closed")
)

// Cache errors.
var (
	ErrCacheNotFound    = errors.New("cache entry not found")
	ErrCacheExpired     = errors.New("cache entry expired")
	ErrCacheDBError     = errors.New("cache database error")
	ErrCacheInitFailed  = errors.New("cache initialization failed")
)
