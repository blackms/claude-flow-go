// Package hooks provides the hooks system for self-learning operations.
package hooks

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestNewHooksManager(t *testing.T) {
	config := shared.HooksConfig{
		MaxPatterns:      5000,
		MaxHooksPerEvent: 50,
		LearningRate:     0.2,
		DefaultTimeoutMs: 100,
	}

	hm := NewHooksManager(config)

	if hm.config.MaxPatterns != 5000 {
		t.Errorf("expected MaxPatterns 5000, got %d", hm.config.MaxPatterns)
	}
	if hm.config.LearningRate != 0.2 {
		t.Errorf("expected LearningRate 0.2, got %v", hm.config.LearningRate)
	}
	if hm.patterns == nil {
		t.Error("patterns store should be initialized")
	}
	if hm.routing == nil {
		t.Error("routing engine should be initialized")
	}
}

func TestNewHooksManagerWithDefaults(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	defaultConfig := shared.DefaultHooksConfig()
	if hm.config.MaxPatterns != defaultConfig.MaxPatterns {
		t.Errorf("expected default MaxPatterns %d, got %d", defaultConfig.MaxPatterns, hm.config.MaxPatterns)
	}
}

func TestHooksManager_InitializeShutdown(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	// Initialize
	if err := hm.Initialize(); err != nil {
		t.Errorf("unexpected error on initialize: %v", err)
	}

	if !hm.running {
		t.Error("manager should be running after initialize")
	}

	// Double initialize should be no-op
	if err := hm.Initialize(); err != nil {
		t.Errorf("double initialize should not error: %v", err)
	}

	// Shutdown
	if err := hm.Shutdown(); err != nil {
		t.Errorf("unexpected error on shutdown: %v", err)
	}

	if hm.running {
		t.Error("manager should not be running after shutdown")
	}
}

func TestHooksManager_Register(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	hook := &shared.HookRegistration{
		ID:          "test-hook-1",
		Name:        "Test Hook",
		Event:       shared.HookEventPreEdit,
		Priority:    shared.HookPriorityNormal,
		Description: "A test hook",
	}

	err := hm.Register(hook)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify registration
	if hm.HookCount() != 1 {
		t.Errorf("expected 1 hook, got %d", hm.HookCount())
	}

	// Verify hook is enabled by default
	registered := hm.GetHook("test-hook-1")
	if registered == nil {
		t.Fatal("hook should be retrievable")
	}
	if !registered.Enabled {
		t.Error("hook should be enabled by default")
	}

	// Verify CreatedAt was set
	if registered.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
}

func TestHooksManager_RegisterDuplicate(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	hook := &shared.HookRegistration{
		ID:    "test-hook-1",
		Event: shared.HookEventPreEdit,
	}

	_ = hm.Register(hook)

	// Registering same ID again should fail
	err := hm.Register(hook)
	if !errors.Is(err, shared.ErrHookAlreadyExists) {
		t.Errorf("expected ErrHookAlreadyExists, got %v", err)
	}
}

func TestHooksManager_RegisterMaxHooksPerEvent(t *testing.T) {
	config := shared.DefaultHooksConfig()
	config.MaxHooksPerEvent = 2
	hm := NewHooksManager(config)

	// Register up to max
	for i := 0; i < 2; i++ {
		hook := &shared.HookRegistration{
			ID:    shared.GenerateID("hook"),
			Event: shared.HookEventPreEdit,
		}
		if err := hm.Register(hook); err != nil {
			t.Errorf("unexpected error registering hook %d: %v", i, err)
		}
	}

	// One more should fail
	hook := &shared.HookRegistration{
		ID:    shared.GenerateID("hook"),
		Event: shared.HookEventPreEdit,
	}
	err := hm.Register(hook)
	if !errors.Is(err, shared.ErrMaxHooksReached) {
		t.Errorf("expected ErrMaxHooksReached, got %v", err)
	}
}

