// Package hooks provides the hooks system for self-learning operations.
package hooks

import (
	"sync"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestNewPatternStore(t *testing.T) {
	tests := []struct {
		name        string
		maxPatterns int
		expected    int
	}{
		{"positive max patterns", 5000, 5000},
		{"zero defaults to 10000", 0, 10000},
		{"negative defaults to 10000", -1, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := NewPatternStore(tt.maxPatterns)
			if ps.maxPatterns != tt.expected {
				t.Errorf("expected maxPatterns %d, got %d", tt.expected, ps.maxPatterns)
			}
			if ps.patterns == nil {
				t.Error("patterns map should be initialized")
			}
			if ps.byType == nil {
				t.Error("byType map should be initialized")
			}
			if ps.byKeyword == nil {
				t.Error("byKeyword map should be initialized")
			}
		})
	}
}

func TestPatternStore_Store(t *testing.T) {
	ps := NewPatternStore(1000)

	pattern := &shared.Pattern{
		ID:       "pattern-1",
		Type:     shared.PatternTypeEdit,
		Content:  "test pattern",
		Keywords: []string{"test", "pattern"},
	}

	err := ps.Store(pattern)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if ps.Count() != 1 {
		t.Errorf("expected 1 pattern, got %d", ps.Count())
	}

	// Verify timestamps were set
	stored := ps.Get("pattern-1")
	if stored.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
	if stored.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set")
	}
}

func TestPatternStore_StoreUpdate(t *testing.T) {
	ps := NewPatternStore(1000)

	pattern := &shared.Pattern{
		ID:           "pattern-1",
		Type:         shared.PatternTypeEdit,
		Content:      "test pattern",
		Keywords:     []string{"test"},
		SuccessCount: 1,
	}

	_ = ps.Store(pattern)
	originalUpdatedAt := ps.Get("pattern-1").UpdatedAt

	// Store again with same ID should update
	pattern.SuccessCount = 5
	_ = ps.Store(pattern)

	stored := ps.Get("pattern-1")
	if stored.SuccessCount != 5 {
		t.Errorf("expected SuccessCount 5, got %d", stored.SuccessCount)
	}
	if stored.UpdatedAt <= originalUpdatedAt {
		t.Error("UpdatedAt should be updated")
	}

	// Count should still be 1
	if ps.Count() != 1 {
		t.Errorf("expected 1 pattern after update, got %d", ps.Count())
	}
}

func TestPatternStore_StoreEviction(t *testing.T) {
	ps := NewPatternStore(3)

	// Store 3 patterns
	for i := 0; i < 3; i++ {
		pattern := &shared.Pattern{
			ID:       shared.GenerateID("pattern"),
			Type:     shared.PatternTypeEdit,
			Keywords: []string{"keyword"},
		}
		_ = ps.Store(pattern)
	}

	if ps.Count() != 3 {
		t.Errorf("expected 3 patterns, got %d", ps.Count())
	}

	// Store one more should evict the oldest
	newPattern := &shared.Pattern{
		ID:       shared.GenerateID("pattern"),
		Type:     shared.PatternTypeEdit,
		Keywords: []string{"keyword"},
	}
	_ = ps.Store(newPattern)

	if ps.Count() != 3 {
		t.Errorf("expected 3 patterns after eviction, got %d", ps.Count())
	}
}

func TestPatternStore_Get(t *testing.T) {
	ps := NewPatternStore(1000)

	pattern := &shared.Pattern{
		ID:      "pattern-1",
		Type:    shared.PatternTypeEdit,
		Content: "test pattern",
	}
	_ = ps.Store(pattern)

	// Get existing
	retrieved := ps.Get("pattern-1")
	if retrieved == nil {
		t.Error("should retrieve pattern")
	}
	if retrieved.Content != "test pattern" {
		t.Errorf("expected content 'test pattern', got '%s'", retrieved.Content)
	}

	// Get non-existing
	notFound := ps.Get("non-existent")
	if notFound != nil {
		t.Error("should return nil for non-existent pattern")
	}
}

