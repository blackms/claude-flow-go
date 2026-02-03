// Package neural provides neural learning application services.
package neural

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
)

// TrajectoryLearner performs learning from trajectories.
type TrajectoryLearner struct {
	mu          sync.RWMutex
	transformer *infraNeural.DecisionTransformer
	generator   *infraNeural.EmbeddingGenerator
	config      TrajectoryLearnerConfig
	stats       *TrajectoryLearnerStats
}

// TrajectoryLearnerConfig configures the trajectory learner.
type TrajectoryLearnerConfig struct {
	// Reward configuration
	Reward domainNeural.RewardConfig `json:"reward"`

	// DecisionTransformer configuration
	Transformer domainNeural.DecisionTransformerConfig `json:"transformer"`

	// MinTrajectoriesForLearning is the minimum trajectories before learning.
	MinTrajectoriesForLearning int `json:"minTrajectoriesForLearning"`

	// LearningBatchSize is the batch size for learning updates.
	LearningBatchSize int `json:"learningBatchSize"`

	// PatternExtractionThreshold is the quality threshold for pattern extraction.
	PatternExtractionThreshold float64 `json:"patternExtractionThreshold"`

	// MemoryConsolidationThreshold is the quality threshold for memory consolidation.
	MemoryConsolidationThreshold float64 `json:"memoryConsolidationThreshold"`

	// EmbeddingDimension for pattern embeddings.
	EmbeddingDimension int `json:"embeddingDimension"`
}

// DefaultTrajectoryLearnerConfig returns the default configuration.
func DefaultTrajectoryLearnerConfig() TrajectoryLearnerConfig {
	return TrajectoryLearnerConfig{
		Reward:                       domainNeural.DefaultRewardConfig(),
		Transformer:                  domainNeural.DefaultDecisionTransformerConfig(),
		MinTrajectoriesForLearning:   10,
		LearningBatchSize:            32,
		PatternExtractionThreshold:   0.7,
		MemoryConsolidationThreshold: 0.8,
		EmbeddingDimension:           256,
	}
}

// TrajectoryLearnerStats contains learner statistics.
type TrajectoryLearnerStats struct {
	TotalLearningCycles   int64   `json:"totalLearningCycles"`
	TotalPatternsExtracted int64  `json:"totalPatternsExtracted"`
	TotalMemoriesDistilled int64  `json:"totalMemoriesDistilled"`
	AvgLearningTimeMs      float64 `json:"avgLearningTimeMs"`
	AvgRewardCalcTimeMs    float64 `json:"avgRewardCalcTimeMs"`
	LastLearningTime       time.Time `json:"lastLearningTime"`
	AvgLoss                float64 `json:"avgLoss"`
	AvgAccuracy            float64 `json:"avgAccuracy"`
}

// NewTrajectoryLearner creates a new trajectory learner.
func NewTrajectoryLearner(config TrajectoryLearnerConfig) *TrajectoryLearner {
	return &TrajectoryLearner{
		transformer: infraNeural.NewDecisionTransformer(config.Transformer),
		generator:   infraNeural.NewEmbeddingGenerator(config.EmbeddingDimension),
		config:      config,
		stats:       &TrajectoryLearnerStats{},
	}
}

// ComputeReward computes the reward for a trajectory.
// Target: <10ms
func (l *TrajectoryLearner) ComputeReward(ctx context.Context, traj *domainNeural.ExtendedTrajectory) *domainNeural.RewardComponents {
	startTime := time.Now()
	defer func() {
		elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
		l.mu.Lock()
		l.stats.AvgRewardCalcTimeMs = (l.stats.AvgRewardCalcTimeMs*float64(l.stats.TotalLearningCycles) + elapsed) / float64(l.stats.TotalLearningCycles+1)
		l.mu.Unlock()
	}()

	cfg := l.config.Reward
	components := &domainNeural.RewardComponents{}

	// Quality score component
	components.QualityScore = cfg.QualityWeight * traj.QualityScore

	// Time efficiency component
	// Faster completion = higher reward (normalize to 0-1)
	if traj.DurationMs > 0 {
		// Assume 60 seconds is a reasonable max duration
		maxDurationMs := int64(60000)
		timeRatio := 1.0 - (float64(traj.DurationMs) / float64(maxDurationMs))
		if timeRatio < 0 {
			timeRatio = 0
		}
		components.TimeEfficiency = cfg.TimeWeight * timeRatio
	}

	// Resource usage (steps as proxy for resource usage)
	// Fewer steps = less penalty
	if len(traj.Steps) > 0 {
		stepPenalty := float64(len(traj.Steps)) * cfg.StepPenalty
		if stepPenalty > cfg.ResourcePenalty {
			stepPenalty = cfg.ResourcePenalty
		}
		components.ResourceUsage = stepPenalty
		components.StepPenalty = stepPenalty
	}

	// Success bonus
	if traj.Verdict != nil && traj.Verdict.Success {
		components.SuccessBonus = cfg.SuccessBonus
	} else if traj.Verdict != nil && !traj.Verdict.Success {
		components.SuccessBonus = -cfg.FailurePenalty
	}

	// Calculate total reward
	total := components.QualityScore + components.TimeEfficiency - components.ResourceUsage + components.SuccessBonus

	// Clamp to min/max
	if total < cfg.MinReward {
		total = cfg.MinReward
	}
	if total > cfg.MaxReward {
		total = cfg.MaxReward
	}

	components.TotalReward = total

	return components
}

