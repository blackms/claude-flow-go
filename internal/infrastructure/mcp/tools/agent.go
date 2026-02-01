// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/pool"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// AllowedAgentTypes contains all valid agent types (33 types total).
var AllowedAgentTypes = []string{
	// Basic types (6)
	"coder", "tester", "reviewer", "coordinator", "designer", "deployer",
	// Queen Domain (1)
	"queen",
	// Security Domain (3)
	"security-architect", "cve-remediation", "threat-modeler",
	// Core Domain (5)
	"ddd-designer", "memory-specialist", "type-modernizer", "swarm-specialist", "mcp-optimizer",
	// Integration Domain (3)
	"agentic-flow", "cli-developer", "neural-integrator",
	// Support Domain (3)
	"tdd-tester", "performance-engineer", "release-manager",
	// Extended Types (12)
	"researcher", "architect", "analyst", "optimizer", "security-auditor",
	"core-architect", "test-architect", "integration-architect", "hooks-developer",
	"mcp-specialist", "documentation-lead", "devops-engineer",
}

// AgentTools provides MCP tools for agent management operations.
type AgentTools struct {
	coordinator   *coordinator.SwarmCoordinator
	poolManager   *pool.AgentPoolManager
	registry      *agent.AgentTypeRegistry
	healthMonitor *pool.TypeHealthMonitor
}

// NewAgentTools creates a new AgentTools instance.
func NewAgentTools(coord *coordinator.SwarmCoordinator) *AgentTools {
	return &AgentTools{
		coordinator: coord,
	}
}

// NewAgentToolsWithPool creates AgentTools with pool management support.
func NewAgentToolsWithPool(coord *coordinator.SwarmCoordinator, pm *pool.AgentPoolManager, reg *agent.AgentTypeRegistry, hm *pool.TypeHealthMonitor) *AgentTools {
	return &AgentTools{
		coordinator:   coord,
		poolManager:   pm,
		registry:      reg,
		healthMonitor: hm,
	}
}

// GetTools returns available agent tools.
func (t *AgentTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "agent_spawn",
			Description: "Spawn a new agent in the swarm. Supports 33 agent types.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Unique agent identifier (optional, auto-generated if not provided)",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Agent type: coder, tester, reviewer, coordinator, designer, deployer, queen, security-architect, cve-remediation, threat-modeler, ddd-designer, memory-specialist, type-modernizer, swarm-specialist, mcp-optimizer, agentic-flow, cli-developer, neural-integrator, tdd-tester, performance-engineer, release-manager, researcher, architect, analyst, optimizer, security-auditor, core-architect, test-architect, integration-architect, hooks-developer, mcp-specialist, documentation-lead, devops-engineer",
						"enum":        AllowedAgentTypes,
					},
					"capabilities": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Agent capabilities (optional, defaults based on type)",
					},
				},
				"required": []string{"type"},
			},
		},
		{
			Name:        "agent_list",
			Description: "List all agents in the swarm",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by agent type (optional)",
					},
				},
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
		{
			Name:        "agent_types_list",
			Description: "List all available agent types with their capabilities",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tag": map[string]interface{}{
						"type":        "string",
						"description": "Filter by tag (optional)",
					},
					"domain": map[string]interface{}{
						"type":        "string",
						"description": "Filter by domain: queen, security, core, integration, support (optional)",
					},
				},
			},
		},
		{
			Name:        "agent_pool_list",
			Description: "List agent pools with statistics",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by agent type (optional)",
					},
				},
			},
		},
		{
			Name:        "agent_pool_scale",
			Description: "Scale an agent pool to a target size",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Agent type to scale",
						"enum":        AllowedAgentTypes,
					},
					"targetSize": map[string]interface{}{
						"type":        "integer",
						"description": "Target number of agents",
					},
				},
				"required": []string{"type", "targetSize"},
			},
		},
		{
			Name:        "agent_health",
			Description: "Get health metrics for agent types",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Agent type (optional, returns all if not specified)",
					},
				},
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
		return t.listAgents(params)
	case "agent_terminate":
		return t.terminateAgent(params)
	case "agent_metrics":
		return t.getAgentMetrics(params)
	case "agent_types_list":
		return t.listAgentTypes(params)
	case "agent_pool_list":
		return t.listPools(params)
	case "agent_pool_scale":
		return t.scalePool(params)
	case "agent_health":
		return t.getHealth(params)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// isValidAgentType checks if an agent type is valid.
func isValidAgentType(agentType string) bool {
	for _, vt := range AllowedAgentTypes {
		if agentType == vt {
			return true
		}
	}
	return false
}

