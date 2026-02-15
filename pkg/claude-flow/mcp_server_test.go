package claudeflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/federation"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

type runStdioFailingWriter struct{}

func (runStdioFailingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failure")
}

func requireFederationSchemaNumericConstraint(
	t *testing.T,
	toolByName map[string]map[string]interface{},
	toolName string,
	propertyName string,
	constraintKey string,
	expected float64,
) {
	t.Helper()

	params, ok := toolByName[toolName]
	if !ok {
		t.Fatalf("expected tool %s to be present", toolName)
	}
	propertiesRaw, ok := params["properties"]
	if !ok {
		t.Fatalf("expected properties in %s schema", toolName)
	}
	properties, ok := propertiesRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map for %s, got %T", toolName, propertiesRaw)
	}
	propertyRaw, ok := properties[propertyName]
	if !ok {
		t.Fatalf("expected property %s in %s schema", propertyName, toolName)
	}
	property, ok := propertyRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected property map for %s.%s, got %T", toolName, propertyName, propertyRaw)
	}
	valueRaw, ok := property[constraintKey]
	if !ok {
		t.Fatalf("expected %s.%s to define %s", toolName, propertyName, constraintKey)
	}
	value, ok := valueRaw.(float64)
	if !ok {
		t.Fatalf("expected %s.%s %s to be float64, got %T", toolName, propertyName, constraintKey, valueRaw)
	}
	if value != expected {
		t.Fatalf("expected %s.%s %s %v, got %v", toolName, propertyName, constraintKey, expected, value)
	}
}

func TestNewMCPServer_RegistersFederationTools(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	hasFederationStatus := false
	hasFederationSpawn := false
	for _, tool := range tools {
		switch tool.Name {
		case "federation/status":
			hasFederationStatus = true
		case "federation/spawn-ephemeral":
			hasFederationSpawn = true
		}
	}

	if !hasFederationStatus {
		t.Fatal("expected federation/status tool to be registered")
	}
	if !hasFederationSpawn {
		t.Fatal("expected federation/spawn-ephemeral tool to be registered")
	}
}

func TestMCPServer_ListToolsReturnsSortedNames(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected non-empty tool list")
	}

	prev := ""
	for i, tool := range tools {
		if tool.Name == "" {
			t.Fatalf("expected non-empty tool name at index %d", i)
		}
		if prev != "" && prev > tool.Name {
			t.Fatalf("expected sorted tool names, got %q before %q", prev, tool.Name)
		}
		prev = tool.Name
	}
}

func TestMCPServer_ListToolsReturnsDefensiveCopies(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	extractSpawnSchema := func(tools []MCPTool) map[string]interface{} {
		for _, tool := range tools {
			if tool.Name == "federation/spawn-ephemeral" {
				return tool.Parameters
			}
		}
		return nil
	}

	firstSchema := extractSpawnSchema(server.ListTools())
	if firstSchema == nil {
		t.Fatal("expected federation/spawn-ephemeral schema")
	}

	firstProperties, ok := firstSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties map, got %T", firstSchema["properties"])
	}
	firstTTL, ok := firstProperties["ttl"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ttl map, got %T", firstProperties["ttl"])
	}
	firstTTL["minimum"] = float64(-99)
	firstSchema["externallyMutated"] = true

	secondSchema := extractSpawnSchema(server.ListTools())
	if secondSchema == nil {
		t.Fatal("expected federation/spawn-ephemeral schema on second read")
	}
	if _, ok := secondSchema["externallyMutated"]; ok {
		t.Fatalf("expected schema mutation to be isolated, got %v", secondSchema["externallyMutated"])
	}
	secondProperties, ok := secondSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected second properties map, got %T", secondSchema["properties"])
	}
	secondTTL, ok := secondProperties["ttl"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected second ttl map, got %T", secondProperties["ttl"])
	}
	if secondTTL["minimum"] == float64(-99) {
		t.Fatalf("expected ttl minimum mutation to be isolated, got %v", secondTTL["minimum"])
	}
}

func TestMCPServer_StopShutsDownFederationHub(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}
	if server.federationHub == nil {
		t.Fatal("expected MCP server to retain federation hub reference")
	}
	fedHub := server.federationHub

	if err := server.Stop(); err != nil {
		t.Fatalf("expected first stop to succeed, got %v", err)
	}
	if err := server.Stop(); err != nil {
		t.Fatalf("expected second stop to remain idempotent, got %v", err)
	}
	if server.federationHub != nil {
		t.Fatal("expected MCP server to clear federation hub reference after stop")
	}

	err := fedHub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "stopped-server-swarm",
		Name:      "Stopped Server Swarm",
		MaxAgents: 1,
	})
	if err == nil {
		t.Fatal("expected federation hub to reject mutations after server stop")
	}
	if err.Error() != "federation hub is shut down" {
		t.Fatalf("expected shutdown lifecycle error, got %q", err.Error())
	}
}

func TestMCPServer_StopHandlesMalformedFederationHubGracefully(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	// Simulate malformed wiring: non-nil, unconfigured federation hub.
	server.federationHub = &federation.FederationHub{}

	err := server.Stop()
	if err == nil {
		t.Fatal("expected stop to surface malformed federation hub error")
	}
	if err.Error() != "federation hub is not configured" {
		t.Fatalf("expected malformed federation hub error, got %q", err.Error())
	}
	if server.federationHub != nil {
		t.Fatal("expected stop to clear malformed federation hub reference")
	}

	// Subsequent stop should remain safe and non-panicking.
	if err := server.Stop(); err != nil {
		t.Fatalf("expected second stop to remain safe, got %v", err)
	}
}

func TestMCPServer_StopZeroValueWithMalformedFederationHubKeepsPrimaryError(t *testing.T) {
	server := MCPServer{
		federationHub: &federation.FederationHub{},
	}

	err := server.Stop()
	if err == nil {
		t.Fatal("expected stop to fail for zero-value server")
	}
	if err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected primary stop error, got %q", err.Error())
	}
	if server.federationHub != nil {
		t.Fatal("expected stop to clear malformed federation hub reference")
	}
}

