// Package attention provides attention mechanisms for agent coordination.
package attention

import (
	"math"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// HyperbolicAttention implements attention in hyperbolic space.
// Uses Poincaré distance for hierarchical data structures.
// Best for: Queen-worker hierarchies, tree-structured swarms.
type HyperbolicAttention struct {
	config shared.HyperbolicAttentionConfig
	roles  map[string]string // agentID -> role (queen, worker, etc.)
}

// NewHyperbolicAttention creates a new HyperbolicAttention instance.
func NewHyperbolicAttention(config shared.HyperbolicAttentionConfig) *HyperbolicAttention {
	return &HyperbolicAttention{
		config: config,
		roles:  make(map[string]string),
	}
}

// NewHyperbolicAttentionWithDefaults creates a HyperbolicAttention with default configuration.
func NewHyperbolicAttentionWithDefaults() *HyperbolicAttention {
	return NewHyperbolicAttention(shared.DefaultHyperbolicAttentionConfig())
}

// SetRole sets the role for an agent (used for hierarchical weighting).
func (ha *HyperbolicAttention) SetRole(agentID, role string) {
	ha.roles[agentID] = role
}

// SetRoles sets roles for multiple agents.
func (ha *HyperbolicAttention) SetRoles(roles map[string]string) {
	ha.roles = roles
}

// Coordinate performs Hyperbolic Attention coordination on agent outputs.
func (ha *HyperbolicAttention) Coordinate(outputs []shared.AttentionAgentOutput) *shared.AttentionCoordinationResult {
	startTime := shared.Now()

	n := len(outputs)
	if n == 0 {
		return &shared.AttentionCoordinationResult{
			Mechanism: shared.AttentionHyperbolic,
		}
	}

	// Ensure all outputs have embeddings
	embeddingDim := ha.config.Dimension
	embeddings := make([][]float64, n)
	for i, output := range outputs {
		if len(output.Embedding) > 0 {
			embeddings[i] = output.Embedding
		} else {
			embeddings[i] = GenerateEmbedding(output.Content, embeddingDim)
		}
		// Project to Poincaré ball (ensure ||x|| < 1)
		embeddings[i] = ha.projectToPoincareBall(embeddings[i])
	}

	// Compute hyperbolic attention
	weights := ha.computeHyperbolicAttention(embeddings, outputs)

	// Build consensus
	result := ha.buildConsensus(outputs, weights)

	// Calculate memory usage
	memoryBytes := int64(n * embeddingDim * 8)

	latency := float64(shared.Now() - startTime)

	result.LatencyMs = latency
	result.MemoryBytes = memoryBytes
	result.Mechanism = shared.AttentionHyperbolic

	return result
}

// projectToPoincareBall projects a vector onto the Poincaré ball (||x|| < 1).
func (ha *HyperbolicAttention) projectToPoincareBall(v []float64) []float64 {
	norm := L2Norm(v)
	if norm == 0 {
		return v
	}

	// Scale to be inside the ball with some margin
	maxNorm := 0.99
	if norm >= maxNorm {
		scale := maxNorm / norm
		result := make([]float64, len(v))
		for i, x := range v {
			result[i] = x * scale
		}
		return result
	}

	return v
}

// poincareDistance computes the Poincaré distance between two points.
// Formula: d(x, y) = acosh(1 + 2c × ||x - y||² / ((1 - c||x||²)(1 - c||y||²)))
// where c = -curvature (positive for negative curvature space)
func (ha *HyperbolicAttention) poincareDistance(x, y []float64) float64 {
	c := -ha.config.Curvature // Convert to positive for computation
	if c <= 0 {
		c = 1.0 // Default curvature
	}

	// Compute ||x||², ||y||², and ||x - y||²
	normXSq := 0.0
	normYSq := 0.0
	normDiffSq := 0.0

	for i := 0; i < len(x) && i < len(y); i++ {
		normXSq += x[i] * x[i]
		normYSq += y[i] * y[i]
		diff := x[i] - y[i]
		normDiffSq += diff * diff
	}

	// Compute denominators
	denomX := 1.0 - c*normXSq
	denomY := 1.0 - c*normYSq

	// Avoid division by zero
	if denomX <= 0 {
		denomX = 1e-10
	}
	if denomY <= 0 {
		denomY = 1e-10
	}

	// Compute argument of acosh
	arg := 1.0 + 2.0*c*normDiffSq/(denomX*denomY)

	// Clamp to valid range for acosh (>= 1)
	if arg < 1.0 {
		arg = 1.0
	}

	return Acosh(arg)
}

// computeHyperbolicAttention computes attention using hyperbolic distances.
func (ha *HyperbolicAttention) computeHyperbolicAttention(embeddings [][]float64, outputs []shared.AttentionAgentOutput) map[string]float64 {
	n := len(embeddings)
	scores := make([]float64, n)

	for i := 0; i < n; i++ {
		totalScore := 0.0
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}

			// Compute hyperbolic distance
			distance := ha.poincareDistance(embeddings[i], embeddings[j])

			// Convert distance to similarity (closer = higher similarity)
			similarity := math.Exp(-distance)

			// Scale by confidence
			confidence := (outputs[i].Confidence + outputs[j].Confidence) / 2
			if confidence == 0 {
				confidence = 0.5
			}

			score := similarity * confidence

			// Apply hierarchical weighting
			score *= ha.getHierarchicalWeight(outputs[i].AgentID, outputs[j].AgentID)

			totalScore += score
		}

		if n > 1 {
			scores[i] = totalScore / float64(n-1)
		}
	}

	// Apply softmax
	normalizedScores := Softmax(scores)

	// Map to agent IDs
	weights := make(map[string]float64)
	for i, output := range outputs {
		weights[output.AgentID] = normalizedScores[i]
	}

	return weights
}

