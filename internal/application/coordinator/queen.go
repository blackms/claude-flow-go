// Package coordinator provides swarm coordination functionality.
package coordinator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/shared"
	"github.com/google/uuid"
)

// QueenCoordinator is the top-level coordinator for the 15-agent domain architecture.
// It manages task analysis, agent delegation, and swarm health.
type QueenCoordinator struct {
	swarm          *SwarmCoordinator
	domains        map[shared.AgentDomain]*DomainPool
	neuralLearning shared.NeuralLearningSystem
	memoryService  shared.MemoryService
	healthMonitor  *HealthMonitor
	config         QueenConfig
	mu             sync.RWMutex

	// Queen agent reference
	queenAgent *agent.Agent

	// Metrics
	tasksAnalyzed   int
	tasksDelegated  int
	totalAnalysisTime int64
}

// QueenConfig holds configuration for the Queen Coordinator.
type QueenConfig struct {
	// Performance targets
	MaxAnalysisTime    time.Duration // Target: <50ms
	MaxScoringTime     time.Duration // Target: <20ms
	MaxCoordinationTime time.Duration // Target: <100ms

	// Scoring weights
	CapabilityWeight   float64 // Default: 0.4
	LoadWeight         float64 // Default: 0.25
	PerformanceWeight  float64 // Default: 0.2
	HealthWeight       float64 // Default: 0.15

	// Behavior settings
	EnablePatternMatching bool
	PatternMatchK         int   // Number of patterns to match
	BackupAgentCount      int   // Number of backup agents to select
}

// DefaultQueenConfig returns the default Queen Coordinator configuration.
func DefaultQueenConfig() QueenConfig {
	return QueenConfig{
		MaxAnalysisTime:     50 * time.Millisecond,
		MaxScoringTime:      20 * time.Millisecond,
		MaxCoordinationTime: 100 * time.Millisecond,
		CapabilityWeight:    0.4,
		LoadWeight:          0.25,
		PerformanceWeight:   0.2,
		HealthWeight:        0.15,
		EnablePatternMatching: true,
		PatternMatchK:       5,
		BackupAgentCount:    2,
	}
}

// NewQueenCoordinator creates a new Queen Coordinator.
func NewQueenCoordinator(swarm *SwarmCoordinator, config QueenConfig) (*QueenCoordinator, error) {
	if swarm == nil {
		return nil, shared.NewValidationError("swarm coordinator is required", nil)
	}

	qc := &QueenCoordinator{
		swarm:         swarm,
		domains:       make(map[shared.AgentDomain]*DomainPool),
		healthMonitor: NewHealthMonitor(DefaultHealthMonitorConfig()),
		config:        config,
	}

	// Initialize domain pools from default configurations
	for _, domainConfig := range shared.DefaultDomainConfigs() {
		qc.domains[domainConfig.Name] = NewDomainPool(domainConfig)
	}

	return qc, nil
}

// SetNeuralLearning sets the neural learning system.
func (qc *QueenCoordinator) SetNeuralLearning(nl shared.NeuralLearningSystem) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.neuralLearning = nl
}

// SetMemoryService sets the memory service.
func (qc *QueenCoordinator) SetMemoryService(ms shared.MemoryService) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.memoryService = ms
}

// Start starts the Queen Coordinator and its health monitoring.
func (qc *QueenCoordinator) Start() {
	qc.healthMonitor.Start()
}

// Stop stops the Queen Coordinator.
func (qc *QueenCoordinator) Stop() {
	qc.healthMonitor.Stop()

	// Close all domain pools
	for _, pool := range qc.domains {
		pool.Close()
	}
}

