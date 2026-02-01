// Package memory provides memory backend implementations.
package memory

import (
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// HybridBackend combines SQLite for structured queries with AgentDB for vector search.
type HybridBackend struct {
	mu             sync.RWMutex
	sqliteBackend  *SQLiteBackend
	agentDBBackend *AgentDBBackend
	initialized    bool
}

// NewHybridBackend creates a new hybrid memory backend.
func NewHybridBackend(sqliteBackend *SQLiteBackend, agentDBBackend *AgentDBBackend) *HybridBackend {
	return &HybridBackend{
		sqliteBackend:  sqliteBackend,
		agentDBBackend: agentDBBackend,
	}
}

// Initialize initializes both backends.
func (hb *HybridBackend) Initialize() error {
	hb.mu.Lock()
	defer hb.mu.Unlock()

	if hb.initialized {
		return nil
	}

	// Initialize both backends concurrently
	var wg sync.WaitGroup
	var sqliteErr, agentDBErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		sqliteErr = hb.sqliteBackend.Initialize()
	}()

	go func() {
		defer wg.Done()
		agentDBErr = hb.agentDBBackend.Initialize()
	}()

	wg.Wait()

	if sqliteErr != nil {
		return sqliteErr
	}
	if agentDBErr != nil {
		return agentDBErr
	}

	hb.initialized = true
	return nil
}

// Close closes both backends.
func (hb *HybridBackend) Close() error {
	hb.mu.Lock()
	defer hb.mu.Unlock()

	var wg sync.WaitGroup
	var sqliteErr, agentDBErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		sqliteErr = hb.sqliteBackend.Close()
	}()

	go func() {
		defer wg.Done()
		agentDBErr = hb.agentDBBackend.Close()
	}()

	wg.Wait()

	hb.initialized = false

	if sqliteErr != nil {
		return sqliteErr
	}
	return agentDBErr
}

// Store stores memory in both backends.
func (hb *HybridBackend) Store(memory shared.Memory) (shared.Memory, error) {
	// Store in SQLite for structured queries
	result, err := hb.sqliteBackend.Store(memory)
	if err != nil {
		return result, err
	}

	// Store in AgentDB if has embedding
	if len(memory.Embedding) > 0 {
		_, err = hb.agentDBBackend.Store(memory)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

// Retrieve retrieves memory by ID (from SQLite primary).
func (hb *HybridBackend) Retrieve(id string) (*shared.Memory, error) {
	return hb.sqliteBackend.Retrieve(id)
}

// Update updates memory in both backends.
func (hb *HybridBackend) Update(memory shared.Memory) error {
	err := hb.sqliteBackend.Update(memory)
	if err != nil {
		return err
	}

	if len(memory.Embedding) > 0 {
		err = hb.agentDBBackend.Update(memory)
		if err != nil {
			return err
		}
	}

	return nil
}

// Delete deletes memory from both backends.
func (hb *HybridBackend) Delete(id string) error {
	var wg sync.WaitGroup
	var sqliteErr, agentDBErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		sqliteErr = hb.sqliteBackend.Delete(id)
	}()

	go func() {
		defer wg.Done()
		agentDBErr = hb.agentDBBackend.Delete(id)
	}()

	wg.Wait()

	if sqliteErr != nil {
		return sqliteErr
	}
	return agentDBErr
}

// Query queries memories using SQLite.
func (hb *HybridBackend) Query(query shared.MemoryQuery) ([]shared.Memory, error) {
	return hb.sqliteBackend.Query(query)
}

// VectorSearch performs vector search using AgentDB.
func (hb *HybridBackend) VectorSearch(embedding []float64, k int) ([]shared.MemorySearchResult, error) {
	return hb.agentDBBackend.VectorSearch(embedding, k)
}

// ClearAgent clears all memories for an agent from both backends.
func (hb *HybridBackend) ClearAgent(agentID string) error {
	var wg sync.WaitGroup
	var sqliteErr, agentDBErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		sqliteErr = hb.sqliteBackend.ClearAgent(agentID)
	}()

	go func() {
		defer wg.Done()
		agentDBErr = hb.agentDBBackend.ClearAgent(agentID)
	}()

	wg.Wait()

	if sqliteErr != nil {
		return sqliteErr
	}
	return agentDBErr
}

// HybridSearch combines SQL filtering with vector similarity.
func (hb *HybridBackend) HybridSearch(query shared.MemoryQuery, embedding []float64, k int) ([]shared.MemorySearchResult, error) {
	if len(embedding) == 0 {
		// No embedding, fall back to SQL query
		results, err := hb.Query(query)
		if err != nil {
			return nil, err
		}

		searchResults := make([]shared.MemorySearchResult, len(results))
		for i, m := range results {
			searchResults[i] = shared.MemorySearchResult{
				Memory:     m,
				Similarity: 1.0,
			}
		}
		return searchResults, nil
	}

	// Get vector search results (get more than needed for filtering)
	vectorResults, err := hb.VectorSearch(embedding, k*2)
	if err != nil {
		return nil, err
	}

	// Filter by query criteria
	filtered := make([]shared.MemorySearchResult, 0)

	for _, result := range vectorResults {
		// Filter by agentId
		if query.AgentID != "" && result.AgentID != query.AgentID {
			continue
		}

		// Filter by type
		if query.Type != "" && result.Type != query.Type {
			continue
		}

		// Filter by time range
		if query.TimeRange != nil {
			if result.Timestamp < query.TimeRange.Start || result.Timestamp > query.TimeRange.End {
				continue
			}
		}

		// Filter by metadata
		if query.Metadata != nil {
			match := true
			for key, value := range query.Metadata {
				if result.Metadata == nil || result.Metadata[key] != value {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		filtered = append(filtered, result)
	}

	// Limit to k results
	if k > 0 && k < len(filtered) {
		filtered = filtered[:k]
	}

	return filtered, nil
}

// GetStats returns statistics about both backends.
func (hb *HybridBackend) GetStats() map[string]int {
	return map[string]int{
		"sqlite":  hb.sqliteBackend.GetCount(),
		"agentdb": hb.agentDBBackend.GetCount(),
	}
}
