// Package memory provides memory backend implementations.
package memory

import (
	"database/sql"
	"encoding/json"
	"sort"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
	_ "modernc.org/sqlite"
)

// SQLiteBackend implements MemoryBackend using SQLite.
type SQLiteBackend struct {
	mu          sync.RWMutex
	dbPath      string
	db          *sql.DB
	memories    map[string]shared.Memory // In-memory fallback
	initialized bool
	useInMemory bool
}

// SQLiteOption configures the SQLiteBackend.
type SQLiteOption func(*SQLiteBackend)

// WithInMemoryFallback enables in-memory storage fallback.
func WithInMemoryFallback() SQLiteOption {
	return func(sb *SQLiteBackend) {
		sb.useInMemory = true
	}
}

// NewSQLiteBackend creates a new SQLite-based memory backend.
func NewSQLiteBackend(dbPath string, opts ...SQLiteOption) *SQLiteBackend {
	sb := &SQLiteBackend{
		dbPath:      dbPath,
		memories:    make(map[string]shared.Memory),
		useInMemory: false,
	}

	for _, opt := range opts {
		opt(sb)
	}

	return sb
}

// Initialize initializes the SQLite database.
func (sb *SQLiteBackend) Initialize() error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.initialized {
		return nil
	}

	if sb.useInMemory || sb.dbPath == "" || sb.dbPath == ":memory:" {
		// Use in-memory storage
		sb.useInMemory = true
		sb.initialized = true
		return nil
	}

	// Open SQLite database
	db, err := sql.Open("sqlite", sb.dbPath)
	if err != nil {
		// Fall back to in-memory
		sb.useInMemory = true
		sb.initialized = true
		return nil
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			content TEXT NOT NULL,
			type TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			embedding BLOB,
			metadata TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_memories_agent_id ON memories(agent_id);
		CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
		CREATE INDEX IF NOT EXISTS idx_memories_timestamp ON memories(timestamp);
	`)
	if err != nil {
		db.Close()
		// Fall back to in-memory
		sb.useInMemory = true
		sb.initialized = true
		return nil
	}

	sb.db = db
	sb.initialized = true
	return nil
}

// Close closes the database connection.
func (sb *SQLiteBackend) Close() error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.db != nil {
		err := sb.db.Close()
		sb.db = nil
		return err
	}

	sb.memories = make(map[string]shared.Memory)
	sb.initialized = false
	return nil
}

// Store stores a memory entry.
func (sb *SQLiteBackend) Store(memory shared.Memory) (shared.Memory, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.useInMemory {
		sb.memories[memory.ID] = memory
		return memory, nil
	}

	// Store in SQLite
	metadataJSON, err := json.Marshal(memory.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	var embeddingJSON []byte
	if len(memory.Embedding) > 0 {
		embeddingJSON, _ = json.Marshal(memory.Embedding)
	}

	_, err = sb.db.Exec(`
		INSERT OR REPLACE INTO memories (id, agent_id, content, type, timestamp, embedding, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, memory.ID, memory.AgentID, memory.Content, string(memory.Type), memory.Timestamp, embeddingJSON, metadataJSON)

	if err != nil {
		return memory, shared.NewMemoryError("failed to store memory", map[string]interface{}{"error": err.Error()})
	}

	return memory, nil
}

// Retrieve retrieves a memory entry by ID.
func (sb *SQLiteBackend) Retrieve(id string) (*shared.Memory, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	if sb.useInMemory {
		memory, exists := sb.memories[id]
		if !exists {
			return nil, nil
		}
		return &memory, nil
	}

	row := sb.db.QueryRow(`
		SELECT id, agent_id, content, type, timestamp, embedding, metadata
		FROM memories WHERE id = ?
	`, id)

	var memory shared.Memory
	var memType string
	var embeddingJSON, metadataJSON []byte

	err := row.Scan(&memory.ID, &memory.AgentID, &memory.Content, &memType, &memory.Timestamp, &embeddingJSON, &metadataJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, shared.NewMemoryError("failed to retrieve memory", map[string]interface{}{"error": err.Error()})
	}

	memory.Type = shared.MemoryType(memType)

	if len(embeddingJSON) > 0 {
		json.Unmarshal(embeddingJSON, &memory.Embedding)
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &memory.Metadata)
	}

	return &memory, nil
}

// Update updates a memory entry.
func (sb *SQLiteBackend) Update(memory shared.Memory) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.useInMemory {
		if _, exists := sb.memories[memory.ID]; exists {
			sb.memories[memory.ID] = memory
		}
		return nil
	}

	metadataJSON, _ := json.Marshal(memory.Metadata)
	var embeddingJSON []byte
	if len(memory.Embedding) > 0 {
		embeddingJSON, _ = json.Marshal(memory.Embedding)
	}

	_, err := sb.db.Exec(`
		UPDATE memories SET agent_id = ?, content = ?, type = ?, timestamp = ?, embedding = ?, metadata = ?
		WHERE id = ?
	`, memory.AgentID, memory.Content, string(memory.Type), memory.Timestamp, embeddingJSON, metadataJSON, memory.ID)

	return err
}

