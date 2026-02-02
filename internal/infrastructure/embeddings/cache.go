// Package embeddings provides infrastructure for the embeddings service.
package embeddings

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	domainEmbeddings "github.com/anthropics/claude-flow-go/internal/domain/embeddings"
	_ "modernc.org/sqlite"
)

// Cache provides persistent caching for embeddings using SQLite.
type Cache struct {
	mu     sync.RWMutex
	db     *sql.DB
	config domainEmbeddings.CacheConfig
	closed bool
}

// NewCache creates a new persistent embedding cache.
func NewCache(config domainEmbeddings.CacheConfig) (*Cache, error) {
	if !config.Enabled {
		return nil, nil
	}

	// Ensure directory exists
	if config.DBPath != "" {
		dir := filepath.Dir(config.DBPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("%w: failed to create cache directory: %v", ErrCacheInitFailed, err)
		}
	}

	db, err := sql.Open("sqlite", config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open database: %v", ErrCacheInitFailed, err)
	}

	cache := &Cache{
		db:     db,
		config: config,
	}

	if err := cache.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return cache, nil
}

// initSchema creates the cache tables if they don't exist.
func (c *Cache) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS embeddings (
			key TEXT PRIMARY KEY,
			embedding BLOB NOT NULL,
			dimensions INTEGER NOT NULL,
			provider TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			accessed_at INTEGER NOT NULL,
			access_count INTEGER DEFAULT 1
		);
		CREATE INDEX IF NOT EXISTS idx_accessed_at ON embeddings(accessed_at);
		CREATE INDEX IF NOT EXISTS idx_created_at ON embeddings(created_at);
	`

	if _, err := c.db.Exec(schema); err != nil {
		return fmt.Errorf("%w: failed to create schema: %v", ErrCacheInitFailed, err)
	}

	return nil
}

// Get retrieves an embedding from the cache.
func (c *Cache) Get(key string) (*domainEmbeddings.CacheEntry, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, ErrCacheDBError
	}
	c.mu.RUnlock()

	var embeddingBlob []byte
	var dimensions int
	var provider string
	var createdAt, accessedAt int64
	var accessCount int

	err := c.db.QueryRow(`
		SELECT embedding, dimensions, provider, created_at, accessed_at, access_count
		FROM embeddings WHERE key = ?
	`, key).Scan(&embeddingBlob, &dimensions, &provider, &createdAt, &accessedAt, &accessCount)

	if err == sql.ErrNoRows {
		return nil, ErrCacheNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCacheDBError, err)
	}

	// Check TTL
	if c.config.TTLMs > 0 {
		expiresAt := createdAt + c.config.TTLMs
		if time.Now().UnixMilli() > expiresAt {
			// Entry expired, delete it
			c.Delete(key)
			return nil, ErrCacheExpired
		}
	}

	// Decode embedding
	embedding := decodeEmbedding(embeddingBlob, dimensions)

	// Update access time and count
	go c.updateAccess(key)

	return &domainEmbeddings.CacheEntry{
		Key:         key,
		Embedding:   embedding,
		Dimensions:  dimensions,
		Provider:    domainEmbeddings.ProviderType(provider),
		CreatedAt:   time.UnixMilli(createdAt),
		AccessedAt:  time.Now(),
		AccessCount: accessCount + 1,
	}, nil
}

// Set stores an embedding in the cache.
func (c *Cache) Set(key string, embedding []float32, provider domainEmbeddings.ProviderType) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrCacheDBError
	}
	c.mu.RUnlock()

	// Encode embedding
	blob := encodeEmbedding(embedding)
	now := time.Now().UnixMilli()

	_, err := c.db.Exec(`
		INSERT OR REPLACE INTO embeddings 
		(key, embedding, dimensions, provider, created_at, accessed_at, access_count)
		VALUES (?, ?, ?, ?, ?, ?, 1)
	`, key, blob, len(embedding), string(provider), now, now)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrCacheDBError, err)
	}

	// Check if eviction is needed
	go c.evictIfNeeded()

	return nil
}

// Delete removes an embedding from the cache.
func (c *Cache) Delete(key string) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrCacheDBError
	}
	c.mu.RUnlock()

	_, err := c.db.Exec(`DELETE FROM embeddings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCacheDBError, err)
	}

	return nil
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrCacheDBError
	}
	c.mu.RUnlock()

	_, err := c.db.Exec(`DELETE FROM embeddings`)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCacheDBError, err)
	}

	return nil
}