func TestMCPServer_StopWithoutInternalStillShutsDownFederationHub(t *testing.T) {
	fedHub := federation.NewFederationHubWithDefaults()
	if err := fedHub.Initialize(); err != nil {
		t.Fatalf("failed to initialize standalone federation hub: %v", err)
	}

	server := MCPServer{
		federationHub: fedHub,
	}

	err := server.Stop()
	if err == nil {
		t.Fatal("expected stop to fail without internal server")
	}
	if err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected primary stop error, got %q", err.Error())
	}
	if server.federationHub != nil {
		t.Fatal("expected stop to clear federation hub reference")
	}

	registerErr := fedHub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "post-stop-no-internal",
		Name:      "Post Stop No Internal",
		MaxAgents: 1,
	})
	if registerErr == nil {
		t.Fatal("expected federation hub to reject mutations after stop without internal server")
	}
	if registerErr.Error() != "federation hub is shut down" {
		t.Fatalf("expected shutdown lifecycle error, got %q", registerErr.Error())
	}

	if err := server.Stop(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected repeated stop to preserve initialization error, got %v", err)
	}
}

func TestMCPServer_StopWithMalformedInternalStillShutsDownFederationHub(t *testing.T) {
	fedHub := federation.NewFederationHubWithDefaults()
	if err := fedHub.Initialize(); err != nil {
		t.Fatalf("failed to initialize standalone federation hub: %v", err)
	}

	server := MCPServer{
		internal:      &mcp.Server{},
		federationHub: fedHub,
	}

	err := server.Stop()
	if err == nil {
		t.Fatal("expected stop to fail with malformed internal server")
	}
	if err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected primary stop error, got %q", err.Error())
	}
	if server.federationHub != nil {
		t.Fatal("expected stop to clear federation hub reference")
	}

	registerErr := fedHub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "post-stop-malformed-internal",
		Name:      "Post Stop Malformed Internal",
		MaxAgents: 1,
	})
	if registerErr == nil {
		t.Fatal("expected federation hub to reject mutations after stop with malformed internal server")
	}
	if registerErr.Error() != "federation hub is shut down" {
		t.Fatalf("expected shutdown lifecycle error, got %q", registerErr.Error())
	}
}

func TestMCPServer_ZeroValueMethodsFailGracefully(t *testing.T) {
	var server MCPServer

	if err := server.Start(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected start initialization error, got %v", err)
	}
	if err := server.Stop(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected stop initialization error, got %v", err)
	}

	if tools := server.ListTools(); len(tools) != 0 {
		t.Fatalf("expected zero-value server to expose no tools, got %d", len(tools))
	}
	status := server.GetStatus()
	if running, ok := status["running"].(bool); !ok || running {
		t.Fatalf("expected zero-value status running=false, got %v", status["running"])
	}
	if status["error"] != "mcp server is not initialized" {
		t.Fatalf("expected zero-value status error, got %v", status["error"])
	}

	if err := server.RunStdio(context.Background(), bytes.NewBuffer(nil), bytes.NewBuffer(nil)); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected stdio initialization error, got %v", err)
	}
	if err := server.RunStdio(nil, bytes.NewBuffer(nil), bytes.NewBuffer(nil)); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected stdio initialization error precedence for nil context, got %v", err)
	}
	if err := server.RunStdio(context.Background(), nil, bytes.NewBuffer(nil)); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected stdio initialization error precedence for nil reader, got %v", err)
	}
	if err := server.RunStdio(context.Background(), bytes.NewBuffer(nil), nil); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected stdio initialization error precedence for nil writer, got %v", err)
	}
}

func TestMCPServer_GetStatusReturnsDefensiveCopy(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

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

func TestMCPServer_NilReceiverMethodsFailGracefully(t *testing.T) {
	var server *MCPServer

	if err := server.Start(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected nil-receiver start error, got %v", err)
	}
	if err := server.Stop(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected nil-receiver stop error, got %v", err)
	}

	if tools := server.ListTools(); len(tools) != 0 {
		t.Fatalf("expected nil-receiver to expose no tools, got %d", len(tools))
	}
	status := server.GetStatus()
	if running, ok := status["running"].(bool); !ok || running {
		t.Fatalf("expected nil-receiver status running=false, got %v", status["running"])
	}
	if status["error"] != "mcp server is not initialized" {
		t.Fatalf("expected nil-receiver status error, got %v", status["error"])
	}

	if err := server.RunStdio(context.Background(), bytes.NewBuffer(nil), bytes.NewBuffer(nil)); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected nil-receiver stdio error, got %v", err)
	}
	if err := server.RunStdio(nil, bytes.NewBuffer(nil), bytes.NewBuffer(nil)); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected nil-receiver stdio error precedence for nil context, got %v", err)
	}
	if err := server.RunStdio(context.Background(), nil, bytes.NewBuffer(nil)); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected nil-receiver stdio error precedence for nil reader, got %v", err)
	}
	if err := server.RunStdio(context.Background(), bytes.NewBuffer(nil), nil); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected nil-receiver stdio error precedence for nil writer, got %v", err)
	}
}

func TestMCPServer_MalformedInternalServerFailsGracefully(t *testing.T) {
	server := MCPServer{
		internal: &mcp.Server{},
	}

	if err := server.Start(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected malformed-internal start error, got %v", err)
	}
	if err := server.Stop(); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected malformed-internal stop error, got %v", err)
	}

	if tools := server.ListTools(); len(tools) != 0 {
		t.Fatalf("expected malformed-internal to expose no tools, got %d", len(tools))
	}
	status := server.GetStatus()
	if running, ok := status["running"].(bool); !ok || running {
		t.Fatalf("expected malformed-internal status running=false, got %v", status["running"])
	}
	if status["error"] != "mcp server is not initialized" {
		t.Fatalf("expected malformed-internal status error, got %v", status["error"])
	}

	if err := server.RunStdio(context.Background(), bytes.NewBuffer(nil), bytes.NewBuffer(nil)); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected malformed-internal stdio init error, got %v", err)
	}
	if err := server.RunStdio(nil, nil, nil); err == nil || err.Error() != "mcp server is not initialized" {
		t.Fatalf("expected malformed-internal stdio init error precedence, got %v", err)
	}
}

