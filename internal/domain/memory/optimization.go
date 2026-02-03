// Package memory provides the Memory domain entity and related types.
package memory

import (
	"time"
)

// StorageTier represents storage tier levels.
type StorageTier string

const (
	// TierHot is the fastest tier (in-memory).
	TierHot StorageTier = "hot"
	// TierWarm is the medium tier (local storage).
	TierWarm StorageTier = "warm"
	// TierCold is the slow tier (compressed archive).
	TierCold StorageTier = "cold"
	// TierArchive is the archive tier (long-term storage).
	TierArchive StorageTier = "archive"
)

// EvictionPolicy represents cache eviction policies.
type EvictionPolicy string

const (
	// EvictionLRU is least-recently-used eviction.
	EvictionLRU EvictionPolicy = "lru"
	// EvictionLFU is least-frequently-used eviction.
	EvictionLFU EvictionPolicy = "lfu"
	// EvictionFIFO is first-in-first-out eviction.
	EvictionFIFO EvictionPolicy = "fifo"
	// EvictionRandom is random eviction.
	EvictionRandom EvictionPolicy = "random"
	// EvictionTTL is time-to-live based eviction.
	EvictionTTL EvictionPolicy = "ttl"
)

// CompressionType represents compression algorithms.
type CompressionType string

const (
	// CompressionNone indicates no compression.
	CompressionNone CompressionType = "none"
	// CompressionGzip uses gzip compression.
	CompressionGzip CompressionType = "gzip"
	// CompressionZstd uses zstd compression.
	CompressionZstd CompressionType = "zstd"
	// CompressionLZ4 uses lz4 compression.
	CompressionLZ4 CompressionType = "lz4"
)

// OptimizationPolicy defines optimization behavior.
type OptimizationPolicy struct {
	// TierThresholds maps tiers to age thresholds in milliseconds.
	TierThresholds map[StorageTier]int64 `json:"tierThresholds,omitempty"`

	// AccessCountThresholds maps tiers to access count thresholds.
	AccessCountThresholds map[StorageTier]int `json:"accessCountThresholds,omitempty"`

	// AutoMigration enables automatic tier migration.
	AutoMigration bool `json:"autoMigration,omitempty"`

	// MigrationIntervalMs is the migration check interval.
	MigrationIntervalMs int64 `json:"migrationIntervalMs,omitempty"`

	// Compression is the compression type for cold storage.
	Compression CompressionType `json:"compression,omitempty"`

	// CompressAfterMs compresses memories older than this.
	CompressAfterMs int64 `json:"compressAfterMs,omitempty"`
}

// DefaultOptimizationPolicy returns the default optimization policy.
func DefaultOptimizationPolicy() OptimizationPolicy {
	return OptimizationPolicy{
		TierThresholds: map[StorageTier]int64{
			TierHot:     60 * 60 * 1000,           // 1 hour
			TierWarm:    24 * 60 * 60 * 1000,      // 24 hours
			TierCold:    7 * 24 * 60 * 60 * 1000,  // 7 days
			TierArchive: 30 * 24 * 60 * 60 * 1000, // 30 days
		},
		AccessCountThresholds: map[StorageTier]int{
			TierHot:  10,
			TierWarm: 5,
			TierCold: 1,
		},
		AutoMigration:       true,
		MigrationIntervalMs: 60 * 60 * 1000, // 1 hour
		Compression:         CompressionGzip,
		CompressAfterMs:     7 * 24 * 60 * 60 * 1000, // 7 days
	}
}

// CleanupConfig configures memory cleanup.
type CleanupConfig struct {
	// Enabled enables cleanup.
	Enabled bool `json:"enabled"`

	// MaxAgeMs is the maximum memory age in milliseconds.
	MaxAgeMs int64 `json:"maxAgeMs,omitempty"`

	// MinAccessCount is the minimum access count to keep.
	MinAccessCount int `json:"minAccessCount,omitempty"`

	// MaxMemoryCount is the maximum memory count.
	MaxMemoryCount int `json:"maxMemoryCount,omitempty"`

	// MaxStorageBytes is the maximum storage in bytes.
	MaxStorageBytes int64 `json:"maxStorageBytes,omitempty"`

	// CleanupIntervalMs is the cleanup check interval.
	CleanupIntervalMs int64 `json:"cleanupIntervalMs,omitempty"`

	// DeleteOrphanedEmbeddings deletes embeddings without memories.
	DeleteOrphanedEmbeddings bool `json:"deleteOrphanedEmbeddings,omitempty"`
}

// DefaultCleanupConfig returns the default cleanup configuration.
func DefaultCleanupConfig() CleanupConfig {
	return CleanupConfig{
		Enabled:                  true,
		MaxAgeMs:                 90 * 24 * 60 * 60 * 1000, // 90 days
		MinAccessCount:           0,
		MaxMemoryCount:           100000,
		MaxStorageBytes:          1024 * 1024 * 1024, // 1GB
		CleanupIntervalMs:        24 * 60 * 60 * 1000, // 24 hours
		DeleteOrphanedEmbeddings: true,
	}
}

