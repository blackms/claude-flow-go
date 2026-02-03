// Package memory provides infrastructure for memory management.
package memory

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"time"

	domainMemory "github.com/anthropics/claude-flow-go/internal/domain/memory"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// TieredStorage implements tiered memory storage.
type TieredStorage struct {
	mu     sync.RWMutex
	config TieredStorageConfig

	// Storage tiers
	hotTier    map[string]*tieredEntry
	warmTier   map[string]*tieredEntry
	coldTier   map[string]*tieredEntry
	accessLog  map[string]*domainMemory.MemoryAccessLog

	// Statistics
	stats *domainMemory.OptimizationStats

	// Eviction tracking
	hotOrder  []string
	warmOrder []string
}

// tieredEntry wraps a memory with tier metadata.
type tieredEntry struct {
	Memory      *shared.Memory
	Tier        domainMemory.StorageTier
	Compressed  bool
	Data        []byte // For compressed storage
	AccessCount int64
	LastAccess  time.Time
	CreatedAt   time.Time
	SizeBytes   int64
}

// TieredStorageConfig configures tiered storage.
type TieredStorageConfig struct {
	// CacheConfig for tier sizes and eviction.
	CacheConfig domainMemory.CacheConfig

	// Policy for optimization.
	Policy domainMemory.OptimizationPolicy

	// Cleanup configuration.
	Cleanup domainMemory.CleanupConfig
}

// DefaultTieredStorageConfig returns the default configuration.
func DefaultTieredStorageConfig() TieredStorageConfig {
	return TieredStorageConfig{
		CacheConfig: domainMemory.DefaultCacheConfig(),
		Policy:      domainMemory.DefaultOptimizationPolicy(),
		Cleanup:     domainMemory.DefaultCleanupConfig(),
	}
}

// NewTieredStorage creates a new tiered storage.
func NewTieredStorage(config TieredStorageConfig) *TieredStorage {
	return &TieredStorage{
		config:    config,
		hotTier:   make(map[string]*tieredEntry),
		warmTier:  make(map[string]*tieredEntry),
		coldTier:  make(map[string]*tieredEntry),
		accessLog: make(map[string]*domainMemory.MemoryAccessLog),
		stats: &domainMemory.OptimizationStats{
			TierStats: map[domainMemory.StorageTier]*domainMemory.TierStats{
				domainMemory.TierHot:  {},
				domainMemory.TierWarm: {},
				domainMemory.TierCold: {},
			},
		},
		hotOrder:  make([]string, 0),
		warmOrder: make([]string, 0),
	}
}

// Store stores a memory, placing it in the appropriate tier.
func (s *TieredStorage) Store(memory *shared.Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := &tieredEntry{
		Memory:     memory,
		Tier:       domainMemory.TierHot,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		SizeBytes:  s.estimateSize(memory),
	}

	// Check if hot tier has space
	if len(s.hotTier) >= s.config.CacheConfig.HotTierSize {
		s.evictFromHot()
	}

	s.hotTier[memory.ID] = entry
	s.hotOrder = append(s.hotOrder, memory.ID)
	s.updateStats()

	return nil
}

// Get retrieves a memory by ID.
func (s *TieredStorage) Get(id string) (*shared.Memory, domainMemory.StorageTier, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check hot tier
	if entry, ok := s.hotTier[id]; ok {
		s.recordAccess(id, domainMemory.TierHot, true)
		atomic.AddInt64(&entry.AccessCount, 1)
		entry.LastAccess = time.Now()
		return entry.Memory, domainMemory.TierHot, nil
	}

	// Check warm tier
	if entry, ok := s.warmTier[id]; ok {
		s.recordAccess(id, domainMemory.TierWarm, true)
		atomic.AddInt64(&entry.AccessCount, 1)
		entry.LastAccess = time.Now()
		// Promote to hot if frequently accessed
		if entry.AccessCount > 5 {
			s.promoteToHot(id, entry)
		}
		return entry.Memory, domainMemory.TierWarm, nil
	}

	// Check cold tier
	if entry, ok := s.coldTier[id]; ok {
		s.recordAccess(id, domainMemory.TierCold, true)
		atomic.AddInt64(&entry.AccessCount, 1)
		entry.LastAccess = time.Now()

		// Decompress if needed
		memory := entry.Memory
		if entry.Compressed {
			var err error
			memory, err = s.decompress(entry.Data)
			if err != nil {
				return nil, domainMemory.TierCold, err
			}
		}

		// Promote to warm
		s.promoteToWarm(id, entry, memory)
		return memory, domainMemory.TierCold, nil
	}

	s.recordAccess(id, "", false)
	return nil, "", nil
}

