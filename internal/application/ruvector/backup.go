// Package ruvector provides RuVector application services.
package ruvector

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	infraRuvector "github.com/anthropics/claude-flow-go/internal/infrastructure/ruvector"
)

// BackupManager handles backup and restore operations.
type BackupManager struct {
	service *Service
}

// NewBackupManager creates a new backup manager.
func NewBackupManager(service *Service) *BackupManager {
	return &BackupManager{service: service}
}

// BackupData represents the JSON backup format.
type BackupData struct {
	Metadata infraRuvector.BackupMetadata `json:"metadata"`
	Tables   map[string][]map[string]interface{} `json:"tables"`
}

// CreateBackup creates a backup of the database.
func (b *BackupManager) CreateBackup(ctx context.Context, config infraRuvector.BackupConfig) error {
	schema := b.service.Config().Schema

	// Get tables to backup
	tables := config.Tables
	if len(tables) == 0 {
		// Default to all tables
		tables = []string{"embeddings", "attention_patterns", "graph_edges", "metadata", "migrations"}
	}

	// Collect row counts
	rowCounts := make(map[string]int64)
	for _, table := range tables {
		var count int64
		err := b.service.Backend().Connection().QueryRow(ctx, fmt.Sprintf(
			"SELECT COUNT(*) FROM %s.%s", schema, table,
		)).Scan(&count)
		if err != nil {
			// Table might not exist
			continue
		}
		rowCounts[table] = count
	}

	metadata := infraRuvector.BackupMetadata{
		Version:    "1.0.0",
		CreatedAt:  time.Now(),
		Schema:     schema,
		Tables:     tables,
		RowCounts:  rowCounts,
		Dimensions: b.service.Config().Dimensions,
		Compressed: config.Compress,
	}

	switch config.Format {
	case "json":
		return b.createJSONBackup(ctx, config, metadata, tables)
	default:
		return b.createSQLBackup(ctx, config, metadata, tables)
	}
}

