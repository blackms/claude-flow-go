// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"
	"math"
	"reflect"
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
						"minimum":     float64(1),
						"maximum":     float64(math.MaxInt64),
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
						"minimum":     float64(1),
						"maximum":     float64(math.MaxInt),
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
	sortSwarmRegistrations(swarms)
	swarmInfos := make([]map[string]interface{}, 0, len(swarms))
	for _, swarm := range swarms {
		if swarm == nil {
			continue
		}
		swarmInfos = append(swarmInfos, map[string]interface{}{
			"swarmId":       swarm.SwarmID,
			"name":          swarm.Name,
			"status":        swarm.Status,
			"currentAgents": swarm.CurrentAgents,
			"maxAgents":     swarm.MaxAgents,
		})
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

	if rawSwarmID, exists := args["swarmId"]; exists {
		swarmID, ok := rawSwarmID.(string)
		if !ok {
			return shared.MCPToolResult{
				Success: false,
				Error:   "swarmId must be a string",
			}, fmt.Errorf("swarmId must be a string")
		}
		opts.SwarmID = strings.TrimSpace(swarmID)
	}
	if rawType, exists := args["type"]; exists {
		agentType, ok := rawType.(string)
		if !ok {
			return shared.MCPToolResult{
				Success: false,
				Error:   "type must be a string",
			}, fmt.Errorf("type must be a string")
		}
		opts.Type = strings.TrimSpace(agentType)
	}
	if rawTask, exists := args["task"]; exists {
		task, ok := rawTask.(string)
		if !ok {
			return shared.MCPToolResult{
				Success: false,
				Error:   "task must be a string",
			}, fmt.Errorf("task must be a string")
		}
		opts.Task = strings.TrimSpace(task)
	}
	if rawTTL, exists := args["ttl"]; exists {
		switch ttl := rawTTL.(type) {
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
		default:
			return shared.MCPToolResult{
				Success: false,
				Error:   "ttl must be an integer",
			}, fmt.Errorf("ttl must be an integer")
		}
		if opts.TTL <= 0 {
			return shared.MCPToolResult{
				Success: false,
				Error:   "ttl must be greater than 0",
			}, fmt.Errorf("ttl must be greater than 0")
		}
		if opts.TTL > math.MaxInt64-shared.Now() {
			return shared.MCPToolResult{
				Success: false,
				Error:   "ttl is out of range",
			}, fmt.Errorf("ttl is out of range")
		}
	}
	if capsRaw, ok := args["capabilities"]; ok {
		if err := validateCapabilitiesInput(capsRaw); err != nil {
			return shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, err
		}
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
	rawAgentID, hasAgentID := args["agentId"]
	if !hasAgentID {
		return shared.MCPToolResult{
			Success: false,
			Error:   "agentId is required",
		}, fmt.Errorf("agentId is required")
	}
	agentID, ok := rawAgentID.(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "agentId must be a string",
		}, fmt.Errorf("agentId must be a string")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "agentId is required",
		}, fmt.Errorf("agentId is required")
	}

	errorMsg := ""
	if rawError, exists := args["error"]; exists {
		e, ok := rawError.(string)
		if !ok {
			return shared.MCPToolResult{
				Success: false,
				Error:   "error must be a string",
			}, fmt.Errorf("error must be a string")
		}
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
	if rawSwarmID, exists := args["swarmId"]; exists {
		swarmIDStr, ok := rawSwarmID.(string)
		if !ok {
			return shared.MCPToolResult{
				Success: false,
				Error:   "swarmId must be a string",
			}, fmt.Errorf("swarmId must be a string")
		}
		swarmID = strings.TrimSpace(swarmIDStr)
	}

	hasStatusFilter := false
	var statusFilter shared.EphemeralAgentStatus
	if rawStatus, exists := args["status"]; exists {
		statusStr, ok := rawStatus.(string)
		if !ok {
			return shared.MCPToolResult{
				Success: false,
				Error:   "status must be a string",
			}, fmt.Errorf("status must be a string")
		}
		if strings.TrimSpace(statusStr) != "" {
			parsedStatus, valid := parseEphemeralAgentStatus(statusStr)
			if !valid {
				return shared.MCPToolResult{
					Success: false,
					Error:   "status must be one of: spawning, active, completing, terminated",
				}, fmt.Errorf("status must be one of: spawning, active, completing, terminated")
			}
			hasStatusFilter = true
			statusFilter = parsedStatus
		}
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

	rawSwarmID, hasSwarmID := args["swarmId"]
	if !hasSwarmID {
		return shared.MCPToolResult{
			Success: false,
			Error:   "swarmId is required",
		}, fmt.Errorf("swarmId is required")
	}
	swarmID, ok := rawSwarmID.(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "swarmId must be a string",
		}, fmt.Errorf("swarmId must be a string")
	}
	if strings.TrimSpace(swarmID) == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "swarmId is required",
		}, fmt.Errorf("swarmId is required")
	}
	swarm.SwarmID = strings.TrimSpace(swarmID)

	rawName, hasName := args["name"]
	if !hasName {
		return shared.MCPToolResult{
			Success: false,
			Error:   "name is required",
		}, fmt.Errorf("name is required")
	}
	name, ok := rawName.(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "name must be a string",
		}, fmt.Errorf("name must be a string")
	}
	if strings.TrimSpace(name) == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "name is required",
		}, fmt.Errorf("name is required")
	}
	swarm.Name = strings.TrimSpace(name)

	if rawEndpoint, exists := args["endpoint"]; exists {
		endpoint, ok := rawEndpoint.(string)
		if !ok {
			return shared.MCPToolResult{
				Success: false,
				Error:   "endpoint must be a string",
			}, fmt.Errorf("endpoint must be a string")
		}
		swarm.Endpoint = strings.TrimSpace(endpoint)
	}

	rawMaxAgents, hasMaxAgents := args["maxAgents"]
	if !hasMaxAgents {
		return shared.MCPToolResult{
			Success: false,
			Error:   "maxAgents is required",
		}, fmt.Errorf("maxAgents is required")
	}
	switch maxAgents := rawMaxAgents.(type) {
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
		if maxAgents < int64(math.MinInt) || maxAgents > int64(math.MaxInt) {
			return shared.MCPToolResult{
				Success: false,
				Error:   "maxAgents is out of range",
			}, fmt.Errorf("maxAgents is out of range")
		}
		swarm.MaxAgents = int(maxAgents)
	default:
		return shared.MCPToolResult{
			Success: false,
			Error:   "maxAgents must be an integer",
		}, fmt.Errorf("maxAgents must be an integer")
	}

	if swarm.MaxAgents <= 0 {
		return shared.MCPToolResult{
			Success: false,
			Error:   "maxAgents must be greater than 0",
		}, fmt.Errorf("maxAgents must be greater than 0")
	}
	if capsRaw, ok := args["capabilities"]; ok {
		if err := validateCapabilitiesInput(capsRaw); err != nil {
			return shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, err
		}
		swarm.Capabilities = append(swarm.Capabilities, normalizeCapabilities(capsRaw)...)
	}
	if _, exists := t.hub.GetSwarm(swarm.SwarmID); exists {
		return shared.MCPToolResult{
			Success: false,
			Error:   "swarmId already exists",
		}, fmt.Errorf("swarmId already exists")
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
	rawSourceSwarmID, hasSourceSwarmID := args["sourceSwarmId"]
	if !hasSourceSwarmID {
		return shared.MCPToolResult{
			Success: false,
			Error:   "sourceSwarmId is required",
		}, fmt.Errorf("sourceSwarmId is required")
	}
	sourceSwarmID, ok := rawSourceSwarmID.(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "sourceSwarmId must be a string",
		}, fmt.Errorf("sourceSwarmId must be a string")
	}
	sourceSwarmID = strings.TrimSpace(sourceSwarmID)
	if sourceSwarmID == "" {
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

	msg, err := t.hub.Broadcast(sourceSwarmID, cloneStringInterfaceMap(payloadMap))
	if err != nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return shared.MCPToolResult{
		Success: true,
		Data:    cloneFederationMessage(msg),
	}, nil
}

