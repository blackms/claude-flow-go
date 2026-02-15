package mcp

import (
	"context"
	"fmt"
	"reflect"
	"strings"
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

type mutatingProvider struct {
	methodName string
	paramKey   string
}

func (p mutatingProvider) GetTools() []shared.MCPTool {
	return []shared.MCPTool{{Name: p.methodName}}
}

func (p mutatingProvider) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	if toolName != p.methodName {
		return nil, nil
	}
	if params != nil {
		params[p.paramKey] = "mutated"
	}
	return nil, fmt.Errorf("intentionally unhandled")
}

type expectingProvider struct {
	methodName    string
	paramKey      string
	expectedValue string
}

func (p expectingProvider) GetTools() []shared.MCPTool {
	return []shared.MCPTool{{Name: p.methodName}}
}

func (p expectingProvider) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	if toolName != p.methodName {
		return nil, nil
	}
	actual, _ := params[p.paramKey].(string)
	if actual != p.expectedValue {
		return nil, fmt.Errorf("expected %s=%q, got %q", p.paramKey, p.expectedValue, actual)
	}
	return &shared.MCPToolResult{Success: true, Data: map[string]interface{}{"ok": true}}, nil
}

type nonSerializableResultProvider struct {
	methodName string
}

func (p nonSerializableResultProvider) GetTools() []shared.MCPTool {
	return []shared.MCPTool{{Name: p.methodName}}
}

func (p nonSerializableResultProvider) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	if toolName != p.methodName {
		return nil, nil
	}
	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"nonSerializable": func() {},
		},
	}, nil
}

type writableParamsProvider struct {
	methodName string
}

func (p writableParamsProvider) GetTools() []shared.MCPTool {
	return []shared.MCPTool{{Name: p.methodName}}
}

func (p writableParamsProvider) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	if toolName != p.methodName {
		return nil, nil
	}
	if params == nil {
		return nil, fmt.Errorf("params map is nil")
	}
	params["injected"] = "ok"
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