func TestPatternStore_FindSimilar(t *testing.T) {
	ps := NewPatternStore(1000)

	// Store some patterns
	patterns := []*shared.Pattern{
		{ID: "p1", Type: shared.PatternTypeEdit, Content: "edit file.go", Keywords: []string{"edit", "file", "go"}, SuccessCount: 5},
		{ID: "p2", Type: shared.PatternTypeEdit, Content: "edit config.yaml", Keywords: []string{"edit", "config", "yaml"}, SuccessCount: 3},
		{ID: "p3", Type: shared.PatternTypeCommand, Content: "go test", Keywords: []string{"go", "test"}},
		{ID: "p4", Type: shared.PatternTypeEdit, Content: "edit test.go", Keywords: []string{"edit", "test", "go"}, SuccessCount: 10},
	}

	for _, p := range patterns {
		_ = ps.Store(p)
	}

	// Find similar edit patterns
	similar := ps.FindSimilar("edit go file", shared.PatternTypeEdit, 10)

	if len(similar) == 0 {
		t.Error("should find similar patterns")
	}

	// Should be sorted by similarity score and success rate
	// Patterns with "edit", "go", "file" keywords should rank higher
	found := false
	for _, s := range similar {
		if s.ID == "p1" || s.ID == "p4" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should find patterns matching 'edit go file'")
	}
}

func TestPatternStore_FindSimilarLimit(t *testing.T) {
	ps := NewPatternStore(1000)

	// Store many patterns
	for i := 0; i < 20; i++ {
		pattern := &shared.Pattern{
			ID:       shared.GenerateID("pattern"),
			Type:     shared.PatternTypeEdit,
			Keywords: []string{"common", "keyword"},
		}
		_ = ps.Store(pattern)
	}

	// Find with limit
	similar := ps.FindSimilar("common keyword", shared.PatternTypeEdit, 5)

	if len(similar) != 5 {
		t.Errorf("expected 5 patterns, got %d", len(similar))
	}
}

func TestPatternStore_FindSimilarNoMatch(t *testing.T) {
	ps := NewPatternStore(1000)

	pattern := &shared.Pattern{
		ID:       "p1",
		Type:     shared.PatternTypeEdit,
		Keywords: []string{"alpha", "beta"},
	}
	_ = ps.Store(pattern)

	// Search with completely different keywords
	similar := ps.FindSimilar("gamma delta", shared.PatternTypeEdit, 10)

	if len(similar) != 0 {
		t.Errorf("expected no matches, got %d", len(similar))
	}
}

func TestPatternStore_FindSimilarByType(t *testing.T) {
	ps := NewPatternStore(1000)

	editPattern := &shared.Pattern{
		ID:       "edit-1",
		Type:     shared.PatternTypeEdit,
		Keywords: []string{"common"},
	}
	commandPattern := &shared.Pattern{
		ID:       "cmd-1",
		Type:     shared.PatternTypeCommand,
		Keywords: []string{"common"},
	}

	_ = ps.Store(editPattern)
	_ = ps.Store(commandPattern)

	// Find only edit patterns
	editResults := ps.FindSimilar("common", shared.PatternTypeEdit, 10)
	if len(editResults) != 1 {
		t.Errorf("expected 1 edit pattern, got %d", len(editResults))
	}
	if editResults[0].ID != "edit-1" {
		t.Errorf("expected edit-1, got %s", editResults[0].ID)
	}

	// Find only command patterns
	cmdResults := ps.FindSimilar("common", shared.PatternTypeCommand, 10)
	if len(cmdResults) != 1 {
		t.Errorf("expected 1 command pattern, got %d", len(cmdResults))
	}
	if cmdResults[0].ID != "cmd-1" {
		t.Errorf("expected cmd-1, got %s", cmdResults[0].ID)
	}
}

