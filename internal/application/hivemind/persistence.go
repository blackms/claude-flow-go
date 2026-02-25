package hivemind

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// PersistentState holds the hive-mind state that is saved to disk.
type PersistentState struct {
	Config     shared.HiveMindConfig `json:"config"`
	Algorithm  string                `json:"algorithm"`
	V3Mode     bool                  `json:"v3Mode"`
	Quorum     float64               `json:"quorum"`
	AgentSpecs []AgentSpec           `json:"agents,omitempty"`
}

// AgentSpec records the configuration of a spawned agent for re-creation.
type AgentSpec struct {
	ID     string            `json:"id"`
	Type   shared.AgentType  `json:"type"`
	Role   shared.AgentRole  `json:"role"`
	Domain shared.AgentDomain `json:"domain"`
}

// statePath returns the path to the state file.
func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude-flow", "hivemind", "state.json"), nil
}

// SaveState persists the hive-mind state to disk.
func SaveState(state *PersistentState) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// LoadState loads the hive-mind state from disk.
func LoadState() (*PersistentState, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// ClearState removes the persisted state from disk.
func ClearState() error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove state file: %w", err)
	}
	return nil
}

// HasPersistedState checks if a state file exists on disk.
func HasPersistedState() bool {
	path, err := statePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
