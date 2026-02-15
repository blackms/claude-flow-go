// Package resources provides MCP resource registry implementation.
package resources

import (
	"math"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// CachedResource represents a cached resource with TTL.
type CachedResource struct {
	Content   *shared.ResourceContent
	ExpiresAt int64
}

// ResourceCache implements an LRU cache for resources with TTL.
type ResourceCache struct {
	mu          sync.RWMutex
	entries     map[string]*CachedResource
	accessOrder []string // For LRU eviction
	config      shared.ResourceCacheConfig
}

// NewResourceCache creates a new resource cache.
func NewResourceCache(config shared.ResourceCacheConfig) *ResourceCache {
	config = normalizeResourceCacheConfig(config)
	return &ResourceCache{
		entries:     make(map[string]*CachedResource),
		accessOrder: make([]string, 0),
		config:      config,
	}
}

func normalizeResourceCacheConfig(config shared.ResourceCacheConfig) shared.ResourceCacheConfig {
	defaults := shared.DefaultResourceCacheConfig()

	if config.MaxEntries <= 0 {
		config.MaxEntries = defaults.MaxEntries
	}
	if config.TTLSeconds <= 0 {
		config.TTLSeconds = defaults.TTLSeconds
	}

	maxTTLSec := int64(math.MaxInt64 / 1000)
	if config.TTLSeconds > maxTTLSec {
		config.TTLSeconds = maxTTLSec
	}

	return config
}

// NewResourceCacheWithDefaults creates a resource cache with default configuration.
func NewResourceCacheWithDefaults() *ResourceCache {
	return NewResourceCache(shared.DefaultResourceCacheConfig())
}

// Get retrieves a resource from cache.
func (c *ResourceCache) Get(uri string) (*shared.ResourceContent, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[uri]
	if !exists {
		return nil, false
	}

	// Check if expired
	if shared.Now() > entry.ExpiresAt {
		c.removeLocked(uri)
		return nil, false
	}

	// Update access order for LRU
	c.updateAccessOrderLocked(uri)

	return cloneResourceContent(entry.Content), true
}

// Set adds or updates a resource in the cache.
func (c *ResourceCache) Set(uri string, content *shared.ResourceContent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	for len(c.entries) >= c.config.MaxEntries {
		c.evictOldestLocked()
	}

	now := shared.Now()
	ttlMillis := c.config.TTLSeconds * 1000
	expiresAt := now + ttlMillis
	if ttlMillis > math.MaxInt64-now {
		expiresAt = math.MaxInt64
	}

	c.entries[uri] = &CachedResource{
		Content:   cloneResourceContent(content),
		ExpiresAt: expiresAt,
	}

	c.updateAccessOrderLocked(uri)
}

// Invalidate removes a resource from the cache.
func (c *ResourceCache) Invalidate(uri string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(uri)
}

// InvalidateAll clears the entire cache.
func (c *ResourceCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CachedResource)
	c.accessOrder = make([]string, 0)
}

// Size returns the number of entries in the cache.
func (c *ResourceCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// updateAccessOrderLocked updates the access order (caller must hold lock).
func (c *ResourceCache) updateAccessOrderLocked(uri string) {
	// Remove from current position
	for i, u := range c.accessOrder {
		if u == uri {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			break
		}
	}
	// Add to end (most recently accessed)
	c.accessOrder = append(c.accessOrder, uri)
}

// evictOldestLocked removes the least recently accessed entry (caller must hold lock).
func (c *ResourceCache) evictOldestLocked() {
	if len(c.accessOrder) == 0 {
		return
	}
	oldest := c.accessOrder[0]
	c.removeLocked(oldest)
}

// removeLocked removes an entry from the cache (caller must hold lock).
func (c *ResourceCache) removeLocked(uri string) {
	delete(c.entries, uri)
	for i, u := range c.accessOrder {
		if u == uri {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			break
		}
	}
}

// Cleanup removes expired entries.
func (c *ResourceCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := shared.Now()
	removed := 0

	for uri, entry := range c.entries {
		if now > entry.ExpiresAt {
			c.removeLocked(uri)
			removed++
		}
	}

	return removed
}

// GetConfig returns the cache configuration.
func (c *ResourceCache) GetConfig() shared.ResourceCacheConfig {
	return c.config
}

// Stats holds cache statistics.
type CacheStats struct {
	Size       int   `json:"size"`
	MaxSize    int   `json:"maxSize"`
	TTLSeconds int64 `json:"ttlSeconds"`
}

// GetStats returns cache statistics.
func (c *ResourceCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:       len(c.entries),
		MaxSize:    c.config.MaxEntries,
		TTLSeconds: c.config.TTLSeconds,
	}
}
