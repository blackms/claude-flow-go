// Package claims provides domain types for the claims system with event sourcing.
package claims

import (
	"fmt"
	"time"
)

// Claim represents a claim on an issue.
type Claim struct {
	ID             string          `json:"id"`
	IssueID        string          `json:"issueId"`
	Claimant       Claimant        `json:"claimant"`
	Status         ClaimStatus     `json:"status"`
	ClaimedAt      time.Time       `json:"claimedAt"`
	LastActivityAt time.Time       `json:"lastActivityAt"`
	ExpiresAt      *time.Time      `json:"expiresAt,omitempty"`
	Progress       float64         `json:"progress"` // 0.0 to 1.0
	Notes          []string        `json:"notes,omitempty"`
	HandoffChain   []HandoffRecord `json:"handoffChain,omitempty"`
	Version        int             `json:"version"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// NewClaim creates a new claim.
func NewClaim(id, issueID string, claimant Claimant) *Claim {
	now := time.Now()
	return &Claim{
		ID:             id,
		IssueID:        issueID,
		Claimant:       claimant,
		Status:         StatusActive,
		ClaimedAt:      now,
		LastActivityAt: now,
		Progress:       0.0,
		Notes:          make([]string, 0),
		HandoffChain:   make([]HandoffRecord, 0),
		Version:        1,
		Metadata:       make(map[string]interface{}),
	}
}

// UpdateStatus updates the claim status.
func (c *Claim) UpdateStatus(status ClaimStatus) error {
	if c.Status.IsTerminal() {
		return fmt.Errorf("cannot change status of terminal claim")
	}
	c.Status = status
	c.LastActivityAt = time.Now()
	c.Version++
	return nil
}

// UpdateProgress updates the progress.
func (c *Claim) UpdateProgress(progress float64) {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	c.Progress = progress
	c.LastActivityAt = time.Now()
	c.Version++
}

// AddNote adds a note to the claim.
func (c *Claim) AddNote(note string) {
	c.Notes = append(c.Notes, note)
	c.LastActivityAt = time.Now()
	c.Version++
}

// AddHandoff adds a handoff record.
func (c *Claim) AddHandoff(handoff HandoffRecord) {
	c.HandoffChain = append(c.HandoffChain, handoff)
	c.LastActivityAt = time.Now()
	c.Version++
}

// GetPendingHandoff returns the pending handoff if any.
func (c *Claim) GetPendingHandoff() *HandoffRecord {
	for i := len(c.HandoffChain) - 1; i >= 0; i-- {
		if c.HandoffChain[i].IsPending() {
			return &c.HandoffChain[i]
		}
	}
	return nil
}

// HasPendingHandoff returns true if there's a pending handoff.
func (c *Claim) HasPendingHandoff() bool {
	return c.GetPendingHandoff() != nil
}

// AcceptHandoff accepts the pending handoff.
func (c *Claim) AcceptHandoff() error {
	handoff := c.GetPendingHandoff()
	if handoff == nil {
		return fmt.Errorf("no pending handoff")
	}

	handoff.Accept()
	c.Claimant = handoff.To
	c.Status = StatusActive
	c.LastActivityAt = time.Now()
	c.Version++
	return nil
}

// RejectHandoff rejects the pending handoff.
func (c *Claim) RejectHandoff(reason string) error {
	handoff := c.GetPendingHandoff()
	if handoff == nil {
		return fmt.Errorf("no pending handoff")
	}

	handoff.Reject(reason)
	c.Status = StatusActive
	c.LastActivityAt = time.Now()
	c.Version++
	return nil
}

// Release releases the claim.
func (c *Claim) Release() {
	c.Status = StatusReleased
	c.LastActivityAt = time.Now()
	c.Version++
}

// Complete completes the claim.
func (c *Claim) Complete() {
	c.Status = StatusCompleted
	c.Progress = 1.0
	c.LastActivityAt = time.Now()
	c.Version++
}

// Expire expires the claim.
func (c *Claim) Expire() {
	c.Status = StatusExpired
	c.LastActivityAt = time.Now()
	c.Version++
}

// MarkStealable marks the claim as stealable.
func (c *Claim) MarkStealable() {
	c.Status = StatusStealable
	c.LastActivityAt = time.Now()
	c.Version++
}

// IsActive returns true if the claim is active.
func (c *Claim) IsActive() bool {
	return c.Status.IsActive()
}

// IsTerminal returns true if the claim is in a terminal state.
func (c *Claim) IsTerminal() bool {
	return c.Status.IsTerminal()
}

// IsStealable returns true if the claim can be stolen.
func (c *Claim) IsStealable() bool {
	return c.Status == StatusStealable
}

// IsStale returns true if the claim is stale (no activity for given duration).
func (c *Claim) IsStale(threshold time.Duration) bool {
	return time.Since(c.LastActivityAt) > threshold
}

// IsExpired returns true if the claim has expired.
func (c *Claim) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*c.ExpiresAt)
}

// Duration returns the duration since the claim was created.
func (c *Claim) Duration() time.Duration {
	return time.Since(c.ClaimedAt)
}

// InactivityDuration returns the duration since last activity.
func (c *Claim) InactivityDuration() time.Duration {
	return time.Since(c.LastActivityAt)
}

// IsOwnedBy returns true if the claim is owned by the given claimant.
func (c *Claim) IsOwnedBy(claimantID string) bool {
	return c.Claimant.ID == claimantID
}
