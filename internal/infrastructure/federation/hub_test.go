package federation

import (
	"math"
	"strings"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestFederationHub_RegisterSwarmRejectsDuplicateIDs(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-dup",
		Name:      "First Swarm",
		MaxAgents: 4,
	}); err != nil {
		t.Fatalf("failed to register initial swarm: %v", err)
	}

	err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-dup",
		Name:      "Second Swarm",
		MaxAgents: 8,
	})
	if err == nil {
		t.Fatal("expected duplicate swarm registration to fail")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error to mention already exists, got %v", err)
	}

	swarm, ok := hub.GetSwarm("swarm-dup")
	if !ok {
		t.Fatal("expected original swarm to remain registered")
	}
	if swarm.Name != "First Swarm" {
		t.Fatalf("expected original swarm to remain intact, got name %q", swarm.Name)
	}

	stats := hub.GetStats()
	if stats.TotalSwarms != 1 {
		t.Fatalf("expected total swarms to remain 1 after duplicate attempt, got %d", stats.TotalSwarms)
	}
}

func TestFederationHub_SpawnEphemeralRejectsTTLOverflow(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-ttl-overflow",
		Name:      "TTL Overflow Swarm",
		MaxAgents: 4,
	}); err != nil {
		t.Fatalf("failed to register swarm: %v", err)
	}

	_, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-ttl-overflow",
		Type:    "coder",
		Task:    "overflow ttl",
		TTL:     math.MaxInt64,
	})
	if err == nil {
		t.Fatal("expected overflow ttl spawn to fail")
	}
	if !strings.Contains(err.Error(), "ttl is out of range") {
		t.Fatalf("expected ttl overflow error, got %v", err)
	}

	agents := hub.GetAgents()
	if len(agents) != 0 {
		t.Fatalf("expected no agents to be created on ttl overflow, got %d", len(agents))
	}
}
