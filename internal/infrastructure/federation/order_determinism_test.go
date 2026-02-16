package federation

import (
	"fmt"
	"sort"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestFederationHub_GettersReturnDeterministicOrdering(t *testing.T) {
	cfg := shared.DefaultFederationConfig()
	cfg.ConsensusQuorum = 1.0

	hub := NewFederationHub(cfg)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	registerTestSwarm(t, hub, "swarm-order-c", "Swarm C")
	registerTestSwarm(t, hub, "swarm-order-a", "Swarm A")
	registerTestSwarm(t, hub, "swarm-order-b", "Swarm B")

	swarms := hub.GetSwarms()
	assertSortedStrings(t, swarmIDs(swarms), "GetSwarms IDs")

	activeSwarms := hub.GetActiveSwarms()
	assertSortedStrings(t, swarmIDs(activeSwarms), "GetActiveSwarms IDs")

	spawnedAgents := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		spawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
			SwarmID: "swarm-order-a",
			Type:    "coder",
			Task:    fmt.Sprintf("order-task-%d", i),
		})
		if err != nil {
			t.Fatalf("failed to spawn agent %d: %v", i, err)
		}
		spawnedAgents = append(spawnedAgents, spawn.AgentID)
	}

	for _, agentID := range spawnedAgents {
		if err := hub.TerminateAgent(agentID, "order-check"); err != nil {
			t.Fatalf("failed to terminate agent %s: %v", agentID, err)
		}
	}

	allAgents := hub.GetAgents()
	assertSortedStrings(t, agentIDs(allAgents), "GetAgents IDs")

	agentsBySwarm := hub.GetAgentsBySwarm("swarm-order-a")
	assertSortedStrings(t, agentIDs(agentsBySwarm), "GetAgentsBySwarm IDs")

	terminatedAgents := hub.GetAgentsByStatus(shared.EphemeralStatusTerminated)
	assertSortedStrings(t, agentIDs(terminatedAgents), "GetAgentsByStatus IDs")

	for i := 0; i < 7; i++ {
		_, err := hub.Propose("swarm-order-a", "rollout", map[string]interface{}{
			"index": i,
		})
		if err != nil {
			t.Fatalf("failed to create proposal %d: %v", i, err)
		}
	}

	allProposals := hub.GetProposals()
	assertSortedStrings(t, proposalIDs(allProposals), "GetProposals IDs")

	pendingProposals := hub.GetPendingProposals()
	assertSortedStrings(t, proposalIDs(pendingProposals), "GetPendingProposals IDs")

	pendingByStatus := hub.GetProposalsByStatus(shared.FederationProposalPending)
	assertSortedStrings(t, proposalIDs(pendingByStatus), "GetProposalsByStatus IDs")
}

func assertSortedStrings(t *testing.T, values []string, context string) {
	t.Helper()
	if !sort.StringsAreSorted(values) {
		t.Fatalf("expected %s to be sorted, got %v", context, values)
	}
}

func swarmIDs(swarms []*shared.SwarmRegistration) []string {
	ids := make([]string, 0, len(swarms))
	for _, swarm := range swarms {
		if swarm == nil {
			continue
		}
		ids = append(ids, swarm.SwarmID)
	}
	return ids
}

func agentIDs(agents []*shared.EphemeralAgent) []string {
	ids := make([]string, 0, len(agents))
	for _, agent := range agents {
		if agent == nil {
			continue
		}
		ids = append(ids, agent.ID)
	}
	return ids
}

func proposalIDs(proposals []*shared.FederationProposal) []string {
	ids := make([]string, 0, len(proposals))
	for _, proposal := range proposals {
		if proposal == nil {
			continue
		}
		ids = append(ids, proposal.ID)
	}
	return ids
}