// SpawnFullHierarchy spawns the complete 15-agent hierarchy.
func (qc *QueenCoordinator) SpawnFullHierarchy(ctx context.Context) error {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	// Spawn agents for each domain
	for agentNum := 1; agentNum <= 15; agentNum++ {
		domain := shared.GetDomainForAgentNumber(agentNum)
		agentType := shared.GetAgentTypeForNumber(agentNum)

		// Determine role
		role := shared.AgentRoleWorker
		if agentNum == 1 {
			role = shared.AgentRoleLeader
		}

		config := shared.AgentConfig{
			ID:          fmt.Sprintf("agent-%d", agentNum),
			Type:        agentType,
			Role:        role,
			Domain:      domain,
			AgentNumber: agentNum,
		}

		// Spawn via swarm coordinator
		spawnedAgent, err := qc.swarm.SpawnAgent(config)
		if err != nil {
			return shared.NewCoordinationError(
				fmt.Sprintf("failed to spawn agent %d", agentNum),
				map[string]interface{}{"agentNumber": agentNum, "error": err.Error()},
			)
		}

		// Create domain agent and add to pool
		domainAgent := agent.FromConfig(config)
		if pool, ok := qc.domains[domain]; ok {
			if err := pool.AddAgent(domainAgent); err != nil {
				return err
			}
		}

		// Register with health monitor
		qc.healthMonitor.RegisterAgent(spawnedAgent.ID, domain)

		// Store queen agent reference
		if agentNum == 1 {
			qc.queenAgent = domainAgent
		}
	}

	return nil
}

// AnalyzeTask performs strategic analysis of a task.
func (qc *QueenCoordinator) AnalyzeTask(ctx context.Context, task shared.Task) (*shared.TaskAnalysis, error) {
	startTime := time.Now()
	defer func() {
		qc.mu.Lock()
		qc.tasksAnalyzed++
		qc.totalAnalysisTime += time.Since(startTime).Milliseconds()
		qc.mu.Unlock()
	}()

	analysis := &shared.TaskAnalysis{
		TaskID:   task.ID,
		Metadata: make(map[string]interface{}),
	}

	// Calculate complexity score
	analysis.ComplexityScore = qc.calculateComplexity(task)

	// Extract required capabilities
	analysis.RequiredCapabilities = qc.extractCapabilities(task)

	// Determine recommended domain
	analysis.RecommendedDomain = qc.recommendDomain(analysis.RequiredCapabilities)

	// Estimate duration based on complexity
	analysis.EstimatedDuration = qc.estimateDuration(analysis.ComplexityScore, task.Type)

	// Calculate parallelization score
	analysis.ParallelizationScore = qc.calculateParallelizationScore(task)

	// Pattern matching (if neural learning is available)
	if qc.config.EnablePatternMatching && qc.neuralLearning != nil {
		patterns, err := qc.neuralLearning.GetPatternMatches(ctx, task.Description, qc.config.PatternMatchK)
		if err == nil {
			analysis.PatternMatches = patterns
		}
	}

	return analysis, nil
}

// calculateComplexity calculates the complexity score for a task.
func (qc *QueenCoordinator) calculateComplexity(task shared.Task) float64 {
	complexity := 0.0

	// Base complexity from task type
	switch task.Type {
	case shared.TaskTypeWorkflow:
		complexity += 0.4
	case shared.TaskTypeDesign:
		complexity += 0.3
	case shared.TaskTypeCode:
		complexity += 0.25
	case shared.TaskTypeReview:
		complexity += 0.2
	case shared.TaskTypeTest:
		complexity += 0.15
	case shared.TaskTypeDeploy:
		complexity += 0.2
	}

	// Description length contributes to complexity
	descLen := len(task.Description)
	if descLen > 500 {
		complexity += 0.2
	} else if descLen > 200 {
		complexity += 0.1
	}

	// Priority affects perceived complexity
	switch task.Priority {
	case shared.PriorityHigh:
		complexity += 0.1
	case shared.PriorityMedium:
		complexity += 0.05
	}

	// Dependencies add complexity
	if len(task.Dependencies) > 0 {
		complexity += float64(len(task.Dependencies)) * 0.05
	}

	// Workflow tasks are inherently more complex
	if task.Workflow != nil {
		complexity += float64(len(task.Workflow.Tasks)) * 0.03
	}

	// Clamp to 0-1
	if complexity > 1.0 {
		complexity = 1.0
	}

	return complexity
}

