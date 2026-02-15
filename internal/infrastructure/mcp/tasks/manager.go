// Package tasks provides the TaskManager for MCP task operations.
package tasks

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ProgressCallback is called when task progress is updated.
type ProgressCallback func(taskID string, progress float64)

// TaskManager manages tasks with async execution, queuing, and progress tracking.
type TaskManager struct {
	mu              sync.RWMutex
	tasks           map[string]*shared.ManagedTask
	queue           []*shared.ManagedTask
	runningCount    int
	config          shared.TaskManagerConfig
	coordinator     *coordinator.SwarmCoordinator
	onProgress      ProgressCallback
	stats           *shared.TaskManagerStats
	ctx             context.Context
	cancel          context.CancelFunc
	running         bool
	cleanupInterval time.Duration
}

// NewTaskManager creates a new TaskManager.
func NewTaskManager(config shared.TaskManagerConfig, coord *coordinator.SwarmCoordinator) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskManager{
		tasks:           make(map[string]*shared.ManagedTask),
		queue:           make([]*shared.ManagedTask, 0),
		config:          config,
		coordinator:     coord,
		stats:           &shared.TaskManagerStats{},
		ctx:             ctx,
		cancel:          cancel,
		cleanupInterval: time.Minute,
	}
}

// NewTaskManagerWithDefaults creates a TaskManager with default configuration.
func NewTaskManagerWithDefaults(coord *coordinator.SwarmCoordinator) *TaskManager {
	return NewTaskManager(shared.DefaultTaskManagerConfig(), coord)
}

// Initialize starts the TaskManager.
func (tm *TaskManager) Initialize() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.running {
		return nil
	}

	tm.running = true

	// Start cleanup goroutine
	go tm.cleanupLoop()

	// Start queue processor
	go tm.processQueue()

	return nil
}

// Shutdown stops the TaskManager.
func (tm *TaskManager) Shutdown() error {
	tm.mu.Lock()
	if !tm.running {
		tm.mu.Unlock()
		return nil
	}
	tm.running = false
	tm.mu.Unlock()

	tm.cancel()
	return nil
}

// CreateTask creates a new task.
func (tm *TaskManager) CreateTask(req *shared.TaskCreateRequest) (*shared.ManagedTask, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Validate priority
	priority := req.Priority
	if priority < 1 {
		priority = 5 // Default priority
	}
	if priority > 10 {
		return nil, shared.ErrInvalidTaskPriority
	}

	// Check queue capacity
	if len(tm.queue) >= tm.config.QueueSize {
		return nil, shared.ErrTaskQueueFull
	}

	// Check for circular dependencies
	if len(req.Dependencies) > 0 {
		if err := tm.checkCircularDependencies(req.Dependencies); err != nil {
			return nil, err
		}
	}

	// Generate ID
	taskID := shared.GenerateID("task")

	// Determine timeout
	timeoutMs := req.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = tm.config.DefaultTimeoutMs
	}

	// Create managed task
	task := &shared.ManagedTask{
		ID:           taskID,
		Type:         req.Type,
		Description:  req.Description,
		Priority:     priority,
		Status:       shared.ManagedTaskStatusPending,
		AssignedTo:   req.AssignToAgent,
		Dependencies: req.Dependencies,
		Metadata:     req.Metadata,
		Input:        req.Input,
		Progress:     0.0,
		CreatedAt:    shared.Now(),
		TimeoutMs:    timeoutMs,
		History:      []shared.TaskHistoryEntry{},
		Artifacts:    []shared.TaskArtifact{},
	}

	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}
	if task.Dependencies == nil {
		task.Dependencies = []string{}
	}

	// Add history entry
	task.History = append(task.History, shared.TaskHistoryEntry{
		Timestamp: shared.Now(),
		Event:     "created",
		Details:   map[string]interface{}{"priority": priority},
	})

	// Store task
	tm.tasks[taskID] = task
	tm.stats.TotalTasks++
	tm.stats.PendingTasks++

	// Add to queue
	tm.enqueue(task)

	return task, nil
}

