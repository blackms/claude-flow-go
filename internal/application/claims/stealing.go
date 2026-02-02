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

// WorkStealingService provides work stealing operations.
type WorkStealingService struct {
	mu            sync.RWMutex
	eventStore    infraClaims.EventStore
	claimRepo     infraClaims.ClaimRepository
	claimantRepo  infraClaims.ClaimantRepository
	config        domainClaims.StealConfig
	contestWindow time.Duration
	activeContests map[string]*StealContest
	correlationID string
}

// StealContest represents an active contest for a steal.
type StealContest struct {
	ID               string                    `json:"id"`
	ClaimID          string                    `json:"claimId"`
	Stealer          *domainClaims.Claimant    `json:"stealer"`
	OriginalOwner    *domainClaims.Claimant    `json:"originalOwner"`
	StartedAt        time.Time                 `json:"startedAt"`
	ExpiresAt        time.Time                 `json:"expiresAt"`
	Contested        bool                      `json:"contested"`
	ContestReason    string                    `json:"contestReason,omitempty"`
	Resolution       StealContestResolution    `json:"resolution,omitempty"`
	ResolvedAt       *time.Time                `json:"resolvedAt,omitempty"`
}

// StealContestResolution represents the resolution of a steal contest.
type StealContestResolution string

const (
	ResolutionPending       StealContestResolution = "pending"
	ResolutionStealApproved StealContestResolution = "steal_approved"
	ResolutionStealRejected StealContestResolution = "steal_rejected"
)

// NewWorkStealingService creates a new work stealing service.
func NewWorkStealingService(
	eventStore infraClaims.EventStore,
	claimRepo infraClaims.ClaimRepository,
	claimantRepo infraClaims.ClaimantRepository,
) *WorkStealingService {
	config := domainClaims.DefaultStealConfig()
	return &WorkStealingService{
		eventStore:     eventStore,
		claimRepo:      claimRepo,
		claimantRepo:   claimantRepo,
		config:         config,
		contestWindow:  config.ContestWindow,
		activeContests: make(map[string]*StealContest),
	}
}

// SetConfig sets the steal configuration.
func (s *WorkStealingService) SetConfig(config domainClaims.StealConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
	s.contestWindow = config.ContestWindow
}

// SetCorrelationID sets the correlation ID for operations.
func (s *WorkStealingService) SetCorrelationID(correlationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.correlationID = correlationID
}

// MarkStealable marks a claim as stealable.
func (s *WorkStealingService) MarkStealable(claimID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	// Check if marking as stealable is allowed
	canMark, errReason := domainClaims.CanMarkAsStealable(claim, s.config)
	if !canMark {
		return fmt.Errorf("cannot mark as stealable: %s", errReason)
	}

	// Update claim
	claim.MarkStealable()
	if err := s.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Emit event
	event := domainClaims.NewIssueMarkedStealableEvent(claimID, claim.Version, reason)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	return nil
}

// Steal attempts to steal a claim.
func (s *WorkStealingService) Steal(claimID string, stealer *domainClaims.Claimant) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	claim, err := s.claimRepo.FindByID(claimID)
	if err != nil {
		return fmt.Errorf("claim not found: %w", err)
	}

	// Check if steal is allowed
	canSteal, reason := domainClaims.CanStealClaim(claim, stealer, s.config)
	if !canSteal {
		return fmt.Errorf("cannot steal claim: %s", reason)
	}

	// Check if contest is required
	requiresContest := domainClaims.RequiresStealContest(claim, stealer)
	if requiresContest {
		return s.startContest(claim, stealer)
	}

	// Immediate steal
	return s.executeSteal(claim, stealer)
}

// startContest starts a steal contest.
func (s *WorkStealingService) startContest(claim *domainClaims.Claim, stealer *domainClaims.Claimant) error {
	contestID := uuid.New().String()
	now := time.Now()

	contest := &StealContest{
		ID:            contestID,
		ClaimID:       claim.ID,
		Stealer:       stealer,
		OriginalOwner: &claim.Claimant,
		StartedAt:     now,
		ExpiresAt:     now.Add(s.contestWindow),
		Contested:     false,
		Resolution:    ResolutionPending,
	}

	s.activeContests[claim.ID] = contest

	// Emit contest started event
	event := domainClaims.NewClaimEvent(domainClaims.EventStealContestStarted, claim.ID, claim.Version+1, map[string]interface{}{
		"contestId":      contestID,
		"claimId":        claim.ID,
		"stealerId":      stealer.ID,
		"originalOwnerId": claim.Claimant.ID,
		"expiresAt":      contest.ExpiresAt,
	})
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	return nil
}

// ContestSteal contests an active steal attempt.
func (s *WorkStealingService) ContestSteal(claimID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	contest, exists := s.activeContests[claimID]
	if !exists {
		return fmt.Errorf("no active contest for claim: %s", claimID)
	}

	if time.Now().After(contest.ExpiresAt) {
		return fmt.Errorf("contest window has expired")
	}

	contest.Contested = true
	contest.ContestReason = reason

	return nil
}

