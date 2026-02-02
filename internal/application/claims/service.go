// Package claims provides application services for the claims system.
package claims

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	domainClaims "github.com/anthropics/claude-flow-go/internal/domain/claims"
	infraClaims "github.com/anthropics/claude-flow-go/internal/infrastructure/claims"
)

// ClaimService provides claim operations.
type ClaimService struct {
	mu               sync.RWMutex
	eventStore       infraClaims.EventStore
	claimRepo        infraClaims.ClaimRepository
	issueRepo        infraClaims.IssueRepository
	claimantRepo     infraClaims.ClaimantRepository
	stealConfig      domainClaims.StealConfig
	correlationID    string
}

// NewClaimService creates a new claim service.
func NewClaimService(
	eventStore infraClaims.EventStore,
	claimRepo infraClaims.ClaimRepository,
	issueRepo infraClaims.IssueRepository,
	claimantRepo infraClaims.ClaimantRepository,
) *ClaimService {
	return &ClaimService{
		eventStore:   eventStore,
		claimRepo:    claimRepo,
		issueRepo:    issueRepo,
		claimantRepo: claimantRepo,
		stealConfig:  domainClaims.DefaultStealConfig(),
	}
}

// SetCorrelationID sets the correlation ID for operations.
func (s *ClaimService) SetCorrelationID(correlationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.correlationID = correlationID
}

// Claim creates a new claim on an issue.
func (s *ClaimService) Claim(issueID string, claimant *domainClaims.Claimant) (*domainClaims.Claim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get the issue
	issue, err := s.issueRepo.FindByID(issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found: %w", err)
	}

	// Get existing claims
	existingClaims, err := s.claimRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("failed to get existing claims: %w", err)
	}

	// Check if claim is allowed
	canClaim, reason := domainClaims.CanClaimIssue(claimant, issue, existingClaims)
	if !canClaim {
		return nil, fmt.Errorf("cannot claim issue: %s", reason)
	}

	// Create the claim
	claim := domainClaims.NewClaim(uuid.New().String(), issueID, *claimant)

	// Save the claim
	if err := s.claimRepo.Save(claim); err != nil {
		return nil, fmt.Errorf("failed to save claim: %w", err)
	}

	// Update claimant workload
	claimant.IncrementWorkload()
	if err := s.claimantRepo.Save(claimant); err != nil {
		// Log but don't fail
		fmt.Printf("warning: failed to update claimant workload: %v\n", err)
	}

	// Emit event
	event := domainClaims.NewClaimCreatedEvent(claim)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	if err := s.eventStore.Append(event); err != nil {
		// Log but don't fail - claim is already persisted
		fmt.Printf("warning: failed to append event: %v\n", err)
	}

	return claim, nil
}

// Release releases a claim.
func (s *ClaimService) Release(claimID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	if claim.IsTerminal() {
		return fmt.Errorf("claim is already in terminal state")
	}

	// Update claim
	claim.Release()
	if err := s.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Update claimant workload
	claimant, err := s.claimantRepo.FindByID(claim.Claimant.ID)
	if err == nil {
		claimant.DecrementWorkload()
		_ = s.claimantRepo.Save(claimant)
	}

	// Emit event
	event := domainClaims.NewClaimReleasedEvent(claimID, claim.Version, reason)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	return nil
}

// Complete completes a claim.
func (s *ClaimService) Complete(claimID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	if claim.IsTerminal() {
		return fmt.Errorf("claim is already in terminal state")
	}

	// Update claim
	claim.Complete()
	if err := s.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Update claimant workload
	claimant, err := s.claimantRepo.FindByID(claim.Claimant.ID)
	if err == nil {
		claimant.DecrementWorkload()
		_ = s.claimantRepo.Save(claimant)
	}

	// Emit event
	event := domainClaims.NewClaimCompletedEvent(claimID, claim.Version)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	return nil
}

