package eventsourcing

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/anthropics/claude-flow-go/internal/domain/eventsourcing"
	infra "github.com/anthropics/claude-flow-go/internal/infrastructure/eventsourcing"
)

type repositoryTestAggregate struct {
	id               string
	version          int
	appliedVersions  []int
	fromSnapshotCall bool
	setVersionCall   bool
	setVersionArg    int
}

type repositoryNoSetterAggregate struct {
	id               string
	version          int
	appliedVersions  []int
	fromSnapshotCall bool
}

type repositorySnapshotFailAggregate struct {
	id                string
	version           int
	appliedVersions   []int
	fromSnapshotCalls *int
	setVersionCall    bool
}

type repositorySetterOnlyAggregate struct {
	id             string
	version        int
	appliedVersions []int
	setVersionCall bool
}

type nilSnapshotStore struct{}

func (s *nilSnapshotStore) Save(ctx context.Context, snapshot *domain.Snapshot) error { return nil }

func (s *nilSnapshotStore) Load(ctx context.Context, aggregateID string) (*domain.Snapshot, error) {
	return nil, nil
}

func (s *nilSnapshotStore) Delete(ctx context.Context, aggregateID string) error { return nil }

func (s *nilSnapshotStore) DeleteOlderThan(ctx context.Context, aggregateID string, version int) error {
	return nil
}

func (s *nilSnapshotStore) Close() error { return nil }

func (a *repositoryTestAggregate) ID() string { return a.id }

func (a *repositoryTestAggregate) Type() string { return "repository-test" }

func (a *repositoryTestAggregate) Version() int { return a.version }

func (a *repositoryTestAggregate) ApplyEvent(event domain.Event) error {
	a.appliedVersions = append(a.appliedVersions, event.Version())
	a.version = event.Version()
	return nil
}

func (a *repositoryTestAggregate) UncommittedEvents() []domain.Event { return nil }

func (a *repositoryTestAggregate) ClearUncommittedEvents() {}

func (a *repositoryTestAggregate) ToSnapshot() ([]byte, error) { return []byte(`{"ok":true}`), nil }

func (a *repositoryTestAggregate) FromSnapshot(data []byte) error {
	a.fromSnapshotCall = true
	return nil
}

func (a *repositoryTestAggregate) SetVersion(version int) {
	a.setVersionCall = true
	a.setVersionArg = version
	a.version = version
}

func (a *repositoryNoSetterAggregate) ID() string { return a.id }

func (a *repositoryNoSetterAggregate) Type() string { return "repository-test" }

func (a *repositoryNoSetterAggregate) Version() int { return a.version }

func (a *repositoryNoSetterAggregate) ApplyEvent(event domain.Event) error {
	a.appliedVersions = append(a.appliedVersions, event.Version())
	a.version = event.Version()
	return nil
}

func (a *repositoryNoSetterAggregate) UncommittedEvents() []domain.Event { return nil }

func (a *repositoryNoSetterAggregate) ClearUncommittedEvents() {}

func (a *repositoryNoSetterAggregate) ToSnapshot() ([]byte, error) { return []byte(`{"ok":true}`), nil }

func (a *repositoryNoSetterAggregate) FromSnapshot(data []byte) error {
	a.fromSnapshotCall = true
	return nil
}

func (a *repositorySnapshotFailAggregate) ID() string { return a.id }

func (a *repositorySnapshotFailAggregate) Type() string { return "repository-test" }

func (a *repositorySnapshotFailAggregate) Version() int { return a.version }

func (a *repositorySnapshotFailAggregate) ApplyEvent(event domain.Event) error {
	a.appliedVersions = append(a.appliedVersions, event.Version())
	a.version = event.Version()
	return nil
}

func (a *repositorySnapshotFailAggregate) UncommittedEvents() []domain.Event { return nil }

func (a *repositorySnapshotFailAggregate) ClearUncommittedEvents() {}

func (a *repositorySnapshotFailAggregate) ToSnapshot() ([]byte, error) { return []byte(`{"ok":true}`), nil }

