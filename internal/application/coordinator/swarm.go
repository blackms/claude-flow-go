// Package coordinator provides the SwarmCoordinator for multi-agent orchestration.
package coordinator

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/domain/task"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/events"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// TaskExecutorInterface allows injecting an LLM-backed executor into the coordinator.
type TaskExecutorInterface interface {
	Execute(ctx context.Context, a *agent.Agent, t shared.Task, phaseContext string) (shared.TaskResult, error)
}

// SwarmCoordinator coordinates multi-agent swarms with support for hierarchical and mesh topologies.
type SwarmCoordinator struct {
	mu             sync.RWMutex
	topology       shared.SwarmTopology
	agents         map[string]*agent.Agent
	memoryBackend  shared.MemoryBackend
	eventBus       *events.EventBus
	pluginManager  shared.PluginManager
	agentMetrics   map[string]*shared.AgentMetrics
	connections    []shared.MeshConnection
	initialized    bool
	taskExecutor   TaskExecutorInterface
}

// Options holds configuration options for SwarmCoordinator.
type Options struct {
	Topology      shared.SwarmTopology
	MemoryBackend shared.MemoryBackend
	EventBus      *events.EventBus
	PluginManager shared.PluginManager
}

// New creates a new SwarmCoordinator.
func New(opts Options) *SwarmCoordinator {
	eventBus := opts.EventBus
	if eventBus == nil {
		eventBus = events.New()
	}

	return &SwarmCoordinator{
		topology:      opts.Topology,
		agents:        make(map[string]*agent.Agent),
		memoryBackend: opts.MemoryBackend,
		eventBus:      eventBus,
		pluginManager: opts.PluginManager,
		agentMetrics:  make(map[string]*shared.AgentMetrics),
		connections:   make([]shared.MeshConnection, 0),
	}
}

// Initialize initializes the coordinator.
func (sc *SwarmCoordinator) Initialize() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.initialized {
		return nil
	}

	sc.initialized = true
	return nil
}

// Shutdown shuts down the coordinator.
func (sc *SwarmCoordinator) Shutdown() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Terminate all agents
	for _, a := range sc.agents {
		a.Terminate()
	}

	sc.agents = make(map[string]*agent.Agent)
	sc.connections = make([]shared.MeshConnection, 0)
	sc.agentMetrics = make(map[string]*shared.AgentMetrics)
	sc.initialized = false

	return nil
}

// SpawnAgent spawns a new agent in the swarm.
func (sc *SwarmCoordinator) SpawnAgent(config shared.AgentConfig) (*agent.Agent, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	a := agent.FromConfig(config)
	sc.agents[a.ID] = a

	// Initialize metrics
	sc.agentMetrics[a.ID] = &shared.AgentMetrics{
		AgentID:              a.ID,
		TasksCompleted:       0,
		TasksFailed:          0,
		AverageExecutionTime: 0,
		SuccessRate:          1.0,
		Health:               "healthy",
	}

	// Create connections based on topology
	sc.updateConnections(a)

	// Emit spawn event
	sc.eventBus.EmitAgentSpawned(a.ID, a.Type)

	// Store in memory if backend available
	if sc.memoryBackend != nil {
		sc.memoryBackend.Store(shared.Memory{
			ID:        fmt.Sprintf("agent-spawn-%s", a.ID),
			AgentID:   "system",
			Content:   fmt.Sprintf("Agent %s spawned", a.ID),
			Type:      shared.MemoryTypeEvent,
			Timestamp: shared.Now(),
			Metadata: map[string]interface{}{
				"eventType": "agent-spawn",
				"agentId":   a.ID,
				"agentType": string(a.Type),
			},
		})
	}

	return a, nil
}

// ListAgents returns all agents in the swarm.
func (sc *SwarmCoordinator) ListAgents() []*agent.Agent {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	agents := make([]*agent.Agent, 0, len(sc.agents))
	for _, a := range sc.agents {
		agents = append(agents, a)
	}
	return agents
}

// GetAgent returns an agent by ID.
func (sc *SwarmCoordinator) GetAgent(agentID string) (*agent.Agent, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	a, exists := sc.agents[agentID]
	if !exists {
		return nil, shared.NewCoordinationError("agent not found", map[string]interface{}{"agentId": agentID})
	}
	return a, nil
}

