package federation

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

type cyclicNode struct {
	Value string
	Next  *cyclicNode
}

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

func TestFederationHub_InputMutationsAfterCallsDoNotAffectStoredState(t *testing.T) {
	cfg := shared.DefaultFederationConfig()
	cfg.ConsensusQuorum = 1.0

	hub := NewFederationHub(cfg)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	swarmMetadata := map[string]interface{}{
		"owner": "team-a",
		"nested": map[string]interface{}{
			"tier": "gold",
		},
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-ingress-source",
		Name:      "Ingress Source",
		MaxAgents: 4,
		Metadata:  swarmMetadata,
	}); err != nil {
		t.Fatalf("failed to register source swarm: %v", err)
	}
	registerTestSwarm(t, hub, "swarm-ingress-target", "Ingress Target")

	agentMetadata := map[string]interface{}{
		"trace": "initial",
		"nested": map[string]interface{}{
			"attempt": 1,
		},
	}
	spawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID:   "swarm-ingress-source",
		Type:      "coder",
		Task:      "ingress-mutation-test",
		Metadata:  agentMetadata,
		TTL:       60000,
	})
	if err != nil {
		t.Fatalf("failed to spawn agent: %v", err)
	}

	directPayload := map[string]interface{}{
		"payload": map[string]interface{}{"count": 10},
	}
	directMsg, err := hub.SendMessage("swarm-ingress-source", "swarm-ingress-target", directPayload)
	if err != nil {
		t.Fatalf("failed to send direct message: %v", err)
	}

	broadcastPayload := map[string]interface{}{
		"payload": map[string]interface{}{"count": 20},
	}
	broadcastMsg, err := hub.Broadcast("swarm-ingress-source", broadcastPayload)
	if err != nil {
		t.Fatalf("failed to send broadcast message: %v", err)
	}

	proposalValue := map[string]interface{}{
		"plan": map[string]interface{}{"step": "ship"},
	}
	proposal, err := hub.Propose("swarm-ingress-source", "deploy", proposalValue)
	if err != nil {
		t.Fatalf("failed to propose value: %v", err)
	}

	completionResult := map[string]interface{}{
		"status": "ok",
		"nested": map[string]interface{}{
			"attempt": 1,
		},
	}
	if err := hub.CompleteAgent(spawn.AgentID, completionResult); err != nil {
		t.Fatalf("failed to complete agent: %v", err)
	}

	swarmMetadata["owner"] = "tampered-owner"
	requireMap(t, swarmMetadata["nested"], "mutated swarm metadata nested")["tier"] = "bronze"
	agentMetadata["trace"] = "tampered-trace"
	requireMap(t, agentMetadata["nested"], "mutated agent metadata nested")["attempt"] = 99
	requireMap(t, directPayload["payload"], "mutated direct payload nested")["count"] = 999
	requireMap(t, broadcastPayload["payload"], "mutated broadcast payload nested")["count"] = 999
	requireMap(t, proposalValue["plan"], "mutated proposal value nested")["step"] = "tampered-step"
	completionResult["status"] = "tampered-status"
	requireMap(t, completionResult["nested"], "mutated completion result nested")["attempt"] = 999

	storedSwarm, ok := hub.GetSwarm("swarm-ingress-source")
	if !ok {
		t.Fatal("expected source swarm to exist")
	}
	if requireMap(t, storedSwarm.Metadata, "stored source swarm metadata")["owner"] != "team-a" {
		t.Fatalf("expected stored swarm owner to remain team-a, got %v", storedSwarm.Metadata["owner"])
	}
	if requireMap(t, requireMap(t, storedSwarm.Metadata, "stored source swarm metadata root")["nested"], "stored source swarm metadata nested")["tier"] != "gold" {
		t.Fatalf("expected stored swarm nested tier to remain gold, got %v", storedSwarm.Metadata["nested"])
	}

	storedAgent, ok := hub.GetAgent(spawn.AgentID)
	if !ok {
		t.Fatal("expected spawned agent to exist")
	}
	if requireMap(t, storedAgent.Metadata, "stored agent metadata")["trace"] != "initial" {
		t.Fatalf("expected stored agent trace to remain initial, got %v", storedAgent.Metadata["trace"])
	}
	if requireMap(t, requireMap(t, storedAgent.Metadata, "stored agent metadata root")["nested"], "stored agent metadata nested")["attempt"] != 1 {
		t.Fatalf("expected stored agent nested attempt to remain 1, got %v", storedAgent.Metadata["nested"])
	}
	if requireMap(t, storedAgent.Result, "stored agent result")["status"] != "ok" {
		t.Fatalf("expected stored agent result status to remain ok, got %v", storedAgent.Result)
	}
	if requireMap(t, requireMap(t, storedAgent.Result, "stored agent result root")["nested"], "stored agent result nested")["attempt"] != 1 {
		t.Fatalf("expected stored agent result nested attempt to remain 1, got %v", storedAgent.Result)
	}

	storedDirect, ok := hub.GetMessage(directMsg.ID)
	if !ok {
		t.Fatal("expected stored direct message to exist")
	}
	if requireMap(t, requireMap(t, storedDirect.Payload, "stored direct payload")["payload"], "stored direct payload nested")["count"] != 10 {
		t.Fatalf("expected stored direct message payload to remain unchanged, got %v", storedDirect.Payload)
	}

	storedBroadcast, ok := hub.GetMessage(broadcastMsg.ID)
	if !ok {
		t.Fatal("expected stored broadcast message to exist")
	}
	if requireMap(t, requireMap(t, storedBroadcast.Payload, "stored broadcast payload")["payload"], "stored broadcast payload nested")["count"] != 20 {
		t.Fatalf("expected stored broadcast payload to remain unchanged, got %v", storedBroadcast.Payload)
	}

	storedProposal, ok := hub.GetProposal(proposal.ID)
	if !ok {
		t.Fatal("expected stored proposal to exist")
	}
	if requireMap(t, requireMap(t, storedProposal.Value, "stored proposal value")["plan"], "stored proposal value nested")["step"] != "ship" {
		t.Fatalf("expected stored proposal nested value to remain unchanged, got %v", storedProposal.Value)
	}

	events := hub.GetEvents(0)
	foundCompletedEvent := false
	for _, event := range events {
		if event == nil || event.Type != shared.FederationEventAgentCompleted || event.AgentID != spawn.AgentID {
			continue
		}
		eventData := requireMap(t, event.Data, "agent completed event data")
		if eventData["status"] == "ok" {
			if requireMap(t, eventData["nested"], "agent completed event nested data")["attempt"] != 1 {
				t.Fatalf("expected event nested attempt to remain 1, got %v", eventData["nested"])
			}
			foundCompletedEvent = true
			break
		}
	}
	if !foundCompletedEvent {
		t.Fatal("expected completed event with immutable original result payload")
	}
}

