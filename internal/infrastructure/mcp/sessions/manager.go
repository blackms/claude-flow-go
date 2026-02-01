// Package sessions provides session management for MCP.
package sessions

import (
	"context"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// SessionManager manages MCP sessions.
type SessionManager struct {
	mu              sync.RWMutex
	sessions        map[string]*shared.Session
	config          shared.SessionConfig
	stats           *shared.SessionStats
	persistence     *PersistenceStore
	running         bool
	ctx             context.Context
	cancel          context.CancelFunc
	cleanupInterval time.Duration
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(config shared.SessionConfig) *SessionManager {
	return &SessionManager{
		sessions:        make(map[string]*shared.Session),
		config:          config,
		cleanupInterval: time.Duration(config.CleanupInterval) * time.Millisecond,
		stats: &shared.SessionStats{
			ByState:     make(map[shared.SessionState]int),
			ByTransport: make(map[shared.TransportType]int),
		},
		persistence: NewPersistenceStore(""),
	}
}

// NewSessionManagerWithDefaults creates a SessionManager with default configuration.
func NewSessionManagerWithDefaults() *SessionManager {
	return NewSessionManager(shared.DefaultSessionConfig())
}

// Initialize starts the SessionManager.
func (sm *SessionManager) Initialize() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.running {
		return nil
	}

	sm.ctx, sm.cancel = context.WithCancel(context.Background())
	sm.running = true

	// Start cleanup goroutine
	go sm.cleanupLoop()

	return nil
}

// Shutdown stops the SessionManager.
func (sm *SessionManager) Shutdown() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.running {
		return nil
	}

	sm.running = false
	if sm.cancel != nil {
		sm.cancel()
	}

	// Close all active sessions
	for id, session := range sm.sessions {
		if session.State != shared.SessionStateClosed && session.State != shared.SessionStateExpired {
			session.State = shared.SessionStateClosed
			sm.stats.TotalClosed++
		}
		delete(sm.sessions, id)
	}

	return nil
}

// CreateSession creates a new session.
func (sm *SessionManager) CreateSession(transport shared.TransportType) (*shared.Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check max sessions
	if len(sm.sessions) >= sm.config.MaxSessions {
		return nil, shared.ErrMaxSessionsReached
	}

	now := shared.Now()
	session := &shared.Session{
		ID:             shared.GenerateID("session"),
		State:          shared.SessionStateCreated,
		Transport:      transport,
		CreatedAt:      now,
		LastActivityAt: now,
		ExpiresAt:      now + sm.config.SessionTimeout,
		IsInitialized:  false,
		IsAuthenticated: false,
		Agents:         make([]string, 0),
		Tasks:          make([]string, 0),
		Metadata:       make(map[string]interface{}),
	}

	sm.sessions[session.ID] = session
	sm.stats.TotalCreated++
	sm.stats.ActiveCount++
	sm.stats.ByState[session.State]++
	sm.stats.ByTransport[transport]++

	return session, nil
}

// GetSession returns a session by ID.
func (sm *SessionManager) GetSession(id string) (*shared.Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[id]
	if !exists {
		return nil, shared.ErrSessionNotFound
	}

	if session.State == shared.SessionStateExpired {
		return nil, shared.ErrSessionExpired
	}

	if session.State == shared.SessionStateClosed {
		return nil, shared.ErrSessionClosed
	}

	return session, nil
}

// UpdateActivity updates the last activity timestamp for a session.
func (sm *SessionManager) UpdateActivity(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return shared.ErrSessionNotFound
	}

	if session.State == shared.SessionStateClosed || session.State == shared.SessionStateExpired {
		return shared.ErrSessionClosed
	}

	now := shared.Now()
	session.LastActivityAt = now
	session.ExpiresAt = now + sm.config.SessionTimeout

	// Update state to active if ready
	if session.State == shared.SessionStateReady {
		sm.stats.ByState[session.State]--
		session.State = shared.SessionStateActive
		sm.stats.ByState[session.State]++
	}

	return nil
}

