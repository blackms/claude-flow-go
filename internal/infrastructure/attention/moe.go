// Package attention provides attention mechanisms for agent coordination.
package attention

import (
	"sort"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// MoERouter implements Mixture of Experts routing.
// Routes tasks to top-K experts based on gating scores.
// Target latency: <5ms
type MoERouter struct {
	config shared.MoEConfig
}

// NewMoERouter creates a new MoERouter instance.
func NewMoERouter(config shared.MoEConfig) *MoERouter {
	return &MoERouter{config: config}
}

// NewMoERouterWithDefaults creates a MoERouter with default configuration.
func NewMoERouterWithDefaults() *MoERouter {
	return NewMoERouter(shared.DefaultMoEConfig())
}

// RouteToExperts routes a task to the top-K experts.
func (m *MoERouter) RouteToExperts(taskEmbedding []float64, experts []shared.Expert) *shared.ExpertRoutingResult {
	startTime := shared.Now()

	if len(experts) == 0 {
		return &shared.ExpertRoutingResult{
			SelectedExperts:  []shared.ExpertSelection{},
			RoutingLatencyMs: 0,
			LoadBalanced:     false,
		}
	}

	// Compute gating scores for each expert
	scores := m.computeGatingScores(taskEmbedding, experts)

	// Apply load balancing penalty if enabled
	if m.config.LoadBalancingLoss {
		scores = m.applyLoadBalancing(scores, experts)
	}

	// Select top-K experts
	selectedExperts := m.selectTopK(scores, experts)

	latency := float64(shared.Now() - startTime)

	return &shared.ExpertRoutingResult{
		SelectedExperts:  selectedExperts,
		RoutingLatencyMs: latency,
		LoadBalanced:     m.config.LoadBalancingLoss,
	}
}

// computeGatingScores computes gating scores for each expert.
func (m *MoERouter) computeGatingScores(taskEmbedding []float64, experts []shared.Expert) []float64 {
	scores := make([]float64, len(experts))

	for i, expert := range experts {
		// Base score: similarity between task and expert embedding
		if len(expert.Embedding) > 0 && len(taskEmbedding) > 0 {
			scores[i] = CosineSimilarity(taskEmbedding, expert.Embedding)
		} else {
			// Fallback: generate embeddings
			expertEmb := GenerateEmbedding(expert.AgentID, 64)
			taskEmb := taskEmbedding
			if len(taskEmb) == 0 {
				taskEmb = make([]float64, 64)
			}
			scores[i] = CosineSimilarity(taskEmb, expertEmb)
		}

		// Boost score based on available capacity
		if expert.Capacity > 0 {
			capacityRatio := float64(expert.Capacity-expert.CurrentLoad) / float64(expert.Capacity)
			scores[i] *= (1.0 + 0.5*capacityRatio) // Boost up to 50% for empty experts
		}
	}

	// Apply softmax to get probability distribution
	return Softmax(scores)
}

// applyLoadBalancing applies load balancing penalty to scores.
func (m *MoERouter) applyLoadBalancing(scores []float64, experts []shared.Expert) []float64 {
	result := make([]float64, len(scores))

	for i, score := range scores {
		expert := experts[i]

		// Compute load penalty (0 = empty, 1 = full)
		loadPenalty := 0.0
		if expert.Capacity > 0 {
			loadPenalty = float64(expert.CurrentLoad) / float64(expert.Capacity)
		}

		// Apply penalty: reduce score for overloaded experts
		penaltyFactor := 1.0 - loadPenalty*0.3 // Max 30% reduction
		result[i] = score * penaltyFactor
	}

	// Re-normalize
	return Softmax(result)
}

// selectTopK selects the top-K experts based on scores.
func (m *MoERouter) selectTopK(scores []float64, experts []shared.Expert) []shared.ExpertSelection {
	k := m.config.TopK
	if k > len(experts) {
		k = len(experts)
	}

	// Create index-score pairs
	type indexScore struct {
		index int
		score float64
	}
	pairs := make([]indexScore, len(scores))
	for i, s := range scores {
		pairs[i] = indexScore{i, s}
	}

	// Sort by score descending
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].score > pairs[j].score
	})

	// Check capacity constraints
	selected := make([]shared.ExpertSelection, 0, k)
	totalWeight := 0.0

	for i := 0; i < len(pairs) && len(selected) < k; i++ {
		idx := pairs[i].index
		expert := experts[idx]

		// Check capacity
		effectiveCapacity := int(float64(expert.Capacity) * m.config.CapacityFactor)
		if expert.CurrentLoad >= effectiveCapacity && expert.Capacity > 0 {
			continue // Skip overloaded expert
		}

		selected = append(selected, shared.ExpertSelection{
			Expert: expert,
			Score:  pairs[i].score,
		})
		totalWeight += pairs[i].score
	}

	// Normalize weights
	if totalWeight > 0 {
		for i := range selected {
			selected[i].Weight = selected[i].Score / totalWeight
		}
	}

	return selected
}

