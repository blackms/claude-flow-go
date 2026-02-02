// Package hooks provides the hooks system for self-learning operations.
package hooks

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// EventEmitter defines the interface for emitting events.
// This allows decoupling from the concrete EventBus implementation.
type EventEmitter interface {
	Emit(event shared.Event)
}

// PluginInvoker defines the interface for invoking plugin extension points.
// This allows decoupling from the concrete PluginManager implementation.
type PluginInvoker interface {
	InvokeExtensionPoint(ctx context.Context, name string, data interface{}) ([]interface{}, error)
	HasExtensionPoint(name string) bool
}

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

	// Integration with other infrastructure components
	eventBus      EventEmitter
	pluginManager PluginInvoker
}

// ManagerOptions holds optional dependencies for the HooksManager.
type ManagerOptions struct {
	// EventBus for emitting hook-related events
	EventBus EventEmitter
	// PluginManager for invoking extension points
	PluginManager PluginInvoker
}

// NewHooksManager creates a new HooksManager.
func NewHooksManager(config shared.HooksConfig) *HooksManager {
	return NewHooksManagerWithOptions(config, ManagerOptions{})
}

// NewHooksManagerWithOptions creates a HooksManager with the given configuration and options.
func NewHooksManagerWithOptions(config shared.HooksConfig, opts ManagerOptions) *HooksManager {
	return &HooksManager{
		hooks:         make(map[shared.HookEvent][]*shared.HookRegistration),
		hooksByID:     make(map[string]*shared.HookRegistration),
		patterns:      NewPatternStore(config.MaxPatterns),
		routing:       NewRoutingEngine(config.LearningRate),
		config:        config,
		metrics:       &shared.HooksMetrics{
			HooksByEvent: make(map[shared.HookEvent]int64),
		},
		eventBus:      opts.EventBus,
		pluginManager: opts.PluginManager,
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

	// Emit initialization event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.initialized"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"maxPatterns":      hm.config.MaxPatterns,
			"maxHooksPerEvent": hm.config.MaxHooksPerEvent,
			"learningEnabled":  hm.config.EnableLearning,
		},
	})

	return nil
}

// Shutdown stops the HooksManager.
func (hm *HooksManager) Shutdown() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.running = false

	// Emit shutdown event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.shutdown"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"totalExecutions": hm.metrics.TotalExecutions,
			"patternCount":    hm.patterns.Count(),
		},
	})

	return nil
}

// SetEventBus sets the event bus for emitting hook events.
func (hm *HooksManager) SetEventBus(eventBus EventEmitter) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.eventBus = eventBus
}

// SetPluginManager sets the plugin manager for extension point integration.
func (hm *HooksManager) SetPluginManager(pm PluginInvoker) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.pluginManager = pm
}

// emitEvent emits an event if the event bus is configured.
func (hm *HooksManager) emitEvent(event shared.Event) {
	if hm.eventBus != nil {
		hm.eventBus.Emit(event)
	}
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

	// Emit registration event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.registered"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"hookId":   hook.ID,
			"hookName": hook.Name,
			"event":    string(hook.Event),
			"priority": int(hook.Priority),
		},
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

	hookEvent := hook.Event
	hookName := hook.Name

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

	// Emit unregistration event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.unregistered"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"hookId":   hookID,
			"hookName": hookName,
			"event":    string(hookEvent),
		},
	})

	return nil
}

// Execute executes all hooks for an event.
func (hm *HooksManager) Execute(ctx context.Context, hookCtx *shared.HookContext) (*shared.HookExecutionResult, error) {
	startTime := time.Now()

	hm.mu.RLock()
	hooks := make([]*shared.HookRegistration, len(hm.hooks[hookCtx.Event]))
	copy(hooks, hm.hooks[hookCtx.Event])
	timeout := hm.config.DefaultTimeoutMs
	pluginManager := hm.pluginManager
	hm.mu.RUnlock()

	result := &shared.HookExecutionResult{
		Event:   hookCtx.Event,
		Results: make([]*shared.HookResult, 0),
	}

	// Invoke plugin extension point for pre-hook execution
	extensionPoint := "hooks." + string(hookCtx.Event)
	if pluginManager != nil && pluginManager.HasExtensionPoint(extensionPoint) {
		pluginManager.InvokeExtensionPoint(ctx, extensionPoint, hookCtx.Data)
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

	// Emit execution event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.executed"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"event":       string(hookCtx.Event),
			"hooksRun":    result.HooksRun,
			"successful":  result.Successful,
			"failed":      result.Failed,
			"totalTimeMs": result.TotalTimeMs,
		},
	})

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

// ============================================================================
// Pattern Recording Methods
// ============================================================================