// extractCapabilities extracts required capabilities from a task.
func (qc *QueenCoordinator) extractCapabilities(task shared.Task) []string {
	caps := make([]string, 0)

	// Map task type to capabilities
	switch task.Type {
	case shared.TaskTypeCode:
		caps = append(caps, "code", "develop", "implement")
	case shared.TaskTypeTest:
		caps = append(caps, "test", "validate", "qa")
	case shared.TaskTypeReview:
		caps = append(caps, "review", "analyze", "audit")
	case shared.TaskTypeDesign:
		caps = append(caps, "design", "architect", "plan")
	case shared.TaskTypeDeploy:
		caps = append(caps, "deploy", "release", "ops")
	case shared.TaskTypeWorkflow:
		caps = append(caps, "coordinate", "orchestrate", "manage")
	}

	// Extract from description keywords
	description := strings.ToLower(task.Description)
	
	if strings.Contains(description, "security") || strings.Contains(description, "cve") {
		caps = append(caps, "security-architecture", "security-testing")
	}
	if strings.Contains(description, "performance") || strings.Contains(description, "benchmark") {
		caps = append(caps, "performance-benchmarking")
	}
	if strings.Contains(description, "ddd") || strings.Contains(description, "domain-driven") {
		caps = append(caps, "ddd-design")
	}
	if strings.Contains(description, "memory") || strings.Contains(description, "agentdb") {
		caps = append(caps, "memory-unification")
	}
	if strings.Contains(description, "mcp") || strings.Contains(description, "protocol") {
		caps = append(caps, "mcp-optimization")
	}
	if strings.Contains(description, "neural") || strings.Contains(description, "learning") {
		caps = append(caps, "neural-integration")
	}
	if strings.Contains(description, "cli") || strings.Contains(description, "command") {
		caps = append(caps, "cli-modernization")
	}
	if strings.Contains(description, "tdd") || strings.Contains(description, "test-driven") {
		caps = append(caps, "tdd-testing")
	}

	// Check metadata for explicit capabilities
	if task.Metadata != nil {
		if reqCaps, ok := task.Metadata["requiredCapabilities"].([]string); ok {
			caps = append(caps, reqCaps...)
		}
	}

	return caps
}

// recommendDomain recommends the best domain for a set of capabilities.
func (qc *QueenCoordinator) recommendDomain(capabilities []string) shared.AgentDomain {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	bestDomain := shared.DomainCore // Default
	bestScore := 0.0

	for domain, pool := range qc.domains {
		if domain == shared.DomainQueen {
			continue // Queen doesn't execute tasks directly
		}

		score := pool.MatchCapabilities(capabilities)
		if score > bestScore {
			bestScore = score
			bestDomain = domain
		}
	}

	return bestDomain
}

// estimateDuration estimates task duration based on complexity.
func (qc *QueenCoordinator) estimateDuration(complexity float64, taskType shared.TaskType) int64 {
	// Base duration in milliseconds
	baseDuration := int64(5000) // 5 seconds

	// Adjust by task type
	switch taskType {
	case shared.TaskTypeWorkflow:
		baseDuration = 30000 // 30 seconds
	case shared.TaskTypeDesign:
		baseDuration = 20000 // 20 seconds
	case shared.TaskTypeCode:
		baseDuration = 15000 // 15 seconds
	case shared.TaskTypeReview:
		baseDuration = 10000 // 10 seconds
	case shared.TaskTypeTest:
		baseDuration = 10000 // 10 seconds
	case shared.TaskTypeDeploy:
		baseDuration = 15000 // 15 seconds
	}

	// Scale by complexity
	return int64(float64(baseDuration) * (1 + complexity))
}

// calculateParallelizationScore determines how parallelizable a task is.
func (qc *QueenCoordinator) calculateParallelizationScore(task shared.Task) float64 {
	// Workflow tasks have parallelization potential
	if task.Workflow != nil && len(task.Workflow.Tasks) > 1 {
		// Check for independent tasks
		independentTasks := 0
		for _, t := range task.Workflow.Tasks {
			if len(t.Dependencies) == 0 {
				independentTasks++
			}
		}
		return float64(independentTasks) / float64(len(task.Workflow.Tasks))
	}

	// Single tasks have low parallelization
	return 0.1
}