func TestServer_IsConfiguredReflectsStructuralState(t *testing.T) {
	var nilServer *Server
	if nilServer.IsConfigured() {
		t.Fatal("expected nil server to be unconfigured")
	}

	var zeroValue Server
	if zeroValue.IsConfigured() {
		t.Fatal("expected zero-value server to be unconfigured")
	}

	configured := NewServer(Options{})
	if configured == nil {
		t.Fatal("expected configured server")
	}
	if !configured.IsConfigured() {
		t.Fatal("expected NewServer instance to be configured")
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

func TestServer_GetStatusReturnsDefensiveCopy(t *testing.T) {
	server := NewServer(Options{})
	if server == nil {
		t.Fatal("expected server")
	}

	first := server.GetStatus()
	if first == nil {
		t.Fatal("expected status map")
	}
	first["running"] = true
	first["error"] = "mutated"
	first["toolCount"] = 999

	second := server.GetStatus()
	if second == nil {
		t.Fatal("expected second status map")
	}
	if running, ok := second["running"].(bool); !ok || running {
		t.Fatalf("expected defensive status running=false, got %v", second["running"])
	}
	if toolCount, ok := second["toolCount"].(int); !ok || toolCount == 999 {
		t.Fatalf("expected defensive status toolCount unchanged, got %v", second["toolCount"])
	}
	if second["error"] == "mutated" {
		t.Fatalf("expected defensive status to drop injected error field, got %v", second["error"])
	}
}

func TestServer_StartRejectsInvalidPortAndHost(t *testing.T) {
	invalidPortServer := NewServer(Options{Port: -1})
	if invalidPortServer == nil {
		t.Fatal("expected server with invalid port to be constructed")
	}
	if err := invalidPortServer.Start(); err == nil || err.Error() != "invalid port: -1" {
		t.Fatalf("expected invalid-port start error, got %v", err)
	}

	overflowPortServer := NewServer(Options{Port: 70000})
	if overflowPortServer == nil {
		t.Fatal("expected server with overflow port to be constructed")
	}
	if err := overflowPortServer.Start(); err == nil || err.Error() != "invalid port: 70000" {
		t.Fatalf("expected overflow-port start error, got %v", err)
	}

	malformedHostServer := NewServer(Options{})
	if malformedHostServer == nil {
		t.Fatal("expected server for malformed-host test")
	}
	malformedHostServer.host = "   "
	if err := malformedHostServer.Start(); err == nil || err.Error() != "host is required" {
		t.Fatalf("expected host-required start error, got %v", err)
	}
}

func TestServer_NewServerNormalizesHostInput(t *testing.T) {
	blankHostServer := NewServer(Options{Host: "   "})
	if blankHostServer == nil {
		t.Fatal("expected server with blank host options")
	}
	if blankHostServer.host != "localhost" {
		t.Fatalf("expected blank host to default to localhost, got %q", blankHostServer.host)
	}

	trimmedHostServer := NewServer(Options{Host: " 127.0.0.1 "})
	if trimmedHostServer == nil {
		t.Fatal("expected server with padded host options")
	}
	if trimmedHostServer.host != "127.0.0.1" {
		t.Fatalf("expected host trimming, got %q", trimmedHostServer.host)
	}
}

func TestServer_BuildListenAddressSupportsIPv6(t *testing.T) {
	if got := buildListenAddress("localhost", 3000); got != "localhost:3000" {
		t.Fatalf("expected localhost listen address, got %q", got)
	}
	if got := buildListenAddress("::1", 3000); got != "[::1]:3000" {
		t.Fatalf("expected IPv6 listen address with brackets, got %q", got)
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
		"nested": map[string]interface{}{
			"enabled": true,
		},
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
	nested, _ := caps.Experimental["nested"].(map[string]interface{})
	nested["enabled"] = false

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
	refetchedNested, _ := refetched.Experimental["nested"].(map[string]interface{})
	if got := refetchedNested["enabled"]; got != true {
		t.Fatalf("expected nested experimental flag to remain true, got %v", got)
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

func TestServer_HandleRequestRejectsBlankMethod(t *testing.T) {
	server := NewServer(Options{})
	if server == nil {
		t.Fatal("expected server")
	}

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "blank-method",
		Method: "   ",
		Params: map[string]interface{}{},
	})
	if resp.Error == nil {
		t.Fatal("expected blank method error")
	}
	if resp.Error.Code != -32600 {
		t.Fatalf("expected invalid request code for blank method, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "Method is required" {
		t.Fatalf("expected method-required error message, got %q", resp.Error.Message)
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

	toolCallResp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "panic-provider-tool-call",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name":      "guard/provider-tool",
			"arguments": map[string]interface{}{},
		},
	})
	if toolCallResp.Error != nil {
		t.Fatalf("expected tools/call to skip panicking provider and succeed, got %v", toolCallResp.Error)
	}
}

func TestServer_AddToolProviderPanickingGetToolsDoesNotCorruptRegistry(t *testing.T) {
	server := NewServer(Options{})
	if server == nil {
		t.Fatal("expected server")
	}

	baseline := server.ListTools()
	server.AddToolProvider(panicProvider{panicOnGetTools: true})
	after := server.ListTools()

	if len(after) != len(baseline) {
		t.Fatalf("expected panicking provider to add no tools, baseline=%d after=%d", len(baseline), len(after))
	}
	for i := range baseline {
		if baseline[i].Name != after[i].Name {
			t.Fatalf("expected tool ordering/content unchanged, mismatch at %d: %q != %q", i, baseline[i].Name, after[i].Name)
		}
	}
}

func TestServer_HandleRequestProviderParamsAreDefensivelyCopied(t *testing.T) {
	const method = "guard/copied-params"

	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			mutatingProvider{methodName: method, paramKey: "flag"},
			expectingProvider{methodName: method, paramKey: "flag", expectedValue: "original"},
		},
	})

	originalParams := map[string]interface{}{"flag": "original"}
	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "copied-params",
		Method: method,
		Params: originalParams,
	})
	if resp.Error != nil {
		t.Fatalf("expected provider chain success with copied params, got %v", resp.Error)
	}
	if got, _ := originalParams["flag"].(string); got != "original" {
		t.Fatalf("expected caller params to remain unchanged, got %q", got)
	}
}

