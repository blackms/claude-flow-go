// Package shared provides shared types used across all modules in claude-flow-go.
package shared

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// Agent Types
// ============================================================================

// AgentStatus represents the current status of an agent.
type AgentStatus string

const (
	AgentStatusActive     AgentStatus = "active"
	AgentStatusIdle       AgentStatus = "idle"
	AgentStatusBusy       AgentStatus = "busy"
	AgentStatusTerminated AgentStatus = "terminated"
	AgentStatusError      AgentStatus = "error"
)

// AgentRole represents the role of an agent in a swarm.
type AgentRole string

const (
	AgentRoleLeader AgentRole = "leader"
	AgentRoleWorker AgentRole = "worker"
	AgentRolePeer   AgentRole = "peer"
)

// AgentType represents the type of an agent.
type AgentType string

const (
	AgentTypeCoder       AgentType = "coder"
	AgentTypeTester      AgentType = "tester"
	AgentTypeReviewer    AgentType = "reviewer"
	AgentTypeCoordinator AgentType = "coordinator"
	AgentTypeDesigner    AgentType = "designer"
	AgentTypeDeployer    AgentType = "deployer"
)

// AgentConfig holds configuration for creating an agent.
type AgentConfig struct {
	ID           string                 `json:"id"`
	Type         AgentType              `json:"type"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Role         AgentRole              `json:"role,omitempty"`
	Parent       string                 `json:"parent,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Agent represents an AI agent in the system.
type Agent struct {
	ID           string                 `json:"id"`
	Type         AgentType              `json:"type"`
	Status       AgentStatus            `json:"status"`
	Capabilities []string               `json:"capabilities"`
	Role         AgentRole              `json:"role,omitempty"`
	Parent       string                 `json:"parent,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    int64                  `json:"createdAt"`
	LastActive   int64                  `json:"lastActive"`
}

// ============================================================================
// Task Types
// ============================================================================

// TaskPriority represents the priority of a task.
type TaskPriority string

const (
	PriorityHigh   TaskPriority = "high"
	PriorityMedium TaskPriority = "medium"
	PriorityLow    TaskPriority = "low"
)

// TaskStatus represents the current status of a task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in-progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// TaskType represents the type of a task.
type TaskType string

const (
	TaskTypeCode     TaskType = "code"
	TaskTypeTest     TaskType = "test"
	TaskTypeReview   TaskType = "review"
	TaskTypeDesign   TaskType = "design"
	TaskTypeDeploy   TaskType = "deploy"
	TaskTypeWorkflow TaskType = "workflow"
)

// Task represents a task to be executed by agents.
type Task struct {
	ID           string                 `json:"id"`
	Type         TaskType               `json:"type"`
	Description  string                 `json:"description"`
	Priority     TaskPriority           `json:"priority"`
	Status       TaskStatus             `json:"status,omitempty"`
	AssignedTo   string                 `json:"assignedTo,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Workflow     *WorkflowDefinition    `json:"workflow,omitempty"`
	OnExecute    func(context.Context) error `json:"-"`
	OnRollback   func(context.Context) error `json:"-"`
}

// TaskResult represents the result of a task execution.
type TaskResult struct {
	TaskID   string      `json:"taskId"`
	Status   TaskStatus  `json:"status"`
	Result   interface{} `json:"result,omitempty"`
	Error    string      `json:"error,omitempty"`
	Duration int64       `json:"duration,omitempty"`
	AgentID  string      `json:"agentId,omitempty"`
}

// TaskAssignment represents a task assignment to an agent.
type TaskAssignment struct {
	TaskID     string       `json:"taskId"`
	AgentID    string       `json:"agentId"`
	AssignedAt int64        `json:"assignedAt"`
	Priority   TaskPriority `json:"priority"`
}

// ============================================================================
// Memory Types
// ============================================================================

// MemoryType represents the type of a memory entry.
type MemoryType string

const (
	MemoryTypeTask         MemoryType = "task"
	MemoryTypeContext      MemoryType = "context"
	MemoryTypeEvent        MemoryType = "event"
	MemoryTypeTaskStart    MemoryType = "task-start"
	MemoryTypeTaskComplete MemoryType = "task-complete"
	MemoryTypeWorkflow     MemoryType = "workflow-state"
)

// Memory represents a memory entry in the system.
type Memory struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agentId"`
	Content   string                 `json:"content"`
	Type      MemoryType             `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Embedding []float64              `json:"embedding,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryQuery represents a query for memory entries.
type MemoryQuery struct {
	AgentID   string                 `json:"agentId,omitempty"`
	Type      MemoryType             `json:"type,omitempty"`
	TimeRange *TimeRange             `json:"timeRange,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Limit     int                    `json:"limit,omitempty"`
	Offset    int                    `json:"offset,omitempty"`
}

// TimeRange represents a time range for queries.
type TimeRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// MemorySearchResult extends Memory with similarity score.
type MemorySearchResult struct {
	Memory
	Similarity float64 `json:"similarity,omitempty"`
}

// ============================================================================
// Workflow Types
// ============================================================================

// WorkflowStatus represents the status of a workflow.
type WorkflowStatus string

const (
	WorkflowStatusPending    WorkflowStatus = "pending"
	WorkflowStatusInProgress WorkflowStatus = "in-progress"
	WorkflowStatusPaused     WorkflowStatus = "paused"
	WorkflowStatusCompleted  WorkflowStatus = "completed"
	WorkflowStatusFailed     WorkflowStatus = "failed"
	WorkflowStatusCancelled  WorkflowStatus = "cancelled"
)

// WorkflowDefinition defines a workflow with tasks.
type WorkflowDefinition struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Tasks             []Task `json:"tasks"`
	Debug             bool   `json:"debug,omitempty"`
	RollbackOnFailure bool   `json:"rollbackOnFailure,omitempty"`
}

// WorkflowState represents the current state of a workflow.
type WorkflowState struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Tasks          []Task         `json:"tasks"`
	Status         WorkflowStatus `json:"status"`
	CompletedTasks []string       `json:"completedTasks"`
	CurrentTask    string         `json:"currentTask,omitempty"`
	StartedAt      int64          `json:"startedAt,omitempty"`
	CompletedAt    int64          `json:"completedAt,omitempty"`
}

// WorkflowResult represents the result of a workflow execution.
type WorkflowResult struct {
	ID             string   `json:"id"`
	Status         string   `json:"status"`
	TasksCompleted int      `json:"tasksCompleted"`
	TasksFailed    int      `json:"tasksFailed,omitempty"`
	Errors         []error  `json:"-"`
	ErrorMessages  []string `json:"errors,omitempty"`
	ExecutionOrder []string `json:"executionOrder,omitempty"`
	Duration       int64    `json:"duration,omitempty"`
}

// WorkflowMetrics provides metrics about workflow execution.
type WorkflowMetrics struct {
	TasksTotal          int     `json:"tasksTotal"`
	TasksCompleted      int     `json:"tasksCompleted"`
	TasksFailed         int     `json:"tasksFailed,omitempty"`
	TotalDuration       int64   `json:"totalDuration"`
	AverageTaskDuration float64 `json:"averageTaskDuration"`
	SuccessRate         float64 `json:"successRate"`
}

// WorkflowDebugInfo provides debug information for workflows.
type WorkflowDebugInfo struct {
	ExecutionTrace  []ExecutionTraceEntry          `json:"executionTrace"`
	TaskTimings     map[string]TaskTiming          `json:"taskTimings"`
	MemorySnapshots []MemorySnapshot               `json:"memorySnapshots"`
	EventLog        []EventLogEntry                `json:"eventLog"`
}

// ExecutionTraceEntry represents a single execution trace entry.
type ExecutionTraceEntry struct {
	TaskID    string `json:"taskId"`
	Timestamp int64  `json:"timestamp"`
	Action    string `json:"action"`
}

// TaskTiming represents timing information for a task.
type TaskTiming struct {
	Start    int64 `json:"start"`
	End      int64 `json:"end"`
	Duration int64 `json:"duration"`
}

// MemorySnapshot represents a snapshot of memory state.
type MemorySnapshot struct {
	Timestamp int64                  `json:"timestamp"`
	Snapshot  map[string]interface{} `json:"snapshot"`
}

// EventLogEntry represents an event log entry.
type EventLogEntry struct {
	Timestamp int64       `json:"timestamp"`
	Event     string      `json:"event"`
	Data      interface{} `json:"data"`
}

// ============================================================================
// Swarm/Coordination Types
// ============================================================================

// SwarmTopology represents the topology of a swarm.
type SwarmTopology string

const (
	TopologyHierarchical SwarmTopology = "hierarchical"
	TopologyMesh         SwarmTopology = "mesh"
	TopologySimple       SwarmTopology = "simple"
	TopologyAdaptive     SwarmTopology = "adaptive"
)

// SwarmConfig holds configuration for a swarm.
type SwarmConfig struct {
	Topology      SwarmTopology  `json:"topology"`
	MemoryBackend MemoryBackend  `json:"-"`
	MaxAgents     int            `json:"maxAgents,omitempty"`
}

// SwarmState represents the current state of a swarm.
type SwarmState struct {
	Agents            []Agent       `json:"agents"`
	Topology          SwarmTopology `json:"topology"`
	Leader            string        `json:"leader,omitempty"`
	ActiveConnections int           `json:"activeConnections,omitempty"`
}

// SwarmHierarchy represents the hierarchy of a swarm.
type SwarmHierarchy struct {
	Leader  string         `json:"leader"`
	Workers []WorkerInfo   `json:"workers"`
}

// WorkerInfo represents information about a worker agent.
type WorkerInfo struct {
	ID     string `json:"id"`
	Parent string `json:"parent"`
}

// MeshConnection represents a connection in a mesh topology.
type MeshConnection struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Type   string  `json:"type"`
	Weight float64 `json:"weight,omitempty"`
}

// AgentMessage represents a message between agents.
type AgentMessage struct {
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp int64                  `json:"timestamp,omitempty"`
}

// AgentMetrics represents metrics for an agent.
type AgentMetrics struct {
	AgentID              string  `json:"agentId"`
	TasksCompleted       int     `json:"tasksCompleted"`
	TasksFailed          int     `json:"tasksFailed,omitempty"`
	AverageExecutionTime float64 `json:"averageExecutionTime"`
	SuccessRate          float64 `json:"successRate"`
	Health               string  `json:"health"`
}

// ConsensusDecision represents a decision for consensus.
type ConsensusDecision struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// ConsensusResult represents the result of a consensus vote.
type ConsensusResult struct {
	Decision         interface{}  `json:"decision"`
	Votes            []AgentVote  `json:"votes"`
	ConsensusReached bool         `json:"consensusReached"`
}

// AgentVote represents a vote from an agent.
type AgentVote struct {
	AgentID string      `json:"agentId"`
	Vote    interface{} `json:"vote"`
}

// ============================================================================
// Plugin Types
// ============================================================================

// Plugin defines the interface for plugins.
type Plugin interface {
	ID() string
	Name() string
	Version() string
	Description() string
	Author() string
	Homepage() string
	Priority() int
	Dependencies() []string
	ConfigSchema() map[string]interface{}
	MinCoreVersion() string
	MaxCoreVersion() string
	Initialize(config map[string]interface{}) error
	Shutdown() error
	GetExtensionPoints() []ExtensionPoint
}

// ExtensionPoint represents an extension point provided by a plugin.
type ExtensionPoint struct {
	Name     string                                    `json:"name"`
	Handler  func(ctx context.Context, data interface{}) (interface{}, error) `json:"-"`
	Priority int                                       `json:"priority,omitempty"`
}

// PluginMetadata represents metadata about a plugin.
type PluginMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
}

// PluginManager defines the interface for plugin management.
type PluginManager interface {
	LoadPlugin(plugin Plugin, config map[string]interface{}) error
	UnloadPlugin(pluginID string) error
	ReloadPlugin(pluginID string, plugin Plugin) error
	ListPlugins() []Plugin
	GetPluginMetadata(pluginID string) *PluginMetadata
	InvokeExtensionPoint(ctx context.Context, name string, data interface{}) ([]interface{}, error)
	GetCoreVersion() string
	Initialize() error
	Shutdown() error
}

// ============================================================================
// MCP Types
// ============================================================================

// MCPServerOptions holds options for the MCP server.
type MCPServerOptions struct {
	Tools []MCPToolProvider `json:"-"`
	Port  int               `json:"port,omitempty"`
	Host  string            `json:"host,omitempty"`
}

// MCPTool represents an MCP tool definition.
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// MCPToolProvider defines the interface for MCP tool providers.
type MCPToolProvider interface {
	Execute(ctx context.Context, toolName string, params map[string]interface{}) (*MCPToolResult, error)
	GetTools() []MCPTool
}

// MCPToolResult represents the result of an MCP tool execution.
type MCPToolResult struct {
	Success bool                   `json:"success"`
	Agent   *Agent                 `json:"agent,omitempty"`
	Agents  []Agent                `json:"agents,omitempty"`
	Metrics *AgentMetrics          `json:"metrics,omitempty"`
	Memories []Memory              `json:"memories,omitempty"`
	Results []MemorySearchResult   `json:"results,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
	Valid   bool                   `json:"valid,omitempty"`
	Errors  []string               `json:"errors,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// MCPRequest represents an MCP request.
type MCPRequest struct {
	ID     string                 `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// MCPResponse represents an MCP response.
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ============================================================================
// Backend Interfaces
// ============================================================================

// MemoryBackend defines the interface for memory storage backends.
type MemoryBackend interface {
	Initialize() error
	Close() error
	Store(memory Memory) (Memory, error)
	Retrieve(id string) (*Memory, error)
	Update(memory Memory) error
	Delete(id string) error
	Query(query MemoryQuery) ([]Memory, error)
	VectorSearch(embedding []float64, k int) ([]MemorySearchResult, error)
	ClearAgent(agentID string) error
}

// SQLiteOptions holds options for SQLite backend.
type SQLiteOptions struct {
	DBPath  string `json:"dbPath"`
	Timeout int    `json:"timeout,omitempty"`
}

// AgentDBOptions holds options for AgentDB backend.
type AgentDBOptions struct {
	DBPath         string `json:"dbPath"`
	Dimensions     int    `json:"dimensions,omitempty"`
	HNSWM          int    `json:"hnswM,omitempty"`
	EFConstruction int    `json:"efConstruction,omitempty"`
}

// ============================================================================
// Event Types
// ============================================================================

// EventType represents the type of an event.
type EventType string

const (
	EventAgentSpawned     EventType = "agent:spawned"
	EventAgentTerminated  EventType = "agent:terminated"
	EventAgentMessage     EventType = "agent:message"
	EventTaskStarted      EventType = "task:started"
	EventTaskCompleted    EventType = "task:completed"
	EventTaskFailed       EventType = "task:failed"
	EventWorkflowStarted  EventType = "workflow:started"
	EventWorkflowComplete EventType = "workflow:taskComplete"
	EventWorkflowCompleted EventType = "workflow:completed"
	EventWorkflowFailed   EventType = "workflow:failed"
	EventPluginLoaded     EventType = "plugin:loaded"
	EventPluginUnloaded   EventType = "plugin:unloaded"
	EventPluginError      EventType = "plugin:error"
)

// Event represents a generic event in the system.
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
}

// ============================================================================
// Error Types
// ============================================================================

// V3Error is the base error type for all claude-flow errors.
type V3Error struct {
	Message string
	Code    string
	Details map[string]interface{}
}

func (e *V3Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewV3Error creates a new V3Error.
func NewV3Error(message, code string, details map[string]interface{}) *V3Error {
	return &V3Error{
		Message: message,
		Code:    code,
		Details: details,
	}
}

// ValidationError represents a validation error.
type ValidationError struct {
	V3Error
}

// NewValidationError creates a new ValidationError.
func NewValidationError(message string, details map[string]interface{}) *ValidationError {
	return &ValidationError{
		V3Error: V3Error{
			Message: message,
			Code:    "VALIDATION_ERROR",
			Details: details,
		},
	}
}

// ExecutionError represents an execution error.
type ExecutionError struct {
	V3Error
}

// NewExecutionError creates a new ExecutionError.
func NewExecutionError(message string, details map[string]interface{}) *ExecutionError {
	return &ExecutionError{
		V3Error: V3Error{
			Message: message,
			Code:    "EXECUTION_ERROR",
			Details: details,
		},
	}
}

// CoordinationError represents a coordination error.
type CoordinationError struct {
	V3Error
}

// NewCoordinationError creates a new CoordinationError.
func NewCoordinationError(message string, details map[string]interface{}) *CoordinationError {
	return &CoordinationError{
		V3Error: V3Error{
			Message: message,
			Code:    "COORDINATION_ERROR",
			Details: details,
		},
	}
}

// PluginError represents a plugin error.
type PluginError struct {
	V3Error
}

// NewPluginError creates a new PluginError.
func NewPluginError(message string, details map[string]interface{}) *PluginError {
	return &PluginError{
		V3Error: V3Error{
			Message: message,
			Code:    "PLUGIN_ERROR",
			Details: details,
		},
	}
}

// MemoryError represents a memory error.
type MemoryError struct {
	V3Error
}

// NewMemoryError creates a new MemoryError.
func NewMemoryError(message string, details map[string]interface{}) *MemoryError {
	return &MemoryError{
		V3Error: V3Error{
			Message: message,
			Code:    "MEMORY_ERROR",
			Details: details,
		},
	}
}

// ============================================================================
// Utility Functions
// ============================================================================

// Now returns the current time in milliseconds.
func Now() int64 {
	return time.Now().UnixMilli()
}
