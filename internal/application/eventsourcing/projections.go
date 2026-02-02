// Package eventsourcing provides application services for event sourcing.
package eventsourcing

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	domain "github.com/anthropics/claude-flow-go/internal/domain/eventsourcing"
	infra "github.com/anthropics/claude-flow-go/internal/infrastructure/eventsourcing"
)

// Projection processes events to build read models.
type Projection interface {
	// Name returns the projection name.
	Name() string

	// Handle processes an event.
	Handle(ctx context.Context, event *domain.StoredEvent) error

	// Reset resets the projection state.
	Reset(ctx context.Context) error

	// Version returns the last processed event version.
	Version() int64

	// SetVersion sets the last processed version.
	SetVersion(version int64)
}

// ProjectionStatus represents the status of a projection.
type ProjectionStatus string

const (
	ProjectionStatusStopped   ProjectionStatus = "stopped"
	ProjectionStatusRunning   ProjectionStatus = "running"
	ProjectionStatusCatchingUp ProjectionStatus = "catching_up"
	ProjectionStatusError     ProjectionStatus = "error"
)

// ProjectionState contains the state of a projection.
type ProjectionState struct {
	// Name is the projection name.
	Name string `json:"name"`

	// Status is the current status.
	Status ProjectionStatus `json:"status"`

	// Version is the last processed event version.
	Version int64 `json:"version"`

	// ProcessedCount is the number of events processed.
	ProcessedCount int64 `json:"processedCount"`

	// ErrorCount is the number of errors.
	ErrorCount int64 `json:"errorCount"`

	// LastError is the last error message.
	LastError string `json:"lastError,omitempty"`

	// LastProcessedAt is when the last event was processed.
	LastProcessedAt *time.Time `json:"lastProcessedAt,omitempty"`

	// StartedAt is when the projection started.
	StartedAt *time.Time `json:"startedAt,omitempty"`
}

// ProjectionManager manages projections.
type ProjectionManager struct {
	mu            sync.RWMutex
	eventStore    infra.EventStore
	projections   map[string]*managedProjection
	subscriptions []domain.Subscription
	running       bool
	stopChan      chan struct{}
}

type managedProjection struct {
	projection Projection
	state      *ProjectionState
}

// NewProjectionManager creates a new projection manager.
func NewProjectionManager(eventStore infra.EventStore) *ProjectionManager {
	return &ProjectionManager{
		eventStore:    eventStore,
		projections:   make(map[string]*managedProjection),
		subscriptions: make([]domain.Subscription, 0),
	}
}

// Register registers a projection.
func (m *ProjectionManager) Register(projection Projection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.projections[projection.Name()] = &managedProjection{
		projection: projection,
		state: &ProjectionState{
			Name:   projection.Name(),
			Status: ProjectionStatusStopped,
		},
	}
}

// Unregister removes a projection.
func (m *ProjectionManager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.projections, name)
}

// Start starts all projections.
func (m *ProjectionManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	// Catch up all projections
	if err := m.CatchUpAll(ctx); err != nil {
		return err
	}

	// Subscribe to live events
	sub := m.eventStore.SubscribeAll(func(event *domain.StoredEvent) error {
		return m.dispatchEvent(ctx, event)
	})
	m.subscriptions = append(m.subscriptions, sub)

	// Update status
	m.mu.Lock()
	for _, mp := range m.projections {
		mp.state.Status = ProjectionStatusRunning
		now := time.Now()
		mp.state.StartedAt = &now
	}
	m.mu.Unlock()

	return nil
}

// Stop stops all projections.
func (m *ProjectionManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)

	// Unsubscribe all
	for _, sub := range m.subscriptions {
		sub.Unsubscribe()
	}
	m.subscriptions = make([]domain.Subscription, 0)

	// Update status
	for _, mp := range m.projections {
		mp.state.Status = ProjectionStatusStopped
	}
}

// CatchUpAll catches up all projections to current state.
func (m *ProjectionManager) CatchUpAll(ctx context.Context) error {
	m.mu.RLock()
	projections := make([]*managedProjection, 0, len(m.projections))
	for _, mp := range m.projections {
		projections = append(projections, mp)
	}
	m.mu.RUnlock()

	for _, mp := range projections {
		if err := m.catchUp(ctx, mp); err != nil {
			return err
		}
	}

	return nil
}

// CatchUp catches up a specific projection.
func (m *ProjectionManager) CatchUp(ctx context.Context, name string) error {
	m.mu.RLock()
	mp, exists := m.projections[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("projection not found: %s", name)
	}

	return m.catchUp(ctx, mp)
}

// catchUp catches up a projection.
func (m *ProjectionManager) catchUp(ctx context.Context, mp *managedProjection) error {
	mp.state.Status = ProjectionStatusCatchingUp

	// Get current version
	currentVersion := mp.projection.Version()

	// Load events from that version
	events, err := m.eventStore.Query(ctx, &domain.EventQuery{
		FromVersion: int(currentVersion) + 1,
	})
	if err != nil {
		mp.state.Status = ProjectionStatusError
		mp.state.LastError = err.Error()
		return err
	}

	// Process events
	for _, event := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := m.processEvent(ctx, mp, event); err != nil {
			mp.state.Status = ProjectionStatusError
			return err
		}
	}

	return nil
}

