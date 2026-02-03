// Package neural provides neural network infrastructure.
package neural

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// EWCEngine implements EWC++ regularization.
type EWCEngine struct {
	mu     sync.RWMutex
	config domainNeural.EWCConfig
	state  *domainNeural.EWCState
	fisher *FisherMatrix
	tasks  map[string]*domainNeural.EWCTask
	stats  *EWCEngineStats
}

// EWCEngineStats contains engine statistics.
type EWCEngineStats struct {
	TotalTasks           int     `json:"totalTasks"`
	TotalConsolidations  int     `json:"totalConsolidations"`
	TotalRegUpdates      int64   `json:"totalRegUpdates"`
	AvgRegTimeMs         float64 `json:"avgRegTimeMs"`
	TotalForgottenWeight float64 `json:"totalForgottenWeight"`
}

// NewEWCEngine creates a new EWC engine.
func NewEWCEngine(config domainNeural.EWCConfig) *EWCEngine {
	fisherConfig := FisherConfig{
		Dimension:              config.ParameterDimension,
		UseRunningAverage:      true,
		DecayFactor:            config.Gamma,
		ClipValue:              config.ClipImportance,
		MinValue:               config.MinImportance,
		Normalize:              config.NormalizeImportance,
		TrackTaskContributions: true,
	}

	return &EWCEngine{
		config: config,
		state: &domainNeural.EWCState{
			ID:             uuid.New().String(),
			Config:         config,
			OldParameters:  make([]float64, config.ParameterDimension),
			FisherDiagonal: make([]float64, config.ParameterDimension),
			TaskHeads:      make(map[string]*domainNeural.TaskHead),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		fisher: NewFisherMatrix(fisherConfig),
		tasks:  make(map[string]*domainNeural.EWCTask),
		stats:  &EWCEngineStats{},
	}
}

// RegisterTask registers a new task for continual learning.
func (e *EWCEngine) RegisterTask(task *domainNeural.EWCTask) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.tasks[task.ID]; exists {
		return fmt.Errorf("task already registered: %s", task.ID)
	}

	e.tasks[task.ID] = task
	e.stats.TotalTasks++

	// Create task head if enabled
	if e.config.EnableTaskHeads {
		head := &domainNeural.TaskHead{
			ID:              uuid.New().String(),
			TaskID:          task.ID,
			InputDimension:  e.config.ParameterDimension,
			OutputDimension: 1, // Default single output
			CreatedAt:       time.Now(),
		}
		e.state.TaskHeads[task.ID] = head
	}

	return nil
}

// ConsolidateTask consolidates a completed task.
// This updates the Fisher information and stores old parameters.
func (e *EWCEngine) ConsolidateTask(taskID string, parameters []float64, gradientBatch [][]float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.IsConsolidated {
		return fmt.Errorf("task already consolidated: %s", taskID)
	}

	// Compute Fisher information for this task
	taskFisher := make([]float64, e.config.ParameterDimension)
	if len(gradientBatch) > 0 {
		for _, grads := range gradientBatch {
			for i := 0; i < len(grads) && i < len(taskFisher); i++ {
				taskFisher[i] += grads[i] * grads[i]
			}
		}
		n := float64(len(gradientBatch))
		for i := range taskFisher {
			taskFisher[i] /= n
		}
	} else {
		// Use uniform importance if no gradients provided
		for i := range taskFisher {
			taskFisher[i] = 1.0
		}
	}

	// Store task-specific Fisher
	task.FisherDiagonal = taskFisher
	task.Parameters = make([]float64, len(parameters))
	copy(task.Parameters, parameters)
	task.IsConsolidated = true
	task.CompletedAt = time.Now()

	// Update global Fisher with online formula (EWC++)
	e.fisher.AddTaskContribution(taskID, taskFisher)

	// Update old parameters with weighted average
	if e.state.TaskCount == 0 {
		// First task - just copy parameters
		e.state.OldParameters = make([]float64, len(parameters))
		copy(e.state.OldParameters, parameters)
	} else {
		// Weighted average based on importance
		gamma := e.config.Gamma
		for i := 0; i < len(parameters) && i < len(e.state.OldParameters); i++ {
			e.state.OldParameters[i] = gamma*e.state.OldParameters[i] + (1-gamma)*parameters[i]
		}
	}

	// Update consolidated Fisher diagonal
	e.state.FisherDiagonal = e.fisher.GetDiagonal()
	e.state.TaskCount++
	e.state.TaskIDs = append(e.state.TaskIDs, taskID)
	e.state.LastConsolidation = time.Now()
	e.state.UpdatedAt = time.Now()

	e.stats.TotalConsolidations++

	return nil
}

