// Package worker provides worker infrastructure.
package worker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	domainWorker "github.com/anthropics/claude-flow-go/internal/domain/worker"
)

// WorkerDispatchService manages worker dispatch and lifecycle.
type WorkerDispatchService struct {
	mu sync.RWMutex

	// Workers indexed by ID
	workers map[string]*domainWorker.WorkerInstance

	// Workers indexed by session ID
	sessionWorkers map[string][]string

	// Trigger registry and detector
	registry *TriggerRegistry
	detector *TriggerDetector

	// Statistics
	totalDispatched int64
	totalCompleted  int64
	totalFailed     int64
	totalCancelled  int64
	totalDurationMs int64
}

// NewWorkerDispatchService creates a new worker dispatch service.
func NewWorkerDispatchService() *WorkerDispatchService {
	registry := NewTriggerRegistry()
	detector := NewTriggerDetector(registry)

	return &WorkerDispatchService{
		workers:        make(map[string]*domainWorker.WorkerInstance),
		sessionWorkers: make(map[string][]string),
		registry:       registry,
		detector:       detector,
	}
}

// Dispatch dispatches a new worker.
func (s *WorkerDispatchService) Dispatch(
	ctx context.Context,
	trigger domainWorker.WorkerTrigger,
	workerContext string,
	sessionID string,
	options *domainWorker.DispatchOptions,
) (string, error) {
	if !trigger.IsValid() {
		return "", fmt.Errorf("invalid trigger: %s", trigger)
	}

	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
	}

	// Get trigger config
	config, ok := s.registry.GetTrigger(trigger)
	if !ok {
		return "", fmt.Errorf("trigger config not found: %s", trigger)
	}

	// Apply options
	priority := config.Priority
	timeout := config.EstimatedDuration * 3 // 3x estimated duration
	var metadata map[string]interface{}

	if options != nil {
		if options.Priority != "" {
			priority = options.Priority
		}
		if options.Timeout > 0 {
			timeout = options.Timeout
		}
		metadata = options.Metadata
	}

	// Create worker instance
	workerID := uuid.New().String()
	now := time.Now()

	worker := &domainWorker.WorkerInstance{
		ID:        workerID,
		Trigger:   trigger,
		SessionID: sessionID,
		Context:   workerContext,
		Status:    domainWorker.StatusPending,
		Priority:  priority,
		Progress:  0,
		Phase:     "initializing",
		StartedAt: now,
		Timeout:   timeout,
		Metadata:  metadata,
	}

	s.mu.Lock()
	s.workers[workerID] = worker
	s.sessionWorkers[sessionID] = append(s.sessionWorkers[sessionID], workerID)
	s.totalDispatched++
	s.mu.Unlock()

	// Start the worker in a goroutine
	go s.runWorker(ctx, worker)

	return workerID, nil
}

// runWorker simulates worker execution.
func (s *WorkerDispatchService) runWorker(ctx context.Context, worker *domainWorker.WorkerInstance) {
	// Create cancellable context
	workerCtx, cancel := context.WithCancel(ctx)
	worker.CancelFunc = cancel

	// Get estimated duration
	config, _ := s.registry.GetTrigger(worker.Trigger)
	estimatedDuration := config.EstimatedDuration

	// Update to running
	s.mu.Lock()
	worker.Status = domainWorker.StatusRunning
	worker.Phase = "processing"
	s.mu.Unlock()

	// Simulate work in phases
	phases := []string{"analyzing", "processing", "synthesizing", "finalizing"}
	progressPerPhase := 100 / len(phases)
	phaseTime := estimatedDuration / time.Duration(len(phases))

	for i, phase := range phases {
		select {
		case <-workerCtx.Done():
			// Worker was cancelled
			s.mu.Lock()
			worker.Status = domainWorker.StatusCancelled
			now := time.Now()
			worker.CompletedAt = &now
			s.totalCancelled++
			s.mu.Unlock()
			return

		case <-time.After(phaseTime):
			// Update progress
			s.mu.Lock()
			worker.Phase = phase
			worker.Progress = (i + 1) * progressPerPhase
			s.mu.Unlock()
		}
	}

	// Complete the worker
	s.mu.Lock()
	now := time.Now()
	worker.Status = domainWorker.StatusCompleted
	worker.Phase = "completed"
	worker.Progress = 100
	worker.CompletedAt = &now
	worker.Result = &domainWorker.WorkerResult{
		Summary: fmt.Sprintf("Completed %s for context: %s", worker.Trigger, truncateString(worker.Context, 50)),
		Data: map[string]interface{}{
			"trigger":   string(worker.Trigger),
			"sessionId": worker.SessionID,
			"duration":  worker.Duration().Milliseconds(),
		},
		Metrics: map[string]float64{
			"durationMs": float64(worker.Duration().Milliseconds()),
			"progress":   100,
		},
	}
	s.totalCompleted++
	s.totalDurationMs += worker.Duration().Milliseconds()
	s.mu.Unlock()
}

// GetWorker returns a worker by ID.
func (s *WorkerDispatchService) GetWorker(workerID string) (*domainWorker.WorkerInstance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	worker, ok := s.workers[workerID]
	return worker, ok
}

// Cancel cancels a running worker.
func (s *WorkerDispatchService) Cancel(workerID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, ok := s.workers[workerID]
	if !ok {
		return false, fmt.Errorf("worker not found: %s", workerID)
	}

	if worker.Status.IsTerminal() {
		return false, fmt.Errorf("worker already in terminal state: %s", worker.Status)
	}

	if worker.CancelFunc != nil {
		worker.CancelFunc()
	}

	worker.Status = domainWorker.StatusCancelled
	now := time.Now()
	worker.CompletedAt = &now
	worker.Error = "cancelled by user"
	s.totalCancelled++

	return true, nil
}

