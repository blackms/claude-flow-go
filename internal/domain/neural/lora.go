// Package neural provides domain types for neural learning systems.
package neural

import (
	"time"
)

// LoRARank represents standard LoRA ranks.
type LoRARank int

const (
	// LoRARank4 is a low-rank adaptation with rank 4.
	LoRARank4 LoRARank = 4
	// LoRARank8 is the default rank for LoRA.
	LoRARank8 LoRARank = 8
	// LoRARank16 is a higher-rank adaptation.
	LoRARank16 LoRARank = 16
	// LoRARank32 is a high-rank adaptation for complex tasks.
	LoRARank32 LoRARank = 32
	// LoRARank64 is the highest rank for maximum expressivity.
	LoRARank64 LoRARank = 64
)

// AdapterInjectionPoint specifies where to inject LoRA adapters.
type AdapterInjectionPoint string

const (
	// InjectionEmbedding injects adapter at embedding layer.
	InjectionEmbedding AdapterInjectionPoint = "embedding"
	// InjectionAttention injects adapter at attention layers.
	InjectionAttention AdapterInjectionPoint = "attention"
	// InjectionFeedForward injects adapter at feed-forward layers.
	InjectionFeedForward AdapterInjectionPoint = "feed_forward"
	// InjectionOutput injects adapter at output layer.
	InjectionOutput AdapterInjectionPoint = "output"
	// InjectionAll injects adapters at all supported points.
	InjectionAll AdapterInjectionPoint = "all"
)

// LoRAConfig configures the LoRA adapter.
type LoRAConfig struct {
	// Rank is the rank of the low-rank matrices (r << d).
	Rank LoRARank `json:"rank"`

	// Alpha is the scaling factor for the adapter output.
	// The effective scaling is alpha/rank.
	Alpha float64 `json:"alpha"`

	// Dropout is the dropout probability for regularization.
	Dropout float64 `json:"dropout"`

	// InjectionPoints specifies where to inject adapters.
	InjectionPoints []AdapterInjectionPoint `json:"injectionPoints"`

	// InputDimension is the input dimension for the adapter.
	InputDimension int `json:"inputDimension"`

	// OutputDimension is the output dimension for the adapter.
	OutputDimension int `json:"outputDimension"`

	// InitScale is the initialization scale for matrix B (A is zero-initialized).
	InitScale float64 `json:"initScale"`

	// EnableBias enables bias terms in the adapter.
	EnableBias bool `json:"enableBias"`
}

// DefaultLoRAConfig returns the default LoRA configuration.
func DefaultLoRAConfig() LoRAConfig {
	return LoRAConfig{
		Rank:            LoRARank8,
		Alpha:           16.0,
		Dropout:         0.05,
		InjectionPoints: []AdapterInjectionPoint{InjectionEmbedding},
		InputDimension:  256,
		OutputDimension: 256,
		InitScale:       0.01,
		EnableBias:      false,
	}
}

// ScalingFactor returns the effective scaling factor (alpha/rank).
func (c LoRAConfig) ScalingFactor() float64 {
	if c.Rank <= 0 {
		return 0
	}
	return c.Alpha / float64(c.Rank)
}