func TestMCPServer_RunStdioRejectsNilContext(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	err := server.RunStdio(nil, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	if err == nil {
		t.Fatal("expected RunStdio to reject nil context")
	}
	if err.Error() != "context is required" {
		t.Fatalf("expected context-required error, got %q", err.Error())
	}

	if err := server.RunStdio(nil, nil, bytes.NewBuffer(nil)); err == nil || err.Error() != "context is required" {
		t.Fatalf("expected context-required precedence with nil reader, got %v", err)
	}
	if err := server.RunStdio(nil, bytes.NewBuffer(nil), nil); err == nil || err.Error() != "context is required" {
		t.Fatalf("expected context-required precedence with nil writer, got %v", err)
	}
}

func TestMCPServer_RunStdioRejectsNilReaderOrWriter(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	if err := server.RunStdio(context.Background(), nil, bytes.NewBuffer(nil)); err == nil || err.Error() != "reader is required" {
		t.Fatalf("expected reader-required error, got %v", err)
	}
	if err := server.RunStdio(context.Background(), bytes.NewBuffer(nil), nil); err == nil || err.Error() != "writer is required" {
		t.Fatalf("expected writer-required error, got %v", err)
	}
	if err := server.RunStdio(context.Background(), nil, nil); err == nil || err.Error() != "reader is required" {
		t.Fatalf("expected reader-required precedence when both streams are nil, got %v", err)
	}
}

func TestMCPServer_RunStdioReturnsDecodeError(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	err := server.RunStdio(context.Background(), bytes.NewBufferString("{invalid-json"), bytes.NewBuffer(nil))
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "failed to decode mcp request:") {
		t.Fatalf("expected decode error prefix, got %v", err)
	}
}

func TestMCPServer_RunStdioReturnsEncodeError(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	err := server.RunStdio(
		context.Background(),
		bytes.NewBufferString(`{"id":"1","method":"initialize","params":{}}`),
		runStdioFailingWriter{},
	)
	if err == nil {
		t.Fatal("expected encode error")
	}
	if !strings.Contains(err.Error(), "failed to encode mcp response:") {
		t.Fatalf("expected encode error prefix, got %v", err)
	}
}

func TestMCPServer_RunStdioContextCancellationUnblocksPipeReader(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	reader, writer := io.Pipe()
	defer writer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- server.RunStdio(ctx, reader, bytes.NewBuffer(nil))
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected RunStdio to return promptly after context cancellation")
	}
}

func TestMCPServer_RunStdioContinuesAfterProtocolError(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	input := bytes.NewBufferString(
		`{"id":"bad-method","method":"   ","params":{}}` + "\n" +
			`{"id":"ok-init","method":"initialize","params":{}}` + "\n",
	)
	output := bytes.NewBuffer(nil)

	if err := server.RunStdio(context.Background(), input, output); err != nil {
		t.Fatalf("expected RunStdio to complete request stream, got %v", err)
	}

	decoder := json.NewDecoder(output)

	var first shared.MCPResponse
	if err := decoder.Decode(&first); err != nil {
		t.Fatalf("expected first response, got %v", err)
	}
	if first.ID != "bad-method" {
		t.Fatalf("expected first response id bad-method, got %q", first.ID)
	}
	if first.Error == nil || first.Error.Code != -32600 || first.Error.Message != "Method is required" {
		t.Fatalf("expected blank-method protocol error response, got %+v", first.Error)
	}

	var second shared.MCPResponse
	if err := decoder.Decode(&second); err != nil {
		t.Fatalf("expected second response, got %v", err)
	}
	if second.ID != "ok-init" || second.Error != nil {
		t.Fatalf("expected successful initialize response after error, got %+v", second)
	}
}

func TestNewMCPServer_InitializesFederationHubLifecycle(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server")
	}
	if server.federationHub == nil {
		t.Fatal("expected federation hub to be attached")
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	err := server.federationHub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "mcp-server-init-swarm",
		Name:      "MCP Server Init Swarm",
		MaxAgents: 1,
	})
	if err != nil {
		t.Fatalf("expected federation hub to be initialized in NewMCPServer, got %v", err)
	}
}

func TestNewMCPServer_RegistersAllFederationTools(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expected := map[string]bool{
		"federation/status":              true,
		"federation/spawn-ephemeral":     true,
		"federation/terminate-ephemeral": true,
		"federation/list-ephemeral":      true,
		"federation/register-swarm":      true,
		"federation/broadcast":           true,
		"federation/propose":             true,
		"federation/vote":                true,
	}

	counts := make(map[string]int, len(expected))
	for _, tool := range tools {
		if expected[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expected {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_FederationToolSchemas_ExposeValidatedStatusAndIntegers(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	toolByName := make(map[string]map[string]interface{}, len(tools))
	for _, tool := range tools {
		toolByName[tool.Name] = tool.Parameters
	}

	requirePropertyType := func(toolName, propertyName, expectedType string) {
		t.Helper()

		params, ok := toolByName[toolName]
		if !ok {
			t.Fatalf("expected tool %s to be present", toolName)
		}
		propertiesRaw, ok := params["properties"]
		if !ok {
			t.Fatalf("expected properties in %s schema", toolName)
		}
		properties, ok := propertiesRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected properties map for %s, got %T", toolName, propertiesRaw)
		}
		propertyRaw, ok := properties[propertyName]
		if !ok {
			t.Fatalf("expected property %s in %s schema", propertyName, toolName)
		}
		property, ok := propertyRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected property map for %s.%s, got %T", toolName, propertyName, propertyRaw)
		}
		if property["type"] != expectedType {
			t.Fatalf("expected %s.%s type %q, got %v", toolName, propertyName, expectedType, property["type"])
		}
	}

	requirePropertyType("federation/spawn-ephemeral", "ttl", "integer")
	requirePropertyType("federation/register-swarm", "maxAgents", "integer")
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/spawn-ephemeral", "ttl", "minimum", 1)
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/spawn-ephemeral", "ttl", "maximum", float64(math.MaxInt64))
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/register-swarm", "maxAgents", "minimum", 1)
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/register-swarm", "maxAgents", "maximum", float64(math.MaxInt))

	params, ok := toolByName["federation/list-ephemeral"]
	if !ok {
		t.Fatal("expected federation/list-ephemeral tool to be present")
	}
	propertiesRaw, ok := params["properties"]
	if !ok {
		t.Fatal("expected properties in federation/list-ephemeral schema")
	}
	properties, ok := propertiesRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected list-ephemeral properties map, got %T", propertiesRaw)
	}
	statusRaw, ok := properties["status"]
	if !ok {
		t.Fatal("expected status property in federation/list-ephemeral schema")
	}
	statusProperty, ok := statusRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected status property map, got %T", statusRaw)
	}
	enumRaw, ok := statusProperty["enum"]
	if !ok {
		t.Fatal("expected enum in list-ephemeral status property")
	}

	enumValues := map[string]bool{}
	switch values := enumRaw.(type) {
	case []string:
		for _, value := range values {
			enumValues[value] = true
		}
	case []interface{}:
		for _, value := range values {
			s, ok := value.(string)
			if !ok {
				t.Fatalf("expected enum entries to be strings, got %T", value)
			}
			enumValues[s] = true
		}
	default:
		t.Fatalf("expected status enum to be []string or []interface{}, got %T", enumRaw)
	}

	expectedEnum := []string{"spawning", "active", "completing", "terminated"}
	if len(enumValues) != len(expectedEnum) {
		t.Fatalf("expected %d unique status enum values, got %d (%v)", len(expectedEnum), len(enumValues), enumValues)
	}
	for _, expected := range expectedEnum {
		if !enumValues[expected] {
			t.Fatalf("expected status enum to include %q", expected)
		}
	}
}