// Delete removes a memory from all tiers.
func (s *TieredStorage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.hotTier, id)
	delete(s.warmTier, id)
	delete(s.coldTier, id)
	delete(s.accessLog, id)

	s.removeFromOrder(&s.hotOrder, id)
	s.removeFromOrder(&s.warmOrder, id)

	s.updateStats()
	return nil
}

// MigrateToWarm moves memories from hot to warm tier.
func (s *TieredStorage) MigrateToWarm() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	threshold := s.config.Policy.TierThresholds[domainMemory.TierHot]
	now := time.Now()
	migrated := 0

	for id, entry := range s.hotTier {
		age := now.Sub(entry.LastAccess).Milliseconds()
		if age > threshold {
			s.warmTier[id] = entry
			entry.Tier = domainMemory.TierWarm
			s.warmOrder = append(s.warmOrder, id)
			delete(s.hotTier, id)
			s.removeFromOrder(&s.hotOrder, id)
			migrated++
		}
	}

	if migrated > 0 {
		s.updateStats()
	}
	return migrated
}

// MigrateToCold moves memories from warm to cold tier with compression.
func (s *TieredStorage) MigrateToCold() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	threshold := s.config.Policy.TierThresholds[domainMemory.TierWarm]
	now := time.Now()
	migrated := 0

	for id, entry := range s.warmTier {
		age := now.Sub(entry.LastAccess).Milliseconds()
		if age > threshold {
			// Compress the memory
			if s.config.Policy.Compression != domainMemory.CompressionNone {
				compressed, err := s.compress(entry.Memory)
				if err == nil {
					entry.Data = compressed
					entry.Compressed = true
					entry.Memory = nil // Free memory
				}
			}

			s.coldTier[id] = entry
			entry.Tier = domainMemory.TierCold
			delete(s.warmTier, id)
			s.removeFromOrder(&s.warmOrder, id)
			migrated++
		}
	}

	if migrated > 0 {
		s.updateStats()
	}
	return migrated
}

// evictFromHot evicts the least recently used entry from hot tier.
func (s *TieredStorage) evictFromHot() {
	if len(s.hotOrder) == 0 {
		return
	}

	// LRU: evict first (oldest) entry
	id := s.hotOrder[0]
	s.hotOrder = s.hotOrder[1:]

	if entry, ok := s.hotTier[id]; ok {
		// Move to warm tier
		s.warmTier[id] = entry
		entry.Tier = domainMemory.TierWarm
		s.warmOrder = append(s.warmOrder, id)
		delete(s.hotTier, id)

		// Check warm tier size
		if len(s.warmTier) > s.config.CacheConfig.WarmTierSize {
			s.evictFromWarm()
		}
	}
}

// evictFromWarm evicts from warm tier to cold.
func (s *TieredStorage) evictFromWarm() {
	if len(s.warmOrder) == 0 {
		return
	}

	id := s.warmOrder[0]
	s.warmOrder = s.warmOrder[1:]

	if entry, ok := s.warmTier[id]; ok {
		// Compress and move to cold
		if s.config.Policy.Compression != domainMemory.CompressionNone {
			compressed, err := s.compress(entry.Memory)
			if err == nil {
				entry.Data = compressed
				entry.Compressed = true
				entry.Memory = nil
			}
		}

		s.coldTier[id] = entry
		entry.Tier = domainMemory.TierCold
		delete(s.warmTier, id)
	}
}

// promoteToHot promotes an entry to hot tier.
func (s *TieredStorage) promoteToHot(id string, entry *tieredEntry) {
	if len(s.hotTier) >= s.config.CacheConfig.HotTierSize {
		s.evictFromHot()
	}

	delete(s.warmTier, id)
	s.removeFromOrder(&s.warmOrder, id)

	s.hotTier[id] = entry
	entry.Tier = domainMemory.TierHot
	s.hotOrder = append(s.hotOrder, id)
}

