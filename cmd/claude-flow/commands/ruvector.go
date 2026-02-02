// Package commands provides CLI command implementations.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	appRuvector "github.com/anthropics/claude-flow-go/internal/application/ruvector"
	infraRuvector "github.com/anthropics/claude-flow-go/internal/infrastructure/ruvector"
)

// Global connection flags
var (
	rvHost     string
	rvPort     int
	rvDatabase string
	rvUser     string
	rvPassword string
	rvSchema   string
	rvSSL      bool
)

// Init command flags
var (
	initForce      bool
	initDimensions int
	initIndexType  string
)

// Setup command flags
var (
	setupOutput string
	setupPrint  bool
	setupForce  bool
)

// Migrate command flags
var (
	migrateUp     bool
	migrateDown   bool
	migrateTo     string
	migrateDryRun bool
)

// Optimize command flags
var (
	optimizeAnalyze bool
	optimizeApply   bool
	optimizeVacuum  bool
	optimizeReindex bool
)

// Import command flags
var (
	importInput     string
	importFromSQLite string
	importBatchSize int
	importVerbose   bool
)

// Benchmark command flags
var (
	benchVectors    int
	benchDimensions int
	benchQueries    int
	benchK          int
	benchMetric     string
	benchIndex      string
	benchCleanup    bool
)

// Backup command flags
var (
	backupOutput   string
	backupTables   string
	backupFormat   string
	backupCompress bool
	restoreInput   string
	restoreClean   bool
	restoreDryRun  bool
)

// Status command flags
var (
	statusVerboseRV bool
	statusJSON      bool
)

// RuVectorCmd is the parent command for all ruvector subcommands.
var RuVectorCmd = &cobra.Command{
	Use:     "ruvector",
	Aliases: []string{"rv", "pgvector"},
	Short:   "RuVector PostgreSQL bridge for vector operations",
	Long: `Commands for managing PostgreSQL/pgvector vector storage.

RuVector provides:
  - PostgreSQL integration with pgvector extension
  - HNSW and IVFFlat indexing for fast similarity search
  - Data migration from SQLite
  - Performance benchmarking and optimization`,
}

// initCmd initializes RuVector in PostgreSQL
var rvInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize RuVector in PostgreSQL",
	Long:  `Initialize the RuVector schema, tables, and indexes in PostgreSQL.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rvDatabase == "" {
			return fmt.Errorf("database name is required (--database)")
		}

		config := buildConfig()
		config.Dimensions = initDimensions
		config.IndexType = initIndexType

		service, err := appRuvector.NewService(config)
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		ctx := context.Background()
		if err := service.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		fmt.Printf("Initializing RuVector in %s/%s...\n", rvDatabase, rvSchema)

		if err := service.Initialize(ctx, initForce); err != nil {
			return err
		}

		fmt.Println("\nRuVector initialized successfully!")
		fmt.Printf("  Schema:     %s\n", rvSchema)
		fmt.Printf("  Dimensions: %d\n", initDimensions)
		fmt.Printf("  Index type: %s\n", initIndexType)

		return nil
	},
}

// setupCmd generates Docker and SQL setup files
var rvSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate Docker and SQL setup files",
	Long:  `Generate docker-compose.yml, init SQL, and README for RuVector PostgreSQL setup.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config := buildConfig()
		config.Dimensions = initDimensions

		service, err := appRuvector.NewService(config)
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}

		if setupPrint {
			return service.GenerateSetupFiles("", true)
		}

		// Check if directory exists
		if !setupForce {
			if _, err := os.Stat(setupOutput); err == nil {
				return fmt.Errorf("output directory %s already exists. Use --force to overwrite", setupOutput)
			}
		}

		if err := service.GenerateSetupFiles(setupOutput, false); err != nil {
			return err
		}

		fmt.Printf("Setup files generated in %s/\n", setupOutput)
		fmt.Println("\nTo start PostgreSQL:")
		fmt.Printf("  cd %s && docker-compose up -d\n", setupOutput)

		return nil
	},
}

