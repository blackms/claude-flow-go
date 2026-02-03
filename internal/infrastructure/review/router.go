// Package review provides review infrastructure.
package review

import (
	"sync"

	domainReview "github.com/anthropics/claude-flow-go/internal/domain/review"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ReviewerInfo contains information about an available reviewer.
type ReviewerInfo struct {
	// AgentID is the reviewer's agent ID.
	AgentID string

	// AgentType is the type of agent.
	AgentType shared.AgentType

	// Capabilities are the reviewer's capabilities.
	Capabilities []string

	// ActiveReviews is how many reviews they're currently doing.
	ActiveReviews int

	// MaxConcurrent is their max concurrent reviews.
	MaxConcurrent int

	// Reputation is their reputation score.
	Reputation float64

	// Specializations are their review specializations.
	Specializations []domainReview.ReviewType
}

// RouterConfig contains router configuration.
type RouterConfig struct {
	// MaxReviewersPerSession is the max reviewers per session.
	MaxReviewersPerSession int

	// PreferSpecialists prefers specialized reviewers.
	PreferSpecialists bool

	// LoadBalancing enables load balancing.
	LoadBalancing bool

	// AvoidConflicts avoids assigning authors as reviewers.
	AvoidConflicts bool
}

// DefaultRouterConfig returns default router configuration.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		MaxReviewersPerSession: 5,
		PreferSpecialists:      true,
		LoadBalancing:          true,
		AvoidConflicts:         true,
	}
}

// ReviewRouter routes review tasks to appropriate reviewers.
type ReviewRouter struct {
	mu     sync.RWMutex
	config RouterConfig

	// Available reviewers
	reviewers map[string]*ReviewerInfo

	// Type to review type mapping
	typeSpecializations map[shared.AgentType][]domainReview.ReviewType
}

// NewReviewRouter creates a new review router.
func NewReviewRouter(config RouterConfig) *ReviewRouter {
	r := &ReviewRouter{
		config:              config,
		reviewers:           make(map[string]*ReviewerInfo),
		typeSpecializations: make(map[shared.AgentType][]domainReview.ReviewType),
	}

	r.initTypeSpecializations()
	return r
}

func (r *ReviewRouter) initTypeSpecializations() {
	r.typeSpecializations[shared.AgentTypeReviewer] = []domainReview.ReviewType{
		domainReview.ReviewTypeQuality,
		domainReview.ReviewTypeFull,
	}
	r.typeSpecializations[shared.AgentTypeSecurityAuditor] = []domainReview.ReviewType{
		domainReview.ReviewTypeSecurity,
	}
	r.typeSpecializations[shared.AgentTypeRedTeam] = []domainReview.ReviewType{
		domainReview.ReviewTypeSecurity,
	}
	r.typeSpecializations[shared.AgentTypeCritic] = []domainReview.ReviewType{
		domainReview.ReviewTypeQuality,
		domainReview.ReviewTypeArchitecture,
	}
	r.typeSpecializations[shared.AgentTypeDevilsAdvocate] = []domainReview.ReviewType{
		domainReview.ReviewTypeArchitecture,
		domainReview.ReviewTypeFull,
	}
	r.typeSpecializations[shared.AgentTypeQualityGate] = []domainReview.ReviewType{
		domainReview.ReviewTypeQuality,
		domainReview.ReviewTypeCompliance,
	}
	r.typeSpecializations[shared.AgentTypePerformanceEngineer] = []domainReview.ReviewType{
		domainReview.ReviewTypePerformance,
	}
	r.typeSpecializations[shared.AgentTypeArchitect] = []domainReview.ReviewType{
		domainReview.ReviewTypeArchitecture,
	}
	r.typeSpecializations[shared.AgentTypeTester] = []domainReview.ReviewType{
		domainReview.ReviewTypeQuality,
	}
}

// RegisterReviewer registers a reviewer.
func (r *ReviewRouter) RegisterReviewer(info ReviewerInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Add specializations based on agent type
	if specs, ok := r.typeSpecializations[info.AgentType]; ok {
		info.Specializations = append(info.Specializations, specs...)
	}

	r.reviewers[info.AgentID] = &info
}

// UnregisterReviewer unregisters a reviewer.
func (r *ReviewRouter) UnregisterReviewer(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.reviewers, agentID)
}

