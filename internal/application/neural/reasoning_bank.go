// Package neural provides neural learning application services.
package neural

import (
	"fmt"
	"strings"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
)

// ReasoningBank orchestrates pattern storage, retrieval, and learning.
// Implements the 4-step pipeline: RETRIEVE → JUDGE → DISTILL → CONSOLIDATE.
type ReasoningBank struct {
	mu         sync.RWMutex
	config     domainNeural.ReasoningConfig
	store      *infraNeural.ReasoningStore
	serializer *infraNeural.ReasoningSerializer

	// Statistics
	retrievalCount      int64
	distillCount        int64
	consolidationCount  int64
	avgRetrievalLatency float64
	avgDistillLatency   float64
	lastConsolidation   *time.Time
}

// NewReasoningBank creates a new ReasoningBank.
func NewReasoningBank(config domainNeural.ReasoningConfig) (*ReasoningBank, error) {
	store, err := infraNeural.NewReasoningStore(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create reasoning store: %w", err)
	}

	return &ReasoningBank{
		config:     config,
		store:      store,
		serializer: infraNeural.NewReasoningSerializer(),
	}, nil
}

// Close closes the ReasoningBank.
func (rb *ReasoningBank) Close() error {
	return rb.store.Close()
}

// ============================================================================
// STEP 1: RETRIEVE - Find relevant patterns using MMR
// ============================================================================

