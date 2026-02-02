// Package ruvector provides PostgreSQL/pgvector integration for vector operations.
package ruvector

import (
	"context"
	"fmt"
	"strings"
)

// SchemaManager handles database schema operations.
type SchemaManager struct {
	conn *Connection
}

// NewSchemaManager creates a new schema manager.
func NewSchemaManager(conn *Connection) *SchemaManager {
	return &SchemaManager{conn: conn}
}

// CreateExtension ensures the pgvector extension is installed.
func (s *SchemaManager) CreateExtension(ctx context.Context) error {
	_, err := s.conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}
	return nil
}

// CreateSchema creates the RuVector schema if it doesn't exist.
func (s *SchemaManager) CreateSchema(ctx context.Context) error {
	schema := s.conn.Config().Schema
	_, err := s.conn.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	return nil
}

// DropSchema drops the RuVector schema and all its contents.
func (s *SchemaManager) DropSchema(ctx context.Context) error {
	schema := s.conn.Config().Schema
	_, err := s.conn.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
	if err != nil {
		return fmt.Errorf("failed to drop schema: %w", err)
	}
	return nil
}

// CreateTables creates all required tables.
func (s *SchemaManager) CreateTables(ctx context.Context) error {
	config := s.conn.Config()
	schema := config.Schema
	dimensions := config.Dimensions

	tables := []string{
		// Main embeddings table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.embeddings (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				key TEXT NOT NULL,
				namespace TEXT DEFAULT 'default',
				embedding vector(%d),
				content TEXT,
				metadata JSONB DEFAULT '{}',
				created_at TIMESTAMPTZ DEFAULT NOW(),
				updated_at TIMESTAMPTZ DEFAULT NOW(),
				UNIQUE(key, namespace)
			)
		`, schema, dimensions),

		// Attention patterns table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.attention_patterns (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				pattern_type TEXT NOT NULL,
				query_embedding vector(%d),
				key_embedding vector(%d),
				value_embedding vector(%d),
				attention_score FLOAT,
				metadata JSONB DEFAULT '{}',
				created_at TIMESTAMPTZ DEFAULT NOW()
			)
		`, schema, dimensions, dimensions, dimensions),

		// Graph edges table for GNN
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.graph_edges (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				source_id TEXT NOT NULL,
				target_id TEXT NOT NULL,
				edge_type TEXT DEFAULT 'default',
				weight FLOAT DEFAULT 1.0,
				embedding vector(%d),
				metadata JSONB DEFAULT '{}',
				created_at TIMESTAMPTZ DEFAULT NOW()
			)
		`, schema, dimensions),

		// Migrations tracking table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.migrations (
				version TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TIMESTAMPTZ DEFAULT NOW(),
				checksum TEXT
			)
		`, schema),

		// Metadata table
		fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.metadata (
				key TEXT PRIMARY KEY,
				value JSONB NOT NULL,
				updated_at TIMESTAMPTZ DEFAULT NOW()
			)
		`, schema),
	}

	for _, tableSQL := range tables {
		if _, err := s.conn.Exec(ctx, tableSQL); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// CreateIndexes creates all required indexes.
func (s *SchemaManager) CreateIndexes(ctx context.Context) error {
	config := s.conn.Config()
	schema := config.Schema
	hnswConfig := DefaultHNSWConfig()

	var indexes []string

	if config.IndexType == "hnsw" {
		indexes = []string{
			// HNSW index on embeddings
			fmt.Sprintf(`
				CREATE INDEX IF NOT EXISTS idx_embeddings_hnsw
				ON %s.embeddings
				USING hnsw (embedding vector_cosine_ops)
				WITH (m = %d, ef_construction = %d)
			`, schema, hnswConfig.M, hnswConfig.EfConstruction),

			// HNSW index on attention patterns
			fmt.Sprintf(`
				CREATE INDEX IF NOT EXISTS idx_attention_query_hnsw
				ON %s.attention_patterns
				USING hnsw (query_embedding vector_cosine_ops)
				WITH (m = %d, ef_construction = %d)
			`, schema, hnswConfig.M, hnswConfig.EfConstruction),

			// HNSW index on graph edges
			fmt.Sprintf(`
				CREATE INDEX IF NOT EXISTS idx_graph_edges_hnsw
				ON %s.graph_edges
				USING hnsw (embedding vector_cosine_ops)
				WITH (m = %d, ef_construction = %d)
			`, schema, hnswConfig.M, hnswConfig.EfConstruction),
		}
	} else {
		// IVFFlat indexes
		ivfConfig := DefaultIVFFlatConfig()
		indexes = []string{
			fmt.Sprintf(`
				CREATE INDEX IF NOT EXISTS idx_embeddings_ivfflat
				ON %s.embeddings
				USING ivfflat (embedding vector_cosine_ops)
				WITH (lists = %d)
			`, schema, ivfConfig.Lists),
		}
	}

	// B-tree indexes for common queries
	btreeIndexes := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_embeddings_key ON %s.embeddings(key)", schema),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_embeddings_namespace ON %s.embeddings(namespace)", schema),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_embeddings_created ON %s.embeddings(created_at)", schema),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_graph_source ON %s.graph_edges(source_id)", schema),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_graph_target ON %s.graph_edges(target_id)", schema),
	}

	indexes = append(indexes, btreeIndexes...)

	for _, indexSQL := range indexes {
		if _, err := s.conn.Exec(ctx, indexSQL); err != nil {
			// Ignore "already exists" errors for indexes
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("failed to create index: %w", err)
			}
		}
	}

	return nil
}

// StoreMetadata stores initialization metadata.
func (s *SchemaManager) StoreMetadata(ctx context.Context) error {
	config := s.conn.Config()
	schema := config.Schema

	_, err := s.conn.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.metadata (key, value, updated_at)
		VALUES ('ruvector_init', $1::jsonb, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1::jsonb, updated_at = NOW()
	`, schema), fmt.Sprintf(`{
		"version": "1.0.0",
		"dimensions": %d,
		"index_type": "%s",
		"initialized_at": "%s"
	}`, config.Dimensions, config.IndexType, "now"))

	if err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}
	return nil
}

