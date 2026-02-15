// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/federation"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestFederationTools_ImplementsMCPToolProvider(t *testing.T) {
	var _ shared.MCPToolProvider = (*FederationTools)(nil)
}

func TestFederationTools_GetTools(t *testing.T) {
	ft := &FederationTools{}

	tools := ft.GetTools()
	if len(tools) == 0 {
		t.Fatal("expected federation tools to be registered")
	}
}

func TestFederationTools_GetTools_ExpectedUniqueNames(t *testing.T) {
	ft := &FederationTools{}

	tools := ft.GetTools()
	if len(tools) == 0 {
		t.Fatal("expected federation tools to be registered")
	}

	expectedNames := map[string]bool{
		"federation/status":              true,
		"federation/spawn-ephemeral":     true,
		"federation/terminate-ephemeral": true,
		"federation/list-ephemeral":      true,
		"federation/register-swarm":      true,
		"federation/broadcast":           true,
		"federation/propose":             true,
		"federation/vote":                true,
	}

	if len(tools) != len(expectedNames) {
		t.Fatalf("expected %d federation tools, got %d", len(expectedNames), len(tools))
	}

	seen := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("duplicate federation tool name: %s", tool.Name)
		}
		seen[tool.Name] = true

		if !expectedNames[tool.Name] {
			t.Fatalf("unexpected federation tool name: %s", tool.Name)
		}
	}

	for name := range expectedNames {
		if !seen[name] {
			t.Fatalf("expected federation tool missing: %s", name)
		}
	}
}

func TestFederationTools_GetTools_HaveObjectSchemasAndRequiredFields(t *testing.T) {
	ft := &FederationTools{}

	tools := ft.GetTools()
	if len(tools) == 0 {
		t.Fatal("expected federation tools to be registered")
	}

	type requiredExpectation struct {
		fields map[string]bool
	}
	expectedRequired := map[string]requiredExpectation{
		"federation/spawn-ephemeral":     {fields: map[string]bool{"type": true, "task": true}},
		"federation/terminate-ephemeral": {fields: map[string]bool{"agentId": true}},
		"federation/register-swarm":      {fields: map[string]bool{"swarmId": true, "name": true, "maxAgents": true}},
		"federation/broadcast":           {fields: map[string]bool{"sourceSwarmId": true, "payload": true}},
		"federation/propose":             {fields: map[string]bool{"proposerId": true, "proposalType": true, "value": true}},
		"federation/vote":                {fields: map[string]bool{"voterId": true, "proposalId": true, "approve": true}},
	}

	for _, tool := range tools {
		if tool.Description == "" {
			t.Fatalf("tool %s should have a non-empty description", tool.Name)
		}

		if tool.Parameters["type"] != "object" {
			t.Fatalf("tool %s should use an object schema, got %v", tool.Name, tool.Parameters["type"])
		}

		expectation, needsRequired := expectedRequired[tool.Name]
		if !needsRequired {
			continue
		}

		rawRequired, ok := tool.Parameters["required"]
		if !ok {
			t.Fatalf("tool %s should define required fields", tool.Name)
		}

		requiredList, ok := rawRequired.([]string)
		if !ok {
			t.Fatalf("tool %s required fields should be []string, got %T", tool.Name, rawRequired)
		}

		if len(requiredList) != len(expectation.fields) {
			t.Fatalf("tool %s expected %d required fields, got %d", tool.Name, len(expectation.fields), len(requiredList))
		}

		seen := make(map[string]bool, len(requiredList))
		for _, field := range requiredList {
			seen[field] = true
		}

		for field := range expectation.fields {
			if !seen[field] {
				t.Fatalf("tool %s missing required field %q", tool.Name, field)
			}
		}
	}
}