// Retrieve finds patterns similar to the query embedding.
// Target: <1ms
func (rb *ReasoningBank) Retrieve(queryEmbedding []float32, k int) []domainNeural.RetrievalResult {
	startTime := time.Now()

	rb.mu.Lock()
	rb.retrievalCount++
	rb.mu.Unlock()

	if k <= 0 {
		k = rb.config.RetrievalK
	}

	results := rb.store.SearchSimilarMMR(
		queryEmbedding,
		k,
		rb.config.MinRelevanceThreshold,
		rb.config.MMRLambda,
	)

	// Update usage for retrieved patterns
	for _, result := range results {
		pattern := result.Pattern
		pattern.RecordUsage()
		rb.store.SavePattern(pattern)
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0

	rb.mu.Lock()
	rb.avgRetrievalLatency = (rb.avgRetrievalLatency*float64(rb.retrievalCount-1) + elapsed) / float64(rb.retrievalCount)
	rb.mu.Unlock()

	return results
}

// RetrieveByContent retrieves patterns by text content similarity.
func (rb *ReasoningBank) RetrieveByContent(content string, k int) []domainNeural.RetrievalResult {
	rb.mu.Lock()
	rb.retrievalCount++
	rb.mu.Unlock()

	if k <= 0 {
		k = rb.config.RetrievalK
	}

	// Get all patterns and compute content similarity
	patterns, err := rb.store.ListPatterns(nil, 0)
	if err != nil {
		return nil
	}

	type scoredPattern struct {
		pattern domainNeural.ReasoningPattern
		score   float64
	}

	scored := make([]scoredPattern, 0, len(patterns))
	for _, p := range patterns {
		sim := rb.computeContentSimilarity(content, p.Strategy)
		if sim >= rb.config.MinRelevanceThreshold {
			scored = append(scored, scoredPattern{p, sim})
		}
	}

	// Sort by score
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Take top k
	if len(scored) > k {
		scored = scored[:k]
	}

	results := make([]domainNeural.RetrievalResult, len(scored))
	for i, sp := range scored {
		results[i] = domainNeural.RetrievalResult{
			Pattern:        sp.pattern,
			RelevanceScore: sp.score,
			DiversityScore: 1.0,
			CombinedScore:  sp.score,
		}
	}

	return results
}

// ============================================================================
// STEP 2: JUDGE - Evaluate trajectory quality
// ============================================================================

// Judge evaluates a trajectory and returns a verdict.
func (rb *ReasoningBank) Judge(trajectory domainNeural.ExtendedTrajectory) *domainNeural.TrajectoryVerdict {
	verdict := &domainNeural.TrajectoryVerdict{
		JudgedAt: time.Now(),
	}

	// Compute quality metrics
	verdict.Success = trajectory.QualityScore >= rb.config.DistillationThreshold

	// Analyze strengths
	verdict.Strengths = rb.identifyStrengths(trajectory)

	// Analyze weaknesses
	verdict.Weaknesses = rb.identifyWeaknesses(trajectory)

	// Suggest improvements
	verdict.Improvements = rb.suggestImprovements(trajectory)

	// Compute confidence based on trajectory completeness
	verdict.Confidence = rb.computeConfidence(trajectory)

	// Compute relevance score based on domain
	verdict.RelevanceScore = rb.computeRelevanceScore(trajectory)

	return verdict
}

// ============================================================================
// STEP 3: DISTILL - Extract memories from successful trajectories
// ============================================================================

// Distill extracts a reasoning memory from a trajectory.
// Target: <5ms
func (rb *ReasoningBank) Distill(trajectory domainNeural.ExtendedTrajectory, verdict *domainNeural.TrajectoryVerdict) (*domainNeural.ReasoningMemory, error) {
	startTime := time.Now()

	rb.mu.Lock()
	rb.distillCount++
	rb.mu.Unlock()

	// Only distill if quality meets threshold
	if trajectory.QualityScore < rb.config.DistillationThreshold {
		return nil, fmt.Errorf("trajectory quality %.2f below threshold %.2f",
			trajectory.QualityScore, rb.config.DistillationThreshold)
	}

	// Extract strategy from trajectory
	strategy := rb.extractStrategy(trajectory)

	// Extract key learnings
	keyLearnings := rb.extractKeyLearnings(trajectory)

	// Compute aggregate embedding (weighted average of step embeddings)
	embedding := rb.computeAggregateEmbedding(trajectory)

	// Create distilled memory
	memory := &domainNeural.ReasoningMemory{
		MemoryID:     fmt.Sprintf("mem_%d", time.Now().UnixNano()),
		TrajectoryID: trajectory.TrajectoryID,
		Memory: domainNeural.DistilledMemory{
			MemoryID:     fmt.Sprintf("mem_%d", time.Now().UnixNano()),
			TrajectoryID: trajectory.TrajectoryID,
			Strategy:     strategy,
			KeyLearnings: keyLearnings,
			Embedding:    embedding,
			Quality:      trajectory.QualityScore,
			UsageCount:   0,
			LastUsed:     time.Now(),
			CreatedAt:    time.Now(),
		},
		Verdict:      verdict,
		Consolidated: false,
		CreatedAt:    time.Now(),
	}

	// Save memory
	if err := rb.store.SaveMemory(*memory); err != nil {
		return nil, fmt.Errorf("failed to save memory: %w", err)
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0

	rb.mu.Lock()
	rb.avgDistillLatency = (rb.avgDistillLatency*float64(rb.distillCount-1) + elapsed) / float64(rb.distillCount)
	rb.mu.Unlock()

	return memory, nil
}

// ============================================================================
// STEP 4: CONSOLIDATE - Deduplicate, merge, and prune patterns
// ============================================================================

// Consolidate performs pattern consolidation.
// Target: <100ms
func (rb *ReasoningBank) Consolidate() domainNeural.ConsolidationResult {
	startTime := time.Now()

	rb.mu.Lock()
	rb.consolidationCount++
	rb.mu.Unlock()

	result := domainNeural.ConsolidationResult{
		Timestamp: time.Now(),
	}

	// Get all patterns for consolidation
	patterns, err := rb.store.GetPatternsForConsolidation()
	if err != nil {
		return result
	}

	// Step 1: Deduplication
	result.Deduplicated = rb.deduplicatePatterns(patterns)

	// Step 2: Merge similar patterns
	result.Merged = rb.mergeSimilarPatterns(patterns)

	// Step 3: Prune old/unused patterns
	result.Pruned = rb.prunePatterns(patterns)

	// Step 4: Detect contradictions
	result.ContradictionsFound = rb.detectContradictions(patterns)

	result.DurationMs = time.Since(startTime).Milliseconds()

	now := time.Now()
	rb.mu.Lock()
	rb.lastConsolidation = &now
	rb.mu.Unlock()

	return result
}

// ============================================================================
// Pattern Evolution
// ============================================================================

// EvolvePattern updates a pattern based on new outcome.
func (rb *ReasoningBank) EvolvePattern(patternID string, outcome domainNeural.LearningOutcome) (*domainNeural.PatternEvolution, error) {
	pattern, err := rb.store.GetPattern(patternID)
	if err != nil {
		return nil, fmt.Errorf("pattern not found: %w", err)
	}

	// Save version before evolution
	rb.store.SavePatternVersion(*pattern)

	// Create evolution record
	evolution := domainNeural.PatternEvolution{
		EvolutionID:       fmt.Sprintf("evo_%d", time.Now().UnixNano()),
		PatternID:         patternID,
		Type:              domainNeural.EvolutionImprovement,
		PreviousVersion:   pattern.Version,
		NewVersion:        pattern.Version + 1,
		QualityBefore:     pattern.SuccessRate,
		SuccessRateBefore: pattern.SuccessRate,
		Reason:            outcome.Feedback,
		Timestamp:         time.Now(),
	}

	// Update pattern quality history
	pattern.UpdateQualityHistory(outcome.QualityScore)

	// Update evolution record with new values
	evolution.QualityAfter = pattern.SuccessRate
	evolution.SuccessRateAfter = pattern.SuccessRate

	// Add evolution to pattern
	pattern.AddEvolution(evolution)

	// Save updated pattern
	if err := rb.store.SavePattern(*pattern); err != nil {
		return nil, fmt.Errorf("failed to save pattern: %w", err)
	}

	return &evolution, nil
}

// Learn processes a trajectory through the full pipeline.
func (rb *ReasoningBank) Learn(trajectory domainNeural.ExtendedTrajectory) (*domainNeural.LearningOutcome, error) {
	outcome := &domainNeural.LearningOutcome{
		Success:      false,
		QualityScore: trajectory.QualityScore,
	}

	// Step 1: Judge the trajectory
	verdict := rb.Judge(trajectory)
	outcome.Success = verdict.Success

	// Step 2: If successful, distill to memory
	if verdict.Success {
		memory, err := rb.Distill(trajectory, verdict)
		if err != nil {
			return outcome, nil // Not an error, just skip distillation
		}

		// Step 3: Create or update pattern from memory
		pattern, created := rb.createOrUpdatePattern(memory)
		outcome.PatternID = pattern.PatternID
		outcome.PatternUpdated = !created
		outcome.NewPatternCreated = created
	}

	return outcome, nil
}

// ============================================================================
// Export/Import
// ============================================================================

// Export exports all patterns to CFP format.
func (rb *ReasoningBank) Export(metadata infraNeural.ExportMetadata) (*transfer.CFPFormat, error) {
	patterns, err := rb.store.ListPatterns(nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list patterns: %w", err)
	}

	return rb.serializer.ExportToCFP(patterns, metadata)
}

// Import imports patterns from CFP format.
func (rb *ReasoningBank) Import(cfp *transfer.CFPFormat) (int, error) {
	patterns, err := rb.serializer.ImportFromCFP(cfp)
	if err != nil {
		return 0, fmt.Errorf("failed to import from CFP: %w", err)
	}

	imported := 0
	for _, pattern := range patterns {
		if err := rb.store.SavePattern(pattern); err == nil {
			imported++
		}
	}

	return imported, nil
}

// ============================================================================
// Statistics
// ============================================================================

// GetStats returns ReasoningBank statistics.
func (rb *ReasoningBank) GetStats() domainNeural.ReasoningBankStats {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	stats, _ := rb.store.GetStats()
	stats.RetrievalCount = rb.retrievalCount
	stats.DistillCount = rb.distillCount
	stats.ConsolidationCount = rb.consolidationCount
	stats.AvgRetrievalLatencyMs = rb.avgRetrievalLatency
	stats.AvgDistillLatencyMs = rb.avgDistillLatency
	stats.LastConsolidation = rb.lastConsolidation

	return stats
}

// GetPattern retrieves a pattern by ID.
func (rb *ReasoningBank) GetPattern(patternID string) (*domainNeural.ReasoningPattern, error) {
	return rb.store.GetPattern(patternID)
}

// SavePattern saves a pattern.
func (rb *ReasoningBank) SavePattern(pattern domainNeural.ReasoningPattern) error {
	return rb.store.SavePattern(pattern)
}

// DeletePattern deletes a pattern.
func (rb *ReasoningBank) DeletePattern(patternID string) error {
	return rb.store.DeletePattern(patternID)
}

// ListPatterns lists patterns.
func (rb *ReasoningBank) ListPatterns(domain *domainNeural.ReasoningDomain, limit int) ([]domainNeural.ReasoningPattern, error) {
	return rb.store.ListPatterns(domain, limit)
}

// ============================================================================
// Private methods
// ============================================================================

func (rb *ReasoningBank) computeContentSimilarity(content, strategy string) float64 {
	if content == "" || strategy == "" {
		return 0
	}

	// Simple word overlap similarity
	contentWords := strings.Fields(strings.ToLower(content))
	strategyWords := strings.Fields(strings.ToLower(strategy))

	if len(contentWords) == 0 || len(strategyWords) == 0 {
		return 0
	}

	wordSet := make(map[string]bool)
	for _, w := range contentWords {
		wordSet[w] = true
	}

	var overlap int
	for _, w := range strategyWords {
		if wordSet[w] {
			overlap++
		}
	}

	// Jaccard similarity
	union := len(contentWords) + len(strategyWords) - overlap
	if union == 0 {
		return 0
	}

	return float64(overlap) / float64(union)
}

func (rb *ReasoningBank) identifyStrengths(trajectory domainNeural.ExtendedTrajectory) []string {
	strengths := make([]string, 0)

	if trajectory.QualityScore >= 0.8 {
		strengths = append(strengths, "High quality execution")
	}

	if len(trajectory.Steps) > 0 && len(trajectory.Steps) <= 5 {
		strengths = append(strengths, "Efficient step count")
	}

	if trajectory.TotalReward >= 0.7 {
		strengths = append(strengths, "Strong reward accumulation")
	}

	if trajectory.IsComplete {
		strengths = append(strengths, "Complete trajectory")
	}

	return strengths
}

func (rb *ReasoningBank) identifyWeaknesses(trajectory domainNeural.ExtendedTrajectory) []string {
	weaknesses := make([]string, 0)

	if trajectory.QualityScore < 0.5 {
		weaknesses = append(weaknesses, "Low quality score")
	}

	if len(trajectory.Steps) > 10 {
		weaknesses = append(weaknesses, "Too many steps")
	}

	if trajectory.TotalReward < 0.3 {
		weaknesses = append(weaknesses, "Low reward accumulation")
	}

	if !trajectory.IsComplete {
		weaknesses = append(weaknesses, "Incomplete trajectory")
	}

	return weaknesses
}

func (rb *ReasoningBank) suggestImprovements(trajectory domainNeural.ExtendedTrajectory) []string {
	improvements := make([]string, 0)

	if trajectory.QualityScore < 0.6 {
		improvements = append(improvements, "Focus on improving task completion quality")
	}

	if len(trajectory.Steps) > 8 {
		improvements = append(improvements, "Try to reduce number of steps")
	}

	if trajectory.DurationMs > 30000 {
		improvements = append(improvements, "Optimize for faster execution")
	}

	return improvements
}

func (rb *ReasoningBank) computeConfidence(trajectory domainNeural.ExtendedTrajectory) float64 {
	confidence := 0.5 // Base confidence

	if trajectory.IsComplete {
		confidence += 0.2
	}

	if len(trajectory.Steps) > 0 {
		confidence += 0.1
	}

	if trajectory.Verdict != nil {
		confidence += 0.1
	}

	if trajectory.QualityScore > 0 {
		confidence += 0.1 * trajectory.QualityScore
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func (rb *ReasoningBank) computeRelevanceScore(trajectory domainNeural.ExtendedTrajectory) float64 {
	// Base relevance
	relevance := 0.5

	// Boost for reasoning domain
	if trajectory.Domain == domainNeural.DomainReasoning {
		relevance += 0.2
	}

	// Boost for quality
	relevance += 0.3 * trajectory.QualityScore

	if relevance > 1.0 {
		relevance = 1.0
	}

	return relevance
}

func (rb *ReasoningBank) extractStrategy(trajectory domainNeural.ExtendedTrajectory) string {
	if len(trajectory.Steps) == 0 {
		return trajectory.Context
	}

	// Extract action sequence as strategy
	var actions []string
	for _, step := range trajectory.Steps {
		if step.Action != "" {
			actions = append(actions, step.Action)
		}
	}

	if len(actions) == 0 {
		return trajectory.Context
	}

	return strings.Join(actions, " -> ")
}

func (rb *ReasoningBank) extractKeyLearnings(trajectory domainNeural.ExtendedTrajectory) []string {
	learnings := make([]string, 0)

	// Extract from existing distilled memory if present
	if trajectory.DistilledMemory != nil {
		learnings = append(learnings, trajectory.DistilledMemory.KeyLearnings...)
	}

	// Extract from verdict improvements
	if trajectory.Verdict != nil {
		for _, improvement := range trajectory.Verdict.Improvements {
			learnings = append(learnings, improvement)
		}
	}

	// Generate learnings from trajectory
	if trajectory.QualityScore >= 0.8 {
		learnings = append(learnings, fmt.Sprintf("High-quality approach for %s domain", trajectory.Domain))
	}

	if len(trajectory.Steps) <= 3 && trajectory.QualityScore >= 0.6 {
		learnings = append(learnings, "Efficient solution with minimal steps")
	}

	return learnings
}

func (rb *ReasoningBank) computeAggregateEmbedding(trajectory domainNeural.ExtendedTrajectory) []float32 {
	if len(trajectory.Steps) == 0 {
		return make([]float32, rb.config.VectorDimension)
	}

	// Weighted average of step embeddings
	dim := rb.config.VectorDimension
	aggregate := make([]float32, dim)
	var totalWeight float64

	for i, step := range trajectory.Steps {
		embedding := step.StateAfter
		if len(embedding) == 0 {
			embedding = step.StateBefore
		}
		if len(embedding) == 0 {
			continue
		}

		// Weight by position (later steps weighted more)
		weight := float64(i+1) / float64(len(trajectory.Steps))

		// Weight by reward
		if step.Reward > 0 {
			weight *= (1 + step.Reward)
		}

		for j := 0; j < len(embedding) && j < dim; j++ {
			aggregate[j] += float32(weight) * embedding[j]
		}
		totalWeight += weight
	}

	// Normalize
	if totalWeight > 0 {
		for i := range aggregate {
			aggregate[i] /= float32(totalWeight)
		}
	}

	return aggregate
}

func (rb *ReasoningBank) createOrUpdatePattern(memory *domainNeural.ReasoningMemory) (*domainNeural.ReasoningPattern, bool) {
	// Check if similar pattern exists
	existingPatterns := rb.store.SearchSimilar(memory.Memory.Embedding, 1, rb.config.MergeThreshold)

	if len(existingPatterns) > 0 {
		// Update existing pattern
		existing := existingPatterns[0].Pattern
		existing.UpdateQualityHistory(memory.Memory.Quality)
		existing.RecordUsage()
		existing.SourceTrajectoryIDs = append(existing.SourceTrajectoryIDs, memory.TrajectoryID)

		// Merge key learnings
		learningsSet := make(map[string]bool)
		for _, kl := range existing.KeyLearnings {
			learningsSet[kl] = true
		}
		for _, kl := range memory.Memory.KeyLearnings {
			if !learningsSet[kl] {
				existing.KeyLearnings = append(existing.KeyLearnings, kl)
			}
		}

		rb.store.SavePattern(existing)
		rb.store.MarkMemoryConsolidated(memory.MemoryID, existing.PatternID)

		return &existing, false
	}

	// Create new pattern
	pattern := domainNeural.ReasoningPattern{
		PatternID:           fmt.Sprintf("pat_%d", time.Now().UnixNano()),
		Name:                fmt.Sprintf("Pattern from %s", memory.TrajectoryID),
		Domain:              domainNeural.ReasoningDomainGeneral,
		Embedding:           memory.Memory.Embedding,
		Strategy:            memory.Memory.Strategy,
		SuccessRate:         memory.Memory.Quality,
		UsageCount:          1,
		QualityHistory:      []float64{memory.Memory.Quality},
		EvolutionHistory:    []domainNeural.PatternEvolution{},
		Version:             1,
		KeyLearnings:        memory.Memory.KeyLearnings,
		SourceTrajectoryIDs: []string{memory.TrajectoryID},
		LastUsed:            time.Now(),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	// Add creation evolution
	pattern.EvolutionHistory = append(pattern.EvolutionHistory, domainNeural.PatternEvolution{
		EvolutionID:     fmt.Sprintf("evo_%d", time.Now().UnixNano()),
		PatternID:       pattern.PatternID,
		Type:            domainNeural.EvolutionCreate,
		PreviousVersion: 0,
		NewVersion:      1,
		QualityAfter:    memory.Memory.Quality,
		Reason:          "Created from distilled memory",
		Timestamp:       time.Now(),
	})

	rb.store.SavePattern(pattern)
	rb.store.MarkMemoryConsolidated(memory.MemoryID, pattern.PatternID)

	return &pattern, true
}

func (rb *ReasoningBank) deduplicatePatterns(patterns []domainNeural.ReasoningPattern) int {
	deduplicated := 0
	seen := make(map[string]bool)

	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			if seen[patterns[j].PatternID] {
				continue
			}

			// Check similarity
			sim := rb.computeEmbeddingSimilarity(patterns[i].Embedding, patterns[j].Embedding)
			if sim >= rb.config.DedupThreshold {
				// Keep the one with higher success rate
				if patterns[i].SuccessRate >= patterns[j].SuccessRate {
					rb.store.DeletePattern(patterns[j].PatternID)
					seen[patterns[j].PatternID] = true
				} else {
					rb.store.DeletePattern(patterns[i].PatternID)
					seen[patterns[i].PatternID] = true
				}
				deduplicated++
			}
		}
	}

	return deduplicated
}

func (rb *ReasoningBank) mergeSimilarPatterns(patterns []domainNeural.ReasoningPattern) int {
	merged := 0

	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			// Check if same domain and similar
			if patterns[i].Domain != patterns[j].Domain {
				continue
			}

			sim := rb.computeEmbeddingSimilarity(patterns[i].Embedding, patterns[j].Embedding)
			if sim >= rb.config.MergeThreshold && sim < rb.config.DedupThreshold {
				// Merge j into i
				rb.mergePatterns(&patterns[i], &patterns[j])
				rb.store.SavePattern(patterns[i])
				rb.store.DeletePattern(patterns[j].PatternID)
				merged++
			}
		}
	}

	return merged
}

func (rb *ReasoningBank) mergePatterns(target, source *domainNeural.ReasoningPattern) {
	// Merge quality histories
	target.QualityHistory = append(target.QualityHistory, source.QualityHistory...)
	if len(target.QualityHistory) > 100 {
		target.QualityHistory = target.QualityHistory[len(target.QualityHistory)-100:]
	}

	// Merge key learnings
	learningsSet := make(map[string]bool)
	for _, kl := range target.KeyLearnings {
		learningsSet[kl] = true
	}
	for _, kl := range source.KeyLearnings {
		if !learningsSet[kl] {
			target.KeyLearnings = append(target.KeyLearnings, kl)
		}
	}

	// Merge source trajectory IDs
	target.SourceTrajectoryIDs = append(target.SourceTrajectoryIDs, source.SourceTrajectoryIDs...)

	// Update usage count
	target.UsageCount += source.UsageCount

	// Average embeddings
	for i := range target.Embedding {
		if i < len(source.Embedding) {
			target.Embedding[i] = (target.Embedding[i] + source.Embedding[i]) / 2
		}
	}

	// Add merge evolution
	evolution := domainNeural.PatternEvolution{
		EvolutionID:       fmt.Sprintf("evo_%d", time.Now().UnixNano()),
		PatternID:         target.PatternID,
		Type:              domainNeural.EvolutionMerge,
		PreviousVersion:   target.Version,
		NewVersion:        target.Version + 1,
		MergedPatternIDs:  []string{source.PatternID},
		Reason:            "Merged similar pattern",
		Timestamp:         time.Now(),
	}
	target.AddEvolution(evolution)
}

func (rb *ReasoningBank) prunePatterns(patterns []domainNeural.ReasoningPattern) int {
	pruned := 0
	now := time.Now()
	maxAge := time.Duration(rb.config.MaxPatternAgeDays) * 24 * time.Hour

	for _, pattern := range patterns {
		age := now.Sub(pattern.UpdatedAt)

		// Prune if old and low usage
		if age > maxAge && pattern.UsageCount < rb.config.MinUsageForRetention {
			rb.store.DeletePattern(pattern.PatternID)
			pruned++
		}
	}

	return pruned
}

func (rb *ReasoningBank) detectContradictions(patterns []domainNeural.ReasoningPattern) int {
	contradictions := 0

	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			// Check if similar context but opposite outcomes
			sim := rb.computeEmbeddingSimilarity(patterns[i].Embedding, patterns[j].Embedding)
			if sim >= 0.8 {
				// Check if success rates are very different
				if (patterns[i].SuccessRate >= 0.7 && patterns[j].SuccessRate <= 0.3) ||
					(patterns[j].SuccessRate >= 0.7 && patterns[i].SuccessRate <= 0.3) {
					contradictions++
				}
			}
		}
	}

	return contradictions
}

func (rb *ReasoningBank) computeEmbeddingSimilarity(a, b []float32) float64 {
	return infraNeural.CosineSimilarityFloat32Exported(a, b)
}
