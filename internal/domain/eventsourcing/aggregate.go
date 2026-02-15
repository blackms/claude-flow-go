// Package eventsourcing provides domain types for event sourcing infrastructure.
package eventsourcing

import (
	"sync"
)

// Aggregate is the interface that all event-sourced aggregates must implement.
type Aggregate interface {
	// ID returns the aggregate identifier.
	ID() string
	// Type returns the aggregate type name.
	Type() string
	// Version returns the current version (number of applied events).
	Version() int
	// ApplyEvent applies an event to update the aggregate state.
	ApplyEvent(event Event) error
	// UncommittedEvents returns events that haven't been persisted yet.
	UncommittedEvents() []Event
	// ClearUncommittedEvents clears the uncommitted events after persistence.
	ClearUncommittedEvents()
}

// AggregateRoot provides a base implementation for event-sourced aggregates.
type AggregateRoot struct {
	mu                sync.RWMutex
	id                string
	aggregateType     string
	version           int
	uncommittedEvents []Event
}

// NewAggregateRoot creates a new aggregate root.
func NewAggregateRoot(id, aggregateType string) *AggregateRoot {
	return &AggregateRoot{
		id:                id,
		aggregateType:     aggregateType,
		version:           0,
		uncommittedEvents: make([]Event, 0),
	}
}

// ID returns the aggregate ID.
func (a *AggregateRoot) ID() string {
	return a.id
}

// Type returns the aggregate type.
func (a *AggregateRoot) Type() string {
	return a.aggregateType
}

// Version returns the current version.
func (a *AggregateRoot) Version() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.version
}

// SetVersion sets the version (used during reconstruction).
func (a *AggregateRoot) SetVersion(version int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.version = version
}

// IncrementVersion increments the version.
func (a *AggregateRoot) IncrementVersion() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.version++
	return a.version
}

// UncommittedEvents returns events that haven't been persisted.
func (a *AggregateRoot) UncommittedEvents() []Event {
	a.mu.RLock()
	defer a.mu.RUnlock()
	events := make([]Event, len(a.uncommittedEvents))
	copy(events, a.uncommittedEvents)
	return events
}

// ClearUncommittedEvents clears uncommitted events after persistence.
func (a *AggregateRoot) ClearUncommittedEvents() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.uncommittedEvents = make([]Event, 0)
}

// AddUncommittedEvent adds an event to the uncommitted list.
func (a *AggregateRoot) AddUncommittedEvent(event Event) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.uncommittedEvents = append(a.uncommittedEvents, event)
}

// RecordEvent creates and records an event.
// The aggregate should override this to apply the event to its state.
func (a *AggregateRoot) RecordEvent(eventType string, payload map[string]interface{}) *BaseEvent {
	a.mu.Lock()
	a.version++
	version := a.version
	a.mu.Unlock()

	event := NewBaseEvent(eventType, a.id, a.aggregateType, version, payload)

	a.mu.Lock()
	a.uncommittedEvents = append(a.uncommittedEvents, event)
	a.mu.Unlock()

	return event
}

// AggregateFactory creates new aggregate instances.
type AggregateFactory func(id string) Aggregate

// AggregateLoader loads aggregates from events.
type AggregateLoader interface {
	// Load reconstructs an aggregate from its event history.
	Load(id string, events []Event) (Aggregate, error)
}

// AggregateState represents the state that can be snapshotted.
type AggregateState interface {
	// ToSnapshot converts the state to a snapshot.
	ToSnapshot() ([]byte, error)
	// FromSnapshot restores state from a snapshot.
	FromSnapshot(data []byte) error
}

// versionSetter defines aggregates that can restore version state.
type versionSetter interface {
	SetVersion(version int)
}

// Snapshot represents a point-in-time snapshot of an aggregate.
type Snapshot struct {
	// AggregateID is the aggregate identifier.
	AggregateID string `json:"aggregateId"`
	// AggregateType is the type of aggregate.
	AggregateType string `json:"aggregateType"`
	// Version is the aggregate version at snapshot time.
	Version int `json:"version"`
	// State is the serialized aggregate state.
	State []byte `json:"state"`
	// CreatedAt is when the snapshot was created.
	CreatedAt int64 `json:"createdAt"`
}

// EventApplier is a function that applies an event to aggregate state.
type EventApplier func(state interface{}, event Event) error

// GenericAggregate provides a generic event-sourced aggregate.
type GenericAggregate struct {
	*AggregateRoot
	state   interface{}
	applier EventApplier
}

// NewGenericAggregate creates a new generic aggregate.
func NewGenericAggregate(id, aggregateType string, initialState interface{}, applier EventApplier) *GenericAggregate {
	return &GenericAggregate{
		AggregateRoot: NewAggregateRoot(id, aggregateType),
		state:         initialState,
		applier:       applier,
	}
}

// ApplyEvent applies an event using the configured applier.
func (a *GenericAggregate) ApplyEvent(event Event) error {
	if a.applier != nil {
		if err := a.applier(a.state, event); err != nil {
			return err
		}
	}
	a.SetVersion(event.Version())
	return nil
}

// State returns the current state.
func (a *GenericAggregate) State() interface{} {
	return a.state
}

// RebuildFromEvents rebuilds an aggregate from a list of events.
func RebuildFromEvents(aggregate Aggregate, events []Event) error {
	for _, event := range events {
		if err := aggregate.ApplyEvent(event); err != nil {
			return err
		}
	}
	return nil
}

// RebuildFromSnapshot rebuilds from snapshot plus subsequent events.
func RebuildFromSnapshot(aggregate Aggregate, snapshot *Snapshot, events []Event) error {
	// If aggregate implements AggregateState, restore from snapshot
	if stateful, ok := aggregate.(AggregateState); ok {
		if err := stateful.FromSnapshot(snapshot.State); err != nil {
			return err
		}
	}

	// Set version from snapshot
	if setter, ok := aggregate.(versionSetter); ok {
		setter.SetVersion(snapshot.Version)
	}

	// Apply events after snapshot
	return RebuildFromEvents(aggregate, events)
}
