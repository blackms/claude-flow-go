// Package neural provides neural network infrastructure.
package neural

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	_ "modernc.org/sqlite"
)

// ReasoningStore provides SQLite-based storage for reasoning patterns.
type ReasoningStore struct {
	mu     sync.RWMutex
	db     *sql.DB
	config domainNeural.ReasoningConfig

	// In-memory vector index for fast similarity search
	vectorIndex map[string][]float32

	// LRU tracking
	accessOrder []string
}

// NewReasoningStore creates a new reasoning store.
func NewReasoningStore(config domainNeural.ReasoningConfig) (*ReasoningStore, error) {
	db, err := sql.Open("sqlite", config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &ReasoningStore{
		db:          db,
		config:      config,
		vectorIndex: make(map[string][]float32),
		accessOrder: make([]string, 0),
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	if err := store.loadVectorIndex(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load vector index: %w", err)
	}

	return store, nil
}

// initSchema creates the database tables.
func (s *ReasoningStore) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS reasoning_patterns (
			pattern_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			domain TEXT NOT NULL,
			embedding BLOB,
			strategy TEXT NOT NULL,
			success_rate REAL DEFAULT 0,
			usage_count INTEGER DEFAULT 0,
			quality_history TEXT DEFAULT '[]',
			evolution_history TEXT DEFAULT '[]',
			version INTEGER DEFAULT 1,
			key_learnings TEXT DEFAULT '[]',
			source_trajectory_ids TEXT DEFAULT '[]',
			last_used DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS reasoning_memories (
			memory_id TEXT PRIMARY KEY,
			trajectory_id TEXT NOT NULL,
			strategy TEXT NOT NULL,
			key_learnings TEXT DEFAULT '[]',
			embedding BLOB,
			quality REAL DEFAULT 0,
			usage_count INTEGER DEFAULT 0,
			consolidated INTEGER DEFAULT 0,
			pattern_id TEXT,
			relevance_score REAL DEFAULT 0,
			diversity_score REAL DEFAULT 0,
			verdict TEXT,
			last_used DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS pattern_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pattern_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			data TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_patterns_domain ON reasoning_patterns(domain);
		CREATE INDEX IF NOT EXISTS idx_patterns_success_rate ON reasoning_patterns(success_rate);
		CREATE INDEX IF NOT EXISTS idx_patterns_last_used ON reasoning_patterns(last_used);
		CREATE INDEX IF NOT EXISTS idx_memories_trajectory ON reasoning_memories(trajectory_id);
		CREATE INDEX IF NOT EXISTS idx_memories_consolidated ON reasoning_memories(consolidated);
		CREATE INDEX IF NOT EXISTS idx_versions_pattern ON pattern_versions(pattern_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// loadVectorIndex loads all pattern embeddings into memory.
func (s *ReasoningStore) loadVectorIndex() error {
	rows, err := s.db.Query("SELECT pattern_id, embedding FROM reasoning_patterns WHERE embedding IS NOT NULL")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var patternID string
		var embeddingBytes []byte

		if err := rows.Scan(&patternID, &embeddingBytes); err != nil {
			continue
		}

		embedding := bytesToFloat32Slice(embeddingBytes)
		if len(embedding) > 0 {
			s.vectorIndex[patternID] = embedding
			s.accessOrder = append(s.accessOrder, patternID)
		}
	}

	return rows.Err()
}

// SavePattern saves a reasoning pattern.
func (s *ReasoningStore) SavePattern(pattern domainNeural.ReasoningPattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	qualityHistoryJSON, _ := json.Marshal(pattern.QualityHistory)
	evolutionHistoryJSON, _ := json.Marshal(pattern.EvolutionHistory)
	keyLearningsJSON, _ := json.Marshal(pattern.KeyLearnings)
	sourceTrajectoryIDsJSON, _ := json.Marshal(pattern.SourceTrajectoryIDs)
	embeddingBytes := float32SliceToBytes(pattern.Embedding)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO reasoning_patterns (
			pattern_id, name, domain, embedding, strategy, success_rate,
			usage_count, quality_history, evolution_history, version,
			key_learnings, source_trajectory_ids, last_used, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		pattern.PatternID, pattern.Name, pattern.Domain, embeddingBytes,
		pattern.Strategy, pattern.SuccessRate, pattern.UsageCount,
		string(qualityHistoryJSON), string(evolutionHistoryJSON), pattern.Version,
		string(keyLearningsJSON), string(sourceTrajectoryIDsJSON),
		pattern.LastUsed, pattern.CreatedAt, pattern.UpdatedAt,
	)

	if err != nil {
		return err
	}

	// Update vector index
	if len(pattern.Embedding) > 0 {
		s.vectorIndex[pattern.PatternID] = pattern.Embedding
		s.updateAccessOrder(pattern.PatternID)
	}

	// Evict if over limit
	if len(s.vectorIndex) > s.config.MaxPatterns {
		s.evictLRU()
	}

	return nil
}

// GetPattern retrieves a pattern by ID.
func (s *ReasoningStore) GetPattern(patternID string) (*domainNeural.ReasoningPattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row := s.db.QueryRow(`
		SELECT pattern_id, name, domain, embedding, strategy, success_rate,
			usage_count, quality_history, evolution_history, version,
			key_learnings, source_trajectory_ids, last_used, created_at, updated_at
		FROM reasoning_patterns WHERE pattern_id = ?
	`, patternID)

	pattern, err := s.scanPattern(row)
	if err != nil {
		return nil, err
	}

	s.updateAccessOrder(patternID)
	return pattern, nil
}

// SearchSimilar finds patterns similar to the query embedding.
// Target: <1ms
func (s *ReasoningStore) SearchSimilar(queryEmbedding []float32, k int, minRelevance float64) []domainNeural.RetrievalResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(queryEmbedding) == 0 || len(s.vectorIndex) == 0 {
		return nil
	}

	// Compute similarities
	type scoredPattern struct {
		patternID string
		score     float64
	}

	scores := make([]scoredPattern, 0, len(s.vectorIndex))
	for patternID, embedding := range s.vectorIndex {
		similarity := cosineSimilarityFloat32(queryEmbedding, embedding)
		if similarity >= minRelevance {
			scores = append(scores, scoredPattern{patternID, similarity})
		}
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Take top k
	if len(scores) > k {
		scores = scores[:k]
	}

	// Fetch full patterns
	results := make([]domainNeural.RetrievalResult, 0, len(scores))
	for _, sp := range scores {
		pattern, err := s.getPatternUnlocked(sp.patternID)
		if err != nil {
			continue
		}
		results = append(results, domainNeural.RetrievalResult{
			Pattern:        *pattern,
			RelevanceScore: sp.score,
			DiversityScore: 1.0, // Will be computed by MMR
			CombinedScore:  sp.score,
		})
	}

	return results
}

// SearchSimilarMMR performs MMR-based retrieval for diversity.
func (s *ReasoningStore) SearchSimilarMMR(queryEmbedding []float32, k int, minRelevance, lambda float64) []domainNeural.RetrievalResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(queryEmbedding) == 0 || len(s.vectorIndex) == 0 {
		return nil
	}

	// Get more candidates than needed for diversity selection
	candidates := k * 3
	if candidates > len(s.vectorIndex) {
		candidates = len(s.vectorIndex)
	}

	// Compute all similarities
	type candidate struct {
		patternID  string
		embedding  []float32
		relevance  float64
	}

	allCandidates := make([]candidate, 0, len(s.vectorIndex))
	for patternID, embedding := range s.vectorIndex {
		similarity := cosineSimilarityFloat32(queryEmbedding, embedding)
		if similarity >= minRelevance {
			allCandidates = append(allCandidates, candidate{patternID, embedding, similarity})
		}
	}

	// Sort by relevance
	sort.Slice(allCandidates, func(i, j int) bool {
		return allCandidates[i].relevance > allCandidates[j].relevance
	})

	if len(allCandidates) > candidates {
		allCandidates = allCandidates[:candidates]
	}

	// MMR selection
	selected := make([]candidate, 0, k)
	selectedEmbeddings := make([][]float32, 0, k)

	for len(selected) < k && len(allCandidates) > 0 {
		bestIdx := -1
		bestMMR := -1.0

		for i, c := range allCandidates {
			// Compute max similarity to already selected
			maxSimToSelected := 0.0
			for _, selEmb := range selectedEmbeddings {
				sim := cosineSimilarityFloat32(c.embedding, selEmb)
				if sim > maxSimToSelected {
					maxSimToSelected = sim
				}
			}

			// MMR score: λ * relevance + (1-λ) * diversity
			diversity := 1.0 - maxSimToSelected
			mmr := lambda*c.relevance + (1-lambda)*diversity

			if mmr > bestMMR {
				bestMMR = mmr
				bestIdx = i
			}
		}

		if bestIdx >= 0 {
			selected = append(selected, allCandidates[bestIdx])
			selectedEmbeddings = append(selectedEmbeddings, allCandidates[bestIdx].embedding)
			// Remove from candidates
			allCandidates = append(allCandidates[:bestIdx], allCandidates[bestIdx+1:]...)
		} else {
			break
		}
	}

	// Fetch full patterns and compute final scores
	results := make([]domainNeural.RetrievalResult, 0, len(selected))
	for i, c := range selected {
		pattern, err := s.getPatternUnlocked(c.patternID)
		if err != nil {
			continue
		}

		// Compute diversity score
		maxSimToOthers := 0.0
		for j, other := range selected {
			if i != j {
				sim := cosineSimilarityFloat32(c.embedding, other.embedding)
				if sim > maxSimToOthers {
					maxSimToOthers = sim
				}
			}
		}
		diversity := 1.0 - maxSimToOthers
		combined := lambda*c.relevance + (1-lambda)*diversity

		results = append(results, domainNeural.RetrievalResult{
			Pattern:        *pattern,
			RelevanceScore: c.relevance,
			DiversityScore: diversity,
			CombinedScore:  combined,
		})
	}

	return results
}

// DeletePattern deletes a pattern.
func (s *ReasoningStore) DeletePattern(patternID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM reasoning_patterns WHERE pattern_id = ?", patternID)
	if err != nil {
		return err
	}

	delete(s.vectorIndex, patternID)
	s.removeFromAccessOrder(patternID)

	return nil
}

// ListPatterns lists patterns with optional filtering.
func (s *ReasoningStore) ListPatterns(domain *domainNeural.ReasoningDomain, limit int) ([]domainNeural.ReasoningPattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := "SELECT pattern_id, name, domain, embedding, strategy, success_rate, usage_count, quality_history, evolution_history, version, key_learnings, source_trajectory_ids, last_used, created_at, updated_at FROM reasoning_patterns"
	var args []interface{}

	if domain != nil {
		query += " WHERE domain = ?"
		args = append(args, *domain)
	}

	query += " ORDER BY updated_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	patterns := make([]domainNeural.ReasoningPattern, 0)
	for rows.Next() {
		pattern, err := s.scanPatternRows(rows)
		if err != nil {
			continue
		}
		patterns = append(patterns, *pattern)
	}

	return patterns, rows.Err()
}

// SaveMemory saves a reasoning memory.
func (s *ReasoningStore) SaveMemory(memory domainNeural.ReasoningMemory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyLearningsJSON, _ := json.Marshal(memory.Memory.KeyLearnings)
	embeddingBytes := float32SliceToBytes(memory.Memory.Embedding)
	verdictJSON, _ := json.Marshal(memory.Verdict)

	consolidated := 0
	if memory.Consolidated {
		consolidated = 1
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO reasoning_memories (
			memory_id, trajectory_id, strategy, key_learnings, embedding,
			quality, usage_count, consolidated, pattern_id, relevance_score,
			diversity_score, verdict, last_used, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		memory.MemoryID, memory.TrajectoryID, memory.Memory.Strategy,
		string(keyLearningsJSON), embeddingBytes, memory.Memory.Quality,
		memory.Memory.UsageCount, consolidated, memory.PatternID,
		memory.RelevanceScore, memory.DiversityScore, string(verdictJSON),
		memory.Memory.LastUsed, memory.CreatedAt,
	)

	return err
}

// GetMemory retrieves a memory by ID.
func (s *ReasoningStore) GetMemory(memoryID string) (*domainNeural.ReasoningMemory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`
		SELECT memory_id, trajectory_id, strategy, key_learnings, embedding,
			quality, usage_count, consolidated, pattern_id, relevance_score,
			diversity_score, verdict, last_used, created_at
		FROM reasoning_memories WHERE memory_id = ?
	`, memoryID)

	return s.scanMemory(row)
}

// GetUnconsolidatedMemories returns memories that haven't been consolidated.
func (s *ReasoningStore) GetUnconsolidatedMemories(limit int) ([]domainNeural.ReasoningMemory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT memory_id, trajectory_id, strategy, key_learnings, embedding,
			quality, usage_count, consolidated, pattern_id, relevance_score,
			diversity_score, verdict, last_used, created_at
		FROM reasoning_memories WHERE consolidated = 0
		ORDER BY created_at DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memories := make([]domainNeural.ReasoningMemory, 0)
	for rows.Next() {
		memory, err := s.scanMemoryRows(rows)
		if err != nil {
			continue
		}
		memories = append(memories, *memory)
	}

	return memories, rows.Err()
}

// MarkMemoryConsolidated marks a memory as consolidated.
func (s *ReasoningStore) MarkMemoryConsolidated(memoryID, patternID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE reasoning_memories 
		SET consolidated = 1, pattern_id = ?
		WHERE memory_id = ?
	`, patternID, memoryID)

	return err
}

// SavePatternVersion saves a version snapshot.
func (s *ReasoningStore) SavePatternVersion(pattern domainNeural.ReasoningPattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(pattern)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO pattern_versions (pattern_id, version, data)
		VALUES (?, ?, ?)
	`, pattern.PatternID, pattern.Version, string(data))

	return err
}

// GetPatternVersion retrieves a specific version of a pattern.
func (s *ReasoningStore) GetPatternVersion(patternID string, version int) (*domainNeural.ReasoningPattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data string
	err := s.db.QueryRow(`
		SELECT data FROM pattern_versions
		WHERE pattern_id = ? AND version = ?
	`, patternID, version).Scan(&data)

	if err != nil {
		return nil, err
	}

	var pattern domainNeural.ReasoningPattern
	if err := json.Unmarshal([]byte(data), &pattern); err != nil {
		return nil, err
	}

	return &pattern, nil
}

// GetStats returns store statistics.
func (s *ReasoningStore) GetStats() (domainNeural.ReasoningBankStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := domainNeural.ReasoningBankStats{
		DomainDistribution: make(map[domainNeural.ReasoningDomain]int),
	}

	// Pattern count
	s.db.QueryRow("SELECT COUNT(*) FROM reasoning_patterns").Scan(&stats.PatternCount)

	// Memory count
	s.db.QueryRow("SELECT COUNT(*) FROM reasoning_memories").Scan(&stats.MemoryCount)

	// Average stats
	s.db.QueryRow("SELECT COALESCE(AVG(success_rate), 0), COALESCE(AVG(usage_count), 0) FROM reasoning_patterns").
		Scan(&stats.AvgSuccessRate, &stats.AvgUsageCount)

	// Domain distribution
	rows, _ := s.db.Query("SELECT domain, COUNT(*) FROM reasoning_patterns GROUP BY domain")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var domain string
			var count int
			rows.Scan(&domain, &count)
			stats.DomainDistribution[domainNeural.ReasoningDomain(domain)] = count
		}
	}

	return stats, nil
}

