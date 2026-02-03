// Package review provides review infrastructure.
package review

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	domainReview "github.com/anthropics/claude-flow-go/internal/domain/review"
)

// ChallengeManagerConfig contains configuration for the challenge manager.
type ChallengeManagerConfig struct {
	// DefaultContestWindow is the default contest window.
	DefaultContestWindow time.Duration

	// MaxChallengesPerWork is the max challenges per work item.
	MaxChallengesPerWork int

	// CooldownPeriod is the cooldown between challenges from the same agent.
	CooldownPeriod time.Duration

	// AutoExpireChallenges expires challenges automatically.
	AutoExpireChallenges bool
}

// DefaultChallengeManagerConfig returns default configuration.
func DefaultChallengeManagerConfig() ChallengeManagerConfig {
	return ChallengeManagerConfig{
		DefaultContestWindow: 5 * time.Minute,
		MaxChallengesPerWork: 3,
		CooldownPeriod:       1 * time.Minute,
		AutoExpireChallenges: true,
	}
}

// ChallengeManager manages review challenges.
type ChallengeManager struct {
	mu     sync.RWMutex
	config ChallengeManagerConfig

	// Challenges indexed by ID
	challenges map[string]*domainReview.Challenge

	// Challenges indexed by work ID
	challengesByWork map[string][]string

	// Last challenge time per agent
	lastChallenge map[string]time.Time

	// Challenge statistics
	totalChallenges    int64
	acceptedChallenges int64
	rejectedChallenges int64
}

// NewChallengeManager creates a new challenge manager.
func NewChallengeManager(config ChallengeManagerConfig) *ChallengeManager {
	cm := &ChallengeManager{
		config:           config,
		challenges:       make(map[string]*domainReview.Challenge),
		challengesByWork: make(map[string][]string),
		lastChallenge:    make(map[string]time.Time),
	}

	// Start expiration goroutine if enabled
	if config.AutoExpireChallenges {
		go cm.expirationLoop()
	}

	return cm
}

// CreateChallenge creates a new challenge.
func (cm *ChallengeManager) CreateChallenge(
	workID string,
	sessionID string,
	challengerID string,
	reason string,
	evidence []string,
	requestedAction string,
) (*domainReview.Challenge, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check cooldown
	if lastTime, ok := cm.lastChallenge[challengerID]; ok {
		if time.Since(lastTime) < cm.config.CooldownPeriod {
			return nil, fmt.Errorf("challenge cooldown not elapsed, wait %v", cm.config.CooldownPeriod-time.Since(lastTime))
		}
	}

	// Check max challenges per work
	existing := cm.challengesByWork[workID]
	pendingCount := 0
	for _, cid := range existing {
		if c, ok := cm.challenges[cid]; ok && c.Status == domainReview.ChallengeStatusPending {
			pendingCount++
		}
	}

	if pendingCount >= cm.config.MaxChallengesPerWork {
		return nil, fmt.Errorf("max challenges (%d) reached for work %s", cm.config.MaxChallengesPerWork, workID)
	}

	now := time.Now()
	challenge := &domainReview.Challenge{
		ChallengeID:     uuid.New().String(),
		WorkID:          workID,
		SessionID:       sessionID,
		ChallengerID:    challengerID,
		Reason:          reason,
		Evidence:        evidence,
		RequestedAction: requestedAction,
		Status:          domainReview.ChallengeStatusPending,
		ContestWindow:   cm.config.DefaultContestWindow,
		ExpiresAt:       now.Add(cm.config.DefaultContestWindow),
		CreatedAt:       now,
	}

	cm.challenges[challenge.ChallengeID] = challenge
	cm.challengesByWork[workID] = append(cm.challengesByWork[workID], challenge.ChallengeID)
	cm.lastChallenge[challengerID] = now
	cm.totalChallenges++

	return challenge, nil
}

// ResolveChallenge resolves a challenge.
func (cm *ChallengeManager) ResolveChallenge(
	challengeID string,
	resolverID string,
	decision domainReview.ChallengeStatus,
	reasoning string,
) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	challenge, ok := cm.challenges[challengeID]
	if !ok {
		return fmt.Errorf("challenge not found: %s", challengeID)
	}

	if !challenge.CanResolve() {
		return fmt.Errorf("challenge cannot be resolved: status=%s, expired=%v", challenge.Status, challenge.IsExpired())
	}

	if decision != domainReview.ChallengeStatusAccepted &&
		decision != domainReview.ChallengeStatusRejected {
		return fmt.Errorf("invalid resolution decision: %s", decision)
	}

	challenge.Status = decision
	challenge.Resolution = &domainReview.ChallengeResolution{
		ResolvedBy: resolverID,
		Decision:   decision,
		Reasoning:  reasoning,
		ResolvedAt: time.Now(),
	}

	if decision == domainReview.ChallengeStatusAccepted {
		cm.acceptedChallenges++
	} else {
		cm.rejectedChallenges++
	}

	return nil
}

