// Package neural provides neural network infrastructure.
package neural

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// TrajectoryStore provides SQLite persistence for trajectories.
type TrajectoryStore struct {
	mu     sync.RWMutex
	db     *sql.DB
	config TrajectoryStoreConfig
	stats  *TrajectoryStoreStats
}

// TrajectoryStoreConfig configures the trajectory store.
type TrajectoryStoreConfig struct {
	// DBPath is the SQLite database path.
	DBPath string `json:"dbPath"`

	// MaxTrajectories is the maximum number to keep.
	MaxTrajectories int `json:"maxTrajectories"`

	// PruneAge is the age after which to prune (hours).
	PruneAge int `json:"pruneAge"`

	// MinQualityForKeep is the minimum quality to avoid pruning.
	MinQualityForKeep float64 `json:"minQualityForKeep"`

	// EnableVacuum enables periodic vacuum.
	EnableVacuum bool `json:"enableVacuum"`
}

// DefaultTrajectoryStoreConfig returns the default configuration.
func DefaultTrajectoryStoreConfig() TrajectoryStoreConfig {
	return TrajectoryStoreConfig{
		DBPath:            ":memory:",
		MaxTrajectories:   10000,
		PruneAge:          168, // 1 week
		MinQualityForKeep: 0.5,
		EnableVacuum:      true,
	}
}

// TrajectoryStoreStats contains store statistics.
type TrajectoryStoreStats struct {
	TotalTrajectories int     `json:"totalTrajectories"`
	TotalSteps        int     `json:"totalSteps"`
	AvgQuality        float64 `json:"avgQuality"`
	PrunedCount       int64   `json:"prunedCount"`
	LastPrune         time.Time `json:"lastPrune"`
}

