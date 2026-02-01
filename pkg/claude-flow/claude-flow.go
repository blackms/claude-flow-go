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

	"github.com/anthropics/claude-flow-go/internal/application/consensus"
	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/application/hivemind"
	"github.com/anthropics/claude-flow-go/internal/application/workflow"
	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/attention"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/events"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/federation"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/tools"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/memory"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/messaging"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/plugins"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/pool"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/routing"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/topology"
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

	// Distributed Consensus Algorithm types
	ConsensusAlgorithmType     = shared.ConsensusAlgorithmType
	RaftState                  = shared.RaftState
	RaftLogEntry               = shared.RaftLogEntry
	RaftNode                   = shared.RaftNode
	RaftConfig                 = shared.RaftConfig
	ByzantinePhase             = shared.ByzantinePhase
	ByzantineMessage           = shared.ByzantineMessage
	ByzantineNode              = shared.ByzantineNode
	ByzantineConfig            = shared.ByzantineConfig
	GossipMessageType          = shared.GossipMessageType
	GossipMessage              = shared.GossipMessage
	GossipNode                 = shared.GossipNode
	GossipConfig               = shared.GossipConfig
	FaultToleranceMode         = shared.FaultToleranceMode
	ConsistencyMode            = shared.ConsistencyMode
	NetworkScale               = shared.NetworkScale
	LatencyPriority            = shared.LatencyPriority
	AlgorithmSelectionOptions  = shared.AlgorithmSelectionOptions
	ConsensusProposal          = shared.ConsensusProposal
	ConsensusVote              = shared.ConsensusVote
	DistributedConsensusResult = shared.DistributedConsensusResult
	AlgorithmStats             = shared.AlgorithmStats

	// Message Bus types
	MessagePriority   = shared.MessagePriority
	BusMessageType    = shared.BusMessageType
	BusMessage        = shared.BusMessage
	MessageAck        = shared.MessageAck
	MessageBusConfig  = shared.MessageBusConfig
	MessageBusStats   = shared.MessageBusStats
	MessageEntry      = shared.MessageEntry

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
	SwarmTopology  = shared.SwarmTopology
	SwarmConfig    = shared.SwarmConfig
	SwarmState     = shared.SwarmState
	SwarmHierarchy = shared.SwarmHierarchy
	AgentMessage   = shared.AgentMessage
	AgentMetrics   = shared.AgentMetrics

	// Topology Manager types
	TopologyNodeRole   = shared.TopologyNodeRole
	TopologyNodeStatus = shared.TopologyNodeStatus
	TopologyNode       = shared.TopologyNode
	TopologyEdge       = shared.TopologyEdge
	TopologyPartition  = shared.TopologyPartition
	TopologyConfig     = shared.TopologyConfig
	TopologyState      = shared.TopologyState
	TopologyStats      = shared.TopologyStats
	PartitionStrategy  = shared.PartitionStrategy

	// Federation Hub types
	EphemeralAgentStatus     = shared.EphemeralAgentStatus
	EphemeralAgent           = shared.EphemeralAgent
	SpawnEphemeralOptions    = shared.SpawnEphemeralOptions
	SpawnResult              = shared.SpawnResult
	SwarmRegistrationStatus  = shared.SwarmRegistrationStatus
	SwarmRegistration        = shared.SwarmRegistration
	FederationMessageType    = shared.FederationMessageType
	FederationMessage        = shared.FederationMessage
	FederationProposalStatus = shared.FederationProposalStatus
	FederationProposal       = shared.FederationProposal
	FederationEventType      = shared.FederationEventType
	FederationEvent          = shared.FederationEvent
	FederationConfig         = shared.FederationConfig
	FederationStats          = shared.FederationStats

	// Attention Mechanism types
	AttentionMechanism         = shared.AttentionMechanism
	AttentionAgentOutput       = shared.AttentionAgentOutput
	AttentionCoordinationResult = shared.AttentionCoordinationResult
	FlashAttentionConfig       = shared.FlashAttentionConfig
	MultiHeadAttentionConfig   = shared.MultiHeadAttentionConfig
	LinearAttentionConfig      = shared.LinearAttentionConfig
	HyperbolicAttentionConfig  = shared.HyperbolicAttentionConfig
	MoEConfig                  = shared.MoEConfig
	GraphRoPEConfig            = shared.GraphRoPEConfig
	AttentionConfig            = shared.AttentionConfig
	Expert                     = shared.Expert
	ExpertRoutingResult        = shared.ExpertRoutingResult
	ExpertSelection            = shared.ExpertSelection
	AttentionPerformanceStats  = shared.AttentionPerformanceStats

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
	AgentTypeQueen               = shared.AgentTypeQueen
	AgentTypeSecurityArchitect   = shared.AgentTypeSecurityArchitect
	AgentTypeCVERemediation      = shared.AgentTypeCVERemediation
	AgentTypeThreatModeler       = shared.AgentTypeThreatModeler
	AgentTypeDDDDesigner         = shared.AgentTypeDDDDesigner
	AgentTypeMemorySpecialist    = shared.AgentTypeMemorySpecialist
	AgentTypeTypeModernizer      = shared.AgentTypeTypeModernizer
	AgentTypeSwarmSpecialist     = shared.AgentTypeSwarmSpecialist
	AgentTypeMCPOptimizer        = shared.AgentTypeMCPOptimizer
	AgentTypeAgenticFlow         = shared.AgentTypeAgenticFlow
	AgentTypeCLIDeveloper        = shared.AgentTypeCLIDeveloper
	AgentTypeNeuralIntegrator    = shared.AgentTypeNeuralIntegrator
	AgentTypeTDDTester           = shared.AgentTypeTDDTester
	AgentTypePerformanceEngineer = shared.AgentTypePerformanceEngineer
	AgentTypeReleaseManager      = shared.AgentTypeReleaseManager

	// Extended Agent Types (12 additional types)
	AgentTypeResearcher          = shared.AgentTypeResearcher
	AgentTypeArchitect           = shared.AgentTypeArchitect
	AgentTypeAnalyst             = shared.AgentTypeAnalyst
	AgentTypeOptimizer           = shared.AgentTypeOptimizer
	AgentTypeSecurityAuditor     = shared.AgentTypeSecurityAuditor
	AgentTypeCoreArchitect       = shared.AgentTypeCoreArchitect
	AgentTypeTestArchitect       = shared.AgentTypeTestArchitect
	AgentTypeIntegrationArchitect = shared.AgentTypeIntegrationArchitect
	AgentTypeHooksDeveloper      = shared.AgentTypeHooksDeveloper
	AgentTypeMCPSpecialist       = shared.AgentTypeMCPSpecialist
	AgentTypeDocumentationLead   = shared.AgentTypeDocumentationLead
	AgentTypeDevOpsEngineer      = shared.AgentTypeDevOpsEngineer
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