// DelegateTask delegates a task to the best available agent.
func (qc *QueenCoordinator) DelegateTask(ctx context.Context, task shared.Task) (*shared.DelegationResult, error) {
	startTime := time.Now()

	// First, analyze the task
	analysis, err := qc.AnalyzeTask(ctx, task)
	if err != nil {
		return nil, err
	}

	qc.mu.Lock()
	defer qc.mu.Unlock()

	// Get the recommended domain pool
	pool, ok := qc.domains[analysis.RecommendedDomain]
	if !ok {
		return nil, shared.NewCoordinationError(
			"recommended domain not found",
			map[string]interface{}{"domain": analysis.RecommendedDomain},
		)
	}

	// Get agent health for scoring
	agentHealth := qc.healthMonitor.GetAllAgentHealth()

	// Find best agent
	bestAgent, bestScore := pool.FindBestAgent(ctx, task, agentHealth)
	if bestAgent == nil {
		// Try other domains
		for domain, p := range qc.domains {
			if domain == analysis.RecommendedDomain || domain == shared.DomainQueen {
				continue
			}
			bestAgent, bestScore = p.FindBestAgent(ctx, task, agentHealth)
			if bestAgent != nil {
				pool = p
				break
			}
		}

		if bestAgent == nil {
			return nil, shared.NewCoordinationError(
				"no available agents for task",
				map[string]interface{}{"taskId": task.ID},
			)
		}
	}

	// Find backup agents
	backupAgents := qc.findBackupAgents(ctx, task, bestAgent.ID, pool, agentHealth)

	// Determine execution strategy
	strategy := qc.determineStrategy(analysis, pool)

	result := &shared.DelegationResult{
		TaskID:        task.ID,
		PrimaryAgent:  *bestScore,
		BackupAgents:  backupAgents,
		Domain:        pool.Config.Name,
		Strategy:      strategy,
		EstimatedTime: analysis.EstimatedDuration,
	}

	qc.tasksDelegated++

	// Check performance target
	elapsed := time.Since(startTime)
	if elapsed > qc.config.MaxCoordinationTime {
		result.EstimatedTime += elapsed.Milliseconds() // Adjust estimate
	}

	return result, nil
}

// findBackupAgents finds backup agents for a task.
func (qc *QueenCoordinator) findBackupAgents(ctx context.Context, task shared.Task, excludeID string, pool *DomainPool, agentHealth map[string]*shared.AgentHealth) []shared.AgentScore {
	backups := make([]shared.AgentScore, 0)

	agents := pool.ListAgents()
	scores := make([]shared.AgentScore, 0)

	for _, a := range agents {
		if a.ID == excludeID {
			continue
		}

		sharedAgent := a.ToShared()
		if sharedAgent.Status == shared.AgentStatusTerminated || sharedAgent.Status == shared.AgentStatusError {
			continue
		}

		score := qc.scoreAgentForTask(a, task, agentHealth)
		score.CalculateTotalScore()
		scores = append(scores, *score)
	}

	// Sort by total score (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].TotalScore > scores[j].TotalScore
	})

	// Take top N backups
	for i := 0; i < qc.config.BackupAgentCount && i < len(scores); i++ {
		backups = append(backups, scores[i])
	}

	return backups
}

// scoreAgentForTask scores an agent for a specific task.
func (qc *QueenCoordinator) scoreAgentForTask(a *agent.Agent, task shared.Task, agentHealth map[string]*shared.AgentHealth) *shared.AgentScore {
	score := &shared.AgentScore{
		AgentID: a.ID,
	}

	// Capability score
	requiredCaps := qc.extractCapabilities(task)
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

	// Load score
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

	// Performance score (default)
	score.PerformanceScore = 0.8

	// Health score
	if health, ok := agentHealth[a.ID]; ok {
		score.HealthScore = health.HealthScore
	} else {
		score.HealthScore = 1.0
	}

	return score
}

// determineStrategy determines the execution strategy for a task.
func (qc *QueenCoordinator) determineStrategy(analysis *shared.TaskAnalysis, pool *DomainPool) shared.ExecutionStrategy {
	// High parallelization score and multiple agents = parallel
	if analysis.ParallelizationScore > 0.5 && pool.GetActiveAgentCount() > 1 {
		return shared.StrategyParallel
	}

	// Complex tasks with dependencies = pipeline
	if analysis.ComplexityScore > 0.6 && len(analysis.RequiredCapabilities) > 3 {
		return shared.StrategyPipeline
	}

	// Default to sequential
	return shared.StrategySequential
}