// NewTrajectoryStore creates a new trajectory store.
func NewTrajectoryStore(config TrajectoryStoreConfig) (*TrajectoryStore, error) {
	db, err := sql.Open("sqlite", config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &TrajectoryStore{
		db:     db,
		config: config,
		stats:  &TrajectoryStoreStats{},
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema initializes the database schema.
func (s *TrajectoryStore) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS trajectories (
			trajectory_id TEXT PRIMARY KEY,
			context TEXT NOT NULL,
			domain TEXT NOT NULL,
			quality_score REAL DEFAULT 0,
			is_complete INTEGER DEFAULT 0,
			start_time INTEGER NOT NULL,
			end_time INTEGER,
			agent_id TEXT,
			total_reward REAL DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			verdict_json TEXT,
			distilled_memory_json TEXT,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS trajectory_steps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trajectory_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			step_index INTEGER NOT NULL,
			timestamp INTEGER NOT NULL,
			action TEXT NOT NULL,
			state_before BLOB,
			state_after BLOB,
			reward REAL DEFAULT 0,
			attention_weights BLOB,
			context_json TEXT,
			outcome TEXT,
			FOREIGN KEY (trajectory_id) REFERENCES trajectories(trajectory_id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS distilled_memories (
			memory_id TEXT PRIMARY KEY,
			trajectory_id TEXT NOT NULL,
			strategy TEXT NOT NULL,
			key_learnings_json TEXT,
			embedding BLOB,
			quality REAL DEFAULT 0,
			usage_count INTEGER DEFAULT 0,
			last_used INTEGER,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (trajectory_id) REFERENCES trajectories(trajectory_id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_trajectories_domain ON trajectories(domain);
		CREATE INDEX IF NOT EXISTS idx_trajectories_quality ON trajectories(quality_score);
		CREATE INDEX IF NOT EXISTS idx_trajectories_agent ON trajectories(agent_id);
		CREATE INDEX IF NOT EXISTS idx_trajectories_start ON trajectories(start_time);
		CREATE INDEX IF NOT EXISTS idx_steps_trajectory ON trajectory_steps(trajectory_id);
		CREATE INDEX IF NOT EXISTS idx_memories_trajectory ON distilled_memories(trajectory_id);
		CREATE INDEX IF NOT EXISTS idx_memories_quality ON distilled_memories(quality);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveTrajectory saves a trajectory to the store.
func (s *TrajectoryStore) SaveTrajectory(traj *domainNeural.ExtendedTrajectory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Serialize verdict
	var verdictJSON []byte
	if traj.Verdict != nil {
		verdictJSON, _ = json.Marshal(traj.Verdict)
	}

	// Serialize distilled memory
	var distilledJSON []byte
	if traj.DistilledMemory != nil {
		distilledJSON, _ = json.Marshal(traj.DistilledMemory)
	}

	var endTime *int64
	if traj.EndTime != nil {
		t := traj.EndTime.UnixMilli()
		endTime = &t
	}

	// Insert trajectory
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO trajectories 
		(trajectory_id, context, domain, quality_score, is_complete, start_time, end_time, 
		 agent_id, total_reward, duration_ms, verdict_json, distilled_memory_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		traj.TrajectoryID, traj.Context, string(traj.Domain), traj.QualityScore,
		boolToInt(traj.IsComplete), traj.StartTime.UnixMilli(), endTime,
		traj.AgentID, traj.TotalReward, traj.DurationMs,
		string(verdictJSON), string(distilledJSON), time.Now().UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert trajectory: %w", err)
	}

	// Delete existing steps
	_, err = tx.Exec("DELETE FROM trajectory_steps WHERE trajectory_id = ?", traj.TrajectoryID)
	if err != nil {
		return fmt.Errorf("failed to delete old steps: %w", err)
	}

	// Insert steps
	for i, step := range traj.Steps {
		contextJSON, _ := json.Marshal(step.Context)
		stateBefore := float32SliceToBytes(step.StateBefore)
		stateAfter := float32SliceToBytes(step.StateAfter)
		attentionWeights := float32SliceToBytes(step.AttentionWeights)

		_, err = tx.Exec(`
			INSERT INTO trajectory_steps 
			(trajectory_id, step_id, step_index, timestamp, action, state_before, state_after, 
			 reward, attention_weights, context_json, outcome)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			traj.TrajectoryID, step.StepID, i, step.Timestamp.UnixMilli(), step.Action,
			stateBefore, stateAfter, step.Reward, attentionWeights,
			string(contextJSON), step.Outcome,
		)
		if err != nil {
			return fmt.Errorf("failed to insert step: %w", err)
		}
	}

	return tx.Commit()
}

// GetTrajectory retrieves a trajectory by ID.
func (s *TrajectoryStore) GetTrajectory(trajectoryID string) (*domainNeural.ExtendedTrajectory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`
		SELECT trajectory_id, context, domain, quality_score, is_complete, start_time, end_time,
		       agent_id, total_reward, duration_ms, verdict_json, distilled_memory_json
		FROM trajectories WHERE trajectory_id = ?`, trajectoryID)

	var traj domainNeural.ExtendedTrajectory
	var domain string
	var isComplete int
	var startTimeMs, endTimeMs sql.NullInt64
	var verdictJSON, distilledJSON sql.NullString

	err := row.Scan(
		&traj.TrajectoryID, &traj.Context, &domain, &traj.QualityScore,
		&isComplete, &startTimeMs, &endTimeMs, &traj.AgentID, &traj.TotalReward,
		&traj.DurationMs, &verdictJSON, &distilledJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query trajectory: %w", err)
	}

	traj.Domain = domainNeural.TrajectoryDomain(domain)
	traj.IsComplete = isComplete != 0
	traj.StartTime = time.UnixMilli(startTimeMs.Int64)
	if endTimeMs.Valid {
		t := time.UnixMilli(endTimeMs.Int64)
		traj.EndTime = &t
	}

	if verdictJSON.Valid && verdictJSON.String != "" {
		var verdict domainNeural.TrajectoryVerdict
		if err := json.Unmarshal([]byte(verdictJSON.String), &verdict); err == nil {
			traj.Verdict = &verdict
		}
	}

	if distilledJSON.Valid && distilledJSON.String != "" {
		var distilled domainNeural.DistilledMemory
		if err := json.Unmarshal([]byte(distilledJSON.String), &distilled); err == nil {
			traj.DistilledMemory = &distilled
		}
	}

	// Load steps
	rows, err := s.db.Query(`
		SELECT step_id, timestamp, action, state_before, state_after, reward, 
		       attention_weights, context_json, outcome
		FROM trajectory_steps WHERE trajectory_id = ? ORDER BY step_index`, trajectoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to query steps: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var step domainNeural.ExtendedTrajectoryStep
		var timestampMs int64
		var stateBefore, stateAfter, attentionWeights []byte
		var contextJSON sql.NullString

		err := rows.Scan(
			&step.StepID, &timestampMs, &step.Action, &stateBefore, &stateAfter,
			&step.Reward, &attentionWeights, &contextJSON, &step.Outcome,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan step: %w", err)
		}

		step.Timestamp = time.UnixMilli(timestampMs)
		step.StateBefore = bytesToFloat32Slice(stateBefore)
		step.StateAfter = bytesToFloat32Slice(stateAfter)
		step.AttentionWeights = bytesToFloat32Slice(attentionWeights)

		if contextJSON.Valid && contextJSON.String != "" {
			var ctx map[string]interface{}
			if err := json.Unmarshal([]byte(contextJSON.String), &ctx); err == nil {
				step.Context = ctx
			}
		}

		traj.Steps = append(traj.Steps, step)
	}

	return &traj, nil
}

// QueryTrajectories queries trajectories based on criteria.
func (s *TrajectoryStore) QueryTrajectories(query domainNeural.TrajectoryQuery) ([]*domainNeural.ExtendedTrajectory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sql := "SELECT trajectory_id FROM trajectories WHERE 1=1"
	args := make([]interface{}, 0)

	if query.Domain != nil {
		sql += " AND domain = ?"
		args = append(args, string(*query.Domain))
	}
	if query.AgentID != "" {
		sql += " AND agent_id = ?"
		args = append(args, query.AgentID)
	}
	if query.MinQuality > 0 {
		sql += " AND quality_score >= ?"
		args = append(args, query.MinQuality)
	}
	if query.SuccessOnly {
		sql += " AND json_extract(verdict_json, '$.success') = 1"
	}
	if query.StartAfter != nil {
		sql += " AND start_time >= ?"
		args = append(args, query.StartAfter.UnixMilli())
	}
	if query.StartBefore != nil {
		sql += " AND start_time <= ?"
		args = append(args, query.StartBefore.UnixMilli())
	}

	// Order by
	orderBy := "start_time"
	if query.OrderBy != "" {
		orderBy = query.OrderBy
	}
	if query.Descending {
		sql += fmt.Sprintf(" ORDER BY %s DESC", orderBy)
	} else {
		sql += fmt.Sprintf(" ORDER BY %s ASC", orderBy)
	}

	// Limit and offset
	if query.Limit > 0 {
		sql += " LIMIT ?"
		args = append(args, query.Limit)
	}
	if query.Offset > 0 {
		sql += " OFFSET ?"
		args = append(args, query.Offset)
	}

	rows, err := s.db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query trajectories: %w", err)
	}
	defer rows.Close()

	var trajectories []*domainNeural.ExtendedTrajectory
	for rows.Next() {
		var trajectoryID string
		if err := rows.Scan(&trajectoryID); err != nil {
			continue
		}

		// Use RUnlock/RLock dance to avoid deadlock
		s.mu.RUnlock()
		traj, err := s.GetTrajectory(trajectoryID)
		s.mu.RLock()
		if err == nil && traj != nil {
			trajectories = append(trajectories, traj)
		}
	}

	return trajectories, nil
}

// FindSimilarTrajectories finds trajectories similar to the given embedding.
func (s *TrajectoryStore) FindSimilarTrajectories(embedding []float32, k int, minQuality float64) ([]domainNeural.TrajectorySearchResult, error) {
	startTime := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Load all trajectories with steps that have embeddings
	rows, err := s.db.Query(`
		SELECT DISTINCT t.trajectory_id 
		FROM trajectories t
		JOIN trajectory_steps s ON t.trajectory_id = s.trajectory_id
		WHERE t.quality_score >= ? AND s.state_after IS NOT NULL
		ORDER BY t.quality_score DESC
		LIMIT ?`, minQuality, k*10) // Get more than needed for filtering
	if err != nil {
		return nil, fmt.Errorf("failed to query trajectories: %w", err)
	}
	defer rows.Close()

	type scoredTraj struct {
		trajectoryID string
		similarity   float64
	}
	var scored []scoredTraj

	for rows.Next() {
		var trajectoryID string
		if err := rows.Scan(&trajectoryID); err != nil {
			continue
		}

		// Get last step's embedding for comparison
		var stateAfter []byte
		err := s.db.QueryRow(`
			SELECT state_after FROM trajectory_steps 
			WHERE trajectory_id = ? AND state_after IS NOT NULL
			ORDER BY step_index DESC LIMIT 1`, trajectoryID).Scan(&stateAfter)
		if err != nil {
			continue
		}

		trajEmbedding := bytesToFloat32Slice(stateAfter)
		if len(trajEmbedding) > 0 {
			sim := cosineSimilarityFloat32(embedding, trajEmbedding)
			scored = append(scored, scoredTraj{trajectoryID, sim})
		}
	}

	// Sort by similarity
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].similarity > scored[i].similarity {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Take top k
	if len(scored) > k {
		scored = scored[:k]
	}

	latencyMs := float64(time.Since(startTime).Microseconds()) / 1000.0

	results := make([]domainNeural.TrajectorySearchResult, 0, len(scored))
	for _, s := range scored {
		traj, err := s.GetTrajectory(s.trajectoryID)
		if err == nil && traj != nil {
			results = append(results, domainNeural.TrajectorySearchResult{
				Trajectory: traj,
				Similarity: s.similarity,
				LatencyMs:  latencyMs,
			})
		}
	}

	return results, nil
}

// GetBestTrajectory returns the highest quality trajectory matching criteria.
func (s *TrajectoryStore) GetBestTrajectory(domain *domainNeural.TrajectoryDomain, agentID string) (*domainNeural.ExtendedTrajectory, error) {
	query := domainNeural.TrajectoryQuery{
		Domain:      domain,
		AgentID:     agentID,
		SuccessOnly: true,
		OrderBy:     "quality_score",
		Descending:  true,
		Limit:       1,
	}

	trajectories, err := s.QueryTrajectories(query)
	if err != nil {
		return nil, err
	}

	if len(trajectories) == 0 {
		return nil, nil
	}

	return trajectories[0], nil
}

// PruneOldTrajectories removes old, low-quality trajectories.
func (s *TrajectoryStore) PruneOldTrajectories() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoffTime := time.Now().Add(-time.Duration(s.config.PruneAge) * time.Hour).UnixMilli()

	result, err := s.db.Exec(`
		DELETE FROM trajectories 
		WHERE start_time < ? AND quality_score < ?`,
		cutoffTime, s.config.MinQualityForKeep)
	if err != nil {
		return 0, fmt.Errorf("failed to prune trajectories: %w", err)
	}

	pruned, _ := result.RowsAffected()
	s.stats.PrunedCount += pruned
	s.stats.LastPrune = time.Now()

	// Vacuum if enabled
	if s.config.EnableVacuum && pruned > 100 {
		s.db.Exec("VACUUM")
	}

	return pruned, nil
}

// SaveDistilledMemory saves a distilled memory.
func (s *TrajectoryStore) SaveDistilledMemory(memory *domainNeural.DistilledMemory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyLearningsJSON, _ := json.Marshal(memory.KeyLearnings)
	embedding := float32SliceToBytes(memory.Embedding)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO distilled_memories
		(memory_id, trajectory_id, strategy, key_learnings_json, embedding, 
		 quality, usage_count, last_used, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		memory.MemoryID, memory.TrajectoryID, memory.Strategy,
		string(keyLearningsJSON), embedding, memory.Quality,
		memory.UsageCount, memory.LastUsed.UnixMilli(), memory.CreatedAt.UnixMilli(),
	)

	return err
}

// GetDistilledMemories returns distilled memories above a quality threshold.
func (s *TrajectoryStore) GetDistilledMemories(minQuality float64, limit int) ([]*domainNeural.DistilledMemory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT memory_id, trajectory_id, strategy, key_learnings_json, embedding,
		       quality, usage_count, last_used, created_at
		FROM distilled_memories
		WHERE quality >= ?
		ORDER BY quality DESC
		LIMIT ?`, minQuality, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}
	defer rows.Close()

	var memories []*domainNeural.DistilledMemory
	for rows.Next() {
		var memory domainNeural.DistilledMemory
		var keyLearningsJSON string
		var embedding []byte
		var lastUsedMs, createdAtMs int64

		err := rows.Scan(
			&memory.MemoryID, &memory.TrajectoryID, &memory.Strategy,
			&keyLearningsJSON, &embedding, &memory.Quality,
			&memory.UsageCount, &lastUsedMs, &createdAtMs,
		)
		if err != nil {
			continue
		}

		json.Unmarshal([]byte(keyLearningsJSON), &memory.KeyLearnings)
		memory.Embedding = bytesToFloat32Slice(embedding)
		memory.LastUsed = time.UnixMilli(lastUsedMs)
		memory.CreatedAt = time.UnixMilli(createdAtMs)

		memories = append(memories, &memory)
	}

	return memories, nil
}

// GetStats returns store statistics.
func (s *TrajectoryStore) GetStats() (*TrajectoryStoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalTraj, totalSteps int
	var avgQuality float64

	s.db.QueryRow("SELECT COUNT(*) FROM trajectories").Scan(&totalTraj)
	s.db.QueryRow("SELECT COUNT(*) FROM trajectory_steps").Scan(&totalSteps)
	s.db.QueryRow("SELECT COALESCE(AVG(quality_score), 0) FROM trajectories").Scan(&avgQuality)

	s.stats.TotalTrajectories = totalTraj
	s.stats.TotalSteps = totalSteps
	s.stats.AvgQuality = avgQuality

	return s.stats, nil
}

// Close closes the store.
func (s *TrajectoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// Helper functions

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func float32SliceToBytes(slice []float32) []byte {
	if len(slice) == 0 {
		return nil
	}
	bytes := make([]byte, len(slice)*4)
	for i, v := range slice {
		bits := math.Float32bits(v)
		bytes[i*4] = byte(bits)
		bytes[i*4+1] = byte(bits >> 8)
		bytes[i*4+2] = byte(bits >> 16)
		bytes[i*4+3] = byte(bits >> 24)
	}
	return bytes
}

func bytesToFloat32Slice(bytes []byte) []float32 {
	if len(bytes) == 0 || len(bytes)%4 != 0 {
		return nil
	}
	slice := make([]float32, len(bytes)/4)
	for i := range slice {
		bits := uint32(bytes[i*4]) |
			uint32(bytes[i*4+1])<<8 |
			uint32(bytes[i*4+2])<<16 |
			uint32(bytes[i*4+3])<<24
		slice[i] = math.Float32frombits(bits)
	}
	return slice
}

func cosineSimilarityFloat32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
