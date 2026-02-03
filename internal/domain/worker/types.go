// Package worker provides domain types for background worker management.
package worker

import (
	"time"
)

// WorkerTrigger represents a type of background worker task.
type WorkerTrigger string

const (
	// TriggerUltralearn is for deep learning from trajectories.
	TriggerUltralearn WorkerTrigger = "ultralearn"

	// TriggerOptimize is for pattern optimization.
	TriggerOptimize WorkerTrigger = "optimize"

	// TriggerConsolidate is for memory consolidation.
	TriggerConsolidate WorkerTrigger = "consolidate"

	// TriggerPredict is for predictive analysis.
	TriggerPredict WorkerTrigger = "predict"

	// TriggerAudit is for security/quality audit.
	TriggerAudit WorkerTrigger = "audit"

	// TriggerMap is for codebase mapping.
	TriggerMap WorkerTrigger = "map"

	// TriggerPreload is for preloading embeddings.
	TriggerPreload WorkerTrigger = "preload"

	// TriggerDeepdive is for deep code analysis.
	TriggerDeepdive WorkerTrigger = "deepdive"

	// TriggerDocument is for auto-documentation.
	TriggerDocument WorkerTrigger = "document"

	// TriggerRefactor is for refactoring suggestions.
	TriggerRefactor WorkerTrigger = "refactor"

	// TriggerBenchmark is for performance benchmarking.
	TriggerBenchmark WorkerTrigger = "benchmark"

	// TriggerTestgaps is for test coverage gap analysis.
	TriggerTestgaps WorkerTrigger = "testgaps"
)

// AllTriggers returns all available worker triggers.
func AllTriggers() []WorkerTrigger {
	return []WorkerTrigger{
		TriggerUltralearn, TriggerOptimize, TriggerConsolidate, TriggerPredict,
		TriggerAudit, TriggerMap, TriggerPreload, TriggerDeepdive,
		TriggerDocument, TriggerRefactor, TriggerBenchmark, TriggerTestgaps,
	}
}

// IsValid checks if the trigger is a valid trigger type.
func (t WorkerTrigger) IsValid() bool {
	for _, trigger := range AllTriggers() {
		if trigger == t {
			return true
		}
	}
	return false
}

// WorkerStatus represents the lifecycle state of a worker.
type WorkerStatus string

const (
	// StatusPending means the worker is queued but not yet started.
	StatusPending WorkerStatus = "pending"

	// StatusRunning means the worker is currently executing.
	StatusRunning WorkerStatus = "running"

	// StatusCompleted means the worker finished successfully.
	StatusCompleted WorkerStatus = "completed"

	// StatusFailed means the worker encountered an error.
	StatusFailed WorkerStatus = "failed"

	// StatusCancelled means the worker was cancelled.
	StatusCancelled WorkerStatus = "cancelled"
)

// AllStatuses returns all worker statuses.
func AllStatuses() []WorkerStatus {
	return []WorkerStatus{StatusPending, StatusRunning, StatusCompleted, StatusFailed, StatusCancelled}
}

// IsTerminal returns true if the status is a terminal state.
func (s WorkerStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}

// WorkerPriority represents the execution priority of a worker.
type WorkerPriority string

const (
	// PriorityLow is for background tasks.
	PriorityLow WorkerPriority = "low"

	// PriorityNormal is the default priority.
	PriorityNormal WorkerPriority = "normal"

	// PriorityHigh is for important tasks.
	PriorityHigh WorkerPriority = "high"

	// PriorityCritical is for urgent tasks.
	PriorityCritical WorkerPriority = "critical"
)

// AllPriorities returns all worker priorities.
func AllPriorities() []WorkerPriority {
	return []WorkerPriority{PriorityLow, PriorityNormal, PriorityHigh, PriorityCritical}
}

// Weight returns a numeric weight for the priority (higher = more important).
func (p WorkerPriority) Weight() int {
	switch p {
	case PriorityCritical:
		return 4
	case PriorityHigh:
		return 3
	case PriorityNormal:
		return 2
	case PriorityLow:
		return 1
	default:
		return 2
	}
}

