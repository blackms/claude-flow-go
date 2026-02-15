// Package eventsourcing provides application services for event sourcing.
package eventsourcing

import (
	"context"
	"fmt"
	"sync"
	"time"

	domain "github.com/anthropics/claude-flow-go/internal/domain/eventsourcing"
	infra "github.com/anthropics/claude-flow-go/internal/infrastructure/eventsourcing"
)

type aggregateVersionSetter interface {
	SetVersion(version int)
}

// AggregateRepository provides storage and retrieval of event-sourced aggregates.
type AggregateRepository struct {
	mu             sync.RWMutex
	eventStore     infra.EventStore
	snapshotStore  infra.SnapshotStore
	serializer     infra.EventSerializer
	snapshotConfig infra.SnapshotConfig
	factory        domain.AggregateFactory
}

// RepositoryConfig configures the aggregate repository.
type RepositoryConfig struct {
	// EventStore is the event store to use.
	EventStore infra.EventStore

	// SnapshotStore is the snapshot store to use (optional).
	SnapshotStore infra.SnapshotStore

	// Serializer is the event serializer (defaults to JSON).
	Serializer infra.EventSerializer

	// SnapshotConfig configures snapshotting.
	SnapshotConfig infra.SnapshotConfig

	// Factory creates new aggregate instances.
	Factory domain.AggregateFactory
}

// NewAggregateRepository creates a new aggregate repository.
func NewAggregateRepository(config RepositoryConfig) *AggregateRepository {
	serializer := config.Serializer
	if serializer == nil {
		serializer = infra.NewJSONEventSerializer()
	}

	return &AggregateRepository{
		eventStore:     config.EventStore,
		snapshotStore:  config.SnapshotStore,
		serializer:     serializer,
		snapshotConfig: config.SnapshotConfig,
		factory:        config.Factory,
	}
}

// Save persists uncommitted events from an aggregate.
func (r *AggregateRepository) Save(ctx context.Context, aggregate domain.Aggregate) error {
	events := aggregate.UncommittedEvents()
	if len(events) == 0 {
		return nil
	}

	// Serialize events
	storedEvents := make([]*domain.StoredEvent, len(events))
	for i, event := range events {
		stored, err := r.serializer.Serialize(event)
		if err != nil {
			return fmt.Errorf("failed to serialize event: %w", err)
		}
		storedEvents[i] = stored
	}

	// Append to store
	if err := r.eventStore.Append(ctx, storedEvents); err != nil {
		return fmt.Errorf("failed to append events: %w", err)
	}

	// Clear uncommitted events
	aggregate.ClearUncommittedEvents()

	// Check if snapshot is needed
	if r.snapshotStore != nil && r.snapshotConfig.Enabled {
		go r.maybeCreateSnapshot(ctx, aggregate)
	}

	return nil
}

// Load loads an aggregate from its event history.
func (r *AggregateRepository) Load(ctx context.Context, id string) (domain.Aggregate, error) {
	if r.factory == nil {
		return nil, fmt.Errorf("aggregate factory not configured")
	}

	aggregate := r.factory(id)

	// Try to load from snapshot first
	var fromVersion int
	if r.snapshotStore != nil {
		snapshot, err := r.snapshotStore.Load(ctx, id)
		if err == nil && !r.snapshotConfig.IsExpired(snapshot) {
			// Restore from snapshot
			if stateful, ok := aggregate.(domain.AggregateState); ok {
				if err := stateful.FromSnapshot(snapshot.State); err != nil {
					// Snapshot restoration failed, start from scratch
					aggregate = r.factory(id)
				} else {
					fromVersion = snapshot.Version + 1
					if setter, ok := aggregate.(aggregateVersionSetter); ok {
						setter.SetVersion(snapshot.Version)
					}
				}
			}
		}
	}

	// Load events from store
	storedEvents, err := r.eventStore.Load(ctx, id, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}

	if len(storedEvents) == 0 && fromVersion == 0 {
		return nil, domain.ErrAggregateNotFound
	}

	// Apply events
	for _, stored := range storedEvents {
		event, err := r.serializer.Deserialize(stored)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize event: %w", err)
		}
		if err := aggregate.ApplyEvent(event); err != nil {
			return nil, fmt.Errorf("failed to apply event: %w", err)
		}
	}

	return aggregate, nil
}

