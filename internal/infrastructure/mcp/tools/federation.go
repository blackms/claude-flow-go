// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

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
						"type":        "integer",
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
						"enum":        []string{"spawning", "active", "completing", "terminated"},
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
						"type":        "integer",
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
	sort.Slice(swarms, func(i, j int) bool {
		return swarms[i].SwarmID < swarms[j].SwarmID
	})
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
		opts.SwarmID = strings.TrimSpace(swarmID)
	}
	if agentType, ok := args["type"].(string); ok {
		opts.Type = strings.TrimSpace(agentType)
	}
	if task, ok := args["task"].(string); ok {
		opts.Task = strings.TrimSpace(task)
	}
	switch ttl := args["ttl"].(type) {
	case float64:
		if math.IsNaN(ttl) || math.IsInf(ttl, 0) {
			return shared.MCPToolResult{
				Success: false,
				Error:   "ttl must be a finite integer",
			}, fmt.Errorf("ttl must be a finite integer")
		}
		if math.Trunc(ttl) != ttl {
			return shared.MCPToolResult{
				Success: false,
				Error:   "ttl must be an integer",
			}, fmt.Errorf("ttl must be an integer")
		}
		if ttl < float64(math.MinInt64) || ttl > float64(math.MaxInt64) {
			return shared.MCPToolResult{
				Success: false,
				Error:   "ttl is out of range",
			}, fmt.Errorf("ttl is out of range")
		}
		opts.TTL = int64(ttl)
	case int:
		opts.TTL = int64(ttl)
	case int64:
		opts.TTL = ttl
	}
	if capsRaw, ok := args["capabilities"]; ok {
		opts.Capabilities = normalizeCapabilities(capsRaw)
	}

	if strings.TrimSpace(opts.Type) == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "type is required",
		}, fmt.Errorf("type is required")
	}

	if strings.TrimSpace(opts.Task) == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "task is required",
		}, fmt.Errorf("task is required")
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
	agentID = strings.TrimSpace(agentID)
	if !ok || agentID == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "agentId is required",
		}, fmt.Errorf("agentId is required")
	}

	errorMsg := ""
	if e, ok := args["error"].(string); ok {
		errorMsg = strings.TrimSpace(e)
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
	swarmID := ""
	if rawSwarmID, ok := args["swarmId"].(string); ok {
		swarmID = strings.TrimSpace(rawSwarmID)
	}

	hasStatusFilter := false
	var statusFilter shared.EphemeralAgentStatus
	if rawStatus, ok := args["status"].(string); ok && strings.TrimSpace(rawStatus) != "" {
		parsedStatus, valid := parseEphemeralAgentStatus(rawStatus)
		if !valid {
			return shared.MCPToolResult{
				Success: false,
				Error:   "status must be one of: spawning, active, completing, terminated",
			}, fmt.Errorf("status must be one of: spawning, active, completing, terminated")
		}
		hasStatusFilter = true
		statusFilter = parsedStatus
	}

	var agents []*shared.EphemeralAgent
	switch {
	case swarmID != "":
		agents = t.hub.GetAgentsBySwarm(swarmID)
	case hasStatusFilter:
		agents = t.hub.GetAgentsByStatus(statusFilter)
	default:
		agents = t.hub.GetAgents()
	}

	if swarmID != "" && hasStatusFilter {
		filtered := make([]*shared.EphemeralAgent, 0, len(agents))
		for _, agent := range agents {
			if agent.Status == statusFilter {
				filtered = append(filtered, agent)
			}
		}
		agents = filtered
	}
	sortEphemeralAgents(agents)

	return shared.MCPToolResult{
		Success: true,
		Data:    cloneEphemeralAgents(agents),
	}, nil
}

