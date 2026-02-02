// Package claims provides domain types for the claims system with event sourcing.
package claims

import (
	"time"
)

// HandoffRecord represents a handoff request between claimants.
type HandoffRecord struct {
	ID              string        `json:"id"`
	From            Claimant      `json:"from"`
	To              Claimant      `json:"to"`
	Reason          string        `json:"reason"`
	Status          HandoffStatus `json:"status"`
	RequestedAt     time.Time     `json:"requestedAt"`
	ResolvedAt      *time.Time    `json:"resolvedAt,omitempty"`
	RejectionReason string        `json:"rejectionReason,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// NewHandoffRecord creates a new handoff record.
func NewHandoffRecord(id string, from, to Claimant, reason string) *HandoffRecord {
	return &HandoffRecord{
		ID:          id,
		From:        from,
		To:          to,
		Reason:      reason,
		Status:      HandoffStatusPending,
		RequestedAt: time.Now(),
		Metadata:    make(map[string]interface{}),
	}
}

// Accept accepts the handoff.
func (h *HandoffRecord) Accept() {
	now := time.Now()
	h.Status = HandoffStatusAccepted
	h.ResolvedAt = &now
}

// Reject rejects the handoff with a reason.
func (h *HandoffRecord) Reject(reason string) {
	now := time.Now()
	h.Status = HandoffStatusRejected
	h.ResolvedAt = &now
	h.RejectionReason = reason
}

// IsPending returns true if the handoff is pending.
func (h *HandoffRecord) IsPending() bool {
	return h.Status == HandoffStatusPending
}

// IsAccepted returns true if the handoff was accepted.
func (h *HandoffRecord) IsAccepted() bool {
	return h.Status == HandoffStatusAccepted
}

// IsRejected returns true if the handoff was rejected.
func (h *HandoffRecord) IsRejected() bool {
	return h.Status == HandoffStatusRejected
}

// IsResolved returns true if the handoff has been resolved.
func (h *HandoffRecord) IsResolved() bool {
	return h.Status != HandoffStatusPending
}

// Duration returns the duration from request to resolution.
func (h *HandoffRecord) Duration() time.Duration {
	if h.ResolvedAt == nil {
		return time.Since(h.RequestedAt)
	}
	return h.ResolvedAt.Sub(h.RequestedAt)
}
