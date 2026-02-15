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

func TestFederationHub_SpawnEphemeralRejectsOverflowDefaultTTL(t *testing.T) {
	config := shared.DefaultFederationConfig()
	config.DefaultAgentTTL = math.MaxInt64

	hub := NewFederationHub(config)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-default-ttl-overflow",
		Name:      "Default TTL Overflow Swarm",
		MaxAgents: 4,
	}); err != nil {
		t.Fatalf("failed to register swarm: %v", err)
	}

	_, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-default-ttl-overflow",
		Type:    "coder",
		Task:    "overflow via default ttl",
		TTL:     0, // trigger default TTL
	})
	if err == nil {
		t.Fatal("expected spawn to fail when default ttl overflows")
	}
	if !strings.Contains(err.Error(), "ttl is out of range") {
		t.Fatalf("expected ttl overflow error, got %v", err)
	}
	if agents := hub.GetAgents(); len(agents) != 0 {
		t.Fatalf("expected no agents to be created on default ttl overflow, got %d", len(agents))
	}
}

func TestFederationHub_RegisterSwarmRejectsInvalidInputs(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	tests := []struct {
		name         string
		registration shared.SwarmRegistration
		expectedErr  string
	}{
		{
			name: "blank swarm id",
			registration: shared.SwarmRegistration{
				SwarmID:   "   ",
				Name:      "blank-id",
				MaxAgents: 1,
			},
			expectedErr: "swarmId is required",
		},
		{
			name: "non-positive maxAgents",
			registration: shared.SwarmRegistration{
				SwarmID:   "swarm-invalid-capacity",
				Name:      "invalid-capacity",
				MaxAgents: 0,
			},
			expectedErr: "maxAgents must be greater than 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := hub.RegisterSwarm(tc.registration)
			if err == nil {
				t.Fatalf("expected registration error %q", tc.expectedErr)
			}
			if err.Error() != tc.expectedErr {
				t.Fatalf("expected error %q, got %q", tc.expectedErr, err.Error())
			}
		})
	}

	stats := hub.GetStats()
	if stats.TotalSwarms != 0 {
		t.Fatalf("expected no swarms to be registered after invalid inputs, got %d", stats.TotalSwarms)
	}
}

func TestFederationHub_RegisterSwarmTrimsFields(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "  swarm-trim  ",
		Name:      "  Trimmed Name  ",
		Endpoint:  "  http://example.local  ",
		MaxAgents: 3,
	})
	if err != nil {
		t.Fatalf("expected trimmed registration to succeed, got %v", err)
	}

	swarm, ok := hub.GetSwarm("swarm-trim")
	if !ok {
		t.Fatal("expected trimmed swarm ID to be used as key")
	}
	if swarm.Name != "Trimmed Name" {
		t.Fatalf("expected trimmed swarm name, got %q", swarm.Name)
	}
	if swarm.Endpoint != "http://example.local" {
		t.Fatalf("expected trimmed endpoint, got %q", swarm.Endpoint)
	}
}
