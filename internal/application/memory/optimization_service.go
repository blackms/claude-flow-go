// Package memory provides application services for memory management.
package memory

import (
	"context"
	"sync"
	"time"

	domainMemory "github.com/anthropics/claude-flow-go/internal/domain/memory"
	infraMemory "github.com/anthropics/claude-flow-go/internal/infrastructure/memory"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// OptimizationService provides memory optimization operations.
type OptimizationService struct {
	mu            sync.RWMutex
	backend       shared.MemoryBackend
	tieredStorage *infraMemory.TieredStorage
	config        OptimizationServiceConfig
	running       bool
	stopChan      chan struct{}
	lastMigration *time.Time
	lastCleanup   *time.Time
}

// OptimizationServiceConfig configures the optimization service.
type OptimizationServiceConfig struct {
	// Policy is the optimization policy.
	Policy domainMemory.OptimizationPolicy `json:"policy"`

	// Cleanup is the cleanup configuration.
	Cleanup domainMemory.CleanupConfig `json:"cleanup"`

	// Cache is the cache configuration.
	Cache domainMemory.CacheConfig `json:"cache"`

	// MigrationIntervalMs is the tier migration interval.
	MigrationIntervalMs int64 `json:"migrationIntervalMs"`

	// CleanupIntervalMs is the cleanup interval.
	CleanupIntervalMs int64 `json:"cleanupIntervalMs"`

	// EnableAutoOptimization enables automatic optimization.
	EnableAutoOptimization bool `json:"enableAutoOptimization"`
}

// DefaultOptimizationServiceConfig returns the default configuration.
func DefaultOptimizationServiceConfig() OptimizationServiceConfig {
	return OptimizationServiceConfig{
		Policy:                 domainMemory.DefaultOptimizationPolicy(),
		Cleanup:                domainMemory.DefaultCleanupConfig(),
		Cache:                  domainMemory.DefaultCacheConfig(),
		MigrationIntervalMs:    60 * 60 * 1000, // 1 hour
		CleanupIntervalMs:      24 * 60 * 60 * 1000, // 24 hours
		EnableAutoOptimization: true,
	}
}

// NewOptimizationService creates a new optimization service.
func NewOptimizationService(backend shared.MemoryBackend, tieredStorage *infraMemory.TieredStorage, config OptimizationServiceConfig) *OptimizationService {
	return &OptimizationService{
		backend:       backend,
		tieredStorage: tieredStorage,
		config:        config,
	}
}

// Start starts the optimization service.
func (s *OptimizationService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	if s.config.EnableAutoOptimization {
		go s.optimizationLoop(ctx)
	}

	return nil
}

// Stop stops the optimization service.
func (s *OptimizationService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)
}

// optimizationLoop runs the optimization loop.
func (s *OptimizationService) optimizationLoop(ctx context.Context) {
	migrationTicker := time.NewTicker(time.Duration(s.config.MigrationIntervalMs) * time.Millisecond)
	cleanupTicker := time.NewTicker(time.Duration(s.config.CleanupIntervalMs) * time.Millisecond)

	defer migrationTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-migrationTicker.C:
			s.RunMigration(ctx)
		case <-cleanupTicker.C:
			s.RunCleanup(ctx)
		}
	}
}

// RunMigration runs tier migration.
func (s *OptimizationService) RunMigration(ctx context.Context) *MigrationResult {
	if s.tieredStorage == nil {
		return &MigrationResult{}
	}

	s.mu.Lock()
	now := time.Now()
	s.lastMigration = &now
	s.mu.Unlock()

	result := &MigrationResult{
		StartedAt: now,
	}

	// Migrate hot to warm
	result.HotToWarm = s.tieredStorage.MigrateToWarm()

	// Migrate warm to cold
	result.WarmToCold = s.tieredStorage.MigrateToCold()

	result.CompletedAt = time.Now()
	result.TotalMigrated = result.HotToWarm + result.WarmToCold

	return result
}

