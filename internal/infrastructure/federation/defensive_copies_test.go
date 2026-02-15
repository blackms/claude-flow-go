package federation

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestFederationHub_GettersReturnDefensiveCopies(t *testing.T) {
	cfg := shared.DefaultFederationConfig()
	cfg.ConsensusQuorum = 1.0

	hub := NewFederationHub(cfg)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:      "swarm-defensive-source",
		Name:         "Defensive Source",
		MaxAgents:    4,
		Capabilities: []string{"core"},
		Metadata: map[string]interface{}{
			"region": "eu",
			"nested": map[string]interface{}{"zone": "z1"},
		},
	}); err != nil {
		t.Fatalf("failed to register source swarm: %v", err)
	}

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-defensive-target",
		Name:      "Defensive Target",
		MaxAgents: 4,
	}); err != nil {
		t.Fatalf("failed to register target swarm: %v", err)
	}

	spawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-defensive-source",
		Type:    "coder",
		Task:    "defensive-task",
		Metadata: map[string]interface{}{
			"scope": "internal",
			"nested": map[string]interface{}{
				"attempt": 1,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to spawn agent: %v", err)
	}

	msg, err := hub.SendMessage("swarm-defensive-source", "swarm-defensive-target", map[string]interface{}{
		"payload": map[string]interface{}{"count": 1},
	})
	if err != nil {
		t.Fatalf("failed to send direct message: %v", err)
	}

	proposal, err := hub.Propose("swarm-defensive-source", "deploy", map[string]interface{}{
		"plan": map[string]interface{}{"step": "initial"},
	})
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	swarmCopy, ok := hub.GetSwarm("swarm-defensive-source")
	if !ok {
		t.Fatal("expected source swarm to exist")
	}
	swarmCopy.Name = "tampered-name"
	swarmCopy.Capabilities[0] = "tampered-capability"
	swarmMetadata := requireMap(t, swarmCopy.Metadata, "swarm metadata copy")
	swarmMetadata["region"] = "tampered-region"
	requireMap(t, swarmMetadata["nested"], "swarm nested metadata copy")["zone"] = "tampered-zone"

	for _, swarm := range hub.GetSwarms() {
		if swarm.SwarmID == "swarm-defensive-source" {
			swarm.Name = "tampered-name-list"
			break
		}
	}

	agentCopy, ok := hub.GetAgent(spawn.AgentID)
	if !ok {
		t.Fatal("expected spawned agent to exist")
	}
	agentCopy.Task = "tampered-task"
	agentMetadata := requireMap(t, agentCopy.Metadata, "agent metadata copy")
	agentMetadata["scope"] = "tampered-scope"
	requireMap(t, agentMetadata["nested"], "agent nested metadata copy")["attempt"] = 999

	for _, agent := range hub.GetAgents() {
		if agent.ID == spawn.AgentID {
			agent.Task = "tampered-task-list"
			break
		}
	}

	msgCopy, ok := hub.GetMessage(msg.ID)
	if !ok {
		t.Fatal("expected sent message to exist")
	}
	requireMap(t, requireMap(t, msgCopy.Payload, "message payload copy")["payload"], "message nested payload copy")["count"] = 999

	for _, listedMsg := range hub.GetMessagesByType(shared.FederationMsgDirect, 0) {
		if listedMsg.ID == msg.ID {
			requireMap(t, requireMap(t, listedMsg.Payload, "listed message payload copy")["payload"], "listed message nested payload copy")["count"] = 777
			break
		}
	}

	proposalCopy, ok := hub.GetProposal(proposal.ID)
	if !ok {
		t.Fatal("expected proposal to exist")
	}
	proposalCopy.Type = "tampered-type"
	proposalCopy.Votes["swarm-defensive-target"] = false
	requireMap(t, requireMap(t, proposalCopy.Value, "proposal value copy")["plan"], "proposal nested value copy")["step"] = "tampered-step"

	for _, listedProposal := range hub.GetProposals() {
		if listedProposal.ID == proposal.ID {
			listedProposal.Type = "tampered-type-list"
			break
		}
	}

	events := hub.GetEvents(0)
	var consensusStartEvent *shared.FederationEvent
	for _, event := range events {
		if event != nil && event.Type == shared.FederationEventConsensusStarted {
			consensusStartEvent = event
			break
		}
	}
	if consensusStartEvent == nil {
		t.Fatal("expected consensus started event to exist")
	}
	eventData := requireMap(t, consensusStartEvent.Data, "consensus event data copy")
	originalEventProposalID := eventData["proposalId"]
	eventData["proposalId"] = "tampered-proposal-id"

	storedSwarm, ok := hub.GetSwarm("swarm-defensive-source")
	if !ok {
		t.Fatal("expected source swarm to still exist")
	}
	if storedSwarm.Name != "Defensive Source" {
		t.Fatalf("expected source swarm name to remain unchanged, got %q", storedSwarm.Name)
	}
	if storedSwarm.Capabilities[0] != "core" {
		t.Fatalf("expected source swarm capabilities to remain unchanged, got %v", storedSwarm.Capabilities)
	}
	if requireMap(t, storedSwarm.Metadata, "stored swarm metadata")["region"] != "eu" {
		t.Fatalf("expected source swarm metadata region to remain unchanged, got %v", storedSwarm.Metadata["region"])
	}
	if requireMap(t, requireMap(t, storedSwarm.Metadata, "stored swarm metadata nested root")["nested"], "stored swarm nested metadata")["zone"] != "z1" {
		t.Fatalf("expected source swarm nested metadata to remain unchanged, got %v", storedSwarm.Metadata["nested"])
	}

	storedAgent, ok := hub.GetAgent(spawn.AgentID)
	if !ok {
		t.Fatal("expected agent to still exist")
	}
	if storedAgent.Task != "defensive-task" {
		t.Fatalf("expected agent task to remain unchanged, got %q", storedAgent.Task)
	}
	if requireMap(t, storedAgent.Metadata, "stored agent metadata")["scope"] != "internal" {
		t.Fatalf("expected agent metadata scope to remain unchanged, got %v", storedAgent.Metadata["scope"])
	}
	if requireMap(t, requireMap(t, storedAgent.Metadata, "stored agent metadata nested root")["nested"], "stored agent nested metadata")["attempt"] != 1 {
		t.Fatalf("expected agent nested metadata to remain unchanged, got %v", storedAgent.Metadata["nested"])
	}

	storedMessage, ok := hub.GetMessage(msg.ID)
	if !ok {
		t.Fatal("expected message to still exist")
	}
	messagePayload := requireMap(t, storedMessage.Payload, "stored message payload")
	if requireMap(t, messagePayload["payload"], "stored message nested payload")["count"] != 1 {
		t.Fatalf("expected message payload count to remain unchanged, got %v", messagePayload["payload"])
	}

	storedProposal, ok := hub.GetProposal(proposal.ID)
	if !ok {
		t.Fatal("expected proposal to still exist")
	}
	if storedProposal.Type != "deploy" {
		t.Fatalf("expected proposal type to remain unchanged, got %q", storedProposal.Type)
	}
	if _, hasVote := storedProposal.Votes["swarm-defensive-target"]; hasVote {
		t.Fatalf("expected defensive vote mutation not to persist, votes=%v", storedProposal.Votes)
	}
	proposalValue := requireMap(t, storedProposal.Value, "stored proposal value")
	if requireMap(t, proposalValue["plan"], "stored proposal nested value")["step"] != "initial" {
		t.Fatalf("expected proposal nested value to remain unchanged, got %v", proposalValue["plan"])
	}

	eventsAfterMutation := hub.GetEvents(0)
	var storedConsensusEvent *shared.FederationEvent
	for _, event := range eventsAfterMutation {
		if event != nil && event.Type == shared.FederationEventConsensusStarted {
			storedConsensusEvent = event
			break
		}
	}
	if storedConsensusEvent == nil {
		t.Fatal("expected consensus started event to still exist")
	}
	if requireMap(t, storedConsensusEvent.Data, "stored consensus event data")["proposalId"] != originalEventProposalID {
		t.Fatalf("expected consensus event data to remain unchanged, got %v", storedConsensusEvent.Data)
	}
}

