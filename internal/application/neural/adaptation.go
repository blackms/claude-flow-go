// Package neural provides neural learning application services.
package neural

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
)

// AdaptationService manages LoRA adapters for efficient fine-tuning.
type AdaptationService struct {
	mu           sync.RWMutex
	loraEngine   *infraNeural.LoRAEngine
	adapterSets  map[string]*domainNeural.LoRAAdapterSet
	agentAdapters map[string][]string // agentID -> adapter IDs
	config       AdaptationServiceConfig
	stats        *AdaptationServiceStats
}

// AdaptationServiceConfig configures the adaptation service.
type AdaptationServiceConfig struct {
	// DefaultLoRAConfig is the default LoRA configuration.
	DefaultLoRAConfig domainNeural.LoRAConfig

	// MaxAdaptersPerAgent is the maximum adapters per agent.
	MaxAdaptersPerAgent int

	// EnableAutoMerge enables automatic adapter merging.
	EnableAutoMerge bool

	// MergeThreshold triggers merge when adapter count exceeds this.
	MergeThreshold int

	// EnableCheckpointing enables automatic checkpointing.
	EnableCheckpointing bool

	// CheckpointInterval is the interval between checkpoints.
	CheckpointInterval int64
}

// DefaultAdaptationServiceConfig returns the default configuration.
func DefaultAdaptationServiceConfig() AdaptationServiceConfig {
	return AdaptationServiceConfig{
		DefaultLoRAConfig:   domainNeural.DefaultLoRAConfig(),
		MaxAdaptersPerAgent: 10,
		EnableAutoMerge:     true,
		MergeThreshold:      5,
		EnableCheckpointing: true,
		CheckpointInterval:  1000,
	}
}

// AdaptationServiceStats contains service statistics.
type AdaptationServiceStats struct {
	TotalAdapters      int     `json:"totalAdapters"`
	TotalAdapterSets   int     `json:"totalAdapterSets"`
	TotalAdaptations   int64   `json:"totalAdaptations"`
	TotalMerges        int64   `json:"totalMerges"`
	AvgAdaptationTimeMs float64 `json:"avgAdaptationTimeMs"`
	MemoryUsageBytes   int64   `json:"memoryUsageBytes"`
}

// NewAdaptationService creates a new adaptation service.
func NewAdaptationService(config AdaptationServiceConfig) *AdaptationService {
	engineConfig := infraNeural.LoRAEngineConfig{
		DefaultConfig: config.DefaultLoRAConfig,
		MaxAdapters:   1000,
		EnableCaching: true,
		BatchSize:     32,
	}

	return &AdaptationService{
		loraEngine:    infraNeural.NewLoRAEngine(engineConfig),
		adapterSets:   make(map[string]*domainNeural.LoRAAdapterSet),
		agentAdapters: make(map[string][]string),
		config:        config,
		stats:         &AdaptationServiceStats{},
	}
}

