// Package neural provides domain types for neural learning systems.
package neural

import (
	"time"
)

// SONAMode represents a SONA learning mode.
type SONAMode string

const (
	// ModeRealTime is for real-time adaptation with sub-millisecond latency.
	// Target: <0.5ms adaptation, 70%+ quality, 25MB memory
	ModeRealTime SONAMode = "real-time"

	// ModeBalanced is the default mode balancing speed and quality.
	// Target: <18ms adaptation, 75%+ quality, 50MB memory
	ModeBalanced SONAMode = "balanced"

	// ModeResearch is for maximum quality with relaxed latency.
	// Target: <100ms adaptation, 95%+ quality, 100MB memory
	ModeResearch SONAMode = "research"

	// ModeEdge is for resource-constrained environments.
	// Target: <1ms adaptation, 80%+ quality, 5MB memory
	ModeEdge SONAMode = "edge"

	// ModeBatch is for high-throughput batch processing.
	// Target: <50ms adaptation, 85%+ quality, 75MB memory
	ModeBatch SONAMode = "batch"
)

// AllSONAModes returns all available SONA modes.
func AllSONAModes() []SONAMode {
	return []SONAMode{ModeRealTime, ModeBalanced, ModeResearch, ModeEdge, ModeBatch}
}

// ModeConfig contains configuration for a SONA mode.
type ModeConfig struct {
	// Mode is the SONA mode.
	Mode SONAMode `json:"mode"`

	// LoRARank is the LoRA adapter rank (1-16).
	LoRARank int `json:"loraRank"`

	// LoRAAlpha is the LoRA scaling factor.
	LoRAAlpha float64 `json:"loraAlpha"`

	// LearningRate for gradient updates.
	LearningRate float64 `json:"learningRate"`

	// MaxLatencyMs is the maximum adaptation latency target.
	MaxLatencyMs float64 `json:"maxLatencyMs"`

	// MemoryBudgetMB is the memory budget in megabytes.
	MemoryBudgetMB int `json:"memoryBudgetMb"`

	// QualityThreshold is the minimum quality for learning.
	QualityThreshold float64 `json:"qualityThreshold"`

	// TrajectoryCapacity is the maximum trajectories to store.
	TrajectoryCapacity int `json:"trajectoryCapacity"`

	// PatternClusters is the number of pattern clusters.
	PatternClusters int `json:"patternClusters"`

	// EWCLambda is the EWC regularization strength.
	EWCLambda float64 `json:"ewcLambda"`

	// Optimizations are mode-specific optimization flags.
	Optimizations ModeOptimizations `json:"optimizations"`
}

// ModeOptimizations contains mode-specific optimization flags.
type ModeOptimizations struct {
	// EnableSIMD enables SIMD vectorized operations.
	EnableSIMD bool `json:"enableSimd"`

	// UseMicroLoRA uses reduced parameter count.
	UseMicroLoRA bool `json:"useMicroLora"`

	// GradientCheckpointing enables memory-efficient gradients.
	GradientCheckpointing bool `json:"gradientCheckpointing"`

	// UseHalfPrecision enables FP16 computation.
	UseHalfPrecision bool `json:"useHalfPrecision"`

	// PatternCaching enables pattern result caching.
	PatternCaching bool `json:"patternCaching"`

	// AsyncUpdates enables asynchronous learning updates.
	AsyncUpdates bool `json:"asyncUpdates"`

	// UseQuantization enables weight quantization.
	UseQuantization bool `json:"useQuantization"`

	// CompressEmbeddings enables embedding compression.
	CompressEmbeddings bool `json:"compressEmbeddings"`

	// GradientAccumulation enables gradient accumulation.
	GradientAccumulation bool `json:"gradientAccumulation"`

	// GradientAccumulationSteps is the number of steps for accumulation.
	GradientAccumulationSteps int `json:"gradientAccumulationSteps"`
}