// InitializeSession marks a session as initialized.
func (sm *SessionManager) InitializeSession(id string, clientInfo *shared.SessionClientInfo, protocolVersion string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return shared.ErrSessionNotFound
	}

	if session.State != shared.SessionStateCreated {
		return shared.ErrInvalidSessionState
	}

	sm.stats.ByState[session.State]--
	session.State = shared.SessionStateReady
	session.IsInitialized = true
	session.ClientInfo = clientInfo
	session.ProtocolVersion = protocolVersion
	session.LastActivityAt = shared.Now()
	sm.stats.ByState[session.State]++

	return nil
}

// CloseSession closes a session.
func (sm *SessionManager) CloseSession(id, reason string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return shared.ErrSessionNotFound
	}

	if session.State == shared.SessionStateClosed {
		return nil // Already closed
	}

	sm.stats.ByState[session.State]--
	session.State = shared.SessionStateClosed
	sm.stats.ByState[session.State]++
	sm.stats.TotalClosed++
	sm.stats.ActiveCount--

	// Store close reason in metadata
	if reason != "" {
		session.Metadata["closeReason"] = reason
	}
	session.Metadata["closedAt"] = shared.Now()

	// Update average duration
	duration := float64(shared.Now() - session.CreatedAt)
	n := float64(sm.stats.TotalClosed)
	sm.stats.AvgDurationMs = (sm.stats.AvgDurationMs*(n-1) + duration) / n

	return nil
}

// AddAgent adds an agent to a session.
func (sm *SessionManager) AddAgent(sessionID, agentID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return shared.ErrSessionNotFound
	}

	session.Agents = append(session.Agents, agentID)
	session.LastActivityAt = shared.Now()

	return nil
}

// RemoveAgent removes an agent from a session.
func (sm *SessionManager) RemoveAgent(sessionID, agentID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return shared.ErrSessionNotFound
	}

	for i, id := range session.Agents {
		if id == agentID {
			session.Agents = append(session.Agents[:i], session.Agents[i+1:]...)
			break
		}
	}
	session.LastActivityAt = shared.Now()

	return nil
}

// AddTask adds a task to a session.
func (sm *SessionManager) AddTask(sessionID, taskID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return shared.ErrSessionNotFound
	}

	session.Tasks = append(session.Tasks, taskID)
	session.LastActivityAt = shared.Now()

	return nil
}

// RemoveTask removes a task from a session.
func (sm *SessionManager) RemoveTask(sessionID, taskID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return shared.ErrSessionNotFound
	}

	for i, id := range session.Tasks {
		if id == taskID {
			session.Tasks = append(session.Tasks[:i], session.Tasks[i+1:]...)
			break
		}
	}
	session.LastActivityAt = shared.Now()

	return nil
}

// ListSessions returns sessions matching the filter.
func (sm *SessionManager) ListSessions(filter *shared.SessionListRequest) *shared.SessionListResult {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var summaries []*shared.SessionSummary

	for _, session := range sm.sessions {
		// Filter by type
		if filter.Type == "active" {
			if session.State == shared.SessionStateClosed || session.State == shared.SessionStateExpired {
				continue
			}
		}

		summary := &shared.SessionSummary{
			ID:             session.ID,
			State:          session.State,
			Transport:      session.Transport,
			CreatedAt:      session.CreatedAt,
			LastActivityAt: session.LastActivityAt,
			AgentCount:     len(session.Agents),
			TaskCount:      len(session.Tasks),
		}
		summaries = append(summaries, summary)
	}

	total := len(summaries)

	// Apply pagination
	offset := filter.Offset
	if offset > total {
		offset = total
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	endIndex := offset + limit
	if endIndex > total {
		endIndex = total
	}

	if offset < total {
		summaries = summaries[offset:endIndex]
	} else {
		summaries = []*shared.SessionSummary{}
	}

	return &shared.SessionListResult{
		Sessions: summaries,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}
}

// CleanupExpiredSessions removes expired sessions.
func (sm *SessionManager) CleanupExpiredSessions() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := shared.Now()
	count := 0

	for id, session := range sm.sessions {
		if session.State == shared.SessionStateClosed {
			delete(sm.sessions, id)
			continue
		}

		// Check if session has expired
		if now > session.ExpiresAt {
			sm.stats.ByState[session.State]--
			session.State = shared.SessionStateExpired
			sm.stats.ByState[session.State]++
			sm.stats.TotalExpired++
			sm.stats.ActiveCount--
			count++

			// Keep expired sessions for a short time before removing
			session.Metadata["expiredAt"] = now
		}

		// Remove sessions that have been expired for more than 5 minutes
		if session.State == shared.SessionStateExpired {
			if expiredAt, ok := session.Metadata["expiredAt"].(int64); ok {
				if now-expiredAt > 5*60*1000 { // 5 minutes
					delete(sm.sessions, id)
				}
			}
		}
	}

	return count
}