// propose creates a proposal.
func (t *FederationTools) propose(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	rawProposerID, hasProposerID := args["proposerId"]
	if !hasProposerID {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposerId is required",
		}, fmt.Errorf("proposerId is required")
	}
	proposerID, ok := rawProposerID.(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposerId must be a string",
		}, fmt.Errorf("proposerId must be a string")
	}
	proposerID = strings.TrimSpace(proposerID)
	if proposerID == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposerId is required",
		}, fmt.Errorf("proposerId is required")
	}

	rawProposalType, hasProposalType := args["proposalType"]
	if !hasProposalType {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposalType is required",
		}, fmt.Errorf("proposalType is required")
	}
	proposalType, ok := rawProposalType.(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposalType must be a string",
		}, fmt.Errorf("proposalType must be a string")
	}
	proposalType = strings.TrimSpace(proposalType)
	if proposalType == "" {
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

	proposal, err := t.hub.Propose(proposerID, proposalType, cloneStringInterfaceMap(valueMap))
	if err != nil {
		return shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return shared.MCPToolResult{
		Success: true,
		Data:    cloneFederationProposal(proposal),
	}, nil
}

// vote votes on a proposal.
func (t *FederationTools) vote(ctx context.Context, args map[string]interface{}) (shared.MCPToolResult, error) {
	rawVoterID, hasVoterID := args["voterId"]
	if !hasVoterID {
		return shared.MCPToolResult{
			Success: false,
			Error:   "voterId is required",
		}, fmt.Errorf("voterId is required")
	}
	voterID, ok := rawVoterID.(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "voterId must be a string",
		}, fmt.Errorf("voterId must be a string")
	}
	voterID = strings.TrimSpace(voterID)
	if voterID == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "voterId is required",
		}, fmt.Errorf("voterId is required")
	}

	rawProposalID, hasProposalID := args["proposalId"]
	if !hasProposalID {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposalId is required",
		}, fmt.Errorf("proposalId is required")
	}
	proposalID, ok := rawProposalID.(string)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposalId must be a string",
		}, fmt.Errorf("proposalId must be a string")
	}
	proposalID = strings.TrimSpace(proposalID)
	if proposalID == "" {
		return shared.MCPToolResult{
			Success: false,
			Error:   "proposalId is required",
		}, fmt.Errorf("proposalId is required")
	}

	rawApprove, hasApprove := args["approve"]
	if !hasApprove {
		return shared.MCPToolResult{
			Success: false,
			Error:   "approve is required",
		}, fmt.Errorf("approve is required")
	}
	approve, ok := rawApprove.(bool)
	if !ok {
		return shared.MCPToolResult{
			Success: false,
			Error:   "approve must be a boolean",
		}, fmt.Errorf("approve must be a boolean")
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
			"proposal": cloneFederationProposal(proposal),
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

func validateCapabilitiesInput(raw interface{}) error {
	switch caps := raw.(type) {
	case []string:
		return nil
	case []interface{}:
		for _, cap := range caps {
			if _, ok := cap.(string); !ok {
				return fmt.Errorf("capabilities must contain only strings")
			}
		}
		return nil
	default:
		return fmt.Errorf("capabilities must be an array of strings")
	}
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
		if left == nil || right == nil {
			if left == nil && right == nil {
				return false
			}
			return left != nil
		}
		if left.CreatedAt == right.CreatedAt {
			return left.ID < right.ID
		}
		return left.CreatedAt < right.CreatedAt
	})
}