func TestHooksManager_Unregister(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	hook := &shared.HookRegistration{
		ID:    "test-hook-1",
		Event: shared.HookEventPreEdit,
	}

	_ = hm.Register(hook)

	err := hm.Unregister("test-hook-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if hm.HookCount() != 0 {
		t.Errorf("expected 0 hooks, got %d", hm.HookCount())
	}

	// Unregistering again should fail
	err = hm.Unregister("test-hook-1")
	if !errors.Is(err, shared.ErrHookNotFound) {
		t.Errorf("expected ErrHookNotFound, got %v", err)
	}
}

func TestHooksManager_Execute(t *testing.T) {
	hm := NewHooksManagerWithDefaults()
	_ = hm.Initialize()

	executed := false
	hook := &shared.HookRegistration{
		ID:       "test-hook-1",
		Event:    shared.HookEventPreEdit,
		Priority: shared.HookPriorityNormal,
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			executed = true
			return "result", nil
		},
	}

	_ = hm.Register(hook)

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Data:      map[string]interface{}{"file": "test.go"},
		Timestamp: shared.Now(),
	}

	result, err := hm.Execute(ctx, hookCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("hook handler should have been executed")
	}

	if result.HooksRun != 1 {
		t.Errorf("expected 1 hook run, got %d", result.HooksRun)
	}

	if result.Successful != 1 {
		t.Errorf("expected 1 successful, got %d", result.Successful)
	}
}

func TestHooksManager_ExecuteMultiple(t *testing.T) {
	hm := NewHooksManagerWithDefaults()
	_ = hm.Initialize()

	order := make([]int, 0)
	var mu sync.Mutex

	// Register hooks with different priorities
	for i, priority := range []shared.HookPriority{shared.HookPriorityLow, shared.HookPriorityHigh, shared.HookPriorityNormal} {
		i := i
		hook := &shared.HookRegistration{
			ID:       shared.GenerateID("hook"),
			Event:    shared.HookEventPreEdit,
			Priority: priority,
			Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
				return nil, nil
			},
		}
		_ = hm.Register(hook)
	}

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}

	result, err := hm.Execute(ctx, hookCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.HooksRun != 3 {
		t.Errorf("expected 3 hooks run, got %d", result.HooksRun)
	}

	// High priority (1) should run first, then Normal (2), then Low (0)
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 0 {
		t.Errorf("expected execution order [1,2,0], got %v", order)
	}
}

func TestHooksManager_ExecuteWithError(t *testing.T) {
	hm := NewHooksManagerWithDefaults()
	_ = hm.Initialize()

	hook := &shared.HookRegistration{
		ID:    "test-hook-1",
		Event: shared.HookEventPreEdit,
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			return nil, errors.New("hook error")
		},
	}

	_ = hm.Register(hook)

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}

	result, _ := hm.Execute(ctx, hookCtx)

	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}

	if result.Results[0].Error == "" {
		t.Error("error message should be set")
	}
}

func TestHooksManager_ExecuteWithTimeout(t *testing.T) {
	config := shared.DefaultHooksConfig()
	config.DefaultTimeoutMs = 10 // 10ms timeout
	hm := NewHooksManager(config)
	_ = hm.Initialize()

	hook := &shared.HookRegistration{
		ID:    "slow-hook",
		Event: shared.HookEventPreEdit,
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			time.Sleep(100 * time.Millisecond) // Longer than timeout
			return nil, nil
		},
	}

	_ = hm.Register(hook)

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}

	result, _ := hm.Execute(ctx, hookCtx)

	if result.Failed != 1 {
		t.Errorf("expected 1 failed due to timeout, got %d failed", result.Failed)
	}
}