// registerSwarm registers a swarm.
func (t *FederationTools) registerSwarm(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	swarm := shared.SwarmRegistration{
		Capabilities: []string{},
	}

	if swarmID, ok := args["swarmId"].(string); ok && strings.TrimSpace(swarmID) != "" {
		swarm.SwarmID = strings.TrimSpace(swarmID)
	} else {
		return shared.MCPToolResult{
			Success: false,
			Error:   "swarmId is required",
		}, fmt.Errorf("swarmId is required")
	}

	if name, ok := args["name"].(string); ok && strings.TrimSpace(name) != "" {
		swarm.Name = strings.TrimSpace(name)
	} else {
		return shared.MCPToolResult{
			Success: false,
			Error:   "name is required",
		}, fmt.Errorf("name is required")
	}
	if endpoint, ok := args["endpoint"].(string); ok {
		swarm.Endpoint = strings.TrimSpace(endpoint)
	}

	switch maxAgents := args["maxAgents"].(type) {
	case float64:
		if math.IsNaN(maxAgents) || math.IsInf(maxAgents, 0) {
			return shared.MCPToolResult{
				Success: false,
				Error:   "maxAgents must be a finite integer",
			}, fmt.Errorf("maxAgents must be a finite integer")
		}
		if math.Trunc(maxAgents) != maxAgents {
			return shared.MCPToolResult{
				Success: false,
				Error:   "maxAgents must be an integer",
			}, fmt.Errorf("maxAgents must be an integer")
		}
		if maxAgents < float64(math.MinInt) || maxAgents > float64(math.MaxInt) {
			return shared.MCPToolResult{
				Success: false,
				Error:   "maxAgents is out of range",
			}, fmt.Errorf("maxAgents is out of range")
		}
		swarm.MaxAgents = int(maxAgents)
	case int:
		swarm.MaxAgents = maxAgents
	case int64:
		swarm.MaxAgents = int(maxAgents)
	default:
		return shared.MCPToolResult{
			Success: false,
			Error:   "maxAgents is required",
		}, fmt.Errorf("maxAgents is required")
	}

	if swarm.MaxAgents <= 0 {
		return shared.MCPToolResult{
			Success: false,
			Error:   "maxAgents must be greater than 0",
		}, fmt.Errorf("maxAgents must be greater than 0")
	}
	if capsRaw, ok := args["capabilities"]; ok {
		swarm.Capabilities = append(swarm.Capabilities, normalizeCapabilities(capsRaw)...)
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
	sourceSwarmID = strings.TrimSpace(sourceSwarmID)
	if !ok || sourceSwarmID == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "sourceSwarmId is required",
		}, fmt.Errorf("sourceSwarmId is required")
	}

	payload, hasPayload := args["payload"]
	if !hasPayload || payload == nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   "payload is required",
		}, fmt.Errorf("payload is required")
	}
	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "payload must be an object",
		}, fmt.Errorf("payload must be an object")
	}

	msg, err := t.hub.Broadcast(sourceSwarmID, payloadMap)
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
	proposerID = strings.TrimSpace(proposerID)
	if !ok || proposerID == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposerId is required",
		}, fmt.Errorf("proposerId is required")
	}

	proposalType, ok := args["proposalType"].(string)
	proposalType = strings.TrimSpace(proposalType)
	if !ok || proposalType == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposalType is required",
		}, fmt.Errorf("proposalType is required")
	}

	value, hasValue := args["value"]
	if !hasValue || value == nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   "value is required",
		}, fmt.Errorf("value is required")
	}
	valueMap, ok := value.(map[string]interface{})
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "value must be an object",
		}, fmt.Errorf("value must be an object")
	}

	proposal, err := t.hub.Propose(proposerID, proposalType, valueMap)
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
	voterID = strings.TrimSpace(voterID)
	if !ok || voterID == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "voterId is required",
		}, fmt.Errorf("voterId is required")
	}

	proposalID, ok := args["proposalId"].(string)
	proposalID = strings.TrimSpace(proposalID)
	if !ok || proposalID == "" {
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

func normalizeCapabilities(raw interface{}) []string {
	capabilities := make([]string, 0)
	seen := make(map[string]bool)
	appendCapability := func(value string) {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" {
			return
		}
		if seen[cleaned] {
			return
		}
		seen[cleaned] = true
		capabilities = append(capabilities, cleaned)
	}

	switch caps := raw.(type) {
	case []interface{}:
		for _, cap := range caps {
			if s, ok := cap.(string); ok {
				appendCapability(s)
			}
		}
	case []string:
		for _, cap := range caps {
			appendCapability(cap)
		}
	}

	return capabilities
}

func parseEphemeralAgentStatus(raw string) (shared.EphemeralAgentStatus, bool) {
	status := shared.EphemeralAgentStatus(strings.ToLower(strings.TrimSpace(raw)))
	switch status {
	case shared.EphemeralStatusSpawning,
		shared.EphemeralStatusActive,
		shared.EphemeralStatusCompleting,
		shared.EphemeralStatusTerminated:
		return status, true
	default:
		return "", false
	}
}

func sortEphemeralAgents(agents []*shared.EphemeralAgent) {
	sort.Slice(agents, func(i, j int) bool {
		left := agents[i]
		right := agents[j]
		if left.CreatedAt == right.CreatedAt {
			return left.ID < right.ID
		}
		return left.CreatedAt < right.CreatedAt
	})
}

func cloneEphemeralAgents(agents []*shared.EphemeralAgent) []*shared.EphemeralAgent {
	cloned := make([]*shared.EphemeralAgent, 0, len(agents))
	for _, agent := range agents {
		if agent == nil {
			continue
		}
		copyAgent := *agent
		copyAgent.Metadata = cloneStringInterfaceMap(agent.Metadata)
		copyAgent.Result = cloneInterfaceValue(agent.Result)
		cloned = append(cloned, &copyAgent)
	}
	return cloned
}

func cloneStringInterfaceMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	output := make(map[string]interface{}, len(input))
	for key, value := range input {
		output[key] = cloneInterfaceValue(value)
	}
	return output
}

func cloneInterfaceValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneStringInterfaceMap(typed)
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for i := range typed {
			cloned[i] = cloneInterfaceValue(typed[i])
		}
		return cloned
	case []string:
		cloned := make([]string, len(typed))
		copy(cloned, typed)
		return cloned
	default:
		return value
	}
}
