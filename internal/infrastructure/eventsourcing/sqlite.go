// Package eventsourcing provides infrastructure for event sourcing.
package eventsourcing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	domain "github.com/anthropics/claude-flow-go/internal/domain/eventsourcing"
)

// SQLiteEventStore implements EventStore using SQLite.
type SQLiteEventStore struct {
	mu            sync.RWMutex
	db            *sql.DB
	config        EventStoreConfig
	closed        bool
	subscriptions map[string]*eventSubscription
}

type eventSubscription struct {
	id         string
	eventTypes []string
	handler    domain.EventHandler
	all        bool
}

// NewSQLiteEventStore creates a new SQLite event store.
func NewSQLiteEventStore(config EventStoreConfig) (*SQLiteEventStore, error) {
	dbPath := config.DatabasePath
	if dbPath == "" {
		dbPath = ".data/events.db"
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("%w: failed to create directory: %v", domain.ErrStoreInitFailed, err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open database: %v", domain.ErrStoreInitFailed, err)
	}

	store := &SQLiteEventStore{
		db:            db,
		config:        config,
		subscriptions: make(map[string]*eventSubscription),
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initSchema creates the database schema.
func (s *SQLiteEventStore) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			aggregate_id TEXT NOT NULL,
			aggregate_type TEXT NOT NULL,
			event_type TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'domain',
			version INTEGER NOT NULL,
			payload BLOB NOT NULL,
			metadata TEXT,
			timestamp INTEGER NOT NULL,
			UNIQUE(aggregate_id, version)
		);

		CREATE INDEX IF NOT EXISTS idx_events_aggregate ON events(aggregate_id, version);
		CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
		CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
		CREATE INDEX IF NOT EXISTS idx_events_category ON events(category);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("%w: failed to create schema: %v", domain.ErrStoreInitFailed, err)
	}

	return nil
}

// Append appends events to the store atomically.
func (s *SQLiteEventStore) Append(ctx context.Context, events []*domain.StoredEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return domain.ErrStoreClosed
	}

	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrTransactionFailed, err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO events (id, aggregate_id, aggregate_type, event_type, category, version, payload, metadata, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrTransactionFailed, err)
	}
	defer stmt.Close()

	for _, event := range events {
		// Validate version
		var currentVersion int
		err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(version), 0) FROM events WHERE aggregate_id = ?
		`, event.AggregateID).Scan(&currentVersion)
		if err != nil {
			return fmt.Errorf("%w: %v", domain.ErrTransactionFailed, err)
		}

		if event.Version != currentVersion+1 {
			return fmt.Errorf("%w: expected version %d, got %d",
				domain.ErrConcurrencyConflict, currentVersion+1, event.Version)
		}

		// Serialize metadata
		var metadataJSON []byte
		if event.Metadata != nil {
			metadataJSON, err = json.Marshal(event.Metadata)
			if err != nil {
				return fmt.Errorf("%w: %v", domain.ErrSerializationFailed, err)
			}
		}

		_, err = stmt.ExecContext(ctx,
			event.ID,
			event.AggregateID,
			event.AggregateType,
			event.Type,
			string(event.Category),
			event.Version,
			event.Payload,
			metadataJSON,
			event.Timestamp.UnixMilli(),
		)
		if err != nil {
			return fmt.Errorf("%w: %v", domain.ErrTransactionFailed, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrTransactionFailed, err)
	}

	// Notify subscribers
	for _, event := range events {
		s.notifySubscribers(event)
	}

	return nil
}

// Load loads events for an aggregate from a given version.
func (s *SQLiteEventStore) Load(ctx context.Context, aggregateID string, fromVersion int) ([]*domain.StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, aggregate_id, aggregate_type, event_type, category, version, payload, metadata, timestamp
		FROM events
		WHERE aggregate_id = ? AND version >= ?
		ORDER BY version ASC
	`, aggregateID, fromVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// LoadByType loads events by event type.