func (a *repositorySnapshotFailAggregate) FromSnapshot(data []byte) error {
	if a.fromSnapshotCalls != nil {
		(*a.fromSnapshotCalls)++
	}
	return errors.New("snapshot decode failed")
}

func (a *repositorySnapshotFailAggregate) SetVersion(version int) {
	a.setVersionCall = true
	a.version = version
}

func (a *repositorySetterOnlyAggregate) ID() string { return a.id }

func (a *repositorySetterOnlyAggregate) Type() string { return "repository-test" }

func (a *repositorySetterOnlyAggregate) Version() int { return a.version }

func (a *repositorySetterOnlyAggregate) ApplyEvent(event domain.Event) error {
	a.appliedVersions = append(a.appliedVersions, event.Version())
	a.version = event.Version()
	return nil
}

func (a *repositorySetterOnlyAggregate) UncommittedEvents() []domain.Event { return nil }

func (a *repositorySetterOnlyAggregate) ClearUncommittedEvents() {}

func (a *repositorySetterOnlyAggregate) SetVersion(version int) {
	a.setVersionCall = true
	a.version = version
}

func TestAggregateRepository_Load_UsesSetVersionCapability(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-1"

	eventStore := infra.NewInMemoryEventStore()
	snapshotStore := infra.NewInMemorySnapshotStore()
	serializer := infra.NewJSONEventSerializer()

	for version := 1; version <= 6; version++ {
		event := domain.NewBaseEvent("repo:event", aggregateID, "repository-test", version, map[string]interface{}{"version": version})
		stored, err := serializer.Serialize(event)
		if err != nil {
			t.Fatalf("failed to serialize event version %d: %v", version, err)
		}
		if err := eventStore.Append(ctx, []*domain.StoredEvent{stored}); err != nil {
			t.Fatalf("failed to append event version %d: %v", version, err)
		}
	}

	if err := snapshotStore.Save(ctx, &domain.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: "repository-test",
		Version:       5,
		State:         []byte(`{"restored":true}`),
		CreatedAt:     time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: snapshotStore,
		Serializer:    serializer,
		Factory: func(id string) domain.Aggregate {
			return &repositoryTestAggregate{id: id}
		},
	})

	loaded, err := repo.Load(ctx, aggregateID)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	agg, ok := loaded.(*repositoryTestAggregate)
	if !ok {
		t.Fatalf("expected *repositoryTestAggregate, got %T", loaded)
	}

	if !agg.fromSnapshotCall {
		t.Fatal("expected aggregate snapshot restore to be called")
	}

	if !agg.setVersionCall {
		t.Fatal("expected SetVersion capability to be called")
	}

	if agg.setVersionArg != 5 {
		t.Fatalf("expected SetVersion(5), got SetVersion(%d)", agg.setVersionArg)
	}

	if len(agg.appliedVersions) != 1 || agg.appliedVersions[0] != 6 {
		t.Fatalf("expected only post-snapshot event version 6 to be applied, got %v", agg.appliedVersions)
	}

	if agg.Version() != 6 {
		t.Fatalf("expected final aggregate version 6, got %d", agg.Version())
	}
}

func TestAggregateRepository_Load_WithoutSetVersionCapability(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-2"

	eventStore := infra.NewInMemoryEventStore()
	snapshotStore := infra.NewInMemorySnapshotStore()
	serializer := infra.NewJSONEventSerializer()

	for version := 1; version <= 6; version++ {
		event := domain.NewBaseEvent("repo:event", aggregateID, "repository-test", version, map[string]interface{}{"version": version})
		stored, err := serializer.Serialize(event)
		if err != nil {
			t.Fatalf("failed to serialize event version %d: %v", version, err)
		}
		if err := eventStore.Append(ctx, []*domain.StoredEvent{stored}); err != nil {
			t.Fatalf("failed to append event version %d: %v", version, err)
		}
	}

	if err := snapshotStore.Save(ctx, &domain.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: "repository-test",
		Version:       5,
		State:         []byte(`{"restored":true}`),
		CreatedAt:     time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: snapshotStore,
		Serializer:    serializer,
		Factory: func(id string) domain.Aggregate {
			return &repositoryNoSetterAggregate{id: id}
		},
	})

	loaded, err := repo.Load(ctx, aggregateID)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	agg, ok := loaded.(*repositoryNoSetterAggregate)
	if !ok {
		t.Fatalf("expected *repositoryNoSetterAggregate, got %T", loaded)
	}

	if !agg.fromSnapshotCall {
		t.Fatal("expected aggregate snapshot restore to be called")
	}

	if len(agg.appliedVersions) != 1 || agg.appliedVersions[0] != 6 {
		t.Fatalf("expected only post-snapshot event version 6 to be applied, got %v", agg.appliedVersions)
	}

	if agg.Version() != 6 {
		t.Fatalf("expected final aggregate version 6, got %d", agg.Version())
	}
}

