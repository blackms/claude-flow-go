// Package neural provides neural network infrastructure.
package neural

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// LoRAEngine implements LoRA matrix operations.
type LoRAEngine struct {
	mu       sync.RWMutex
	adapters map[string]*domainNeural.LoRAAdapter
	config   LoRAEngineConfig
	stats    *LoRAEngineStats
	rng      *rand.Rand
}

// LoRAEngineConfig configures the LoRA engine.
type LoRAEngineConfig struct {
	// DefaultConfig is the default adapter configuration.
	DefaultConfig domainNeural.LoRAConfig

	// MaxAdapters is the maximum number of adapters.
	MaxAdapters int

	// EnableCaching caches computed delta weights.
	EnableCaching bool

	// BatchSize for batched operations.
	BatchSize int
}

// DefaultLoRAEngineConfig returns the default engine configuration.
func DefaultLoRAEngineConfig() LoRAEngineConfig {
	return LoRAEngineConfig{
		DefaultConfig: domainNeural.DefaultLoRAConfig(),
		MaxAdapters:   100,
		EnableCaching: true,
		BatchSize:     32,
	}
}

// LoRAEngineStats contains engine statistics.
type LoRAEngineStats struct {
	TotalAdapters   int     `json:"totalAdapters"`
	TotalUpdates    int64   `json:"totalUpdates"`
	TotalForwards   int64   `json:"totalForwards"`
	AvgUpdateTimeMs float64 `json:"avgUpdateTimeMs"`
	AvgForwardTimeMs float64 `json:"avgForwardTimeMs"`
}

