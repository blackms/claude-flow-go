// Package claims provides domain types for the claims system with event sourcing.
package claims

import (
	"time"
)

// ========================================================================
// Claim Eligibility Rules
// ========================================================================

// CanClaimIssue checks if a claimant can claim an issue.
func CanClaimIssue(claimant *Claimant, issue *Issue, existingClaims []*Claim) (bool, string) {
	// Check capacity
	if !claimant.CanAcceptMoreWork() {
		return false, "claimant has reached maximum concurrent claims"
	}

	// Check if already claimed by anyone
	for _, claim := range existingClaims {
		if claim.IssueID == issue.ID && claim.IsActive() {
			return false, "issue is already claimed"
		}
	}

	// Check capabilities
	if len(issue.RequiredCapabilities) > 0 {
		if !claimant.HasAllCapabilities(issue.RequiredCapabilities) {
			return false, "claimant lacks required capabilities"
		}
	}

	return true, ""
}

// IsIssueClaimed returns true if the issue is currently claimed.
func IsIssueClaimed(issueID string, claims []*Claim) bool {
	for _, claim := range claims {
		if claim.IssueID == issueID && claim.IsActive() {
			return true
		}
	}
	return false
}

// GetClaimForIssue returns the active claim for an issue if any.
func GetClaimForIssue(issueID string, claims []*Claim) *Claim {
	for _, claim := range claims {
		if claim.IssueID == issueID && claim.IsActive() {
			return claim
		}
	}
	return nil
}

// ========================================================================
// Status Transition Rules
// ========================================================================

// validTransitions defines valid status transitions.
var validTransitions = map[ClaimStatus][]ClaimStatus{
	StatusActive: {
		StatusPaused,
		StatusBlocked,
		StatusPendingHandoff,
		StatusInReview,
		StatusStealable,
		StatusCompleted,
		StatusReleased,
	},
	StatusPaused: {
		StatusActive,
		StatusBlocked,
		StatusReleased,
	},
	StatusBlocked: {
		StatusActive,
		StatusPaused,
		StatusStealable,
		StatusReleased,
	},
	StatusPendingHandoff: {
		StatusActive, // handoff rejected
		StatusReleased,
	},
	StatusInReview: {
		StatusActive,
		StatusCompleted,
		StatusReleased,
	},
	StatusStealable: {
		StatusActive, // steal contested and won
		StatusReleased,
	},
	// Terminal states have no transitions
	StatusCompleted: {},
	StatusReleased:  {},
	StatusExpired:   {},
}

// CanTransitionStatus checks if a status transition is valid.
func CanTransitionStatus(from, to ClaimStatus) bool {
	validTargets, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, valid := range validTargets {
		if valid == to {
			return true
		}
	}
	return false
}

// GetValidStatusTransitions returns valid target statuses from the current status.
func GetValidStatusTransitions(from ClaimStatus) []ClaimStatus {
	return validTransitions[from]
}

// ========================================================================
// Work Stealing Rules
// ========================================================================

// StealConfig holds configuration for work stealing.
type StealConfig struct {
	GracePeriod        time.Duration // Time before a claim can be stolen
	ProgressProtection float64       // Progress threshold that protects from stealing
	StaleThreshold     time.Duration // Time without activity to be considered stale
	BlockedThreshold   time.Duration // Time blocked before auto-stealable
	ContestWindow      time.Duration // Time window to contest a steal
}

// DefaultStealConfig returns the default steal configuration.
func DefaultStealConfig() StealConfig {
	return StealConfig{
		GracePeriod:        5 * time.Minute,
		ProgressProtection: 0.75,
		StaleThreshold:     30 * time.Minute,
		BlockedThreshold:   60 * time.Minute,
		ContestWindow:      5 * time.Minute,
	}
}

// CanMarkAsStealable checks if a claim can be marked as stealable.
func CanMarkAsStealable(claim *Claim, config StealConfig) (bool, string) {
	// Terminal claims cannot be stolen
	if claim.IsTerminal() {
		return false, "claim is in terminal state"
	}

	// Already stealable
	if claim.Status == StatusStealable {
		return false, "claim is already stealable"
	}

	// Check grace period
	if time.Since(claim.ClaimedAt) < config.GracePeriod {
		return false, "claim is within grace period"
	}

	// High progress protects from stealing
	if claim.Progress >= config.ProgressProtection {
		return false, "claim has high progress"
	}

	return true, ""
}

// CanStealClaim checks if a claimant can steal a claim.
func CanStealClaim(claim *Claim, stealer *Claimant, config StealConfig) (bool, string) {
	// Must be stealable
	if claim.Status != StatusStealable {
		return false, "claim is not stealable"
	}

	// Cannot steal own claim
	if claim.IsOwnedBy(stealer.ID) {
		return false, "cannot steal own claim"
	}

	// Stealer must have capacity
	if !stealer.CanAcceptMoreWork() {
		return false, "stealer has reached maximum concurrent claims"
	}

	return true, ""
}

// RequiresStealContest checks if a steal requires a contest window.
func RequiresStealContest(claim *Claim, stealer *Claimant) bool {
	// Cross-type stealing (agent stealing from human or vice versa) requires contest
	return claim.Claimant.Type != stealer.Type
}

// IsClaimStale checks if a claim is stale.
func IsClaimStale(claim *Claim, config StealConfig) bool {
	if !claim.IsActive() {
		return false
	}
	return claim.IsStale(config.StaleThreshold)
}

// IsClaimBlocked checks if a claim has been blocked too long.
func IsClaimBlocked(claim *Claim, config StealConfig) bool {
	if claim.Status != StatusBlocked {
		return false
	}
	return claim.IsStale(config.BlockedThreshold)
}

