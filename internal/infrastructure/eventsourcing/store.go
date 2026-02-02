// Package eventsourcing provides infrastructure for event sourcing.
package eventsourcing

import (
	"context"

	domain "github.com/anthropics/claude-flow-go/internal/domain/eventsourcing"
)

// EventStore defines the interface for event persistence.
type EventStore interface {
	// Append appends events to the store atomically.
	Append(ctx context.Context, events []*domain.StoredEvent) error

	// Load loads events for an aggregate from a given version.
	Load(ctx context.Context, aggregateID string, fromVersion int) ([]*domain.StoredEvent, error)

	// LoadByType loads events by event type.
	LoadByType(ctx context.Context, eventType string) ([]*domain.StoredEvent, error)

	// Query queries events matching the given criteria.
	Query(ctx context.Context, query *domain.EventQuery) ([]*domain.StoredEvent, error)

	// GetVersion returns the current version of an aggregate.
	GetVersion(ctx context.Context, aggregateID string) (int, error)

	// Subscribe subscribes to events of specific types.
	Subscribe(eventTypes []string, handler domain.EventHandler) domain.Subscription

	// SubscribeAll subscribes to all events.
	SubscribeAll(handler domain.EventHandler) domain.Subscription

	// Count returns the total number of events.
	Count(ctx context.Context) (int64, error)

	// CountByAggregate returns the number of events for an aggregate.
	CountByAggregate(ctx context.Context, aggregateID string) (int64, error)

	// Close closes the event store.
	Close() error
}

// SnapshotStore defines the interface for snapshot persistence.
type SnapshotStore interface {
	// Save saves a snapshot.
	Save(ctx context.Context, snapshot *domain.Snapshot) error

	// Load loads the latest snapshot for an aggregate.
	Load(ctx context.Context, aggregateID string) (*domain.Snapshot, error)

	// Delete deletes snapshots for an aggregate.
	Delete(ctx context.Context, aggregateID string) error

	// DeleteOlderThan deletes snapshots older than a given version.
	DeleteOlderThan(ctx context.Context, aggregateID string, version int) error

	// Close closes the snapshot store.
	Close() error
}

// EventStoreConfig configures the event store.
type EventStoreConfig struct {
	// DatabasePath is the path to the SQLite database file.
	DatabasePath string `json:"databasePath,omitempty"`

	// InMemory uses in-memory storage.
	InMemory bool `json:"inMemory,omitempty"`

	// MaxBatchSize limits the batch size for appends.
	MaxBatchSize int `json:"maxBatchSize,omitempty"`

	// EnableSnapshots enables snapshot support.
	EnableSnapshots bool `json:"enableSnapshots,omitempty"`

	// SnapshotFrequency is the number of events between snapshots.
	SnapshotFrequency int `json:"snapshotFrequency,omitempty"`
}

// DefaultEventStoreConfig returns the default configuration.
func DefaultEventStoreConfig() EventStoreConfig {
	return EventStoreConfig{
		DatabasePath:      ".data/events.db",
		InMemory:          false,
		MaxBatchSize:      1000,
		EnableSnapshots:   true,
		SnapshotFrequency: 100,
	}
}

// subscription implements the Subscription interface.
type subscription struct {
	id          string
	unsubscribe func()
}

// ID returns the subscription ID.
func (s *subscription) ID() string {
	return s.id
}

// Unsubscribe cancels the subscription.
func (s *subscription) Unsubscribe() {
	if s.unsubscribe != nil {
		s.unsubscribe()
	}
}

// newSubscription creates a new subscription.
func newSubscription(id string, unsub func()) *subscription {
	return &subscription{
		id:          id,
		unsubscribe: unsub,
	}
}
