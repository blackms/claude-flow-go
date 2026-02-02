// Package ruvector provides RuVector application services.
package ruvector

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	infraRuvector "github.com/anthropics/claude-flow-go/internal/infrastructure/ruvector"
)

// BenchmarkRunner runs performance benchmarks.
type BenchmarkRunner struct {
	service *Service
	rng     *rand.Rand
}

// NewBenchmarkRunner creates a new benchmark runner.
func NewBenchmarkRunner(service *Service) *BenchmarkRunner {
	return &BenchmarkRunner{
		service: service,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Run executes a complete benchmark suite.
func (b *BenchmarkRunner) Run(ctx context.Context, config infraRuvector.BenchmarkConfig) (*infraRuvector.BenchmarkResult, error) {
	result := &infraRuvector.BenchmarkResult{
		Operation:   "full_benchmark",
		VectorCount: config.VectorCount,
		Dimensions:  config.Dimensions,
		IndexType:   config.IndexType,
	}

	schema := b.service.Config().Schema
	tableName := fmt.Sprintf("%s.benchmark_%d", schema, time.Now().Unix())

	// Create benchmark table
	if err := b.createBenchmarkTable(ctx, tableName, config); err != nil {
		return nil, fmt.Errorf("failed to create benchmark table: %w", err)
	}

	// Cleanup on exit if requested
	if config.Cleanup {
		defer b.dropBenchmarkTable(ctx, tableName)
	}

	// Generate and insert vectors
	insertStart := time.Now()
	if err := b.insertVectors(ctx, tableName, config); err != nil {
		return nil, fmt.Errorf("failed to insert vectors: %w", err)
	}
	insertDuration := time.Since(insertStart)
	result.InsertThroughput = float64(config.VectorCount) / insertDuration.Seconds()

	// Create index
	indexStart := time.Now()
	if err := b.createIndex(ctx, tableName, config); err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}
	result.IndexBuildTimeMs = float64(time.Since(indexStart).Milliseconds())

	// Set index search parameters
	if err := b.setSearchParams(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to set search params: %w", err)
	}

	// Run query benchmark
	latencies, err := b.runQueryBenchmark(ctx, tableName, config)
	if err != nil {
		return nil, fmt.Errorf("query benchmark failed: %w", err)
	}

	// Calculate latency statistics
	result.QueryLatencyAvg = average(latencies)
	result.QueryLatencyP50 = percentile(latencies, 50)
	result.QueryLatencyP95 = percentile(latencies, 95)
	result.QueryLatencyP99 = percentile(latencies, 99)

	totalQueryTime := sum(latencies)
	result.QPS = float64(len(latencies)) / (totalQueryTime / 1000000) // Convert μs to seconds

	// Estimate recall based on index type
	switch config.IndexType {
	case "hnsw":
		result.RecallEstimate = 0.99
	case "ivfflat":
		result.RecallEstimate = 0.95
	default:
		result.RecallEstimate = 1.0
	}

	// Get size statistics
	if err := b.getSizeStats(ctx, tableName, result); err != nil {
		// Non-fatal error
		fmt.Printf("Warning: failed to get size stats: %v\n", err)
	}

	result.BytesPerVector = float64(result.TableSizeBytes+result.IndexSizeBytes) / float64(config.VectorCount)

	return result, nil
}

// createBenchmarkTable creates a temporary table for benchmarking.
func (b *BenchmarkRunner) createBenchmarkTable(ctx context.Context, tableName string, config infraRuvector.BenchmarkConfig) error {
	_, err := b.service.Backend().Connection().Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			id SERIAL PRIMARY KEY,
			embedding vector(%d)
		)
	`, tableName, config.Dimensions))
	return err
}

// dropBenchmarkTable drops the benchmark table.
func (b *BenchmarkRunner) dropBenchmarkTable(ctx context.Context, tableName string) error {
	_, err := b.service.Backend().Connection().Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
	return err
}

// insertVectors inserts random vectors in batches.
func (b *BenchmarkRunner) insertVectors(ctx context.Context, tableName string, config infraRuvector.BenchmarkConfig) error {
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	for i := 0; i < config.VectorCount; i += batchSize {
		end := i + batchSize
		if end > config.VectorCount {
			end = config.VectorCount
		}

		// Build batch insert
		var values []string
		for j := i; j < end; j++ {
			vector := b.generateRandomVector(config.Dimensions)
			values = append(values, fmt.Sprintf("('%s'::vector(%d))", formatVector(vector), config.Dimensions))
		}

		_, err := b.service.Backend().Connection().Exec(ctx, fmt.Sprintf(
			"INSERT INTO %s (embedding) VALUES %s",
			tableName, strings.Join(values, ","),
		))
		if err != nil {
			return err
		}
	}

	return nil
}

// createIndex creates the vector index.
func (b *BenchmarkRunner) createIndex(ctx context.Context, tableName string, config infraRuvector.BenchmarkConfig) error {
	if config.IndexType == "none" {
		return nil
	}

	var indexSQL string
	switch config.IndexType {
	case "hnsw":
		opsClass := getOpsClass(config.Metric)
		indexSQL = fmt.Sprintf(`
			CREATE INDEX ON %s 
			USING hnsw (embedding %s)
			WITH (m = 16, ef_construction = 64)
		`, tableName, opsClass)
	case "ivfflat":
		opsClass := getOpsClass(config.Metric)
		lists := int(math.Sqrt(float64(config.VectorCount)))
		if lists < 10 {
			lists = 10
		}
		indexSQL = fmt.Sprintf(`
			CREATE INDEX ON %s 
			USING ivfflat (embedding %s)
			WITH (lists = %d)
		`, tableName, opsClass, lists)
	default:
		return fmt.Errorf("unknown index type: %s", config.IndexType)
	}

	_, err := b.service.Backend().Connection().Exec(ctx, indexSQL)
	return err
}

// setSearchParams sets index-specific search parameters.
func (b *BenchmarkRunner) setSearchParams(ctx context.Context, config infraRuvector.BenchmarkConfig) error {
	switch config.IndexType {
	case "hnsw":
		_, err := b.service.Backend().Connection().Exec(ctx, "SET hnsw.ef_search = 100")
		return err
	case "ivfflat":
		_, err := b.service.Backend().Connection().Exec(ctx, "SET ivfflat.probes = 10")
		return err
	}
	return nil
}

// runQueryBenchmark runs query performance tests.
func (b *BenchmarkRunner) runQueryBenchmark(ctx context.Context, tableName string, config infraRuvector.BenchmarkConfig) ([]float64, error) {
	latencies := make([]float64, 0, config.QueryCount)
	distanceOp := getDistanceOperator(config.Metric)

	for i := 0; i < config.QueryCount; i++ {
		queryVector := b.generateRandomVector(config.Dimensions)
		vectorStr := formatVector(queryVector)

		start := time.Now()
		rows, err := b.service.Backend().Connection().Query(ctx, fmt.Sprintf(`
			SELECT id, embedding %s '%s'::vector(%d) as distance
			FROM %s
			ORDER BY embedding %s '%s'::vector(%d)
			LIMIT %d
		`, distanceOp, vectorStr, config.Dimensions, tableName,
			distanceOp, vectorStr, config.Dimensions, config.K))
		if err != nil {
			return nil, err
		}

		// Consume results
		count := 0
		for rows.Next() {
			var id int
			var distance float64
			rows.Scan(&id, &distance)
			count++
		}
		rows.Close()

		latencyUs := float64(time.Since(start).Microseconds())
		latencies = append(latencies, latencyUs)
	}

	return latencies, nil
}

// getSizeStats gets table and index size statistics.
func (b *BenchmarkRunner) getSizeStats(ctx context.Context, tableName string, result *infraRuvector.BenchmarkResult) error {
	var tableSize, indexSize int64

	err := b.service.Backend().Connection().QueryRow(ctx, fmt.Sprintf(`
		SELECT pg_relation_size('%s')
	`, tableName)).Scan(&tableSize)
	if err != nil {
		return err
	}
	result.TableSizeBytes = tableSize

	// Get total index size
	err = b.service.Backend().Connection().QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(pg_relation_size(indexrelid)), 0)
		FROM pg_index
		WHERE indrelid = '%s'::regclass
	`, tableName)).Scan(&indexSize)
	if err != nil {
		return err
	}
	result.IndexSizeBytes = indexSize

	return nil
}

