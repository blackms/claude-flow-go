package claudeflow

import (
	"reflect"
	"testing"
)

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
		"agent_spawn":       true,
		"agent_list":        true,
		"agent_terminate":   true,
		"agent_metrics":     true,
		"agent_types_list":  true,
		"agent_pool_list":   true,
		"agent_pool_scale":  true,
		"agent_health":      true,
		"config_get":        true,
		"config_set":        true,
		"config_list":       true,
		"config_validate":   true,
		"swarm_state":       true,
		"swarm_reconfigure": true,
		"orchestrate_plan":  true,
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
		"agent_spawn":       true,
		"agent_list":        true,
		"agent_terminate":   true,
		"agent_metrics":     true,
		"agent_types_list":  true,
		"agent_pool_list":   true,
		"agent_pool_scale":  true,
		"agent_health":      true,
		"config_get":        true,
		"config_set":        true,
		"config_list":       true,
		"config_validate":   true,
		"swarm_state":       true,
		"swarm_reconfigure": true,
		"orchestrate_plan":  true,
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
