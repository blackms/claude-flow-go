// Package ruvector provides PostgreSQL/pgvector integration for vector operations.
package ruvector

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// MigrationManager handles database migrations.
type MigrationManager struct {
	conn *Connection
}

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(conn *Connection) *MigrationManager {
	return &MigrationManager{conn: conn}
}

// AvailableMigrations returns all available migrations in order.
func AvailableMigrations(schema string, dimensions int) []Migration {
	return []Migration{
		{
			Version: "1.0.0",
			Name:    "Initial RuVector setup",
			SQL:     "", // Handled by init command
		},
		{
			Version: "1.1.0",
			Name:    "Add full-text search",
			SQL: fmt.Sprintf(`
				-- Add GIN index for full-text search on content
				CREATE INDEX IF NOT EXISTS idx_embeddings_content_fts
				ON %s.embeddings
				USING gin(to_tsvector('english', content));
			`, schema),
			Rollback: fmt.Sprintf(`
				DROP INDEX IF EXISTS %s.idx_embeddings_content_fts;
			`, schema),
		},
		{
			Version: "1.2.0",
			Name:    "Add embedding statistics",
			SQL: fmt.Sprintf(`
				-- Statistics table for embeddings
				CREATE TABLE IF NOT EXISTS %s.embedding_stats (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					namespace TEXT NOT NULL,
					total_count INT DEFAULT 0,
					avg_magnitude FLOAT,
					min_magnitude FLOAT,
					max_magnitude FLOAT,
					updated_at TIMESTAMPTZ DEFAULT NOW()
				);
				
				-- Function to update statistics
				CREATE OR REPLACE FUNCTION %s.update_embedding_stats()
				RETURNS TRIGGER AS $$
				BEGIN
					INSERT INTO %s.embedding_stats (namespace, total_count, updated_at)
					VALUES (NEW.namespace, 1, NOW())
					ON CONFLICT (namespace) DO UPDATE SET
						total_count = %s.embedding_stats.total_count + 1,
						updated_at = NOW();
					RETURN NEW;
				END;
				$$ LANGUAGE plpgsql;
			`, schema, schema, schema, schema),
			Rollback: fmt.Sprintf(`
				DROP FUNCTION IF EXISTS %s.update_embedding_stats();
				DROP TABLE IF EXISTS %s.embedding_stats;
			`, schema, schema),
		},
		{
			Version: "1.3.0",
			Name:    "Add query cache",
			SQL: fmt.Sprintf(`
				-- Query cache for frequent searches
				CREATE TABLE IF NOT EXISTS %s.query_cache (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					query_hash TEXT UNIQUE NOT NULL,
					query_embedding vector(%d),
					results JSONB,
					hit_count INT DEFAULT 1,
					created_at TIMESTAMPTZ DEFAULT NOW(),
					expires_at TIMESTAMPTZ DEFAULT NOW() + INTERVAL '1 hour'
				);
				
				-- Index for cache lookup
				CREATE INDEX IF NOT EXISTS idx_query_cache_hash
				ON %s.query_cache(query_hash);
				
				-- Index for cache expiration
				CREATE INDEX IF NOT EXISTS idx_query_cache_expires
				ON %s.query_cache(expires_at);
			`, schema, dimensions, schema, schema),
			Rollback: fmt.Sprintf(`
				DROP TABLE IF EXISTS %s.query_cache;
			`, schema),
		},
		{
			Version: "1.4.0",
			Name:    "Add batch operations",
			SQL: fmt.Sprintf(`
				-- Batch import staging table
				CREATE TABLE IF NOT EXISTS %s.import_staging (
					id SERIAL PRIMARY KEY,
					batch_id UUID NOT NULL,
					key TEXT NOT NULL,
					namespace TEXT DEFAULT 'default',
					embedding vector(%d),
					content TEXT,
					metadata JSONB DEFAULT '{}',
					status TEXT DEFAULT 'pending',
					error_message TEXT,
					created_at TIMESTAMPTZ DEFAULT NOW()
				);
				
				-- Index for batch processing
				CREATE INDEX IF NOT EXISTS idx_staging_batch
				ON %s.import_staging(batch_id, status);
			`, schema, dimensions, schema),
			Rollback: fmt.Sprintf(`
				DROP TABLE IF EXISTS %s.import_staging;
			`, schema),
		},
		{
			Version: "1.5.0",
			Name:    "Add neural pattern learning",
			SQL: fmt.Sprintf(`
				-- Neural patterns table
				CREATE TABLE IF NOT EXISTS %s.neural_patterns (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					pattern_type TEXT NOT NULL,
					embedding vector(%d),
					content TEXT,
					confidence FLOAT DEFAULT 0.5,
					usage_count INT DEFAULT 0,
					created_at TIMESTAMPTZ DEFAULT NOW(),
					last_used_at TIMESTAMPTZ DEFAULT NOW()
				);
				
				-- HNSW index for neural patterns
				CREATE INDEX IF NOT EXISTS idx_neural_patterns_hnsw
				ON %s.neural_patterns
				USING hnsw (embedding vector_cosine_ops)
				WITH (m = 16, ef_construction = 64);
				
				-- Index for pattern type lookup
				CREATE INDEX IF NOT EXISTS idx_neural_patterns_type
				ON %s.neural_patterns(pattern_type);
			`, schema, dimensions, schema, schema),
			Rollback: fmt.Sprintf(`
				DROP TABLE IF EXISTS %s.neural_patterns;
			`, schema),
		},
	}
}

