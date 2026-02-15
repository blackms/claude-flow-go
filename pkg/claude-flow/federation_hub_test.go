package claudeflow

import (
	"context"
	"testing"
)

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

func TestNewFederationTools_AllowsNilHubAndFailsGracefully(t *testing.T) {
	fedTools := NewFederationTools(nil)
	if fedTools == nil {
		t.Fatal("expected federation tools wrapper even with nil hub")
	}

	result, err := fedTools.Execute(context.Background(), "federation/status", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected Execute to fail without configured hub")
	}
	if result == nil {
		t.Fatal("expected Execute result without configured hub")
	}
	if err.Error() != "federation hub is not configured" {
		t.Fatalf("expected configured-hub error, got %q", err.Error())
	}
	if result.Error != "federation hub is not configured" {
		t.Fatalf("expected result configured-hub error, got %q", result.Error)
	}
	if result.Success {
		t.Fatal("expected failed Execute result without configured hub")
	}
}

func TestFederationHub_PublicLifecycleReadsAvailableBeforeInitialize(t *testing.T) {
	hub := NewFederationHubWithDefaults()

	if swarms := hub.GetSwarms(); len(swarms) != 0 {
		t.Fatalf("expected zero swarms before initialize, got %d", len(swarms))
	}
	if agents := hub.GetAgents(); len(agents) != 0 {
		t.Fatalf("expected zero agents before initialize, got %d", len(agents))
	}
	if messages := hub.GetMessages(0); len(messages) != 0 {
		t.Fatalf("expected zero messages before initialize, got %d", len(messages))
	}
	if proposals := hub.GetProposals(); len(proposals) != 0 {
		t.Fatalf("expected zero proposals before initialize, got %d", len(proposals))
	}
	if events := hub.GetEvents(0); len(events) != 0 {
		t.Fatalf("expected zero events before initialize, got %d", len(events))
	}
}

func TestFederationHub_PublicZeroValueMethodsFailGracefully(t *testing.T) {
	var hub FederationHub

	if err := hub.Initialize(); err == nil || err.Error() != "federation hub is not configured" {
		t.Fatalf("expected initialize configuration error, got %v", err)
	}
	if err := hub.Shutdown(); err == nil || err.Error() != "federation hub is not configured" {
		t.Fatalf("expected shutdown configuration error, got %v", err)
	}
	if err := hub.RegisterSwarm(SwarmRegistration{SwarmID: "swarm", Name: "swarm", MaxAgents: 1}); err == nil || err.Error() != "federation hub is not configured" {
		t.Fatalf("expected register configuration error, got %v", err)
	}

	spawnResult, spawnErr := hub.SpawnEphemeralAgent(SpawnEphemeralOptions{Type: "coder", Task: "zero value"})
	if spawnErr == nil || spawnErr.Error() != "federation hub is not configured" {
		t.Fatalf("expected spawn configuration error, got %v", spawnErr)
	}
	if spawnResult == nil || spawnResult.Error != "federation hub is not configured" {
		t.Fatalf("expected spawn result configuration error, got %+v", spawnResult)
	}

	if swarms := hub.GetSwarms(); len(swarms) != 0 {
		t.Fatalf("expected zero-value swarms empty, got %d", len(swarms))
	}
	if agents := hub.GetAgents(); len(agents) != 0 {
		t.Fatalf("expected zero-value agents empty, got %d", len(agents))
	}
	if _, ok := hub.GetSwarm("swarm"); ok {
		t.Fatal("expected zero-value GetSwarm to fail")
	}
	if _, ok := hub.GetAgent("agent"); ok {
		t.Fatal("expected zero-value GetAgent to fail")
	}
}

func TestFederationHub_PublicNilReceiverMethodsFailGracefully(t *testing.T) {
	var hub *FederationHub

	if err := hub.Initialize(); err == nil || err.Error() != "federation hub is not configured" {
		t.Fatalf("expected nil initialize configuration error, got %v", err)
	}
	if err := hub.Shutdown(); err == nil || err.Error() != "federation hub is not configured" {
		t.Fatalf("expected nil shutdown configuration error, got %v", err)
	}

	spawnResult, spawnErr := hub.SpawnEphemeralAgent(SpawnEphemeralOptions{Type: "coder", Task: "nil receiver"})
	if spawnErr == nil || spawnErr.Error() != "federation hub is not configured" {
		t.Fatalf("expected nil spawn configuration error, got %v", spawnErr)
	}
	if spawnResult == nil || spawnResult.Error != "federation hub is not configured" {
		t.Fatalf("expected nil spawn result configuration error, got %+v", spawnResult)
	}

	if swarms := hub.GetSwarms(); len(swarms) != 0 {
		t.Fatalf("expected nil receiver swarms empty, got %d", len(swarms))
	}
	if agents := hub.GetAgents(); len(agents) != 0 {
		t.Fatalf("expected nil receiver agents empty, got %d", len(agents))
	}
	if _, ok := hub.GetSwarm("swarm"); ok {
		t.Fatal("expected nil receiver GetSwarm to fail")
	}
	if _, ok := hub.GetAgent("agent"); ok {
		t.Fatal("expected nil receiver GetAgent to fail")
	}
}