// RunCleanup runs memory cleanup.
func (s *OptimizationService) RunCleanup(ctx context.Context) *CleanupResult {
	s.mu.Lock()
	now := time.Now()
	s.lastCleanup = &now
	s.mu.Unlock()

	result := &CleanupResult{
		StartedAt: now,
	}

	if !s.config.Cleanup.Enabled {
		result.CompletedAt = time.Now()
		return result
	}

	// Get all memories
	memories, err := s.backend.Query(shared.MemoryQuery{})
	if err != nil {
		result.Error = err.Error()
		result.CompletedAt = time.Now()
		return result
	}

	// Find memories to clean
	toDelete := make([]string, 0)
	cutoffTime := time.Now().UnixMilli() - s.config.Cleanup.MaxAgeMs

	for _, m := range memories {
		// Check age
		if s.config.Cleanup.MaxAgeMs > 0 && m.Timestamp < cutoffTime {
			toDelete = append(toDelete, m.ID)
			result.ExpiredCount++
		}
	}

	// Check count limit
	if s.config.Cleanup.MaxMemoryCount > 0 && len(memories) > s.config.Cleanup.MaxMemoryCount {
		// Sort by timestamp and delete oldest
		excess := len(memories) - s.config.Cleanup.MaxMemoryCount
		for i := 0; i < excess && i < len(memories); i++ {
			id := memories[i].ID
			found := false
			for _, d := range toDelete {
				if d == id {
					found = true
					break
				}
			}
			if !found {
				toDelete = append(toDelete, id)
				result.ExcessCount++
			}
		}
	}

	// Delete memories
	for _, id := range toDelete {
		if err := s.backend.Delete(id); err == nil {
			result.DeletedCount++
			if s.tieredStorage != nil {
				s.tieredStorage.Delete(id)
			}
		}
	}

	result.CompletedAt = time.Now()
	return result
}

// Optimize runs all optimization operations.
func (s *OptimizationService) Optimize(ctx context.Context) *OptimizationResult {
	result := &OptimizationResult{
		StartedAt: time.Now(),
	}

	result.Migration = s.RunMigration(ctx)
	result.Cleanup = s.RunCleanup(ctx)

	if s.tieredStorage != nil {
		result.Stats = s.tieredStorage.GetStats()
	}

	result.CompletedAt = time.Now()
	return result
}

// GetStats returns optimization statistics.
func (s *OptimizationService) GetStats() *domainMemory.OptimizationStats {
	if s.tieredStorage != nil {
		return s.tieredStorage.GetStats()
	}
	return &domainMemory.OptimizationStats{
		TierStats: make(map[domainMemory.StorageTier]*domainMemory.TierStats),
	}
}

// GetTierStats returns statistics for a specific tier.
func (s *OptimizationService) GetTierStats(tier domainMemory.StorageTier) *domainMemory.TierStats {
	stats := s.GetStats()
	if stats.TierStats != nil {
		return stats.TierStats[tier]
	}
	return nil
}

// SetPolicy updates the optimization policy.
func (s *OptimizationService) SetPolicy(policy domainMemory.OptimizationPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Policy = policy
}

// SetCleanupConfig updates the cleanup configuration.
func (s *OptimizationService) SetCleanupConfig(cleanup domainMemory.CleanupConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Cleanup = cleanup
}

// IsRunning returns true if the service is running.
func (s *OptimizationService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetLastMigration returns when the last migration occurred.
func (s *OptimizationService) GetLastMigration() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastMigration
}

// GetLastCleanup returns when the last cleanup occurred.
func (s *OptimizationService) GetLastCleanup() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastCleanup
}

// MigrationResult contains migration results.
type MigrationResult struct {
	StartedAt      time.Time `json:"startedAt"`
	CompletedAt    time.Time `json:"completedAt"`
	HotToWarm      int       `json:"hotToWarm"`
	WarmToCold     int       `json:"warmToCold"`
	TotalMigrated  int       `json:"totalMigrated"`
}

// CleanupResult contains cleanup results.
type CleanupResult struct {
	StartedAt    time.Time `json:"startedAt"`
	CompletedAt  time.Time `json:"completedAt"`
	DeletedCount int       `json:"deletedCount"`
	ExpiredCount int       `json:"expiredCount"`
	ExcessCount  int       `json:"excessCount"`
	Error        string    `json:"error,omitempty"`
}

// OptimizationResult contains overall optimization results.
type OptimizationResult struct {
	StartedAt   time.Time                       `json:"startedAt"`
	CompletedAt time.Time                       `json:"completedAt"`
	Migration   *MigrationResult                `json:"migration,omitempty"`
	Cleanup     *CleanupResult                  `json:"cleanup,omitempty"`
	Stats       *domainMemory.OptimizationStats `json:"stats,omitempty"`
}

// DurationMs returns the optimization duration in milliseconds.
func (r *OptimizationResult) DurationMs() int64 {
	return r.CompletedAt.Sub(r.StartedAt).Milliseconds()
}
