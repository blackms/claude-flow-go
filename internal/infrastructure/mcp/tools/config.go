// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ConfigTools provides MCP tools for configuration operations.
type ConfigTools struct {
	coordinator *coordinator.SwarmCoordinator
	config      map[string]interface{}
}

// NewConfigTools creates a new ConfigTools instance.
func NewConfigTools(coord *coordinator.SwarmCoordinator) *ConfigTools {
	return &ConfigTools{
		coordinator: coord,
		config:      make(map[string]interface{}),
	}
}

// GetTools returns available config tools.
func (t *ConfigTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "config_get",
			Description: "Get configuration value",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Configuration key",
					},
				},
				"required": []string{"key"},
			},
		},
		{
			Name:        "config_set",
			Description: "Set configuration value",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Configuration key",
					},
					"value": map[string]interface{}{
						"description": "Configuration value",
					},
				},
				"required": []string{"key", "value"},
			},
		},
		{
			Name:        "config_list",
			Description: "List all configuration values",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "config_validate",
			Description: "Validate configuration",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"config": map[string]interface{}{
						"type":        "object",
						"description": "Configuration to validate",
					},
				},
				"required": []string{"config"},
			},
		},
		{
			Name:        "swarm_state",
			Description: "Get current swarm state",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "swarm_reconfigure",
			Description: "Reconfigure the swarm",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"topology": map[string]interface{}{
						"type":        "string",
						"description": "Swarm topology (hierarchical, mesh, simple, adaptive)",
					},
				},
				"required": []string{"topology"},
			},
		},
	}
}

// Execute executes a config tool.
func (t *ConfigTools) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	switch toolName {
	case "config_get":
		return t.getConfig(params)
	case "config_set":
		return t.setConfig(params)
	case "config_list":
		return t.listConfig()
	case "config_validate":
		return t.validateConfig(params)
	case "swarm_state":
		return t.getSwarmState()
	case "swarm_reconfigure":
		return t.reconfigureSwarm(params)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (t *ConfigTools) getConfig(params map[string]interface{}) (*shared.MCPToolResult, error) {
	key, _ := params["key"].(string)
	if key == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: key is required",
		}, nil
	}

	value, exists := t.config[key]
	if !exists {
		return &shared.MCPToolResult{
			Success: true,
			Config:  map[string]interface{}{key: nil},
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
		Config:  map[string]interface{}{key: value},
	}, nil
}

func (t *ConfigTools) setConfig(params map[string]interface{}) (*shared.MCPToolResult, error) {
	key, _ := params["key"].(string)
	if key == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: key is required",
		}, nil
	}

	value := params["value"]
	t.config[key] = value

	return &shared.MCPToolResult{
		Success: true,
		Config:  map[string]interface{}{key: value},
	}, nil
}

func (t *ConfigTools) listConfig() (*shared.MCPToolResult, error) {
	configCopy := make(map[string]interface{})
	for k, v := range t.config {
		configCopy[k] = v
	}

	return &shared.MCPToolResult{
		Success: true,
		Config:  configCopy,
	}, nil
}

func (t *ConfigTools) validateConfig(params map[string]interface{}) (*shared.MCPToolResult, error) {
	config, ok := params["config"].(map[string]interface{})
	if !ok {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: config must be an object",
		}, nil
	}

	errors := make([]string, 0)

	// Validate topology if present
	if topology, ok := config["topology"].(string); ok {
		validTopologies := []string{"hierarchical", "mesh", "simple", "adaptive"}
		isValid := false
		for _, vt := range validTopologies {
			if topology == vt {
				isValid = true
				break
			}
		}
		if !isValid {
			errors = append(errors, fmt.Sprintf("invalid topology: %s", topology))
		}
	}

	// Validate maxAgents if present
	if maxAgents, ok := config["maxAgents"].(float64); ok {
		if maxAgents < 1 || maxAgents > 1000 {
			errors = append(errors, "maxAgents must be between 1 and 1000")
		}
	}

	if len(errors) > 0 {
		return &shared.MCPToolResult{
			Success: false,
			Valid:   false,
			Errors:  errors,
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
		Valid:   true,
	}, nil
}

func (t *ConfigTools) getSwarmState() (*shared.MCPToolResult, error) {
	state := t.coordinator.GetSwarmState()

	return &shared.MCPToolResult{
		Success: true,
		Config: map[string]interface{}{
			"topology":          string(state.Topology),
			"agentCount":        len(state.Agents),
			"leader":            state.Leader,
			"activeConnections": state.ActiveConnections,
		},
	}, nil
}

func (t *ConfigTools) reconfigureSwarm(params map[string]interface{}) (*shared.MCPToolResult, error) {
	topology, _ := params["topology"].(string)
	if topology == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: topology is required",
		}, nil
	}

	validTopologies := []string{"hierarchical", "mesh", "simple", "adaptive"}
	isValid := false
	for _, vt := range validTopologies {
		if topology == vt {
			isValid = true
			break
		}
	}
	if !isValid {
		return &shared.MCPToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid topology: %s, must be one of %v", topology, validTopologies),
		}, nil
	}

	if err := t.coordinator.Reconfigure(shared.SwarmTopology(topology)); err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
		Config: map[string]interface{}{
			"topology": topology,
		},
	}, nil
}