// ResolveContest resolves a steal contest.
func (s *WorkStealingService) ResolveContest(claimID string, approveSteal bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	contest, exists := s.activeContests[claimID]
	if !exists {
		return fmt.Errorf("no active contest for claim: %s", claimID)
	}

	now := time.Now()
	contest.ResolvedAt = &now

	if approveSteal {
		contest.Resolution = ResolutionStealApproved

		claim, err := s.claimRepo.FindByID(claimID)
		if err != nil {
			return fmt.Errorf("claim not found: %w", err)
		}

		if err := s.executeSteal(claim, contest.Stealer); err != nil {
			return err
		}
	} else {
		contest.Resolution = ResolutionStealRejected
	}

	// Emit contest resolved event
	claim, _ := s.claimRepo.FindByID(claimID)
	version := 1
	if claim != nil {
		version = claim.Version
	}

	event := domainClaims.NewClaimEvent(domainClaims.EventStealContestResolved, claimID, version, map[string]interface{}{
		"contestId":  contest.ID,
		"claimId":    claimID,
		"resolution": contest.Resolution,
		"contested":  contest.Contested,
		"reason":     contest.ContestReason,
	})
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	// Remove from active contests
	delete(s.activeContests, claimID)

	return nil
}

// executeSteal executes the steal operation.
func (s *WorkStealingService) executeSteal(claim *domainClaims.Claim, stealer *domainClaims.Claimant) error {
	originalOwnerID := claim.Claimant.ID

	// Update claim ownership
	claim.Claimant = *stealer
	claim.Status = domainClaims.StatusActive
	claim.LastActivityAt = time.Now()
	claim.Version++

	if err := s.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Update workloads
	originalOwner, err := s.claimantRepo.FindByID(originalOwnerID)
	if err == nil {
		originalOwner.DecrementWorkload()
		_ = s.claimantRepo.Save(originalOwner)
	}

	stealer.IncrementWorkload()
	_ = s.claimantRepo.Save(stealer)

	// Emit stolen event
	event := domainClaims.NewIssueStolenEvent(claim.ID, claim.Version, stealer.ID, originalOwnerID)
	if s.correlationID != "" {
		event.WithCorrelation(s.correlationID)
	}
	_ = s.eventStore.Append(event)

	// Remove from active contests if any
	delete(s.activeContests, claim.ID)

	return nil
}

// GetStealable returns all stealable claims.
func (s *WorkStealingService) GetStealable() ([]*domainClaims.Claim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.claimRepo.FindStealable()
}

// GetActiveContest returns the active contest for a claim.
func (s *WorkStealingService) GetActiveContest(claimID string) (*StealContest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	contest, exists := s.activeContests[claimID]
	if !exists {
		return nil, false
	}
	contestCopy := *contest
	return &contestCopy, true
}

// GetActiveContests returns all active contests.
func (s *WorkStealingService) GetActiveContests() []*StealContest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*StealContest, 0, len(s.activeContests))
	for _, contest := range s.activeContests {
		contestCopy := *contest
		result = append(result, &contestCopy)
	}
	return result
}

// DetectStaleWork detects and marks stale claims as stealable.
func (s *WorkStealingService) DetectStaleWork() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	claims, err := s.claimRepo.FindActive()
	if err != nil {
		return nil, err
	}

	markedIDs := make([]string, 0)

	for _, claim := range claims {
		// Skip if already stealable
		if claim.Status == domainClaims.StatusStealable {
			continue
		}

		// Check if stale
		if domainClaims.IsClaimStale(claim, s.config) {
			canMark, _ := domainClaims.CanMarkAsStealable(claim, s.config)
			if canMark {
				claim.MarkStealable()
				if err := s.claimRepo.Save(claim); err == nil {
					event := domainClaims.NewIssueMarkedStealableEvent(claim.ID, claim.Version, "auto-detected stale")
					_ = s.eventStore.Append(event)
					markedIDs = append(markedIDs, claim.ID)
				}
			}
		}

		// Check if blocked too long
		if domainClaims.IsClaimBlocked(claim, s.config) {
			canMark, _ := domainClaims.CanMarkAsStealable(claim, s.config)
			if canMark {
				claim.MarkStealable()
				if err := s.claimRepo.Save(claim); err == nil {
					event := domainClaims.NewIssueMarkedStealableEvent(claim.ID, claim.Version, "auto-detected blocked too long")
					_ = s.eventStore.Append(event)
					markedIDs = append(markedIDs, claim.ID)
				}
			}
		}
	}

	return markedIDs, nil
}

// ProcessExpiredContests processes and auto-resolves expired contests.
func (s *WorkStealingService) ProcessExpiredContests() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	resolvedIDs := make([]string, 0)

	for claimID, contest := range s.activeContests {
		if now.After(contest.ExpiresAt) && contest.Resolution == ResolutionPending {
			// If not contested, approve steal
			if !contest.Contested {
				claim, err := s.claimRepo.FindByID(claimID)
				if err == nil {
					if err := s.executeSteal(claim, contest.Stealer); err == nil {
						resolvedIDs = append(resolvedIDs, claimID)
					}
				}
			} else {
				// If contested, reject steal (requires manual resolution)
				contest.Resolution = ResolutionStealRejected
				resolvedTime := now
				contest.ResolvedAt = &resolvedTime
				delete(s.activeContests, claimID)
				resolvedIDs = append(resolvedIDs, claimID)
			}
		}
	}

	return resolvedIDs, nil
}

// GetStealStats returns work stealing statistics.
func (s *WorkStealingService) GetStealStats() StealStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stealable, _ := s.claimRepo.FindStealable()

	return StealStats{
		StealableCount:  len(stealable),
		ActiveContests:  len(s.activeContests),
		GracePeriod:     s.config.GracePeriod,
		StaleThreshold:  s.config.StaleThreshold,
		ContestWindow:   s.contestWindow,
	}
}

// StealStats holds work stealing statistics.
type StealStats struct {
	StealableCount  int           `json:"stealableCount"`
	ActiveContests  int           `json:"activeContests"`
	GracePeriod     time.Duration `json:"gracePeriod"`
	StaleThreshold  time.Duration `json:"staleThreshold"`
	ContestWindow   time.Duration `json:"contestWindow"`
}
