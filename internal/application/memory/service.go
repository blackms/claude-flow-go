// Package memory provides application services for memory management.
package memory

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	domainMemory "github.com/anthropics/claude-flow-go/internal/domain/memory"
	infraMemory "github.com/anthropics/claude-flow-go/internal/infrastructure/memory"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// MemoryService provides high-level memory operations.
type MemoryService struct {
	mu            sync.RWMutex
	backend       shared.MemoryBackend
	tieredStorage *infraMemory.TieredStorage
	driftDetector *infraMemory.DriftDetector
	optimizer     *infraMemory.QueryOptimizer
	lazyLoader    *infraMemory.LazyLoader
	swarmSync     *infraMemory.SwarmSynchronizer
	config        MemoryServiceConfig

	// Statistics
	storeCount   int64
	retrieveCount int64
	searchCount   int64
	totalLatencyMs float64
}

// MemoryServiceConfig configures the memory service.
type MemoryServiceConfig struct {
	// EnableTieredStorage enables tiered storage.
	EnableTieredStorage bool `json:"enableTieredStorage"`

	// EnableDriftDetection enables drift detection.
	EnableDriftDetection bool `json:"enableDriftDetection"`

	// EnableQueryOptimization enables query optimization.
	EnableQueryOptimization bool `json:"enableQueryOptimization"`

	// EnableLazyLoading enables lazy loading.
	EnableLazyLoading bool `json:"enableLazyLoading"`

	// EnableSwarmSync enables swarm synchronization.
	EnableSwarmSync bool `json:"enableSwarmSync"`

	// TieredStorageConfig configures tiered storage.
	TieredStorageConfig *infraMemory.TieredStorageConfig `json:"tieredStorageConfig,omitempty"`

	// DriftConfig configures drift detection.
	DriftConfig *domainMemory.DriftConfig `json:"driftConfig,omitempty"`

	// OptimizerConfig configures query optimization.
	OptimizerConfig *infraMemory.QueryOptimizerConfig `json:"optimizerConfig,omitempty"`

	// LazyLoaderConfig configures lazy loading.
	LazyLoaderConfig *infraMemory.LazyLoaderConfig `json:"lazyLoaderConfig,omitempty"`

	// SwarmSyncConfig configures swarm synchronization.
	SwarmSyncConfig *infraMemory.SwarmSyncConfig `json:"swarmSyncConfig,omitempty"`
}

// DefaultMemoryServiceConfig returns the default configuration.
func DefaultMemoryServiceConfig() MemoryServiceConfig {
	return MemoryServiceConfig{
		EnableTieredStorage:     true,
		EnableDriftDetection:    true,
		EnableQueryOptimization: true,
		EnableLazyLoading:       false, // Disabled by default
		EnableSwarmSync:         false, // Disabled by default
	}
}

// NewMemoryService creates a new memory service.
func NewMemoryService(backend shared.MemoryBackend, config MemoryServiceConfig) *MemoryService {
	svc := &MemoryService{
		backend: backend,
		config:  config,
	}

	// Initialize tiered storage
	if config.EnableTieredStorage {
		storageConfig := infraMemory.DefaultTieredStorageConfig()
		if config.TieredStorageConfig != nil {
			storageConfig = *config.TieredStorageConfig
		}
		svc.tieredStorage = infraMemory.NewTieredStorage(storageConfig)
	}

	// Initialize drift detector
	if config.EnableDriftDetection {
		driftConfig := domainMemory.DefaultDriftConfig()
		if config.DriftConfig != nil {
			driftConfig = *config.DriftConfig
		}
		svc.driftDetector = infraMemory.NewDriftDetector(driftConfig)
	}

	// Initialize query optimizer
	if config.EnableQueryOptimization {
		optConfig := infraMemory.DefaultQueryOptimizerConfig()
		if config.OptimizerConfig != nil {
			optConfig = *config.OptimizerConfig
		}
		svc.optimizer = infraMemory.NewQueryOptimizer(optConfig)
	}

	// Initialize lazy loader
	if config.EnableLazyLoading {
		lazyConfig := infraMemory.DefaultLazyLoaderConfig()
		if config.LazyLoaderConfig != nil {
			lazyConfig = *config.LazyLoaderConfig
		}
		svc.lazyLoader = infraMemory.NewLazyLoader(backend, lazyConfig)
	}

	// Initialize swarm sync
	if config.EnableSwarmSync {
		syncConfig := infraMemory.DefaultSwarmSyncConfig()
		if config.SwarmSyncConfig != nil {
			syncConfig = *config.SwarmSyncConfig
		}
		svc.swarmSync = infraMemory.NewSwarmSynchronizer(syncConfig)
	}

	return svc
}