func TestNewMCPServer_WithCoordinatorAndMemory_FederationToolSchemas_ExposeValidatedStatusAndIntegers(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
		Memory:      backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	toolByName := make(map[string]map[string]interface{}, len(tools))
	for _, tool := range tools {
		toolByName[tool.Name] = tool.Parameters
	}

	requirePropertyType := func(toolName, propertyName, expectedType string) {
		t.Helper()

		params, ok := toolByName[toolName]
		if !ok {
			t.Fatalf("expected tool %s to be present", toolName)
		}
		propertiesRaw, ok := params["properties"]
		if !ok {
			t.Fatalf("expected properties in %s schema", toolName)
		}
		properties, ok := propertiesRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected properties map for %s, got %T", toolName, propertiesRaw)
		}
		propertyRaw, ok := properties[propertyName]
		if !ok {
			t.Fatalf("expected property %s in %s schema", propertyName, toolName)
		}
		property, ok := propertyRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected property map for %s.%s, got %T", toolName, propertyName, propertyRaw)
		}
		if property["type"] != expectedType {
			t.Fatalf("expected %s.%s type %q, got %v", toolName, propertyName, expectedType, property["type"])
		}
	}

	requirePropertyType("federation/spawn-ephemeral", "ttl", "integer")
	requirePropertyType("federation/register-swarm", "maxAgents", "integer")
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/spawn-ephemeral", "ttl", "minimum", 1)
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/spawn-ephemeral", "ttl", "maximum", float64(math.MaxInt64))
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/register-swarm", "maxAgents", "minimum", 1)
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/register-swarm", "maxAgents", "maximum", float64(math.MaxInt))

	params, ok := toolByName["federation/list-ephemeral"]
	if !ok {
		t.Fatal("expected federation/list-ephemeral tool to be present")
	}
	propertiesRaw, ok := params["properties"]
	if !ok {
		t.Fatal("expected properties in federation/list-ephemeral schema")
	}
	properties, ok := propertiesRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected list-ephemeral properties map, got %T", propertiesRaw)
	}
	statusRaw, ok := properties["status"]
	if !ok {
		t.Fatal("expected status property in federation/list-ephemeral schema")
	}
	statusProperty, ok := statusRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected status property map, got %T", statusRaw)
	}
	enumRaw, ok := statusProperty["enum"]
	if !ok {
		t.Fatal("expected enum in list-ephemeral status property")
	}

	enumValues := map[string]bool{}
	switch values := enumRaw.(type) {
	case []string:
		for _, value := range values {
			enumValues[value] = true
		}
	case []interface{}:
		for _, value := range values {
			s, ok := value.(string)
			if !ok {
				t.Fatalf("expected enum entries to be strings, got %T", value)
			}
			enumValues[s] = true
		}
	default:
		t.Fatalf("expected status enum to be []string or []interface{}, got %T", enumRaw)
	}

	expectedEnum := []string{"spawning", "active", "completing", "terminated"}
	if len(enumValues) != len(expectedEnum) {
		t.Fatalf("expected %d unique status enum values, got %d (%v)", len(expectedEnum), len(enumValues), enumValues)
	}
	for _, expected := range expectedEnum {
		if !enumValues[expected] {
			t.Fatalf("expected status enum to include %q", expected)
		}
	}
}

func TestNewMCPServer_WithMemory_FederationToolSchemas_ExposeValidatedStatusAndIntegers(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Memory: backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	toolByName := make(map[string]map[string]interface{}, len(tools))
	for _, tool := range tools {
		toolByName[tool.Name] = tool.Parameters
	}

	requirePropertyType := func(toolName, propertyName, expectedType string) {
		t.Helper()

		params, ok := toolByName[toolName]
		if !ok {
			t.Fatalf("expected tool %s to be present", toolName)
		}
		propertiesRaw, ok := params["properties"]
		if !ok {
			t.Fatalf("expected properties in %s schema", toolName)
		}
		properties, ok := propertiesRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected properties map for %s, got %T", toolName, propertiesRaw)
		}
		propertyRaw, ok := properties[propertyName]
		if !ok {
			t.Fatalf("expected property %s in %s schema", propertyName, toolName)
		}
		property, ok := propertyRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected property map for %s.%s, got %T", toolName, propertyName, propertyRaw)
		}
		if property["type"] != expectedType {
			t.Fatalf("expected %s.%s type %q, got %v", toolName, propertyName, expectedType, property["type"])
		}
	}

	requirePropertyType("federation/spawn-ephemeral", "ttl", "integer")
	requirePropertyType("federation/register-swarm", "maxAgents", "integer")
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/spawn-ephemeral", "ttl", "minimum", 1)
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/spawn-ephemeral", "ttl", "maximum", float64(math.MaxInt64))
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/register-swarm", "maxAgents", "minimum", 1)
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/register-swarm", "maxAgents", "maximum", float64(math.MaxInt))

	params, ok := toolByName["federation/list-ephemeral"]
	if !ok {
		t.Fatal("expected federation/list-ephemeral tool to be present")
	}
	propertiesRaw, ok := params["properties"]
	if !ok {
		t.Fatal("expected properties in federation/list-ephemeral schema")
	}
	properties, ok := propertiesRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected list-ephemeral properties map, got %T", propertiesRaw)
	}
	statusRaw, ok := properties["status"]
	if !ok {
		t.Fatal("expected status property in federation/list-ephemeral schema")
	}
	statusProperty, ok := statusRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected status property map, got %T", statusRaw)
	}
	enumRaw, ok := statusProperty["enum"]
	if !ok {
		t.Fatal("expected enum in list-ephemeral status property")
	}

	enumValues := map[string]bool{}
	switch values := enumRaw.(type) {
	case []string:
		for _, value := range values {
			enumValues[value] = true
		}
	case []interface{}:
		for _, value := range values {
			s, ok := value.(string)
			if !ok {
				t.Fatalf("expected enum entries to be strings, got %T", value)
			}
			enumValues[s] = true
		}
	default:
		t.Fatalf("expected status enum to be []string or []interface{}, got %T", enumRaw)
	}

	expectedEnum := []string{"spawning", "active", "completing", "terminated"}
	if len(enumValues) != len(expectedEnum) {
		t.Fatalf("expected %d unique status enum values, got %d (%v)", len(expectedEnum), len(enumValues), enumValues)
	}
	for _, expected := range expectedEnum {
		if !enumValues[expected] {
			t.Fatalf("expected status enum to include %q", expected)
		}
	}
}