// promoteToWarm promotes an entry to warm tier.
func (s *TieredStorage) promoteToWarm(id string, entry *tieredEntry, memory *shared.Memory) {
	delete(s.coldTier, id)

	entry.Memory = memory
	entry.Compressed = false
	entry.Data = nil
	entry.Tier = domainMemory.TierWarm

	s.warmTier[id] = entry
	s.warmOrder = append(s.warmOrder, id)
}

// compress compresses a memory.
func (s *TieredStorage) compress(memory *shared.Memory) ([]byte, error) {
	data, err := json.Marshal(memory)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompress decompresses memory data.
func (s *TieredStorage) decompress(data []byte) (*shared.Memory, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var memory shared.Memory
	if err := json.Unmarshal(decompressed, &memory); err != nil {
		return nil, err
	}

	return &memory, nil
}

// recordAccess records memory access.
func (s *TieredStorage) recordAccess(id string, tier domainMemory.StorageTier, hit bool) {
	log := s.accessLog[id]
	if log == nil {
		log = &domainMemory.MemoryAccessLog{
			MemoryID:    id,
			CurrentTier: tier,
		}
		s.accessLog[id] = log
	}

	log.AccessCount++
	log.LastAccessAt = time.Now()
	log.CurrentTier = tier

	// Update tier stats
	if hit {
		if stats, ok := s.stats.TierStats[tier]; ok {
			stats.HitCount++
		}
	} else {
		for _, stats := range s.stats.TierStats {
			stats.MissCount++
		}
	}
}

// removeFromOrder removes an ID from an order slice.
func (s *TieredStorage) removeFromOrder(order *[]string, id string) {
	for i, oid := range *order {
		if oid == id {
			*order = append((*order)[:i], (*order)[i+1:]...)
			return
		}
	}
}

// estimateSize estimates memory size in bytes.
func (s *TieredStorage) estimateSize(memory *shared.Memory) int64 {
	size := int64(len(memory.Content))
	size += int64(len(memory.Embedding) * 8) // 8 bytes per float64
	size += 100                               // Metadata overhead
	return size
}

// updateStats updates storage statistics.
func (s *TieredStorage) updateStats() {
	// Hot tier stats
	hotStats := s.stats.TierStats[domainMemory.TierHot]
	hotStats.Count = int64(len(s.hotTier))
	hotStats.SizeBytes = 0
	for _, entry := range s.hotTier {
		hotStats.SizeBytes += entry.SizeBytes
	}

	// Warm tier stats
	warmStats := s.stats.TierStats[domainMemory.TierWarm]
	warmStats.Count = int64(len(s.warmTier))
	warmStats.SizeBytes = 0
	for _, entry := range s.warmTier {
		warmStats.SizeBytes += entry.SizeBytes
	}

	// Cold tier stats
	coldStats := s.stats.TierStats[domainMemory.TierCold]
	coldStats.Count = int64(len(s.coldTier))
	coldStats.SizeBytes = 0
	coldStats.CompressedBytes = 0
	for _, entry := range s.coldTier {
		coldStats.SizeBytes += entry.SizeBytes
		if entry.Compressed {
			coldStats.CompressedBytes += int64(len(entry.Data))
		}
	}

	// Overall stats
	s.stats.TotalMemories = hotStats.Count + warmStats.Count + coldStats.Count
	s.stats.TotalSizeBytes = hotStats.SizeBytes + warmStats.SizeBytes + coldStats.SizeBytes
	s.stats.CompressedSizeBytes = coldStats.CompressedBytes

	if s.stats.TotalSizeBytes > 0 {
		actualSize := hotStats.SizeBytes + warmStats.SizeBytes + coldStats.CompressedBytes
		s.stats.MemoryReductionPercent = (1 - float64(actualSize)/float64(s.stats.TotalSizeBytes)) * 100
	}
}

// GetStats returns storage statistics.
func (s *TieredStorage) GetStats() *domainMemory.OptimizationStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.updateStats()
	return s.stats
}

// GetTier returns the tier for a memory ID.
func (s *TieredStorage) GetTier(id string) domainMemory.StorageTier {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.hotTier[id]; ok {
		return domainMemory.TierHot
	}
	if _, ok := s.warmTier[id]; ok {
		return domainMemory.TierWarm
	}
	if _, ok := s.coldTier[id]; ok {
		return domainMemory.TierCold
	}
	return ""
}

// Count returns total memory count.
func (s *TieredStorage) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.hotTier) + len(s.warmTier) + len(s.coldTier)
}
