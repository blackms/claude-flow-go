// Package hooks provides the hooks system for self-learning operations.
package hooks

import (
	"sort"
	"strings"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// PatternStore stores and retrieves learned patterns.
type PatternStore struct {
	mu          sync.RWMutex
	patterns    map[string]*shared.Pattern
	byType      map[shared.PatternType][]*shared.Pattern
	byKeyword   map[string][]*shared.Pattern
	maxPatterns int
}

// NewPatternStore creates a new PatternStore.
func NewPatternStore(maxPatterns int) *PatternStore {
	if maxPatterns <= 0 {
		maxPatterns = 10000
	}
	return &PatternStore{
		patterns:    make(map[string]*shared.Pattern),
		byType:      make(map[shared.PatternType][]*shared.Pattern),
		byKeyword:   make(map[string][]*shared.Pattern),
		maxPatterns: maxPatterns,
	}
}

// Store stores a pattern.
func (ps *PatternStore) Store(pattern *shared.Pattern) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Check if already exists
	if existing, exists := ps.patterns[pattern.ID]; exists {
		// Update existing pattern
		existing.SuccessCount = pattern.SuccessCount
		existing.FailureCount = pattern.FailureCount
		now := shared.Now()
		if now <= existing.UpdatedAt {
			now = existing.UpdatedAt + 1
		}
		existing.UpdatedAt = now
		existing.LastUsedAt = now
		return nil
	}

	// Check max patterns
	if len(ps.patterns) >= ps.maxPatterns {
		// Remove oldest unused pattern
		ps.evictOldest()
	}

	// Set timestamps
	if pattern.CreatedAt == 0 {
		pattern.CreatedAt = shared.Now()
	}
	pattern.UpdatedAt = shared.Now()

	// Store pattern
	ps.patterns[pattern.ID] = pattern

	// Index by type
	ps.byType[pattern.Type] = append(ps.byType[pattern.Type], pattern)

	// Index by keywords
	for _, keyword := range pattern.Keywords {
		keyword = strings.ToLower(keyword)
		ps.byKeyword[keyword] = append(ps.byKeyword[keyword], pattern)
	}

	return nil
}

// Get retrieves a pattern by ID.
func (ps *PatternStore) Get(id string) *shared.Pattern {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.patterns[id]
}

// FindSimilar finds patterns similar to the query.
func (ps *PatternStore) FindSimilar(query string, patternType shared.PatternType, limit int) []*shared.Pattern {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// Extract keywords from query
	queryKeywords := extractKeywords(query)

	// Score patterns by keyword match
	type scoredPattern struct {
		pattern *shared.Pattern
		score   float64
	}

	var scored []scoredPattern

	// Get patterns to search
	var searchPatterns []*shared.Pattern
	if patternType != "" {
		searchPatterns = ps.byType[patternType]
	} else {
		for _, p := range ps.patterns {
			searchPatterns = append(searchPatterns, p)
		}
	}

	// Score each pattern
	for _, pattern := range searchPatterns {
		score := ps.calculateSimilarity(queryKeywords, pattern.Keywords)
		if score > 0 {
			scored = append(scored, scoredPattern{pattern: pattern, score: score})
		}
	}

	// Sort by score (descending) and success rate
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].pattern.GetSuccessRate() > scored[j].pattern.GetSuccessRate()
	})

	// Return top results
	result := make([]*shared.Pattern, 0, limit)
	for i := 0; i < len(scored) && i < limit; i++ {
		result = append(result, scored[i].pattern)
	}

	return result
}

// calculateSimilarity calculates similarity between two keyword sets.
func (ps *PatternStore) calculateSimilarity(query, pattern []string) float64 {
	if len(query) == 0 || len(pattern) == 0 {
		return 0
	}

	patternSet := make(map[string]bool)
	for _, k := range pattern {
		patternSet[strings.ToLower(k)] = true
	}

	matches := 0
	for _, k := range query {
		if patternSet[strings.ToLower(k)] {
			matches++
		}
	}

	// Jaccard similarity
	union := len(query) + len(pattern) - matches
	if union == 0 {
		return 0
	}
	return float64(matches) / float64(union)
}

// RecordSuccess records a successful use of a pattern.
func (ps *PatternStore) RecordSuccess(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	pattern, exists := ps.patterns[id]
	if !exists {
		return shared.ErrPatternNotFound
	}

	pattern.SuccessCount++
	pattern.LastUsedAt = shared.Now()
	pattern.UpdatedAt = shared.Now()

	return nil
}

// RecordFailure records a failed use of a pattern.
func (ps *PatternStore) RecordFailure(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	pattern, exists := ps.patterns[id]
	if !exists {
		return shared.ErrPatternNotFound
	}

	pattern.FailureCount++
	pattern.LastUsedAt = shared.Now()
	pattern.UpdatedAt = shared.Now()

	return nil
}

// GetSuccessRate returns the success rate for a pattern.
func (ps *PatternStore) GetSuccessRate(id string) float64 {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	pattern, exists := ps.patterns[id]
	if !exists {
		return 0
	}

	return pattern.GetSuccessRate()
}