// TerminateAgent terminates an agent.
func (sc *SwarmCoordinator) TerminateAgent(agentID string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	a, exists := sc.agents[agentID]
	if !exists {
		return nil
	}

	a.Terminate()
	delete(sc.agents, agentID)
	delete(sc.agentMetrics, agentID)

	// Remove connections
	newConnections := make([]shared.MeshConnection, 0)
	for _, c := range sc.connections {
		if c.From != agentID && c.To != agentID {
			newConnections = append(newConnections, c)
		}
	}
	sc.connections = newConnections

	sc.eventBus.EmitAgentTerminated(agentID)

	return nil
}

// DistributeTasks distributes tasks across agents.
func (sc *SwarmCoordinator) DistributeTasks(tasks []shared.Task) ([]shared.TaskAssignment, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	assignments := make([]shared.TaskAssignment, 0)
	agentLoads := make(map[string]int)

	// Initialize load counts
	for id := range sc.agents {
		agentLoads[id] = 0
	}

	// Convert to task entities and sort by priority
	taskEntities := task.ConvertFromShared(tasks)
	sortedTasks := task.SortByPriority(taskEntities)

	for _, t := range sortedTasks {
		// Find suitable agents
		suitableAgents := make([]*agent.Agent, 0)
		for _, a := range sc.agents {
			if a.CanExecute(t.Type) && a.GetStatus() == shared.AgentStatusActive {
				suitableAgents = append(suitableAgents, a)
			}
		}

		if len(suitableAgents) == 0 {
			continue
		}

		// Load balance: assign to agent with lowest load
		var bestAgent *agent.Agent
		lowestLoad := int(^uint(0) >> 1) // max int

		for _, a := range suitableAgents {
			load := agentLoads[a.ID]
			if load < lowestLoad {
				lowestLoad = load
				bestAgent = a
			}
		}

		if bestAgent != nil {
			assignments = append(assignments, shared.TaskAssignment{
				TaskID:     t.ID,
				AgentID:    bestAgent.ID,
				AssignedAt: shared.Now(),
				Priority:   t.Priority,
			})
			agentLoads[bestAgent.ID]++
		}
	}

	return assignments, nil
}

// SetTaskExecutor sets an optional LLM-backed task executor.
// When set, ExecuteTask will delegate to the executor instead of the agent's built-in method.
func (sc *SwarmCoordinator) SetTaskExecutor(te TaskExecutorInterface) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.taskExecutor = te
}

// ExecuteTask executes a task on a specific agent.
func (sc *SwarmCoordinator) ExecuteTask(ctx context.Context, agentID string, t shared.Task) (shared.TaskResult, error) {
	sc.mu.RLock()
	a, exists := sc.agents[agentID]
	te := sc.taskExecutor
	sc.mu.RUnlock()

	if !exists {
		return shared.TaskResult{
			TaskID:  t.ID,
			Status:  shared.TaskStatusFailed,
			Error:   fmt.Sprintf("Agent %s not found", agentID),
			AgentID: agentID,
		}, nil
	}

	// Use LLM executor if available; otherwise fall back to agent's built-in method.
	var result shared.TaskResult
	if te != nil {
		phaseContext, _ := t.Metadata["phaseContext"].(string)
		var err error
		result, err = te.Execute(ctx, a, t, phaseContext)
		if err != nil {
			// Fallback to built-in on LLM error.
			result = a.ExecuteTask(ctx, t)
		}
	} else {
		result = a.ExecuteTask(ctx, t)
	}

	// Update metrics
	sc.mu.Lock()
	metrics := sc.agentMetrics[agentID]
	if metrics != nil {
		if result.Status == shared.TaskStatusCompleted {
			metrics.TasksCompleted++
		} else {
			metrics.TasksFailed++
		}
		total := metrics.TasksCompleted + metrics.TasksFailed
		metrics.SuccessRate = float64(metrics.TasksCompleted) / float64(total)
		metrics.AverageExecutionTime = (metrics.AverageExecutionTime*float64(total-1) + float64(result.Duration)) / float64(total)
	}
	sc.mu.Unlock()

	// Store result in memory
	if sc.memoryBackend != nil {
		sc.memoryBackend.Store(shared.Memory{
			ID:        fmt.Sprintf("task-result-%s", t.ID),
			AgentID:   agentID,
			Content:   fmt.Sprintf("Task %s %s", t.ID, result.Status),
			Type:      shared.MemoryTypeTaskComplete,
			Timestamp: shared.Now(),
			Metadata: map[string]interface{}{
				"taskId":   t.ID,
				"status":   string(result.Status),
				"duration": result.Duration,
				"error":    result.Error,
			},
		})
	}

	return result, nil
}

