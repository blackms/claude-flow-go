package federation

import (
	"math"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestFederationHub_MessagingOperationsTrimIdentifiersAndValidateInputs(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	registerTestSwarm(t, hub, "swarm-source-msg", "Source Swarm")
	registerTestSwarm(t, hub, "swarm-target-msg", "Target Swarm")

	directMsg, err := hub.SendMessage("  swarm-source-msg  ", "  swarm-target-msg  ", map[string]interface{}{"kind": "direct"})
	if err != nil {
		t.Fatalf("expected direct message with padded IDs to succeed, got %v", err)
	}
	if directMsg.SourceSwarmID != "swarm-source-msg" {
		t.Fatalf("expected trimmed direct source swarm ID, got %q", directMsg.SourceSwarmID)
	}
	if directMsg.TargetSwarmID != "swarm-target-msg" {
		t.Fatalf("expected trimmed direct target swarm ID, got %q", directMsg.TargetSwarmID)
	}

	broadcastMsg, err := hub.Broadcast("  swarm-source-msg  ", map[string]interface{}{"kind": "broadcast"})
	if err != nil {
		t.Fatalf("expected broadcast with padded source ID to succeed, got %v", err)
	}
	if broadcastMsg.SourceSwarmID != "swarm-source-msg" {
		t.Fatalf("expected trimmed broadcast source swarm ID, got %q", broadcastMsg.SourceSwarmID)
	}

	heartbeatMsg, err := hub.SendHeartbeat("  swarm-source-msg  ", "  swarm-target-msg  ")
	if err != nil {
		t.Fatalf("expected heartbeat with padded IDs to succeed, got %v", err)
	}
	if heartbeatMsg.SourceSwarmID != "swarm-source-msg" || heartbeatMsg.TargetSwarmID != "swarm-target-msg" {
		t.Fatalf("expected trimmed heartbeat IDs, got source=%q target=%q", heartbeatMsg.SourceSwarmID, heartbeatMsg.TargetSwarmID)
	}

	consensusMsg, err := hub.SendConsensusMessage("  swarm-source-msg  ", map[string]interface{}{"kind": "consensus"}, "  swarm-target-msg  ")
	if err != nil {
		t.Fatalf("expected consensus message with padded IDs to succeed, got %v", err)
	}
	if consensusMsg.SourceSwarmID != "swarm-source-msg" || consensusMsg.TargetSwarmID != "swarm-target-msg" {
		t.Fatalf("expected trimmed consensus IDs, got source=%q target=%q", consensusMsg.SourceSwarmID, consensusMsg.TargetSwarmID)
	}

	if _, ok := hub.GetMessage("  " + directMsg.ID + "  "); !ok {
		t.Fatal("expected GetMessage to resolve trimmed message identifier")
	}
	if _, ok := hub.GetMessage("   "); ok {
		t.Fatal("expected blank message identifier lookup to fail")
	}

	sourceMessages := hub.GetMessagesBySwarm("  swarm-source-msg  ", 0)
	if len(sourceMessages) != 4 {
		t.Fatalf("expected all source messages when limit=0, got %d", len(sourceMessages))
	}
	if blankSourceMessages := hub.GetMessagesBySwarm("   ", 10); len(blankSourceMessages) != 0 {
		t.Fatalf("expected blank swarm message lookup to return none, got %d", len(blankSourceMessages))
	}
	directMessages := hub.GetMessagesByType(shared.FederationMsgDirect, 0)
	if len(directMessages) != 1 {
		t.Fatalf("expected one direct message when limit=0, got %d", len(directMessages))
	}

	if _, err := hub.SendMessage("   ", "swarm-target-msg", map[string]interface{}{"kind": "direct"}); err == nil || err.Error() != "sourceSwarmId is required" {
		t.Fatalf("expected blank source swarm validation error, got %v", err)
	}
	if _, err := hub.SendMessage("swarm-source-msg", "   ", map[string]interface{}{"kind": "direct"}); err == nil || err.Error() != "targetSwarmId is required" {
		t.Fatalf("expected blank target swarm validation error, got %v", err)
	}
	if _, err := hub.SendMessage("swarm-source-msg", "swarm-target-msg", nil); err == nil || err.Error() != "payload is required" {
		t.Fatalf("expected nil direct payload validation error, got %v", err)
	}
	if _, err := hub.Broadcast("   ", map[string]interface{}{"kind": "broadcast"}); err == nil || err.Error() != "sourceSwarmId is required" {
		t.Fatalf("expected blank source broadcast validation error, got %v", err)
	}
	if _, err := hub.Broadcast("swarm-source-msg", nil); err == nil || err.Error() != "payload is required" {
		t.Fatalf("expected nil broadcast payload validation error, got %v", err)
	}
	if _, err := hub.SendHeartbeat("   ", "swarm-target-msg"); err == nil || err.Error() != "sourceSwarmId is required" {
		t.Fatalf("expected blank source heartbeat validation error, got %v", err)
	}
	if _, err := hub.SendHeartbeat("swarm-source-msg", "   "); err == nil || err.Error() != "targetSwarmId is required" {
		t.Fatalf("expected blank target heartbeat validation error, got %v", err)
	}
	if _, err := hub.SendConsensusMessage("   ", map[string]interface{}{"kind": "consensus"}, "swarm-target-msg"); err == nil || err.Error() != "sourceSwarmId is required" {
		t.Fatalf("expected blank source consensus validation error, got %v", err)
	}
	if _, err := hub.SendConsensusMessage("swarm-source-msg", nil, "swarm-target-msg"); err == nil || err.Error() != "payload is required" {
		t.Fatalf("expected nil consensus payload validation error, got %v", err)
	}
}

func TestFederationHub_ConsensusOperationsTrimIdentifiersAndValidateInputs(t *testing.T) {
	cfg := shared.DefaultFederationConfig()
	cfg.ConsensusQuorum = 1.0
	hub := NewFederationHub(cfg)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	registerTestSwarm(t, hub, "swarm-proposer-consensus", "Consensus Proposer")
	registerTestSwarm(t, hub, "swarm-voter-consensus", "Consensus Voter")

	proposal, err := hub.Propose("  swarm-proposer-consensus  ", "  policy-update  ", map[string]interface{}{"value": "on"})
	if err != nil {
		t.Fatalf("expected propose with padded identifiers to succeed, got %v", err)
	}
	if proposal.ProposerID != "swarm-proposer-consensus" {
		t.Fatalf("expected trimmed proposer ID, got %q", proposal.ProposerID)
	}
	if proposal.Type != "policy-update" {
		t.Fatalf("expected trimmed proposal type, got %q", proposal.Type)
	}
	if proposal.Status != shared.FederationProposalPending {
		t.Fatalf("expected proposal pending prior to second vote, got %q", proposal.Status)
	}

	if err := hub.Vote("  swarm-voter-consensus  ", "  "+proposal.ID+"  ", true); err != nil {
		t.Fatalf("expected vote with padded identifiers to succeed, got %v", err)
	}

	votedProposal, ok := hub.GetProposal("  " + proposal.ID + "  ")
	if !ok {
		t.Fatal("expected GetProposal to resolve trimmed proposal identifier")
	}
	if votedProposal.Status != shared.FederationProposalAccepted {
		t.Fatalf("expected proposal to be accepted after second vote, got %q", votedProposal.Status)
	}

	approvals, rejections, err := hub.GetProposalVotes("  " + proposal.ID + "  ")
	if err != nil {
		t.Fatalf("expected trimmed proposal vote lookup to succeed, got %v", err)
	}
	if approvals != 2 || rejections != 0 {
		t.Fatalf("expected vote tally approvals=2 rejections=0, got approvals=%d rejections=%d", approvals, rejections)
	}

	if _, err := hub.Propose("   ", "policy-update", map[string]interface{}{"value": "on"}); err == nil || err.Error() != "proposerId is required" {
		t.Fatalf("expected blank proposer validation error, got %v", err)
	}
	if _, err := hub.Propose("swarm-proposer-consensus", "   ", map[string]interface{}{"value": "on"}); err == nil || err.Error() != "proposalType is required" {
		t.Fatalf("expected blank proposalType validation error, got %v", err)
	}
	if _, err := hub.Propose("swarm-proposer-consensus", "policy-update", nil); err == nil || err.Error() != "value is required" {
		t.Fatalf("expected nil proposal value validation error, got %v", err)
	}
	if err := hub.Vote("   ", proposal.ID, true); err == nil || err.Error() != "voterId is required" {
		t.Fatalf("expected blank voter validation error, got %v", err)
	}
	if err := hub.Vote("swarm-voter-consensus", "   ", true); err == nil || err.Error() != "proposalId is required" {
		t.Fatalf("expected blank proposalId validation error, got %v", err)
	}
	if _, ok := hub.GetProposal("   "); ok {
		t.Fatal("expected blank proposal lookup to fail")
	}
	if _, _, err := hub.GetProposalVotes("   "); err == nil || err.Error() != "proposalId is required" {
		t.Fatalf("expected blank proposalId votes validation error, got %v", err)
	}
}

func TestFederationHub_ProposeRejectsInvalidProposalTimeoutConfiguration(t *testing.T) {
	zeroTimeoutCfg := shared.DefaultFederationConfig()
	zeroTimeoutCfg.ProposalTimeout = 0
	zeroTimeoutHub := NewFederationHub(zeroTimeoutCfg)
	if err := zeroTimeoutHub.Initialize(); err != nil {
		t.Fatalf("failed to initialize zero-timeout hub: %v", err)
	}
	t.Cleanup(func() {
		_ = zeroTimeoutHub.Shutdown()
	})

	if _, err := zeroTimeoutHub.Propose("any-proposer", "policy-update", map[string]interface{}{"value": "on"}); err == nil || err.Error() != "proposal timeout must be greater than 0" {
		t.Fatalf("expected non-positive proposal timeout validation error, got %v", err)
	}

	overflowCfg := shared.DefaultFederationConfig()
	overflowCfg.ProposalTimeout = math.MaxInt64
	overflowHub := NewFederationHub(overflowCfg)
	if err := overflowHub.Initialize(); err != nil {
		t.Fatalf("failed to initialize overflow-timeout hub: %v", err)
	}
	t.Cleanup(func() {
		_ = overflowHub.Shutdown()
	})

	registerTestSwarm(t, overflowHub, "overflow-proposer", "Overflow Proposer")
	if _, err := overflowHub.Propose("overflow-proposer", "policy-update", map[string]interface{}{"value": "on"}); err == nil || err.Error() != "proposal timeout is out of range" {
		t.Fatalf("expected proposal timeout overflow validation error, got %v", err)
	}
}

func registerTestSwarm(t *testing.T, hub *FederationHub, swarmID, name string) {
	t.Helper()

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   swarmID,
		Name:      name,
		MaxAgents: 4,
	}); err != nil {
		t.Fatalf("failed to register test swarm %q: %v", swarmID, err)
	}
}