// UpdatePolicy trains the Decision Transformer on trajectories.
// Target: <50ms per batch
func (l *TrajectoryLearner) UpdatePolicy(ctx context.Context, trajectories []*domainNeural.ExtendedTrajectory) (*domainNeural.TrainingResult, error) {
	startTime := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(trajectories) < l.config.MinTrajectoriesForLearning {
		return nil, nil
	}

	// Add trajectories to transformer buffer
	for _, traj := range trajectories {
		if traj.IsComplete && len(traj.Steps) > 0 {
			// Compute step rewards if not set
			if traj.TotalReward == 0 {
				reward := l.ComputeReward(ctx, traj)
				// Distribute reward across steps
				perStepReward := reward.TotalReward / float64(len(traj.Steps))
				for i := range traj.Steps {
					traj.Steps[i].Reward = perStepReward
				}
				traj.TotalReward = reward.TotalReward
			}

			l.transformer.AddTrajectory(traj)
		}
	}

	// Train the transformer
	result := l.transformer.Train()

	l.stats.TotalLearningCycles++
	l.stats.LastLearningTime = time.Now()
	l.stats.AvgLoss = result.Loss
	l.stats.AvgAccuracy = result.Accuracy

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	l.stats.AvgLearningTimeMs = (l.stats.AvgLearningTimeMs*float64(l.stats.TotalLearningCycles-1) + elapsed) / float64(l.stats.TotalLearningCycles)

	return &result, nil
}

// ExtractPatterns extracts patterns from successful trajectories.
func (l *TrajectoryLearner) ExtractPatterns(ctx context.Context, trajectories []*domainNeural.ExtendedTrajectory) ([]*domainNeural.PatternExtraction, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	patterns := make([]*domainNeural.PatternExtraction, 0)

	// Group trajectories by common action sequences
	actionSequences := make(map[string][]*domainNeural.ExtendedTrajectory)

	for _, traj := range trajectories {
		if traj.QualityScore < l.config.PatternExtractionThreshold {
			continue
		}

		// Extract action sequence as pattern key
		var key string
		for i, step := range traj.Steps {
			if i > 0 {
				key += "->"
			}
			key += step.Action
		}

		actionSequences[key] = append(actionSequences[key], traj)
	}

	// Create patterns from frequent sequences
	for key, trajs := range actionSequences {
		if len(trajs) < 2 {
			continue
		}

		// Calculate average quality
		var avgQuality float64
		sourceIDs := make([]string, len(trajs))
		for i, t := range trajs {
			avgQuality += t.QualityScore
			sourceIDs[i] = t.TrajectoryID
		}
		avgQuality /= float64(len(trajs))

		// Generate embedding for the pattern
		embedding := l.generator.Generate(key)

		pattern := &domainNeural.PatternExtraction{
			PatternID:           uuid.New().String(),
			SourceTrajectoryIDs: sourceIDs,
			Pattern:             key,
			Frequency:           len(trajs),
			AvgQuality:          avgQuality,
			Embedding:           embedding,
			CreatedAt:           time.Now(),
		}

		patterns = append(patterns, pattern)
		l.stats.TotalPatternsExtracted++
	}

	return patterns, nil
}