// dispatchEvent dispatches an event to all projections.
func (m *ProjectionManager) dispatchEvent(ctx context.Context, event *domain.StoredEvent) error {
	m.mu.RLock()
	projections := make([]*managedProjection, 0, len(m.projections))
	for _, mp := range m.projections {
		projections = append(projections, mp)
	}
	m.mu.RUnlock()

	var lastErr error
	for _, mp := range projections {
		if err := m.processEvent(ctx, mp, event); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// processEvent processes an event through a projection.
func (m *ProjectionManager) processEvent(ctx context.Context, mp *managedProjection, event *domain.StoredEvent) error {
	if err := mp.projection.Handle(ctx, event); err != nil {
		atomic.AddInt64(&mp.state.ErrorCount, 1)
		mp.state.LastError = err.Error()
		return err
	}

	atomic.AddInt64(&mp.state.ProcessedCount, 1)
	mp.projection.SetVersion(int64(event.Version))
	now := time.Now()
	mp.state.LastProcessedAt = &now

	return nil
}

// Reset resets a projection.
func (m *ProjectionManager) Reset(ctx context.Context, name string) error {
	m.mu.RLock()
	mp, exists := m.projections[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("projection not found: %s", name)
	}

	if err := mp.projection.Reset(ctx); err != nil {
		return err
	}

	mp.state.Version = 0
	mp.state.ProcessedCount = 0
	mp.state.ErrorCount = 0
	mp.state.LastError = ""
	mp.state.LastProcessedAt = nil

	return nil
}

// Rebuild resets and rebuilds a projection.
func (m *ProjectionManager) Rebuild(ctx context.Context, name string) error {
	if err := m.Reset(ctx, name); err != nil {
		return err
	}
	return m.CatchUp(ctx, name)
}

// RebuildAll resets and rebuilds all projections.
func (m *ProjectionManager) RebuildAll(ctx context.Context) error {
	m.mu.RLock()
	names := make([]string, 0, len(m.projections))
	for name := range m.projections {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		if err := m.Rebuild(ctx, name); err != nil {
			return err
		}
	}

	return nil
}

// GetState returns the state of a projection.
func (m *ProjectionManager) GetState(name string) (*ProjectionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mp, exists := m.projections[name]
	if !exists {
		return nil, fmt.Errorf("projection not found: %s", name)
	}

	return mp.state, nil
}

// GetAllStates returns the state of all projections.
func (m *ProjectionManager) GetAllStates() []*ProjectionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]*ProjectionState, 0, len(m.projections))
	for _, mp := range m.projections {
		states = append(states, mp.state)
	}

	return states
}

// IsRunning returns true if the manager is running.
func (m *ProjectionManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// BaseProjection provides a base implementation for projections.
type BaseProjection struct {
	mu      sync.RWMutex
	name    string
	version int64
}

// NewBaseProjection creates a new base projection.
func NewBaseProjection(name string) *BaseProjection {
	return &BaseProjection{name: name}
}

// Name returns the projection name.
func (p *BaseProjection) Name() string {
	return p.name
}

// Version returns the last processed version.
func (p *BaseProjection) Version() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.version
}

// SetVersion sets the last processed version.
func (p *BaseProjection) SetVersion(version int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.version = version
}

// Handle is a placeholder - subclasses should override.
func (p *BaseProjection) Handle(ctx context.Context, event *domain.StoredEvent) error {
	return nil
}

// Reset resets the projection.
func (p *BaseProjection) Reset(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.version = 0
	return nil
}

// FunctionalProjection wraps a function as a projection.
type FunctionalProjection struct {
	*BaseProjection
	handler   func(ctx context.Context, event *domain.StoredEvent) error
	resetFunc func(ctx context.Context) error
}

// NewFunctionalProjection creates a new functional projection.
func NewFunctionalProjection(name string, handler func(ctx context.Context, event *domain.StoredEvent) error) *FunctionalProjection {
	return &FunctionalProjection{
		BaseProjection: NewBaseProjection(name),
		handler:        handler,
	}
}

// WithReset sets the reset function.
func (p *FunctionalProjection) WithReset(resetFunc func(ctx context.Context) error) *FunctionalProjection {
	p.resetFunc = resetFunc
	return p
}

// Handle processes an event.
func (p *FunctionalProjection) Handle(ctx context.Context, event *domain.StoredEvent) error {
	if p.handler != nil {
		return p.handler(ctx, event)
	}
	return nil
}

// Reset resets the projection.
func (p *FunctionalProjection) Reset(ctx context.Context) error {
	if err := p.BaseProjection.Reset(ctx); err != nil {
		return err
	}
	if p.resetFunc != nil {
		return p.resetFunc(ctx)
	}
	return nil
}

// FilteredProjection wraps a projection with event type filtering.
type FilteredProjection struct {
	inner      Projection
	eventTypes map[string]bool
}

// NewFilteredProjection creates a new filtered projection.
func NewFilteredProjection(inner Projection, eventTypes []string) *FilteredProjection {
	typeMap := make(map[string]bool)
	for _, t := range eventTypes {
		typeMap[t] = true
	}
	return &FilteredProjection{
		inner:      inner,
		eventTypes: typeMap,
	}
}

// Name returns the projection name.
func (p *FilteredProjection) Name() string {
	return p.inner.Name()
}

// Handle processes an event if it matches the filter.
func (p *FilteredProjection) Handle(ctx context.Context, event *domain.StoredEvent) error {
	if len(p.eventTypes) > 0 && !p.eventTypes[event.Type] {
		return nil // Skip
	}
	return p.inner.Handle(ctx, event)
}

// Reset resets the projection.
func (p *FilteredProjection) Reset(ctx context.Context) error {
	return p.inner.Reset(ctx)
}

// Version returns the last processed version.
func (p *FilteredProjection) Version() int64 {
	return p.inner.Version()
}

// SetVersion sets the last processed version.
func (p *FilteredProjection) SetVersion(version int64) {
	p.inner.SetVersion(version)
}
