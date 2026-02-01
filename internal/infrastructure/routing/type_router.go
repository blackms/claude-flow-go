// Package routing provides task-to-agent type routing.
package routing

import (
	"sort"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/pool"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ModelTier represents the model tier for routing.
type ModelTier = agent.ModelTier

const (
	ModelTierOpus   = agent.ModelTierOpus
	ModelTierSonnet = agent.ModelTierSonnet
	ModelTierHaiku  = agent.ModelTierHaiku
)

// RoutingResult holds the result of a routing decision.
type RoutingResult struct {
	AgentType       shared.AgentType `json:"agentType"`
	ModelTier       ModelTier        `json:"modelTier"`
	Score           float64          `json:"score"`
	FallbackTypes   []shared.AgentType `json:"fallbackTypes,omitempty"`
	Reason          string           `json:"reason,omitempty"`
}

// TypeRouter routes tasks to appropriate agent types.
type TypeRouter struct {
	mu       sync.RWMutex
	registry *agent.AgentTypeRegistry
	pools    *pool.AgentPoolManager

	// Routing preferences
	preferAvailable bool
	maxFallbacks    int

	// Model tier defaults
	defaultModelTier ModelTier
}

// NewTypeRouter creates a new TypeRouter.
func NewTypeRouter(registry *agent.AgentTypeRegistry, pools *pool.AgentPoolManager) *TypeRouter {
	return &TypeRouter{
		registry:         registry,
		pools:            pools,
		preferAvailable:  true,
		maxFallbacks:     3,
		defaultModelTier: ModelTierSonnet,
	}
}

// RouteTask routes a task to the best agent type.
func (tr *TypeRouter) RouteTask(task shared.Task) (*RoutingResult, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	// Extract required capabilities from task
	requiredCaps := tr.extractCapabilities(task)

	// Find best matching agent type
	bestType, score := tr.registry.FindBestMatch(requiredCaps)

	if bestType == "" {
		// No match found, use default based on task type
		bestType = tr.getDefaultForTaskType(task.Type)
		score = 0.5
	}

	// Get model tier
	modelTier := tr.registry.GetModelTier(bestType)
	if modelTier == "" {
		modelTier = tr.defaultModelTier
	}

	// Find fallback types
	fallbacks := tr.findFallbacks(requiredCaps, bestType)

	return &RoutingResult{
		AgentType:     bestType,
		ModelTier:     modelTier,
		Score:         score,
		FallbackTypes: fallbacks,
		Reason:        "capability-based routing",
	}, nil
}

// extractCapabilities extracts required capabilities from a task.
func (tr *TypeRouter) extractCapabilities(task shared.Task) []string {
	caps := make([]string, 0)

	// Add capabilities from task metadata
	if task.Metadata != nil {
		if reqCaps, ok := task.Metadata["requiredCapabilities"].([]string); ok {
			caps = append(caps, reqCaps...)
		}
		if reqCaps, ok := task.Metadata["requiredCapabilities"].([]interface{}); ok {
			for _, c := range reqCaps {
				if s, ok := c.(string); ok {
					caps = append(caps, s)
				}
			}
		}
	}

	// Map task type to capability
	switch task.Type {
	case shared.TaskTypeCode:
		caps = append(caps, "code-generation", "refactoring")
	case shared.TaskTypeTest:
		caps = append(caps, "unit-testing", "integration-testing")
	case shared.TaskTypeReview:
		caps = append(caps, "code-review", "security-audit")
	case shared.TaskTypeDesign:
		caps = append(caps, "system-design", "architecture")
	case shared.TaskTypeDeploy:
		caps = append(caps, "deployment", "ci-cd")
	}

	return caps
}

// getDefaultForTaskType returns the default agent type for a task type.
func (tr *TypeRouter) getDefaultForTaskType(taskType shared.TaskType) shared.AgentType {
	switch taskType {
	case shared.TaskTypeCode:
		return shared.AgentTypeCoder
	case shared.TaskTypeTest:
		return shared.AgentTypeTester
	case shared.TaskTypeReview:
		return shared.AgentTypeReviewer
	case shared.TaskTypeDesign:
		return shared.AgentTypeArchitect
	case shared.TaskTypeDeploy:
		return shared.AgentTypeDeployer
	default:
		return shared.AgentTypeCoder
	}
}

// findFallbacks finds fallback agent types.
func (tr *TypeRouter) findFallbacks(requiredCaps []string, exclude shared.AgentType) []shared.AgentType {
	type scoredType struct {
		agentType shared.AgentType
		score     float64
	}

	allTypes := tr.registry.ListAll()
	scored := make([]scoredType, 0, len(allTypes))

	for _, t := range allTypes {
		if t == exclude {
			continue
		}

		caps := tr.registry.GetCapabilities(t)
		score := tr.calculateScore(caps, requiredCaps)

		if score > 0 {
			scored = append(scored, scoredType{t, score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Take top N fallbacks
	maxFallbacks := tr.maxFallbacks
	if maxFallbacks > len(scored) {
		maxFallbacks = len(scored)
	}

	fallbacks := make([]shared.AgentType, maxFallbacks)
	for i := 0; i < maxFallbacks; i++ {
		fallbacks[i] = scored[i].agentType
	}

	return fallbacks
}

// calculateScore calculates the capability match score.
func (tr *TypeRouter) calculateScore(capabilities, requirements []string) float64 {
	if len(requirements) == 0 {
		return 0
	}

	capSet := make(map[string]bool)
	for _, cap := range capabilities {
		capSet[cap] = true
	}

	matches := 0
	for _, req := range requirements {
		if capSet[req] {
			matches++
		}
	}

	return float64(matches) / float64(len(requirements))
}

// SelectModel returns the recommended model for an agent type.
func (tr *TypeRouter) SelectModel(agentType shared.AgentType) string {
	tier := tr.registry.GetModelTier(agentType)

	switch tier {
	case ModelTierOpus:
		return "claude-opus"
	case ModelTierHaiku:
		return "claude-haiku"
	default:
		return "claude-sonnet"
	}
}

// FindBestType finds the best agent type for the given capabilities.
func (tr *TypeRouter) FindBestType(capabilities []string) (shared.AgentType, float64) {
	return tr.registry.FindBestMatch(capabilities)
}

// GetAvailableType returns the best available agent type.
// If preferAvailable is true, it checks pool availability.
func (tr *TypeRouter) GetAvailableType(requiredCaps []string) (shared.AgentType, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	bestType, score := tr.registry.FindBestMatch(requiredCaps)

	if score == 0 {
		return "", shared.ErrInvalidAgentType
	}

	if !tr.preferAvailable || tr.pools == nil {
		return bestType, nil
	}

	// Check if pool has available agents
	stats := tr.pools.GetPoolStats(bestType)
	if stats.IdleAgents > 0 || stats.TotalAgents < stats.MaxAgents {
		return bestType, nil
	}

	// Try fallbacks
	fallbacks := tr.findFallbacks(requiredCaps, bestType)
	for _, fb := range fallbacks {
		stats := tr.pools.GetPoolStats(fb)
		if stats.IdleAgents > 0 || stats.TotalAgents < stats.MaxAgents {
			return fb, nil
		}
	}

	// Return best type even if not immediately available
	return bestType, nil
}

// RouteByCapability routes to the best agent type for a single capability.
func (tr *TypeRouter) RouteByCapability(capability string) []shared.AgentType {
	return tr.registry.ListByCapability(capability)
}

// RouteByTag routes to agent types with a specific tag.
func (tr *TypeRouter) RouteByTag(tag string) []shared.AgentType {
	return tr.registry.ListByTag(tag)
}

// RouteByDomain routes to agent types in a specific domain.
func (tr *TypeRouter) RouteByDomain(domain shared.AgentDomain) []shared.AgentType {
	return tr.registry.ListByDomain(domain)
}

// RouteByModelTier routes to agent types for a specific model tier.
func (tr *TypeRouter) RouteByModelTier(tier ModelTier) []shared.AgentType {
	return tr.registry.ListByModelTier(tier)
}

// SetPreferAvailable sets whether to prefer available agents.
func (tr *TypeRouter) SetPreferAvailable(prefer bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.preferAvailable = prefer
}

// SetMaxFallbacks sets the maximum number of fallback types.
func (tr *TypeRouter) SetMaxFallbacks(max int) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.maxFallbacks = max
}

// SetDefaultModelTier sets the default model tier.
func (tr *TypeRouter) SetDefaultModelTier(tier ModelTier) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.defaultModelTier = tier
}

// GetRoutingStats returns routing statistics.
func (tr *TypeRouter) GetRoutingStats() RoutingStats {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return RoutingStats{
		TotalTypes:       tr.registry.Count(),
		PreferAvailable:  tr.preferAvailable,
		MaxFallbacks:     tr.maxFallbacks,
		DefaultModelTier: tr.defaultModelTier,
	}
}

// RoutingStats holds routing statistics.
type RoutingStats struct {
	TotalTypes       int       `json:"totalTypes"`
	PreferAvailable  bool      `json:"preferAvailable"`
	MaxFallbacks     int       `json:"maxFallbacks"`
	DefaultModelTier ModelTier `json:"defaultModelTier"`
}
