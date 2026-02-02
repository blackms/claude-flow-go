// Package eventsourcing provides domain types for event sourcing infrastructure.
package eventsourcing

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventCategory represents the category of an event.
type EventCategory string

const (
	// CategoryDomain represents domain events (business logic).
	CategoryDomain EventCategory = "domain"
	// CategoryIntegration represents integration events (cross-boundary).
	CategoryIntegration EventCategory = "integration"
	// CategorySystem represents system events (infrastructure).
	CategorySystem EventCategory = "system"
)

// Event is the interface that all events must implement.
type Event interface {
	// ID returns the unique event identifier.
	ID() string
	// Type returns the event type (e.g., "order:created").
	Type() string
	// AggregateID returns the aggregate root identifier.
	AggregateID() string
	// AggregateType returns the aggregate type (e.g., "order").
	AggregateType() string
	// Version returns the aggregate version after this event.
	Version() int
	// Timestamp returns when the event occurred.
	Timestamp() time.Time
	// Category returns the event category.
	Category() EventCategory
	// Payload returns the event data.
	Payload() map[string]interface{}
	// Metadata returns event metadata.
	Metadata() *EventMetadata
}

// EventMetadata contains metadata associated with an event.
type EventMetadata struct {
	// CorrelationID links related events across boundaries.
	CorrelationID string `json:"correlationId,omitempty"`
	// CausationID is the ID of the event that caused this event.
	CausationID string `json:"causationId,omitempty"`
	// Actor is the identity that caused the event.
	Actor string `json:"actor,omitempty"`
	// Source is the system/service that produced the event.
	Source string `json:"source,omitempty"`
	// TraceID for distributed tracing.
	TraceID string `json:"traceId,omitempty"`
	// SpanID for distributed tracing.
	SpanID string `json:"spanId,omitempty"`
	// Custom contains additional custom metadata.
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// StoredEvent represents an event as stored in the event store.
type StoredEvent struct {
	// ID is the unique event identifier.
	ID string `json:"id"`
	// Type is the event type.
	Type string `json:"type"`
	// AggregateID is the aggregate root identifier.
	AggregateID string `json:"aggregateId"`
	// AggregateType is the type of aggregate.
	AggregateType string `json:"aggregateType"`
	// Version is the aggregate version after this event.
	Version int `json:"version"`
	// Category is the event category.
	Category EventCategory `json:"category"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// Payload is the serialized event data.
	Payload []byte `json:"payload"`
	// Metadata is the event metadata.
	Metadata *EventMetadata `json:"metadata,omitempty"`
}

// GetPayload deserializes the payload into a map.
func (e *StoredEvent) GetPayload() (map[string]interface{}, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(e.Payload, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// GetPayloadAs deserializes the payload into a specific type.
func (e *StoredEvent) GetPayloadAs(v interface{}) error {
	return json.Unmarshal(e.Payload, v)
}

// BaseEvent provides a base implementation of the Event interface.
type BaseEvent struct {
	id            string
	eventType     string
	aggregateID   string
	aggregateType string
	version       int
	timestamp     time.Time
	category      EventCategory
	payload       map[string]interface{}
	metadata      *EventMetadata
}

// NewBaseEvent creates a new base event.
func NewBaseEvent(
	eventType string,
	aggregateID string,
	aggregateType string,
	version int,
	payload map[string]interface{},
) *BaseEvent {
	return &BaseEvent{
		id:            uuid.New().String(),
		eventType:     eventType,
		aggregateID:   aggregateID,
		aggregateType: aggregateType,
		version:       version,
		timestamp:     time.Now(),
		category:      CategoryDomain,
		payload:       payload,
		metadata:      &EventMetadata{},
	}
}

// ID returns the event ID.
func (e *BaseEvent) ID() string { return e.id }

// Type returns the event type.
func (e *BaseEvent) Type() string { return e.eventType }

// AggregateID returns the aggregate ID.
func (e *BaseEvent) AggregateID() string { return e.aggregateID }

// AggregateType returns the aggregate type.
func (e *BaseEvent) AggregateType() string { return e.aggregateType }

// Version returns the version.
func (e *BaseEvent) Version() int { return e.version }

// Timestamp returns the timestamp.
func (e *BaseEvent) Timestamp() time.Time { return e.timestamp }

// Category returns the category.
func (e *BaseEvent) Category() EventCategory { return e.category }

// Payload returns the payload.
func (e *BaseEvent) Payload() map[string]interface{} { return e.payload }

// Metadata returns the metadata.
func (e *BaseEvent) Metadata() *EventMetadata { return e.metadata }

// WithCategory sets the category.
func (e *BaseEvent) WithCategory(category EventCategory) *BaseEvent {
	e.category = category
	return e
}

// WithCorrelation sets the correlation ID.
func (e *BaseEvent) WithCorrelation(correlationID string) *BaseEvent {
	e.metadata.CorrelationID = correlationID
	return e
}

// WithCausation sets the causation ID.
func (e *BaseEvent) WithCausation(causationID string) *BaseEvent {
	e.metadata.CausationID = causationID
	return e
}

// WithActor sets the actor.
func (e *BaseEvent) WithActor(actor string) *BaseEvent {
	e.metadata.Actor = actor
	return e
}

// WithSource sets the source.
func (e *BaseEvent) WithSource(source string) *BaseEvent {
	e.metadata.Source = source
	return e
}

// WithTrace sets tracing information.
func (e *BaseEvent) WithTrace(traceID, spanID string) *BaseEvent {
	e.metadata.TraceID = traceID
	e.metadata.SpanID = spanID
	return e
}

// WithCustomMetadata adds custom metadata.
func (e *BaseEvent) WithCustomMetadata(key string, value interface{}) *BaseEvent {
	if e.metadata.Custom == nil {
		e.metadata.Custom = make(map[string]interface{})
	}
	e.metadata.Custom[key] = value
	return e
}

// ToStoredEvent converts to a stored event.
func (e *BaseEvent) ToStoredEvent() (*StoredEvent, error) {
	payload, err := json.Marshal(e.payload)
	if err != nil {
		return nil, err
	}

	return &StoredEvent{
		ID:            e.id,
		Type:          e.eventType,
		AggregateID:   e.aggregateID,
		AggregateType: e.aggregateType,
		Version:       e.version,
		Category:      e.category,
		Timestamp:     e.timestamp,
		Payload:       payload,
		Metadata:      e.metadata,
	}, nil
}

// EventFromStored reconstructs a BaseEvent from a StoredEvent.
func EventFromStored(stored *StoredEvent) (*BaseEvent, error) {
	payload, err := stored.GetPayload()
	if err != nil {
		return nil, err
	}

	return &BaseEvent{
		id:            stored.ID,
		eventType:     stored.Type,
		aggregateID:   stored.AggregateID,
		aggregateType: stored.AggregateType,
		version:       stored.Version,
		timestamp:     stored.Timestamp,
		category:      stored.Category,
		payload:       payload,
		metadata:      stored.Metadata,
	}, nil
}

// EventQuery defines criteria for querying events.
type EventQuery struct {
	// AggregateID filters by aggregate ID.
	AggregateID string
	// AggregateType filters by aggregate type.
	AggregateType string
	// EventTypes filters by event types.
	EventTypes []string
	// Categories filters by event categories.
	Categories []EventCategory
	// FromVersion filters events >= this version.
	FromVersion int
	// ToVersion filters events <= this version.
	ToVersion int
	// FromTimestamp filters events >= this timestamp.
	FromTimestamp *time.Time
	// ToTimestamp filters events <= this timestamp.
	ToTimestamp *time.Time
	// CorrelationID filters by correlation ID.
	CorrelationID string
	// Actor filters by actor.
	Actor string
	// Limit limits the number of results.
	Limit int
	// Offset skips the first N results.
	Offset int
}

// Matches returns true if the event matches the query.
func (q *EventQuery) Matches(event *StoredEvent) bool {
	if q.AggregateID != "" && event.AggregateID != q.AggregateID {
		return false
	}

	if q.AggregateType != "" && event.AggregateType != q.AggregateType {
		return false
	}

	if len(q.EventTypes) > 0 {
		found := false
		for _, t := range q.EventTypes {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(q.Categories) > 0 {
		found := false
		for _, c := range q.Categories {
			if event.Category == c {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if q.FromVersion > 0 && event.Version < q.FromVersion {
		return false
	}

	if q.ToVersion > 0 && event.Version > q.ToVersion {
		return false
	}

	if q.FromTimestamp != nil && event.Timestamp.Before(*q.FromTimestamp) {
		return false
	}

	if q.ToTimestamp != nil && event.Timestamp.After(*q.ToTimestamp) {
		return false
	}

	if q.CorrelationID != "" && (event.Metadata == nil || event.Metadata.CorrelationID != q.CorrelationID) {
		return false
	}

	if q.Actor != "" && (event.Metadata == nil || event.Metadata.Actor != q.Actor) {
		return false
	}

	return true
}

// EventHandler is a function that handles events.
type EventHandler func(event *StoredEvent) error

// Subscription represents an event subscription.
type Subscription interface {
	// ID returns the subscription identifier.
	ID() string
	// Unsubscribe cancels the subscription.
	Unsubscribe()
}