// ExecuteTask executes a task through delegation.
func (qc *QueenCoordinator) ExecuteTask(ctx context.Context, task shared.Task) (*shared.TaskResult, error) {
	// Delegate the task
	delegation, err := qc.DelegateTask(ctx, task)
	if err != nil {
		return nil, err
	}

	// Execute through swarm coordinator
	result := qc.swarm.ExecuteTask(delegation.PrimaryAgent.AgentID, task)

	// Record completion in health monitor
	qc.healthMonitor.RecordTaskCompletion(
		delegation.PrimaryAgent.AgentID,
		result.Duration,
		result.Status == shared.TaskStatusCompleted,
	)

	// Learn from outcome (if neural learning is available)
	if qc.neuralLearning != nil {
		go func() {
			_ = qc.neuralLearning.LearnFromOutcome(context.Background(), task.ID, *result)
		}()
	}

	return result, nil
}

// ExecuteTasksParallel executes multiple tasks in parallel across domains.
func (qc *QueenCoordinator) ExecuteTasksParallel(ctx context.Context, tasks []shared.Task) ([]shared.TaskResult, error) {
	results := make([]shared.TaskResult, len(tasks))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t shared.Task) {
			defer wg.Done()

			result, err := qc.ExecuteTask(ctx, t)
			mu.Lock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if result != nil {
				results[idx] = *result
			} else {
				results[idx] = shared.TaskResult{
					TaskID: t.ID,
					Status: shared.TaskStatusFailed,
					Error:  err.Error(),
				}
			}
			mu.Unlock()
		}(i, task)
	}

	wg.Wait()
	return results, firstErr
}

// GetDomainHealth returns the health status of all domains.
func (qc *QueenCoordinator) GetDomainHealth() map[shared.AgentDomain]*shared.DomainHealth {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	health := make(map[shared.AgentDomain]*shared.DomainHealth)

	for domain, pool := range qc.domains {
		domainHealth := pool.GetHealth()
		health[domain] = &domainHealth

		// Update in health monitor
		qc.healthMonitor.UpdateDomainHealth(domain, domainHealth)
	}

	return health
}

// GetAgentsByDomain returns all agents in a specific domain.
func (qc *QueenCoordinator) GetAgentsByDomain(domain shared.AgentDomain) []*agent.Agent {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	pool, ok := qc.domains[domain]
	if !ok {
		return nil
	}

	return pool.ListAgents()
}

// GetDomainPool returns the domain pool for a specific domain.
func (qc *QueenCoordinator) GetDomainPool(domain shared.AgentDomain) (*DomainPool, bool) {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	pool, ok := qc.domains[domain]
	return pool, ok
}

// GetHealthMonitor returns the health monitor.
func (qc *QueenCoordinator) GetHealthMonitor() *HealthMonitor {
	return qc.healthMonitor
}

// GetSwarm returns the underlying swarm coordinator.
func (qc *QueenCoordinator) GetSwarm() *SwarmCoordinator {
	return qc.swarm
}

// GetQueen returns the Queen agent.
func (qc *QueenCoordinator) GetQueen() *agent.Agent {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return qc.queenAgent
}

// GetMetrics returns metrics about the Queen Coordinator.
func (qc *QueenCoordinator) GetMetrics() QueenMetrics {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	avgAnalysisTime := float64(0)
	if qc.tasksAnalyzed > 0 {
		avgAnalysisTime = float64(qc.totalAnalysisTime) / float64(qc.tasksAnalyzed)
	}

	return QueenMetrics{
		TasksAnalyzed:       qc.tasksAnalyzed,
		TasksDelegated:      qc.tasksDelegated,
		AvgAnalysisTime:     avgAnalysisTime,
		DomainCount:         len(qc.domains),
		TotalAgents:         qc.countTotalAgents(),
	}
}

// countTotalAgents counts the total number of agents across all domains.
func (qc *QueenCoordinator) countTotalAgents() int {
	total := 0
	for _, pool := range qc.domains {
		total += pool.GetAgentCount()
	}
	return total
}

// QueenMetrics represents metrics for the Queen Coordinator.
type QueenMetrics struct {
	TasksAnalyzed   int     `json:"tasksAnalyzed"`
	TasksDelegated  int     `json:"tasksDelegated"`
	AvgAnalysisTime float64 `json:"avgAnalysisTime"` // milliseconds
	DomainCount     int     `json:"domainCount"`
	TotalAgents     int     `json:"totalAgents"`
}

