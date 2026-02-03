// Package neural provides domain types for neural learning systems.
package neural

import (
	"time"
)

// ReasoningDomain represents the domain classification of a reasoning pattern.
type ReasoningDomain string

const (
	// ReasoningDomainCode is for code-related reasoning.
	ReasoningDomainCode ReasoningDomain = "code"
	// ReasoningDomainCreative is for creative reasoning.
	ReasoningDomainCreative ReasoningDomain = "creative"
	// ReasoningDomainLogical is for logical/analytical reasoning.
	ReasoningDomainLogical ReasoningDomain = "logical"
	// ReasoningDomainMath is for mathematical reasoning.
	ReasoningDomainMath ReasoningDomain = "math"
	// ReasoningDomainPlanning is for planning and strategy.
	ReasoningDomainPlanning ReasoningDomain = "planning"
	// ReasoningDomainGeneral is the default domain.
	ReasoningDomainGeneral ReasoningDomain = "general"
)

// PatternEvolutionType represents the type of pattern evolution event.
type PatternEvolutionType string

const (
	// EvolutionImprovement indicates pattern quality improved.
	EvolutionImprovement PatternEvolutionType = "improvement"
	// EvolutionMerge indicates patterns were merged.
	EvolutionMerge PatternEvolutionType = "merge"
	// EvolutionSplit indicates a pattern was split.
	EvolutionSplit PatternEvolutionType = "split"
	// EvolutionPrune indicates a pattern was pruned.
	EvolutionPrune PatternEvolutionType = "prune"
	// EvolutionCreate indicates a new pattern was created.
	EvolutionCreate PatternEvolutionType = "create"
)

// PatternEvolution represents an evolution event for a reasoning pattern.
type PatternEvolution struct {
	// EvolutionID is the unique identifier.
	EvolutionID string `json:"evolutionId"`

	// PatternID is the pattern that evolved.
	PatternID string `json:"patternId"`

	// Type is the type of evolution.
	Type PatternEvolutionType `json:"type"`

	// PreviousVersion is the version before evolution.
	PreviousVersion int `json:"previousVersion"`

	// NewVersion is the version after evolution.
	NewVersion int `json:"newVersion"`

	// QualityBefore is the quality score before evolution.
	QualityBefore float64 `json:"qualityBefore"`

	// QualityAfter is the quality score after evolution.
	QualityAfter float64 `json:"qualityAfter"`

	// SuccessRateBefore is the success rate before evolution.
	SuccessRateBefore float64 `json:"successRateBefore"`

	// SuccessRateAfter is the success rate after evolution.
	SuccessRateAfter float64 `json:"successRateAfter"`

	// MergedPatternIDs are IDs of patterns merged (for merge events).
	MergedPatternIDs []string `json:"mergedPatternIds,omitempty"`

	// Reason describes why the evolution occurred.
	Reason string `json:"reason"`

	// Timestamp is when the evolution occurred.
	Timestamp time.Time `json:"timestamp"`
}