func (s *SQLiteEventStore) LoadByType(ctx context.Context, eventType string) ([]*domain.StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, aggregate_id, aggregate_type, event_type, category, version, payload, metadata, timestamp
		FROM events
		WHERE event_type = ?
		ORDER BY timestamp ASC
	`, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// Query queries events matching the given criteria.
func (s *SQLiteEventStore) Query(ctx context.Context, query *domain.EventQuery) ([]*domain.StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	// Build query dynamically
	sqlQuery := `
		SELECT id, aggregate_id, aggregate_type, event_type, category, version, payload, metadata, timestamp
		FROM events WHERE 1=1
	`
	args := make([]interface{}, 0)

	if query.AggregateID != "" {
		sqlQuery += " AND aggregate_id = ?"
		args = append(args, query.AggregateID)
	}

	if query.AggregateType != "" {
		sqlQuery += " AND aggregate_type = ?"
		args = append(args, query.AggregateType)
	}

	if len(query.EventTypes) > 0 {
		placeholders := ""
		for i, t := range query.EventTypes {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, t)
		}
		sqlQuery += " AND event_type IN (" + placeholders + ")"
	}

	if len(query.Categories) > 0 {
		placeholders := ""
		for i, c := range query.Categories {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, string(c))
		}
		sqlQuery += " AND category IN (" + placeholders + ")"
	}

	if query.FromVersion > 0 {
		sqlQuery += " AND version >= ?"
		args = append(args, query.FromVersion)
	}

	if query.ToVersion > 0 {
		sqlQuery += " AND version <= ?"
		args = append(args, query.ToVersion)
	}

	if query.FromTimestamp != nil {
		sqlQuery += " AND timestamp >= ?"
		args = append(args, query.FromTimestamp.UnixMilli())
	}

	if query.ToTimestamp != nil {
		sqlQuery += " AND timestamp <= ?"
		args = append(args, query.ToTimestamp.UnixMilli())
	}

	sqlQuery += " ORDER BY timestamp ASC"

	if query.Limit > 0 {
		sqlQuery += " LIMIT ?"
		args = append(args, query.Limit)
	}

	if query.Offset > 0 {
		sqlQuery += " OFFSET ?"
		args = append(args, query.Offset)
	}

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events, err := s.scanEvents(rows)
	if err != nil {
		return nil, err
	}

	// Apply metadata filters in memory
	if query.CorrelationID != "" || query.Actor != "" {
		filtered := make([]*domain.StoredEvent, 0)
		for _, event := range events {
			if query.CorrelationID != "" && (event.Metadata == nil || event.Metadata.CorrelationID != query.CorrelationID) {
				continue
			}
			if query.Actor != "" && (event.Metadata == nil || event.Metadata.Actor != query.Actor) {
				continue
			}
			filtered = append(filtered, event)
		}
		return filtered, nil
	}

	return events, nil
}

// scanEvents scans rows into StoredEvent slice.
func (s *SQLiteEventStore) scanEvents(rows *sql.Rows) ([]*domain.StoredEvent, error) {
	events := make([]*domain.StoredEvent, 0)

	for rows.Next() {
		var (
			id            string
			aggregateID   string
			aggregateType string
			eventType     string
			category      string
			version       int
			payload       []byte
			metadataJSON  sql.NullString
			timestampMs   int64
		)

		if err := rows.Scan(&id, &aggregateID, &aggregateType, &eventType, &category, &version, &payload, &metadataJSON, &timestampMs); err != nil {
			return nil, err
		}

		event := &domain.StoredEvent{
			ID:            id,
			AggregateID:   aggregateID,
			AggregateType: aggregateType,
			Type:          eventType,
			Category:      domain.EventCategory(category),
			Version:       version,
			Payload:       payload,
			Timestamp:     time.UnixMilli(timestampMs),
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			var metadata domain.EventMetadata
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
				event.Metadata = &metadata
			}
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// GetVersion returns the current version of an aggregate.
func (s *SQLiteEventStore) GetVersion(ctx context.Context, aggregateID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, domain.ErrStoreClosed
	}

	var version int
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM events WHERE aggregate_id = ?
	`, aggregateID).Scan(&version)
	if err != nil {
		return 0, err
	}

	return version, nil
}

// Subscribe subscribes to events of specific types.
func (s *SQLiteEventStore) Subscribe(eventTypes []string, handler domain.EventHandler) domain.Subscription {
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
func (s *SQLiteEventStore) SubscribeAll(handler domain.EventHandler) domain.Subscription {
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

// notifySubscribers notifies matching subscribers of an event.
func (s *SQLiteEventStore) notifySubscribers(event *domain.StoredEvent) {
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
func (s *SQLiteEventStore) Count(ctx context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, domain.ErrStoreClosed
	}

	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&count)
	return count, err
}

// CountByAggregate returns the number of events for an aggregate.
func (s *SQLiteEventStore) CountByAggregate(ctx context.Context, aggregateID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, domain.ErrStoreClosed
	}

	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM events WHERE aggregate_id = ?
	`, aggregateID).Scan(&count)
	return count, err
}

// Close closes the event store.
func (s *SQLiteEventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.db.Close()
}

// Vacuum reclaims unused space in the database.
func (s *SQLiteEventStore) Vacuum(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return domain.ErrStoreClosed
	}

	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
}

// GetStats returns store statistics.
func (s *SQLiteEventStore) GetStats(ctx context.Context) (*EventStoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	stats := &EventStoreStats{}

	// Total events
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&stats.TotalEvents)

	// Unique aggregates
	s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT aggregate_id) FROM events`).Scan(&stats.UniqueAggregates)

	// Events by category
	rows, err := s.db.QueryContext(ctx, `
		SELECT category, COUNT(*) FROM events GROUP BY category
	`)
	if err == nil {
		stats.EventsByCategory = make(map[string]int64)
		defer rows.Close()
		for rows.Next() {
			var cat string
			var count int64
			if rows.Scan(&cat, &count) == nil {
				stats.EventsByCategory[cat] = count
			}
		}
	}

	// Oldest and newest timestamps
	s.db.QueryRowContext(ctx, `SELECT MIN(timestamp) FROM events`).Scan(&stats.OldestEventMs)
	s.db.QueryRowContext(ctx, `SELECT MAX(timestamp) FROM events`).Scan(&stats.NewestEventMs)

	return stats, nil
}

// EventStoreStats contains store statistics.
type EventStoreStats struct {
	TotalEvents      int64            `json:"totalEvents"`
	UniqueAggregates int64            `json:"uniqueAggregates"`
	EventsByCategory map[string]int64 `json:"eventsByCategory"`
	OldestEventMs    int64            `json:"oldestEventMs"`
	NewestEventMs    int64            `json:"newestEventMs"`
}
