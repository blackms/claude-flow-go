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