// CreateAdapter creates a new LoRA adapter.
func (s *AdaptationService) CreateAdapter(ctx context.Context, name string, config *domainNeural.LoRAConfig) (*domainNeural.LoRAAdapter, error) {
	if config == nil {
		cfg := s.config.DefaultLoRAConfig
		config = &cfg
	}

	adapter, err := s.loraEngine.CreateAdapter(name, *config, domainNeural.InjectionEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	s.mu.Lock()
	s.stats.TotalAdapters++
	s.mu.Unlock()

	return adapter, nil
}

// CreateAdapterForAgent creates an adapter for a specific agent.
func (s *AdaptationService) CreateAdapterForAgent(ctx context.Context, agentID, name string, config *domainNeural.LoRAConfig) (*domainNeural.LoRAAdapter, error) {
	s.mu.Lock()
	adapters := s.agentAdapters[agentID]
	if len(adapters) >= s.config.MaxAdaptersPerAgent {
		s.mu.Unlock()
		return nil, fmt.Errorf("maximum adapters reached for agent: %s", agentID)
	}
	s.mu.Unlock()

	adapter, err := s.CreateAdapter(ctx, fmt.Sprintf("%s-%s", agentID, name), config)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.agentAdapters[agentID] = append(s.agentAdapters[agentID], adapter.ID)
	s.mu.Unlock()

	// Check for auto-merge
	if s.config.EnableAutoMerge && len(s.agentAdapters[agentID]) >= s.config.MergeThreshold {
		go s.autoMergeAdapters(ctx, agentID)
	}

	return adapter, nil
}

// Adapt applies adaptation to an embedding using an agent's adapters.
func (s *AdaptationService) Adapt(ctx context.Context, agentID string, embedding []float64) ([]float64, error) {
	startTime := time.Now()
	defer func() {
		s.mu.Lock()
		s.stats.TotalAdaptations++
		elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
		s.stats.AvgAdaptationTimeMs = (s.stats.AvgAdaptationTimeMs*float64(s.stats.TotalAdaptations-1) + elapsed) / float64(s.stats.TotalAdaptations)
		s.mu.Unlock()
	}()

	s.mu.RLock()
	adapterIDs := s.agentAdapters[agentID]
	s.mu.RUnlock()

	if len(adapterIDs) == 0 {
		// No adapters, return original
		return embedding, nil
	}

	// Apply all adapters
	result := make([]float64, len(embedding))
	copy(result, embedding)

	for _, adapterID := range adapterIDs {
		delta, err := s.loraEngine.Forward(adapterID, embedding)
		if err != nil {
			continue // Skip failed adapters
		}

		// Add delta to result
		for i := range result {
			if i < len(delta) {
				result[i] += delta[i]
			}
		}
	}

	return result, nil
}

// AdaptWithAdapter applies a specific adapter to an embedding.
func (s *AdaptationService) AdaptWithAdapter(ctx context.Context, adapterID string, embedding []float64) ([]float64, error) {
	delta, err := s.loraEngine.Forward(adapterID, embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to apply adapter: %w", err)
	}

	result := make([]float64, len(embedding))
	for i := range result {
		result[i] = embedding[i]
		if i < len(delta) {
			result[i] += delta[i]
		}
	}

	return result, nil
}

// UpdateAdapter updates an adapter with gradients.
func (s *AdaptationService) UpdateAdapter(ctx context.Context, update *domainNeural.LoRAUpdate) error {
	return s.loraEngine.Update(update)
}

// BatchUpdate applies multiple updates efficiently.
func (s *AdaptationService) BatchUpdate(ctx context.Context, updates []*domainNeural.LoRAUpdate) error {
	var lastErr error
	for _, update := range updates {
		if err := s.loraEngine.Update(update); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// MergeAgentAdapters merges all adapters for an agent.
func (s *AdaptationService) MergeAgentAdapters(ctx context.Context, agentID string) (*domainNeural.LoRAAdapter, error) {
	s.mu.RLock()
	adapterIDs := s.agentAdapters[agentID]
	s.mu.RUnlock()

	if len(adapterIDs) == 0 {
		return nil, fmt.Errorf("no adapters for agent: %s", agentID)
	}

	if len(adapterIDs) == 1 {
		return s.loraEngine.GetAdapter(adapterIDs[0])
	}

	// Equal weights for all adapters
	weights := make(map[string]float64)
	for _, id := range adapterIDs {
		weights[id] = 1.0 / float64(len(adapterIDs))
	}

	mergeConfig := domainNeural.LoRAMergeConfig{
		Weights:   weights,
		Strategy:  domainNeural.MergeStrategyAdd,
		Normalize: true,
	}

	merged, err := s.loraEngine.MergeAdapters(adapterIDs, mergeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to merge adapters: %w", err)
	}

	s.mu.Lock()
	// Replace old adapters with merged one
	s.agentAdapters[agentID] = []string{merged.ID}
	s.stats.TotalMerges++
	s.mu.Unlock()

	return merged, nil
}

// autoMergeAdapters automatically merges adapters when threshold is reached.
func (s *AdaptationService) autoMergeAdapters(ctx context.Context, agentID string) {
	_, _ = s.MergeAgentAdapters(ctx, agentID)
}

// CreateAdapterSet creates a new adapter set.
func (s *AdaptationService) CreateAdapterSet(ctx context.Context, name, description string) *domainNeural.LoRAAdapterSet {
	set := &domainNeural.LoRAAdapterSet{
		ID:             uuid.New().String(),
		Name:           name,
		Description:    description,
		Adapters:       make(map[string]*domainNeural.LoRAAdapter),
		ActiveAdapters: make([]string, 0),
		CreatedAt:      time.Now(),
	}

	s.mu.Lock()
	s.adapterSets[set.ID] = set
	s.stats.TotalAdapterSets++
	s.mu.Unlock()

	return set
}

// AddToAdapterSet adds an adapter to a set.
func (s *AdaptationService) AddToAdapterSet(ctx context.Context, setID, adapterID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	set, exists := s.adapterSets[setID]
	if !exists {
		return fmt.Errorf("adapter set not found: %s", setID)
	}

	adapter, err := s.loraEngine.GetAdapter(adapterID)
	if err != nil {
		return err
	}

	set.Adapters[adapterID] = adapter
	return nil
}

// ActivateAdapterInSet activates an adapter in a set.
func (s *AdaptationService) ActivateAdapterInSet(ctx context.Context, setID, adapterID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	set, exists := s.adapterSets[setID]
	if !exists {
		return fmt.Errorf("adapter set not found: %s", setID)
	}

	if _, exists := set.Adapters[adapterID]; !exists {
		return fmt.Errorf("adapter not in set: %s", adapterID)
	}

	// Check if already active
	for _, id := range set.ActiveAdapters {
		if id == adapterID {
			return nil
		}
	}

	set.ActiveAdapters = append(set.ActiveAdapters, adapterID)
	return nil
}

// GetAdapter returns an adapter by ID.
func (s *AdaptationService) GetAdapter(ctx context.Context, adapterID string) (*domainNeural.LoRAAdapter, error) {
	return s.loraEngine.GetAdapter(adapterID)
}

// GetAgentAdapters returns all adapters for an agent.
func (s *AdaptationService) GetAgentAdapters(ctx context.Context, agentID string) ([]*domainNeural.LoRAAdapter, error) {
	s.mu.RLock()
	adapterIDs := s.agentAdapters[agentID]
	s.mu.RUnlock()

	adapters := make([]*domainNeural.LoRAAdapter, 0, len(adapterIDs))
	for _, id := range adapterIDs {
		adapter, err := s.loraEngine.GetAdapter(id)
		if err == nil {
			adapters = append(adapters, adapter)
		}
	}

	return adapters, nil
}

// DeleteAdapter deletes an adapter.
func (s *AdaptationService) DeleteAdapter(ctx context.Context, adapterID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from agent mappings
	for agentID, adapterIDs := range s.agentAdapters {
		newIDs := make([]string, 0)
		for _, id := range adapterIDs {
			if id != adapterID {
				newIDs = append(newIDs, id)
			}
		}
		s.agentAdapters[agentID] = newIDs
	}

	// Remove from adapter sets
	for _, set := range s.adapterSets {
		delete(set.Adapters, adapterID)
		newActive := make([]string, 0)
		for _, id := range set.ActiveAdapters {
			if id != adapterID {
				newActive = append(newActive, id)
			}
		}
		set.ActiveAdapters = newActive
	}

	return s.loraEngine.DeleteAdapter(adapterID)
}

// FreezeAdapter freezes an adapter.
func (s *AdaptationService) FreezeAdapter(ctx context.Context, adapterID string) error {
	return s.loraEngine.FreezeAdapter(adapterID)
}

// UnfreezeAdapter unfreezes an adapter.
func (s *AdaptationService) UnfreezeAdapter(ctx context.Context, adapterID string) error {
	return s.loraEngine.UnfreezeAdapter(adapterID)
}

// CreateCheckpoint creates a checkpoint for an adapter.
func (s *AdaptationService) CreateCheckpoint(ctx context.Context, adapterID string) (*domainNeural.LoRACheckpoint, error) {
	return s.loraEngine.CreateCheckpoint(adapterID)
}

// RestoreCheckpoint restores an adapter from a checkpoint.
func (s *AdaptationService) RestoreCheckpoint(ctx context.Context, checkpoint *domainNeural.LoRACheckpoint) error {
	return s.loraEngine.RestoreCheckpoint(checkpoint)
}

// GetStats returns service statistics.
func (s *AdaptationService) GetStats() *AdaptationServiceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	engineStats := s.loraEngine.GetStats()
	s.stats.TotalAdapters = engineStats.TotalAdapters

	// Calculate memory usage
	var memoryUsage int64
	for _, adapterIDs := range s.agentAdapters {
		for _, id := range adapterIDs {
			if stats, err := s.loraEngine.GetAdapterStats(id); err == nil {
				memoryUsage += stats.MemoryBytes
			}
		}
	}
	s.stats.MemoryUsageBytes = memoryUsage

	return s.stats
}

// GetAdapterStats returns statistics for a specific adapter.
func (s *AdaptationService) GetAdapterStats(ctx context.Context, adapterID string) (*domainNeural.LoRAStats, error) {
	return s.loraEngine.GetAdapterStats(adapterID)
}

// ComputeDeltaWeight computes the full delta weight matrix for an adapter.
func (s *AdaptationService) ComputeDeltaWeight(ctx context.Context, adapterID string) ([][]float64, error) {
	return s.loraEngine.ComputeDeltaWeight(adapterID)
}
