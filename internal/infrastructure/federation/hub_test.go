package federation

import (
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	if stats.ActiveSwarms != 1 {
		t.Fatalf("expected active swarms to remain 1 after duplicate attempt, got %d", stats.ActiveSwarms)
	}
}

func TestFederationHub_InitializeRejectsInvalidIntervals(t *testing.T) {
	tests := []struct {
		name        string
		configure   func(cfg *shared.FederationConfig)
		expectedErr string
	}{
		{
			name: "non-positive heartbeat interval",
			configure: func(cfg *shared.FederationConfig) {
				cfg.HeartbeatInterval = 0
			},
			expectedErr: "heartbeat interval must be greater than 0",
		},
		{
			name: "heartbeat interval out of range",
			configure: func(cfg *shared.FederationConfig) {
				cfg.HeartbeatInterval = math.MaxInt64
			},
			expectedErr: "heartbeat interval is out of range",
		},
		{
			name: "non-positive sync interval",
			configure: func(cfg *shared.FederationConfig) {
				cfg.SyncInterval = 0
			},
			expectedErr: "sync interval must be greater than 0",
		},
		{
			name: "sync interval out of range",
			configure: func(cfg *shared.FederationConfig) {
				cfg.SyncInterval = math.MaxInt64
			},
			expectedErr: "sync interval is out of range",
		},
		{
			name: "cleanup enabled with non-positive cleanup interval",
			configure: func(cfg *shared.FederationConfig) {
				cfg.AutoCleanupEnabled = true
				cfg.CleanupInterval = 0
			},
			expectedErr: "cleanup interval must be greater than 0",
		},
		{
			name: "cleanup enabled with out-of-range cleanup interval",
			configure: func(cfg *shared.FederationConfig) {
				cfg.AutoCleanupEnabled = true
				cfg.CleanupInterval = math.MaxInt64
			},
			expectedErr: "cleanup interval is out of range",
		},
		{
			name: "non-positive max message history",
			configure: func(cfg *shared.FederationConfig) {
				cfg.MaxMessageHistory = 0
			},
			expectedErr: "max message history must be greater than 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := shared.DefaultFederationConfig()
			tc.configure(&cfg)

			hub := NewFederationHub(cfg)
			err := hub.Initialize()
			if err == nil {
				t.Fatalf("expected initialization error %q", tc.expectedErr)
			}
			if err.Error() != tc.expectedErr {
				t.Fatalf("expected error %q, got %q", tc.expectedErr, err.Error())
			}
		})
	}
}

func TestFederationHub_InitializeAllowsDisabledCleanupWithNonPositiveCleanupInterval(t *testing.T) {
	cfg := shared.DefaultFederationConfig()
	cfg.AutoCleanupEnabled = false
	cfg.CleanupInterval = 0

	hub := NewFederationHub(cfg)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("expected initialization to succeed when cleanup is disabled, got %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})
}

func TestFederationHub_InitializeRejectsAlreadyInitialized(t *testing.T) {
	hub := NewFederationHubWithDefaults()
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

func TestFederationHub_ShutdownIsIdempotent(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}

	if err := hub.Shutdown(); err != nil {
		t.Fatalf("expected first shutdown to succeed, got %v", err)
	}
	if err := hub.Shutdown(); err != nil {
		t.Fatalf("expected second shutdown to succeed, got %v", err)
	}
}

func TestFederationHub_InitializeRejectsAfterShutdown(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	if err := hub.Shutdown(); err != nil {
		t.Fatalf("failed to shutdown federation hub: %v", err)
	}

	err := hub.Initialize()
	if err == nil {
		t.Fatal("expected initialize after shutdown to fail")
	}
	if err.Error() != "federation hub is shut down" {
		t.Fatalf("expected shutdown lifecycle error, got %q", err.Error())
	}
}

func TestFederationHub_InitializeRejectsWhenShutdownBeforeStart(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Shutdown(); err != nil {
		t.Fatalf("shutdown before initialize should succeed, got %v", err)
	}

	err := hub.Initialize()
	if err == nil {
		t.Fatal("expected initialize to fail after pre-start shutdown")
	}
	if err.Error() != "federation hub is shut down" {
		t.Fatalf("expected shutdown lifecycle error, got %q", err.Error())
	}
}

func TestFederationHub_SetEventHandlerSupportsUpdateAndClear(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	var handled atomic.Int64
	hub.SetEventHandler(func(event shared.FederationEvent) {
		handled.Add(1)
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "event-handler-swarm-a",
		Name:      "Event Handler A",
		MaxAgents: 2,
	}); err != nil {
		t.Fatalf("failed to register first swarm: %v", err)
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for handled.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if handled.Load() < 1 {
		t.Fatalf("expected event handler to be invoked at least once, got %d", handled.Load())
	}

	hub.SetEventHandler(nil)

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "event-handler-swarm-b",
		Name:      "Event Handler B",
		MaxAgents: 2,
	}); err != nil {
		t.Fatalf("failed to register second swarm: %v", err)
	}

	time.Sleep(30 * time.Millisecond)
	if handled.Load() != 1 {
		t.Fatalf("expected cleared event handler to prevent additional callbacks, got %d invocations", handled.Load())
	}
}