func TestFederationHub_CommandResultsReturnDefensiveCopies(t *testing.T) {
	cfg := shared.DefaultFederationConfig()
	cfg.ConsensusQuorum = 1.0

	hub := NewFederationHub(cfg)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	registerTestSwarm(t, hub, "swarm-command-source", "Command Source")
	registerTestSwarm(t, hub, "swarm-command-target", "Command Target")

	returnedMessage, err := hub.SendMessage("swarm-command-source", "swarm-command-target", map[string]interface{}{
		"payload": map[string]interface{}{"count": 3},
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
	requireMap(t, requireMap(t, returnedMessage.Payload, "returned direct message payload")["payload"], "returned direct message nested payload")["count"] = 999

	storedMessage, ok := hub.GetMessage(returnedMessage.ID)
	if !ok {
		t.Fatal("expected stored direct message")
	}
	storedDirectPayload := requireMap(t, storedMessage.Payload, "stored direct message payload")
	if requireMap(t, storedDirectPayload["payload"], "stored direct nested payload")["count"] != 3 {
		t.Fatalf("expected stored direct payload count to remain unchanged, got %v", storedDirectPayload["payload"])
	}

	returnedBroadcast, err := hub.Broadcast("swarm-command-source", map[string]interface{}{
		"payload": map[string]interface{}{"count": 5},
	})
	if err != nil {
		t.Fatalf("failed to broadcast message: %v", err)
	}
	requireMap(t, requireMap(t, returnedBroadcast.Payload, "returned broadcast payload")["payload"], "returned broadcast nested payload")["count"] = 111

	storedBroadcast, ok := hub.GetMessage(returnedBroadcast.ID)
	if !ok {
		t.Fatal("expected stored broadcast message")
	}
	storedBroadcastPayload := requireMap(t, storedBroadcast.Payload, "stored broadcast payload")
	if requireMap(t, storedBroadcastPayload["payload"], "stored broadcast nested payload")["count"] != 5 {
		t.Fatalf("expected stored broadcast payload count to remain unchanged, got %v", storedBroadcastPayload["payload"])
	}

	returnedProposal, err := hub.Propose("swarm-command-source", "rollout", map[string]interface{}{
		"plan": map[string]interface{}{"step": "prepare"},
	})
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}
	returnedProposal.Type = "tampered-type"
	returnedProposal.Votes["swarm-command-target"] = false
	requireMap(t, requireMap(t, returnedProposal.Value, "returned proposal value")["plan"], "returned proposal nested value")["step"] = "tampered-step"

	storedProposal, ok := hub.GetProposal(returnedProposal.ID)
	if !ok {
		t.Fatal("expected stored proposal")
	}
	if storedProposal.Type != "rollout" {
		t.Fatalf("expected stored proposal type to remain unchanged, got %q", storedProposal.Type)
	}
	if _, hasVote := storedProposal.Votes["swarm-command-target"]; hasVote {
		t.Fatalf("expected returned proposal vote mutation not to persist, got votes=%v", storedProposal.Votes)
	}
	storedProposalValue := requireMap(t, storedProposal.Value, "stored proposal value")
	if requireMap(t, storedProposalValue["plan"], "stored proposal nested value")["step"] != "prepare" {
		t.Fatalf("expected stored proposal nested value to remain unchanged, got %v", storedProposalValue["plan"])
	}
}

func requireMap(t *testing.T, value interface{}, context string) map[string]interface{} {
	t.Helper()
	mapped, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected %s to be map[string]interface{}, got %T", context, value)
	}
	return mapped
}