// ============================================================================
// Distributed Consensus Algorithms
// ============================================================================

// Algorithm type constants
const (
	AlgorithmRaft      = shared.AlgorithmRaft
	AlgorithmByzantine = shared.AlgorithmByzantine
	AlgorithmGossip    = shared.AlgorithmGossip
	AlgorithmPaxos     = shared.AlgorithmPaxos
)

// Raft state constants
const (
	RaftStateFollower  = shared.RaftStateFollower
	RaftStateCandidate = shared.RaftStateCandidate
	RaftStateLeader    = shared.RaftStateLeader
)

// Byzantine phase constants
const (
	ByzantinePhasePrePrepare = shared.ByzantinePhasePrePrepare
	ByzantinePhasePrepare    = shared.ByzantinePhasePrepare
	ByzantinePhaseCommit     = shared.ByzantinePhaseCommit
	ByzantinePhaseReply      = shared.ByzantinePhaseReply
)

// Gossip message type constants
const (
	GossipMessageProposal = shared.GossipMessageProposal
	GossipMessageVote     = shared.GossipMessageVote
	GossipMessageState    = shared.GossipMessageState
	GossipMessageAck      = shared.GossipMessageAck
)

// Algorithm selection constants
const (
	FaultToleranceCrash     = shared.FaultToleranceCrash
	FaultToleranceByzantine = shared.FaultToleranceByzantine
	ConsistencyStrong       = shared.ConsistencyStrong
	ConsistencyEventual     = shared.ConsistencyEventual
	NetworkScaleSmall       = shared.NetworkScaleSmall
	NetworkScaleMedium      = shared.NetworkScaleMedium
	NetworkScaleLarge       = shared.NetworkScaleLarge
	LatencyPriorityLow      = shared.LatencyPriorityLow
	LatencyPriorityMedium   = shared.LatencyPriorityMedium
	LatencyPriorityHigh     = shared.LatencyPriorityHigh
)

// ConsensusEngine wraps the internal consensus engine for public use.
type ConsensusEngine struct {
	internal *consensus.Engine
}

// NewConsensusEngine creates a new consensus engine with the specified algorithm.
func NewConsensusEngine(nodeID string, algorithmType ConsensusAlgorithmType) (*ConsensusEngine, error) {
	engine, err := consensus.NewEngine(nodeID, algorithmType)
	if err != nil {
		return nil, err
	}
	return &ConsensusEngine{internal: engine}, nil
}

// NewConsensusEngineWithConfig creates a new consensus engine with custom configuration.
func NewConsensusEngineWithConfig(nodeID string, algorithmType ConsensusAlgorithmType, config interface{}) (*ConsensusEngine, error) {
	engine, err := consensus.NewEngineWithConfig(nodeID, algorithmType, config)
	if err != nil {
		return nil, err
	}
	return &ConsensusEngine{internal: engine}, nil
}

// Initialize initializes the consensus engine.
func (ce *ConsensusEngine) Initialize(ctx context.Context) error {
	return ce.internal.Initialize(ctx)
}

// Shutdown shuts down the consensus engine.
func (ce *ConsensusEngine) Shutdown() error {
	return ce.internal.Shutdown()
}

// AddNode adds a node to the consensus cluster.
func (ce *ConsensusEngine) AddNode(nodeID string) error {
	return ce.internal.AddNode(nodeID)
}

// RemoveNode removes a node from the consensus cluster.
func (ce *ConsensusEngine) RemoveNode(nodeID string) error {
	return ce.internal.RemoveNode(nodeID)
}

// Propose proposes a value for consensus.
func (ce *ConsensusEngine) Propose(ctx context.Context, value interface{}) (*ConsensusProposal, error) {
	return ce.internal.Propose(ctx, value)
}

// Vote submits a vote for a proposal.
func (ce *ConsensusEngine) Vote(ctx context.Context, proposalID string, vote ConsensusVote) error {
	return ce.internal.Vote(ctx, proposalID, vote)
}

// AwaitConsensus waits for consensus on a proposal.
func (ce *ConsensusEngine) AwaitConsensus(ctx context.Context, proposalID string) (*DistributedConsensusResult, error) {
	return ce.internal.AwaitConsensus(ctx, proposalID)
}

// GetProposal returns a proposal by ID.
func (ce *ConsensusEngine) GetProposal(proposalID string) (*ConsensusProposal, bool) {
	return ce.internal.GetProposal(proposalID)
}

// GetActiveProposals returns all active proposals.
func (ce *ConsensusEngine) GetActiveProposals() []*ConsensusProposal {
	return ce.internal.GetActiveProposals()
}

// GetStats returns algorithm statistics.
func (ce *ConsensusEngine) GetStats() AlgorithmStats {
	return ce.internal.GetStats()
}

// GetAlgorithmType returns the algorithm type.
func (ce *ConsensusEngine) GetAlgorithmType() ConsensusAlgorithmType {
	return ce.internal.GetAlgorithmType()
}

// IsLeader returns true if this node is the leader (for Raft/BFT).
func (ce *ConsensusEngine) IsLeader() bool {
	return ce.internal.IsLeader()
}

// GetLeaderID returns the leader ID (for Raft).
func (ce *ConsensusEngine) GetLeaderID() string {
	return ce.internal.GetLeaderID()
}

// SelectOptimalAlgorithm selects the optimal consensus algorithm based on requirements.
func SelectOptimalAlgorithm(opts AlgorithmSelectionOptions) ConsensusAlgorithmType {
	return consensus.SelectOptimalAlgorithm(opts)
}

// GetAlgorithmInfo returns information about all available algorithms.
func GetAlgorithmInfo() []consensus.AlgorithmInfo {
	return consensus.GetAlgorithmInfo()
}

// DefaultRaftConfig returns the default Raft configuration.
func DefaultRaftConfig() RaftConfig {
	return shared.DefaultRaftConfig()
}

// DefaultByzantineConfig returns the default Byzantine configuration.
func DefaultByzantineConfig() ByzantineConfig {
	return shared.DefaultByzantineConfig()
}

// DefaultGossipConfig returns the default Gossip configuration.
func DefaultGossipConfig() GossipConfig {
	return shared.DefaultGossipConfig()
}

// ============================================================================
// Message Bus (Priority Queues)
// ============================================================================

// Message priority constants
const (
	MessagePriorityUrgent = shared.MessagePriorityUrgent
	MessagePriorityHigh   = shared.MessagePriorityHigh
	MessagePriorityNormal = shared.MessagePriorityNormal
	MessagePriorityLow    = shared.MessagePriorityLow
)

