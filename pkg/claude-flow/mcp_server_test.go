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