// CacheConfig configures memory caching.
type CacheConfig struct {
	// Enabled enables caching.
	Enabled bool `json:"enabled"`

	// HotTierSize is the hot tier size (number of entries).
	HotTierSize int `json:"hotTierSize,omitempty"`

	// WarmTierSize is the warm tier size.
	WarmTierSize int `json:"warmTierSize,omitempty"`

	// EvictionPolicy is the cache eviction policy.
	EvictionPolicy EvictionPolicy `json:"evictionPolicy,omitempty"`

	// TTLMs is the default time-to-live in milliseconds.
	TTLMs int64 `json:"ttlMs,omitempty"`

	// PreloadHot preloads frequently accessed memories to hot tier.
	PreloadHot bool `json:"preloadHot,omitempty"`
}

// DefaultCacheConfig returns the default cache configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:        true,
		HotTierSize:    1000,
		WarmTierSize:   10000,
		EvictionPolicy: EvictionLRU,
		TTLMs:          24 * 60 * 60 * 1000, // 24 hours
		PreloadHot:     true,
	}
}

// TierStats contains statistics for a storage tier.
type TierStats struct {
	// Tier is the tier identifier.
	Tier StorageTier `json:"tier"`

	// Count is the number of memories in this tier.
	Count int64 `json:"count"`

	// SizeBytes is the storage size in bytes.
	SizeBytes int64 `json:"sizeBytes"`

	// CompressedBytes is the compressed size (if applicable).
	CompressedBytes int64 `json:"compressedBytes,omitempty"`

	// HitCount is the cache hit count.
	HitCount int64 `json:"hitCount"`

	// MissCount is the cache miss count.
	MissCount int64 `json:"missCount"`

	// AvgAccessTimeMs is the average access time.
	AvgAccessTimeMs float64 `json:"avgAccessTimeMs"`

	// OldestMemoryMs is the age of the oldest memory.
	OldestMemoryMs int64 `json:"oldestMemoryMs"`

	// NewestMemoryMs is the age of the newest memory.
	NewestMemoryMs int64 `json:"newestMemoryMs"`
}

// HitRate returns the cache hit rate.
func (s *TierStats) HitRate() float64 {
	total := s.HitCount + s.MissCount
	if total == 0 {
		return 0
	}
	return float64(s.HitCount) / float64(total)
}

// CompressionRatio returns the compression ratio.
func (s *TierStats) CompressionRatio() float64 {
	if s.SizeBytes == 0 {
		return 1
	}
	if s.CompressedBytes == 0 {
		return 1
	}
	return float64(s.CompressedBytes) / float64(s.SizeBytes)
}

// OptimizationStats contains overall optimization statistics.
type OptimizationStats struct {
	// TierStats is stats per tier.
	TierStats map[StorageTier]*TierStats `json:"tierStats"`

	// TotalMemories is the total memory count.
	TotalMemories int64 `json:"totalMemories"`

	// TotalSizeBytes is the total storage size.
	TotalSizeBytes int64 `json:"totalSizeBytes"`

	// CompressedSizeBytes is the total compressed size.
	CompressedSizeBytes int64 `json:"compressedSizeBytes"`

	// MemoryReductionPercent is the memory reduction percentage.
	MemoryReductionPercent float64 `json:"memoryReductionPercent"`

	// LastMigrationAt is when the last migration occurred.
	LastMigrationAt *time.Time `json:"lastMigrationAt,omitempty"`

	// LastCleanupAt is when the last cleanup occurred.
	LastCleanupAt *time.Time `json:"lastCleanupAt,omitempty"`

	// CleanedCount is the number of cleaned memories.
	CleanedCount int64 `json:"cleanedCount"`

	// MigratedCount is the number of migrated memories.
	MigratedCount int64 `json:"migratedCount"`
}

// MigrationTask represents a tier migration task.
type MigrationTask struct {
	// ID is the task identifier.
	ID string `json:"id"`

	// MemoryID is the memory to migrate.
	MemoryID string `json:"memoryId"`

	// FromTier is the source tier.
	FromTier StorageTier `json:"fromTier"`

	// ToTier is the destination tier.
	ToTier StorageTier `json:"toTier"`

	// Reason is the migration reason.
	Reason string `json:"reason"`

	// CreatedAt is when the task was created.
	CreatedAt time.Time `json:"createdAt"`

	// CompletedAt is when the task completed.
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// Error is any error during migration.
	Error string `json:"error,omitempty"`
}

// MemoryAccessLog tracks memory access patterns.
type MemoryAccessLog struct {
	// MemoryID is the accessed memory.
	MemoryID string `json:"memoryId"`

	// AccessCount is the total access count.
	AccessCount int `json:"accessCount"`

	// LastAccessAt is the last access time.
	LastAccessAt time.Time `json:"lastAccessAt"`

	// AccessTimes is recent access timestamps.
	AccessTimes []time.Time `json:"accessTimes,omitempty"`

	// CurrentTier is the current storage tier.
	CurrentTier StorageTier `json:"currentTier"`
}
