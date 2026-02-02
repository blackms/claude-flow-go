// Package ruvector provides RuVector application services.
package ruvector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	infraRuvector "github.com/anthropics/claude-flow-go/internal/infrastructure/ruvector"
)

// Service orchestrates RuVector operations.
type Service struct {
	backend    *infraRuvector.Backend
	schema     *infraRuvector.SchemaManager
	migrations *infraRuvector.MigrationManager
	config     infraRuvector.RuVectorConfig
}

// NewService creates a new RuVector service.
func NewService(config infraRuvector.RuVectorConfig) (*Service, error) {
	backend, err := infraRuvector.NewBackend(config)
	if err != nil {
		return nil, err
	}

	return &Service{
		backend:    backend,
		schema:     infraRuvector.NewSchemaManager(backend.Connection()),
		migrations: infraRuvector.NewMigrationManager(backend.Connection()),
		config:     config,
	}, nil
}

// Connect establishes connection to PostgreSQL.
func (s *Service) Connect(ctx context.Context) error {
	return s.backend.Initialize()
}

// Close closes the connection.
func (s *Service) Close() error {
	return s.backend.Close()
}

// Initialize initializes the RuVector schema and tables.
func (s *Service) Initialize(ctx context.Context, force bool) error {
	// Check if already initialized
	exists, err := s.backend.Connection().SchemaExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check schema: %w", err)
	}

	if exists && !force {
		return fmt.Errorf("schema %s already exists. Use --force to reinitialize", s.config.Schema)
	}

	if exists && force {
		if err := s.schema.DropSchema(ctx); err != nil {
			return fmt.Errorf("failed to drop schema: %w", err)
		}
	}

	// Create pgvector extension
	if err := s.schema.CreateExtension(ctx); err != nil {
		return err
	}

	// Create schema
	if err := s.schema.CreateSchema(ctx); err != nil {
		return err
	}

	// Create tables
	if err := s.schema.CreateTables(ctx); err != nil {
		return err
	}

	// Create indexes
	if err := s.schema.CreateIndexes(ctx); err != nil {
		return err
	}

	// Store metadata
	if err := s.schema.StoreMetadata(ctx); err != nil {
		return err
	}

	// Record initial migration
	initialMig := infraRuvector.Migration{
		Version: "1.0.0",
		Name:    "Initial RuVector setup",
	}
	if err := s.migrations.RecordMigration(ctx, initialMig); err != nil {
		return err
	}

	return nil
}

// GetStatus returns the current system status.
func (s *Service) GetStatus(ctx context.Context, verbose bool) (*infraRuvector.SystemStatus, error) {
	status := &infraRuvector.SystemStatus{}

	// Check connection
	latency, err := s.backend.Connection().Ping(ctx)
	if err != nil {
		status.Connected = false
		return status, nil
	}
	status.Connected = true
	status.LatencyMs = float64(latency.Milliseconds())

	// Get PostgreSQL version
	if version, err := s.backend.Connection().GetPostgresVersion(ctx); err == nil {
		status.PostgresVersion = version
	}

	// Get pgvector version
	if version, err := s.backend.Connection().GetPgVectorVersion(ctx); err == nil {
		status.PgVectorVersion = version
	}

	// Check schema
	if exists, err := s.backend.Connection().SchemaExists(ctx); err == nil {
		status.SchemaExists = exists
	}

	if status.SchemaExists {
		// Get metadata
		if meta, err := s.schema.GetMetadata(ctx); err == nil {
			status.Initialized = true
			if raw, ok := meta["raw"].(string); ok {
				status.Version = raw
			}
		}

		// Get table stats
		if tables, err := s.schema.GetTableStats(ctx); err == nil {
			status.Tables = tables
		}

		if verbose {
			// Get index stats
			if indexes, err := s.schema.GetIndexStats(ctx); err == nil {
				status.Indexes = indexes
			}

			// Get migration status
			if migStatus, err := s.migrations.GetMigrationStatus(ctx); err == nil {
				status.Migrations = migStatus.Applied
			}
		}
	}

	status.Dimensions = s.config.Dimensions

	return status, nil
}

// RunMigrations runs pending migrations.
func (s *Service) RunMigrations(ctx context.Context, dryRun bool) ([]infraRuvector.Migration, error) {
	return s.migrations.RunPendingMigrations(ctx, dryRun)
}

// RollbackMigration rolls back the last migration.
func (s *Service) RollbackMigration(ctx context.Context) (*infraRuvector.Migration, error) {
	return s.migrations.RollbackLastMigration(ctx)
}

// MigrateToVersion migrates to a specific version.
func (s *Service) MigrateToVersion(ctx context.Context, version string, dryRun bool) ([]infraRuvector.Migration, error) {
	return s.migrations.MigrateToVersion(ctx, version, dryRun)
}

// GetMigrationStatus returns the current migration status.
func (s *Service) GetMigrationStatus(ctx context.Context) (*infraRuvector.MigrationStatus, error) {
	return s.migrations.GetMigrationStatus(ctx)
}

