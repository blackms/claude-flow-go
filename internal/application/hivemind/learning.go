// Package hivemind provides the Hive Mind consensus system for multi-agent coordination.
package hivemind

import (
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// LearningModule tracks proposal outcomes and learns from consensus patterns.
type LearningModule struct {
	enabled     bool
	outcomes    []shared.ProposalOutcome
	patterns    map[string]*ProposalPattern
	typeStats   map[shared.ConsensusType]*ConsensusStats
	maxOutcomes int
	mu          sync.RWMutex
}

// ProposalPattern represents learned patterns for proposal types.
type ProposalPattern struct {
	ProposalType     string  `json:"proposalType"`
	TotalProposals   int     `json:"totalProposals"`
	ApprovedCount    int     `json:"approvedCount"`
	RejectedCount    int     `json:"rejectedCount"`
	ExpiredCount     int     `json:"expiredCount"`
	AvgDuration      float64 `json:"avgDuration"`      // milliseconds
	AvgVoteCount     float64 `json:"avgVoteCount"`
	AvgWeightedScore float64 `json:"avgWeightedScore"`
	SuccessRate      float64 `json:"successRate"`      // 0.0 - 1.0
}

// ConsensusStats represents statistics for a consensus type.
type ConsensusStats struct {
	ConsensusType  shared.ConsensusType `json:"consensusType"`
	TotalUsed      int                  `json:"totalUsed"`
	SuccessCount   int                  `json:"successCount"`
	FailureCount   int                  `json:"failureCount"`
	AvgDuration    float64              `json:"avgDuration"`
	AvgVoteCount   float64              `json:"avgVoteCount"`
	SuccessRate    float64              `json:"successRate"`
}

// NewLearningModule creates a new Learning Module.
func NewLearningModule(enabled bool) *LearningModule {
	return &LearningModule{
		enabled:     enabled,
		outcomes:    make([]shared.ProposalOutcome, 0),
		patterns:    make(map[string]*ProposalPattern),
		typeStats:   make(map[shared.ConsensusType]*ConsensusStats),
		maxOutcomes: 1000, // Keep last 1000 outcomes
	}
}

// RecordOutcome records a proposal outcome for learning.
func (lm *LearningModule) RecordOutcome(outcome shared.ProposalOutcome) {
	if !lm.enabled {
		return
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Add to outcomes
	lm.outcomes = append(lm.outcomes, outcome)

	// Trim if over limit
	if len(lm.outcomes) > lm.maxOutcomes {
		lm.outcomes = lm.outcomes[len(lm.outcomes)-lm.maxOutcomes:]
	}

	// Update patterns
	lm.updatePattern(outcome)

	// Update consensus type stats
	lm.updateTypeStats(outcome)
}

// updatePattern updates the pattern for a proposal type.
func (lm *LearningModule) updatePattern(outcome shared.ProposalOutcome) {
	pattern, exists := lm.patterns[outcome.ProposalType]
	if !exists {
		pattern = &ProposalPattern{
			ProposalType: outcome.ProposalType,
		}
		lm.patterns[outcome.ProposalType] = pattern
	}

	pattern.TotalProposals++

	if outcome.WasApproved {
		pattern.ApprovedCount++
	} else {
		pattern.RejectedCount++
	}

	// Update averages using cumulative moving average
	n := float64(pattern.TotalProposals)
	pattern.AvgDuration = pattern.AvgDuration*(n-1)/n + float64(outcome.Duration)/n
	pattern.AvgVoteCount = pattern.AvgVoteCount*(n-1)/n + float64(outcome.VoteCount)/n
	pattern.AvgWeightedScore = pattern.AvgWeightedScore*(n-1)/n + outcome.WeightedScore/n

	// Calculate success rate
	pattern.SuccessRate = float64(pattern.ApprovedCount) / float64(pattern.TotalProposals)
}

// updateTypeStats updates statistics for a consensus type.
func (lm *LearningModule) updateTypeStats(outcome shared.ProposalOutcome) {
	stats, exists := lm.typeStats[outcome.ConsensusType]
	if !exists {
		stats = &ConsensusStats{
			ConsensusType: outcome.ConsensusType,
		}
		lm.typeStats[outcome.ConsensusType] = stats
	}

	stats.TotalUsed++

	if outcome.WasApproved {
		stats.SuccessCount++
	} else {
		stats.FailureCount++
	}

	// Update averages
	n := float64(stats.TotalUsed)
	stats.AvgDuration = stats.AvgDuration*(n-1)/n + float64(outcome.Duration)/n
	stats.AvgVoteCount = stats.AvgVoteCount*(n-1)/n + float64(outcome.VoteCount)/n

	// Calculate success rate
	stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalUsed)
}

// PredictSuccess predicts the success probability for a proposal.
func (lm *LearningModule) PredictSuccess(proposal shared.Proposal) float64 {
	if !lm.enabled {
		return 0.5 // Default to 50%
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// Base prediction on proposal type pattern
	pattern, exists := lm.patterns[proposal.Type]
	if !exists || pattern.TotalProposals < 5 {
		// Not enough data, use consensus type stats
		return lm.predictFromConsensusType(proposal.RequiredType)
	}

	return pattern.SuccessRate
}

// predictFromConsensusType predicts success based on consensus type history.
func (lm *LearningModule) predictFromConsensusType(consensusType shared.ConsensusType) float64 {
	stats, exists := lm.typeStats[consensusType]
	if !exists || stats.TotalUsed < 3 {
		// Default predictions based on consensus type
		switch consensusType {
		case shared.ConsensusTypeQueenOverride:
			return 1.0 // Always succeeds
		case shared.ConsensusTypeMajority:
			return 0.7 // High likelihood
		case shared.ConsensusTypeSuperMajority:
			return 0.5 // Medium likelihood
		case shared.ConsensusTypeUnanimous:
			return 0.2 // Low likelihood
		default:
			return 0.5
		}
	}

	return stats.SuccessRate
}

// GetPattern returns the pattern for a proposal type.
func (lm *LearningModule) GetPattern(proposalType string) (*ProposalPattern, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	pattern, exists := lm.patterns[proposalType]
	if !exists {
		return nil, false
	}

	// Return a copy
	copy := *pattern
	return &copy, true
}

// GetConsensusStats returns statistics for a consensus type.
func (lm *LearningModule) GetConsensusStats(consensusType shared.ConsensusType) (*ConsensusStats, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	stats, exists := lm.typeStats[consensusType]
	if !exists {
		return nil, false
	}

	// Return a copy
	copy := *stats
	return &copy, true
}

// GetAllPatterns returns all learned patterns.
func (lm *LearningModule) GetAllPatterns() map[string]*ProposalPattern {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make(map[string]*ProposalPattern)
	for k, v := range lm.patterns {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetAllConsensusStats returns statistics for all consensus types.
func (lm *LearningModule) GetAllConsensusStats() map[shared.ConsensusType]*ConsensusStats {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make(map[shared.ConsensusType]*ConsensusStats)
	for k, v := range lm.typeStats {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetRecentOutcomes returns recent proposal outcomes.
func (lm *LearningModule) GetRecentOutcomes(limit int) []shared.ProposalOutcome {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if limit <= 0 || limit > len(lm.outcomes) {
		limit = len(lm.outcomes)
	}

	// Return most recent
	start := len(lm.outcomes) - limit
	result := make([]shared.ProposalOutcome, limit)
	copy(result, lm.outcomes[start:])
	return result
}

// RecommendConsensusType recommends the best consensus type for a proposal.
func (lm *LearningModule) RecommendConsensusType(proposal shared.Proposal) shared.ConsensusType {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// Check proposal type pattern
	pattern, exists := lm.patterns[proposal.Type]
	if exists && pattern.TotalProposals >= 10 {
		// Based on historical success rate
		if pattern.SuccessRate >= 0.9 {
			return shared.ConsensusTypeMajority // Easy proposals
		} else if pattern.SuccessRate >= 0.6 {
			return shared.ConsensusTypeSuperMajority // Needs more agreement
		} else {
			return shared.ConsensusTypeWeighted // Use weighted for controversial
		}
	}

	// Default based on priority
	switch proposal.Priority {
	case shared.PriorityHigh:
		return shared.ConsensusTypeMajority // Fast decision
	case shared.PriorityMedium:
		return shared.ConsensusTypeSuperMajority
	case shared.PriorityLow:
		return shared.ConsensusTypeWeighted // More deliberation
	default:
		return shared.ConsensusTypeMajority
	}
}

// GetLearningStats returns overall learning statistics.
func (lm *LearningModule) GetLearningStats() LearningStats {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	stats := LearningStats{
		Enabled:          lm.enabled,
		TotalOutcomes:    len(lm.outcomes),
		UniquePatterns:   len(lm.patterns),
		ConsensusTypes:   len(lm.typeStats),
	}

	// Calculate overall success rate
	if len(lm.outcomes) > 0 {
		approvedCount := 0
		for _, o := range lm.outcomes {
			if o.WasApproved {
				approvedCount++
			}
		}
		stats.OverallSuccessRate = float64(approvedCount) / float64(len(lm.outcomes))
	}

	return stats
}

// LearningStats represents overall learning statistics.
type LearningStats struct {
	Enabled            bool    `json:"enabled"`
	TotalOutcomes      int     `json:"totalOutcomes"`
	UniquePatterns     int     `json:"uniquePatterns"`
	ConsensusTypes     int     `json:"consensusTypes"`
	OverallSuccessRate float64 `json:"overallSuccessRate"`
}

// Reset clears all learned data.
func (lm *LearningModule) Reset() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.outcomes = make([]shared.ProposalOutcome, 0)
	lm.patterns = make(map[string]*ProposalPattern)
	lm.typeStats = make(map[shared.ConsensusType]*ConsensusStats)
}

// SetEnabled enables or disables learning.
func (lm *LearningModule) SetEnabled(enabled bool) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.enabled = enabled
}

// IsEnabled returns whether learning is enabled.
func (lm *LearningModule) IsEnabled() bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.enabled
}
