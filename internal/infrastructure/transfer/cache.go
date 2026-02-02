// Package transfer provides infrastructure for pattern transfer and sharing.
package transfer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
)

// Cache manages pattern caching.
type Cache struct {
	mu       sync.RWMutex
	basePath string
	expiry   time.Duration
	entries  map[string]transfer.CacheEntry
	indexPath string
}

// NewCache creates a new pattern cache.
func NewCache(basePath string, expirySeconds int64) (*Cache, error) {
	if basePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		basePath = filepath.Join(home, ".claude-flow", "patterns", "cache")
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	expiry := time.Duration(expirySeconds) * time.Second
	if expirySeconds <= 0 {
		expiry = time.Hour
	}

	cache := &Cache{
		basePath:  basePath,
		expiry:    expiry,
		entries:   make(map[string]transfer.CacheEntry),
		indexPath: filepath.Join(basePath, "index.json"),
	}

	// Load existing index
	cache.loadIndex()

	return cache, nil
}

// loadIndex loads the cache index from disk.
func (c *Cache) loadIndex() {
	data, err := os.ReadFile(c.indexPath)
	if err != nil {
		return // Index doesn't exist yet
	}

	var entries map[string]transfer.CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}

	c.entries = entries
}

// saveIndex saves the cache index to disk.
func (c *Cache) saveIndex() error {
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.indexPath, data, 0644)
}

// Get retrieves a cached pattern.
func (c *Cache) Get(cid string) ([]byte, *transfer.CacheEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[cid]
	if !exists {
		return nil, nil, transfer.ErrPatternNotFound
	}

	// Check expiry
	if entry.IsExpired() {
		// Remove expired entry
		delete(c.entries, cid)
		os.Remove(entry.Path)
		c.saveIndex()
		return nil, nil, transfer.ErrPatternNotFound
	}

	// Read data
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return nil, nil, err
	}

	// Update hit count
	entry.HitCount++
	c.entries[cid] = entry
	c.saveIndex()

	return data, &entry, nil
}

// Put stores a pattern in the cache.
func (c *Cache) Put(cid string, data []byte, checksum string) (*transfer.CacheEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := filepath.Join(c.basePath, cid+".cfp")

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write cache file: %w", err)
	}

	entry := transfer.CacheEntry{
		CID:       cid,
		Path:      path,
		Size:      int64(len(data)),
		Checksum:  checksum,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(c.expiry),
		HitCount:  0,
	}

	c.entries[cid] = entry

	if err := c.saveIndex(); err != nil {
		return nil, err
	}

	return &entry, nil
}

// Has checks if a pattern is cached.
func (c *Cache) Has(cid string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[cid]
	if !exists {
		return false
	}

	return !entry.IsExpired()
}

// Remove removes a pattern from the cache.
func (c *Cache) Remove(cid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[cid]
	if !exists {
		return nil
	}

	delete(c.entries, cid)
	os.Remove(entry.Path)

	return c.saveIndex()
}

// Clear clears all cached patterns.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entry := range c.entries {
		os.Remove(entry.Path)
	}

	c.entries = make(map[string]transfer.CacheEntry)
	return c.saveIndex()
}

// Prune removes expired entries.
func (c *Cache) Prune() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	pruned := 0
	for cid, entry := range c.entries {
		if entry.IsExpired() {
			delete(c.entries, cid)
			os.Remove(entry.Path)
			pruned++
		}
	}

	if pruned > 0 {
		if err := c.saveIndex(); err != nil {
			return pruned, err
		}
	}

	return pruned, nil
}

// Stats returns cache statistics.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		EntryCount: len(c.entries),
	}

	for _, entry := range c.entries {
		stats.TotalSize += entry.Size
		stats.TotalHits += entry.HitCount
	}

	return stats
}

// CacheStats holds cache statistics.
type CacheStats struct {
	EntryCount int   `json:"entryCount"`
	TotalSize  int64 `json:"totalSize"`
	TotalHits  int   `json:"totalHits"`
}

// Entries returns all cache entries.
func (c *Cache) Entries() []transfer.CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := make([]transfer.CacheEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, entry)
	}
	return entries
}