// UpdateStatus updates the status of a claim.
func (s *ClaimService) UpdateStatus(claimID string, status domainClaims.ClaimStatus, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	// Validate transition
	if !domainClaims.CanTransitionStatus(claim.Status, status) {
		return fmt.Errorf("invalid status transition from %s to %s", claim.Status, status)
	}

	oldStatus := claim.Status

	// Update claim
	if err := claim.UpdateStatus(status); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if note != "" {
		claim.AddNote(note)
	}

	if err := s.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Emit event
	event := domainClaims.NewClaimStatusChangedEvent(claimID, claim.Version, oldStatus, status, note)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	return nil
}

// UpdateProgress updates the progress of a claim.
func (s *ClaimService) UpdateProgress(claimID string, progress float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	if claim.IsTerminal() {
		return fmt.Errorf("cannot update progress of terminal claim")
	}

	// Update claim
	claim.UpdateProgress(progress)
	if err := s.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Emit event
	event := domainClaims.NewClaimProgressUpdatedEvent(claimID, claim.Version, progress)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	return nil
}

// RequestHandoff requests a handoff to another claimant.
func (s *ClaimService) RequestHandoff(claimID string, to *domainClaims.Claimant, reason string) (*domainClaims.HandoffRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return nil, fmt.Errorf("claim not found: %w", err)
	}

	// Check if handoff is allowed
	canHandoff, errReason := domainClaims.CanInitiateHandoff(claim, &claim.Claimant, to)
	if !canHandoff {
		return nil, fmt.Errorf("cannot initiate handoff: %s", errReason)
	}

	// Create handoff record
	handoff := domainClaims.NewHandoffRecord(uuid.New().String(), claim.Claimant, *to, reason)

	// Update claim
	claim.AddHandoff(*handoff)
	if err := claim.UpdateStatus(domainClaims.StatusPendingHandoff); err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	if err := s.claimRepo.Save(claim); err != nil {
		return nil, fmt.Errorf("failed to save claim: %w", err)
	}

	// Emit event
	event := domainClaims.NewHandoffRequestedEvent(claimID, claim.Version, handoff)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	return handoff, nil
}

// AcceptHandoff accepts a pending handoff.
func (s *ClaimService) AcceptHandoff(claimID string, accepter *domainClaims.Claimant) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	// Check if can accept
	canAccept, reason := domainClaims.CanAcceptHandoff(claim, accepter)
	if !canAccept {
		return fmt.Errorf("cannot accept handoff: %s", reason)
	}

	handoff := claim.GetPendingHandoff()

	// Accept handoff
	if err := claim.AcceptHandoff(); err != nil {
		return fmt.Errorf("failed to accept handoff: %w", err)
	}

	if err := s.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Update workloads
	fromClaimant, err := s.claimantRepo.FindByID(handoff.From.ID)
	if err == nil {
		fromClaimant.DecrementWorkload()
		_ = s.claimantRepo.Save(fromClaimant)
	}

	accepter.IncrementWorkload()
	_ = s.claimantRepo.Save(accepter)

	// Emit event
	event := domainClaims.NewHandoffAcceptedEvent(claimID, claim.Version, handoff.ID)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	return nil
}

// RejectHandoff rejects a pending handoff.
func (s *ClaimService) RejectHandoff(claimID string, rejecter *domainClaims.Claimant, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	// Check if can reject
	canReject, errReason := domainClaims.CanRejectHandoff(claim, rejecter)
	if !canReject {
		return fmt.Errorf("cannot reject handoff: %s", errReason)
	}

	handoff := claim.GetPendingHandoff()

	// Reject handoff
	if err := claim.RejectHandoff(reason); err != nil {
		return fmt.Errorf("failed to reject handoff: %w", err)
	}

	if err := s.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Emit event
	event := domainClaims.NewHandoffRejectedEvent(claimID, claim.Version, handoff.ID, reason)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	return nil
}

// GetClaimByID returns a claim by ID.
func (s *ClaimService) GetClaimByID(claimID string) (*domainClaims.Claim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.claimRepo.FindByID(claimID)
}

// GetClaimedBy returns claims for a claimant.
func (s *ClaimService) GetClaimedBy(claimantID string) ([]*domainClaims.Claim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.claimRepo.FindByClaimant(claimantID)
}

// GetActiveClaims returns all active claims.
func (s *ClaimService) GetActiveClaims() ([]*domainClaims.Claim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.claimRepo.FindActive()
}

