package eventsourcing

import (
	"errors"
	"testing"
)

type snapshotTestAggregate struct {
	id               string
	aggregateType    string
	version          int
	appliedVersions  []int
	fromSnapshotCall bool
	setVersionCall   bool
}

type snapshotNoSetterAggregate struct {
	id               string
	aggregateType    string
	version          int
	appliedVersions  []int
	fromSnapshotCall bool
}

type snapshotFailAggregate struct {
	id               string
	aggregateType    string
	version          int
	fromSnapshotCall bool
	setVersionCall   bool
}

func (a *snapshotTestAggregate) ID() string { return a.id }

func (a *snapshotTestAggregate) Type() string { return a.aggregateType }

func (a *snapshotTestAggregate) Version() int { return a.version }

func (a *snapshotTestAggregate) ApplyEvent(event Event) error {
	a.appliedVersions = append(a.appliedVersions, event.Version())
	a.version = event.Version()
	return nil
}

func (a *snapshotTestAggregate) UncommittedEvents() []Event { return nil }

func (a *snapshotTestAggregate) ClearUncommittedEvents() {}

func (a *snapshotTestAggregate) ToSnapshot() ([]byte, error) { return []byte(`{"ok":true}`), nil }

func (a *snapshotTestAggregate) FromSnapshot(data []byte) error {
	a.fromSnapshotCall = true
	return nil
}

func (a *snapshotTestAggregate) SetVersion(version int) {
	a.setVersionCall = true
	a.version = version
}

func (a *snapshotNoSetterAggregate) ID() string { return a.id }

func (a *snapshotNoSetterAggregate) Type() string { return a.aggregateType }

func (a *snapshotNoSetterAggregate) Version() int { return a.version }

func (a *snapshotNoSetterAggregate) ApplyEvent(event Event) error {
	a.appliedVersions = append(a.appliedVersions, event.Version())
	a.version = event.Version()
	return nil
}

func (a *snapshotNoSetterAggregate) UncommittedEvents() []Event { return nil }

func (a *snapshotNoSetterAggregate) ClearUncommittedEvents() {}

func (a *snapshotNoSetterAggregate) ToSnapshot() ([]byte, error) { return []byte(`{"ok":true}`), nil }

func (a *snapshotNoSetterAggregate) FromSnapshot(data []byte) error {
	a.fromSnapshotCall = true
	return nil
}

func (a *snapshotFailAggregate) ID() string { return a.id }

func (a *snapshotFailAggregate) Type() string { return a.aggregateType }

func (a *snapshotFailAggregate) Version() int { return a.version }

func (a *snapshotFailAggregate) ApplyEvent(event Event) error {
	a.version = event.Version()
	return nil
}

func (a *snapshotFailAggregate) UncommittedEvents() []Event { return nil }

func (a *snapshotFailAggregate) ClearUncommittedEvents() {}

func (a *snapshotFailAggregate) ToSnapshot() ([]byte, error) { return []byte(`{"ok":true}`), nil }

func (a *snapshotFailAggregate) FromSnapshot(data []byte) error {
	a.fromSnapshotCall = true
	return errors.New("snapshot decode failed")
}

func (a *snapshotFailAggregate) SetVersion(version int) {
	a.setVersionCall = true
	a.version = version
}

func TestRebuildFromSnapshot_UsesSetVersionCapability(t *testing.T) {
	agg := &snapshotTestAggregate{
		id:            "agg-1",
		aggregateType: "test-aggregate",
	}

	snapshot := &Snapshot{
		AggregateID:   "agg-1",
		AggregateType: "test-aggregate",
		Version:       5,
		State:         []byte(`{"state":"snapshot"}`),
	}

	events := []Event{
		NewBaseEvent("evt-1", "agg-1", "test-aggregate", 6, map[string]interface{}{"k": "v"}),
		NewBaseEvent("evt-2", "agg-1", "test-aggregate", 7, map[string]interface{}{"k": "v2"}),
	}

	if err := RebuildFromSnapshot(agg, snapshot, events); err != nil {
		t.Fatalf("unexpected error rebuilding from snapshot: %v", err)
	}

	if !agg.fromSnapshotCall {
		t.Fatal("expected snapshot restoration to be called")
	}

	if !agg.setVersionCall {
		t.Fatal("expected SetVersion capability to be used")
	}

	if len(agg.appliedVersions) != 2 {
		t.Fatalf("expected 2 events to be applied, got %d", len(agg.appliedVersions))
	}

	if agg.appliedVersions[0] != 6 || agg.appliedVersions[1] != 7 {
		t.Fatalf("unexpected applied versions: %v", agg.appliedVersions)
	}

	if agg.Version() != 7 {
		t.Fatalf("expected final version 7, got %d", agg.Version())
	}
}

func TestRebuildFromSnapshot_WithoutSetVersionCapability(t *testing.T) {
	agg := &snapshotNoSetterAggregate{
		id:            "agg-2",
		aggregateType: "test-aggregate",
	}

	snapshot := &Snapshot{
		AggregateID:   "agg-2",
		AggregateType: "test-aggregate",
		Version:       5,
		State:         []byte(`{"state":"snapshot"}`),
	}

	events := []Event{
		NewBaseEvent("evt-1", "agg-2", "test-aggregate", 6, map[string]interface{}{"k": "v"}),
	}

	if err := RebuildFromSnapshot(agg, snapshot, events); err != nil {
		t.Fatalf("unexpected error rebuilding from snapshot: %v", err)
	}

	if !agg.fromSnapshotCall {
		t.Fatal("expected snapshot restoration to be called")
	}

	if len(agg.appliedVersions) != 1 || agg.appliedVersions[0] != 6 {
		t.Fatalf("expected event version 6 to be applied, got %v", agg.appliedVersions)
	}

	if agg.Version() != 6 {
		t.Fatalf("expected final version 6, got %d", agg.Version())
	}
}

func TestRebuildFromSnapshot_FromSnapshotError(t *testing.T) {
	agg := &snapshotFailAggregate{
		id:            "agg-3",
		aggregateType: "test-aggregate",
	}

	snapshot := &Snapshot{
		AggregateID:   "agg-3",
		AggregateType: "test-aggregate",
		Version:       5,
		State:         []byte(`{"state":"snapshot"}`),
	}

	events := []Event{
		NewBaseEvent("evt-1", "agg-3", "test-aggregate", 6, map[string]interface{}{"k": "v"}),
	}

	err := RebuildFromSnapshot(agg, snapshot, events)
	if err == nil {
		t.Fatal("expected snapshot restoration error")
	}

	if !agg.fromSnapshotCall {
		t.Fatal("expected snapshot restoration to be attempted")
	}

	if agg.setVersionCall {
		t.Fatal("setVersion should not be called when snapshot restoration fails")
	}
}
