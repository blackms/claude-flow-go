// Package federation provides cross-swarm coordination and ephemeral agent management.
package federation

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ============================================================================
// Ephemeral Agent Management
// ============================================================================

// SpawnEphemeralAgent spawns a new ephemeral agent.
func (fh *FederationHub) SpawnEphemeralAgent(opts shared.SpawnEphemeralOptions) (*shared.SpawnResult, error) {
	startTime := shared.Now()

	fh.mu.Lock()
	defer fh.mu.Unlock()

	opts.SwarmID = strings.TrimSpace(opts.SwarmID)
	opts.Type = strings.TrimSpace(opts.Type)
	opts.Task = strings.TrimSpace(opts.Task)
	opts.Capabilities = normalizeStringValues(opts.Capabilities)
	if opts.Type == "" {
		return &shared.SpawnResult{
			Status: "failed",
			Error:  "type is required",
		}, fmt.Errorf("type is required")
	}
	if opts.Task == "" {
		return &shared.SpawnResult{
			Status: "failed",
			Error:  "task is required",
		}, fmt.Errorf("task is required")
	}

	// Set default TTL
	if opts.TTL <= 0 {
		opts.TTL = fh.config.DefaultAgentTTL
	}
	if opts.TTL <= 0 {
		return &shared.SpawnResult{
			Status: "failed",
			Error:  "ttl must be greater than 0",
		}, fmt.Errorf("ttl must be greater than 0")
	}

	// Select optimal swarm if not specified
	var swarmID string
	if opts.SwarmID != "" {
		swarmID = opts.SwarmID
		// Verify swarm exists and is active
		swarm, exists := fh.swarms[swarmID]
		if !exists {
			return &shared.SpawnResult{
				Status: "failed",
				Error:  fmt.Sprintf("swarm %s not found", swarmID),
			}, fmt.Errorf("swarm %s not found", swarmID)
		}
		if swarm.Status != shared.SwarmStatusActive {
			return &shared.SpawnResult{
				Status: "failed",
				Error:  fmt.Sprintf("swarm %s is not active", swarmID),
			}, fmt.Errorf("swarm %s is not active", swarmID)
		}
		if swarm.CurrentAgents >= swarm.MaxAgents {
			return &shared.SpawnResult{
				Status: "failed",
				Error:  fmt.Sprintf("swarm %s is at capacity", swarmID),
			}, fmt.Errorf("swarm %s is at capacity", swarmID)
		}
	} else {
		// Auto-select optimal swarm
		selected := fh.selectOptimalSwarm(opts.Capabilities)
		if selected == nil {
			return &shared.SpawnResult{
				Status: "failed",
				Error:  "no suitable swarm available",
			}, fmt.Errorf("no suitable swarm available")
		}
		swarmID = selected.SwarmID
	}

	// Create ephemeral agent
	now := shared.Now()
	if opts.TTL > math.MaxInt64-now {
		return &shared.SpawnResult{
			Status: "failed",
			Error:  "ttl is out of range",
		}, fmt.Errorf("ttl is out of range")
	}
	agentID := generateID("ephemeral")

	agent := &shared.EphemeralAgent{
		ID:        agentID,
		SwarmID:   swarmID,
		Type:      opts.Type,
		Task:      opts.Task,
		Status:    shared.EphemeralStatusSpawning,
		TTL:       opts.TTL,
		CreatedAt: now,
		ExpiresAt: now + opts.TTL,
		Metadata:  opts.Metadata,
	}

	// Add to indexes
	fh.ephemeralAgents[agentID] = agent
	fh.agentsBySwarm[swarmID][agentID] = true
	fh.agentsByStatus[shared.EphemeralStatusSpawning][agentID] = true

	// Update swarm agent count
	if swarm, exists := fh.swarms[swarmID]; exists {
		swarm.CurrentAgents++
	}

	// Update stats
	fh.stats.TotalAgents++
	fh.stats.ActiveAgents++

	// Emit event
	fh.emitEvent(shared.FederationEvent{
		Type:      shared.FederationEventAgentSpawned,
		SwarmID:   swarmID,
		AgentID:   agentID,
		Timestamp: now,
	})

	// Transition to active after spawn delay (simulate spawn time)
	go func() {
		time.Sleep(10 * time.Millisecond) // Target: <50ms
		fh.activateAgent(agentID)
	}()

	// Record spawn time
	spawnDuration := shared.Now() - startTime
	fh.recordSpawnTime(spawnDuration)

	return &shared.SpawnResult{
		AgentID:      agentID,
		SwarmID:      swarmID,
		Status:       "spawned",
		EstimatedTTL: opts.TTL,
	}, nil
}

