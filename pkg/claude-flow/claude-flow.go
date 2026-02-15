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
	"io"

	"github.com/anthropics/claude-flow-go/internal/application/consensus"
	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/application/hivemind"
	"github.com/anthropics/claude-flow-go/internal/application/workflow"
	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/attention"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/events"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/federation"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/hooks"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp"
	mcpcompletion "github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/completion"
	mcplogging "github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/logging"
	mcpprompts "github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/prompts"
	mcpresources "github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/resources"
	"github.com/anthropics/claude-flow-go/internal/application/executor"
	mcpsampling "github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/sampling"
	mcpsessions "github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/sessions"
	mcptasks "github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/tasks"
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

	// Queen Coordinator types
	QueenMetrics   = coordinator.QueenMetrics
	BottleneckInfo = coordinator.BottleneckInfo
	BottleneckType = coordinator.BottleneckType
	Severity       = coordinator.Severity

	// Health Monitor types
	HealthMonitorConfig = coordinator.HealthMonitorConfig

	// Queen Coordinator service interfaces
	NeuralLearningSystem = shared.NeuralLearningSystem
	MemoryService        = shared.MemoryService
	TrajectoryStep       = shared.TrajectoryStep

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

	// MCP 2025-11-25 Compliance Types
	// Resource types
	MCPResource        = shared.MCPResource
	ResourceContent    = shared.ResourceContent
	ResourceTemplate   = shared.ResourceTemplate
	ResourceListResult = shared.ResourceListResult
	ResourceReadResult = shared.ResourceReadResult
	ResourceCacheConfig = shared.ResourceCacheConfig

	// Prompt types
	MCPPrompt         = shared.MCPPrompt
	PromptArgument    = shared.PromptArgument
	PromptContent     = shared.PromptContent
	PromptContentType = shared.PromptContentType
	PromptMessage     = shared.PromptMessage
	PromptListResult  = shared.PromptListResult
	PromptGetResult   = shared.PromptGetResult

	// Sampling types
	SamplingMessage      = shared.SamplingMessage
	ModelPreferences     = shared.ModelPreferences
	ModelHint            = shared.ModelHint
	CreateMessageRequest = shared.CreateMessageRequest
	CreateMessageResult  = shared.CreateMessageResult
	SamplingConfig       = shared.SamplingConfig
	SamplingStats        = shared.SamplingStats

	// Completion types
	CompletionReference     = shared.CompletionReference
	CompletionReferenceType = shared.CompletionReferenceType
	CompletionArgument      = shared.CompletionArgument
	CompletionResult        = shared.CompletionResult

	// Logging types
	MCPLogLevel    = shared.MCPLogLevel
	LoggingMessage = shared.LoggingMessage

	// Capabilities
	MCPCapabilities     = shared.MCPCapabilities
	LoggingCapability   = shared.LoggingCapability
	PromptsCapability   = shared.PromptsCapability
	ResourcesCapability = shared.ResourcesCapability
	ToolsCapability     = shared.ToolsCapability
	SamplingCapability  = shared.SamplingCapability

	// Task Management Types
	ManagedTask          = shared.ManagedTask
	ManagedTaskStatus    = shared.ManagedTaskStatus
	TaskArtifact         = shared.TaskArtifact
	TaskHistoryEntry     = shared.TaskHistoryEntry
	TaskMetrics          = shared.TaskMetrics
	TaskFilter           = shared.TaskFilter
	TaskUpdate           = shared.TaskUpdate
	ManagedTaskResult    = shared.ManagedTaskResult
	TaskManagerConfig    = shared.TaskManagerConfig
	TaskManagerStats     = shared.TaskManagerStats
	TaskCreateRequest    = shared.TaskCreateRequest
	TaskListResult       = shared.TaskListResult
	TaskDependencyAction = shared.TaskDependencyAction
	TaskResultFormat     = shared.TaskResultFormat

	// Hooks System Types
	HookEvent           = shared.HookEvent
	HookPriority        = shared.HookPriority
	HookRegistration    = shared.HookRegistration
	HookContext         = shared.HookContext
	HookResult          = shared.HookResult
	HookExecutionResult = shared.HookExecutionResult
	PatternType         = shared.PatternType
	Pattern             = shared.Pattern
	RiskLevel           = shared.RiskLevel
	RiskAssessment      = shared.RiskAssessment
	RoutingResult       = shared.RoutingResult
	RoutingAlternative  = shared.RoutingAlternative
	RoutingFactor       = shared.RoutingFactor
	RoutingExplanation  = shared.RoutingExplanation
	RoutingHistory      = shared.RoutingHistory
	AgentRoutingStats   = shared.AgentRoutingStats
	HooksMetrics        = shared.HooksMetrics
	HooksConfig         = shared.HooksConfig
	PreEditContext      = shared.PreEditContext
	PreEditResult       = shared.PreEditResult
	PostEditContext     = shared.PostEditContext
	PostEditResult      = shared.PostEditResult
	PreCommandContext   = shared.PreCommandContext
	PreCommandResult    = shared.PreCommandResult
	PostCommandContext  = shared.PostCommandContext
	PostCommandResult   = shared.PostCommandResult

	// Session Management Types
	SessionState         = shared.SessionState
	TransportType        = shared.TransportType
	Session              = shared.Session
	SessionClientInfo    = shared.SessionClientInfo
	SessionConfig        = shared.SessionConfig
	SessionStats         = shared.SessionStats
	SessionSaveRequest   = shared.SessionSaveRequest
	SessionSaveResult    = shared.SessionSaveResult
	SessionRestoreRequest = shared.SessionRestoreRequest
	SessionRestoreResult = shared.SessionRestoreResult
	SessionListRequest   = shared.SessionListRequest
	SessionListResult    = shared.SessionListResult
	SessionSummary       = shared.SessionSummary
	SavedSession         = shared.SavedSession
	SavedSessionAgent    = shared.SavedSessionAgent
	SavedSessionTask     = shared.SavedSessionTask
	SavedSessionMemory   = shared.SavedSessionMemory

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

