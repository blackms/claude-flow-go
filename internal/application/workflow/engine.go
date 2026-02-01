// Package workflow provides the WorkflowEngine for executing workflows.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/domain/task"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/events"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// WorkflowExecution holds the state of a workflow execution.
type WorkflowExecution struct {
	mu              sync.RWMutex
	ID              string
	State           shared.WorkflowState
	ExecutionOrder  []string
	TaskTimings     map[string]shared.TaskTiming
	EventLog        []shared.EventLogEntry
	MemorySnapshots []shared.MemorySnapshot
	cancel          context.CancelFunc
}

// Engine executes and manages workflows.
type Engine struct {
	mu            sync.RWMutex
	coordinator   *coordinator.SwarmCoordinator
	memoryBackend shared.MemoryBackend
	eventBus      *events.EventBus
	pluginManager shared.PluginManager
	workflows     map[string]*WorkflowExecution
	initialized   bool
}

// Options holds configuration options for the WorkflowEngine.
type Options struct {
	Coordinator   *coordinator.SwarmCoordinator
	MemoryBackend shared.MemoryBackend
	EventBus      *events.EventBus
	PluginManager shared.PluginManager
}

// NewEngine creates a new WorkflowEngine.
func NewEngine(opts Options) *Engine {
	eventBus := opts.EventBus
	if eventBus == nil && opts.Coordinator != nil {
		eventBus = opts.Coordinator.GetEventBus()
	}
	if eventBus == nil {
		eventBus = events.New()
	}

	return &Engine{
		coordinator:   opts.Coordinator,
		memoryBackend: opts.MemoryBackend,
		eventBus:      eventBus,
		pluginManager: opts.PluginManager,
		workflows:     make(map[string]*WorkflowExecution),
	}
}

// Initialize initializes the workflow engine.
func (e *Engine) Initialize() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.initialized {
		return nil
	}

	e.initialized = true
	return nil
}

// Shutdown shuts down the workflow engine.
func (e *Engine) Shutdown() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Cancel all running workflows
	for _, execution := range e.workflows {
		execution.mu.Lock()
		if execution.State.Status == shared.WorkflowStatusInProgress {
			execution.State.Status = shared.WorkflowStatusCancelled
			if execution.cancel != nil {
				execution.cancel()
			}
		}
		execution.mu.Unlock()
	}

	e.workflows = make(map[string]*WorkflowExecution)
	e.initialized = false
	return nil
}

// ExecuteTask executes a single task.
func (e *Engine) ExecuteTask(ctx context.Context, t shared.Task, agentID string) (shared.TaskResult, error) {
	// Store task start
	if e.memoryBackend != nil {
		e.memoryBackend.Store(shared.Memory{
			ID:        fmt.Sprintf("task-start-%s", t.ID),
			AgentID:   agentID,
			Content:   fmt.Sprintf("Task %s started", t.ID),
			Type:      shared.MemoryTypeTaskStart,
			Timestamp: shared.Now(),
			Metadata: map[string]interface{}{
				"taskId":  t.ID,
				"agentId": agentID,
			},
		})
	}

	result, err := e.coordinator.ExecuteTask(ctx, agentID, t)
	if err != nil {
		return result, err
	}

	// Store task completion
	if e.memoryBackend != nil {
		e.memoryBackend.Store(shared.Memory{
			ID:        fmt.Sprintf("task-complete-%s", t.ID),
			AgentID:   agentID,
			Content:   fmt.Sprintf("Task %s %s", t.ID, result.Status),
			Type:      shared.MemoryTypeTaskComplete,
			Timestamp: shared.Now(),
			Metadata: map[string]interface{}{
				"taskId":   t.ID,
				"agentId":  agentID,
				"status":   string(result.Status),
				"duration": result.Duration,
			},
		})
	}

	return result, nil
}