// Size returns the number of entries in the cache.
func (c *Cache) Size() (int, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, ErrCacheDBError
	}
	c.mu.RUnlock()

	var count int
	err := c.db.QueryRow(`SELECT COUNT(*) FROM embeddings`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrCacheDBError, err)
	}

	return count, nil
}

// updateAccess updates the access time and count for an entry.
func (c *Cache) updateAccess(key string) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()

	c.db.Exec(`
		UPDATE embeddings 
		SET accessed_at = ?, access_count = access_count + 1
		WHERE key = ?
	`, time.Now().UnixMilli(), key)
}

// evictIfNeeded evicts oldest entries if cache size exceeds max.
func (c *Cache) evictIfNeeded() {
	if c.config.MaxSize <= 0 {
		return
	}

	size, err := c.Size()
	if err != nil || size <= c.config.MaxSize {
		return
	}

	// Evict oldest 10% of entries
	evictCount := c.config.MaxSize / 10
	if evictCount < 1 {
		evictCount = 1
	}

	c.db.Exec(`
		DELETE FROM embeddings WHERE key IN (
			SELECT key FROM embeddings ORDER BY accessed_at ASC LIMIT ?
		)
	`, evictCount)
}

// Close closes the cache database.
func (c *Cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.db.Close()
}

// GetStats returns cache statistics.
func (c *Cache) GetStats() (*CacheStats, error) {
	size, err := c.Size()
	if err != nil {
		return nil, err
	}

	return &CacheStats{
		Size:    size,
		MaxSize: c.config.MaxSize,
	}, nil
}

// CacheStats contains cache statistics.
type CacheStats struct {
	Size    int
	MaxSize int
}

// ========================================================================
// Key Generation
// ========================================================================

// GenerateCacheKey generates a cache key from text and provider.
func GenerateCacheKey(text string, provider domainEmbeddings.ProviderType) string {
	h := fnv.New64a()
	h.Write([]byte(text))
	h.Write([]byte(":"))
	h.Write([]byte(provider))
	return fmt.Sprintf("%016x", h.Sum64())
}

// ========================================================================
// Encoding/Decoding
// ========================================================================

// encodeEmbedding encodes an embedding to bytes.
func encodeEmbedding(embedding []float32) []byte {
	buf := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// decodeEmbedding decodes bytes to an embedding.
func decodeEmbedding(data []byte, dimensions int) []float32 {
	embedding := make([]float32, dimensions)
	for i := 0; i < dimensions && i*4+4 <= len(data); i++ {
		bits := binary.LittleEndian.Uint32(data[i*4:])
		embedding[i] = math.Float32frombits(bits)
	}
	return embedding
}

// ========================================================================
// In-Memory Cache (for fast access)
// ========================================================================

// MemoryCache provides an in-memory LRU cache for embeddings.
type MemoryCache struct {
	mu       sync.RWMutex
	cache    map[string]*memoryCacheEntry
	order    []string
	maxSize  int
}

type memoryCacheEntry struct {
	embedding  []float32
	accessTime time.Time
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(maxSize int) *MemoryCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &MemoryCache{
		cache:   make(map[string]*memoryCacheEntry),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

// Get retrieves an embedding from memory.
func (c *MemoryCache) Get(key string) ([]float32, bool) {
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Update access time
	c.mu.Lock()
	entry.accessTime = time.Now()
	c.mu.Unlock()

	return entry.embedding, true
}

// Set stores an embedding in memory.
func (c *MemoryCache) Set(key string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if _, exists := c.cache[key]; exists {
		c.cache[key].embedding = embedding
		c.cache[key].accessTime = time.Now()
		return
	}

	// Evict if needed
	if len(c.cache) >= c.maxSize {
		c.evictOldest()
	}

	c.cache[key] = &memoryCacheEntry{
		embedding:  embedding,
		accessTime: time.Now(),
	}
	c.order = append(c.order, key)
}

// evictOldest removes the least recently used entry.
func (c *MemoryCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}

	// Find oldest entry
	oldestIdx := 0
	oldestTime := c.cache[c.order[0]].accessTime

	for i, key := range c.order {
		if entry, ok := c.cache[key]; ok && entry.accessTime.Before(oldestTime) {
			oldestTime = entry.accessTime
			oldestIdx = i
		}
	}

	// Remove oldest
	oldestKey := c.order[oldestIdx]
	delete(c.cache, oldestKey)
	c.order = append(c.order[:oldestIdx], c.order[oldestIdx+1:]...)
}

// Size returns the cache size.
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Clear clears the cache.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*memoryCacheEntry)
	c.order = make([]string, 0, c.maxSize)
}