func TestServer_HandleToolsCallProviderArgumentsAreDefensivelyCopied(t *testing.T) {
	const method = "guard/copied-tool-args"

	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			mutatingProvider{methodName: method, paramKey: "flag"},
			expectingProvider{methodName: method, paramKey: "flag", expectedValue: "original"},
		},
	})

	arguments := map[string]interface{}{"flag": "original"}
	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "copied-tool-args",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name":      method,
			"arguments": arguments,
		},
	})
	if resp.Error != nil {
		t.Fatalf("expected tools/call success with copied arguments, got %v", resp.Error)
	}
	if got, _ := arguments["flag"].(string); got != "original" {
		t.Fatalf("expected caller arguments to remain unchanged, got %q", got)
	}
}

func TestServer_HandleRequestProvidesWritableParamsForNilRequestParams(t *testing.T) {
	const method = "guard/writable-params"
	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			writableParamsProvider{methodName: method},
		},
	})

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "writable-params-method",
		Method: method,
		Params: nil,
	})
	if resp.Error != nil {
		t.Fatalf("expected nil params to be normalized to writable map, got %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected successful provider result with writable params")
	}
}

func TestServer_HandleToolsCallProvidesWritableParamsWhenArgumentsMissing(t *testing.T) {
	const method = "guard/writable-tool-args"
	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			writableParamsProvider{methodName: method},
		},
	})

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "writable-params-tool-call",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name": method,
		},
	})
	if resp.Error != nil {
		t.Fatalf("expected missing arguments to be normalized to writable map, got %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected successful tools/call provider result with writable params")
	}
}

func TestServer_HandleToolsCallReturnsSerializationError(t *testing.T) {
	const method = "guard/non-serializable"

	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			nonSerializableResultProvider{methodName: method},
		},
	})

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "non-serializable-result",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name":      method,
			"arguments": map[string]interface{}{},
		},
	})
	if resp.Error == nil {
		t.Fatal("expected serialization error")
	}
	if resp.Error.Code != -32603 {
		t.Fatalf("expected internal serialization error code, got %d", resp.Error.Code)
	}
	if !strings.HasPrefix(resp.Error.Message, "Failed to serialize tool result:") {
		t.Fatalf("expected serialization error message prefix, got %q", resp.Error.Message)
	}
}