// ExecuteTasksConcurrently executes multiple tasks concurrently.
func (sc *SwarmCoordinator) ExecuteTasksConcurrently(ctx context.Context, tasks []shared.Task) ([]shared.TaskResult, error) {
	assignments, err := sc.DistributeTasks(tasks)
	if err != nil {
		return nil, err
	}

	taskMap := make(map[string]shared.Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	results := make([]shared.TaskResult, len(assignments))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, assignment := range assignments {
		wg.Add(1)
		go func(idx int, a shared.TaskAssignment) {
			defer wg.Done()

			t, exists := taskMap[a.TaskID]
			if !exists {
				mu.Lock()
				results[idx] = shared.TaskResult{
					TaskID: a.TaskID,
					Status: shared.TaskStatusFailed,
					Error:  "Task not found",
				}
				mu.Unlock()
				return
			}

			result, _ := sc.ExecuteTask(ctx, a.AgentID, t)

			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, assignment)
	}

	wg.Wait()
	return results, nil
}

// SendMessage sends a message between agents.
func (sc *SwarmCoordinator) SendMessage(message shared.AgentMessage) error {
	message.Timestamp = shared.Now()
	sc.eventBus.EmitAgentMessage(message)
	return nil
}

// GetSwarmState returns the current swarm state.
func (sc *SwarmCoordinator) GetSwarmState() shared.SwarmState {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	agents := make([]shared.Agent, 0, len(sc.agents))
	for _, a := range sc.agents {
		agents = append(agents, a.ToShared())
	}

	var leader string
	if l := sc.getLeader(); l != nil {
		leader = l.ID
	}

	return shared.SwarmState{
		Agents:            agents,
		Topology:          sc.topology,
		Leader:            leader,
		ActiveConnections: len(sc.connections),
	}
}

// GetTopology returns the current topology.
func (sc *SwarmCoordinator) GetTopology() shared.SwarmTopology {
	return sc.topology
}

// GetHierarchy returns the swarm hierarchy.
func (sc *SwarmCoordinator) GetHierarchy() shared.SwarmHierarchy {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	leader := sc.getLeader()
	leaderID := ""
	if leader != nil {
		leaderID = leader.ID
	}

	workers := make([]shared.WorkerInfo, 0)
	for _, a := range sc.agents {
		if a.Role != shared.AgentRoleLeader {
			parent := a.Parent
			if parent == "" {
				parent = leaderID
			}
			workers = append(workers, shared.WorkerInfo{
				ID:     a.ID,
				Parent: parent,
			})
		}
	}

	return shared.SwarmHierarchy{
		Leader:  leaderID,
		Workers: workers,
	}
}

// GetMeshConnections returns mesh connections.
func (sc *SwarmCoordinator) GetMeshConnections() []shared.MeshConnection {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	connections := make([]shared.MeshConnection, len(sc.connections))
	copy(connections, sc.connections)
	return connections
}

// ScaleAgents scales agents of a specific type.
func (sc *SwarmCoordinator) ScaleAgents(agentType shared.AgentType, count int) error {
	sc.mu.Lock()
	existingOfType := make([]*agent.Agent, 0)
	for _, a := range sc.agents {
		if a.Type == agentType {
			existingOfType = append(existingOfType, a)
		}
	}
	sc.mu.Unlock()

	currentCount := len(existingOfType)

	if count > 0 {
		// Scale up
		for i := 0; i < count; i++ {
			_, err := sc.SpawnAgent(shared.AgentConfig{
				ID:           fmt.Sprintf("%s-%d-%d", agentType, shared.Now(), i),
				Type:         agentType,
				Capabilities: agent.GetDefaultCapabilities(agentType),
			})
			if err != nil {
				return err
			}
		}
	} else if count < 0 {
		// Scale down
		toRemove := min(currentCount, -count)
		for i := 0; i < toRemove; i++ {
			sc.TerminateAgent(existingOfType[i].ID)
		}
	}

	return nil
}

// ReachConsensus reaches consensus among agents.
func (sc *SwarmCoordinator) ReachConsensus(decision shared.ConsensusDecision, agentIDs []string) (shared.ConsensusResult, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	votes := make([]shared.AgentVote, 0)

	for _, agentID := range agentIDs {
		if _, exists := sc.agents[agentID]; exists {
			// Simulate voting (in real implementation, would involve LLM calls)
			vote := "approve"
			if rand.Float64() < 0.5 {
				vote = "reject"
			}
			votes = append(votes, shared.AgentVote{
				AgentID: agentID,
				Vote:    vote,
			})
		}
	}

	approves := 0
	for _, v := range votes {
		if v.Vote == "approve" {
			approves++
		}
	}

	consensusReached := approves > len(votes)/2

	var consensusDecision interface{}
	if consensusReached {
		consensusDecision = decision.Payload
	}

	return shared.ConsensusResult{
		Decision:         consensusDecision,
		Votes:            votes,
		ConsensusReached: consensusReached,
	}, nil
}

// ResolveTaskDependencies resolves task dependencies.
func (sc *SwarmCoordinator) ResolveTaskDependencies(tasks []shared.Task) ([]shared.Task, error) {
	taskEntities := task.ConvertFromShared(tasks)
	ordered, err := task.ResolveExecutionOrder(taskEntities)
	if err != nil {
		return nil, err
	}
	return task.ConvertToShared(ordered), nil
}

// GetAgentMetrics returns metrics for an agent.
func (sc *SwarmCoordinator) GetAgentMetrics(agentID string) shared.AgentMetrics {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	metrics := sc.agentMetrics[agentID]
	if metrics == nil {
		return shared.AgentMetrics{
			AgentID:              agentID,
			TasksCompleted:       0,
			AverageExecutionTime: 0,
			SuccessRate:          0,
			Health:               "unhealthy",
		}
	}
	return *metrics
}

// Reconfigure reconfigures the swarm.
func (sc *SwarmCoordinator) Reconfigure(topology shared.SwarmTopology) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.topology = topology

	// Rebuild connections based on new topology
	sc.connections = make([]shared.MeshConnection, 0)
	for _, a := range sc.agents {
		sc.updateConnections(a)
	}

	return nil
}

