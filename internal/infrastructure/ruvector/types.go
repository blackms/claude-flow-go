// Package ruvector provides PostgreSQL/pgvector integration for vector operations.
package ruvector

import (
	"time"
)

// RuVectorConfig holds PostgreSQL connection configuration.
type RuVectorConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Database   string `json:"database"`
	User       string `json:"user"`
	Password   string `json:"password"`
	Schema     string `json:"schema"`
	SSL        bool   `json:"ssl"`
	Dimensions int    `json:"dimensions"`
	IndexType  string `json:"indexType"` // hnsw or ivfflat
}

// DefaultConfig returns sensible defaults for RuVector.
func DefaultConfig() RuVectorConfig {
	return RuVectorConfig{
		Host:       "localhost",
		Port:       5432,
		User:       "postgres",
		Schema:     "claude_flow",
		Dimensions: 1536,
		IndexType:  "hnsw",
	}
}

// HNSWConfig holds HNSW index parameters.
type HNSWConfig struct {
	M              int `json:"m"`              // Connectivity parameter (default: 16)
	EfConstruction int `json:"efConstruction"` // Build-time quality (default: 64)
	EfSearch       int `json:"efSearch"`       // Query-time parameter (default: 100)
}

// DefaultHNSWConfig returns sensible HNSW defaults.
func DefaultHNSWConfig() HNSWConfig {
	return HNSWConfig{
		M:              16,
		EfConstruction: 64,
		EfSearch:       100,
	}
}

// IVFFlatConfig holds IVFFlat index parameters.
type IVFFlatConfig struct {
	Lists  int `json:"lists"`  // Number of clusters (default: 100)
	Probes int `json:"probes"` // Query-time parameter (default: 10)
}

// DefaultIVFFlatConfig returns sensible IVFFlat defaults.
func DefaultIVFFlatConfig() IVFFlatConfig {
	return IVFFlatConfig{
		Lists:  100,
		Probes: 10,
	}
}

// Migration represents a database migration.
type Migration struct {
	Version   string    `json:"version"`
	Name      string    `json:"name"`
	SQL       string    `json:"sql"`
	Rollback  string    `json:"rollback"`
	AppliedAt time.Time `json:"appliedAt,omitempty"`
	Checksum  string    `json:"checksum,omitempty"`
}

// MigrationStatus represents the current migration state.
type MigrationStatus struct {
	CurrentVersion string      `json:"currentVersion"`
	Applied        []Migration `json:"applied"`
	Pending        []Migration `json:"pending"`
}

