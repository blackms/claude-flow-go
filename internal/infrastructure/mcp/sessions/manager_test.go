package sessions

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestSessionManager_NormalizesConfigAndHandlesNilListFilter(t *testing.T) {
	manager := NewSessionManager(shared.SessionConfig{})
	defaults := shared.DefaultSessionConfig()

	config := manager.GetConfig()
	if config.MaxSessions != defaults.MaxSessions {
		t.Fatalf("expected default max sessions %d, got %d", defaults.MaxSessions, config.MaxSessions)
	}
	if config.SessionTimeout != defaults.SessionTimeout {
		t.Fatalf("expected default session timeout %d, got %d", defaults.SessionTimeout, config.SessionTimeout)
	}
	if config.CleanupInterval != defaults.CleanupInterval {
		t.Fatalf("expected default cleanup interval %d, got %d", defaults.CleanupInterval, config.CleanupInterval)
	}

	created, err := manager.CreateSession(shared.TransportHTTP)
	if err != nil {
		t.Fatalf("expected create session success with normalized config, got %v", err)
	}
	if created == nil || created.ID == "" {
		t.Fatalf("expected created session with id, got %+v", created)
	}

	list := manager.ListSessions(nil)
	if list == nil {
		t.Fatal("expected non-nil list result")
	}
	if list.Total != 1 {
		t.Fatalf("expected one listed session, got %d", list.Total)
	}
}

func TestSessionManager_CreateAndGetSessionReturnDefensiveCopies(t *testing.T) {
	manager := NewSessionManagerWithDefaults()

	created, err := manager.CreateSession(shared.TransportStdio)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	sessionID := created.ID

	created.Metadata["external"] = "mutated"
	created.Agents = append(created.Agents, "external-agent")
	created.Tasks = append(created.Tasks, "external-task")

	fetched, err := manager.GetSession(sessionID)
	if err != nil {
		t.Fatalf("failed to fetch session: %v", err)
	}
	if _, exists := fetched.Metadata["external"]; exists {
		t.Fatal("expected internal metadata to be isolated from create result mutations")
	}
	if len(fetched.Agents) != 0 {
		t.Fatalf("expected no agents from internal state, got %d", len(fetched.Agents))
	}
	if len(fetched.Tasks) != 0 {
		t.Fatalf("expected no tasks from internal state, got %d", len(fetched.Tasks))
	}

	clientInfo := &shared.SessionClientInfo{Name: "client-a", Version: "1.0"}
	if err := manager.InitializeSession(sessionID, clientInfo, "2025-11-25"); err != nil {
		t.Fatalf("failed to initialize session: %v", err)
	}
	clientInfo.Name = "mutated-client"

	initialized, err := manager.GetSession(sessionID)
	if err != nil {
		t.Fatalf("failed to fetch initialized session: %v", err)
	}
	if initialized.ClientInfo == nil || initialized.ClientInfo.Name != "client-a" {
		t.Fatalf("expected cloned client info name client-a, got %+v", initialized.ClientInfo)
	}

	initialized.Metadata["roundtrip"] = "mutated"
	initialized.Agents = append(initialized.Agents, "agent-roundtrip")

	refetched, err := manager.GetSession(sessionID)
	if err != nil {
		t.Fatalf("failed to refetch session: %v", err)
	}
	if _, exists := refetched.Metadata["roundtrip"]; exists {
		t.Fatal("expected metadata mutations on fetched session to be isolated")
	}
	if len(refetched.Agents) != 0 {
		t.Fatalf("expected fetched session mutations not to persist, got %d agents", len(refetched.Agents))
	}
}

func TestSessionManager_ListSessionsUsesDeterministicOrdering(t *testing.T) {
	manager := NewSessionManagerWithDefaults()

	manager.mu.Lock()
	manager.sessions = map[string]*shared.Session{
		"session-c": {ID: "session-c", State: shared.SessionStateCreated, CreatedAt: 100},
		"session-a": {ID: "session-a", State: shared.SessionStateCreated, CreatedAt: 100},
		"session-b": {ID: "session-b", State: shared.SessionStateCreated, CreatedAt: 100},
	}
	manager.mu.Unlock()

	list := manager.ListSessions(&shared.SessionListRequest{})
	if list == nil {
		t.Fatal("expected list result")
	}
	if len(list.Sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(list.Sessions))
	}

	expectedIDs := []string{"session-a", "session-b", "session-c"}
	for i, expectedID := range expectedIDs {
		if list.Sessions[i].ID != expectedID {
			t.Fatalf("expected sessions[%d].ID = %q, got %q", i, expectedID, list.Sessions[i].ID)
		}
	}
}

func TestSessionManager_SaveSessionRejectsNilRequest(t *testing.T) {
	manager := NewSessionManagerWithDefaults()
	_, err := manager.SaveSession(nil)
	if err == nil {
		t.Fatal("expected error for nil save request")
	}
	if err.Error() != "session save request is required" {
		t.Fatalf("expected nil save request error, got %q", err.Error())
	}
}