// NewLoRAEngine creates a new LoRA engine.
func NewLoRAEngine(config LoRAEngineConfig) *LoRAEngine {
	return &LoRAEngine{
		adapters: make(map[string]*domainNeural.LoRAAdapter),
		config:   config,
		stats:    &LoRAEngineStats{},
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// CreateAdapter creates a new LoRA adapter.
func (e *LoRAEngine) CreateAdapter(name string, config domainNeural.LoRAConfig, injectionPoint domainNeural.AdapterInjectionPoint) (*domainNeural.LoRAAdapter, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.adapters) >= e.config.MaxAdapters {
		return nil, fmt.Errorf("maximum adapters reached: %d", e.config.MaxAdapters)
	}

	rank := int(config.Rank)
	inputDim := config.InputDimension
	outputDim := config.OutputDimension

	// Initialize matrix A with small random values
	// A: (rank x inputDim)
	A := make([][]float64, rank)
	for i := 0; i < rank; i++ {
		A[i] = make([]float64, inputDim)
		for j := 0; j < inputDim; j++ {
			A[i][j] = e.rng.NormFloat64() * config.InitScale
		}
	}

	// Initialize matrix B to zeros for stable start
	// B: (outputDim x rank)
	B := make([][]float64, outputDim)
	for i := 0; i < outputDim; i++ {
		B[i] = make([]float64, rank)
		// All zeros
	}

	adapter := &domainNeural.LoRAAdapter{
		ID:             uuid.New().String(),
		Name:           name,
		Config:         config,
		A:              A,
		B:              B,
		InjectionPoint: injectionPoint,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Initialize biases if enabled
	if config.EnableBias {
		adapter.BiasA = make([]float64, rank)
		adapter.BiasB = make([]float64, outputDim)
	}

	e.adapters[adapter.ID] = adapter
	e.stats.TotalAdapters++

	return adapter, nil
}

// Forward computes the LoRA forward pass.
// output = W_0(x) + (alpha/r) * B @ A @ x
// This returns the delta: (alpha/r) * B @ A @ x
func (e *LoRAEngine) Forward(adapterID string, input []float64) ([]float64, error) {
	startTime := time.Now()
	defer func() {
		e.mu.Lock()
		e.stats.TotalForwards++
		elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
		e.stats.AvgForwardTimeMs = (e.stats.AvgForwardTimeMs*float64(e.stats.TotalForwards-1) + elapsed) / float64(e.stats.TotalForwards)
		e.mu.Unlock()
	}()

	e.mu.RLock()
	adapter, exists := e.adapters[adapterID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("adapter not found: %s", adapterID)
	}

	rank := int(adapter.Config.Rank)
	outputDim := adapter.Config.OutputDimension
	scaling := adapter.Config.ScalingFactor()

	// Step 1: Compute A @ x -> intermediate (rank,)
	intermediate := make([]float64, rank)
	for i := 0; i < rank; i++ {
		var sum float64
		for j, val := range input {
			if j < len(adapter.A[i]) {
				sum += adapter.A[i][j] * val
			}
		}
		intermediate[i] = sum
		if adapter.BiasA != nil {
			intermediate[i] += adapter.BiasA[i]
		}
	}

	// Step 2: Compute B @ intermediate -> output (outputDim,)
	output := make([]float64, outputDim)
	for i := 0; i < outputDim; i++ {
		var sum float64
		for j := 0; j < rank; j++ {
			sum += adapter.B[i][j] * intermediate[j]
		}
		output[i] = sum * scaling
		if adapter.BiasB != nil {
			output[i] += adapter.BiasB[i] * scaling
		}
	}

	// Apply dropout during training (simplified - should use training mode flag)
	if adapter.Config.Dropout > 0 && !adapter.Frozen {
		for i := range output {
			if e.rng.Float64() < adapter.Config.Dropout {
				output[i] = 0
			} else {
				output[i] /= (1 - adapter.Config.Dropout)
			}
		}
	}

	return output, nil
}

// Update applies a gradient update to an adapter.
func (e *LoRAEngine) Update(update *domainNeural.LoRAUpdate) error {
	startTime := time.Now()
	defer func() {
		e.mu.Lock()
		e.stats.TotalUpdates++
		elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
		e.stats.AvgUpdateTimeMs = (e.stats.AvgUpdateTimeMs*float64(e.stats.TotalUpdates-1) + elapsed) / float64(e.stats.TotalUpdates)
		e.mu.Unlock()
	}()

	e.mu.Lock()
	defer e.mu.Unlock()

	adapter, exists := e.adapters[update.AdapterID]
	if !exists {
		return fmt.Errorf("adapter not found: %s", update.AdapterID)
	}

	if adapter.Frozen {
		return fmt.Errorf("adapter is frozen: %s", update.AdapterID)
	}

	lr := update.LearningRate

	// Update matrix A
	for i := range adapter.A {
		if i < len(update.GradA) {
			for j := range adapter.A[i] {
				if j < len(update.GradA[i]) {
					adapter.A[i][j] -= lr * update.GradA[i][j]
				}
			}
		}
	}

	// Update matrix B
	for i := range adapter.B {
		if i < len(update.GradB) {
			for j := range adapter.B[i] {
				if j < len(update.GradB[i]) {
					adapter.B[i][j] -= lr * update.GradB[i][j]
				}
			}
		}
	}

	adapter.UpdatedAt = time.Now()
	adapter.TrainingSteps++

	return nil
}

// ComputeDeltaWeight computes the full delta weight matrix: B @ A.
func (e *LoRAEngine) ComputeDeltaWeight(adapterID string) ([][]float64, error) {
	e.mu.RLock()
	adapter, exists := e.adapters[adapterID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("adapter not found: %s", adapterID)
	}

	rank := int(adapter.Config.Rank)
	outputDim := adapter.Config.OutputDimension
	inputDim := adapter.Config.InputDimension
	scaling := adapter.Config.ScalingFactor()

	// Delta = scaling * B @ A
	delta := make([][]float64, outputDim)
	for i := 0; i < outputDim; i++ {
		delta[i] = make([]float64, inputDim)
		for j := 0; j < inputDim; j++ {
			var sum float64
			for k := 0; k < rank; k++ {
				sum += adapter.B[i][k] * adapter.A[k][j]
			}
			delta[i][j] = sum * scaling
		}
	}

	return delta, nil
}

// MergeAdapters merges multiple adapters into one.
func (e *LoRAEngine) MergeAdapters(adapterIDs []string, config domainNeural.LoRAMergeConfig) (*domainNeural.LoRAAdapter, error) {
	e.mu.RLock()
	adapters := make([]*domainNeural.LoRAAdapter, 0, len(adapterIDs))
	for _, id := range adapterIDs {
		if adapter, exists := e.adapters[id]; exists {
			adapters = append(adapters, adapter)
		}
	}
	e.mu.RUnlock()

	if len(adapters) == 0 {
		return nil, fmt.Errorf("no valid adapters to merge")
	}

	// Normalize weights if requested
	weights := config.Weights
	if config.Normalize {
		var total float64
		for _, id := range adapterIDs {
			total += weights[id]
		}
		if total > 0 {
			for id := range weights {
				weights[id] /= total
			}
		}
	}

	// Use first adapter's config as base
	baseConfig := adapters[0].Config
	rank := int(baseConfig.Rank)
	inputDim := baseConfig.InputDimension
	outputDim := baseConfig.OutputDimension

	// Initialize merged matrices
	mergedA := make([][]float64, rank)
	for i := 0; i < rank; i++ {
		mergedA[i] = make([]float64, inputDim)
	}
	mergedB := make([][]float64, outputDim)
	for i := 0; i < outputDim; i++ {
		mergedB[i] = make([]float64, rank)
	}

	// Merge with weights
	for _, adapter := range adapters {
		weight := weights[adapter.ID]
		if weight == 0 {
			weight = 1.0 / float64(len(adapters))
		}

		for i := 0; i < rank && i < len(adapter.A); i++ {
			for j := 0; j < inputDim && j < len(adapter.A[i]); j++ {
				mergedA[i][j] += weight * adapter.A[i][j]
			}
		}
		for i := 0; i < outputDim && i < len(adapter.B); i++ {
			for j := 0; j < rank && j < len(adapter.B[i]); j++ {
				mergedB[i][j] += weight * adapter.B[i][j]
			}
		}
	}

	merged := &domainNeural.LoRAAdapter{
		ID:             uuid.New().String(),
		Name:           "merged-adapter",
		Config:         baseConfig,
		A:              mergedA,
		B:              mergedB,
		InjectionPoint: adapters[0].InjectionPoint,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	e.mu.Lock()
	e.adapters[merged.ID] = merged
	e.stats.TotalAdapters++
	e.mu.Unlock()

	return merged, nil
}

// GetAdapter returns an adapter by ID.
func (e *LoRAEngine) GetAdapter(adapterID string) (*domainNeural.LoRAAdapter, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	adapter, exists := e.adapters[adapterID]
	if !exists {
		return nil, fmt.Errorf("adapter not found: %s", adapterID)
	}
	return adapter, nil
}

// ListAdapters returns all adapter IDs.
func (e *LoRAEngine) ListAdapters() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]string, 0, len(e.adapters))
	for id := range e.adapters {
		ids = append(ids, id)
	}
	return ids
}

