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

// ReplayProgress reports replay progress.
type ReplayProgress struct {
	// TotalEvents is the total number of events to replay.
	TotalEvents int64 `json:"totalEvents"`

	// ProcessedEvents is the number of events processed.
	ProcessedEvents int64 `json:"processedEvents"`

	// FailedEvents is the number of failed events.
	FailedEvents int64 `json:"failedEvents"`

	// CurrentEvent is the current event being processed.
	CurrentEvent *domain.StoredEvent `json:"currentEvent,omitempty"`

	// StartedAt is when the replay started.
	StartedAt time.Time `json:"startedAt"`

	// ElapsedMs is the elapsed time in milliseconds.
	ElapsedMs int64 `json:"elapsedMs"`

	// EventsPerSecond is the processing rate.
	EventsPerSecond float64 `json:"eventsPerSecond"`

	// Completed indicates if replay is complete.
	Completed bool `json:"completed"`

	// Error is any error that occurred.
	Error string `json:"error,omitempty"`
}

// ProgressPercent returns the progress percentage.
func (p *ReplayProgress) ProgressPercent() float64 {
	if p.TotalEvents == 0 {
		return 100.0
	}
	return float64(p.ProcessedEvents) / float64(p.TotalEvents) * 100.0
}

// ReplayConfig configures replay behavior.
type ReplayConfig struct {
	// BatchSize is the number of events to process in each batch.
	BatchSize int `json:"batchSize"`

	// ParallelWorkers is the number of parallel workers.
	ParallelWorkers int `json:"parallelWorkers"`

	// ContinueOnError continues replay on errors.
	ContinueOnError bool `json:"continueOnError"`

	// ProgressCallback is called periodically with progress updates.
	ProgressCallback func(progress *ReplayProgress)

	// ProgressIntervalMs is the progress callback interval.
	ProgressIntervalMs int `json:"progressIntervalMs"`
}

// DefaultReplayConfig returns the default replay configuration.
func DefaultReplayConfig() ReplayConfig {
	return ReplayConfig{
		BatchSize:          1000,
		ParallelWorkers:    1,
		ContinueOnError:    false,
		ProgressIntervalMs: 1000,
	}
}

// EventReplayer provides event replay functionality.
type EventReplayer struct {
	eventStore    infra.EventStore
	snapshotStore infra.SnapshotStore
	serializer    infra.EventSerializer
	config        ReplayConfig
}

// NewEventReplayer creates a new event replayer.
func NewEventReplayer(eventStore infra.EventStore, config ReplayConfig) *EventReplayer {
	return &EventReplayer{
		eventStore: eventStore,
		serializer: infra.NewJSONEventSerializer(),
		config:     config,
	}
}

// WithSnapshotStore sets the snapshot store.
func (r *EventReplayer) WithSnapshotStore(store infra.SnapshotStore) *EventReplayer {
	r.snapshotStore = store
	return r
}

// ReplayAll replays all events through a handler.
func (r *EventReplayer) ReplayAll(ctx context.Context, handler domain.EventHandler) (*ReplayProgress, error) {
	return r.Replay(ctx, &domain.EventQuery{}, handler)
}

// ReplayAggregate replays events for a specific aggregate.
func (r *EventReplayer) ReplayAggregate(ctx context.Context, aggregateID string, handler domain.EventHandler) (*ReplayProgress, error) {
	return r.Replay(ctx, &domain.EventQuery{AggregateID: aggregateID}, handler)
}

// ReplayFromSnapshot replays from snapshot plus subsequent events.
func (r *EventReplayer) ReplayFromSnapshot(ctx context.Context, aggregateID string, handler domain.EventHandler) (*ReplayProgress, error) {
	fromVersion := 0

	if r.snapshotStore != nil {
		snapshot, err := r.snapshotStore.Load(ctx, aggregateID)
		if err == nil {
			fromVersion = snapshot.Version + 1
		}
	}

	return r.Replay(ctx, &domain.EventQuery{
		AggregateID: aggregateID,
		FromVersion: fromVersion,
	}, handler)
}

// ReplayToPoint replays events up to a specific point in time.
func (r *EventReplayer) ReplayToPoint(ctx context.Context, pointInTime time.Time, handler domain.EventHandler) (*ReplayProgress, error) {
	return r.Replay(ctx, &domain.EventQuery{
		ToTimestamp: &pointInTime,
	}, handler)
}

// ReplayByType replays events of specific types.
func (r *EventReplayer) ReplayByType(ctx context.Context, eventTypes []string, handler domain.EventHandler) (*ReplayProgress, error) {
	return r.Replay(ctx, &domain.EventQuery{
		EventTypes: eventTypes,
	}, handler)
}

