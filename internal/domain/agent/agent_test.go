package agent

import (
	"context"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestNewAgent(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
		Role: shared.AgentRoleWorker,
	})

	if agent.ID != "agent-1" {
		t.Fatalf("expected agent-1, got %s", agent.ID)
	}
	if agent.Type != shared.AgentTypeCoder {
		t.Fatalf("expected coder, got %s", agent.Type)
	}
	if agent.Status != shared.AgentStatusActive {
		t.Fatalf("expected active, got %s", agent.Status)
	}
	if len(agent.Capabilities) == 0 {
		t.Fatal("coder should have default capabilities")
	}
}

func TestNewAgentDefaultCapabilities(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
	})

	caps := agent.Capabilities
	if len(caps) != 3 {
		t.Fatalf("expected 3 default coder caps, got %d", len(caps))
	}

	expected := map[string]bool{"code": true, "refactor": true, "debug": true}
	for _, c := range caps {
		if !expected[c] {
			t.Fatalf("unexpected capability: %s", c)
		}
	}
}

func TestNewAgentCustomCapabilities(t *testing.T) {
	agent := New(Config{
		ID:           "agent-1",
		Type:         shared.AgentTypeCoder,
		Capabilities: []string{"custom-cap"},
	})

	if len(agent.Capabilities) != 1 || agent.Capabilities[0] != "custom-cap" {
		t.Fatal("custom capabilities should be used when provided")
	}
}

func TestAgentHasCapability(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
	})

	if !agent.HasCapability("code") {
		t.Fatal("coder should have code capability")
	}
	if agent.HasCapability("fly") {
		t.Fatal("coder should not have fly capability")
	}
}

func TestAgentCanExecute(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
	})

	if !agent.CanExecute(shared.TaskTypeCode) {
		t.Fatal("coder should be able to execute code tasks")
	}
	if agent.CanExecute(shared.TaskTypeTest) {
		t.Fatal("coder should not be able to execute test tasks")
	}
}

func TestAgentTerminate(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
	})

	agent.Terminate()
	if agent.GetStatus() != shared.AgentStatusTerminated {
		t.Fatal("agent should be terminated")
	}
}

func TestAgentSetIdle(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
	})

	agent.SetIdle()
	if agent.GetStatus() != shared.AgentStatusIdle {
		t.Fatal("agent should be idle")
	}
}

func TestAgentActivate(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
	})

	agent.SetIdle()
	agent.Activate()
	if agent.GetStatus() != shared.AgentStatusActive {
		t.Fatal("agent should be active")
	}
}

func TestAgentActivateAfterTerminate(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
	})

	agent.Terminate()
	agent.Activate()
	if agent.GetStatus() != shared.AgentStatusTerminated {
		t.Fatal("terminated agent should not be reactivatable")
	}
}

func TestAgentIsQueen(t *testing.T) {
	queen := New(Config{
		ID:          "queen",
		Type:        shared.AgentTypeQueen,
		AgentNumber: 1,
	})
	if !queen.IsQueen() {
		t.Fatal("queen should be queen")
	}

	worker := New(Config{
		ID:          "worker",
		Type:        shared.AgentTypeCoder,
		AgentNumber: 5,
	})
	if worker.IsQueen() {
		t.Fatal("worker should not be queen")
	}
}

func TestAgentDomainFromNumber(t *testing.T) {
	agent := New(Config{
		ID:          "agent-2",
		Type:        shared.AgentTypeSecurityArchitect,
		AgentNumber: 2,
	})

	if agent.Domain != shared.DomainSecurity {
		t.Fatalf("agent 2 should be in security domain, got %s", agent.Domain)
	}
}

func TestAgentToShared(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
		Role: shared.AgentRoleWorker,
	})

	shared := agent.ToShared()
	if shared.ID != "agent-1" {
		t.Fatal("shared ID mismatch")
	}
	if shared.Type != agent.Type {
		t.Fatal("shared Type mismatch")
	}
	if shared.Status != agent.Status {
		t.Fatal("shared Status mismatch")
	}
}

func TestAgentExecuteTask(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
	})

	task := shared.Task{
		ID:       "task-1",
		Priority: shared.PriorityHigh,
	}

	ctx := context.Background()
	result := agent.ExecuteTask(ctx, task)
	if result.Status != shared.TaskStatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if result.AgentID != "agent-1" {
		t.Fatalf("expected agent-1, got %s", result.AgentID)
	}
}

