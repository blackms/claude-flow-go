package eventsourcing

import "testing"

type snapshotTestAggregate struct {
	id               string
	aggregateType    string
	version          int
	appliedVersions  []int
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