// ComputeRegularizationLoss computes the EWC regularization loss.
// L_ewc = (lambda/2) * sum_i(F_i * (theta_i - theta_old_i)^2)
func (e *EWCEngine) ComputeRegularizationLoss(currentParams []float64) *domainNeural.EWCLoss {
	startTime := time.Now()
	defer func() {
		e.mu.Lock()
		e.stats.TotalRegUpdates++
		elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
		e.stats.AvgRegTimeMs = (e.stats.AvgRegTimeMs*float64(e.stats.TotalRegUpdates-1) + elapsed) / float64(e.stats.TotalRegUpdates)
		e.mu.Unlock()
	}()

	e.mu.RLock()
	defer e.mu.RUnlock()

	loss := e.fisher.ComputeRegularizationLoss(currentParams, e.state.OldParameters, e.config.Lambda)

	e.state.TotalRegularizationLoss += loss.RegularizationLoss

	return loss
}

// ComputeGradient computes the gradient of the EWC regularization term.
// d L_ewc / d theta_i = lambda * F_i * (theta_i - theta_old_i)
func (e *EWCEngine) ComputeGradient(currentParams []float64) []float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dim := len(e.state.FisherDiagonal)
	gradient := make([]float64, dim)

	for i := 0; i < dim && i < len(currentParams) && i < len(e.state.OldParameters); i++ {
		diff := currentParams[i] - e.state.OldParameters[i]
		gradient[i] = e.config.Lambda * e.state.FisherDiagonal[i] * diff
	}

	return gradient
}

// ApplyRegularization applies EWC regularization to a gradient update.
// Returns the regularized gradient: grad_total = grad_task + grad_ewc
func (e *EWCEngine) ApplyRegularization(taskGradient, currentParams []float64) []float64 {
	ewcGradient := e.ComputeGradient(currentParams)

	result := make([]float64, len(taskGradient))
	for i := range taskGradient {
		result[i] = taskGradient[i]
		if i < len(ewcGradient) {
			result[i] += ewcGradient[i]
		}
	}

	return result
}

// GetState returns the current EWC state.
func (e *EWCEngine) GetState() *domainNeural.EWCState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// GetTask returns a task by ID.
func (e *EWCEngine) GetTask(taskID string) (*domainNeural.EWCTask, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

// ListTasks returns all task IDs.
func (e *EWCEngine) ListTasks() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]string, 0, len(e.tasks))
	for id := range e.tasks {
		ids = append(ids, id)
	}
	return ids
}

// GetTaskHead returns a task-specific head.
func (e *EWCEngine) GetTaskHead(taskID string) (*domainNeural.TaskHead, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	head, exists := e.state.TaskHeads[taskID]
	if !exists {
		return nil, fmt.Errorf("task head not found: %s", taskID)
	}
	return head, nil
}

// UpdateTaskHead updates a task head's weights.
func (e *EWCEngine) UpdateTaskHead(taskID string, weights [][]float64, bias []float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	head, exists := e.state.TaskHeads[taskID]
	if !exists {
		return fmt.Errorf("task head not found: %s", taskID)
	}

	head.Weights = weights
	head.Bias = bias
	return nil
}

// FreezeTaskHead freezes a task head.
func (e *EWCEngine) FreezeTaskHead(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	head, exists := e.state.TaskHeads[taskID]
	if !exists {
		return fmt.Errorf("task head not found: %s", taskID)
	}

	head.Frozen = true
	return nil
}

// GetImportanceWeights returns importance weights for all parameters.
func (e *EWCEngine) GetImportanceWeights() []domainNeural.ImportanceWeight {
	return e.fisher.GetImportanceWeights()
}

// GetStats returns engine statistics.
func (e *EWCEngine) GetStats() *domainNeural.EWCStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fisherStats := e.fisher.GetStats()

	// Build task performances map
	taskPerfs := make(map[string]float64)
	for id, task := range e.tasks {
		taskPerfs[id] = task.Performance
	}

	return &domainNeural.EWCStats{
		StateID:             e.state.ID,
		TaskCount:           e.state.TaskCount,
		ParameterCount:      len(e.state.FisherDiagonal),
		AvgImportance:       fisherStats.AvgValue,
		MaxImportance:       fisherStats.MaxValue,
		TotalRegularization: e.state.TotalRegularizationLoss,
		AvgRegTimeMs:        e.stats.AvgRegTimeMs,
		MemoryBytes:         int64(len(e.state.FisherDiagonal)*8 + len(e.state.OldParameters)*8),
		TaskPerformances:    taskPerfs,
	}
}

