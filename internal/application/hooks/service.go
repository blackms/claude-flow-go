// Package hooks provides application services for the hooks system.
package hooks

import (
	"context"
	"fmt"
	"time"

	domainHooks "github.com/anthropics/claude-flow-go/internal/domain/hooks"
	infraHooks "github.com/anthropics/claude-flow-go/internal/infrastructure/hooks"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Service orchestrates hooks operations.
type Service struct {
	manager     *infraHooks.HooksManager
	persistence *infraHooks.Persistence
}

// NewService creates a new hooks service.
func NewService(basePath string) (*Service, error) {
	persistence, err := infraHooks.NewPersistence(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistence: %w", err)
	}

	// Load config from persistence
	config := persistence.GetConfig()
	manager := infraHooks.NewHooksManager(config)

	// Register hooks from persistence
	hooks := persistence.GetHooks()
	for _, hook := range hooks {
		if hook.Enabled {
			reg := &shared.HookRegistration{
				ID:          hook.ID,
				Name:        hook.Name,
				Event:       hook.Type,
				Priority:    hook.Priority,
				Enabled:     hook.Enabled,
				Description: hook.Description,
			}
			manager.Register(reg)
		}
	}

	if err := manager.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize manager: %w", err)
	}

	return &Service{
		manager:     manager,
		persistence: persistence,
	}, nil
}

// Close shuts down the service.
func (s *Service) Close() error {
	return s.manager.Shutdown()
}

// ListHooks returns all hooks with optional filtering.
func (s *Service) ListHooks(hookType shared.HookEvent, enabledOnly, disabledOnly bool) []domainHooks.HookDefinition {
	hooks := s.persistence.GetHooks()

	var result []domainHooks.HookDefinition
	for _, hook := range hooks {
		// Filter by type
		if hookType != "" && hook.Type != hookType {
			continue
		}

		// Filter by enabled status
		if enabledOnly && !hook.Enabled {
			continue
		}
		if disabledOnly && hook.Enabled {
			continue
		}

		result = append(result, hook)
	}

	return result
}

// GetHook returns a hook by ID or name.
func (s *Service) GetHook(idOrName string) *domainHooks.HookDefinition {
	return s.persistence.GetHook(idOrName)
}

// EnableHook enables a hook.
func (s *Service) EnableHook(idOrName string) error {
	if err := s.persistence.EnableHook(idOrName); err != nil {
		return err
	}

	// Update manager
	hook := s.persistence.GetHook(idOrName)
	if hook != nil {
		return s.manager.EnableHook(hook.ID)
	}
	return nil
}

// DisableHook disables a hook.
func (s *Service) DisableHook(idOrName string) error {
	if err := s.persistence.DisableHook(idOrName); err != nil {
		return err
	}

	// Update manager
	hook := s.persistence.GetHook(idOrName)
	if hook != nil {
		return s.manager.DisableHook(hook.ID)
	}
	return nil
}

// EnableAllHooks enables all hooks.
func (s *Service) EnableAllHooks() error {
	if err := s.persistence.EnableAllHooks(); err != nil {
		return err
	}

	// Re-register all hooks
	hooks := s.persistence.GetHooks()
	for _, hook := range hooks {
		s.manager.EnableHook(hook.ID)
	}

	return nil
}

// DisableAllHooks disables all hooks.
func (s *Service) DisableAllHooks() error {
	if err := s.persistence.DisableAllHooks(); err != nil {
		return err
	}

	hooks := s.persistence.GetHooks()
	for _, hook := range hooks {
		s.manager.DisableHook(hook.ID)
	}

	return nil
}

// GetConfig returns the current configuration.
func (s *Service) GetConfig() shared.HooksConfig {
	return s.persistence.GetConfig()
}

// UpdateConfig updates the configuration.
func (s *Service) UpdateConfig(update domainHooks.ConfigUpdate) error {
	return s.persistence.UpdateConfig(update)
}