// Embedding represents a stored vector embedding.
type Embedding struct {
	ID        string                 `json:"id"`
	Key       string                 `json:"key"`
	Namespace string                 `json:"namespace"`
	Vector    []float32              `json:"vector"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

// SearchResult represents a vector search result.
type SearchResult struct {
	Embedding  Embedding `json:"embedding"`
	Distance   float64   `json:"distance"`
	Similarity float64   `json:"similarity"`
}

// TableStats holds statistics about a database table.
type TableStats struct {
	Name       string `json:"name"`
	RowCount   int64  `json:"rowCount"`
	Size       string `json:"size"`
	TotalSize  string `json:"totalSize"`
	DeadTuples int64  `json:"deadTuples"`
}

// IndexStats holds statistics about a database index.
type IndexStats struct {
	Name      string `json:"name"`
	Table     string `json:"table"`
	Type      string `json:"type"`
	Size      string `json:"size"`
	IsValid   bool   `json:"isValid"`
	ScanCount int64  `json:"scanCount"`
}

// SystemStatus holds the overall RuVector system status.
type SystemStatus struct {
	Connected       bool          `json:"connected"`
	LatencyMs       float64       `json:"latencyMs"`
	PostgresVersion string        `json:"postgresVersion"`
	PgVectorVersion string        `json:"pgvectorVersion"`
	SchemaExists    bool          `json:"schemaExists"`
	Initialized     bool          `json:"initialized"`
	Version         string        `json:"version"`
	Dimensions      int           `json:"dimensions"`
	Tables          []TableStats  `json:"tables"`
	Indexes         []IndexStats  `json:"indexes,omitempty"`
	Migrations      []Migration   `json:"migrations,omitempty"`
}

// BenchmarkConfig holds benchmark configuration.
type BenchmarkConfig struct {
	VectorCount int    `json:"vectorCount"`
	Dimensions  int    `json:"dimensions"`
	QueryCount  int    `json:"queryCount"`
	K           int    `json:"k"`
	Metric      string `json:"metric"`    // cosine, l2, inner
	IndexType   string `json:"indexType"` // hnsw, ivfflat, none
	BatchSize   int    `json:"batchSize"`
	Cleanup     bool   `json:"cleanup"`
}

// DefaultBenchmarkConfig returns sensible benchmark defaults.
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		VectorCount: 10000,
		Dimensions:  1536,
		QueryCount:  100,
		K:           10,
		Metric:      "cosine",
		IndexType:   "hnsw",
		BatchSize:   1000,
		Cleanup:     true,
	}
}

// BenchmarkResult holds benchmark metrics.
type BenchmarkResult struct {
	Operation        string  `json:"operation"`
	VectorCount      int     `json:"vectorCount"`
	Dimensions       int     `json:"dimensions"`
	IndexType        string  `json:"indexType"`
	InsertThroughput float64 `json:"insertThroughput"` // vectors/sec
	IndexBuildTimeMs float64 `json:"indexBuildTimeMs"`
	QueryLatencyAvg  float64 `json:"queryLatencyAvg"`  // μs
	QueryLatencyP50  float64 `json:"queryLatencyP50"`  // μs
	QueryLatencyP95  float64 `json:"queryLatencyP95"`  // μs
	QueryLatencyP99  float64 `json:"queryLatencyP99"`  // μs
	QPS              float64 `json:"qps"`
	RecallEstimate   float64 `json:"recallEstimate"`
	TableSizeBytes   int64   `json:"tableSizeBytes"`
	IndexSizeBytes   int64   `json:"indexSizeBytes"`
	BytesPerVector   float64 `json:"bytesPerVector"`
}

// BackupConfig holds backup configuration.
type BackupConfig struct {
	OutputPath string   `json:"outputPath"`
	Tables     []string `json:"tables"`
	Format     string   `json:"format"` // sql, json
	Compress   bool     `json:"compress"`
}

// RestoreConfig holds restore configuration.
type RestoreConfig struct {
	InputPath string `json:"inputPath"`
	Clean     bool   `json:"clean"`
	DryRun    bool   `json:"dryRun"`
}

// BackupMetadata holds backup file metadata.
type BackupMetadata struct {
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
	Schema      string    `json:"schema"`
	Tables      []string  `json:"tables"`
	RowCounts   map[string]int64 `json:"rowCounts"`
	Dimensions  int       `json:"dimensions"`
	Compressed  bool      `json:"compressed"`
}

// ImportConfig holds import configuration.
type ImportConfig struct {
	InputPath  string `json:"inputPath"`
	FromSQLite string `json:"fromSqlite"`
	BatchSize  int    `json:"batchSize"`
	Verbose    bool   `json:"verbose"`
}

// ImportStats holds import statistics.
type ImportStats struct {
	Total          int            `json:"total"`
	Imported       int            `json:"imported"`
	Skipped        int            `json:"skipped"`
	Errors         int            `json:"errors"`
	WithEmbeddings int            `json:"withEmbeddings"`
	ByNamespace    map[string]int `json:"byNamespace"`
	DurationMs     int64          `json:"durationMs"`
}

// OptimizationRecommendation represents an optimization suggestion.
type OptimizationRecommendation struct {
	Priority    string `json:"priority"` // critical, high, medium, low
	Category    string `json:"category"` // bloat, index, statistics, memory
	Description string `json:"description"`
	SQL         string `json:"sql,omitempty"`
}

// OptimizationReport holds optimization analysis results.
type OptimizationReport struct {
	AnalyzedAt      time.Time                    `json:"analyzedAt"`
	Recommendations []OptimizationRecommendation `json:"recommendations"`
	TableBloat      map[string]float64           `json:"tableBloat"`
	IndexHealth     map[string]bool              `json:"indexHealth"`
}
