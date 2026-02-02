// Package claims provides domain types for the claims system with event sourcing.
package claims

import (
	"time"

	"github.com/google/uuid"
)

// ClaimEventType represents the type of a claim domain event.
type ClaimEventType string

const (
	// Core claim events
	EventClaimCreated       ClaimEventType = "claim:created"
	EventClaimReleased      ClaimEventType = "claim:released"
	EventClaimExpired       ClaimEventType = "claim:expired"
	EventClaimCompleted     ClaimEventType = "claim:completed"
	EventClaimStatusChanged ClaimEventType = "claim:status-changed"
	EventClaimNoteAdded     ClaimEventType = "claim:note-added"
	EventClaimProgressUpdated ClaimEventType = "claim:progress-updated"

	// Handoff events
	EventHandoffRequested ClaimEventType = "handoff:requested"
	EventHandoffAccepted  ClaimEventType = "handoff:accepted"
	EventHandoffRejected  ClaimEventType = "handoff:rejected"

	// Work stealing events
	EventIssueMarkedStealable ClaimEventType = "steal:issue-marked-stealable"
	EventIssueStolen          ClaimEventType = "steal:issue-stolen"
	EventStealContestStarted  ClaimEventType = "steal:contest-started"
	EventStealContestResolved ClaimEventType = "steal:contest-resolved"

	// Load balancing events
	EventSwarmRebalanced  ClaimEventType = "swarm:rebalanced"
	EventAgentOverloaded  ClaimEventType = "agent:overloaded"
	EventAgentUnderloaded ClaimEventType = "agent:underloaded"
)

