// Package ruvector provides PostgreSQL/pgvector integration for vector operations.
package ruvector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
	"github.com/google/uuid"
)

// Backend implements the MemoryBackend interface using PostgreSQL/pgvector.
type Backend struct {
	conn   *Connection
	schema *SchemaManager
}

// NewBackend creates a new RuVector backend.
func NewBackend(config RuVectorConfig) (*Backend, error) {
	conn := NewConnection(config)
	return &Backend{
		conn:   conn,
		schema: NewSchemaManager(conn),
	}, nil
}

// Initialize connects to the database.
func (b *Backend) Initialize() error {
	ctx := context.Background()
	return b.conn.Connect(ctx)
}

// Close closes the database connection.
func (b *Backend) Close() error {
	return b.conn.Close()
}

// Store stores a memory in the database.
func (b *Backend) Store(memory shared.Memory) (shared.Memory, error) {
	ctx := context.Background()
	config := b.conn.Config()
	schema := config.Schema

	if memory.ID == "" {
		memory.ID = uuid.New().String()
	}
	if memory.Timestamp == 0 {
		memory.Timestamp = time.Now().UnixMilli()
	}

	// Convert embedding to PostgreSQL format
	embeddingStr := formatEmbedding(memory.Embedding, config.Dimensions)
	metadataJSON, _ := json.Marshal(memory.Metadata)

	_, err := b.conn.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.embeddings (id, key, namespace, embedding, content, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, %s, $4, $5, $6, $6)
		ON CONFLICT (key, namespace) DO UPDATE SET
			embedding = EXCLUDED.embedding,
			content = EXCLUDED.content,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
	`, schema, embeddingStr),
		memory.ID,
		memory.ID,                           // Use ID as key
		string(memory.Type),                 // Use type as namespace
		memory.Content,
		string(metadataJSON),
		time.UnixMilli(memory.Timestamp),
	)

	if err != nil {
		return memory, fmt.Errorf("failed to store memory: %w", err)
	}

	return memory, nil
}

// Retrieve retrieves a memory by ID.
func (b *Backend) Retrieve(id string) (*shared.Memory, error) {
	ctx := context.Background()
	schema := b.conn.Config().Schema

	var memory shared.Memory
	var metadataStr string
	var createdAt time.Time
	var namespace string

	err := b.conn.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, key, namespace, content, metadata, created_at
		FROM %s.embeddings
		WHERE id = $1 OR key = $1
	`, schema), id).Scan(
		&memory.ID,
		&memory.AgentID, // key used as AgentID placeholder
		&namespace,
		&memory.Content,
		&metadataStr,
		&createdAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	memory.Type = shared.MemoryType(namespace)
	memory.Timestamp = createdAt.UnixMilli()
	if metadataStr != "" {
		json.Unmarshal([]byte(metadataStr), &memory.Metadata)
	}

	return &memory, nil
}

// Update updates an existing memory.
func (b *Backend) Update(memory shared.Memory) error {
	_, err := b.Store(memory)
	return err
}

// Delete deletes a memory by ID.
func (b *Backend) Delete(id string) error {
	ctx := context.Background()
	schema := b.conn.Config().Schema

	_, err := b.conn.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.embeddings WHERE id = $1 OR key = $1
	`, schema), id)

	return err
}

// Query queries memories with filters.
func (b *Backend) Query(query shared.MemoryQuery) ([]shared.Memory, error) {
	ctx := context.Background()
	schema := b.conn.Config().Schema

	var conditions []string
	var args []interface{}
	argIdx := 1

	if query.AgentID != "" {
		conditions = append(conditions, fmt.Sprintf("key LIKE $%d", argIdx))
		args = append(args, query.AgentID+"%")
		argIdx++
	}

	if query.Type != "" {
		conditions = append(conditions, fmt.Sprintf("namespace = $%d", argIdx))
		args = append(args, string(query.Type))
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}

	rows, err := b.conn.Query(ctx, fmt.Sprintf(`
		SELECT id, key, namespace, content, metadata, created_at
		FROM %s.embeddings
		%s
		ORDER BY created_at DESC
		LIMIT %d
	`, schema, whereClause, limit), args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []shared.Memory
	for rows.Next() {
		var m shared.Memory
		var metadataStr string
		var createdAt time.Time
		var namespace, key string

		if err := rows.Scan(&m.ID, &key, &namespace, &m.Content, &metadataStr, &createdAt); err != nil {
			return nil, err
		}

		m.AgentID = key
		m.Type = shared.MemoryType(namespace)
		m.Timestamp = createdAt.UnixMilli()
		if metadataStr != "" {
			json.Unmarshal([]byte(metadataStr), &m.Metadata)
		}

		memories = append(memories, m)
	}

	return memories, rows.Err()
}

// VectorSearch performs similarity search using pgvector.
func (b *Backend) VectorSearch(embedding []float64, k int) ([]shared.MemorySearchResult, error) {
	ctx := context.Background()
	config := b.conn.Config()
	schema := config.Schema

	// Convert float64 to float32 and format for pgvector
	embedding32 := make([]float32, len(embedding))
	for i, v := range embedding {
		embedding32[i] = float32(v)
	}
	embeddingStr := formatEmbedding(embedding32, config.Dimensions)

	rows, err := b.conn.Query(ctx, fmt.Sprintf(`
		SELECT 
			id, key, namespace, content, metadata, created_at,
			1 - (embedding <=> %s) as similarity
		FROM %s.embeddings
		WHERE embedding IS NOT NULL
		ORDER BY embedding <=> %s
		LIMIT $1
	`, embeddingStr, schema, embeddingStr), k)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []shared.MemorySearchResult
	for rows.Next() {
		var m shared.Memory
		var metadataStr string
		var createdAt time.Time
		var namespace, key string
		var similarity float64

		if err := rows.Scan(&m.ID, &key, &namespace, &m.Content, &metadataStr, &createdAt, &similarity); err != nil {
			return nil, err
		}

		m.AgentID = key
		m.Type = shared.MemoryType(namespace)
		m.Timestamp = createdAt.UnixMilli()
		if metadataStr != "" {
			json.Unmarshal([]byte(metadataStr), &m.Metadata)
		}

		results = append(results, shared.MemorySearchResult{
			Memory:     m,
			Similarity: similarity,
		})
	}

	return results, rows.Err()
}

// ClearAgent clears all memories for an agent.
func (b *Backend) ClearAgent(agentID string) error {
	ctx := context.Background()
	schema := b.conn.Config().Schema

	_, err := b.conn.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.embeddings WHERE key LIKE $1
	`, schema), agentID+"%")

	return err
}