// DefaultModeConfig returns the default configuration for a mode.
func DefaultModeConfig(mode SONAMode) ModeConfig {
	switch mode {
	case ModeRealTime:
		return ModeConfig{
			Mode:               ModeRealTime,
			LoRARank:           2,
			LoRAAlpha:          0.1,
			LearningRate:       0.001,
			MaxLatencyMs:       0.5,
			MemoryBudgetMB:     25,
			QualityThreshold:   0.7,
			TrajectoryCapacity: 1000,
			PatternClusters:    25,
			EWCLambda:          1500,
			Optimizations: ModeOptimizations{
				EnableSIMD:    true,
				UseMicroLoRA:  true,
				AsyncUpdates:  true,
				PatternCaching: true,
			},
		}

	case ModeBalanced:
		return ModeConfig{
			Mode:               ModeBalanced,
			LoRARank:           4,
			LoRAAlpha:          0.2,
			LearningRate:       0.002,
			MaxLatencyMs:       18,
			MemoryBudgetMB:     50,
			QualityThreshold:   0.5,
			TrajectoryCapacity: 3000,
			PatternClusters:    50,
			EWCLambda:          2000,
			Optimizations: ModeOptimizations{
				PatternCaching: true,
				EnableSIMD:     true,
			},
		}

	case ModeResearch:
		return ModeConfig{
			Mode:               ModeResearch,
			LoRARank:           16,
			LoRAAlpha:          0.3,
			LearningRate:       0.002,
			MaxLatencyMs:       100,
			MemoryBudgetMB:     100,
			QualityThreshold:   0.2,
			TrajectoryCapacity: 10000,
			PatternClusters:    100,
			EWCLambda:          2500,
			Optimizations: ModeOptimizations{
				GradientCheckpointing: true,
				PatternCaching:        true,
			},
		}

	case ModeEdge:
		return ModeConfig{
			Mode:               ModeEdge,
			LoRARank:           1,
			LoRAAlpha:          0.05,
			LearningRate:       0.001,
			MaxLatencyMs:       1,
			MemoryBudgetMB:     5,
			QualityThreshold:   0.8,
			TrajectoryCapacity: 200,
			PatternClusters:    15,
			EWCLambda:          1500,
			Optimizations: ModeOptimizations{
				EnableSIMD:         true,
				UseMicroLoRA:       true,
				UseQuantization:    true,
				CompressEmbeddings: true,
				AsyncUpdates:       true,
			},
		}

	case ModeBatch:
		return ModeConfig{
			Mode:               ModeBatch,
			LoRARank:           8,
			LoRAAlpha:          0.25,
			LearningRate:       0.002,
			MaxLatencyMs:       50,
			MemoryBudgetMB:     75,
			QualityThreshold:   0.4,
			TrajectoryCapacity: 5000,
			PatternClusters:    75,
			EWCLambda:          2000,
			Optimizations: ModeOptimizations{
				GradientAccumulation:      true,
				GradientAccumulationSteps: 4,
				PatternCaching:            true,
			},
		}

	default:
		return DefaultModeConfig(ModeBalanced)
	}
}

// TrajectoryStats contains trajectory-related statistics.
type TrajectoryStats struct {
	// Total is the total number of trajectories.
	Total int `json:"total"`

	// Active is the number of active trajectories.
	Active int `json:"active"`

	// Completed is the number of completed trajectories.
	Completed int `json:"completed"`

	// Utilization is the capacity utilization (0-1).
	Utilization float64 `json:"utilization"`
}

// PerformanceStats contains performance-related statistics.
type PerformanceStats struct {
	// AvgQualityScore is the average quality score.
	AvgQualityScore float64 `json:"avgQualityScore"`

	// OpsPerSecond is the operations per second.
	OpsPerSecond float64 `json:"opsPerSecond"`

	// LearningCycles is the number of learning cycles.
	LearningCycles int64 `json:"learningCycles"`

	// AvgLatencyMs is the average latency in milliseconds.
	AvgLatencyMs float64 `json:"avgLatencyMs"`

	// TotalAdaptations is the total number of adaptations.
	TotalAdaptations int64 `json:"totalAdaptations"`
}

// PatternStats contains pattern-related statistics.
type PatternStats struct {
	// TotalPatterns is the total number of patterns.
	TotalPatterns int `json:"totalPatterns"`

	// AvgMatchTimeMs is the average pattern match time.
	AvgMatchTimeMs float64 `json:"avgMatchTimeMs"`

	// CacheHitRate is the cache hit rate (0-1).
	CacheHitRate float64 `json:"cacheHitRate"`

	// EvolutionCount is the number of pattern evolutions.
	EvolutionCount int `json:"evolutionCount"`
}

