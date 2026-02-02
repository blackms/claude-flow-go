// Package hooks provides the hooks system for self-learning operations.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	domainHooks "github.com/anthropics/claude-flow-go/internal/domain/hooks"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Persistence handles JSON file persistence for hooks state.
type Persistence struct {
	mu       sync.RWMutex
	basePath string
	state    *domainHooks.HooksState
}

// NewPersistence creates a new persistence manager.
func NewPersistence(basePath string) (*Persistence, error) {
	if basePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		basePath = filepath.Join(home, ".claude-flow", "hooks")
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	p := &Persistence{
		basePath: basePath,
	}

	// Load or create state
	if err := p.load(); err != nil {
		// Create new state if file doesn't exist
		p.state = domainHooks.NewHooksState()
		if err := p.save(); err != nil {
			return nil, err
		}
	}

	return p, nil
}

// statePath returns the path to the state file.
func (p *Persistence) statePath() string {
	return filepath.Join(p.basePath, "state.json")
}

// load loads the state from disk.
func (p *Persistence) load() error {
	data, err := os.ReadFile(p.statePath())
	if err != nil {
		return err
	}

	var state domainHooks.HooksState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	p.state = &state
	return nil
}

// save saves the state to disk.
func (p *Persistence) save() error {
	p.state.UpdatedAt = shared.Now()

	data, err := json.MarshalIndent(p.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	if err := os.WriteFile(p.statePath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Save persists the current state to disk.
func (p *Persistence) Save() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.save()
}

// Reload reloads the state from disk.
func (p *Persistence) Reload() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.load()
}

// GetState returns a copy of the current state.
func (p *Persistence) GetState() *domainHooks.HooksState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy
	stateCopy := *p.state
	return &stateCopy
}

// GetConfig returns the current configuration.
func (p *Persistence) GetConfig() shared.HooksConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.Config
}

// UpdateConfig updates the configuration.
func (p *Persistence) UpdateConfig(update domainHooks.ConfigUpdate) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	update.Apply(&p.state.Config)
	return p.save()
}

// SetConfig sets the entire configuration.
func (p *Persistence) SetConfig(config shared.HooksConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.state.Config = config
	return p.save()
}

// GetHooks returns all hook definitions.
func (p *Persistence) GetHooks() []domainHooks.HookDefinition {
	p.mu.RLock()
	defer p.mu.RUnlock()

	hooks := make([]domainHooks.HookDefinition, len(p.state.Hooks))
	copy(hooks, p.state.Hooks)
	return hooks
}

// GetHook returns a hook by ID or name.
func (p *Persistence) GetHook(idOrName string) *domainHooks.HookDefinition {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for i := range p.state.Hooks {
		if p.state.Hooks[i].ID == idOrName || p.state.Hooks[i].Name == idOrName {
			hook := p.state.Hooks[i]
			return &hook
		}
	}
	return nil
}

// UpdateHook updates a hook definition.
func (p *Persistence) UpdateHook(hook domainHooks.HookDefinition) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.state.Hooks {
		if p.state.Hooks[i].ID == hook.ID {
			hook.UpdatedAt = shared.Now()
			p.state.Hooks[i] = hook
			return p.save()
		}
	}

	return shared.ErrHookNotFound
}

// AddHook adds a new hook definition.
func (p *Persistence) AddHook(hook domainHooks.HookDefinition) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if hook already exists
	for _, h := range p.state.Hooks {
		if h.ID == hook.ID || h.Name == hook.Name {
			return shared.ErrHookAlreadyExists
		}
	}

	now := shared.Now()
	if hook.CreatedAt == 0 {
		hook.CreatedAt = now
	}
	hook.UpdatedAt = now

	p.state.Hooks = append(p.state.Hooks, hook)
	return p.save()
}

// RemoveHook removes a hook by ID.
func (p *Persistence) RemoveHook(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.state.Hooks {
		if p.state.Hooks[i].ID == id {
			p.state.Hooks = append(p.state.Hooks[:i], p.state.Hooks[i+1:]...)
			return p.save()
		}
	}

	return shared.ErrHookNotFound
}

// EnableHook enables a hook by ID or name.
func (p *Persistence) EnableHook(idOrName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.state.Hooks {
		if p.state.Hooks[i].ID == idOrName || p.state.Hooks[i].Name == idOrName {
			p.state.Hooks[i].Enabled = true
			p.state.Hooks[i].UpdatedAt = shared.Now()
			return p.save()
		}
	}

	return shared.ErrHookNotFound
}

