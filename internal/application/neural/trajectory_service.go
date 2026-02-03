// Package neural provides neural learning application services.
package neural

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// TrajectoryService orchestrates trajectory learning operations.
type TrajectoryService struct {
	mu       sync.RWMutex
	recorder *TrajectoryRecorder
	learner  *TrajectoryLearner
	store    *infraNeural.TrajectoryStore
	config   TrajectoryServiceConfig
	stats    *domainNeural.TrajectoryStats

	// Background learning
	learningTicker *time.Ticker
	stopCh         chan struct{}
	running        bool
}

// TrajectoryServiceConfig configures the trajectory service.
type TrajectoryServiceConfig struct {
	// BasePath is the base storage path.
	BasePath string `json:"basePath"`

	// StoreConfig is the store configuration.
	StoreConfig infraNeural.TrajectoryStoreConfig `json:"storeConfig"`

	// RecorderConfig is the recorder configuration.
	RecorderConfig TrajectoryRecorderConfig `json:"recorderConfig"`

	// LearnerConfig is the learner configuration.
	LearnerConfig TrajectoryLearnerConfig `json:"learnerConfig"`

	// EnableBackgroundLearning enables periodic learning.
	EnableBackgroundLearning bool `json:"enableBackgroundLearning"`

	// LearningIntervalSeconds is the interval between learning cycles.
	LearningIntervalSeconds int `json:"learningIntervalSeconds"`

	// AutoPruneEnabled enables automatic pruning.
	AutoPruneEnabled bool `json:"autoPruneEnabled"`

	// PruneIntervalHours is the interval between prune operations.
	PruneIntervalHours int `json:"pruneIntervalHours"`
}

// DefaultTrajectoryServiceConfig returns the default configuration.
func DefaultTrajectoryServiceConfig() TrajectoryServiceConfig {
	return TrajectoryServiceConfig{
		BasePath:                 "",
		StoreConfig:              infraNeural.DefaultTrajectoryStoreConfig(),
		RecorderConfig:           DefaultTrajectoryRecorderConfig(),
		LearnerConfig:            DefaultTrajectoryLearnerConfig(),
		EnableBackgroundLearning: true,
		LearningIntervalSeconds:  60,
		AutoPruneEnabled:         true,
		PruneIntervalHours:       24,
	}
}

// NewTrajectoryService creates a new trajectory service.
func NewTrajectoryService(config TrajectoryServiceConfig) (*TrajectoryService, error) {
	// Setup base path
	if config.BasePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.BasePath = filepath.Join(home, ".claude-flow", "trajectories")
	}

	// Ensure directory exists
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create trajectory directory: %w", err)
	}

	// Configure store with proper path
	storeConfig := config.StoreConfig
	if storeConfig.DBPath == ":memory:" || storeConfig.DBPath == "" {
		storeConfig.DBPath = filepath.Join(config.BasePath, "trajectories.db")
	}

	// Create store
	store, err := infraNeural.NewTrajectoryStore(storeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create trajectory store: %w", err)
	}

	svc := &TrajectoryService{
		recorder: NewTrajectoryRecorder(config.RecorderConfig),
		learner:  NewTrajectoryLearner(config.LearnerConfig),
		store:    store,
		config:   config,
		stats:    &domainNeural.TrajectoryStats{},
		stopCh:   make(chan struct{}),
	}

	// Start background learning if enabled
	if config.EnableBackgroundLearning {
		svc.StartBackgroundLearning()
	}

	return svc, nil
}

// StartRecording starts recording a new trajectory.
func (s *TrajectoryService) StartRecording(ctx context.Context, trajectoryContext string, domain domainNeural.TrajectoryDomain, agentID string) (string, error) {
	trajectoryID, err := s.recorder.StartRecording(ctx, trajectoryContext, domain, agentID)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.stats.ActiveTrajectories++
	s.mu.Unlock()

	return trajectoryID, nil
}

// RecordStep records a step in a trajectory.
func (s *TrajectoryService) RecordStep(ctx context.Context, trajectoryID string, action string, contextData map[string]interface{}, outcome string) error {
	return s.recorder.RecordStep(ctx, trajectoryID, action, contextData, outcome)
}

// RecordStepWithReward records a step with an explicit reward.
func (s *TrajectoryService) RecordStepWithReward(ctx context.Context, trajectoryID string, action string, reward float64, stateBefore, stateAfter []float32) error {
	return s.recorder.RecordStepWithReward(ctx, trajectoryID, action, reward, stateBefore, stateAfter)
}

