// Package eventsourcing provides domain types for event sourcing infrastructure.
package eventsourcing

import "errors"

// Domain errors for event sourcing.
var (
	// ErrAggregateNotFound indicates the aggregate was not found.
	ErrAggregateNotFound = errors.New("aggregate not found")

	// ErrConcurrencyConflict indicates a version conflict.
	ErrConcurrencyConflict = errors.New("concurrency conflict: version mismatch")

	// ErrEventNotFound indicates an event was not found.
	ErrEventNotFound = errors.New("event not found")

	// ErrSnapshotNotFound indicates a snapshot was not found.
	ErrSnapshotNotFound = errors.New("snapshot not found")

	// ErrInvalidEvent indicates an invalid event.
	ErrInvalidEvent = errors.New("invalid event")

	// ErrInvalidEventType indicates an unknown event type.
	ErrInvalidEventType = errors.New("invalid event type")

	// ErrInvalidAggregateID indicates an invalid aggregate ID.
	ErrInvalidAggregateID = errors.New("invalid aggregate ID")

	// ErrInvalidVersion indicates an invalid version number.
	ErrInvalidVersion = errors.New("invalid version number")

	// ErrSerializationFailed indicates serialization failure.
	ErrSerializationFailed = errors.New("event serialization failed")

	// ErrDeserializationFailed indicates deserialization failure.
	ErrDeserializationFailed = errors.New("event deserialization failed")

	// ErrStoreClosed indicates the store has been closed.
	ErrStoreClosed = errors.New("event store is closed")

	// ErrStoreInitFailed indicates store initialization failed.
	ErrStoreInitFailed = errors.New("event store initialization failed")

	// ErrTransactionFailed indicates a transaction failure.
	ErrTransactionFailed = errors.New("transaction failed")

	// ErrReplayFailed indicates event replay failure.
	ErrReplayFailed = errors.New("event replay failed")

	// ErrProjectionFailed indicates projection processing failure.
	ErrProjectionFailed = errors.New("projection failed")

	// ErrSubscriptionFailed indicates subscription failure.
	ErrSubscriptionFailed = errors.New("subscription failed")

	// ErrSnapshotCreationFailed indicates snapshot creation failure.
	ErrSnapshotCreationFailed = errors.New("snapshot creation failed")
)
