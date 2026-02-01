// Package claudeflow provides the public API for claude-flow-go.
//
// This package provides a high-level interface for creating and managing
// multi-agent swarms, executing workflows, and integrating with the MCP protocol.
//
// Example:
//
//	coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
//	    Topology: claudeflow.TopologyMesh,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer coordinator.Shutdown()
//
//	agent, err := coordinator.SpawnAgent(claudeflow.AgentConfig{
//	    ID:   "coder-1",
//	    Type: claudeflow.AgentTypeCoder,
//	})
package claudeflow

import (
	"context"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/application/hivemind"
	"github.com/anthropics/claude-flow-go/internal/application/workflow"
	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/events"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/tools"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/memory"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/plugins"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Re-export types for public API
type (
	// Agent types
	AgentStatus = shared.AgentStatus
	AgentRole   = shared.AgentRole
	AgentType   = shared.AgentType
	AgentConfig = shared.AgentConfig
	Agent       = shared.Agent

	// 15-Agent Domain Architecture types
	AgentDomain       = shared.AgentDomain
	DomainConfig      = shared.DomainConfig
	AgentScore        = shared.AgentScore
	TaskAnalysis      = shared.TaskAnalysis
	DelegationResult  = shared.DelegationResult
	ExecutionStrategy = shared.ExecutionStrategy
	DomainHealth      = shared.DomainHealth
	AgentHealth       = shared.AgentHealth
	HealthAlert       = shared.HealthAlert
	AlertLevel        = shared.AlertLevel
	DomainMetrics     = shared.DomainMetrics

	// Hive Mind Consensus types
	ConsensusType    = shared.ConsensusType
	ProposalStatus   = shared.ProposalStatus
	Proposal         = shared.Proposal
	WeightedVote     = shared.WeightedVote
	ProposalResult   = shared.ProposalResult
	HiveMindConfig   = shared.HiveMindConfig
	HiveMindState    = shared.HiveMindState
	ProposalOutcome  = shared.ProposalOutcome

	// Task types
	TaskPriority = shared.TaskPriority
	TaskStatus   = shared.TaskStatus
	TaskType     = shared.TaskType
	Task         = shared.Task
	TaskResult   = shared.TaskResult

	// Memory types
	MemoryType         = shared.MemoryType
	Memory             = shared.Memory
	MemoryQuery        = shared.MemoryQuery
	MemorySearchResult = shared.MemorySearchResult

	// Workflow types
	WorkflowStatus     = shared.WorkflowStatus
	WorkflowDefinition = shared.WorkflowDefinition
	WorkflowState      = shared.WorkflowState
	WorkflowResult     = shared.WorkflowResult
	WorkflowMetrics    = shared.WorkflowMetrics

	// Swarm types
	SwarmTopology = shared.SwarmTopology
	SwarmConfig   = shared.SwarmConfig
	SwarmState    = shared.SwarmState
	AgentMessage  = shared.AgentMessage
	AgentMetrics  = shared.AgentMetrics

	// Plugin types
	Plugin           = shared.Plugin
	ExtensionPoint   = shared.ExtensionPoint
	PluginMetadata   = shared.PluginMetadata
	PluginManager    = shared.PluginManager

	// MCP types
	MCPTool         = shared.MCPTool
	MCPToolProvider = shared.MCPToolProvider
	MCPToolResult   = shared.MCPToolResult
	MCPRequest      = shared.MCPRequest
	MCPResponse     = shared.MCPResponse

	// Backend types
	MemoryBackend = shared.MemoryBackend

	// Event types
	EventType = shared.EventType
	Event     = shared.Event
)

// Agent status constants
const (
	AgentStatusActive     = shared.AgentStatusActive
	AgentStatusIdle       = shared.AgentStatusIdle
	AgentStatusBusy       = shared.AgentStatusBusy
	AgentStatusTerminated = shared.AgentStatusTerminated
	AgentStatusError      = shared.AgentStatusError
)

// Agent role constants
const (
	AgentRoleLeader = shared.AgentRoleLeader
	AgentRoleWorker = shared.AgentRoleWorker
	AgentRolePeer   = shared.AgentRolePeer
)

// Agent type constants
const (
	AgentTypeCoder       = shared.AgentTypeCoder
	AgentTypeTester      = shared.AgentTypeTester
	AgentTypeReviewer    = shared.AgentTypeReviewer
	AgentTypeCoordinator = shared.AgentTypeCoordinator
	AgentTypeDesigner    = shared.AgentTypeDesigner
	AgentTypeDeployer    = shared.AgentTypeDeployer

	// 15-Agent Domain Architecture Types
	AgentTypeQueen             = shared.AgentTypeQueen
	AgentTypeSecurityArchitect = shared.AgentTypeSecurityArchitect
	AgentTypeCVERemediation    = shared.AgentTypeCVERemediation
	AgentTypeThreatModeler     = shared.AgentTypeThreatModeler
	AgentTypeDDDDesigner       = shared.AgentTypeDDDDesigner
	AgentTypeMemorySpecialist  = shared.AgentTypeMemorySpecialist
	AgentTypeTypeModernizer    = shared.AgentTypeTypeModernizer
	AgentTypeSwarmSpecialist   = shared.AgentTypeSwarmSpecialist
	AgentTypeMCPOptimizer      = shared.AgentTypeMCPOptimizer
	AgentTypeAgenticFlow       = shared.AgentTypeAgenticFlow
	AgentTypeCLIDeveloper      = shared.AgentTypeCLIDeveloper
	AgentTypeNeuralIntegrator  = shared.AgentTypeNeuralIntegrator
	AgentTypeTDDTester         = shared.AgentTypeTDDTester
	AgentTypePerformanceEngineer = shared.AgentTypePerformanceEngineer
	AgentTypeReleaseManager    = shared.AgentTypeReleaseManager
)

// Domain constants
const (
	DomainQueen       = shared.DomainQueen
	DomainSecurity    = shared.DomainSecurity
	DomainCore        = shared.DomainCore
	DomainIntegration = shared.DomainIntegration
	DomainSupport     = shared.DomainSupport
)

// Execution strategy constants
const (
	StrategySequential  = shared.StrategySequential
	StrategyParallel    = shared.StrategyParallel
	StrategyPipeline    = shared.StrategyPipeline
	StrategyFanOutFanIn = shared.StrategyFanOutFanIn
	StrategyHybrid      = shared.StrategyHybrid
)

// Alert level constants
const (
	AlertLevelInfo     = shared.AlertLevelInfo
	AlertLevelWarning  = shared.AlertLevelWarning
	AlertLevelCritical = shared.AlertLevelCritical
)

// Task priority constants
const (
	PriorityHigh   = shared.PriorityHigh
	PriorityMedium = shared.PriorityMedium
	PriorityLow    = shared.PriorityLow
)

// Task status constants
const (
	TaskStatusPending    = shared.TaskStatusPending
	TaskStatusInProgress = shared.TaskStatusInProgress
	TaskStatusCompleted  = shared.TaskStatusCompleted
	TaskStatusFailed     = shared.TaskStatusFailed
	TaskStatusCancelled  = shared.TaskStatusCancelled
)

// Task type constants
const (
	TaskTypeCode     = shared.TaskTypeCode
	TaskTypeTest     = shared.TaskTypeTest
	TaskTypeReview   = shared.TaskTypeReview
	TaskTypeDesign   = shared.TaskTypeDesign
	TaskTypeDeploy   = shared.TaskTypeDeploy
	TaskTypeWorkflow = shared.TaskTypeWorkflow
)

// Memory type constants
const (
	MemoryTypeTask         = shared.MemoryTypeTask
	MemoryTypeContext      = shared.MemoryTypeContext
	MemoryTypeEvent        = shared.MemoryTypeEvent
	MemoryTypeTaskStart    = shared.MemoryTypeTaskStart
	MemoryTypeTaskComplete = shared.MemoryTypeTaskComplete
	MemoryTypeWorkflow     = shared.MemoryTypeWorkflow
)

// Topology constants
const (
	TopologyHierarchical = shared.TopologyHierarchical
	TopologyMesh         = shared.TopologyMesh
	TopologySimple       = shared.TopologySimple
	TopologyAdaptive     = shared.TopologyAdaptive
)

// SwarmCoordinator wraps the internal coordinator for public use.
type SwarmCoordinator struct {
	internal *coordinator.SwarmCoordinator
}

// NewSwarmCoordinator creates a new SwarmCoordinator.
func NewSwarmCoordinator(config SwarmConfig) (*SwarmCoordinator, error) {
	eventBus := events.New()

	opts := coordinator.Options{
		Topology:      config.Topology,
		MemoryBackend: config.MemoryBackend,
		EventBus:      eventBus,
	}

	coord := coordinator.New(opts)
	if err := coord.Initialize(); err != nil {
		return nil, err
	}

	return &SwarmCoordinator{internal: coord}, nil
}

// SpawnAgent spawns a new agent.
func (sc *SwarmCoordinator) SpawnAgent(config AgentConfig) (*Agent, error) {
	a, err := sc.internal.SpawnAgent(config)
	if err != nil {
		return nil, err
	}
	agentData := a.ToShared()
	return &agentData, nil
}

// ListAgents returns all agents.
func (sc *SwarmCoordinator) ListAgents() []Agent {
	agents := sc.internal.ListAgents()
	result := make([]Agent, len(agents))
	for i, a := range agents {
		result[i] = a.ToShared()
	}
	return result
}

// TerminateAgent terminates an agent.
func (sc *SwarmCoordinator) TerminateAgent(agentID string) error {
	return sc.internal.TerminateAgent(agentID)
}

// ExecuteTask executes a task on an agent.
func (sc *SwarmCoordinator) ExecuteTask(agentID string, task Task) (TaskResult, error) {
	return sc.internal.ExecuteTask(context.Background(), agentID, task)
}

// ExecuteTaskWithContext executes a task with context.
func (sc *SwarmCoordinator) ExecuteTaskWithContext(ctx context.Context, agentID string, task Task) (TaskResult, error) {
	return sc.internal.ExecuteTask(ctx, agentID, task)
}

// GetSwarmState returns the current swarm state.
func (sc *SwarmCoordinator) GetSwarmState() SwarmState {
	return sc.internal.GetSwarmState()
}

// GetAgentMetrics returns metrics for an agent.
func (sc *SwarmCoordinator) GetAgentMetrics(agentID string) AgentMetrics {
	return sc.internal.GetAgentMetrics(agentID)
}

// Reconfigure reconfigures the swarm topology.
func (sc *SwarmCoordinator) Reconfigure(topology SwarmTopology) error {
	return sc.internal.Reconfigure(topology)
}

// Shutdown shuts down the coordinator.
func (sc *SwarmCoordinator) Shutdown() error {
	return sc.internal.Shutdown()
}

// Internal returns the internal coordinator (for advanced usage).
func (sc *SwarmCoordinator) Internal() *coordinator.SwarmCoordinator {
	return sc.internal
}

// WorkflowEngine wraps the internal workflow engine for public use.
type WorkflowEngine struct {
	internal *workflow.Engine
}

// WorkflowEngineConfig holds configuration for creating a WorkflowEngine.
type WorkflowEngineConfig struct {
	Coordinator   *SwarmCoordinator
	MemoryBackend MemoryBackend
}

// NewWorkflowEngine creates a new WorkflowEngine.
func NewWorkflowEngine(config WorkflowEngineConfig) (*WorkflowEngine, error) {
	opts := workflow.Options{
		Coordinator:   config.Coordinator.internal,
		MemoryBackend: config.MemoryBackend,
	}

	engine := workflow.NewEngine(opts)
	if err := engine.Initialize(); err != nil {
		return nil, err
	}

	return &WorkflowEngine{internal: engine}, nil
}

// ExecuteWorkflow executes a workflow.
func (we *WorkflowEngine) ExecuteWorkflow(ctx context.Context, workflow WorkflowDefinition) (WorkflowResult, error) {
	return we.internal.ExecuteWorkflow(ctx, workflow)
}

// GetWorkflowState returns the workflow state.
func (we *WorkflowEngine) GetWorkflowState(workflowID string) (WorkflowState, error) {
	return we.internal.GetWorkflowState(workflowID)
}

// GetWorkflowMetrics returns workflow metrics.
func (we *WorkflowEngine) GetWorkflowMetrics(workflowID string) (WorkflowMetrics, error) {
	return we.internal.GetWorkflowMetrics(workflowID)
}

// PauseWorkflow pauses a workflow.
func (we *WorkflowEngine) PauseWorkflow(workflowID string) error {
	return we.internal.PauseWorkflow(workflowID)
}

// ResumeWorkflow resumes a workflow.
func (we *WorkflowEngine) ResumeWorkflow(workflowID string) error {
	return we.internal.ResumeWorkflow(workflowID)
}

// CancelWorkflow cancels a workflow.
func (we *WorkflowEngine) CancelWorkflow(workflowID string) error {
	return we.internal.CancelWorkflow(workflowID)
}

// Shutdown shuts down the workflow engine.
func (we *WorkflowEngine) Shutdown() error {
	return we.internal.Shutdown()
}

// MCPServer wraps the internal MCP server for public use.
type MCPServer struct {
	internal *mcp.Server
}

// MCPServerConfig holds configuration for creating an MCP server.
type MCPServerConfig struct {
	Port        int
	Host        string
	Coordinator *SwarmCoordinator
	Memory      MemoryBackend
}

// NewMCPServer creates a new MCP server.
func NewMCPServer(config MCPServerConfig) *MCPServer {
	providers := make([]shared.MCPToolProvider, 0)

	if config.Coordinator != nil {
		providers = append(providers, tools.NewAgentTools(config.Coordinator.internal))
		providers = append(providers, tools.NewConfigTools(config.Coordinator.internal))
	}
	if config.Memory != nil {
		providers = append(providers, tools.NewMemoryTools(config.Memory))
	}

	opts := mcp.Options{
		Tools: providers,
		Port:  config.Port,
		Host:  config.Host,
	}

	return &MCPServer{internal: mcp.NewServer(opts)}
}

// Start starts the MCP server.
func (ms *MCPServer) Start() error {
	return ms.internal.Start()
}

// Stop stops the MCP server.
func (ms *MCPServer) Stop() error {
	return ms.internal.Stop()
}

// ListTools returns available tools.
func (ms *MCPServer) ListTools() []MCPTool {
	return ms.internal.ListTools()
}

// GetStatus returns server status.
func (ms *MCPServer) GetStatus() map[string]interface{} {
	return ms.internal.GetStatus()
}

// MemoryBackendConfig holds configuration for memory backends.
type MemoryBackendConfig struct {
	SQLitePath string
	Dimensions int
}

// NewMemoryBackend creates a new hybrid memory backend.
func NewMemoryBackend(config MemoryBackendConfig) (MemoryBackend, error) {
	dbPath := config.SQLitePath
	if dbPath == "" {
		dbPath = ":memory:"
	}

	sqliteBackend := memory.NewSQLiteBackend(dbPath, memory.WithInMemoryFallback())
	agentDBBackend := memory.NewAgentDBBackend(dbPath, memory.WithDimensions(config.Dimensions))

	hybrid := memory.NewHybridBackend(sqliteBackend, agentDBBackend)
	if err := hybrid.Initialize(); err != nil {
		return nil, err
	}

	return hybrid, nil
}

// NewSQLiteBackend creates a new SQLite memory backend.
func NewSQLiteBackend(dbPath string) (MemoryBackend, error) {
	backend := memory.NewSQLiteBackend(dbPath, memory.WithInMemoryFallback())
	if err := backend.Initialize(); err != nil {
		return nil, err
	}
	return backend, nil
}

// PluginManagerConfig holds configuration for the plugin manager.
type PluginManagerConfig struct {
	CoreVersion string
}

// NewPluginManager creates a new plugin manager.
func NewPluginManager(config PluginManagerConfig) (PluginManager, error) {
	opts := plugins.Options{
		CoreVersion: config.CoreVersion,
	}

	manager := plugins.NewManager(opts)
	if err := manager.Initialize(); err != nil {
		return nil, err
	}

	return manager, nil
}

// GetDefaultCapabilities returns default capabilities for an agent type.
func GetDefaultCapabilities(agentType AgentType) []string {
	return agent.GetDefaultCapabilities(agentType)
}

// Now returns the current time in milliseconds.
func Now() int64 {
	return shared.Now()
}

// DefaultDomainConfigs returns the default domain configurations for the 15-agent architecture.
func DefaultDomainConfigs() []DomainConfig {
	return shared.DefaultDomainConfigs()
}

// GetDomainForAgentNumber returns the domain for a given agent number (1-15).
func GetDomainForAgentNumber(agentNumber int) AgentDomain {
	return shared.GetDomainForAgentNumber(agentNumber)
}

// GetAgentTypeForNumber returns the default agent type for a given agent number (1-15).
func GetAgentTypeForNumber(agentNumber int) AgentType {
	return shared.GetAgentTypeForNumber(agentNumber)
}

// ============================================================================
// Queen Coordinator (15-Agent Domain Architecture)
// ============================================================================

// QueenCoordinator wraps the internal Queen Coordinator for public use.
type QueenCoordinator struct {
	internal *coordinator.QueenCoordinator
}

// QueenConfig holds configuration for the Queen Coordinator.
type QueenConfig = coordinator.QueenConfig

// DefaultQueenConfig returns the default Queen Coordinator configuration.
func DefaultQueenConfig() QueenConfig {
	return coordinator.DefaultQueenConfig()
}

// NewQueenCoordinator creates a new Queen Coordinator.
func NewQueenCoordinator(swarm *SwarmCoordinator, config QueenConfig) (*QueenCoordinator, error) {
	qc, err := coordinator.NewQueenCoordinator(swarm.internal, config)
	if err != nil {
		return nil, err
	}
	return &QueenCoordinator{internal: qc}, nil
}

// Start starts the Queen Coordinator and its health monitoring.
func (qc *QueenCoordinator) Start() {
	qc.internal.Start()
}

// Stop stops the Queen Coordinator.
func (qc *QueenCoordinator) Stop() {
	qc.internal.Stop()
}

// SpawnFullHierarchy spawns the complete 15-agent hierarchy.
func (qc *QueenCoordinator) SpawnFullHierarchy(ctx context.Context) error {
	return qc.internal.SpawnFullHierarchy(ctx)
}

// AnalyzeTask performs strategic analysis of a task.
func (qc *QueenCoordinator) AnalyzeTask(ctx context.Context, task Task) (*TaskAnalysis, error) {
	return qc.internal.AnalyzeTask(ctx, task)
}

// DelegateTask delegates a task to the best available agent.
func (qc *QueenCoordinator) DelegateTask(ctx context.Context, task Task) (*DelegationResult, error) {
	return qc.internal.DelegateTask(ctx, task)
}

// ExecuteTask executes a task through delegation.
func (qc *QueenCoordinator) ExecuteTask(ctx context.Context, task Task) (*TaskResult, error) {
	return qc.internal.ExecuteTask(ctx, task)
}

// ExecuteTasksParallel executes multiple tasks in parallel across domains.
func (qc *QueenCoordinator) ExecuteTasksParallel(ctx context.Context, tasks []Task) ([]TaskResult, error) {
	return qc.internal.ExecuteTasksParallel(ctx, tasks)
}

// GetDomainHealth returns the health status of all domains.
func (qc *QueenCoordinator) GetDomainHealth() map[AgentDomain]*DomainHealth {
	return qc.internal.GetDomainHealth()
}

// GetAgentsByDomain returns all agents in a specific domain.
func (qc *QueenCoordinator) GetAgentsByDomain(domain AgentDomain) []Agent {
	agents := qc.internal.GetAgentsByDomain(domain)
	result := make([]Agent, len(agents))
	for i, a := range agents {
		result[i] = a.ToShared()
	}
	return result
}

// ScoreAgentsForTask scores all available agents for a task.
func (qc *QueenCoordinator) ScoreAgentsForTask(ctx context.Context, task Task) []AgentScore {
	return qc.internal.ScoreAgentsForTask(ctx, task)
}

// ReachConsensus coordinates a consensus decision across agents.
func (qc *QueenCoordinator) ReachConsensus(ctx context.Context, decision shared.ConsensusDecision, consensusType string) (*shared.ConsensusResult, error) {
	return qc.internal.ReachConsensus(ctx, decision, coordinator.ConsensusType(consensusType))
}

// AssignTaskToDomain assigns a task directly to a specific domain.
func (qc *QueenCoordinator) AssignTaskToDomain(ctx context.Context, task Task, domain AgentDomain) error {
	return qc.internal.AssignTaskToDomain(ctx, task, domain)
}

// BroadcastMessage broadcasts a message to all agents in a domain.
func (qc *QueenCoordinator) BroadcastMessage(ctx context.Context, domain AgentDomain, message AgentMessage) error {
	return qc.internal.BroadcastMessage(ctx, domain, message)
}

// GetAlerts returns recent health alerts.
func (qc *QueenCoordinator) GetAlerts(limit int) []HealthAlert {
	return qc.internal.GetAlerts(limit)
}

// DetectBottlenecks detects bottlenecks in the system.
func (qc *QueenCoordinator) DetectBottlenecks() []coordinator.BottleneckInfo {
	return qc.internal.DetectBottlenecks()
}

// GetMetrics returns metrics about the Queen Coordinator.
func (qc *QueenCoordinator) GetMetrics() coordinator.QueenMetrics {
	return qc.internal.GetMetrics()
}

// GetSwarm returns the underlying swarm coordinator.
func (qc *QueenCoordinator) GetSwarm() *SwarmCoordinator {
	return &SwarmCoordinator{internal: qc.internal.GetSwarm()}
}

// Consensus type constants
const (
	ConsensusMajority      = string(coordinator.ConsensusMajority)
	ConsensusSuperMajority = string(coordinator.ConsensusSuperMajority)
	ConsensusUnanimous     = string(coordinator.ConsensusUnanimous)
	ConsensusWeighted      = string(coordinator.ConsensusWeighted)
	ConsensusQueenOverride = string(coordinator.ConsensusQueenOverride)
)

// Hive Mind consensus type constants
const (
	ConsensusTypeMajority      = shared.ConsensusTypeMajority
	ConsensusTypeSuperMajority = shared.ConsensusTypeSuperMajority
	ConsensusTypeUnanimous     = shared.ConsensusTypeUnanimous
	ConsensusTypeWeighted      = shared.ConsensusTypeWeighted
	ConsensusTypeQueenOverride = shared.ConsensusTypeQueenOverride
)

// Proposal status constants
const (
	ProposalStatusPending   = shared.ProposalStatusPending
	ProposalStatusVoting    = shared.ProposalStatusVoting
	ProposalStatusApproved  = shared.ProposalStatusApproved
	ProposalStatusRejected  = shared.ProposalStatusRejected
	ProposalStatusExpired   = shared.ProposalStatusExpired
	ProposalStatusCancelled = shared.ProposalStatusCancelled
)

// ============================================================================
// Hive Mind Manager (Consensus System)
// ============================================================================

// HiveMindManager wraps the internal Hive Mind Manager for public use.
type HiveMindManager struct {
	internal *hivemind.Manager
}

// DefaultHiveMindConfig returns the default Hive Mind configuration.
func DefaultHiveMindConfig() HiveMindConfig {
	return shared.DefaultHiveMindConfig()
}

// NewHiveMindManager creates a new Hive Mind Manager.
func NewHiveMindManager(queen *QueenCoordinator, config HiveMindConfig) (*HiveMindManager, error) {
	m, err := hivemind.NewManager(queen.internal, config)
	if err != nil {
		return nil, err
	}
	return &HiveMindManager{internal: m}, nil
}

// Initialize initializes the Hive Mind system.
func (hm *HiveMindManager) Initialize(ctx context.Context) error {
	return hm.internal.Initialize(ctx)
}

// Shutdown shuts down the Hive Mind Manager.
func (hm *HiveMindManager) Shutdown() error {
	return hm.internal.Shutdown()
}

// CreateProposal creates a new proposal for voting.
func (hm *HiveMindManager) CreateProposal(ctx context.Context, proposal Proposal) (*Proposal, error) {
	return hm.internal.CreateProposal(ctx, proposal)
}

// Vote submits a vote for a proposal.
func (hm *HiveMindManager) Vote(ctx context.Context, vote WeightedVote) error {
	return hm.internal.Vote(ctx, vote)
}

// GetProposal returns a proposal by ID.
func (hm *HiveMindManager) GetProposal(proposalID string) (*Proposal, bool) {
	return hm.internal.GetProposal(proposalID)
}

// GetProposalResult returns the result of a proposal.
func (hm *HiveMindManager) GetProposalResult(proposalID string) (*ProposalResult, bool) {
	return hm.internal.GetProposalResult(proposalID)
}

// GetProposalVotes returns the votes for a proposal.
func (hm *HiveMindManager) GetProposalVotes(proposalID string) []WeightedVote {
	return hm.internal.GetProposalVotes(proposalID)
}

// ListProposals returns all proposals with optional status filter.
func (hm *HiveMindManager) ListProposals(status ProposalStatus) []*Proposal {
	return hm.internal.ListProposals(status)
}

// CancelProposal cancels a pending proposal.
func (hm *HiveMindManager) CancelProposal(ctx context.Context, proposalID string, reason string) error {
	return hm.internal.CancelProposal(ctx, proposalID, reason)
}

// CollectVotesSync synchronously collects votes from all agents.
func (hm *HiveMindManager) CollectVotesSync(ctx context.Context, proposalID string) (*ProposalResult, error) {
	return hm.internal.CollectVotesSync(ctx, proposalID)
}

// GetState returns the current state of the Hive Mind.
func (hm *HiveMindManager) GetState() HiveMindState {
	return hm.internal.GetState()
}

// GetQueen returns the Queen Coordinator.
func (hm *HiveMindManager) GetQueen() *QueenCoordinator {
	return &QueenCoordinator{internal: hm.internal.GetQueen()}
}

// GetLearningStats returns learning statistics.
func (hm *HiveMindManager) GetLearningStats() hivemind.LearningStats {
	return hm.internal.GetLearning().GetLearningStats()
}

// PredictSuccess predicts the success probability for a proposal.
func (hm *HiveMindManager) PredictSuccess(proposal Proposal) float64 {
	return hm.internal.GetLearning().PredictSuccess(proposal)
}

// GetConfig returns the current configuration.
func (hm *HiveMindManager) GetConfig() HiveMindConfig {
	return hm.internal.GetConfig()
}

// GetConsensusTypeInfo returns information about all consensus types.
func GetConsensusTypeInfo() []hivemind.ConsensusTypeInfo {
	return hivemind.GetConsensusTypeInfo()
}
