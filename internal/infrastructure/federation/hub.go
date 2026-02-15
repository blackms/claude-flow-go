// Package federation provides cross-swarm coordination and ephemeral agent management.
package federation

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
	"github.com/google/uuid"
)

// EventHandler is a callback for federation events.
type EventHandler func(event shared.FederationEvent)

// FederationHub manages cross-swarm coordination and ephemeral agents.
type FederationHub struct {
	config shared.FederationConfig

	// Swarm registry
	swarms map[string]*shared.SwarmRegistration

	// Ephemeral agents
	ephemeralAgents map[string]*shared.EphemeralAgent

	// O(1) indexes
	agentsBySwarm  map[string]map[string]bool                    // swarmID -> set of agentIDs
	agentsByStatus map[shared.EphemeralAgentStatus]map[string]bool // status -> set of agentIDs

	// Messaging
	messages     []*shared.FederationMessage
	messageCount atomic.Int64

	// Consensus
	proposals map[string]*shared.FederationProposal

	// Event history
	events       []*shared.FederationEvent
	eventHandler EventHandler

	// Statistics
	stats          shared.FederationStats
	spawnTimes     []int64
	messageTimes   []int64
	statsMaxSamples int

	// Background processing
	ctx          context.Context
	cancel       context.CancelFunc
	syncTicker   *time.Ticker
	cleanupTicker *time.Ticker

	mu sync.RWMutex
}

// NewFederationHub creates a new FederationHub with the given configuration.
func NewFederationHub(config shared.FederationConfig) *FederationHub {
	ctx, cancel := context.WithCancel(context.Background())

	hub := &FederationHub{
		config:          config,
		swarms:          make(map[string]*shared.SwarmRegistration),
		ephemeralAgents: make(map[string]*shared.EphemeralAgent),
		agentsBySwarm:   make(map[string]map[string]bool),
		agentsByStatus:  make(map[shared.EphemeralAgentStatus]map[string]bool),
		messages:        make([]*shared.FederationMessage, 0),
		proposals:       make(map[string]*shared.FederationProposal),
		events:          make([]*shared.FederationEvent, 0),
		spawnTimes:      make([]int64, 0),
		messageTimes:    make([]int64, 0),
		statsMaxSamples: 100,
		ctx:             ctx,
		cancel:          cancel,
	}

	// Initialize status indexes
	hub.agentsByStatus[shared.EphemeralStatusSpawning] = make(map[string]bool)
	hub.agentsByStatus[shared.EphemeralStatusActive] = make(map[string]bool)
	hub.agentsByStatus[shared.EphemeralStatusCompleting] = make(map[string]bool)
	hub.agentsByStatus[shared.EphemeralStatusTerminated] = make(map[string]bool)

	return hub
}

// NewFederationHubWithDefaults creates a new FederationHub with default configuration.
func NewFederationHubWithDefaults() *FederationHub {
	return NewFederationHub(shared.DefaultFederationConfig())
}

// Initialize starts the federation hub background processes.
func (fh *FederationHub) Initialize() error {
	// Start sync loop
	fh.syncTicker = time.NewTicker(time.Duration(fh.config.SyncInterval) * time.Millisecond)
	go fh.syncLoop()

	// Start cleanup loop if enabled
	if fh.config.AutoCleanupEnabled {
		fh.cleanupTicker = time.NewTicker(time.Duration(fh.config.CleanupInterval) * time.Millisecond)
		go fh.cleanupLoop()
	}

	return nil
}

// Shutdown stops the federation hub and cleans up resources.
func (fh *FederationHub) Shutdown() error {
	fh.cancel()

	if fh.syncTicker != nil {
		fh.syncTicker.Stop()
	}
	if fh.cleanupTicker != nil {
		fh.cleanupTicker.Stop()
	}

	// Terminate all ephemeral agents
	fh.mu.Lock()
	for agentID := range fh.ephemeralAgents {
		fh.terminateAgentInternal(agentID, "federation shutdown")
	}
	fh.mu.Unlock()

	return nil
}

// ============================================================================
// Swarm Registration
// ============================================================================

