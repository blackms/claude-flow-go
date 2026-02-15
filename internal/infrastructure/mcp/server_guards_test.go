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

type mixedNameProvider struct {
	tools []shared.MCPTool
}

func (p mixedNameProvider) GetTools() []shared.MCPTool {
	return p.tools
}

func (p mixedNameProvider) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	return nil, nil
}

type panicProvider struct {
	panicOnGetTools bool
	panicOnExecute  bool
}

func (p panicProvider) GetTools() []shared.MCPTool {
	if p.panicOnGetTools {
		panic("panic provider GetTools")
	}
	return []shared.MCPTool{{Name: "guard/panic-provider"}}
}

func (p panicProvider) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	if p.panicOnExecute {
		panic("panic provider Execute")
	}
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

func TestServer_GetCapabilitiesReturnsDefensiveCopy(t *testing.T) {
	server := NewServer(Options{})
	if server == nil {
		t.Fatal("expected server")
	}

	// Seed internal experimental map for copy-on-read checks.
	server.mu.Lock()
	server.capabilities.Experimental = map[string]interface{}{
		"alpha": "one",
	}
	server.mu.Unlock()

	caps := server.GetCapabilities()
	if caps == nil {
		t.Fatal("expected capabilities")
	}

	// Mutate returned snapshot.
	caps.Logging.Level = shared.MCPLogLevelDebug
	caps.Resources.Subscribe = false
	caps.Tools.ListChanged = false
	caps.Experimental["alpha"] = "mutated"
	caps.Experimental["beta"] = "new"

	// Re-read and ensure internal state is unchanged.
	refetched := server.GetCapabilities()
	if refetched == nil {
		t.Fatal("expected refetched capabilities")
	}
	if refetched.Logging.Level != shared.MCPLogLevelInfo {
		t.Fatalf("expected internal logging level to remain %q, got %q", shared.MCPLogLevelInfo, refetched.Logging.Level)
	}
	if refetched.Resources == nil || !refetched.Resources.Subscribe {
		t.Fatalf("expected internal resources.subscribe to remain true, got %+v", refetched.Resources)
	}
	if refetched.Tools == nil || !refetched.Tools.ListChanged {
		t.Fatalf("expected internal tools.listChanged to remain true, got %+v", refetched.Tools)
	}
	if got := refetched.Experimental["alpha"]; got != "one" {
		t.Fatalf("expected experimental alpha to remain \"one\", got %v", got)
	}
	if _, ok := refetched.Experimental["beta"]; ok {
		t.Fatalf("expected experimental beta key to be absent, got %v", refetched.Experimental["beta"])
	}
}

func TestServer_RegisterToolAndProviderToolsNormalizeNames(t *testing.T) {
	server := NewServer(Options{})
	if server == nil {
		t.Fatal("expected server")
	}

	server.RegisterTool(shared.MCPTool{
		Name:        "  guard/trimmed-register  ",
		Description: "trimmed",
		Parameters:  map[string]interface{}{"type": "object"},
	})
	server.RegisterTool(shared.MCPTool{
		Name:        "   ",
		Description: "should be ignored",
	})
	server.AddToolProvider(mixedNameProvider{
		tools: []shared.MCPTool{
			{Name: "  guard/trimmed-provider  "},
			{Name: ""},
			{Name: "   "},
		},
	})

	tools := server.ListTools()
	var sawTrimmedRegister, sawTrimmedProvider bool
	for _, tool := range tools {
		if tool.Name != normalizeToolName(tool.Name) {
			t.Fatalf("expected normalized tool name, got %q", tool.Name)
		}
		if tool.Name == "guard/trimmed-register" {
			sawTrimmedRegister = true
		}
		if tool.Name == "guard/trimmed-provider" {
			sawTrimmedProvider = true
		}
		if tool.Name == "" {
			t.Fatalf("expected blank tool names to be excluded, got %+v", tools)
		}
	}
	if !sawTrimmedRegister || !sawTrimmedProvider {
		t.Fatalf("expected normalized names to be present, saw register=%v provider=%v", sawTrimmedRegister, sawTrimmedProvider)
	}
}