func TestFederationTools_Execute_UnknownTool(t *testing.T) {
	ft := &FederationTools{}

	result, err := ft.Execute(context.Background(), "federation/unknown-tool", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Success {
		t.Fatal("unknown tool should not succeed")
	}
	if result.Error == "" {
		t.Fatal("expected error message in tool result")
	}
}

func TestFederationTools_Execute_Status(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	result, err := ft.Execute(context.Background(), "federation/status", map[string]interface{}{})
	if err != nil {
		t.Fatalf("expected status tool to succeed, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Success {
		t.Fatalf("expected successful result, got error: %s", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected status data map, got %T", result.Data)
	}
	if _, ok := data["federationId"]; !ok {
		t.Fatal("expected federationId in status response")
	}
	if _, ok := data["stats"]; !ok {
		t.Fatal("expected stats in status response")
	}
}

func TestFederationTools_ExecuteAndExecuteTool_StatusParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/status", map[string]interface{}{})
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/status", map[string]interface{}{})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected Execute and ExecuteTool success parity, got %v vs %v", execResult.Success, directResult.Success)
	}

	execData, ok := execResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute data map, got %T", execResult.Data)
	}
	directData, ok := directResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool data map, got %T", directResult.Data)
	}

	if _, ok := execData["federationId"]; !ok {
		t.Fatal("expected Execute data to contain federationId")
	}
	if _, ok := directData["federationId"]; !ok {
		t.Fatal("expected ExecuteTool data to contain federationId")
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)
	args := map[string]interface{}{}

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", args)
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", args)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execAgents, ok := execResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected Execute data type []*shared.EphemeralAgent, got %T", execResult.Data)
	}
	directAgents, ok := directResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected ExecuteTool data type []*shared.EphemeralAgent, got %T", directResult.Data)
	}

	if len(execAgents) != len(directAgents) {
		t.Fatalf("expected list parity, got Execute=%d ExecuteTool=%d", len(execAgents), len(directAgents))
	}
}

func TestFederationTools_Execute_ValidationErrorPropagation(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	result, err := ft.Execute(context.Background(), "federation/terminate-ephemeral", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected validation error for missing agentId")
	}
	if result == nil {
		t.Fatal("expected non-nil result for validation failure")
	}
	if result.Success {
		t.Fatal("validation failure should not report success")
	}
	if result.Error == "" {
		t.Fatal("expected validation error message in result")
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ValidationErrorParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)
	args := map[string]interface{}{} // missing required agentId

	execResult, execErr := ft.Execute(context.Background(), "federation/terminate-ephemeral", args)
	if execErr == nil {
		t.Fatal("expected Execute validation error")
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/terminate-ephemeral", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool validation error")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	if execResult.Error != directResult.Error {
		t.Fatalf("expected error message parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_RegisterSwarmValidationParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"name":      "missing-id",
		"maxAgents": float64(5),
	} // swarmId intentionally omitted

	execResult, execErr := ft.Execute(context.Background(), "federation/register-swarm", args)
	if execErr == nil {
		t.Fatal("expected Execute validation error")
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/register-swarm", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool validation error")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}
	if execResult.Error != directResult.Error {
		t.Fatalf("expected error message parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ValidationParityForRequiredFields(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
	}{
		{
			name:     "broadcast missing sourceSwarmId",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"payload": map[string]interface{}{"event": "x"},
			},
		},
		{
			name:     "propose missing proposerId",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposalType": "scaling",
				"value":        map[string]interface{}{"maxAgents": 10},
			},
		},
		{
			name:     "vote missing voterId",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"proposalId": "p-1",
				"approve":    true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			execResult, execErr := ft.Execute(context.Background(), tc.toolName, tc.args)
			if execErr == nil {
				t.Fatalf("expected Execute validation error for %s", tc.toolName)
			}
			if execResult == nil {
				t.Fatalf("expected Execute result for %s", tc.toolName)
			}

			directResult, directErr := ft.ExecuteTool(context.Background(), tc.toolName, tc.args)
			if directErr == nil {
				t.Fatalf("expected ExecuteTool validation error for %s", tc.toolName)
			}

			if execResult.Success != directResult.Success {
				t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
			}
			if execResult.Error != directResult.Error {
				t.Fatalf("expected error parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
			}
		})
	}
}

func TestFederationTools_ExecuteAndExecuteTool_UnknownToolParity(t *testing.T) {
	ft := &FederationTools{}

	args := map[string]interface{}{}
	execResult, execErr := ft.Execute(context.Background(), "federation/unknown-tool", args)
	if execErr == nil {
		t.Fatal("expected Execute error for unknown tool")
	}
	if execResult == nil {
		t.Fatal("expected Execute result for unknown tool")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/unknown-tool", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool error for unknown tool")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}
	if execResult.Error != directResult.Error {
		t.Fatalf("expected error message parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}