// Replay replays events matching the query through a handler.
func (r *EventReplayer) Replay(ctx context.Context, query *domain.EventQuery, handler domain.EventHandler) (*ReplayProgress, error) {
	progress := &ReplayProgress{
		StartedAt: time.Now(),
	}

	// Get total count
	totalCount, err := r.eventStore.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get event count: %w", err)
	}
	progress.TotalEvents = totalCount

	// Load events
	events, err := r.eventStore.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}

	progress.TotalEvents = int64(len(events))

	// Start progress reporting
	var progressTicker *time.Ticker
	var progressDone chan struct{}
	if r.config.ProgressCallback != nil && r.config.ProgressIntervalMs > 0 {
		progressTicker = time.NewTicker(time.Duration(r.config.ProgressIntervalMs) * time.Millisecond)
		progressDone = make(chan struct{})
		go func() {
			for {
				select {
				case <-progressTicker.C:
					progress.ElapsedMs = time.Since(progress.StartedAt).Milliseconds()
					if progress.ElapsedMs > 0 {
						progress.EventsPerSecond = float64(progress.ProcessedEvents) / (float64(progress.ElapsedMs) / 1000.0)
					}
					r.config.ProgressCallback(progress)
				case <-progressDone:
					return
				}
			}
		}()
	}

	// Process events
	for _, event := range events {
		select {
		case <-ctx.Done():
			if progressTicker != nil {
				progressTicker.Stop()
				close(progressDone)
			}
			return progress, ctx.Err()
		default:
		}

		progress.CurrentEvent = event

		if err := handler(event); err != nil {
			atomic.AddInt64(&progress.FailedEvents, 1)
			if !r.config.ContinueOnError {
				if progressTicker != nil {
					progressTicker.Stop()
					close(progressDone)
				}
				progress.Error = err.Error()
				return progress, fmt.Errorf("%w: %v", domain.ErrReplayFailed, err)
			}
		}

		atomic.AddInt64(&progress.ProcessedEvents, 1)
	}

	// Finalize
	if progressTicker != nil {
		progressTicker.Stop()
		close(progressDone)
	}

	progress.Completed = true
	progress.ElapsedMs = time.Since(progress.StartedAt).Milliseconds()
	if progress.ElapsedMs > 0 {
		progress.EventsPerSecond = float64(progress.ProcessedEvents) / (float64(progress.ElapsedMs) / 1000.0)
	}

	if r.config.ProgressCallback != nil {
		r.config.ProgressCallback(progress)
	}

	return progress, nil
}

// ParallelReplay replays events in parallel.
func (r *EventReplayer) ParallelReplay(ctx context.Context, query *domain.EventQuery, handler domain.EventHandler) (*ReplayProgress, error) {
	if r.config.ParallelWorkers <= 1 {
		return r.Replay(ctx, query, handler)
	}

	progress := &ReplayProgress{
		StartedAt: time.Now(),
	}

	// Load all events
	events, err := r.eventStore.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}

	progress.TotalEvents = int64(len(events))

	// Create worker pool
	eventChan := make(chan *domain.StoredEvent, r.config.BatchSize)
	errChan := make(chan error, r.config.ParallelWorkers)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < r.config.ParallelWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for event := range eventChan {
				if err := handler(event); err != nil {
					atomic.AddInt64(&progress.FailedEvents, 1)
					if !r.config.ContinueOnError {
						errChan <- err
						return
					}
				}
				atomic.AddInt64(&progress.ProcessedEvents, 1)
			}
		}()
	}

	// Feed events to workers
	go func() {
		for _, event := range events {
			select {
			case <-ctx.Done():
				close(eventChan)
				return
			case eventChan <- event:
			}
		}
		close(eventChan)
	}()

	// Wait for completion
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			progress.Error = err.Error()
			return progress, err
		}
	}

	progress.Completed = true
	progress.ElapsedMs = time.Since(progress.StartedAt).Milliseconds()
	if progress.ElapsedMs > 0 {
		progress.EventsPerSecond = float64(progress.ProcessedEvents) / (float64(progress.ElapsedMs) / 1000.0)
	}

	return progress, nil
}

// RebuildAggregate rebuilds an aggregate from its events.
func (r *EventReplayer) RebuildAggregate(ctx context.Context, aggregateID string, aggregate domain.Aggregate) error {
	events, err := r.eventStore.Load(ctx, aggregateID, 0)
	if err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}

	for _, stored := range events {
		event, err := r.serializer.Deserialize(stored)
		if err != nil {
			return fmt.Errorf("failed to deserialize event: %w", err)
		}
		if err := aggregate.ApplyEvent(event); err != nil {
			return fmt.Errorf("failed to apply event: %w", err)
		}
	}

	return nil
}