// Bus message type constants
const (
	BusMessageTaskAssign       = shared.BusMessageTaskAssign
	BusMessageTaskComplete     = shared.BusMessageTaskComplete
	BusMessageTaskFail         = shared.BusMessageTaskFail
	BusMessageHeartbeat        = shared.BusMessageHeartbeat
	BusMessageStatusUpdate     = shared.BusMessageStatusUpdate
	BusMessageAgentJoin        = shared.BusMessageAgentJoin
	BusMessageAgentLeave       = shared.BusMessageAgentLeave
	BusMessageConsensusPropose = shared.BusMessageConsensusPropose
	BusMessageConsensusVote    = shared.BusMessageConsensusVote
	BusMessageConsensusCommit  = shared.BusMessageConsensusCommit
	BusMessageTopologyUpdate   = shared.BusMessageTopologyUpdate
	BusMessageBroadcast        = shared.BusMessageBroadcast
	BusMessageDirect           = shared.BusMessageDirect
)

// MessageBus wraps the internal message bus for public use.
type MessageBus struct {
	internal *messaging.MessageBus
}

// MessageHandler is a callback function for handling messages.
type MessageHandler = messaging.MessageHandler

// NewMessageBus creates a new MessageBus with the given configuration.
func NewMessageBus(config MessageBusConfig) *MessageBus {
	return &MessageBus{internal: messaging.NewMessageBus(config)}
}

// NewMessageBusWithDefaults creates a new MessageBus with default configuration.
func NewMessageBusWithDefaults() *MessageBus {
	return &MessageBus{internal: messaging.NewMessageBusWithDefaults()}
}

// Initialize starts the message bus processing loops.
func (mb *MessageBus) Initialize() error {
	return mb.internal.Initialize()
}

// Shutdown stops the message bus and cleans up resources.
func (mb *MessageBus) Shutdown() error {
	return mb.internal.Shutdown()
}

// Send sends a message to a specific agent.
func (mb *MessageBus) Send(msg BusMessage) (string, error) {
	return mb.internal.Send(msg)
}

// Broadcast sends a message to all subscribed agents except the sender.
func (mb *MessageBus) Broadcast(msg BusMessage) (string, error) {
	return mb.internal.Broadcast(msg)
}

// Subscribe registers an agent to receive messages.
func (mb *MessageBus) Subscribe(agentID string, callback MessageHandler, filter []BusMessageType) {
	mb.internal.Subscribe(agentID, callback, filter)
}

// Unsubscribe removes an agent's subscription.
func (mb *MessageBus) Unsubscribe(agentID string) {
	mb.internal.Unsubscribe(agentID)
}

// Acknowledge processes an acknowledgment for a message.
func (mb *MessageBus) Acknowledge(ack MessageAck) error {
	return mb.internal.Acknowledge(ack)
}

// GetMessages retrieves and removes all pending messages for an agent (pull mode).
func (mb *MessageBus) GetMessages(agentID string) []*BusMessage {
	return mb.internal.GetMessages(agentID)
}

// HasPendingMessages checks if an agent has pending messages.
func (mb *MessageBus) HasPendingMessages(agentID string) bool {
	return mb.internal.HasPendingMessages(agentID)
}

// GetMessage retrieves a specific message by ID without removing it.
func (mb *MessageBus) GetMessage(messageID string) (*BusMessage, bool) {
	return mb.internal.GetMessage(messageID)
}

// GetStats returns the current message bus statistics.
func (mb *MessageBus) GetStats() MessageBusStats {
	return mb.internal.GetStats()
}

// GetQueueDepth returns the total number of queued messages.
func (mb *MessageBus) GetQueueDepth() int {
	return mb.internal.GetQueueDepth()
}

// GetConfig returns the current configuration.
func (mb *MessageBus) GetConfig() MessageBusConfig {
	return mb.internal.GetConfig()
}

// GetSubscriptionCount returns the number of subscriptions.
func (mb *MessageBus) GetSubscriptionCount() int {
	return mb.internal.GetSubscriptionCount()
}

// IsSubscribed checks if an agent is subscribed.
func (mb *MessageBus) IsSubscribed(agentID string) bool {
	return mb.internal.IsSubscribed(agentID)
}

// SetOnEnqueued sets the callback for when a message is enqueued.
func (mb *MessageBus) SetOnEnqueued(callback func(messageID, to string)) {
	mb.internal.SetOnEnqueued(callback)
}

// SetOnDelivered sets the callback for when a message is delivered.
func (mb *MessageBus) SetOnDelivered(callback func(messageID, to string)) {
	mb.internal.SetOnDelivered(callback)
}

// SetOnExpired sets the callback for when a message expires.
func (mb *MessageBus) SetOnExpired(callback func(messageID string)) {
	mb.internal.SetOnExpired(callback)
}

// SetOnFailed sets the callback for when a message delivery fails.
func (mb *MessageBus) SetOnFailed(callback func(messageID string, err error)) {
	mb.internal.SetOnFailed(callback)
}

// DefaultMessageBusConfig returns the default message bus configuration.
func DefaultMessageBusConfig() MessageBusConfig {
	return shared.DefaultMessageBusConfig()
}

// ============================================================================
// Topology Manager
// ============================================================================

// Topology type constants
const (
	TopologyRing   = shared.TopologyRing
	TopologyStar   = shared.TopologyStar
	TopologyHybrid = shared.TopologyHybrid
)

// Topology role constants
const (
	TopologyRoleQueen       = shared.TopologyRoleQueen
	TopologyRoleWorker      = shared.TopologyRoleWorker
	TopologyRoleCoordinator = shared.TopologyRoleCoordinator
	TopologyRolePeer        = shared.TopologyRolePeer
)

// Topology status constants
const (
	TopologyStatusActive   = shared.TopologyStatusActive
	TopologyStatusInactive = shared.TopologyStatusInactive
	TopologyStatusSyncing  = shared.TopologyStatusSyncing
	TopologyStatusFailed   = shared.TopologyStatusFailed
)

// Partition strategy constants
const (
	PartitionStrategyHash       = shared.PartitionStrategyHash
	PartitionStrategyRange      = shared.PartitionStrategyRange
	PartitionStrategyRoundRobin = shared.PartitionStrategyRoundRobin
)

// TopologyManager wraps the internal topology manager for public use.
type TopologyManager struct {
	internal *topology.TopologyManager
}

// NewTopologyManager creates a new TopologyManager with the given configuration.
func NewTopologyManager(config TopologyConfig) *TopologyManager {
	return &TopologyManager{internal: topology.NewTopologyManager(config)}
}

// NewTopologyManagerWithDefaults creates a new TopologyManager with default configuration.
func NewTopologyManagerWithDefaults() *TopologyManager {
	return &TopologyManager{internal: topology.NewTopologyManagerWithDefaults()}
}

// Initialize initializes the topology manager.
func (tm *TopologyManager) Initialize() error {
	return tm.internal.Initialize()
}

// Shutdown shuts down the topology manager.
func (tm *TopologyManager) Shutdown() error {
	return tm.internal.Shutdown()
}

// AddNode adds a node to the topology with the given role.
func (tm *TopologyManager) AddNode(agentID string, role TopologyNodeRole) (*TopologyNode, error) {
	return tm.internal.AddNode(agentID, role)
}

