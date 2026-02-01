// Package hooks provides the hooks system for self-learning operations.
package hooks

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// HooksManager manages hook registration, execution, and learning.
type HooksManager struct {
	mu            sync.RWMutex
	hooks         map[shared.HookEvent][]*shared.HookRegistration
	hooksByID     map[string]*shared.HookRegistration
	patterns      *PatternStore
	routing       *RoutingEngine
	config        shared.HooksConfig
	metrics       *shared.HooksMetrics
	running       bool
}

// NewHooksManager creates a new HooksManager.
func NewHooksManager(config shared.HooksConfig) *HooksManager {
	return &HooksManager{
		hooks:     make(map[shared.HookEvent][]*shared.HookRegistration),
		hooksByID: make(map[string]*shared.HookRegistration),
		patterns:  NewPatternStore(config.MaxPatterns),
		routing:   NewRoutingEngine(config.LearningRate),
		config:    config,
		metrics: &shared.HooksMetrics{
			HooksByEvent: make(map[shared.HookEvent]int64),
		},
	}
}

// NewHooksManagerWithDefaults creates a HooksManager with default configuration.
func NewHooksManagerWithDefaults() *HooksManager {
	return NewHooksManager(shared.DefaultHooksConfig())
}

// Initialize starts the HooksManager.
func (hm *HooksManager) Initialize() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if hm.running {
		return nil
	}

	hm.running = true
	return nil
}

// Shutdown stops the HooksManager.
func (hm *HooksManager) Shutdown() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.running = false
	return nil
}

// Register registers a hook.
func (hm *HooksManager) Register(hook *shared.HookRegistration) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Check if hook already exists
	if _, exists := hm.hooksByID[hook.ID]; exists {
		return shared.ErrHookAlreadyExists
	}

	// Check max hooks per event
	if len(hm.hooks[hook.Event]) >= hm.config.MaxHooksPerEvent {
		return shared.ErrMaxHooksReached
	}

	// Set defaults
	if hook.CreatedAt == 0 {
		hook.CreatedAt = shared.Now()
	}
	hook.Enabled = true

	// Add to maps
	hm.hooks[hook.Event] = append(hm.hooks[hook.Event], hook)
	hm.hooksByID[hook.ID] = hook

	// Sort by priority (higher first)
	sort.SliceStable(hm.hooks[hook.Event], func(i, j int) bool {
		return hm.hooks[hook.Event][i].Priority > hm.hooks[hook.Event][j].Priority
	})

	return nil
}

// Unregister removes a hook.
func (hm *HooksManager) Unregister(hookID string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hook, exists := hm.hooksByID[hookID]
	if !exists {
		return shared.ErrHookNotFound
	}

	// Remove from event hooks
	eventHooks := hm.hooks[hook.Event]
	for i, h := range eventHooks {
		if h.ID == hookID {
			hm.hooks[hook.Event] = append(eventHooks[:i], eventHooks[i+1:]...)
			break
		}
	}

	// Remove from ID map
	delete(hm.hooksByID, hookID)

	return nil
}

// Execute executes all hooks for an event.
func (hm *HooksManager) Execute(ctx context.Context, hookCtx *shared.HookContext) (*shared.HookExecutionResult, error) {
	startTime := time.Now()

	hm.mu.RLock()
	hooks := make([]*shared.HookRegistration, len(hm.hooks[hookCtx.Event]))
	copy(hooks, hm.hooks[hookCtx.Event])
	timeout := hm.config.DefaultTimeoutMs
	hm.mu.RUnlock()

	result := &shared.HookExecutionResult{
		Event:   hookCtx.Event,
		Results: make([]*shared.HookResult, 0),
	}

	// Execute each hook
	for _, hook := range hooks {
		if !hook.Enabled {
			continue
		}

		hookResult := hm.executeHook(ctx, hook, hookCtx, timeout)
		result.Results = append(result.Results, hookResult)
		result.HooksRun++

		if hookResult.Success {
			result.Successful++
		} else {
			result.Failed++
		}
	}

	result.TotalTimeMs = time.Since(startTime).Milliseconds()

	// Update metrics
	hm.mu.Lock()
	hm.metrics.TotalExecutions++
	hm.metrics.SuccessfulExecutions += int64(result.Successful)
	hm.metrics.FailedExecutions += int64(result.Failed)
	hm.metrics.HooksByEvent[hookCtx.Event]++
	
	// Update average execution time
	n := float64(hm.metrics.TotalExecutions)
	hm.metrics.AvgExecutionMs = (hm.metrics.AvgExecutionMs*(n-1) + float64(result.TotalTimeMs)) / n
	hm.mu.Unlock()

	return result, nil
}

