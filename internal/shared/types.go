// Package shared provides shared types used across all modules in claude-flow-go.
package shared

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// Errors
// ============================================================================

var (
	// ErrMaxAgentsReached is returned when the maximum number of agents is reached.
	ErrMaxAgentsReached = errors.New("maximum number of agents reached")
	// ErrAgentNotFound is returned when an agent is not found.
	ErrAgentNotFound = errors.New("agent not found")
	// ErrPoolNotFound is returned when a pool is not found.
	ErrPoolNotFound = errors.New("pool not found")
	// ErrInvalidAgentType is returned when an invalid agent type is provided.
	ErrInvalidAgentType = errors.New("invalid agent type")
	// ErrResourceNotFound is returned when a resource is not found.
	ErrResourceNotFound = errors.New("resource not found")
	// ErrPromptNotFound is returned when a prompt is not found.
	ErrPromptNotFound = errors.New("prompt not found")
	// ErrProviderNotFound is returned when an LLM provider is not found.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrNoProvidersAvailable is returned when no LLM providers are available.
	ErrNoProvidersAvailable = errors.New("no providers available")
	// ErrSamplingTimeout is returned when a sampling request times out.
	ErrSamplingTimeout = errors.New("sampling request timed out")
	// ErrMissingRequiredArgument is returned when a required argument is missing.
	ErrMissingRequiredArgument = errors.New("missing required argument")
	// ErrMaxPromptsReached is returned when the maximum number of prompts is reached.
	ErrMaxPromptsReached = errors.New("maximum number of prompts reached")
)

// idCounter is used for generating unique IDs.
var idCounter int64
var idMu sync.Mutex

// GenerateID generates a unique ID with a given prefix.
func GenerateID(prefix string) string {
	idMu.Lock()
	idCounter++
	id := idCounter
	idMu.Unlock()
	return fmt.Sprintf("%s-%d-%d", prefix, Now(), id)
}

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
	// Basic agent types
	AgentTypeCoder       AgentType = "coder"
	AgentTypeTester      AgentType = "tester"
	AgentTypeReviewer    AgentType = "reviewer"
	AgentTypeCoordinator AgentType = "coordinator"
	AgentTypeDesigner    AgentType = "designer"
	AgentTypeDeployer    AgentType = "deployer"

	// 15-Agent Domain Architecture Types
	// Queen Domain (Agent 1)
	AgentTypeQueen AgentType = "queen"

	// Security Domain (Agents 2-4)
	AgentTypeSecurityArchitect AgentType = "security-architect"
	AgentTypeCVERemediation    AgentType = "cve-remediation"
	AgentTypeThreatModeler     AgentType = "threat-modeler"

	// Core Domain (Agents 5-9)
	AgentTypeDDDDesigner      AgentType = "ddd-designer"
	AgentTypeMemorySpecialist AgentType = "memory-specialist"
	AgentTypeTypeModernizer   AgentType = "type-modernizer"
	AgentTypeSwarmSpecialist  AgentType = "swarm-specialist"
	AgentTypeMCPOptimizer     AgentType = "mcp-optimizer"

	// Integration Domain (Agents 10-12)
	AgentTypeAgenticFlow     AgentType = "agentic-flow"
	AgentTypeCLIDeveloper    AgentType = "cli-developer"
	AgentTypeNeuralIntegrator AgentType = "neural-integrator"

	// Support Domain (Agents 13-15)
	AgentTypeTDDTester           AgentType = "tdd-tester"
	AgentTypePerformanceEngineer AgentType = "performance-engineer"
	AgentTypeReleaseManager      AgentType = "release-manager"

	// Extended Agent Types (20+ types)
	AgentTypeResearcher         AgentType = "researcher"
	AgentTypeArchitect          AgentType = "architect"
	AgentTypeAnalyst            AgentType = "analyst"
	AgentTypeOptimizer          AgentType = "optimizer"
	AgentTypeSecurityAuditor    AgentType = "security-auditor"
	AgentTypeCoreArchitect      AgentType = "core-architect"
	AgentTypeTestArchitect      AgentType = "test-architect"
	AgentTypeIntegrationArchitect AgentType = "integration-architect"
	AgentTypeHooksDeveloper     AgentType = "hooks-developer"
	AgentTypeMCPSpecialist      AgentType = "mcp-specialist"
	AgentTypeDocumentationLead  AgentType = "documentation-lead"
	AgentTypeDevOpsEngineer     AgentType = "devops-engineer"
)

// AgentDomain represents a domain in the 15-agent architecture.
type AgentDomain string

const (
	DomainQueen       AgentDomain = "queen"
	DomainSecurity    AgentDomain = "security"
	DomainCore        AgentDomain = "core"
	DomainIntegration AgentDomain = "integration"
	DomainSupport     AgentDomain = "support"
)

// DomainConfig holds the configuration for a domain in the 15-agent architecture.
type DomainConfig struct {
	Name         AgentDomain `json:"name"`
	AgentNumbers []int       `json:"agentNumbers"`
	Priority     int         `json:"priority"`
	Capabilities []string    `json:"capabilities"`
	Description  string      `json:"description"`
}

// DefaultDomainConfigs returns the default domain configurations for the 15-agent architecture.
func DefaultDomainConfigs() []DomainConfig {
	return []DomainConfig{
		{
			Name:         DomainQueen,
			AgentNumbers: []int{1},
			Priority:     0,
			Capabilities: []string{"coordination", "planning", "oversight", "consensus"},
			Description:  "Top-level swarm coordination and orchestration",
		},
		{
			Name:         DomainSecurity,
			AgentNumbers: []int{2, 3, 4},
			Priority:     1,
			Capabilities: []string{"security-architecture", "cve-remediation", "security-testing", "threat-modeling"},
			Description:  "Security architecture, CVE fixes, and security testing",
		},
		{
			Name:         DomainCore,
			AgentNumbers: []int{5, 6, 7, 8, 9},
			Priority:     2,
			Capabilities: []string{"ddd-design", "type-modernization", "memory-unification", "swarm-coordination", "mcp-optimization"},
			Description:  "Core architecture, DDD, memory unification, and MCP optimization",
		},
		{
			Name:         DomainIntegration,
			AgentNumbers: []int{10, 11, 12},
			Priority:     3,
			Capabilities: []string{"agentic-flow-integration", "cli-modernization", "neural-integration", "hooks-system"},
			Description:  "agentic-flow integration, CLI modernization, and neural features",
		},
		{
			Name:         DomainSupport,
			AgentNumbers: []int{13, 14, 15},
			Priority:     4,
			Capabilities: []string{"tdd-testing", "performance-benchmarking", "deployment", "release-management"},
			Description:  "Testing, performance optimization, and deployment",
		},
	}
}

// GetDomainForAgentNumber returns the domain for a given agent number (1-15).
func GetDomainForAgentNumber(agentNumber int) AgentDomain {
	switch {
	case agentNumber == 1:
		return DomainQueen
	case agentNumber >= 2 && agentNumber <= 4:
		return DomainSecurity
	case agentNumber >= 5 && agentNumber <= 9:
		return DomainCore
	case agentNumber >= 10 && agentNumber <= 12:
		return DomainIntegration
	case agentNumber >= 13 && agentNumber <= 15:
		return DomainSupport
	default:
		return ""
	}
}

// GetAgentTypeForNumber returns the default agent type for a given agent number (1-15).
func GetAgentTypeForNumber(agentNumber int) AgentType {
	switch agentNumber {
	case 1:
		return AgentTypeQueen
	case 2:
		return AgentTypeSecurityArchitect
	case 3:
		return AgentTypeCVERemediation
	case 4:
		return AgentTypeThreatModeler
	case 5:
		return AgentTypeDDDDesigner
	case 6:
		return AgentTypeMemorySpecialist
	case 7:
		return AgentTypeTypeModernizer
	case 8:
		return AgentTypeSwarmSpecialist
	case 9:
		return AgentTypeMCPOptimizer
	case 10:
		return AgentTypeAgenticFlow
	case 11:
		return AgentTypeCLIDeveloper
	case 12:
		return AgentTypeNeuralIntegrator
	case 13:
		return AgentTypeTDDTester
	case 14:
		return AgentTypePerformanceEngineer
	case 15:
		return AgentTypeReleaseManager
	default:
		return AgentTypeCoder
	}
}

// AgentConfig holds configuration for creating an agent.
type AgentConfig struct {
	ID           string                 `json:"id"`
	Type         AgentType              `json:"type"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Role         AgentRole              `json:"role,omitempty"`
	Parent       string                 `json:"parent,omitempty"`
	Domain       AgentDomain            `json:"domain,omitempty"`
	AgentNumber  int                    `json:"agentNumber,omitempty"`
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
	Domain       AgentDomain            `json:"domain,omitempty"`
	AgentNumber  int                    `json:"agentNumber,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    int64                  `json:"createdAt"`
	LastActive   int64                  `json:"lastActive"`
}

// AgentConfig holds configuration for creating an agent.
// (Moved to be after Agent struct for logical grouping)

// ============================================================================
// Agent Scoring Types (15-Agent Architecture)
// ============================================================================

// AgentScore represents the scoring of an agent for task delegation.
type AgentScore struct {
	AgentID          string  `json:"agentId"`
	CapabilityScore  float64 `json:"capabilityScore"`  // 0.4 weight
	LoadScore        float64 `json:"loadScore"`        // 0.25 weight
	PerformanceScore float64 `json:"performanceScore"` // 0.2 weight
	HealthScore      float64 `json:"healthScore"`      // 0.15 weight
	TotalScore       float64 `json:"totalScore"`
}

// CalculateTotalScore computes the weighted total score.
func (s *AgentScore) CalculateTotalScore() float64 {
	s.TotalScore = s.CapabilityScore*0.4 + s.LoadScore*0.25 + s.PerformanceScore*0.2 + s.HealthScore*0.15
	return s.TotalScore
}

