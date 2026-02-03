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

// TrajectoryRecorder records agent execution trajectories.
type TrajectoryRecorder struct {
	mu           sync.RWMutex
	generator    *infraNeural.EmbeddingGenerator
	active       map[string]*domainNeural.ExtendedTrajectory
	config       TrajectoryRecorderConfig
	stats        *TrajectoryRecorderStats
}

// TrajectoryRecorderConfig configures the recorder.
type TrajectoryRecorderConfig struct {
	// EmbeddingDimension is the dimension for state embeddings.
	EmbeddingDimension int `json:"embeddingDimension"`

	// MaxActiveTrajectories is the maximum concurrent recordings.
	MaxActiveTrajectories int `json:"maxActiveTrajectories"`

	// MaxStepsPerTrajectory limits steps in a trajectory.
	MaxStepsPerTrajectory int `json:"maxStepsPerTrajectory"`

	// AutoComputeEmbeddings automatically computes state embeddings.
	AutoComputeEmbeddings bool `json:"autoComputeEmbeddings"`

	// RecordAttentionWeights records attention weights.
	RecordAttentionWeights bool `json:"recordAttentionWeights"`
}

// DefaultTrajectoryRecorderConfig returns the default configuration.
func DefaultTrajectoryRecorderConfig() TrajectoryRecorderConfig {
	return TrajectoryRecorderConfig{
		EmbeddingDimension:     256,
		MaxActiveTrajectories:  100,
		MaxStepsPerTrajectory:  1000,
		AutoComputeEmbeddings:  true,
		RecordAttentionWeights: false,
	}
}

// TrajectoryRecorderStats contains recorder statistics.
type TrajectoryRecorderStats struct {
	ActiveCount    int     `json:"activeCount"`
	TotalRecorded  int64   `json:"totalRecorded"`
	TotalSteps     int64   `json:"totalSteps"`
	AvgStepTimeMs  float64 `json:"avgStepTimeMs"`
	DroppedCount   int64   `json:"droppedCount"`
}

// NewTrajectoryRecorder creates a new trajectory recorder.
func NewTrajectoryRecorder(config TrajectoryRecorderConfig) *TrajectoryRecorder {
	return &TrajectoryRecorder{
		generator: infraNeural.NewEmbeddingGenerator(config.EmbeddingDimension),
		active:    make(map[string]*domainNeural.ExtendedTrajectory),
		config:    config,
		stats:     &TrajectoryRecorderStats{},
	}
}

// StartRecording begins recording a new trajectory.
func (r *TrajectoryRecorder) StartRecording(ctx context.Context, trajectoryContext string, domain domainNeural.TrajectoryDomain, agentID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.active) >= r.config.MaxActiveTrajectories {
		r.stats.DroppedCount++
		return "", fmt.Errorf("maximum active trajectories reached: %d", r.config.MaxActiveTrajectories)
	}

	trajectoryID := uuid.New().String()
	now := time.Now()

	traj := &domainNeural.ExtendedTrajectory{
		TrajectoryID: trajectoryID,
		Context:      trajectoryContext,
		Domain:       domain,
		Steps:        make([]domainNeural.ExtendedTrajectoryStep, 0),
		QualityScore: 0,
		IsComplete:   false,
		StartTime:    now,
		AgentID:      agentID,
	}

	r.active[trajectoryID] = traj
	r.stats.ActiveCount = len(r.active)

	return trajectoryID, nil
}

// RecordStep records a step in a trajectory.
// Target: <5ms per step.
func (r *TrajectoryRecorder) RecordStep(ctx context.Context, trajectoryID string, action string, contextData map[string]interface{}, outcome string) error {
	startTime := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	traj, exists := r.active[trajectoryID]
	if !exists {
		return fmt.Errorf("trajectory not found: %s", trajectoryID)
	}

	if len(traj.Steps) >= r.config.MaxStepsPerTrajectory {
		return fmt.Errorf("maximum steps reached for trajectory: %s", trajectoryID)
	}

	stepID := uuid.New().String()

	step := domainNeural.ExtendedTrajectoryStep{
		StepID:    stepID,
		Timestamp: time.Now(),
		Action:    action,
		Context:   contextData,
		Outcome:   outcome,
	}

	// Compute embeddings if enabled
	if r.config.AutoComputeEmbeddings {
		// State before: embed the action + context
		beforeContent := action
		if len(contextData) > 0 {
			beforeContent = fmt.Sprintf("%s with context", action)
		}
		step.StateBefore = r.generator.Generate(beforeContent)

		// State after: embed the outcome
		if outcome != "" {
			step.StateAfter = r.generator.Generate(outcome)
		}
	}

	traj.Steps = append(traj.Steps, step)
	r.stats.TotalSteps++

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	r.stats.AvgStepTimeMs = (r.stats.AvgStepTimeMs*float64(r.stats.TotalSteps-1) + elapsed) / float64(r.stats.TotalSteps)

	return nil
}

