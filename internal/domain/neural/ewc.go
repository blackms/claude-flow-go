// Package neural provides domain types for neural learning systems.
package neural

import (
	"time"
)

// EWCVariant represents the EWC algorithm variant.
type EWCVariant string

const (
	// EWCVariantStandard is the original EWC algorithm.
	EWCVariantStandard EWCVariant = "standard"
	// EWCVariantOnline is EWC++ with online Fisher updates.
	EWCVariantOnline EWCVariant = "online"
	// EWCVariantProgressive is progressive neural networks style.
	EWCVariantProgressive EWCVariant = "progressive"
)

// EWCConfig configures the EWC++ algorithm.
type EWCConfig struct {
	// Variant is the EWC variant to use.
	Variant EWCVariant `json:"variant"`

	// Lambda is the regularization strength.
	// Higher values provide stronger protection against forgetting.
	Lambda float64 `json:"lambda"`

	// Gamma is the decay factor for online EWC (EWC++).
	// F_new = gamma * F_old + (1-gamma) * F_current
	Gamma float64 `json:"gamma"`

	// FisherSamples is the number of samples for Fisher estimation.
	FisherSamples int `json:"fisherSamples"`

	// UseDiagonalFisher uses diagonal approximation of Fisher matrix.
	UseDiagonalFisher bool `json:"useDiagonalFisher"`

	// NormalizeImportance normalizes importance weights.
	NormalizeImportance bool `json:"normalizeImportance"`

	// ClipImportance clips importance values to this maximum.
	ClipImportance float64 `json:"clipImportance"`

	// MinImportance is the minimum importance value.
	MinImportance float64 `json:"minImportance"`

	// EnableTaskHeads enables separate task-specific heads.
	EnableTaskHeads bool `json:"enableTaskHeads"`

	// ParameterDimension is the dimension of parameters to track.
	ParameterDimension int `json:"parameterDimension"`
}

// DefaultEWCConfig returns the default EWC++ configuration.
func DefaultEWCConfig() EWCConfig {
	return EWCConfig{
		Variant:             EWCVariantOnline,
		Lambda:              1000.0,
		Gamma:               0.9,
		FisherSamples:       100,
		UseDiagonalFisher:   true,
		NormalizeImportance: true,
		ClipImportance:      100.0,
		MinImportance:       1e-6,
		EnableTaskHeads:     true,
		ParameterDimension:  256,
	}
}

// ImportanceWeight represents the importance of a parameter.
type ImportanceWeight struct {
	// ParameterIndex is the index of the parameter.
	ParameterIndex int `json:"parameterIndex"`

	// Value is the importance value (Fisher diagonal).
	Value float64 `json:"value"`

	// TaskContributions tracks per-task contributions.
	TaskContributions map[string]float64 `json:"taskContributions,omitempty"`
}

// TaskHead represents a task-specific output head.
type TaskHead struct {
	// ID is the unique identifier for this head.
	ID string `json:"id"`

	// TaskID is the associated task.
	TaskID string `json:"taskId"`

	// Weights are the head weights.
	Weights [][]float64 `json:"weights"`

	// Bias is the head bias.
	Bias []float64 `json:"bias,omitempty"`

	// InputDimension is the input dimension.
	InputDimension int `json:"inputDimension"`

	// OutputDimension is the output dimension.
	OutputDimension int `json:"outputDimension"`

	// Frozen indicates if the head is frozen.
	Frozen bool `json:"frozen"`

	// CreatedAt is when the head was created.
	CreatedAt time.Time `json:"createdAt"`

	// Performance tracks head performance.
	Performance float64 `json:"performance"`
}