func TestFederationHub_MutatingOperationsRejectAfterShutdown(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	if err := hub.Shutdown(); err != nil {
		t.Fatalf("failed to shutdown federation hub: %v", err)
	}

	const expectedErr = "federation hub is shut down"

	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm", Name: "swarm", MaxAgents: 1}); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected register swarm shutdown error %q, got %v", expectedErr, err)
	}
	if err := hub.UnregisterSwarm("swarm"); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected unregister swarm shutdown error %q, got %v", expectedErr, err)
	}
	if err := hub.Heartbeat("swarm"); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected heartbeat shutdown error %q, got %v", expectedErr, err)
	}

	spawnResult, spawnErr := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		Type: "coder",
		Task: "shutdown-check",
	})
	if spawnErr == nil || spawnErr.Error() != expectedErr {
		t.Fatalf("expected spawn shutdown error %q, got %v", expectedErr, spawnErr)
	}
	if spawnResult == nil || spawnResult.Error != expectedErr {
		t.Fatalf("expected spawn result shutdown error %q, got %+v", expectedErr, spawnResult)
	}

	if err := hub.CompleteAgent("agent-id", nil); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected complete agent shutdown error %q, got %v", expectedErr, err)
	}
	if err := hub.TerminateAgent("agent-id", ""); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected terminate agent shutdown error %q, got %v", expectedErr, err)
	}

	if _, err := hub.SendMessage("source", "target", map[string]interface{}{"ok": true}); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected send message shutdown error %q, got %v", expectedErr, err)
	}
	if _, err := hub.Broadcast("source", map[string]interface{}{"ok": true}); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected broadcast shutdown error %q, got %v", expectedErr, err)
	}
	if _, err := hub.SendHeartbeat("source", "target"); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected send heartbeat shutdown error %q, got %v", expectedErr, err)
	}
	if _, err := hub.SendConsensusMessage("source", map[string]interface{}{"ok": true}, "target"); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected send consensus message shutdown error %q, got %v", expectedErr, err)
	}

	if _, err := hub.Propose("source", "proposal", map[string]interface{}{"ok": true}); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected propose shutdown error %q, got %v", expectedErr, err)
	}
	if err := hub.Vote("source", "proposal", true); err == nil || err.Error() != expectedErr {
		t.Fatalf("expected vote shutdown error %q, got %v", expectedErr, err)
	}
}

