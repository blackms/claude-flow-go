// Package neural provides domain types for neural learning systems.
package neural

import (
	"time"
)

// TrajectoryDomain represents the domain classification of a trajectory.
type TrajectoryDomain string

const (
	// DomainCode is for code-related trajectories.
	DomainCode TrajectoryDomain = "code"
	// DomainCreative is for creative tasks.
	DomainCreative TrajectoryDomain = "creative"
	// DomainReasoning is for logical reasoning tasks.
	DomainReasoning TrajectoryDomain = "reasoning"
	// DomainChat is for conversational tasks.
	DomainChat TrajectoryDomain = "chat"
	// DomainMath is for mathematical tasks.
	DomainMath TrajectoryDomain = "math"
	// DomainGeneral is the default domain.
	DomainGeneral TrajectoryDomain = "general"
)

// ExtendedTrajectoryStep represents a detailed step in a learning trajectory.
type ExtendedTrajectoryStep struct {
	// StepID is the unique identifier for this step.
	StepID string `json:"stepId"`

	// Timestamp is when the step occurred.
	Timestamp time.Time `json:"timestamp"`

	// Action is the action taken at this step.
	Action string `json:"action"`

	// StateBefore is the embedding of state before action.
	StateBefore []float32 `json:"stateBefore,omitempty"`

	// StateAfter is the embedding of state after action.
	StateAfter []float32 `json:"stateAfter,omitempty"`

	// Reward is the reward signal for this step (0-1).
	Reward float64 `json:"reward"`

	// AttentionWeights from model (optional).
	AttentionWeights []float32 `json:"attentionWeights,omitempty"`

	// LayerActivations are optional layer activations.
	LayerActivations [][]float32 `json:"layerActivations,omitempty"`

	// Context is additional context at this step.
	Context map[string]interface{} `json:"context,omitempty"`

	// Outcome describes what happened after the action.
	Outcome string `json:"outcome,omitempty"`
}

// ExtendedTrajectory represents a complete learning trajectory with full details.
type ExtendedTrajectory struct {
	// TrajectoryID is the unique identifier.
	TrajectoryID string `json:"trajectoryId"`

	// Context is the task context/description.
	Context string `json:"context"`

	// Domain is the domain classification.
	Domain TrajectoryDomain `json:"domain"`

	// Steps is the sequence of steps.
	Steps []ExtendedTrajectoryStep `json:"steps"`

	// QualityScore is the overall quality (0-1).
	QualityScore float64 `json:"qualityScore"`

	// IsComplete indicates if the trajectory is complete.
	IsComplete bool `json:"isComplete"`

	// StartTime is when the trajectory started.
	StartTime time.Time `json:"startTime"`

	// EndTime is when the trajectory ended (if complete).
	EndTime *time.Time `json:"endTime,omitempty"`

	// Verdict is the judgment of this trajectory.
	Verdict *TrajectoryVerdict `json:"verdict,omitempty"`

	// DistilledMemory is the extracted memory (if processed).
	DistilledMemory *DistilledMemory `json:"distilledMemory,omitempty"`

	// AgentID is the agent that created this trajectory.
	AgentID string `json:"agentId,omitempty"`

	// TotalReward is the sum of all step rewards.
	TotalReward float64 `json:"totalReward"`

	// Duration is the trajectory duration in milliseconds.
	DurationMs int64 `json:"durationMs"`
}

// TrajectoryVerdict represents the judgment of a trajectory.
type TrajectoryVerdict struct {
	// Success indicates whether the trajectory was successful.
	Success bool `json:"success"`

	// Confidence is the confidence in the verdict (0-1).
	Confidence float64 `json:"confidence"`

	// Strengths are identified strengths.
	Strengths []string `json:"strengths"`

	// Weaknesses are identified weaknesses.
	Weaknesses []string `json:"weaknesses"`

	// Improvements are suggested improvements.
	Improvements []string `json:"improvements"`

	// RelevanceScore is for similar task relevance (0-1).
	RelevanceScore float64 `json:"relevanceScore"`

	// JudgedAt is when the verdict was made.
	JudgedAt time.Time `json:"judgedAt"`
}

