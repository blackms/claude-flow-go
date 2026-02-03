// Package neural provides neural network infrastructure.
package neural

import (
	"math"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// FisherMatrix computes and stores Fisher information.
type FisherMatrix struct {
	mu     sync.RWMutex
	config FisherConfig

	// Diagonal Fisher approximation (most memory-efficient)
	diagonal []float64

	// Sample count for running average
	sampleCount int64

	// Task-specific contributions
	taskContributions map[string][]float64

	// Statistics
	stats *FisherStats
}

// FisherConfig configures Fisher information computation.
type FisherConfig struct {
	// Dimension is the number of parameters.
	Dimension int `json:"dimension"`

	// UseRunningAverage uses running average for updates.
	UseRunningAverage bool `json:"useRunningAverage"`

	// DecayFactor for online updates (gamma in EWC++).
	DecayFactor float64 `json:"decayFactor"`

	// ClipValue clips Fisher values to this maximum.
	ClipValue float64 `json:"clipValue"`

	// MinValue is the minimum Fisher value.
	MinValue float64 `json:"minValue"`

	// Normalize normalizes the Fisher diagonal.
	Normalize bool `json:"normalize"`

	// TrackTaskContributions tracks per-task contributions.
	TrackTaskContributions bool `json:"trackTaskContributions"`
}

// DefaultFisherConfig returns the default Fisher configuration.
func DefaultFisherConfig() FisherConfig {
	return FisherConfig{
		Dimension:              256,
		UseRunningAverage:      true,
		DecayFactor:            0.9,
		ClipValue:              100.0,
		MinValue:               1e-6,
		Normalize:              true,
		TrackTaskContributions: true,
	}
}

// FisherStats contains Fisher computation statistics.
type FisherStats struct {
	SampleCount        int64   `json:"sampleCount"`
	AvgValue           float64 `json:"avgValue"`
	MaxValue           float64 `json:"maxValue"`
	MinValue           float64 `json:"minValue"`
	Sparsity           float64 `json:"sparsity"`
	AvgComputeTimeMs   float64 `json:"avgComputeTimeMs"`
	TotalComputeTimeMs float64 `json:"totalComputeTimeMs"`
	TaskCount          int     `json:"taskCount"`
}

// NewFisherMatrix creates a new Fisher matrix.
func NewFisherMatrix(config FisherConfig) *FisherMatrix {
	return &FisherMatrix{
		config:            config,
		diagonal:          make([]float64, config.Dimension),
		taskContributions: make(map[string][]float64),
		stats:             &FisherStats{},
	}
}

// Update updates the Fisher diagonal with gradient information.
// For Fisher information: F_i = E[(d log p / d theta_i)^2]
// Approximated as: F_i ≈ (1/N) * sum(grad_i^2)
func (f *FisherMatrix) Update(gradients []float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sampleCount++
	n := float64(f.sampleCount)

	for i := 0; i < len(gradients) && i < len(f.diagonal); i++ {
		gradSquared := gradients[i] * gradients[i]

		if f.config.UseRunningAverage {
			// Running average: F = (n-1)/n * F + 1/n * grad^2
			f.diagonal[i] = ((n-1)/n)*f.diagonal[i] + (1/n)*gradSquared
		} else {
			// Simple accumulation
			f.diagonal[i] += gradSquared
		}

		// Clip values
		if f.diagonal[i] > f.config.ClipValue {
			f.diagonal[i] = f.config.ClipValue
		}
		if f.diagonal[i] < f.config.MinValue {
			f.diagonal[i] = f.config.MinValue
		}
	}

	f.stats.SampleCount = f.sampleCount
}

// UpdateOnline performs online EWC++ update.
// F_new = gamma * F_old + (1-gamma) * F_current
func (f *FisherMatrix) UpdateOnline(newFisher []float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	gamma := f.config.DecayFactor

	for i := 0; i < len(newFisher) && i < len(f.diagonal); i++ {
		f.diagonal[i] = gamma*f.diagonal[i] + (1-gamma)*newFisher[i]

		// Clip values
		if f.diagonal[i] > f.config.ClipValue {
			f.diagonal[i] = f.config.ClipValue
		}
		if f.diagonal[i] < f.config.MinValue {
			f.diagonal[i] = f.config.MinValue
		}
	}
}

// AddTaskContribution adds Fisher contribution from a specific task.
func (f *FisherMatrix) AddTaskContribution(taskID string, fisher []float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.config.TrackTaskContributions {
		contribution := make([]float64, len(fisher))
		copy(contribution, fisher)
		f.taskContributions[taskID] = contribution
	}

	// Update diagonal with online formula
	f.UpdateOnline(fisher)
	f.stats.TaskCount = len(f.taskContributions)
}

// GetDiagonal returns the Fisher diagonal.
func (f *FisherMatrix) GetDiagonal() []float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]float64, len(f.diagonal))
	copy(result, f.diagonal)
	return result
}

// GetNormalizedDiagonal returns the normalized Fisher diagonal.
func (f *FisherMatrix) GetNormalizedDiagonal() []float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]float64, len(f.diagonal))
	copy(result, f.diagonal)

	// Find max for normalization
	var maxVal float64
	for _, v := range result {
		if v > maxVal {
			maxVal = v
		}
	}

	if maxVal > 0 {
		for i := range result {
			result[i] /= maxVal
		}
	}

	return result
}

