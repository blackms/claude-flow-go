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