// migrateCmd manages database migrations
var rvMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Run pending migrations or rollback to a previous version.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rvDatabase == "" {
			return fmt.Errorf("database name is required (--database)")
		}

		service, err := appRuvector.NewService(buildConfig())
		if err != nil {
			return err
		}
		defer service.Close()

		ctx := context.Background()
		if err := service.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		if migrateDown {
			mig, err := service.RollbackMigration(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("Rolled back migration: %s - %s\n", mig.Version, mig.Name)
			return nil
		}

		if migrateTo != "" {
			migrations, err := service.MigrateToVersion(ctx, migrateTo, migrateDryRun)
			if err != nil {
				return err
			}
			action := "Applied"
			if migrateDryRun {
				action = "Would apply"
			}
			for _, m := range migrations {
				fmt.Printf("%s: %s - %s\n", action, m.Version, m.Name)
			}
			return nil
		}

		// Default: run pending migrations
		migrations, err := service.RunMigrations(ctx, migrateDryRun)
		if err != nil {
			return err
		}

		if len(migrations) == 0 {
			fmt.Println("No pending migrations")
			return nil
		}

		action := "Applied"
		if migrateDryRun {
			action = "Would apply"
		}
		for _, m := range migrations {
			fmt.Printf("%s: %s - %s\n", action, m.Version, m.Name)
		}

		return nil
	},
}

// optimizeCmd analyzes and optimizes the database
var rvOptimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Optimize vector indexes and tables",
	Long:  `Analyze database performance and apply optimizations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rvDatabase == "" {
			return fmt.Errorf("database name is required (--database)")
		}

		service, err := appRuvector.NewService(buildConfig())
		if err != nil {
			return err
		}
		defer service.Close()

		ctx := context.Background()
		if err := service.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		fmt.Println("Analyzing database...")

		report, err := service.Optimize(ctx, optimizeVacuum, optimizeReindex)
		if err != nil {
			return err
		}

		if len(report.Recommendations) == 0 {
			fmt.Println("\nNo optimization recommendations. Database is healthy!")
			return nil
		}

		fmt.Printf("\nOptimization Recommendations (%d found):\n", len(report.Recommendations))
		fmt.Println(strings.Repeat("-", 60))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PRIORITY\tCATEGORY\tDESCRIPTION")
		for _, rec := range report.Recommendations {
			fmt.Fprintf(w, "%s\t%s\t%s\n", rec.Priority, rec.Category, rec.Description)
		}
		w.Flush()

		if optimizeVacuum {
			fmt.Println("\nVACUUM ANALYZE completed.")
		}
		if optimizeReindex {
			fmt.Println("\nREINDEX completed.")
		}

		return nil
	},
}

// importCmd imports embeddings from JSON/SQLite
var rvImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import embeddings from JSON or SQLite",
	Long:  `Import vector embeddings from JSON files or SQLite databases.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rvDatabase == "" {
			return fmt.Errorf("database name is required (--database)")
		}
		if importInput == "" && importFromSQLite == "" {
			return fmt.Errorf("input source required (--input or --from-sqlite)")
		}

		service, err := appRuvector.NewService(buildConfig())
		if err != nil {
			return err
		}
		defer service.Close()

		ctx := context.Background()
		if err := service.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		importer := appRuvector.NewImportManager(service)

		config := infraRuvector.ImportConfig{
			InputPath:  importInput,
			FromSQLite: importFromSQLite,
			BatchSize:  importBatchSize,
			Verbose:    importVerbose,
		}

		var stats *infraRuvector.ImportStats

		if importFromSQLite != "" {
			fmt.Printf("Importing from SQLite: %s\n", importFromSQLite)
			stats, err = importer.ImportFromSQLite(ctx, config)
		} else {
			fmt.Printf("Importing from JSON: %s\n", importInput)
			stats, err = importer.ImportFromJSON(ctx, config)
		}

		if err != nil {
			return err
		}

		fmt.Println("\nImport Complete!")
		fmt.Printf("  Total:           %d\n", stats.Total)
		fmt.Printf("  Imported:        %d\n", stats.Imported)
		fmt.Printf("  With embeddings: %d\n", stats.WithEmbeddings)
		fmt.Printf("  Errors:          %d\n", stats.Errors)
		fmt.Printf("  Duration:        %dms\n", stats.DurationMs)

		if len(stats.ByNamespace) > 0 {
			fmt.Println("\nBy Namespace:")
			for ns, count := range stats.ByNamespace {
				fmt.Printf("  %s: %d\n", ns, count)
			}
		}

		return nil
	},
}