// generateRandomVector generates a random normalized vector.
func (b *BenchmarkRunner) generateRandomVector(dimensions int) []float32 {
	vector := make([]float32, dimensions)
	var sum float64

	for i := range vector {
		val := b.rng.Float32()*2 - 1
		vector[i] = val
		sum += float64(val * val)
	}

	// Normalize
	norm := float32(math.Sqrt(sum))
	if norm > 0 {
		for i := range vector {
			vector[i] /= norm
		}
	}

	return vector
}

// formatVector formats a vector for PostgreSQL.
func formatVector(v []float32) string {
	parts := make([]string, len(v))
	for i, val := range v {
		parts[i] = fmt.Sprintf("%f", val)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// getOpsClass returns the operator class for the metric.
func getOpsClass(metric string) string {
	switch metric {
	case "l2":
		return "vector_l2_ops"
	case "inner":
		return "vector_ip_ops"
	default:
		return "vector_cosine_ops"
	}
}

// getDistanceOperator returns the distance operator for the metric.
func getDistanceOperator(metric string) string {
	switch metric {
	case "l2":
		return "<->"
	case "inner":
		return "<#>"
	default:
		return "<=>"
	}
}

// average calculates the average of a slice.
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return sum(values) / float64(len(values))
}

// sum calculates the sum of a slice.
func sum(values []float64) float64 {
	var total float64
	for _, v := range values {
		total += v
	}
	return total
}

// percentile calculates the p-th percentile of a slice.
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	idx := int(float64(len(sorted)-1) * p / 100)
	return sorted[idx]
}