// GetTask retrieves a task by ID.
func (tm *TaskManager) GetTask(id string) (*shared.ManagedTask, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[id]
	if !exists {
		return nil, shared.ErrTaskNotFound
	}

	return task, nil
}

// ListTasks lists tasks with optional filtering.
func (tm *TaskManager) ListTasks(filter *shared.TaskFilter) *shared.TaskListResult {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Collect matching tasks
	var matches []*shared.ManagedTask
	for _, task := range tm.tasks {
		if tm.matchesFilter(task, filter) {
			matches = append(matches, task)
		}
	}

	// Sort
	tm.sortTasks(matches, filter)

	// Apply pagination
	total := len(matches)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	return &shared.TaskListResult{
		Tasks:  matches[start:end],
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
}

// CancelTask cancels a task.
func (tm *TaskManager) CancelTask(id string, reason string, force bool) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[id]
	if !exists {
		return shared.ErrTaskNotFound
	}

	// Check if cancellable
	if !force {
		switch task.Status {
		case shared.ManagedTaskStatusCompleted, shared.ManagedTaskStatusFailed, shared.ManagedTaskStatusCancelled:
			return shared.ErrTaskNotCancellable
		}
	}

	prevStatus := task.Status
	task.Status = shared.ManagedTaskStatusCancelled
	task.CompletedAt = shared.Now()
	task.Error = reason

	// Update stats
	tm.updateStatsForStatusChange(prevStatus, shared.ManagedTaskStatusCancelled)

	// Add history entry
	task.History = append(task.History, shared.TaskHistoryEntry{
		Timestamp: shared.Now(),
		Event:     "cancelled",
		Details:   map[string]interface{}{"reason": reason, "force": force},
	})

	// Remove from queue if pending/queued
	tm.removeFromQueue(id)

	return nil
}

// AssignTask assigns a task to an agent.
func (tm *TaskManager) AssignTask(id, agentID string, reassign bool) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[id]
	if !exists {
		return shared.ErrTaskNotFound
	}

	// Check if already assigned
	if task.AssignedTo != "" && !reassign {
		return shared.ErrTaskAlreadyAssigned
	}

	prevAgent := task.AssignedTo
	task.AssignedTo = agentID

	// Add history entry
	task.History = append(task.History, shared.TaskHistoryEntry{
		Timestamp: shared.Now(),
		Event:     "assigned",
		Details:   map[string]interface{}{"agentId": agentID, "previousAgent": prevAgent},
	})

	return nil
}

// UpdateTask updates task properties.
func (tm *TaskManager) UpdateTask(id string, update *shared.TaskUpdate) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[id]
	if !exists {
		return shared.ErrTaskNotFound
	}

	changes := make(map[string]interface{})

	if update.Priority != nil {
		if *update.Priority < 1 || *update.Priority > 10 {
			return shared.ErrInvalidTaskPriority
		}
		changes["priority"] = map[string]interface{}{"from": task.Priority, "to": *update.Priority}
		task.Priority = *update.Priority
	}

	if update.Description != nil {
		changes["description"] = "updated"
		task.Description = *update.Description
	}

	if update.TimeoutMs != nil {
		changes["timeoutMs"] = map[string]interface{}{"from": task.TimeoutMs, "to": *update.TimeoutMs}
		task.TimeoutMs = *update.TimeoutMs
	}

	if update.Metadata != nil {
		for k, v := range update.Metadata {
			task.Metadata[k] = v
		}
		changes["metadata"] = "updated"
	}

	// Add history entry
	task.History = append(task.History, shared.TaskHistoryEntry{
		Timestamp: shared.Now(),
		Event:     "updated",
		Details:   changes,
	})

	return nil
}