// WorkerResult contains the result of a completed worker.
type WorkerResult struct {
	// Summary is a brief description of the result.
	Summary string `json:"summary,omitempty"`

	// Data contains the result data.
	Data map[string]interface{} `json:"data,omitempty"`

	// Artifacts are any files or outputs produced.
	Artifacts []string `json:"artifacts,omitempty"`

	// Metrics contains performance metrics.
	Metrics map[string]float64 `json:"metrics,omitempty"`
}

// WorkerInstance represents an active or completed worker.
type WorkerInstance struct {
	// ID is the unique worker identifier.
	ID string `json:"id"`

	// Trigger is the type of worker task.
	Trigger WorkerTrigger `json:"trigger"`

	// SessionID is the session that owns this worker.
	SessionID string `json:"sessionId"`

	// Context is the input context for the worker.
	Context string `json:"context"`

	// Status is the current worker status.
	Status WorkerStatus `json:"status"`

	// Priority is the worker priority.
	Priority WorkerPriority `json:"priority"`

	// Progress is the completion percentage (0-100).
	Progress int `json:"progress"`

	// Phase is the current execution phase.
	Phase string `json:"phase,omitempty"`

	// StartedAt is when the worker started.
	StartedAt time.Time `json:"startedAt"`

	// CompletedAt is when the worker completed.
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// Timeout is the worker timeout duration.
	Timeout time.Duration `json:"timeout,omitempty"`

	// Result contains the worker result.
	Result *WorkerResult `json:"result,omitempty"`

	// Error contains any error message.
	Error string `json:"error,omitempty"`

	// Metadata contains additional metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// CancelFunc is used to cancel the worker.
	CancelFunc func() `json:"-"`
}

// IsTerminal returns true if the worker is in a terminal state.
func (w *WorkerInstance) IsTerminal() bool {
	return w.Status.IsTerminal()
}

// Duration returns the worker execution duration.
func (w *WorkerInstance) Duration() time.Duration {
	if w.CompletedAt != nil {
		return w.CompletedAt.Sub(w.StartedAt)
	}
	return time.Since(w.StartedAt)
}

