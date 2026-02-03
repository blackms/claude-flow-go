// Package review provides domain types for adversarial review systems.
package review

import (
	"time"
)

// AdversarialRole represents the role of an agent in a review.
type AdversarialRole string

const (
	// RoleChallenger challenges the work being reviewed.
	RoleChallenger AdversarialRole = "challenger"

	// RoleDefender defends the work being reviewed.
	RoleDefender AdversarialRole = "defender"

	// RoleJudge makes the final decision.
	RoleJudge AdversarialRole = "judge"

	// RoleObserver watches and learns from the review.
	RoleObserver AdversarialRole = "observer"
)

// ReviewDecision represents the outcome of a review.
type ReviewDecision string

const (
	// DecisionApprove approves the work.
	DecisionApprove ReviewDecision = "approve"

	// DecisionReject rejects the work.
	DecisionReject ReviewDecision = "reject"

	// DecisionRevise requests revisions.
	DecisionRevise ReviewDecision = "revise"

	// DecisionEscalate escalates to higher authority.
	DecisionEscalate ReviewDecision = "escalate"
)

// IsApproved returns true if the decision is approval.
func (d ReviewDecision) IsApproved() bool {
	return d == DecisionApprove
}

// IsTerminal returns true if the decision is final.
func (d ReviewDecision) IsTerminal() bool {
	return d == DecisionApprove || d == DecisionReject
}

// Severity represents the severity of an issue.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Weight returns a numeric weight for the severity.
func (s Severity) Weight() int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 1
	}
}

// ReviewType represents the type of review.
type ReviewType string

const (
	ReviewTypeSecurity     ReviewType = "security"
	ReviewTypeQuality      ReviewType = "quality"
	ReviewTypePerformance  ReviewType = "performance"
	ReviewTypeArchitecture ReviewType = "architecture"
	ReviewTypeCompliance   ReviewType = "compliance"
	ReviewTypeFull         ReviewType = "full"
)

// AllReviewTypes returns all review types.
func AllReviewTypes() []ReviewType {
	return []ReviewType{
		ReviewTypeSecurity, ReviewTypeQuality, ReviewTypePerformance,
		ReviewTypeArchitecture, ReviewTypeCompliance, ReviewTypeFull,
	}
}

// ReviewIssue represents an issue found during review.
type ReviewIssue struct {
	// ID is the unique issue identifier.
	ID string `json:"id"`

	// Type is the category of issue.
	Type string `json:"type"`

	// Severity is the issue severity.
	Severity Severity `json:"severity"`

	// Title is a brief description.
	Title string `json:"title"`

	// Description is the full description.
	Description string `json:"description"`

	// Location is where the issue was found.
	Location string `json:"location,omitempty"`

	// Recommendation is how to fix it.
	Recommendation string `json:"recommendation,omitempty"`

	// IsBlocker indicates if this blocks approval.
	IsBlocker bool `json:"isBlocker"`
}