// Store stores a memory.
func (s *MemoryService) Store(ctx context.Context, memory *shared.Memory) error {
	startTime := time.Now()
	defer func() {
		atomic.AddInt64(&s.storeCount, 1)
		s.updateLatency(time.Since(startTime))
	}()

	// Store in backend
	if err := s.backend.Store(*memory); err != nil {
		return err
	}

	// Store in tiered storage
	if s.tieredStorage != nil {
		if err := s.tieredStorage.Store(memory); err != nil {
			return err
		}
	}

	// Record for swarm sync
	if s.swarmSync != nil {
		s.swarmSync.RecordOperation(infraMemory.SyncOpCreate, memory)
	}

	// Register for lazy loading
	if s.lazyLoader != nil {
		s.lazyLoader.RegisterMemory(memory)
	}

	return nil
}

// Retrieve retrieves a memory by ID.
func (s *MemoryService) Retrieve(ctx context.Context, id string) (*shared.Memory, error) {
	startTime := time.Now()
	defer func() {
		atomic.AddInt64(&s.retrieveCount, 1)
		s.updateLatency(time.Since(startTime))
	}()

	// Try tiered storage first (fastest)
	if s.tieredStorage != nil {
		memory, _, err := s.tieredStorage.Get(id)
		if err == nil && memory != nil {
			return memory, nil
		}
	}

	// Try lazy loader
	if s.lazyLoader != nil {
		memory, err := s.lazyLoader.Get(ctx, id)
		if err == nil && memory != nil {
			return memory, nil
		}
	}

	// Fall back to backend
	memory, err := s.backend.Retrieve(id)
	if err != nil {
		return nil, err
	}

	return &memory, nil
}

// Delete deletes a memory.
func (s *MemoryService) Delete(ctx context.Context, id string) error {
	// Delete from backend
	if err := s.backend.Delete(id); err != nil {
		return err
	}

	// Delete from tiered storage
	if s.tieredStorage != nil {
		s.tieredStorage.Delete(id)
	}

	// Record for swarm sync
	if s.swarmSync != nil {
		s.swarmSync.RecordOperation(infraMemory.SyncOpDelete, &shared.Memory{ID: id})
	}

	return nil
}

// Query queries memories using standard query.
func (s *MemoryService) Query(ctx context.Context, query shared.MemoryQuery) ([]shared.Memory, error) {
	return s.backend.Query(query)
}

