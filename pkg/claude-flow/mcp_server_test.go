package claudeflow

import "testing"

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
