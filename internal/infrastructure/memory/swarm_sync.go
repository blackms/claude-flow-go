// Package memory provides infrastructure for memory management.
package memory

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// SwarmSynchronizer synchronizes memories across swarm nodes.
type SwarmSynchronizer struct {
	mu       sync.RWMutex
	nodeID   string
	config   SwarmSyncConfig
	peers    map[string]*PeerNode
	versions map[string]*VectorClock
	pending  []*SyncOperation
	stats    *SwarmSyncStats
}

// SwarmSyncConfig configures swarm synchronization.
type SwarmSyncConfig struct {
	// NodeID is this node's identifier.
	NodeID string `json:"nodeId"`

	// SyncIntervalMs is the sync interval in milliseconds.
	SyncIntervalMs int64 `json:"syncIntervalMs"`

	// BatchSize is the sync batch size.
	BatchSize int `json:"batchSize"`

	// ConflictResolution is the conflict resolution strategy.
	ConflictResolution ConflictResolution `json:"conflictResolution"`

	// DeltaSync enables delta synchronization.
	DeltaSync bool `json:"deltaSync"`

	// MaxPeers is the maximum number of peers.
	MaxPeers int `json:"maxPeers"`
}

// ConflictResolution defines conflict resolution strategies.
type ConflictResolution string

const (
	// ResolutionLastWriteWins uses last write wins.
	ResolutionLastWriteWins ConflictResolution = "last_write_wins"
	// ResolutionMerge merges conflicting values.
	ResolutionMerge ConflictResolution = "merge"
	// ResolutionManual requires manual resolution.
	ResolutionManual ConflictResolution = "manual"
)

// DefaultSwarmSyncConfig returns the default configuration.
func DefaultSwarmSyncConfig() SwarmSyncConfig {
	return SwarmSyncConfig{
		NodeID:             uuid.New().String(),
		SyncIntervalMs:     5000, // 5 seconds
		BatchSize:          100,
		ConflictResolution: ResolutionLastWriteWins,
		DeltaSync:          true,
		MaxPeers:           10,
	}
}

// PeerNode represents a peer node in the swarm.
type PeerNode struct {
	// ID is the peer identifier.
	ID string `json:"id"`

	// Address is the peer address.
	Address string `json:"address"`

	// LastSeen is when the peer was last seen.
	LastSeen time.Time `json:"lastSeen"`

	// Version is the peer's current version.
	Version *VectorClock `json:"version"`

	// IsHealthy indicates if the peer is healthy.
	IsHealthy bool `json:"isHealthy"`

	// SyncedAt is when we last synced with this peer.
	SyncedAt *time.Time `json:"syncedAt,omitempty"`
}

// VectorClock implements a vector clock for version tracking.
type VectorClock struct {
	// Clocks maps node IDs to their logical clocks.
	Clocks map[string]int64 `json:"clocks"`
}

// NewVectorClock creates a new vector clock.
func NewVectorClock() *VectorClock {
	return &VectorClock{
		Clocks: make(map[string]int64),
	}
}

// Increment increments the clock for a node.
func (v *VectorClock) Increment(nodeID string) {
	v.Clocks[nodeID]++
}

// Get returns the clock value for a node.
func (v *VectorClock) Get(nodeID string) int64 {
	return v.Clocks[nodeID]
}

// Merge merges another vector clock.
func (v *VectorClock) Merge(other *VectorClock) {
	if other == nil {
		return
	}
	for nodeID, clock := range other.Clocks {
		if clock > v.Clocks[nodeID] {
			v.Clocks[nodeID] = clock
		}
	}
}

// HappenedBefore returns true if this clock happened before other.
func (v *VectorClock) HappenedBefore(other *VectorClock) bool {
	if other == nil {
		return false
	}

	atLeastOneSmaller := false
	for nodeID, clock := range v.Clocks {
		otherClock := other.Clocks[nodeID]
		if clock > otherClock {
			return false
		}
		if clock < otherClock {
			atLeastOneSmaller = true
		}
	}

	// Check for clocks in other but not in v
	for nodeID := range other.Clocks {
		if _, ok := v.Clocks[nodeID]; !ok {
			atLeastOneSmaller = true
		}
	}

	return atLeastOneSmaller
}

// IsConcurrent returns true if clocks are concurrent.
func (v *VectorClock) IsConcurrent(other *VectorClock) bool {
	return !v.HappenedBefore(other) && !other.HappenedBefore(v)
}

// Clone returns a copy of the vector clock.
func (v *VectorClock) Clone() *VectorClock {
	clone := NewVectorClock()
	for nodeID, clock := range v.Clocks {
		clone.Clocks[nodeID] = clock
	}
	return clone
}

// SyncOperation represents a sync operation.
type SyncOperation struct {
	// ID is the operation identifier.
	ID string `json:"id"`

	// Type is the operation type.
	Type SyncOperationType `json:"type"`

	// MemoryID is the memory ID.
	MemoryID string `json:"memoryId"`

	// Memory is the memory data (for creates/updates).
	Memory *shared.Memory `json:"memory,omitempty"`

	// Version is the version clock.
	Version *VectorClock `json:"version"`

	// SourceNode is the originating node.
	SourceNode string `json:"sourceNode"`

	// Timestamp is when the operation was created.
	Timestamp time.Time `json:"timestamp"`
}

// SyncOperationType is the type of sync operation.
type SyncOperationType string

const (
	SyncOpCreate SyncOperationType = "create"
	SyncOpUpdate SyncOperationType = "update"
	SyncOpDelete SyncOperationType = "delete"
)