func TestFederationHub_CloningHandlesCyclicInputPayloads(t *testing.T) {
	cfg := shared.DefaultFederationConfig()
	cfg.ConsensusQuorum = 1.0

	hub := NewFederationHub(cfg)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	registerTestSwarm(t, hub, "swarm-cycle-source", "Cycle Source")
	registerTestSwarm(t, hub, "swarm-cycle-target", "Cycle Target")

	cyclicPayload := map[string]interface{}{"kind": "cycle"}
	cyclicPayload["self"] = cyclicPayload

	sentMessage, err := hub.SendMessage("swarm-cycle-source", "swarm-cycle-target", cyclicPayload)
	if err != nil {
		t.Fatalf("expected send message with cyclic payload to succeed, got %v", err)
	}
	if sentMessage == nil {
		t.Fatal("expected sent message")
	}

	cyclicProposalValue := map[string]interface{}{"mode": "proposal-cycle"}
	cyclicProposalValue["self"] = cyclicProposalValue

	proposal, err := hub.Propose("swarm-cycle-source", "cyclic", cyclicProposalValue)
	if err != nil {
		t.Fatalf("expected propose with cyclic value to succeed, got %v", err)
	}
	if proposal == nil {
		t.Fatal("expected proposal")
	}

	cyclicPayload["kind"] = "tampered"
	cyclicProposalValue["mode"] = "tampered"

	storedMessage, ok := hub.GetMessage(sentMessage.ID)
	if !ok {
		t.Fatal("expected stored message")
	}
	storedMessagePayload := requireMap(t, storedMessage.Payload, "stored cyclic message payload")
	if storedMessagePayload["kind"] != "cycle" {
		t.Fatalf("expected stored cyclic message payload to remain unchanged, got %v", storedMessagePayload["kind"])
	}
	if nestedSelf := requireMap(t, storedMessagePayload["self"], "stored cyclic message self reference"); nestedSelf["kind"] != "cycle" {
		t.Fatalf("expected stored cyclic message self reference to be cloned, got %v", nestedSelf["kind"])
	}

	storedProposal, ok := hub.GetProposal(proposal.ID)
	if !ok {
		t.Fatal("expected stored proposal")
	}
	storedProposalValue := requireMap(t, storedProposal.Value, "stored cyclic proposal value")
	if storedProposalValue["mode"] != "proposal-cycle" {
		t.Fatalf("expected stored cyclic proposal value to remain unchanged, got %v", storedProposalValue["mode"])
	}
	if nestedSelf := requireMap(t, storedProposalValue["self"], "stored cyclic proposal self reference"); nestedSelf["mode"] != "proposal-cycle" {
		t.Fatalf("expected stored cyclic proposal self reference to be cloned, got %v", nestedSelf["mode"])
	}
}

