// Package sessions provides session management for MCP.
package sessions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// PersistenceStore handles session persistence to disk.
type PersistenceStore struct {
	mu        sync.RWMutex
	baseDir   string
	savedList []*shared.SessionSummary
}

// NewPersistenceStore creates a new PersistenceStore.
func NewPersistenceStore(baseDir string) *PersistenceStore {
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			baseDir = ".claude-flow/sessions"
		} else {
			baseDir = filepath.Join(homeDir, ".claude-flow", "sessions")
		}
	}
	return &PersistenceStore{
		baseDir:   baseDir,
		savedList: make([]*shared.SessionSummary, 0),
	}
}

// ensureDir ensures the storage directory exists.
func (ps *PersistenceStore) ensureDir() error {
	return os.MkdirAll(ps.baseDir, 0755)
}

// getFilePath returns the file path for a session ID.
func (ps *PersistenceStore) getFilePath(sessionID string) string {
	// Sanitize session ID to prevent path traversal
	safeName := strings.ReplaceAll(sessionID, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	safeName = strings.ReplaceAll(safeName, "..", "_")
	return filepath.Join(ps.baseDir, safeName+".json")
}

// calculateChecksum calculates SHA-256 checksum of data.
func (ps *PersistenceStore) calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Save saves a session to disk.
func (ps *PersistenceStore) Save(session *shared.SavedSession) (*shared.SessionSaveResult, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if err := ps.ensureDir(); err != nil {
		return nil, err
	}

	// Marshal without checksum first
	session.Checksum = ""
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, err
	}

	// Calculate checksum
	session.Checksum = ps.calculateChecksum(data)

	// Marshal again with checksum
	data, err = json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, err
	}

	filePath := ps.getFilePath(session.ID)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, err
	}

	// Update saved list
	ps.updateSavedList(session)

	return &shared.SessionSaveResult{
		SessionID: session.ID,
		FilePath:  filePath,
		Size:      int64(len(data)),
		Checksum:  session.Checksum,
		SavedAt:   session.SavedAt,
	}, nil
}

// updateSavedList updates the cached list of saved sessions.
func (ps *PersistenceStore) updateSavedList(session *shared.SavedSession) {
	// Check if already in list
	for i, s := range ps.savedList {
		if s.ID == session.ID {
			ps.savedList[i] = &shared.SessionSummary{
				ID:        session.ID,
				Name:      session.Name,
				CreatedAt: session.CreatedAt,
				Tags:      session.Tags,
			}
			return
		}
	}

	// Add new entry
	ps.savedList = append(ps.savedList, &shared.SessionSummary{
		ID:        session.ID,
		Name:      session.Name,
		CreatedAt: session.CreatedAt,
		Tags:      session.Tags,
	})
}

// Load loads a session from disk.
func (ps *PersistenceStore) Load(sessionID string) (*shared.SavedSession, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	filePath := ps.getFilePath(sessionID)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, shared.ErrSessionSaveNotFound
		}
		return nil, err
	}

	var session shared.SavedSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	// Verify checksum
	storedChecksum := session.Checksum
	session.Checksum = ""
	dataWithoutChecksum, _ := json.MarshalIndent(&session, "", "  ")
	calculatedChecksum := ps.calculateChecksum(dataWithoutChecksum)

	if storedChecksum != calculatedChecksum {
		return nil, shared.ErrSessionChecksumMismatch
	}

	session.Checksum = storedChecksum
	return &session, nil
}

// Delete deletes a saved session from disk.
func (ps *PersistenceStore) Delete(sessionID string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	filePath := ps.getFilePath(sessionID)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return shared.ErrSessionSaveNotFound
		}
		return err
	}

	// Remove from saved list
	for i, s := range ps.savedList {
		if s.ID == sessionID {
			ps.savedList = append(ps.savedList[:i], ps.savedList[i+1:]...)
			break
		}
	}

	return nil
}

// List lists saved sessions.
func (ps *PersistenceStore) List(tags []string) ([]*shared.SessionSummary, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Refresh list from disk
	if err := ps.refreshList(); err != nil {
		return nil, err
	}

	// Filter by tags if provided
	if len(tags) == 0 {
		return ps.savedList, nil
	}

	var filtered []*shared.SessionSummary
	for _, s := range ps.savedList {
		if ps.matchesTags(s.Tags, tags) {
			filtered = append(filtered, s)
		}
	}

	return filtered, nil
}

// refreshList refreshes the cached list from disk.
func (ps *PersistenceStore) refreshList() error {
	if err := ps.ensureDir(); err != nil {
		return err
	}

	entries, err := os.ReadDir(ps.baseDir)
	if err != nil {
		return err
	}

	ps.savedList = make([]*shared.SessionSummary, 0)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(ps.baseDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var session shared.SavedSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		ps.savedList = append(ps.savedList, &shared.SessionSummary{
			ID:         session.ID,
			Name:       session.Name,
			CreatedAt:  session.CreatedAt,
			AgentCount: len(session.Agents),
			TaskCount:  len(session.Tasks),
			Tags:       session.Tags,
		})
	}

	// Sort by created date (newest first)
	sort.Slice(ps.savedList, func(i, j int) bool {
		return ps.savedList[i].CreatedAt > ps.savedList[j].CreatedAt
	})

	return nil
}

// matchesTags checks if session tags match the filter tags.
func (ps *PersistenceStore) matchesTags(sessionTags, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}

	tagSet := make(map[string]bool)
	for _, t := range sessionTags {
		tagSet[strings.ToLower(t)] = true
	}

	for _, t := range filterTags {
		if tagSet[strings.ToLower(t)] {
			return true
		}
	}

	return false
}

// Exists checks if a saved session exists.
func (ps *PersistenceStore) Exists(sessionID string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	filePath := ps.getFilePath(sessionID)
	_, err := os.Stat(filePath)
	return err == nil
}

// GetInfo returns information about a saved session without loading full data.
func (ps *PersistenceStore) GetInfo(sessionID string) (*shared.SessionSummary, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	filePath := ps.getFilePath(sessionID)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, shared.ErrSessionSaveNotFound
		}
		return nil, err
	}

	var session shared.SavedSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &shared.SessionSummary{
		ID:         session.ID,
		Name:       session.Name,
		CreatedAt:  session.CreatedAt,
		AgentCount: len(session.Agents),
		TaskCount:  len(session.Tasks),
		Tags:       session.Tags,
	}, nil
}