// ReviewVerdict represents a reviewer's verdict on work.
type ReviewVerdict struct {
	// VerdictID is the unique verdict identifier.
	VerdictID string `json:"verdictId"`

	// WorkID is the work being reviewed.
	WorkID string `json:"workId"`

	// SessionID is the review session.
	SessionID string `json:"sessionId"`

	// ReviewerID is the reviewer agent.
	ReviewerID string `json:"reviewerId"`

	// ReviewerType is the type of reviewer.
	ReviewerType string `json:"reviewerType"`

	// ReviewerRole is the role in this review.
	ReviewerRole AdversarialRole `json:"reviewerRole"`

	// ReviewType is the type of review performed.
	ReviewType ReviewType `json:"reviewType"`

	// Decision is the verdict decision.
	Decision ReviewDecision `json:"decision"`

	// Confidence is the confidence level (0-1).
	Confidence float64 `json:"confidence"`

	// Issues found during review.
	Issues []ReviewIssue `json:"issues"`

	// Recommendations for improvement.
	Recommendations []string `json:"recommendations"`

	// Strengths found in the work.
	Strengths []string `json:"strengths,omitempty"`

	// Summary is a brief summary.
	Summary string `json:"summary"`

	// StartedAt is when the review started.
	StartedAt time.Time `json:"startedAt"`

	// CompletedAt is when the review completed.
	CompletedAt time.Time `json:"completedAt"`

	// ReviewDuration is the review duration.
	ReviewDuration time.Duration `json:"reviewDuration"`

	// Metadata contains additional data.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HasBlockers returns true if there are blocking issues.
func (v *ReviewVerdict) HasBlockers() bool {
	for _, issue := range v.Issues {
		if issue.IsBlocker {
			return true
		}
	}
	return false
}

// GetBlockers returns all blocking issues.
func (v *ReviewVerdict) GetBlockers() []ReviewIssue {
	blockers := make([]ReviewIssue, 0)
	for _, issue := range v.Issues {
		if issue.IsBlocker {
			blockers = append(blockers, issue)
		}
	}
	return blockers
}

// CountBySeverity returns issue count by severity.
func (v *ReviewVerdict) CountBySeverity() map[Severity]int {
	counts := make(map[Severity]int)
	for _, issue := range v.Issues {
		counts[issue.Severity]++
	}
	return counts
}

// ChallengeStatus represents the status of a challenge.
type ChallengeStatus string

const (
	ChallengeStatusPending   ChallengeStatus = "pending"
	ChallengeStatusAccepted  ChallengeStatus = "accepted"
	ChallengeStatusRejected  ChallengeStatus = "rejected"
	ChallengeStatusEscalated ChallengeStatus = "escalated"
	ChallengeStatusWithdrawn ChallengeStatus = "withdrawn"
)

// IsResolved returns true if the challenge is resolved.
func (s ChallengeStatus) IsResolved() bool {
	return s == ChallengeStatusAccepted || s == ChallengeStatusRejected || s == ChallengeStatusWithdrawn
}

// ChallengeResolution represents how a challenge was resolved.
type ChallengeResolution struct {
	// ResolvedBy is who resolved the challenge.
	ResolvedBy string `json:"resolvedBy"`

	// Decision is the resolution decision.
	Decision ChallengeStatus `json:"decision"`

	// Reasoning explains the decision.
	Reasoning string `json:"reasoning"`

	// ResolvedAt is when it was resolved.
	ResolvedAt time.Time `json:"resolvedAt"`
}

// Challenge represents a challenge to a review decision.
type Challenge struct {
	// ChallengeID is the unique challenge identifier.
	ChallengeID string `json:"challengeId"`

	// WorkID is the work being challenged about.
	WorkID string `json:"workId"`

	// SessionID is the review session.
	SessionID string `json:"sessionId"`

	// VerdictID is the verdict being challenged.
	VerdictID string `json:"verdictId,omitempty"`

	// ChallengerID is who issued the challenge.
	ChallengerID string `json:"challengerId"`

	// Reason is why the challenge was issued.
	Reason string `json:"reason"`

	// Evidence supports the challenge.
	Evidence []string `json:"evidence,omitempty"`

	// RequestedAction is what the challenger wants.
	RequestedAction string `json:"requestedAction"`

	// Status is the challenge status.
	Status ChallengeStatus `json:"status"`

	// ContestWindow is how long to contest.
	ContestWindow time.Duration `json:"contestWindow"`

	// ExpiresAt is when the challenge expires.
	ExpiresAt time.Time `json:"expiresAt"`

	// Resolution contains resolution details.
	Resolution *ChallengeResolution `json:"resolution,omitempty"`

	// CreatedAt is when the challenge was created.
	CreatedAt time.Time `json:"createdAt"`

	// Metadata contains additional data.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// IsExpired returns true if the challenge has expired.
func (c *Challenge) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// CanResolve returns true if the challenge can be resolved.
func (c *Challenge) CanResolve() bool {
	return c.Status == ChallengeStatusPending && !c.IsExpired()
}

// WorkflowStatus represents the status of a review workflow.
type WorkflowStatus string

const (
	WorkflowStatusPending    WorkflowStatus = "pending"
	WorkflowStatusInProgress WorkflowStatus = "in_progress"
	WorkflowStatusCompleted  WorkflowStatus = "completed"
	WorkflowStatusBlocked    WorkflowStatus = "blocked"
	WorkflowStatusCancelled  WorkflowStatus = "cancelled"
)

// IsTerminal returns true if the workflow is in a terminal state.
func (s WorkflowStatus) IsTerminal() bool {
	return s == WorkflowStatusCompleted || s == WorkflowStatusCancelled
}

// ReviewStage represents a stage in the review workflow.
type ReviewStage struct {
	// Name is the stage name.
	Name string `json:"name"`

	// Type is the review type for this stage.
	Type ReviewType `json:"type"`

	// Status is the stage status.
	Status WorkflowStatus `json:"status"`

	// RequiredReviewers is how many reviewers needed.
	RequiredReviewers int `json:"requiredReviewers"`

	// Verdicts are the verdicts for this stage.
	Verdicts []ReviewVerdict `json:"verdicts"`

	// StartedAt is when the stage started.
	StartedAt *time.Time `json:"startedAt,omitempty"`

	// CompletedAt is when the stage completed.
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// IsComplete returns true if the stage is complete.
func (s *ReviewStage) IsComplete() bool {
	return len(s.Verdicts) >= s.RequiredReviewers
}

// GetDecision returns the stage decision based on verdicts.
func (s *ReviewStage) GetDecision() ReviewDecision {
	if len(s.Verdicts) == 0 {
		return DecisionRevise
	}

	approvals := 0
	rejections := 0
	for _, v := range s.Verdicts {
		if v.Decision == DecisionApprove {
			approvals++
		} else if v.Decision == DecisionReject {
			rejections++
		}
	}

	// Majority rules
	if approvals > rejections {
		return DecisionApprove
	} else if rejections > approvals {
		return DecisionReject
	}
	return DecisionRevise
}

// ReviewWorkflow represents a complete review workflow.
type ReviewWorkflow struct {
	// WorkflowID is the unique workflow identifier.
	WorkflowID string `json:"workflowId"`

	// WorkID is the work being reviewed.
	WorkID string `json:"workId"`

	// AuthorID is who created the work.
	AuthorID string `json:"authorId"`

	// Status is the workflow status.
	Status WorkflowStatus `json:"status"`

	// Stages are the review stages.
	Stages []ReviewStage `json:"stages"`

	// CurrentStage is the current stage index.
	CurrentStage int `json:"currentStage"`

	// RequiredApprovals is total approvals needed.
	RequiredApprovals int `json:"requiredApprovals"`

	// CurrentApprovals is current approval count.
	CurrentApprovals int `json:"currentApprovals"`

	// Blockers are blocking issues.
	Blockers []ReviewIssue `json:"blockers,omitempty"`

	// Challenges are active challenges.
	Challenges []Challenge `json:"challenges,omitempty"`

	// FinalDecision is the final workflow decision.
	FinalDecision *ReviewDecision `json:"finalDecision,omitempty"`

	// CreatedAt is when the workflow was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the workflow was last updated.
	UpdatedAt time.Time `json:"updatedAt"`

	// CompletedAt is when the workflow completed.
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// Metadata contains additional data.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// GetCurrentStage returns the current stage.
func (w *ReviewWorkflow) GetCurrentStage() *ReviewStage {
	if w.CurrentStage >= 0 && w.CurrentStage < len(w.Stages) {
		return &w.Stages[w.CurrentStage]
	}
	return nil
}

// HasBlockers returns true if there are blockers.
func (w *ReviewWorkflow) HasBlockers() bool {
	return len(w.Blockers) > 0
}

// HasPendingChallenges returns true if there are pending challenges.
func (w *ReviewWorkflow) HasPendingChallenges() bool {
	for _, c := range w.Challenges {
		if c.Status == ChallengeStatusPending {
			return true
		}
	}
	return false
}

// AdvanceStage moves to the next stage.
func (w *ReviewWorkflow) AdvanceStage() bool {
	if w.CurrentStage < len(w.Stages)-1 {
		w.CurrentStage++
		w.UpdatedAt = time.Now()
		return true
	}
	return false
}

// ReviewSession represents an active review session.
type ReviewSession struct {
	// SessionID is the unique session identifier.
	SessionID string `json:"sessionId"`

	// WorkID is the work being reviewed.
	WorkID string `json:"workId"`

	// WorkflowID is the associated workflow.
	WorkflowID string `json:"workflowId,omitempty"`

	// ReviewType is the type of review.
	ReviewType ReviewType `json:"reviewType"`

	// Reviewers are assigned reviewers.
	Reviewers []ReviewerAssignment `json:"reviewers"`

	// Verdicts are submitted verdicts.
	Verdicts []ReviewVerdict `json:"verdicts"`

	// Status is the session status.
	Status WorkflowStatus `json:"status"`

	// Deadline is the review deadline.
	Deadline *time.Time `json:"deadline,omitempty"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the session was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// ReviewerAssignment represents a reviewer assignment.
type ReviewerAssignment struct {
	// ReviewerID is the reviewer agent ID.
	ReviewerID string `json:"reviewerId"`

	// ReviewerType is the reviewer type.
	ReviewerType string `json:"reviewerType"`

	// Role is the reviewer's role.
	Role AdversarialRole `json:"role"`

	// AssignedAt is when they were assigned.
	AssignedAt time.Time `json:"assignedAt"`

	// HasSubmitted indicates if they submitted.
	HasSubmitted bool `json:"hasSubmitted"`
}

// FinalVerdict represents the aggregated final verdict.
type FinalVerdict struct {
	// WorkID is the work that was reviewed.
	WorkID string `json:"workId"`

	// SessionID is the review session.
	SessionID string `json:"sessionId"`

	// Decision is the final decision.
	Decision ReviewDecision `json:"decision"`

	// Confidence is the aggregated confidence.
	Confidence float64 `json:"confidence"`

	// ConsensusLevel is the level of agreement (0-1).
	ConsensusLevel float64 `json:"consensusLevel"`

	// TotalReviewers is the number of reviewers.
	TotalReviewers int `json:"totalReviewers"`

	// Approvals is the approval count.
	Approvals int `json:"approvals"`

	// Rejections is the rejection count.
	Rejections int `json:"rejections"`

	// AllIssues is all issues from all reviewers.
	AllIssues []ReviewIssue `json:"allIssues"`

	// Blockers are critical blocking issues.
	Blockers []ReviewIssue `json:"blockers"`

	// DivergentReviewers are reviewers who disagreed.
	DivergentReviewers []string `json:"divergentReviewers,omitempty"`

	// Summary is a summary of the review.
	Summary string `json:"summary"`

	// CompletedAt is when the verdict was finalized.
	CompletedAt time.Time `json:"completedAt"`
}

// ReviewOptions contains options for requesting a review.
type ReviewOptions struct {
	// ReviewType is the type of review.
	ReviewType ReviewType `json:"reviewType"`

	// Urgency is how urgent (low, normal, high, critical).
	Urgency string `json:"urgency,omitempty"`

	// RequiredReviewers is the number of reviewers needed.
	RequiredReviewers int `json:"requiredReviewers,omitempty"`

	// SpecificReviewers are specific reviewers to assign.
	SpecificReviewers []string `json:"specificReviewers,omitempty"`

	// Deadline is the review deadline.
	Deadline *time.Time `json:"deadline,omitempty"`

	// Metadata contains additional options.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Work represents work to be reviewed.
type Work struct {
	// WorkID is the unique work identifier.
	WorkID string `json:"workId"`

	// Type is the type of work.
	Type string `json:"type"`

	// AuthorID is who created the work.
	AuthorID string `json:"authorId"`

	// Title is the work title.
	Title string `json:"title"`

	// Description is the work description.
	Description string `json:"description,omitempty"`

	// Content is the work content.
	Content interface{} `json:"content"`

	// CreatedAt is when the work was created.
	CreatedAt time.Time `json:"createdAt"`

	// Metadata contains additional data.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DivergentAgent represents an agent that diverges from consensus.
type DivergentAgent struct {
	// AgentID is the agent identifier.
	AgentID string `json:"agentId"`

	// Distance is the deviation from consensus.
	Distance float64 `json:"distance"`

	// VoteHistory is recent voting history.
	VoteHistory []bool `json:"voteHistory,omitempty"`

	// DisagreementRate is how often they disagree.
	DisagreementRate float64 `json:"disagreementRate"`

	// Reason explains why they're divergent.
	Reason string `json:"reason,omitempty"`
}

// ReviewStats contains review statistics.
type ReviewStats struct {
	// TotalReviews is the total review count.
	TotalReviews int64 `json:"totalReviews"`

	// ActiveSessions is the active session count.
	ActiveSessions int `json:"activeSessions"`

	// CompletedReviews is the completed count.
	CompletedReviews int64 `json:"completedReviews"`

	// ApprovalRate is the approval rate.
	ApprovalRate float64 `json:"approvalRate"`

	// AvgReviewDurationMs is the average duration.
	AvgReviewDurationMs float64 `json:"avgReviewDurationMs"`

	// ByReviewType is counts by review type.
	ByReviewType map[ReviewType]int64 `json:"byReviewType"`

	// TotalChallenges is the challenge count.
	TotalChallenges int64 `json:"totalChallenges"`

	// ChallengeSuccessRate is how often challenges succeed.
	ChallengeSuccessRate float64 `json:"challengeSuccessRate"`
}
