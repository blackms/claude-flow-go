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
	AgentTypeTDDTester          AgentType = "tdd-tester"
	AgentTypePerformanceEngineer AgentType = "performance-engineer"
	AgentTypeReleaseManager      AgentType = "release-manager"
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
// Utility Functions
// ============================================================================

// Now returns the current time in milliseconds.
func Now() int64 {
	return time.Now().UnixMilli()
}
