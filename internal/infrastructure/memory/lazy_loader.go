// Package memory provides infrastructure for memory management.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// LazyLoader provides lazy loading for memories.
type LazyLoader struct {
	mu        sync.RWMutex
	config    LazyLoaderConfig
	backend   shared.MemoryBackend
	cache     map[string]*lazyEntry
	pending   map[string]*loadRequest
	stats     *LazyLoaderStats
	hydrating bool
}

// lazyEntry represents a lazy-loaded memory entry.
type lazyEntry struct {
	// ID is the memory ID.
	ID string

	// Loaded indicates if full memory is loaded.
	Loaded bool

	// Memory is the full memory (nil if not loaded).
	Memory *shared.Memory

	// Metadata contains minimal metadata for unloaded entries.
	Metadata *MemoryMetadata

	// LoadedAt is when the memory was loaded.
	LoadedAt *time.Time

	// AccessCount tracks access for preloading.
	AccessCount int
}

// MemoryMetadata contains minimal memory metadata.
type MemoryMetadata struct {
	ID        string               `json:"id"`
	AgentID   string               `json:"agentId"`
	Type      shared.MemoryType    `json:"type"`
	Timestamp int64                `json:"timestamp"`
	HasEmbed  bool                 `json:"hasEmbed"`
}

// loadRequest tracks pending load requests.
type loadRequest struct {
	ID        string
	Requested time.Time
	Callbacks []func(*shared.Memory, error)
}

// LazyLoaderConfig configures the lazy loader.
type LazyLoaderConfig struct {
	// Enabled enables lazy loading.
	Enabled bool `json:"enabled"`

	// CacheSize is the loaded memory cache size.
	CacheSize int `json:"cacheSize"`

	// PreloadThreshold is the access count threshold for preloading.
	PreloadThreshold int `json:"preloadThreshold"`

	// BackgroundHydration enables background hydration.
	BackgroundHydration bool `json:"backgroundHydration"`

	// HydrationBatchSize is the batch size for hydration.
	HydrationBatchSize int `json:"hydrationBatchSize"`

	// HydrationIntervalMs is the hydration interval.
	HydrationIntervalMs int64 `json:"hydrationIntervalMs"`

	// EvictionPolicy is the cache eviction policy.
	EvictionPolicy string `json:"evictionPolicy"`
}

// DefaultLazyLoaderConfig returns the default configuration.
func DefaultLazyLoaderConfig() LazyLoaderConfig {
	return LazyLoaderConfig{
		Enabled:             true,
		CacheSize:           1000,
		PreloadThreshold:    3,
		BackgroundHydration: true,
		HydrationBatchSize:  50,
		HydrationIntervalMs: 5000,
		EvictionPolicy:      "lru",
	}
}

// LazyLoaderStats contains loader statistics.
type LazyLoaderStats struct {
	// TotalEntries is the total entry count.
	TotalEntries int64 `json:"totalEntries"`

	// LoadedEntries is the loaded entry count.
	LoadedEntries int64 `json:"loadedEntries"`

	// UnloadedEntries is the unloaded entry count.
	UnloadedEntries int64 `json:"unloadedEntries"`

	// LoadRequests is the total load request count.
	LoadRequests int64 `json:"loadRequests"`

	// CacheHits is the cache hit count.
	CacheHits int64 `json:"cacheHits"`

	// CacheMisses is the cache miss count.
	CacheMisses int64 `json:"cacheMisses"`

	// BackgroundLoads is the background load count.
	BackgroundLoads int64 `json:"backgroundLoads"`

	// EvictedEntries is the evicted entry count.
	EvictedEntries int64 `json:"evictedEntries"`

	// AvgLoadTimeMs is the average load time.
	AvgLoadTimeMs float64 `json:"avgLoadTimeMs"`
}

// NewLazyLoader creates a new lazy loader.
func NewLazyLoader(backend shared.MemoryBackend, config LazyLoaderConfig) *LazyLoader {
	return &LazyLoader{
		config:  config,
		backend: backend,
		cache:   make(map[string]*lazyEntry),
		pending: make(map[string]*loadRequest),
		stats:   &LazyLoaderStats{},
	}
}

// Register registers a memory for lazy loading.
func (l *LazyLoader) Register(metadata *MemoryMetadata) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.cache[metadata.ID]; exists {
		return
	}

	l.cache[metadata.ID] = &lazyEntry{
		ID:       metadata.ID,
		Loaded:   false,
		Metadata: metadata,
	}

	l.stats.TotalEntries++
	l.stats.UnloadedEntries++
}

// RegisterMemory registers a full memory (already loaded).
func (l *LazyLoader) RegisterMemory(memory *shared.Memory) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.cache[memory.ID] = &lazyEntry{
		ID:       memory.ID,
		Loaded:   true,
		Memory:   memory,
		LoadedAt: &now,
		Metadata: &MemoryMetadata{
			ID:        memory.ID,
			AgentID:   memory.AgentID,
			Type:      memory.Type,
			Timestamp: memory.Timestamp,
			HasEmbed:  len(memory.Embedding) > 0,
		},
	}

	l.stats.TotalEntries++
	l.stats.LoadedEntries++
}