// EndRecording ends a trajectory recording and saves it.
func (s *TrajectoryService) EndRecording(ctx context.Context, trajectoryID string, success bool, qualityScore float64) (*domainNeural.ExtendedTrajectory, error) {
	traj, err := s.recorder.EndRecording(ctx, trajectoryID, success, qualityScore)
	if err != nil {
		return nil, err
	}

	// Compute reward
	reward := s.learner.ComputeReward(ctx, traj)
	traj.TotalReward = reward.TotalReward

	// Distribute reward across steps
	if len(traj.Steps) > 0 {
		perStepReward := reward.TotalReward / float64(len(traj.Steps))
		for i := range traj.Steps {
			traj.Steps[i].Reward = perStepReward
		}
	}

	// Save to store
	if err := s.store.SaveTrajectory(traj); err != nil {
		return nil, fmt.Errorf("failed to save trajectory: %w", err)
	}

	// Add to learner buffer
	s.learner.LearnFromTrajectory(ctx, traj)

	s.mu.Lock()
	s.stats.ActiveTrajectories--
	s.stats.CompletedTrajectories++
	s.stats.TotalTrajectories++
	if success {
		s.stats.SuccessfulTrajectories++
	}
	s.mu.Unlock()

	return traj, nil
}

// RecordTrajectory implements shared.NeuralLearningSystem.
func (s *TrajectoryService) RecordTrajectory(ctx context.Context, agentID string, steps []shared.TrajectoryStep) error {
	// Start recording
	trajectoryID, err := s.StartRecording(ctx, "agent-execution", domainNeural.DomainGeneral, agentID)
	if err != nil {
		return err
	}

	// Record each step
	for _, step := range steps {
		contextData := step.Context
		if contextData == nil {
			contextData = make(map[string]interface{})
		}

		err := s.recorder.RecordStepWithReward(ctx, trajectoryID, step.Action, step.Reward, nil, nil)
		if err != nil {
			s.recorder.CancelRecording(ctx, trajectoryID)
			return err
		}
	}

	// End recording
	_, err = s.EndRecording(ctx, trajectoryID, true, 0.7)
	return err
}

// TriggerLearning triggers a learning cycle on accumulated trajectories.
func (s *TrajectoryService) TriggerLearning(ctx context.Context) (*domainNeural.TrainingResult, error) {
	// Query recent high-quality trajectories
	query := domainNeural.TrajectoryQuery{
		MinQuality: 0.3,
		Limit:      100,
		OrderBy:    "quality_score",
		Descending: true,
	}

	trajectories, err := s.store.QueryTrajectories(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query trajectories: %w", err)
	}

	if len(trajectories) == 0 {
		return nil, nil
	}

	// Update policy
	result, err := s.learner.UpdatePolicy(ctx, trajectories)
	if err != nil {
		return nil, fmt.Errorf("failed to update policy: %w", err)
	}

	s.mu.Lock()
	s.stats.LearningCycles++
	s.mu.Unlock()

	return result, nil
}

// FindSimilarTrajectories finds trajectories similar to the given embedding.
func (s *TrajectoryService) FindSimilarTrajectories(ctx context.Context, embedding []float32, k int) ([]domainNeural.TrajectorySearchResult, error) {
	return s.store.FindSimilarTrajectories(embedding, k, 0.3)
}

// GetBestTrajectory returns the highest quality trajectory.
func (s *TrajectoryService) GetBestTrajectory(ctx context.Context, domain *domainNeural.TrajectoryDomain, agentID string) (*domainNeural.ExtendedTrajectory, error) {
	return s.store.GetBestTrajectory(domain, agentID)
}

// PruneOldTrajectories removes old, low-quality trajectories.
func (s *TrajectoryService) PruneOldTrajectories(ctx context.Context) (int64, error) {
	return s.store.PruneOldTrajectories()
}

// ExtractPatterns extracts patterns from successful trajectories.
func (s *TrajectoryService) ExtractPatterns(ctx context.Context) ([]*domainNeural.PatternExtraction, error) {
	// Query successful trajectories
	query := domainNeural.TrajectoryQuery{
		SuccessOnly: true,
		MinQuality:  0.7,
		Limit:       100,
		OrderBy:     "quality_score",
		Descending:  true,
	}

	trajectories, err := s.store.QueryTrajectories(query)
	if err != nil {
		return nil, err
	}

	return s.learner.ExtractPatterns(ctx, trajectories)
}

// ConsolidateMemories distills memories from high-quality trajectories.
func (s *TrajectoryService) ConsolidateMemories(ctx context.Context) ([]*domainNeural.DistilledMemory, error) {
	// Query high-quality trajectories
	query := domainNeural.TrajectoryQuery{
		SuccessOnly: true,
		MinQuality:  0.8,
		Limit:       50,
		OrderBy:     "quality_score",
		Descending:  true,
	}

	trajectories, err := s.store.QueryTrajectories(query)
	if err != nil {
		return nil, err
	}

	memories, err := s.learner.ConsolidateMemory(ctx, trajectories)
	if err != nil {
		return nil, err
	}

	// Save memories
	for _, memory := range memories {
		if err := s.store.SaveDistilledMemory(memory); err != nil {
			continue
		}
	}

	s.mu.Lock()
	s.stats.DistilledMemories += len(memories)
	s.mu.Unlock()

	return memories, nil
}