// createJSONBackup creates a JSON format backup.
func (b *BackupManager) createJSONBackup(ctx context.Context, config infraRuvector.BackupConfig, metadata infraRuvector.BackupMetadata, tables []string) error {
	schema := b.service.Config().Schema

	backup := BackupData{
		Metadata: metadata,
		Tables:   make(map[string][]map[string]interface{}),
	}

	for _, table := range tables {
		rows, err := b.service.Backend().Connection().Query(ctx, fmt.Sprintf(
			"SELECT * FROM %s.%s", schema, table,
		))
		if err != nil {
			continue // Skip tables that don't exist
		}

		columns, _ := rows.Columns()
		var tableData []map[string]interface{}

		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				rows.Close()
				return err
			}

			row := make(map[string]interface{})
			for i, col := range columns {
				row[col] = values[i]
			}
			tableData = append(tableData, row)
		}
		rows.Close()

		backup.Tables[table] = tableData
	}

	// Write to file
	var writer io.Writer
	file, err := os.Create(config.OutputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if config.Compress {
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		writer = gzWriter
	} else {
		writer = file
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(backup)
}

// createSQLBackup creates a SQL format backup.
func (b *BackupManager) createSQLBackup(ctx context.Context, config infraRuvector.BackupConfig, metadata infraRuvector.BackupMetadata, tables []string) error {
	schema := b.service.Config().Schema

	// Open output file
	var writer io.Writer
	file, err := os.Create(config.OutputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if config.Compress {
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		writer = gzWriter
	} else {
		writer = file
	}

	// Write header
	fmt.Fprintf(writer, "-- RuVector Backup\n")
	fmt.Fprintf(writer, "-- Created: %s\n", metadata.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(writer, "-- Schema: %s\n", schema)
	fmt.Fprintf(writer, "-- Version: %s\n\n", metadata.Version)

	for _, table := range tables {
		fmt.Fprintf(writer, "-- Table: %s\n", table)

		rows, err := b.service.Backend().Connection().Query(ctx, fmt.Sprintf(
			"SELECT * FROM %s.%s", schema, table,
		))
		if err != nil {
			fmt.Fprintf(writer, "-- Error: table does not exist\n\n")
			continue
		}

		columns, _ := rows.Columns()

		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				rows.Close()
				return err
			}

			// Build INSERT statement
			var valueStrs []string
			for _, v := range values {
				valueStrs = append(valueStrs, formatSQLValue(v))
			}

			fmt.Fprintf(writer, "INSERT INTO %s.%s (%s) VALUES (%s) ON CONFLICT DO NOTHING;\n",
				schema, table,
				strings.Join(columns, ", "),
				strings.Join(valueStrs, ", "),
			)
		}
		rows.Close()

		fmt.Fprintln(writer)
	}

	return nil
}

// RestoreBackup restores from a backup file.
func (b *BackupManager) RestoreBackup(ctx context.Context, config infraRuvector.RestoreConfig) (int, error) {
	// Open input file
	file, err := os.Open(config.InputPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var reader io.Reader = file

	// Check if gzipped
	if strings.HasSuffix(config.InputPath, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return 0, err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Detect format
	ext := filepath.Ext(strings.TrimSuffix(config.InputPath, ".gz"))
	if ext == ".json" {
		return b.restoreJSONBackup(ctx, reader, config)
	}
	return b.restoreSQLBackup(ctx, reader, config)
}

// restoreJSONBackup restores from JSON format.
func (b *BackupManager) restoreJSONBackup(ctx context.Context, reader io.Reader, config infraRuvector.RestoreConfig) (int, error) {
	var backup BackupData
	if err := json.NewDecoder(reader).Decode(&backup); err != nil {
		return 0, err
	}

	if config.DryRun {
		total := 0
		for _, rows := range backup.Tables {
			total += len(rows)
		}
		return total, nil
	}

	schema := b.service.Config().Schema

	// Clean if requested
	if config.Clean {
		if err := b.service.Backend().Schema().DropSchema(ctx); err != nil {
			return 0, err
		}
		// Re-initialize
		if err := b.service.Initialize(ctx, true); err != nil {
			return 0, err
		}
	}

	totalRestored := 0

	for table, rows := range backup.Tables {
		for _, row := range rows {
			columns := make([]string, 0, len(row))
			values := make([]string, 0, len(row))

			for col, val := range row {
				columns = append(columns, col)
				values = append(values, formatSQLValue(val))
			}

			_, err := b.service.Backend().Connection().Exec(ctx, fmt.Sprintf(
				"INSERT INTO %s.%s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
				schema, table,
				strings.Join(columns, ", "),
				strings.Join(values, ", "),
			))
			if err != nil {
				continue // Skip errors
			}
			totalRestored++
		}
	}

	return totalRestored, nil
}

// restoreSQLBackup restores from SQL format.
func (b *BackupManager) restoreSQLBackup(ctx context.Context, reader io.Reader, config infraRuvector.RestoreConfig) (int, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}

	statements := strings.Split(string(data), ";\n")
	restored := 0

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		if config.DryRun {
			if strings.HasPrefix(strings.ToUpper(stmt), "INSERT") {
				restored++
			}
			continue
		}

		if strings.HasPrefix(strings.ToUpper(stmt), "INSERT") {
			if _, err := b.service.Backend().Connection().Exec(ctx, stmt); err != nil {
				continue // Skip errors
			}
			restored++
		}
	}

	return restored, nil
}

// formatSQLValue formats a value for SQL.
func formatSQLValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}

	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''"))
	case []byte:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(string(val), "'", "''"))
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "TRUE"
		}
		return "FALSE"
	case time.Time:
		return fmt.Sprintf("'%s'", val.Format(time.RFC3339))
	default:
		// Try JSON encoding
		if jsonBytes, err := json.Marshal(val); err == nil {
			return fmt.Sprintf("'%s'", strings.ReplaceAll(string(jsonBytes), "'", "''"))
		}
		return fmt.Sprintf("'%v'", val)
	}
}
