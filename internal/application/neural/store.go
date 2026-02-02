// Package neural provides neural application services.
package neural

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/neural"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
)

// PatternStorage represents the JSON structure for pattern persistence.
type PatternStorage struct {
	Patterns []neural.Pattern `json:"patterns"`
	Metadata StorageMetadata  `json:"metadata"`
}

// StorageMetadata holds metadata about the pattern storage.
type StorageMetadata struct {
	Version       string `json:"version"`
	LastUpdated   string `json:"lastUpdated"`
	TotalPatterns int    `json:"totalPatterns"`
}

// PatternStore manages neural pattern persistence.
type PatternStore struct {
	mu           sync.RWMutex
	patterns     map[string]*neural.Pattern
	storagePath  string
	generator    *infraNeural.EmbeddingGenerator
	dirty        bool
}

// NewPatternStore creates a new pattern store.
func NewPatternStore(storagePath string, dimension int) (*PatternStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(storagePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	store := &PatternStore{
		patterns:    make(map[string]*neural.Pattern),
		storagePath: storagePath,
		generator:   infraNeural.NewEmbeddingGenerator(dimension),
	}

	// Load existing patterns if file exists
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load patterns: %w", err)
	}

	return store, nil
}

// load loads patterns from disk.
func (s *PatternStore) load() error {
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return err
	}

	var storage PatternStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("failed to parse storage: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range storage.Patterns {
		p := storage.Patterns[i]
		s.patterns[p.ID] = &p
	}

	return nil
}

// Save persists all patterns to disk.
func (s *PatternStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	patterns := make([]neural.Pattern, 0, len(s.patterns))
	for _, p := range s.patterns {
		patterns = append(patterns, *p)
	}

	// Sort by creation time for deterministic output
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].CreatedAt.Before(patterns[j].CreatedAt)
	})

	storage := PatternStorage{
		Patterns: patterns,
		Metadata: StorageMetadata{
			Version:       "1.0.0",
			LastUpdated:   time.Now().Format(time.RFC3339),
			TotalPatterns: len(patterns),
		},
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal storage: %w", err)
	}

	if err := os.WriteFile(s.storagePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write storage: %w", err)
	}

	s.dirty = false
	return nil
}

// Add adds a new pattern to the store.
func (s *PatternStore) Add(pattern *neural.Pattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.patterns[pattern.ID] = pattern
	s.dirty = true
	return nil
}

// Get retrieves a pattern by ID.
func (s *PatternStore) Get(id string) (*neural.Pattern, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pattern, exists := s.patterns[id]
	return pattern, exists
}

// GetAll returns all patterns.
func (s *PatternStore) GetAll() []*neural.Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*neural.Pattern, 0, len(s.patterns))
	for _, p := range s.patterns {
		result = append(result, p)
	}

	// Sort by creation time
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result
}

// GetByType returns patterns of a specific type.
func (s *PatternStore) GetByType(patternType string) []*neural.Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*neural.Pattern, 0)
	for _, p := range s.patterns {
		if p.Type == patternType {
			result = append(result, p)
		}
	}

	return result
}

// Search finds patterns similar to the query text.
func (s *PatternStore) Search(query string, limit int) []*neural.Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.patterns) == 0 {
		return nil
	}

	queryEmbedding := s.generator.Generate(query)

	type scoredPattern struct {
		pattern *neural.Pattern
		score   float64
	}

	scored := make([]scoredPattern, 0, len(s.patterns))
	for _, p := range s.patterns {
		score := infraNeural.CosineSimilarity(queryEmbedding, p.Embedding)
		scored = append(scored, scoredPattern{pattern: p, score: score})
	}

	// Sort by similarity descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top results
	if limit > len(scored) {
		limit = len(scored)
	}

	result := make([]*neural.Pattern, limit)
	for i := 0; i < limit; i++ {
		result[i] = scored[i].pattern
	}

	return result
}

// Delete removes a pattern by ID.
func (s *PatternStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.patterns[id]; exists {
		delete(s.patterns, id)
		s.dirty = true
		return true
	}
	return false
}

// Count returns the number of patterns.
func (s *PatternStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.patterns)
}

// Clear removes all patterns.
func (s *PatternStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patterns = make(map[string]*neural.Pattern)
	s.dirty = true
}

// IsDirty returns whether there are unsaved changes.
func (s *PatternStore) IsDirty() bool {
	return s.dirty
}

// GetStoragePath returns the storage file path.
func (s *PatternStore) GetStoragePath() string {
	return s.storagePath
}

// Compact removes duplicate patterns based on similarity threshold.
func (s *PatternStore) Compact(similarityThreshold float64) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if similarityThreshold <= 0 {
		similarityThreshold = 0.95
	}

	patterns := make([]*neural.Pattern, 0, len(s.patterns))
	for _, p := range s.patterns {
		patterns = append(patterns, p)
	}

	// Sort by usage count descending (keep more used patterns)
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].UsageCount > patterns[j].UsageCount
	})

	toRemove := make(map[string]bool)

	for i := 0; i < len(patterns); i++ {
		if toRemove[patterns[i].ID] {
			continue
		}
		for j := i + 1; j < len(patterns); j++ {
			if toRemove[patterns[j].ID] {
				continue
			}
			similarity := infraNeural.CosineSimilarity(patterns[i].Embedding, patterns[j].Embedding)
			if similarity >= similarityThreshold {
				toRemove[patterns[j].ID] = true
			}
		}
	}

	for id := range toRemove {
		delete(s.patterns, id)
	}

	if len(toRemove) > 0 {
		s.dirty = true
	}

	return len(toRemove)
}

// GetStats returns statistics about the pattern store.
func (s *PatternStore) GetStats() neural.NeuralSystemStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalUsage int
	var totalConfidence float64
	var lastUpdated string

	for _, p := range s.patterns {
		totalUsage += p.UsageCount
		totalConfidence += p.Confidence
		if p.LastUsedAt.Format(time.RFC3339) > lastUpdated {
			lastUpdated = p.LastUsedAt.Format(time.RFC3339)
		}
	}

	avgConfidence := 0.0
	if len(s.patterns) > 0 {
		avgConfidence = totalConfidence / float64(len(s.patterns))
	}

	return neural.NeuralSystemStatus{
		Initialized:       true,
		PatternCount:      len(s.patterns),
		TotalUsage:        totalUsage,
		AverageConfidence: avgConfidence,
		StoragePath:       s.storagePath,
		LastUpdated:       lastUpdated,
	}
}