func TestNewMCPServer_WithCoordinator_FederationToolSchemas_ExposeValidatedStatusAndIntegers(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	toolByName := make(map[string]map[string]interface{}, len(tools))
	for _, tool := range tools {
		toolByName[tool.Name] = tool.Parameters
	}

	requirePropertyType := func(toolName, propertyName, expectedType string) {
		t.Helper()

		params, ok := toolByName[toolName]
		if !ok {
			t.Fatalf("expected tool %s to be present", toolName)
		}
		propertiesRaw, ok := params["properties"]
		if !ok {
			t.Fatalf("expected properties in %s schema", toolName)
		}
		properties, ok := propertiesRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected properties map for %s, got %T", toolName, propertiesRaw)
		}
		propertyRaw, ok := properties[propertyName]
		if !ok {
			t.Fatalf("expected property %s in %s schema", propertyName, toolName)
		}
		property, ok := propertyRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected property map for %s.%s, got %T", toolName, propertyName, propertyRaw)
		}
		if property["type"] != expectedType {
			t.Fatalf("expected %s.%s type %q, got %v", toolName, propertyName, expectedType, property["type"])
		}
	}

	requirePropertyType("federation/spawn-ephemeral", "ttl", "integer")
	requirePropertyType("federation/register-swarm", "maxAgents", "integer")
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/spawn-ephemeral", "ttl", "minimum", 1)
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/spawn-ephemeral", "ttl", "maximum", float64(math.MaxInt64))
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/register-swarm", "maxAgents", "minimum", 1)
	requireFederationSchemaNumericConstraint(t, toolByName, "federation/register-swarm", "maxAgents", "maximum", float64(math.MaxInt))

	params, ok := toolByName["federation/list-ephemeral"]
	if !ok {
		t.Fatal("expected federation/list-ephemeral tool to be present")
	}
	propertiesRaw, ok := params["properties"]
	if !ok {
		t.Fatal("expected properties in federation/list-ephemeral schema")
	}
	properties, ok := propertiesRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected list-ephemeral properties map, got %T", propertiesRaw)
	}
	statusRaw, ok := properties["status"]
	if !ok {
		t.Fatal("expected status property in federation/list-ephemeral schema")
	}
	statusProperty, ok := statusRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected status property map, got %T", statusRaw)
	}
	enumRaw, ok := statusProperty["enum"]
	if !ok {
		t.Fatal("expected enum in list-ephemeral status property")
	}

	enumValues := map[string]bool{}
	switch values := enumRaw.(type) {
	case []string:
		for _, value := range values {
			enumValues[value] = true
		}
	case []interface{}:
		for _, value := range values {
			s, ok := value.(string)
			if !ok {
				t.Fatalf("expected enum entries to be strings, got %T", value)
			}
			enumValues[s] = true
		}
	default:
		t.Fatalf("expected status enum to be []string or []interface{}, got %T", enumRaw)
	}

	expectedEnum := []string{"spawning", "active", "completing", "terminated"}
	if len(enumValues) != len(expectedEnum) {
		t.Fatalf("expected %d unique status enum values, got %d (%v)", len(expectedEnum), len(enumValues), enumValues)
	}
	for _, expected := range expectedEnum {
		if !enumValues[expected] {
			t.Fatalf("expected status enum to include %q", expected)
		}
	}
}

func TestNewMCPServer_FederationToolSchemas_AreIdenticalAcrossConfigs(t *testing.T) {
	baseServer := NewMCPServer(MCPServerConfig{})
	if baseServer == nil {
		t.Fatal("expected base MCP server to be created")
	}
	baseSchemas := extractFederationToolSchemas(t, baseServer.ListTools())
	if len(baseSchemas) == 0 {
		t.Fatal("expected base MCP server to include federation schemas")
	}

	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator for coordinator-only config: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})
	coordServer := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
	})
	if coordServer == nil {
		t.Fatal("expected coordinator MCP server to be created")
	}
	assertFederationSchemasEqual(t, baseSchemas, extractFederationToolSchemas(t, coordServer.ListTools()), "coordinator-only")

	memoryBackend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend for memory-only config: %v", err)
	}
	memoryServer := NewMCPServer(MCPServerConfig{
		Memory: memoryBackend,
	})
	if memoryServer == nil {
		t.Fatal("expected memory MCP server to be created")
	}
	assertFederationSchemasEqual(t, baseSchemas, extractFederationToolSchemas(t, memoryServer.ListTools()), "memory-only")

	coordMemory, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator for coordinator+memory config: %v", err)
	}
	t.Cleanup(func() {
		_ = coordMemory.Shutdown()
	})
	coordMemoryBackend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend for coordinator+memory config: %v", err)
	}
	coordMemoryServer := NewMCPServer(MCPServerConfig{
		Coordinator: coordMemory,
		Memory:      coordMemoryBackend,
	})
	if coordMemoryServer == nil {
		t.Fatal("expected coordinator+memory MCP server to be created")
	}
	assertFederationSchemasEqual(t, baseSchemas, extractFederationToolSchemas(t, coordMemoryServer.ListTools()), "coordinator+memory")
}

func extractFederationToolSchemas(t *testing.T, tools []MCPTool) map[string]map[string]interface{} {
	t.Helper()

	expectedFederationToolNames := map[string]bool{
		"federation/status":              true,
		"federation/spawn-ephemeral":     true,
		"federation/terminate-ephemeral": true,
		"federation/list-ephemeral":      true,
		"federation/register-swarm":      true,
		"federation/broadcast":           true,
		"federation/propose":             true,
		"federation/vote":                true,
	}

	schemas := make(map[string]map[string]interface{}, len(expectedFederationToolNames))
	for _, tool := range tools {
		if expectedFederationToolNames[tool.Name] {
			schemas[tool.Name] = tool.Parameters
		}
	}

	if len(schemas) != len(expectedFederationToolNames) {
		t.Fatalf("expected %d federation tool schemas, got %d", len(expectedFederationToolNames), len(schemas))
	}
	return schemas
}