// Delete deletes a memory entry.
func (sb *SQLiteBackend) Delete(id string) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.useInMemory {
		delete(sb.memories, id)
		return nil
	}

	_, err := sb.db.Exec("DELETE FROM memories WHERE id = ?", id)
	return err
}

// Query queries memory entries with filters.
func (sb *SQLiteBackend) Query(query shared.MemoryQuery) ([]shared.Memory, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	if sb.useInMemory {
		return sb.queryInMemory(query), nil
	}

	return sb.querySQL(query)
}

func (sb *SQLiteBackend) queryInMemory(query shared.MemoryQuery) []shared.Memory {
	results := make([]shared.Memory, 0)

	for _, memory := range sb.memories {
		// Filter by agentId
		if query.AgentID != "" && memory.AgentID != query.AgentID {
			continue
		}

		// Filter by type
		if query.Type != "" && memory.Type != query.Type {
			continue
		}

		// Filter by time range
		if query.TimeRange != nil {
			if memory.Timestamp < query.TimeRange.Start || memory.Timestamp > query.TimeRange.End {
				continue
			}
		}

		// Filter by metadata
		if query.Metadata != nil {
			match := true
			for k, v := range query.Metadata {
				if memory.Metadata == nil || memory.Metadata[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		results = append(results, memory)
	}

	// Sort by timestamp (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp > results[j].Timestamp
	})

	// Apply pagination
	if query.Offset > 0 {
		if query.Offset >= len(results) {
			return []shared.Memory{}
		}
		results = results[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return results
}

func (sb *SQLiteBackend) querySQL(query shared.MemoryQuery) ([]shared.Memory, error) {
	sqlQuery := "SELECT id, agent_id, content, type, timestamp, embedding, metadata FROM memories WHERE 1=1"
	args := make([]interface{}, 0)

	if query.AgentID != "" {
		sqlQuery += " AND agent_id = ?"
		args = append(args, query.AgentID)
	}

	if query.Type != "" {
		sqlQuery += " AND type = ?"
		args = append(args, string(query.Type))
	}

	if query.TimeRange != nil {
		sqlQuery += " AND timestamp >= ? AND timestamp <= ?"
		args = append(args, query.TimeRange.Start, query.TimeRange.End)
	}

	sqlQuery += " ORDER BY timestamp DESC"

	if query.Limit > 0 {
		sqlQuery += " LIMIT ?"
		args = append(args, query.Limit)
	}

	if query.Offset > 0 {
		sqlQuery += " OFFSET ?"
		args = append(args, query.Offset)
	}

	rows, err := sb.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]shared.Memory, 0)
	for rows.Next() {
		var memory shared.Memory
		var memType string
		var embeddingJSON, metadataJSON []byte

		err := rows.Scan(&memory.ID, &memory.AgentID, &memory.Content, &memType, &memory.Timestamp, &embeddingJSON, &metadataJSON)
		if err != nil {
			continue
		}

		memory.Type = shared.MemoryType(memType)

		if len(embeddingJSON) > 0 {
			json.Unmarshal(embeddingJSON, &memory.Embedding)
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &memory.Metadata)
		}

		// Filter by metadata (SQLite doesn't support JSON queries easily)
		if query.Metadata != nil {
			match := true
			for k, v := range query.Metadata {
				if memory.Metadata == nil || memory.Metadata[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		results = append(results, memory)
	}

	return results, nil
}

// VectorSearch performs vector search (not supported in SQLite, returns empty).
func (sb *SQLiteBackend) VectorSearch(embedding []float64, k int) ([]shared.MemorySearchResult, error) {
	// SQLite doesn't support vector search natively
	// Return empty array - vector search handled by AgentDB
	return []shared.MemorySearchResult{}, nil
}

// ClearAgent clears all memories for an agent.
func (sb *SQLiteBackend) ClearAgent(agentID string) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.useInMemory {
		for id, memory := range sb.memories {
			if memory.AgentID == agentID {
				delete(sb.memories, id)
			}
		}
		return nil
	}

	_, err := sb.db.Exec("DELETE FROM memories WHERE agent_id = ?", agentID)
	return err
}

// GetDBPath returns the database path.
func (sb *SQLiteBackend) GetDBPath() string {
	return sb.dbPath
}

// GetCount returns the number of memories stored.
func (sb *SQLiteBackend) GetCount() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	if sb.useInMemory {
		return len(sb.memories)
	}

	var count int
	sb.db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count)
	return count
}