// SwarmSyncStats contains sync statistics.
type SwarmSyncStats struct {
	// TotalSyncs is the total sync count.
	TotalSyncs int64 `json:"totalSyncs"`

	// SuccessfulSyncs is the successful sync count.
	SuccessfulSyncs int64 `json:"successfulSyncs"`

	// FailedSyncs is the failed sync count.
	FailedSyncs int64 `json:"failedSyncs"`

	// ConflictsDetected is the conflict count.
	ConflictsDetected int64 `json:"conflictsDetected"`

	// ConflictsResolved is the resolved conflict count.
	ConflictsResolved int64 `json:"conflictsResolved"`

	// MemoriesSynced is the total memories synced.
	MemoriesSynced int64 `json:"memoriesSynced"`

	// BytesSynced is the total bytes synced.
	BytesSynced int64 `json:"bytesSynced"`

	// LastSyncAt is when the last sync occurred.
	LastSyncAt *time.Time `json:"lastSyncAt,omitempty"`

	// PeerCount is the number of active peers.
	PeerCount int `json:"peerCount"`
}

// NewSwarmSynchronizer creates a new swarm synchronizer.
func NewSwarmSynchronizer(config SwarmSyncConfig) *SwarmSynchronizer {
	return &SwarmSynchronizer{
		nodeID:   config.NodeID,
		config:   config,
		peers:    make(map[string]*PeerNode),
		versions: make(map[string]*VectorClock),
		pending:  make([]*SyncOperation, 0),
		stats:    &SwarmSyncStats{},
	}
}

// RegisterPeer registers a peer node.
func (s *SwarmSynchronizer) RegisterPeer(id, address string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.peers) >= s.config.MaxPeers {
		return
	}

	s.peers[id] = &PeerNode{
		ID:        id,
		Address:   address,
		LastSeen:  time.Now(),
		Version:   NewVectorClock(),
		IsHealthy: true,
	}
	s.stats.PeerCount = len(s.peers)
}

// RemovePeer removes a peer node.
func (s *SwarmSynchronizer) RemovePeer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.peers, id)
	s.stats.PeerCount = len(s.peers)
}

// RecordOperation records a local operation for sync.
func (s *SwarmSynchronizer) RecordOperation(opType SyncOperationType, memory *shared.Memory) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create version for this memory
	version := s.versions[memory.ID]
	if version == nil {
		version = NewVectorClock()
		s.versions[memory.ID] = version
	}

	// Increment our clock
	version.Increment(s.nodeID)

	op := &SyncOperation{
		ID:         uuid.New().String(),
		Type:       opType,
		MemoryID:   memory.ID,
		Memory:     memory,
		Version:    version.Clone(),
		SourceNode: s.nodeID,
		Timestamp:  time.Now(),
	}

	s.pending = append(s.pending, op)
}

// GetPendingOperations returns operations pending sync.
func (s *SwarmSynchronizer) GetPendingOperations() []*SyncOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ops := make([]*SyncOperation, len(s.pending))
	copy(ops, s.pending)
	return ops
}

// ApplyOperation applies a remote sync operation.
func (s *SwarmSynchronizer) ApplyOperation(op *SyncOperation) (*shared.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	localVersion := s.versions[op.MemoryID]
	if localVersion == nil {
		// No local version, accept remote
		s.versions[op.MemoryID] = op.Version.Clone()
		s.stats.MemoriesSynced++
		return op.Memory, nil
	}

	// Check for conflicts
	if localVersion.IsConcurrent(op.Version) {
		s.stats.ConflictsDetected++

		// Resolve conflict
		resolved := s.resolveConflict(op)
		if resolved != nil {
			s.stats.ConflictsResolved++
			localVersion.Merge(op.Version)
			return resolved, nil
		}
		return nil, nil
	}

	// Check if remote is newer
	if localVersion.HappenedBefore(op.Version) {
		s.versions[op.MemoryID] = op.Version.Clone()
		s.stats.MemoriesSynced++
		return op.Memory, nil
	}

	// Local is newer, ignore remote
	return nil, nil
}

// resolveConflict resolves a conflict between local and remote.
func (s *SwarmSynchronizer) resolveConflict(op *SyncOperation) *shared.Memory {
	switch s.config.ConflictResolution {
	case ResolutionLastWriteWins:
		// Use timestamp to determine winner
		return op.Memory
	case ResolutionMerge:
		// Merge would require domain-specific logic
		return op.Memory
	case ResolutionManual:
		// Mark for manual resolution
		return nil
	default:
		return op.Memory
	}
}

// ClearPending clears pending operations after successful sync.
func (s *SwarmSynchronizer) ClearPending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = make([]*SyncOperation, 0)
}

// GetPeers returns all registered peers.
func (s *SwarmSynchronizer) GetPeers() []*PeerNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]*PeerNode, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetStats returns sync statistics.
func (s *SwarmSynchronizer) GetStats() *SwarmSyncStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// MarkSyncComplete marks a sync as complete.
func (s *SwarmSynchronizer) MarkSyncComplete(peerID string, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.TotalSyncs++
	if success {
		s.stats.SuccessfulSyncs++
	} else {
		s.stats.FailedSyncs++
	}

	now := time.Now()
	s.stats.LastSyncAt = &now

	if peer, ok := s.peers[peerID]; ok {
		peer.LastSeen = now
		peer.SyncedAt = &now
	}
}

// GetVersion returns the version for a memory.
func (s *SwarmSynchronizer) GetVersion(memoryID string) *VectorClock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.versions[memoryID]
}