// RouteToReviewers selects reviewers for a work item.
func (r *ReviewRouter) RouteToReviewers(
	work domainReview.Work,
	reviewType domainReview.ReviewType,
	count int,
) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if count <= 0 {
		count = 1
	}
	if count > r.config.MaxReviewersPerSession {
		count = r.config.MaxReviewersPerSession
	}

	candidates := make([]*ReviewerInfo, 0)

	for _, reviewer := range r.reviewers {
		// Skip if conflict
		if r.config.AvoidConflicts && reviewer.AgentID == work.AuthorID {
			continue
		}

		// Skip if at capacity
		if reviewer.ActiveReviews >= reviewer.MaxConcurrent {
			continue
		}

		candidates = append(candidates, reviewer)
	}

	// Score and rank candidates
	scored := r.scoreReviewers(candidates, reviewType)

	// Select top N
	selected := make([]string, 0, count)
	for i := 0; i < count && i < len(scored); i++ {
		selected = append(selected, scored[i].AgentID)
	}

	return selected
}

// scoreReviewers scores and ranks reviewers for a review type.
func (r *ReviewRouter) scoreReviewers(candidates []*ReviewerInfo, reviewType domainReview.ReviewType) []*ReviewerInfo {
	type scoredReviewer struct {
		reviewer *ReviewerInfo
		score    float64
	}

	scored := make([]scoredReviewer, len(candidates))
	for i, reviewer := range candidates {
		score := r.calculateScore(reviewer, reviewType)
		scored[i] = scoredReviewer{reviewer: reviewer, score: score}
	}

	// Sort by score descending
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	result := make([]*ReviewerInfo, len(scored))
	for i, s := range scored {
		result[i] = s.reviewer
	}

	return result
}

// calculateScore calculates a reviewer's score for a review type.
func (r *ReviewRouter) calculateScore(reviewer *ReviewerInfo, reviewType domainReview.ReviewType) float64 {
	score := 0.5 // Base score

	// Specialization bonus
	if r.config.PreferSpecialists {
		for _, spec := range reviewer.Specializations {
			if spec == reviewType || spec == domainReview.ReviewTypeFull {
				score += 0.3
				break
			}
		}
	}

	// Reputation bonus
	score += reviewer.Reputation * 0.2

	// Load balancing penalty
	if r.config.LoadBalancing && reviewer.MaxConcurrent > 0 {
		loadFactor := float64(reviewer.ActiveReviews) / float64(reviewer.MaxConcurrent)
		score -= loadFactor * 0.2
	}

	// Adversarial agent bonus for certain review types
	if reviewType == domainReview.ReviewTypeSecurity {
		if reviewer.AgentType == shared.AgentTypeRedTeam {
			score += 0.2
		}
	}

	if reviewType == domainReview.ReviewTypeArchitecture {
		if reviewer.AgentType == shared.AgentTypeDevilsAdvocate {
			score += 0.15
		}
	}

	return score
}

// GetReviewersForSwarm gets a diverse set of reviewers for a review swarm.
func (r *ReviewRouter) GetReviewersForSwarm(
	work domainReview.Work,
) map[domainReview.ReviewType][]string {
	result := make(map[domainReview.ReviewType][]string)

	reviewTypes := []domainReview.ReviewType{
		domainReview.ReviewTypeSecurity,
		domainReview.ReviewTypeQuality,
		domainReview.ReviewTypePerformance,
		domainReview.ReviewTypeArchitecture,
	}

	for _, rt := range reviewTypes {
		reviewers := r.RouteToReviewers(work, rt, 1)
		if len(reviewers) > 0 {
			result[rt] = reviewers
		}
	}

	return result
}

// MarkReviewStarted marks a reviewer as having started a review.
func (r *ReviewRouter) MarkReviewStarted(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if reviewer, ok := r.reviewers[agentID]; ok {
		reviewer.ActiveReviews++
	}
}

// MarkReviewCompleted marks a reviewer as having completed a review.
func (r *ReviewRouter) MarkReviewCompleted(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if reviewer, ok := r.reviewers[agentID]; ok {
		reviewer.ActiveReviews--
		if reviewer.ActiveReviews < 0 {
			reviewer.ActiveReviews = 0
		}
	}
}

// GetAvailableReviewers returns all available reviewers.
func (r *ReviewRouter) GetAvailableReviewers() []*ReviewerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ReviewerInfo, 0, len(r.reviewers))
	for _, reviewer := range r.reviewers {
		if reviewer.ActiveReviews < reviewer.MaxConcurrent {
			result = append(result, reviewer)
		}
	}

	return result
}

// GetReviewerCount returns the number of registered reviewers.
func (r *ReviewRouter) GetReviewerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.reviewers)
}