func TestHooksManager_ExecuteSkipsDisabled(t *testing.T) {
	hm := NewHooksManagerWithDefaults()
	_ = hm.Initialize()

	executed := false
	hook := &shared.HookRegistration{
		ID:    "test-hook-1",
		Event: shared.HookEventPreEdit,
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			executed = true
			return nil, nil
		},
	}

	_ = hm.Register(hook)
	_ = hm.DisableHook("test-hook-1")

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}

	result, _ := hm.Execute(ctx, hookCtx)

	if executed {
		t.Error("disabled hook should not be executed")
	}

	if result.HooksRun != 0 {
		t.Errorf("expected 0 hooks run, got %d", result.HooksRun)
	}
}

func TestHooksManager_EnableDisableHook(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	hook := &shared.HookRegistration{
		ID:    "test-hook-1",
		Event: shared.HookEventPreEdit,
	}
	_ = hm.Register(hook)

	// Initially enabled
	if !hm.GetHook("test-hook-1").Enabled {
		t.Error("hook should be enabled initially")
	}

	// Disable
	if err := hm.DisableHook("test-hook-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if hm.GetHook("test-hook-1").Enabled {
		t.Error("hook should be disabled")
	}

	// Enable
	if err := hm.EnableHook("test-hook-1"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !hm.GetHook("test-hook-1").Enabled {
		t.Error("hook should be enabled")
	}

	// Enable non-existent should error
	if err := hm.EnableHook("non-existent"); !errors.Is(err, shared.ErrHookNotFound) {
		t.Errorf("expected ErrHookNotFound, got %v", err)
	}
}

func TestHooksManager_ListHooks(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	// Register hooks for different events
	for _, event := range []shared.HookEvent{shared.HookEventPreEdit, shared.HookEventPreEdit, shared.HookEventPreCommand} {
		hook := &shared.HookRegistration{
			ID:    shared.GenerateID("hook"),
			Event: event,
		}
		_ = hm.Register(hook)
	}

	// Disable one
	hooks := hm.ListHooks("", false)
	_ = hm.DisableHook(hooks[0].ID)

	// List all enabled
	enabledHooks := hm.ListHooks("", false)
	if len(enabledHooks) != 2 {
		t.Errorf("expected 2 enabled hooks, got %d", len(enabledHooks))
	}

	// List all including disabled
	allHooks := hm.ListHooks("", true)
	if len(allHooks) != 3 {
		t.Errorf("expected 3 total hooks, got %d", len(allHooks))
	}

	// List by event
	preEditHooks := hm.ListHooks(shared.HookEventPreEdit, true)
	if len(preEditHooks) != 2 {
		t.Errorf("expected 2 pre-edit hooks, got %d", len(preEditHooks))
	}
}

func TestHooksManager_GetMetrics(t *testing.T) {
	hm := NewHooksManagerWithDefaults()
	_ = hm.Initialize()

	// Register and execute a hook
	hook := &shared.HookRegistration{
		ID:    "test-hook-1",
		Event: shared.HookEventPreEdit,
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			return nil, nil
		},
	}
	_ = hm.Register(hook)

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}

	_, _ = hm.Execute(ctx, hookCtx)
	_, _ = hm.Execute(ctx, hookCtx)

	metrics := hm.GetMetrics()

	if metrics.TotalExecutions != 2 {
		t.Errorf("expected 2 total executions, got %d", metrics.TotalExecutions)
	}

	if metrics.SuccessfulExecutions != 2 {
		t.Errorf("expected 2 successful executions, got %d", metrics.SuccessfulExecutions)
	}

	if metrics.HooksByEvent[shared.HookEventPreEdit] != 2 {
		t.Errorf("expected 2 pre-edit hook executions, got %d", metrics.HooksByEvent[shared.HookEventPreEdit])
	}
}