// Exists checks if an aggregate exists.
func (r *AggregateRepository) Exists(ctx context.Context, id string) (bool, error) {
	version, err := r.eventStore.GetVersion(ctx, id)
	if err != nil {
		return false, err
	}
	return version > 0, nil
}

// GetVersion returns the current version of an aggregate.
func (r *AggregateRepository) GetVersion(ctx context.Context, id string) (int, error) {
	return r.eventStore.GetVersion(ctx, id)
}

// maybeCreateSnapshot creates a snapshot if needed.
func (r *AggregateRepository) maybeCreateSnapshot(ctx context.Context, aggregate domain.Aggregate) {
	stateful, ok := aggregate.(domain.AggregateState)
	if !ok {
		return
	}

	// Get current snapshot version
	lastSnapshotVersion := 0
	if existing, err := r.snapshotStore.Load(ctx, aggregate.ID()); err == nil {
		lastSnapshotVersion = existing.Version
	}

	if !r.snapshotConfig.ShouldSnapshot(aggregate.Version(), lastSnapshotVersion) {
		return
	}

	// Create snapshot
	state, err := stateful.ToSnapshot()
	if err != nil {
		return
	}

	snapshot := &domain.Snapshot{
		AggregateID:   aggregate.ID(),
		AggregateType: aggregate.Type(),
		Version:       aggregate.Version(),
		State:         state,
		CreatedAt:     time.Now().UnixMilli(),
	}

	r.snapshotStore.Save(ctx, snapshot)
}

// Delete deletes an aggregate's snapshots (events are immutable).
func (r *AggregateRepository) Delete(ctx context.Context, id string) error {
	if r.snapshotStore != nil {
		return r.snapshotStore.Delete(ctx, id)
	}
	return nil
}

// GetEvents returns the raw events for an aggregate.
func (r *AggregateRepository) GetEvents(ctx context.Context, id string, fromVersion int) ([]*domain.StoredEvent, error) {
	return r.eventStore.Load(ctx, id, fromVersion)
}

// GenericRepository provides a type-safe wrapper for AggregateRepository.
type GenericRepository[T domain.Aggregate] struct {
	repo    *AggregateRepository
	factory func(id string) T
}

// NewGenericRepository creates a new generic repository.
func NewGenericRepository[T domain.Aggregate](eventStore infra.EventStore, factory func(id string) T) *GenericRepository[T] {
	return &GenericRepository[T]{
		repo: NewAggregateRepository(RepositoryConfig{
			EventStore: eventStore,
			Factory: func(id string) domain.Aggregate {
				return factory(id)
			},
		}),
		factory: factory,
	}
}

// Save saves an aggregate.
func (r *GenericRepository[T]) Save(ctx context.Context, aggregate T) error {
	return r.repo.Save(ctx, aggregate)
}

// Load loads an aggregate.
func (r *GenericRepository[T]) Load(ctx context.Context, id string) (T, error) {
	var zero T
	agg, err := r.repo.Load(ctx, id)
	if err != nil {
		return zero, err
	}
	if typed, ok := agg.(T); ok {
		return typed, nil
	}
	return zero, fmt.Errorf("type assertion failed")
}

// Exists checks if an aggregate exists.
func (r *GenericRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	return r.repo.Exists(ctx, id)
}

// UnitOfWork provides transactional semantics for multiple aggregates.
type UnitOfWork struct {
	mu         sync.Mutex
	repository *AggregateRepository
	pending    []domain.Aggregate
}

// NewUnitOfWork creates a new unit of work.
func NewUnitOfWork(repository *AggregateRepository) *UnitOfWork {
	return &UnitOfWork{
		repository: repository,
		pending:    make([]domain.Aggregate, 0),
	}
}

// Register registers an aggregate for tracking.
func (u *UnitOfWork) Register(aggregate domain.Aggregate) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.pending = append(u.pending, aggregate)
}

// Commit saves all pending aggregates.
func (u *UnitOfWork) Commit(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	for _, aggregate := range u.pending {
		if err := u.repository.Save(ctx, aggregate); err != nil {
			return err
		}
	}

	u.pending = make([]domain.Aggregate, 0)
	return nil
}

// Rollback discards pending changes.
func (u *UnitOfWork) Rollback() {
	u.mu.Lock()
	defer u.mu.Unlock()

	// Clear uncommitted events from all pending aggregates
	for _, aggregate := range u.pending {
		aggregate.ClearUncommittedEvents()
	}

	u.pending = make([]domain.Aggregate, 0)
}