func TestServer_HandleRequestAndToolsCallTrimNames(t *testing.T) {
	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			guardTestProvider{},
		},
	})

	trimmedMethodResp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "trimmed-method",
		Method: "  tools/list  ",
		Params: map[string]interface{}{},
	})
	if trimmedMethodResp.Error != nil {
		t.Fatalf("expected trimmed tools/list method to succeed, got %v", trimmedMethodResp.Error)
	}

	trimmedToolCallResp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "trimmed-tool-name",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name":      "  guard/provider-tool  ",
			"arguments": map[string]interface{}{},
		},
	})
	if trimmedToolCallResp.Error != nil {
		t.Fatalf("expected trimmed tool name to dispatch, got %v", trimmedToolCallResp.Error)
	}

	missingToolCallResp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "missing-tool-name",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": "   ",
		},
	})
	if missingToolCallResp.Error == nil || missingToolCallResp.Error.Message != "Missing tool name" {
		t.Fatalf("expected missing-tool-name validation for whitespace names, got %v", missingToolCallResp.Error)
	}
}

func TestServer_HandleRequestRejectsNilContext(t *testing.T) {
	server := NewServer(Options{})
	if server == nil {
		t.Fatal("expected server")
	}

	resp := server.HandleRequest(nil, shared.MCPRequest{
		ID:     "nil-context",
		Method: "tools/list",
		Params: map[string]interface{}{},
	})
	if resp.Error == nil {
		t.Fatal("expected error for nil context")
	}
	if resp.Error.Code != -32603 {
		t.Fatalf("expected internal error code for nil context, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "context is required" {
		t.Fatalf("expected context-required message, got %q", resp.Error.Message)
	}
}

func TestServer_HandleRequestValidationPrecedence(t *testing.T) {
	var unconfigured Server
	resp := unconfigured.HandleRequest(nil, shared.MCPRequest{
		ID:     "precedence",
		Method: "tools/list",
		Params: map[string]interface{}{},
	})
	if resp.Error == nil {
		t.Fatal("expected precedence error")
	}
	if resp.Error.Message != "mcp server is not initialized" {
		t.Fatalf("expected initialization error precedence, got %q", resp.Error.Message)
	}
}

func TestServer_HandleRequestIgnoresNilResultProviders(t *testing.T) {
	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			guardTestProvider{},
		},
	})

	methodResp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "nil-result-method",
		Method: "guard/unknown-method",
		Params: map[string]interface{}{},
	})
	if methodResp.Error == nil || methodResp.Error.Message != "Method not found: guard/unknown-method" {
		t.Fatalf("expected method-not-found for nil result provider path, got %+v", methodResp)
	}

	toolResp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "nil-result-tool-call",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name":      "guard/unknown-tool",
			"arguments": map[string]interface{}{},
		},
	})
	if toolResp.Error == nil || toolResp.Error.Message != "Tool not found: guard/unknown-tool" {
		t.Fatalf("expected tool-not-found for nil result provider path, got %+v", toolResp)
	}
}

func TestServer_PanickingProviderIsIsolated(t *testing.T) {
	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			panicProvider{panicOnGetTools: true, panicOnExecute: true},
			guardTestProvider{},
		},
	})

	tools := server.ListTools()
	var sawGuard bool
	for _, tool := range tools {
		if tool.Name == "guard/provider-tool" {
			sawGuard = true
		}
		if tool.Name == "guard/panic-provider" {
			t.Fatalf("expected panicking provider tool to be omitted, got %+v", tools)
		}
	}
	if !sawGuard {
		t.Fatalf("expected non-panicking provider tool to remain available, got %+v", tools)
	}

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "panic-provider-dispatch",
		Method: "guard/provider-tool",
		Params: map[string]interface{}{},
	})
	if resp.Error != nil {
		t.Fatalf("expected dispatch to skip panicking provider and succeed, got %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result from fallback provider")
	}
}