// Coordinate performs MoE-based coordination on agent outputs.
// Treats agents as experts and routes based on expertise.
func (m *MoERouter) Coordinate(outputs []shared.AttentionAgentOutput) *shared.AttentionCoordinationResult {
	startTime := shared.Now()

	n := len(outputs)
	if n == 0 {
		return &shared.AttentionCoordinationResult{
			Mechanism: shared.AttentionMoE,
		}
	}

	// Convert outputs to experts
	experts := make([]shared.Expert, n)
	for i, output := range outputs {
		experts[i] = shared.Expert{
			AgentID:     output.AgentID,
			Embedding:   output.Embedding,
			Capacity:    100, // Default capacity
			CurrentLoad: 0,
		}
	}

	// Compute average embedding as "task"
	embeddingDim := 64
	if len(outputs[0].Embedding) > 0 {
		embeddingDim = len(outputs[0].Embedding)
	}
	taskEmbedding := make([]float64, embeddingDim)
	for _, output := range outputs {
		if len(output.Embedding) > 0 {
			for j := 0; j < len(output.Embedding) && j < embeddingDim; j++ {
				taskEmbedding[j] += output.Embedding[j]
			}
		}
	}
	for j := range taskEmbedding {
		taskEmbedding[j] /= float64(n)
	}

	// Route to top-K experts
	result := m.RouteToExperts(taskEmbedding, experts)

	// Convert to attention weights (sparse - only selected experts have non-zero weight)
	weights := make(map[string]float64)
	participants := make([]string, 0, len(outputs))

	for _, output := range outputs {
		participants = append(participants, output.AgentID)
		weights[output.AgentID] = 0 // Default zero
	}

	var bestOutput *shared.AttentionAgentOutput
	bestWeight := -1.0
	totalConfidence := 0.0

	for _, selection := range result.SelectedExperts {
		weights[selection.Expert.AgentID] = selection.Weight

		// Find the corresponding output
		for i := range outputs {
			if outputs[i].AgentID == selection.Expert.AgentID {
				if selection.Weight > bestWeight {
					bestWeight = selection.Weight
					bestOutput = &outputs[i]
				}
				totalConfidence += outputs[i].Confidence * selection.Weight
				break
			}
		}
	}

	var consensusOutput interface{}
	if bestOutput != nil {
		consensusOutput = bestOutput.Content
	}

	latency := float64(shared.Now() - startTime)

	return &shared.AttentionCoordinationResult{
		ConsensusOutput:  consensusOutput,
		AttentionWeights: weights,
		Confidence:       totalConfidence,
		Participants:     participants,
		LatencyMs:        latency,
		MemoryBytes:      int64(n * embeddingDim * 8),
		Mechanism:        shared.AttentionMoE,
	}
}

// GetConfig returns the MoE configuration.
func (m *MoERouter) GetConfig() shared.MoEConfig {
	return m.config
}

// ComputeLoadBalancingLoss computes the auxiliary load balancing loss.
// This can be used during training to encourage balanced expert usage.
func (m *MoERouter) ComputeLoadBalancingLoss(selections []shared.ExpertSelection, totalExperts int) float64 {
	if len(selections) == 0 || totalExperts == 0 {
		return 0
	}

	// Compute fraction of tokens routed to each expert
	fractions := make([]float64, totalExperts)
	for _, sel := range selections {
		// Assume each expert has an index (simplified)
		fractions[0] += sel.Weight
	}

	// Compute average gating score per expert
	avgGate := 1.0 / float64(totalExperts)

	// Loss = sum of (fraction × avgGate) - encourages uniform distribution
	loss := 0.0
	for _, f := range fractions {
		loss += f * avgGate
	}

	return loss * float64(totalExperts)
}