// GetAppliedMigrations returns all migrations that have been applied.
func (m *MigrationManager) GetAppliedMigrations(ctx context.Context) ([]Migration, error) {
	schema := m.conn.Config().Schema

	rows, err := m.conn.Query(ctx, fmt.Sprintf(`
		SELECT version, name, applied_at, COALESCE(checksum, '')
		FROM %s.migrations
		ORDER BY version
	`, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var migrations []Migration
	for rows.Next() {
		var mig Migration
		if err := rows.Scan(&mig.Version, &mig.Name, &mig.AppliedAt, &mig.Checksum); err != nil {
			return nil, err
		}
		migrations = append(migrations, mig)
	}

	return migrations, rows.Err()
}

// GetMigrationStatus returns the current migration status.
func (m *MigrationManager) GetMigrationStatus(ctx context.Context) (*MigrationStatus, error) {
	config := m.conn.Config()
	available := AvailableMigrations(config.Schema, config.Dimensions)
	applied, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		// If migrations table doesn't exist, return all as pending
		if err == sql.ErrNoRows {
			return &MigrationStatus{
				CurrentVersion: "",
				Applied:        nil,
				Pending:        available,
			}, nil
		}
		return nil, err
	}

	// Build map of applied versions
	appliedMap := make(map[string]bool)
	var currentVersion string
	for _, mig := range applied {
		appliedMap[mig.Version] = true
		currentVersion = mig.Version
	}

	// Find pending migrations
	var pending []Migration
	for _, mig := range available {
		if !appliedMap[mig.Version] {
			pending = append(pending, mig)
		}
	}

	return &MigrationStatus{
		CurrentVersion: currentVersion,
		Applied:        applied,
		Pending:        pending,
	}, nil
}

// RunMigration runs a single migration.
func (m *MigrationManager) RunMigration(ctx context.Context, mig Migration) error {
	schema := m.conn.Config().Schema

	// Skip migrations with no SQL (like initial setup)
	if mig.SQL == "" {
		// Just record it as applied
		return m.RecordMigration(ctx, mig)
	}

	tx, err := m.conn.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration SQL
	if _, err := tx.ExecContext(ctx, mig.SQL); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Record migration
	checksum := computeChecksum(mig.SQL)
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s.migrations (version, name, applied_at, checksum)
		VALUES ($1, $2, $3, $4)
	`, schema), mig.Version, mig.Name, time.Now(), checksum)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

// RollbackMigration rolls back a single migration.
func (m *MigrationManager) RollbackMigration(ctx context.Context, mig Migration) error {
	schema := m.conn.Config().Schema

	if mig.Rollback == "" {
		return fmt.Errorf("migration %s has no rollback SQL", mig.Version)
	}

	tx, err := m.conn.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute rollback SQL
	if _, err := tx.ExecContext(ctx, mig.Rollback); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	// Remove migration record
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s.migrations WHERE version = $1
	`, schema), mig.Version)
	if err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	return tx.Commit()
}