// GetPatternsForConsolidation returns patterns that may need consolidation.
func (s *ReasoningStore) GetPatternsForConsolidation() ([]domainNeural.ReasoningPattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get patterns sorted by embedding for similarity checking
	patterns, err := s.ListPatterns(nil, 0)
	if err != nil {
		return nil, err
	}

	return patterns, nil
}

// Close closes the store.
func (s *ReasoningStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// Private methods

func (s *ReasoningStore) getPatternUnlocked(patternID string) (*domainNeural.ReasoningPattern, error) {
	row := s.db.QueryRow(`
		SELECT pattern_id, name, domain, embedding, strategy, success_rate,
			usage_count, quality_history, evolution_history, version,
			key_learnings, source_trajectory_ids, last_used, created_at, updated_at
		FROM reasoning_patterns WHERE pattern_id = ?
	`, patternID)

	return s.scanPattern(row)
}

func (s *ReasoningStore) scanPattern(row *sql.Row) (*domainNeural.ReasoningPattern, error) {
	var pattern domainNeural.ReasoningPattern
	var embeddingBytes []byte
	var qualityHistoryJSON, evolutionHistoryJSON, keyLearningsJSON, sourceTrajectoryIDsJSON string
	var lastUsed, createdAt, updatedAt sql.NullTime

	err := row.Scan(
		&pattern.PatternID, &pattern.Name, &pattern.Domain, &embeddingBytes,
		&pattern.Strategy, &pattern.SuccessRate, &pattern.UsageCount,
		&qualityHistoryJSON, &evolutionHistoryJSON, &pattern.Version,
		&keyLearningsJSON, &sourceTrajectoryIDsJSON, &lastUsed, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	pattern.Embedding = bytesToFloat32Slice(embeddingBytes)
	json.Unmarshal([]byte(qualityHistoryJSON), &pattern.QualityHistory)
	json.Unmarshal([]byte(evolutionHistoryJSON), &pattern.EvolutionHistory)
	json.Unmarshal([]byte(keyLearningsJSON), &pattern.KeyLearnings)
	json.Unmarshal([]byte(sourceTrajectoryIDsJSON), &pattern.SourceTrajectoryIDs)

	if lastUsed.Valid {
		pattern.LastUsed = lastUsed.Time
	}
	if createdAt.Valid {
		pattern.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		pattern.UpdatedAt = updatedAt.Time
	}

	return &pattern, nil
}

func (s *ReasoningStore) scanPatternRows(rows *sql.Rows) (*domainNeural.ReasoningPattern, error) {
	var pattern domainNeural.ReasoningPattern
	var embeddingBytes []byte
	var qualityHistoryJSON, evolutionHistoryJSON, keyLearningsJSON, sourceTrajectoryIDsJSON string
	var lastUsed, createdAt, updatedAt sql.NullTime

	err := rows.Scan(
		&pattern.PatternID, &pattern.Name, &pattern.Domain, &embeddingBytes,
		&pattern.Strategy, &pattern.SuccessRate, &pattern.UsageCount,
		&qualityHistoryJSON, &evolutionHistoryJSON, &pattern.Version,
		&keyLearningsJSON, &sourceTrajectoryIDsJSON, &lastUsed, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	pattern.Embedding = bytesToFloat32Slice(embeddingBytes)
	json.Unmarshal([]byte(qualityHistoryJSON), &pattern.QualityHistory)
	json.Unmarshal([]byte(evolutionHistoryJSON), &pattern.EvolutionHistory)
	json.Unmarshal([]byte(keyLearningsJSON), &pattern.KeyLearnings)
	json.Unmarshal([]byte(sourceTrajectoryIDsJSON), &pattern.SourceTrajectoryIDs)

	if lastUsed.Valid {
		pattern.LastUsed = lastUsed.Time
	}
	if createdAt.Valid {
		pattern.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		pattern.UpdatedAt = updatedAt.Time
	}

	return &pattern, nil
}

func (s *ReasoningStore) scanMemory(row *sql.Row) (*domainNeural.ReasoningMemory, error) {
	var memory domainNeural.ReasoningMemory
	var embeddingBytes []byte
	var keyLearningsJSON, verdictJSON string
	var consolidated int
	var lastUsed, createdAt sql.NullTime

	err := row.Scan(
		&memory.MemoryID, &memory.TrajectoryID, &memory.Memory.Strategy,
		&keyLearningsJSON, &embeddingBytes, &memory.Memory.Quality,
		&memory.Memory.UsageCount, &consolidated, &memory.PatternID,
		&memory.RelevanceScore, &memory.DiversityScore, &verdictJSON,
		&lastUsed, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	memory.Memory.Embedding = bytesToFloat32Slice(embeddingBytes)
	memory.Consolidated = consolidated == 1
	json.Unmarshal([]byte(keyLearningsJSON), &memory.Memory.KeyLearnings)
	json.Unmarshal([]byte(verdictJSON), &memory.Verdict)

	if lastUsed.Valid {
		memory.Memory.LastUsed = lastUsed.Time
	}
	if createdAt.Valid {
		memory.CreatedAt = createdAt.Time
	}

	return &memory, nil
}

func (s *ReasoningStore) scanMemoryRows(rows *sql.Rows) (*domainNeural.ReasoningMemory, error) {
	var memory domainNeural.ReasoningMemory
	var embeddingBytes []byte
	var keyLearningsJSON, verdictJSON string
	var consolidated int
	var lastUsed, createdAt sql.NullTime

	err := rows.Scan(
		&memory.MemoryID, &memory.TrajectoryID, &memory.Memory.Strategy,
		&keyLearningsJSON, &embeddingBytes, &memory.Memory.Quality,
		&memory.Memory.UsageCount, &consolidated, &memory.PatternID,
		&memory.RelevanceScore, &memory.DiversityScore, &verdictJSON,
		&lastUsed, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	memory.Memory.Embedding = bytesToFloat32Slice(embeddingBytes)
	memory.Consolidated = consolidated == 1
	json.Unmarshal([]byte(keyLearningsJSON), &memory.Memory.KeyLearnings)
	json.Unmarshal([]byte(verdictJSON), &memory.Verdict)

	if lastUsed.Valid {
		memory.Memory.LastUsed = lastUsed.Time
	}
	if createdAt.Valid {
		memory.CreatedAt = createdAt.Time
	}

	return &memory, nil
}

func (s *ReasoningStore) updateAccessOrder(patternID string) {
	s.removeFromAccessOrder(patternID)
	s.accessOrder = append(s.accessOrder, patternID)
}

func (s *ReasoningStore) removeFromAccessOrder(patternID string) {
	for i, id := range s.accessOrder {
		if id == patternID {
			s.accessOrder = append(s.accessOrder[:i], s.accessOrder[i+1:]...)
			break
		}
	}
}

func (s *ReasoningStore) evictLRU() {
	if len(s.accessOrder) == 0 {
		return
	}

	// Evict oldest 10%
	toEvict := len(s.accessOrder) / 10
	if toEvict < 1 {
		toEvict = 1
	}

	for i := 0; i < toEvict && len(s.accessOrder) > 0; i++ {
		oldestID := s.accessOrder[0]
		s.accessOrder = s.accessOrder[1:]
		delete(s.vectorIndex, oldestID)
		s.db.Exec("DELETE FROM reasoning_patterns WHERE pattern_id = ?", oldestID)
	}
}

// CosineSimilarityFloat32Exported computes cosine similarity between two float32 vectors.
// Exported for use by application layer.
func CosineSimilarityFloat32Exported(a, b []float32) float64 {
	return cosineSimilarityFloat32(a, b)
}

// cosineSimilarityFloat32 computes cosine similarity between two float32 vectors.
func cosineSimilarityFloat32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
