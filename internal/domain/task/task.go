// Package task provides the Task domain entity.
package task

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Task represents a task to be executed by agents in the V3 system.
type Task struct {
	mu           sync.RWMutex
	ID           string
	Type         shared.TaskType
	Description  string
	Priority     shared.TaskPriority
	Status       shared.TaskStatus
	AssignedTo   string
	Dependencies []string
	Metadata     map[string]interface{}
	Workflow     *shared.WorkflowDefinition
	OnExecute    func(context.Context) error
	OnRollback   func(context.Context) error
	startedAt    int64
	completedAt  int64
}

// Config holds configuration for creating a task.
type Config struct {
	ID           string
	Type         shared.TaskType
	Description  string
	Priority     shared.TaskPriority
	Status       shared.TaskStatus
	AssignedTo   string
	Dependencies []string
	Metadata     map[string]interface{}
	Workflow     *shared.WorkflowDefinition
	OnExecute    func(context.Context) error
	OnRollback   func(context.Context) error
}

// New creates a new Task from the given configuration.
func New(config Config) *Task {
	status := config.Status
	if status == "" {
		status = shared.TaskStatusPending
	}

	dependencies := config.Dependencies
	if dependencies == nil {
		dependencies = []string{}
	}

	metadata := config.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &Task{
		ID:           config.ID,
		Type:         config.Type,
		Description:  config.Description,
		Priority:     config.Priority,
		Status:       status,
		AssignedTo:   config.AssignedTo,
		Dependencies: dependencies,
		Metadata:     metadata,
		Workflow:     config.Workflow,
		OnExecute:    config.OnExecute,
		OnRollback:   config.OnRollback,
	}
}

// FromShared creates a Task from a shared.Task.
func FromShared(t shared.Task) *Task {
	status := t.Status
	if status == "" {
		status = shared.TaskStatusPending
	}

	dependencies := t.Dependencies
	if dependencies == nil {
		dependencies = []string{}
	}

	metadata := t.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &Task{
		ID:           t.ID,
		Type:         t.Type,
		Description:  t.Description,
		Priority:     t.Priority,
		Status:       status,
		AssignedTo:   t.AssignedTo,
		Dependencies: dependencies,
		Metadata:     metadata,
		Workflow:     t.Workflow,
		OnExecute:    t.OnExecute,
		OnRollback:   t.OnRollback,
	}
}

// AreDependenciesResolved checks if all task dependencies are resolved.
func (t *Task) AreDependenciesResolved(completedTasks map[string]bool) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, dep := range t.Dependencies {
		if !completedTasks[dep] {
			return false
		}
	}
	return true
}

// Start marks the task as started.
func (t *Task) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Status == shared.TaskStatusPending {
		t.Status = shared.TaskStatusInProgress
		t.startedAt = shared.Now()
	}
}

// Complete marks the task as completed.
func (t *Task) Complete() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Status == shared.TaskStatusInProgress {
		t.Status = shared.TaskStatusCompleted
		t.completedAt = shared.Now()
	}
}

// Fail marks the task as failed.
func (t *Task) Fail(errMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = shared.TaskStatusFailed
	t.completedAt = shared.Now()
	if errMsg != "" {
		t.Metadata["error"] = errMsg
	}
}

// Cancel cancels the task.
func (t *Task) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Status != shared.TaskStatusCompleted && t.Status != shared.TaskStatusFailed {
		t.Status = shared.TaskStatusCancelled
		t.completedAt = shared.Now()
	}
}

// GetDuration returns the task duration in milliseconds.
func (t *Task) GetDuration() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.startedAt > 0 && t.completedAt > 0 {
		return t.completedAt - t.startedAt
	}
	if t.startedAt > 0 {
		return time.Now().UnixMilli() - t.startedAt
	}
	return 0
}

// IsWorkflow checks if the task is a nested workflow.
func (t *Task) IsWorkflow() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Type == shared.TaskTypeWorkflow && t.Workflow != nil
}

// AssignTo assigns the task to an agent.
func (t *Task) AssignTo(agentID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.AssignedTo = agentID
}

// GetPriorityValue returns the priority as a numeric value for sorting.
func (t *Task) GetPriorityValue() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	values := map[shared.TaskPriority]int{
		shared.PriorityHigh:   3,
		shared.PriorityMedium: 2,
		shared.PriorityLow:    1,
	}

	value, exists := values[t.Priority]
	if !exists {
		return 2 // default to medium
	}
	return value
}

// GetStatus returns the current status of the task.
func (t *Task) GetStatus() shared.TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status
}

// ToShared converts the Task to a shared.Task.
func (t *Task) ToShared() shared.Task {
	t.mu.RLock()
	defer t.mu.RUnlock()

	metadata := make(map[string]interface{})
	for k, v := range t.Metadata {
		metadata[k] = v
	}
	metadata["startedAt"] = t.startedAt
	metadata["completedAt"] = t.completedAt
	metadata["duration"] = t.GetDuration()

	return shared.Task{
		ID:           t.ID,
		Type:         t.Type,
		Description:  t.Description,
		Priority:     t.Priority,
		Status:       t.Status,
		AssignedTo:   t.AssignedTo,
		Dependencies: t.Dependencies,
		Metadata:     metadata,
		Workflow:     t.Workflow,
		OnExecute:    t.OnExecute,
		OnRollback:   t.OnRollback,
	}
}

// SortByPriority sorts tasks by priority (high to low).
func SortByPriority(tasks []*Task) []*Task {
	sorted := make([]*Task, len(tasks))
	copy(sorted, tasks)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].GetPriorityValue() > sorted[j].GetPriorityValue()
	})

	return sorted
}

// ResolveExecutionOrder resolves task execution order based on dependencies using topological sort.
func ResolveExecutionOrder(tasks []*Task) ([]*Task, error) {
	// Build a map of task IDs to tasks
	taskMap := make(map[string]*Task)
	for _, task := range tasks {
		taskMap[task.ID] = task
	}

	// Topological sort using Kahn's algorithm
	resolved := make([]*Task, 0, len(tasks))
	resolvedIDs := make(map[string]bool)
	remaining := make([]*Task, len(tasks))
	copy(remaining, tasks)

	for len(remaining) > 0 {
		// Find tasks with all dependencies resolved
		ready := make([]*Task, 0)
		notReady := make([]*Task, 0)

		for _, task := range remaining {
			if task.AreDependenciesResolved(resolvedIDs) {
				ready = append(ready, task)
			} else {
				notReady = append(notReady, task)
			}
		}

		if len(ready) == 0 && len(remaining) > 0 {
			return nil, errors.New("circular dependency detected in tasks")
		}

		// Sort ready tasks by priority
		ready = SortByPriority(ready)

		// Add ready tasks to resolved list
		for _, task := range ready {
			resolved = append(resolved, task)
			resolvedIDs[task.ID] = true
		}

		remaining = notReady
	}

	return resolved, nil
}

// ConvertFromShared converts a slice of shared.Task to []*Task.
func ConvertFromShared(tasks []shared.Task) []*Task {
	result := make([]*Task, len(tasks))
	for i, t := range tasks {
		result[i] = FromShared(t)
	}
	return result
}

// ConvertToShared converts a slice of *Task to []shared.Task.
func ConvertToShared(tasks []*Task) []shared.Task {
	result := make([]shared.Task, len(tasks))
	for i, t := range tasks {
		result[i] = t.ToShared()
	}
	return result
}