// UpdateDependencies manages task dependencies.
func (tm *TaskManager) UpdateDependencies(id string, action shared.TaskDependencyAction, deps []string) ([]string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[id]
	if !exists {
		return nil, shared.ErrTaskNotFound
	}

	switch action {
	case shared.TaskDependencyActionAdd:
		// Check for circular dependencies
		allDeps := append(task.Dependencies, deps...)
		if err := tm.checkCircularDependencies(allDeps); err != nil {
			return nil, err
		}
		for _, dep := range deps {
			if !tm.containsDep(task.Dependencies, dep) {
				task.Dependencies = append(task.Dependencies, dep)
			}
		}

	case shared.TaskDependencyActionRemove:
		newDeps := make([]string, 0)
		for _, d := range task.Dependencies {
			if !tm.containsDep(deps, d) {
				newDeps = append(newDeps, d)
			}
		}
		task.Dependencies = newDeps

	case shared.TaskDependencyActionClear:
		task.Dependencies = []string{}

	case shared.TaskDependencyActionList:
		// Just return current dependencies
	}

	// Add history entry
	task.History = append(task.History, shared.TaskHistoryEntry{
		Timestamp: shared.Now(),
		Event:     "dependencies_" + string(action),
		Details:   map[string]interface{}{"dependencies": task.Dependencies},
	})

	return task.Dependencies, nil
}

// GetResults retrieves task results.
func (tm *TaskManager) GetResults(id string, format shared.TaskResultFormat, includeArtifacts bool) (*shared.ManagedTaskResult, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[id]
	if !exists {
		return nil, shared.ErrTaskNotFound
	}

	result := &shared.ManagedTaskResult{
		TaskID:      task.ID,
		Status:      task.Status,
		CompletedAt: task.CompletedAt,
	}

	// Calculate execution time
	if task.StartedAt > 0 && task.CompletedAt > 0 {
		result.ExecutionTime = task.CompletedAt - task.StartedAt
	}

	switch format {
	case shared.TaskResultFormatRaw:
		result.Output = task.Output
		result.Error = task.Error
		if includeArtifacts {
			result.Artifacts = task.Artifacts
		}

	case shared.TaskResultFormatDetailed:
		result.Output = task.Output
		result.Error = task.Error
		if includeArtifacts {
			result.Artifacts = task.Artifacts
		}

	case shared.TaskResultFormatSummary:
		// Only include summary info
		if task.Error != "" {
			result.Error = task.Error
		}
	}

	return result, nil
}

// ReportProgress updates task progress.
func (tm *TaskManager) ReportProgress(id string, progress float64) error {
	tm.mu.Lock()
	task, exists := tm.tasks[id]
	if !exists {
		tm.mu.Unlock()
		return shared.ErrTaskNotFound
	}

	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	task.Progress = progress
	callback := tm.onProgress
	tm.mu.Unlock()

	// Notify callback
	if callback != nil {
		callback(id, progress)
	}

	return nil
}

// SetProgressCallback sets the progress callback.
func (tm *TaskManager) SetProgressCallback(callback ProgressCallback) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.onProgress = callback
}

// GetStats returns TaskManager statistics.
func (tm *TaskManager) GetStats() *shared.TaskManagerStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	// Return a copy
	stats := *tm.stats
	return &stats
}

// GetConfig returns the TaskManager configuration.
func (tm *TaskManager) GetConfig() shared.TaskManagerConfig {
	return tm.config
}

// ============================================================================
// Internal Methods
// ============================================================================

func (tm *TaskManager) enqueue(task *shared.ManagedTask) {
	task.Status = shared.ManagedTaskStatusQueued
	task.QueuePosition = len(tm.queue) + 1
	tm.queue = append(tm.queue, task)

	// Sort queue by priority (higher priority first)
	sort.SliceStable(tm.queue, func(i, j int) bool {
		return tm.queue[i].Priority > tm.queue[j].Priority
	})

	// Update queue positions
	for i, t := range tm.queue {
		t.QueuePosition = i + 1
	}

	tm.stats.PendingTasks--
	tm.stats.QueuedTasks++

	task.History = append(task.History, shared.TaskHistoryEntry{
		Timestamp: shared.Now(),
		Event:     "queued",
		Details:   map[string]interface{}{"position": task.QueuePosition},
	})
}

func (tm *TaskManager) removeFromQueue(id string) {
	for i, t := range tm.queue {
		if t.ID == id {
			tm.queue = append(tm.queue[:i], tm.queue[i+1:]...)
			// Update positions
			for j := i; j < len(tm.queue); j++ {
				tm.queue[j].QueuePosition = j + 1
			}
			break
		}
	}
}