// ConsolidateMemory creates distilled memories from high-quality trajectories.
// Target: <50ms
func (l *TrajectoryLearner) ConsolidateMemory(ctx context.Context, trajectories []*domainNeural.ExtendedTrajectory) ([]*domainNeural.DistilledMemory, error) {
	startTime := time.Now()
	defer func() {
		elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
		_ = elapsed // Track if needed
	}()

	l.mu.Lock()
	defer l.mu.Unlock()

	memories := make([]*domainNeural.DistilledMemory, 0)

	for _, traj := range trajectories {
		if traj.QualityScore < l.config.MemoryConsolidationThreshold {
			continue
		}

		if !traj.IsComplete {
			continue
		}

		// Extract strategy from action sequence
		var strategy string
		for i, step := range traj.Steps {
			if i > 0 {
				strategy += " -> "
			}
			strategy += step.Action
		}

		// Extract key learnings
		keyLearnings := make([]string, 0)
		if traj.Verdict != nil {
			keyLearnings = append(keyLearnings, traj.Verdict.Strengths...)
		}

		// Add context as a learning if available
		if traj.Context != "" {
			keyLearnings = append(keyLearnings, "Context: "+traj.Context)
		}

		// Generate embedding
		embedding := l.generator.Generate(strategy + " " + traj.Context)

		memory := &domainNeural.DistilledMemory{
			MemoryID:     uuid.New().String(),
			TrajectoryID: traj.TrajectoryID,
			Strategy:     strategy,
			KeyLearnings: keyLearnings,
			Embedding:    embedding,
			Quality:      traj.QualityScore,
			UsageCount:   0,
			LastUsed:     time.Now(),
			CreatedAt:    time.Now(),
		}

		memories = append(memories, memory)
		l.stats.TotalMemoriesDistilled++
	}

	return memories, nil
}

// GetPredictedAction returns the predicted action for a given state.
func (l *TrajectoryLearner) GetPredictedAction(ctx context.Context, states [][]float32, actions []int, targetReturn float64) int {
	return l.transformer.GetAction(states, actions, targetReturn)
}

// LearnFromTrajectory performs a complete learning cycle on a single trajectory.
func (l *TrajectoryLearner) LearnFromTrajectory(ctx context.Context, traj *domainNeural.ExtendedTrajectory) (*domainNeural.RewardComponents, error) {
	if !traj.IsComplete {
		return nil, nil
	}

	// Compute reward
	reward := l.ComputeReward(ctx, traj)

	// Distribute reward across steps
	if len(traj.Steps) > 0 {
		perStepReward := reward.TotalReward / float64(len(traj.Steps))
		for i := range traj.Steps {
			traj.Steps[i].Reward = perStepReward
		}
	}

	// Add to transformer buffer
	l.transformer.AddTrajectory(traj)

	return reward, nil
}

// TriggerTraining triggers a training cycle.
func (l *TrajectoryLearner) TriggerTraining(ctx context.Context) *domainNeural.TrainingResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	result := l.transformer.Train()
	l.stats.TotalLearningCycles++
	l.stats.LastLearningTime = time.Now()
	l.stats.AvgLoss = result.Loss
	l.stats.AvgAccuracy = result.Accuracy

	return &result
}

// GetStats returns learner statistics.
func (l *TrajectoryLearner) GetStats() *TrajectoryLearnerStats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.stats
}

// GetTransformerStats returns Decision Transformer statistics.
func (l *TrajectoryLearner) GetTransformerStats() map[string]interface{} {
	return l.transformer.GetStats()
}

// GetBufferSize returns the trajectory buffer size.
func (l *TrajectoryLearner) GetBufferSize() int {
	return l.transformer.GetBufferSize()
}

// ClearBuffer clears the trajectory buffer.
func (l *TrajectoryLearner) ClearBuffer() {
	l.transformer.ClearBuffer()
}

// ComputeReturnToGo computes returns-to-go for a trajectory.
func (l *TrajectoryLearner) ComputeReturnToGo(traj *domainNeural.ExtendedTrajectory) []float64 {
	if len(traj.Steps) == 0 {
		return nil
	}

	gamma := l.config.Transformer.Gamma
	returnsToGo := make([]float64, len(traj.Steps))
	var cumReturn float64

	for t := len(traj.Steps) - 1; t >= 0; t-- {
		cumReturn = traj.Steps[t].Reward + gamma*cumReturn
		returnsToGo[t] = cumReturn
	}

	return returnsToGo
}

// EvaluateTrajectory evaluates a trajectory's quality.
func (l *TrajectoryLearner) EvaluateTrajectory(ctx context.Context, traj *domainNeural.ExtendedTrajectory) float64 {
	if len(traj.Steps) == 0 {
		return 0
	}

	// Factors: step rewards, outcome success, efficiency
	var totalReward float64
	for _, step := range traj.Steps {
		totalReward += step.Reward
	}

	// Normalize by number of steps
	avgStepReward := totalReward / float64(len(traj.Steps))

	// Success bonus
	successFactor := 0.0
	if traj.Verdict != nil && traj.Verdict.Success {
		successFactor = 0.3
	}

	// Efficiency (fewer steps is better, normalized)
	efficiencyFactor := 1.0 / (1.0 + math.Log(float64(len(traj.Steps)+1)))

	quality := 0.4*avgStepReward + 0.3*successFactor + 0.3*efficiencyFactor

	// Clamp to 0-1
	if quality < 0 {
		quality = 0
	}
	if quality > 1 {
		quality = 1
	}

	return quality
}