// StoreEmbedding stores a vector embedding directly.
func (b *Backend) StoreEmbedding(emb Embedding) error {
	ctx := context.Background()
	config := b.conn.Config()
	schema := config.Schema

	if emb.ID == "" {
		emb.ID = uuid.New().String()
	}
	if emb.Namespace == "" {
		emb.Namespace = "default"
	}

	embeddingStr := formatEmbedding(emb.Vector, config.Dimensions)
	metadataJSON, _ := json.Marshal(emb.Metadata)

	_, err := b.conn.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.embeddings (id, key, namespace, embedding, content, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, %s, $4, $5, NOW(), NOW())
		ON CONFLICT (key, namespace) DO UPDATE SET
			embedding = EXCLUDED.embedding,
			content = EXCLUDED.content,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
	`, schema, embeddingStr),
		emb.ID,
		emb.Key,
		emb.Namespace,
		emb.Content,
		string(metadataJSON),
	)

	return err
}

// SearchSimilar performs similarity search and returns embeddings.
func (b *Backend) SearchSimilar(queryVector []float32, k int, namespace string) ([]SearchResult, error) {
	ctx := context.Background()
	config := b.conn.Config()
	schema := config.Schema

	embeddingStr := formatEmbedding(queryVector, config.Dimensions)

	var args []interface{}
	args = append(args, k)

	whereClause := "WHERE embedding IS NOT NULL"
	if namespace != "" {
		whereClause += " AND namespace = $2"
		args = append(args, namespace)
	}

	rows, err := b.conn.Query(ctx, fmt.Sprintf(`
		SELECT 
			id, key, namespace, content, metadata, created_at, updated_at,
			embedding <=> %s as distance
		FROM %s.embeddings
		%s
		ORDER BY embedding <=> %s
		LIMIT $1
	`, embeddingStr, schema, whereClause, embeddingStr), args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var emb Embedding
		var metadataStr string
		var distance float64

		if err := rows.Scan(
			&emb.ID, &emb.Key, &emb.Namespace, &emb.Content,
			&metadataStr, &emb.CreatedAt, &emb.UpdatedAt, &distance,
		); err != nil {
			return nil, err
		}

		if metadataStr != "" {
			json.Unmarshal([]byte(metadataStr), &emb.Metadata)
		}

		results = append(results, SearchResult{
			Embedding:  emb,
			Distance:   distance,
			Similarity: 1 - distance,
		})
	}

	return results, rows.Err()
}

// GetEmbeddingCount returns the number of embeddings in the database.
func (b *Backend) GetEmbeddingCount(ctx context.Context) (int64, error) {
	schema := b.conn.Config().Schema
	var count int64
	err := b.conn.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.embeddings
	`, schema)).Scan(&count)
	return count, err
}

// formatEmbedding formats a vector for PostgreSQL.
func formatEmbedding(vector []float32, dimensions int) string {
	if len(vector) == 0 {
		return "NULL"
	}

	// Pad or truncate to match dimensions
	adjusted := make([]float32, dimensions)
	copy(adjusted, vector)

	parts := make([]string, len(adjusted))
	for i, v := range adjusted {
		parts[i] = fmt.Sprintf("%f", v)
	}

	return fmt.Sprintf("'[%s]'::vector(%d)", strings.Join(parts, ","), dimensions)
}

// Connection returns the underlying connection for advanced operations.
func (b *Backend) Connection() *Connection {
	return b.conn
}

// Schema returns the schema manager.
func (b *Backend) Schema() *SchemaManager {
	return b.schema
}
