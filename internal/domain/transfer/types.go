// Package transfer provides domain types for pattern transfer and sharing.
package transfer

import (
	"errors"
	"time"
)

// Common errors
var (
	ErrInvalidMagic    = errors.New("invalid CFP magic bytes")
	ErrMissingVersion  = errors.New("missing version")
	ErrMissingName     = errors.New("missing pattern name")
	ErrPatternNotFound = errors.New("pattern not found")
	ErrRegistryNotFound = errors.New("registry not found")
	ErrChecksumMismatch = errors.New("checksum mismatch")
	ErrSignatureInvalid = errors.New("signature verification failed")
	ErrUploadFailed     = errors.New("upload failed")
	ErrDownloadFailed   = errors.New("download failed")
)

// StoreConfig holds pattern store configuration.
type StoreConfig struct {
	Registry     string `json:"registry"`
	Gateway      string `json:"gateway"`
	CacheDir     string `json:"cacheDir"`
	CacheExpiry  int64  `json:"cacheExpiry"` // seconds
	Verification bool   `json:"verification"`
}

// DefaultStoreConfig returns default store configuration.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		Registry:     "claude-flow-official",
		Gateway:      "https://w3s.link",
		CacheDir:     ".claude-flow/patterns/cache",
		CacheExpiry:  3600, // 1 hour
		Verification: true,
	}
}

// DownloadOptions holds download options.
type DownloadOptions struct {
	Name       string
	OutputPath string
	Verify     bool
	Import     bool
}

// PublishOptions holds publish options.
type PublishOptions struct {
	InputPath    string
	Name         string
	Description  string
	Categories   []string
	Tags         []string
	License      string
	Anonymize    AnonymizationLevel
	Language     string
	Framework    string
	Sign         bool
}

// DefaultPublishOptions returns default publish options.
func DefaultPublishOptions() PublishOptions {
	return PublishOptions{
		License:   "MIT",
		Anonymize: AnonymizationStrict,
		Sign:      true,
	}
}

// PublishResult holds the result of a publish operation.
type PublishResult struct {
	CID        string    `json:"cid"`
	GatewayURL string    `json:"gatewayUrl"`
	PatternID  string    `json:"patternId"`
	Checksum   string    `json:"checksum"`
	Size       int64     `json:"size"`
	PublishedAt time.Time `json:"publishedAt"`
}

// DownloadResult holds the result of a download operation.
type DownloadResult struct {
	Path        string    `json:"path"`
	CID         string    `json:"cid"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	Verified    bool      `json:"verified"`
	FromCache   bool      `json:"fromCache"`
	DownloadedAt time.Time `json:"downloadedAt"`
}

// CacheEntry represents a cached pattern.
type CacheEntry struct {
	CID         string    `json:"cid"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	CachedAt    time.Time `json:"cachedAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	HitCount    int       `json:"hitCount"`
}

// IsExpired checks if the cache entry has expired.
func (e CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// SearchOptions holds search options.
type SearchOptions struct {
	Query     string
	Category  string
	Language  string
	Framework string
	Tags      []string
	MinRating float64
	Verified  bool
	Limit     int
}

// ListOptions holds list options.
type ListOptions struct {
	Registry string
	Category string
	Featured bool
	Trending bool
	Newest   bool
	Limit    int
	Format   string
}

// InfoOptions holds info options.
type InfoOptions struct {
	Name   string
	AsJSON bool
}

// StorageBackend defines the interface for storage backends.
type StorageBackend interface {
	Upload(data []byte, name string) (string, error)    // Returns CID or URI
	Download(cid string) ([]byte, error)
	Exists(cid string) (bool, error)
	Delete(cid string) error
}

// BootstrapRegistry represents a bootstrap registry.
type BootstrapRegistry struct {
	Name    string `json:"name"`
	IPNS    string `json:"ipns"`
	Trusted bool   `json:"trusted"`
}

// DefaultBootstrapRegistries returns the default bootstrap registries.
func DefaultBootstrapRegistries() []BootstrapRegistry {
	return []BootstrapRegistry{
		{
			Name:    "claude-flow-official",
			IPNS:    "k51qzi5uqu5dggso7kiw5y4ot7bxbqcqbfnpewl3x4zf5z7r5rfzewvvnqwz0z",
			Trusted: true,
		},
		{
			Name:    "community-patterns",
			IPNS:    "k51qzi5uqu5dgcommunitypatternsexample",
			Trusted: false,
		},
	}
}

// IPFSGateways returns the list of IPFS gateways to try.
func IPFSGateways() []string {
	return []string{
		"https://w3s.link/ipfs/",
		"https://ipfs.io/ipfs/",
		"https://dweb.link/ipfs/",
		"https://cloudflare-ipfs.com/ipfs/",
		"https://gateway.pinata.cloud/ipfs/",
	}
}