func (tm *TaskManager) processQueue() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.tryStartTasks()
		}
	}
}

func (tm *TaskManager) tryStartTasks() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for tm.runningCount < tm.config.MaxConcurrent && len(tm.queue) > 0 {
		// Find a task with resolved dependencies
		var taskToRun *shared.ManagedTask
		var taskIndex int

		for i, task := range tm.queue {
			if tm.areDependenciesResolved(task) {
				taskToRun = task
				taskIndex = i
				break
			}
		}

		if taskToRun == nil {
			break
		}

		// Remove from queue
		tm.queue = append(tm.queue[:taskIndex], tm.queue[taskIndex+1:]...)
		for j := taskIndex; j < len(tm.queue); j++ {
			tm.queue[j].QueuePosition = j + 1
		}

		// Start the task
		tm.startTask(taskToRun)
	}
}

func (tm *TaskManager) startTask(task *shared.ManagedTask) {
	task.Status = shared.ManagedTaskStatusRunning
	task.StartedAt = shared.Now()
	task.QueuePosition = 0
	tm.runningCount++
	tm.stats.QueuedTasks--
	tm.stats.RunningTasks++

	task.History = append(task.History, shared.TaskHistoryEntry{
		Timestamp: shared.Now(),
		Event:     "started",
		Details:   nil,
	})

	// Execute in goroutine
	go tm.executeTask(task)
}

func (tm *TaskManager) executeTask(task *shared.ManagedTask) {
	// Create timeout context
	ctx, cancel := context.WithTimeout(tm.ctx, time.Duration(task.TimeoutMs)*time.Millisecond)
	defer cancel()

	var err error
	var output interface{}

	// Execute via coordinator if available
	if tm.coordinator != nil && task.AssignedTo != "" {
		// Create a shared.Task for the coordinator
		sharedTask := shared.Task{
			ID:          task.ID,
			Type:        task.Type,
			Description: task.Description,
			Priority:    shared.IntToPriority(task.Priority),
			Status:      shared.TaskStatusPending,
			AssignedTo:  task.AssignedTo,
			Metadata:    task.Metadata,
		}

		result, execErr := tm.coordinator.ExecuteTask(ctx, task.AssignedTo, sharedTask)
		if execErr != nil {
			err = execErr
		} else {
			output = result
		}
	} else {
		// Simulate task execution for unassigned tasks
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-time.After(100 * time.Millisecond):
			output = map[string]interface{}{"status": "completed", "taskId": task.ID}
		}
	}

	// Complete the task
	tm.completeTask(task, output, err)
}

func (tm *TaskManager) completeTask(task *shared.ManagedTask, output interface{}, err error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task.CompletedAt = shared.Now()
	task.Output = output
	tm.runningCount--
	tm.stats.RunningTasks--

	// Calculate metrics
	task.Metrics = &shared.TaskMetrics{
		ExecutionTimeMs: task.CompletedAt - task.StartedAt,
		WaitTimeMs:      task.StartedAt - task.CreatedAt,
	}

	if err != nil {
		task.Status = shared.ManagedTaskStatusFailed
		task.Error = err.Error()
		tm.stats.FailedTasks++

		task.History = append(task.History, shared.TaskHistoryEntry{
			Timestamp: shared.Now(),
			Event:     "failed",
			Details:   map[string]interface{}{"error": err.Error()},
		})
	} else {
		task.Status = shared.ManagedTaskStatusCompleted
		task.Progress = 1.0
		tm.stats.CompletedTasks++

		task.History = append(task.History, shared.TaskHistoryEntry{
			Timestamp: shared.Now(),
			Event:     "completed",
			Details:   map[string]interface{}{"executionTimeMs": task.Metrics.ExecutionTimeMs},
		})
	}

	// Update average times
	tm.updateAverages(task.Metrics)
}

func (tm *TaskManager) updateAverages(metrics *shared.TaskMetrics) {
	completed := tm.stats.CompletedTasks + tm.stats.FailedTasks
	if completed == 0 {
		return
	}

	// Running average
	n := float64(completed)
	tm.stats.AvgExecTimeMs = (tm.stats.AvgExecTimeMs*(n-1) + float64(metrics.ExecutionTimeMs)) / n
	tm.stats.AvgWaitTimeMs = (tm.stats.AvgWaitTimeMs*(n-1) + float64(metrics.WaitTimeMs)) / n
}