// CreateCheckpoint creates a checkpoint of the current state.
func (e *EWCEngine) CreateCheckpoint() *domainNeural.EWCCheckpoint {
	e.mu.RLock()
	defer e.mu.RUnlock()

	oldParams := make([]float64, len(e.state.OldParameters))
	copy(oldParams, e.state.OldParameters)

	fisherDiag := make([]float64, len(e.state.FisherDiagonal))
	copy(fisherDiag, e.state.FisherDiagonal)

	return &domainNeural.EWCCheckpoint{
		ID:             uuid.New().String(),
		StateID:        e.state.ID,
		TaskCount:      e.state.TaskCount,
		OldParameters:  oldParams,
		FisherDiagonal: fisherDiag,
		CreatedAt:      time.Now(),
	}
}

// RestoreCheckpoint restores from a checkpoint.
func (e *EWCEngine) RestoreCheckpoint(checkpoint *domainNeural.EWCCheckpoint) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if checkpoint.StateID != e.state.ID {
		return fmt.Errorf("checkpoint state ID mismatch")
	}

	e.state.OldParameters = checkpoint.OldParameters
	e.state.FisherDiagonal = checkpoint.FisherDiagonal
	e.state.TaskCount = checkpoint.TaskCount
	e.state.UpdatedAt = time.Now()

	return nil
}

// Reset resets the EWC engine to initial state.
func (e *EWCEngine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state = &domainNeural.EWCState{
		ID:             uuid.New().String(),
		Config:         e.config,
		OldParameters:  make([]float64, e.config.ParameterDimension),
		FisherDiagonal: make([]float64, e.config.ParameterDimension),
		TaskHeads:      make(map[string]*domainNeural.TaskHead),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	e.fisher.Reset()
	e.tasks = make(map[string]*domainNeural.EWCTask)
	e.stats = &EWCEngineStats{}
}

// ComputeForgettingMetric computes how much a task has been forgotten.
// Returns 0 if no forgetting, higher values indicate more forgetting.
func (e *EWCEngine) ComputeForgettingMetric(taskID string, currentParams []float64) (float64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return 0, fmt.Errorf("task not found: %s", taskID)
	}

	if !task.IsConsolidated {
		return 0, fmt.Errorf("task not consolidated: %s", taskID)
	}

	// Compute weighted parameter drift
	var weightedDrift float64
	var totalWeight float64

	for i := 0; i < len(task.Parameters) && i < len(currentParams) && i < len(task.FisherDiagonal); i++ {
		diff := currentParams[i] - task.Parameters[i]
		weightedDrift += task.FisherDiagonal[i] * diff * diff
		totalWeight += task.FisherDiagonal[i]
	}

	if totalWeight > 0 {
		return math.Sqrt(weightedDrift / totalWeight), nil
	}
	return 0, nil
}

// SelectiveConsolidation performs selective consolidation based on importance.
// Only consolidates the top K most important parameters.
func (e *EWCEngine) SelectiveConsolidation(taskID string, parameters []float64, gradientBatch [][]float64, topK int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Compute Fisher for this task
	taskFisher := make([]float64, e.config.ParameterDimension)
	if len(gradientBatch) > 0 {
		for _, grads := range gradientBatch {
			for i := 0; i < len(grads) && i < len(taskFisher); i++ {
				taskFisher[i] += grads[i] * grads[i]
			}
		}
		n := float64(len(gradientBatch))
		for i := range taskFisher {
			taskFisher[i] /= n
		}
	}

	// Find top K important parameters
	topIndices := e.fisher.TopKImportant(topK)

	// Only update Fisher for top K parameters
	selectiveFisher := make([]float64, len(taskFisher))
	for _, idx := range topIndices {
		if idx < len(taskFisher) {
			selectiveFisher[idx] = taskFisher[idx]
		}
	}

	task.FisherDiagonal = selectiveFisher
	task.IsConsolidated = true
	task.CompletedAt = time.Now()

	// Update global state with selective consolidation
	e.fisher.AddTaskContribution(taskID, selectiveFisher)
	e.state.FisherDiagonal = e.fisher.GetDiagonal()
	e.state.TaskCount++
	e.state.TaskIDs = append(e.state.TaskIDs, taskID)

	return nil
}