func TestFederationHub_RegisterSwarmRejectsTrimmedDuplicateIDs(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-dup-trim",
		Name:      "First Trimmed Swarm",
		MaxAgents: 3,
	}); err != nil {
		t.Fatalf("failed to register initial swarm: %v", err)
	}

	err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "  swarm-dup-trim  ",
		Name:      "Second Trimmed Swarm",
		MaxAgents: 5,
	})
	if err == nil {
		t.Fatal("expected duplicate trimmed swarm registration to fail")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error to mention already exists, got %v", err)
	}

	swarm, ok := hub.GetSwarm("swarm-dup-trim")
	if !ok {
		t.Fatal("expected original trimmed swarm to remain registered")
	}
	if swarm.Name != "First Trimmed Swarm" {
		t.Fatalf("expected original trimmed swarm name to remain intact, got %q", swarm.Name)
	}

	stats := hub.GetStats()
	if stats.TotalSwarms != 1 {
		t.Fatalf("expected total swarms to remain 1 after duplicate trimmed attempt, got %d", stats.TotalSwarms)
	}
	if stats.ActiveSwarms != 1 {
		t.Fatalf("expected active swarms to remain 1 after duplicate trimmed attempt, got %d", stats.ActiveSwarms)
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

func TestFederationHub_SpawnEphemeralRejectsNonPositiveDefaultTTL(t *testing.T) {
	config := shared.DefaultFederationConfig()
	config.DefaultAgentTTL = 0

	hub := NewFederationHub(config)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-default-ttl-non-positive",
		Name:      "Default TTL Non Positive Swarm",
		MaxAgents: 3,
	}); err != nil {
		t.Fatalf("failed to register swarm: %v", err)
	}

	_, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-default-ttl-non-positive",
		Type:    "coder",
		Task:    "invalid default ttl",
		TTL:     0, // trigger default TTL
	})
	if err == nil {
		t.Fatal("expected spawn to fail when default ttl is non-positive")
	}
	if err.Error() != "ttl must be greater than 0" {
		t.Fatalf("expected non-positive ttl error %q, got %q", "ttl must be greater than 0", err.Error())
	}
	if agents := hub.GetAgents(); len(agents) != 0 {
		t.Fatalf("expected no agents to be created when default ttl is non-positive, got %d", len(agents))
	}
}

func TestFederationHub_SpawnEphemeralRejectsBlankTypeOrTask(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "spawn-validation-swarm",
		Name:      "Spawn Validation Swarm",
		MaxAgents: 3,
	}); err != nil {
		t.Fatalf("failed to register swarm: %v", err)
	}

	tests := []struct {
		name      string
		opts      shared.SpawnEphemeralOptions
		expected  string
	}{
		{
			name: "blank type",
			opts: shared.SpawnEphemeralOptions{
				SwarmID: "spawn-validation-swarm",
				Type:    "   ",
				Task:    "task",
			},
			expected: "type is required",
		},
		{
			name: "blank task",
			opts: shared.SpawnEphemeralOptions{
				SwarmID: "spawn-validation-swarm",
				Type:    "coder",
				Task:    "   ",
			},
			expected: "task is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := hub.SpawnEphemeralAgent(tc.opts)
			if err == nil {
				t.Fatalf("expected spawn validation error %q", tc.expected)
			}
			if err.Error() != tc.expected {
				t.Fatalf("expected error %q, got %q", tc.expected, err.Error())
			}
		})
	}

	if agents := hub.GetAgents(); len(agents) != 0 {
		t.Fatalf("expected no agents to be created on blank type/task, got %d", len(agents))
	}
}

func TestFederationHub_SpawnEphemeralTrimsStringInputs(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "spawn-trim-swarm",
		Name:      "Spawn Trim Swarm",
		MaxAgents: 3,
	}); err != nil {
		t.Fatalf("failed to register swarm: %v", err)
	}

	result, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "  spawn-trim-swarm  ",
		Type:    "  coder  ",
		Task:    "  implement feature  ",
		TTL:     1234,
	})
	if err != nil {
		t.Fatalf("expected spawn with padded fields to succeed, got %v", err)
	}
	if result == nil {
		t.Fatal("expected spawn result")
	}

	agent, ok := hub.GetAgent(result.AgentID)
	if !ok {
		t.Fatalf("expected spawned agent %s to be retrievable", result.AgentID)
	}
	if agent.SwarmID != "spawn-trim-swarm" {
		t.Fatalf("expected trimmed swarm ID, got %q", agent.SwarmID)
	}
	if agent.Type != "coder" {
		t.Fatalf("expected trimmed type, got %q", agent.Type)
	}
	if agent.Task != "implement feature" {
		t.Fatalf("expected trimmed task, got %q", agent.Task)
	}
}