// ExecuteWorkflow executes a workflow.
func (e *Engine) ExecuteWorkflow(ctx context.Context, workflow shared.WorkflowDefinition) (shared.WorkflowResult, error) {
	execution := e.createExecution(workflow)

	e.mu.Lock()
	e.workflows[workflow.ID] = execution
	e.mu.Unlock()

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	execution.cancel = cancel

	// Invoke before execute hook
	if e.pluginManager != nil {
		e.pluginManager.InvokeExtensionPoint(ctx, "workflow.beforeExecute", workflow)
	}

	e.eventBus.EmitWorkflowStarted(workflow.ID, len(workflow.Tasks))

	execution.mu.Lock()
	execution.EventLog = append(execution.EventLog, shared.EventLogEntry{
		Timestamp: shared.Now(),
		Event:     "workflow:started",
		Data:      map[string]interface{}{"workflowId": workflow.ID},
	})
	execution.mu.Unlock()

	result, err := e.runWorkflow(ctx, execution, workflow)

	if err != nil {
		failedResult := shared.WorkflowResult{
			ID:             workflow.ID,
			Status:         "failed",
			TasksCompleted: len(execution.State.CompletedTasks),
			Errors:         []error{err},
			ErrorMessages:  []string{err.Error()},
			ExecutionOrder: execution.ExecutionOrder,
		}

		// Handle rollback if configured
		if workflow.RollbackOnFailure {
			e.rollbackWorkflow(ctx, execution, workflow)
		}

		e.eventBus.EmitWorkflowFailed(workflow.ID, err)
		return failedResult, nil
	}

	// Invoke after execute hook
	if e.pluginManager != nil {
		e.pluginManager.InvokeExtensionPoint(ctx, "workflow.afterExecute", result)
	}

	e.eventBus.EmitWorkflowCompleted(workflow.ID, result)

	return result, nil
}

// StartWorkflow starts a workflow asynchronously.
func (e *Engine) StartWorkflow(ctx context.Context, workflow shared.WorkflowDefinition) (<-chan shared.WorkflowResult, error) {
	resultChan := make(chan shared.WorkflowResult, 1)

	go func() {
		result, _ := e.ExecuteWorkflow(ctx, workflow)
		resultChan <- result
		close(resultChan)
	}()

	return resultChan, nil
}

// PauseWorkflow pauses a workflow.
func (e *Engine) PauseWorkflow(workflowID string) error {
	e.mu.RLock()
	execution := e.workflows[workflowID]
	e.mu.RUnlock()

	if execution == nil {
		return errors.New("workflow not found")
	}

	execution.mu.Lock()
	defer execution.mu.Unlock()

	if execution.State.Status == shared.WorkflowStatusInProgress {
		execution.State.Status = shared.WorkflowStatusPaused
		execution.EventLog = append(execution.EventLog, shared.EventLogEntry{
			Timestamp: shared.Now(),
			Event:     "workflow:paused",
			Data:      map[string]interface{}{"workflowId": workflowID},
		})
	}

	return nil
}

// ResumeWorkflow resumes a paused workflow.
func (e *Engine) ResumeWorkflow(workflowID string) error {
	e.mu.RLock()
	execution := e.workflows[workflowID]
	e.mu.RUnlock()

	if execution == nil {
		return errors.New("workflow not found")
	}

	execution.mu.Lock()
	defer execution.mu.Unlock()

	if execution.State.Status == shared.WorkflowStatusPaused {
		execution.State.Status = shared.WorkflowStatusInProgress
		execution.EventLog = append(execution.EventLog, shared.EventLogEntry{
			Timestamp: shared.Now(),
			Event:     "workflow:resumed",
			Data:      map[string]interface{}{"workflowId": workflowID},
		})
	}

	return nil
}

// CancelWorkflow cancels a workflow.
func (e *Engine) CancelWorkflow(workflowID string) error {
	e.mu.RLock()
	execution := e.workflows[workflowID]
	e.mu.RUnlock()

	if execution == nil {
		return errors.New("workflow not found")
	}

	execution.mu.Lock()
	defer execution.mu.Unlock()

	execution.State.Status = shared.WorkflowStatusCancelled
	if execution.cancel != nil {
		execution.cancel()
	}

	return nil
}