// GetStats returns hooks statistics.
func (s *Service) GetStats(hookID string) (*StatsResult, error) {
	metrics := s.manager.GetMetrics()

	result := &StatsResult{
		TotalExecutions:      metrics.TotalExecutions,
		SuccessfulExecutions: metrics.SuccessfulExecutions,
		FailedExecutions:     metrics.FailedExecutions,
		AvgExecutionMs:       metrics.AvgExecutionMs,
		PatternCount:         metrics.PatternCount,
		RoutingCount:         metrics.RoutingCount,
		RoutingSuccessRate:   metrics.RoutingSuccessRate,
		HooksByEvent:         metrics.HooksByEvent,
	}

	if hookID != "" {
		hook := s.persistence.GetHook(hookID)
		if hook == nil {
			return nil, shared.ErrHookNotFound
		}
		result.HookStats = &hook.Stats
	}

	// Get pattern stats
	patterns := s.persistence.GetPatterns()
	result.EditPatterns = 0
	result.CommandPatterns = 0
	for _, p := range patterns {
		switch p.Type {
		case shared.PatternTypeEdit:
			result.EditPatterns++
		case shared.PatternTypeCommand:
			result.CommandPatterns++
		}
	}

	// Get routing stats
	routing := s.persistence.GetRouting()
	result.RoutingCount = routing.TotalRoutings
	if routing.TotalRoutings > 0 {
		result.RoutingSuccessRate = float64(routing.SuccessCount) / float64(routing.TotalRoutings)
	}

	return result, nil
}

// StatsResult holds statistics results.
type StatsResult struct {
	TotalExecutions      int64                     `json:"totalExecutions"`
	SuccessfulExecutions int64                     `json:"successfulExecutions"`
	FailedExecutions     int64                     `json:"failedExecutions"`
	AvgExecutionMs       float64                   `json:"avgExecutionMs"`
	PatternCount         int64                     `json:"patternCount"`
	EditPatterns         int64                     `json:"editPatterns"`
	CommandPatterns      int64                     `json:"commandPatterns"`
	RoutingCount         int64                     `json:"routingCount"`
	RoutingSuccessRate   float64                   `json:"routingSuccessRate"`
	HooksByEvent         map[shared.HookEvent]int64 `json:"hooksByEvent"`
	HookStats            *domainHooks.HookStats    `json:"hookStats,omitempty"`
}

// TestHook tests a hook with provided input.
func (s *Service) TestHook(hookID string, input domainHooks.TestInput) (*domainHooks.TestResult, error) {
	hook := s.persistence.GetHook(hookID)
	if hook == nil {
		return nil, shared.ErrHookNotFound
	}

	result := &domainHooks.TestResult{
		HookID:   hook.ID,
		HookName: hook.Name,
		DryRun:   input.DryRun,
		Logs:     make([]string, 0),
	}

	start := time.Now()

	if input.DryRun {
		// Dry run - just validate and simulate
		result.Logs = append(result.Logs, fmt.Sprintf("Hook: %s (%s)", hook.Name, hook.ID))
		result.Logs = append(result.Logs, fmt.Sprintf("Type: %s", hook.Type))
		result.Logs = append(result.Logs, fmt.Sprintf("Priority: %d", hook.Priority))
		result.Logs = append(result.Logs, fmt.Sprintf("Enabled: %v", hook.Enabled))
		result.Logs = append(result.Logs, fmt.Sprintf("Timeout: %dms", hook.Timeout))

		if !hook.Enabled {
			result.Logs = append(result.Logs, "WARNING: Hook is disabled")
		}

		result.Success = true
		result.Result = map[string]interface{}{
			"message": "Dry run completed successfully",
			"hook":    hook.Name,
			"event":   hook.Type,
		}
	} else {
		// Actually execute the hook
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(hook.Timeout)*time.Millisecond)
		defer cancel()

		hookCtx := &shared.HookContext{
			Event:     hook.Type,
			Data:      input.Data,
			Timestamp: shared.Now(),
		}

		execResult, err := s.manager.Execute(ctx, hookCtx)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = execResult.Failed == 0
			result.Result = execResult
			if execResult.Failed > 0 {
				result.Error = fmt.Sprintf("%d hooks failed", execResult.Failed)
			}
		}

		result.Logs = append(result.Logs, fmt.Sprintf("Executed %d hooks", execResult.HooksRun))
		result.Logs = append(result.Logs, fmt.Sprintf("Successful: %d", execResult.Successful))
		result.Logs = append(result.Logs, fmt.Sprintf("Failed: %d", execResult.Failed))
	}

	result.ExecutionMs = time.Since(start).Milliseconds()

	return result, nil
}