// RemoveNode removes a node from the topology.
func (tm *TopologyManager) RemoveNode(agentID string) error {
	return tm.internal.RemoveNode(agentID)
}

// UpdateNode updates a node's properties.
func (tm *TopologyManager) UpdateNode(agentID string, updates map[string]interface{}) error {
	return tm.internal.UpdateNode(agentID, updates)
}

// GetNode returns a node by agentID. O(1).
func (tm *TopologyManager) GetNode(agentID string) (*TopologyNode, bool) {
	return tm.internal.GetNode(agentID)
}

// GetNeighbors returns the neighbors of a node. O(1).
func (tm *TopologyManager) GetNeighbors(agentID string) []string {
	return tm.internal.GetNeighbors(agentID)
}

// GetNodesByRole returns all nodes with the given role. O(1).
func (tm *TopologyManager) GetNodesByRole(role TopologyNodeRole) []*TopologyNode {
	return tm.internal.GetNodesByRole(role)
}

// GetQueen returns the queen node. O(1).
func (tm *TopologyManager) GetQueen() *TopologyNode {
	return tm.internal.GetQueen()
}

// GetCoordinator returns the coordinator node. O(1).
func (tm *TopologyManager) GetCoordinator() *TopologyNode {
	return tm.internal.GetCoordinator()
}

// GetLeader returns the current leader agentID.
func (tm *TopologyManager) GetLeader() string {
	return tm.internal.GetLeader()
}

// ElectLeader performs leader election based on role priority.
func (tm *TopologyManager) ElectLeader() string {
	return tm.internal.ElectLeader()
}

// Rebalance rebalances the topology.
func (tm *TopologyManager) Rebalance() {
	tm.internal.Rebalance()
}

// GetState returns the current topology state.
func (tm *TopologyManager) GetState() TopologyState {
	return tm.internal.GetState()
}

// GetConfig returns the current configuration.
func (tm *TopologyManager) GetConfig() TopologyConfig {
	return tm.internal.GetConfig()
}

// GetStats returns topology statistics.
func (tm *TopologyManager) GetStats() TopologyStats {
	return tm.internal.GetStats()
}

// FindOptimalPath finds the shortest path between two nodes using BFS.
func (tm *TopologyManager) FindOptimalPath(from, to string) []string {
	return tm.internal.FindOptimalPath(from, to)
}

// FindAllPaths finds all paths between two nodes (up to maxPaths).
func (tm *TopologyManager) FindAllPaths(from, to string, maxPaths int) [][]string {
	return tm.internal.FindAllPaths(from, to, maxPaths)
}

// GetShortestDistance returns the shortest distance (hops) between two nodes.
func (tm *TopologyManager) GetShortestDistance(from, to string) int {
	return tm.internal.GetShortestDistance(from, to)
}

// IsConnected checks if the topology is fully connected.
func (tm *TopologyManager) IsConnected() bool {
	return tm.internal.IsConnected()
}

// GetConnectedComponents returns all connected components in the topology.
func (tm *TopologyManager) GetConnectedComponents() [][]string {
	return tm.internal.GetConnectedComponents()
}

// GetNodeDegree returns the degree (number of connections) of a node.
func (tm *TopologyManager) GetNodeDegree(agentID string) int {
	return tm.internal.GetNodeDegree(agentID)
}

// Ring topology methods

// GetRingSuccessor returns the next node in the ring.
func (tm *TopologyManager) GetRingSuccessor(agentID string) string {
	return tm.internal.GetRingSuccessor(agentID)
}

// GetRingPredecessor returns the previous node in the ring.
func (tm *TopologyManager) GetRingPredecessor(agentID string) string {
	return tm.internal.GetRingPredecessor(agentID)
}

// Star topology methods

// GetHub returns the hub (coordinator) node.
func (tm *TopologyManager) GetHub() *TopologyNode {
	return tm.internal.GetHub()
}

// GetSpokes returns all non-hub nodes.
func (tm *TopologyManager) GetSpokes() []*TopologyNode {
	return tm.internal.GetSpokes()
}

// IsHub checks if the given agent is the hub.
func (tm *TopologyManager) IsHub(agentID string) bool {
	return tm.internal.IsHub(agentID)
}

// Hybrid topology methods

// GetHybridCoordinators returns all coordinator nodes (queen + coordinators).
func (tm *TopologyManager) GetHybridCoordinators() []*TopologyNode {
	return tm.internal.GetHybridCoordinators()
}

// GetHybridWorkers returns all worker nodes.
func (tm *TopologyManager) GetHybridWorkers() []*TopologyNode {
	return tm.internal.GetHybridWorkers()
}

// Hierarchical topology methods

// GetHierarchy returns the hierarchy structure.
func (tm *TopologyManager) GetHierarchy() SwarmHierarchy {
	return tm.internal.GetHierarchy()
}

// Callback setters

// SetOnNodeAdded sets the callback for when a node is added.
func (tm *TopologyManager) SetOnNodeAdded(callback func(node *TopologyNode)) {
	tm.internal.SetOnNodeAdded(callback)
}

// SetOnNodeRemoved sets the callback for when a node is removed.
func (tm *TopologyManager) SetOnNodeRemoved(callback func(agentID string)) {
	tm.internal.SetOnNodeRemoved(callback)
}

// SetOnLeaderElected sets the callback for when a leader is elected.
func (tm *TopologyManager) SetOnLeaderElected(callback func(leaderID string)) {
	tm.internal.SetOnLeaderElected(callback)
}

// SetOnRebalanced sets the callback for when the topology is rebalanced.
func (tm *TopologyManager) SetOnRebalanced(callback func()) {
	tm.internal.SetOnRebalanced(callback)
}

// DefaultTopologyConfig returns the default topology configuration.
func DefaultTopologyConfig() TopologyConfig {
	return shared.DefaultTopologyConfig()
}

// ============================================================================
// Federation Hub
// ============================================================================

// Ephemeral agent status constants
const (
	EphemeralStatusSpawning   = shared.EphemeralStatusSpawning
	EphemeralStatusActive     = shared.EphemeralStatusActive
	EphemeralStatusCompleting = shared.EphemeralStatusCompleting
	EphemeralStatusTerminated = shared.EphemeralStatusTerminated
)

// Swarm registration status constants
const (
	SwarmStatusActive   = shared.SwarmStatusActive
	SwarmStatusInactive = shared.SwarmStatusInactive
	SwarmStatusDegraded = shared.SwarmStatusDegraded
)

// Federation message type constants
const (
	FederationMsgBroadcast = shared.FederationMsgBroadcast
	FederationMsgDirect    = shared.FederationMsgDirect
	FederationMsgConsensus = shared.FederationMsgConsensus
	FederationMsgHeartbeat = shared.FederationMsgHeartbeat
)

