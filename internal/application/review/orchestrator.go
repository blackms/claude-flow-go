// Package review provides review application services.
package review

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	domainReview "github.com/anthropics/claude-flow-go/internal/domain/review"
	infraReview "github.com/anthropics/claude-flow-go/internal/infrastructure/review"
)

// ReviewOrchestrator orchestrates the review workflow.
type ReviewOrchestrator struct {
	mu sync.RWMutex

	// Infrastructure components
	router           *infraReview.ReviewRouter
	challengeManager *infraReview.ChallengeManager
	divergentDetector *infraReview.DivergentDetector

	// Active sessions
	sessions map[string]*domainReview.ReviewSession

	// Active workflows
	workflows map[string]*domainReview.ReviewWorkflow

	// Statistics
	totalReviews     int64
	completedReviews int64
	approvalCount    int64
	totalDurationMs  int64
}

// NewReviewOrchestrator creates a new review orchestrator.
func NewReviewOrchestrator(
	router *infraReview.ReviewRouter,
	challengeManager *infraReview.ChallengeManager,
	divergentDetector *infraReview.DivergentDetector,
) *ReviewOrchestrator {
	return &ReviewOrchestrator{
		router:           router,
		challengeManager: challengeManager,
		divergentDetector: divergentDetector,
		sessions:         make(map[string]*domainReview.ReviewSession),
		workflows:        make(map[string]*domainReview.ReviewWorkflow),
	}
}

// NewReviewOrchestratorDefault creates an orchestrator with default components.
func NewReviewOrchestratorDefault() *ReviewOrchestrator {
	router := infraReview.NewReviewRouter(infraReview.DefaultRouterConfig())
	challengeManager := infraReview.NewChallengeManager(infraReview.DefaultChallengeManagerConfig())
	divergentDetector := infraReview.NewDivergentDetector(infraReview.DefaultDivergentDetectorConfig())

	return NewReviewOrchestrator(router, challengeManager, divergentDetector)
}