// DispatchOptions contains options for dispatching a worker.
type DispatchOptions struct {
	// Priority is the worker priority.
	Priority WorkerPriority `json:"priority,omitempty"`

	// Timeout is the worker timeout.
	Timeout time.Duration `json:"timeout,omitempty"`

	// Metadata contains additional metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// WorkerPool represents a pool of workers.
type WorkerPool struct {
	// Size is the current pool size.
	Size int `json:"size"`

	// MinWorkers is the minimum number of workers.
	MinWorkers int `json:"minWorkers"`

	// MaxWorkers is the maximum number of workers.
	MaxWorkers int `json:"maxWorkers"`

	// ActiveWorkers is the number of active workers.
	ActiveWorkers int `json:"activeWorkers"`

	// IdleWorkers is the number of idle workers.
	IdleWorkers int `json:"idleWorkers"`

	// PendingTasks is the number of pending tasks.
	PendingTasks int `json:"pendingTasks"`

	// AutoScale indicates if auto-scaling is enabled.
	AutoScale bool `json:"autoScale"`

	// ScaleUpThreshold is the load threshold for scaling up.
	ScaleUpThreshold float64 `json:"scaleUpThreshold"`

	// ScaleDownThreshold is the load threshold for scaling down.
	ScaleDownThreshold float64 `json:"scaleDownThreshold"`
}

// Load returns the current pool load (0-1).
func (p *WorkerPool) Load() float64 {
	if p.Size == 0 {
		return 0
	}
	return float64(p.ActiveWorkers) / float64(p.Size)
}

// ShouldScaleUp returns true if the pool should scale up.
func (p *WorkerPool) ShouldScaleUp() bool {
	if !p.AutoScale || p.Size >= p.MaxWorkers {
		return false
	}
	return p.Load() > p.ScaleUpThreshold || p.PendingTasks > 0
}

// ShouldScaleDown returns true if the pool should scale down.
func (p *WorkerPool) ShouldScaleDown() bool {
	if !p.AutoScale || p.Size <= p.MinWorkers {
		return false
	}
	return p.Load() < p.ScaleDownThreshold && p.PendingTasks == 0
}

// WorkerStats contains aggregate worker statistics.
type WorkerStats struct {
	// Total is the total number of workers.
	Total int `json:"total"`

	// Pending is the number of pending workers.
	Pending int `json:"pending"`

	// Running is the number of running workers.
	Running int `json:"running"`

	// Completed is the number of completed workers.
	Completed int `json:"completed"`

	// Failed is the number of failed workers.
	Failed int `json:"failed"`

	// Cancelled is the number of cancelled workers.
	Cancelled int `json:"cancelled"`

	// ByTrigger is the count by trigger type.
	ByTrigger map[WorkerTrigger]int `json:"byTrigger"`

	// AvgDurationMs is the average duration in milliseconds.
	AvgDurationMs float64 `json:"avgDurationMs"`

	// SuccessRate is the success rate (0-1).
	SuccessRate float64 `json:"successRate"`
}

// TriggerConfig contains configuration for a trigger type.
type TriggerConfig struct {
	// Name is the trigger name.
	Name WorkerTrigger `json:"name"`

	// Description is a human-readable description.
	Description string `json:"description"`

	// Priority is the default priority.
	Priority WorkerPriority `json:"priority"`

	// EstimatedDuration is the estimated execution time.
	EstimatedDuration time.Duration `json:"estimatedDuration"`

	// Capabilities are the capabilities required.
	Capabilities []string `json:"capabilities"`

	// Keywords are keywords that detect this trigger.
	Keywords []string `json:"keywords"`
}

// TriggerDetectionResult contains trigger detection results.
type TriggerDetectionResult struct {
	// Detected indicates if any triggers were detected.
	Detected bool `json:"detected"`

	// Triggers is the list of detected triggers.
	Triggers []DetectedTrigger `json:"triggers"`
}

// DetectedTrigger represents a detected trigger in text.
type DetectedTrigger struct {
	// Trigger is the trigger type.
	Trigger WorkerTrigger `json:"trigger"`

	// Confidence is the detection confidence (0-1).
	Confidence float64 `json:"confidence"`

	// Keyword is the keyword that triggered detection.
	Keyword string `json:"keyword"`

	// Context is the surrounding context.
	Context string `json:"context,omitempty"`
}

// WorkerHealthStatus represents the health status of a worker.
type WorkerHealthStatus string

const (
	// HealthHealthy means the worker is healthy.
	HealthHealthy WorkerHealthStatus = "healthy"

	// HealthDegraded means the worker is degraded.
	HealthDegraded WorkerHealthStatus = "degraded"

	// HealthUnhealthy means the worker is unhealthy.
	HealthUnhealthy WorkerHealthStatus = "unhealthy"

	// HealthUnknown means the health status is unknown.
	HealthUnknown WorkerHealthStatus = "unknown"
)

// WorkerHealth contains health information for a worker.
type WorkerHealth struct {
	// WorkerID is the worker ID (empty for pool health).
	WorkerID string `json:"workerId,omitempty"`

	// Status is the health status.
	Status WorkerHealthStatus `json:"status"`

	// Message is a status message.
	Message string `json:"message,omitempty"`

	// Uptime is the worker uptime.
	Uptime time.Duration `json:"uptime,omitempty"`

	// LastActivity is the time of last activity.
	LastActivity *time.Time `json:"lastActivity,omitempty"`

	// MemoryUsageMB is the memory usage in megabytes.
	MemoryUsageMB float64 `json:"memoryUsageMb,omitempty"`

	// CPUPercent is the CPU usage percentage.
	CPUPercent float64 `json:"cpuPercent,omitempty"`

	// Diagnostics contains diagnostic information.
	Diagnostics map[string]interface{} `json:"diagnostics,omitempty"`

	// CheckedAt is when the health check was performed.
	CheckedAt time.Time `json:"checkedAt"`
}