// cleanupLoop runs the cleanup process periodically.
func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(sm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.CleanupExpiredSessions()
		}
	}
}

// GetStats returns session statistics.
func (sm *SessionManager) GetStats() *shared.SessionStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Create a copy
	stats := *sm.stats

	// Copy maps
	stats.ByState = make(map[shared.SessionState]int)
	for k, v := range sm.stats.ByState {
		stats.ByState[k] = v
	}
	stats.ByTransport = make(map[shared.TransportType]int)
	for k, v := range sm.stats.ByTransport {
		stats.ByTransport[k] = v
	}

	return &stats
}

// GetConfig returns the session configuration.
func (sm *SessionManager) GetConfig() shared.SessionConfig {
	return sm.config
}

// SessionCount returns the total number of sessions.
func (sm *SessionManager) SessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// ActiveSessionCount returns the number of active sessions.
func (sm *SessionManager) ActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, session := range sm.sessions {
		if session.State != shared.SessionStateClosed && session.State != shared.SessionStateExpired {
			count++
		}
	}
	return count
}

// GetPersistence returns the persistence store.
func (sm *SessionManager) GetPersistence() *PersistenceStore {
	return sm.persistence
}

// SaveSession saves a session to disk.
func (sm *SessionManager) SaveSession(req *shared.SessionSaveRequest) (*shared.SessionSaveResult, error) {
	sm.mu.RLock()
	session, exists := sm.sessions[req.SessionID]
	if !exists {
		sm.mu.RUnlock()
		return nil, shared.ErrSessionNotFound
	}

	// Create a copy of the session data
	savedSession := &shared.SavedSession{
		ID:          session.ID,
		Name:        req.Name,
		Description: req.Description,
		Version:     "3.0.0",
		CreatedAt:   session.CreatedAt,
		SavedAt:     shared.Now(),
		Tags:        req.Tags,
		Metadata:    session.Metadata,
	}

	if req.IncludeAgents {
		for _, agentID := range session.Agents {
			savedSession.Agents = append(savedSession.Agents, shared.SavedSessionAgent{
				ID: agentID,
			})
		}
	}

	if req.IncludeTasks {
		for _, taskID := range session.Tasks {
			savedSession.Tasks = append(savedSession.Tasks, shared.SavedSessionTask{
				ID: taskID,
			})
		}
	}
	sm.mu.RUnlock()

	return sm.persistence.Save(savedSession)
}

// RestoreSession restores a session from disk.
func (sm *SessionManager) RestoreSession(req *shared.SessionRestoreRequest) (*shared.SessionRestoreResult, error) {
	savedSession, err := sm.persistence.Load(req.SessionID)
	if err != nil {
		return nil, err
	}

	result := &shared.SessionRestoreResult{
		SessionID:  savedSession.ID,
		RestoredAt: shared.Now(),
	}

	if req.RestoreAgents {
		result.AgentsRestored = len(savedSession.Agents)
	}
	if req.RestoreTasks {
		result.TasksRestored = len(savedSession.Tasks)
	}
	if req.RestoreMemory {
		result.MemoryRestored = len(savedSession.Memory)
	}

	return result, nil
}

// ListSavedSessions lists saved sessions.
func (sm *SessionManager) ListSavedSessions(tags []string) ([]*shared.SessionSummary, error) {
	return sm.persistence.List(tags)
}
