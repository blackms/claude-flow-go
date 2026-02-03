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

// ContinualLearningService manages continual learning with EWC.
type ContinualLearningService struct {
	mu                sync.RWMutex
	ewcEngine         *infraNeural.EWCEngine
	adaptationService *AdaptationService
	config            ContinualLearningServiceConfig
	currentTask       *domainNeural.EWCTask
	taskHistory       []*domainNeural.EWCTask
	gradientBuffer    [][]float64
	stats             *ContinualLearningStats
}

// ContinualLearningServiceConfig configures the continual learning service.
type ContinualLearningServiceConfig struct {
	// EWC is the EWC configuration.
	EWC domainNeural.EWCConfig

	// LoRA is the LoRA configuration.
	LoRA domainNeural.LoRAConfig

	// EnableLoRA enables LoRA adapters for each task.
	EnableLoRA bool

	// EnableEWC enables EWC regularization.
	EnableEWC bool

	// GradientBufferSize is the size of the gradient buffer.
	GradientBufferSize int

	// AutoConsolidate automatically consolidates after task completion.
	AutoConsolidate bool

	// TaskBoundaryThreshold is the threshold for detecting task boundaries.
	TaskBoundaryThreshold float64

	// ForgettingThreshold triggers alerts when forgetting exceeds this.
	ForgettingThreshold float64
}

// DefaultContinualLearningServiceConfig returns the default configuration.
func DefaultContinualLearningServiceConfig() ContinualLearningServiceConfig {
	return ContinualLearningServiceConfig{
		EWC:                   domainNeural.DefaultEWCConfig(),
		LoRA:                  domainNeural.DefaultLoRAConfig(),
		EnableLoRA:            true,
		EnableEWC:             true,
		GradientBufferSize:    100,
		AutoConsolidate:       true,
		TaskBoundaryThreshold: 0.3,
		ForgettingThreshold:   0.5,
	}
}

// ContinualLearningStats contains service statistics.
type ContinualLearningStats struct {
	TotalTasks             int     `json:"totalTasks"`
	CompletedTasks         int     `json:"completedTasks"`
	TotalTrainingSteps     int64   `json:"totalTrainingSteps"`
	TotalRegularizations   int64   `json:"totalRegularizations"`
	AvgTaskPerformance     float64 `json:"avgTaskPerformance"`
	AvgForgettingMetric    float64 `json:"avgForgettingMetric"`
	TotalRegularizationLoss float64 `json:"totalRegularizationLoss"`
}

// NewContinualLearningService creates a new continual learning service.
func NewContinualLearningService(config ContinualLearningServiceConfig) *ContinualLearningService {
	var adaptationSvc *AdaptationService
	if config.EnableLoRA {
		adaptationSvc = NewAdaptationService(AdaptationServiceConfig{
			DefaultLoRAConfig:   config.LoRA,
			MaxAdaptersPerAgent: 10,
			EnableAutoMerge:     true,
			MergeThreshold:      5,
		})
	}

	var ewcEngine *infraNeural.EWCEngine
	if config.EnableEWC {
		ewcEngine = infraNeural.NewEWCEngine(config.EWC)
	}

	return &ContinualLearningService{
		ewcEngine:         ewcEngine,
		adaptationService: adaptationSvc,
		config:            config,
		taskHistory:       make([]*domainNeural.EWCTask, 0),
		gradientBuffer:    make([][]float64, 0),
		stats:             &ContinualLearningStats{},
	}
}

// StartTask starts a new learning task.
func (s *ContinualLearningService) StartTask(ctx context.Context, name, description string) (*domainNeural.EWCTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentTask != nil && !s.currentTask.IsConsolidated {
		return nil, fmt.Errorf("current task not completed: %s", s.currentTask.ID)
	}

	task := &domainNeural.EWCTask{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
	}

	if s.config.EnableEWC {
		if err := s.ewcEngine.RegisterTask(task); err != nil {
			return nil, fmt.Errorf("failed to register task: %w", err)
		}
	}

	s.currentTask = task
	s.gradientBuffer = make([][]float64, 0)
	s.stats.TotalTasks++

	return task, nil
}

