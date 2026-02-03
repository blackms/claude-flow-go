// Package neural provides neural learning application services.
package neural

import (
	"context"
	"fmt"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
)

// SONAManager orchestrates the Self-Optimizing Neural Architecture.
type SONAManager struct {
	mu sync.RWMutex

	// Current mode and implementation
	currentMode domainNeural.SONAMode
	config      domainNeural.ModeConfig
	modeImpl    infraNeural.ModeImplementation

	// Trajectory buffer
	trajectories []*domainNeural.ExtendedTrajectory

	// Pattern storage
	patterns map[string]*domainNeural.ReasoningPattern

	// Statistics
	startTime             time.Time
	lastModeChange        *time.Time
	lastLearning          *time.Time
	modeHistory           []domainNeural.ModeChangeEvent
	totalAdaptations      int64
	totalLearningCycles   int64
	avgAdaptLatency       float64
	avgLearnLatency       float64
	patternCacheHits      int64
	patternCacheMisses    int64
}

// NewSONAManager creates a new SONA manager with the default balanced mode.
func NewSONAManager() *SONAManager {
	return NewSONAManagerWithMode(domainNeural.ModeBalanced)
}

// NewSONAManagerWithMode creates a new SONA manager with the specified mode.
func NewSONAManagerWithMode(mode domainNeural.SONAMode) *SONAManager {
	config := domainNeural.DefaultModeConfig(mode)

	return &SONAManager{
		currentMode:  mode,
		config:       config,
		modeImpl:     infraNeural.CreateModeImplementationWithConfig(config),
		trajectories: make([]*domainNeural.ExtendedTrajectory, 0),
		patterns:     make(map[string]*domainNeural.ReasoningPattern),
		startTime:    time.Now(),
		modeHistory:  make([]domainNeural.ModeChangeEvent, 0),
	}
}

// GetMode returns the current SONA mode.
func (m *SONAManager) GetMode() domainNeural.SONAMode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentMode
}

// GetConfig returns the current mode configuration.
func (m *SONAManager) GetConfig() domainNeural.ModeConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetMode switches to a new SONA mode.
func (m *SONAManager) SetMode(ctx context.Context, mode domainNeural.SONAMode) error {
	return m.SetModeWithReason(ctx, mode, "manual mode switch")
}

// SetModeWithReason switches to a new SONA mode with a reason.
func (m *SONAManager) SetModeWithReason(ctx context.Context, mode domainNeural.SONAMode, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mode == m.currentMode {
		return nil
	}

	previousMode := m.currentMode

	// Cleanup current mode
	if m.modeImpl != nil {
		if err := m.modeImpl.Cleanup(); err != nil {
			return fmt.Errorf("failed to cleanup current mode: %w", err)
		}
	}

	// Update configuration
	m.currentMode = mode
	m.config = domainNeural.DefaultModeConfig(mode)

	// Create new mode implementation
	m.modeImpl = infraNeural.CreateModeImplementationWithConfig(m.config)

	// Record mode change
	now := time.Now()
	m.lastModeChange = &now
	m.modeHistory = append(m.modeHistory, domainNeural.ModeChangeEvent{
		FromMode:  previousMode,
		ToMode:    mode,
		Timestamp: now,
		Reason:    reason,
	})

	// Keep only last 100 mode changes
	if len(m.modeHistory) > 100 {
		m.modeHistory = m.modeHistory[len(m.modeHistory)-100:]
	}

	return nil
}

// Adapt applies adaptation to an embedding.
func (m *SONAManager) Adapt(ctx context.Context, embedding []float64) (*domainNeural.AdaptationResult, error) {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.modeImpl == nil {
		return nil, fmt.Errorf("mode implementation not initialized")
	}

	adapted, err := m.modeImpl.Adapt(embedding)
	if err != nil {
		return nil, fmt.Errorf("adaptation failed: %w", err)
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0

	// Update statistics
	m.totalAdaptations++
	m.avgAdaptLatency = (m.avgAdaptLatency*float64(m.totalAdaptations-1) + elapsed) / float64(m.totalAdaptations)

	// Get cache stats from mode
	modeStats := m.modeImpl.GetStats()

	return &domainNeural.AdaptationResult{
		Embedding: adapted,
		LatencyMs: elapsed,
		Mode:      m.currentMode,
		CacheHit:  modeStats.CacheHits > m.patternCacheHits,
	}, nil
}

// RecordTrajectory records a trajectory for learning.
func (m *SONAManager) RecordTrajectory(ctx context.Context, trajectory *domainNeural.ExtendedTrajectory) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if trajectory == nil {
		return fmt.Errorf("trajectory is nil")
	}

	// Check capacity
	if len(m.trajectories) >= m.config.TrajectoryCapacity {
		// Remove oldest trajectory
		m.trajectories = m.trajectories[1:]
	}

	m.trajectories = append(m.trajectories, trajectory)

	return nil
}