// Get retrieves a memory, loading it if necessary.
func (l *LazyLoader) Get(ctx context.Context, id string) (*shared.Memory, error) {
	l.mu.Lock()
	entry, exists := l.cache[id]
	if !exists {
		l.mu.Unlock()
		l.stats.CacheMisses++
		// Load directly from backend
		return l.loadFromBackend(ctx, id)
	}

	entry.AccessCount++

	if entry.Loaded {
		l.stats.CacheHits++
		memory := entry.Memory
		l.mu.Unlock()
		return memory, nil
	}

	l.stats.CacheMisses++
	l.stats.LoadRequests++
	l.mu.Unlock()

	// Load the memory
	return l.load(ctx, id)
}

// GetMetadata retrieves only metadata without loading.
func (l *LazyLoader) GetMetadata(id string) *MemoryMetadata {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry, exists := l.cache[id]
	if !exists {
		return nil
	}

	return entry.Metadata
}

// IsLoaded checks if a memory is loaded.
func (l *LazyLoader) IsLoaded(id string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry, exists := l.cache[id]
	if !exists {
		return false
	}

	return entry.Loaded
}

// load loads a memory from the backend.
func (l *LazyLoader) load(ctx context.Context, id string) (*shared.Memory, error) {
	startTime := time.Now()

	memory, err := l.loadFromBackend(ctx, id)
	if err != nil {
		return nil, err
	}

	loadTime := time.Since(startTime)

	l.mu.Lock()
	defer l.mu.Unlock()

	if entry, exists := l.cache[id]; exists {
		now := time.Now()
		entry.Loaded = true
		entry.Memory = memory
		entry.LoadedAt = &now

		l.stats.LoadedEntries++
		l.stats.UnloadedEntries--
	}

	// Update average load time
	l.updateAvgLoadTime(loadTime)

	// Check cache size and evict if needed
	l.evictIfNeeded()

	return memory, nil
}

// loadFromBackend loads a memory from the backend.
func (l *LazyLoader) loadFromBackend(ctx context.Context, id string) (*shared.Memory, error) {
	memories, err := l.backend.Query(shared.MemoryQuery{})
	if err != nil {
		return nil, err
	}

	for _, m := range memories {
		if m.ID == id {
			return &m, nil
		}
	}

	// Try to retrieve by ID
	m, err := l.backend.Retrieve(id)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// Preload preloads frequently accessed memories.
func (l *LazyLoader) Preload(ctx context.Context) int {
	l.mu.Lock()

	toLoad := make([]string, 0)
	for id, entry := range l.cache {
		if !entry.Loaded && entry.AccessCount >= l.config.PreloadThreshold {
			toLoad = append(toLoad, id)
			if len(toLoad) >= l.config.HydrationBatchSize {
				break
			}
		}
	}

	l.mu.Unlock()

	loaded := 0
	for _, id := range toLoad {
		if _, err := l.load(ctx, id); err == nil {
			loaded++
			l.stats.BackgroundLoads++
		}
	}

	return loaded
}

// Unload unloads a memory to free space.
func (l *LazyLoader) Unload(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.cache[id]
	if !exists || !entry.Loaded {
		return false
	}

	entry.Loaded = false
	entry.Memory = nil
	entry.LoadedAt = nil

	l.stats.LoadedEntries--
	l.stats.UnloadedEntries++

	return true
}

// evictIfNeeded evicts entries if cache is full.
func (l *LazyLoader) evictIfNeeded() {
	if int(l.stats.LoadedEntries) <= l.config.CacheSize {
		return
	}

	// Find LRU entry to evict
	var oldestID string
	var oldestTime *time.Time

	for id, entry := range l.cache {
		if !entry.Loaded {
			continue
		}
		if oldestTime == nil || (entry.LoadedAt != nil && entry.LoadedAt.Before(*oldestTime)) {
			oldestID = id
			oldestTime = entry.LoadedAt
		}
	}

	if oldestID != "" {
		entry := l.cache[oldestID]
		entry.Loaded = false
		entry.Memory = nil
		entry.LoadedAt = nil

		l.stats.LoadedEntries--
		l.stats.UnloadedEntries++
		l.stats.EvictedEntries++
	}
}

// updateAvgLoadTime updates average load time.
func (l *LazyLoader) updateAvgLoadTime(duration time.Duration) {
	ms := float64(duration.Microseconds()) / 1000.0
	if l.stats.LoadRequests == 1 {
		l.stats.AvgLoadTimeMs = ms
	} else {
		l.stats.AvgLoadTimeMs = (l.stats.AvgLoadTimeMs*float64(l.stats.LoadRequests-1) + ms) / float64(l.stats.LoadRequests)
	}
}

// StartHydration starts background hydration.
func (l *LazyLoader) StartHydration(ctx context.Context) {
	l.mu.Lock()
	if l.hydrating {
		l.mu.Unlock()
		return
	}
	l.hydrating = true
	l.mu.Unlock()

	go func() {
		ticker := time.NewTicker(time.Duration(l.config.HydrationIntervalMs) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				l.mu.Lock()
				l.hydrating = false
				l.mu.Unlock()
				return
			case <-ticker.C:
				l.Preload(ctx)
			}
		}
	}()
}

// GetStats returns loader statistics.
func (l *LazyLoader) GetStats() *LazyLoaderStats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.stats
}

// Clear clears all entries.
func (l *LazyLoader) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cache = make(map[string]*lazyEntry)
	l.pending = make(map[string]*loadRequest)
	l.stats.TotalEntries = 0
	l.stats.LoadedEntries = 0
	l.stats.UnloadedEntries = 0
}

// Count returns the total entry count.
func (l *LazyLoader) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.cache)
}

// LoadedCount returns the loaded entry count.
func (l *LazyLoader) LoadedCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return int(l.stats.LoadedEntries)
}
