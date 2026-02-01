// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// AgentTools provides MCP tools for agent management operations.
type AgentTools struct {
	coordinator *coordinator.SwarmCoordinator
}

// NewAgentTools creates a new AgentTools instance.
func NewAgentTools(coord *coordinator.SwarmCoordinator) *AgentTools {
	return &AgentTools{
		coordinator: coord,
	}
}

// GetTools returns available agent tools.
func (t *AgentTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "agent_spawn",
			Description: "Spawn a new agent in the swarm",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Unique agent identifier",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Agent type (coder, tester, reviewer, etc.)",
					},
					"capabilities": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Agent capabilities",
					},
				},
				"required": []string{"id", "type"},
			},
		},
		{
			Name:        "agent_list",
			Description: "List all agents in the swarm",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "agent_terminate",
			Description: "Terminate an agent",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agentId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the agent to terminate",
					},
				},
				"required": []string{"agentId"},
			},
		},
		{
			Name:        "agent_metrics",
			Description: "Get metrics for an agent",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agentId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the agent",
					},
				},
				"required": []string{"agentId"},
			},
		},
	}
}

// Execute executes an agent tool.
func (t *AgentTools) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	switch toolName {
	case "agent_spawn":
		return t.spawnAgent(params)
	case "agent_list":
		return t.listAgents()
	case "agent_terminate":
		return t.terminateAgent(params)
	case "agent_metrics":
		return t.getAgentMetrics(params)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (t *AgentTools) spawnAgent(params map[string]interface{}) (*shared.MCPToolResult, error) {
	// Extract and validate parameters
	id, ok := params["id"].(string)
	if !ok || id == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: id is required and cannot be empty",
		}, nil
	}

	agentType, ok := params["type"].(string)
	if !ok {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: type is required",
		}, nil
	}

	validTypes := []string{"coder", "tester", "reviewer", "coordinator", "designer", "deployer"}
	isValid := false
	for _, vt := range validTypes {
		if agentType == vt {
			isValid = true
			break
		}
	}
	if !isValid {
		return &shared.MCPToolResult{
			Success: false,
			Error:   fmt.Sprintf("validation: type must be one of %v", validTypes),
		}, nil
	}

	var capabilities []string
	if caps, ok := params["capabilities"].([]interface{}); ok {
		for _, c := range caps {
			if s, ok := c.(string); ok {
				capabilities = append(capabilities, s)
			}
		}
	}

	config := shared.AgentConfig{
		ID:           id,
		Type:         shared.AgentType(agentType),
		Capabilities: capabilities,
	}

	agent, err := t.coordinator.SpawnAgent(config)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	agentData := agent.ToShared()
	return &shared.MCPToolResult{
		Success: true,
		Agent:   &agentData,
	}, nil
}

func (t *AgentTools) listAgents() (*shared.MCPToolResult, error) {
	agents := t.coordinator.ListAgents()

	agentList := make([]shared.Agent, len(agents))
	for i, a := range agents {
		agentList[i] = a.ToShared()
	}

	return &shared.MCPToolResult{
		Success: true,
		Agents:  agentList,
	}, nil
}

func (t *AgentTools) terminateAgent(params map[string]interface{}) (*shared.MCPToolResult, error) {
	agentID, ok := params["agentId"].(string)
	if !ok || agentID == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: agentId is required",
		}, nil
	}

	if err := t.coordinator.TerminateAgent(agentID); err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
	}, nil
}

func (t *AgentTools) getAgentMetrics(params map[string]interface{}) (*shared.MCPToolResult, error) {
	agentID, ok := params["agentId"].(string)
	if !ok || agentID == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: agentId is required",
		}, nil
	}

	metrics := t.coordinator.GetAgentMetrics(agentID)

	return &shared.MCPToolResult{
		Success: true,
		Metrics: &metrics,
	}, nil
}