func TestAggregateRepository_Load_SnapshotRestoreFailureFallsBackToFullReplay(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-3"

	eventStore := infra.NewInMemoryEventStore()
	snapshotStore := infra.NewInMemorySnapshotStore()
	serializer := infra.NewJSONEventSerializer()

	for version := 1; version <= 3; version++ {
		event := domain.NewBaseEvent("repo:event", aggregateID, "repository-test", version, map[string]interface{}{"version": version})
		stored, err := serializer.Serialize(event)
		if err != nil {
			t.Fatalf("failed to serialize event version %d: %v", version, err)
		}
		if err := eventStore.Append(ctx, []*domain.StoredEvent{stored}); err != nil {
			t.Fatalf("failed to append event version %d: %v", version, err)
		}
	}

	if err := snapshotStore.Save(ctx, &domain.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: "repository-test",
		Version:       2,
		State:         []byte(`{"restored":true}`),
		CreatedAt:     time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	snapshotCalls := 0
	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: snapshotStore,
		Serializer:    serializer,
		Factory: func(id string) domain.Aggregate {
			return &repositorySnapshotFailAggregate{
				id:                id,
				fromSnapshotCalls: &snapshotCalls,
			}
		},
	})

	loaded, err := repo.Load(ctx, aggregateID)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	agg, ok := loaded.(*repositorySnapshotFailAggregate)
	if !ok {
		t.Fatalf("expected *repositorySnapshotFailAggregate, got %T", loaded)
	}

	if snapshotCalls != 1 {
		t.Fatalf("expected one snapshot restoration attempt, got %d", snapshotCalls)
	}

	if agg.setVersionCall {
		t.Fatal("setVersion should not be called when snapshot restoration fails")
	}

	if len(agg.appliedVersions) != 3 || agg.appliedVersions[0] != 1 || agg.appliedVersions[1] != 2 || agg.appliedVersions[2] != 3 {
		t.Fatalf("expected full replay of versions [1 2 3], got %v", agg.appliedVersions)
	}

	if agg.Version() != 3 {
		t.Fatalf("expected final aggregate version 3, got %d", agg.Version())
	}
}

func TestAggregateRepository_Load_ExpiredSnapshotIsIgnored(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-expired"

	eventStore := infra.NewInMemoryEventStore()
	snapshotStore := infra.NewInMemorySnapshotStore()
	serializer := infra.NewJSONEventSerializer()

	for version := 1; version <= 3; version++ {
		event := domain.NewBaseEvent("repo:event", aggregateID, "repository-test", version, map[string]interface{}{"version": version})
		stored, err := serializer.Serialize(event)
		if err != nil {
			t.Fatalf("failed to serialize event version %d: %v", version, err)
		}
		if err := eventStore.Append(ctx, []*domain.StoredEvent{stored}); err != nil {
			t.Fatalf("failed to append event version %d: %v", version, err)
		}
	}

	// Snapshot is intentionally old so SnapshotConfig marks it as expired.
	if err := snapshotStore.Save(ctx, &domain.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: "repository-test",
		Version:       2,
		State:         []byte(`{"restored":true}`),
		CreatedAt:     time.Now().Add(-time.Hour).UnixMilli(),
	}); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: snapshotStore,
		Serializer:    serializer,
		SnapshotConfig: infra.SnapshotConfig{
			MaxAgeMs: 1,
		},
		Factory: func(id string) domain.Aggregate {
			return &repositoryTestAggregate{id: id}
		},
	})

	loaded, err := repo.Load(ctx, aggregateID)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	agg, ok := loaded.(*repositoryTestAggregate)
	if !ok {
		t.Fatalf("expected *repositoryTestAggregate, got %T", loaded)
	}

	if agg.fromSnapshotCall {
		t.Fatal("expired snapshot should not be restored")
	}

	if agg.setVersionCall {
		t.Fatal("setVersion should not be called for expired snapshot")
	}

	if len(agg.appliedVersions) != 3 || agg.appliedVersions[0] != 1 || agg.appliedVersions[1] != 2 || agg.appliedVersions[2] != 3 {
		t.Fatalf("expected full replay of versions [1 2 3], got %v", agg.appliedVersions)
	}

	if agg.Version() != 3 {
		t.Fatalf("expected final aggregate version 3, got %d", agg.Version())
	}
}

