// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/federation"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// FederationTools provides MCP tools for federation operations.
type FederationTools struct {
	hub *federation.FederationHub
}

// NewFederationTools creates a new FederationTools instance.
func NewFederationTools(hub *federation.FederationHub) *FederationTools {
	return &FederationTools{
		hub: hub,
	}
}

// GetTools returns available federation tools.
func (t *FederationTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "federation/status",
			Description: "Get federation status and statistics",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "federation/spawn-ephemeral",
			Description: "Spawn an ephemeral agent in the federation",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"swarmId": map[string]interface{}{
						"type":        "string",
						"description": "Target swarm ID (optional, auto-selects if not provided)",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Agent type",
					},
					"task": map[string]interface{}{
						"type":        "string",
						"description": "Task description",
					},
					"ttl": map[string]interface{}{
						"type":        "number",
						"description": "Time-to-live in milliseconds",
					},
					"capabilities": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Required capabilities",
					},
				},
				"required": []string{"type", "task"},
			},
		},
		{
			Name:        "federation/terminate-ephemeral",
			Description: "Terminate an ephemeral agent",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agentId": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID to terminate",
					},
					"error": map[string]interface{}{
						"type":        "string",
						"description": "Optional error message",
					},
				},
				"required": []string{"agentId"},
			},
		},
		{
			Name:        "federation/list-ephemeral",
			Description: "List ephemeral agents",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"swarmId": map[string]interface{}{
						"type":        "string",
						"description": "Filter by swarm ID",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status (spawning, active, completing, terminated)",
					},
				},
			},
		},
		{
			Name:        "federation/register-swarm",
			Description: "Register a swarm with the federation",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"swarmId": map[string]interface{}{
						"type":        "string",
						"description": "Unique swarm identifier",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Swarm name",
					},
					"endpoint": map[string]interface{}{
						"type":        "string",
						"description": "Swarm endpoint URL",
					},
					"maxAgents": map[string]interface{}{
						"type":        "number",
						"description": "Maximum agents capacity",
					},
					"capabilities": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Swarm capabilities",
					},
				},
				"required": []string{"swarmId", "name", "maxAgents"},
			},
		},
		{
			Name:        "federation/broadcast",
			Description: "Broadcast a message to all swarms",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sourceSwarmId": map[string]interface{}{
						"type":        "string",
						"description": "Source swarm ID",
					},
					"payload": map[string]interface{}{
						"type":        "object",
						"description": "Message payload",
					},
				},
				"required": []string{"sourceSwarmId", "payload"},
			},
		},
		{
			Name:        "federation/propose",
			Description: "Create a consensus proposal",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"proposerId": map[string]interface{}{
						"type":        "string",
						"description": "Proposer swarm ID",
					},
					"proposalType": map[string]interface{}{
						"type":        "string",
						"description": "Type of proposal",
					},
					"value": map[string]interface{}{
						"type":        "object",
						"description": "Proposal value",
					},
				},
				"required": []string{"proposerId", "proposalType", "value"},
			},
		},
		{
			Name:        "federation/vote",
			Description: "Vote on a proposal",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"voterId": map[string]interface{}{
						"type":        "string",
						"description": "Voter swarm ID",
					},
					"proposalId": map[string]interface{}{
						"type":        "string",
						"description": "Proposal ID",
					},
					"approve": map[string]interface{}{
						"type":        "boolean",
						"description": "Vote approval (true/false)",
					},
				},
				"required": []string{"voterId", "proposalId", "approve"},
			},
		},
	}
}

// Execute executes a federation tool using MCPToolProvider signature.
func (t *FederationTools) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	result, err := t.ExecuteTool(ctx, toolName, params)
	return &result, err
}

// ExecuteTool executes a federation tool.
func (t *FederationTools) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (shared.MCPToolResult, error) {
	switch name {
	case "federation/status":
		return t.getStatus(ctx, args)
	case "federation/spawn-ephemeral":
		return t.spawnEphemeral(ctx, args)
	case "federation/terminate-ephemeral":
		return t.terminateEphemeral(ctx, args)
	case "federation/list-ephemeral":
		return t.listEphemeral(ctx, args)
	case "federation/register-swarm":
		return t.registerSwarm(ctx, args)
	case "federation/broadcast":
		return t.broadcast(ctx, args)
	case "federation/propose":
		return t.propose(ctx, args)
	case "federation/vote":
		return t.vote(ctx, args)
	default:
		return shared.MCPToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", name),
		}, fmt.Errorf("unknown tool: %s", name)
	}
}

// getStatus returns federation status.
func (t *FederationTools) getStatus(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	stats := t.hub.GetStats()
	config := t.hub.GetConfig()

	swarms := t.hub.GetSwarms()
	swarmInfos := make([]map[string]interface{}, len(swarms))
	for i, swarm := range swarms {
		swarmInfos[i] = map[string]interface{}{
			"swarmId":       swarm.SwarmID,
			"name":          swarm.Name,
			"status":        swarm.Status,
			"currentAgents": swarm.CurrentAgents,
			"maxAgents":     swarm.MaxAgents,
		}
	}

	return shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"federationId":  config.FederationID,
			"stats":         stats,
			"swarms":        swarmInfos,
			"consensusEnabled": config.EnableConsensus,
			"consensusQuorum":  config.ConsensusQuorum,
		},
	}, nil
}