func assertFederationSchemasEqual(t *testing.T, baseline map[string]map[string]interface{}, current map[string]map[string]interface{}, configLabel string) {
	t.Helper()
	for name, baselineSchema := range baseline {
		schema, ok := current[name]
		if !ok {
			t.Fatalf("expected %s schema in %s config", name, configLabel)
		}
		if !reflect.DeepEqual(baselineSchema, schema) {
			t.Fatalf("expected %s schema to match baseline in %s config", name, configLabel)
		}
	}
}

func TestNewMCPServer_RegistersBuiltInHooksAndUniqueTools(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	hasHooksList := false
	toolCounts := make(map[string]int)
	for _, tool := range tools {
		toolCounts[tool.Name]++
		if tool.Name == "hooks/list" {
			hasHooksList = true
		}
	}

	if !hasHooksList {
		t.Fatal("expected hooks/list tool to be registered")
	}

	if toolCounts["federation/status"] != 1 {
		t.Fatalf("expected exactly one federation/status tool, got %d", toolCounts["federation/status"])
	}
	if toolCounts["federation/spawn-ephemeral"] != 1 {
		t.Fatalf("expected exactly one federation/spawn-ephemeral tool, got %d", toolCounts["federation/spawn-ephemeral"])
	}
}

func TestNewMCPServer_RegistersAllHooksTools(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedHooks := map[string]bool{
		"hooks/pre-edit":     true,
		"hooks/post-edit":    true,
		"hooks/pre-command":  true,
		"hooks/post-command": true,
		"hooks/route":        true,
		"hooks/explain":      true,
		"hooks/pretrain":     true,
		"hooks/metrics":      true,
		"hooks/list":         true,
	}

	counts := make(map[string]int, len(expectedHooks))
	for _, tool := range tools {
		if expectedHooks[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedHooks {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_NoDuplicateToolNames(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	seen := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("duplicate tool name found: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
}

func TestNewMCPServer_WithCoordinator_RegistersCoordinatorTools(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	seen := make(map[string]bool, len(tools))
	hasAgentSpawn := false
	hasConfigGet := false
	hasOrchestratePlan := false
	hasMemoryStore := false
	hasMemoryRetrieve := false
	hasFederationStatus := false
	hasFederationSpawn := false
	hasHooksList := false

	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("duplicate tool name found: %s", tool.Name)
		}
		seen[tool.Name] = true

		switch tool.Name {
		case "agent_spawn":
			hasAgentSpawn = true
		case "config_get":
			hasConfigGet = true
		case "orchestrate_plan":
			hasOrchestratePlan = true
		case "memory_store":
			hasMemoryStore = true
		case "memory_retrieve":
			hasMemoryRetrieve = true
		case "federation/status":
			hasFederationStatus = true
		case "federation/spawn-ephemeral":
			hasFederationSpawn = true
		case "hooks/list":
			hasHooksList = true
		}
	}

	if hasMemoryStore || hasMemoryRetrieve {
		t.Fatalf("expected memory tools to be absent when memory backend is not configured; got store=%v retrieve=%v", hasMemoryStore, hasMemoryRetrieve)
	}

	if !hasAgentSpawn || !hasConfigGet || !hasOrchestratePlan || !hasFederationStatus || !hasFederationSpawn || !hasHooksList {
		t.Fatalf(
			"expected core tool families from coordinator/federation/hooks to be present; got agent=%v config=%v orchestrate=%v federationStatus=%v federationSpawn=%v hooks=%v",
			hasAgentSpawn, hasConfigGet, hasOrchestratePlan, hasFederationStatus, hasFederationSpawn, hasHooksList,
		)
	}
}

func TestNewMCPServer_WithCoordinator_RegistersAllFederationTools(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedFederation := map[string]bool{
		"federation/status":              true,
		"federation/spawn-ephemeral":     true,
		"federation/terminate-ephemeral": true,
		"federation/list-ephemeral":      true,
		"federation/register-swarm":      true,
		"federation/broadcast":           true,
		"federation/propose":             true,
		"federation/vote":                true,
	}

	counts := make(map[string]int, len(expectedFederation))
	for _, tool := range tools {
		if expectedFederation[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedFederation {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in coordinator-only config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithCoordinator_RegistersAllHooksTools(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedHooks := map[string]bool{
		"hooks/pre-edit":     true,
		"hooks/post-edit":    true,
		"hooks/pre-command":  true,
		"hooks/post-command": true,
		"hooks/route":        true,
		"hooks/explain":      true,
		"hooks/pretrain":     true,
		"hooks/metrics":      true,
		"hooks/list":         true,
	}

	counts := make(map[string]int, len(expectedHooks))
	for _, tool := range tools {
		if expectedHooks[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedHooks {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in coordinator-only config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithCoordinator_RegistersAllCoordinatorTools(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedCoordinatorTools := map[string]bool{
		"agent_spawn":         true,
		"agent_list":          true,
		"agent_terminate":     true,
		"agent_metrics":       true,
		"agent_types_list":    true,
		"agent_pool_list":     true,
		"agent_pool_scale":    true,
		"agent_health":        true,
		"config_get":          true,
		"config_set":          true,
		"config_list":         true,
		"config_validate":     true,
		"swarm_state":         true,
		"swarm_reconfigure":   true,
		"orchestrate_plan":    true,
		"orchestrate_execute": true,
		"orchestrate_status":  true,
	}

	counts := make(map[string]int, len(expectedCoordinatorTools))
	for _, tool := range tools {
		if expectedCoordinatorTools[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedCoordinatorTools {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in coordinator-only config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithoutCoordinator_DoesNotRegisterCoordinatorTools(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	hasAgentSpawn := false
	hasConfigGet := false
	hasOrchestratePlan := false
	hasMemoryStore := false
	hasMemoryRetrieve := false
	hasFederationStatus := false
	hasFederationSpawn := false
	hasHooksList := false
	seen := make(map[string]bool, len(tools))

	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("duplicate tool name found: %s", tool.Name)
		}
		seen[tool.Name] = true

		switch tool.Name {
		case "agent_spawn":
			hasAgentSpawn = true
		case "config_get":
			hasConfigGet = true
		case "orchestrate_plan":
			hasOrchestratePlan = true
		case "memory_store":
			hasMemoryStore = true
		case "memory_retrieve":
			hasMemoryRetrieve = true
		case "federation/status":
			hasFederationStatus = true
		case "federation/spawn-ephemeral":
			hasFederationSpawn = true
		case "hooks/list":
			hasHooksList = true
		}
	}

	if hasAgentSpawn || hasConfigGet || hasOrchestratePlan || hasMemoryStore || hasMemoryRetrieve {
		t.Fatalf(
			"expected coordinator/memory tools to be absent; got agent=%v config=%v orchestrate=%v memoryStore=%v memoryRetrieve=%v",
			hasAgentSpawn, hasConfigGet, hasOrchestratePlan, hasMemoryStore, hasMemoryRetrieve,
		)
	}

	if !hasFederationStatus || !hasFederationSpawn || !hasHooksList {
		t.Fatalf(
			"expected federation/hooks tools even without coordinator; got federationStatus=%v federationSpawn=%v hooks=%v",
			hasFederationStatus, hasFederationSpawn, hasHooksList,
		)
	}
}

func TestNewMCPServer_WithoutCoordinator_RegistersAllFederationTools(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedFederation := map[string]bool{
		"federation/status":              true,
		"federation/spawn-ephemeral":     true,
		"federation/terminate-ephemeral": true,
		"federation/list-ephemeral":      true,
		"federation/register-swarm":      true,
		"federation/broadcast":           true,
		"federation/propose":             true,
		"federation/vote":                true,
	}

	counts := make(map[string]int, len(expectedFederation))
	for _, tool := range tools {
		if expectedFederation[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedFederation {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in no-coordinator config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithoutCoordinator_RegistersNoCoordinatorTools(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	coordinatorTools := map[string]bool{
		"agent_spawn":         true,
		"agent_list":          true,
		"agent_terminate":     true,
		"agent_metrics":       true,
		"agent_types_list":    true,
		"agent_pool_list":     true,
		"agent_pool_scale":    true,
		"agent_health":        true,
		"config_get":          true,
		"config_set":          true,
		"config_list":         true,
		"config_validate":     true,
		"swarm_state":         true,
		"swarm_reconfigure":   true,
		"orchestrate_plan":    true,
		"orchestrate_execute": true,
		"orchestrate_status":  true,
	}

	for _, tool := range tools {
		if coordinatorTools[tool.Name] {
			t.Fatalf("expected coordinator tool %s to be absent without coordinator", tool.Name)
		}
	}
}

func TestNewMCPServer_WithoutMemory_RegistersNoMemoryTools(t *testing.T) {
	server := NewMCPServer(MCPServerConfig{})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	memoryTools := map[string]bool{
		"memory_store":    true,
		"memory_retrieve": true,
		"memory_query":    true,
		"memory_search":   true,
		"memory_delete":   true,
	}

	for _, tool := range tools {
		if memoryTools[tool.Name] {
			t.Fatalf("expected memory tool %s to be absent without memory backend", tool.Name)
		}
	}
}

func TestNewMCPServer_WithCoordinatorAndWithoutMemory_RegistersNoMemoryTools(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	memoryTools := map[string]bool{
		"memory_store":    true,
		"memory_retrieve": true,
		"memory_query":    true,
		"memory_search":   true,
		"memory_delete":   true,
	}

	for _, tool := range tools {
		if memoryTools[tool.Name] {
			t.Fatalf("expected memory tool %s to be absent in coordinator-only config", tool.Name)
		}
	}
}

func TestNewMCPServer_WithMemory_RegistersMemoryTools(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Memory: backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	hasMemoryStore := false
	hasMemoryRetrieve := false
	hasAgentSpawn := false
	hasConfigGet := false
	hasOrchestratePlan := false
	hasFederationStatus := false
	hasFederationSpawn := false
	hasHooksList := false
	seen := make(map[string]bool, len(tools))

	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("duplicate tool name found: %s", tool.Name)
		}
		seen[tool.Name] = true

		switch tool.Name {
		case "agent_spawn":
			hasAgentSpawn = true
		case "config_get":
			hasConfigGet = true
		case "orchestrate_plan":
			hasOrchestratePlan = true
		case "memory_store":
			hasMemoryStore = true
		case "memory_retrieve":
			hasMemoryRetrieve = true
		case "federation/status":
			hasFederationStatus = true
		case "federation/spawn-ephemeral":
			hasFederationSpawn = true
		case "hooks/list":
			hasHooksList = true
		}
	}

	if !hasMemoryStore || !hasMemoryRetrieve {
		t.Fatalf("expected memory tools to be registered; got store=%v retrieve=%v", hasMemoryStore, hasMemoryRetrieve)
	}
	if hasAgentSpawn || hasConfigGet || hasOrchestratePlan {
		t.Fatalf(
			"expected coordinator tools to be absent in memory-only config; got agent=%v config=%v orchestrate=%v",
			hasAgentSpawn, hasConfigGet, hasOrchestratePlan,
		)
	}
	if !hasFederationStatus || !hasFederationSpawn || !hasHooksList {
		t.Fatalf("expected federation/hooks tools to remain registered; got federationStatus=%v federationSpawn=%v hooks=%v", hasFederationStatus, hasFederationSpawn, hasHooksList)
	}
}

func TestNewMCPServer_WithMemory_RegistersAllFederationTools(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Memory: backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedFederation := map[string]bool{
		"federation/status":              true,
		"federation/spawn-ephemeral":     true,
		"federation/terminate-ephemeral": true,
		"federation/list-ephemeral":      true,
		"federation/register-swarm":      true,
		"federation/broadcast":           true,
		"federation/propose":             true,
		"federation/vote":                true,
	}

	counts := make(map[string]int, len(expectedFederation))
	for _, tool := range tools {
		if expectedFederation[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedFederation {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in memory-only config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithMemory_RegistersAllMemoryTools(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Memory: backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedMemoryTools := map[string]bool{
		"memory_store":    true,
		"memory_retrieve": true,
		"memory_query":    true,
		"memory_search":   true,
		"memory_delete":   true,
	}

	counts := make(map[string]int, len(expectedMemoryTools))
	for _, tool := range tools {
		if expectedMemoryTools[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedMemoryTools {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in memory-only config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithMemory_RegistersNoCoordinatorTools(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Memory: backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	coordinatorTools := map[string]bool{
		"agent_spawn":         true,
		"agent_list":          true,
		"agent_terminate":     true,
		"agent_metrics":       true,
		"agent_types_list":    true,
		"agent_pool_list":     true,
		"agent_pool_scale":    true,
		"agent_health":        true,
		"config_get":          true,
		"config_set":          true,
		"config_list":         true,
		"config_validate":     true,
		"swarm_state":         true,
		"swarm_reconfigure":   true,
		"orchestrate_plan":    true,
		"orchestrate_execute": true,
		"orchestrate_status":  true,
	}

	for _, tool := range tools {
		if coordinatorTools[tool.Name] {
			t.Fatalf("expected coordinator tool %s to be absent in memory-only config", tool.Name)
		}
	}
}

func TestNewMCPServer_WithMemory_RegistersAllHooksTools(t *testing.T) {
	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Memory: backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedHooks := map[string]bool{
		"hooks/pre-edit":     true,
		"hooks/post-edit":    true,
		"hooks/pre-command":  true,
		"hooks/post-command": true,
		"hooks/route":        true,
		"hooks/explain":      true,
		"hooks/pretrain":     true,
		"hooks/metrics":      true,
		"hooks/list":         true,
	}

	counts := make(map[string]int, len(expectedHooks))
	for _, tool := range tools {
		if expectedHooks[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedHooks {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in memory-only config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithCoordinatorAndMemory_RegistersAllToolFamilies(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
		Memory:      backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	seen := make(map[string]bool, len(tools))
	hasAgentSpawn := false
	hasConfigGet := false
	hasOrchestratePlan := false
	hasMemoryStore := false
	hasMemoryRetrieve := false
	hasFederationStatus := false
	hasFederationSpawn := false
	hasHooksList := false

	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("duplicate tool name found: %s", tool.Name)
		}
		seen[tool.Name] = true

		switch tool.Name {
		case "agent_spawn":
			hasAgentSpawn = true
		case "config_get":
			hasConfigGet = true
		case "orchestrate_plan":
			hasOrchestratePlan = true
		case "memory_store":
			hasMemoryStore = true
		case "memory_retrieve":
			hasMemoryRetrieve = true
		case "federation/status":
			hasFederationStatus = true
		case "federation/spawn-ephemeral":
			hasFederationSpawn = true
		case "hooks/list":
			hasHooksList = true
		}
	}

	if !hasAgentSpawn || !hasConfigGet || !hasOrchestratePlan || !hasMemoryStore || !hasMemoryRetrieve || !hasFederationStatus || !hasFederationSpawn || !hasHooksList {
		t.Fatalf(
			"expected coordinator+memory+federation+hooks tools; got agent=%v config=%v orchestrate=%v memoryStore=%v memoryRetrieve=%v federationStatus=%v federationSpawn=%v hooks=%v",
			hasAgentSpawn, hasConfigGet, hasOrchestratePlan, hasMemoryStore, hasMemoryRetrieve, hasFederationStatus, hasFederationSpawn, hasHooksList,
		)
	}
}

func TestNewMCPServer_WithCoordinatorAndMemory_RegistersAllFederationTools(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
		Memory:      backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedFederation := map[string]bool{
		"federation/status":              true,
		"federation/spawn-ephemeral":     true,
		"federation/terminate-ephemeral": true,
		"federation/list-ephemeral":      true,
		"federation/register-swarm":      true,
		"federation/broadcast":           true,
		"federation/propose":             true,
		"federation/vote":                true,
	}

	counts := make(map[string]int, len(expectedFederation))
	for _, tool := range tools {
		if expectedFederation[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedFederation {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in coordinator+memory config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithCoordinatorAndMemory_RegistersAllHooksTools(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
		Memory:      backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedHooks := map[string]bool{
		"hooks/pre-edit":     true,
		"hooks/post-edit":    true,
		"hooks/pre-command":  true,
		"hooks/post-command": true,
		"hooks/route":        true,
		"hooks/explain":      true,
		"hooks/pretrain":     true,
		"hooks/metrics":      true,
		"hooks/list":         true,
	}

	counts := make(map[string]int, len(expectedHooks))
	for _, tool := range tools {
		if expectedHooks[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedHooks {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in coordinator+memory config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithCoordinatorAndMemory_RegistersAllCoordinatorTools(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
		Memory:      backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedCoordinatorTools := map[string]bool{
		"agent_spawn":         true,
		"agent_list":          true,
		"agent_terminate":     true,
		"agent_metrics":       true,
		"agent_types_list":    true,
		"agent_pool_list":     true,
		"agent_pool_scale":    true,
		"agent_health":        true,
		"config_get":          true,
		"config_set":          true,
		"config_list":         true,
		"config_validate":     true,
		"swarm_state":         true,
		"swarm_reconfigure":   true,
		"orchestrate_plan":    true,
		"orchestrate_execute": true,
		"orchestrate_status":  true,
	}

	counts := make(map[string]int, len(expectedCoordinatorTools))
	for _, tool := range tools {
		if expectedCoordinatorTools[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedCoordinatorTools {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in coordinator+memory config, got %d", name, counts[name])
		}
	}
}

func TestNewMCPServer_WithCoordinatorAndMemory_RegistersAllMemoryTools(t *testing.T) {
	coord, err := NewSwarmCoordinator(SwarmConfig{
		Topology: TopologyMesh,
	})
	if err != nil {
		t.Fatalf("failed to create coordinator: %v", err)
	}
	t.Cleanup(func() {
		_ = coord.Shutdown()
	})

	backend, err := NewSQLiteBackend(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize sqlite backend: %v", err)
	}

	server := NewMCPServer(MCPServerConfig{
		Coordinator: coord,
		Memory:      backend,
	})
	if server == nil {
		t.Fatal("expected MCP server to be created")
	}

	tools := server.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected MCP server to expose tools")
	}

	expectedMemoryTools := map[string]bool{
		"memory_store":    true,
		"memory_retrieve": true,
		"memory_query":    true,
		"memory_search":   true,
		"memory_delete":   true,
	}

	counts := make(map[string]int, len(expectedMemoryTools))
	for _, tool := range tools {
		if expectedMemoryTools[tool.Name] {
			counts[tool.Name]++
		}
	}

	for name := range expectedMemoryTools {
		if counts[name] != 1 {
			t.Fatalf("expected exactly one %s tool in coordinator+memory config, got %d", name, counts[name])
		}
	}
}