// GetAvailableIssues returns unclaimed issues.
func (s *ClaimService) GetAvailableIssues() ([]*domainClaims.Issue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	claims, err := s.claimRepo.FindActive()
	if err != nil {
		return nil, err
	}

	return s.issueRepo.FindUnclaimed(claims)
}

// ExpireStaleClaimsCheck checks for and expires stale claims.
func (s *ClaimService) ExpireStaleClaimsCheck() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	claims, err := s.claimRepo.FindActive()
	if err != nil {
		return 0, err
	}

	expiredCount := 0
	for _, claim := range claims {
		if claim.IsExpired() {
			claim.Expire()
			if err := s.claimRepo.Save(claim); err == nil {
				event := domainClaims.NewClaimExpiredEvent(claim.ID, claim.Version)
				_ = s.eventStore.Append(event)
				expiredCount++
			}
		}
	}

	return expiredCount, nil
}

// GetStats returns claim statistics.
func (s *ClaimService) GetStats() ClaimStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return ClaimStats{
		TotalClaims:     s.claimRepo.Count(),
		ActiveClaims:    s.claimRepo.CountByStatus(domainClaims.StatusActive),
		CompletedClaims: s.claimRepo.CountByStatus(domainClaims.StatusCompleted),
		StealableClaims: s.claimRepo.CountByStatus(domainClaims.StatusStealable),
		TotalIssues:     s.issueRepo.Count(),
		TotalClaimants:  s.claimantRepo.Count(),
		EventCount:      s.eventStore.Count(),
	}
}

// ClaimStats holds claim statistics.
type ClaimStats struct {
	TotalClaims     int `json:"totalClaims"`
	ActiveClaims    int `json:"activeClaims"`
	CompletedClaims int `json:"completedClaims"`
	StealableClaims int `json:"stealableClaims"`
	TotalIssues     int `json:"totalIssues"`
	TotalClaimants  int `json:"totalClaimants"`
	EventCount      int `json:"eventCount"`
}

// RegisterIssue registers an issue.
func (s *ClaimService) RegisterIssue(issue *domainClaims.Issue) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.issueRepo.Save(issue)
}

// RegisterClaimant registers a claimant.
func (s *ClaimService) RegisterClaimant(claimant *domainClaims.Claimant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.claimantRepo.Save(claimant)
}

// GetClaimant returns a claimant by ID.
func (s *ClaimService) GetClaimant(claimantID string) (*domainClaims.Claimant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.claimantRepo.FindByID(claimantID)
}

// GetAllClaimants returns all claimants.
func (s *ClaimService) GetAllClaimants() ([]*domainClaims.Claimant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.claimantRepo.FindAll()
}

// GetIssue returns an issue by ID.
func (s *ClaimService) GetIssue(issueID string) (*domainClaims.Issue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.issueRepo.FindByID(issueID)
}

// GetAllIssues returns all issues.
func (s *ClaimService) GetAllIssues() ([]*domainClaims.Issue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.issueRepo.FindAll()
}

// ReplayEvents replays events to rebuild state.
func (s *ClaimService) ReplayEvents(handler domainClaims.EventHandler) error {
	events, err := s.eventStore.GetAllEvents()
	if err != nil {
		return err
	}

	for _, event := range events {
		handler(event)
	}

	return nil
}

// ExpireClaim expires a specific claim.
func (s *ClaimService) ExpireClaim(claimID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	if claim.IsTerminal() {
		return fmt.Errorf("claim is already in terminal state")
	}

	claim.Expire()
	if err := s.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Update claimant workload
	claimant, err := s.claimantRepo.FindByID(claim.Claimant.ID)
	if err == nil {
		claimant.DecrementWorkload()
		_ = s.claimantRepo.Save(claimant)
	}

	event := domainClaims.NewClaimExpiredEvent(claimID, claim.Version)
	_ = s.eventStore.Append(event)

	return nil
}

// SetExpiration sets an expiration time on a claim.
func (s *ClaimService) SetExpiration(claimID string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	claim.ExpiresAt = &expiresAt
	claim.Version++

	return s.claimRepo.Save(claim)
}