// Reset resets hooks state based on options.
func (s *Service) Reset(options domainHooks.ResetOptions) error {
	if err := s.persistence.Reset(options); err != nil {
		return err
	}

	if options.All {
		// Reinitialize manager with default config
		s.manager.Shutdown()
		config := shared.DefaultHooksConfig()
		s.manager = infraHooks.NewHooksManager(config)
		s.manager.Initialize()
	}

	if options.Patterns {
		s.manager.GetPatternStore().Clear()
	}

	return nil
}

// GetManager returns the underlying hooks manager.
func (s *Service) GetManager() *infraHooks.HooksManager {
	return s.manager
}

// GetPersistence returns the persistence layer.
func (s *Service) GetPersistence() *infraHooks.Persistence {
	return s.persistence
}

// SyncFromManager syncs state from the manager to persistence.
func (s *Service) SyncFromManager() error {
	metrics := s.manager.GetMetrics()

	// Update hook stats from manager
	hooks := s.persistence.GetHooks()
	managerHooks := s.manager.ListHooks("", true)

	for _, mh := range managerHooks {
		for i := range hooks {
			if hooks[i].ID == mh.ID {
				hooks[i].Stats.ExecutionCount = mh.ExecutionCount
				hooks[i].Stats.AvgLatencyMs = mh.AvgExecutionMs
				hooks[i].Stats.LastExecutedAt = mh.LastExecutedAt
				s.persistence.UpdateHook(hooks[i])
				break
			}
		}
	}

	// Update patterns from pattern store
	patternStore := s.manager.GetPatternStore()
	patterns := patternStore.ListByType("", 0)
	for _, p := range patterns {
		patternState := domainHooks.PatternState{
			ID:           p.ID,
			Type:         p.Type,
			Content:      p.Content,
			Keywords:     p.Keywords,
			SuccessCount: p.SuccessCount,
			FailureCount: p.FailureCount,
			Metadata:     p.Metadata,
			CreatedAt:    p.CreatedAt,
			UpdatedAt:    p.UpdatedAt,
			LastUsedAt:   p.LastUsedAt,
		}
		s.persistence.AddPattern(patternState)
	}

	// Update routing state
	routingEngine := s.manager.GetRoutingEngine()
	agentStats := routingEngine.GetAgentStats()
	routingState := domainHooks.RoutingState{
		AgentScores:   make(map[string]map[string]float64),
		AgentStats:    make(map[string]domainHooks.AgentStatState),
		TotalRoutings: routingEngine.GetRoutingCount(),
	}

	for agentType, stats := range agentStats {
		routingState.AgentStats[agentType] = domainHooks.AgentStatState{
			AgentType:    stats.AgentType,
			TasksRouted:  stats.TasksRouted,
			SuccessRate:  stats.SuccessRate,
			AvgLatencyMs: stats.AvgLatencyMs,
		}
	}

	// Add routing history
	history := routingEngine.GetHistory(100)
	for _, h := range history {
		routingState.History = append(routingState.History, domainHooks.RoutingDecisionState{
			ID:              h.ID,
			Task:            h.Task,
			TaskType:        h.TaskType,
			SelectedAgent:   h.SelectedAgent,
			Confidence:      h.Confidence,
			Success:         h.Success,
			Timestamp:       h.Timestamp,
			ExecutionTimeMs: h.ExecutionTimeMs,
		})
	}

	// Calculate success rate
	if routingState.TotalRoutings > 0 {
		successCount := int64(0)
		for _, h := range routingState.History {
			if h.Success {
				successCount++
			}
		}
		routingState.SuccessCount = successCount
	}

	s.persistence.UpdateRouting(routingState)

	// Update metrics
	_ = metrics // Could store additional metrics if needed

	return s.persistence.Save()
}