// RequestReview requests a review for a work item.
func (o *ReviewOrchestrator) RequestReview(
	ctx context.Context,
	work domainReview.Work,
	opts domainReview.ReviewOptions,
) (*domainReview.ReviewSession, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Validate options
	if opts.RequiredReviewers <= 0 {
		opts.RequiredReviewers = 1
	}

	// Create session
	now := time.Now()
	sessionID := uuid.New().String()
	session := &domainReview.ReviewSession{
		SessionID:  sessionID,
		WorkID:     work.WorkID,
		ReviewType: opts.ReviewType,
		Reviewers:  make([]domainReview.ReviewerAssignment, 0),
		Verdicts:   make([]domainReview.ReviewVerdict, 0),
		Status:     domainReview.WorkflowStatusPending,
		Deadline:   opts.Deadline,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Assign reviewers
	var reviewerIDs []string
	if len(opts.SpecificReviewers) > 0 {
		reviewerIDs = opts.SpecificReviewers
	} else {
		reviewerIDs = o.router.RouteToReviewers(work, opts.ReviewType, opts.RequiredReviewers)
	}

	for _, reviewerID := range reviewerIDs {
		session.Reviewers = append(session.Reviewers, domainReview.ReviewerAssignment{
			ReviewerID:   reviewerID,
			Role:         domainReview.RoleChallenger,
			AssignedAt:   now,
			HasSubmitted: false,
		})
		o.router.MarkReviewStarted(reviewerID)
	}

	session.Status = domainReview.WorkflowStatusInProgress

	o.sessions[sessionID] = session
	o.totalReviews++

	return session, nil
}

// RequestFullReview creates a full workflow with multiple stages.
func (o *ReviewOrchestrator) RequestFullReview(
	ctx context.Context,
	work domainReview.Work,
	stages []domainReview.ReviewType,
) (*domainReview.ReviewWorkflow, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(stages) == 0 {
		stages = []domainReview.ReviewType{
			domainReview.ReviewTypeSecurity,
			domainReview.ReviewTypeQuality,
			domainReview.ReviewTypeArchitecture,
		}
	}

	now := time.Now()
	workflowID := uuid.New().String()

	workflow := &domainReview.ReviewWorkflow{
		WorkflowID:        workflowID,
		WorkID:            work.WorkID,
		AuthorID:          work.AuthorID,
		Status:            domainReview.WorkflowStatusPending,
		Stages:            make([]domainReview.ReviewStage, len(stages)),
		CurrentStage:      0,
		RequiredApprovals: len(stages),
		CurrentApprovals:  0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	for i, stageType := range stages {
		workflow.Stages[i] = domainReview.ReviewStage{
			Name:              string(stageType),
			Type:              stageType,
			Status:            domainReview.WorkflowStatusPending,
			RequiredReviewers: 1,
			Verdicts:          make([]domainReview.ReviewVerdict, 0),
		}
	}

	// Start first stage
	workflow.Status = domainReview.WorkflowStatusInProgress
	workflow.Stages[0].Status = domainReview.WorkflowStatusInProgress
	stageStart := now
	workflow.Stages[0].StartedAt = &stageStart

	o.workflows[workflowID] = workflow

	return workflow, nil
}

// SubmitVerdict submits a review verdict.
func (o *ReviewOrchestrator) SubmitVerdict(
	ctx context.Context,
	sessionID string,
	verdict domainReview.ReviewVerdict,
) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	session, ok := o.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status == domainReview.WorkflowStatusCompleted {
		return fmt.Errorf("session already completed")
	}

	// Validate reviewer is assigned
	found := false
	for i, assignment := range session.Reviewers {
		if assignment.ReviewerID == verdict.ReviewerID {
			if assignment.HasSubmitted {
				return fmt.Errorf("reviewer has already submitted")
			}
			session.Reviewers[i].HasSubmitted = true
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("reviewer not assigned to session: %s", verdict.ReviewerID)
	}

	verdict.SessionID = sessionID
	verdict.CompletedAt = time.Now()
	verdict.ReviewDuration = verdict.CompletedAt.Sub(verdict.StartedAt)

	session.Verdicts = append(session.Verdicts, verdict)
	session.UpdatedAt = time.Now()

	// Mark review completed for router
	o.router.MarkReviewCompleted(verdict.ReviewerID)

	// Check if all reviewers have submitted
	allSubmitted := true
	for _, assignment := range session.Reviewers {
		if !assignment.HasSubmitted {
			allSubmitted = false
			break
		}
	}

	if allSubmitted {
		session.Status = domainReview.WorkflowStatusCompleted
		o.completedReviews++
		o.totalDurationMs += verdict.ReviewDuration.Milliseconds()

		// Count approvals
		for _, v := range session.Verdicts {
			if v.Decision == domainReview.DecisionApprove {
				o.approvalCount++
			}
		}
	}

	return nil
}

// SubmitWorkflowVerdict submits a verdict to a workflow stage.
func (o *ReviewOrchestrator) SubmitWorkflowVerdict(
	ctx context.Context,
	workflowID string,
	verdict domainReview.ReviewVerdict,
) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	workflow, ok := o.workflows[workflowID]
	if !ok {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	stage := workflow.GetCurrentStage()
	if stage == nil {
		return fmt.Errorf("no current stage")
	}

	if stage.Status == domainReview.WorkflowStatusCompleted {
		return fmt.Errorf("current stage already completed")
	}

	verdict.CompletedAt = time.Now()
	stage.Verdicts = append(stage.Verdicts, verdict)

	// Collect blocking issues
	for _, issue := range verdict.Issues {
		if issue.IsBlocker {
			workflow.Blockers = append(workflow.Blockers, issue)
		}
	}

	// Check if stage is complete
	if stage.IsComplete() {
		now := time.Now()
		stage.CompletedAt = &now
		stage.Status = domainReview.WorkflowStatusCompleted

		stageDecision := stage.GetDecision()
		if stageDecision == domainReview.DecisionApprove {
			workflow.CurrentApprovals++
		}

		// Move to next stage or complete workflow
		if !workflow.AdvanceStage() {
			// No more stages
			o.completeWorkflow(workflow)
		} else {
			// Start next stage
			nextStage := workflow.GetCurrentStage()
			if nextStage != nil {
				nextStage.Status = domainReview.WorkflowStatusInProgress
				nextStage.StartedAt = &now
			}
		}
	}

	workflow.UpdatedAt = time.Now()

	return nil
}

func (o *ReviewOrchestrator) completeWorkflow(workflow *domainReview.ReviewWorkflow) {
	now := time.Now()
	workflow.CompletedAt = &now
	workflow.Status = domainReview.WorkflowStatusCompleted

	// Determine final decision
	if len(workflow.Blockers) > 0 {
		decision := domainReview.DecisionReject
		workflow.FinalDecision = &decision
	} else if workflow.CurrentApprovals >= workflow.RequiredApprovals {
		decision := domainReview.DecisionApprove
		workflow.FinalDecision = &decision
	} else {
		decision := domainReview.DecisionRevise
		workflow.FinalDecision = &decision
	}
}

// GetReviewStatus returns the status of a review session.
func (o *ReviewOrchestrator) GetReviewStatus(sessionID string) (*domainReview.ReviewSession, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	session, ok := o.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// GetWorkflowStatus returns the status of a workflow.
func (o *ReviewOrchestrator) GetWorkflowStatus(workflowID string) (*domainReview.ReviewWorkflow, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	workflow, ok := o.workflows[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return workflow, nil
}

// EscalateReview escalates a review session.
func (o *ReviewOrchestrator) EscalateReview(sessionID string, reason string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	session, ok := o.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Create an escalation challenge
	_, err := o.challengeManager.CreateChallenge(
		session.WorkID,
		sessionID,
		"system",
		reason,
		nil,
		"escalate",
	)

	return err
}

// ChallengeDecision challenges a review decision.
func (o *ReviewOrchestrator) ChallengeDecision(
	sessionID string,
	challengerID string,
	reason string,
	evidence []string,
) (*domainReview.Challenge, error) {
	o.mu.RLock()
	session, ok := o.sessions[sessionID]
	o.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return o.challengeManager.CreateChallenge(
		session.WorkID,
		sessionID,
		challengerID,
		reason,
		evidence,
		"re-review",
	)
}

// ResolveChallengeByOrchestrator resolves a challenge.
func (o *ReviewOrchestrator) ResolveChallengeByOrchestrator(
	challengeID string,
	resolverID string,
	accepted bool,
	reasoning string,
) error {
	decision := domainReview.ChallengeStatusRejected
	if accepted {
		decision = domainReview.ChallengeStatusAccepted
	}

	return o.challengeManager.ResolveChallenge(challengeID, resolverID, decision, reasoning)
}

// GetActiveReviews returns all active review sessions.
func (o *ReviewOrchestrator) GetActiveReviews() []*domainReview.ReviewSession {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]*domainReview.ReviewSession, 0)
	for _, session := range o.sessions {
		if session.Status == domainReview.WorkflowStatusInProgress {
			result = append(result, session)
		}
	}

	return result
}

// GetActiveWorkflows returns all active workflows.
func (o *ReviewOrchestrator) GetActiveWorkflows() []*domainReview.ReviewWorkflow {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]*domainReview.ReviewWorkflow, 0)
	for _, workflow := range o.workflows {
		if !workflow.Status.IsTerminal() {
			result = append(result, workflow)
		}
	}

	return result
}

