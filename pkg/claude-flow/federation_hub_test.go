package claudeflow

import "testing"

func TestFederationHub_PublicLifecycleInitializeGuards(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if hub == nil {
		t.Fatal("expected federation hub wrapper")
	}

	if err := hub.Initialize(); err != nil {
		t.Fatalf("expected first initialize to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	err := hub.Initialize()
	if err == nil {
		t.Fatal("expected second initialize to fail")
	}
	if err.Error() != "federation hub is already initialized" {
		t.Fatalf("expected already initialized error, got %q", err.Error())
	}
}

func TestFederationHub_PublicLifecycleShutdownBlocksMutationsButKeepsReads(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}

	if err := hub.RegisterSwarm(SwarmRegistration{
		SwarmID:   "public-shutdown-source",
		Name:      "Public Shutdown Source",
		MaxAgents: 2,
	}); err != nil {
		t.Fatalf("failed to register source swarm: %v", err)
	}
	if err := hub.RegisterSwarm(SwarmRegistration{
		SwarmID:   "public-shutdown-target",
		Name:      "Public Shutdown Target",
		MaxAgents: 2,
	}); err != nil {
		t.Fatalf("failed to register target swarm: %v", err)
	}

	spawn, err := hub.SpawnEphemeralAgent(SpawnEphemeralOptions{
		SwarmID: "public-shutdown-source",
		Type:    "coder",
		Task:    "public wrapper shutdown",
	})
	if err != nil {
		t.Fatalf("failed to spawn agent: %v", err)
	}

	if err := hub.Shutdown(); err != nil {
		t.Fatalf("failed to shutdown federation hub: %v", err)
	}

	err = hub.RegisterSwarm(SwarmRegistration{
		SwarmID:   "public-shutdown-blocked",
		Name:      "Public Shutdown Blocked",
		MaxAgents: 1,
	})
	if err == nil {
		t.Fatal("expected register swarm to fail after shutdown")
	}
	if err.Error() != "federation hub is shut down" {
		t.Fatalf("expected shutdown lifecycle error, got %q", err.Error())
	}

	agents := hub.GetAgents()
	if len(agents) != 1 {
		t.Fatalf("expected one historical agent after shutdown, got %d", len(agents))
	}
	if agents[0].ID != spawn.AgentID {
		t.Fatalf("expected historical agent ID %s, got %s", spawn.AgentID, agents[0].ID)
	}
	if agents[0].Status != EphemeralStatusTerminated {
		t.Fatalf("expected historical agent to be terminated, got %q", agents[0].Status)
	}

	swarms := hub.GetSwarms()
	if len(swarms) != 2 {
		t.Fatalf("expected two historical swarms after shutdown, got %d", len(swarms))
	}
}

func TestFederationHub_PublicLifecycleMutationsRequireInitialize(t *testing.T) {
	hub := NewFederationHubWithDefaults()

	err := hub.RegisterSwarm(SwarmRegistration{
		SwarmID:   "preinit-public-swarm",
		Name:      "Preinit Public Swarm",
		MaxAgents: 1,
	})
	if err == nil {
		t.Fatal("expected register swarm to fail before initialize")
	}
	if err.Error() != "federation hub is not initialized" {
		t.Fatalf("expected pre-init lifecycle error, got %q", err.Error())
	}

	spawnResult, spawnErr := hub.SpawnEphemeralAgent(SpawnEphemeralOptions{
		Type: "coder",
		Task: "preinit-public-spawn",
	})
	if spawnErr == nil {
		t.Fatal("expected spawn to fail before initialize")
	}
	if spawnErr.Error() != "federation hub is not initialized" {
		t.Fatalf("expected pre-init lifecycle error, got %q", spawnErr.Error())
	}
	if spawnResult == nil || spawnResult.Error != "federation hub is not initialized" {
		t.Fatalf("expected spawn result pre-init error, got %+v", spawnResult)
	}
}