// ReasoningPattern represents a learned reasoning pattern.
type ReasoningPattern struct {
	// PatternID is the unique identifier.
	PatternID string `json:"patternId"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Domain is the domain classification.
	Domain ReasoningDomain `json:"domain"`

	// Embedding is the vector embedding for similarity search.
	Embedding []float32 `json:"embedding"`

	// Strategy is the extracted strategy pattern.
	Strategy string `json:"strategy"`

	// SuccessRate is the computed success rate (0-1).
	SuccessRate float64 `json:"successRate"`

	// UsageCount is how often this pattern was used.
	UsageCount int `json:"usageCount"`

	// QualityHistory is the history of quality scores (max 100).
	QualityHistory []float64 `json:"qualityHistory"`

	// EvolutionHistory is the history of evolution events.
	EvolutionHistory []PatternEvolution `json:"evolutionHistory"`

	// Version is the pattern version number.
	Version int `json:"version"`

	// KeyLearnings are key insights extracted from this pattern.
	KeyLearnings []string `json:"keyLearnings"`

	// SourceTrajectoryIDs are trajectories this pattern was derived from.
	SourceTrajectoryIDs []string `json:"sourceTrajectoryIds"`

	// LastUsed is when the pattern was last used.
	LastUsed time.Time `json:"lastUsed"`

	// CreatedAt is when the pattern was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the pattern was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// UpdateQualityHistory adds a quality score to the history.
func (p *ReasoningPattern) UpdateQualityHistory(quality float64) {
	p.QualityHistory = append(p.QualityHistory, quality)
	if len(p.QualityHistory) > 100 {
		p.QualityHistory = p.QualityHistory[1:]
	}
	p.recomputeSuccessRate()
	p.UpdatedAt = time.Now()
}

// recomputeSuccessRate recalculates success rate from history.
func (p *ReasoningPattern) recomputeSuccessRate() {
	if len(p.QualityHistory) == 0 {
		p.SuccessRate = 0
		return
	}

	// Consider a quality >= 0.6 as success
	successThreshold := 0.6
	var successCount int
	for _, q := range p.QualityHistory {
		if q >= successThreshold {
			successCount++
		}
	}
	p.SuccessRate = float64(successCount) / float64(len(p.QualityHistory))
}

// RecordUsage records a usage of this pattern.
func (p *ReasoningPattern) RecordUsage() {
	p.UsageCount++
	p.LastUsed = time.Now()
	p.UpdatedAt = time.Now()
}

// AddEvolution records an evolution event.
func (p *ReasoningPattern) AddEvolution(evolution PatternEvolution) {
	p.EvolutionHistory = append(p.EvolutionHistory, evolution)
	p.Version = evolution.NewVersion
	p.UpdatedAt = time.Now()
}

// ReasoningMemory wraps a distilled memory with additional metadata.
type ReasoningMemory struct {
	// MemoryID is the unique identifier.
	MemoryID string `json:"memoryId"`

	// Memory is the underlying distilled memory.
	Memory DistilledMemory `json:"memory"`

	// TrajectoryID is the source trajectory.
	TrajectoryID string `json:"trajectoryId"`

	// Verdict is the trajectory verdict.
	Verdict *TrajectoryVerdict `json:"verdict,omitempty"`

	// Consolidated indicates if this memory has been consolidated.
	Consolidated bool `json:"consolidated"`

	// PatternID is the pattern derived from this memory (if any).
	PatternID string `json:"patternId,omitempty"`

	// RelevanceScore is the relevance score for retrieval.
	RelevanceScore float64 `json:"relevanceScore"`

	// DiversityScore is the diversity score for MMR.
	DiversityScore float64 `json:"diversityScore"`

	// CreatedAt is when the memory was created.
	CreatedAt time.Time `json:"createdAt"`
}

// ReasoningConfig configures the ReasoningBank.
type ReasoningConfig struct {
	// MaxPatterns is the maximum number of patterns to store.
	MaxPatterns int `json:"maxPatterns"`

	// DistillationThreshold is the minimum quality for distillation.
	DistillationThreshold float64 `json:"distillationThreshold"`

	// RetrievalK is the default number of patterns to retrieve.
	RetrievalK int `json:"retrievalK"`

	// MMRLambda is the lambda for MMR diversity (0-1).
	// Higher values favor relevance, lower favor diversity.
	MMRLambda float64 `json:"mmrLambda"`

	// MaxPatternAgeDays is the max age before pruning.
	MaxPatternAgeDays int `json:"maxPatternAgeDays"`

	// DedupThreshold is the similarity threshold for deduplication.
	DedupThreshold float64 `json:"dedupThreshold"`

	// VectorDimension is the embedding dimension.
	VectorDimension int `json:"vectorDimension"`

	// MinRelevanceThreshold is the minimum relevance for retrieval.
	MinRelevanceThreshold float64 `json:"minRelevanceThreshold"`

	// MergeThreshold is the similarity threshold for merging patterns.
	MergeThreshold float64 `json:"mergeThreshold"`

	// MinUsageForRetention is the min usage to avoid pruning.
	MinUsageForRetention int `json:"minUsageForRetention"`

	// DatabasePath is the path to the SQLite database.
	DatabasePath string `json:"databasePath"`
}

// DefaultReasoningConfig returns the default configuration.
func DefaultReasoningConfig() ReasoningConfig {
	return ReasoningConfig{
		MaxPatterns:           5000,
		DistillationThreshold: 0.6,
		RetrievalK:            5,
		MMRLambda:             0.7,
		MaxPatternAgeDays:     30,
		DedupThreshold:        0.95,
		VectorDimension:       768,
		MinRelevanceThreshold: 0.6,
		MergeThreshold:        0.9,
		MinUsageForRetention:  5,
		DatabasePath:          "reasoning_bank.db",
	}
}

// RetrievalResult represents a pattern retrieval result.
type RetrievalResult struct {
	// Pattern is the retrieved pattern.
	Pattern ReasoningPattern `json:"pattern"`

	// RelevanceScore is the similarity to the query (0-1).
	RelevanceScore float64 `json:"relevanceScore"`

	// DiversityScore is the diversity from selected items (0-1).
	DiversityScore float64 `json:"diversityScore"`

	// CombinedScore is the MMR combined score.
	CombinedScore float64 `json:"combinedScore"`
}

// ReasoningBankStats contains statistics about the ReasoningBank.
type ReasoningBankStats struct {
	// PatternCount is the total number of patterns.
	PatternCount int `json:"patternCount"`

	// MemoryCount is the total number of memories.
	MemoryCount int `json:"memoryCount"`

	// AvgQuality is the average quality across patterns.
	AvgQuality float64 `json:"avgQuality"`

	// AvgSuccessRate is the average success rate.
	AvgSuccessRate float64 `json:"avgSuccessRate"`

	// AvgUsageCount is the average usage count.
	AvgUsageCount float64 `json:"avgUsageCount"`

	// TotalEvolutions is the total evolution events.
	TotalEvolutions int `json:"totalEvolutions"`

	// RetrievalCount is the number of retrievals performed.
	RetrievalCount int64 `json:"retrievalCount"`

	// DistillCount is the number of distillations performed.
	DistillCount int64 `json:"distillCount"`

	// ConsolidationCount is the number of consolidations.
	ConsolidationCount int64 `json:"consolidationCount"`

	// AvgRetrievalLatencyMs is the average retrieval latency.
	AvgRetrievalLatencyMs float64 `json:"avgRetrievalLatencyMs"`

	// AvgDistillLatencyMs is the average distill latency.
	AvgDistillLatencyMs float64 `json:"avgDistillLatencyMs"`

	// LastConsolidation is when the last consolidation occurred.
	LastConsolidation *time.Time `json:"lastConsolidation,omitempty"`

	// DomainDistribution shows patterns per domain.
	DomainDistribution map[ReasoningDomain]int `json:"domainDistribution"`
}

// ConsolidationResult contains results from a consolidation run.
type ConsolidationResult struct {
	// Deduplicated is the number of patterns deduplicated.
	Deduplicated int `json:"deduplicated"`

	// Merged is the number of patterns merged.
	Merged int `json:"merged"`

	// Pruned is the number of patterns pruned.
	Pruned int `json:"pruned"`

	// ContradictionsFound is the number of contradictions found.
	ContradictionsFound int `json:"contradictionsFound"`

	// Duration is how long consolidation took.
	DurationMs int64 `json:"durationMs"`

	// Timestamp is when consolidation occurred.
	Timestamp time.Time `json:"timestamp"`
}

// LearningOutcome represents the outcome of learning from a trajectory.
type LearningOutcome struct {
	// Success indicates if the trajectory was successful.
	Success bool `json:"success"`

	// Feedback is optional feedback string.
	Feedback string `json:"feedback,omitempty"`

	// QualityScore is the quality score (0-1).
	QualityScore float64 `json:"qualityScore"`

	// PatternUpdated indicates if a pattern was updated.
	PatternUpdated bool `json:"patternUpdated"`

	// PatternID is the ID of the updated/created pattern.
	PatternID string `json:"patternId,omitempty"`

	// NewPatternCreated indicates if a new pattern was created.
	NewPatternCreated bool `json:"newPatternCreated"`
}
