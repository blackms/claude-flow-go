// Package executor provides the TaskExecutor application service that bridges
// agents, memory, and LLM providers for real task execution.
package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/sampling"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// TaskExecutorConfig holds configuration for creating a TaskExecutor.
type TaskExecutorConfig struct {
	SamplingManager *sampling.SamplingManager
	MemoryBackend   shared.MemoryBackend
	AgentRegistry   *agent.AgentTypeRegistry
}

// TaskExecutor bridges agents → memory → LLM for real task execution.
type TaskExecutor struct {
	samplingMgr   *sampling.SamplingManager
	memoryBackend shared.MemoryBackend
	agentRegistry *agent.AgentTypeRegistry
}

// New creates a new TaskExecutor.
func New(cfg TaskExecutorConfig) *TaskExecutor {
	return &TaskExecutor{
		samplingMgr:   cfg.SamplingManager,
		memoryBackend: cfg.MemoryBackend,
		agentRegistry: cfg.AgentRegistry,
	}
}

// Execute runs a task using the LLM, incorporating agent spec and memory context.
func (te *TaskExecutor) Execute(ctx context.Context, a *agent.Agent, t shared.Task, phaseContext string) (shared.TaskResult, error) {
	// 1. Build system prompt from agent type spec.
	systemPrompt := te.buildSystemPrompt(a)

	// 2. Retrieve relevant memories.
	memoryContext := te.retrieveMemoryContext(a.ID, t.ID)

	// 3. Build user message.
	userMessage := te.buildUserMessage(t, phaseContext, memoryContext)

	// 4. Map agent's ModelTier → ModelPreferences.
	prefs := te.modelPreferences(a.Type)

	// 5. Call LLM.
	result, err := te.samplingMgr.CreateMessageWithContext(ctx, &shared.CreateMessageRequest{
		SystemPrompt: systemPrompt,
		Messages: []shared.SamplingMessage{
			{
				Role: "user",
				Content: []shared.PromptContent{
					{Type: "text", Text: userMessage},
				},
			},
		},
		ModelPreferences: prefs,
	})
	if err != nil {
		return shared.TaskResult{
			TaskID:  t.ID,
			Status:  shared.TaskStatusFailed,
			Error:   fmt.Sprintf("LLM execution failed: %v", err),
			AgentID: a.ID,
		}, err
	}

	// 6. Store LLM result in memory.
	te.storeResult(a.ID, t.ID, result.Content)

	// 7. Return TaskResult with LLM response.
	return shared.TaskResult{
		TaskID:  t.ID,
		Status:  shared.TaskStatusCompleted,
		Result:  result.Content,
		AgentID: a.ID,
	}, nil
}

// buildSystemPrompt constructs a system prompt from the agent's type spec.
func (te *TaskExecutor) buildSystemPrompt(a *agent.Agent) string {
	if te.agentRegistry == nil {
		return fmt.Sprintf("You are a specialized agent (type: %s). Execute the assigned task. Return concrete, actionable output.", a.Type)
	}

	spec := te.agentRegistry.GetSpec(a.Type)
	if spec == nil {
		return fmt.Sprintf("You are a specialized agent (type: %s). Execute the assigned task. Return concrete, actionable output.", a.Type)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("You are a %s specialized agent.\n", spec.Description))
	if len(spec.Capabilities) > 0 {
		b.WriteString(fmt.Sprintf("Capabilities: %s\n", strings.Join(spec.Capabilities, ", ")))
	}
	if spec.Domain != "" {
		b.WriteString(fmt.Sprintf("Domain: %s\n", spec.Domain))
	}
	b.WriteString("\nExecute the assigned task. Return concrete, actionable output.")
	return b.String()
}

// retrieveMemoryContext gathers relevant memories for the agent and task.
func (te *TaskExecutor) retrieveMemoryContext(agentID, taskID string) string {
	if te.memoryBackend == nil {
		return ""
	}

	// Query agent's prior task completions.
	memories, err := te.memoryBackend.Query(shared.MemoryQuery{
		AgentID: agentID,
		Type:    shared.MemoryTypeTaskComplete,
		Limit:   5,
	})
	if err != nil || len(memories) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Relevant prior context:\n")
	for _, m := range memories {
		b.WriteString(fmt.Sprintf("- %s\n", m.Content))
	}
	return b.String()
}

// buildUserMessage assembles the task description, phase context, and memory context.
func (te *TaskExecutor) buildUserMessage(t shared.Task, phaseContext, memoryContext string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task: %s\n", t.Description))

	if phaseContext != "" {
		b.WriteString(fmt.Sprintf("\nContext from previous phases:\n%s\n", phaseContext))
	}
	if memoryContext != "" {
		b.WriteString(fmt.Sprintf("\n%s\n", memoryContext))
	}

	return b.String()
}

// modelPreferences maps an agent type to model preferences using the registry.
func (te *TaskExecutor) modelPreferences(agentType shared.AgentType) *shared.ModelPreferences {
	tier := "sonnet" // default
	if te.agentRegistry != nil {
		spec := te.agentRegistry.GetSpec(agentType)
		if spec != nil {
			tier = string(spec.ModelTier)
		}
	}

	return &shared.ModelPreferences{
		Hints: []shared.ModelHint{{Name: tier}},
	}
}

// storeResult persists the LLM output in memory for future reference.
func (te *TaskExecutor) storeResult(agentID, taskID, content string) {
	if te.memoryBackend == nil {
		return
	}

	te.memoryBackend.Store(shared.Memory{
		ID:        fmt.Sprintf("exec-%s-%s", agentID, taskID),
		AgentID:   agentID,
		Content:   content,
		Type:      shared.MemoryTypeTaskComplete,
		Timestamp: shared.Now(),
		Metadata: map[string]interface{}{
			"taskId": taskID,
			"source": "llm-executor",
		},
	})
}