func TestServer_HandleToolsCallRejectsNonObjectArguments(t *testing.T) {
	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			guardTestProvider{},
		},
	})

	resp := server.HandleRequest(context.Background(), shared.MCPRequest{
		ID:     "non-object-arguments",
		Method: "tools/call",
		Params: map[string]interface{}{
			"name":      "guard/provider-tool",
			"arguments": "not-an-object",
		},
	})
	if resp.Error == nil {
		t.Fatal("expected invalid arguments error")
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("expected invalid params code for non-object arguments, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "Invalid params: arguments must be an object" {
		t.Fatalf("expected non-object arguments validation message, got %q", resp.Error.Message)
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

func TestServer_ListToolsDefensiveCopyHandlesTypedNestedContainers(t *testing.T) {
	server := NewServer(Options{})
	server.RegisterTool(shared.MCPTool{
		Name:        "guard/typed-containers",
		Description: "typed nested containers",
		Parameters: map[string]interface{}{
			"typedIfaceSlice": []map[string]interface{}{
				{"status": "active"},
			},
			"typedStringMapSlice": []map[string]string{
				{"role": "primary"},
			},
			"typedMapSlice": map[string][]string{
				"labels": []string{"go", "docker"},
			},
			"typedNestedMap": map[string]map[string]interface{}{
				"config": {"enabled": true},
			},
		},
	})

	getSchema := func() map[string]interface{} {
		tools := server.ListTools()
		for _, tool := range tools {
			if tool.Name == "guard/typed-containers" {
				return tool.Parameters
			}
		}
		return nil
	}

	first := getSchema()
	if first == nil {
		t.Fatal("expected typed container schema")
	}

	firstTypedIface := first["typedIfaceSlice"].([]map[string]interface{})
	firstTypedIface[0]["status"] = "inactive"
	firstTypedStringMapSlice := first["typedStringMapSlice"].([]map[string]string)
	firstTypedStringMapSlice[0]["role"] = "secondary"
	firstTypedMapSlice := first["typedMapSlice"].(map[string][]string)
	firstTypedMapSlice["labels"][0] = "rust"
	firstTypedNestedMap := first["typedNestedMap"].(map[string]map[string]interface{})
	firstTypedNestedMap["config"]["enabled"] = false

	second := getSchema()
	if second == nil {
		t.Fatal("expected typed container schema on second read")
	}
	secondTypedIface := second["typedIfaceSlice"].([]map[string]interface{})
	if secondTypedIface[0]["status"] != "active" {
		t.Fatalf("expected typedIfaceSlice clone isolation, got %v", secondTypedIface[0]["status"])
	}
	secondTypedStringMapSlice := second["typedStringMapSlice"].([]map[string]string)
	if secondTypedStringMapSlice[0]["role"] != "primary" {
		t.Fatalf("expected typedStringMapSlice clone isolation, got %v", secondTypedStringMapSlice[0]["role"])
	}
	secondTypedMapSlice := second["typedMapSlice"].(map[string][]string)
	if secondTypedMapSlice["labels"][0] != "go" {
		t.Fatalf("expected typedMapSlice clone isolation, got %v", secondTypedMapSlice["labels"])
	}
	secondTypedNestedMap := second["typedNestedMap"].(map[string]map[string]interface{})
	if secondTypedNestedMap["config"]["enabled"] != true {
		t.Fatalf("expected typedNestedMap clone isolation, got %v", secondTypedNestedMap["config"]["enabled"])
	}
}

func TestServer_ListToolsDefensiveCopyHandlesCyclicSchema(t *testing.T) {
	server := NewServer(Options{})
	cyclic := map[string]interface{}{
		"type": "object",
	}
	cyclic["self"] = cyclic

	server.RegisterTool(shared.MCPTool{
		Name:        "guard/cyclic-schema",
		Description: "cyclic schema",
		Parameters:  cyclic,
	})

	getSchema := func() map[string]interface{} {
		for _, tool := range server.ListTools() {
			if tool.Name == "guard/cyclic-schema" {
				return tool.Parameters
			}
		}
		return nil
	}

	first := getSchema()
	if first == nil {
		t.Fatal("expected cyclic schema")
	}
	firstSelf, ok := first["self"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected cyclic self map, got %T", first["self"])
	}
	if reflect.ValueOf(firstSelf).Pointer() != reflect.ValueOf(first).Pointer() {
		t.Fatalf("expected cloned schema self-reference cycle to be preserved")
	}

	firstSelf["type"] = "mutated"

	second := getSchema()
	if second == nil {
		t.Fatal("expected cyclic schema on second read")
	}
	if second["type"] != "object" {
		t.Fatalf("expected cyclic schema root value isolation, got %v", second["type"])
	}
	secondSelf, ok := second["self"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected second cyclic self map, got %T", second["self"])
	}
	if secondSelf["type"] != "object" {
		t.Fatalf("expected cyclic schema nested value isolation, got %v", secondSelf["type"])
	}
}

func TestServer_GetCapabilitiesDefensiveCopyHandlesCyclicExperimental(t *testing.T) {
	server := NewServer(Options{})
	if server == nil {
		t.Fatal("expected server")
	}

	server.mu.Lock()
	server.capabilities.Experimental = map[string]interface{}{
		"value": "ok",
	}
	server.capabilities.Experimental["self"] = server.capabilities.Experimental
	server.mu.Unlock()

	caps := server.GetCapabilities()
	if caps == nil || caps.Experimental == nil {
		t.Fatal("expected capabilities experimental map")
	}
	capsSelf, ok := caps.Experimental["self"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected cloned experimental self map, got %T", caps.Experimental["self"])
	}
	if reflect.ValueOf(capsSelf).Pointer() != reflect.ValueOf(caps.Experimental).Pointer() {
		t.Fatalf("expected experimental self-reference cycle to be preserved")
	}
	capsSelf["value"] = "mutated"

	refetched := server.GetCapabilities()
	if refetched == nil || refetched.Experimental == nil {
		t.Fatal("expected refetched capabilities experimental map")
	}
	if got := refetched.Experimental["value"]; got != "ok" {
		t.Fatalf("expected cyclic experimental isolation, got %v", got)
	}
	refetchedSelf, ok := refetched.Experimental["self"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected refetched experimental self map, got %T", refetched.Experimental["self"])
	}
	if got := refetchedSelf["value"]; got != "ok" {
		t.Fatalf("expected refetched cyclic nested value isolation, got %v", got)
	}
}