// DeleteAdapter deletes an adapter.
func (e *LoRAEngine) DeleteAdapter(adapterID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.adapters[adapterID]; !exists {
		return fmt.Errorf("adapter not found: %s", adapterID)
	}

	delete(e.adapters, adapterID)
	return nil
}

// FreezeAdapter freezes an adapter.
func (e *LoRAEngine) FreezeAdapter(adapterID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	adapter, exists := e.adapters[adapterID]
	if !exists {
		return fmt.Errorf("adapter not found: %s", adapterID)
	}

	adapter.Frozen = true
	return nil
}

// UnfreezeAdapter unfreezes an adapter.
func (e *LoRAEngine) UnfreezeAdapter(adapterID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	adapter, exists := e.adapters[adapterID]
	if !exists {
		return fmt.Errorf("adapter not found: %s", adapterID)
	}

	adapter.Frozen = false
	return nil
}

// GetStats returns engine statistics.
func (e *LoRAEngine) GetStats() *LoRAEngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// GetAdapterStats returns statistics for a specific adapter.
func (e *LoRAEngine) GetAdapterStats(adapterID string) (*domainNeural.LoRAStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	adapter, exists := e.adapters[adapterID]
	if !exists {
		return nil, fmt.Errorf("adapter not found: %s", adapterID)
	}

	rank := int(adapter.Config.Rank)
	inputDim := adapter.Config.InputDimension
	outputDim := adapter.Config.OutputDimension

	// Calculate parameter counts
	paramA := rank * inputDim
	paramB := outputDim * rank
	totalParams := paramA + paramB

	// Calculate norms
	var normA, normB float64
	for i := range adapter.A {
		for j := range adapter.A[i] {
			normA += adapter.A[i][j] * adapter.A[i][j]
		}
	}
	normA = math.Sqrt(normA)

	for i := range adapter.B {
		for j := range adapter.B[i] {
			normB += adapter.B[i][j] * adapter.B[i][j]
		}
	}
	normB = math.Sqrt(normB)

	// Full fine-tuning would be inputDim * outputDim
	fullParams := inputDim * outputDim
	compressionRatio := 1.0
	if fullParams > 0 {
		compressionRatio = float64(totalParams) / float64(fullParams)
	}

	return &domainNeural.LoRAStats{
		AdapterID:           adapterID,
		ParameterCount:      int64(totalParams),
		TrainableParameters: int64(totalParams),
		MemoryBytes:         int64(totalParams * 8), // 8 bytes per float64
		CompressionRatio:    compressionRatio,
		TrainingSteps:       adapter.TrainingSteps,
		AvgUpdateTimeMs:     e.stats.AvgUpdateTimeMs,
		NormA:               normA,
		NormB:               normB,
	}, nil
}

// CreateCheckpoint creates a checkpoint for an adapter.
func (e *LoRAEngine) CreateCheckpoint(adapterID string) (*domainNeural.LoRACheckpoint, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	adapter, exists := e.adapters[adapterID]
	if !exists {
		return nil, fmt.Errorf("adapter not found: %s", adapterID)
	}

	// Deep copy matrices
	A := make([][]float64, len(adapter.A))
	for i := range adapter.A {
		A[i] = make([]float64, len(adapter.A[i]))
		copy(A[i], adapter.A[i])
	}

	B := make([][]float64, len(adapter.B))
	for i := range adapter.B {
		B[i] = make([]float64, len(adapter.B[i]))
		copy(B[i], adapter.B[i])
	}

	return &domainNeural.LoRACheckpoint{
		ID:        uuid.New().String(),
		AdapterID: adapterID,
		Step:      adapter.TrainingSteps,
		A:         A,
		B:         B,
		CreatedAt: time.Now(),
	}, nil
}

// RestoreCheckpoint restores an adapter from a checkpoint.
func (e *LoRAEngine) RestoreCheckpoint(checkpoint *domainNeural.LoRACheckpoint) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	adapter, exists := e.adapters[checkpoint.AdapterID]
	if !exists {
		return fmt.Errorf("adapter not found: %s", checkpoint.AdapterID)
	}

	adapter.A = checkpoint.A
	adapter.B = checkpoint.B
	adapter.TrainingSteps = checkpoint.Step
	adapter.UpdatedAt = time.Now()

	return nil
}