// activateAgent transitions an agent from spawning to active.
func (fh *FederationHub) activateAgent(agentID string) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	agent, exists := fh.ephemeralAgents[agentID]
	if !exists || agent.Status != shared.EphemeralStatusSpawning {
		return
	}

	// Update status indexes
	delete(fh.agentsByStatus[shared.EphemeralStatusSpawning], agentID)
	fh.agentsByStatus[shared.EphemeralStatusActive][agentID] = true

	agent.Status = shared.EphemeralStatusActive
}

// CompleteAgent marks an agent as completing with a result.
func (fh *FederationHub) CompleteAgent(agentID string, result interface{}) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return fmt.Errorf("agentId is required")
	}

	agent, exists := fh.ephemeralAgents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	if agent.Status == shared.EphemeralStatusTerminated {
		return fmt.Errorf("agent %s already terminated", agentID)
	}

	now := shared.Now()

	// Update status indexes
	delete(fh.agentsByStatus[agent.Status], agentID)
	fh.agentsByStatus[shared.EphemeralStatusCompleting][agentID] = true

	agent.Status = shared.EphemeralStatusCompleting
	agent.Result = result
	agent.CompletedAt = now

	// Emit event
	fh.emitEvent(shared.FederationEvent{
		Type:      shared.FederationEventAgentCompleted,
		SwarmID:   agent.SwarmID,
		AgentID:   agentID,
		Data:      result,
		Timestamp: now,
	})

	// Transition to terminated
	go func() {
		time.Sleep(10 * time.Millisecond)
		fh.terminateAgent(agentID, "")
	}()

	return nil
}

// TerminateAgent terminates an agent with an optional error.
func (fh *FederationHub) TerminateAgent(agentID string, errorMsg string) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return fmt.Errorf("agentId is required")
	}
	return fh.terminateAgentInternal(agentID, errorMsg)
}

// terminateAgent terminates an agent (acquires lock).
func (fh *FederationHub) terminateAgent(agentID string, errorMsg string) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.terminateAgentInternal(agentID, errorMsg)
}

// terminateAgentInternal terminates an agent (must be called with lock held).
func (fh *FederationHub) terminateAgentInternal(agentID string, errorMsg string) error {
	agent, exists := fh.ephemeralAgents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	if agent.Status == shared.EphemeralStatusTerminated {
		return nil // Already terminated
	}

	now := shared.Now()

	// Update status indexes
	delete(fh.agentsByStatus[agent.Status], agentID)
	fh.agentsByStatus[shared.EphemeralStatusTerminated][agentID] = true

	agent.Status = shared.EphemeralStatusTerminated
	if errorMsg != "" {
		agent.Error = errorMsg
	}
	if agent.CompletedAt == 0 {
		agent.CompletedAt = now
	}

	// Update swarm agent count
	if swarm, exists := fh.swarms[agent.SwarmID]; exists {
		swarm.CurrentAgents--
	}

	// Update stats
	fh.stats.ActiveAgents--

	// Emit event
	eventType := shared.FederationEventAgentCompleted
	if errorMsg != "" {
		eventType = shared.FederationEventAgentFailed
	}
	fh.emitEvent(shared.FederationEvent{
		Type:      eventType,
		SwarmID:   agent.SwarmID,
		AgentID:   agentID,
		Data:      map[string]interface{}{"error": errorMsg},
		Timestamp: now,
	})

	return nil
}

// GetAgent returns an agent by ID.
func (fh *FederationHub) GetAgent(agentID string) (*shared.EphemeralAgent, bool) {
	fh.mu.RLock()
	defer fh.mu.RUnlock()
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, false
	}
	agent, exists := fh.ephemeralAgents[agentID]
	if !exists {
		return nil, false
	}
	return cloneEphemeralAgent(agent), true
}

// GetAgents returns all ephemeral agents.
func (fh *FederationHub) GetAgents() []*shared.EphemeralAgent {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	agents := make([]*shared.EphemeralAgent, 0, len(fh.ephemeralAgents))
	for _, agent := range fh.ephemeralAgents {
		agents = append(agents, cloneEphemeralAgent(agent))
	}
	return agents
}

// GetActiveAgents returns all active ephemeral agents.
func (fh *FederationHub) GetActiveAgents() []*shared.EphemeralAgent {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	agents := make([]*shared.EphemeralAgent, 0)
	for agentID := range fh.agentsByStatus[shared.EphemeralStatusActive] {
		if agent, exists := fh.ephemeralAgents[agentID]; exists {
			agents = append(agents, cloneEphemeralAgent(agent))
		}
	}
	return agents
}

