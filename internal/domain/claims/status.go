// Package claims provides domain types for the claims system with event sourcing.
package claims

// ClaimStatus represents the status of a claim.
type ClaimStatus string

const (
	StatusActive         ClaimStatus = "active"
	StatusPaused         ClaimStatus = "paused"
	StatusBlocked        ClaimStatus = "blocked"
	StatusPendingHandoff ClaimStatus = "pending_handoff"
	StatusInReview       ClaimStatus = "in_review"
	StatusStealable      ClaimStatus = "stealable"
	StatusCompleted      ClaimStatus = "completed"
	StatusReleased       ClaimStatus = "released"
	StatusExpired        ClaimStatus = "expired"
)

// IsActive returns true if the status represents an active claim.
func (s ClaimStatus) IsActive() bool {
	return s == StatusActive || s == StatusPaused || s == StatusBlocked ||
		s == StatusPendingHandoff || s == StatusInReview || s == StatusStealable
}

// IsTerminal returns true if the status is terminal (no more transitions).
func (s ClaimStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusReleased || s == StatusExpired
}

// ClaimantType represents the type of claimant.
type ClaimantType string

const (
	ClaimantTypeHuman ClaimantType = "human"
	ClaimantTypeAgent ClaimantType = "agent"
)

// Priority represents issue priority.
type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
)

// PriorityWeight returns a numeric weight for priority comparison.
func (p Priority) Weight() int {
	switch p {
	case PriorityCritical:
		return 4
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}

// Complexity represents issue complexity.
type Complexity string

const (
	ComplexityTrivial  Complexity = "trivial"
	ComplexitySimple   Complexity = "simple"
	ComplexityModerate Complexity = "moderate"
	ComplexityComplex  Complexity = "complex"
	ComplexityEpic     Complexity = "epic"
)

// EstimatedHours returns estimated hours for the complexity level.
func (c Complexity) EstimatedHours() int {
	switch c {
	case ComplexityTrivial:
		return 1
	case ComplexitySimple:
		return 4
	case ComplexityModerate:
		return 8
	case ComplexityComplex:
		return 24
	case ComplexityEpic:
		return 80
	default:
		return 8
	}
}

// HandoffStatus represents the status of a handoff request.
type HandoffStatus string

const (
	HandoffStatusPending  HandoffStatus = "pending"
	HandoffStatusAccepted HandoffStatus = "accepted"
	HandoffStatusRejected HandoffStatus = "rejected"
)