// RecordGradient records a gradient for the current task.
func (s *ContinualLearningService) RecordGradient(ctx context.Context, gradient []float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.gradientBuffer) < s.config.GradientBufferSize {
		gradCopy := make([]float64, len(gradient))
		copy(gradCopy, gradient)
		s.gradientBuffer = append(s.gradientBuffer, gradCopy)
	} else {
		// Ring buffer: replace oldest
		s.gradientBuffer = append(s.gradientBuffer[1:], gradient)
	}

	s.stats.TotalTrainingSteps++
}

// TrainStep performs a training step with EWC regularization.
func (s *ContinualLearningService) TrainStep(ctx context.Context, taskGradient, currentParams []float64) ([]float64, *domainNeural.EWCLoss, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Record gradient
	if len(s.gradientBuffer) < s.config.GradientBufferSize {
		gradCopy := make([]float64, len(taskGradient))
		copy(gradCopy, taskGradient)
		s.gradientBuffer = append(s.gradientBuffer, gradCopy)
	}

	if !s.config.EnableEWC || s.ewcEngine == nil {
		return taskGradient, nil, nil
	}

	// Compute EWC loss
	loss := s.ewcEngine.ComputeRegularizationLoss(currentParams)
	s.stats.TotalRegularizations++
	s.stats.TotalRegularizationLoss += loss.RegularizationLoss

	// Apply regularization to gradient
	regularizedGrad := s.ewcEngine.ApplyRegularization(taskGradient, currentParams)

	return regularizedGrad, loss, nil
}

// CompleteTask completes the current task and consolidates.
func (s *ContinualLearningService) CompleteTask(ctx context.Context, parameters []float64, performance float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentTask == nil {
		return fmt.Errorf("no active task")
	}

	s.currentTask.Performance = performance
	s.currentTask.DataSize = len(s.gradientBuffer)

	if s.config.EnableEWC && s.config.AutoConsolidate {
		if err := s.ewcEngine.ConsolidateTask(s.currentTask.ID, parameters, s.gradientBuffer); err != nil {
			return fmt.Errorf("failed to consolidate task: %w", err)
		}
	}

	s.currentTask.IsConsolidated = true
	s.currentTask.CompletedAt = time.Now()
	s.taskHistory = append(s.taskHistory, s.currentTask)
	s.stats.CompletedTasks++

	// Update average performance
	var totalPerf float64
	for _, task := range s.taskHistory {
		totalPerf += task.Performance
	}
	s.stats.AvgTaskPerformance = totalPerf / float64(len(s.taskHistory))

	s.currentTask = nil
	s.gradientBuffer = make([][]float64, 0)

	return nil
}

// ConsolidateManually manually consolidates a task.
func (s *ContinualLearningService) ConsolidateManually(ctx context.Context, taskID string, parameters []float64, gradients [][]float64) error {
	if !s.config.EnableEWC || s.ewcEngine == nil {
		return fmt.Errorf("EWC not enabled")
	}

	return s.ewcEngine.ConsolidateTask(taskID, parameters, gradients)
}

// EvaluateForgetting evaluates forgetting across all tasks.
func (s *ContinualLearningService) EvaluateForgetting(ctx context.Context, currentParams []float64) (map[string]float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.config.EnableEWC || s.ewcEngine == nil {
		return nil, fmt.Errorf("EWC not enabled")
	}

	forgetting := make(map[string]float64)
	var totalForgetting float64

	for _, task := range s.taskHistory {
		metric, err := s.ewcEngine.ComputeForgettingMetric(task.ID, currentParams)
		if err == nil {
			forgetting[task.ID] = metric
			totalForgetting += metric
		}
	}

	if len(forgetting) > 0 {
		s.stats.AvgForgettingMetric = totalForgetting / float64(len(forgetting))
	}

	return forgetting, nil
}

// CreateTaskAdapter creates a LoRA adapter for the current task.
func (s *ContinualLearningService) CreateTaskAdapter(ctx context.Context) (*domainNeural.LoRAAdapter, error) {
	s.mu.RLock()
	currentTask := s.currentTask
	s.mu.RUnlock()

	if currentTask == nil {
		return nil, fmt.Errorf("no active task")
	}

	if !s.config.EnableLoRA || s.adaptationService == nil {
		return nil, fmt.Errorf("LoRA not enabled")
	}

	return s.adaptationService.CreateAdapter(ctx, fmt.Sprintf("task-%s", currentTask.Name), &s.config.LoRA)
}