// getHierarchicalWeight returns the weight multiplier based on agent roles.
func (ha *HyperbolicAttention) getHierarchicalWeight(agentI, agentJ string) float64 {
	roleI := ha.roles[agentI]
	roleJ := ha.roles[agentJ]

	// Queen agents get higher weight
	isQueenI := roleI == "queen" || roleI == "coordinator"
	isQueenJ := roleJ == "queen" || roleJ == "coordinator"

	if isQueenI && isQueenJ {
		// Both are queens - highest weight
		return ha.config.HierarchicalWeight * ha.config.HierarchicalWeight
	}
	if isQueenI || isQueenJ {
		// One is queen - boosted weight
		return ha.config.HierarchicalWeight
	}

	// Both are workers - standard weight
	return 1.0
}

// buildConsensus builds consensus output from weighted agent outputs.
func (ha *HyperbolicAttention) buildConsensus(outputs []shared.AttentionAgentOutput, weights map[string]float64) *shared.AttentionCoordinationResult {
	if len(outputs) == 0 {
		return &shared.AttentionCoordinationResult{}
	}

	// Find the output with highest weight
	var bestOutput *shared.AttentionAgentOutput
	bestWeight := -1.0

	participants := make([]string, 0, len(outputs))
	totalConfidence := 0.0

	for i := range outputs {
		output := &outputs[i]
		participants = append(participants, output.AgentID)

		weight := weights[output.AgentID]
		if weight > bestWeight {
			bestWeight = weight
			bestOutput = output
		}

		totalConfidence += output.Confidence * weight
	}

	var consensusOutput interface{}
	if bestOutput != nil {
		consensusOutput = bestOutput.Content
	}

	return &shared.AttentionCoordinationResult{
		ConsensusOutput:  consensusOutput,
		AttentionWeights: weights,
		Confidence:       totalConfidence,
		Participants:     participants,
	}
}

// GetConfig returns the Hyperbolic Attention configuration.
func (ha *HyperbolicAttention) GetConfig() shared.HyperbolicAttentionConfig {
	return ha.config
}

// ComputeHierarchyDepth estimates the hierarchy depth based on hyperbolic distances.
func (ha *HyperbolicAttention) ComputeHierarchyDepth(embeddings [][]float64) float64 {
	if len(embeddings) < 2 {
		return 0
	}

	// Compute average distance from center (origin in Poincaré ball)
	origin := make([]float64, len(embeddings[0]))
	totalDepth := 0.0

	for _, emb := range embeddings {
		dist := ha.poincareDistance(origin, emb)
		totalDepth += dist
	}

	return totalDepth / float64(len(embeddings))
}
