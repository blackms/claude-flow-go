// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/sessions"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// SessionTools provides MCP tools for session operations.
type SessionTools struct {
	manager *sessions.SessionManager
}

// NewSessionTools creates a new SessionTools instance.
func NewSessionTools(manager *sessions.SessionManager) *SessionTools {
	return &SessionTools{
		manager: manager,
	}
}

// GetTools returns available session tools.
func (t *SessionTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "session/save",
			Description: "Save session state to disk for later restoration",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the session to save",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the saved session",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Description of the session",
					},
					"includeTasks": map[string]interface{}{
						"type":        "boolean",
						"description": "Include task queue in save",
					},
					"includeAgents": map[string]interface{}{
						"type":        "boolean",
						"description": "Include agent states in save",
					},
					"includeMemory": map[string]interface{}{
						"type":        "boolean",
						"description": "Include memory entries in save",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Tags for organizing saved sessions",
					},
				},
				"required": []string{"sessionId"},
			},
		},
		{
			Name:        "session/restore",
			Description: "Restore a previously saved session",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the saved session to restore",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the saved session to restore",
					},
					"clearExisting": map[string]interface{}{
						"type":        "boolean",
						"description": "Clear existing state before restore",
					},
					"restoreTasks": map[string]interface{}{
						"type":        "boolean",
						"description": "Restore task queue",
					},
					"restoreAgents": map[string]interface{}{
						"type":        "boolean",
						"description": "Restore agent states",
					},
					"restoreMemory": map[string]interface{}{
						"type":        "boolean",
						"description": "Restore memory entries",
					},
				},
			},
		},
		{
			Name:        "session/list",
			Description: "List active or saved sessions",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Type of sessions to list: active, saved, all",
						"enum":        []string{"active", "saved", "all"},
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of sessions to return",
					},
					"offset": map[string]interface{}{
						"type":        "number",
						"description": "Offset for pagination",
					},
					"sortBy": map[string]interface{}{
						"type":        "string",
						"description": "Sort by: createdAt, lastActivityAt, name",
						"enum":        []string{"createdAt", "lastActivityAt", "name"},
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Filter by tags",
					},
				},
			},
		},
		{
			Name:        "session/close",
			Description: "Close an active session",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the session to close",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Reason for closing the session",
					},
					"save": map[string]interface{}{
						"type":        "boolean",
						"description": "Save session before closing",
					},
				},
				"required": []string{"sessionId"},
			},
		},
		{
			Name:        "session/info",
			Description: "Get detailed information about a session",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the session",
					},
					"includeMetrics": map[string]interface{}{
						"type":        "boolean",
						"description": "Include session metrics",
					},
					"includeHistory": map[string]interface{}{
						"type":        "boolean",
						"description": "Include session history",
					},
				},
				"required": []string{"sessionId"},
			},
		},
	}
}

