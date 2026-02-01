// Package attention provides attention mechanisms for agent coordination.
package attention

import (
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// EventHandler is a callback for attention coordination events.
type EventHandler func(event AttentionEvent)

// AttentionEvent represents an event in the attention coordination.
type AttentionEvent struct {
	Type      string                 `json:"type"`
	Mechanism shared.AttentionMechanism `json:"mechanism"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// AttentionCoordinator is the main coordinator for attention mechanisms.
type AttentionCoordinator struct {
	config shared.AttentionConfig

	// Attention mechanisms
	flash      *FlashAttention
	multiHead  *MultiHeadAttention
	linear     *LinearAttention
	hyperbolic *HyperbolicAttention
	moe        *MoERouter
	graphRope  *GraphRoPE

	// Performance tracking
	stats shared.AttentionPerformanceStats
	mu    sync.RWMutex

	// Event handling
	eventHandler EventHandler
}

// NewAttentionCoordinator creates a new AttentionCoordinator with the given configuration.
func NewAttentionCoordinator(config shared.AttentionConfig) *AttentionCoordinator {
	return &AttentionCoordinator{
		config:     config,
		flash:      NewFlashAttention(config.Flash),
		multiHead:  NewMultiHeadAttention(config.MultiHead),
		linear:     NewLinearAttention(config.Linear),
		hyperbolic: NewHyperbolicAttention(config.Hyperbolic),
		moe:        NewMoERouter(config.MoE),
		graphRope:  NewGraphRoPE(config.GraphRoPE),
	}
}

// NewAttentionCoordinatorWithDefaults creates an AttentionCoordinator with default configuration.
func NewAttentionCoordinatorWithDefaults() *AttentionCoordinator {
	return NewAttentionCoordinator(shared.DefaultAttentionConfig())
}

// Coordinate dispatches coordination to the appropriate attention mechanism.
func (ac *AttentionCoordinator) Coordinate(outputs []shared.AttentionAgentOutput) *shared.AttentionCoordinationResult {
	return ac.CoordinateWithMechanism(outputs, ac.config.DefaultMechanism)
}

// CoordinateWithMechanism coordinates using a specific attention mechanism.
func (ac *AttentionCoordinator) CoordinateWithMechanism(outputs []shared.AttentionAgentOutput, mechanism shared.AttentionMechanism) *shared.AttentionCoordinationResult {
	ac.emitEvent(AttentionEvent{
		Type:      "coordination_started",
		Mechanism: mechanism,
		Data:      map[string]interface{}{"numOutputs": len(outputs)},
		Timestamp: shared.Now(),
	})

	var result *shared.AttentionCoordinationResult

	switch mechanism {
	case shared.AttentionFlash:
		result = ac.flash.Coordinate(outputs)
	case shared.AttentionMultiHead:
		result = ac.multiHead.Coordinate(outputs)
	case shared.AttentionLinear:
		result = ac.linear.Coordinate(outputs)
	case shared.AttentionHyperbolic:
		result = ac.hyperbolic.Coordinate(outputs)
	case shared.AttentionMoE:
		result = ac.moe.Coordinate(outputs)
	case shared.AttentionGraphRoPE:
		result = ac.graphRope.Coordinate(outputs)
	default:
		// Default to Flash Attention
		result = ac.flash.Coordinate(outputs)
	}

	// Update stats
	ac.updateStats(result)

	ac.emitEvent(AttentionEvent{
		Type:      "coordination_completed",
		Mechanism: mechanism,
		Data: map[string]interface{}{
			"latencyMs":   result.LatencyMs,
			"memoryBytes": result.MemoryBytes,
			"confidence":  result.Confidence,
		},
		Timestamp: shared.Now(),
	})

	return result
}

// RouteToExperts routes a task to experts using MoE.
func (ac *AttentionCoordinator) RouteToExperts(taskEmbedding []float64, experts []shared.Expert) *shared.ExpertRoutingResult {
	return ac.moe.RouteToExperts(taskEmbedding, experts)
}

// TopologyAwareCoordination performs GraphRoPE-based coordination.
func (ac *AttentionCoordinator) TopologyAwareCoordination(outputs []shared.AttentionAgentOutput, topology map[string][]string) *shared.AttentionCoordinationResult {
	ac.graphRope.SetTopology(topology)
	return ac.graphRope.Coordinate(outputs)
}

// HierarchicalCoordination performs hyperbolic attention for hierarchical swarms.
func (ac *AttentionCoordinator) HierarchicalCoordination(outputs []shared.AttentionAgentOutput, roles map[string]string) *shared.AttentionCoordinationResult {
	ac.hyperbolic.SetRoles(roles)
	return ac.hyperbolic.Coordinate(outputs)
}

// updateStats updates the performance statistics.
func (ac *AttentionCoordinator) updateStats(result *shared.AttentionCoordinationResult) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.stats.TotalCoordinations++

	// Update average latency
	n := float64(ac.stats.TotalCoordinations)
	ac.stats.AvgLatencyMs = (ac.stats.AvgLatencyMs*(n-1) + result.LatencyMs) / n

	// Update average memory
	ac.stats.AvgMemoryBytes = (ac.stats.AvgMemoryBytes*(int64(n)-1) + result.MemoryBytes) / int64(n)

	// Estimate speedup for Flash Attention
	if result.Mechanism == shared.AttentionFlash && len(result.Participants) > 0 {
		ac.stats.SpeedupFactor = ac.flash.EstimateSpeedup(len(result.Participants))

		// Estimate memory reduction
		flashBytes, standardBytes := ac.flash.EstimateMemoryUsage(len(result.Participants), 64)
		if standardBytes > 0 {
			ac.stats.MemoryReduction = 1.0 - float64(flashBytes)/float64(standardBytes)
		}
	}
}

// GetStats returns the performance statistics.
func (ac *AttentionCoordinator) GetStats() shared.AttentionPerformanceStats {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.stats
}

// GetConfig returns the current configuration.
func (ac *AttentionCoordinator) GetConfig() shared.AttentionConfig {
	return ac.config
}

// SetEventHandler sets the event handler for coordination events.
func (ac *AttentionCoordinator) SetEventHandler(handler EventHandler) {
	ac.eventHandler = handler
}

// emitEvent emits a coordination event.
func (ac *AttentionCoordinator) emitEvent(event AttentionEvent) {
	if ac.eventHandler != nil {
		go ac.eventHandler(event)
	}
}

// SetTopology sets the topology for GraphRoPE.
func (ac *AttentionCoordinator) SetTopology(adjacency map[string][]string) {
	ac.graphRope.SetTopology(adjacency)
}

// SetTopologyFromEdges sets the topology from edges for GraphRoPE.
func (ac *AttentionCoordinator) SetTopologyFromEdges(edges []shared.TopologyEdge) {
	ac.graphRope.SetTopologyFromEdges(edges)
}

// SetRoles sets the agent roles for hyperbolic attention.
func (ac *AttentionCoordinator) SetRoles(roles map[string]string) {
	ac.hyperbolic.SetRoles(roles)
}

// GetFlashAttention returns the Flash Attention mechanism for direct access.
func (ac *AttentionCoordinator) GetFlashAttention() *FlashAttention {
	return ac.flash
}

// GetMultiHeadAttention returns the Multi-Head Attention mechanism for direct access.
func (ac *AttentionCoordinator) GetMultiHeadAttention() *MultiHeadAttention {
	return ac.multiHead
}

// GetLinearAttention returns the Linear Attention mechanism for direct access.
func (ac *AttentionCoordinator) GetLinearAttention() *LinearAttention {
	return ac.linear
}

// GetHyperbolicAttention returns the Hyperbolic Attention mechanism for direct access.
func (ac *AttentionCoordinator) GetHyperbolicAttention() *HyperbolicAttention {
	return ac.hyperbolic
}

// GetMoERouter returns the MoE Router for direct access.
func (ac *AttentionCoordinator) GetMoERouter() *MoERouter {
	return ac.moe
}

// GetGraphRoPE returns the GraphRoPE mechanism for direct access.
func (ac *AttentionCoordinator) GetGraphRoPE() *GraphRoPE {
	return ac.graphRope
}

// BenchmarkMechanisms benchmarks all mechanisms with the same input.
func (ac *AttentionCoordinator) BenchmarkMechanisms(outputs []shared.AttentionAgentOutput) map[shared.AttentionMechanism]*shared.AttentionCoordinationResult {
	results := make(map[shared.AttentionMechanism]*shared.AttentionCoordinationResult)

	mechanisms := []shared.AttentionMechanism{
		shared.AttentionFlash,
		shared.AttentionMultiHead,
		shared.AttentionLinear,
		shared.AttentionHyperbolic,
		shared.AttentionMoE,
		shared.AttentionGraphRoPE,
	}

	for _, mechanism := range mechanisms {
		result := ac.CoordinateWithMechanism(outputs, mechanism)
		results[mechanism] = result
	}

	return results
}

// RecommendMechanism recommends the best mechanism based on input characteristics.
func (ac *AttentionCoordinator) RecommendMechanism(numAgents int, hasTopology bool, isHierarchical bool, needsFastRouting bool) shared.AttentionMechanism {
	// MoE for fast routing to experts
	if needsFastRouting {
		return shared.AttentionMoE
	}

	// GraphRoPE for topology-aware coordination
	if hasTopology {
		return shared.AttentionGraphRoPE
	}

	// Hyperbolic for hierarchical swarms
	if isHierarchical {
		return shared.AttentionHyperbolic
	}

	// Linear for very long sequences
	if numAgents > 1000 {
		return shared.AttentionLinear
	}

	// Flash for general efficiency (best for most cases)
	if numAgents > 10 {
		return shared.AttentionFlash
	}

	// Multi-head for small, detailed coordination
	return shared.AttentionMultiHead
}