// TriggerLearning triggers a learning cycle.
func (m *SONAManager) TriggerLearning(ctx context.Context) (*domainNeural.LearningTriggerResult, error) {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	result := &domainNeural.LearningTriggerResult{
		Timestamp: time.Now(),
	}

	if m.modeImpl == nil {
		return result, fmt.Errorf("mode implementation not initialized")
	}

	if len(m.trajectories) == 0 {
		result.Triggered = false
		return result, nil
	}

	result.Triggered = true

	// Process trajectories
	var totalLoss float64
	processedCount := 0

	for _, trajectory := range m.trajectories {
		if trajectory.QualityScore >= m.config.QualityThreshold {
			if err := m.modeImpl.Learn(trajectory); err != nil {
				continue
			}
			processedCount++
			totalLoss += 1.0 - trajectory.QualityScore // Simple loss approximation
		}
	}

	result.TrajectoriesProcessed = processedCount
	if processedCount > 0 {
		result.AvgLoss = totalLoss / float64(processedCount)
	}

	// Clear processed trajectories
	m.trajectories = make([]*domainNeural.ExtendedTrajectory, 0)

	elapsed := time.Since(startTime).Milliseconds()
	result.DurationMs = elapsed

	// Update statistics
	now := time.Now()
	m.lastLearning = &now
	m.totalLearningCycles++
	m.avgLearnLatency = (m.avgLearnLatency*float64(m.totalLearningCycles-1) + float64(elapsed)) / float64(m.totalLearningCycles)

	return result, nil
}

// OptimizePatterns runs pattern optimization.
func (m *SONAManager) OptimizePatterns(ctx context.Context) (*domainNeural.OptimizationResult, error) {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	result := &domainNeural.OptimizationResult{
		Timestamp: time.Now(),
	}

	// Prune low-usage patterns
	prunedCount := 0
	for id, pattern := range m.patterns {
		if pattern.UsageCount < 2 && time.Since(pattern.UpdatedAt) > 24*time.Hour {
			delete(m.patterns, id)
			prunedCount++
		}
	}
	result.PatternsPruned = prunedCount

	// Merge similar patterns (simplified)
	mergedCount := 0
	patternIDs := make([]string, 0, len(m.patterns))
	for id := range m.patterns {
		patternIDs = append(patternIDs, id)
	}

	for i := 0; i < len(patternIDs); i++ {
		for j := i + 1; j < len(patternIDs); j++ {
			p1, ok1 := m.patterns[patternIDs[i]]
			p2, ok2 := m.patterns[patternIDs[j]]
			if !ok1 || !ok2 {
				continue
			}

			// Check if patterns are similar (simplified)
			if p1.Domain == p2.Domain && p1.SuccessRate > 0.7 && p2.SuccessRate > 0.7 {
				sim := m.computePatternSimilarity(p1, p2)
				if sim > 0.9 {
					// Merge p2 into p1
					p1.UsageCount += p2.UsageCount
					p1.QualityHistory = append(p1.QualityHistory, p2.QualityHistory...)
					delete(m.patterns, patternIDs[j])
					mergedCount++
				}
			}
		}
	}
	result.PatternsMerged = mergedCount

	// Estimate memory freed
	result.MemoryFreedMB = float64(prunedCount+mergedCount) * 0.01 // Rough estimate

	result.PatternsOptimized = len(m.patterns)
	result.DurationMs = time.Since(startTime).Milliseconds()

	return result, nil
}