// Execute executes a session tool.
func (t *SessionTools) Execute(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	switch method {
	case "session/save":
		return t.saveSession(ctx, params)
	case "session/restore":
		return t.restoreSession(ctx, params)
	case "session/list":
		return t.listSessions(ctx, params)
	case "session/close":
		return t.closeSession(ctx, params)
	case "session/info":
		return t.sessionInfo(ctx, params)
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

func (t *SessionTools) saveSession(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sessionID, _ := params["sessionId"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("sessionId is required")
	}

	name, _ := params["name"].(string)
	description, _ := params["description"].(string)
	includeTasks, _ := params["includeTasks"].(bool)
	includeAgents, _ := params["includeAgents"].(bool)
	includeMemory, _ := params["includeMemory"].(bool)

	var tags []string
	if tagsArr, ok := params["tags"].([]interface{}); ok {
		for _, tag := range tagsArr {
			if s, ok := tag.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	req := &shared.SessionSaveRequest{
		SessionID:     sessionID,
		Name:          name,
		Description:   description,
		IncludeTasks:  includeTasks,
		IncludeAgents: includeAgents,
		IncludeMemory: includeMemory,
		Tags:          tags,
	}

	result, err := t.manager.SaveSession(req)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"sessionId": result.SessionID,
		"filePath":  result.FilePath,
		"size":      result.Size,
		"checksum":  result.Checksum,
		"savedAt":   result.SavedAt,
		"success":   true,
	}, nil
}

func (t *SessionTools) restoreSession(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sessionID, _ := params["sessionId"].(string)
	name, _ := params["name"].(string)

	if sessionID == "" && name == "" {
		return nil, fmt.Errorf("sessionId or name is required")
	}

	clearExisting, _ := params["clearExisting"].(bool)
	restoreTasks, _ := params["restoreTasks"].(bool)
	restoreAgents, _ := params["restoreAgents"].(bool)
	restoreMemory, _ := params["restoreMemory"].(bool)

	req := &shared.SessionRestoreRequest{
		SessionID:     sessionID,
		Name:          name,
		ClearExisting: clearExisting,
		RestoreTasks:  restoreTasks,
		RestoreAgents: restoreAgents,
		RestoreMemory: restoreMemory,
	}

	result, err := t.manager.RestoreSession(req)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"sessionId":      result.SessionID,
		"restoredAt":     result.RestoredAt,
		"tasksRestored":  result.TasksRestored,
		"agentsRestored": result.AgentsRestored,
		"memoryRestored": result.MemoryRestored,
		"success":        true,
	}, nil
}

func (t *SessionTools) listSessions(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	listType := "active"
	if lt, ok := params["type"].(string); ok && lt != "" {
		listType = lt
	}

	limit := 20
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	offset := 0
	if o, ok := params["offset"].(float64); ok {
		offset = int(o)
	}

	sortBy := "createdAt"
	if sb, ok := params["sortBy"].(string); ok && sb != "" {
		sortBy = sb
	}

	var tags []string
	if tagsArr, ok := params["tags"].([]interface{}); ok {
		for _, tag := range tagsArr {
			if s, ok := tag.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	var allSessions []*shared.SessionSummary

	// Get active sessions
	if listType == "active" || listType == "all" {
		req := &shared.SessionListRequest{
			Type:   "active",
			Limit:  limit,
			Offset: offset,
			SortBy: sortBy,
			Tags:   tags,
		}
		result := t.manager.ListSessions(req)
		allSessions = append(allSessions, result.Sessions...)
	}

	// Get saved sessions
	if listType == "saved" || listType == "all" {
		savedSessions, err := t.manager.ListSavedSessions(tags)
		if err == nil {
			allSessions = append(allSessions, savedSessions...)
		}
	}

	// Apply pagination
	total := len(allSessions)
	if offset >= total {
		allSessions = []*shared.SessionSummary{}
	} else {
		endIdx := offset + limit
		if endIdx > total {
			endIdx = total
		}
		allSessions = allSessions[offset:endIdx]
	}

	return map[string]interface{}{
		"sessions": allSessions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"type":     listType,
	}, nil
}

func (t *SessionTools) closeSession(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sessionID, _ := params["sessionId"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("sessionId is required")
	}

	reason, _ := params["reason"].(string)
	save, _ := params["save"].(bool)

	// Optionally save before closing
	if save {
		saveReq := &shared.SessionSaveRequest{
			SessionID:     sessionID,
			Name:          "auto-save-before-close",
			IncludeTasks:  true,
			IncludeAgents: true,
		}
		_, _ = t.manager.SaveSession(saveReq)
	}

	// Get session info before closing
	session, _ := t.manager.GetSession(sessionID)
	var agentCount, taskCount int
	if session != nil {
		agentCount = len(session.Agents)
		taskCount = len(session.Tasks)
	}

	if err := t.manager.CloseSession(sessionID, reason); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"sessionId":  sessionID,
		"closed":     true,
		"reason":     reason,
		"saved":      save,
		"agentCount": agentCount,
		"taskCount":  taskCount,
	}, nil
}

func (t *SessionTools) sessionInfo(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sessionID, _ := params["sessionId"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("sessionId is required")
	}

	includeMetrics, _ := params["includeMetrics"].(bool)

	// Try to get active session
	session, err := t.manager.GetSession(sessionID)
	if err != nil {
		// Try to get saved session info
		persistence := t.manager.GetPersistence()
		savedInfo, err := persistence.GetInfo(sessionID)
		if err != nil {
			return nil, shared.ErrSessionNotFound
		}

		return map[string]interface{}{
			"sessionId":  savedInfo.ID,
			"name":       savedInfo.Name,
			"type":       "saved",
			"createdAt":  savedInfo.CreatedAt,
			"agentCount": savedInfo.AgentCount,
			"taskCount":  savedInfo.TaskCount,
			"tags":       savedInfo.Tags,
		}, nil
	}

	result := map[string]interface{}{
		"sessionId":       session.ID,
		"type":            "active",
		"state":           session.State,
		"transport":       session.Transport,
		"createdAt":       session.CreatedAt,
		"lastActivityAt":  session.LastActivityAt,
		"expiresAt":       session.ExpiresAt,
		"isInitialized":   session.IsInitialized,
		"isAuthenticated": session.IsAuthenticated,
		"protocolVersion": session.ProtocolVersion,
		"agentCount":      len(session.Agents),
		"taskCount":       len(session.Tasks),
		"agents":          session.Agents,
		"tasks":           session.Tasks,
	}

	if session.ClientInfo != nil {
		result["clientInfo"] = session.ClientInfo
	}

	if includeMetrics {
		stats := t.manager.GetStats()
		result["metrics"] = map[string]interface{}{
			"totalSessions":    stats.TotalCreated,
			"activeSessions":   stats.ActiveCount,
			"expiredSessions":  stats.TotalExpired,
			"avgDurationMs":    stats.AvgDurationMs,
		}
	}

	return result, nil
}
