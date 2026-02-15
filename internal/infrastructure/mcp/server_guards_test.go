package mcp

import (
	"context"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

type guardTestProvider struct{}

func (p guardTestProvider) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "guard/provider-tool",
			Description: "guard provider tool",
			Parameters: map[string]interface{}{
				"type": "object",
			},
		},
	}
}

func (p guardTestProvider) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	if toolName != "guard/provider-tool" {
		return nil, nil
	}
	return &shared.MCPToolResult{Success: true, Data: map[string]interface{}{"ok": true}}, nil
}

func TestServer_NilReceiverMethodsFailGracefully(t *testing.T) {
	var server *Server

	if err := server.Start(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected nil Start initialization error, got %v", err)
	}
	if err := server.Stop(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected nil Stop initialization error, got %v", err)
	}

	if tools := server.ListTools(); len(tools) != 0 {
		t.Fatalf("expected nil ListTools empty, got %d", len(tools))
	}

	status := server.GetStatus()
	if running, ok := status["running"].(bool); !ok || running {
		t.Fatalf("expected nil status running=false, got %v", status["running"])
	}
	if status["error"] != "mcp server is not initialized" {
		t.Fatalf("expected nil status initialization error, got %v", status["error"])
	}

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{ID: "nil", Method: "tools/list", Params: map[string]interface{}{}})
	if resp.Error == nil {
		t.Fatal("expected nil HandleRequest error")
	}
	if resp.Error.Message != "mcp server is not initialized" {
		t.Fatalf("expected nil HandleRequest initialization error, got %q", resp.Error.Message)
	}

	// Should be safe no-ops on nil receiver.
	server.RegisterTool(shared.MCPTool{Name: "guard/nil-tool"})
	server.AddToolProvider(guardTestProvider{})
	server.RegisterMockProvider()

	if server.GetResources() != nil || server.GetPrompts() != nil || server.GetSampling() != nil ||
		server.GetCompletion() != nil || server.GetLogging() != nil || server.GetCapabilities() != nil ||
		server.GetTasks() != nil || server.GetHooks() != nil || server.GetSessions() != nil {
		t.Fatal("expected nil receiver component accessors to return nil")
	}
}

func TestServer_ZeroValueMethodsFailGracefully(t *testing.T) {
	var server Server

	if err := server.Start(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected zero-value Start initialization error, got %v", err)
	}
	if err := server.Stop(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected zero-value Stop initialization error, got %v", err)
	}

	if tools := server.ListTools(); len(tools) != 0 {
		t.Fatalf("expected zero-value ListTools empty, got %d", len(tools))
	}

	status := server.GetStatus()
	if running, ok := status["running"].(bool); !ok || running {
		t.Fatalf("expected zero-value status running=false, got %v", status["running"])
	}
	if status["error"] != "mcp server is not initialized" {
		t.Fatalf("expected zero-value status initialization error, got %v", status["error"])
	}

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{ID: "zero", Method: "tools/list", Params: map[string]interface{}{}})
	if resp.Error == nil {
		t.Fatal("expected zero-value HandleRequest error")
	}
	if resp.Error.Message != "mcp server is not initialized" {
		t.Fatalf("expected zero-value HandleRequest initialization error, got %q", resp.Error.Message)
	}

	// Should be safe operations without panics.
	server.RegisterTool(shared.MCPTool{Name: "guard/zero-tool"})
	server.AddToolProvider(guardTestProvider{})
	server.RegisterMockProvider()

	if tools := server.ListTools(); len(tools) != 2 {
		t.Fatalf("expected RegisterTool/AddToolProvider to register 2 tools, got %d", len(tools))
	}
}

func TestServer_NewServerConfiguredPathStillWorks(t *testing.T) {
	server := NewServer(Options{})
	if server == nil {
		t.Fatal("expected MCP server")
	}

	server.RegisterTool(shared.MCPTool{
		Name:        "guard/direct-tool",
		Description: "guard direct tool",
		Parameters: map[string]interface{}{
			"type": "object",
		},
	})
	server.AddToolProvider(guardTestProvider{})

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected configured server to list tools")
	}

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{ID: "ok", Method: "tools/list", Params: map[string]interface{}{}})
	if resp.Error != nil {
		t.Fatalf("expected configured HandleRequest to succeed, got %v", resp.Error)
	}

	status := server.GetStatus()
	if running, ok := status["running"].(bool); !ok || running {
		t.Fatalf("expected configured idle status running=false, got %v", status["running"])
	}
}