// TaskAnalysis represents the analysis of a task by the Queen Coordinator.
type TaskAnalysis struct {
	TaskID              string                 `json:"taskId"`
	ComplexityScore     float64                `json:"complexityScore"`     // 0.0 - 1.0
	RequiredCapabilities []string              `json:"requiredCapabilities"`
	RecommendedDomain   AgentDomain            `json:"recommendedDomain"`
	EstimatedDuration   int64                  `json:"estimatedDuration"`   // milliseconds
	ParallelizationScore float64               `json:"parallelizationScore"` // 0.0 - 1.0
	PatternMatches      []string               `json:"patternMatches,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// DelegationResult represents the result of task delegation.
type DelegationResult struct {
	TaskID        string       `json:"taskId"`
	PrimaryAgent  AgentScore   `json:"primaryAgent"`
	BackupAgents  []AgentScore `json:"backupAgents,omitempty"`
	Domain        AgentDomain  `json:"domain"`
	Strategy      ExecutionStrategy `json:"strategy"`
	EstimatedTime int64        `json:"estimatedTime"` // milliseconds
}

// ExecutionStrategy represents the strategy for task execution.
type ExecutionStrategy string

const (
	StrategySequential   ExecutionStrategy = "sequential"
	StrategyParallel     ExecutionStrategy = "parallel"
	StrategyPipeline     ExecutionStrategy = "pipeline"
	StrategyFanOutFanIn  ExecutionStrategy = "fan-out-fan-in"
	StrategyHybrid       ExecutionStrategy = "hybrid"
)

// DomainHealth represents the health status of a domain.
type DomainHealth struct {
	Domain       AgentDomain `json:"domain"`
	HealthScore  float64     `json:"healthScore"`  // 0.0 - 1.0
	ActiveAgents int         `json:"activeAgents"`
	TotalAgents  int         `json:"totalAgents"`
	AvgLoad      float64     `json:"avgLoad"`      // 0.0 - 1.0
	Bottlenecks  []string    `json:"bottlenecks,omitempty"`
	LastCheck    int64       `json:"lastCheck"`
}

// AgentHealth represents the health status of an individual agent.
type AgentHealth struct {
	AgentID       string  `json:"agentId"`
	HealthScore   float64 `json:"healthScore"`   // 0.0 - 1.0
	CurrentLoad   float64 `json:"currentLoad"`   // 0.0 - 1.0
	TasksInQueue  int     `json:"tasksInQueue"`
	AvgResponseTime int64 `json:"avgResponseTime"` // milliseconds
	ErrorRate     float64 `json:"errorRate"`     // 0.0 - 1.0
	LastHeartbeat int64   `json:"lastHeartbeat"`
	IsAvailable   bool    `json:"isAvailable"`
}

// HealthAlert represents an alert from the health monitoring system.
type HealthAlert struct {
	ID        string      `json:"id"`
	Level     AlertLevel  `json:"level"`
	Domain    AgentDomain `json:"domain,omitempty"`
	AgentID   string      `json:"agentId,omitempty"`
	Message   string      `json:"message"`
	Timestamp int64       `json:"timestamp"`
}

// AlertLevel represents the severity level of an alert.
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// DomainMetrics represents metrics for a domain.
type DomainMetrics struct {
	Domain           AgentDomain `json:"domain"`
	TasksCompleted   int         `json:"tasksCompleted"`
	TasksFailed      int         `json:"tasksFailed"`
	AvgExecutionTime float64     `json:"avgExecutionTime"` // milliseconds
	SuccessRate      float64     `json:"successRate"`      // 0.0 - 1.0
	ThroughputPerSec float64     `json:"throughputPerSec"`
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
	TopologyRing         SwarmTopology = "ring"
	TopologyStar         SwarmTopology = "star"
	TopologyHybrid       SwarmTopology = "hybrid"
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

// ============================================================================
// Topology Manager Types
// ============================================================================

// TopologyNodeRole represents the role of a node in the topology.
type TopologyNodeRole string

const (
	TopologyRoleQueen       TopologyNodeRole = "queen"
	TopologyRoleWorker      TopologyNodeRole = "worker"
	TopologyRoleCoordinator TopologyNodeRole = "coordinator"
	TopologyRolePeer        TopologyNodeRole = "peer"
)

// TopologyNodeStatus represents the status of a node.
type TopologyNodeStatus string

const (
	TopologyStatusActive   TopologyNodeStatus = "active"
	TopologyStatusInactive TopologyNodeStatus = "inactive"
	TopologyStatusSyncing  TopologyNodeStatus = "syncing"
	TopologyStatusFailed   TopologyNodeStatus = "failed"
)

// TopologyNode represents a node in the topology.
type TopologyNode struct {
	ID          string                 `json:"id"`          // Format: "node_{agentId}"
	AgentID     string                 `json:"agentId"`     // Unique agent identifier
	Role        TopologyNodeRole       `json:"role"`        // queen, worker, coordinator, peer
	Status      TopologyNodeStatus     `json:"status"`      // active, inactive, syncing, failed
	Connections []string               `json:"connections"` // Connected agentIds
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TopologyEdge represents an edge between nodes.
type TopologyEdge struct {
	From          string  `json:"from"`          // Source agentId
	To            string  `json:"to"`            // Target agentId
	Weight        float64 `json:"weight"`        // Edge weight (default: 1)
	Bidirectional bool    `json:"bidirectional"` // True for mesh, false for hierarchical
	LatencyMs     int64   `json:"latencyMs,omitempty"`
}

// PartitionStrategy represents the partition strategy.
type PartitionStrategy string

const (
	PartitionStrategyHash       PartitionStrategy = "hash"
	PartitionStrategyRange      PartitionStrategy = "range"
	PartitionStrategyRoundRobin PartitionStrategy = "round-robin"
)

// TopologyPartition represents a partition in mesh/hybrid topologies.
type TopologyPartition struct {
	ID           string   `json:"id"`           // Format: "partition_{index}"
	Nodes        []string `json:"nodes"`        // AgentIds in partition
	Leader       string   `json:"leader"`       // Partition leader agentId
	ReplicaCount int      `json:"replicaCount"` // Replication count
}

// TopologyConfig holds configuration for the topology manager.
type TopologyConfig struct {
	Type              SwarmTopology     `json:"type"`
	MaxAgents         int               `json:"maxAgents"`
	ReplicationFactor int               `json:"replicationFactor"` // default: 2
	PartitionStrategy PartitionStrategy `json:"partitionStrategy"` // hash, range, round-robin
	FailoverEnabled   bool              `json:"failoverEnabled"`
	AutoRebalance     bool              `json:"autoRebalance"`
}

// DefaultTopologyConfig returns the default topology configuration.
func DefaultTopologyConfig() TopologyConfig {
	return TopologyConfig{
		Type:              TopologyMesh,
		MaxAgents:         100,
		ReplicationFactor: 2,
		PartitionStrategy: PartitionStrategyRoundRobin,
		FailoverEnabled:   true,
		AutoRebalance:     true,
	}
}

// TopologyState holds the current state of the topology.
type TopologyState struct {
	Nodes      []TopologyNode      `json:"nodes"`
	Edges      []TopologyEdge      `json:"edges"`
	Leader     string              `json:"leader,omitempty"`
	Partitions []TopologyPartition `json:"partitions,omitempty"`
}

// TopologyStats holds statistics about the topology.
type TopologyStats struct {
	NodeCount      int     `json:"nodeCount"`
	EdgeCount      int     `json:"edgeCount"`
	PartitionCount int     `json:"partitionCount"`
	AvgConnections float64 `json:"avgConnections"`
	LeaderID       string  `json:"leaderId,omitempty"`
}

// ============================================================================
// Federation Hub Types
// ============================================================================

// EphemeralAgentStatus represents the lifecycle status of an ephemeral agent.
type EphemeralAgentStatus string

const (
	EphemeralStatusSpawning   EphemeralAgentStatus = "spawning"
	EphemeralStatusActive     EphemeralAgentStatus = "active"
	EphemeralStatusCompleting EphemeralAgentStatus = "completing"
	EphemeralStatusTerminated EphemeralAgentStatus = "terminated"
)

// EphemeralAgent represents a TTL-based ephemeral agent.
type EphemeralAgent struct {
	ID          string                 `json:"id"`
	SwarmID     string                 `json:"swarmId"`
	Type        string                 `json:"type"`
	Task        string                 `json:"task"`
	Status      EphemeralAgentStatus   `json:"status"`
	TTL         int64                  `json:"ttl"`         // TTL in milliseconds
	CreatedAt   int64                  `json:"createdAt"`   // Unix timestamp ms
	ExpiresAt   int64                  `json:"expiresAt"`   // Unix timestamp ms
	CompletedAt int64                  `json:"completedAt,omitempty"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SpawnEphemeralOptions holds options for spawning an ephemeral agent.
type SpawnEphemeralOptions struct {
	SwarmID           string                 `json:"swarmId,omitempty"`   // Optional, auto-selects if empty
	Type              string                 `json:"type"`                // Agent type
	Task              string                 `json:"task"`                // Task description
	TTL               int64                  `json:"ttl,omitempty"`       // TTL in milliseconds (default: 60000)
	Capabilities      []string               `json:"capabilities,omitempty"`
	Priority          int                    `json:"priority,omitempty"`
	WaitForCompletion bool                   `json:"waitForCompletion,omitempty"`
	CompletionTimeout int64                  `json:"completionTimeout,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// SpawnResult represents the result of spawning an ephemeral agent.
type SpawnResult struct {
	AgentID      string      `json:"agentId"`
	SwarmID      string      `json:"swarmId"`
	Status       string      `json:"status"` // spawned, queued, failed
	EstimatedTTL int64       `json:"estimatedTtl"`
	Result       interface{} `json:"result,omitempty"`
	Error        string      `json:"error,omitempty"`
}

// SwarmRegistrationStatus represents the status of a registered swarm.
type SwarmRegistrationStatus string

const (
	SwarmStatusActive   SwarmRegistrationStatus = "active"
	SwarmStatusInactive SwarmRegistrationStatus = "inactive"
	SwarmStatusDegraded SwarmRegistrationStatus = "degraded"
)

// SwarmRegistration represents a swarm registered with the federation.
type SwarmRegistration struct {
	SwarmID       string                  `json:"swarmId"`
	Name          string                  `json:"name"`
	Endpoint      string                  `json:"endpoint,omitempty"`
	Capabilities  []string                `json:"capabilities"`
	MaxAgents     int                     `json:"maxAgents"`
	CurrentAgents int                     `json:"currentAgents"`
	Status        SwarmRegistrationStatus `json:"status"`
	RegisteredAt  int64                   `json:"registeredAt"`
	LastHeartbeat int64                   `json:"lastHeartbeat"`
	Metadata      map[string]interface{}  `json:"metadata,omitempty"`
}

// FederationMessageType represents the type of federation message.
type FederationMessageType string

const (
	FederationMsgBroadcast FederationMessageType = "broadcast"
	FederationMsgDirect    FederationMessageType = "direct"
	FederationMsgConsensus FederationMessageType = "consensus"
	FederationMsgHeartbeat FederationMessageType = "heartbeat"
)

// FederationMessage represents a message in the federation.
type FederationMessage struct {
	ID            string                `json:"id"`
	Type          FederationMessageType `json:"type"`
	SourceSwarmID string                `json:"sourceSwarmId"`
	TargetSwarmID string                `json:"targetSwarmId,omitempty"` // Empty for broadcast
	Payload       interface{}           `json:"payload"`
	Timestamp     int64                 `json:"timestamp"`
	TTL           int64                 `json:"ttl,omitempty"`
}

// FederationProposalStatus represents the status of a federation proposal.
type FederationProposalStatus string

const (
	FederationProposalPending  FederationProposalStatus = "pending"
	FederationProposalAccepted FederationProposalStatus = "accepted"
	FederationProposalRejected FederationProposalStatus = "rejected"
)

// FederationProposal represents a consensus proposal in the federation.
type FederationProposal struct {
	ID         string                   `json:"id"`
	ProposerID string                   `json:"proposerId"` // SwarmID of proposer
	Type       string                   `json:"type"`
	Value      interface{}              `json:"value"`
	Votes      map[string]bool          `json:"votes"` // SwarmID -> approve/reject
	Status     FederationProposalStatus `json:"status"`
	CreatedAt  int64                    `json:"createdAt"`
	ExpiresAt  int64                    `json:"expiresAt"`
}

// FederationEventType represents the type of federation event.
type FederationEventType string

const (
	// Swarm events
	FederationEventSwarmJoined   FederationEventType = "swarm_joined"
	FederationEventSwarmLeft     FederationEventType = "swarm_left"
	FederationEventSwarmDegraded FederationEventType = "swarm_degraded"

	// Agent events
	FederationEventAgentSpawned   FederationEventType = "agent_spawned"
	FederationEventAgentCompleted FederationEventType = "agent_completed"
	FederationEventAgentFailed    FederationEventType = "agent_failed"
	FederationEventAgentExpired   FederationEventType = "agent_expired"

	// Message events
	FederationEventMessageSent     FederationEventType = "message_sent"
	FederationEventMessageReceived FederationEventType = "message_received"

	// Consensus events
	FederationEventConsensusStarted   FederationEventType = "consensus_started"
	FederationEventConsensusCompleted FederationEventType = "consensus_completed"

	// System events
	FederationEventSynced FederationEventType = "federation_synced"
)

// FederationEvent represents an event in the federation.
type FederationEvent struct {
	Type      FederationEventType `json:"type"`
	SwarmID   string              `json:"swarmId,omitempty"`
	AgentID   string              `json:"agentId,omitempty"`
	Data      interface{}         `json:"data,omitempty"`
	Timestamp int64               `json:"timestamp"`
}

// FederationConfig holds configuration for the federation hub.
type FederationConfig struct {
	FederationID       string `json:"federationId"`
	EnableConsensus    bool   `json:"enableConsensus"`
	ConsensusQuorum    float64 `json:"consensusQuorum"`    // 0.0-1.0, default 0.66
	HeartbeatInterval  int64  `json:"heartbeatInterval"`   // ms
	SyncInterval       int64  `json:"syncInterval"`        // ms
	CleanupInterval    int64  `json:"cleanupInterval"`     // ms
	DefaultAgentTTL    int64  `json:"defaultAgentTtl"`     // ms
	ProposalTimeout    int64  `json:"proposalTimeout"`     // ms
	MaxMessageHistory  int    `json:"maxMessageHistory"`
	AutoCleanupEnabled bool   `json:"autoCleanupEnabled"`
}

// DefaultFederationConfig returns the default federation configuration.
func DefaultFederationConfig() FederationConfig {
	return FederationConfig{
		FederationID:       "default",
		EnableConsensus:    true,
		ConsensusQuorum:    0.66,
		HeartbeatInterval:  5000,  // 5s
		SyncInterval:       10000, // 10s
		CleanupInterval:    5000,  // 5s
		DefaultAgentTTL:    60000, // 60s
		ProposalTimeout:    30000, // 30s
		MaxMessageHistory:  1000,
		AutoCleanupEnabled: true,
	}
}

// FederationStats holds statistics about the federation.
type FederationStats struct {
	TotalSwarms        int     `json:"totalSwarms"`
	ActiveSwarms       int     `json:"activeSwarms"`
	TotalAgents        int     `json:"totalAgents"`
	ActiveAgents       int     `json:"activeAgents"`
	TotalMessages      int64   `json:"totalMessages"`
	PendingProposals   int     `json:"pendingProposals"`
	AcceptedProposals  int     `json:"acceptedProposals"`
	RejectedProposals  int     `json:"rejectedProposals"`
	AvgSpawnTimeMs     float64 `json:"avgSpawnTimeMs"`
	AvgMessageLatencyMs float64 `json:"avgMessageLatencyMs"`
}

// ============================================================================
// Attention Mechanism Types
// ============================================================================

// AttentionMechanism represents the type of attention mechanism.
type AttentionMechanism string

const (
	AttentionFlash      AttentionMechanism = "flash"
	AttentionMultiHead  AttentionMechanism = "multi_head"
	AttentionLinear     AttentionMechanism = "linear"
	AttentionHyperbolic AttentionMechanism = "hyperbolic"
	AttentionMoE        AttentionMechanism = "moe"
	AttentionGraphRoPE  AttentionMechanism = "graph_rope"
)

// AttentionAgentOutput represents an agent's output for attention coordination.
type AttentionAgentOutput struct {
	AgentID    string                 `json:"agentId"`
	Content    string                 `json:"content"`
	Embedding  []float64              `json:"embedding,omitempty"`
	Confidence float64                `json:"confidence"`
	Tokens     int                    `json:"tokens,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AttentionCoordinationResult represents the result of attention coordination.
type AttentionCoordinationResult struct {
	ConsensusOutput  interface{}        `json:"consensusOutput"`
	AttentionWeights map[string]float64 `json:"attentionWeights"`
	Confidence       float64            `json:"confidence"`
	LatencyMs        float64            `json:"latencyMs"`
	MemoryBytes      int64              `json:"memoryBytes"`
	Participants     []string           `json:"participants"`
	Mechanism        AttentionMechanism `json:"mechanism"`
}

// FlashAttentionConfig holds configuration for Flash Attention.
type FlashAttentionConfig struct {
	BlockSize int  `json:"blockSize"` // Default: 256
	Causal    bool `json:"causal"`
}

// DefaultFlashAttentionConfig returns the default Flash Attention configuration.
func DefaultFlashAttentionConfig() FlashAttentionConfig {
	return FlashAttentionConfig{
		BlockSize: 256,
		Causal:    false,
	}
}

// MultiHeadAttentionConfig holds configuration for Multi-Head Attention.
type MultiHeadAttentionConfig struct {
	NumHeads      int `json:"numHeads"`      // Default: 8
	HeadDimension int `json:"headDimension"` // Default: 64
}

// DefaultMultiHeadAttentionConfig returns the default Multi-Head Attention configuration.
func DefaultMultiHeadAttentionConfig() MultiHeadAttentionConfig {
	return MultiHeadAttentionConfig{
		NumHeads:      8,
		HeadDimension: 64,
	}
}

// LinearAttentionConfig holds configuration for Linear Attention.
type LinearAttentionConfig struct {
	FeatureMapType string `json:"featureMapType"` // "relu", "elu", "softmax"
}

// DefaultLinearAttentionConfig returns the default Linear Attention configuration.
func DefaultLinearAttentionConfig() LinearAttentionConfig {
	return LinearAttentionConfig{
		FeatureMapType: "relu",
	}
}

// HyperbolicAttentionConfig holds configuration for Hyperbolic Attention.
type HyperbolicAttentionConfig struct {
	Curvature          float64 `json:"curvature"`          // Default: -1.0
	Dimension          int     `json:"dimension"`          // Default: 64
	HierarchicalWeight float64 `json:"hierarchicalWeight"` // Weight boost for queen vs worker
}

// DefaultHyperbolicAttentionConfig returns the default Hyperbolic Attention configuration.
func DefaultHyperbolicAttentionConfig() HyperbolicAttentionConfig {
	return HyperbolicAttentionConfig{
		Curvature:          -1.0,
		Dimension:          64,
		HierarchicalWeight: 2.0,
	}
}

// MoEConfig holds configuration for Mixture of Experts.
type MoEConfig struct {
	TopK              int     `json:"topK"`              // Default: 2
	CapacityFactor    float64 `json:"capacityFactor"`    // Default: 1.25
	LoadBalancingLoss bool    `json:"loadBalancingLoss"` // Enable load balancing
}

// DefaultMoEConfig returns the default MoE configuration.
func DefaultMoEConfig() MoEConfig {
	return MoEConfig{
		TopK:              2,
		CapacityFactor:    1.25,
		LoadBalancingLoss: true,
	}
}

// GraphRoPEConfig holds configuration for GraphRoPE.
type GraphRoPEConfig struct {
	MaxDistance   int     `json:"maxDistance"`   // Default: 10
	DistanceScale float64 `json:"distanceScale"` // Default: 1.0
	EncodingDim   int     `json:"encodingDim"`   // Default: 32
}

// DefaultGraphRoPEConfig returns the default GraphRoPE configuration.
func DefaultGraphRoPEConfig() GraphRoPEConfig {
	return GraphRoPEConfig{
		MaxDistance:   10,
		DistanceScale: 1.0,
		EncodingDim:   32,
	}
}

// AttentionConfig holds configuration for the attention coordinator.
type AttentionConfig struct {
	DefaultMechanism AttentionMechanism       `json:"defaultMechanism"`
	Flash            FlashAttentionConfig     `json:"flash"`
	MultiHead        MultiHeadAttentionConfig `json:"multiHead"`
	Linear           LinearAttentionConfig    `json:"linear"`
	Hyperbolic       HyperbolicAttentionConfig `json:"hyperbolic"`
	MoE              MoEConfig                `json:"moe"`
	GraphRoPE        GraphRoPEConfig          `json:"graphRope"`
}

// DefaultAttentionConfig returns the default attention configuration.
func DefaultAttentionConfig() AttentionConfig {
	return AttentionConfig{
		DefaultMechanism: AttentionFlash,
		Flash:            DefaultFlashAttentionConfig(),
		MultiHead:        DefaultMultiHeadAttentionConfig(),
		Linear:           DefaultLinearAttentionConfig(),
		Hyperbolic:       DefaultHyperbolicAttentionConfig(),
		MoE:              DefaultMoEConfig(),
		GraphRoPE:        DefaultGraphRoPEConfig(),
	}
}

// Expert represents a specialized agent for MoE routing.
type Expert struct {
	AgentID     string    `json:"agentId"`
	Expertise   []string  `json:"expertise"`
	Embedding   []float64 `json:"embedding,omitempty"`
	Capacity    int       `json:"capacity"`
	CurrentLoad int       `json:"currentLoad"`
}

// ExpertRoutingResult represents the result of MoE expert routing.
type ExpertRoutingResult struct {
	SelectedExperts []ExpertSelection `json:"selectedExperts"`
	RoutingLatencyMs float64          `json:"routingLatencyMs"`
	LoadBalanced    bool              `json:"loadBalanced"`
}

// ExpertSelection represents a selected expert with its score.
type ExpertSelection struct {
	Expert Expert  `json:"expert"`
	Score  float64 `json:"score"`
	Weight float64 `json:"weight"`
}

// AttentionPerformanceStats holds performance statistics for attention.
type AttentionPerformanceStats struct {
	TotalCoordinations int64   `json:"totalCoordinations"`
	AvgLatencyMs       float64 `json:"avgLatencyMs"`
	AvgMemoryBytes     int64   `json:"avgMemoryBytes"`
	SpeedupFactor      float64 `json:"speedupFactor"`
	MemoryReduction    float64 `json:"memoryReduction"` // 0.0 - 1.0
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
// Hive Mind Consensus Types
// ============================================================================

// ConsensusType represents the type of consensus mechanism.
type ConsensusType string

const (
	ConsensusTypeMajority      ConsensusType = "majority"
	ConsensusTypeSuperMajority ConsensusType = "supermajority"
	ConsensusTypeUnanimous     ConsensusType = "unanimous"
	ConsensusTypeWeighted      ConsensusType = "weighted"
	ConsensusTypeQueenOverride ConsensusType = "queen-override"
)

// ProposalStatus represents the status of a proposal.
type ProposalStatus string

const (
	ProposalStatusPending  ProposalStatus = "pending"
	ProposalStatusVoting   ProposalStatus = "voting"
	ProposalStatusApproved ProposalStatus = "approved"
	ProposalStatusRejected ProposalStatus = "rejected"
	ProposalStatusExpired  ProposalStatus = "expired"
	ProposalStatusCancelled ProposalStatus = "cancelled"
)

// Proposal represents a consensus proposal in the Hive Mind system.
type Proposal struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	Payload         map[string]interface{} `json:"payload,omitempty"`
	RequiredType    ConsensusType          `json:"requiredType"`
	Proposer        string                 `json:"proposer"`
	Domain          AgentDomain            `json:"domain,omitempty"`
	CreatedAt       int64                  `json:"createdAt"`
	ExpiresAt       int64                  `json:"expiresAt"`
	Status          ProposalStatus         `json:"status"`
	RequiredQuorum  float64                `json:"requiredQuorum"` // 0.0 - 1.0
	Priority        TaskPriority           `json:"priority,omitempty"`
}

// WeightedVote represents a vote with weight and reasoning.
type WeightedVote struct {
	AgentID    string  `json:"agentId"`
	ProposalID string  `json:"proposalId"`
	Vote       bool    `json:"vote"`
	Weight     float64 `json:"weight"`     // 0.0 - 1.0, based on agent performance
	Confidence float64 `json:"confidence"` // 0.0 - 1.0
	Reason     string  `json:"reason,omitempty"`
	Timestamp  int64   `json:"timestamp"`
}

// ProposalResult represents the result of a proposal vote.
type ProposalResult struct {
	ProposalID       string         `json:"proposalId"`
	Status           ProposalStatus `json:"status"`
	TotalVotes       int            `json:"totalVotes"`
	ApprovalVotes    int            `json:"approvalVotes"`
	RejectionVotes   int            `json:"rejectionVotes"`
	WeightedApproval float64        `json:"weightedApproval"` // 0.0 - 1.0
	WeightedRejection float64       `json:"weightedRejection"`
	QuorumReached    bool           `json:"quorumReached"`
	ConsensusReached bool           `json:"consensusReached"`
	Votes            []WeightedVote `json:"votes"`
	CompletedAt      int64          `json:"completedAt,omitempty"`
	Duration         int64          `json:"duration,omitempty"` // milliseconds
}

// HiveMindConfig holds configuration for the Hive Mind system.
type HiveMindConfig struct {
	ConsensusAlgorithm ConsensusType `json:"consensusAlgorithm"`
	VoteTimeout        int64         `json:"voteTimeout"`   // milliseconds
	MaxProposals       int           `json:"maxProposals"`
	EnableLearning     bool          `json:"enableLearning"`
	DefaultQuorum      float64       `json:"defaultQuorum"` // 0.0 - 1.0
	MinVoteWeight      float64       `json:"minVoteWeight"` // minimum weight for a vote to count
}

// DefaultHiveMindConfig returns the default Hive Mind configuration.
func DefaultHiveMindConfig() HiveMindConfig {
	return HiveMindConfig{
		ConsensusAlgorithm: ConsensusTypeMajority,
		VoteTimeout:        30000, // 30 seconds
		MaxProposals:       100,
		EnableLearning:     true,
		DefaultQuorum:      0.5,
		MinVoteWeight:      0.1,
	}
}

// HiveMindState represents the current state of the Hive Mind.
type HiveMindState struct {
	Initialized     bool           `json:"initialized"`
	Algorithm       ConsensusType  `json:"algorithm"`
	ActiveProposals int            `json:"activeProposals"`
	TotalAgents     int            `json:"totalAgents"`
	ActiveAgents    int            `json:"activeAgents"`
	DomainsActive   []AgentDomain  `json:"domainsActive"`
	LastConsensus   int64          `json:"lastConsensus,omitempty"`
	QueenAgentID    string         `json:"queenAgentId,omitempty"`
}

// ProposalOutcome represents the outcome of a completed proposal for learning.
type ProposalOutcome struct {
	ProposalID     string                 `json:"proposalId"`
	ProposalType   string                 `json:"proposalType"`
	ConsensusType  ConsensusType          `json:"consensusType"`
	WasApproved    bool                   `json:"wasApproved"`
	VoteCount      int                    `json:"voteCount"`
	WeightedScore  float64                `json:"weightedScore"`
	Duration       int64                  `json:"duration"`
	Timestamp      int64                  `json:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ============================================================================
// Distributed Consensus Algorithm Types
// ============================================================================

// ConsensusAlgorithmType represents the type of distributed consensus algorithm.
type ConsensusAlgorithmType string

const (
	AlgorithmRaft      ConsensusAlgorithmType = "raft"
	AlgorithmByzantine ConsensusAlgorithmType = "byzantine"
	AlgorithmGossip    ConsensusAlgorithmType = "gossip"
	AlgorithmPaxos     ConsensusAlgorithmType = "paxos" // Falls back to Raft
)

// ============================================================================
// Raft Consensus Types
// ============================================================================

// RaftState represents the state of a Raft node.
type RaftState string

const (
	RaftStateFollower  RaftState = "follower"
	RaftStateCandidate RaftState = "candidate"
	RaftStateLeader    RaftState = "leader"
)

// RaftLogEntry represents an entry in the Raft log.
type RaftLogEntry struct {
	Term      int64       `json:"term"`
	Index     int64       `json:"index"`
	Command   interface{} `json:"command"`
	Timestamp int64       `json:"timestamp"`
}

// RaftNode represents the state of a Raft node.
type RaftNode struct {
	ID          string         `json:"id"`
	State       RaftState      `json:"state"`
	CurrentTerm int64          `json:"currentTerm"`
	VotedFor    string         `json:"votedFor,omitempty"`
	Log         []RaftLogEntry `json:"log"`
	CommitIndex int64          `json:"commitIndex"`
	LastApplied int64          `json:"lastApplied"`
}

// RaftConfig holds configuration for Raft consensus.
type RaftConfig struct {
	ElectionTimeoutMin int64   `json:"electionTimeoutMin"` // milliseconds (default: 150)
	ElectionTimeoutMax int64   `json:"electionTimeoutMax"` // milliseconds (default: 300)
	HeartbeatInterval  int64   `json:"heartbeatInterval"`  // milliseconds (default: 50)
	Threshold          float64 `json:"threshold"`          // default: 0.66 (2/3)
	TimeoutMs          int64   `json:"timeoutMs"`          // proposal timeout
	MaxRounds          int     `json:"maxRounds"`
	RequireQuorum      bool    `json:"requireQuorum"`
}

// DefaultRaftConfig returns the default Raft configuration.
func DefaultRaftConfig() RaftConfig {
	return RaftConfig{
		ElectionTimeoutMin: 150,
		ElectionTimeoutMax: 300,
		HeartbeatInterval:  50,
		Threshold:          0.66,
		TimeoutMs:          30000,
		MaxRounds:          10,
		RequireQuorum:      true,
	}
}

// ============================================================================
// Byzantine Consensus Types (PBFT)
// ============================================================================

// ByzantinePhase represents a phase in PBFT consensus.
type ByzantinePhase string

const (
	ByzantinePhasePrePrepare ByzantinePhase = "pre-prepare"
	ByzantinePhasePrepare    ByzantinePhase = "prepare"
	ByzantinePhaseCommit     ByzantinePhase = "commit"
	ByzantinePhaseReply      ByzantinePhase = "reply"
)

// ByzantineMessage represents a PBFT protocol message.
type ByzantineMessage struct {
	Type           ByzantinePhase `json:"type"`
	ViewNumber     int64          `json:"viewNumber"`
	SequenceNumber int64          `json:"sequenceNumber"`
	Digest         string         `json:"digest"`
	SenderID       string         `json:"senderId"`
	Timestamp      int64          `json:"timestamp"`
	Payload        interface{}    `json:"payload,omitempty"`
	Signature      string         `json:"signature,omitempty"`
}

// ByzantineNode represents the state of a Byzantine node.
type ByzantineNode struct {
	ID             string `json:"id"`
	IsPrimary      bool   `json:"isPrimary"`
	ViewNumber     int64  `json:"viewNumber"`
	SequenceNumber int64  `json:"sequenceNumber"`
}

// ByzantineConfig holds configuration for Byzantine consensus.
type ByzantineConfig struct {
	MaxFaultyNodes      int     `json:"maxFaultyNodes"`      // f in 3f+1
	ViewChangeTimeoutMs int64   `json:"viewChangeTimeoutMs"` // default: 5000
	Threshold           float64 `json:"threshold"`
	TimeoutMs           int64   `json:"timeoutMs"`
	MaxRounds           int     `json:"maxRounds"`
	RequireQuorum       bool    `json:"requireQuorum"`
}

// DefaultByzantineConfig returns the default Byzantine configuration.
func DefaultByzantineConfig() ByzantineConfig {
	return ByzantineConfig{
		MaxFaultyNodes:      1,
		ViewChangeTimeoutMs: 5000,
		Threshold:           0.66,
		TimeoutMs:           30000,
		MaxRounds:           10,
		RequireQuorum:       true,
	}
}

// ============================================================================
// Gossip Protocol Types
// ============================================================================

// GossipMessageType represents the type of gossip message.
type GossipMessageType string

const (
	GossipMessageProposal GossipMessageType = "proposal"
	GossipMessageVote     GossipMessageType = "vote"
	GossipMessageState    GossipMessageType = "state"
	GossipMessageAck      GossipMessageType = "ack"
)

// GossipMessage represents a message in the gossip protocol.
type GossipMessage struct {
	ID        string            `json:"id"`
	Type      GossipMessageType `json:"type"`
	SenderID  string            `json:"senderId"`
	Version   int64             `json:"version"`
	Payload   interface{}       `json:"payload"`
	Timestamp int64             `json:"timestamp"`
	TTL       int               `json:"ttl"`
	Hops      int               `json:"hops"`
	Path      []string          `json:"path"`
}

// GossipNode represents the state of a gossip node.
type GossipNode struct {
	ID        string   `json:"id"`
	Version   int64    `json:"version"`
	Neighbors []string `json:"neighbors"`
	LastSync  int64    `json:"lastSync"`
}

// GossipConfig holds configuration for gossip protocol.
type GossipConfig struct {
	Fanout               int     `json:"fanout"`               // default: 3
	GossipIntervalMs     int64   `json:"gossipIntervalMs"`     // default: 100
	MaxHops              int     `json:"maxHops"`              // default: 10
	ConvergenceThreshold float64 `json:"convergenceThreshold"` // default: 0.9
	Threshold            float64 `json:"threshold"`            // approval threshold
	TimeoutMs            int64   `json:"timeoutMs"`
	MaxRounds            int     `json:"maxRounds"`
	RequireQuorum        bool    `json:"requireQuorum"` // false for eventual consistency
}

// DefaultGossipConfig returns the default gossip configuration.
func DefaultGossipConfig() GossipConfig {
	return GossipConfig{
		Fanout:               3,
		GossipIntervalMs:     100,
		MaxHops:              10,
		ConvergenceThreshold: 0.9,
		Threshold:            0.66,
		TimeoutMs:            30000,
		MaxRounds:            10,
		RequireQuorum:        false, // Gossip is eventually consistent
	}
}

// ============================================================================
// Algorithm Selection Types
// ============================================================================

// FaultToleranceMode represents the fault tolerance requirement.
type FaultToleranceMode string

const (
	FaultToleranceCrash     FaultToleranceMode = "crash"
	FaultToleranceByzantine FaultToleranceMode = "byzantine"
)

// ConsistencyMode represents the consistency requirement.
type ConsistencyMode string

const (
	ConsistencyStrong   ConsistencyMode = "strong"
	ConsistencyEventual ConsistencyMode = "eventual"
)

// NetworkScale represents the network scale.
type NetworkScale string

const (
	NetworkScaleSmall  NetworkScale = "small"  // < 10 nodes
	NetworkScaleMedium NetworkScale = "medium" // 10-50 nodes
	NetworkScaleLarge  NetworkScale = "large"  // 50+ nodes
)

// LatencyPriority represents the latency priority.
type LatencyPriority string

const (
	LatencyPriorityLow    LatencyPriority = "low"
	LatencyPriorityMedium LatencyPriority = "medium"
	LatencyPriorityHigh   LatencyPriority = "high"
)

// AlgorithmSelectionOptions holds options for algorithm selection.
type AlgorithmSelectionOptions struct {
	FaultTolerance  FaultToleranceMode `json:"faultTolerance"`
	Consistency     ConsistencyMode    `json:"consistency"`
	NetworkScale    NetworkScale       `json:"networkScale"`
	LatencyPriority LatencyPriority    `json:"latencyPriority"`
}

// ConsensusProposal represents a proposal in the distributed consensus.
type ConsensusProposal struct {
	ID         string                 `json:"id"`
	ProposerID string                 `json:"proposerId"`
	Value      interface{}            `json:"value"`
	Term       int64                  `json:"term"`
	Timestamp  int64                  `json:"timestamp"`
	Status     string                 `json:"status"` // pending, accepted, rejected, expired
	Votes      map[string]interface{} `json:"votes"`
}

// ConsensusVote represents a vote in the distributed consensus.
type ConsensusVote struct {
	VoterID    string  `json:"voterId"`
	Approve    bool    `json:"approve"`
	Confidence float64 `json:"confidence"`
	Timestamp  int64   `json:"timestamp"`
}

// DistributedConsensusResult represents the result of a distributed consensus.
type DistributedConsensusResult struct {
	ProposalID        string      `json:"proposalId"`
	Approved          bool        `json:"approved"`
	ApprovalRate      float64     `json:"approvalRate"`
	ParticipationRate float64     `json:"participationRate"`
	FinalValue        interface{} `json:"finalValue"`
	Rounds            int         `json:"rounds"`
	DurationMs        int64       `json:"durationMs"`
}

// AlgorithmStats represents statistics for a consensus algorithm.
type AlgorithmStats struct {
	Algorithm         ConsensusAlgorithmType `json:"algorithm"`
	TotalProposals    int                    `json:"totalProposals"`
	PendingProposals  int                    `json:"pendingProposals"`
	AcceptedProposals int                    `json:"acceptedProposals"`
	RejectedProposals int                    `json:"rejectedProposals"`
	ExpiredProposals  int                    `json:"expiredProposals"`
	AverageDurationMs float64                `json:"averageDurationMs"`
}

// ============================================================================
// Message Bus Types
// ============================================================================

// MessagePriority represents the priority level of a message.
type MessagePriority int

const (
	MessagePriorityUrgent MessagePriority = 0 // Critical system messages
	MessagePriorityHigh   MessagePriority = 1 // Task assignments, consensus
	MessagePriorityNormal MessagePriority = 2 // Regular communication
	MessagePriorityLow    MessagePriority = 3 // Background operations
)

// MessagePriorityCount is the number of priority levels.
const MessagePriorityCount = 4

// String returns the string representation of the priority.
func (p MessagePriority) String() string {
	switch p {
	case MessagePriorityUrgent:
		return "urgent"
	case MessagePriorityHigh:
		return "high"
	case MessagePriorityNormal:
		return "normal"
	case MessagePriorityLow:
		return "low"
	default:
		return "unknown"
	}
}

// BusMessageType represents the type of message in the message bus.
type BusMessageType string

const (
	// Task-related messages
	BusMessageTaskAssign   BusMessageType = "task_assign"
	BusMessageTaskComplete BusMessageType = "task_complete"
	BusMessageTaskFail     BusMessageType = "task_fail"

	// Agent lifecycle messages
	BusMessageHeartbeat    BusMessageType = "heartbeat"
	BusMessageStatusUpdate BusMessageType = "status_update"
	BusMessageAgentJoin    BusMessageType = "agent_join"
	BusMessageAgentLeave   BusMessageType = "agent_leave"

	// Consensus messages
	BusMessageConsensusPropose BusMessageType = "consensus_propose"
	BusMessageConsensusVote    BusMessageType = "consensus_vote"
	BusMessageConsensusCommit  BusMessageType = "consensus_commit"

	// Topology messages
	BusMessageTopologyUpdate BusMessageType = "topology_update"

	// Communication messages
	BusMessageBroadcast BusMessageType = "broadcast"
	BusMessageDirect    BusMessageType = "direct"
)

// BusMessage represents a message in the message bus.
type BusMessage struct {
	ID          string                 `json:"id"`
	Type        BusMessageType         `json:"type"`
	From        string                 `json:"from"`
	To          string                 `json:"to"` // "broadcast" for broadcast messages
	Priority    MessagePriority        `json:"priority"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	Timestamp   int64                  `json:"timestamp"`
	TTLMs       int64                  `json:"ttlMs"`       // Time-to-live in milliseconds
	RequiresAck bool                   `json:"requiresAck"` // Whether acknowledgment is required
}

// MessageAck represents an acknowledgment for a message.
type MessageAck struct {
	MessageID string `json:"messageId"`
	AgentID   string `json:"agentId"`
	Received  bool   `json:"received"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// MessageBusConfig holds configuration for the message bus.
type MessageBusConfig struct {
	MaxQueueSize         int   `json:"maxQueueSize"`         // Max messages per agent queue
	ProcessingIntervalMs int64 `json:"processingIntervalMs"` // Processing loop interval
	AckTimeoutMs         int64 `json:"ackTimeoutMs"`         // Timeout for acknowledgments
	RetryAttempts        int   `json:"retryAttempts"`        // Max retry attempts
	DefaultTTLMs         int64 `json:"defaultTtlMs"`         // Default TTL for messages
	EnablePersistence    bool  `json:"enablePersistence"`    // Persist messages to disk
	BatchSize            int   `json:"batchSize"`            // Messages per batch delivery
}

// DefaultMessageBusConfig returns the default message bus configuration.
func DefaultMessageBusConfig() MessageBusConfig {
	return MessageBusConfig{
		MaxQueueSize:         1000,
		ProcessingIntervalMs: 10,
		AckTimeoutMs:         5000,
		RetryAttempts:        3,
		DefaultTTLMs:         30000, // 30 seconds
		EnablePersistence:    false,
		BatchSize:            10,
	}
}

// MessageBusStats represents statistics for the message bus.
type MessageBusStats struct {
	TotalMessages     int64   `json:"totalMessages"`
	MessagesPerSecond float64 `json:"messagesPerSecond"`
	AvgLatencyMs      float64 `json:"avgLatencyMs"`
	QueueDepth        int     `json:"queueDepth"`
	AckRate           float64 `json:"ackRate"`   // 0.0 - 1.0
	ErrorRate         float64 `json:"errorRate"` // 0.0 - 1.0
	DroppedMessages   int64   `json:"droppedMessages"`
}

// MessageEntry represents an entry in the message queue.
type MessageEntry struct {
	Message      *BusMessage `json:"message"`
	Attempts     int         `json:"attempts"`
	EnqueuedAt   int64       `json:"enqueuedAt"`
	LastAttemptAt int64      `json:"lastAttemptAt,omitempty"`
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
// Queen Coordinator Interfaces
// ============================================================================

// NeuralLearningSystem defines the interface for neural learning integration.
type NeuralLearningSystem interface {
	// LearnFromOutcome records the outcome of a task for learning.
	LearnFromOutcome(ctx context.Context, taskID string, outcome TaskResult) error
	// GetPatternMatches retrieves patterns matching the given task description.
	GetPatternMatches(ctx context.Context, description string, k int) ([]string, error)
	// RecordTrajectory records an agent's execution trajectory.
	RecordTrajectory(ctx context.Context, agentID string, trajectory []TrajectoryStep) error
}

// TrajectoryStep represents a step in an agent's execution trajectory.
type TrajectoryStep struct {
	Timestamp int64                  `json:"timestamp"`
	Action    string                 `json:"action"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Outcome   string                 `json:"outcome,omitempty"`
	Reward    float64                `json:"reward,omitempty"`
}

// MemoryService defines the interface for memory operations used by Queen Coordinator.
type MemoryService interface {
	// StoreTaskMemory stores a memory entry for a task.
	StoreTaskMemory(ctx context.Context, agentID string, taskID string, content string) error
	// RetrieveContext retrieves relevant context for a task.
	RetrieveContext(ctx context.Context, taskDescription string, k int) ([]Memory, error)
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
// MCP 2025-11-25 Compliance Types
// ============================================================================

// MCPResource represents a resource in the MCP protocol.
type MCPResource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// ResourceContent represents the content of a resource.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     []byte `json:"blob,omitempty"`
}

// ResourceTemplate represents a resource URI template.
type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceListResult represents the result of listing resources.
type ResourceListResult struct {
	Resources  []MCPResource `json:"resources"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

// ResourceReadResult represents the result of reading a resource.
type ResourceReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceSubscription represents a subscription to resource updates.
type ResourceSubscription struct {
	ID       string `json:"id"`
	URI      string `json:"uri"`
	Callback func(uri string, content *ResourceContent)
}

// MCPPrompt represents a prompt in the MCP protocol.
type MCPPrompt struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents an argument for a prompt.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptContentType represents the type of prompt content.
type PromptContentType string

const (
	PromptContentTypeText     PromptContentType = "text"
	PromptContentTypeImage    PromptContentType = "image"
	PromptContentTypeResource PromptContentType = "resource"
)

// PromptContent represents content in a prompt message.
type PromptContent struct {
	Type        PromptContentType `json:"type"`
	Text        string            `json:"text,omitempty"`
	ResourceURI string            `json:"uri,omitempty"`
	MimeType    string            `json:"mimeType,omitempty"`
	Data        string            `json:"data,omitempty"` // Base64 for images
}

// PromptMessage represents a message in a prompt.
type PromptMessage struct {
	Role    string          `json:"role"` // "user" or "assistant"
	Content []PromptContent `json:"content"`
}

// PromptListResult represents the result of listing prompts.
type PromptListResult struct {
	Prompts    []MCPPrompt `json:"prompts"`
	NextCursor string      `json:"nextCursor,omitempty"`
}

// PromptGetResult represents the result of getting a prompt.
type PromptGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// SamplingMessage represents a message in a sampling request.
type SamplingMessage struct {
	Role    string          `json:"role"` // "user" or "assistant"
	Content []PromptContent `json:"content"`
}

// ModelPreferences represents preferences for model selection.
type ModelPreferences struct {
	Hints                  []ModelHint `json:"hints,omitempty"`
	CostPriority           float64     `json:"costPriority,omitempty"`           // 0.0-1.0
	SpeedPriority          float64     `json:"speedPriority,omitempty"`          // 0.0-1.0
	IntelligencePriority   float64     `json:"intelligencePriority,omitempty"`   // 0.0-1.0
}

// ModelHint represents a hint for model selection.
type ModelHint struct {
	Name string `json:"name,omitempty"`
}

// CreateMessageRequest represents a request to create a message via sampling.
type CreateMessageRequest struct {
	Messages         []SamplingMessage `json:"messages"`
	MaxTokens        int               `json:"maxTokens"`
	SystemPrompt     string            `json:"systemPrompt,omitempty"`
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	IncludeContext   string            `json:"includeContext,omitempty"` // "none", "thisServer", "allServers"
	Temperature      float64           `json:"temperature,omitempty"`
	StopSequences    []string          `json:"stopSequences,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// CreateMessageResult represents the result of creating a message.
type CreateMessageResult struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stopReason,omitempty"`
}

// CompletionReferenceType represents the type of completion reference.
type CompletionReferenceType string

const (
	CompletionRefPrompt   CompletionReferenceType = "ref/prompt"
	CompletionRefResource CompletionReferenceType = "ref/resource"
)

// CompletionReference represents a reference for completion.
type CompletionReference struct {
	Type CompletionReferenceType `json:"type"`
	Name string                  `json:"name,omitempty"` // For prompts
	URI  string                  `json:"uri,omitempty"`  // For resources
}

// CompletionArgument represents an argument for completion.
type CompletionArgument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CompletionResult represents the result of a completion request.
type CompletionResult struct {
	Values  []string `json:"values"`
	Total   int      `json:"total,omitempty"`
	HasMore bool     `json:"hasMore,omitempty"`
}

// MCPLogLevel represents a log level.
type MCPLogLevel string

const (
	MCPLogLevelDebug     MCPLogLevel = "debug"
	MCPLogLevelInfo      MCPLogLevel = "info"
	MCPLogLevelNotice    MCPLogLevel = "notice"
	MCPLogLevelWarning   MCPLogLevel = "warning"
	MCPLogLevelError     MCPLogLevel = "error"
	MCPLogLevelCritical  MCPLogLevel = "critical"
	MCPLogLevelAlert     MCPLogLevel = "alert"
	MCPLogLevelEmergency MCPLogLevel = "emergency"
)

// LoggingMessage represents a log message.
type LoggingMessage struct {
	Level  MCPLogLevel `json:"level"`
	Logger string      `json:"logger,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

// MCPCapabilities represents server capabilities.
type MCPCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Logging      *LoggingCapability     `json:"logging,omitempty"`
	Prompts      *PromptsCapability     `json:"prompts,omitempty"`
	Resources    *ResourcesCapability   `json:"resources,omitempty"`
	Tools        *ToolsCapability       `json:"tools,omitempty"`
	Sampling     *SamplingCapability    `json:"sampling,omitempty"`
}

// LoggingCapability represents logging capability.
type LoggingCapability struct {
	Level MCPLogLevel `json:"level,omitempty"`
}

// PromptsCapability represents prompts capability.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability represents resources capability.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolsCapability represents tools capability.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability represents sampling capability.
type SamplingCapability struct{}

// ResourceCacheConfig holds configuration for resource caching.
type ResourceCacheConfig struct {
	MaxEntries int   `json:"maxEntries"` // Default: 1000
	TTLSeconds int64 `json:"ttlSeconds"` // Default: 300 (5 minutes)
}

// DefaultResourceCacheConfig returns the default resource cache configuration.
func DefaultResourceCacheConfig() ResourceCacheConfig {
	return ResourceCacheConfig{
		MaxEntries: 1000,
		TTLSeconds: 300,
	}
}

// SamplingConfig holds configuration for the sampling manager.
type SamplingConfig struct {
	DefaultMaxTokens int     `json:"defaultMaxTokens"` // Default: 4096
	DefaultTemperature float64 `json:"defaultTemperature"` // Default: 0.7
	TimeoutMs        int64   `json:"timeoutMs"`        // Default: 30000
	EnableLogging    bool    `json:"enableLogging"`    // Default: true
}

// DefaultSamplingConfig returns the default sampling configuration.
func DefaultSamplingConfig() SamplingConfig {
	return SamplingConfig{
		DefaultMaxTokens:   4096,
		DefaultTemperature: 0.7,
		TimeoutMs:          30000,
		EnableLogging:      true,
	}
}

// SamplingStats holds statistics for sampling operations.
type SamplingStats struct {
	TotalRequests    int64   `json:"totalRequests"`
	SuccessfulRequests int64 `json:"successfulRequests"`
	FailedRequests   int64   `json:"failedRequests"`
	TotalInputTokens int64   `json:"totalInputTokens"`
	TotalOutputTokens int64  `json:"totalOutputTokens"`
	AvgLatencyMs     float64 `json:"avgLatencyMs"`
}

// ============================================================================
// Task Management Types (MCP Task Tools)
// ============================================================================

// ManagedTaskStatus represents the status of a managed task.
type ManagedTaskStatus string

const (
	ManagedTaskStatusPending   ManagedTaskStatus = "pending"
	ManagedTaskStatusQueued    ManagedTaskStatus = "queued"
	ManagedTaskStatusRunning   ManagedTaskStatus = "running"
	ManagedTaskStatusCompleted ManagedTaskStatus = "completed"
	ManagedTaskStatusFailed    ManagedTaskStatus = "failed"
	ManagedTaskStatusCancelled ManagedTaskStatus = "cancelled"
)

// ManagedTask represents a task managed by the TaskManager.
type ManagedTask struct {
	ID           string                 `json:"id"`
	Type         TaskType               `json:"type"`
	Description  string                 `json:"description"`
	Priority     int                    `json:"priority"` // 1-10, higher is more important
	Status       ManagedTaskStatus      `json:"status"`
	AssignedTo   string                 `json:"assignedTo,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Input        interface{}            `json:"input,omitempty"`
	Output       interface{}            `json:"output,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Progress     float64                `json:"progress"` // 0.0-1.0
	CreatedAt    int64                  `json:"createdAt"`
	StartedAt    int64                  `json:"startedAt,omitempty"`
	CompletedAt  int64                  `json:"completedAt,omitempty"`
	TimeoutMs    int64                  `json:"timeoutMs,omitempty"`
	QueuePosition int                   `json:"queuePosition,omitempty"`
	Artifacts    []TaskArtifact         `json:"artifacts,omitempty"`
	History      []TaskHistoryEntry     `json:"history,omitempty"`
	Metrics      *TaskMetrics           `json:"metrics,omitempty"`
}

// TaskArtifact represents an artifact produced by a task.
type TaskArtifact struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Path     string `json:"path,omitempty"`
	Data     string `json:"data,omitempty"`
	Size     int64  `json:"size,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// TaskHistoryEntry represents a historical event for a task.
type TaskHistoryEntry struct {
	Timestamp int64       `json:"timestamp"`
	Event     string      `json:"event"`
	Details   interface{} `json:"details,omitempty"`
}

// TaskMetrics represents execution metrics for a task.
type TaskMetrics struct {
	ExecutionTimeMs int64   `json:"executionTimeMs"`
	WaitTimeMs      int64   `json:"waitTimeMs"`
	RetryCount      int     `json:"retryCount"`
	MemoryUsedBytes int64   `json:"memoryUsedBytes,omitempty"`
	CPUTimeMs       int64   `json:"cpuTimeMs,omitempty"`
}

// TaskFilter represents filters for listing tasks.
type TaskFilter struct {
	Status    ManagedTaskStatus `json:"status,omitempty"`
	AgentID   string            `json:"agentId,omitempty"`
	Type      TaskType          `json:"type,omitempty"`
	Priority  int               `json:"priority,omitempty"`
	Limit     int               `json:"limit,omitempty"`
	Offset    int               `json:"offset,omitempty"`
	SortBy    string            `json:"sortBy,omitempty"`    // created, priority, status, updated
	SortOrder string            `json:"sortOrder,omitempty"` // asc, desc
}

// TaskUpdate represents updates to apply to a task.
type TaskUpdate struct {
	Priority    *int                    `json:"priority,omitempty"`
	Description *string                 `json:"description,omitempty"`
	TimeoutMs   *int64                  `json:"timeoutMs,omitempty"`
	Metadata    map[string]interface{}  `json:"metadata,omitempty"`
}

// TaskResult represents the result of a completed task.
type TaskResult struct {
	TaskID        string         `json:"taskId"`
	Status        ManagedTaskStatus `json:"status"`
	Output        interface{}    `json:"output,omitempty"`
	Error         string         `json:"error,omitempty"`
	Artifacts     []TaskArtifact `json:"artifacts,omitempty"`
	ExecutionTime int64          `json:"executionTimeMs"`
	CompletedAt   int64          `json:"completedAt"`
}

// TaskManagerConfig holds configuration for the TaskManager.
type TaskManagerConfig struct {
	MaxConcurrent     int   `json:"maxConcurrent"`     // Default: 10
	DefaultTimeoutMs  int64 `json:"defaultTimeoutMs"`  // Default: 300000 (5 minutes)
	RetentionPeriodMs int64 `json:"retentionPeriodMs"` // Default: 3600000 (1 hour)
	QueueSize         int   `json:"queueSize"`         // Default: 1000
}

// DefaultTaskManagerConfig returns the default TaskManager configuration.
func DefaultTaskManagerConfig() TaskManagerConfig {
	return TaskManagerConfig{
		MaxConcurrent:     10,
		DefaultTimeoutMs:  300000,  // 5 minutes
		RetentionPeriodMs: 3600000, // 1 hour
		QueueSize:         1000,
	}
}

// TaskManagerStats holds statistics for the TaskManager.
type TaskManagerStats struct {
	TotalTasks     int64 `json:"totalTasks"`
	PendingTasks   int64 `json:"pendingTasks"`
	QueuedTasks    int64 `json:"queuedTasks"`
	RunningTasks   int64 `json:"runningTasks"`
	CompletedTasks int64 `json:"completedTasks"`
	FailedTasks    int64 `json:"failedTasks"`
	CancelledTasks int64 `json:"cancelledTasks"`
	AvgWaitTimeMs  float64 `json:"avgWaitTimeMs"`
	AvgExecTimeMs  float64 `json:"avgExecTimeMs"`
}

// TaskCreateRequest represents a request to create a task.
type TaskCreateRequest struct {
	Type          TaskType               `json:"type"`
	Description   string                 `json:"description"`
	Priority      int                    `json:"priority,omitempty"` // 1-10
	Dependencies  []string               `json:"dependencies,omitempty"`
	AssignToAgent string                 `json:"assignToAgent,omitempty"`
	Input         interface{}            `json:"input,omitempty"`
	TimeoutMs     int64                  `json:"timeoutMs,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// TaskListResult represents the result of listing tasks.
type TaskListResult struct {
	Tasks  []*ManagedTask `json:"tasks"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// TaskDependencyAction represents an action on task dependencies.
type TaskDependencyAction string

const (
	TaskDependencyActionAdd    TaskDependencyAction = "add"
	TaskDependencyActionRemove TaskDependencyAction = "remove"
	TaskDependencyActionList   TaskDependencyAction = "list"
	TaskDependencyActionClear  TaskDependencyAction = "clear"
)

// TaskResultFormat represents the format for task results.
type TaskResultFormat string

const (
	TaskResultFormatSummary  TaskResultFormat = "summary"
	TaskResultFormatDetailed TaskResultFormat = "detailed"
	TaskResultFormatRaw      TaskResultFormat = "raw"
)

// Task-related errors
var (
	// ErrTaskNotFound is returned when a task is not found.
	ErrTaskNotFound = errors.New("task not found")
	// ErrTaskAlreadyAssigned is returned when trying to assign an already assigned task.
	ErrTaskAlreadyAssigned = errors.New("task already assigned")
	// ErrTaskNotCancellable is returned when a task cannot be cancelled.
	ErrTaskNotCancellable = errors.New("task cannot be cancelled in current state")
	// ErrTaskQueueFull is returned when the task queue is full.
	ErrTaskQueueFull = errors.New("task queue is full")
	// ErrInvalidTaskPriority is returned for invalid priority values.
	ErrInvalidTaskPriority = errors.New("priority must be between 1 and 10")
	// ErrCircularDependency is returned when a circular dependency is detected.
	ErrCircularDependency = errors.New("circular dependency detected")
)

// ============================================================================
// Hooks System Types
// ============================================================================

// HookEvent represents a hook event type.
type HookEvent string

const (
	HookEventPreEdit      HookEvent = "pre-edit"
	HookEventPostEdit     HookEvent = "post-edit"
	HookEventPreCommand   HookEvent = "pre-command"
	HookEventPostCommand  HookEvent = "post-command"
	HookEventPreRoute     HookEvent = "pre-route"
	HookEventPostRoute    HookEvent = "post-route"
	HookEventPreTask      HookEvent = "pre-task"
	HookEventPostTask     HookEvent = "post-task"
	HookEventAgentSpawn   HookEvent = "agent-spawn"
	HookEventAgentTerminate HookEvent = "agent-terminate"
	HookEventSessionStart HookEvent = "session-start"
	HookEventSessionEnd   HookEvent = "session-end"
	HookEventPatternLearned HookEvent = "pattern-learned"
)

// HookPriority represents the priority of a hook.
type HookPriority int

const (
	HookPriorityCritical   HookPriority = 100
	HookPriorityHigh       HookPriority = 75
	HookPriorityNormal     HookPriority = 50
	HookPriorityLow        HookPriority = 25
	HookPriorityBackground HookPriority = 10
)

// HookHandler is a function that handles a hook event.
type HookHandler func(ctx context.Context, data interface{}) (interface{}, error)

// HookRegistration represents a registered hook.
type HookRegistration struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Event       HookEvent    `json:"event"`
	Priority    HookPriority `json:"priority"`
	Enabled     bool         `json:"enabled"`
	Description string       `json:"description,omitempty"`
	Handler     HookHandler  `json:"-"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   int64        `json:"createdAt"`
	ExecutionCount int64     `json:"executionCount"`
	LastExecutedAt int64     `json:"lastExecutedAt,omitempty"`
	AvgExecutionMs float64   `json:"avgExecutionMs"`
}

// HookContext represents the context for hook execution.
type HookContext struct {
	Event     HookEvent              `json:"event"`
	Data      interface{}            `json:"data"`
	Timestamp int64                  `json:"timestamp"`
	SessionID string                 `json:"sessionId,omitempty"`
	AgentID   string                 `json:"agentId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HookResult represents the result of hook execution.
type HookResult struct {
	HookID      string        `json:"hookId"`
	Success     bool          `json:"success"`
	Result      interface{}   `json:"result,omitempty"`
	Error       string        `json:"error,omitempty"`
	ExecutionMs int64         `json:"executionMs"`
	Timestamp   int64         `json:"timestamp"`
}

// HookExecutionResult represents the aggregate result of executing all hooks for an event.
type HookExecutionResult struct {
	Event       HookEvent     `json:"event"`
	HooksRun    int           `json:"hooksRun"`
	Successful  int           `json:"successful"`
	Failed      int           `json:"failed"`
	Results     []*HookResult `json:"results"`
	TotalTimeMs int64         `json:"totalTimeMs"`
}

// PatternType represents the type of a learned pattern.
type PatternType string

const (
	PatternTypeEdit    PatternType = "edit"
	PatternTypeCommand PatternType = "command"
	PatternTypeRoute   PatternType = "route"
	PatternTypeTask    PatternType = "task"
)

// Pattern represents a learned pattern from hook execution.
type Pattern struct {
	ID           string                 `json:"id"`
	Type         PatternType            `json:"type"`
	Content      string                 `json:"content"`
	Keywords     []string               `json:"keywords"`
	SuccessCount int64                  `json:"successCount"`
	FailureCount int64                  `json:"failureCount"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    int64                  `json:"createdAt"`
	UpdatedAt    int64                  `json:"updatedAt"`
	LastUsedAt   int64                  `json:"lastUsedAt,omitempty"`
}

// GetSuccessRate returns the success rate for a pattern.
func (p *Pattern) GetSuccessRate() float64 {
	total := p.SuccessCount + p.FailureCount
	if total == 0 {
		return 0.0
	}
	return float64(p.SuccessCount) / float64(total)
}

// RiskLevel represents the risk level of an operation.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// RiskAssessment represents the risk assessment of an operation.
type RiskAssessment struct {
	Level         RiskLevel `json:"level"`
	Score         float64   `json:"score"` // 0.0-1.0
	Concerns      []string  `json:"concerns,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
	ShouldProceed bool      `json:"shouldProceed"`
}

// RoutingResult represents the result of routing a task.
type RoutingResult struct {
	ID             string                 `json:"id"`
	RecommendedAgent string               `json:"recommendedAgent"`
	Confidence     float64                `json:"confidence"` // 0.0-1.0
	Alternatives   []RoutingAlternative   `json:"alternatives,omitempty"`
	Explanation    string                 `json:"explanation,omitempty"`
	Factors        []RoutingFactor        `json:"factors,omitempty"`
	Timestamp      int64                  `json:"timestamp"`
}

// RoutingAlternative represents an alternative routing option.
type RoutingAlternative struct {
	AgentID    string  `json:"agentId"`
	AgentType  string  `json:"agentType"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

// RoutingFactor represents a factor in the routing decision.
type RoutingFactor struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason,omitempty"`
}

// RoutingExplanation provides detailed explanation of a routing decision.
type RoutingExplanation struct {
	Task           string           `json:"task"`
	Analysis       string           `json:"analysis"`
	Factors        []RoutingFactor  `json:"factors"`
	HistoricalData *RoutingHistory  `json:"historicalData,omitempty"`
	Patterns       []*Pattern       `json:"patterns,omitempty"`
}

// RoutingHistory represents historical routing data.
type RoutingHistory struct {
	TotalRoutings  int64              `json:"totalRoutings"`
	SuccessRate    float64            `json:"successRate"`
	AgentStats     map[string]AgentRoutingStats `json:"agentStats"`
}

// AgentRoutingStats represents routing statistics for an agent.
type AgentRoutingStats struct {
	AgentID      string  `json:"agentId"`
	AgentType    string  `json:"agentType"`
	TasksRouted  int64   `json:"tasksRouted"`
	SuccessRate  float64 `json:"successRate"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
}

// HooksMetrics represents metrics for the hooks system.
type HooksMetrics struct {
	TotalExecutions    int64   `json:"totalExecutions"`
	SuccessfulExecutions int64 `json:"successfulExecutions"`
	FailedExecutions   int64   `json:"failedExecutions"`
	AvgExecutionMs     float64 `json:"avgExecutionMs"`
	PatternCount       int64   `json:"patternCount"`
	RoutingCount       int64   `json:"routingCount"`
	RoutingSuccessRate float64 `json:"routingSuccessRate"`
	EditPatterns       int64   `json:"editPatterns"`
	CommandPatterns    int64   `json:"commandPatterns"`
	HooksByEvent       map[HookEvent]int64 `json:"hooksByEvent"`
}

// HooksConfig holds configuration for the hooks system.
type HooksConfig struct {
	MaxPatterns       int   `json:"maxPatterns"`       // Default: 10000
	MaxHooksPerEvent  int   `json:"maxHooksPerEvent"`  // Default: 100
	DefaultTimeoutMs  int64 `json:"defaultTimeoutMs"`  // Default: 5000
	EnableLearning    bool  `json:"enableLearning"`    // Default: true
	LearningRate      float64 `json:"learningRate"`    // Default: 0.1
}

// DefaultHooksConfig returns the default hooks configuration.
func DefaultHooksConfig() HooksConfig {
	return HooksConfig{
		MaxPatterns:      10000,
		MaxHooksPerEvent: 100,
		DefaultTimeoutMs: 5000,
		EnableLearning:   true,
		LearningRate:     0.1,
	}
}

// PreEditContext represents context for pre-edit hooks.
type PreEditContext struct {
	FilePath       string                 `json:"filePath"`
	Operation      string                 `json:"operation"`
	IncludeContext bool                   `json:"includeContext"`
	IncludeSuggestions bool               `json:"includeSuggestions"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// PreEditResult represents the result of a pre-edit hook.
type PreEditResult struct {
	FilePath        string           `json:"filePath"`
	SimilarPatterns []*Pattern       `json:"similarPatterns,omitempty"`
	Warnings        []string         `json:"warnings,omitempty"`
	Suggestions     []string         `json:"suggestions,omitempty"`
	Context         map[string]interface{} `json:"context,omitempty"`
}

// PostEditContext represents context for post-edit hooks.
type PostEditContext struct {
	FilePath  string                 `json:"filePath"`
	Operation string                 `json:"operation"`
	Success   bool                   `json:"success"`
	Outcome   string                 `json:"outcome,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// PostEditResult represents the result of a post-edit hook.
type PostEditResult struct {
	Recorded  bool   `json:"recorded"`
	PatternID string `json:"patternId,omitempty"`
	Message   string `json:"message,omitempty"`
}

// PreCommandContext represents context for pre-command hooks.
type PreCommandContext struct {
	Command            string `json:"command"`
	WorkingDirectory   string `json:"workingDirectory,omitempty"`
	IncludeRiskAssessment bool `json:"includeRiskAssessment"`
	IncludeSuggestions bool   `json:"includeSuggestions"`
}

// PreCommandResult represents the result of a pre-command hook.
type PreCommandResult struct {
	Command        string          `json:"command"`
	RiskAssessment *RiskAssessment `json:"riskAssessment,omitempty"`
	Suggestions    []string        `json:"suggestions,omitempty"`
	SimilarPatterns []*Pattern     `json:"similarPatterns,omitempty"`
}

// PostCommandContext represents context for post-command hooks.
type PostCommandContext struct {
	Command       string `json:"command"`
	ExitCode      int    `json:"exitCode"`
	Success       bool   `json:"success"`
	Output        string `json:"output,omitempty"`
	Error         string `json:"error,omitempty"`
	ExecutionTime int64  `json:"executionTimeMs"`
}

// PostCommandResult represents the result of a post-command hook.
type PostCommandResult struct {
	Recorded  bool   `json:"recorded"`
	PatternID string `json:"patternId,omitempty"`
	Message   string `json:"message,omitempty"`
}

// Hooks-related errors
var (
	// ErrHookNotFound is returned when a hook is not found.
	ErrHookNotFound = errors.New("hook not found")
	// ErrHookAlreadyExists is returned when a hook already exists.
	ErrHookAlreadyExists = errors.New("hook already exists")
	// ErrMaxHooksReached is returned when the maximum number of hooks is reached.
	ErrMaxHooksReached = errors.New("maximum number of hooks reached")
	// ErrPatternNotFound is returned when a pattern is not found.
	ErrPatternNotFound = errors.New("pattern not found")
	// ErrMaxPatternsReached is returned when the maximum number of patterns is reached.
	ErrMaxPatternsReached = errors.New("maximum number of patterns reached")
	// ErrHookExecutionTimeout is returned when hook execution times out.
	ErrHookExecutionTimeout = errors.New("hook execution timed out")
)

// ============================================================================
// Utility Functions
// ============================================================================

// Now returns the current time in milliseconds.
func Now() int64 {
	return time.Now().UnixMilli()
}
