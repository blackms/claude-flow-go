// Package agent provides the Agent domain entity.
package agent

import (
	"context"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Agent represents an AI agent in the V3 system.
type Agent struct {
	mu           sync.RWMutex
	ID           string
	Type         shared.AgentType
	Status       shared.AgentStatus
	Capabilities []string
	Role         shared.AgentRole
	Parent       string
	Metadata     map[string]interface{}
	CreatedAt    int64
	LastActive   int64
}

// Config holds configuration for creating an agent.
type Config struct {
	ID           string
	Type         shared.AgentType
	Capabilities []string
	Role         shared.AgentRole
	Parent       string
	Metadata     map[string]interface{}
}

// New creates a new Agent from the given configuration.
func New(config Config) *Agent {
	now := shared.Now()
	capabilities := config.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}
	metadata := config.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &Agent{
		ID:           config.ID,
		Type:         config.Type,
		Status:       shared.AgentStatusActive,
		Capabilities: capabilities,
		Role:         config.Role,
		Parent:       config.Parent,
		Metadata:     metadata,
		CreatedAt:    now,
		LastActive:   now,
	}
}

// FromConfig creates an Agent from a shared.AgentConfig.
func FromConfig(config shared.AgentConfig) *Agent {
	return New(Config{
		ID:           config.ID,
		Type:         config.Type,
		Capabilities: config.Capabilities,
		Role:         config.Role,
		Parent:       config.Parent,
		Metadata:     config.Metadata,
	})
}

// ExecuteTask executes a task assigned to this agent.
func (a *Agent) ExecuteTask(ctx context.Context, task shared.Task) shared.TaskResult {
	a.mu.Lock()
	status := a.Status
	a.mu.Unlock()

	if status != shared.AgentStatusActive && status != shared.AgentStatusIdle {
		return shared.TaskResult{
			TaskID:  task.ID,
			Status:  shared.TaskStatusFailed,
			Error:   "Agent " + a.ID + " is not available (status: " + string(a.Status) + ")",
			AgentID: a.ID,
		}
	}

	startTime := time.Now()

	a.mu.Lock()
	a.Status = shared.AgentStatusBusy
	a.LastActive = shared.Now()
	a.mu.Unlock()

	var execErr error

	// Execute task-specific callback if provided
	if task.OnExecute != nil {
		execErr = task.OnExecute(ctx)
	}

	if execErr == nil {
		// Process task with minimal overhead
		execErr = a.processTaskExecution(ctx, task)
	}

	duration := time.Since(startTime).Milliseconds()

	a.mu.Lock()
	a.Status = shared.AgentStatusActive
	a.LastActive = shared.Now()
	a.mu.Unlock()

	if execErr != nil {
		return shared.TaskResult{
			TaskID:   task.ID,
			Status:   shared.TaskStatusFailed,
			Error:    execErr.Error(),
			Duration: duration,
			AgentID:  a.ID,
		}
	}

	return shared.TaskResult{
		TaskID:   task.ID,
		Status:   shared.TaskStatusCompleted,
		Result:   "Task " + task.ID + " completed successfully",
		Duration: duration,
		AgentID:  a.ID,
	}
}

// processTaskExecution executes task processing with priority-based timing.
func (a *Agent) processTaskExecution(ctx context.Context, task shared.Task) error {
	// Minimal processing overhead based on priority
	processingTime := map[shared.TaskPriority]time.Duration{
		shared.PriorityHigh:   1 * time.Millisecond,
		shared.PriorityMedium: 5 * time.Millisecond,
		shared.PriorityLow:    10 * time.Millisecond,
	}

	overhead := processingTime[task.Priority]
	if overhead == 0 {
		overhead = 5 * time.Millisecond
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(overhead):
		return nil
	}
}

// HasCapability checks if the agent has a specific capability.
func (a *Agent) HasCapability(capability string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, cap := range a.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// CanExecute checks if the agent can execute a task type.
func (a *Agent) CanExecute(taskType shared.TaskType) bool {
	typeToCapability := map[shared.TaskType]string{
		shared.TaskTypeCode:   "code",
		shared.TaskTypeTest:   "test",
		shared.TaskTypeReview: "review",
		shared.TaskTypeDesign: "design",
		shared.TaskTypeDeploy: "deploy",
	}

	requiredCapability, exists := typeToCapability[taskType]
	if !exists {
		return true
	}
	return a.HasCapability(requiredCapability)
}

// Terminate terminates the agent.
func (a *Agent) Terminate() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Status = shared.AgentStatusTerminated
	a.LastActive = shared.Now()
}

// SetIdle marks the agent as idle.
func (a *Agent) SetIdle() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Status == shared.AgentStatusActive || a.Status == shared.AgentStatusBusy {
		a.Status = shared.AgentStatusIdle
		a.LastActive = shared.Now()
	}
}

// Activate activates the agent.
func (a *Agent) Activate() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Status != shared.AgentStatusTerminated {
		a.Status = shared.AgentStatusActive
		a.LastActive = shared.Now()
	}
}

// GetStatus returns the current status of the agent.
func (a *Agent) GetStatus() shared.AgentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Status
}

// ToShared converts the Agent to a shared.Agent.
func (a *Agent) ToShared() shared.Agent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return shared.Agent{
		ID:           a.ID,
		Type:         a.Type,
		Status:       a.Status,
		Capabilities: a.Capabilities,
		Role:         a.Role,
		Parent:       a.Parent,
		Metadata:     a.Metadata,
		CreatedAt:    a.CreatedAt,
		LastActive:   a.LastActive,
	}
}

// GetDefaultCapabilities returns default capabilities for an agent type.
func GetDefaultCapabilities(agentType shared.AgentType) []string {
	defaults := map[shared.AgentType][]string{
		shared.AgentTypeCoder:       {"code", "refactor", "debug"},
		shared.AgentTypeTester:      {"test", "validate", "e2e"},
		shared.AgentTypeReviewer:    {"review", "analyze", "security-audit"},
		shared.AgentTypeCoordinator: {"coordinate", "manage", "orchestrate"},
		shared.AgentTypeDesigner:    {"design", "prototype"},
		shared.AgentTypeDeployer:    {"deploy", "release"},
	}

	caps, exists := defaults[agentType]
	if !exists {
		return []string{}
	}
	return caps
}