// GetAgentsBySwarm returns all agents in a swarm. O(1) lookup.
func (fh *FederationHub) GetAgentsBySwarm(swarmID string) []*shared.EphemeralAgent {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	swarmID = strings.TrimSpace(swarmID)
	if swarmID == "" {
		return []*shared.EphemeralAgent{}
	}

	agents := make([]*shared.EphemeralAgent, 0)
	if agentIDs, exists := fh.agentsBySwarm[swarmID]; exists {
		for agentID := range agentIDs {
			if agent, exists := fh.ephemeralAgents[agentID]; exists {
				agents = append(agents, cloneEphemeralAgent(agent))
			}
		}
	}
	return agents
}

// GetAgentsByStatus returns all agents with a given status. O(1) lookup.
func (fh *FederationHub) GetAgentsByStatus(status shared.EphemeralAgentStatus) []*shared.EphemeralAgent {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	agents := make([]*shared.EphemeralAgent, 0)
	if agentIDs, exists := fh.agentsByStatus[status]; exists {
		for agentID := range agentIDs {
			if agent, exists := fh.ephemeralAgents[agentID]; exists {
				agents = append(agents, cloneEphemeralAgent(agent))
			}
		}
	}
	return agents
}

// cleanupExpiredAgents removes expired agents.
func (fh *FederationHub) cleanupExpiredAgents() {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	now := shared.Now()
	expiredAgents := make([]string, 0)

	// Find expired agents
	for agentID, agent := range fh.ephemeralAgents {
		if agent.Status != shared.EphemeralStatusTerminated && now > agent.ExpiresAt {
			expiredAgents = append(expiredAgents, agentID)
		}
	}

	// Terminate expired agents
	for _, agentID := range expiredAgents {
		agent := fh.ephemeralAgents[agentID]

		// Update status indexes
		delete(fh.agentsByStatus[agent.Status], agentID)
		fh.agentsByStatus[shared.EphemeralStatusTerminated][agentID] = true

		agent.Status = shared.EphemeralStatusTerminated
		agent.Error = "TTL expired"
		agent.CompletedAt = now

		// Update swarm agent count
		if swarm, exists := fh.swarms[agent.SwarmID]; exists {
			swarm.CurrentAgents--
		}

		// Update stats
		fh.stats.ActiveAgents--

		// Emit event
		fh.emitEvent(shared.FederationEvent{
			Type:      shared.FederationEventAgentExpired,
			SwarmID:   agent.SwarmID,
			AgentID:   agentID,
			Timestamp: now,
		})
	}

	// Optionally clean up terminated agents from memory
	// (keep for history or remove after a grace period)
}

// selectOptimalSwarm selects the best swarm for spawning an agent.
func (fh *FederationHub) selectOptimalSwarm(requiredCapabilities []string) *shared.SwarmRegistration {
	var bestSwarm *shared.SwarmRegistration
	bestScore := -1.0

	for _, swarm := range fh.swarms {
		// Must be active
		if swarm.Status != shared.SwarmStatusActive {
			continue
		}

		// Must have capacity
		if swarm.CurrentAgents >= swarm.MaxAgents {
			continue
		}

		// Check capabilities
		if !fh.hasCapabilities(swarm, requiredCapabilities) {
			continue
		}

		// Calculate score
		score := fh.calculateSwarmScore(swarm)
		if score > bestScore {
			bestScore = score
			bestSwarm = swarm
		}
	}

	return bestSwarm
}

// hasCapabilities checks if a swarm has the required capabilities.
func (fh *FederationHub) hasCapabilities(swarm *shared.SwarmRegistration, required []string) bool {
	if len(required) == 0 {
		return true
	}

	swarmCaps := make(map[string]bool)
	for _, cap := range swarm.Capabilities {
		swarmCaps[cap] = true
	}

	for _, cap := range required {
		if !swarmCaps[cap] {
			return false
		}
	}
	return true
}

// calculateSwarmScore calculates a score for swarm selection.
func (fh *FederationHub) calculateSwarmScore(swarm *shared.SwarmRegistration) float64 {
	// Higher score = better choice
	// Factors: available capacity, status

	// Capacity score (0-1): prefer swarms with more available slots
	capacityRatio := 1.0 - (float64(swarm.CurrentAgents) / float64(swarm.MaxAgents))

	// Status score
	statusScore := 1.0
	if swarm.Status == shared.SwarmStatusDegraded {
		statusScore = 0.5
	}

	return capacityRatio * statusScore
}
