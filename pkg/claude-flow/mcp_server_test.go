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
	hasFederationStatus := false
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
		case "federation/status":
			hasFederationStatus = true
		case "hooks/list":
			hasHooksList = true
		}
	}

	if !hasAgentSpawn || !hasConfigGet || !hasOrchestratePlan || !hasFederationStatus || !hasHooksList {
		t.Fatalf(
			"expected core tool families from coordinator/federation/hooks to be present; got agent=%v config=%v orchestrate=%v federation=%v hooks=%v",
			hasAgentSpawn, hasConfigGet, hasOrchestratePlan, hasFederationStatus, hasHooksList,
		)
	}
}