func TestAggregateRepository_Load_NonStatefulAggregateIgnoresSnapshot(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-non-stateful"

	eventStore := infra.NewInMemoryEventStore()
	snapshotStore := infra.NewInMemorySnapshotStore()
	serializer := infra.NewJSONEventSerializer()

	for version := 1; version <= 3; version++ {
		event := domain.NewBaseEvent("repo:event", aggregateID, "repository-test", version, map[string]interface{}{"version": version})
		stored, err := serializer.Serialize(event)
		if err != nil {
			t.Fatalf("failed to serialize event version %d: %v", version, err)
		}
		if err := eventStore.Append(ctx, []*domain.StoredEvent{stored}); err != nil {
			t.Fatalf("failed to append event version %d: %v", version, err)
		}
	}

	if err := snapshotStore.Save(ctx, &domain.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: "repository-test",
		Version:       2,
		State:         []byte(`{"restored":true}`),
		CreatedAt:     time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: snapshotStore,
		Serializer:    serializer,
		Factory: func(id string) domain.Aggregate {
			return &repositorySetterOnlyAggregate{id: id}
		},
	})

	loaded, err := repo.Load(ctx, aggregateID)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	agg, ok := loaded.(*repositorySetterOnlyAggregate)
	if !ok {
		t.Fatalf("expected *repositorySetterOnlyAggregate, got %T", loaded)
	}

	if agg.setVersionCall {
		t.Fatal("setVersion should not be called when aggregate is not AggregateState")
	}

	if len(agg.appliedVersions) != 3 || agg.appliedVersions[0] != 1 || agg.appliedVersions[1] != 2 || agg.appliedVersions[2] != 3 {
		t.Fatalf("expected full replay of versions [1 2 3], got %v", agg.appliedVersions)
	}

	if agg.Version() != 3 {
		t.Fatalf("expected final aggregate version 3, got %d", agg.Version())
	}
}

func TestAggregateRepository_Load_SnapshotOnlyAggregate(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-snapshot-only"

	eventStore := infra.NewInMemoryEventStore()
	snapshotStore := infra.NewInMemorySnapshotStore()
	serializer := infra.NewJSONEventSerializer()

	if err := snapshotStore.Save(ctx, &domain.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: "repository-test",
		Version:       3,
		State:         []byte(`{"restored":true}`),
		CreatedAt:     time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: snapshotStore,
		Serializer:    serializer,
		Factory: func(id string) domain.Aggregate {
			return &repositoryTestAggregate{id: id}
		},
	})

	loaded, err := repo.Load(ctx, aggregateID)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	agg, ok := loaded.(*repositoryTestAggregate)
	if !ok {
		t.Fatalf("expected *repositoryTestAggregate, got %T", loaded)
	}

	if !agg.fromSnapshotCall {
		t.Fatal("expected aggregate snapshot restore to be called")
	}

	if !agg.setVersionCall {
		t.Fatal("expected setVersion from snapshot to be called")
	}

	if len(agg.appliedVersions) != 0 {
		t.Fatalf("expected no event replay after snapshot-only load, got %v", agg.appliedVersions)
	}

	if agg.Version() != 3 {
		t.Fatalf("expected final aggregate version 3, got %d", agg.Version())
	}
}

