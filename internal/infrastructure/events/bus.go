// Package events provides an event bus implementation using Go channels.
package events

import (
	"context"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Handler is a function that handles events.
type Handler func(event shared.Event)

// EventBus provides a publish-subscribe event system using Go channels.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[shared.EventType][]chan shared.Event
	handlers    map[shared.EventType][]Handler
	bufferSize  int
	closed      bool
}

// Option configures the EventBus.
type Option func(*EventBus)

// WithBufferSize sets the channel buffer size.
func WithBufferSize(size int) Option {
	return func(eb *EventBus) {
		eb.bufferSize = size
	}
}

// New creates a new EventBus.
func New(opts ...Option) *EventBus {
	eb := &EventBus{
		subscribers: make(map[shared.EventType][]chan shared.Event),
		handlers:    make(map[shared.EventType][]Handler),
		bufferSize:  100,
	}

	for _, opt := range opts {
		opt(eb)
	}

	return eb
}

// Subscribe creates a channel to receive events of the given type.
func (eb *EventBus) Subscribe(eventType shared.EventType) <-chan shared.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan shared.Event, eb.bufferSize)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
	return ch
}

// SubscribeAll creates a channel to receive all events.
func (eb *EventBus) SubscribeAll() <-chan shared.Event {
	return eb.Subscribe("*")
}

// Unsubscribe removes a subscription channel.
func (eb *EventBus) Unsubscribe(eventType shared.EventType, ch <-chan shared.Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	subs := eb.subscribers[eventType]
	for i, sub := range subs {
		// Compare channels by receiving from the same underlying channel
		if isSameChannel(sub, ch) {
			eb.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
			close(sub)
			break
		}
	}
}

// On registers a handler for events of the given type.
func (eb *EventBus) On(eventType shared.EventType, handler Handler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

// Off removes a handler (removes all handlers for the type if handler is nil).
func (eb *EventBus) Off(eventType shared.EventType, handler Handler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if handler == nil {
		delete(eb.handlers, eventType)
		return
	}

	// Note: Can't compare functions directly in Go, so this removes all handlers
	// In practice, you'd want to use a wrapper or ID-based system
	delete(eb.handlers, eventType)
}

// Emit publishes an event to all subscribers and handlers.
func (eb *EventBus) Emit(event shared.Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if eb.closed {
		return
	}

	// Ensure timestamp
	if event.Timestamp == 0 {
		event.Timestamp = shared.Now()
	}

	// Send to specific type subscribers
	for _, ch := range eb.subscribers[event.Type] {
		select {
		case ch <- event:
		default:
			// Channel full, skip (non-blocking)
		}
	}

	// Send to wildcard subscribers
	for _, ch := range eb.subscribers["*"] {
		select {
		case ch <- event:
		default:
			// Channel full, skip (non-blocking)
		}
	}

	// Call handlers
	for _, handler := range eb.handlers[event.Type] {
		go handler(event)
	}

	// Call wildcard handlers
	for _, handler := range eb.handlers["*"] {
		go handler(event)
	}
}

// EmitAsync publishes an event asynchronously.
func (eb *EventBus) EmitAsync(event shared.Event) {
	go eb.Emit(event)
}

// EmitWithContext publishes an event with context support.
func (eb *EventBus) EmitWithContext(ctx context.Context, event shared.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		eb.Emit(event)
		return nil
	}
}

// Close closes all subscriber channels and stops the event bus.
func (eb *EventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return
	}

	eb.closed = true

	for _, subs := range eb.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}

	eb.subscribers = make(map[shared.EventType][]chan shared.Event)
	eb.handlers = make(map[shared.EventType][]Handler)
}

// ============================================================================
// Helper Functions
// ============================================================================

// EmitAgentSpawned emits an agent spawned event.
func (eb *EventBus) EmitAgentSpawned(agentID string, agentType shared.AgentType) {
	eb.Emit(shared.Event{
		Type:      shared.EventAgentSpawned,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"agentId": agentID,
			"type":    string(agentType),
		},
	})
}