// benchmarkCmd runs performance benchmarks
var rvBenchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Benchmark vector search performance",
	Long:  `Run performance benchmarks on vector insert and search operations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rvDatabase == "" {
			return fmt.Errorf("database name is required (--database)")
		}

		service, err := appRuvector.NewService(buildConfig())
		if err != nil {
			return err
		}
		defer service.Close()

		ctx := context.Background()
		if err := service.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		config := infraRuvector.BenchmarkConfig{
			VectorCount: benchVectors,
			Dimensions:  benchDimensions,
			QueryCount:  benchQueries,
			K:           benchK,
			Metric:      benchMetric,
			IndexType:   benchIndex,
			Cleanup:     benchCleanup,
		}

		fmt.Printf("Running benchmark (vectors: %d, dim: %d, index: %s)...\n\n",
			config.VectorCount, config.Dimensions, config.IndexType)

		runner := appRuvector.NewBenchmarkRunner(service)
		result, err := runner.Run(ctx, config)
		if err != nil {
			return err
		}

		fmt.Println("Benchmark Results")
		fmt.Println(strings.Repeat("-", 50))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "METRIC\tVALUE")
		fmt.Fprintf(w, "Vectors\t%d\n", result.VectorCount)
		fmt.Fprintf(w, "Dimensions\t%d\n", result.Dimensions)
		fmt.Fprintf(w, "Index Type\t%s\n", result.IndexType)
		fmt.Fprintf(w, "Insert Throughput\t%.0f vectors/sec\n", result.InsertThroughput)
		fmt.Fprintf(w, "Index Build Time\t%.0f ms\n", result.IndexBuildTimeMs)
		fmt.Fprintf(w, "Query Latency (avg)\t%.2f μs\n", result.QueryLatencyAvg)
		fmt.Fprintf(w, "Query Latency (p50)\t%.2f μs\n", result.QueryLatencyP50)
		fmt.Fprintf(w, "Query Latency (p95)\t%.2f μs\n", result.QueryLatencyP95)
		fmt.Fprintf(w, "Query Latency (p99)\t%.2f μs\n", result.QueryLatencyP99)
		fmt.Fprintf(w, "QPS\t%.0f\n", result.QPS)
		fmt.Fprintf(w, "Recall Estimate\t%.0f%%\n", result.RecallEstimate*100)
		fmt.Fprintf(w, "Table Size\t%d bytes\n", result.TableSizeBytes)
		fmt.Fprintf(w, "Index Size\t%d bytes\n", result.IndexSizeBytes)
		fmt.Fprintf(w, "Bytes per Vector\t%.1f\n", result.BytesPerVector)
		w.Flush()

		return nil
	},
}

// backupCmd handles backup operations
var rvBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup and restore vector data",
	Long:  `Create backups or restore from backup files.`,
}

var rvBackupCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		if rvDatabase == "" {
			return fmt.Errorf("database name is required (--database)")
		}
		if backupOutput == "" {
			return fmt.Errorf("output path is required (--output)")
		}

		service, err := appRuvector.NewService(buildConfig())
		if err != nil {
			return err
		}
		defer service.Close()

		ctx := context.Background()
		if err := service.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		var tables []string
		if backupTables != "" {
			tables = strings.Split(backupTables, ",")
		}

		config := infraRuvector.BackupConfig{
			OutputPath: backupOutput,
			Tables:     tables,
			Format:     backupFormat,
			Compress:   backupCompress,
		}

		backup := appRuvector.NewBackupManager(service)
		if err := backup.CreateBackup(ctx, config); err != nil {
			return err
		}

		fmt.Printf("Backup created: %s\n", backupOutput)
		return nil
	},
}

var rvBackupRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore from a backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		if rvDatabase == "" {
			return fmt.Errorf("database name is required (--database)")
		}
		if restoreInput == "" {
			return fmt.Errorf("input path is required (--input)")
		}

		service, err := appRuvector.NewService(buildConfig())
		if err != nil {
			return err
		}
		defer service.Close()

		ctx := context.Background()
		if err := service.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		config := infraRuvector.RestoreConfig{
			InputPath: restoreInput,
			Clean:     restoreClean,
			DryRun:    restoreDryRun,
		}

		backup := appRuvector.NewBackupManager(service)
		count, err := backup.RestoreBackup(ctx, config)
		if err != nil {
			return err
		}

		action := "Restored"
		if restoreDryRun {
			action = "Would restore"
		}
		fmt.Printf("%s %d records\n", action, count)

		return nil
	},
}

// statusCmd shows system status
var rvStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show RuVector system status",
	Long:  `Show connection status, schema information, and statistics.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rvDatabase == "" {
			return fmt.Errorf("database name is required (--database)")
		}

		service, err := appRuvector.NewService(buildConfig())
		if err != nil {
			return err
		}
		defer service.Close()

		ctx := context.Background()
		if err := service.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		status, err := service.GetStatus(ctx, statusVerboseRV)
		if err != nil {
			return err
		}

		if statusJSON {
			output, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		fmt.Println("RuVector Status")
		fmt.Println(strings.Repeat("-", 40))

		fmt.Printf("Connected:        %v\n", status.Connected)
		fmt.Printf("Latency:          %.2f ms\n", status.LatencyMs)
		fmt.Printf("PostgreSQL:       %s\n", truncateVersion(status.PostgresVersion))
		if status.PgVectorVersion != "" {
			fmt.Printf("pgvector:         v%s\n", status.PgVectorVersion)
		} else {
			fmt.Printf("pgvector:         not installed\n")
		}
		fmt.Printf("Schema exists:    %v\n", status.SchemaExists)
		fmt.Printf("Initialized:      %v\n", status.Initialized)
		fmt.Printf("Dimensions:       %d\n", status.Dimensions)

		if len(status.Tables) > 0 {
			fmt.Println("\nTables:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tROWS\tSIZE")
			for _, t := range status.Tables {
				fmt.Fprintf(w, "  %s\t%d\t%s\n", t.Name, t.RowCount, t.Size)
			}
			w.Flush()
		}

		if statusVerboseRV && len(status.Indexes) > 0 {
			fmt.Println("\nIndexes:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tTYPE\tSIZE\tVALID")
			for _, idx := range status.Indexes {
				valid := "yes"
				if !idx.IsValid {
					valid = "NO"
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", idx.Name, idx.Type, idx.Size, valid)
			}
			w.Flush()
		}

		if statusVerboseRV && len(status.Migrations) > 0 {
			fmt.Println("\nMigrations:")
			for _, m := range status.Migrations {
				fmt.Printf("  %s: %s (%s)\n", m.Version, m.Name, m.AppliedAt.Format(time.RFC3339))
			}
		}

		return nil
	},
}

