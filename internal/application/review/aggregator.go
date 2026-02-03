// Package review provides review application services.
package review

import (
	"sort"
	"time"

	domainReview "github.com/anthropics/claude-flow-go/internal/domain/review"
)

// AggregatorConfig contains configuration for verdict aggregation.
type AggregatorConfig struct {
	// WeightByReputation weights votes by reviewer reputation.
	WeightByReputation bool

	// WeightByConfidence weights votes by verdict confidence.
	WeightByConfidence bool

	// CriticalBlockerThreshold is the threshold for critical blockers.
	CriticalBlockerThreshold int

	// MinConsensusLevel is the minimum consensus level.
	MinConsensusLevel float64
}

// DefaultAggregatorConfig returns default aggregator configuration.
func DefaultAggregatorConfig() AggregatorConfig {
	return AggregatorConfig{
		WeightByReputation:       true,
		WeightByConfidence:       true,
		CriticalBlockerThreshold: 1,
		MinConsensusLevel:        0.5,
	}
}

// VerdictAggregator aggregates multiple review verdicts.
type VerdictAggregator struct {
	config AggregatorConfig

	// Reviewer reputation scores
	reputations map[string]float64
}

// NewVerdictAggregator creates a new verdict aggregator.
func NewVerdictAggregator(config AggregatorConfig) *VerdictAggregator {
	return &VerdictAggregator{
		config:      config,
		reputations: make(map[string]float64),
	}
}

// SetReputation sets a reviewer's reputation.
func (va *VerdictAggregator) SetReputation(reviewerID string, reputation float64) {
	va.reputations[reviewerID] = reputation
}

// AggregateVerdicts aggregates multiple verdicts into a final verdict.
func (va *VerdictAggregator) AggregateVerdicts(
	verdicts []domainReview.ReviewVerdict,
	requireUnanimity bool,
) (*domainReview.FinalVerdict, error) {
	if len(verdicts) == 0 {
		return nil, nil
	}

	now := time.Now()
	workID := verdicts[0].WorkID
	sessionID := verdicts[0].SessionID

	// Calculate weighted votes
	approvalWeight := 0.0
	rejectionWeight := 0.0
	revisionWeight := 0.0
	totalWeight := 0.0

	for _, verdict := range verdicts {
		weight := va.calculateWeight(verdict)
		totalWeight += weight

		switch verdict.Decision {
		case domainReview.DecisionApprove:
			approvalWeight += weight
		case domainReview.DecisionReject:
			rejectionWeight += weight
		case domainReview.DecisionRevise:
			revisionWeight += weight
		}
	}

	// Collect all issues
	allIssues := make([]domainReview.ReviewIssue, 0)
	blockers := make([]domainReview.ReviewIssue, 0)

	for _, verdict := range verdicts {
		for _, issue := range verdict.Issues {
			allIssues = append(allIssues, issue)
			if issue.IsBlocker {
				blockers = append(blockers, issue)
			}
		}
	}

	// Deduplicate and sort issues by severity
	allIssues = va.deduplicateIssues(allIssues)
	blockers = va.deduplicateIssues(blockers)

	sort.Slice(allIssues, func(i, j int) bool {
		return allIssues[i].Severity.Weight() > allIssues[j].Severity.Weight()
	})

	// Determine final decision
	var finalDecision domainReview.ReviewDecision

	// Check for critical blockers
	criticalBlockers := 0
	for _, blocker := range blockers {
		if blocker.Severity == domainReview.SeverityCritical {
			criticalBlockers++
		}
	}

	if criticalBlockers >= va.config.CriticalBlockerThreshold {
		finalDecision = domainReview.DecisionReject
	} else if requireUnanimity {
		// All must approve
		allApprove := true
		for _, verdict := range verdicts {
			if verdict.Decision != domainReview.DecisionApprove {
				allApprove = false
				break
			}
		}
		if allApprove {
			finalDecision = domainReview.DecisionApprove
		} else {
			finalDecision = domainReview.DecisionRevise
		}
	} else {
		// Weighted majority
		if approvalWeight > rejectionWeight && approvalWeight > revisionWeight {
			finalDecision = domainReview.DecisionApprove
		} else if rejectionWeight > approvalWeight && rejectionWeight > revisionWeight {
			finalDecision = domainReview.DecisionReject
		} else {
			finalDecision = domainReview.DecisionRevise
		}
	}

	// Calculate consensus level
	var maxWeight float64
	if approvalWeight >= rejectionWeight && approvalWeight >= revisionWeight {
		maxWeight = approvalWeight
	} else if rejectionWeight >= approvalWeight && rejectionWeight >= revisionWeight {
		maxWeight = rejectionWeight
	} else {
		maxWeight = revisionWeight
	}

	consensusLevel := 0.0
	if totalWeight > 0 {
		consensusLevel = maxWeight / totalWeight
	}

	// Calculate average confidence
	totalConfidence := 0.0
	for _, verdict := range verdicts {
		totalConfidence += verdict.Confidence
	}
	avgConfidence := totalConfidence / float64(len(verdicts))

	// Identify divergent reviewers
	divergentReviewers := va.identifyDivergent(verdicts, finalDecision)

	// Generate summary
	summary := va.generateSummary(verdicts, finalDecision, blockers)

	// Count votes
	approvals := 0
	rejections := 0
	for _, verdict := range verdicts {
		if verdict.Decision == domainReview.DecisionApprove {
			approvals++
		} else if verdict.Decision == domainReview.DecisionReject {
			rejections++
		}
	}

	return &domainReview.FinalVerdict{
		WorkID:             workID,
		SessionID:          sessionID,
		Decision:           finalDecision,
		Confidence:         avgConfidence,
		ConsensusLevel:     consensusLevel,
		TotalReviewers:     len(verdicts),
		Approvals:          approvals,
		Rejections:         rejections,
		AllIssues:          allIssues,
		Blockers:           blockers,
		DivergentReviewers: divergentReviewers,
		Summary:            summary,
		CompletedAt:        now,
	}, nil
}