// Delete removes a pattern.
func (ps *PatternStore) Delete(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	pattern, exists := ps.patterns[id]
	if !exists {
		return shared.ErrPatternNotFound
	}

	// Remove from type index
	typePatterns := ps.byType[pattern.Type]
	for i, p := range typePatterns {
		if p.ID == id {
			ps.byType[pattern.Type] = append(typePatterns[:i], typePatterns[i+1:]...)
			break
		}
	}

	// Remove from keyword index
	for _, keyword := range pattern.Keywords {
		keyword = strings.ToLower(keyword)
		keywordPatterns := ps.byKeyword[keyword]
		for i, p := range keywordPatterns {
			if p.ID == id {
				ps.byKeyword[keyword] = append(keywordPatterns[:i], keywordPatterns[i+1:]...)
				break
			}
		}
	}

	// Remove from main map
	delete(ps.patterns, id)

	return nil
}

// Count returns the total number of patterns.
func (ps *PatternStore) Count() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.patterns)
}

// CountByType returns the number of patterns of a specific type.
func (ps *PatternStore) CountByType(patternType shared.PatternType) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.byType[patternType])
}

// ListByType returns all patterns of a specific type.
func (ps *PatternStore) ListByType(patternType shared.PatternType, limit int) []*shared.Pattern {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	patterns := ps.byType[patternType]
	if limit <= 0 || limit > len(patterns) {
		limit = len(patterns)
	}

	result := make([]*shared.Pattern, limit)
	copy(result, patterns[:limit])
	return result
}

// evictOldest removes the oldest unused pattern.
func (ps *PatternStore) evictOldest() {
	if len(ps.patterns) == 0 {
		return
	}

	var oldest *shared.Pattern
	for _, p := range ps.patterns {
		if oldest == nil || p.LastUsedAt < oldest.LastUsedAt {
			oldest = p
		}
	}

	if oldest != nil {
		// Remove from type index
		typePatterns := ps.byType[oldest.Type]
		for i, p := range typePatterns {
			if p.ID == oldest.ID {
				ps.byType[oldest.Type] = append(typePatterns[:i], typePatterns[i+1:]...)
				break
			}
		}

		// Remove from keyword index
		for _, keyword := range oldest.Keywords {
			keyword = strings.ToLower(keyword)
			keywordPatterns := ps.byKeyword[keyword]
			for i, p := range keywordPatterns {
				if p.ID == oldest.ID {
					ps.byKeyword[keyword] = append(keywordPatterns[:i], keywordPatterns[i+1:]...)
					break
				}
			}
		}

		delete(ps.patterns, oldest.ID)
	}
}

// Clear removes all patterns.
func (ps *PatternStore) Clear() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.patterns = make(map[string]*shared.Pattern)
	ps.byType = make(map[shared.PatternType][]*shared.Pattern)
	ps.byKeyword = make(map[string][]*shared.Pattern)
}

// extractKeywords extracts keywords from text.
func extractKeywords(text string) []string {
	// Simple keyword extraction: split by common delimiters and filter
	text = strings.ToLower(text)
	
	// Replace common delimiters with spaces
	for _, delim := range []string{"/", "\\", ".", "-", "_", ":", ";", ",", "(", ")", "[", "]", "{", "}"} {
		text = strings.ReplaceAll(text, delim, " ")
	}

	words := strings.Fields(text)
	
	// Filter out common stop words and short words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true,
		"of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"and": true, "or": true, "not": true, "but": true,
		"this": true, "that": true, "these": true, "those": true,
		"it": true, "its": true, "if": true, "then": true, "else": true,
	}

	var keywords []string
	seen := make(map[string]bool)
	
	for _, word := range words {
		if len(word) < 2 {
			continue
		}
		if stopWords[word] {
			continue
		}
		if seen[word] {
			continue
		}
		seen[word] = true
		keywords = append(keywords, word)
	}

	return keywords
}

// CreateEditPattern creates a pattern from an edit operation.
func CreateEditPattern(filePath, operation string, success bool, metadata map[string]interface{}) *shared.Pattern {
	keywords := extractKeywords(filePath + " " + operation)
	
	pattern := &shared.Pattern{
		ID:        shared.GenerateID("pattern"),
		Type:      shared.PatternTypeEdit,
		Content:   filePath + ": " + operation,
		Keywords:  keywords,
		Metadata:  metadata,
		CreatedAt: shared.Now(),
	}

	if success {
		pattern.SuccessCount = 1
	} else {
		pattern.FailureCount = 1
	}

	return pattern
}

// CreateCommandPattern creates a pattern from a command execution.
func CreateCommandPattern(command string, success bool, exitCode int, executionTime int64) *shared.Pattern {
	keywords := extractKeywords(command)
	
	pattern := &shared.Pattern{
		ID:       shared.GenerateID("pattern"),
		Type:     shared.PatternTypeCommand,
		Content:  command,
		Keywords: keywords,
		Metadata: map[string]interface{}{
			"exitCode":      exitCode,
			"executionTime": executionTime,
		},
		CreatedAt: shared.Now(),
	}

	if success {
		pattern.SuccessCount = 1
	} else {
		pattern.FailureCount = 1
	}

	return pattern
}