// buildConfig creates a RuVectorConfig from flags
func buildConfig() infraRuvector.RuVectorConfig {
	config := infraRuvector.DefaultConfig()
	config.Host = rvHost
	config.Port = rvPort
	config.Database = rvDatabase
	config.User = rvUser
	config.Password = rvPassword
	config.Schema = rvSchema
	config.SSL = rvSSL
	return config
}

// truncateVersion shortens PostgreSQL version string
func truncateVersion(version string) string {
	if len(version) > 50 {
		return version[:50] + "..."
	}
	return version
}

func init() {
	// Global connection flags
	RuVectorCmd.PersistentFlags().StringVarP(&rvHost, "host", "H", "localhost", "PostgreSQL host")
	RuVectorCmd.PersistentFlags().IntVarP(&rvPort, "port", "p", 5432, "PostgreSQL port")
	RuVectorCmd.PersistentFlags().StringVarP(&rvDatabase, "database", "d", "", "Database name (required)")
	RuVectorCmd.PersistentFlags().StringVarP(&rvUser, "user", "u", "postgres", "Database user")
	RuVectorCmd.PersistentFlags().StringVarP(&rvPassword, "password", "P", "", "Database password (or PGPASSWORD env)")
	RuVectorCmd.PersistentFlags().StringVarP(&rvSchema, "schema", "s", "claude_flow", "Schema name")
	RuVectorCmd.PersistentFlags().BoolVar(&rvSSL, "ssl", false, "Enable SSL connection")

	// Init command
	rvInitCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Force re-initialization")
	rvInitCmd.Flags().IntVar(&initDimensions, "dimensions", 1536, "Vector dimensions")
	rvInitCmd.Flags().StringVar(&initIndexType, "index-type", "hnsw", "Index type (hnsw|ivfflat)")

	// Setup command
	rvSetupCmd.Flags().StringVarP(&setupOutput, "output", "o", "./ruvector-postgres", "Output directory")
	rvSetupCmd.Flags().BoolVar(&setupPrint, "print", false, "Print to stdout instead of files")
	rvSetupCmd.Flags().BoolVarP(&setupForce, "force", "f", false, "Overwrite existing files")

	// Migrate command
	rvMigrateCmd.Flags().BoolVar(&migrateUp, "up", true, "Run pending migrations")
	rvMigrateCmd.Flags().BoolVar(&migrateDown, "down", false, "Rollback last migration")
	rvMigrateCmd.Flags().StringVar(&migrateTo, "to", "", "Migrate to specific version")
	rvMigrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "Show SQL without executing")

	// Optimize command
	rvOptimizeCmd.Flags().BoolVarP(&optimizeAnalyze, "analyze", "a", true, "Analyze current performance")
	rvOptimizeCmd.Flags().BoolVar(&optimizeApply, "apply", false, "Apply suggested optimizations")
	rvOptimizeCmd.Flags().BoolVar(&optimizeVacuum, "vacuum", false, "Run VACUUM ANALYZE")
	rvOptimizeCmd.Flags().BoolVar(&optimizeReindex, "reindex", false, "Rebuild indexes")

	// Import command
	rvImportCmd.Flags().StringVarP(&importInput, "input", "i", "", "Input JSON file path")
	rvImportCmd.Flags().StringVar(&importFromSQLite, "from-sqlite", "", "Import from SQLite database")
	rvImportCmd.Flags().IntVarP(&importBatchSize, "batch-size", "b", 100, "Batch size for imports")
	rvImportCmd.Flags().BoolVarP(&importVerbose, "verbose", "v", false, "Verbose output")

	// Benchmark command
	rvBenchmarkCmd.Flags().IntVarP(&benchVectors, "vectors", "n", 10000, "Number of test vectors")
	rvBenchmarkCmd.Flags().IntVar(&benchDimensions, "dimensions", 1536, "Vector dimensions")
	rvBenchmarkCmd.Flags().IntVarP(&benchQueries, "queries", "q", 100, "Number of test queries")
	rvBenchmarkCmd.Flags().IntVar(&benchK, "k", 10, "Top-k results to retrieve")
	rvBenchmarkCmd.Flags().StringVarP(&benchMetric, "metric", "m", "cosine", "Distance metric (cosine|l2|inner)")
	rvBenchmarkCmd.Flags().StringVar(&benchIndex, "index", "hnsw", "Index type (hnsw|ivfflat|none)")
	rvBenchmarkCmd.Flags().BoolVar(&benchCleanup, "cleanup", true, "Clean up test data after benchmark")

	// Backup create command
	rvBackupCreateCmd.Flags().StringVarP(&backupOutput, "output", "o", "", "Output file path (required)")
	rvBackupCreateCmd.Flags().StringVarP(&backupTables, "tables", "t", "", "Specific tables (comma-separated)")
	rvBackupCreateCmd.Flags().StringVarP(&backupFormat, "format", "f", "sql", "Output format (sql|json)")
	rvBackupCreateCmd.Flags().BoolVarP(&backupCompress, "compress", "c", false, "Compress with gzip")

	// Backup restore command
	rvBackupRestoreCmd.Flags().StringVarP(&restoreInput, "input", "i", "", "Input file path (required)")
	rvBackupRestoreCmd.Flags().BoolVar(&restoreClean, "clean", false, "Drop existing tables first")
	rvBackupRestoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "Show what would be restored")

	// Status command
	rvStatusCmd.Flags().BoolVarP(&statusVerboseRV, "verbose", "v", false, "Show detailed information")
	rvStatusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")

	// Add backup subcommands
	rvBackupCmd.AddCommand(rvBackupCreateCmd)
	rvBackupCmd.AddCommand(rvBackupRestoreCmd)

	// Add all subcommands to ruvector
	RuVectorCmd.AddCommand(rvInitCmd)
	RuVectorCmd.AddCommand(rvSetupCmd)
	RuVectorCmd.AddCommand(rvMigrateCmd)
	RuVectorCmd.AddCommand(rvOptimizeCmd)
	RuVectorCmd.AddCommand(rvImportCmd)
	RuVectorCmd.AddCommand(rvBenchmarkCmd)
	RuVectorCmd.AddCommand(rvBackupCmd)
	RuVectorCmd.AddCommand(rvStatusCmd)
}
