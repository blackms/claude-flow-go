// Package coordinator provides swarm coordination functionality.
package coordinator

import (
	"context"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// DomainPool manages agents within a specific domain.
type DomainPool struct {
	Config    shared.DomainConfig
	Agents    map[string]*agent.Agent
	TaskQueue chan shared.Task
	Metrics   *shared.DomainMetrics
	mu        sync.RWMutex

	// Internal tracking
	taskCount      int
	tasksCompleted int
	tasksFailed    int
	totalExecTime  int64
}

// NewDomainPool creates a new domain pool with the given configuration.
func NewDomainPool(config shared.DomainConfig) *DomainPool {
	return &DomainPool{
		Config:    config,
		Agents:    make(map[string]*agent.Agent),
		TaskQueue: make(chan shared.Task, 100), // Buffer for 100 tasks
		Metrics: &shared.DomainMetrics{
			Domain:      config.Name,
			SuccessRate: 1.0, // Start optimistic
		},
	}
}

// AddAgent adds an agent to the domain pool.
func (dp *DomainPool) AddAgent(a *agent.Agent) error {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	if _, exists := dp.Agents[a.ID]; exists {
		return shared.NewValidationError("agent already exists in domain", map[string]interface{}{
			"agentId": a.ID,
			"domain":  dp.Config.Name,
		})
	}

	dp.Agents[a.ID] = a
	return nil
}

// RemoveAgent removes an agent from the domain pool.
func (dp *DomainPool) RemoveAgent(agentID string) error {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	if _, exists := dp.Agents[agentID]; !exists {
		return shared.NewValidationError("agent not found in domain", map[string]interface{}{
			"agentId": agentID,
			"domain":  dp.Config.Name,
		})
	}

	delete(dp.Agents, agentID)
	return nil
}

// GetAgent retrieves an agent from the domain pool.
func (dp *DomainPool) GetAgent(agentID string) (*agent.Agent, bool) {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	a, exists := dp.Agents[agentID]
	return a, exists
}

// ListAgents returns all agents in the domain pool.
func (dp *DomainPool) ListAgents() []*agent.Agent {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	agents := make([]*agent.Agent, 0, len(dp.Agents))
	for _, a := range dp.Agents {
		agents = append(agents, a)
	}
	return agents
}

// GetAvailableAgents returns agents that are available for task assignment.
func (dp *DomainPool) GetAvailableAgents() []*agent.Agent {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	available := make([]*agent.Agent, 0)
	for _, a := range dp.Agents {
		sharedAgent := a.ToShared()
		if sharedAgent.Status == shared.AgentStatusActive || sharedAgent.Status == shared.AgentStatusIdle {
			available = append(available, a)
		}
	}
	return available
}

// GetAgentCount returns the number of agents in the domain.
func (dp *DomainPool) GetAgentCount() int {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	return len(dp.Agents)
}

// GetActiveAgentCount returns the number of active/idle agents.
func (dp *DomainPool) GetActiveAgentCount() int {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	count := 0
	for _, a := range dp.Agents {
		sharedAgent := a.ToShared()
		if sharedAgent.Status == shared.AgentStatusActive || sharedAgent.Status == shared.AgentStatusIdle {
			count++
		}
	}
	return count
}

// SubmitTask submits a task to the domain's task queue.
func (dp *DomainPool) SubmitTask(task shared.Task) error {
	select {
	case dp.TaskQueue <- task:
		dp.mu.Lock()
		dp.taskCount++
		dp.mu.Unlock()
		return nil
	default:
		return shared.NewExecutionError("task queue is full", map[string]interface{}{
			"domain":    dp.Config.Name,
			"queueSize": len(dp.TaskQueue),
		})
	}
}

// HasCapability checks if the domain has the given capability.
func (dp *DomainPool) HasCapability(capability string) bool {
	for _, cap := range dp.Config.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// MatchCapabilities returns a score (0-1) for how well the domain matches the required capabilities.
func (dp *DomainPool) MatchCapabilities(required []string) float64 {
	if len(required) == 0 {
		return 1.0
	}

	matched := 0
	for _, req := range required {
		if dp.HasCapability(req) {
			matched++
		}
	}

	return float64(matched) / float64(len(required))
}

// RecordTaskCompletion records that a task was completed.
func (dp *DomainPool) RecordTaskCompletion(duration int64, success bool) {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	if success {
		dp.tasksCompleted++
	} else {
		dp.tasksFailed++
	}
	dp.totalExecTime += duration

	// Update metrics
	total := dp.tasksCompleted + dp.tasksFailed
	if total > 0 {
		dp.Metrics.TasksCompleted = dp.tasksCompleted
		dp.Metrics.TasksFailed = dp.tasksFailed
		dp.Metrics.SuccessRate = float64(dp.tasksCompleted) / float64(total)
		dp.Metrics.AvgExecutionTime = float64(dp.totalExecTime) / float64(total)
	}
}

// GetMetrics returns the current domain metrics.
func (dp *DomainPool) GetMetrics() shared.DomainMetrics {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	return *dp.Metrics
}

// GetHealth returns the current health status of the domain.
func (dp *DomainPool) GetHealth() shared.DomainHealth {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	activeAgents := 0
	totalLoad := 0.0

	for _, a := range dp.Agents {
		sharedAgent := a.ToShared()
		if sharedAgent.Status == shared.AgentStatusActive || sharedAgent.Status == shared.AgentStatusIdle {
			activeAgents++
		}
		// Estimate load based on status
		switch sharedAgent.Status {
		case shared.AgentStatusBusy:
			totalLoad += 1.0
		case shared.AgentStatusActive:
			totalLoad += 0.5
		case shared.AgentStatusIdle:
			totalLoad += 0.1
		}
	}

	avgLoad := 0.0
	if len(dp.Agents) > 0 {
		avgLoad = totalLoad / float64(len(dp.Agents))
	}

	// Calculate health score based on various factors
	healthScore := 1.0
	bottlenecks := []string{}

	// Check agent availability
	if len(dp.Agents) > 0 {
		availabilityRatio := float64(activeAgents) / float64(len(dp.Agents))
		if availabilityRatio < 0.5 {
			healthScore -= 0.3
			bottlenecks = append(bottlenecks, "low agent availability")
		}
	} else {
		healthScore -= 0.5
		bottlenecks = append(bottlenecks, "no agents in domain")
	}

	// Check load
	if avgLoad > 0.8 {
		healthScore -= 0.2
		bottlenecks = append(bottlenecks, "high average load")
	}

	// Check queue depth
	queueDepth := len(dp.TaskQueue)
	if queueDepth > 50 {
		healthScore -= 0.2
		bottlenecks = append(bottlenecks, "task queue backlog")
	}

	// Check success rate
	if dp.Metrics.SuccessRate < 0.8 {
		healthScore -= 0.2
		bottlenecks = append(bottlenecks, "low success rate")
	}

	// Clamp health score
	if healthScore < 0 {
		healthScore = 0
	}

	return shared.DomainHealth{
		Domain:       dp.Config.Name,
		HealthScore:  healthScore,
		ActiveAgents: activeAgents,
		TotalAgents:  len(dp.Agents),
		AvgLoad:      avgLoad,
		Bottlenecks:  bottlenecks,
		LastCheck:    time.Now().UnixMilli(),
	}
}

// FindBestAgent finds the best agent for a task based on scoring.
func (dp *DomainPool) FindBestAgent(ctx context.Context, task shared.Task, agentHealth map[string]*shared.AgentHealth) (*agent.Agent, *shared.AgentScore) {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	var bestAgent *agent.Agent
	var bestScore *shared.AgentScore
	highestScore := -1.0

	for _, a := range dp.Agents {
		sharedAgent := a.ToShared()

		// Skip unavailable agents
		if sharedAgent.Status == shared.AgentStatusTerminated || sharedAgent.Status == shared.AgentStatusError {
			continue
		}

		score := dp.scoreAgent(a, task, agentHealth)
		score.CalculateTotalScore()

		if score.TotalScore > highestScore {
			highestScore = score.TotalScore
			bestAgent = a
			bestScore = score
		}
	}

	return bestAgent, bestScore
}

// scoreAgent calculates the score for an agent based on task requirements.
func (dp *DomainPool) scoreAgent(a *agent.Agent, task shared.Task, agentHealth map[string]*shared.AgentHealth) *shared.AgentScore {
	score := &shared.AgentScore{
		AgentID: a.ID,
	}

	// Capability score (40% weight)
	requiredCaps := extractRequiredCapabilities(task)
	matchedCaps := 0
	for _, req := range requiredCaps {
		if a.HasCapability(req) {
			matchedCaps++
		}
	}
	if len(requiredCaps) > 0 {
		score.CapabilityScore = float64(matchedCaps) / float64(len(requiredCaps))
	} else {
		score.CapabilityScore = 1.0
	}

	// Load score (25% weight) - lower load = higher score
	sharedAgent := a.ToShared()
	switch sharedAgent.Status {
	case shared.AgentStatusIdle:
		score.LoadScore = 1.0
	case shared.AgentStatusActive:
		score.LoadScore = 0.7
	case shared.AgentStatusBusy:
		score.LoadScore = 0.3
	default:
		score.LoadScore = 0.0
	}

	// Performance score (20% weight) - from historical metrics
	score.PerformanceScore = 0.8 // Default to good performance

	// Health score (15% weight)
	if health, ok := agentHealth[a.ID]; ok {
		score.HealthScore = health.HealthScore
	} else {
		score.HealthScore = 1.0 // Assume healthy if no data
	}

	return score
}

// extractRequiredCapabilities extracts required capabilities from a task.
func extractRequiredCapabilities(task shared.Task) []string {
	caps := []string{}

	// Map task type to capabilities
	switch task.Type {
	case shared.TaskTypeCode:
		caps = append(caps, "code", "develop")
	case shared.TaskTypeTest:
		caps = append(caps, "test", "validate")
	case shared.TaskTypeReview:
		caps = append(caps, "review", "analyze")
	case shared.TaskTypeDesign:
		caps = append(caps, "design", "architect")
	case shared.TaskTypeDeploy:
		caps = append(caps, "deploy", "release")
	}

	// Check metadata for additional capabilities
	if task.Metadata != nil {
		if reqCaps, ok := task.Metadata["requiredCapabilities"].([]string); ok {
			caps = append(caps, reqCaps...)
		}
	}

	return caps
}

// Close closes the domain pool and cleans up resources.
func (dp *DomainPool) Close() {
	close(dp.TaskQueue)
}