func TestAgentExecuteTaskTerminated(t *testing.T) {
	agent := New(Config{
		ID:   "agent-1",
		Type: shared.AgentTypeCoder,
	})

	agent.Terminate()

	task := shared.Task{
		ID:       "task-1",
		Priority: shared.PriorityHigh,
	}

	ctx := context.Background()
	result := agent.ExecuteTask(ctx, task)
	if result.Status != shared.TaskStatusFailed {
		t.Fatal("terminated agent should fail task")
	}
}

func TestFromConfig(t *testing.T) {
	config := shared.AgentConfig{
		ID:   "agent-1",
		Type: shared.AgentTypeTester,
		Role: shared.AgentRoleWorker,
	}

	agent := FromConfig(config)
	if agent.ID != "agent-1" {
		t.Fatal("ID mismatch")
	}
	if agent.Type != shared.AgentTypeTester {
		t.Fatal("Type mismatch")
	}
}

func TestGetDefaultCapabilities(t *testing.T) {
	tests := []struct {
		agentType shared.AgentType
		minCaps   int
	}{
		{shared.AgentTypeCoder, 3},
		{shared.AgentTypeTester, 3},
		{shared.AgentTypeQueen, 5},
		{shared.AgentTypeSecurityArchitect, 3},
	}

	for _, tt := range tests {
		caps := GetDefaultCapabilities(tt.agentType)
		if len(caps) < tt.minCaps {
			t.Errorf("agent type %s: expected at least %d caps, got %d", tt.agentType, tt.minCaps, len(caps))
		}
	}
}

func TestGetDefaultCapabilitiesUnknown(t *testing.T) {
	caps := GetDefaultCapabilities("unknown-type")
	if len(caps) != 0 {
		t.Fatal("unknown type should return empty capabilities")
	}
}

func TestAgentTypeRegistry(t *testing.T) {
	registry := NewAgentTypeRegistry()

	if registry.Count() == 0 {
		t.Fatal("registry should have default specs")
	}

	if !registry.HasType(shared.AgentTypeCoder) {
		t.Fatal("registry should have coder type")
	}

	spec := registry.GetSpec(shared.AgentTypeCoder)
	if spec == nil {
		t.Fatal("spec should not be nil")
	}
	if spec.ModelTier != ModelTierSonnet {
		t.Fatalf("coder should be sonnet tier, got %s", spec.ModelTier)
	}
}

func TestRegistryListByCapability(t *testing.T) {
	registry := NewAgentTypeRegistry()

	coders := registry.ListByCapability("code-generation")
	if len(coders) == 0 {
		t.Fatal("should find types with code-generation capability")
	}

	none := registry.ListByCapability("nonexistent")
	if len(none) != 0 {
		t.Fatal("should find no types with nonexistent capability")
	}
}

func TestRegistryListByDomain(t *testing.T) {
	registry := NewAgentTypeRegistry()

	security := registry.ListByDomain(shared.DomainSecurity)
	if len(security) == 0 {
		t.Fatal("should find security domain types")
	}
}

func TestRegistryFindBestMatch(t *testing.T) {
	registry := NewAgentTypeRegistry()

	bestType, score := registry.FindBestMatch([]string{"code-generation", "debugging"})
	if score == 0 {
		t.Fatal("should find a matching type")
	}
	if bestType == "" {
		t.Fatal("best type should not be empty")
	}
}

func TestRegistryListByModelTier(t *testing.T) {
	registry := NewAgentTypeRegistry()

	opus := registry.ListByModelTier(ModelTierOpus)
	if len(opus) == 0 {
		t.Fatal("should find opus-tier types")
	}

	sonnet := registry.ListByModelTier(ModelTierSonnet)
	if len(sonnet) == 0 {
		t.Fatal("should find sonnet-tier types")
	}
}

func TestRegistryGetNeuralPatterns(t *testing.T) {
	registry := NewAgentTypeRegistry()

	patterns := registry.GetNeuralPatterns(shared.AgentTypeQueen)
	if len(patterns) == 0 {
		t.Fatal("queen should have neural patterns")
	}

	unknown := registry.GetNeuralPatterns("unknown")
	if len(unknown) != 0 {
		t.Fatal("unknown type should have no patterns")
	}
}
