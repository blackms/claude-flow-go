// Package memory provides memory backend implementations.
package memory

import (
	"math"
	"sort"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// AgentDBBackend implements MemoryBackend with vector search using HNSW.
type AgentDBBackend struct {
	mu           sync.RWMutex
	dbPath       string
	dimensions   int
	hnswM        int
	efConstruction int
	memories     map[string]shared.Memory
	initialized  bool
}

// AgentDBOption configures the AgentDBBackend.
type AgentDBOption func(*AgentDBBackend)

// WithDimensions sets the embedding dimensions.
func WithDimensions(dim int) AgentDBOption {
	return func(ab *AgentDBBackend) {
		ab.dimensions = dim
	}
}

// WithHNSWM sets the HNSW M parameter.
func WithHNSWM(m int) AgentDBOption {
	return func(ab *AgentDBBackend) {
		ab.hnswM = m
	}
}

// WithEFConstruction sets the HNSW efConstruction parameter.
func WithEFConstruction(ef int) AgentDBOption {
	return func(ab *AgentDBBackend) {
		ab.efConstruction = ef
	}
}

// NewAgentDBBackend creates a new AgentDB-based memory backend.
func NewAgentDBBackend(dbPath string, opts ...AgentDBOption) *AgentDBBackend {
	ab := &AgentDBBackend{
		dbPath:       dbPath,
		dimensions:   768, // default embedding dimension
		hnswM:        16,
		efConstruction: 200,
		memories:     make(map[string]shared.Memory),
	}

	for _, opt := range opts {
		opt(ab)
	}

	return ab
}

// Initialize initializes the AgentDB backend.
func (ab *AgentDBBackend) Initialize() error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if ab.initialized {
		return nil
	}

	// In production, this would initialize an HNSW index
	// For now, we use in-memory storage with brute-force search
	ab.initialized = true
	return nil
}

// Close closes the AgentDB backend.
func (ab *AgentDBBackend) Close() error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	ab.memories = make(map[string]shared.Memory)
	ab.initialized = false
	return nil
}

// Store stores a memory entry.
func (ab *AgentDBBackend) Store(memory shared.Memory) (shared.Memory, error) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	ab.memories[memory.ID] = memory
	return memory, nil
}

// Retrieve retrieves a memory entry by ID.
func (ab *AgentDBBackend) Retrieve(id string) (*shared.Memory, error) {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	memory, exists := ab.memories[id]
	if !exists {
		return nil, nil
	}
	return &memory, nil
}

// Update updates a memory entry.
func (ab *AgentDBBackend) Update(memory shared.Memory) error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if _, exists := ab.memories[memory.ID]; exists {
		ab.memories[memory.ID] = memory
	}
	return nil
}

// Delete deletes a memory entry.
func (ab *AgentDBBackend) Delete(id string) error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	delete(ab.memories, id)
	return nil
}

// Query queries memory entries (basic filtering, use VectorSearch for semantic search).
func (ab *AgentDBBackend) Query(query shared.MemoryQuery) ([]shared.Memory, error) {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	results := make([]shared.Memory, 0)

	for _, memory := range ab.memories {
		if query.AgentID != "" && memory.AgentID != query.AgentID {
			continue
		}
		if query.Type != "" && memory.Type != query.Type {
			continue
		}
		if query.TimeRange != nil {
			if memory.Timestamp < query.TimeRange.Start || memory.Timestamp > query.TimeRange.End {
				continue
			}
		}
		results = append(results, memory)
	}

	// Sort by timestamp (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp > results[j].Timestamp
	})

	// Apply pagination
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return results, nil
}

// VectorSearch performs vector similarity search.
func (ab *AgentDBBackend) VectorSearch(embedding []float64, k int) ([]shared.MemorySearchResult, error) {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	if len(embedding) == 0 {
		return []shared.MemorySearchResult{}, nil
	}

	type scoredMemory struct {
		memory     shared.Memory
		similarity float64
	}

	scored := make([]scoredMemory, 0)

	for _, memory := range ab.memories {
		if len(memory.Embedding) == 0 {
			continue
		}

		// Calculate cosine similarity
		similarity := cosineSimilarity(embedding, memory.Embedding)
		scored = append(scored, scoredMemory{
			memory:     memory,
			similarity: similarity,
		})
	}

	// Sort by similarity (highest first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].similarity > scored[j].similarity
	})

	// Limit to k results
	if k > 0 && k < len(scored) {
		scored = scored[:k]
	}

	results := make([]shared.MemorySearchResult, len(scored))
	for i, s := range scored {
		results[i] = shared.MemorySearchResult{
			Memory:     s.memory,
			Similarity: s.similarity,
		}
	}

	return results, nil
}

// ClearAgent clears all memories for an agent.
func (ab *AgentDBBackend) ClearAgent(agentID string) error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	for id, memory := range ab.memories {
		if memory.AgentID == agentID {
			delete(ab.memories, id)
		}
	}
	return nil
}

// GetDBPath returns the database path.
func (ab *AgentDBBackend) GetDBPath() string {
	return ab.dbPath
}

// GetCount returns the number of memories with embeddings.
func (ab *AgentDBBackend) GetCount() int {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	count := 0
	for _, memory := range ab.memories {
		if len(memory.Embedding) > 0 {
			count++
		}
	}
	return count
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// euclideanDistance calculates the Euclidean distance between two vectors.
func euclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}