// EscalateChallenge escalates a challenge.
func (cm *ChallengeManager) EscalateChallenge(challengeID string, reason string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	challenge, ok := cm.challenges[challengeID]
	if !ok {
		return fmt.Errorf("challenge not found: %s", challengeID)
	}

	if challenge.Status != domainReview.ChallengeStatusPending {
		return fmt.Errorf("only pending challenges can be escalated")
	}

	challenge.Status = domainReview.ChallengeStatusEscalated
	if challenge.Metadata == nil {
		challenge.Metadata = make(map[string]interface{})
	}
	challenge.Metadata["escalationReason"] = reason
	challenge.Metadata["escalatedAt"] = time.Now()

	return nil
}

// WithdrawChallenge withdraws a challenge.
func (cm *ChallengeManager) WithdrawChallenge(challengeID string, challengerID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	challenge, ok := cm.challenges[challengeID]
	if !ok {
		return fmt.Errorf("challenge not found: %s", challengeID)
	}

	if challenge.ChallengerID != challengerID {
		return fmt.Errorf("only the challenger can withdraw")
	}

	if challenge.Status != domainReview.ChallengeStatusPending {
		return fmt.Errorf("only pending challenges can be withdrawn")
	}

	challenge.Status = domainReview.ChallengeStatusWithdrawn

	return nil
}

// GetChallenge returns a challenge by ID.
func (cm *ChallengeManager) GetChallenge(challengeID string) (*domainReview.Challenge, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	challenge, ok := cm.challenges[challengeID]
	return challenge, ok
}

// GetChallengesForWork returns all challenges for a work item.
func (cm *ChallengeManager) GetChallengesForWork(workID string) []*domainReview.Challenge {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	challengeIDs := cm.challengesByWork[workID]
	result := make([]*domainReview.Challenge, 0, len(challengeIDs))

	for _, cid := range challengeIDs {
		if c, ok := cm.challenges[cid]; ok {
			result = append(result, c)
		}
	}

	return result
}

// GetPendingChallenges returns all pending challenges.
func (cm *ChallengeManager) GetPendingChallenges() []*domainReview.Challenge {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*domainReview.Challenge, 0)
	for _, c := range cm.challenges {
		if c.Status == domainReview.ChallengeStatusPending {
			result = append(result, c)
		}
	}

	return result
}

// GetContestWindow returns the contest window for a work item.
func (cm *ChallengeManager) GetContestWindow(workID string) time.Duration {
	// Could be customized per work type in the future
	return cm.config.DefaultContestWindow
}

// HasPendingChallenge returns true if there's a pending challenge for the work.
func (cm *ChallengeManager) HasPendingChallenge(workID string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, cid := range cm.challengesByWork[workID] {
		if c, ok := cm.challenges[cid]; ok && c.Status == domainReview.ChallengeStatusPending {
			return true
		}
	}

	return false
}

// expirationLoop expires old challenges.
func (cm *ChallengeManager) expirationLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cm.expireChallenges()
	}
}

func (cm *ChallengeManager) expireChallenges() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for _, c := range cm.challenges {
		if c.Status == domainReview.ChallengeStatusPending && now.After(c.ExpiresAt) {
			c.Status = domainReview.ChallengeStatusRejected
			c.Resolution = &domainReview.ChallengeResolution{
				ResolvedBy: "system",
				Decision:   domainReview.ChallengeStatusRejected,
				Reasoning:  "Challenge expired without resolution",
				ResolvedAt: now,
			}
			cm.rejectedChallenges++
		}
	}
}

// GetStats returns challenge statistics.
func (cm *ChallengeManager) GetStats() domainReview.ReviewStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	total := cm.totalChallenges
	successRate := 0.0
	if total > 0 {
		successRate = float64(cm.acceptedChallenges) / float64(total)
	}

	return domainReview.ReviewStats{
		TotalChallenges:      total,
		ChallengeSuccessRate: successRate,
	}
}

// Cleanup removes old resolved challenges.
func (cm *ChallengeManager) Cleanup(maxAge time.Duration) int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, c := range cm.challenges {
		if c.Status.IsResolved() && c.CreatedAt.Before(cutoff) {
			delete(cm.challenges, id)
			removed++
		}
	}

	// Clean up work index
	for workID, cids := range cm.challengesByWork {
		remaining := make([]string, 0)
		for _, cid := range cids {
			if _, ok := cm.challenges[cid]; ok {
				remaining = append(remaining, cid)
			}
		}
		if len(remaining) == 0 {
			delete(cm.challengesByWork, workID)
		} else {
			cm.challengesByWork[workID] = remaining
		}
	}

	return removed
}