func TestFederationHub_SpawnEphemeralNormalizesRequiredCapabilities(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:       "swarm-cap-target",
		Name:          "Capability Target",
		MaxAgents:     2,
		Capabilities:  []string{"code"},
	}); err != nil {
		t.Fatalf("failed to register capability target swarm: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:      "swarm-cap-other",
		Name:         "Capability Other",
		MaxAgents:    2,
		Capabilities: []string{"review"},
	}); err != nil {
		t.Fatalf("failed to register capability other swarm: %v", err)
	}

	result, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		Type: "coder",
		Task: "match normalized capabilities",
		Capabilities: []string{
			"  code  ",
			"",
			"code",
			"   ",
		},
	})
	if err != nil {
		t.Fatalf("expected spawn with normalized required capabilities to succeed, got %v", err)
	}
	if result == nil {
		t.Fatal("expected spawn result")
	}
	if result.SwarmID != "swarm-cap-target" {
		t.Fatalf("expected capability-matched swarm swarm-cap-target, got %q", result.SwarmID)
	}
}

func TestFederationHub_SpawnEphemeralAutoSelectionDeterministicTieBreak(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:      "swarm-auto-b",
		Name:         "Auto Select B",
		MaxAgents:    3,
		Capabilities: []string{"code"},
	}); err != nil {
		t.Fatalf("failed to register swarm-auto-b: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:      "swarm-auto-a",
		Name:         "Auto Select A",
		MaxAgents:    3,
		Capabilities: []string{"code"},
	}); err != nil {
		t.Fatalf("failed to register swarm-auto-a: %v", err)
	}

	result, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		Type: "coder",
		Task: "deterministic tie break",
	})
	if err != nil {
		t.Fatalf("expected deterministic auto-selection spawn to succeed, got %v", err)
	}
	if result == nil {
		t.Fatal("expected spawn result")
	}
	if result.SwarmID != "swarm-auto-a" {
		t.Fatalf("expected deterministic tie-break to pick lexicographically smallest swarm-auto-a, got %q", result.SwarmID)
	}
}

func TestFederationHub_SpawnEphemeralFailureDoesNotMutateAgentStats(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "spawn-stats-swarm",
		Name:      "Spawn Stats Swarm",
		MaxAgents: 3,
	}); err != nil {
		t.Fatalf("failed to register swarm: %v", err)
	}

	_, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "spawn-stats-swarm",
		Type:    "coder",
		Task:    "   ", // invalid, blank after trim
	})
	if err == nil {
		t.Fatal("expected spawn to fail for blank task")
	}

	stats := hub.GetStats()
	if stats.TotalAgents != 0 {
		t.Fatalf("expected total agents to remain 0 after failed spawn, got %d", stats.TotalAgents)
	}
	if stats.ActiveAgents != 0 {
		t.Fatalf("expected active agents to remain 0 after failed spawn, got %d", stats.ActiveAgents)
	}
	if agents := hub.GetAgents(); len(agents) != 0 {
		t.Fatalf("expected no agents to exist after failed spawn, got %d", len(agents))
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
		{
			name: "blank name",
			registration: shared.SwarmRegistration{
				SwarmID:   "swarm-blank-name",
				Name:      "   ",
				MaxAgents: 1,
			},
			expectedErr: "name is required",
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
	if stats.ActiveSwarms != 0 {
		t.Fatalf("expected no active swarms after invalid inputs, got %d", stats.ActiveSwarms)
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
		Capabilities: []string{
			"  code  ",
			"",
			"code",
			" review ",
			"   ",
			"review",
		},
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
	expectedCapabilities := []string{"code", "review"}
	if len(swarm.Capabilities) != len(expectedCapabilities) {
		t.Fatalf("expected %d normalized capabilities, got %d (%v)", len(expectedCapabilities), len(swarm.Capabilities), swarm.Capabilities)
	}
	for i, capability := range expectedCapabilities {
		if swarm.Capabilities[i] != capability {
			t.Fatalf("expected capability %q at index %d, got %q", capability, i, swarm.Capabilities[i])
		}
	}
}

func TestFederationHub_SwarmOperationsTrimIdentifiers(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-op-trim",
		Name:      "Swarm Operation Trim",
		MaxAgents: 2,
	}); err != nil {
		t.Fatalf("failed to register swarm: %v", err)
	}

	if _, ok := hub.GetSwarm("  swarm-op-trim  "); !ok {
		t.Fatal("expected GetSwarm to resolve trimmed swarm identifier")
	}

	if err := hub.Heartbeat("  swarm-op-trim  "); err != nil {
		t.Fatalf("expected heartbeat with padded swarmId to succeed, got %v", err)
	}

	if err := hub.UnregisterSwarm("  swarm-op-trim  "); err != nil {
		t.Fatalf("expected unregister with padded swarmId to succeed, got %v", err)
	}

	if _, ok := hub.GetSwarm("swarm-op-trim"); ok {
		t.Fatal("expected swarm to be unregistered")
	}

	if err := hub.Heartbeat("   "); err == nil || err.Error() != "swarmId is required" {
		t.Fatalf("expected heartbeat blank id error, got %v", err)
	}

	if err := hub.UnregisterSwarm("   "); err == nil || err.Error() != "swarmId is required" {
		t.Fatalf("expected unregister blank id error, got %v", err)
	}
}