func TestHooksManager_HookCountByEvent(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	// Register hooks for different events
	hook1 := &shared.HookRegistration{ID: "h1", Event: shared.HookEventPreEdit}
	hook2 := &shared.HookRegistration{ID: "h2", Event: shared.HookEventPreEdit}
	hook3 := &shared.HookRegistration{ID: "h3", Event: shared.HookEventPreCommand}

	_ = hm.Register(hook1)
	_ = hm.Register(hook2)
	_ = hm.Register(hook3)

	if hm.HookCountByEvent(shared.HookEventPreEdit) != 2 {
		t.Errorf("expected 2 pre-edit hooks, got %d", hm.HookCountByEvent(shared.HookEventPreEdit))
	}

	if hm.HookCountByEvent(shared.HookEventPreCommand) != 1 {
		t.Errorf("expected 1 pre-command hooks, got %d", hm.HookCountByEvent(shared.HookEventPreCommand))
	}
}

func TestHooksManager_GetPatternStore(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	ps := hm.GetPatternStore()
	if ps == nil {
		t.Error("pattern store should not be nil")
	}
}

func TestHooksManager_GetRoutingEngine(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	re := hm.GetRoutingEngine()
	if re == nil {
		t.Error("routing engine should not be nil")
	}
}

func TestHooksManager_GetConfig(t *testing.T) {
	config := shared.HooksConfig{
		MaxPatterns:      5000,
		MaxHooksPerEvent: 50,
		LearningRate:     0.2,
		DefaultTimeoutMs: 100,
	}

	hm := NewHooksManager(config)

	returnedConfig := hm.GetConfig()
	if returnedConfig.MaxPatterns != 5000 {
		t.Errorf("expected MaxPatterns 5000, got %d", returnedConfig.MaxPatterns)
	}
}

func TestHooksManager_ConcurrentAccess(t *testing.T) {
	hm := NewHooksManagerWithDefaults()
	_ = hm.Initialize()

	// Add a hook with a handler
	hook := &shared.HookRegistration{
		ID:    "concurrent-hook",
		Event: shared.HookEventPreEdit,
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			return nil, nil
		},
	}
	_ = hm.Register(hook)

	var wg sync.WaitGroup
	ctx := context.Background()

	// Concurrent executions
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hookCtx := &shared.HookContext{
				Event:     shared.HookEventPreEdit,
				Timestamp: shared.Now(),
			}
			_, _ = hm.Execute(ctx, hookCtx)
		}()
	}

	// Concurrent registrations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			hook := &shared.HookRegistration{
				ID:    shared.GenerateID("hook"),
				Event: shared.HookEvent("event-" + string(rune('a'+i))),
			}
			_ = hm.Register(hook)
		}()
	}

	// Concurrent reads
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = hm.GetMetrics()
			_ = hm.ListHooks("", true)
			_ = hm.HookCount()
		}()
	}

	wg.Wait()

	// Verify no race conditions
	if hm.HookCount() < 1 {
		t.Error("should have at least one hook")
	}

	metrics := hm.GetMetrics()
	if metrics.TotalExecutions != 50 {
		t.Errorf("expected 50 executions, got %d", metrics.TotalExecutions)
	}
}

func TestHooksManager_HookWithNilHandler(t *testing.T) {
	hm := NewHooksManagerWithDefaults()
	_ = hm.Initialize()

	// Hook with nil handler should still execute successfully
	hook := &shared.HookRegistration{
		ID:      "nil-handler-hook",
		Event:   shared.HookEventPreEdit,
		Handler: nil,
	}
	_ = hm.Register(hook)

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}

	result, err := hm.Execute(ctx, hookCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.Successful != 1 {
		t.Errorf("nil handler hook should succeed, got %d successful", result.Successful)
	}
}