// executeHook executes a single hook with timeout.
func (hm *HooksManager) executeHook(ctx context.Context, hook *shared.HookRegistration, hookCtx *shared.HookContext, timeoutMs int64) *shared.HookResult {
	startTime := time.Now()
	result := &shared.HookResult{
		HookID:    hook.ID,
		Timestamp: shared.Now(),
	}

	if hook.Handler == nil {
		result.Success = true
		result.ExecutionMs = time.Since(startTime).Milliseconds()
		return result
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Execute handler
	done := make(chan struct{})
	var handlerResult interface{}
	var handlerErr error

	go func() {
		handlerResult, handlerErr = hook.Handler(execCtx, hookCtx.Data)
		close(done)
	}()

	select {
	case <-done:
		result.ExecutionMs = time.Since(startTime).Milliseconds()
		if handlerErr != nil {
			result.Success = false
			result.Error = handlerErr.Error()
		} else {
			result.Success = true
			result.Result = handlerResult
		}
	case <-execCtx.Done():
		result.ExecutionMs = time.Since(startTime).Milliseconds()
		result.Success = false
		result.Error = shared.ErrHookExecutionTimeout.Error()
	}

	// Update hook stats
	hm.mu.Lock()
	hook.ExecutionCount++
	hook.LastExecutedAt = shared.Now()
	n := float64(hook.ExecutionCount)
	hook.AvgExecutionMs = (hook.AvgExecutionMs*(n-1) + float64(result.ExecutionMs)) / n
	hm.mu.Unlock()

	return result
}

// GetHook returns a hook by ID.
func (hm *HooksManager) GetHook(id string) *shared.HookRegistration {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.hooksByID[id]
}

// ListHooks returns hooks, optionally filtered by event.
func (hm *HooksManager) ListHooks(event shared.HookEvent, includeDisabled bool) []*shared.HookRegistration {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var result []*shared.HookRegistration

	if event != "" {
		for _, hook := range hm.hooks[event] {
			if includeDisabled || hook.Enabled {
				result = append(result, hook)
			}
		}
	} else {
		for _, hooks := range hm.hooks {
			for _, hook := range hooks {
				if includeDisabled || hook.Enabled {
					result = append(result, hook)
				}
			}
		}
	}

	return result
}

// EnableHook enables a hook.
func (hm *HooksManager) EnableHook(id string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hook, exists := hm.hooksByID[id]
	if !exists {
		return shared.ErrHookNotFound
	}

	hook.Enabled = true
	return nil
}

// DisableHook disables a hook.
func (hm *HooksManager) DisableHook(id string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hook, exists := hm.hooksByID[id]
	if !exists {
		return shared.ErrHookNotFound
	}

	hook.Enabled = false
	return nil
}

// GetPatternStore returns the pattern store.
func (hm *HooksManager) GetPatternStore() *PatternStore {
	return hm.patterns
}

// GetRoutingEngine returns the routing engine.
func (hm *HooksManager) GetRoutingEngine() *RoutingEngine {
	return hm.routing
}

// GetMetrics returns the hooks metrics.
func (hm *HooksManager) GetMetrics() *shared.HooksMetrics {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	// Create a copy
	metrics := *hm.metrics
	metrics.PatternCount = int64(hm.patterns.Count())
	metrics.EditPatterns = int64(hm.patterns.CountByType(shared.PatternTypeEdit))
	metrics.CommandPatterns = int64(hm.patterns.CountByType(shared.PatternTypeCommand))
	metrics.RoutingCount = hm.routing.GetRoutingCount()
	metrics.RoutingSuccessRate = hm.routing.GetSuccessRate()

	// Copy hooks by event map
	metrics.HooksByEvent = make(map[shared.HookEvent]int64)
	for k, v := range hm.metrics.HooksByEvent {
		metrics.HooksByEvent[k] = v
	}

	return &metrics
}

// GetConfig returns the hooks configuration.
func (hm *HooksManager) GetConfig() shared.HooksConfig {
	return hm.config
}

// HookCount returns the total number of registered hooks.
func (hm *HooksManager) HookCount() int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.hooksByID)
}

// HookCountByEvent returns the number of hooks for a specific event.
func (hm *HooksManager) HookCountByEvent(event shared.HookEvent) int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.hooks[event])
}