func TestPatternStore_RecordSuccess(t *testing.T) {
	ps := NewPatternStore(1000)

	pattern := &shared.Pattern{
		ID:           "pattern-1",
		Type:         shared.PatternTypeEdit,
		SuccessCount: 0,
	}
	_ = ps.Store(pattern)

	err := ps.RecordSuccess("pattern-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stored := ps.Get("pattern-1")
	if stored.SuccessCount != 1 {
		t.Errorf("expected SuccessCount 1, got %d", stored.SuccessCount)
	}
	if stored.LastUsedAt == 0 {
		t.Error("LastUsedAt should be set")
	}

	// Record for non-existent should error
	err = ps.RecordSuccess("non-existent")
	if err != shared.ErrPatternNotFound {
		t.Errorf("expected ErrPatternNotFound, got %v", err)
	}
}

func TestPatternStore_RecordFailure(t *testing.T) {
	ps := NewPatternStore(1000)

	pattern := &shared.Pattern{
		ID:           "pattern-1",
		Type:         shared.PatternTypeEdit,
		FailureCount: 0,
	}
	_ = ps.Store(pattern)

	err := ps.RecordFailure("pattern-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stored := ps.Get("pattern-1")
	if stored.FailureCount != 1 {
		t.Errorf("expected FailureCount 1, got %d", stored.FailureCount)
	}
}

func TestPatternStore_GetSuccessRate(t *testing.T) {
	ps := NewPatternStore(1000)

	pattern := &shared.Pattern{
		ID:           "pattern-1",
		Type:         shared.PatternTypeEdit,
		SuccessCount: 7,
		FailureCount: 3,
	}
	_ = ps.Store(pattern)

	rate := ps.GetSuccessRate("pattern-1")
	expected := 0.7 // 7/10
	if rate != expected {
		t.Errorf("expected success rate %v, got %v", expected, rate)
	}

	// Non-existent pattern
	rate = ps.GetSuccessRate("non-existent")
	if rate != 0 {
		t.Errorf("expected 0 for non-existent, got %v", rate)
	}
}

func TestPatternStore_Delete(t *testing.T) {
	ps := NewPatternStore(1000)

	pattern := &shared.Pattern{
		ID:       "pattern-1",
		Type:     shared.PatternTypeEdit,
		Keywords: []string{"test", "keyword"},
	}
	_ = ps.Store(pattern)

	err := ps.Delete("pattern-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if ps.Count() != 0 {
		t.Errorf("expected 0 patterns, got %d", ps.Count())
	}

	if ps.Get("pattern-1") != nil {
		t.Error("pattern should not exist after deletion")
	}

	// Delete non-existent should error
	err = ps.Delete("non-existent")
	if err != shared.ErrPatternNotFound {
		t.Errorf("expected ErrPatternNotFound, got %v", err)
	}
}

func TestPatternStore_Count(t *testing.T) {
	ps := NewPatternStore(1000)

	if ps.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", ps.Count())
	}

	for i := 0; i < 5; i++ {
		pattern := &shared.Pattern{
			ID:   shared.GenerateID("pattern"),
			Type: shared.PatternTypeEdit,
		}
		_ = ps.Store(pattern)
	}

	if ps.Count() != 5 {
		t.Errorf("expected 5, got %d", ps.Count())
	}
}

func TestPatternStore_CountByType(t *testing.T) {
	ps := NewPatternStore(1000)

	patterns := []*shared.Pattern{
		{ID: "e1", Type: shared.PatternTypeEdit},
		{ID: "e2", Type: shared.PatternTypeEdit},
		{ID: "c1", Type: shared.PatternTypeCommand},
		{ID: "r1", Type: shared.PatternTypeRoute},
	}

	for _, p := range patterns {
		_ = ps.Store(p)
	}

	if ps.CountByType(shared.PatternTypeEdit) != 2 {
		t.Errorf("expected 2 edit patterns, got %d", ps.CountByType(shared.PatternTypeEdit))
	}

	if ps.CountByType(shared.PatternTypeCommand) != 1 {
		t.Errorf("expected 1 command pattern, got %d", ps.CountByType(shared.PatternTypeCommand))
	}

	if ps.CountByType(shared.PatternTypeRoute) != 1 {
		t.Errorf("expected 1 route pattern, got %d", ps.CountByType(shared.PatternTypeRoute))
	}

	if ps.CountByType(shared.PatternTypeTask) != 0 {
		t.Errorf("expected 0 task patterns, got %d", ps.CountByType(shared.PatternTypeTask))
	}
}

func TestPatternStore_ListByType(t *testing.T) {
	ps := NewPatternStore(1000)

	for i := 0; i < 10; i++ {
		pattern := &shared.Pattern{
			ID:   shared.GenerateID("pattern"),
			Type: shared.PatternTypeEdit,
		}
		_ = ps.Store(pattern)
	}

	// List all
	all := ps.ListByType(shared.PatternTypeEdit, 0)
	if len(all) != 10 {
		t.Errorf("expected 10 patterns, got %d", len(all))
	}

	// List with limit
	limited := ps.ListByType(shared.PatternTypeEdit, 5)
	if len(limited) != 5 {
		t.Errorf("expected 5 patterns, got %d", len(limited))
	}
}

func TestPatternStore_Clear(t *testing.T) {
	ps := NewPatternStore(1000)

	for i := 0; i < 5; i++ {
		pattern := &shared.Pattern{
			ID:       shared.GenerateID("pattern"),
			Type:     shared.PatternTypeEdit,
			Keywords: []string{"keyword"},
		}
		_ = ps.Store(pattern)
	}

	ps.Clear()

	if ps.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", ps.Count())
	}

	if ps.CountByType(shared.PatternTypeEdit) != 0 {
		t.Errorf("expected 0 by type after clear, got %d", ps.CountByType(shared.PatternTypeEdit))
	}
}