func TestServer_ListToolsReturnsDefensiveToolSchemaCopies(t *testing.T) {
	server := NewServer(Options{})
	server.RegisterTool(shared.MCPTool{
		Name:        "guard/schema-copy",
		Description: "schema copy",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"value": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"value"},
		},
	})

	readSchema := func() map[string]interface{} {
		tools := server.ListTools()
		for _, tool := range tools {
			if tool.Name == "guard/schema-copy" {
				return tool.Parameters
			}
		}
		return nil
	}

	firstSchema := readSchema()
	if firstSchema == nil {
		t.Fatal("expected tool schema")
	}

	// Mutate returned schema copy.
	properties, _ := firstSchema["properties"].(map[string]interface{})
	valueSchema, _ := properties["value"].(map[string]interface{})
	valueSchema["type"] = "number"
	required, _ := firstSchema["required"].([]string)
	if len(required) > 0 {
		required[0] = "mutated"
	}
	firstSchema["newField"] = "added"

	secondSchema := readSchema()
	if secondSchema == nil {
		t.Fatal("expected tool schema on second read")
	}
	secondProperties, _ := secondSchema["properties"].(map[string]interface{})
	secondValueSchema, _ := secondProperties["value"].(map[string]interface{})
	if secondValueSchema["type"] != "string" {
		t.Fatalf("expected original nested schema value type, got %v", secondValueSchema["type"])
	}
	secondRequired, _ := secondSchema["required"].([]string)
	if len(secondRequired) == 0 || secondRequired[0] != "value" {
		t.Fatalf("expected original required array, got %v", secondRequired)
	}
	if _, ok := secondSchema["newField"]; ok {
		t.Fatalf("expected schema to exclude external mutation field, got %v", secondSchema["newField"])
	}
}

func TestServer_HandleToolsListReturnsDefensiveSchemaCopies(t *testing.T) {
	server := NewServer(Options{})
	server.RegisterTool(shared.MCPTool{
		Name:        "guard/tools-list-schema",
		Description: "tools list schema",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"flag": map[string]interface{}{
					"type": "boolean",
				},
			},
		},
	})

	extractSchema := func(resp shared.MCPResponse) map[string]interface{} {
		result, ok := resp.Result.(map[string]interface{})
		if !ok {
			return nil
		}
		rawTools, ok := result["tools"].([]map[string]interface{})
		if !ok {
			return nil
		}
		for _, tool := range rawTools {
			if name, _ := tool["name"].(string); name == "guard/tools-list-schema" {
				schema, _ := tool["inputSchema"].(map[string]interface{})
				return schema
			}
		}
		return nil
	}

	firstResp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "tools-list-copy-1",
		Method: "tools/list",
		Params: map[string]interface{}{},
	})
	if firstResp.Error != nil {
		t.Fatalf("expected first tools/list success, got %v", firstResp.Error)
	}
	firstSchema := extractSchema(firstResp)
	if firstSchema == nil {
		t.Fatal("expected first tools/list schema")
	}

	firstProps, _ := firstSchema["properties"].(map[string]interface{})
	firstFlag, _ := firstProps["flag"].(map[string]interface{})
	firstFlag["type"] = "string"
	firstSchema["externallyMutated"] = true

	secondResp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "tools-list-copy-2",
		Method: "tools/list",
		Params: map[string]interface{}{},
	})
	if secondResp.Error != nil {
		t.Fatalf("expected second tools/list success, got %v", secondResp.Error)
	}
	secondSchema := extractSchema(secondResp)
	if secondSchema == nil {
		t.Fatal("expected second tools/list schema")
	}

	secondProps, _ := secondSchema["properties"].(map[string]interface{})
	secondFlag, _ := secondProps["flag"].(map[string]interface{})
	if secondFlag["type"] != "boolean" {
		t.Fatalf("expected original schema in second response, got %v", secondFlag["type"])
	}
	if _, ok := secondSchema["externallyMutated"]; ok {
		t.Fatalf("expected second schema to omit external mutation field, got %v", secondSchema["externallyMutated"])
	}
}