func TestFederationHub_CloningHandlesPointerCycles(t *testing.T) {
	cfg := shared.DefaultFederationConfig()
	cfg.ConsensusQuorum = 1.0

	hub := NewFederationHub(cfg)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	registerTestSwarm(t, hub, "swarm-pointer-source", "Pointer Source")
	registerTestSwarm(t, hub, "swarm-pointer-target", "Pointer Target")

	messageNode := &cyclicNode{Value: "message-initial"}
	messageNode.Next = messageNode

	sentMessage, err := hub.SendMessage("swarm-pointer-source", "swarm-pointer-target", map[string]interface{}{
		"node": messageNode,
	})
	if err != nil {
		t.Fatalf("expected send with pointer-cycle payload to succeed, got %v", err)
	}

	proposalNode := &cyclicNode{Value: "proposal-initial"}
	proposalNode.Next = proposalNode

	proposal, err := hub.Propose("swarm-pointer-source", "pointer-cycle", map[string]interface{}{
		"node": proposalNode,
	})
	if err != nil {
		t.Fatalf("expected propose with pointer-cycle value to succeed, got %v", err)
	}

	messageNode.Value = "message-mutated"
	proposalNode.Value = "proposal-mutated"

	storedMessage, ok := hub.GetMessage(sentMessage.ID)
	if !ok {
		t.Fatal("expected stored message")
	}
	messagePayload := requireMap(t, storedMessage.Payload, "stored pointer-cycle message payload")
	storedMessageNode, ok := messagePayload["node"].(*cyclicNode)
	if !ok {
		t.Fatalf("expected stored message node to be *cyclicNode, got %T", messagePayload["node"])
	}
	if storedMessageNode.Value != "message-initial" {
		t.Fatalf("expected stored message node value to remain initial, got %q", storedMessageNode.Value)
	}
	if storedMessageNode.Next != storedMessageNode {
		t.Fatal("expected stored message node cycle to be preserved")
	}

	storedProposal, ok := hub.GetProposal(proposal.ID)
	if !ok {
		t.Fatal("expected stored proposal")
	}
	proposalValue := requireMap(t, storedProposal.Value, "stored pointer-cycle proposal value")
	storedProposalNode, ok := proposalValue["node"].(*cyclicNode)
	if !ok {
		t.Fatalf("expected stored proposal node to be *cyclicNode, got %T", proposalValue["node"])
	}
	if storedProposalNode.Value != "proposal-initial" {
		t.Fatalf("expected stored proposal node value to remain initial, got %q", storedProposalNode.Value)
	}
	if storedProposalNode.Next != storedProposalNode {
		t.Fatal("expected stored proposal node cycle to be preserved")
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
