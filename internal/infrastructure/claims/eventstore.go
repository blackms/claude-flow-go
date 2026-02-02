// Package claims provides infrastructure for the claims system.
package claims

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	domainClaims "github.com/anthropics/claude-flow-go/internal/domain/claims"
)

// EventStore defines the interface for the event store.
type EventStore interface {
	Append(event *domainClaims.ClaimEvent) error
	AppendBatch(events []*domainClaims.ClaimEvent) error
	GetEvents(aggregateID string, fromVersion int) ([]*domainClaims.ClaimEvent, error)
	GetEventsByType(eventType domainClaims.ClaimEventType) ([]*domainClaims.ClaimEvent, error)
	GetAllEvents() ([]*domainClaims.ClaimEvent, error)
	Query(filter domainClaims.EventFilter) ([]*domainClaims.ClaimEvent, error)
	Subscribe(eventTypes []domainClaims.ClaimEventType, handler domainClaims.EventHandler) string
	SubscribeAll(handler domainClaims.EventHandler) string
	Unsubscribe(subscriptionID string)
	GetAggregateVersion(aggregateID string) (int, error)
	SaveSnapshot(aggregateID string, state interface{}, version int) error
	GetSnapshot(aggregateID string) (*Snapshot, error)
	Count() int
}

// Snapshot represents a snapshot of an aggregate state.
type Snapshot struct {
	AggregateID string      `json:"aggregateId"`
	Version     int         `json:"version"`
	State       interface{} `json:"state"`
	CreatedAt   time.Time   `json:"createdAt"`
}

// Subscription represents an event subscription.
type Subscription struct {
	ID         string
	EventTypes []domainClaims.ClaimEventType
	Handler    domainClaims.EventHandler
	All        bool
}

// InMemoryEventStore provides an in-memory event store implementation.
type InMemoryEventStore struct {
	mu            sync.RWMutex
	events        []*domainClaims.ClaimEvent
	byAggregate   map[string][]*domainClaims.ClaimEvent
	byType        map[domainClaims.ClaimEventType][]*domainClaims.ClaimEvent
	versions      map[string]int
	snapshots     map[string]*Snapshot
	subscriptions map[string]*Subscription
}

// NewInMemoryEventStore creates a new in-memory event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events:        make([]*domainClaims.ClaimEvent, 0),
		byAggregate:   make(map[string][]*domainClaims.ClaimEvent),
		byType:        make(map[domainClaims.ClaimEventType][]*domainClaims.ClaimEvent),
		versions:      make(map[string]int),
		snapshots:     make(map[string]*Snapshot),
		subscriptions: make(map[string]*Subscription),
	}
}

// Append appends an event to the store.
func (s *InMemoryEventStore) Append(event *domainClaims.ClaimEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check version for optimistic concurrency
	currentVersion := s.versions[event.AggregateID]
	if event.Version != currentVersion+1 {
		return fmt.Errorf("version conflict: expected %d, got %d", currentVersion+1, event.Version)
	}

	// Append to main log
	s.events = append(s.events, event)

	// Index by aggregate
	s.byAggregate[event.AggregateID] = append(s.byAggregate[event.AggregateID], event)

	// Index by type
	s.byType[event.Type] = append(s.byType[event.Type], event)

	// Update version
	s.versions[event.AggregateID] = event.Version

	// Notify subscribers
	s.notifySubscribers(event)

	return nil
}

// AppendBatch appends multiple events atomically.
func (s *InMemoryEventStore) AppendBatch(events []*domainClaims.ClaimEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate all versions first
	for _, event := range events {
		currentVersion := s.versions[event.AggregateID]
		expectedVersion := currentVersion + 1
		for _, e := range events {
			if e.AggregateID == event.AggregateID && e != event {
				expectedVersion++
			}
		}
		if event.Version > expectedVersion {
			return fmt.Errorf("version conflict for %s: expected <= %d, got %d",
				event.AggregateID, expectedVersion, event.Version)
		}
	}

	// Append all events
	for _, event := range events {
		s.events = append(s.events, event)
		s.byAggregate[event.AggregateID] = append(s.byAggregate[event.AggregateID], event)
		s.byType[event.Type] = append(s.byType[event.Type], event)
		s.versions[event.AggregateID] = event.Version
		s.notifySubscribers(event)
	}

	return nil
}