// RegisterSwarm registers a swarm with the federation.
func (fh *FederationHub) RegisterSwarm(swarm shared.SwarmRegistration) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	swarmID := strings.TrimSpace(swarm.SwarmID)
	if swarmID == "" {
		return fmt.Errorf("swarmId is required")
	}
	if swarm.MaxAgents <= 0 {
		return fmt.Errorf("maxAgents must be greater than 0")
	}

	swarm.SwarmID = swarmID
	swarm.Name = strings.TrimSpace(swarm.Name)
	swarm.Endpoint = strings.TrimSpace(swarm.Endpoint)

	if _, exists := fh.swarms[swarm.SwarmID]; exists {
		return fmt.Errorf("swarm %s already exists", swarm.SwarmID)
	}

	now := shared.Now()
	swarm.RegisteredAt = now
	swarm.LastHeartbeat = now
	swarm.Status = shared.SwarmStatusActive

	fh.swarms[swarm.SwarmID] = &swarm
	fh.agentsBySwarm[swarm.SwarmID] = make(map[string]bool)

	fh.stats.TotalSwarms++
	fh.stats.ActiveSwarms++

	fh.emitEvent(shared.FederationEvent{
		Type:      shared.FederationEventSwarmJoined,
		SwarmID:   swarm.SwarmID,
		Timestamp: now,
	})

	return nil
}

// UnregisterSwarm removes a swarm from the federation.
func (fh *FederationHub) UnregisterSwarm(swarmID string) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	swarm, exists := fh.swarms[swarmID]
	if !exists {
		return fmt.Errorf("swarm %s not found", swarmID)
	}

	// Terminate all agents in this swarm
	for agentID := range fh.agentsBySwarm[swarmID] {
		fh.terminateAgentInternal(agentID, "swarm unregistered")
	}

	delete(fh.swarms, swarmID)
	delete(fh.agentsBySwarm, swarmID)

	if swarm.Status == shared.SwarmStatusActive {
		fh.stats.ActiveSwarms--
	}

	fh.emitEvent(shared.FederationEvent{
		Type:      shared.FederationEventSwarmLeft,
		SwarmID:   swarmID,
		Timestamp: shared.Now(),
	})

	return nil
}

// Heartbeat updates the heartbeat for a swarm.
func (fh *FederationHub) Heartbeat(swarmID string) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	swarm, exists := fh.swarms[swarmID]
	if !exists {
		return fmt.Errorf("swarm %s not found", swarmID)
	}

	now := shared.Now()
	swarm.LastHeartbeat = now

	// Reactivate if was degraded/inactive
	if swarm.Status != shared.SwarmStatusActive {
		oldStatus := swarm.Status
		swarm.Status = shared.SwarmStatusActive
		if oldStatus == shared.SwarmStatusInactive {
			fh.stats.ActiveSwarms++
		}
	}

	return nil
}

// GetSwarm returns a swarm by ID.
func (fh *FederationHub) GetSwarm(swarmID string) (*shared.SwarmRegistration, bool) {
	fh.mu.RLock()
	defer fh.mu.RUnlock()
	swarm, exists := fh.swarms[swarmID]
	return swarm, exists
}

// GetSwarms returns all registered swarms.
func (fh *FederationHub) GetSwarms() []*shared.SwarmRegistration {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	swarms := make([]*shared.SwarmRegistration, 0, len(fh.swarms))
	for _, swarm := range fh.swarms {
		swarms = append(swarms, swarm)
	}
	return swarms
}

// GetActiveSwarms returns all active swarms.
func (fh *FederationHub) GetActiveSwarms() []*shared.SwarmRegistration {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	swarms := make([]*shared.SwarmRegistration, 0)
	for _, swarm := range fh.swarms {
		if swarm.Status == shared.SwarmStatusActive {
			swarms = append(swarms, swarm)
		}
	}
	return swarms
}

// ============================================================================
// Background Loops
// ============================================================================

func (fh *FederationHub) syncLoop() {
	for {
		select {
		case <-fh.ctx.Done():
			return
		case <-fh.syncTicker.C:
			fh.syncFederation()
		}
	}
}

func (fh *FederationHub) cleanupLoop() {
	for {
		select {
		case <-fh.ctx.Done():
			return
		case <-fh.cleanupTicker.C:
			fh.cleanupExpiredAgents()
		}
	}
}