// LoRAAdapter represents a low-rank adapter.
// Implements W_delta = B @ A where W_delta is (output_dim x input_dim).
type LoRAAdapter struct {
	// ID is the unique identifier for this adapter.
	ID string `json:"id"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Config is the adapter configuration.
	Config LoRAConfig `json:"config"`

	// A is the down-projection matrix (rank x input_dim).
	// Initialized to small random values.
	A [][]float64 `json:"a"`

	// B is the up-projection matrix (output_dim x rank).
	// Initialized to zeros for stable training start.
	B [][]float64 `json:"b"`

	// BiasA is the optional bias for matrix A.
	BiasA []float64 `json:"biasA,omitempty"`

	// BiasB is the optional bias for matrix B.
	BiasB []float64 `json:"biasB,omitempty"`

	// InjectionPoint is where this adapter is injected.
	InjectionPoint AdapterInjectionPoint `json:"injectionPoint"`

	// Frozen indicates if the adapter is frozen (not trainable).
	Frozen bool `json:"frozen"`

	// CreatedAt is when the adapter was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the adapter was last updated.
	UpdatedAt time.Time `json:"updatedAt"`

	// TrainingSteps is the number of training steps applied.
	TrainingSteps int64 `json:"trainingSteps"`
}

// LoRAUpdate represents an update to apply to a LoRA adapter.
type LoRAUpdate struct {
	// AdapterID is the adapter to update.
	AdapterID string `json:"adapterId"`

	// GradA is the gradient for matrix A.
	GradA [][]float64 `json:"gradA"`

	// GradB is the gradient for matrix B.
	GradB [][]float64 `json:"gradB"`

	// LearningRate is the learning rate for this update.
	LearningRate float64 `json:"learningRate"`

	// Timestamp is when the update was created.
	Timestamp time.Time `json:"timestamp"`
}

// LoRAMergeConfig configures adapter merging.
type LoRAMergeConfig struct {
	// Weights are the merge weights for each adapter.
	Weights map[string]float64 `json:"weights"`

	// Strategy is the merge strategy.
	Strategy LoRAMergeStrategy `json:"strategy"`

	// Normalize normalizes weights to sum to 1.
	Normalize bool `json:"normalize"`
}

// LoRAMergeStrategy defines how to merge adapters.
type LoRAMergeStrategy string

const (
	// MergeStrategyAdd adds adapters with weights.
	MergeStrategyAdd LoRAMergeStrategy = "add"
	// MergeStrategySVD uses SVD for merging.
	MergeStrategySVD LoRAMergeStrategy = "svd"
	// MergeStrategyTies uses TIES merging.
	MergeStrategyTies LoRAMergeStrategy = "ties"
)

// LoRAStats contains statistics about a LoRA adapter.
type LoRAStats struct {
	// AdapterID is the adapter identifier.
	AdapterID string `json:"adapterId"`

	// ParameterCount is the total number of parameters.
	ParameterCount int64 `json:"parameterCount"`

	// TrainableParameters is the number of trainable parameters.
	TrainableParameters int64 `json:"trainableParameters"`

	// MemoryBytes is the memory usage in bytes.
	MemoryBytes int64 `json:"memoryBytes"`

	// CompressionRatio is the ratio vs full fine-tuning.
	CompressionRatio float64 `json:"compressionRatio"`

	// TrainingSteps is the number of training steps.
	TrainingSteps int64 `json:"trainingSteps"`

	// AvgUpdateTimeMs is the average update time in milliseconds.
	AvgUpdateTimeMs float64 `json:"avgUpdateTimeMs"`

	// NormA is the Frobenius norm of matrix A.
	NormA float64 `json:"normA"`

	// NormB is the Frobenius norm of matrix B.
	NormB float64 `json:"normB"`
}

// LoRAAdapterSet manages multiple adapters.
type LoRAAdapterSet struct {
	// ID is the set identifier.
	ID string `json:"id"`

	// Name is the set name.
	Name string `json:"name"`

	// Adapters is the map of adapters by ID.
	Adapters map[string]*LoRAAdapter `json:"adapters"`

	// ActiveAdapters lists currently active adapter IDs.
	ActiveAdapters []string `json:"activeAdapters"`

	// CreatedAt is when the set was created.
	CreatedAt time.Time `json:"createdAt"`

	// Description describes the adapter set.
	Description string `json:"description,omitempty"`
}

// LoRACheckpoint represents a saved adapter state.
type LoRACheckpoint struct {
	// ID is the checkpoint identifier.
	ID string `json:"id"`

	// AdapterID is the adapter this checkpoint belongs to.
	AdapterID string `json:"adapterId"`

	// Step is the training step at checkpoint.
	Step int64 `json:"step"`

	// A is the saved matrix A.
	A [][]float64 `json:"a"`

	// B is the saved matrix B.
	B [][]float64 `json:"b"`

	// Metrics are metrics at checkpoint time.
	Metrics map[string]float64 `json:"metrics,omitempty"`

	// CreatedAt is when the checkpoint was created.
	CreatedAt time.Time `json:"createdAt"`
}