// GetEvents returns events for an aggregate from a given version.
func (s *InMemoryEventStore) GetEvents(aggregateID string, fromVersion int) ([]*domainClaims.ClaimEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.byAggregate[aggregateID]
	if len(events) == 0 {
		return []*domainClaims.ClaimEvent{}, nil
	}

	result := make([]*domainClaims.ClaimEvent, 0)
	for _, e := range events {
		if e.Version >= fromVersion {
			result = append(result, e)
		}
	}

	return result, nil
}

// GetEventsByType returns all events of a given type.
func (s *InMemoryEventStore) GetEventsByType(eventType domainClaims.ClaimEventType) ([]*domainClaims.ClaimEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.byType[eventType]
	result := make([]*domainClaims.ClaimEvent, len(events))
	copy(result, events)
	return result, nil
}

// GetAllEvents returns all events.
func (s *InMemoryEventStore) GetAllEvents() ([]*domainClaims.ClaimEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domainClaims.ClaimEvent, len(s.events))
	copy(result, s.events)
	return result, nil
}

// Query returns events matching the filter.
func (s *InMemoryEventStore) Query(filter domainClaims.EventFilter) ([]*domainClaims.ClaimEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domainClaims.ClaimEvent, 0)

	var source []*domainClaims.ClaimEvent
	if filter.AggregateID != "" {
		source = s.byAggregate[filter.AggregateID]
	} else {
		source = s.events
	}

	for _, event := range source {
		if filter.Matches(event) {
			result = append(result, event)
		}
	}

	return result, nil
}

// Subscribe subscribes to specific event types.
func (s *InMemoryEventStore) Subscribe(eventTypes []domainClaims.ClaimEventType, handler domainClaims.EventHandler) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	s.subscriptions[id] = &Subscription{
		ID:         id,
		EventTypes: eventTypes,
		Handler:    handler,
		All:        false,
	}

	return id
}

// SubscribeAll subscribes to all events.
func (s *InMemoryEventStore) SubscribeAll(handler domainClaims.EventHandler) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	s.subscriptions[id] = &Subscription{
		ID:      id,
		Handler: handler,
		All:     true,
	}

	return id
}

// Unsubscribe removes a subscription.
func (s *InMemoryEventStore) Unsubscribe(subscriptionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscriptions, subscriptionID)
}

// notifySubscribers notifies all matching subscribers of an event.
// Must be called with lock held.
func (s *InMemoryEventStore) notifySubscribers(event *domainClaims.ClaimEvent) {
	for _, sub := range s.subscriptions {
		if sub.All {
			go sub.Handler(event)
			continue
		}

		for _, t := range sub.EventTypes {
			if t == event.Type {
				go sub.Handler(event)
				break
			}
		}
	}
}

// GetAggregateVersion returns the current version of an aggregate.
func (s *InMemoryEventStore) GetAggregateVersion(aggregateID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.versions[aggregateID], nil
}

// SaveSnapshot saves a snapshot of an aggregate state.
func (s *InMemoryEventStore) SaveSnapshot(aggregateID string, state interface{}, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots[aggregateID] = &Snapshot{
		AggregateID: aggregateID,
		Version:     version,
		State:       state,
		CreatedAt:   time.Now(),
	}

	return nil
}

// GetSnapshot returns the snapshot for an aggregate.
func (s *InMemoryEventStore) GetSnapshot(aggregateID string) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, exists := s.snapshots[aggregateID]
	if !exists {
		return nil, nil
	}

	return snapshot, nil
}

// Count returns the total number of events.
func (s *InMemoryEventStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

// Clear clears all events (for testing).
func (s *InMemoryEventStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = make([]*domainClaims.ClaimEvent, 0)
	s.byAggregate = make(map[string][]*domainClaims.ClaimEvent)
	s.byType = make(map[domainClaims.ClaimEventType][]*domainClaims.ClaimEvent)
	s.versions = make(map[string]int)
	s.snapshots = make(map[string]*Snapshot)
}

// ReplayEvents replays events through a handler.
func (s *InMemoryEventStore) ReplayEvents(aggregateID string, handler domainClaims.EventHandler) error {
	events, err := s.GetEvents(aggregateID, 0)
	if err != nil {
		return err
	}

	for _, event := range events {
		handler(event)
	}

	return nil
}

// ReplayAllEvents replays all events through a handler.
func (s *InMemoryEventStore) ReplayAllEvents(handler domainClaims.EventHandler) error {
	events, err := s.GetAllEvents()
	if err != nil {
		return err
	}

	for _, event := range events {
		handler(event)
	}

	return nil
}