// GetStats returns comprehensive SONA statistics.
func (m *SONAManager) GetStats() domainNeural.SONAStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	modeStats := m.modeImpl.GetStats()

	// Compute trajectory utilization
	utilization := 0.0
	if m.config.TrajectoryCapacity > 0 {
		utilization = float64(len(m.trajectories)) / float64(m.config.TrajectoryCapacity)
	}

	// Compute cache hit rate
	cacheHitRate := 0.0
	totalCacheOps := modeStats.CacheHits + modeStats.CacheMisses
	if totalCacheOps > 0 {
		cacheHitRate = float64(modeStats.CacheHits) / float64(totalCacheOps)
	}

	// Compute ops per second
	uptime := time.Since(m.startTime).Seconds()
	opsPerSecond := 0.0
	if uptime > 0 {
		opsPerSecond = float64(m.totalAdaptations) / uptime
	}

	return domainNeural.SONAStats{
		Mode:   m.currentMode,
		Config: m.config,
		Trajectories: domainNeural.TrajectoryStats{
			Total:       len(m.trajectories),
			Active:      len(m.trajectories),
			Completed:   0, // Would need separate tracking
			Utilization: utilization,
		},
		Performance: domainNeural.PerformanceStats{
			AvgQualityScore:  m.computeAvgQuality(),
			OpsPerSecond:     opsPerSecond,
			LearningCycles:   m.totalLearningCycles,
			AvgLatencyMs:     m.avgAdaptLatency,
			TotalAdaptations: m.totalAdaptations,
		},
		Patterns: domainNeural.PatternStats{
			TotalPatterns:  len(m.patterns),
			AvgMatchTimeMs: modeStats.AvgAdaptLatencyMs,
			CacheHitRate:   cacheHitRate,
			EvolutionCount: 0, // Would need separate tracking
		},
		Memory: domainNeural.MemoryStats{
			UsedMB:          float64(modeStats.MemoryUsedBytes) / (1024 * 1024),
			BudgetMB:        float64(m.config.MemoryBudgetMB),
			TrajectoryBytes: int64(len(m.trajectories) * 1024), // Rough estimate
			PatternBytes:    int64(len(m.patterns) * 512),      // Rough estimate
		},
		StartTime:      m.startTime,
		LastModeChange: m.lastModeChange,
		LastLearning:   m.lastLearning,
		ModeHistory:    m.modeHistory,
	}
}

// GetTrajectoryCount returns the current trajectory count.
func (m *SONAManager) GetTrajectoryCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.trajectories)
}

// GetPatternCount returns the current pattern count.
func (m *SONAManager) GetPatternCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.patterns)
}

// AddPattern adds a pattern to the manager.
func (m *SONAManager) AddPattern(pattern *domainNeural.ReasoningPattern) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pattern == nil {
		return fmt.Errorf("pattern is nil")
	}

	m.patterns[pattern.PatternID] = pattern
	return nil
}

// GetPattern retrieves a pattern by ID.
func (m *SONAManager) GetPattern(patternID string) (*domainNeural.ReasoningPattern, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pattern, ok := m.patterns[patternID]
	return pattern, ok
}

// SuggestMode suggests an optimal mode based on context.
func (m *SONAManager) SuggestMode(ctx context.Context, constraints ModeConstraints) domainNeural.SONAMode {
	// Real-time: strict latency requirements
	if constraints.MaxLatencyMs > 0 && constraints.MaxLatencyMs <= 0.5 {
		return domainNeural.ModeRealTime
	}

	// Edge: memory constrained
	if constraints.MaxMemoryMB > 0 && constraints.MaxMemoryMB <= 10 {
		return domainNeural.ModeEdge
	}

	// Research: quality priority
	if constraints.MinQuality > 0 && constraints.MinQuality >= 0.9 {
		return domainNeural.ModeResearch
	}

	// Batch: high throughput
	if constraints.BatchProcessing {
		return domainNeural.ModeBatch
	}

	// Default to balanced
	return domainNeural.ModeBalanced
}

// ModeConstraints represents constraints for mode selection.
type ModeConstraints struct {
	// MaxLatencyMs is the maximum allowed latency.
	MaxLatencyMs float64

	// MaxMemoryMB is the maximum allowed memory.
	MaxMemoryMB int

	// MinQuality is the minimum required quality.
	MinQuality float64

	// BatchProcessing indicates batch processing mode.
	BatchProcessing bool
}

// Cleanup cleans up the SONA manager.
func (m *SONAManager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.modeImpl != nil {
		if err := m.modeImpl.Cleanup(); err != nil {
			return err
		}
	}

	m.trajectories = make([]*domainNeural.ExtendedTrajectory, 0)
	m.patterns = make(map[string]*domainNeural.ReasoningPattern)

	return nil
}

// Private methods

func (m *SONAManager) computeAvgQuality() float64 {
	if len(m.trajectories) == 0 {
		return 0
	}

	var total float64
	for _, t := range m.trajectories {
		total += t.QualityScore
	}

	return total / float64(len(m.trajectories))
}

func (m *SONAManager) computePatternSimilarity(p1, p2 *domainNeural.ReasoningPattern) float64 {
	if len(p1.Embedding) == 0 || len(p2.Embedding) == 0 {
		return 0
	}

	// Cosine similarity
	var dot, normA, normB float64
	minLen := len(p1.Embedding)
	if len(p2.Embedding) < minLen {
		minLen = len(p2.Embedding)
	}

	for i := 0; i < minLen; i++ {
		dot += float64(p1.Embedding[i]) * float64(p2.Embedding[i])
		normA += float64(p1.Embedding[i]) * float64(p1.Embedding[i])
		normB += float64(p2.Embedding[i]) * float64(p2.Embedding[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (normA * normB)
}