func sortSwarmRegistrations(swarms []*shared.SwarmRegistration) {
	sort.Slice(swarms, func(i, j int) bool {
		left := swarms[i]
		right := swarms[j]
		if left == nil || right == nil {
			if left == nil && right == nil {
				return false
			}
			return left != nil
		}
		return left.SwarmID < right.SwarmID
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
	case map[string]string:
		cloned := make(map[string]string, len(typed))
		for key, val := range typed {
			cloned[key] = val
		}
		return cloned
	case map[string]bool:
		cloned := make(map[string]bool, len(typed))
		for key, val := range typed {
			cloned[key] = val
		}
		return cloned
	case map[string]int:
		cloned := make(map[string]int, len(typed))
		for key, val := range typed {
			cloned[key] = val
		}
		return cloned
	case map[string]int64:
		cloned := make(map[string]int64, len(typed))
		for key, val := range typed {
			cloned[key] = val
		}
		return cloned
	case map[string]float64:
		cloned := make(map[string]float64, len(typed))
		for key, val := range typed {
			cloned[key] = val
		}
		return cloned
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for i := range typed {
			cloned[i] = cloneInterfaceValue(typed[i])
		}
		return cloned
	case []map[string]interface{}:
		cloned := make([]map[string]interface{}, len(typed))
		for i := range typed {
			cloned[i] = cloneStringInterfaceMap(typed[i])
		}
		return cloned
	case []map[string]string:
		cloned := make([]map[string]string, len(typed))
		for i := range typed {
			item := make(map[string]string, len(typed[i]))
			for key, val := range typed[i] {
				item[key] = val
			}
			cloned[i] = item
		}
		return cloned
	case []map[string]bool:
		cloned := make([]map[string]bool, len(typed))
		for i := range typed {
			item := make(map[string]bool, len(typed[i]))
			for key, val := range typed[i] {
				item[key] = val
			}
			cloned[i] = item
		}
		return cloned
	case []map[string]int:
		cloned := make([]map[string]int, len(typed))
		for i := range typed {
			item := make(map[string]int, len(typed[i]))
			for key, val := range typed[i] {
				item[key] = val
			}
			cloned[i] = item
		}
		return cloned
	case []map[string]int64:
		cloned := make([]map[string]int64, len(typed))
		for i := range typed {
			item := make(map[string]int64, len(typed[i]))
			for key, val := range typed[i] {
				item[key] = val
			}
			cloned[i] = item
		}
		return cloned
	case []map[string]float64:
		cloned := make([]map[string]float64, len(typed))
		for i := range typed {
			item := make(map[string]float64, len(typed[i]))
			for key, val := range typed[i] {
				item[key] = val
			}
			cloned[i] = item
		}
		return cloned
	case []string:
		cloned := make([]string, len(typed))
		copy(cloned, typed)
		return cloned
	case []bool:
		cloned := make([]bool, len(typed))
		copy(cloned, typed)
		return cloned
	case []int:
		cloned := make([]int, len(typed))
		copy(cloned, typed)
		return cloned
	case []int64:
		cloned := make([]int64, len(typed))
		copy(cloned, typed)
		return cloned
	case []float64:
		cloned := make([]float64, len(typed))
		copy(cloned, typed)
		return cloned
	default:
		return cloneReflectContainer(value)
	}
}

func cloneReflectContainer(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Map:
		return cloneMapValue(reflected).Interface()
	case reflect.Slice:
		return cloneSliceValue(reflected).Interface()
	case reflect.Array:
		return cloneArrayValue(reflected).Interface()
	case reflect.Ptr:
		return clonePointerValue(reflected).Interface()
	case reflect.Struct:
		return cloneStructValue(reflected).Interface()
	default:
		return value
	}
}