// Federation proposal status constants
const (
	FederationProposalPending  = shared.FederationProposalPending
	FederationProposalAccepted = shared.FederationProposalAccepted
	FederationProposalRejected = shared.FederationProposalRejected
)

// Federation event type constants
const (
	FederationEventSwarmJoined        = shared.FederationEventSwarmJoined
	FederationEventSwarmLeft          = shared.FederationEventSwarmLeft
	FederationEventSwarmDegraded      = shared.FederationEventSwarmDegraded
	FederationEventAgentSpawned       = shared.FederationEventAgentSpawned
	FederationEventAgentCompleted     = shared.FederationEventAgentCompleted
	FederationEventAgentFailed        = shared.FederationEventAgentFailed
	FederationEventAgentExpired       = shared.FederationEventAgentExpired
	FederationEventMessageSent        = shared.FederationEventMessageSent
	FederationEventMessageReceived    = shared.FederationEventMessageReceived
	FederationEventConsensusStarted   = shared.FederationEventConsensusStarted
	FederationEventConsensusCompleted = shared.FederationEventConsensusCompleted
	FederationEventSynced             = shared.FederationEventSynced
)

// FederationHub wraps the internal federation hub for public use.
type FederationHub struct {
	internal *federation.FederationHub
}

// FederationEventHandler is a callback for federation events.
type FederationEventHandler = federation.EventHandler

// NewFederationHub creates a new FederationHub with the given configuration.
func NewFederationHub(config FederationConfig) *FederationHub {
	return &FederationHub{internal: federation.NewFederationHub(config)}
}

// NewFederationHubWithDefaults creates a new FederationHub with default configuration.
func NewFederationHubWithDefaults() *FederationHub {
	return &FederationHub{internal: federation.NewFederationHubWithDefaults()}
}

// Initialize starts the federation hub background processes.
func (fh *FederationHub) Initialize() error {
	return fh.internal.Initialize()
}

// Shutdown stops the federation hub and cleans up resources.
func (fh *FederationHub) Shutdown() error {
	return fh.internal.Shutdown()
}

// Swarm Registration

// RegisterSwarm registers a swarm with the federation.
func (fh *FederationHub) RegisterSwarm(swarm SwarmRegistration) error {
	return fh.internal.RegisterSwarm(swarm)
}

// UnregisterSwarm removes a swarm from the federation.
func (fh *FederationHub) UnregisterSwarm(swarmID string) error {
	return fh.internal.UnregisterSwarm(swarmID)
}

// Heartbeat updates the heartbeat for a swarm.
func (fh *FederationHub) Heartbeat(swarmID string) error {
	return fh.internal.Heartbeat(swarmID)
}

// GetSwarm returns a swarm by ID.
func (fh *FederationHub) GetSwarm(swarmID string) (*SwarmRegistration, bool) {
	return fh.internal.GetSwarm(swarmID)
}

// GetSwarms returns all registered swarms.
func (fh *FederationHub) GetSwarms() []*SwarmRegistration {
	return fh.internal.GetSwarms()
}

// GetActiveSwarms returns all active swarms.
func (fh *FederationHub) GetActiveSwarms() []*SwarmRegistration {
	return fh.internal.GetActiveSwarms()
}

// Ephemeral Agents

// SpawnEphemeralAgent spawns a new ephemeral agent.
func (fh *FederationHub) SpawnEphemeralAgent(opts SpawnEphemeralOptions) (*SpawnResult, error) {
	return fh.internal.SpawnEphemeralAgent(opts)
}

// CompleteAgent marks an agent as completing with a result.
func (fh *FederationHub) CompleteAgent(agentID string, result interface{}) error {
	return fh.internal.CompleteAgent(agentID, result)
}

// TerminateAgent terminates an agent with an optional error.
func (fh *FederationHub) TerminateAgent(agentID string, errorMsg string) error {
	return fh.internal.TerminateAgent(agentID, errorMsg)
}

// GetAgent returns an agent by ID.
func (fh *FederationHub) GetAgent(agentID string) (*EphemeralAgent, bool) {
	return fh.internal.GetAgent(agentID)
}

// GetAgents returns all ephemeral agents.
func (fh *FederationHub) GetAgents() []*EphemeralAgent {
	return fh.internal.GetAgents()
}

// GetActiveAgents returns all active ephemeral agents.
func (fh *FederationHub) GetActiveAgents() []*EphemeralAgent {
	return fh.internal.GetActiveAgents()
}

// GetAgentsBySwarm returns all agents in a swarm. O(1) lookup.
func (fh *FederationHub) GetAgentsBySwarm(swarmID string) []*EphemeralAgent {
	return fh.internal.GetAgentsBySwarm(swarmID)
}

// GetAgentsByStatus returns all agents with a given status. O(1) lookup.
func (fh *FederationHub) GetAgentsByStatus(status EphemeralAgentStatus) []*EphemeralAgent {
	return fh.internal.GetAgentsByStatus(status)
}

// Cross-Swarm Messaging

// SendMessage sends a direct message to a specific swarm.
func (fh *FederationHub) SendMessage(sourceSwarmID, targetSwarmID string, payload interface{}) (*FederationMessage, error) {
	return fh.internal.SendMessage(sourceSwarmID, targetSwarmID, payload)
}

// Broadcast sends a message to all active swarms except the sender.
func (fh *FederationHub) Broadcast(sourceSwarmID string, payload interface{}) (*FederationMessage, error) {
	return fh.internal.Broadcast(sourceSwarmID, payload)
}

// GetMessages returns recent messages.
func (fh *FederationHub) GetMessages(limit int) []*FederationMessage {
	return fh.internal.GetMessages(limit)
}

// GetMessagesBySwarm returns messages for a specific swarm.
func (fh *FederationHub) GetMessagesBySwarm(swarmID string, limit int) []*FederationMessage {
	return fh.internal.GetMessagesBySwarm(swarmID, limit)
}

// GetMessage returns a message by ID.
func (fh *FederationHub) GetMessage(messageID string) (*FederationMessage, bool) {
	return fh.internal.GetMessage(messageID)
}

// Federation Consensus

// Propose creates a new consensus proposal.
func (fh *FederationHub) Propose(proposerID, proposalType string, value interface{}) (*FederationProposal, error) {
	return fh.internal.Propose(proposerID, proposalType, value)
}

// Vote submits a vote on a proposal.
func (fh *FederationHub) Vote(voterID, proposalID string, approve bool) error {
	return fh.internal.Vote(voterID, proposalID, approve)
}

// GetProposal returns a proposal by ID.
func (fh *FederationHub) GetProposal(proposalID string) (*FederationProposal, bool) {
	return fh.internal.GetProposal(proposalID)
}

// GetProposals returns all proposals.
func (fh *FederationHub) GetProposals() []*FederationProposal {
	return fh.internal.GetProposals()
}