func TestPatternStore_ConcurrentAccess(t *testing.T) {
	ps := NewPatternStore(1000)

	var wg sync.WaitGroup

	// Concurrent stores
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pattern := &shared.Pattern{
				ID:       shared.GenerateID("pattern"),
				Type:     shared.PatternTypeEdit,
				Keywords: []string{"test"},
			}
			_ = ps.Store(pattern)
		}()
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ps.Count()
			_ = ps.FindSimilar("test", shared.PatternTypeEdit, 10)
			_ = ps.ListByType(shared.PatternTypeEdit, 10)
		}()
	}

	wg.Wait()

	if ps.Count() != 50 {
		t.Errorf("expected 50 patterns, got %d", ps.Count())
	}
}

func TestCreateEditPattern(t *testing.T) {
	pattern := CreateEditPattern("/path/to/file.go", "modify", true, map[string]interface{}{"lines": 10})

	if pattern.ID == "" {
		t.Error("ID should be set")
	}
	if pattern.Type != shared.PatternTypeEdit {
		t.Errorf("expected type Edit, got %s", pattern.Type)
	}
	if pattern.SuccessCount != 1 {
		t.Errorf("expected SuccessCount 1 for success=true, got %d", pattern.SuccessCount)
	}
	if len(pattern.Keywords) == 0 {
		t.Error("keywords should be extracted")
	}

	// Create failure pattern
	failPattern := CreateEditPattern("/path/to/file.go", "delete", false, nil)
	if failPattern.FailureCount != 1 {
		t.Errorf("expected FailureCount 1 for success=false, got %d", failPattern.FailureCount)
	}
}

func TestCreateCommandPattern(t *testing.T) {
	pattern := CreateCommandPattern("go build -o app ./...", true, 0, 1500)

	if pattern.ID == "" {
		t.Error("ID should be set")
	}
	if pattern.Type != shared.PatternTypeCommand {
		t.Errorf("expected type Command, got %s", pattern.Type)
	}
	if pattern.SuccessCount != 1 {
		t.Errorf("expected SuccessCount 1 for success=true, got %d", pattern.SuccessCount)
	}
	if len(pattern.Keywords) == 0 {
		t.Error("keywords should be extracted")
	}

	// Check metadata
	if pattern.Metadata["exitCode"] != 0 {
		t.Errorf("expected exitCode 0, got %v", pattern.Metadata["exitCode"])
	}
	if pattern.Metadata["executionTime"] != int64(1500) {
		t.Errorf("expected executionTime 1500, got %v", pattern.Metadata["executionTime"])
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"/path/to/file.go", []string{"path", "to", "file", "go"}},
		{"go test ./...", []string{"go", "test"}},
		{"the a an is are", []string{}}, // all stop words
		{"Config-Parser_v2.js", []string{"config", "parser", "v2", "js"}},
		{"  multiple   spaces  ", []string{"multiple", "spaces"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractKeywords(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d keywords, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i, kw := range tt.expected {
				if result[i] != kw {
					t.Errorf("expected keyword[%d] = %s, got %s", i, kw, result[i])
				}
			}
		})
	}
}

func TestPatternStore_CalculateSimilarity(t *testing.T) {
	ps := NewPatternStore(1000)

	tests := []struct {
		query    []string
		pattern  []string
		expected float64
	}{
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}, 1.0},                   // perfect match
		{[]string{"a", "b"}, []string{"c", "d"}, 0.0},                             // no match
		{[]string{"a", "b", "c"}, []string{"a", "b", "d"}, 0.5},                   // 2 matches out of 4 unique
		{[]string{}, []string{"a", "b"}, 0.0},                                     // empty query
		{[]string{"a"}, []string{}, 0.0},                                          // empty pattern
	}

	for _, tt := range tests {
		result := ps.calculateSimilarity(tt.query, tt.pattern)
		if result != tt.expected {
			t.Errorf("similarity(%v, %v) = %v, expected %v", tt.query, tt.pattern, result, tt.expected)
		}
	}
}

func TestPattern_GetSuccessRate(t *testing.T) {
	tests := []struct {
		success  int
		failure  int
		expected float64
	}{
		{10, 0, 1.0},
		{0, 10, 0.0},
		{7, 3, 0.7},
		{0, 0, 0.0},
	}

	for _, tt := range tests {
		pattern := &shared.Pattern{
			SuccessCount: tt.success,
			FailureCount: tt.failure,
		}

		rate := pattern.GetSuccessRate()
		if rate != tt.expected {
			t.Errorf("GetSuccessRate(%d, %d) = %v, expected %v", tt.success, tt.failure, rate, tt.expected)
		}
	}
}