func (t *AgentTools) spawnAgent(params map[string]interface{}) (*shared.MCPToolResult, error) {
	agentType, ok := params["type"].(string)
	if !ok || agentType == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: type is required",
		}, nil
	}

	if !isValidAgentType(agentType) {
		return &shared.MCPToolResult{
			Success: false,
			Error:   fmt.Sprintf("validation: type must be one of %v", AllowedAgentTypes),
		}, nil
	}

	// ID is optional - generate if not provided
	id, _ := params["id"].(string)
	if id == "" {
		id = shared.GenerateID("agent")
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

	// Use pool manager if available, otherwise use coordinator
	if t.poolManager != nil {
		a, err := t.poolManager.Spawn(shared.AgentType(agentType), config)
		if err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		agentData := a.ToShared()
		return &shared.MCPToolResult{
			Success: true,
			Agent:   &agentData,
		}, nil
	}

	a, err := t.coordinator.SpawnAgent(config)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	agentData := a.ToShared()
	return &shared.MCPToolResult{
		Success: true,
		Agent:   &agentData,
	}, nil
}

func (t *AgentTools) listAgents(params map[string]interface{}) (*shared.MCPToolResult, error) {
	agents := t.coordinator.ListAgents()

	// Filter by type if provided
	filterType, hasFilter := params["type"].(string)

	agentList := make([]shared.Agent, 0, len(agents))
	for _, a := range agents {
		if hasFilter && filterType != "" {
			if string(a.Type) != filterType {
				continue
			}
		}
		agentList = append(agentList, a.ToShared())
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

func (t *AgentTools) listAgentTypes(params map[string]interface{}) (*shared.MCPToolResult, error) {
	if t.registry == nil {
		// Return basic list without registry
		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"types": AllowedAgentTypes,
				"count": len(AllowedAgentTypes),
			},
		}, nil
	}

	// Filter by tag or domain
	tag, _ := params["tag"].(string)
	domain, _ := params["domain"].(string)

	var types []shared.AgentType
	if tag != "" {
		types = t.registry.ListByTag(tag)
	} else if domain != "" {
		types = t.registry.ListByDomain(shared.AgentDomain(domain))
	} else {
		types = t.registry.ListAll()
	}

	// Build detailed list
	typeDetails := make([]map[string]interface{}, 0, len(types))
	for _, at := range types {
		spec := t.registry.GetSpec(at)
		if spec != nil {
			typeDetails = append(typeDetails, map[string]interface{}{
				"type":           string(spec.Type),
				"description":    spec.Description,
				"capabilities":   spec.Capabilities,
				"modelTier":      string(spec.ModelTier),
				"tags":           spec.Tags,
				"neuralPatterns": spec.NeuralPatterns,
				"domain":         string(spec.Domain),
			})
		}
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"types": typeDetails,
			"count": len(typeDetails),
		},
	}, nil
}

func (t *AgentTools) listPools(params map[string]interface{}) (*shared.MCPToolResult, error) {
	if t.poolManager == nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "pool manager not available",
		}, nil
	}

	filterType, _ := params["type"].(string)

	if filterType != "" {
		stats := t.poolManager.GetPoolStats(shared.AgentType(filterType))
		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"pool": stats,
			},
		}, nil
	}

	allStats := t.poolManager.GetAllPoolStats()
	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"pools": allStats,
			"count": len(allStats),
			"totalAgents": t.poolManager.GetTotalAgentCount(),
		},
	}, nil
}

func (t *AgentTools) scalePool(params map[string]interface{}) (*shared.MCPToolResult, error) {
	if t.poolManager == nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "pool manager not available",
		}, nil
	}

	agentType, ok := params["type"].(string)
	if !ok || agentType == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: type is required",
		}, nil
	}

	if !isValidAgentType(agentType) {
		return &shared.MCPToolResult{
			Success: false,
			Error:   fmt.Sprintf("validation: type must be one of %v", AllowedAgentTypes),
		}, nil
	}

	targetSize, ok := params["targetSize"].(float64)
	if !ok {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: targetSize is required",
		}, nil
	}

	err := t.poolManager.Scale(shared.AgentType(agentType), int(targetSize))
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	stats := t.poolManager.GetPoolStats(shared.AgentType(agentType))
	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"pool":    stats,
			"message": fmt.Sprintf("Pool scaled to %d agents", int(targetSize)),
		},
	}, nil
}

func (t *AgentTools) getHealth(params map[string]interface{}) (*shared.MCPToolResult, error) {
	if t.healthMonitor == nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "health monitor not available",
		}, nil
	}

	filterType, _ := params["type"].(string)

	if filterType != "" {
		metrics := t.healthMonitor.GetMetrics(shared.AgentType(filterType))
		if metrics == nil {
			return &shared.MCPToolResult{
				Success: true,
				Data: map[string]interface{}{
					"metrics": nil,
					"message": "No metrics available for this type",
				},
			}, nil
		}
		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"metrics": metrics,
			},
		}, nil
	}

	allMetrics := t.healthMonitor.GetAllMetrics()
	overallHealth := t.healthMonitor.GetOverallHealth()
	alerts := t.healthMonitor.GetUnresolvedAlerts()

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"metrics":       allMetrics,
			"overallHealth": string(overallHealth),
			"unresolvedAlerts": alerts,
		},
	}, nil
}