// AdaptEmbedding adapts an embedding using task-specific LoRA.
func (s *ContinualLearningService) AdaptEmbedding(ctx context.Context, agentID string, embedding []float64) ([]float64, error) {
	if !s.config.EnableLoRA || s.adaptationService == nil {
		return embedding, nil
	}

	return s.adaptationService.Adapt(ctx, agentID, embedding)
}

// GetCurrentTask returns the current task.
func (s *ContinualLearningService) GetCurrentTask() *domainNeural.EWCTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTask
}

// GetTaskHistory returns the task history.
func (s *ContinualLearningService) GetTaskHistory() []*domainNeural.EWCTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]*domainNeural.EWCTask, len(s.taskHistory))
	copy(history, s.taskHistory)
	return history
}

// GetTask returns a task by ID.
func (s *ContinualLearningService) GetTask(ctx context.Context, taskID string) (*domainNeural.EWCTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, task := range s.taskHistory {
		if task.ID == taskID {
			return task, nil
		}
	}

	if s.currentTask != nil && s.currentTask.ID == taskID {
		return s.currentTask, nil
	}

	return nil, fmt.Errorf("task not found: %s", taskID)
}

// GetEWCState returns the current EWC state.
func (s *ContinualLearningService) GetEWCState() *domainNeural.EWCState {
	if s.ewcEngine == nil {
		return nil
	}
	return s.ewcEngine.GetState()
}

// GetImportanceWeights returns importance weights.
func (s *ContinualLearningService) GetImportanceWeights() []domainNeural.ImportanceWeight {
	if s.ewcEngine == nil {
		return nil
	}
	return s.ewcEngine.GetImportanceWeights()
}

// GetStats returns service statistics.
func (s *ContinualLearningService) GetStats() *ContinualLearningStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// GetEWCStats returns EWC statistics.
func (s *ContinualLearningService) GetEWCStats() *domainNeural.EWCStats {
	if s.ewcEngine == nil {
		return nil
	}
	return s.ewcEngine.GetStats()
}

// GetAdaptationStats returns adaptation statistics.
func (s *ContinualLearningService) GetAdaptationStats() *AdaptationServiceStats {
	if s.adaptationService == nil {
		return nil
	}
	return s.adaptationService.GetStats()
}

// CreateCheckpoint creates a checkpoint of the current state.
func (s *ContinualLearningService) CreateCheckpoint(ctx context.Context) (*ContinualLearningCheckpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	checkpoint := &ContinualLearningCheckpoint{
		ID:        uuid.New().String(),
		CreatedAt: time.Now(),
	}

	if s.ewcEngine != nil {
		checkpoint.EWCCheckpoint = s.ewcEngine.CreateCheckpoint()
	}

	checkpoint.TaskCount = len(s.taskHistory)
	checkpoint.Stats = s.stats

	return checkpoint, nil
}

// RestoreCheckpoint restores from a checkpoint.
func (s *ContinualLearningService) RestoreCheckpoint(ctx context.Context, checkpoint *ContinualLearningCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if checkpoint.EWCCheckpoint != nil && s.ewcEngine != nil {
		if err := s.ewcEngine.RestoreCheckpoint(checkpoint.EWCCheckpoint); err != nil {
			return fmt.Errorf("failed to restore EWC checkpoint: %w", err)
		}
	}

	return nil
}

// Reset resets the service to initial state.
func (s *ContinualLearningService) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ewcEngine != nil {
		s.ewcEngine.Reset()
	}

	s.currentTask = nil
	s.taskHistory = make([]*domainNeural.EWCTask, 0)
	s.gradientBuffer = make([][]float64, 0)
	s.stats = &ContinualLearningStats{}
}

// ContinualLearningCheckpoint represents a saved state.
type ContinualLearningCheckpoint struct {
	ID            string                       `json:"id"`
	EWCCheckpoint *domainNeural.EWCCheckpoint  `json:"ewcCheckpoint,omitempty"`
	TaskCount     int                          `json:"taskCount"`
	Stats         *ContinualLearningStats      `json:"stats,omitempty"`
	CreatedAt     time.Time                    `json:"createdAt"`
}

// GetConfig returns the current configuration.
func (s *ContinualLearningService) GetConfig() ContinualLearningServiceConfig {
	return s.config
}