func cloneMapValue(value reflect.Value) reflect.Value {
	if value.IsNil() {
		return reflect.Zero(value.Type())
	}

	cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
	elemType := value.Type().Elem()
	iter := value.MapRange()
	for iter.Next() {
		cloned.SetMapIndex(iter.Key(), cloneValueForType(elemType, iter.Value()))
	}
	return cloned
}

func cloneSliceValue(value reflect.Value) reflect.Value {
	if value.IsNil() {
		return reflect.Zero(value.Type())
	}

	cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
	elemType := value.Type().Elem()
	for i := 0; i < value.Len(); i++ {
		cloned.Index(i).Set(cloneValueForType(elemType, value.Index(i)))
	}
	return cloned
}

func cloneArrayValue(value reflect.Value) reflect.Value {
	cloned := reflect.New(value.Type()).Elem()
	elemType := value.Type().Elem()
	for i := 0; i < value.Len(); i++ {
		cloned.Index(i).Set(cloneValueForType(elemType, value.Index(i)))
	}
	return cloned
}

func clonePointerValue(value reflect.Value) reflect.Value {
	if value.IsNil() {
		return reflect.Zero(value.Type())
	}

	elemType := value.Type().Elem()
	clonedPtr := reflect.New(elemType)
	clonedPtr.Elem().Set(cloneValueForType(elemType, value.Elem()))
	return clonedPtr
}

func cloneStructValue(value reflect.Value) reflect.Value {
	cloned := reflect.New(value.Type()).Elem()
	cloned.Set(value)
	for i := 0; i < value.NumField(); i++ {
		targetField := cloned.Field(i)
		if !targetField.CanSet() {
			continue
		}
		targetField.Set(cloneValueForType(targetField.Type(), value.Field(i)))
	}
	return cloned
}

func cloneValueForType(targetType reflect.Type, value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return reflect.Zero(targetType)
	}

	if value.CanInterface() {
		clonedInterface := cloneInterfaceValue(value.Interface())
		if clonedInterface != nil {
			clonedValue := reflect.ValueOf(clonedInterface)
			if clonedValue.IsValid() {
				if clonedValue.Type().AssignableTo(targetType) {
					return clonedValue
				}
				if clonedValue.Type().ConvertibleTo(targetType) {
					return clonedValue.Convert(targetType)
				}
			}
		}
	}

	if value.Type().AssignableTo(targetType) {
		return value
	}
	if value.Type().ConvertibleTo(targetType) {
		return value.Convert(targetType)
	}
	return reflect.Zero(targetType)
}

func cloneFederationMessage(msg *shared.FederationMessage) *shared.FederationMessage {
	if msg == nil {
		return nil
	}
	cloned := *msg
	cloned.Payload = cloneInterfaceValue(msg.Payload)
	return &cloned
}

func cloneFederationProposal(proposal *shared.FederationProposal) *shared.FederationProposal {
	if proposal == nil {
		return nil
	}
	cloned := *proposal
	cloned.Value = cloneInterfaceValue(proposal.Value)
	if proposal.Votes != nil {
		cloned.Votes = make(map[string]bool, len(proposal.Votes))
		for swarmID, vote := range proposal.Votes {
			cloned.Votes[swarmID] = vote
		}
	}
	return &cloned
}