// RecordMigration records a migration as applied without running it.
func (m *MigrationManager) RecordMigration(ctx context.Context, mig Migration) error {
	schema := m.conn.Config().Schema
	checksum := computeChecksum(mig.SQL)

	_, err := m.conn.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.migrations (version, name, applied_at, checksum)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (version) DO NOTHING
	`, schema), mig.Version, mig.Name, time.Now(), checksum)

	return err
}

// RunPendingMigrations runs all pending migrations.
func (m *MigrationManager) RunPendingMigrations(ctx context.Context, dryRun bool) ([]Migration, error) {
	status, err := m.GetMigrationStatus(ctx)
	if err != nil {
		return nil, err
	}

	if len(status.Pending) == 0 {
		return nil, nil
	}

	var applied []Migration
	for _, mig := range status.Pending {
		if dryRun {
			applied = append(applied, mig)
			continue
		}

		if err := m.RunMigration(ctx, mig); err != nil {
			return applied, fmt.Errorf("migration %s failed: %w", mig.Version, err)
		}
		applied = append(applied, mig)
	}

	return applied, nil
}

// RollbackLastMigration rolls back the most recently applied migration.
func (m *MigrationManager) RollbackLastMigration(ctx context.Context) (*Migration, error) {
	status, err := m.GetMigrationStatus(ctx)
	if err != nil {
		return nil, err
	}

	if len(status.Applied) == 0 {
		return nil, fmt.Errorf("no migrations to rollback")
	}

	// Get the last applied migration
	lastApplied := status.Applied[len(status.Applied)-1]

	// Find the migration definition to get rollback SQL
	config := m.conn.Config()
	available := AvailableMigrations(config.Schema, config.Dimensions)
	var migToRollback *Migration
	for _, mig := range available {
		if mig.Version == lastApplied.Version {
			migToRollback = &mig
			break
		}
	}

	if migToRollback == nil {
		return nil, fmt.Errorf("migration definition not found: %s", lastApplied.Version)
	}

	if err := m.RollbackMigration(ctx, *migToRollback); err != nil {
		return nil, err
	}

	return migToRollback, nil
}

// MigrateToVersion migrates to a specific version.
func (m *MigrationManager) MigrateToVersion(ctx context.Context, targetVersion string, dryRun bool) ([]Migration, error) {
	status, err := m.GetMigrationStatus(ctx)
	if err != nil {
		return nil, err
	}

	// Determine direction
	if status.CurrentVersion < targetVersion {
		// Migrate up
		var toApply []Migration
		for _, mig := range status.Pending {
			if mig.Version <= targetVersion {
				toApply = append(toApply, mig)
			}
		}

		var applied []Migration
		for _, mig := range toApply {
			if !dryRun {
				if err := m.RunMigration(ctx, mig); err != nil {
					return applied, err
				}
			}
			applied = append(applied, mig)
		}
		return applied, nil
	} else if status.CurrentVersion > targetVersion {
		// Rollback
		var toRollback []Migration
		for i := len(status.Applied) - 1; i >= 0; i-- {
			if status.Applied[i].Version > targetVersion {
				toRollback = append(toRollback, status.Applied[i])
			}
		}

		config := m.conn.Config()
		available := AvailableMigrations(config.Schema, config.Dimensions)
		availableMap := make(map[string]Migration)
		for _, mig := range available {
			availableMap[mig.Version] = mig
		}

		var rolledBack []Migration
		for _, mig := range toRollback {
			def := availableMap[mig.Version]
			if !dryRun {
				if err := m.RollbackMigration(ctx, def); err != nil {
					return rolledBack, err
				}
			}
			rolledBack = append(rolledBack, def)
		}
		return rolledBack, nil
	}

	return nil, nil // Already at target version
}

// computeChecksum computes SHA256 checksum of SQL.
func computeChecksum(sql string) string {
	hash := sha256.Sum256([]byte(sql))
	return hex.EncodeToString(hash[:8])
}