// ComputeFromGradients computes Fisher from a batch of gradients.
func (f *FisherMatrix) ComputeFromGradients(gradientBatch [][]float64) {
	startTime := time.Now()

	if len(gradientBatch) == 0 {
		return
	}

	dim := len(f.diagonal)
	newFisher := make([]float64, dim)

	// Compute average squared gradients
	for _, grads := range gradientBatch {
		for i := 0; i < len(grads) && i < dim; i++ {
			newFisher[i] += grads[i] * grads[i]
		}
	}

	n := float64(len(gradientBatch))
	for i := range newFisher {
		newFisher[i] /= n
	}

	// Update with online formula
	f.UpdateOnline(newFisher)

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	f.mu.Lock()
	f.stats.TotalComputeTimeMs += elapsed
	f.stats.AvgComputeTimeMs = f.stats.TotalComputeTimeMs / float64(f.sampleCount+1)
	f.mu.Unlock()
}

// ComputeRegularizationLoss computes the EWC regularization loss.
// L_ewc = (lambda/2) * sum_i(F_i * (theta_i - theta_old_i)^2)
func (f *FisherMatrix) ComputeRegularizationLoss(currentParams, oldParams []float64, lambda float64) *domainNeural.EWCLoss {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var regLoss float64
	var totalDrift float64
	var maxDrift float64

	minLen := len(f.diagonal)
	if len(currentParams) < minLen {
		minLen = len(currentParams)
	}
	if len(oldParams) < minLen {
		minLen = len(oldParams)
	}

	for i := 0; i < minLen; i++ {
		diff := currentParams[i] - oldParams[i]
		diffSquared := diff * diff
		
		// Weighted by Fisher importance
		regLoss += f.diagonal[i] * diffSquared
		totalDrift += math.Abs(diff)

		if math.Abs(diff) > maxDrift {
			maxDrift = math.Abs(diff)
		}
	}

	regLoss *= lambda / 2.0

	return &domainNeural.EWCLoss{
		RegularizationLoss: regLoss,
		ParameterDrift:     totalDrift / float64(minLen),
		MaxDrift:           maxDrift,
	}
}

// GetImportanceWeights returns importance weights for all parameters.
func (f *FisherMatrix) GetImportanceWeights() []domainNeural.ImportanceWeight {
	f.mu.RLock()
	defer f.mu.RUnlock()

	weights := make([]domainNeural.ImportanceWeight, len(f.diagonal))
	for i, val := range f.diagonal {
		weight := domainNeural.ImportanceWeight{
			ParameterIndex: i,
			Value:          val,
		}

		// Add task contributions if tracked
		if f.config.TrackTaskContributions {
			weight.TaskContributions = make(map[string]float64)
			for taskID, contrib := range f.taskContributions {
				if i < len(contrib) {
					weight.TaskContributions[taskID] = contrib[i]
				}
			}
		}

		weights[i] = weight
	}

	return weights
}

// GetStats returns Fisher statistics.
func (f *FisherMatrix) GetStats() *FisherStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Compute statistics
	var sum, maxVal float64
	minVal := math.MaxFloat64
	zeroCount := 0

	for _, v := range f.diagonal {
		sum += v
		if v > maxVal {
			maxVal = v
		}
		if v < minVal {
			minVal = v
		}
		if v < f.config.MinValue*2 {
			zeroCount++
		}
	}

	f.stats.AvgValue = sum / float64(len(f.diagonal))
	f.stats.MaxValue = maxVal
	f.stats.MinValue = minVal
	f.stats.Sparsity = float64(zeroCount) / float64(len(f.diagonal))

	return f.stats
}

// Reset resets the Fisher matrix.
func (f *FisherMatrix) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.diagonal = make([]float64, f.config.Dimension)
	f.sampleCount = 0
	f.taskContributions = make(map[string][]float64)
	f.stats = &FisherStats{}
}

// Clone creates a copy of the Fisher matrix.
func (f *FisherMatrix) Clone() *FisherMatrix {
	f.mu.RLock()
	defer f.mu.RUnlock()

	clone := NewFisherMatrix(f.config)
	copy(clone.diagonal, f.diagonal)
	clone.sampleCount = f.sampleCount

	for taskID, contrib := range f.taskContributions {
		cloneContrib := make([]float64, len(contrib))
		copy(cloneContrib, contrib)
		clone.taskContributions[taskID] = cloneContrib
	}

	return clone
}

// Merge merges another Fisher matrix into this one.
func (f *FisherMatrix) Merge(other *FisherMatrix, weight float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	for i := 0; i < len(f.diagonal) && i < len(other.diagonal); i++ {
		f.diagonal[i] = (1-weight)*f.diagonal[i] + weight*other.diagonal[i]
	}

	// Merge task contributions
	for taskID, contrib := range other.taskContributions {
		if _, exists := f.taskContributions[taskID]; !exists {
			f.taskContributions[taskID] = make([]float64, len(contrib))
			copy(f.taskContributions[taskID], contrib)
		}
	}

	f.stats.TaskCount = len(f.taskContributions)
}

// TopKImportant returns the indices of the top K most important parameters.
func (f *FisherMatrix) TopKImportant(k int) []int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if k > len(f.diagonal) {
		k = len(f.diagonal)
	}

	// Create index-value pairs
	type indexValue struct {
		index int
		value float64
	}
	pairs := make([]indexValue, len(f.diagonal))
	for i, v := range f.diagonal {
		pairs[i] = indexValue{i, v}
	}

	// Simple selection sort for top K (efficient for small K)
	for i := 0; i < k; i++ {
		maxIdx := i
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].value > pairs[maxIdx].value {
				maxIdx = j
			}
		}
		pairs[i], pairs[maxIdx] = pairs[maxIdx], pairs[i]
	}

	result := make([]int, k)
	for i := 0; i < k; i++ {
		result[i] = pairs[i].index
	}

	return result
}
