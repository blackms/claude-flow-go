// Package neural provides neural network infrastructure.
package neural

import (
	"math"
	"math/rand"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// ModeImplementation defines the interface for SONA mode implementations.
type ModeImplementation interface {
	// Adapt applies mode-specific adaptation to an embedding.
	Adapt(embedding []float64) ([]float64, error)

	// Learn performs mode-specific learning from a trajectory.
	Learn(trajectory *domainNeural.ExtendedTrajectory) error

	// GetStats returns mode-specific statistics.
	GetStats() ModeStats

	// Cleanup cleans up mode resources.
	Cleanup() error

	// GetConfig returns the mode configuration.
	GetConfig() domainNeural.ModeConfig
}

// ModeStats contains mode-specific statistics.
type ModeStats struct {
	// Adaptations is the number of adaptations performed.
	Adaptations int64 `json:"adaptations"`

	// LearningCycles is the number of learning cycles.
	LearningCycles int64 `json:"learningCycles"`

	// AvgAdaptLatencyMs is the average adaptation latency.
	AvgAdaptLatencyMs float64 `json:"avgAdaptLatencyMs"`

	// AvgLearnLatencyMs is the average learning latency.
	AvgLearnLatencyMs float64 `json:"avgLearnLatencyMs"`

	// CacheHits is the number of cache hits.
	CacheHits int64 `json:"cacheHits"`

	// CacheMisses is the number of cache misses.
	CacheMisses int64 `json:"cacheMisses"`

	// MemoryUsedBytes is the memory used.
	MemoryUsedBytes int64 `json:"memoryUsedBytes"`
}

// BaseModeImplementation provides common functionality for all modes.
type BaseModeImplementation struct {
	mu     sync.RWMutex
	config domainNeural.ModeConfig
	rng    *rand.Rand

	// LoRA weights
	loraA [][]float64 // Down-projection: hidden_dim x rank
	loraB [][]float64 // Up-projection: rank x hidden_dim

	// Pattern cache
	patternCache map[string][]float64

	// Statistics
	stats ModeStats
}

// NewBaseModeImplementation creates a base mode implementation.
func NewBaseModeImplementation(config domainNeural.ModeConfig) *BaseModeImplementation {
	hiddenDim := 256 // Default embedding dimension
	rank := config.LoRARank

	base := &BaseModeImplementation{
		config:       config,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
		loraA:        make([][]float64, hiddenDim),
		loraB:        make([][]float64, rank),
		patternCache: make(map[string][]float64),
	}

	// Initialize LoRA weights with Xavier initialization
	scale := math.Sqrt(2.0 / float64(hiddenDim))
	for i := 0; i < hiddenDim; i++ {
		base.loraA[i] = make([]float64, rank)
		for j := 0; j < rank; j++ {
			base.loraA[i][j] = (base.rng.Float64() - 0.5) * scale
		}
	}

	for i := 0; i < rank; i++ {
		base.loraB[i] = make([]float64, hiddenDim)
		for j := 0; j < hiddenDim; j++ {
			base.loraB[i][j] = (base.rng.Float64() - 0.5) * scale * 0.01 // Small initialization for B
		}
	}

	return base
}

// applyLoRATransform applies the LoRA transformation.
// output = input + alpha * (B @ A @ input)
func (b *BaseModeImplementation) applyLoRATransform(input []float64) []float64 {
	rank := b.config.LoRARank
	alpha := b.config.LoRAAlpha
	hiddenDim := len(input)

	if hiddenDim == 0 || hiddenDim > len(b.loraA) {
		return input
	}

	// Step 1: A @ input -> intermediate (rank,)
	intermediate := make([]float64, rank)
	for i := 0; i < rank; i++ {
		var sum float64
		for j := 0; j < hiddenDim && j < len(b.loraA); j++ {
			if i < len(b.loraA[j]) {
				sum += b.loraA[j][i] * input[j]
			}
		}
		intermediate[i] = sum
	}

	// Step 2: B @ intermediate -> delta (hiddenDim,)
	delta := make([]float64, hiddenDim)
	for i := 0; i < rank && i < len(b.loraB); i++ {
		for j := 0; j < hiddenDim && j < len(b.loraB[i]); j++ {
			delta[j] += b.loraB[i][j] * intermediate[i]
		}
	}

	// Step 3: output = input + alpha * delta
	output := make([]float64, hiddenDim)
	for i := 0; i < hiddenDim; i++ {
		output[i] = input[i] + alpha*delta[i]
	}

	return output
}

// GetConfig returns the mode configuration.
func (b *BaseModeImplementation) GetConfig() domainNeural.ModeConfig {
	return b.config
}

// GetStats returns the mode statistics.
func (b *BaseModeImplementation) GetStats() ModeStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.stats
}