// GetPendingProposals returns all pending proposals.
func (fh *FederationHub) GetPendingProposals() []*FederationProposal {
	return fh.internal.GetPendingProposals()
}

// GetQuorumInfo returns information about quorum requirements.
func (fh *FederationHub) GetQuorumInfo() (activeSwarms int, requiredVotes int, quorumPercentage float64) {
	return fh.internal.GetQuorumInfo()
}

// Events and Stats

// SetEventHandler sets the event handler for federation events.
func (fh *FederationHub) SetEventHandler(handler FederationEventHandler) {
	fh.internal.SetEventHandler(handler)
}

// GetEvents returns recent federation events.
func (fh *FederationHub) GetEvents(limit int) []*FederationEvent {
	return fh.internal.GetEvents(limit)
}

// GetStats returns federation statistics.
func (fh *FederationHub) GetStats() FederationStats {
	return fh.internal.GetStats()
}

// GetConfig returns the federation configuration.
func (fh *FederationHub) GetConfig() FederationConfig {
	return fh.internal.GetConfig()
}

// DefaultFederationConfig returns the default federation configuration.
func DefaultFederationConfig() FederationConfig {
	return shared.DefaultFederationConfig()
}

// NewFederationTools creates MCP tools for federation operations.
func NewFederationTools(hub *FederationHub) *tools.FederationTools {
	return tools.NewFederationTools(hub.internal)
}

// ============================================================================
// Attention Coordinator
// ============================================================================

// Attention mechanism constants
const (
	AttentionFlash      = shared.AttentionFlash
	AttentionMultiHead  = shared.AttentionMultiHead
	AttentionLinear     = shared.AttentionLinear
	AttentionHyperbolic = shared.AttentionHyperbolic
	AttentionMoE        = shared.AttentionMoE
	AttentionGraphRoPE  = shared.AttentionGraphRoPE
)

// AttentionCoordinator wraps the internal attention coordinator for public use.
type AttentionCoordinator struct {
	internal *attention.AttentionCoordinator
}

// AttentionEventHandler is a callback for attention coordination events.
type AttentionEventHandler = attention.EventHandler

// AttentionEvent represents an attention coordination event.
type AttentionEvent = attention.AttentionEvent

// NewAttentionCoordinator creates a new AttentionCoordinator with the given configuration.
func NewAttentionCoordinator(config AttentionConfig) *AttentionCoordinator {
	return &AttentionCoordinator{internal: attention.NewAttentionCoordinator(config)}
}

// NewAttentionCoordinatorWithDefaults creates an AttentionCoordinator with default configuration.
func NewAttentionCoordinatorWithDefaults() *AttentionCoordinator {
	return &AttentionCoordinator{internal: attention.NewAttentionCoordinatorWithDefaults()}
}

// Coordinate dispatches coordination to the default attention mechanism.
func (ac *AttentionCoordinator) Coordinate(outputs []AttentionAgentOutput) *AttentionCoordinationResult {
	return ac.internal.Coordinate(outputs)
}

// CoordinateWithMechanism coordinates using a specific attention mechanism.
func (ac *AttentionCoordinator) CoordinateWithMechanism(outputs []AttentionAgentOutput, mechanism AttentionMechanism) *AttentionCoordinationResult {
	return ac.internal.CoordinateWithMechanism(outputs, mechanism)
}

// RouteToExperts routes a task to experts using MoE.
func (ac *AttentionCoordinator) RouteToExperts(taskEmbedding []float64, experts []Expert) *ExpertRoutingResult {
	return ac.internal.RouteToExperts(taskEmbedding, experts)
}

// TopologyAwareCoordination performs GraphRoPE-based coordination.
func (ac *AttentionCoordinator) TopologyAwareCoordination(outputs []AttentionAgentOutput, topology map[string][]string) *AttentionCoordinationResult {
	return ac.internal.TopologyAwareCoordination(outputs, topology)
}

// HierarchicalCoordination performs hyperbolic attention for hierarchical swarms.
func (ac *AttentionCoordinator) HierarchicalCoordination(outputs []AttentionAgentOutput, roles map[string]string) *AttentionCoordinationResult {
	return ac.internal.HierarchicalCoordination(outputs, roles)
}

// GetStats returns the performance statistics.
func (ac *AttentionCoordinator) GetStats() AttentionPerformanceStats {
	return ac.internal.GetStats()
}

// GetConfig returns the current configuration.
func (ac *AttentionCoordinator) GetConfig() AttentionConfig {
	return ac.internal.GetConfig()
}

// SetEventHandler sets the event handler for coordination events.
func (ac *AttentionCoordinator) SetEventHandler(handler AttentionEventHandler) {
	ac.internal.SetEventHandler(handler)
}

// SetTopology sets the topology for GraphRoPE.
func (ac *AttentionCoordinator) SetTopology(adjacency map[string][]string) {
	ac.internal.SetTopology(adjacency)
}

// SetTopologyFromEdges sets the topology from edges for GraphRoPE.
func (ac *AttentionCoordinator) SetTopologyFromEdges(edges []TopologyEdge) {
	ac.internal.SetTopologyFromEdges(edges)
}

// SetRoles sets the agent roles for hyperbolic attention.
func (ac *AttentionCoordinator) SetRoles(roles map[string]string) {
	ac.internal.SetRoles(roles)
}

// BenchmarkMechanisms benchmarks all mechanisms with the same input.
func (ac *AttentionCoordinator) BenchmarkMechanisms(outputs []AttentionAgentOutput) map[AttentionMechanism]*AttentionCoordinationResult {
	return ac.internal.BenchmarkMechanisms(outputs)
}

// RecommendMechanism recommends the best mechanism based on input characteristics.
func (ac *AttentionCoordinator) RecommendMechanism(numAgents int, hasTopology bool, isHierarchical bool, needsFastRouting bool) AttentionMechanism {
	return ac.internal.RecommendMechanism(numAgents, hasTopology, isHierarchical, needsFastRouting)
}

// Default configuration functions

// DefaultAttentionConfig returns the default attention configuration.
func DefaultAttentionConfig() AttentionConfig {
	return shared.DefaultAttentionConfig()
}

// DefaultFlashAttentionConfig returns the default Flash Attention configuration.
func DefaultFlashAttentionConfig() FlashAttentionConfig {
	return shared.DefaultFlashAttentionConfig()
}

// DefaultMultiHeadAttentionConfig returns the default Multi-Head Attention configuration.
func DefaultMultiHeadAttentionConfig() MultiHeadAttentionConfig {
	return shared.DefaultMultiHeadAttentionConfig()
}

// DefaultLinearAttentionConfig returns the default Linear Attention configuration.
func DefaultLinearAttentionConfig() LinearAttentionConfig {
	return shared.DefaultLinearAttentionConfig()
}

// DefaultHyperbolicAttentionConfig returns the default Hyperbolic Attention configuration.
func DefaultHyperbolicAttentionConfig() HyperbolicAttentionConfig {
	return shared.DefaultHyperbolicAttentionConfig()
}