// Optimize analyzes and optimizes the database.
func (s *Service) Optimize(ctx context.Context, vacuum, reindex bool) (*infraRuvector.OptimizationReport, error) {
	report := &infraRuvector.OptimizationReport{
		AnalyzedAt:      time.Now(),
		Recommendations: make([]infraRuvector.OptimizationRecommendation, 0),
		TableBloat:      make(map[string]float64),
		IndexHealth:     make(map[string]bool),
	}

	// Get table stats for bloat analysis
	tables, err := s.schema.GetTableStats(ctx)
	if err != nil {
		return nil, err
	}

	for _, table := range tables {
		// Calculate bloat ratio
		if table.RowCount > 0 {
			bloatRatio := float64(table.DeadTuples) / float64(table.RowCount)
			report.TableBloat[table.Name] = bloatRatio

			if bloatRatio > 0.3 {
				report.Recommendations = append(report.Recommendations, infraRuvector.OptimizationRecommendation{
					Priority:    "critical",
					Category:    "bloat",
					Description: fmt.Sprintf("Table %s has %.1f%% dead tuples", table.Name, bloatRatio*100),
					SQL:         fmt.Sprintf("VACUUM FULL %s.%s", s.config.Schema, table.Name),
				})
			} else if bloatRatio > 0.1 {
				report.Recommendations = append(report.Recommendations, infraRuvector.OptimizationRecommendation{
					Priority:    "high",
					Category:    "bloat",
					Description: fmt.Sprintf("Table %s has %.1f%% dead tuples", table.Name, bloatRatio*100),
					SQL:         fmt.Sprintf("VACUUM ANALYZE %s.%s", s.config.Schema, table.Name),
				})
			}
		}
	}

	// Get index stats
	indexes, err := s.schema.GetIndexStats(ctx)
	if err != nil {
		return nil, err
	}

	for _, idx := range indexes {
		report.IndexHealth[idx.Name] = idx.IsValid

		if !idx.IsValid {
			report.Recommendations = append(report.Recommendations, infraRuvector.OptimizationRecommendation{
				Priority:    "critical",
				Category:    "index",
				Description: fmt.Sprintf("Index %s is invalid", idx.Name),
				SQL:         fmt.Sprintf("REINDEX INDEX %s.%s", s.config.Schema, idx.Name),
			})
		}

		if idx.ScanCount == 0 {
			report.Recommendations = append(report.Recommendations, infraRuvector.OptimizationRecommendation{
				Priority:    "low",
				Category:    "index",
				Description: fmt.Sprintf("Index %s has never been used", idx.Name),
			})
		}
	}

	// Run vacuum if requested
	if vacuum {
		_, err := s.backend.Connection().Exec(ctx, fmt.Sprintf("VACUUM ANALYZE %s.embeddings", s.config.Schema))
		if err != nil {
			return report, fmt.Errorf("vacuum failed: %w", err)
		}
	}

	// Run reindex if requested
	if reindex {
		_, err := s.backend.Connection().Exec(ctx, fmt.Sprintf("REINDEX SCHEMA %s", s.config.Schema))
		if err != nil {
			return report, fmt.Errorf("reindex failed: %w", err)
		}
	}

	return report, nil
}

// GenerateSetupFiles generates Docker and SQL setup files.
func (s *Service) GenerateSetupFiles(outputDir string, printOnly bool) error {
	if printOnly {
		fmt.Println("=== docker-compose.yml ===")
		fmt.Println(infraRuvector.GenerateDockerCompose())
		fmt.Println("\n=== init-db.sql ===")
		fmt.Println(infraRuvector.GenerateInitSQL(s.config))
		fmt.Println("\n=== README.md ===")
		fmt.Println(infraRuvector.GenerateReadme())
		return nil
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create scripts directory
	scriptsDir := filepath.Join(outputDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return fmt.Errorf("failed to create scripts directory: %w", err)
	}

	// Write docker-compose.yml
	if err := os.WriteFile(
		filepath.Join(outputDir, "docker-compose.yml"),
		[]byte(infraRuvector.GenerateDockerCompose()),
		0644,
	); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	// Write init-db.sql
	if err := os.WriteFile(
		filepath.Join(scriptsDir, "init-db.sql"),
		[]byte(infraRuvector.GenerateInitSQL(s.config)),
		0644,
	); err != nil {
		return fmt.Errorf("failed to write init-db.sql: %w", err)
	}

	// Write README.md
	if err := os.WriteFile(
		filepath.Join(outputDir, "README.md"),
		[]byte(infraRuvector.GenerateReadme()),
		0644,
	); err != nil {
		return fmt.Errorf("failed to write README.md: %w", err)
	}

	return nil
}

// Backend returns the underlying backend.
func (s *Service) Backend() *infraRuvector.Backend {
	return s.backend
}

// Config returns the configuration.
func (s *Service) Config() infraRuvector.RuVectorConfig {
	return s.config
}