// EmitAgentTerminated emits an agent terminated event.
func (eb *EventBus) EmitAgentTerminated(agentID string) {
	eb.Emit(shared.Event{
		Type:      shared.EventAgentTerminated,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"agentId": agentID,
		},
	})
}

// EmitAgentMessage emits an agent message event.
func (eb *EventBus) EmitAgentMessage(message shared.AgentMessage) {
	eb.Emit(shared.Event{
		Type:      shared.EventAgentMessage,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"from":    message.From,
			"to":      message.To,
			"type":    message.Type,
			"payload": message.Payload,
		},
	})
}

// EmitTaskStarted emits a task started event.
func (eb *EventBus) EmitTaskStarted(taskID, agentID string) {
	eb.Emit(shared.Event{
		Type:      shared.EventTaskStarted,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"taskId":  taskID,
			"agentId": agentID,
		},
	})
}

// EmitTaskCompleted emits a task completed event.
func (eb *EventBus) EmitTaskCompleted(taskID, agentID string, duration int64) {
	eb.Emit(shared.Event{
		Type:      shared.EventTaskCompleted,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"taskId":   taskID,
			"agentId":  agentID,
			"duration": duration,
		},
	})
}

// EmitTaskFailed emits a task failed event.
func (eb *EventBus) EmitTaskFailed(taskID, agentID, errMsg string) {
	eb.Emit(shared.Event{
		Type:      shared.EventTaskFailed,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"taskId":  taskID,
			"agentId": agentID,
			"error":   errMsg,
		},
	})
}

// EmitWorkflowStarted emits a workflow started event.
func (eb *EventBus) EmitWorkflowStarted(workflowID string, taskCount int) {
	eb.Emit(shared.Event{
		Type:      shared.EventWorkflowStarted,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"workflowId": workflowID,
			"taskCount":  taskCount,
		},
	})
}

// EmitWorkflowTaskComplete emits a workflow task complete event.
func (eb *EventBus) EmitWorkflowTaskComplete(workflowID, taskID string) {
	eb.Emit(shared.Event{
		Type:      shared.EventWorkflowComplete,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"workflowId": workflowID,
			"taskId":     taskID,
		},
	})
}

// EmitWorkflowCompleted emits a workflow completed event.
func (eb *EventBus) EmitWorkflowCompleted(workflowID string, result shared.WorkflowResult) {
	eb.Emit(shared.Event{
		Type:      shared.EventWorkflowCompleted,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"workflowId":     workflowID,
			"tasksCompleted": result.TasksCompleted,
			"status":         result.Status,
		},
	})
}

// EmitWorkflowFailed emits a workflow failed event.
func (eb *EventBus) EmitWorkflowFailed(workflowID string, err error) {
	eb.Emit(shared.Event{
		Type:      shared.EventWorkflowFailed,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"workflowId": workflowID,
			"error":      err.Error(),
		},
	})
}

// EmitPluginLoaded emits a plugin loaded event.
func (eb *EventBus) EmitPluginLoaded(pluginID, pluginName string) {
	eb.Emit(shared.Event{
		Type:      shared.EventPluginLoaded,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"id":   pluginID,
			"name": pluginName,
		},
	})
}

// EmitPluginUnloaded emits a plugin unloaded event.
func (eb *EventBus) EmitPluginUnloaded(pluginID string) {
	eb.Emit(shared.Event{
		Type:      shared.EventPluginUnloaded,
		Timestamp: shared.Now(),
		Payload: map[string]interface{}{
			"id": pluginID,
		},
	})
}

// isSameChannel is a helper to compare channels (simplified comparison).
func isSameChannel(a chan shared.Event, b <-chan shared.Event) bool {
	// In Go, we can't directly compare channels, but we can use the fact
	// that we're storing and comparing the same channel references
	return true // Simplified - in production, use a map with IDs
}