// GetSessionWorkers returns all workers for a session.
func (s *WorkerDispatchService) GetSessionWorkers(sessionID string) []*domainWorker.WorkerInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workerIDs, ok := s.sessionWorkers[sessionID]
	if !ok {
		return nil
	}

	workers := make([]*domainWorker.WorkerInstance, 0, len(workerIDs))
	for _, id := range workerIDs {
		if worker, ok := s.workers[id]; ok {
			workers = append(workers, worker)
		}
	}

	return workers
}

// GetAllWorkers returns all workers.
func (s *WorkerDispatchService) GetAllWorkers() []*domainWorker.WorkerInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make([]*domainWorker.WorkerInstance, 0, len(s.workers))
	for _, worker := range s.workers {
		workers = append(workers, worker)
	}

	return workers
}

// FilterWorkers returns workers matching the filter criteria.
func (s *WorkerDispatchService) FilterWorkers(
	sessionID string,
	trigger *domainWorker.WorkerTrigger,
	status *domainWorker.WorkerStatus,
	limit int,
) []*domainWorker.WorkerInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []*domainWorker.WorkerInstance

	// Start with session-specific or all workers
	if sessionID != "" {
		workerIDs := s.sessionWorkers[sessionID]
		for _, id := range workerIDs {
			if worker, ok := s.workers[id]; ok {
				candidates = append(candidates, worker)
			}
		}
	} else {
		for _, worker := range s.workers {
			candidates = append(candidates, worker)
		}
	}

	// Filter by trigger
	if trigger != nil {
		filtered := make([]*domainWorker.WorkerInstance, 0)
		for _, w := range candidates {
			if w.Trigger == *trigger {
				filtered = append(filtered, w)
			}
		}
		candidates = filtered
	}

	// Filter by status
	if status != nil {
		filtered := make([]*domainWorker.WorkerInstance, 0)
		for _, w := range candidates {
			if w.Status == *status {
				filtered = append(filtered, w)
			}
		}
		candidates = filtered
	}

	// Apply limit
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates
}

// GetStats returns aggregate worker statistics.
func (s *WorkerDispatchService) GetStats() domainWorker.WorkerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := domainWorker.WorkerStats{
		ByTrigger: make(map[domainWorker.WorkerTrigger]int),
	}

	for _, trigger := range domainWorker.AllTriggers() {
		stats.ByTrigger[trigger] = 0
	}

	for _, worker := range s.workers {
		stats.Total++
		stats.ByTrigger[worker.Trigger]++

		switch worker.Status {
		case domainWorker.StatusPending:
			stats.Pending++
		case domainWorker.StatusRunning:
			stats.Running++
		case domainWorker.StatusCompleted:
			stats.Completed++
		case domainWorker.StatusFailed:
			stats.Failed++
		case domainWorker.StatusCancelled:
			stats.Cancelled++
		}
	}

	// Calculate average duration
	if s.totalCompleted > 0 {
		stats.AvgDurationMs = float64(s.totalDurationMs) / float64(s.totalCompleted)
	}

	// Calculate success rate
	terminalCount := stats.Completed + stats.Failed + stats.Cancelled
	if terminalCount > 0 {
		stats.SuccessRate = float64(stats.Completed) / float64(terminalCount)
	}

	return stats
}

// GetContextForInjection returns formatted context for prompt injection.
func (s *WorkerDispatchService) GetContextForInjection(sessionID string) string {
	workers := s.GetSessionWorkers(sessionID)
	if len(workers) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("## Background Worker Results\n\n")

	for _, worker := range workers {
		if worker.Status != domainWorker.StatusCompleted {
			continue
		}

		builder.WriteString(fmt.Sprintf("### %s Worker\n", strings.Title(string(worker.Trigger))))
		if worker.Result != nil && worker.Result.Summary != "" {
			builder.WriteString(fmt.Sprintf("- **Summary**: %s\n", worker.Result.Summary))
		}
		builder.WriteString(fmt.Sprintf("- **Duration**: %dms\n", worker.Duration().Milliseconds()))
		builder.WriteString("\n")
	}

	return builder.String()
}

// DetectTriggers detects triggers in text.
func (s *WorkerDispatchService) DetectTriggers(text string) domainWorker.TriggerDetectionResult {
	return s.detector.DetectTriggers(text)
}

// GetTriggers returns all trigger configurations.
func (s *WorkerDispatchService) GetTriggers() map[domainWorker.WorkerTrigger]domainWorker.TriggerConfig {
	return s.registry.GetAllTriggers()
}

// GetTriggerList returns all triggers as a list.
func (s *WorkerDispatchService) GetTriggerList() []domainWorker.TriggerConfig {
	return s.registry.GetTriggerList()
}

// Cleanup removes completed workers older than the given duration.
func (s *WorkerDispatchService) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removedCount := 0

	for id, worker := range s.workers {
		if worker.Status.IsTerminal() && worker.CompletedAt != nil && worker.CompletedAt.Before(cutoff) {
			delete(s.workers, id)
			removedCount++

			// Remove from session workers
			if workerIDs, ok := s.sessionWorkers[worker.SessionID]; ok {
				newIDs := make([]string, 0, len(workerIDs)-1)
				for _, wid := range workerIDs {
					if wid != id {
						newIDs = append(newIDs, wid)
					}
				}
				if len(newIDs) == 0 {
					delete(s.sessionWorkers, worker.SessionID)
				} else {
					s.sessionWorkers[worker.SessionID] = newIDs
				}
			}
		}
	}

	return removedCount
}

// Helper function to truncate strings.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