// ReachConsensus coordinates a consensus decision across agents.
func (qc *QueenCoordinator) ReachConsensus(ctx context.Context, decision shared.ConsensusDecision, consensusType ConsensusType) (*shared.ConsensusResult, error) {
	// Get all active agents
	var agents []*agent.Agent
	for _, pool := range qc.domains {
		agents = append(agents, pool.GetAvailableAgents()...)
	}

	if len(agents) == 0 {
		return nil, shared.NewCoordinationError("no agents available for consensus", nil)
	}

	// Collect votes
	votes := make([]shared.AgentVote, 0, len(agents))
	for _, a := range agents {
		// Simulate voting (in real implementation, this would query the agent)
		votes = append(votes, shared.AgentVote{
			AgentID: a.ID,
			Vote:    true, // Simplified: assume agreement
		})
	}

	// Calculate threshold based on consensus type
	threshold := qc.getConsensusThreshold(consensusType, len(votes))

	// Count approvals
	approvals := 0
	for _, v := range votes {
		if approved, ok := v.Vote.(bool); ok && approved {
			approvals++
		}
	}

	consensusReached := approvals >= threshold

	return &shared.ConsensusResult{
		Decision:         decision.Payload,
		Votes:            votes,
		ConsensusReached: consensusReached,
	}, nil
}

// ConsensusType represents the type of consensus mechanism.
type ConsensusType string

const (
	ConsensusMajority     ConsensusType = "majority"
	ConsensusSuperMajority ConsensusType = "supermajority"
	ConsensusUnanimous    ConsensusType = "unanimous"
	ConsensusWeighted     ConsensusType = "weighted"
	ConsensusQueenOverride ConsensusType = "queen-override"
)

// getConsensusThreshold returns the required votes for consensus.
func (qc *QueenCoordinator) getConsensusThreshold(consensusType ConsensusType, totalVotes int) int {
	switch consensusType {
	case ConsensusMajority:
		return (totalVotes / 2) + 1
	case ConsensusSuperMajority:
		return int(float64(totalVotes) * 0.67)
	case ConsensusUnanimous:
		return totalVotes
	case ConsensusQueenOverride:
		return 1 // Queen can override
	default:
		return (totalVotes / 2) + 1
	}
}

// AssignTaskToDomain assigns a task directly to a specific domain.
func (qc *QueenCoordinator) AssignTaskToDomain(ctx context.Context, task shared.Task, domain shared.AgentDomain) error {
	qc.mu.RLock()
	pool, ok := qc.domains[domain]
	qc.mu.RUnlock()

	if !ok {
		return shared.NewValidationError("domain not found", map[string]interface{}{
			"domain": domain,
		})
	}

	return pool.SubmitTask(task)
}

// BroadcastMessage broadcasts a message to all agents in a domain.
func (qc *QueenCoordinator) BroadcastMessage(ctx context.Context, domain shared.AgentDomain, message shared.AgentMessage) error {
	qc.mu.RLock()
	pool, ok := qc.domains[domain]
	qc.mu.RUnlock()

	if !ok {
		return shared.NewValidationError("domain not found", map[string]interface{}{
			"domain": domain,
		})
	}

	agents := pool.ListAgents()
	for _, a := range agents {
		msg := message
		msg.To = a.ID
		msg.Timestamp = shared.Now()
		
		// Send through swarm coordinator
		_ = qc.swarm.SendMessage(msg)
	}

	return nil
}

// ScoreAgentsForTask scores all available agents for a task.
func (qc *QueenCoordinator) ScoreAgentsForTask(ctx context.Context, task shared.Task) []shared.AgentScore {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	scores := make([]shared.AgentScore, 0)
	agentHealth := qc.healthMonitor.GetAllAgentHealth()

	for _, pool := range qc.domains {
		for _, a := range pool.ListAgents() {
			score := qc.scoreAgentForTask(a, task, agentHealth)
			score.CalculateTotalScore()
			scores = append(scores, *score)
		}
	}

	// Sort by total score (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].TotalScore > scores[j].TotalScore
	})

	return scores
}

// GetAlerts returns recent health alerts.
func (qc *QueenCoordinator) GetAlerts(limit int) []shared.HealthAlert {
	return qc.healthMonitor.GetAlerts(limit)
}

// DetectBottlenecks detects bottlenecks in the system.
func (qc *QueenCoordinator) DetectBottlenecks() []BottleneckInfo {
	return qc.healthMonitor.DetectBottlenecks()
}

// GenerateAgentID generates a unique agent ID.
func GenerateAgentID() string {
	return uuid.New().String()
}