// MemoryStats contains memory-related statistics.
type MemoryStats struct {
	// UsedMB is the memory used in megabytes.
	UsedMB float64 `json:"usedMb"`

	// BudgetMB is the memory budget in megabytes.
	BudgetMB float64 `json:"budgetMb"`

	// TrajectoryBytes is bytes used by trajectories.
	TrajectoryBytes int64 `json:"trajectoryBytes"`

	// PatternBytes is bytes used by patterns.
	PatternBytes int64 `json:"patternBytes"`

	// AdapterBytes is bytes used by LoRA adapters.
	AdapterBytes int64 `json:"adapterBytes"`
}

// SONAStats contains comprehensive SONA statistics.
type SONAStats struct {
	// Mode is the current SONA mode.
	Mode SONAMode `json:"mode"`

	// Config is the current mode configuration.
	Config ModeConfig `json:"config"`

	// Trajectories contains trajectory statistics.
	Trajectories TrajectoryStats `json:"trajectories"`

	// Performance contains performance statistics.
	Performance PerformanceStats `json:"performance"`

	// Patterns contains pattern statistics.
	Patterns PatternStats `json:"patterns"`

	// Memory contains memory statistics.
	Memory MemoryStats `json:"memory"`

	// StartTime is when SONA was started.
	StartTime time.Time `json:"startTime"`

	// LastModeChange is when the mode was last changed.
	LastModeChange *time.Time `json:"lastModeChange,omitempty"`

	// LastLearning is when learning was last triggered.
	LastLearning *time.Time `json:"lastLearning,omitempty"`

	// ModeHistory contains recent mode changes.
	ModeHistory []ModeChangeEvent `json:"modeHistory,omitempty"`
}

// ModeChangeEvent represents a mode change event.
type ModeChangeEvent struct {
	// FromMode is the previous mode.
	FromMode SONAMode `json:"fromMode"`

	// ToMode is the new mode.
	ToMode SONAMode `json:"toMode"`

	// Timestamp is when the change occurred.
	Timestamp time.Time `json:"timestamp"`

	// Reason is the reason for the change.
	Reason string `json:"reason,omitempty"`
}

// OptimizationResult contains results from pattern optimization.
type OptimizationResult struct {
	// PatternsOptimized is the number of patterns optimized.
	PatternsOptimized int `json:"patternsOptimized"`

	// PatternsPruned is the number of patterns pruned.
	PatternsPruned int `json:"patternsPruned"`

	// PatternsMerged is the number of patterns merged.
	PatternsMerged int `json:"patternsMerged"`

	// MemoryFreedMB is the memory freed in megabytes.
	MemoryFreedMB float64 `json:"memoryFreedMb"`

	// DurationMs is the optimization duration.
	DurationMs int64 `json:"durationMs"`

	// Timestamp is when optimization occurred.
	Timestamp time.Time `json:"timestamp"`
}

// AdaptationResult contains results from an adaptation.
type AdaptationResult struct {
	// Embedding is the adapted embedding.
	Embedding []float64 `json:"embedding"`

	// LatencyMs is the adaptation latency.
	LatencyMs float64 `json:"latencyMs"`

	// Mode is the mode used for adaptation.
	Mode SONAMode `json:"mode"`

	// AdapterUsed is the adapter ID used.
	AdapterUsed string `json:"adapterUsed,omitempty"`

	// CacheHit indicates if pattern cache was used.
	CacheHit bool `json:"cacheHit"`
}

// LearningTriggerResult contains results from a learning trigger.
type LearningTriggerResult struct {
	// Triggered indicates if learning was triggered.
	Triggered bool `json:"triggered"`

	// TrajectoriesProcessed is the number of trajectories processed.
	TrajectoriesProcessed int `json:"trajectoriesProcessed"`

	// PatternsLearned is the number of patterns learned.
	PatternsLearned int `json:"patternsLearned"`

	// AvgLoss is the average loss.
	AvgLoss float64 `json:"avgLoss"`

	// DurationMs is the learning duration.
	DurationMs int64 `json:"durationMs"`

	// Timestamp is when learning occurred.
	Timestamp time.Time `json:"timestamp"`
}