func TestHooksManager_ExecutionTimeTracking(t *testing.T) {
	hm := NewHooksManagerWithDefaults()
	_ = hm.Initialize()

	hook := &shared.HookRegistration{
		ID:    "timed-hook",
		Event: shared.HookEventPreEdit,
		Handler: func(ctx context.Context, data interface{}) (interface{}, error) {
			time.Sleep(10 * time.Millisecond)
			return nil, nil
		},
	}
	_ = hm.Register(hook)

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}

	result, _ := hm.Execute(ctx, hookCtx)

	// Check execution time was tracked
	if result.TotalTimeMs < 10 {
		t.Errorf("expected execution time >= 10ms, got %d", result.TotalTimeMs)
	}

	// Check hook stats were updated
	registeredHook := hm.GetHook("timed-hook")
	if registeredHook.ExecutionCount != 1 {
		t.Errorf("expected ExecutionCount 1, got %d", registeredHook.ExecutionCount)
	}
	if registeredHook.AvgExecutionMs < 10 {
		t.Errorf("expected AvgExecutionMs >= 10, got %v", registeredHook.AvgExecutionMs)
	}
	if registeredHook.LastExecutedAt == 0 {
		t.Error("LastExecutedAt should be set")
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

// mockEventEmitter implements EventEmitter for testing
type mockEventEmitter struct {
	mu     sync.Mutex
	events []shared.Event
}

func (m *mockEventEmitter) Emit(event shared.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *mockEventEmitter) getEvents() []shared.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]shared.Event, len(m.events))
	copy(result, m.events)
	return result
}

// mockPluginInvoker implements PluginInvoker for testing
type mockPluginInvoker struct {
	mu              sync.Mutex
	extensionPoints map[string]bool
	invocations     []string
}

func (m *mockPluginInvoker) InvokeExtensionPoint(ctx context.Context, name string, data interface{}) ([]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invocations = append(m.invocations, name)
	return nil, nil
}

func (m *mockPluginInvoker) HasExtensionPoint(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.extensionPoints[name]
}

func (m *mockPluginInvoker) getInvocations() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.invocations))
	copy(result, m.invocations)
	return result
}

func TestNewHooksManagerWithOptions(t *testing.T) {
	eventBus := &mockEventEmitter{}
	pluginManager := &mockPluginInvoker{extensionPoints: map[string]bool{}}

	opts := ManagerOptions{
		EventBus:      eventBus,
		PluginManager: pluginManager,
	}

	hm := NewHooksManagerWithOptions(shared.DefaultHooksConfig(), opts)

	if !hm.HasEventBus() {
		t.Error("expected HasEventBus to be true")
	}

	if !hm.HasPluginManager() {
		t.Error("expected HasPluginManager to be true")
	}
}

func TestHooksManager_SetEventBus(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	if hm.HasEventBus() {
		t.Error("expected HasEventBus to be false initially")
	}

	eventBus := &mockEventEmitter{}
	hm.SetEventBus(eventBus)

	if !hm.HasEventBus() {
		t.Error("expected HasEventBus to be true after setting")
	}
}

func TestHooksManager_SetPluginManager(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	if hm.HasPluginManager() {
		t.Error("expected HasPluginManager to be false initially")
	}

	pm := &mockPluginInvoker{extensionPoints: map[string]bool{}}
	hm.SetPluginManager(pm)

	if !hm.HasPluginManager() {
		t.Error("expected HasPluginManager to be true after setting")
	}
}