// DefaultMoEConfig returns the default MoE configuration.
func DefaultMoEConfig() MoEConfig {
	return shared.DefaultMoEConfig()
}

// DefaultGraphRoPEConfig returns the default GraphRoPE configuration.
func DefaultGraphRoPEConfig() GraphRoPEConfig {
	return shared.DefaultGraphRoPEConfig()
}

// ============================================================================
// Agent Type Registry
// ============================================================================

// ModelTier represents the model tier for an agent type.
type ModelTier = agent.ModelTier

// Model tier constants
const (
	ModelTierOpus   = agent.ModelTierOpus
	ModelTierSonnet = agent.ModelTierSonnet
	ModelTierHaiku  = agent.ModelTierHaiku
)

// AgentTypeSpec defines the specification for an agent type.
type AgentTypeSpec = agent.AgentTypeSpec

// NeuralPatternConfig holds neural pattern configuration for an agent type.
type NeuralPatternConfig = agent.NeuralPatternConfig

// AgentTypeRegistry manages all agent type specifications.
type AgentTypeRegistry struct {
	internal *agent.AgentTypeRegistry
}

// NewAgentTypeRegistry creates a new AgentTypeRegistry with all default specs.
func NewAgentTypeRegistry() *AgentTypeRegistry {
	return &AgentTypeRegistry{internal: agent.NewAgentTypeRegistry()}
}

// GetSpec returns the specification for an agent type.
func (r *AgentTypeRegistry) GetSpec(t AgentType) *AgentTypeSpec {
	return r.internal.GetSpec(t)
}

// ListAll returns all registered agent types.
func (r *AgentTypeRegistry) ListAll() []AgentType {
	return r.internal.ListAll()
}

// ListByCapability returns agent types that have the specified capability.
func (r *AgentTypeRegistry) ListByCapability(capability string) []AgentType {
	return r.internal.ListByCapability(capability)
}

// ListByTag returns agent types that have the specified tag.
func (r *AgentTypeRegistry) ListByTag(tag string) []AgentType {
	return r.internal.ListByTag(tag)
}

// ListByModelTier returns agent types for the specified model tier.
func (r *AgentTypeRegistry) ListByModelTier(tier ModelTier) []AgentType {
	return r.internal.ListByModelTier(tier)
}

// ListByDomain returns agent types in the specified domain.
func (r *AgentTypeRegistry) ListByDomain(domain AgentDomain) []AgentType {
	return r.internal.ListByDomain(domain)
}

// GetCapabilities returns the capabilities for an agent type.
func (r *AgentTypeRegistry) GetCapabilities(t AgentType) []string {
	return r.internal.GetCapabilities(t)
}

// GetModelTier returns the model tier for an agent type.
func (r *AgentTypeRegistry) GetModelTier(t AgentType) ModelTier {
	return r.internal.GetModelTier(t)
}

// GetNeuralPatterns returns the neural patterns for an agent type.
func (r *AgentTypeRegistry) GetNeuralPatterns(t AgentType) []string {
	return r.internal.GetNeuralPatterns(t)
}

// FindBestMatch finds the best agent type for the given capabilities.
func (r *AgentTypeRegistry) FindBestMatch(requiredCapabilities []string) (AgentType, float64) {
	return r.internal.FindBestMatch(requiredCapabilities)
}

// Count returns the total number of registered agent types.
func (r *AgentTypeRegistry) Count() int {
	return r.internal.Count()
}

// HasType checks if an agent type is registered.
func (r *AgentTypeRegistry) HasType(t AgentType) bool {
	return r.internal.HasType(t)
}

// GetAllSpecs returns all registered specifications.
func (r *AgentTypeRegistry) GetAllSpecs() []*AgentTypeSpec {
	return r.internal.GetAllSpecs()
}

// GetDefaultCapabilities returns default capabilities for an agent type.
func GetDefaultCapabilities(agentType AgentType) []string {
	return agent.GetDefaultCapabilities(agentType)
}

// GetNeuralPatternsForType returns the neural pattern configuration for an agent type.
func GetNeuralPatternsForType(t AgentType) *NeuralPatternConfig {
	return agent.GetNeuralPatternsForType(t)
}

// DefaultNeuralPatternConfig returns the default neural pattern configuration.
func DefaultNeuralPatternConfig() NeuralPatternConfig {
	return agent.DefaultNeuralPatternConfig()
}

// ============================================================================
// Agent Pool Manager
// ============================================================================

// TypePoolConfig holds configuration for a type-specific agent pool.
type TypePoolConfig = pool.TypePoolConfig

// PoolStats holds statistics for an agent pool.
type PoolStats = pool.PoolStats

// AgentPoolManager manages pools of agents by type.
type AgentPoolManager struct {
	internal *pool.AgentPoolManager
}

// NewAgentPoolManager creates a new AgentPoolManager.
func NewAgentPoolManager(registry *AgentTypeRegistry) *AgentPoolManager {
	return &AgentPoolManager{internal: pool.NewAgentPoolManager(registry.internal)}
}

// Initialize initializes the pool manager and starts background tasks.
func (pm *AgentPoolManager) Initialize() error {
	return pm.internal.Initialize()
}

// Shutdown shuts down the pool manager.
func (pm *AgentPoolManager) Shutdown() error {
	return pm.internal.Shutdown()
}

// SetConfig sets the configuration for a specific agent type.
func (pm *AgentPoolManager) SetConfig(agentType AgentType, config TypePoolConfig) {
	pm.internal.SetConfig(agentType, config)
}

// GetConfig returns the configuration for a specific agent type.
func (pm *AgentPoolManager) GetConfig(agentType AgentType) TypePoolConfig {
	return pm.internal.GetConfig(agentType)
}

// Acquire acquires an available agent of the specified type.
func (pm *AgentPoolManager) Acquire(agentType AgentType) (*Agent, error) {
	a, err := pm.internal.Acquire(agentType)
	if err != nil {
		return nil, err
	}
	return &Agent{internal: a}, nil
}

// Release releases an agent back to the pool.
func (pm *AgentPoolManager) Release(a *Agent) {
	pm.internal.Release(a.internal)
}

// GetPoolStats returns statistics for a specific agent type pool.
func (pm *AgentPoolManager) GetPoolStats(agentType AgentType) PoolStats {
	return pm.internal.GetPoolStats(agentType)
}

// GetAllPoolStats returns statistics for all pools.
func (pm *AgentPoolManager) GetAllPoolStats() []PoolStats {
	return pm.internal.GetAllPoolStats()
}

// Scale scales a pool to the specified target size.
func (pm *AgentPoolManager) Scale(agentType AgentType, targetSize int) error {
	return pm.internal.Scale(agentType, targetSize)
}