// Bottleneck type constants
const (
	BottleneckTypeAgent  = coordinator.BottleneckTypeAgent
	BottleneckTypeDomain = coordinator.BottleneckTypeDomain
	BottleneckTypeQueue  = coordinator.BottleneckTypeQueue
)

// Severity constants
const (
	SeverityLow      = coordinator.SeverityLow
	SeverityMedium   = coordinator.SeverityMedium
	SeverityHigh     = coordinator.SeverityHigh
	SeverityCritical = coordinator.SeverityCritical
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
	internal       *mcp.Server
	federationHub *federation.FederationHub
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

	// Create sampling manager + Claude CLI provider.
	samplingMgr := mcpsampling.NewSamplingManagerWithDefaults()
	cliProvider := mcpsampling.NewClaudeCLIProvider(mcpsampling.ClaudeCLIConfig{})
	if cliProvider.IsAvailable() {
		samplingMgr.RegisterProvider(cliProvider, true)
	}

	// Wire LLM executor into the coordinator when LLM is available.
	if config.Coordinator != nil && samplingMgr.IsAvailable() {
		taskExec := executor.New(executor.TaskExecutorConfig{
			SamplingManager: samplingMgr,
			MemoryBackend:   config.Memory,
			AgentRegistry:   agent.NewAgentTypeRegistry(),
		})
		config.Coordinator.internal.SetTaskExecutor(taskExec)
	}

	// Register orchestration tools when coordinator is available.
	if config.Coordinator != nil {
		routingEngine := hooks.NewRoutingEngine(0.1)
		providers = append(providers, tools.NewOrchestrateTools(config.Coordinator.internal, routingEngine, config.Memory, samplingMgr))
	}

	// Register federation tools
	fedHub := federation.NewFederationHubWithDefaults()
	if err := fedHub.Initialize(); err != nil {
		fedHub = nil
	}
	providers = append(providers, tools.NewFederationTools(fedHub))

	opts := mcp.Options{
		Tools: providers,
		Port:  config.Port,
		Host:  config.Host,
	}

	return &MCPServer{
		internal:       mcp.NewServer(opts),
		federationHub: fedHub,
	}
}

// Start starts the MCP server.
func (ms *MCPServer) Start() error {
	return ms.internal.Start()
}

// Stop stops the MCP server.
func (ms *MCPServer) Stop() error {
	stopErr := ms.internal.Stop()
	if ms.federationHub != nil {
		fedHub := ms.federationHub
		ms.federationHub = nil
		if shutdownErr := fedHub.Shutdown(); stopErr == nil {
			stopErr = shutdownErr
		}
	}
	return stopErr
}

// ListTools returns available tools.
func (ms *MCPServer) ListTools() []MCPTool {
	return ms.internal.ListTools()
}

// GetStatus returns server status.
func (ms *MCPServer) GetStatus() map[string]interface{} {
	return ms.internal.GetStatus()
}

