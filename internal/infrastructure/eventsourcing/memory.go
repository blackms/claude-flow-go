// Package eventsourcing provides infrastructure for event sourcing.
package eventsourcing

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	domain "github.com/anthropics/claude-flow-go/internal/domain/eventsourcing"
)

// InMemoryEventStore implements EventStore using in-memory storage.
type InMemoryEventStore struct {
	mu            sync.RWMutex
	events        []*domain.StoredEvent
	byAggregate   map[string][]*domain.StoredEvent
	byType        map[string][]*domain.StoredEvent
	versions      map[string]int
	subscriptions map[string]*eventSubscription
	closed        bool
}

// NewInMemoryEventStore creates a new in-memory event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events:        make([]*domain.StoredEvent, 0),
		byAggregate:   make(map[string][]*domain.StoredEvent),
		byType:        make(map[string][]*domain.StoredEvent),
		versions:      make(map[string]int),
		subscriptions: make(map[string]*eventSubscription),
	}
}

// Append appends events to the store atomically.
func (s *InMemoryEventStore) Append(ctx context.Context, events []*domain.StoredEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return domain.ErrStoreClosed
	}

	if len(events) == 0 {
		return nil
	}

	// Validate all versions first
	for _, event := range events {
		currentVersion := s.versions[event.AggregateID]
		if event.Version != currentVersion+1 {
			return fmt.Errorf("%w: expected version %d, got %d",
				domain.ErrConcurrencyConflict, currentVersion+1, event.Version)
		}
		// Increment for next check in same batch
		s.versions[event.AggregateID] = event.Version
	}

	// Reset versions and apply for real
	for _, event := range events {
		s.versions[event.AggregateID] = event.Version - 1
	}

	// Append all events
	for _, event := range events {
		s.events = append(s.events, event)
		s.byAggregate[event.AggregateID] = append(s.byAggregate[event.AggregateID], event)
		s.byType[event.Type] = append(s.byType[event.Type], event)
		s.versions[event.AggregateID] = event.Version
	}

	// Notify subscribers
	for _, event := range events {
		s.notifySubscribersLocked(event)
	}

	return nil
}

// Load loads events for an aggregate from a given version.
func (s *InMemoryEventStore) Load(ctx context.Context, aggregateID string, fromVersion int) ([]*domain.StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	events := s.byAggregate[aggregateID]
	if len(events) == 0 {
		return []*domain.StoredEvent{}, nil
	}

	result := make([]*domain.StoredEvent, 0)
	for _, e := range events {
		if e.Version >= fromVersion {
			result = append(result, e)
		}
	}

	return result, nil
}

// LoadByType loads events by event type.
func (s *InMemoryEventStore) LoadByType(ctx context.Context, eventType string) ([]*domain.StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	events := s.byType[eventType]
	result := make([]*domain.StoredEvent, len(events))
	copy(result, events)
	return result, nil
}

// Query queries events matching the given criteria.
func (s *InMemoryEventStore) Query(ctx context.Context, query *domain.EventQuery) ([]*domain.StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	var source []*domain.StoredEvent
	if query.AggregateID != "" {
		source = s.byAggregate[query.AggregateID]
	} else {
		source = s.events
	}

	result := make([]*domain.StoredEvent, 0)
	for _, event := range source {
		if query.Matches(event) {
			result = append(result, event)
		}
	}

	// Apply limit and offset
	if query.Offset > 0 {
		if query.Offset >= len(result) {
			return []*domain.StoredEvent{}, nil
		}
		result = result[query.Offset:]
	}

	if query.Limit > 0 && len(result) > query.Limit {
		result = result[:query.Limit]
	}

	return result, nil
}

// GetVersion returns the current version of an aggregate.
func (s *InMemoryEventStore) GetVersion(ctx context.Context, aggregateID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, domain.ErrStoreClosed
	}

	return s.versions[aggregateID], nil
}

// Subscribe subscribes to events of specific types.
func (s *InMemoryEventStore) Subscribe(eventTypes []string, handler domain.EventHandler) domain.Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	s.subscriptions[id] = &eventSubscription{
		id:         id,
		eventTypes: eventTypes,
		handler:    handler,
		all:        false,
	}

	return newSubscription(id, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.subscriptions, id)
	})
}

// SubscribeAll subscribes to all events.
func (s *InMemoryEventStore) SubscribeAll(handler domain.EventHandler) domain.Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	s.subscriptions[id] = &eventSubscription{
		id:      id,
		handler: handler,
		all:     true,
	}

	return newSubscription(id, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.subscriptions, id)
	})
}

// notifySubscribersLocked notifies subscribers (must be called with lock held).
func (s *InMemoryEventStore) notifySubscribersLocked(event *domain.StoredEvent) {
	for _, sub := range s.subscriptions {
		if sub.all {
			go func(h domain.EventHandler) {
				h(event)
			}(sub.handler)
			continue
		}

		for _, t := range sub.eventTypes {
			if t == event.Type {
				go func(h domain.EventHandler) {
					h(event)
				}(sub.handler)
				break
			}
		}
	}
}

// Count returns the total number of events.
func (s *InMemoryEventStore) Count(ctx context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, domain.ErrStoreClosed
	}

	return int64(len(s.events)), nil
}

// CountByAggregate returns the number of events for an aggregate.
func (s *InMemoryEventStore) CountByAggregate(ctx context.Context, aggregateID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, domain.ErrStoreClosed
	}

	return int64(len(s.byAggregate[aggregateID])), nil
}

// Close closes the event store.
func (s *InMemoryEventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

// Clear clears all events (for testing).
func (s *InMemoryEventStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = make([]*domain.StoredEvent, 0)
	s.byAggregate = make(map[string][]*domain.StoredEvent)
	s.byType = make(map[string][]*domain.StoredEvent)
	s.versions = make(map[string]int)
}

// GetAllEvents returns all events (for testing/debugging).
func (s *InMemoryEventStore) GetAllEvents() ([]*domain.StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	result := make([]*domain.StoredEvent, len(s.events))
	copy(result, s.events)
	return result, nil
}