// GetStats returns orchestrator statistics.
func (o *ReviewOrchestrator) GetStats() domainReview.ReviewStats {
	o.mu.RLock()
	defer o.mu.RUnlock()

	activeSessions := 0
	for _, session := range o.sessions {
		if session.Status == domainReview.WorkflowStatusInProgress {
			activeSessions++
		}
	}

	approvalRate := 0.0
	if o.completedReviews > 0 {
		approvalRate = float64(o.approvalCount) / float64(o.completedReviews)
	}

	avgDuration := 0.0
	if o.completedReviews > 0 {
		avgDuration = float64(o.totalDurationMs) / float64(o.completedReviews)
	}

	challengeStats := o.challengeManager.GetStats()

	return domainReview.ReviewStats{
		TotalReviews:         o.totalReviews,
		ActiveSessions:       activeSessions,
		CompletedReviews:     o.completedReviews,
		ApprovalRate:         approvalRate,
		AvgReviewDurationMs:  avgDuration,
		TotalChallenges:      challengeStats.TotalChallenges,
		ChallengeSuccessRate: challengeStats.ChallengeSuccessRate,
	}
}

// GetRouter returns the review router.
func (o *ReviewOrchestrator) GetRouter() *infraReview.ReviewRouter {
	return o.router
}

// GetChallengeManager returns the challenge manager.
func (o *ReviewOrchestrator) GetChallengeManager() *infraReview.ChallengeManager {
	return o.challengeManager
}

// GetDivergentDetector returns the divergent detector.
func (o *ReviewOrchestrator) GetDivergentDetector() *infraReview.DivergentDetector {
	return o.divergentDetector
}

// RegisterReviewer registers a reviewer with the orchestrator.
func (o *ReviewOrchestrator) RegisterReviewer(info infraReview.ReviewerInfo) {
	o.router.RegisterReviewer(info)
}

// Cleanup removes old completed sessions and workflows.
func (o *ReviewOrchestrator) Cleanup(maxAge time.Duration) int {
	o.mu.Lock()
	defer o.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, session := range o.sessions {
		if session.Status.IsTerminal() && session.CreatedAt.Before(cutoff) {
			delete(o.sessions, id)
			removed++
		}
	}

	for id, workflow := range o.workflows {
		if workflow.Status.IsTerminal() && workflow.CreatedAt.Before(cutoff) {
			delete(o.workflows, id)
			removed++
		}
	}

	return removed
}