func TestAggregateRepository_Load_SnapshotRestoreFailureWithoutEventsReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-snapshot-fail-no-events"

	eventStore := infra.NewInMemoryEventStore()
	snapshotStore := infra.NewInMemorySnapshotStore()
	serializer := infra.NewJSONEventSerializer()

	if err := snapshotStore.Save(ctx, &domain.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: "repository-test",
		Version:       2,
		State:         []byte(`{"restored":true}`),
		CreatedAt:     time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: snapshotStore,
		Serializer:    serializer,
		Factory: func(id string) domain.Aggregate {
			return &repositorySnapshotFailAggregate{id: id}
		},
	})

	_, err := repo.Load(ctx, aggregateID)
	if !errors.Is(err, domain.ErrAggregateNotFound) {
		t.Fatalf("expected ErrAggregateNotFound after snapshot restore failure with no events, got %v", err)
	}
}

func TestAggregateRepository_Load_NonStatefulSnapshotWithoutEventsReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-non-stateful-no-events"

	eventStore := infra.NewInMemoryEventStore()
	snapshotStore := infra.NewInMemorySnapshotStore()
	serializer := infra.NewJSONEventSerializer()

	if err := snapshotStore.Save(ctx, &domain.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: "repository-test",
		Version:       2,
		State:         []byte(`{"restored":true}`),
		CreatedAt:     time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: snapshotStore,
		Serializer:    serializer,
		Factory: func(id string) domain.Aggregate {
			return &repositorySetterOnlyAggregate{id: id}
		},
	})

	_, err := repo.Load(ctx, aggregateID)
	if !errors.Is(err, domain.ErrAggregateNotFound) {
		t.Fatalf("expected ErrAggregateNotFound for non-stateful aggregate with snapshot-only history, got %v", err)
	}
}

func TestAggregateRepository_Load_NilSnapshotValueIsIgnored(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-nil-snapshot-value"

	eventStore := infra.NewInMemoryEventStore()
	serializer := infra.NewJSONEventSerializer()

	event := domain.NewBaseEvent("repo:event", aggregateID, "repository-test", 1, map[string]interface{}{"version": 1})
	stored, err := serializer.Serialize(event)
	if err != nil {
		t.Fatalf("failed to serialize event: %v", err)
	}
	if err := eventStore.Append(ctx, []*domain.StoredEvent{stored}); err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: &nilSnapshotStore{},
		Serializer:    serializer,
		Factory: func(id string) domain.Aggregate {
			return &repositoryTestAggregate{id: id}
		},
	})

	loaded, err := repo.Load(ctx, aggregateID)
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	agg, ok := loaded.(*repositoryTestAggregate)
	if !ok {
		t.Fatalf("expected *repositoryTestAggregate, got %T", loaded)
	}

	if agg.fromSnapshotCall {
		t.Fatal("snapshot restore should not run for nil snapshot value")
	}

	if len(agg.appliedVersions) != 1 || agg.appliedVersions[0] != 1 {
		t.Fatalf("expected replay of version [1], got %v", agg.appliedVersions)
	}
}

func TestAggregateRepository_Load_NilSnapshotAndNoEventsReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	aggregateID := "aggregate-nil-snapshot-no-events"

	eventStore := infra.NewInMemoryEventStore()
	serializer := infra.NewJSONEventSerializer()

	repo := NewAggregateRepository(RepositoryConfig{
		EventStore:    eventStore,
		SnapshotStore: &nilSnapshotStore{},
		Serializer:    serializer,
		Factory: func(id string) domain.Aggregate {
			return &repositoryTestAggregate{id: id}
		},
	})

	_, err := repo.Load(ctx, aggregateID)
	if !errors.Is(err, domain.ErrAggregateNotFound) {
		t.Fatalf("expected ErrAggregateNotFound for nil snapshot with no events, got %v", err)
	}
}