// RecordEditPattern records an edit pattern and emits an event.
func (hm *HooksManager) RecordEditPattern(filePath, operation string, success bool, metadata map[string]interface{}) (*shared.Pattern, error) {
	pattern := CreateEditPattern(filePath, operation, success, metadata)
	
	if err := hm.patterns.Store(pattern); err != nil {
		return nil, err
	}

	// Emit pattern learned event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.pattern_learned"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"patternId":   pattern.ID,
			"patternType": string(shared.PatternTypeEdit),
			"filePath":    filePath,
			"operation":   operation,
			"success":     success,
		},
	})

	return pattern, nil
}

// RecordCommandPattern records a command pattern and emits an event.
func (hm *HooksManager) RecordCommandPattern(command string, success bool, exitCode int, executionTimeMs int64) (*shared.Pattern, error) {
	pattern := CreateCommandPattern(command, success, exitCode, executionTimeMs)
	
	if err := hm.patterns.Store(pattern); err != nil {
		return nil, err
	}

	// Emit pattern learned event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.pattern_learned"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"patternId":       pattern.ID,
			"patternType":     string(shared.PatternTypeCommand),
			"command":         command,
			"exitCode":        exitCode,
			"executionTimeMs": executionTimeMs,
			"success":         success,
		},
	})

	return pattern, nil
}

// RecordPatternOutcome records the outcome of using a pattern.
func (hm *HooksManager) RecordPatternOutcome(patternID string, success bool) error {
	var err error
	if success {
		err = hm.patterns.RecordSuccess(patternID)
	} else {
		err = hm.patterns.RecordFailure(patternID)
	}

	if err != nil {
		return err
	}

	// Emit outcome event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.pattern_outcome"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"patternId": patternID,
			"success":   success,
		},
	})

	return nil
}

// ============================================================================
// Routing Methods
// ============================================================================

// RouteTask routes a task to the optimal agent and emits an event.
func (hm *HooksManager) RouteTask(task string, context map[string]interface{}, preferredAgents []string, constraints map[string]interface{}) *shared.RoutingResult {
	result := hm.routing.Route(task, context, preferredAgents, constraints)

	// Emit routing decision event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.routing_decision"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"routingId":        result.ID,
			"task":             task,
			"recommendedAgent": result.RecommendedAgent,
			"confidence":       result.Confidence,
			"alternatives":     len(result.Alternatives),
		},
	})

	return result
}

// RecordRoutingOutcome records the outcome of a routing decision.
func (hm *HooksManager) RecordRoutingOutcome(routingID string, success bool, executionTimeMs int64) error {
	err := hm.routing.RecordOutcome(routingID, success, executionTimeMs)
	if err != nil {
		return err
	}

	// Emit outcome event
	hm.emitEvent(shared.Event{
		Type:      shared.EventType("hooks.routing_outcome"),
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"routingId":       routingID,
			"success":         success,
			"executionTimeMs": executionTimeMs,
		},
	})

	return nil
}

// ExplainRouting explains routing decisions for a task.
func (hm *HooksManager) ExplainRouting(task string, context map[string]interface{}, verbose bool) *shared.RoutingExplanation {
	return hm.routing.Explain(task, context, verbose)
}

// ============================================================================
// Integration Methods
// ============================================================================

// ExecuteWithPluginContext executes hooks and invokes plugin extension points.
func (hm *HooksManager) ExecuteWithPluginContext(ctx context.Context, hookCtx *shared.HookContext) (*shared.HookExecutionResult, error) {
	hm.mu.RLock()
	pluginManager := hm.pluginManager
	hm.mu.RUnlock()

	// Invoke pre-execution extension point
	preExtPoint := "hooks.before." + string(hookCtx.Event)
	if pluginManager != nil && pluginManager.HasExtensionPoint(preExtPoint) {
		pluginManager.InvokeExtensionPoint(ctx, preExtPoint, hookCtx.Data)
	}

	// Execute the hooks
	result, err := hm.Execute(ctx, hookCtx)

	// Invoke post-execution extension point
	postExtPoint := "hooks.after." + string(hookCtx.Event)
	if pluginManager != nil && pluginManager.HasExtensionPoint(postExtPoint) {
		pluginManager.InvokeExtensionPoint(ctx, postExtPoint, map[string]interface{}{
			"event":  hookCtx.Event,
			"result": result,
			"data":   hookCtx.Data,
		})
	}

	return result, err
}

// IsRunning returns whether the HooksManager is currently running.
func (hm *HooksManager) IsRunning() bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.running
}

// HasEventBus returns whether an event bus is configured.
func (hm *HooksManager) HasEventBus() bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.eventBus != nil
}

// HasPluginManager returns whether a plugin manager is configured.
func (hm *HooksManager) HasPluginManager() bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.pluginManager != nil
}
