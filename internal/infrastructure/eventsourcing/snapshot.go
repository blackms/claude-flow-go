// Package eventsourcing provides infrastructure for event sourcing.
package eventsourcing

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	domain "github.com/anthropics/claude-flow-go/internal/domain/eventsourcing"
)

// SQLiteSnapshotStore implements SnapshotStore using SQLite.
type SQLiteSnapshotStore struct {
	mu     sync.RWMutex
	db     *sql.DB
	closed bool
}

// NewSQLiteSnapshotStore creates a new SQLite snapshot store.
func NewSQLiteSnapshotStore(dbPath string) (*SQLiteSnapshotStore, error) {
	if dbPath == "" {
		dbPath = ".data/snapshots.db"
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

	store := &SQLiteSnapshotStore{db: db}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initSchema creates the database schema.
func (s *SQLiteSnapshotStore) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS snapshots (
			aggregate_id TEXT NOT NULL,
			aggregate_type TEXT NOT NULL,
			version INTEGER NOT NULL,
			state BLOB NOT NULL,
			created_at INTEGER NOT NULL,
			PRIMARY KEY (aggregate_id)
		);

		CREATE INDEX IF NOT EXISTS idx_snapshots_type ON snapshots(aggregate_type);
		CREATE INDEX IF NOT EXISTS idx_snapshots_created ON snapshots(created_at);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("%w: failed to create schema: %v", domain.ErrStoreInitFailed, err)
	}

	return nil
}

// Save saves a snapshot.
func (s *SQLiteSnapshotStore) Save(ctx context.Context, snapshot *domain.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return domain.ErrStoreClosed
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO snapshots (aggregate_id, aggregate_type, version, state, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, snapshot.AggregateID, snapshot.AggregateType, snapshot.Version, snapshot.State, snapshot.CreatedAt)

	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrSnapshotCreationFailed, err)
	}

	return nil
}

// Load loads the latest snapshot for an aggregate.
func (s *SQLiteSnapshotStore) Load(ctx context.Context, aggregateID string) (*domain.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	var (
		aggregateType string
		version       int
		state         []byte
		createdAt     int64
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT aggregate_type, version, state, created_at
		FROM snapshots WHERE aggregate_id = ?
	`, aggregateID).Scan(&aggregateType, &version, &state, &createdAt)

	if err == sql.ErrNoRows {
		return nil, domain.ErrSnapshotNotFound
	}
	if err != nil {
		return nil, err
	}

	return &domain.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		Version:       version,
		State:         state,
		CreatedAt:     createdAt,
	}, nil
}

// Delete deletes snapshots for an aggregate.
func (s *SQLiteSnapshotStore) Delete(ctx context.Context, aggregateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return domain.ErrStoreClosed
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM snapshots WHERE aggregate_id = ?`, aggregateID)
	return err
}

// DeleteOlderThan deletes snapshots older than a given version.
func (s *SQLiteSnapshotStore) DeleteOlderThan(ctx context.Context, aggregateID string, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return domain.ErrStoreClosed
	}

	_, err := s.db.ExecContext(ctx, `
		DELETE FROM snapshots WHERE aggregate_id = ? AND version < ?
	`, aggregateID, version)
	return err
}

// Close closes the snapshot store.
func (s *SQLiteSnapshotStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.db.Close()
}

// InMemorySnapshotStore implements SnapshotStore using in-memory storage.
type InMemorySnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[string]*domain.Snapshot
	closed    bool
}

// NewInMemorySnapshotStore creates a new in-memory snapshot store.
func NewInMemorySnapshotStore() *InMemorySnapshotStore {
	return &InMemorySnapshotStore{
		snapshots: make(map[string]*domain.Snapshot),
	}
}

// Save saves a snapshot.
func (s *InMemorySnapshotStore) Save(ctx context.Context, snapshot *domain.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return domain.ErrStoreClosed
	}

	s.snapshots[snapshot.AggregateID] = snapshot
	return nil
}

// Load loads the latest snapshot for an aggregate.
func (s *InMemorySnapshotStore) Load(ctx context.Context, aggregateID string) (*domain.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, domain.ErrStoreClosed
	}

	snapshot, exists := s.snapshots[aggregateID]
	if !exists {
		return nil, domain.ErrSnapshotNotFound
	}

	return snapshot, nil
}

// Delete deletes snapshots for an aggregate.
func (s *InMemorySnapshotStore) Delete(ctx context.Context, aggregateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return domain.ErrStoreClosed
	}

	delete(s.snapshots, aggregateID)
	return nil
}

// DeleteOlderThan deletes snapshots older than a given version.
func (s *InMemorySnapshotStore) DeleteOlderThan(ctx context.Context, aggregateID string, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return domain.ErrStoreClosed
	}

	if snapshot, exists := s.snapshots[aggregateID]; exists {
		if snapshot.Version < version {
			delete(s.snapshots, aggregateID)
		}
	}

	return nil
}

// Close closes the snapshot store.
func (s *InMemorySnapshotStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

// Clear clears all snapshots (for testing).
func (s *InMemorySnapshotStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots = make(map[string]*domain.Snapshot)
}

// SnapshotConfig configures snapshot behavior.
type SnapshotConfig struct {
	// Enabled enables snapshotting.
	Enabled bool `json:"enabled"`

	// Frequency is the number of events between snapshots.
	Frequency int `json:"frequency"`

	// MaxAge is the maximum age of snapshots in milliseconds.
	MaxAgeMs int64 `json:"maxAgeMs"`
}

// DefaultSnapshotConfig returns the default snapshot configuration.
func DefaultSnapshotConfig() SnapshotConfig {
	return SnapshotConfig{
		Enabled:   true,
		Frequency: 100,
		MaxAgeMs:  7 * 24 * 60 * 60 * 1000, // 7 days
	}
}

// ShouldSnapshot returns true if a snapshot should be created.
func (c SnapshotConfig) ShouldSnapshot(currentVersion int, lastSnapshotVersion int) bool {
	if !c.Enabled || c.Frequency <= 0 {
		return false
	}
	return currentVersion-lastSnapshotVersion >= c.Frequency
}

// IsExpired returns true if a snapshot is expired.
func (c SnapshotConfig) IsExpired(snapshot *domain.Snapshot) bool {
	if c.MaxAgeMs <= 0 {
		return false
	}
	age := time.Now().UnixMilli() - snapshot.CreatedAt
	return age > c.MaxAgeMs
}
