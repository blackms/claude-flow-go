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

type orderedProvider struct {
	tools []shared.MCPTool
}

func (p orderedProvider) GetTools() []shared.MCPTool {
	return p.tools
}

func (p orderedProvider) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	return nil, nil
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

func TestServer_ListToolsReturnsDeterministicSortedNames(t *testing.T) {
	server := &Server{
		toolRegistry: map[string]shared.MCPTool{
			"guard/tool-z": {Name: "guard/tool-z"},
			"guard/tool-a": {Name: "guard/tool-a"},
		},
		tools: []shared.MCPToolProvider{
			orderedProvider{
				tools: []shared.MCPTool{
					{Name: "guard/tool-m"},
					{Name: "guard/tool-b"},
					{Name: "guard/tool-a"}, // duplicate should be deduplicated.
				},
			},
		},
	}

	expected := []string{"guard/tool-a", "guard/tool-b", "guard/tool-m", "guard/tool-z"}
	for run := 0; run < 20; run++ {
		tools := server.ListTools()
		if len(tools) != len(expected) {
			t.Fatalf("run %d: expected %d tools, got %d", run, len(expected), len(tools))
		}
		for i, want := range expected {
			if tools[i].Name != want {
				t.Fatalf("run %d: expected sorted tool %q at index %d, got %q", run, want, i, tools[i].Name)
			}
		}
	}
}

func TestServer_HandleRequestToolsListIncludesSortedToolNames(t *testing.T) {
	server := NewServer(Options{})
	server.RegisterTool(shared.MCPTool{
		Name:        "guard/zzz-local",
		Description: "zzz local",
		Parameters:  map[string]interface{}{"type": "object"},
	})
	server.RegisterTool(shared.MCPTool{
		Name:        "guard/aaa-local",
		Description: "aaa local",
		Parameters:  map[string]interface{}{"type": "object"},
	})

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "tools-list-order",
		Method: "tools/list",
		Params: map[string]interface{}{},
	})
	if resp.Error != nil {
		t.Fatalf("expected tools/list success, got error %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	rawTools, ok := result["tools"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map tools result, got %T", result["tools"])
	}
	if len(rawTools) < 2 {
		t.Fatalf("expected at least custom tools in response, got %d entries", len(rawTools))
	}

	var sawAAA, sawZZZ bool
	prev := ""
	for i, tool := range rawTools {
		name, ok := tool["name"].(string)
		if !ok || name == "" {
			t.Fatalf("expected non-empty tool name at index %d, got %v", i, tool["name"])
		}
		if prev != "" && prev > name {
			t.Fatalf("expected sorted tool names, got %q before %q", prev, name)
		}
		if name == "guard/aaa-local" {
			sawAAA = true
		}
		if name == "guard/zzz-local" {
			sawZZZ = true
		}
		prev = name
	}
	if !sawAAA || !sawZZZ {
		t.Fatalf("expected custom registered tools in list, sawAAA=%v sawZZZ=%v", sawAAA, sawZZZ)
	}
}

func TestServer_OptionsAndLoopsIgnoreNilProviders(t *testing.T) {
	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			nil,
			guardTestProvider{},
		},
	})
	if server == nil {
		t.Fatal("expected server")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected ListTools to succeed with nil provider present")
	}

	var sawGuardProvider bool
	for _, tool := range tools {
		if tool.Name == "guard/provider-tool" {
			sawGuardProvider = true
			break
		}
	}
	if !sawGuardProvider {
		t.Fatalf("expected guard provider tool in list, got %+v", tools)
	}

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "nil-provider-dispatch",
		Method: "guard/provider-tool",
		Params: map[string]interface{}{},
	})
	if resp.Error != nil {
		t.Fatalf("expected request dispatch to skip nil provider and succeed, got %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result when non-nil provider handles request")
	}
}
