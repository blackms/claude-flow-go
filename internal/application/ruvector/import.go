// Package ruvector provides RuVector application services.
package ruvector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	infraRuvector "github.com/anthropics/claude-flow-go/internal/infrastructure/ruvector"
)

// ImportManager handles data import operations.
type ImportManager struct {
	service *Service
}

// NewImportManager creates a new import manager.
func NewImportManager(service *Service) *ImportManager {
	return &ImportManager{service: service}
}

// JSONEntry represents an entry in a JSON import file.
type JSONEntry struct {
	Key       string                 `json:"key"`
	Value     string                 `json:"value"`
	Namespace string                 `json:"namespace"`
	Content   string                 `json:"content"`
	Embedding []float32              `json:"embedding"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ImportFromJSON imports embeddings from a JSON file.
func (i *ImportManager) ImportFromJSON(ctx context.Context, config infraRuvector.ImportConfig) (*infraRuvector.ImportStats, error) {
	stats := &infraRuvector.ImportStats{
		ByNamespace: make(map[string]int),
	}
	startTime := time.Now()

	// Read JSON file
	data, err := os.ReadFile(config.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to parse as array
	var entries []JSONEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		// Try object with entries field
		var obj struct {
			Entries []JSONEntry `json:"entries"`
			Results []JSONEntry `json:"results"`
		}
		if err := json.Unmarshal(data, &obj); err != nil {
			// Try key-value object
			var kvObj map[string]string
			if err := json.Unmarshal(data, &kvObj); err != nil {
				return nil, fmt.Errorf("failed to parse JSON: unsupported format")
			}
			// Convert to entries
			for k, v := range kvObj {
				entries = append(entries, JSONEntry{Key: k, Value: v, Content: v})
			}
		} else {
			if len(obj.Entries) > 0 {
				entries = obj.Entries
			} else {
				entries = obj.Results
			}
		}
	}

	stats.Total = len(entries)

	// Import in batches
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	for idx := 0; idx < len(entries); idx += batchSize {
		end := idx + batchSize
		if end > len(entries) {
			end = len(entries)
		}

		batch := entries[idx:end]
		for _, entry := range batch {
			if err := i.importEntry(ctx, entry, stats, config.Verbose); err != nil {
				stats.Errors++
				if config.Verbose {
					fmt.Printf("Error importing %s: %v\n", entry.Key, err)
				}
				continue
			}
			stats.Imported++
		}
	}

	stats.DurationMs = time.Since(startTime).Milliseconds()
	return stats, nil
}

// ImportFromSQLite imports data from a SQLite database.
func (i *ImportManager) ImportFromSQLite(ctx context.Context, config infraRuvector.ImportConfig) (*infraRuvector.ImportStats, error) {
	stats := &infraRuvector.ImportStats{
		ByNamespace: make(map[string]int),
	}
	startTime := time.Now()

	// Open SQLite database
	sqliteDB, err := sql.Open("sqlite", config.FromSQLite)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer sqliteDB.Close()

	// Query memories table
	rows, err := sqliteDB.QueryContext(ctx, `
		SELECT id, agent_id, content, type, timestamp, embedding, metadata
		FROM memories
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query SQLite: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, agentID, content, memType string
		var timestamp int64
		var embeddingBlob []byte
		var metadataStr sql.NullString

		if err := rows.Scan(&id, &agentID, &content, &memType, &timestamp, &embeddingBlob, &metadataStr); err != nil {
			stats.Errors++
			continue
		}

		stats.Total++

		// Parse embedding from JSON blob
		var embedding []float32
		if len(embeddingBlob) > 0 {
			if err := json.Unmarshal(embeddingBlob, &embedding); err != nil {
				// Try as []float64
				var embedding64 []float64
				if err := json.Unmarshal(embeddingBlob, &embedding64); err == nil {
					embedding = make([]float32, len(embedding64))
					for i, v := range embedding64 {
						embedding[i] = float32(v)
					}
				}
			}
		}

		// Parse metadata
		var metadata map[string]interface{}
		if metadataStr.Valid && metadataStr.String != "" {
			json.Unmarshal([]byte(metadataStr.String), &metadata)
		}

		entry := JSONEntry{
			Key:       id,
			Namespace: memType,
			Content:   content,
			Embedding: embedding,
			Metadata:  metadata,
		}

		if err := i.importEntry(ctx, entry, stats, config.Verbose); err != nil {
			stats.Errors++
			if config.Verbose {
				fmt.Printf("Error importing %s: %v\n", id, err)
			}
			continue
		}

		stats.Imported++
	}

	stats.DurationMs = time.Since(startTime).Milliseconds()
	return stats, nil
}

// importEntry imports a single entry.
func (i *ImportManager) importEntry(ctx context.Context, entry JSONEntry, stats *infraRuvector.ImportStats, verbose bool) error {
	namespace := entry.Namespace
	if namespace == "" {
		namespace = "default"
	}

	content := entry.Content
	if content == "" {
		content = entry.Value
	}

	emb := infraRuvector.Embedding{
		Key:       entry.Key,
		Namespace: namespace,
		Content:   content,
		Vector:    entry.Embedding,
		Metadata:  entry.Metadata,
	}

	if len(emb.Vector) > 0 {
		stats.WithEmbeddings++
	}

	if err := i.service.Backend().StoreEmbedding(emb); err != nil {
		return err
	}

	stats.ByNamespace[namespace]++

	if verbose {
		fmt.Printf("Imported: %s (namespace: %s)\n", entry.Key, namespace)
	}

	return nil
}