// GetWorkflowState returns the workflow state.
func (e *Engine) GetWorkflowState(workflowID string) (shared.WorkflowState, error) {
	e.mu.RLock()
	execution := e.workflows[workflowID]
	e.mu.RUnlock()

	if execution == nil {
		return shared.WorkflowState{}, errors.New("workflow not found")
	}

	execution.mu.RLock()
	defer execution.mu.RUnlock()

	return execution.State, nil
}

// ExecuteParallel executes tasks in parallel.
func (e *Engine) ExecuteParallel(ctx context.Context, tasks []shared.Task) ([]shared.TaskResult, error) {
	return e.coordinator.ExecuteTasksConcurrently(ctx, tasks)
}

// ExecuteDistributedWorkflow executes a workflow across multiple coordinators.
func (e *Engine) ExecuteDistributedWorkflow(ctx context.Context, workflow shared.WorkflowDefinition, coordinators []*coordinator.SwarmCoordinator) (shared.WorkflowResult, error) {
	// Distribute tasks across coordinators
	tasksPerCoordinator := (len(workflow.Tasks) + len(coordinators) - 1) / len(coordinators)

	results := make([]shared.TaskResult, 0)
	errs := make([]error, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < len(workflow.Tasks); i += tasksPerCoordinator {
		end := i + tasksPerCoordinator
		if end > len(workflow.Tasks) {
			end = len(workflow.Tasks)
		}

		taskChunk := workflow.Tasks[i:end]
		coordIdx := i / tasksPerCoordinator % len(coordinators)
		coord := coordinators[coordIdx]

		wg.Add(1)
		go func(tasks []shared.Task, c *coordinator.SwarmCoordinator) {
			defer wg.Done()

			for _, t := range tasks {
				agents := c.ListAgents()
				if len(agents) == 0 {
					mu.Lock()
					errs = append(errs, fmt.Errorf("no agents available for task %s", t.ID))
					mu.Unlock()
					continue
				}

				result, err := c.ExecuteTask(ctx, agents[0].ID, t)
				mu.Lock()
				results = append(results, result)
				if err != nil || result.Status == shared.TaskStatusFailed {
					errs = append(errs, errors.New(result.Error))
				}
				mu.Unlock()
			}
		}(taskChunk, coord)
	}

	wg.Wait()

	completedCount := 0
	for _, r := range results {
		if r.Status == shared.TaskStatusCompleted {
			completedCount++
		}
	}

	status := "completed"
	if len(errs) > 0 {
		status = "failed"
	}

	executionOrder := make([]string, len(workflow.Tasks))
	for i, t := range workflow.Tasks {
		executionOrder[i] = t.ID
	}

	errorMessages := make([]string, len(errs))
	for i, err := range errs {
		errorMessages[i] = err.Error()
	}

	return shared.WorkflowResult{
		ID:             workflow.ID,
		Status:         status,
		TasksCompleted: completedCount,
		Errors:         errs,
		ErrorMessages:  errorMessages,
		ExecutionOrder: executionOrder,
	}, nil
}

// GetWorkflowMetrics returns workflow metrics.
func (e *Engine) GetWorkflowMetrics(workflowID string) (shared.WorkflowMetrics, error) {
	e.mu.RLock()
	execution := e.workflows[workflowID]
	e.mu.RUnlock()

	if execution == nil {
		return shared.WorkflowMetrics{}, errors.New("workflow not found")
	}

	execution.mu.RLock()
	defer execution.mu.RUnlock()

	totalTasks := len(execution.State.Tasks)
	completedTasks := len(execution.State.CompletedTasks)

	var totalDuration int64
	for _, timing := range execution.TaskTimings {
		totalDuration += timing.Duration
	}

	var avgDuration float64
	if len(execution.TaskTimings) > 0 {
		avgDuration = float64(totalDuration) / float64(len(execution.TaskTimings))
	}

	var successRate float64
	if totalTasks > 0 {
		successRate = float64(completedTasks) / float64(totalTasks)
	}

	return shared.WorkflowMetrics{
		TasksTotal:          totalTasks,
		TasksCompleted:      completedTasks,
		TotalDuration:       totalDuration,
		AverageTaskDuration: avgDuration,
		SuccessRate:         successRate,
	}, nil
}

// GetWorkflowDebugInfo returns workflow debug information.
func (e *Engine) GetWorkflowDebugInfo(workflowID string) (shared.WorkflowDebugInfo, error) {
	e.mu.RLock()
	execution := e.workflows[workflowID]
	e.mu.RUnlock()

	if execution == nil {
		return shared.WorkflowDebugInfo{}, errors.New("workflow not found")
	}

	execution.mu.RLock()
	defer execution.mu.RUnlock()

	trace := make([]shared.ExecutionTraceEntry, len(execution.ExecutionOrder))
	for i, taskID := range execution.ExecutionOrder {
		timing := execution.TaskTimings[taskID]
		trace[i] = shared.ExecutionTraceEntry{
			TaskID:    taskID,
			Timestamp: timing.Start,
			Action:    "execute",
		}
	}

	return shared.WorkflowDebugInfo{
		ExecutionTrace:  trace,
		TaskTimings:     execution.TaskTimings,
		MemorySnapshots: execution.MemorySnapshots,
		EventLog:        execution.EventLog,
	}, nil
}

// RestoreWorkflow restores a workflow from persisted state.
func (e *Engine) RestoreWorkflow(workflowID string) (shared.WorkflowState, error) {
	if e.memoryBackend == nil {
		return shared.WorkflowState{}, errors.New("memory backend not available")
	}

	memory, err := e.memoryBackend.Retrieve(fmt.Sprintf("workflow-state-%s", workflowID))
	if err != nil {
		return shared.WorkflowState{}, err
	}
	if memory == nil {
		return shared.WorkflowState{}, errors.New("workflow state not found")
	}

	// Parse state from memory content
	// In a real implementation, this would deserialize the state
	return shared.WorkflowState{
		ID:     workflowID,
		Status: shared.WorkflowStatusPending,
	}, nil
}

// ============================================================================
// Private Helper Methods
// ============================================================================

func (e *Engine) createExecution(workflow shared.WorkflowDefinition) *WorkflowExecution {
	return &WorkflowExecution{
		ID: workflow.ID,
		State: shared.WorkflowState{
			ID:             workflow.ID,
			Name:           workflow.Name,
			Tasks:          workflow.Tasks,
			Status:         shared.WorkflowStatusInProgress,
			CompletedTasks: make([]string, 0),
			StartedAt:      shared.Now(),
		},
		ExecutionOrder:  make([]string, 0),
		TaskTimings:     make(map[string]shared.TaskTiming),
		EventLog:        make([]shared.EventLogEntry, 0),
		MemorySnapshots: make([]shared.MemorySnapshot, 0),
	}
}

func (e *Engine) runWorkflow(ctx context.Context, execution *WorkflowExecution, workflow shared.WorkflowDefinition) (shared.WorkflowResult, error) {
	tasks := task.ConvertFromShared(workflow.Tasks)
	completedTasks := make(map[string]bool)
	var errs []error

	// Resolve execution order
	orderedTasks, err := task.ResolveExecutionOrder(tasks)
	if err != nil {
		return shared.WorkflowResult{}, err
	}

	for _, t := range orderedTasks {
		// Check context cancellation
		select {
		case <-ctx.Done():
			execution.mu.Lock()
			execution.State.Status = shared.WorkflowStatusCancelled
			execution.mu.Unlock()
			return shared.WorkflowResult{
				ID:             workflow.ID,
				Status:         "cancelled",
				TasksCompleted: len(completedTasks),
				ExecutionOrder: execution.ExecutionOrder,
			}, nil
		default:
		}

		// Check if paused
		for {
			execution.mu.RLock()
			status := execution.State.Status
			execution.mu.RUnlock()

			if status == shared.WorkflowStatusPaused {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if status == shared.WorkflowStatusCancelled {
				return shared.WorkflowResult{
					ID:             workflow.ID,
					Status:         "cancelled",
					TasksCompleted: len(completedTasks),
					ExecutionOrder: execution.ExecutionOrder,
				}, nil
			}
			break
		}

		execution.mu.Lock()
		execution.State.CurrentTask = t.ID
		execution.mu.Unlock()

		startTime := shared.Now()

		var taskErr error

		// Handle nested workflows
		if t.IsWorkflow() && t.Workflow != nil {
			nestedResult, _ := e.ExecuteWorkflow(ctx, *t.Workflow)
			if nestedResult.Status == "failed" {
				taskErr = errors.New("nested workflow failed")
			}
		} else {
			// Find assigned agent or distribute
			agentID := t.AssignedTo
			if agentID == "" {
				agents := e.coordinator.ListAgents()
				for _, a := range agents {
					if a.CanExecute(t.Type) {
						agentID = a.ID
						break
					}
				}
			}

			if agentID == "" {
				taskErr = fmt.Errorf("no agent available for task %s", t.ID)
			} else {
				result, _ := e.ExecuteTask(ctx, t.ToShared(), agentID)
				if result.Status == shared.TaskStatusFailed {
					taskErr = errors.New(result.Error)
				}
			}
		}

		endTime := shared.Now()

		execution.mu.Lock()
		execution.TaskTimings[t.ID] = shared.TaskTiming{
			Start:    startTime,
			End:      endTime,
			Duration: endTime - startTime,
		}
		execution.mu.Unlock()

		if taskErr != nil {
			errs = append(errs, taskErr)

			if workflow.RollbackOnFailure {
				return shared.WorkflowResult{}, taskErr
			}
			continue
		}

		completedTasks[t.ID] = true

		execution.mu.Lock()
		execution.State.CompletedTasks = append(execution.State.CompletedTasks, t.ID)
		execution.ExecutionOrder = append(execution.ExecutionOrder, t.ID)
		execution.EventLog = append(execution.EventLog, shared.EventLogEntry{
			Timestamp: endTime,
			Event:     "task:completed",
			Data: map[string]interface{}{
				"taskId":   t.ID,
				"duration": endTime - startTime,
			},
		})
		execution.mu.Unlock()

		e.eventBus.EmitWorkflowTaskComplete(workflow.ID, t.ID)
	}

	execution.mu.Lock()
	if len(errs) > 0 {
		execution.State.Status = shared.WorkflowStatusFailed
	} else {
		execution.State.Status = shared.WorkflowStatusCompleted
	}
	execution.State.CompletedAt = shared.Now()
	execution.mu.Unlock()

	status := "completed"
	if len(errs) > 0 {
		status = "failed"
	}

	errorMessages := make([]string, len(errs))
	for i, err := range errs {
		errorMessages[i] = err.Error()
	}

	return shared.WorkflowResult{
		ID:             workflow.ID,
		Status:         status,
		TasksCompleted: len(completedTasks),
		Errors:         errs,
		ErrorMessages:  errorMessages,
		ExecutionOrder: execution.ExecutionOrder,
		Duration:       execution.State.CompletedAt - execution.State.StartedAt,
	}, nil
}

func (e *Engine) rollbackWorkflow(ctx context.Context, execution *WorkflowExecution, workflow shared.WorkflowDefinition) {
	execution.mu.RLock()
	completedTaskIDs := make([]string, len(execution.State.CompletedTasks))
	copy(completedTaskIDs, execution.State.CompletedTasks)
	execution.mu.RUnlock()

	// Rollback in reverse order
	for i := len(completedTaskIDs) - 1; i >= 0; i-- {
		taskID := completedTaskIDs[i]

		for _, t := range workflow.Tasks {
			if t.ID == taskID && t.OnRollback != nil {
				err := t.OnRollback(ctx)

				execution.mu.Lock()
				if err != nil {
					execution.EventLog = append(execution.EventLog, shared.EventLogEntry{
						Timestamp: shared.Now(),
						Event:     "rollback:error",
						Data: map[string]interface{}{
							"taskId": taskID,
							"error":  err.Error(),
						},
					})
				} else {
					execution.EventLog = append(execution.EventLog, shared.EventLogEntry{
						Timestamp: shared.Now(),
						Event:     "task:rolledback",
						Data:      map[string]interface{}{"taskId": taskID},
					})
				}
				execution.mu.Unlock()
				break
			}
		}
	}
}