// spawnEphemeral spawns an ephemeral agent.
func (t *FederationTools) spawnEphemeral(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	opts := shared.SpawnEphemeralOptions{}

	if swarmID, ok := args["swarmId"].(string); ok {
		opts.SwarmID = swarmID
	}
	if agentType, ok := args["type"].(string); ok {
		opts.Type = agentType
	}
	if task, ok := args["task"].(string); ok {
		opts.Task = task
	}
	if ttl, ok := args["ttl"].(float64); ok {
		opts.TTL = int64(ttl)
	}
	if caps, ok := args["capabilities"].([]interface{}); ok {
		opts.Capabilities = make([]string, len(caps))
		for i, cap := range caps {
			if s, ok := cap.(string); ok {
				opts.Capabilities[i] = s
			}
		}
	}

	result, err := t.hub.SpawnEphemeralAgent(opts)
	if err != nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return shared.MCPToolResult{
		Success: true,
		Data:    result,
	}, nil
}

// terminateEphemeral terminates an ephemeral agent.
func (t *FederationTools) terminateEphemeral(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	agentID, ok := args["agentId"].(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "agentId is required",
		}, fmt.Errorf("agentId is required")
	}

	errorMsg := ""
	if e, ok := args["error"].(string); ok {
		errorMsg = e
	}

	err := t.hub.TerminateAgent(agentID, errorMsg)
	if err != nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return shared.MCPToolResult{
		Success: true,
		Data:    map[string]interface{}{"terminated": agentID},
	}, nil
}

// listEphemeral lists ephemeral agents.
func (t *FederationTools) listEphemeral(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	var agents []*shared.EphemeralAgent

	if swarmID, ok := args["swarmId"].(string); ok && swarmID != "" {
		agents = t.hub.GetAgentsBySwarm(swarmID)
	} else if statusStr, ok := args["status"].(string); ok && statusStr != "" {
		status := shared.EphemeralAgentStatus(statusStr)
		agents = t.hub.GetAgentsByStatus(status)
	} else {
		agents = t.hub.GetAgents()
	}

	return shared.MCPToolResult{
		Success: true,
		Data:    agents,
	}, nil
}

// registerSwarm registers a swarm.
func (t *FederationTools) registerSwarm(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	swarm := shared.SwarmRegistration{
		Capabilities: []string{},
	}

	if swarmID, ok := args["swarmId"].(string); ok {
		swarm.SwarmID = swarmID
	} else {
		return shared.MCPToolResult{
			Success: false,
			Error:   "swarmId is required",
		}, fmt.Errorf("swarmId is required")
	}

	if name, ok := args["name"].(string); ok {
		swarm.Name = name
	}
	if endpoint, ok := args["endpoint"].(string); ok {
		swarm.Endpoint = endpoint
	}
	if maxAgents, ok := args["maxAgents"].(float64); ok {
		swarm.MaxAgents = int(maxAgents)
	}
	if caps, ok := args["capabilities"].([]interface{}); ok {
		for _, cap := range caps {
			if s, ok := cap.(string); ok {
				swarm.Capabilities = append(swarm.Capabilities, s)
			}
		}
	}

	err := t.hub.RegisterSwarm(swarm)
	if err != nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return shared.MCPToolResult{
		Success: true,
		Data:    map[string]interface{}{"registered": swarm.SwarmID},
	}, nil
}

// broadcast broadcasts a message.
func (t *FederationTools) broadcast(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	sourceSwarmID, ok := args["sourceSwarmId"].(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "sourceSwarmId is required",
		}, fmt.Errorf("sourceSwarmId is required")
	}

	payload := args["payload"]

	msg, err := t.hub.Broadcast(sourceSwarmID, payload)
	if err != nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return shared.MCPToolResult{
		Success: true,
		Data:    msg,
	}, nil
}

// propose creates a proposal.
func (t *FederationTools) propose(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	proposerID, ok := args["proposerId"].(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposerId is required",
		}, fmt.Errorf("proposerId is required")
	}

	proposalType, ok := args["proposalType"].(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposalType is required",
		}, fmt.Errorf("proposalType is required")
	}

	value := args["value"]

	proposal, err := t.hub.Propose(proposerID, proposalType, value)
	if err != nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return shared.MCPToolResult{
		Success: true,
		Data:    proposal,
	}, nil
}

// vote votes on a proposal.
func (t *FederationTools) vote(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	voterID, ok := args["voterId"].(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "voterId is required",
		}, fmt.Errorf("voterId is required")
	}

	proposalID, ok := args["proposalId"].(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposalId is required",
		}, fmt.Errorf("proposalId is required")
	}

	approve, ok := args["approve"].(bool)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "approve is required",
		}, fmt.Errorf("approve is required")
	}

	err := t.hub.Vote(voterID, proposalID, approve)
	if err != nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	// Get updated proposal status
	proposal, _ := t.hub.GetProposal(proposalID)

	return shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"voted":    true,
			"proposal": proposal,
		},
	}, nil
}