// calculateWeight calculates the weight for a verdict.
func (va *VerdictAggregator) calculateWeight(verdict domainReview.ReviewVerdict) float64 {
	weight := 1.0

	if va.config.WeightByConfidence {
		weight *= verdict.Confidence
	}

	if va.config.WeightByReputation {
		if rep, ok := va.reputations[verdict.ReviewerID]; ok {
			weight *= (0.5 + rep*0.5) // Scale reputation to 0.5-1.0
		}
	}

	return weight
}

// identifyDivergent identifies reviewers who disagreed with the final decision.
func (va *VerdictAggregator) identifyDivergent(
	verdicts []domainReview.ReviewVerdict,
	finalDecision domainReview.ReviewDecision,
) []string {
	divergent := make([]string, 0)

	for _, verdict := range verdicts {
		if verdict.Decision != finalDecision {
			divergent = append(divergent, verdict.ReviewerID)
		}
	}

	return divergent
}

// generateSummary generates a summary of the review.
func (va *VerdictAggregator) generateSummary(
	verdicts []domainReview.ReviewVerdict,
	decision domainReview.ReviewDecision,
	blockers []domainReview.ReviewIssue,
) string {
	summary := ""

	switch decision {
	case domainReview.DecisionApprove:
		summary = "Work approved by review panel. "
	case domainReview.DecisionReject:
		summary = "Work rejected by review panel. "
	case domainReview.DecisionRevise:
		summary = "Work requires revision. "
	}

	if len(blockers) > 0 {
		summary += generateBlockerSummary(len(blockers))
	}

	totalIssues := 0
	for _, v := range verdicts {
		totalIssues += len(v.Issues)
	}

	if totalIssues > 0 {
		summary += generateIssueSummary(totalIssues)
	}

	return summary
}

func generateBlockerSummary(count int) string {
	if count == 1 {
		return "1 blocking issue found. "
	}
	return string(rune('0'+count)) + " blocking issues found. "
}

func generateIssueSummary(count int) string {
	if count == 1 {
		return "1 issue identified."
	}
	return string(rune('0'+count)) + " issues identified."
}

// deduplicateIssues removes duplicate issues.
func (va *VerdictAggregator) deduplicateIssues(issues []domainReview.ReviewIssue) []domainReview.ReviewIssue {
	seen := make(map[string]bool)
	result := make([]domainReview.ReviewIssue, 0)

	for _, issue := range issues {
		key := issue.Title + issue.Location
		if !seen[key] {
			seen[key] = true
			result = append(result, issue)
		}
	}

	return result
}

// AggregateByReviewType aggregates verdicts grouped by review type.
func (va *VerdictAggregator) AggregateByReviewType(
	verdicts []domainReview.ReviewVerdict,
) map[domainReview.ReviewType]*domainReview.FinalVerdict {
	// Group by review type
	byType := make(map[domainReview.ReviewType][]domainReview.ReviewVerdict)
	for _, verdict := range verdicts {
		byType[verdict.ReviewType] = append(byType[verdict.ReviewType], verdict)
	}

	// Aggregate each group
	result := make(map[domainReview.ReviewType]*domainReview.FinalVerdict)
	for reviewType, typeVerdicts := range byType {
		final, _ := va.AggregateVerdicts(typeVerdicts, false)
		result[reviewType] = final
	}

	return result
}

// GetConsensusBreakdown returns a breakdown of consensus by review type.
func (va *VerdictAggregator) GetConsensusBreakdown(
	verdicts []domainReview.ReviewVerdict,
) map[domainReview.ReviewType]float64 {
	byType := va.AggregateByReviewType(verdicts)

	result := make(map[domainReview.ReviewType]float64)
	for reviewType, final := range byType {
		if final != nil {
			result[reviewType] = final.ConsensusLevel
		}
	}

	return result
}