func TestHooksManager_EventEmission(t *testing.T) {
	eventBus := &mockEventEmitter{}
	hm := NewHooksManagerWithOptions(shared.DefaultHooksConfig(), ManagerOptions{
		EventBus: eventBus,
	})

	// Test initialize event
	_ = hm.Initialize()
	events := eventBus.getEvents()
	if len(events) != 1 || events[0].Type != shared.EventType("hooks.initialized") {
		t.Errorf("expected hooks.initialized event, got %v", events)
	}

	// Test register event
	hook := &shared.HookRegistration{
		ID:    "test-hook",
		Name:  "Test Hook",
		Event: shared.HookEventPreEdit,
	}
	_ = hm.Register(hook)
	events = eventBus.getEvents()
	if len(events) != 2 || events[1].Type != shared.EventType("hooks.registered") {
		t.Errorf("expected hooks.registered event, got %v", events)
	}

	// Test execute event
	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}
	_, _ = hm.Execute(ctx, hookCtx)
	events = eventBus.getEvents()
	if len(events) != 3 || events[2].Type != shared.EventType("hooks.executed") {
		t.Errorf("expected hooks.executed event, got %v", events)
	}

	// Test unregister event
	_ = hm.Unregister("test-hook")
	events = eventBus.getEvents()
	if len(events) != 4 || events[3].Type != shared.EventType("hooks.unregistered") {
		t.Errorf("expected hooks.unregistered event, got %v", events)
	}

	// Test shutdown event
	_ = hm.Shutdown()
	events = eventBus.getEvents()
	if len(events) != 5 || events[4].Type != shared.EventType("hooks.shutdown") {
		t.Errorf("expected hooks.shutdown event, got %v", events)
	}
}

func TestHooksManager_PluginIntegration(t *testing.T) {
	pm := &mockPluginInvoker{
		extensionPoints: map[string]bool{
			"hooks.pre-edit": true,
		},
	}
	hm := NewHooksManagerWithOptions(shared.DefaultHooksConfig(), ManagerOptions{
		PluginManager: pm,
	})
	_ = hm.Initialize()

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}

	_, _ = hm.Execute(ctx, hookCtx)

	invocations := pm.getInvocations()
	if len(invocations) != 1 || invocations[0] != "hooks.pre-edit" {
		t.Errorf("expected plugin extension point invocation, got %v", invocations)
	}
}

func TestHooksManager_ExecuteWithPluginContext(t *testing.T) {
	pm := &mockPluginInvoker{
		extensionPoints: map[string]bool{
			"hooks.before.pre-edit": true,
			"hooks.after.pre-edit":  true,
		},
	}
	hm := NewHooksManagerWithOptions(shared.DefaultHooksConfig(), ManagerOptions{
		PluginManager: pm,
	})
	_ = hm.Initialize()

	ctx := context.Background()
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Timestamp: shared.Now(),
	}

	_, _ = hm.ExecuteWithPluginContext(ctx, hookCtx)

	invocations := pm.getInvocations()
	// Should have 3 invocations: before, during (hooks.pre-edit), and after
	// But hooks.pre-edit is not in the map, so only before and after
	if len(invocations) != 2 {
		t.Errorf("expected 2 plugin extension point invocations, got %d: %v", len(invocations), invocations)
	}

	// Verify order
	if invocations[0] != "hooks.before.pre-edit" {
		t.Errorf("expected first invocation to be hooks.before.pre-edit, got %s", invocations[0])
	}
	if invocations[1] != "hooks.after.pre-edit" {
		t.Errorf("expected second invocation to be hooks.after.pre-edit, got %s", invocations[1])
	}
}

