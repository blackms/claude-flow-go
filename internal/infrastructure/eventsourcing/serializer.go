// Package eventsourcing provides infrastructure for event sourcing.
package eventsourcing

import (
	"encoding/json"
	"fmt"
	"time"

	domain "github.com/anthropics/claude-flow-go/internal/domain/eventsourcing"
)

// EventSerializer handles event serialization and deserialization.
type EventSerializer interface {
	// Serialize serializes an event to StoredEvent.
	Serialize(event domain.Event) (*domain.StoredEvent, error)

	// Deserialize deserializes a StoredEvent to Event.
	Deserialize(stored *domain.StoredEvent) (domain.Event, error)

	// SerializePayload serializes a payload to bytes.
	SerializePayload(payload map[string]interface{}) ([]byte, error)

	// DeserializePayload deserializes bytes to payload.
	DeserializePayload(data []byte) (map[string]interface{}, error)

	// SerializeState serializes aggregate state for snapshots.
	SerializeState(state interface{}) ([]byte, error)

	// DeserializeState deserializes snapshot state.
	DeserializeState(data []byte, target interface{}) error
}

// JSONEventSerializer implements EventSerializer using JSON.
type JSONEventSerializer struct{}

// NewJSONEventSerializer creates a new JSON event serializer.
func NewJSONEventSerializer() *JSONEventSerializer {
	return &JSONEventSerializer{}
}

// Serialize serializes an event to StoredEvent.
func (s *JSONEventSerializer) Serialize(event domain.Event) (*domain.StoredEvent, error) {
	payload, err := s.SerializePayload(event.Payload())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrSerializationFailed, err)
	}

	return &domain.StoredEvent{
		ID:            event.ID(),
		Type:          event.Type(),
		AggregateID:   event.AggregateID(),
		AggregateType: event.AggregateType(),
		Version:       event.Version(),
		Category:      event.Category(),
		Timestamp:     event.Timestamp(),
		Payload:       payload,
		Metadata:      event.Metadata(),
	}, nil
}

// Deserialize deserializes a StoredEvent to Event.
func (s *JSONEventSerializer) Deserialize(stored *domain.StoredEvent) (domain.Event, error) {
	return domain.EventFromStored(stored)
}

// SerializePayload serializes a payload to bytes.
func (s *JSONEventSerializer) SerializePayload(payload map[string]interface{}) ([]byte, error) {
	if payload == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(payload)
}

// DeserializePayload deserializes bytes to payload.
func (s *JSONEventSerializer) DeserializePayload(data []byte) (map[string]interface{}, error) {
	if len(data) == 0 {
		return make(map[string]interface{}), nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrDeserializationFailed, err)
	}
	return payload, nil
}

// SerializeState serializes aggregate state for snapshots.
func (s *JSONEventSerializer) SerializeState(state interface{}) ([]byte, error) {
	return json.Marshal(state)
}

// DeserializeState deserializes snapshot state.
func (s *JSONEventSerializer) DeserializeState(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}

// EventEnvelope wraps an event with transport metadata.
type EventEnvelope struct {
	// Event is the wrapped event.
	Event *domain.StoredEvent `json:"event"`

	// Schema is the schema version.
	Schema string `json:"schema,omitempty"`

	// Encoding is the payload encoding (e.g., "json", "protobuf").
	Encoding string `json:"encoding,omitempty"`

	// Compressed indicates if the payload is compressed.
	Compressed bool `json:"compressed,omitempty"`

	// SentAt is when the envelope was created.
	SentAt time.Time `json:"sentAt"`
}

// NewEventEnvelope creates a new event envelope.
func NewEventEnvelope(event *domain.StoredEvent) *EventEnvelope {
	return &EventEnvelope{
		Event:    event,
		Schema:   "1.0",
		Encoding: "json",
		SentAt:   time.Now(),
	}
}

// EventBatch represents a batch of events for bulk operations.
type EventBatch struct {
	// Events are the events in the batch.
	Events []*domain.StoredEvent `json:"events"`

	// AggregateID is the aggregate ID (if all events belong to same aggregate).
	AggregateID string `json:"aggregateId,omitempty"`

	// FromVersion is the starting version.
	FromVersion int `json:"fromVersion,omitempty"`

	// ToVersion is the ending version.
	ToVersion int `json:"toVersion,omitempty"`

	// CreatedAt is when the batch was created.
	CreatedAt time.Time `json:"createdAt"`
}

// NewEventBatch creates a new event batch.
func NewEventBatch(events []*domain.StoredEvent) *EventBatch {
	batch := &EventBatch{
		Events:    events,
		CreatedAt: time.Now(),
	}

	if len(events) > 0 {
		batch.AggregateID = events[0].AggregateID
		batch.FromVersion = events[0].Version
		batch.ToVersion = events[len(events)-1].Version

		// Verify all events are for same aggregate
		for _, e := range events {
			if e.AggregateID != batch.AggregateID {
				batch.AggregateID = "" // Mixed aggregates
				break
			}
		}
	}

	return batch
}

// Size returns the number of events in the batch.
func (b *EventBatch) Size() int {
	return len(b.Events)
}

// IsEmpty returns true if the batch is empty.
func (b *EventBatch) IsEmpty() bool {
	return len(b.Events) == 0
}

// EventTypeRegistry manages event type mappings.
type EventTypeRegistry struct {
	types map[string]func() interface{}
}

// NewEventTypeRegistry creates a new event type registry.
func NewEventTypeRegistry() *EventTypeRegistry {
	return &EventTypeRegistry{
		types: make(map[string]func() interface{}),
	}
}

// Register registers an event type.
func (r *EventTypeRegistry) Register(eventType string, factory func() interface{}) {
	r.types[eventType] = factory
}

// Create creates an instance of a registered event type.
func (r *EventTypeRegistry) Create(eventType string) (interface{}, bool) {
	factory, exists := r.types[eventType]
	if !exists {
		return nil, false
	}
	return factory(), true
}

// IsRegistered returns true if an event type is registered.
func (r *EventTypeRegistry) IsRegistered(eventType string) bool {
	_, exists := r.types[eventType]
	return exists
}

// Types returns all registered event types.
func (r *EventTypeRegistry) Types() []string {
	types := make([]string, 0, len(r.types))
	for t := range r.types {
		types = append(types, t)
	}
	return types
}
