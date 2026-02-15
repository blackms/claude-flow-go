package federation

import (
	"reflect"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func assertNotConfiguredError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected not-configured error")
	}
	if err.Error() != "federation hub is not configured" {
		t.Fatalf("expected not-configured error, got %q", err.Error())
	}
}

func TestFederationHub_ZeroValueMethodsFailGracefully(t *testing.T) {
	var hub FederationHub

	assertNotConfiguredError(t, hub.Initialize())
	assertNotConfiguredError(t, hub.Shutdown())
	assertNotConfiguredError(t, hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm", Name: "swarm", MaxAgents: 1}))
	assertNotConfiguredError(t, hub.UnregisterSwarm("swarm"))
	assertNotConfiguredError(t, hub.Heartbeat("swarm"))

	spawnResult, spawnErr := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{Type: "coder", Task: "zero"})
	assertNotConfiguredError(t, spawnErr)
	if spawnResult == nil || spawnResult.Error != "federation hub is not configured" {
		t.Fatalf("expected spawn not-configured result, got %+v", spawnResult)
	}

	assertNotConfiguredError(t, hub.CompleteAgent("agent", map[string]interface{}{"ok": true}))
	assertNotConfiguredError(t, hub.TerminateAgent("agent", "error"))

	if _, err := hub.SendMessage("source", "target", map[string]interface{}{"k": "v"}); err == nil {
		t.Fatal("expected send-message not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}
	if _, err := hub.Broadcast("source", map[string]interface{}{"k": "v"}); err == nil {
		t.Fatal("expected broadcast not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}
	if _, err := hub.SendHeartbeat("source", "target"); err == nil {
		t.Fatal("expected send-heartbeat not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}
	if _, err := hub.SendConsensusMessage("source", map[string]interface{}{"k": "v"}, "target"); err == nil {
		t.Fatal("expected send-consensus not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}

	if _, err := hub.Propose("source", "proposal", map[string]interface{}{"k": "v"}); err == nil {
		t.Fatal("expected propose not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}
	assertNotConfiguredError(t, hub.Vote("source", "proposal", true))

	if _, _, err := hub.GetProposalVotes("proposal"); err == nil {
		t.Fatal("expected proposal-votes not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}

	if _, ok := hub.GetSwarm("swarm"); ok {
		t.Fatal("expected GetSwarm false on zero-value hub")
	}
	if swarms := hub.GetSwarms(); len(swarms) != 0 {
		t.Fatalf("expected GetSwarms empty, got %d", len(swarms))
	}
	if active := hub.GetActiveSwarms(); len(active) != 0 {
		t.Fatalf("expected GetActiveSwarms empty, got %d", len(active))
	}
	if _, ok := hub.GetAgent("agent"); ok {
		t.Fatal("expected GetAgent false on zero-value hub")
	}
	if agents := hub.GetAgents(); len(agents) != 0 {
		t.Fatalf("expected GetAgents empty, got %d", len(agents))
	}
	if activeAgents := hub.GetActiveAgents(); len(activeAgents) != 0 {
		t.Fatalf("expected GetActiveAgents empty, got %d", len(activeAgents))
	}
	if bySwarm := hub.GetAgentsBySwarm("swarm"); len(bySwarm) != 0 {
		t.Fatalf("expected GetAgentsBySwarm empty, got %d", len(bySwarm))
	}
	if byStatus := hub.GetAgentsByStatus(shared.EphemeralStatusActive); len(byStatus) != 0 {
		t.Fatalf("expected GetAgentsByStatus empty, got %d", len(byStatus))
	}
	if messages := hub.GetMessages(10); len(messages) != 0 {
		t.Fatalf("expected GetMessages empty, got %d", len(messages))
	}
	if bySwarm := hub.GetMessagesBySwarm("swarm", 10); len(bySwarm) != 0 {
		t.Fatalf("expected GetMessagesBySwarm empty, got %d", len(bySwarm))
	}
	if byType := hub.GetMessagesByType(shared.FederationMsgDirect, 10); len(byType) != 0 {
		t.Fatalf("expected GetMessagesByType empty, got %d", len(byType))
	}
	if _, ok := hub.GetMessage("msg"); ok {
		t.Fatal("expected GetMessage false on zero-value hub")
	}
	if _, ok := hub.GetProposal("proposal"); ok {
		t.Fatal("expected GetProposal false on zero-value hub")
	}
	if proposals := hub.GetProposals(); len(proposals) != 0 {
		t.Fatalf("expected GetProposals empty, got %d", len(proposals))
	}
	if pending := hub.GetPendingProposals(); len(pending) != 0 {
		t.Fatalf("expected GetPendingProposals empty, got %d", len(pending))
	}
	if byStatus := hub.GetProposalsByStatus(shared.FederationProposalPending); len(byStatus) != 0 {
		t.Fatalf("expected GetProposalsByStatus empty, got %d", len(byStatus))
	}

	if activeSwarms, requiredVotes, quorum := hub.GetQuorumInfo(); activeSwarms != 0 || requiredVotes != 0 || quorum != 0 {
		t.Fatalf("expected zero quorum info, got active=%d required=%d quorum=%v", activeSwarms, requiredVotes, quorum)
	}
	if events := hub.GetEvents(10); len(events) != 0 {
		t.Fatalf("expected GetEvents empty, got %d", len(events))
	}
	if stats := hub.GetStats(); stats != (shared.FederationStats{}) {
		t.Fatalf("expected zero stats, got %+v", stats)
	}
	expectedConfig := shared.DefaultFederationConfig()
	if config := hub.GetConfig(); !reflect.DeepEqual(config, expectedConfig) {
		t.Fatalf("expected default config %+v, got %+v", expectedConfig, config)
	}

	// no-op path should not panic
	hub.SetEventHandler(func(event shared.FederationEvent) {})
}

func TestFederationHub_NilReceiverMethodsFailGracefully(t *testing.T) {
	var hub *FederationHub

	assertNotConfiguredError(t, hub.Initialize())
	assertNotConfiguredError(t, hub.Shutdown())
	assertNotConfiguredError(t, hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm", Name: "swarm", MaxAgents: 1}))
	assertNotConfiguredError(t, hub.UnregisterSwarm("swarm"))
	assertNotConfiguredError(t, hub.Heartbeat("swarm"))

	spawnResult, spawnErr := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{Type: "coder", Task: "nil"})
	assertNotConfiguredError(t, spawnErr)
	if spawnResult == nil || spawnResult.Error != "federation hub is not configured" {
		t.Fatalf("expected spawn not-configured result, got %+v", spawnResult)
	}

	assertNotConfiguredError(t, hub.CompleteAgent("agent", map[string]interface{}{"ok": true}))
	assertNotConfiguredError(t, hub.TerminateAgent("agent", "error"))
	assertNotConfiguredError(t, hub.Vote("source", "proposal", true))

	if _, err := hub.SendMessage("source", "target", map[string]interface{}{"k": "v"}); err == nil {
		t.Fatal("expected send-message not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}
	if _, err := hub.Broadcast("source", map[string]interface{}{"k": "v"}); err == nil {
		t.Fatal("expected broadcast not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}
	if _, err := hub.SendHeartbeat("source", "target"); err == nil {
		t.Fatal("expected send-heartbeat not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}
	if _, err := hub.SendConsensusMessage("source", map[string]interface{}{"k": "v"}, "target"); err == nil {
		t.Fatal("expected send-consensus not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}
	if _, err := hub.Propose("source", "proposal", map[string]interface{}{"k": "v"}); err == nil {
		t.Fatal("expected propose not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}

	if _, _, err := hub.GetProposalVotes("proposal"); err == nil {
		t.Fatal("expected proposal-votes not-configured error")
	} else {
		assertNotConfiguredError(t, err)
	}

	if _, ok := hub.GetSwarm("swarm"); ok {
		t.Fatal("expected nil receiver GetSwarm false")
	}
	if _, ok := hub.GetAgent("agent"); ok {
		t.Fatal("expected nil receiver GetAgent false")
	}
	if _, ok := hub.GetMessage("msg"); ok {
		t.Fatal("expected nil receiver GetMessage false")
	}
	if _, ok := hub.GetProposal("proposal"); ok {
		t.Fatal("expected nil receiver GetProposal false")
	}

	if len(hub.GetSwarms()) != 0 || len(hub.GetActiveSwarms()) != 0 ||
		len(hub.GetAgents()) != 0 || len(hub.GetActiveAgents()) != 0 ||
		len(hub.GetAgentsBySwarm("swarm")) != 0 || len(hub.GetAgentsByStatus(shared.EphemeralStatusActive)) != 0 ||
		len(hub.GetMessages(10)) != 0 || len(hub.GetMessagesBySwarm("swarm", 10)) != 0 ||
		len(hub.GetMessagesByType(shared.FederationMsgDirect, 10)) != 0 ||
		len(hub.GetProposals()) != 0 || len(hub.GetPendingProposals()) != 0 ||
		len(hub.GetProposalsByStatus(shared.FederationProposalPending)) != 0 ||
		len(hub.GetEvents(10)) != 0 {
		t.Fatal("expected nil receiver collection getters to return empty slices")
	}

	if activeSwarms, requiredVotes, quorum := hub.GetQuorumInfo(); activeSwarms != 0 || requiredVotes != 0 || quorum != 0 {
		t.Fatalf("expected nil receiver quorum info zeros, got active=%d required=%d quorum=%v", activeSwarms, requiredVotes, quorum)
	}
	if stats := hub.GetStats(); stats != (shared.FederationStats{}) {
		t.Fatalf("expected nil receiver stats zero, got %+v", stats)
	}
	expectedConfig := shared.DefaultFederationConfig()
	if config := hub.GetConfig(); !reflect.DeepEqual(config, expectedConfig) {
		t.Fatalf("expected nil receiver default config %+v, got %+v", expectedConfig, config)
	}

	// no-op path should not panic
	hub.SetEventHandler(func(event shared.FederationEvent) {})
}

func TestFederationHub_IsConfigured(t *testing.T) {
	var nilHub *FederationHub
	if nilHub.IsConfigured() {
		t.Fatal("expected nil receiver to be unconfigured")
	}

	var zero FederationHub
	if zero.IsConfigured() {
		t.Fatal("expected zero-value hub to be unconfigured")
	}

	constructed := NewFederationHubWithDefaults()
	if !constructed.IsConfigured() {
		t.Fatal("expected constructor-created hub to be configured")
	}

	if err := constructed.Initialize(); err != nil {
		t.Fatalf("expected constructed hub to initialize, got %v", err)
	}
	if !constructed.IsConfigured() {
		t.Fatal("expected initialized hub to remain configured")
	}

	if err := constructed.Shutdown(); err != nil {
		t.Fatalf("expected shutdown to succeed, got %v", err)
	}
	if !constructed.IsConfigured() {
		t.Fatal("expected shutdown hub to remain structurally configured")
	}
}