func TestHooksManager_RecordEditPattern(t *testing.T) {
	eventBus := &mockEventEmitter{}
	hm := NewHooksManagerWithOptions(shared.DefaultHooksConfig(), ManagerOptions{
		EventBus: eventBus,
	})
	_ = hm.Initialize()

	pattern, err := hm.RecordEditPattern("/path/to/file.go", "modify", true, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if pattern == nil {
		t.Fatal("expected pattern to be returned")
	}

	if pattern.Type != shared.PatternTypeEdit {
		t.Errorf("expected pattern type edit, got %s", pattern.Type)
	}

	// Check event was emitted
	events := eventBus.getEvents()
	found := false
	for _, e := range events {
		if e.Type == shared.EventType("hooks.pattern_learned") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hooks.pattern_learned event to be emitted")
	}
}

func TestHooksManager_RecordCommandPattern(t *testing.T) {
	eventBus := &mockEventEmitter{}
	hm := NewHooksManagerWithOptions(shared.DefaultHooksConfig(), ManagerOptions{
		EventBus: eventBus,
	})
	_ = hm.Initialize()

	pattern, err := hm.RecordCommandPattern("go build ./...", true, 0, 1500)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if pattern == nil {
		t.Fatal("expected pattern to be returned")
	}

	if pattern.Type != shared.PatternTypeCommand {
		t.Errorf("expected pattern type command, got %s", pattern.Type)
	}

	// Check event was emitted
	events := eventBus.getEvents()
	found := false
	for _, e := range events {
		if e.Type == shared.EventType("hooks.pattern_learned") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hooks.pattern_learned event to be emitted")
	}
}

func TestHooksManager_RecordPatternOutcome(t *testing.T) {
	eventBus := &mockEventEmitter{}
	hm := NewHooksManagerWithOptions(shared.DefaultHooksConfig(), ManagerOptions{
		EventBus: eventBus,
	})
	_ = hm.Initialize()

	// First create a pattern
	pattern, _ := hm.RecordEditPattern("/path/to/file.go", "modify", true, nil)

	// Record success outcome
	err := hm.RecordPatternOutcome(pattern.ID, true)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify success rate increased
	successRate := hm.GetPatternStore().GetSuccessRate(pattern.ID)
	if successRate < 0.5 {
		t.Errorf("expected success rate > 0.5, got %f", successRate)
	}

	// Record failure outcome
	err = hm.RecordPatternOutcome(pattern.ID, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHooksManager_RouteTask(t *testing.T) {
	eventBus := &mockEventEmitter{}
	hm := NewHooksManagerWithOptions(shared.DefaultHooksConfig(), ManagerOptions{
		EventBus: eventBus,
	})
	_ = hm.Initialize()

	result := hm.RouteTask("implement a new feature", nil, nil, nil)

	if result == nil {
		t.Fatal("expected routing result")
	}

	if result.RecommendedAgent == "" {
		t.Error("expected a recommended agent")
	}

	// Check event was emitted
	events := eventBus.getEvents()
	found := false
	for _, e := range events {
		if e.Type == shared.EventType("hooks.routing_decision") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hooks.routing_decision event to be emitted")
	}
}

func TestHooksManager_RecordRoutingOutcome(t *testing.T) {
	eventBus := &mockEventEmitter{}
	hm := NewHooksManagerWithOptions(shared.DefaultHooksConfig(), ManagerOptions{
		EventBus: eventBus,
	})
	_ = hm.Initialize()

	// First route a task
	result := hm.RouteTask("implement a new feature", nil, nil, nil)

	// Record outcome
	err := hm.RecordRoutingOutcome(result.ID, true, 5000)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check event was emitted
	events := eventBus.getEvents()
	found := false
	for _, e := range events {
		if e.Type == shared.EventType("hooks.routing_outcome") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hooks.routing_outcome event to be emitted")
	}
}

func TestHooksManager_ExplainRouting(t *testing.T) {
	hm := NewHooksManagerWithDefaults()
	_ = hm.Initialize()

	explanation := hm.ExplainRouting("implement a new feature", nil, true)

	if explanation == nil {
		t.Fatal("expected routing explanation")
	}

	if explanation.Task != "implement a new feature" {
		t.Errorf("expected task to be 'implement a new feature', got %s", explanation.Task)
	}

	if explanation.Analysis == "" {
		t.Error("expected analysis to be set")
	}
}

func TestHooksManager_IsRunning(t *testing.T) {
	hm := NewHooksManagerWithDefaults()

	if hm.IsRunning() {
		t.Error("expected IsRunning to be false before initialize")
	}

	_ = hm.Initialize()

	if !hm.IsRunning() {
		t.Error("expected IsRunning to be true after initialize")
	}

	_ = hm.Shutdown()

	if hm.IsRunning() {
		t.Error("expected IsRunning to be false after shutdown")
	}
}