// Cleanup cleans up resources.
func (b *BaseModeImplementation) Cleanup() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.patternCache = make(map[string][]float64)
	return nil
}

// ============================================================================
// Real-Time Mode Implementation
// ============================================================================

// RealTimeModeImpl implements real-time mode with sub-millisecond latency.
type RealTimeModeImpl struct {
	*BaseModeImplementation
}

// NewRealTimeModeImpl creates a new real-time mode implementation.
func NewRealTimeModeImpl(config domainNeural.ModeConfig) *RealTimeModeImpl {
	return &RealTimeModeImpl{
		BaseModeImplementation: NewBaseModeImplementation(config),
	}
}

// Adapt applies real-time adaptation (target: <0.5ms).
func (m *RealTimeModeImpl) Adapt(embedding []float64) ([]float64, error) {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Apply micro-LoRA transformation
	result := m.applyLoRATransform(embedding)

	// Update statistics
	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.Adaptations++
	m.stats.AvgAdaptLatencyMs = (m.stats.AvgAdaptLatencyMs*float64(m.stats.Adaptations-1) + elapsed) / float64(m.stats.Adaptations)

	return result, nil
}

// Learn performs minimal learning for real-time mode.
func (m *RealTimeModeImpl) Learn(trajectory *domainNeural.ExtendedTrajectory) error {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	if trajectory == nil || trajectory.QualityScore < m.config.QualityThreshold {
		return nil
	}

	// Simplified gradient update for real-time
	// Only update if quality is high enough
	lr := m.config.LearningRate * 0.1 // Reduced learning rate for stability

	for _, step := range trajectory.Steps {
		if len(step.StateAfter) == 0 {
			continue
		}

		// Simple weight nudge based on reward
		reward := step.Reward
		for i := range m.loraA {
			for j := range m.loraA[i] {
				m.loraA[i][j] += lr * reward * (m.rng.Float64() - 0.5) * 0.01
			}
		}
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.LearningCycles++
	m.stats.AvgLearnLatencyMs = (m.stats.AvgLearnLatencyMs*float64(m.stats.LearningCycles-1) + elapsed) / float64(m.stats.LearningCycles)

	return nil
}

// ============================================================================
// Balanced Mode Implementation
// ============================================================================

// BalancedModeImpl implements balanced mode with good speed/quality tradeoff.
type BalancedModeImpl struct {
	*BaseModeImplementation
	momentum [][]float64 // Momentum for gradient updates
}

// NewBalancedModeImpl creates a new balanced mode implementation.
func NewBalancedModeImpl(config domainNeural.ModeConfig) *BalancedModeImpl {
	base := NewBaseModeImplementation(config)
	hiddenDim := len(base.loraA)
	rank := config.LoRARank

	momentum := make([][]float64, hiddenDim)
	for i := 0; i < hiddenDim; i++ {
		momentum[i] = make([]float64, rank)
	}

	return &BalancedModeImpl{
		BaseModeImplementation: base,
		momentum:               momentum,
	}
}

// Adapt applies balanced adaptation (target: <18ms).
func (m *BalancedModeImpl) Adapt(embedding []float64) ([]float64, error) {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check pattern cache
	cacheKey := embeddingCacheKey(embedding)
	if cached, ok := m.patternCache[cacheKey]; ok && m.config.Optimizations.PatternCaching {
		m.stats.CacheHits++
		return cached, nil
	}
	m.stats.CacheMisses++

	// Apply LoRA transformation
	result := m.applyLoRATransform(embedding)

	// Cache result
	if m.config.Optimizations.PatternCaching {
		m.patternCache[cacheKey] = result
		// Limit cache size
		if len(m.patternCache) > 1000 {
			// Remove random entry
			for k := range m.patternCache {
				delete(m.patternCache, k)
				break
			}
		}
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.Adaptations++
	m.stats.AvgAdaptLatencyMs = (m.stats.AvgAdaptLatencyMs*float64(m.stats.Adaptations-1) + elapsed) / float64(m.stats.Adaptations)

	return result, nil
}

// Learn performs balanced learning with momentum.
func (m *BalancedModeImpl) Learn(trajectory *domainNeural.ExtendedTrajectory) error {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	if trajectory == nil || trajectory.QualityScore < m.config.QualityThreshold {
		return nil
	}

	lr := m.config.LearningRate
	beta := 0.9 // Momentum coefficient

	// Compute gradients from trajectory steps
	for _, step := range trajectory.Steps {
		if len(step.StateAfter) == 0 {
			continue
		}

		reward := step.Reward
		for i := 0; i < len(m.loraA) && i < len(step.StateAfter); i++ {
			for j := range m.loraA[i] {
				grad := reward * float64(step.StateAfter[i]) * 0.001
				m.momentum[i][j] = beta*m.momentum[i][j] + (1-beta)*grad
				m.loraA[i][j] += lr * m.momentum[i][j]
			}
		}
	}

	// Clear pattern cache after learning
	m.patternCache = make(map[string][]float64)

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.LearningCycles++
	m.stats.AvgLearnLatencyMs = (m.stats.AvgLearnLatencyMs*float64(m.stats.LearningCycles-1) + elapsed) / float64(m.stats.LearningCycles)

	return nil
}

// ============================================================================
// Research Mode Implementation
// ============================================================================

// ResearchModeImpl implements research mode for maximum quality.
type ResearchModeImpl struct {
	*BaseModeImplementation
	adamM      [][]float64 // First moment estimate
	adamV      [][]float64 // Second moment estimate
	adamStep   int64       // Adam step counter
	checkpoints [][]float64 // Gradient checkpoints
}

// NewResearchModeImpl creates a new research mode implementation.
func NewResearchModeImpl(config domainNeural.ModeConfig) *ResearchModeImpl {
	base := NewBaseModeImplementation(config)
	hiddenDim := len(base.loraA)
	rank := config.LoRARank

	adamM := make([][]float64, hiddenDim)
	adamV := make([][]float64, hiddenDim)
	for i := 0; i < hiddenDim; i++ {
		adamM[i] = make([]float64, rank)
		adamV[i] = make([]float64, rank)
	}

	return &ResearchModeImpl{
		BaseModeImplementation: base,
		adamM:                  adamM,
		adamV:                  adamV,
		checkpoints:            make([][]float64, 0),
	}
}

// Adapt applies research adaptation (target: <100ms).
func (m *ResearchModeImpl) Adapt(embedding []float64) ([]float64, error) {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check pattern cache
	cacheKey := embeddingCacheKey(embedding)
	if cached, ok := m.patternCache[cacheKey]; ok && m.config.Optimizations.PatternCaching {
		m.stats.CacheHits++
		return cached, nil
	}
	m.stats.CacheMisses++

	// Apply full LoRA transformation with higher rank
	result := m.applyLoRATransform(embedding)

	// Cache result
	if m.config.Optimizations.PatternCaching {
		m.patternCache[cacheKey] = result
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.Adaptations++
	m.stats.AvgAdaptLatencyMs = (m.stats.AvgAdaptLatencyMs*float64(m.stats.Adaptations-1) + elapsed) / float64(m.stats.Adaptations)

	return result, nil
}

// Learn performs research learning with Adam optimizer.
func (m *ResearchModeImpl) Learn(trajectory *domainNeural.ExtendedTrajectory) error {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	if trajectory == nil {
		return nil
	}

	// Accept lower quality trajectories for research
	if trajectory.QualityScore < m.config.QualityThreshold {
		return nil
	}

	lr := m.config.LearningRate
	beta1 := 0.9
	beta2 := 0.999
	epsilon := 1e-8

	m.adamStep++
	t := float64(m.adamStep)

	// Compute gradients and apply Adam update
	for _, step := range trajectory.Steps {
		if len(step.StateAfter) == 0 {
			continue
		}

		reward := step.Reward
		for i := 0; i < len(m.loraA) && i < len(step.StateAfter); i++ {
			for j := range m.loraA[i] {
				grad := reward * float64(step.StateAfter[i]) * 0.001

				// Update biased first moment estimate
				m.adamM[i][j] = beta1*m.adamM[i][j] + (1-beta1)*grad

				// Update biased second raw moment estimate
				m.adamV[i][j] = beta2*m.adamV[i][j] + (1-beta2)*grad*grad

				// Compute bias-corrected first moment estimate
				mHat := m.adamM[i][j] / (1 - math.Pow(beta1, t))

				// Compute bias-corrected second raw moment estimate
				vHat := m.adamV[i][j] / (1 - math.Pow(beta2, t))

				// Update weights
				m.loraA[i][j] += lr * mHat / (math.Sqrt(vHat) + epsilon)
			}
		}
	}

	// Checkpoint if enabled
	if m.config.Optimizations.GradientCheckpointing {
		checkpoint := make([]float64, 0)
		for i := range m.loraA {
			checkpoint = append(checkpoint, m.loraA[i]...)
		}
		m.checkpoints = append(m.checkpoints, checkpoint)
		// Keep only last 10 checkpoints
		if len(m.checkpoints) > 10 {
			m.checkpoints = m.checkpoints[1:]
		}
	}

	// Clear pattern cache after learning
	m.patternCache = make(map[string][]float64)

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.LearningCycles++
	m.stats.AvgLearnLatencyMs = (m.stats.AvgLearnLatencyMs*float64(m.stats.LearningCycles-1) + elapsed) / float64(m.stats.LearningCycles)

	return nil
}

// ============================================================================
// Edge Mode Implementation
// ============================================================================

// EdgeModeImpl implements edge mode for resource-constrained environments.
type EdgeModeImpl struct {
	*BaseModeImplementation
	quantizedA [][]int8   // Quantized weights
	scale      float64    // Quantization scale
}

// NewEdgeModeImpl creates a new edge mode implementation.
func NewEdgeModeImpl(config domainNeural.ModeConfig) *EdgeModeImpl {
	base := NewBaseModeImplementation(config)
	
	// Quantize weights
	quantizedA := make([][]int8, len(base.loraA))
	var maxVal float64
	for i := range base.loraA {
		for j := range base.loraA[i] {
			if math.Abs(base.loraA[i][j]) > maxVal {
				maxVal = math.Abs(base.loraA[i][j])
			}
		}
	}

	scale := maxVal / 127.0
	if scale == 0 {
		scale = 1.0
	}

	for i := range base.loraA {
		quantizedA[i] = make([]int8, len(base.loraA[i]))
		for j := range base.loraA[i] {
			quantizedA[i][j] = int8(base.loraA[i][j] / scale)
		}
	}

	return &EdgeModeImpl{
		BaseModeImplementation: base,
		quantizedA:             quantizedA,
		scale:                  scale,
	}
}

// Adapt applies edge adaptation (target: <1ms).
func (m *EdgeModeImpl) Adapt(embedding []float64) ([]float64, error) {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	rank := m.config.LoRARank
	alpha := m.config.LoRAAlpha
	hiddenDim := len(embedding)

	if hiddenDim == 0 {
		return embedding, nil
	}

	// Use quantized weights for faster computation
	intermediate := make([]float64, rank)
	for i := 0; i < rank; i++ {
		var sum float64
		for j := 0; j < hiddenDim && j < len(m.quantizedA); j++ {
			if i < len(m.quantizedA[j]) {
				sum += float64(m.quantizedA[j][i]) * m.scale * embedding[j]
			}
		}
		intermediate[i] = sum
	}

	// Apply B transformation
	output := make([]float64, hiddenDim)
	copy(output, embedding)
	for i := 0; i < rank && i < len(m.loraB); i++ {
		for j := 0; j < hiddenDim && j < len(m.loraB[i]); j++ {
			output[j] += alpha * m.loraB[i][j] * intermediate[i]
		}
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.Adaptations++
	m.stats.AvgAdaptLatencyMs = (m.stats.AvgAdaptLatencyMs*float64(m.stats.Adaptations-1) + elapsed) / float64(m.stats.Adaptations)

	return output, nil
}

// Learn performs minimal learning for edge mode.
func (m *EdgeModeImpl) Learn(trajectory *domainNeural.ExtendedTrajectory) error {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	if trajectory == nil || trajectory.QualityScore < m.config.QualityThreshold {
		return nil
	}

	// Minimal EMA-based update
	lr := m.config.LearningRate * 0.1

	for _, step := range trajectory.Steps {
		if step.Reward <= 0 {
			continue
		}

		// Simple weight update
		for i := range m.loraA {
			for j := range m.loraA[i] {
				m.loraA[i][j] += lr * step.Reward * (m.rng.Float64() - 0.5) * 0.01
			}
		}
	}

	// Re-quantize
	var maxVal float64
	for i := range m.loraA {
		for j := range m.loraA[i] {
			if math.Abs(m.loraA[i][j]) > maxVal {
				maxVal = math.Abs(m.loraA[i][j])
			}
		}
	}
	m.scale = maxVal / 127.0
	if m.scale == 0 {
		m.scale = 1.0
	}
	for i := range m.loraA {
		for j := range m.loraA[i] {
			m.quantizedA[i][j] = int8(m.loraA[i][j] / m.scale)
		}
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.LearningCycles++
	m.stats.AvgLearnLatencyMs = (m.stats.AvgLearnLatencyMs*float64(m.stats.LearningCycles-1) + elapsed) / float64(m.stats.LearningCycles)

	return nil
}

// ============================================================================
// Batch Mode Implementation
// ============================================================================

// BatchModeImpl implements batch mode for high-throughput processing.
type BatchModeImpl struct {
	*BaseModeImplementation
	gradientBuffer [][]float64 // Accumulated gradients
	accumStep      int         // Current accumulation step
}

// NewBatchModeImpl creates a new batch mode implementation.
func NewBatchModeImpl(config domainNeural.ModeConfig) *BatchModeImpl {
	base := NewBaseModeImplementation(config)
	hiddenDim := len(base.loraA)
	rank := config.LoRARank

	gradientBuffer := make([][]float64, hiddenDim)
	for i := 0; i < hiddenDim; i++ {
		gradientBuffer[i] = make([]float64, rank)
	}

	return &BatchModeImpl{
		BaseModeImplementation: base,
		gradientBuffer:         gradientBuffer,
		accumStep:              0,
	}
}

// Adapt applies batch adaptation (target: <50ms).
func (m *BatchModeImpl) Adapt(embedding []float64) ([]float64, error) {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check pattern cache
	cacheKey := embeddingCacheKey(embedding)
	if cached, ok := m.patternCache[cacheKey]; ok && m.config.Optimizations.PatternCaching {
		m.stats.CacheHits++
		return cached, nil
	}
	m.stats.CacheMisses++

	// Apply LoRA transformation
	result := m.applyLoRATransform(embedding)

	// Cache result
	if m.config.Optimizations.PatternCaching {
		m.patternCache[cacheKey] = result
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.Adaptations++
	m.stats.AvgAdaptLatencyMs = (m.stats.AvgAdaptLatencyMs*float64(m.stats.Adaptations-1) + elapsed) / float64(m.stats.Adaptations)

	return result, nil
}

// Learn performs batch learning with gradient accumulation.
func (m *BatchModeImpl) Learn(trajectory *domainNeural.ExtendedTrajectory) error {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	if trajectory == nil || trajectory.QualityScore < m.config.QualityThreshold {
		return nil
	}

	accumSteps := m.config.Optimizations.GradientAccumulationSteps
	if accumSteps <= 0 {
		accumSteps = 4
	}

	// Accumulate gradients
	for _, step := range trajectory.Steps {
		if len(step.StateAfter) == 0 {
			continue
		}

		reward := step.Reward
		for i := 0; i < len(m.gradientBuffer) && i < len(step.StateAfter); i++ {
			for j := range m.gradientBuffer[i] {
				grad := reward * float64(step.StateAfter[i]) * 0.001
				m.gradientBuffer[i][j] += grad
			}
		}
	}

	m.accumStep++

	// Apply accumulated gradients when reaching accumulation steps
	if m.accumStep >= accumSteps {
		lr := m.config.LearningRate / float64(accumSteps)

		for i := range m.loraA {
			for j := range m.loraA[i] {
				m.loraA[i][j] += lr * m.gradientBuffer[i][j]
				m.gradientBuffer[i][j] = 0 // Reset gradient buffer
			}
		}

		m.accumStep = 0

		// Clear pattern cache after applying gradients
		m.patternCache = make(map[string][]float64)
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.stats.LearningCycles++
	m.stats.AvgLearnLatencyMs = (m.stats.AvgLearnLatencyMs*float64(m.stats.LearningCycles-1) + elapsed) / float64(m.stats.LearningCycles)

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// embeddingCacheKey generates a cache key from an embedding.
func embeddingCacheKey(embedding []float64) string {
	// Use first and last few values plus length for quick key
	if len(embedding) == 0 {
		return "empty"
	}

	key := ""
	for i := 0; i < 4 && i < len(embedding); i++ {
		key += string(rune(int(embedding[i]*1000) % 256))
	}
	for i := len(embedding) - 4; i < len(embedding); i++ {
		if i >= 0 {
			key += string(rune(int(embedding[i]*1000) % 256))
		}
	}
	return key
}

// CreateModeImplementation creates a mode implementation for the given mode.
func CreateModeImplementation(mode domainNeural.SONAMode) ModeImplementation {
	config := domainNeural.DefaultModeConfig(mode)
	return CreateModeImplementationWithConfig(config)
}

// CreateModeImplementationWithConfig creates a mode implementation with custom config.
func CreateModeImplementationWithConfig(config domainNeural.ModeConfig) ModeImplementation {
	switch config.Mode {
	case domainNeural.ModeRealTime:
		return NewRealTimeModeImpl(config)
	case domainNeural.ModeBalanced:
		return NewBalancedModeImpl(config)
	case domainNeural.ModeResearch:
		return NewResearchModeImpl(config)
	case domainNeural.ModeEdge:
		return NewEdgeModeImpl(config)
	case domainNeural.ModeBatch:
		return NewBatchModeImpl(config)
	default:
		return NewBalancedModeImpl(domainNeural.DefaultModeConfig(domainNeural.ModeBalanced))
	}
}