// EWCState represents the consolidated EWC state across tasks.
type EWCState struct {
	// ID is the state identifier.
	ID string `json:"id"`

	// Config is the EWC configuration.
	Config EWCConfig `json:"config"`

	// OldParameters are the parameters after previous tasks.
	// theta_old in the EWC loss.
	OldParameters []float64 `json:"oldParameters"`

	// FisherDiagonal is the diagonal Fisher information.
	// Accumulated across all tasks with online updates.
	FisherDiagonal []float64 `json:"fisherDiagonal"`

	// TaskCount is the number of tasks consolidated.
	TaskCount int `json:"taskCount"`

	// TaskIDs lists consolidated task IDs.
	TaskIDs []string `json:"taskIds"`

	// TaskHeads maps task IDs to their heads.
	TaskHeads map[string]*TaskHead `json:"taskHeads,omitempty"`

	// TotalRegularizationLoss tracks cumulative regularization.
	TotalRegularizationLoss float64 `json:"totalRegularizationLoss"`

	// LastConsolidation is when the last consolidation occurred.
	LastConsolidation time.Time `json:"lastConsolidation"`

	// CreatedAt is when the state was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the state was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// EWCTask represents a task for continual learning.
type EWCTask struct {
	// ID is the unique task identifier.
	ID string `json:"id"`

	// Name is the task name.
	Name string `json:"name"`

	// Description describes the task.
	Description string `json:"description,omitempty"`

	// DataSize is the number of samples in the task.
	DataSize int `json:"dataSize"`

	// Parameters are the learned parameters for this task.
	Parameters []float64 `json:"parameters"`

	// FisherDiagonal is the Fisher information for this task.
	FisherDiagonal []float64 `json:"fisherDiagonal"`

	// Performance is the task performance metric.
	Performance float64 `json:"performance"`

	// CompletedAt is when the task was completed.
	CompletedAt time.Time `json:"completedAt"`

	// IsConsolidated indicates if the task is consolidated.
	IsConsolidated bool `json:"isConsolidated"`
}

// EWCLoss represents the components of the EWC loss.
type EWCLoss struct {
	// TaskLoss is the current task loss.
	TaskLoss float64 `json:"taskLoss"`

	// RegularizationLoss is the EWC regularization loss.
	// L_ewc = (lambda/2) * sum_i(F_i * (theta_i - theta_old_i)^2)
	RegularizationLoss float64 `json:"regularizationLoss"`

	// TotalLoss is the combined loss.
	TotalLoss float64 `json:"totalLoss"`

	// ParameterDrift is the drift from old parameters.
	ParameterDrift float64 `json:"parameterDrift"`

	// MaxDrift is the maximum single-parameter drift.
	MaxDrift float64 `json:"maxDrift"`
}

// EWCStats contains statistics about EWC.
type EWCStats struct {
	// StateID is the state identifier.
	StateID string `json:"stateId"`

	// TaskCount is the number of tasks.
	TaskCount int `json:"taskCount"`

	// ParameterCount is the number of parameters tracked.
	ParameterCount int `json:"parameterCount"`

	// AvgImportance is the average importance weight.
	AvgImportance float64 `json:"avgImportance"`

	// MaxImportance is the maximum importance weight.
	MaxImportance float64 `json:"maxImportance"`

	// TotalRegularization is the total regularization applied.
	TotalRegularization float64 `json:"totalRegularization"`

	// AvgRegTimeMs is the average regularization computation time.
	AvgRegTimeMs float64 `json:"avgRegTimeMs"`

	// MemoryBytes is the memory usage.
	MemoryBytes int64 `json:"memoryBytes"`

	// TaskPerformances maps tasks to their performance.
	TaskPerformances map[string]float64 `json:"taskPerformances,omitempty"`
}

// EWCCheckpoint represents a saved EWC state.
type EWCCheckpoint struct {
	// ID is the checkpoint identifier.
	ID string `json:"id"`

	// StateID is the state this checkpoint belongs to.
	StateID string `json:"stateId"`

	// TaskCount is the task count at checkpoint.
	TaskCount int `json:"taskCount"`

	// OldParameters are the saved parameters.
	OldParameters []float64 `json:"oldParameters"`

	// FisherDiagonal is the saved Fisher diagonal.
	FisherDiagonal []float64 `json:"fisherDiagonal"`

	// Metrics are metrics at checkpoint time.
	Metrics map[string]float64 `json:"metrics,omitempty"`

	// CreatedAt is when the checkpoint was created.
	CreatedAt time.Time `json:"createdAt"`
}

// ContinualLearningConfig combines LoRA and EWC settings.
type ContinualLearningConfig struct {
	// LoRA is the LoRA configuration.
	LoRA LoRAConfig `json:"lora"`

	// EWC is the EWC configuration.
	EWC EWCConfig `json:"ewc"`

	// EnableLoRA enables LoRA adapters.
	EnableLoRA bool `json:"enableLoRA"`

	// EnableEWC enables EWC regularization.
	EnableEWC bool `json:"enableEWC"`

	// TaskBoundaryThreshold is the threshold for detecting task boundaries.
	TaskBoundaryThreshold float64 `json:"taskBoundaryThreshold"`

	// AutoConsolidate automatically consolidates after each task.
	AutoConsolidate bool `json:"autoConsolidate"`
}

// DefaultContinualLearningConfig returns the default continual learning config.
func DefaultContinualLearningConfig() ContinualLearningConfig {
	return ContinualLearningConfig{
		LoRA:                  DefaultLoRAConfig(),
		EWC:                   DefaultEWCConfig(),
		EnableLoRA:            true,
		EnableEWC:             true,
		TaskBoundaryThreshold: 0.3,
		AutoConsolidate:       true,
	}
}