// ClaimEvent represents a domain event in the claims system.
type ClaimEvent struct {
	ID            string                 `json:"id"`
	Type          ClaimEventType         `json:"type"`
	AggregateID   string                 `json:"aggregateId"`
	AggregateType string                 `json:"aggregateType"`
	Version       int                    `json:"version"`
	Timestamp     time.Time              `json:"timestamp"`
	Payload       map[string]interface{} `json:"payload"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CorrelationID string                 `json:"correlationId,omitempty"`
	CausationID   string                 `json:"causationId,omitempty"`
	Source        string                 `json:"source,omitempty"`
}

// NewClaimEvent creates a new claim event.
func NewClaimEvent(eventType ClaimEventType, aggregateID string, version int, payload map[string]interface{}) *ClaimEvent {
	return &ClaimEvent{
		ID:            uuid.New().String(),
		Type:          eventType,
		AggregateID:   aggregateID,
		AggregateType: "claim",
		Version:       version,
		Timestamp:     time.Now(),
		Payload:       payload,
		Metadata:      make(map[string]interface{}),
	}
}

// WithCorrelation sets the correlation ID.
func (e *ClaimEvent) WithCorrelation(correlationID string) *ClaimEvent {
	e.CorrelationID = correlationID
	return e
}

// WithCausation sets the causation ID.
func (e *ClaimEvent) WithCausation(causationID string) *ClaimEvent {
	e.CausationID = causationID
	return e
}

// WithSource sets the source.
func (e *ClaimEvent) WithSource(source string) *ClaimEvent {
	e.Source = source
	return e
}

// WithMetadata adds metadata.
func (e *ClaimEvent) WithMetadata(key string, value interface{}) *ClaimEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// Event factory functions

// NewClaimCreatedEvent creates a claim created event.
func NewClaimCreatedEvent(claim *Claim) *ClaimEvent {
	return NewClaimEvent(EventClaimCreated, claim.ID, claim.Version, map[string]interface{}{
		"claimId":    claim.ID,
		"issueId":    claim.IssueID,
		"claimantId": claim.Claimant.ID,
		"claimant":   claim.Claimant,
		"status":     claim.Status,
		"claimedAt":  claim.ClaimedAt,
	})
}

// NewClaimReleasedEvent creates a claim released event.
func NewClaimReleasedEvent(claimID string, version int, reason string) *ClaimEvent {
	return NewClaimEvent(EventClaimReleased, claimID, version, map[string]interface{}{
		"claimId": claimID,
		"reason":  reason,
	})
}

// NewClaimExpiredEvent creates a claim expired event.
func NewClaimExpiredEvent(claimID string, version int) *ClaimEvent {
	return NewClaimEvent(EventClaimExpired, claimID, version, map[string]interface{}{
		"claimId": claimID,
	})
}

// NewClaimCompletedEvent creates a claim completed event.
func NewClaimCompletedEvent(claimID string, version int) *ClaimEvent {
	return NewClaimEvent(EventClaimCompleted, claimID, version, map[string]interface{}{
		"claimId": claimID,
	})
}

// NewClaimStatusChangedEvent creates a status changed event.
func NewClaimStatusChangedEvent(claimID string, version int, oldStatus, newStatus ClaimStatus, note string) *ClaimEvent {
	return NewClaimEvent(EventClaimStatusChanged, claimID, version, map[string]interface{}{
		"claimId":   claimID,
		"oldStatus": oldStatus,
		"newStatus": newStatus,
		"note":      note,
	})
}

// NewClaimProgressUpdatedEvent creates a progress updated event.
func NewClaimProgressUpdatedEvent(claimID string, version int, progress float64) *ClaimEvent {
	return NewClaimEvent(EventClaimProgressUpdated, claimID, version, map[string]interface{}{
		"claimId":  claimID,
		"progress": progress,
	})
}

// NewHandoffRequestedEvent creates a handoff requested event.
func NewHandoffRequestedEvent(claimID string, version int, handoff *HandoffRecord) *ClaimEvent {
	return NewClaimEvent(EventHandoffRequested, claimID, version, map[string]interface{}{
		"claimId":    claimID,
		"handoffId":  handoff.ID,
		"fromId":     handoff.From.ID,
		"toId":       handoff.To.ID,
		"reason":     handoff.Reason,
		"requestedAt": handoff.RequestedAt,
	})
}

// NewHandoffAcceptedEvent creates a handoff accepted event.
func NewHandoffAcceptedEvent(claimID string, version int, handoffID string) *ClaimEvent {
	return NewClaimEvent(EventHandoffAccepted, claimID, version, map[string]interface{}{
		"claimId":   claimID,
		"handoffId": handoffID,
	})
}

// NewHandoffRejectedEvent creates a handoff rejected event.
func NewHandoffRejectedEvent(claimID string, version int, handoffID, reason string) *ClaimEvent {
	return NewClaimEvent(EventHandoffRejected, claimID, version, map[string]interface{}{
		"claimId":   claimID,
		"handoffId": handoffID,
		"reason":    reason,
	})
}

// NewIssueMarkedStealableEvent creates an issue marked stealable event.
func NewIssueMarkedStealableEvent(claimID string, version int, reason string) *ClaimEvent {
	return NewClaimEvent(EventIssueMarkedStealable, claimID, version, map[string]interface{}{
		"claimId": claimID,
		"reason":  reason,
	})
}

// NewIssueStolenEvent creates an issue stolen event.
func NewIssueStolenEvent(claimID string, version int, newClaimantID, originalClaimantID string) *ClaimEvent {
	return NewClaimEvent(EventIssueStolen, claimID, version, map[string]interface{}{
		"claimId":           claimID,
		"newClaimantId":     newClaimantID,
		"originalClaimantId": originalClaimantID,
	})
}

// NewSwarmRebalancedEvent creates a swarm rebalanced event.
func NewSwarmRebalancedEvent(swarmID string, moves []RebalanceMove) *ClaimEvent {
	return NewClaimEvent(EventSwarmRebalanced, swarmID, 1, map[string]interface{}{
		"swarmId":    swarmID,
		"moveCount":  len(moves),
		"moves":      moves,
	})
}

// RebalanceMove represents a claim move during rebalancing.
type RebalanceMove struct {
	ClaimID     string `json:"claimId"`
	FromAgentID string `json:"fromAgentId"`
	ToAgentID   string `json:"toAgentId"`
}

// EventHandler is a function that handles claim events.
type EventHandler func(event *ClaimEvent)

// EventFilter defines criteria for filtering events.
type EventFilter struct {
	AggregateID   string
	EventTypes    []ClaimEventType
	FromTimestamp *time.Time
	ToTimestamp   *time.Time
	FromVersion   int
	ToVersion     int
}

// Matches returns true if the event matches the filter.
func (f EventFilter) Matches(event *ClaimEvent) bool {
	if f.AggregateID != "" && event.AggregateID != f.AggregateID {
		return false
	}

	if len(f.EventTypes) > 0 {
		found := false
		for _, t := range f.EventTypes {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if f.FromTimestamp != nil && event.Timestamp.Before(*f.FromTimestamp) {
		return false
	}

	if f.ToTimestamp != nil && event.Timestamp.After(*f.ToTimestamp) {
		return false
	}

	if f.FromVersion > 0 && event.Version < f.FromVersion {
		return false
	}

	if f.ToVersion > 0 && event.Version > f.ToVersion {
		return false
	}

	return true
}