func TestFederationHub_AgentOperationsTrimIdentifiers(t *testing.T) {
	hub := NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-agent-trim",
		Name:      "Agent Trim Swarm",
		MaxAgents: 4,
	}); err != nil {
		t.Fatalf("failed to register swarm: %v", err)
	}

	firstSpawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-agent-trim",
		Type:    "coder",
		Task:    "first task",
	})
	if err != nil {
		t.Fatalf("failed to spawn first agent: %v", err)
	}

	if _, ok := hub.GetAgent("  " + firstSpawn.AgentID + "  "); !ok {
		t.Fatal("expected GetAgent to resolve trimmed agent identifier")
	}

	agentsBySwarm := hub.GetAgentsBySwarm("  swarm-agent-trim  ")
	if len(agentsBySwarm) != 1 {
		t.Fatalf("expected one agent from trimmed swarm lookup, got %d", len(agentsBySwarm))
	}

	if err := hub.CompleteAgent("  "+firstSpawn.AgentID+"  ", map[string]interface{}{"ok": true}); err != nil {
		t.Fatalf("expected complete with padded agentId to succeed, got %v", err)
	}

	secondSpawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-agent-trim",
		Type:    "reviewer",
		Task:    "second task",
	})
	if err != nil {
		t.Fatalf("failed to spawn second agent: %v", err)
	}

	if err := hub.TerminateAgent("  "+secondSpawn.AgentID+"  ", "terminated for test"); err != nil {
		t.Fatalf("expected terminate with padded agentId to succeed, got %v", err)
	}

	secondAgent, ok := hub.GetAgent(secondSpawn.AgentID)
	if !ok {
		t.Fatal("expected terminated agent to remain discoverable")
	}
	if secondAgent.Status != shared.EphemeralStatusTerminated {
		t.Fatalf("expected second agent to be terminated, got %q", secondAgent.Status)
	}
	if secondAgent.Error != "terminated for test" {
		t.Fatalf("expected terminate error to be recorded, got %q", secondAgent.Error)
	}

	if err := hub.CompleteAgent("   ", nil); err == nil || err.Error() != "agentId is required" {
		t.Fatalf("expected complete blank id error, got %v", err)
	}
	if err := hub.TerminateAgent("   ", ""); err == nil || err.Error() != "agentId is required" {
		t.Fatalf("expected terminate blank id error, got %v", err)
	}
	if _, ok := hub.GetAgent("   "); ok {
		t.Fatal("expected blank GetAgent lookup to fail")
	}
	if agents := hub.GetAgentsBySwarm("   "); len(agents) != 0 {
		t.Fatalf("expected blank swarm lookup to return no agents, got %d", len(agents))
	}
}