// GetEventBus returns the event bus.
func (sc *SwarmCoordinator) GetEventBus() *events.EventBus {
	return sc.eventBus
}

// ============================================================================
// Private Helper Methods
// ============================================================================

func (sc *SwarmCoordinator) getLeader() *agent.Agent {
	for _, a := range sc.agents {
		if a.Role == shared.AgentRoleLeader {
			return a
		}
	}
	return nil
}

func (sc *SwarmCoordinator) updateConnections(a *agent.Agent) {
	switch sc.topology {
	case shared.TopologyMesh:
		// In mesh, connect to all other agents
		for _, other := range sc.agents {
			if other.ID != a.ID {
				sc.connections = append(sc.connections, shared.MeshConnection{
					From: a.ID,
					To:   other.ID,
					Type: "peer",
				})
			}
		}
	case shared.TopologyHierarchical:
		// In hierarchical, connect workers to leader
		leader := sc.getLeader()
		if leader != nil && a.Role != shared.AgentRoleLeader {
			sc.connections = append(sc.connections, shared.MeshConnection{
				From: a.ID,
				To:   leader.ID,
				Type: "leader",
			})
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SortAgentsByID sorts agents by ID for consistent ordering.
func SortAgentsByID(agents []*agent.Agent) []*agent.Agent {
	sorted := make([]*agent.Agent, len(agents))
	copy(sorted, agents)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})
	return sorted
}