// syncFederation performs periodic federation sync.
func (fh *FederationHub) syncFederation() {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	now := shared.Now()
	heartbeatThreshold := fh.config.HeartbeatInterval * 3 // 3x heartbeat = degraded
	inactiveThreshold := fh.config.HeartbeatInterval * 6  // 6x heartbeat = inactive

	// Check swarm health
	for _, swarm := range fh.swarms {
		timeSinceHeartbeat := now - swarm.LastHeartbeat

		if timeSinceHeartbeat > inactiveThreshold && swarm.Status != shared.SwarmStatusInactive {
			if swarm.Status == shared.SwarmStatusActive {
				fh.stats.ActiveSwarms--
			}
			swarm.Status = shared.SwarmStatusInactive
		} else if timeSinceHeartbeat > heartbeatThreshold && swarm.Status == shared.SwarmStatusActive {
			swarm.Status = shared.SwarmStatusDegraded
			fh.emitEvent(shared.FederationEvent{
				Type:      shared.FederationEventSwarmDegraded,
				SwarmID:   swarm.SwarmID,
				Timestamp: now,
			})
		}
	}

	// Expire pending proposals
	for _, proposal := range fh.proposals {
		if proposal.Status == shared.FederationProposalPending && now > proposal.ExpiresAt {
			proposal.Status = shared.FederationProposalRejected
			fh.stats.RejectedProposals++
			fh.stats.PendingProposals--

			fh.emitEvent(shared.FederationEvent{
				Type:      shared.FederationEventConsensusCompleted,
				Data:      map[string]interface{}{"proposalId": proposal.ID, "status": "expired"},
				Timestamp: now,
			})
		}
	}

	fh.emitEvent(shared.FederationEvent{
		Type:      shared.FederationEventSynced,
		Timestamp: now,
	})
}

// ============================================================================
// Event Handling
// ============================================================================

// SetEventHandler sets the event handler for federation events.
func (fh *FederationHub) SetEventHandler(handler EventHandler) {
	fh.eventHandler = handler
}

// emitEvent emits a federation event.
func (fh *FederationHub) emitEvent(event shared.FederationEvent) {
	fh.events = append(fh.events, &event)

	// Limit event history
	if len(fh.events) > fh.config.MaxMessageHistory {
		fh.events = fh.events[1:]
	}

	if fh.eventHandler != nil {
		go fh.eventHandler(event)
	}
}

// GetEvents returns recent federation events.
func (fh *FederationHub) GetEvents(limit int) []*shared.FederationEvent {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	if limit <= 0 || limit > len(fh.events) {
		limit = len(fh.events)
	}

	// Return most recent events
	start := len(fh.events) - limit
	result := make([]*shared.FederationEvent, limit)
	copy(result, fh.events[start:])
	return result
}

// ============================================================================
// Statistics
// ============================================================================

// GetStats returns federation statistics.
func (fh *FederationHub) GetStats() shared.FederationStats {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	// Update calculated stats
	stats := fh.stats
	stats.TotalMessages = fh.messageCount.Load()

	// Calculate averages
	if len(fh.spawnTimes) > 0 {
		var sum int64
		for _, t := range fh.spawnTimes {
			sum += t
		}
		stats.AvgSpawnTimeMs = float64(sum) / float64(len(fh.spawnTimes))
	}

	if len(fh.messageTimes) > 0 {
		var sum int64
		for _, t := range fh.messageTimes {
			sum += t
		}
		stats.AvgMessageLatencyMs = float64(sum) / float64(len(fh.messageTimes))
	}

	return stats
}

// GetConfig returns the federation configuration.
func (fh *FederationHub) GetConfig() shared.FederationConfig {
	return fh.config
}

// recordSpawnTime records a spawn time for statistics.
func (fh *FederationHub) recordSpawnTime(durationMs int64) {
	fh.spawnTimes = append(fh.spawnTimes, durationMs)
	if len(fh.spawnTimes) > fh.statsMaxSamples {
		fh.spawnTimes = fh.spawnTimes[1:]
	}
}

// recordMessageTime records a message delivery time for statistics.
func (fh *FederationHub) recordMessageTime(durationMs int64) {
	fh.messageTimes = append(fh.messageTimes, durationMs)
	if len(fh.messageTimes) > fh.statsMaxSamples {
		fh.messageTimes = fh.messageTimes[1:]
	}
}

// generateID generates a unique ID.
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, uuid.New().String()[:8])
}