// RecordStepWithReward records a step with an explicit reward.
func (r *TrajectoryRecorder) RecordStepWithReward(ctx context.Context, trajectoryID string, action string, reward float64, stateBefore, stateAfter []float32) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	traj, exists := r.active[trajectoryID]
	if !exists {
		return fmt.Errorf("trajectory not found: %s", trajectoryID)
	}

	if len(traj.Steps) >= r.config.MaxStepsPerTrajectory {
		return fmt.Errorf("maximum steps reached for trajectory: %s", trajectoryID)
	}

	step := domainNeural.ExtendedTrajectoryStep{
		StepID:      uuid.New().String(),
		Timestamp:   time.Now(),
		Action:      action,
		Reward:      reward,
		StateBefore: stateBefore,
		StateAfter:  stateAfter,
	}

	traj.Steps = append(traj.Steps, step)
	traj.TotalReward += reward
	r.stats.TotalSteps++

	return nil
}

// EndRecording ends a trajectory recording with a verdict.
func (r *TrajectoryRecorder) EndRecording(ctx context.Context, trajectoryID string, success bool, qualityScore float64) (*domainNeural.ExtendedTrajectory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	traj, exists := r.active[trajectoryID]
	if !exists {
		return nil, fmt.Errorf("trajectory not found: %s", trajectoryID)
	}

	now := time.Now()
	traj.EndTime = &now
	traj.IsComplete = true
	traj.QualityScore = qualityScore
	traj.DurationMs = now.Sub(traj.StartTime).Milliseconds()

	// Create verdict
	traj.Verdict = &domainNeural.TrajectoryVerdict{
		Success:        success,
		Confidence:     qualityScore,
		Strengths:      make([]string, 0),
		Weaknesses:     make([]string, 0),
		Improvements:   make([]string, 0),
		RelevanceScore: qualityScore,
		JudgedAt:       now,
	}

	// Calculate total reward if not already set
	if traj.TotalReward == 0 {
		for _, step := range traj.Steps {
			traj.TotalReward += step.Reward
		}
	}

	// Remove from active
	delete(r.active, trajectoryID)
	r.stats.ActiveCount = len(r.active)
	r.stats.TotalRecorded++

	return traj, nil
}

// EndRecordingWithVerdict ends with a detailed verdict.
func (r *TrajectoryRecorder) EndRecordingWithVerdict(ctx context.Context, trajectoryID string, verdict *domainNeural.TrajectoryVerdict, qualityScore float64) (*domainNeural.ExtendedTrajectory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	traj, exists := r.active[trajectoryID]
	if !exists {
		return nil, fmt.Errorf("trajectory not found: %s", trajectoryID)
	}

	now := time.Now()
	traj.EndTime = &now
	traj.IsComplete = true
	traj.QualityScore = qualityScore
	traj.DurationMs = now.Sub(traj.StartTime).Milliseconds()
	traj.Verdict = verdict

	// Remove from active
	delete(r.active, trajectoryID)
	r.stats.ActiveCount = len(r.active)
	r.stats.TotalRecorded++

	return traj, nil
}

// CancelRecording cancels an active recording.
func (r *TrajectoryRecorder) CancelRecording(ctx context.Context, trajectoryID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.active[trajectoryID]; !exists {
		return fmt.Errorf("trajectory not found: %s", trajectoryID)
	}

	delete(r.active, trajectoryID)
	r.stats.ActiveCount = len(r.active)
	r.stats.DroppedCount++

	return nil
}

// GetActiveTrajectory returns an active trajectory.
func (r *TrajectoryRecorder) GetActiveTrajectory(ctx context.Context, trajectoryID string) (*domainNeural.ExtendedTrajectory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	traj, exists := r.active[trajectoryID]
	if !exists {
		return nil, fmt.Errorf("trajectory not found: %s", trajectoryID)
	}

	return traj, nil
}

// ListActiveTrajectories returns all active trajectory IDs.
func (r *TrajectoryRecorder) ListActiveTrajectories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.active))
	for id := range r.active {
		ids = append(ids, id)
	}
	return ids
}

// GetStats returns recorder statistics.
func (r *TrajectoryRecorder) GetStats() *TrajectoryRecorderStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// UpdateStepReward updates the reward for a specific step.
func (r *TrajectoryRecorder) UpdateStepReward(ctx context.Context, trajectoryID string, stepIndex int, reward float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	traj, exists := r.active[trajectoryID]
	if !exists {
		return fmt.Errorf("trajectory not found: %s", trajectoryID)
	}

	if stepIndex < 0 || stepIndex >= len(traj.Steps) {
		return fmt.Errorf("invalid step index: %d", stepIndex)
	}

	oldReward := traj.Steps[stepIndex].Reward
	traj.Steps[stepIndex].Reward = reward
	traj.TotalReward = traj.TotalReward - oldReward + reward

	return nil
}

// GetStepCount returns the number of steps in a trajectory.
func (r *TrajectoryRecorder) GetStepCount(ctx context.Context, trajectoryID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	traj, exists := r.active[trajectoryID]
	if !exists {
		return 0, fmt.Errorf("trajectory not found: %s", trajectoryID)
	}

	return len(traj.Steps), nil
}

// SetAttentionWeights sets attention weights for the last step.
func (r *TrajectoryRecorder) SetAttentionWeights(ctx context.Context, trajectoryID string, weights []float32) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	traj, exists := r.active[trajectoryID]
	if !exists {
		return fmt.Errorf("trajectory not found: %s", trajectoryID)
	}

	if len(traj.Steps) == 0 {
		return fmt.Errorf("no steps to update")
	}

	traj.Steps[len(traj.Steps)-1].AttentionWeights = weights
	return nil
}