// SemanticSearch performs semantic search.
func (s *MemoryService) SemanticSearch(ctx context.Context, query *domainMemory.SemanticQuery) (*domainMemory.SearchResults, error) {
	startTime := time.Now()
	defer func() {
		atomic.AddInt64(&s.searchCount, 1)
	}()

	// Optimize query
	if s.optimizer != nil {
		s.optimizer.OptimizeSemanticQuery(query)
	}

	// Perform vector search
	results, err := s.backend.VectorSearch(query.Embedding, query.TopK)
	if err != nil {
		return nil, err
	}

	// Filter by type if specified
	if len(query.MemoryTypes) > 0 {
		typeSet := make(map[string]bool)
		for _, t := range query.MemoryTypes {
			typeSet[t] = true
		}
		filtered := make([]shared.SearchResult, 0)
		for _, r := range results {
			if typeSet[string(r.Memory.Type)] {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Filter by agent if specified
	if query.AgentID != "" {
		filtered := make([]shared.SearchResult, 0)
		for _, r := range results {
			if r.Memory.AgentID == query.AgentID {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Filter by min score
	if query.MinScore > 0 {
		filtered := make([]shared.SearchResult, 0)
		for _, r := range results {
			if r.Score >= query.MinScore {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Convert to domain results
	searchResults := &domainMemory.SearchResults{
		Results:     make([]*domainMemory.SearchResult, len(results)),
		TotalCount:  len(results),
		QueryTimeMs: float64(time.Since(startTime).Microseconds()) / 1000.0,
	}

	for i, r := range results {
		memory := domainMemory.FromShared(r.Memory)
		searchResults.Results[i] = &domainMemory.SearchResult{
			Memory:        memory,
			Score:         r.Score,
			SemanticScore: r.Score,
		}
	}

	return searchResults, nil
}

// TemporalSearch performs temporal search.
func (s *MemoryService) TemporalSearch(ctx context.Context, query *domainMemory.TemporalQuery) (*domainMemory.SearchResults, error) {
	startTime := time.Now()

	// Build memory query
	memQuery := shared.MemoryQuery{}
	if query.StartTime != nil || query.EndTime != nil {
		memQuery.TimeRange = &shared.TimeRange{}
		if query.StartTime != nil {
			memQuery.TimeRange.Start = query.StartTime.UnixMilli()
		}
		if query.EndTime != nil {
			memQuery.TimeRange.End = query.EndTime.UnixMilli()
		}
	}
	memQuery.Limit = query.Limit

	// Query backend
	memories, err := s.backend.Query(memQuery)
	if err != nil {
		return nil, err
	}

	// Apply temporal scoring
	now := time.Now()
	if query.ReferenceTime != nil {
		now = *query.ReferenceTime
	}

	results := make([]*domainMemory.SearchResult, len(memories))
	for i, m := range memories {
		memory := domainMemory.FromShared(m)
		age := now.Sub(time.UnixMilli(m.Timestamp)).Seconds()
		score := s.calculateTemporalScore(age, query)

		results[i] = &domainMemory.SearchResult{
			Memory:        memory,
			Score:         score,
			TemporalScore: score,
		}
	}

	// Sort by score descending
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Apply limit
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return &domainMemory.SearchResults{
		Results:     results,
		TotalCount:  len(memories),
		QueryTimeMs: float64(time.Since(startTime).Microseconds()) / 1000.0,
	}, nil
}

// calculateTemporalScore calculates temporal relevance score.
func (s *MemoryService) calculateTemporalScore(ageSeconds float64, query *domainMemory.TemporalQuery) float64 {
	switch query.DecayFunction {
	case domainMemory.DecayLinear:
		maxAge := 86400.0 * 7 // 7 days
		return math.Max(0, 1-ageSeconds/maxAge)
	case domainMemory.DecayExponential:
		rate := query.DecayRate
		if rate <= 0 {
			rate = 0.1
		}
		return math.Exp(-rate * ageSeconds / 3600) // Decay per hour
	case domainMemory.DecayGaussian:
		sigma := 86400.0 // 1 day
		return math.Exp(-(ageSeconds * ageSeconds) / (2 * sigma * sigma))
	case domainMemory.DecayNone:
		return 1.0
	default:
		return 1.0
	}
}

// MultiModalSearch performs multi-modal search.
func (s *MemoryService) MultiModalSearch(ctx context.Context, query *domainMemory.MultiModalQuery) (*domainMemory.SearchResults, error) {
	startTime := time.Now()

	var semanticResults *domainMemory.SearchResults
	var temporalResults *domainMemory.SearchResults
	var err error

	// Execute semantic search
	if query.Semantic != nil {
		semanticResults, err = s.SemanticSearch(ctx, query.Semantic)
		if err != nil {
			return nil, err
		}
	}

	// Execute temporal search
	if query.Temporal != nil {
		temporalResults, err = s.TemporalSearch(ctx, query.Temporal)
		if err != nil {
			return nil, err
		}
	}

	// Combine results
	combined := s.combineResults(semanticResults, temporalResults, query)

	combined.QueryTimeMs = float64(time.Since(startTime).Microseconds()) / 1000.0
	return combined, nil
}

// combineResults combines multi-modal search results.
func (s *MemoryService) combineResults(semantic, temporal *domainMemory.SearchResults, query *domainMemory.MultiModalQuery) *domainMemory.SearchResults {
	resultMap := make(map[string]*domainMemory.SearchResult)

	weights := query.Weights
	if weights.Semantic == 0 && weights.Temporal == 0 {
		weights = domainMemory.DefaultQueryWeights()
	}

	// Add semantic results
	if semantic != nil {
		for _, r := range semantic.Results {
			resultMap[r.Memory.ID] = &domainMemory.SearchResult{
				Memory:        r.Memory,
				SemanticScore: r.Score,
				Score:         r.Score * weights.Semantic,
			}
		}
	}

	// Add/merge temporal results
	if temporal != nil {
		for _, r := range temporal.Results {
			if existing, ok := resultMap[r.Memory.ID]; ok {
				existing.TemporalScore = r.Score
				existing.Score += r.Score * weights.Temporal
			} else {
				resultMap[r.Memory.ID] = &domainMemory.SearchResult{
					Memory:        r.Memory,
					TemporalScore: r.Score,
					Score:         r.Score * weights.Temporal,
				}
			}
		}
	}

	// Convert to slice and sort
	results := make([]*domainMemory.SearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, r)
	}

	// Sort by combined score
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Apply limit
	if query.TopK > 0 && len(results) > query.TopK {
		results = results[:query.TopK]
	}

	return &domainMemory.SearchResults{
		Results:    results,
		TotalCount: len(resultMap),
	}
}

// GetStats returns service statistics.
func (s *MemoryService) GetStats() *MemoryServiceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &MemoryServiceStats{
		StoreCount:     atomic.LoadInt64(&s.storeCount),
		RetrieveCount:  atomic.LoadInt64(&s.retrieveCount),
		SearchCount:    atomic.LoadInt64(&s.searchCount),
		TotalLatencyMs: s.totalLatencyMs,
	}

	totalOps := stats.StoreCount + stats.RetrieveCount + stats.SearchCount
	if totalOps > 0 {
		stats.AvgLatencyMs = s.totalLatencyMs / float64(totalOps)
	}

	if s.tieredStorage != nil {
		stats.TieredStats = s.tieredStorage.GetStats()
	}

	if s.optimizer != nil {
		stats.OptimizerStats = s.optimizer.GetStats()
	}

	if s.lazyLoader != nil {
		stats.LazyLoaderStats = s.lazyLoader.GetStats()
	}

	if s.swarmSync != nil {
		stats.SwarmSyncStats = s.swarmSync.GetStats()
	}

	return stats
}

// updateLatency updates latency statistics.
func (s *MemoryService) updateLatency(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalLatencyMs += float64(duration.Microseconds()) / 1000.0
}

// GetDriftDetector returns the drift detector.
func (s *MemoryService) GetDriftDetector() *infraMemory.DriftDetector {
	return s.driftDetector
}

// GetTieredStorage returns the tiered storage.
func (s *MemoryService) GetTieredStorage() *infraMemory.TieredStorage {
	return s.tieredStorage
}

// GetSwarmSync returns the swarm synchronizer.
func (s *MemoryService) GetSwarmSync() *infraMemory.SwarmSynchronizer {
	return s.swarmSync
}

// MemoryServiceStats contains service statistics.
type MemoryServiceStats struct {
	StoreCount      int64                            `json:"storeCount"`
	RetrieveCount   int64                            `json:"retrieveCount"`
	SearchCount     int64                            `json:"searchCount"`
	TotalLatencyMs  float64                          `json:"totalLatencyMs"`
	AvgLatencyMs    float64                          `json:"avgLatencyMs"`
	TieredStats     *domainMemory.OptimizationStats  `json:"tieredStats,omitempty"`
	OptimizerStats  *infraMemory.QueryOptimizerStats `json:"optimizerStats,omitempty"`
	LazyLoaderStats *infraMemory.LazyLoaderStats     `json:"lazyLoaderStats,omitempty"`
	SwarmSyncStats  *infraMemory.SwarmSyncStats      `json:"swarmSyncStats,omitempty"`
}