// GetTrajectory retrieves a trajectory by ID.
func (s *TrajectoryService) GetTrajectory(ctx context.Context, trajectoryID string) (*domainNeural.ExtendedTrajectory, error) {
	return s.store.GetTrajectory(trajectoryID)
}

// QueryTrajectories queries trajectories based on criteria.
func (s *TrajectoryService) QueryTrajectories(ctx context.Context, query domainNeural.TrajectoryQuery) ([]*domainNeural.ExtendedTrajectory, error) {
	return s.store.QueryTrajectories(query)
}

// GetDistilledMemories returns distilled memories.
func (s *TrajectoryService) GetDistilledMemories(ctx context.Context, minQuality float64, limit int) ([]*domainNeural.DistilledMemory, error) {
	return s.store.GetDistilledMemories(minQuality, limit)
}

// GetStats returns service statistics.
func (s *TrajectoryService) GetStats() *domainNeural.TrajectoryStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Update with store stats
	storeStats, _ := s.store.GetStats()
	if storeStats != nil {
		s.stats.TotalTrajectories = storeStats.TotalTrajectories
	}

	// Update with learner stats
	learnerStats := s.learner.GetStats()
	s.stats.LearningCycles = learnerStats.TotalLearningCycles
	s.stats.DistilledMemories = int(learnerStats.TotalMemoriesDistilled)

	// Update with recorder stats
	recorderStats := s.recorder.GetStats()
	s.stats.ActiveTrajectories = recorderStats.ActiveCount

	return s.stats
}

// StartBackgroundLearning starts periodic background learning.
func (s *TrajectoryService) StartBackgroundLearning() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	interval := time.Duration(s.config.LearningIntervalSeconds) * time.Second
	s.learningTicker = time.NewTicker(interval)
	s.running = true

	go func() {
		for {
			select {
			case <-s.learningTicker.C:
				ctx := context.Background()
				s.TriggerLearning(ctx)

				// Also prune if enabled
				if s.config.AutoPruneEnabled {
					s.PruneOldTrajectories(ctx)
				}

			case <-s.stopCh:
				return
			}
		}
	}()
}

// StopBackgroundLearning stops periodic background learning.
func (s *TrajectoryService) StopBackgroundLearning() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	if s.learningTicker != nil {
		s.learningTicker.Stop()
	}
	close(s.stopCh)
	s.running = false
	s.stopCh = make(chan struct{})
}

// GetPredictedAction returns the predicted action for a given state.
func (s *TrajectoryService) GetPredictedAction(ctx context.Context, states [][]float32, actions []int, targetReturn float64) int {
	return s.learner.GetPredictedAction(ctx, states, actions, targetReturn)
}

// GetRecorderStats returns recorder statistics.
func (s *TrajectoryService) GetRecorderStats() *TrajectoryRecorderStats {
	return s.recorder.GetStats()
}

// GetLearnerStats returns learner statistics.
func (s *TrajectoryService) GetLearnerStats() *TrajectoryLearnerStats {
	return s.learner.GetStats()
}

// GetTransformerStats returns Decision Transformer statistics.
func (s *TrajectoryService) GetTransformerStats() map[string]interface{} {
	return s.learner.GetTransformerStats()
}

// Close closes the service.
func (s *TrajectoryService) Close() error {
	s.StopBackgroundLearning()
	return s.store.Close()
}

// CancelRecording cancels an active recording.
func (s *TrajectoryService) CancelRecording(ctx context.Context, trajectoryID string) error {
	err := s.recorder.CancelRecording(ctx, trajectoryID)
	if err == nil {
		s.mu.Lock()
		s.stats.ActiveTrajectories--
		s.mu.Unlock()
	}
	return err
}

// ListActiveRecordings returns all active recording IDs.
func (s *TrajectoryService) ListActiveRecordings() []string {
	return s.recorder.ListActiveTrajectories()
}

// GetActiveRecording returns an active recording.
func (s *TrajectoryService) GetActiveRecording(ctx context.Context, trajectoryID string) (*domainNeural.ExtendedTrajectory, error) {
	return s.recorder.GetActiveTrajectory(ctx, trajectoryID)
}

// EvaluateTrajectory evaluates a trajectory's quality.
func (s *TrajectoryService) EvaluateTrajectory(ctx context.Context, traj *domainNeural.ExtendedTrajectory) float64 {
	return s.learner.EvaluateTrajectory(ctx, traj)
}

// ComputeReward computes the reward for a trajectory.
func (s *TrajectoryService) ComputeReward(ctx context.Context, traj *domainNeural.ExtendedTrajectory) *domainNeural.RewardComponents {
	return s.learner.ComputeReward(ctx, traj)
}