// ========================================================================
// Handoff Rules
// ========================================================================

// CanInitiateHandoff checks if a handoff can be initiated.
func CanInitiateHandoff(claim *Claim, from, to *Claimant) (bool, string) {
	// Must be claim owner
	if !claim.IsOwnedBy(from.ID) {
		return false, "only claim owner can initiate handoff"
	}

	// Cannot hand off to self
	if from.ID == to.ID {
		return false, "cannot hand off to self"
	}

	// Claim must be active
	if !claim.IsActive() {
		return false, "claim is not active"
	}

	// Cannot have pending handoff
	if claim.HasPendingHandoff() {
		return false, "claim already has pending handoff"
	}

	// Target must have capacity
	if !to.CanAcceptMoreWork() {
		return false, "target has reached maximum concurrent claims"
	}

	return true, ""
}

// CanAcceptHandoff checks if a handoff can be accepted.
func CanAcceptHandoff(claim *Claim, accepter *Claimant) (bool, string) {
	handoff := claim.GetPendingHandoff()
	if handoff == nil {
		return false, "no pending handoff"
	}

	if handoff.To.ID != accepter.ID {
		return false, "only handoff target can accept"
	}

	if !accepter.CanAcceptMoreWork() {
		return false, "accepter has reached maximum concurrent claims"
	}

	return true, ""
}

// CanRejectHandoff checks if a handoff can be rejected.
func CanRejectHandoff(claim *Claim, rejecter *Claimant) (bool, string) {
	handoff := claim.GetPendingHandoff()
	if handoff == nil {
		return false, "no pending handoff"
	}

	if handoff.To.ID != rejecter.ID {
		return false, "only handoff target can reject"
	}

	return true, ""
}

// ========================================================================
// Load Balancing Rules
// ========================================================================

// LoadConfig holds configuration for load balancing.
type LoadConfig struct {
	OverloadThreshold   float64 // Utilization above this is overloaded
	UnderloadThreshold  float64 // Utilization below this is underloaded
	MinProgressToMove   float64 // Minimum progress to not move a claim
	RebalanceInterval   time.Duration
	ImbalanceThreshold  float64 // Coefficient of variation threshold
}

// DefaultLoadConfig returns the default load configuration.
func DefaultLoadConfig() LoadConfig {
	return LoadConfig{
		OverloadThreshold:  1.5,  // 150% of average
		UnderloadThreshold: 0.5,  // 50% of average
		MinProgressToMove:  0.25, // Don't move claims with >25% progress
		RebalanceInterval:  5 * time.Minute,
		ImbalanceThreshold: 0.3, // 30% coefficient of variation
	}
}

// IsAgentOverloaded checks if an agent is overloaded.
func IsAgentOverloaded(claimant *Claimant, avgUtilization float64, config LoadConfig) bool {
	threshold := avgUtilization * config.OverloadThreshold
	return claimant.Utilization() > threshold
}

// IsAgentUnderloaded checks if an agent is underloaded.
func IsAgentUnderloaded(claimant *Claimant, avgUtilization float64, config LoadConfig) bool {
	threshold := avgUtilization * config.UnderloadThreshold
	return claimant.Utilization() < threshold
}

// CanMoveClaim checks if a claim can be moved for load balancing.
func CanMoveClaim(claim *Claim, config LoadConfig) (bool, string) {
	// Cannot move terminal claims
	if claim.IsTerminal() {
		return false, "claim is in terminal state"
	}

	// Cannot move claims with pending handoffs
	if claim.HasPendingHandoff() {
		return false, "claim has pending handoff"
	}

	// Don't move claims with significant progress
	if claim.Progress >= config.MinProgressToMove {
		return false, "claim has too much progress"
	}

	return true, ""
}

// NeedsRebalancing checks if a swarm needs rebalancing.
func NeedsRebalancing(utilizations []float64, config LoadConfig) bool {
	if len(utilizations) < 2 {
		return false
	}

	// Calculate coefficient of variation
	var sum, sumSq float64
	for _, u := range utilizations {
		sum += u
		sumSq += u * u
	}

	n := float64(len(utilizations))
	mean := sum / n
	if mean == 0 {
		return false
	}

	variance := (sumSq / n) - (mean * mean)
	if variance < 0 {
		variance = 0
	}

	// Standard deviation / mean = coefficient of variation
	cv := 0.0
	if mean > 0 {
		cv = (variance / (mean * mean))
		if cv > 0 {
			cv = cv // sqrt would be here but we're comparing squared values
		}
	}

	return cv > config.ImbalanceThreshold*config.ImbalanceThreshold
}

// ========================================================================
// Validation Rules
// ========================================================================

// IsValidPriority checks if a priority value is valid.
func IsValidPriority(p Priority) bool {
	return p == PriorityCritical || p == PriorityHigh || p == PriorityMedium || p == PriorityLow
}

// IsValidComplexity checks if a complexity value is valid.
func IsValidComplexity(c Complexity) bool {
	return c == ComplexityTrivial || c == ComplexitySimple || c == ComplexityModerate ||
		c == ComplexityComplex || c == ComplexityEpic
}

// IsValidStatus checks if a claim status value is valid.
func IsValidStatus(s ClaimStatus) bool {
	return s == StatusActive || s == StatusPaused || s == StatusBlocked ||
		s == StatusPendingHandoff || s == StatusInReview || s == StatusStealable ||
		s == StatusCompleted || s == StatusReleased || s == StatusExpired
}

// IsValidClaimantType checks if a claimant type is valid.
func IsValidClaimantType(t ClaimantType) bool {
	return t == ClaimantTypeHuman || t == ClaimantTypeAgent
}