// GetMetadata retrieves initialization metadata.
func (s *SchemaManager) GetMetadata(ctx context.Context) (map[string]interface{}, error) {
	schema := s.conn.Config().Schema

	var value string
	err := s.conn.QueryRow(ctx, fmt.Sprintf(`
		SELECT value::text FROM %s.metadata WHERE key = 'ruvector_init'
	`, schema)).Scan(&value)

	if err != nil {
		return nil, err
	}

	// Parse JSON manually for simplicity
	result := make(map[string]interface{})
	result["raw"] = value
	return result, nil
}

// GetTableStats returns statistics for all tables in the schema.
func (s *SchemaManager) GetTableStats(ctx context.Context) ([]TableStats, error) {
	schema := s.conn.Config().Schema

	rows, err := s.conn.Query(ctx, `
		SELECT 
			t.tablename,
			COALESCE(s.n_live_tup, 0) as row_count,
			pg_size_pretty(pg_relation_size(quote_ident(t.schemaname) || '.' || quote_ident(t.tablename))) as size,
			pg_size_pretty(pg_total_relation_size(quote_ident(t.schemaname) || '.' || quote_ident(t.tablename))) as total_size,
			COALESCE(s.n_dead_tup, 0) as dead_tuples
		FROM pg_tables t
		LEFT JOIN pg_stat_user_tables s ON s.schemaname = t.schemaname AND s.relname = t.tablename
		WHERE t.schemaname = $1
		ORDER BY t.tablename
	`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []TableStats
	for rows.Next() {
		var s TableStats
		if err := rows.Scan(&s.Name, &s.RowCount, &s.Size, &s.TotalSize, &s.DeadTuples); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetIndexStats returns statistics for all indexes in the schema.
func (s *SchemaManager) GetIndexStats(ctx context.Context) ([]IndexStats, error) {
	schema := s.conn.Config().Schema

	rows, err := s.conn.Query(ctx, `
		SELECT 
			i.indexname,
			i.tablename,
			am.amname as index_type,
			pg_size_pretty(pg_relation_size(quote_ident(i.schemaname) || '.' || quote_ident(i.indexname))) as size,
			idx.indisvalid as is_valid,
			COALESCE(s.idx_scan, 0) as scan_count
		FROM pg_indexes i
		JOIN pg_class c ON c.relname = i.indexname
		JOIN pg_am am ON am.oid = c.relam
		JOIN pg_index idx ON idx.indexrelid = c.oid
		LEFT JOIN pg_stat_user_indexes s ON s.indexrelname = i.indexname
		WHERE i.schemaname = $1
		ORDER BY i.tablename, i.indexname
	`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []IndexStats
	for rows.Next() {
		var s IndexStats
		if err := rows.Scan(&s.Name, &s.Table, &s.Type, &s.Size, &s.IsValid, &s.ScanCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GenerateDockerCompose generates a docker-compose.yml for RuVector PostgreSQL.
func GenerateDockerCompose() string {
	return `version: '3.8'

services:
  ruvector-postgres:
    image: pgvector/pgvector:pg16
    container_name: ruvector-postgres
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: claude_flow
    ports:
      - "5432:5432"
    volumes:
      - ruvector_data:/var/lib/postgresql/data
      - ./scripts:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  ruvector_data:
`
}

// GenerateInitSQL generates the initialization SQL script.
func GenerateInitSQL(config RuVectorConfig) string {
	hnswConfig := DefaultHNSWConfig()

	return fmt.Sprintf(`-- RuVector PostgreSQL Initialization Script
-- Generated for claude-flow-go

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create schema
CREATE SCHEMA IF NOT EXISTS %s;

-- Main embeddings table
CREATE TABLE IF NOT EXISTS %s.embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key TEXT NOT NULL,
    namespace TEXT DEFAULT 'default',
    embedding vector(%d),
    content TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(key, namespace)
);

-- HNSW index for fast similarity search
CREATE INDEX IF NOT EXISTS idx_embeddings_hnsw
ON %s.embeddings
USING hnsw (embedding vector_cosine_ops)
WITH (m = %d, ef_construction = %d);

-- B-tree indexes
CREATE INDEX IF NOT EXISTS idx_embeddings_key ON %s.embeddings(key);
CREATE INDEX IF NOT EXISTS idx_embeddings_namespace ON %s.embeddings(namespace);

-- Migrations tracking
CREATE TABLE IF NOT EXISTS %s.migrations (
    version TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMPTZ DEFAULT NOW(),
    checksum TEXT
);

-- Metadata
CREATE TABLE IF NOT EXISTS %s.metadata (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Store initialization metadata
INSERT INTO %s.metadata (key, value)
VALUES ('ruvector_init', '{"version": "1.0.0", "dimensions": %d, "index_type": "hnsw"}'::jsonb)
ON CONFLICT (key) DO NOTHING;

-- Helper function: Cosine similarity search
CREATE OR REPLACE FUNCTION %s.search_similar(
    query_embedding vector(%d),
    match_count INT DEFAULT 10,
    match_namespace TEXT DEFAULT NULL
)
RETURNS TABLE (
    id UUID,
    key TEXT,
    content TEXT,
    similarity FLOAT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        e.id,
        e.key,
        e.content,
        1 - (e.embedding <=> query_embedding) as similarity
    FROM %s.embeddings e
    WHERE (match_namespace IS NULL OR e.namespace = match_namespace)
    ORDER BY e.embedding <=> query_embedding
    LIMIT match_count;
END;
$$ LANGUAGE plpgsql;
`,
		config.Schema,
		config.Schema, config.Dimensions,
		config.Schema, hnswConfig.M, hnswConfig.EfConstruction,
		config.Schema,
		config.Schema,
		config.Schema,
		config.Schema,
		config.Schema, config.Dimensions,
		config.Schema, config.Dimensions,
		config.Schema,
	)
}

// GenerateReadme generates a README for the setup.
func GenerateReadme() string {
	return `# RuVector PostgreSQL Setup

## Quick Start

1. Start PostgreSQL with pgvector:
   ` + "```bash" + `
   docker-compose up -d
   ` + "```" + `

2. Initialize the RuVector schema:
   ` + "```bash" + `
   claude-flow ruvector init --database claude_flow
   ` + "```" + `

3. Check status:
   ` + "```bash" + `
   claude-flow ruvector status --database claude_flow
   ` + "```" + `

## Connection Details

- Host: localhost
- Port: 5432
- User: postgres
- Password: postgres
- Database: claude_flow

## Environment Variables

` + "```bash" + `
export PGHOST=localhost
export PGPORT=5432
export PGUSER=postgres
export PGPASSWORD=postgres
export PGDATABASE=claude_flow
` + "```" + `
`
}