// DisableHook disables a hook by ID or name.
func (p *Persistence) DisableHook(idOrName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.state.Hooks {
		if p.state.Hooks[i].ID == idOrName || p.state.Hooks[i].Name == idOrName {
			p.state.Hooks[i].Enabled = false
			p.state.Hooks[i].UpdatedAt = shared.Now()
			return p.save()
		}
	}

	return shared.ErrHookNotFound
}

// EnableAllHooks enables all hooks.
func (p *Persistence) EnableAllHooks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := shared.Now()
	for i := range p.state.Hooks {
		p.state.Hooks[i].Enabled = true
		p.state.Hooks[i].UpdatedAt = now
	}

	return p.save()
}

// DisableAllHooks disables all hooks.
func (p *Persistence) DisableAllHooks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := shared.Now()
	for i := range p.state.Hooks {
		p.state.Hooks[i].Enabled = false
		p.state.Hooks[i].UpdatedAt = now
	}

	return p.save()
}

// UpdateHookStats updates the statistics for a hook.
func (p *Persistence) UpdateHookStats(id string, stats domainHooks.HookStats) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.state.Hooks {
		if p.state.Hooks[i].ID == id {
			p.state.Hooks[i].Stats = stats
			p.state.Hooks[i].UpdatedAt = shared.Now()
			return p.save()
		}
	}

	return shared.ErrHookNotFound
}

// GetPatterns returns all patterns.
func (p *Persistence) GetPatterns() []domainHooks.PatternState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	patterns := make([]domainHooks.PatternState, len(p.state.Patterns))
	copy(patterns, p.state.Patterns)
	return patterns
}

// AddPattern adds a pattern.
func (p *Persistence) AddPattern(pattern domainHooks.PatternState) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check max patterns
	if len(p.state.Patterns) >= p.state.Config.MaxPatterns {
		// Remove oldest pattern
		if len(p.state.Patterns) > 0 {
			p.state.Patterns = p.state.Patterns[1:]
		}
	}

	now := shared.Now()
	if pattern.CreatedAt == 0 {
		pattern.CreatedAt = now
	}
	pattern.UpdatedAt = now

	p.state.Patterns = append(p.state.Patterns, pattern)
	return p.save()
}

// ClearPatterns removes all patterns.
func (p *Persistence) ClearPatterns() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.state.Patterns = make([]domainHooks.PatternState, 0)
	return p.save()
}

// GetRouting returns the routing state.
func (p *Persistence) GetRouting() domainHooks.RoutingState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.Routing
}

// UpdateRouting updates the routing state.
func (p *Persistence) UpdateRouting(routing domainHooks.RoutingState) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.state.Routing = routing
	return p.save()
}

// ClearRouting clears routing history and stats.
func (p *Persistence) ClearRouting() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.state.Routing = domainHooks.RoutingState{
		AgentScores: make(map[string]map[string]float64),
		AgentStats:  make(map[string]domainHooks.AgentStatState),
		History:     make([]domainHooks.RoutingDecisionState, 0),
	}
	return p.save()
}

// Reset performs a reset based on options.
func (p *Persistence) Reset(options domainHooks.ResetOptions) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if options.All {
		p.state = domainHooks.NewHooksState()
		return p.save()
	}

	if options.HookID != "" {
		// Reset specific hook stats
		for i := range p.state.Hooks {
			if p.state.Hooks[i].ID == options.HookID || p.state.Hooks[i].Name == options.HookID {
				p.state.Hooks[i].Stats = domainHooks.HookStats{}
				p.state.Hooks[i].UpdatedAt = shared.Now()
				break
			}
		}
	}

	if options.Patterns {
		p.state.Patterns = make([]domainHooks.PatternState, 0)
	}

	if options.Routing {
		p.state.Routing = domainHooks.RoutingState{
			AgentScores: make(map[string]map[string]float64),
			AgentStats:  make(map[string]domainHooks.AgentStatState),
			History:     make([]domainHooks.RoutingDecisionState, 0),
		}
	}

	return p.save()
}

// GetBasePath returns the base path.
func (p *Persistence) GetBasePath() string {
	return p.basePath
}
