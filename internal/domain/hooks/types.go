// Package hooks provides domain types for the hooks system.
package hooks

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// HookDefinition represents a hook with its configuration and statistics.
type HookDefinition struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Type        shared.HookEvent    `json:"type"`
	Description string              `json:"description"`
	Priority    shared.HookPriority `json:"priority"`
	Enabled     bool                `json:"enabled"`
	Timeout     int64               `json:"timeout"` // milliseconds
	Config      map[string]any      `json:"config,omitempty"`
	Stats       HookStats           `json:"stats"`
	CreatedAt   int64               `json:"createdAt"`
	UpdatedAt   int64               `json:"updatedAt"`
}

// HookStats holds execution statistics for a hook.
type HookStats struct {
	ExecutionCount int64   `json:"executionCount"`
	SuccessCount   int64   `json:"successCount"`
	FailureCount   int64   `json:"failureCount"`
	AvgLatencyMs   float64 `json:"avgLatencyMs"`
	LastExecutedAt int64   `json:"lastExecutedAt"`
}

// GetSuccessRate returns the success rate as a percentage.
func (s HookStats) GetSuccessRate() float64 {
	if s.ExecutionCount == 0 {
		return 0
	}
	return float64(s.SuccessCount) / float64(s.ExecutionCount) * 100
}

// HooksState represents the persisted state of the hooks system.
type HooksState struct {
	Version   string             `json:"version"`
	UpdatedAt int64              `json:"updatedAt"`
	Config    shared.HooksConfig `json:"config"`
	Hooks     []HookDefinition   `json:"hooks"`
	Patterns  []PatternState     `json:"patterns"`
	Routing   RoutingState       `json:"routing"`
}

// PatternState represents a persisted pattern.
type PatternState struct {
	ID           string                 `json:"id"`
	Type         shared.PatternType     `json:"type"`
	Content      string                 `json:"content"`
	Keywords     []string               `json:"keywords"`
	SuccessCount int64                  `json:"successCount"`
	FailureCount int64                  `json:"failureCount"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    int64                  `json:"createdAt"`
	UpdatedAt    int64                  `json:"updatedAt"`
	LastUsedAt   int64                  `json:"lastUsedAt"`
}

// RoutingState represents persisted routing data.
type RoutingState struct {
	AgentScores   map[string]map[string]float64 `json:"agentScores"`
	AgentStats    map[string]AgentStatState     `json:"agentStats"`
	TotalRoutings int64                         `json:"totalRoutings"`
	SuccessCount  int64                         `json:"successCount"`
	History       []RoutingDecisionState        `json:"history,omitempty"`
}

// AgentStatState represents persisted agent routing statistics.
type AgentStatState struct {
	AgentType    string  `json:"agentType"`
	TasksRouted  int64   `json:"tasksRouted"`
	SuccessRate  float64 `json:"successRate"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
}

// RoutingDecisionState represents a persisted routing decision.
type RoutingDecisionState struct {
	ID              string  `json:"id"`
	Task            string  `json:"task"`
	TaskType        string  `json:"taskType"`
	SelectedAgent   string  `json:"selectedAgent"`
	Confidence      float64 `json:"confidence"`
	Success         bool    `json:"success"`
	Timestamp       int64   `json:"timestamp"`
	ExecutionTimeMs int64   `json:"executionTimeMs,omitempty"`
}

// TestInput represents input data for hook testing.
type TestInput struct {
	Event   shared.HookEvent       `json:"event"`
	Data    map[string]interface{} `json:"data"`
	DryRun  bool                   `json:"dryRun"`
	Verbose bool                   `json:"verbose"`
}

// TestResult represents the result of a hook test.
type TestResult struct {
	HookID      string        `json:"hookId"`
	HookName    string        `json:"hookName"`
	Success     bool          `json:"success"`
	DryRun      bool          `json:"dryRun"`
	ExecutionMs int64         `json:"executionMs"`
	Result      interface{}   `json:"result,omitempty"`
	Error       string        `json:"error,omitempty"`
	Logs        []string      `json:"logs,omitempty"`
}

// ConfigUpdate represents a configuration update request.
type ConfigUpdate struct {
	Timeout      *int64   `json:"timeout,omitempty"`
	MaxPatterns  *int     `json:"maxPatterns,omitempty"`
	LearningRate *float64 `json:"learningRate,omitempty"`
	Learning     *bool    `json:"learning,omitempty"`
}

// Apply applies the update to a config.
func (u ConfigUpdate) Apply(config *shared.HooksConfig) {
	if u.Timeout != nil {
		config.DefaultTimeoutMs = *u.Timeout
	}
	if u.MaxPatterns != nil {
		config.MaxPatterns = *u.MaxPatterns
	}
	if u.LearningRate != nil {
		config.LearningRate = *u.LearningRate
	}
	if u.Learning != nil {
		config.EnableLearning = *u.Learning
	}
}

// ResetOptions represents options for resetting hooks state.
type ResetOptions struct {
	HookID   string `json:"hookId,omitempty"`
	Patterns bool   `json:"patterns"`
	Routing  bool   `json:"routing"`
	All      bool   `json:"all"`
}

// DefaultHooks returns the default built-in hooks.
func DefaultHooks() []HookDefinition {
	now := shared.Now()
	return []HookDefinition{
		{
			ID:          "hook-pre-edit",
			Name:        "pre-edit",
			Type:        shared.HookEventPreEdit,
			Description: "Executes before file editing operations",
			Priority:    shared.HookPriorityNormal,
			Enabled:     true,
			Timeout:     5000,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "hook-post-edit",
			Name:        "post-edit",
			Type:        shared.HookEventPostEdit,
			Description: "Executes after file editing operations to record outcomes",
			Priority:    shared.HookPriorityNormal,
			Enabled:     true,
			Timeout:     5000,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "hook-pre-command",
			Name:        "pre-command",
			Type:        shared.HookEventPreCommand,
			Description: "Executes before shell commands to assess risk",
			Priority:    shared.HookPriorityHigh,
			Enabled:     true,
			Timeout:     3000,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "hook-post-command",
			Name:        "post-command",
			Type:        shared.HookEventPostCommand,
			Description: "Executes after shell commands to record outcomes",
			Priority:    shared.HookPriorityNormal,
			Enabled:     true,
			Timeout:     5000,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "hook-pre-route",
			Name:        "pre-route",
			Type:        shared.HookEventPreRoute,
			Description: "Executes before task routing decisions",
			Priority:    shared.HookPriorityNormal,
			Enabled:     true,
			Timeout:     3000,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "hook-post-route",
			Name:        "post-route",
			Type:        shared.HookEventPostRoute,
			Description: "Executes after task routing to record outcomes",
			Priority:    shared.HookPriorityLow,
			Enabled:     true,
			Timeout:     5000,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "hook-pattern-learned",
			Name:        "pattern-learned",
			Type:        shared.HookEventPatternLearned,
			Description: "Executes when a new pattern is learned",
			Priority:    shared.HookPriorityBackground,
			Enabled:     true,
			Timeout:     10000,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

// NewHooksState creates a new default hooks state.
func NewHooksState() *HooksState {
	return &HooksState{
		Version:   "1.0.0",
		UpdatedAt: shared.Now(),
		Config:    shared.DefaultHooksConfig(),
		Hooks:     DefaultHooks(),
		Patterns:  make([]PatternState, 0),
		Routing: RoutingState{
			AgentScores: make(map[string]map[string]float64),
			AgentStats:  make(map[string]AgentStatState),
			History:     make([]RoutingDecisionState, 0),
		},
	}
}