// DistilledMemory represents extracted memory from a trajectory.
type DistilledMemory struct {
	// MemoryID is the unique identifier.
	MemoryID string `json:"memoryId"`

	// TrajectoryID is the source trajectory.
	TrajectoryID string `json:"trajectoryId"`

	// Strategy is the extracted strategy pattern.
	Strategy string `json:"strategy"`

	// KeyLearnings are the key learnings extracted.
	KeyLearnings []string `json:"keyLearnings"`

	// Embedding is the embedding for similarity search.
	Embedding []float32 `json:"embedding"`

	// Quality is the quality score (0-1).
	Quality float64 `json:"quality"`

	// UsageCount tracks how often this memory was used.
	UsageCount int `json:"usageCount"`

	// LastUsed is when the memory was last used.
	LastUsed time.Time `json:"lastUsed"`

	// CreatedAt is when the memory was created.
	CreatedAt time.Time `json:"createdAt"`
}

// RewardConfig configures reward calculation.
type RewardConfig struct {
	// QualityWeight is the weight for quality score.
	QualityWeight float64 `json:"qualityWeight"`

	// TimeWeight is the weight for time efficiency.
	TimeWeight float64 `json:"timeWeight"`

	// ResourcePenalty is the penalty factor for resource usage.
	ResourcePenalty float64 `json:"resourcePenalty"`

	// SuccessBonus is the bonus for successful completion.
	SuccessBonus float64 `json:"successBonus"`

	// FailurePenalty is the penalty for failure.
	FailurePenalty float64 `json:"failurePenalty"`

	// StepPenalty is a small penalty per step to encourage efficiency.
	StepPenalty float64 `json:"stepPenalty"`

	// MinReward is the minimum reward value.
	MinReward float64 `json:"minReward"`

	// MaxReward is the maximum reward value.
	MaxReward float64 `json:"maxReward"`
}

// DefaultRewardConfig returns the default reward configuration.
func DefaultRewardConfig() RewardConfig {
	return RewardConfig{
		QualityWeight:   0.4,
		TimeWeight:      0.2,
		ResourcePenalty: 0.1,
		SuccessBonus:    0.3,
		FailurePenalty:  0.2,
		StepPenalty:     0.01,
		MinReward:       0.0,
		MaxReward:       1.0,
	}
}

// RewardComponents holds the individual reward components.
type RewardComponents struct {
	// QualityScore is the quality contribution.
	QualityScore float64 `json:"qualityScore"`

	// TimeEfficiency is the time efficiency contribution.
	TimeEfficiency float64 `json:"timeEfficiency"`

	// ResourceUsage is the resource usage penalty.
	ResourceUsage float64 `json:"resourceUsage"`

	// SuccessBonus is the success bonus.
	SuccessBonus float64 `json:"successBonus"`

	// StepPenalty is the total step penalty.
	StepPenalty float64 `json:"stepPenalty"`

	// TotalReward is the final computed reward.
	TotalReward float64 `json:"totalReward"`
}

// TrajectoryStats contains statistics about trajectories.
type TrajectoryStats struct {
	// TotalTrajectories is the total count.
	TotalTrajectories int `json:"totalTrajectories"`

	// ActiveTrajectories are currently recording.
	ActiveTrajectories int `json:"activeTrajectories"`

	// CompletedTrajectories are finished.
	CompletedTrajectories int `json:"completedTrajectories"`

	// SuccessfulTrajectories passed judgment.
	SuccessfulTrajectories int `json:"successfulTrajectories"`

	// AvgQualityScore is the average quality.
	AvgQualityScore float64 `json:"avgQualityScore"`

	// AvgStepCount is the average steps per trajectory.
	AvgStepCount float64 `json:"avgStepCount"`

	// AvgDurationMs is the average duration.
	AvgDurationMs float64 `json:"avgDurationMs"`

	// DistilledMemories is the count of distilled memories.
	DistilledMemories int `json:"distilledMemories"`

	// LearningCycles is the number of learning updates.
	LearningCycles int64 `json:"learningCycles"`
}

