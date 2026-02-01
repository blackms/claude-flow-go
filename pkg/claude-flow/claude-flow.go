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