// WarmUp pre-spawns agents into the warm pool.
func (pm *AgentPoolManager) WarmUp(agentType AgentType, count int) error {
	return pm.internal.WarmUp(agentType, count)
}

// ListPoolTypes returns all pool types that have been created.
func (pm *AgentPoolManager) ListPoolTypes() []AgentType {
	return pm.internal.ListPoolTypes()
}

// GetTotalAgentCount returns the total number of agents across all pools.
func (pm *AgentPoolManager) GetTotalAgentCount() int {
	return pm.internal.GetTotalAgentCount()
}

// DefaultTypePoolConfig returns the default pool configuration.
func DefaultTypePoolConfig() TypePoolConfig {
	return pool.DefaultTypePoolConfig()
}

// ============================================================================
// Type Router
// ============================================================================

// RoutingResult holds the result of a routing decision.
type RoutingResult = routing.RoutingResult

// RoutingStats holds routing statistics.
type RoutingStats = routing.RoutingStats

// TypeRouter routes tasks to appropriate agent types.
type TypeRouter struct {
	internal *routing.TypeRouter
}

// NewTypeRouter creates a new TypeRouter.
func NewTypeRouter(registry *AgentTypeRegistry, pools *AgentPoolManager) *TypeRouter {
	var poolMgr *pool.AgentPoolManager
	if pools != nil {
		poolMgr = pools.internal
	}
	return &TypeRouter{internal: routing.NewTypeRouter(registry.internal, poolMgr)}
}

// RouteTask routes a task to the best agent type.
func (tr *TypeRouter) RouteTask(task Task) (*RoutingResult, error) {
	return tr.internal.RouteTask(task)
}

// SelectModel returns the recommended model for an agent type.
func (tr *TypeRouter) SelectModel(agentType AgentType) string {
	return tr.internal.SelectModel(agentType)
}

// FindBestType finds the best agent type for the given capabilities.
func (tr *TypeRouter) FindBestType(capabilities []string) (AgentType, float64) {
	return tr.internal.FindBestType(capabilities)
}

// GetAvailableType returns the best available agent type.
func (tr *TypeRouter) GetAvailableType(requiredCaps []string) (AgentType, error) {
	return tr.internal.GetAvailableType(requiredCaps)
}

// RouteByCapability routes to the best agent type for a single capability.
func (tr *TypeRouter) RouteByCapability(capability string) []AgentType {
	return tr.internal.RouteByCapability(capability)
}

// RouteByTag routes to agent types with a specific tag.
func (tr *TypeRouter) RouteByTag(tag string) []AgentType {
	return tr.internal.RouteByTag(tag)
}

// RouteByDomain routes to agent types in a specific domain.
func (tr *TypeRouter) RouteByDomain(domain AgentDomain) []AgentType {
	return tr.internal.RouteByDomain(domain)
}

// SetPreferAvailable sets whether to prefer available agents.
func (tr *TypeRouter) SetPreferAvailable(prefer bool) {
	tr.internal.SetPreferAvailable(prefer)
}

// GetRoutingStats returns routing statistics.
func (tr *TypeRouter) GetRoutingStats() RoutingStats {
	return tr.internal.GetRoutingStats()
}

// ============================================================================
// Type Health Monitor
// ============================================================================

// TypeMetrics holds health metrics for an agent type.
type TypeMetrics = pool.TypeMetrics

// HealthStatus represents the health status of an agent type.
type HealthStatus = pool.HealthStatus

// Health status constants
const (
	HealthStatusHealthy   = pool.HealthStatusHealthy
	HealthStatusDegraded  = pool.HealthStatusDegraded
	HealthStatusUnhealthy = pool.HealthStatusUnhealthy
	HealthStatusUnknown   = pool.HealthStatusUnknown
)

// HealthAlert represents a health alert for an agent type.
type HealthAlert = pool.HealthAlert

// HealthConfig holds configuration for health monitoring.
type HealthConfig = pool.HealthConfig

// TypeHealthMonitor monitors health metrics per agent type.
type TypeHealthMonitor struct {
	internal *pool.TypeHealthMonitor
}

// NewTypeHealthMonitor creates a new TypeHealthMonitor.
func NewTypeHealthMonitor(pools *AgentPoolManager, config HealthConfig) *TypeHealthMonitor {
	var poolMgr *pool.AgentPoolManager
	if pools != nil {
		poolMgr = pools.internal
	}
	return &TypeHealthMonitor{internal: pool.NewTypeHealthMonitor(poolMgr, config)}
}

// NewTypeHealthMonitorWithDefaults creates a TypeHealthMonitor with default config.
func NewTypeHealthMonitorWithDefaults(pools *AgentPoolManager) *TypeHealthMonitor {
	var poolMgr *pool.AgentPoolManager
	if pools != nil {
		poolMgr = pools.internal
	}
	return &TypeHealthMonitor{internal: pool.NewTypeHealthMonitorWithDefaults(poolMgr)}
}

// Initialize starts the health monitoring.
func (thm *TypeHealthMonitor) Initialize() error {
	return thm.internal.Initialize()
}

// Shutdown stops the health monitoring.
func (thm *TypeHealthMonitor) Shutdown() error {
	return thm.internal.Shutdown()
}

// RecordTaskCompletion records a task completion for an agent type.
func (thm *TypeHealthMonitor) RecordTaskCompletion(agentType AgentType, success bool, latencyMs float64) {
	thm.internal.RecordTaskCompletion(agentType, success, latencyMs)
}

// GetMetrics returns metrics for an agent type.
func (thm *TypeHealthMonitor) GetMetrics(agentType AgentType) *TypeMetrics {
	return thm.internal.GetMetrics(agentType)
}

// GetAllMetrics returns metrics for all agent types.
func (thm *TypeHealthMonitor) GetAllMetrics() []*TypeMetrics {
	return thm.internal.GetAllMetrics()
}

// GetHealthStatus returns the health status for an agent type.
func (thm *TypeHealthMonitor) GetHealthStatus(agentType AgentType) HealthStatus {
	return thm.internal.GetHealthStatus(agentType)
}

// GetAlerts returns recent alerts.
func (thm *TypeHealthMonitor) GetAlerts(limit int) []HealthAlert {
	return thm.internal.GetAlerts(limit)
}

// GetUnresolvedAlerts returns unresolved alerts.
func (thm *TypeHealthMonitor) GetUnresolvedAlerts() []HealthAlert {
	return thm.internal.GetUnresolvedAlerts()
}

// GetOverallHealth returns the overall health of all agent types.
func (thm *TypeHealthMonitor) GetOverallHealth() HealthStatus {
	return thm.internal.GetOverallHealth()
}

// DefaultHealthConfig returns the default health configuration.
func DefaultHealthConfig() HealthConfig {
	return pool.DefaultHealthConfig()
}

// AllowedAgentTypes returns all valid agent types (33 types total).
var AllowedAgentTypes = tools.AllowedAgentTypes