// TrajectoryQuery represents a query for trajectories.
type TrajectoryQuery struct {
	// Domain filters by domain.
	Domain *TrajectoryDomain `json:"domain,omitempty"`

	// AgentID filters by agent.
	AgentID string `json:"agentId,omitempty"`

	// MinQuality filters by minimum quality.
	MinQuality float64 `json:"minQuality,omitempty"`

	// SuccessOnly returns only successful trajectories.
	SuccessOnly bool `json:"successOnly,omitempty"`

	// StartAfter filters trajectories after this time.
	StartAfter *time.Time `json:"startAfter,omitempty"`

	// StartBefore filters trajectories before this time.
	StartBefore *time.Time `json:"startBefore,omitempty"`

	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`

	// Offset for pagination.
	Offset int `json:"offset,omitempty"`

	// OrderBy specifies sort order.
	OrderBy string `json:"orderBy,omitempty"`

	// Descending reverses sort order.
	Descending bool `json:"descending,omitempty"`
}

// TrajectorySearchResult represents a similarity search result.
type TrajectorySearchResult struct {
	// Trajectory is the matched trajectory.
	Trajectory *ExtendedTrajectory `json:"trajectory"`

	// Similarity is the similarity score (0-1).
	Similarity float64 `json:"similarity"`

	// LatencyMs is the search latency.
	LatencyMs float64 `json:"latencyMs"`
}

// DecisionTransformerConfig configures the Decision Transformer.
type DecisionTransformerConfig struct {
	// ContextLength is the maximum context length.
	ContextLength int `json:"contextLength"`

	// NumHeads is the number of attention heads.
	NumHeads int `json:"numHeads"`

	// NumLayers is the number of transformer layers.
	NumLayers int `json:"numLayers"`

	// HiddenDim is the hidden dimension.
	HiddenDim int `json:"hiddenDim"`

	// EmbeddingDim is the embedding dimension.
	EmbeddingDim int `json:"embeddingDim"`

	// Dropout is the dropout rate.
	Dropout float64 `json:"dropout"`

	// LearningRate for training.
	LearningRate float64 `json:"learningRate"`

	// Gamma is the discount factor.
	Gamma float64 `json:"gamma"`

	// MiniBatchSize for training.
	MiniBatchSize int `json:"miniBatchSize"`

	// MaxBufferSize for trajectory buffer.
	MaxBufferSize int `json:"maxBufferSize"`

	// NumActions is the action space size.
	NumActions int `json:"numActions"`

	// StateDim is the state embedding dimension.
	StateDim int `json:"stateDim"`
}

// DefaultDecisionTransformerConfig returns the default configuration.
func DefaultDecisionTransformerConfig() DecisionTransformerConfig {
	return DecisionTransformerConfig{
		ContextLength: 20,
		NumHeads:      4,
		NumLayers:     2,
		HiddenDim:     64,
		EmbeddingDim:  32,
		Dropout:       0.1,
		LearningRate:  0.0001,
		Gamma:         0.99,
		MiniBatchSize: 64,
		MaxBufferSize: 1000,
		NumActions:    16,
		StateDim:      256,
	}
}

// SequenceEntry represents an entry in the transformer sequence.
type SequenceEntry struct {
	// ReturnToGo is the expected return from this point.
	ReturnToGo float64 `json:"returnToGo"`

	// State is the state embedding.
	State []float32 `json:"state"`

	// Action is the action index.
	Action int `json:"action"`

	// Timestep is the position in sequence.
	Timestep int `json:"timestep"`
}

// TrainingResult represents the result of a training step.
type TrainingResult struct {
	// Loss is the training loss.
	Loss float64 `json:"loss"`

	// Accuracy is the action prediction accuracy.
	Accuracy float64 `json:"accuracy"`

	// UpdateCount is the number of updates performed.
	UpdateCount int64 `json:"updateCount"`

	// LatencyMs is the training latency.
	LatencyMs float64 `json:"latencyMs"`
}

// PatternExtraction represents an extracted pattern from trajectories.
type PatternExtraction struct {
	// PatternID is the unique identifier.
	PatternID string `json:"patternId"`

	// SourceTrajectoryIDs are the source trajectories.
	SourceTrajectoryIDs []string `json:"sourceTrajectoryIds"`

	// Pattern describes the extracted pattern.
	Pattern string `json:"pattern"`

	// Frequency is how often this pattern appears.
	Frequency int `json:"frequency"`

	// AvgQuality is the average quality when this pattern is used.
	AvgQuality float64 `json:"avgQuality"`

	// Embedding is the pattern embedding.
	Embedding []float32 `json:"embedding"`

	// CreatedAt is when the pattern was extracted.
	CreatedAt time.Time `json:"createdAt"`
}