// RunStdio runs the MCP server using stdio transport (stdin/stdout).
func (ms *MCPServer) RunStdio(ctx context.Context, reader io.Reader, writer io.Writer) error {
	transport := mcp.NewStdioTransport(ms.internal, reader, writer)
	return transport.Run(ctx)
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
func (qc *QueenCoordinator) DetectBottlenecks() []BottleneckInfo {
	return qc.internal.DetectBottlenecks()
}

// GetMetrics returns metrics about the Queen Coordinator.
func (qc *QueenCoordinator) GetMetrics() QueenMetrics {
	return qc.internal.GetMetrics()
}

// GetSwarm returns the underlying swarm coordinator.
func (qc *QueenCoordinator) GetSwarm() *SwarmCoordinator {
	return &SwarmCoordinator{internal: qc.internal.GetSwarm()}
}

// SetNeuralLearning sets the neural learning system for the Queen Coordinator.
func (qc *QueenCoordinator) SetNeuralLearning(nl shared.NeuralLearningSystem) {
	qc.internal.SetNeuralLearning(nl)
}

// SetMemoryService sets the memory service for the Queen Coordinator.
func (qc *QueenCoordinator) SetMemoryService(ms shared.MemoryService) {
	qc.internal.SetMemoryService(ms)
}

// GetQueen returns the Queen agent.
func (qc *QueenCoordinator) GetQueen() *Agent {
	queen := qc.internal.GetQueen()
	if queen == nil {
		return nil
	}
	agentData := queen.ToShared()
	return &agentData
}

// GetHealthMonitor returns the health monitor.
func (qc *QueenCoordinator) GetHealthMonitor() *HealthMonitor {
	return &HealthMonitor{internal: qc.internal.GetHealthMonitor()}
}

// HealthMonitor wraps the internal HealthMonitor for public use.
type HealthMonitor struct {
	internal *coordinator.HealthMonitor
}

// Start begins background health monitoring.
func (hm *HealthMonitor) Start() {
	hm.internal.Start()
}

// Stop stops the health monitor.
func (hm *HealthMonitor) Stop() {
	hm.internal.Stop()
}

// RegisterAgent registers an agent for health monitoring.
func (hm *HealthMonitor) RegisterAgent(agentID string, domain AgentDomain) {
	hm.internal.RegisterAgent(agentID, domain)
}

// UnregisterAgent removes an agent from health monitoring.
func (hm *HealthMonitor) UnregisterAgent(agentID string) {
	hm.internal.UnregisterAgent(agentID)
}

// RecordHeartbeat records a heartbeat from an agent.
func (hm *HealthMonitor) RecordHeartbeat(agentID string) {
	hm.internal.RecordHeartbeat(agentID)
}

// UpdateAgentLoad updates the current load for an agent.
func (hm *HealthMonitor) UpdateAgentLoad(agentID string, load float64, tasksInQueue int) {
	hm.internal.UpdateAgentLoad(agentID, load, tasksInQueue)
}

// RecordTaskCompletion records a task completion for health tracking.
func (hm *HealthMonitor) RecordTaskCompletion(agentID string, duration int64, success bool) {
	hm.internal.RecordTaskCompletion(agentID, duration, success)
}

// GetAgentHealth returns the health status of an agent.
func (hm *HealthMonitor) GetAgentHealth(agentID string) (*AgentHealth, bool) {
	return hm.internal.GetAgentHealth(agentID)
}

// GetAllAgentHealth returns health status for all agents.
func (hm *HealthMonitor) GetAllAgentHealth() map[string]*AgentHealth {
	return hm.internal.GetAllAgentHealth()
}

// GetDomainHealth returns the health status of a domain.
func (hm *HealthMonitor) GetDomainHealth(domain AgentDomain) (*DomainHealth, bool) {
	return hm.internal.GetDomainHealth(domain)
}

// GetAllDomainHealth returns health status for all domains.
func (hm *HealthMonitor) GetAllDomainHealth() map[AgentDomain]*DomainHealth {
	return hm.internal.GetAllDomainHealth()
}

// GetAlerts returns recent alerts.
func (hm *HealthMonitor) GetAlerts(limit int) []HealthAlert {
	return hm.internal.GetAlerts(limit)
}

// GetAlertsByLevel returns alerts filtered by level.
func (hm *HealthMonitor) GetAlertsByLevel(level AlertLevel) []HealthAlert {
	return hm.internal.GetAlertsByLevel(level)
}

// ClearAlerts clears all alerts.
func (hm *HealthMonitor) ClearAlerts() {
	hm.internal.ClearAlerts()
}

// DetectBottlenecks analyzes the system and returns detected bottlenecks.
func (hm *HealthMonitor) DetectBottlenecks() []BottleneckInfo {
	return hm.internal.DetectBottlenecks()
}

// DefaultHealthMonitorConfig returns the default health monitor configuration.
func DefaultHealthMonitorConfig() HealthMonitorConfig {
	return coordinator.DefaultHealthMonitorConfig()
}

// ============================================================================
// Domain Pool (Agent Pool Management)
// ============================================================================

// DomainPool wraps the internal DomainPool for public use.
type DomainPool struct {
	internal *coordinator.DomainPool
}

// GetDomainPool returns the domain pool for a specific domain.
func (qc *QueenCoordinator) GetDomainPool(domain AgentDomain) (*DomainPool, bool) {
	pool, ok := qc.internal.GetDomainPool(domain)
	if !ok {
		return nil, false
	}
	return &DomainPool{internal: pool}, true
}

// GetConfig returns the domain configuration.
func (dp *DomainPool) GetConfig() DomainConfig {
	return dp.internal.Config
}

// AddAgent adds an agent to the domain pool.
func (dp *DomainPool) AddAgent(config AgentConfig) error {
	a := agent.FromConfig(config)
	return dp.internal.AddAgent(a)
}

// RemoveAgent removes an agent from the domain pool.
func (dp *DomainPool) RemoveAgent(agentID string) error {
	return dp.internal.RemoveAgent(agentID)
}

// GetAgent retrieves an agent from the domain pool.
func (dp *DomainPool) GetAgent(agentID string) (*Agent, bool) {
	a, ok := dp.internal.GetAgent(agentID)
	if !ok {
		return nil, false
	}
	agentData := a.ToShared()
	return &agentData, true
}

// ListAgents returns all agents in the domain pool.
func (dp *DomainPool) ListAgents() []Agent {
	agents := dp.internal.ListAgents()
	result := make([]Agent, len(agents))
	for i, a := range agents {
		result[i] = a.ToShared()
	}
	return result
}

// GetAvailableAgents returns agents that are available for task assignment.
func (dp *DomainPool) GetAvailableAgents() []Agent {
	agents := dp.internal.GetAvailableAgents()
	result := make([]Agent, len(agents))
	for i, a := range agents {
		result[i] = a.ToShared()
	}
	return result
}

// GetAgentCount returns the number of agents in the domain.
func (dp *DomainPool) GetAgentCount() int {
	return dp.internal.GetAgentCount()
}

// GetActiveAgentCount returns the number of active/idle agents.
func (dp *DomainPool) GetActiveAgentCount() int {
	return dp.internal.GetActiveAgentCount()
}

// SubmitTask submits a task to the domain's task queue.
func (dp *DomainPool) SubmitTask(task Task) error {
	return dp.internal.SubmitTask(task)
}

// HasCapability checks if the domain has the given capability.
func (dp *DomainPool) HasCapability(capability string) bool {
	return dp.internal.HasCapability(capability)
}

// MatchCapabilities returns a score (0-1) for how well the domain matches the required capabilities.
func (dp *DomainPool) MatchCapabilities(required []string) float64 {
	return dp.internal.MatchCapabilities(required)
}

// RecordTaskCompletion records that a task was completed.
func (dp *DomainPool) RecordTaskCompletion(duration int64, success bool) {
	dp.internal.RecordTaskCompletion(duration, success)
}

// GetMetrics returns the current domain metrics.
func (dp *DomainPool) GetMetrics() DomainMetrics {
	return dp.internal.GetMetrics()
}

// GetHealth returns the current health status of the domain.
func (dp *DomainPool) GetHealth() DomainHealth {
	return dp.internal.GetHealth()
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
	if hub == nil {
		return tools.NewFederationTools(nil)
	}
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
	sa := a.ToShared()
	return &sa, nil
}

// Release releases an agent back to the pool.
func (pm *AgentPoolManager) Release(a *Agent) {
	if da := pm.internal.GetAgent(a.ID); da != nil {
		pm.internal.Release(da)
	}
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

// TypeRoutingResult holds the result of a type routing decision.
type TypeRoutingResult = routing.RoutingResult

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
func (tr *TypeRouter) RouteTask(task Task) (*TypeRoutingResult, error) {
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

// TypeHealthAlert represents a health alert for an agent type.
type TypeHealthAlert = pool.HealthAlert

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
func (thm *TypeHealthMonitor) GetAlerts(limit int) []TypeHealthAlert {
	return thm.internal.GetAlerts(limit)
}

// GetUnresolvedAlerts returns unresolved alerts.
func (thm *TypeHealthMonitor) GetUnresolvedAlerts() []TypeHealthAlert {
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

// ============================================================================
// MCP 2025-11-25 Compliance Components
// ============================================================================

// Prompt content type constants
const (
	PromptContentTypeText     = shared.PromptContentTypeText
	PromptContentTypeImage    = shared.PromptContentTypeImage
	PromptContentTypeResource = shared.PromptContentTypeResource
)

// Completion reference type constants
const (
	CompletionRefPrompt   = shared.CompletionRefPrompt
	CompletionRefResource = shared.CompletionRefResource
)

// Log level constants
const (
	MCPLogLevelDebug     = shared.MCPLogLevelDebug
	MCPLogLevelInfo      = shared.MCPLogLevelInfo
	MCPLogLevelNotice    = shared.MCPLogLevelNotice
	MCPLogLevelWarning   = shared.MCPLogLevelWarning
	MCPLogLevelError     = shared.MCPLogLevelError
	MCPLogLevelCritical  = shared.MCPLogLevelCritical
	MCPLogLevelAlert     = shared.MCPLogLevelAlert
	MCPLogLevelEmergency = shared.MCPLogLevelEmergency
)

// ResourceRegistry manages MCP resources.
type ResourceRegistry struct {
	internal *mcpresources.ResourceRegistry
}

// ResourceHandler is a function that reads a resource.
type ResourceHandler = mcpresources.ResourceHandler

// NewResourceRegistry creates a new ResourceRegistry.
func NewResourceRegistry(config ResourceCacheConfig) *ResourceRegistry {
	return &ResourceRegistry{internal: mcpresources.NewResourceRegistry(config)}
}

// NewResourceRegistryWithDefaults creates a ResourceRegistry with default configuration.
func NewResourceRegistryWithDefaults() *ResourceRegistry {
	return &ResourceRegistry{internal: mcpresources.NewResourceRegistryWithDefaults()}
}

// RegisterResource registers a static resource with its handler.
func (rr *ResourceRegistry) RegisterResource(resource *MCPResource, handler ResourceHandler) error {
	return rr.internal.RegisterResource(resource, handler)
}

// RegisterTemplate registers a resource template with its handler.
func (rr *ResourceRegistry) RegisterTemplate(template *ResourceTemplate, handler ResourceHandler) error {
	return rr.internal.RegisterTemplate(template, handler)
}

// List returns a paginated list of resources.
func (rr *ResourceRegistry) List(cursor string, pageSize int) *ResourceListResult {
	return rr.internal.List(cursor, pageSize)
}

// Read reads a resource by URI.
func (rr *ResourceRegistry) Read(uri string) (*ResourceReadResult, error) {
	return rr.internal.Read(uri)
}

// Subscribe subscribes to updates for a resource URI.
func (rr *ResourceRegistry) Subscribe(uri string, callback func(string, *ResourceContent)) string {
	return rr.internal.Subscribe(uri, callback)
}

// Unsubscribe removes a subscription.
func (rr *ResourceRegistry) Unsubscribe(subscriptionID string) bool {
	return rr.internal.Unsubscribe(subscriptionID)
}

// NotifyUpdate notifies subscribers of a resource update.
func (rr *ResourceRegistry) NotifyUpdate(uri string) {
	rr.internal.NotifyUpdate(uri)
}

// Count returns the number of registered resources.
func (rr *ResourceRegistry) Count() int {
	return rr.internal.Count()
}

// DefaultResourceCacheConfig returns the default resource cache configuration.
func DefaultResourceCacheConfig() ResourceCacheConfig {
	return shared.DefaultResourceCacheConfig()
}

// CreateTextResource creates a text resource handler.
func CreateTextResource(text, mimeType string) ResourceHandler {
	return mcpresources.CreateTextResource(text, mimeType)
}

// PromptRegistry manages MCP prompts.
type PromptRegistry struct {
	internal *mcpprompts.PromptRegistry
}

// PromptHandler is a function that generates prompt messages.
type PromptHandler = mcpprompts.PromptHandler

// NewPromptRegistry creates a new PromptRegistry.
func NewPromptRegistry(maxPrompts int) *PromptRegistry {
	return &PromptRegistry{internal: mcpprompts.NewPromptRegistry(maxPrompts)}
}

// NewPromptRegistryWithDefaults creates a PromptRegistry with default configuration.
func NewPromptRegistryWithDefaults() *PromptRegistry {
	return &PromptRegistry{internal: mcpprompts.NewPromptRegistryWithDefaults()}
}

// Register registers a prompt with its handler.
func (pr *PromptRegistry) Register(prompt *MCPPrompt, handler PromptHandler) error {
	return pr.internal.Register(prompt, handler)
}

// List returns a paginated list of prompts.
func (pr *PromptRegistry) List(cursor string, pageSize int) *PromptListResult {
	return pr.internal.List(cursor, pageSize)
}

// Get retrieves a prompt and generates its messages with the given arguments.
func (pr *PromptRegistry) Get(name string, args map[string]string) (*PromptGetResult, error) {
	return pr.internal.Get(name, args)
}

// HasPrompt checks if a prompt exists.
func (pr *PromptRegistry) HasPrompt(name string) bool {
	return pr.internal.HasPrompt(name)
}

// Count returns the number of registered prompts.
func (pr *PromptRegistry) Count() int {
	return pr.internal.Count()
}

// Interpolate replaces {{arg}} placeholders with argument values.
func Interpolate(template string, args map[string]string) string {
	return mcpprompts.Interpolate(template, args)
}

// TextMessage creates a text content message.
func TextMessage(role, text string) PromptMessage {
	return mcpprompts.TextMessage(role, text)
}

// ResourceMessage creates a message with embedded resource.
func ResourceMessage(role, uri, mimeType string) PromptMessage {
	return mcpprompts.ResourceMessage(role, uri, mimeType)
}

// LLMProvider represents an LLM provider that can create messages.
type LLMProvider = mcpsampling.LLMProvider

// SamplingManager manages LLM providers and sampling requests.
type SamplingManager struct {
	internal *mcpsampling.SamplingManager
}

// NewSamplingManager creates a new SamplingManager.
func NewSamplingManager(config SamplingConfig) *SamplingManager {
	return &SamplingManager{internal: mcpsampling.NewSamplingManager(config)}
}

// NewSamplingManagerWithDefaults creates a SamplingManager with default configuration.
func NewSamplingManagerWithDefaults() *SamplingManager {
	return &SamplingManager{internal: mcpsampling.NewSamplingManagerWithDefaults()}
}

// RegisterProvider registers an LLM provider.
func (sm *SamplingManager) RegisterProvider(provider LLMProvider, isDefault bool) {
	sm.internal.RegisterProvider(provider, isDefault)
}

// CreateMessage creates a message using an LLM provider.
func (sm *SamplingManager) CreateMessage(request *CreateMessageRequest) (*CreateMessageResult, error) {
	return sm.internal.CreateMessage(request)
}

// CreateMessageWithContext creates a message with context.
func (sm *SamplingManager) CreateMessageWithContext(ctx context.Context, request *CreateMessageRequest) (*CreateMessageResult, error) {
	return sm.internal.CreateMessageWithContext(ctx, request)
}

// IsAvailable checks if any provider is available.
func (sm *SamplingManager) IsAvailable() bool {
	return sm.internal.IsAvailable()
}

// GetProviders returns all registered providers.
func (sm *SamplingManager) GetProviders() []string {
	return sm.internal.GetProviders()
}

// GetStats returns sampling statistics.
func (sm *SamplingManager) GetStats() SamplingStats {
	return sm.internal.GetStats()
}

// DefaultSamplingConfig returns the default sampling configuration.
func DefaultSamplingConfig() SamplingConfig {
	return shared.DefaultSamplingConfig()
}

// NewMockProvider creates a mock LLM provider for testing.
func NewMockProvider(name string) LLMProvider {
	return mcpsampling.NewMockProvider(name)
}

// NewMockProviderWithDefaults creates a mock provider with default responses.
func NewMockProviderWithDefaults() LLMProvider {
	return mcpsampling.NewMockProviderWithDefaults()
}

// CompletionHandler handles completion requests.
type CompletionHandler struct {
	internal *mcpcompletion.CompletionHandler
}

// NewCompletionHandler creates a new CompletionHandler.
func NewCompletionHandler(res *ResourceRegistry, pr *PromptRegistry) *CompletionHandler {
	return &CompletionHandler{internal: mcpcompletion.NewCompletionHandler(res.internal, pr.internal)}
}

// Complete handles a completion request.
func (ch *CompletionHandler) Complete(ref *CompletionReference, arg *CompletionArgument) *CompletionResult {
	return ch.internal.Complete(ref, arg)
}

// LogEntry represents a log entry.
type LogEntry = mcplogging.LogEntry

// LogHandler is a callback for log messages.
type LogHandler = mcplogging.LogHandler

// LogManager manages logging configuration and output.
type LogManager struct {
	internal *mcplogging.LogManager
}

// NewLogManager creates a new LogManager.
func NewLogManager(level MCPLogLevel, maxEntries int) *LogManager {
	return &LogManager{internal: mcplogging.NewLogManager(level, maxEntries)}
}

// NewLogManagerWithDefaults creates a LogManager with default settings.
func NewLogManagerWithDefaults() *LogManager {
	return &LogManager{internal: mcplogging.NewLogManagerWithDefaults()}
}

// SetLevel sets the log level.
func (lm *LogManager) SetLevel(level string) error {
	return lm.internal.SetLevel(level)
}

// GetLevel returns the current log level.
func (lm *LogManager) GetLevel() MCPLogLevel {
	return lm.internal.GetLevel()
}

// Log logs a message at the specified level.
func (lm *LogManager) Log(level MCPLogLevel, message string, data interface{}) {
	lm.internal.Log(level, message, data)
}

// Debug logs a debug message.
func (lm *LogManager) Debug(message string, data interface{}) {
	lm.internal.Debug(message, data)
}

// Info logs an info message.
func (lm *LogManager) Info(message string, data interface{}) {
	lm.internal.Info(message, data)
}

// Warning logs a warning message.
func (lm *LogManager) Warning(message string, data interface{}) {
	lm.internal.Warning(message, data)
}

// Error logs an error message.
func (lm *LogManager) Error(message string, data interface{}) {
	lm.internal.Error(message, data)
}

// GetEntries returns recent log entries.
func (lm *LogManager) GetEntries(limit int) []LogEntry {
	return lm.internal.GetEntries(limit)
}

// AddHandler adds a log handler.
func (lm *LogManager) AddHandler(handler LogHandler) {
	lm.internal.AddHandler(handler)
}

// ============================================================================
// Task Management
// ============================================================================

// Managed task status constants
const (
	ManagedTaskStatusPending   = shared.ManagedTaskStatusPending
	ManagedTaskStatusQueued    = shared.ManagedTaskStatusQueued
	ManagedTaskStatusRunning   = shared.ManagedTaskStatusRunning
	ManagedTaskStatusCompleted = shared.ManagedTaskStatusCompleted
	ManagedTaskStatusFailed    = shared.ManagedTaskStatusFailed
	ManagedTaskStatusCancelled = shared.ManagedTaskStatusCancelled
)

// Task dependency action constants
const (
	TaskDependencyActionAdd    = shared.TaskDependencyActionAdd
	TaskDependencyActionRemove = shared.TaskDependencyActionRemove
	TaskDependencyActionList   = shared.TaskDependencyActionList
	TaskDependencyActionClear  = shared.TaskDependencyActionClear
)

// Task result format constants
const (
	TaskResultFormatSummary  = shared.TaskResultFormatSummary
	TaskResultFormatDetailed = shared.TaskResultFormatDetailed
	TaskResultFormatRaw      = shared.TaskResultFormatRaw
)

// ProgressCallback is called when task progress is updated.
type ProgressCallback = mcptasks.ProgressCallback

// TaskManager manages tasks with async execution, queuing, and progress tracking.
type TaskManager struct {
	internal *mcptasks.TaskManager
}

// NewTaskManager creates a new TaskManager.
func NewTaskManager(config TaskManagerConfig, coord *SwarmCoordinator) *TaskManager {
	var internalCoord *coordinator.SwarmCoordinator
	if coord != nil {
		internalCoord = coord.internal
	}
	return &TaskManager{internal: mcptasks.NewTaskManager(config, internalCoord)}
}

// NewTaskManagerWithDefaults creates a TaskManager with default configuration.
func NewTaskManagerWithDefaults(coord *SwarmCoordinator) *TaskManager {
	var internalCoord *coordinator.SwarmCoordinator
	if coord != nil {
		internalCoord = coord.internal
	}
	return &TaskManager{internal: mcptasks.NewTaskManagerWithDefaults(internalCoord)}
}

// Initialize starts the TaskManager.
func (tm *TaskManager) Initialize() error {
	return tm.internal.Initialize()
}

// Shutdown stops the TaskManager.
func (tm *TaskManager) Shutdown() error {
	return tm.internal.Shutdown()
}

// CreateTask creates a new task.
func (tm *TaskManager) CreateTask(req *TaskCreateRequest) (*ManagedTask, error) {
	return tm.internal.CreateTask(req)
}

// GetTask retrieves a task by ID.
func (tm *TaskManager) GetTask(id string) (*ManagedTask, error) {
	return tm.internal.GetTask(id)
}

// ListTasks lists tasks with optional filtering.
func (tm *TaskManager) ListTasks(filter *TaskFilter) *TaskListResult {
	return tm.internal.ListTasks(filter)
}

// CancelTask cancels a task.
func (tm *TaskManager) CancelTask(id string, reason string, force bool) error {
	return tm.internal.CancelTask(id, reason, force)
}

// AssignTask assigns a task to an agent.
func (tm *TaskManager) AssignTask(id, agentID string, reassign bool) error {
	return tm.internal.AssignTask(id, agentID, reassign)
}

// UpdateTask updates task properties.
func (tm *TaskManager) UpdateTask(id string, update *TaskUpdate) error {
	return tm.internal.UpdateTask(id, update)
}

// UpdateDependencies manages task dependencies.
func (tm *TaskManager) UpdateDependencies(id string, action TaskDependencyAction, deps []string) ([]string, error) {
	return tm.internal.UpdateDependencies(id, action, deps)
}

// GetResults retrieves task results.
func (tm *TaskManager) GetResults(id string, format TaskResultFormat, includeArtifacts bool) (*ManagedTaskResult, error) {
	return tm.internal.GetResults(id, format, includeArtifacts)
}

// ReportProgress updates task progress.
func (tm *TaskManager) ReportProgress(id string, progress float64) error {
	return tm.internal.ReportProgress(id, progress)
}

// SetProgressCallback sets the progress callback.
func (tm *TaskManager) SetProgressCallback(callback ProgressCallback) {
	tm.internal.SetProgressCallback(callback)
}

// GetStats returns TaskManager statistics.
func (tm *TaskManager) GetStats() *TaskManagerStats {
	return tm.internal.GetStats()
}

// GetConfig returns the TaskManager configuration.
func (tm *TaskManager) GetConfig() TaskManagerConfig {
	return tm.internal.GetConfig()
}

// DefaultTaskManagerConfig returns the default TaskManager configuration.
func DefaultTaskManagerConfig() TaskManagerConfig {
	return shared.DefaultTaskManagerConfig()
}

// ============================================================================
// Hooks System
// ============================================================================

// Hook event constants
const (
	HookEventPreEdit        = shared.HookEventPreEdit
	HookEventPostEdit       = shared.HookEventPostEdit
	HookEventPreCommand     = shared.HookEventPreCommand
	HookEventPostCommand    = shared.HookEventPostCommand
	HookEventPreRoute       = shared.HookEventPreRoute
	HookEventPostRoute      = shared.HookEventPostRoute
	HookEventPreTask        = shared.HookEventPreTask
	HookEventPostTask       = shared.HookEventPostTask
	HookEventAgentSpawn     = shared.HookEventAgentSpawn
	HookEventAgentTerminate = shared.HookEventAgentTerminate
	HookEventSessionStart   = shared.HookEventSessionStart
	HookEventSessionEnd     = shared.HookEventSessionEnd
	HookEventPatternLearned = shared.HookEventPatternLearned
)

// Hook priority constants
const (
	HookPriorityCritical   = shared.HookPriorityCritical
	HookPriorityHigh       = shared.HookPriorityHigh
	HookPriorityNormal     = shared.HookPriorityNormal
	HookPriorityLow        = shared.HookPriorityLow
	HookPriorityBackground = shared.HookPriorityBackground
)

// Pattern type constants
const (
	PatternTypeEdit    = shared.PatternTypeEdit
	PatternTypeCommand = shared.PatternTypeCommand
	PatternTypeRoute   = shared.PatternTypeRoute
	PatternTypeTask    = shared.PatternTypeTask
)

// Risk level constants
const (
	RiskLevelLow      = shared.RiskLevelLow
	RiskLevelMedium   = shared.RiskLevelMedium
	RiskLevelHigh     = shared.RiskLevelHigh
	RiskLevelCritical = shared.RiskLevelCritical
)

// Hook-related errors
var (
	// ErrHookNotFound is returned when a hook is not found.
	ErrHookNotFound = shared.ErrHookNotFound
	// ErrHookAlreadyExists is returned when a hook already exists.
	ErrHookAlreadyExists = shared.ErrHookAlreadyExists
	// ErrMaxHooksReached is returned when the maximum number of hooks is reached.
	ErrMaxHooksReached = shared.ErrMaxHooksReached
	// ErrPatternNotFound is returned when a pattern is not found.
	ErrPatternNotFound = shared.ErrPatternNotFound
	// ErrMaxPatternsReached is returned when the maximum number of patterns is reached.
	ErrMaxPatternsReached = shared.ErrMaxPatternsReached
	// ErrHookExecutionTimeout is returned when hook execution times out.
	ErrHookExecutionTimeout = shared.ErrHookExecutionTimeout
)

// HookHandler is a function that handles a hook event.
type HookHandler = shared.HookHandler

// HooksManager manages hook registration, execution, and learning.
type HooksManager struct {
	internal *hooks.HooksManager
}

// NewHooksManager creates a new HooksManager.
func NewHooksManager(config HooksConfig) *HooksManager {
	return &HooksManager{internal: hooks.NewHooksManager(config)}
}

// NewHooksManagerWithDefaults creates a HooksManager with default configuration.
func NewHooksManagerWithDefaults() *HooksManager {
	return &HooksManager{internal: hooks.NewHooksManagerWithDefaults()}
}

// Initialize starts the HooksManager.
func (hm *HooksManager) Initialize() error {
	return hm.internal.Initialize()
}

// Shutdown stops the HooksManager.
func (hm *HooksManager) Shutdown() error {
	return hm.internal.Shutdown()
}

// Register registers a hook.
func (hm *HooksManager) Register(hook *HookRegistration) error {
	return hm.internal.Register(hook)
}

// Unregister removes a hook.
func (hm *HooksManager) Unregister(hookID string) error {
	return hm.internal.Unregister(hookID)
}

// Execute executes all hooks for an event.
func (hm *HooksManager) Execute(ctx context.Context, hookCtx *HookContext) (*HookExecutionResult, error) {
	return hm.internal.Execute(ctx, hookCtx)
}

// GetHook returns a hook by ID.
func (hm *HooksManager) GetHook(id string) *HookRegistration {
	return hm.internal.GetHook(id)
}

// ListHooks returns hooks, optionally filtered by event.
func (hm *HooksManager) ListHooks(event HookEvent, includeDisabled bool) []*HookRegistration {
	return hm.internal.ListHooks(event, includeDisabled)
}

// EnableHook enables a hook.
func (hm *HooksManager) EnableHook(id string) error {
	return hm.internal.EnableHook(id)
}

// DisableHook disables a hook.
func (hm *HooksManager) DisableHook(id string) error {
	return hm.internal.DisableHook(id)
}

// GetMetrics returns the hooks metrics.
func (hm *HooksManager) GetMetrics() *HooksMetrics {
	return hm.internal.GetMetrics()
}

// GetConfig returns the hooks configuration.
func (hm *HooksManager) GetConfig() HooksConfig {
	return hm.internal.GetConfig()
}

// HookCount returns the total number of registered hooks.
func (hm *HooksManager) HookCount() int {
	return hm.internal.HookCount()
}

// HookCountByEvent returns the number of hooks for a specific event.
func (hm *HooksManager) HookCountByEvent(event HookEvent) int {
	return hm.internal.HookCountByEvent(event)
}

// DefaultHooksConfig returns the default hooks configuration.
func DefaultHooksConfig() HooksConfig {
	return shared.DefaultHooksConfig()
}

// GetPatternStore returns the pattern store for direct access to learned patterns.
func (hm *HooksManager) GetPatternStore() *PatternStore {
	return &PatternStore{internal: hm.internal.GetPatternStore()}
}

// GetRoutingEngine returns the routing engine for direct access to routing logic.
func (hm *HooksManager) GetRoutingEngine() *RoutingEngine {
	return &RoutingEngine{internal: hm.internal.GetRoutingEngine()}
}

// PatternStore stores and retrieves learned patterns from hook execution.
type PatternStore struct {
	internal *hooks.PatternStore
}

// Store stores a pattern in the pattern store.
func (ps *PatternStore) Store(pattern *Pattern) error {
	return ps.internal.Store(pattern)
}

// Get retrieves a pattern by ID.
func (ps *PatternStore) Get(id string) *Pattern {
	return ps.internal.Get(id)
}

// FindSimilar finds patterns similar to the query.
func (ps *PatternStore) FindSimilar(query string, patternType PatternType, limit int) []*Pattern {
	return ps.internal.FindSimilar(query, patternType, limit)
}

// RecordSuccess records a successful use of a pattern.
func (ps *PatternStore) RecordSuccess(id string) error {
	return ps.internal.RecordSuccess(id)
}

// RecordFailure records a failed use of a pattern.
func (ps *PatternStore) RecordFailure(id string) error {
	return ps.internal.RecordFailure(id)
}

// GetSuccessRate returns the success rate for a pattern.
func (ps *PatternStore) GetSuccessRate(id string) float64 {
	return ps.internal.GetSuccessRate(id)
}

// Delete removes a pattern from the store.
func (ps *PatternStore) Delete(id string) error {
	return ps.internal.Delete(id)
}

// Count returns the total number of patterns.
func (ps *PatternStore) Count() int {
	return ps.internal.Count()
}

// CountByType returns the number of patterns of a specific type.
func (ps *PatternStore) CountByType(patternType PatternType) int {
	return ps.internal.CountByType(patternType)
}

// ListByType returns all patterns of a specific type.
func (ps *PatternStore) ListByType(patternType PatternType, limit int) []*Pattern {
	return ps.internal.ListByType(patternType, limit)
}

// Clear removes all patterns from the store.
func (ps *PatternStore) Clear() {
	ps.internal.Clear()
}

// CreateEditPattern creates a pattern from an edit operation.
func CreateEditPattern(filePath, operation string, success bool, metadata map[string]interface{}) *Pattern {
	return hooks.CreateEditPattern(filePath, operation, success, metadata)
}

// CreateCommandPattern creates a pattern from a command execution.
func CreateCommandPattern(command string, success bool, exitCode int, executionTime int64) *Pattern {
	return hooks.CreateCommandPattern(command, success, exitCode, executionTime)
}

// RoutingEngine provides intelligent routing of tasks to agents.
type RoutingEngine struct {
	internal *hooks.RoutingEngine
}

// Route routes a task to the optimal agent.
func (re *RoutingEngine) Route(task string, context map[string]interface{}, preferredAgents []string, constraints map[string]interface{}) *RoutingResult {
	return re.internal.Route(task, context, preferredAgents, constraints)
}

// RecordOutcome records the outcome of a routing decision for learning.
func (re *RoutingEngine) RecordOutcome(routingID string, success bool, executionTimeMs int64) error {
	return re.internal.RecordOutcome(routingID, success, executionTimeMs)
}

// Explain provides a detailed explanation for routing a task.
func (re *RoutingEngine) Explain(task string, context map[string]interface{}, verbose bool) *RoutingExplanation {
	return re.internal.Explain(task, context, verbose)
}

// GetRoutingCount returns the total number of routing decisions.
func (re *RoutingEngine) GetRoutingCount() int64 {
	return re.internal.GetRoutingCount()
}

// GetSuccessRate returns the overall routing success rate.
func (re *RoutingEngine) GetSuccessRate() float64 {
	return re.internal.GetSuccessRate()
}

// GetAgentStats returns routing statistics for all agents.
func (re *RoutingEngine) GetAgentStats() map[string]*AgentRoutingStats {
	return re.internal.GetAgentStats()
}

// GetHistory returns recent routing decisions for analysis.
func (re *RoutingEngine) GetHistory(limit int) []*RoutingDecision {
	internalHistory := re.internal.GetHistory(limit)
	result := make([]*RoutingDecision, len(internalHistory))
	for i, d := range internalHistory {
		result[i] = &RoutingDecision{
			ID:              d.ID,
			Task:            d.Task,
			TaskType:        d.TaskType,
			SelectedAgent:   d.SelectedAgent,
			AgentType:       d.AgentType,
			Confidence:      d.Confidence,
			Success:         d.Success,
			Timestamp:       d.Timestamp,
			ExecutionTimeMs: d.ExecutionTimeMs,
		}
	}
	return result
}

// RoutingDecision represents a historical routing decision for learning analysis.
type RoutingDecision struct {
	ID              string  `json:"id"`
	Task            string  `json:"task"`
	TaskType        string  `json:"taskType"`
	SelectedAgent   string  `json:"selectedAgent"`
	AgentType       string  `json:"agentType"`
	Confidence      float64 `json:"confidence"`
	Success         bool    `json:"success"`
	Timestamp       int64   `json:"timestamp"`
	ExecutionTimeMs int64   `json:"executionTimeMs,omitempty"`
}

// ============================================================================
// Session Management
// ============================================================================

// Session state constants
const (
	SessionStateCreated  = shared.SessionStateCreated
	SessionStateReady    = shared.SessionStateReady
	SessionStateActive   = shared.SessionStateActive
	SessionStateClosing  = shared.SessionStateClosing
	SessionStateClosed   = shared.SessionStateClosed
	SessionStateExpired  = shared.SessionStateExpired
	SessionStateError    = shared.SessionStateError
)

// Transport type constants
const (
	TransportStdio     = shared.TransportStdio
	TransportHTTP      = shared.TransportHTTP
	TransportWebSocket = shared.TransportWebSocket
	TransportInProcess = shared.TransportInProcess
)

// SessionManager manages MCP sessions.
type SessionManager struct {
	internal *mcpsessions.SessionManager
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(config SessionConfig) *SessionManager {
	return &SessionManager{internal: mcpsessions.NewSessionManager(config)}
}

// NewSessionManagerWithDefaults creates a SessionManager with default configuration.
func NewSessionManagerWithDefaults() *SessionManager {
	return &SessionManager{internal: mcpsessions.NewSessionManagerWithDefaults()}
}

// Initialize starts the SessionManager.
func (sm *SessionManager) Initialize() error {
	return sm.internal.Initialize()
}

// Shutdown stops the SessionManager.
func (sm *SessionManager) Shutdown() error {
	return sm.internal.Shutdown()
}

// CreateSession creates a new session.
func (sm *SessionManager) CreateSession(transport TransportType) (*Session, error) {
	return sm.internal.CreateSession(transport)
}

// GetSession returns a session by ID.
func (sm *SessionManager) GetSession(id string) (*Session, error) {
	return sm.internal.GetSession(id)
}

// UpdateActivity updates the last activity timestamp for a session.
func (sm *SessionManager) UpdateActivity(id string) error {
	return sm.internal.UpdateActivity(id)
}

// InitializeSession marks a session as initialized.
func (sm *SessionManager) InitializeSession(id string, clientInfo *SessionClientInfo, protocolVersion string) error {
	return sm.internal.InitializeSession(id, clientInfo, protocolVersion)
}

// CloseSession closes a session.
func (sm *SessionManager) CloseSession(id, reason string) error {
	return sm.internal.CloseSession(id, reason)
}

// AddAgent adds an agent to a session.
func (sm *SessionManager) AddAgent(sessionID, agentID string) error {
	return sm.internal.AddAgent(sessionID, agentID)
}

// RemoveAgent removes an agent from a session.
func (sm *SessionManager) RemoveAgent(sessionID, agentID string) error {
	return sm.internal.RemoveAgent(sessionID, agentID)
}

// AddTask adds a task to a session.
func (sm *SessionManager) AddTask(sessionID, taskID string) error {
	return sm.internal.AddTask(sessionID, taskID)
}

// RemoveTask removes a task from a session.
func (sm *SessionManager) RemoveTask(sessionID, taskID string) error {
	return sm.internal.RemoveTask(sessionID, taskID)
}

// ListSessions returns sessions matching the filter.
func (sm *SessionManager) ListSessions(filter *SessionListRequest) *SessionListResult {
	return sm.internal.ListSessions(filter)
}

// CleanupExpiredSessions removes expired sessions.
func (sm *SessionManager) CleanupExpiredSessions() int {
	return sm.internal.CleanupExpiredSessions()
}

// GetStats returns session statistics.
func (sm *SessionManager) GetStats() *SessionStats {
	return sm.internal.GetStats()
}

// GetConfig returns the session configuration.
func (sm *SessionManager) GetConfig() SessionConfig {
	return sm.internal.GetConfig()
}

// SessionCount returns the total number of sessions.
func (sm *SessionManager) SessionCount() int {
	return sm.internal.SessionCount()
}

// ActiveSessionCount returns the number of active sessions.
func (sm *SessionManager) ActiveSessionCount() int {
	return sm.internal.ActiveSessionCount()
}

// SaveSession saves a session to disk.
func (sm *SessionManager) SaveSession(req *SessionSaveRequest) (*SessionSaveResult, error) {
	return sm.internal.SaveSession(req)
}

// RestoreSession restores a session from disk.
func (sm *SessionManager) RestoreSession(req *SessionRestoreRequest) (*SessionRestoreResult, error) {
	return sm.internal.RestoreSession(req)
}

// ListSavedSessions lists saved sessions.
func (sm *SessionManager) ListSavedSessions(tags []string) ([]*SessionSummary, error) {
	return sm.internal.ListSavedSessions(tags)
}

// DefaultSessionConfig returns the default session configuration.
func DefaultSessionConfig() SessionConfig {
	return shared.DefaultSessionConfig()
}