func (tm *TaskManager) areDependenciesResolved(task *shared.ManagedTask) bool {
	for _, depID := range task.Dependencies {
		dep, exists := tm.tasks[depID]
		if !exists {
			return false
		}
		if dep.Status != shared.ManagedTaskStatusCompleted {
			return false
		}
	}
	return true
}

func (tm *TaskManager) checkCircularDependencies(deps []string) error {
	// Simple check: ensure none of the dependencies reference tasks that don't exist yet
	// or create cycles. This is a simplified check.
	visited := make(map[string]bool)
	
	var check func(id string) bool
	check = func(id string) bool {
		if visited[id] {
			return true // Cycle detected
		}
		visited[id] = true
		
		task, exists := tm.tasks[id]
		if !exists {
			return false
		}
		
		for _, dep := range task.Dependencies {
			if check(dep) {
				return true
			}
		}
		
		visited[id] = false
		return false
	}
	
	for _, dep := range deps {
		if check(dep) {
			return shared.ErrCircularDependency
		}
	}
	
	return nil
}

func (tm *TaskManager) matchesFilter(task *shared.ManagedTask, filter *shared.TaskFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Status != "" && task.Status != filter.Status {
		return false
	}

	if filter.AgentID != "" && task.AssignedTo != filter.AgentID {
		return false
	}

	if filter.Type != "" && task.Type != filter.Type {
		return false
	}

	if filter.Priority > 0 && task.Priority != filter.Priority {
		return false
	}

	return true
}

func (tm *TaskManager) sortTasks(tasks []*shared.ManagedTask, filter *shared.TaskFilter) {
	if filter == nil {
		return
	}

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created"
	}

	ascending := strings.ToLower(filter.SortOrder) == "asc"

	sort.SliceStable(tasks, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "priority":
			less = tasks[i].Priority < tasks[j].Priority
		case "status":
			less = tasks[i].Status < tasks[j].Status
		case "updated":
			less = tasks[i].CompletedAt < tasks[j].CompletedAt
		default: // "created"
			less = tasks[i].CreatedAt < tasks[j].CreatedAt
		}

		if ascending {
			return less
		}
		return !less
	})
}

func (tm *TaskManager) containsDep(deps []string, dep string) bool {
	for _, d := range deps {
		if d == dep {
			return true
		}
	}
	return false
}

func (tm *TaskManager) updateStatsForStatusChange(from, to shared.ManagedTaskStatus) {
	switch from {
	case shared.ManagedTaskStatusPending:
		tm.stats.PendingTasks--
	case shared.ManagedTaskStatusQueued:
		tm.stats.QueuedTasks--
	case shared.ManagedTaskStatusRunning:
		tm.stats.RunningTasks--
	}

	switch to {
	case shared.ManagedTaskStatusPending:
		tm.stats.PendingTasks++
	case shared.ManagedTaskStatusQueued:
		tm.stats.QueuedTasks++
	case shared.ManagedTaskStatusRunning:
		tm.stats.RunningTasks++
	case shared.ManagedTaskStatusCompleted:
		tm.stats.CompletedTasks++
	case shared.ManagedTaskStatusFailed:
		tm.stats.FailedTasks++
	case shared.ManagedTaskStatusCancelled:
		tm.stats.CancelledTasks++
	}
}

func (tm *TaskManager) cleanupLoop() {
	ticker := time.NewTicker(tm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.cleanup()
		}
	}
}

func (tm *TaskManager) cleanup() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := shared.Now()
	retentionMs := tm.config.RetentionPeriodMs

	for id, task := range tm.tasks {
		// Only clean up completed/failed/cancelled tasks
		switch task.Status {
		case shared.ManagedTaskStatusCompleted, shared.ManagedTaskStatusFailed, shared.ManagedTaskStatusCancelled:
			if task.CompletedAt > 0 && (now-task.CompletedAt) > retentionMs {
				delete(tm.tasks, id)
			}
		}
	}
}
