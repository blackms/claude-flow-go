// Package attention provides attention mechanisms for agent coordination.
package attention

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// LinearAttention implements linear attention with O(n) complexity.
// Uses feature maps (ReLU/ELU) instead of softmax for efficiency.
// Best for: Very long sequences where O(n²) is prohibitive.
type LinearAttention struct {
	config shared.LinearAttentionConfig
}

// NewLinearAttention creates a new LinearAttention instance.
func NewLinearAttention(config shared.LinearAttentionConfig) *LinearAttention {
	return &LinearAttention{config: config}
}

// NewLinearAttentionWithDefaults creates a LinearAttention with default configuration.
func NewLinearAttentionWithDefaults() *LinearAttention {
	return NewLinearAttention(shared.DefaultLinearAttentionConfig())
}

// Coordinate performs Linear Attention coordination on agent outputs.
func (la *LinearAttention) Coordinate(outputs []shared.AttentionAgentOutput) *shared.AttentionCoordinationResult {
	startTime := shared.Now()

	n := len(outputs)
	if n == 0 {
		return &shared.AttentionCoordinationResult{
			Mechanism: shared.AttentionLinear,
		}
	}

	// Ensure all outputs have embeddings
	embeddingDim := 64
	embeddings := make([][]float64, n)
	for i, output := range outputs {
		if len(output.Embedding) > 0 {
			embeddings[i] = output.Embedding
			embeddingDim = len(output.Embedding)
		} else {
			embeddings[i] = GenerateEmbedding(output.Content, embeddingDim)
		}
	}

	// Apply feature map to embeddings
	featureMaps := make([][]float64, n)
	for i, emb := range embeddings {
		featureMaps[i] = la.applyFeatureMap(emb)
	}

	// Linear attention: O(n) complexity
	// Instead of computing N×N attention matrix, we use:
	// Attention(Q, K, V) ≈ φ(Q) × (φ(K)ᵀ × V) / (φ(Q) × Σφ(K))
	weights := la.computeLinearAttention(featureMaps, outputs)

	// Build consensus
	result := la.buildConsensus(outputs, weights)

	// Calculate memory usage (O(n) instead of O(n²))
	memoryBytes := int64(n * embeddingDim * 8 * 2) // embeddings + feature maps

	latency := float64(shared.Now() - startTime)

	result.LatencyMs = latency
	result.MemoryBytes = memoryBytes
	result.Mechanism = shared.AttentionLinear

	return result
}

// applyFeatureMap applies the feature map transformation.
func (la *LinearAttention) applyFeatureMap(embedding []float64) []float64 {
	switch la.config.FeatureMapType {
	case "elu":
		return la.eluFeatureMap(embedding)
	case "softmax":
		return la.softmaxFeatureMap(embedding)
	default: // "relu"
		return la.reluFeatureMap(embedding)
	}
}

// reluFeatureMap applies ReLU feature map: φ(x) = ReLU(x)
func (la *LinearAttention) reluFeatureMap(embedding []float64) []float64 {
	return ReLUVector(embedding)
}

// eluFeatureMap applies ELU feature map: φ(x) = ELU(x) + 1
func (la *LinearAttention) eluFeatureMap(embedding []float64) []float64 {
	result := make([]float64, len(embedding))
	for i, x := range embedding {
		result[i] = ELU(x) + 1 // Ensure positive values
	}
	return result
}

// softmaxFeatureMap applies element-wise exponential as feature map.
func (la *LinearAttention) softmaxFeatureMap(embedding []float64) []float64 {
	// Use exp for positive kernel
	result := make([]float64, len(embedding))

	// Find max for stability
	maxVal := embedding[0]
	for _, x := range embedding[1:] {
		if x > maxVal {
			maxVal = x
		}
	}

	for i, x := range embedding {
		result[i] = expSafe(x - maxVal)
	}
	return result
}

// expSafe computes exp with overflow protection.
func expSafe(x float64) float64 {
	if x > 700 {
		return 1e300
	}
	if x < -700 {
		return 0
	}
	return exp(x)
}

// exp is a simple exponential function.
func exp(x float64) float64 {
	// Use Taylor series for small x, lookup for larger
	if x < 0 {
		return 1.0 / exp(-x)
	}
	if x < 1 {
		// Taylor series: e^x = 1 + x + x²/2! + x³/3! + ...
		result := 1.0
		term := 1.0
		for i := 1; i < 20; i++ {
			term *= x / float64(i)
			result += term
		}
		return result
	}
	// For larger x, use repeated squaring
	if x < 2 {
		return exp(x/2) * exp(x/2)
	}
	half := exp(x / 2)
	return half * half
}

// computeLinearAttention computes attention with O(n) complexity.
func (la *LinearAttention) computeLinearAttention(featureMaps [][]float64, outputs []shared.AttentionAgentOutput) map[string]float64 {
	n := len(featureMaps)
	if n == 0 {
		return make(map[string]float64)
	}

	dim := len(featureMaps[0])
	scores := make([]float64, n)

	// First pass: compute Σφ(K) (sum of all feature maps)
	sumFeatures := make([]float64, dim)
	for _, fm := range featureMaps {
		for j := 0; j < dim && j < len(fm); j++ {
			sumFeatures[j] += fm[j]
		}
	}

	// Second pass: compute attention for each query
	for i := 0; i < n; i++ {
		query := featureMaps[i]

		// Numerator: φ(Q) · Σ(φ(K) × confidence)
		numerator := 0.0
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			dotProd := DotProduct(query, featureMaps[j])
			confidence := outputs[j].Confidence
			if confidence == 0 {
				confidence = 0.5
			}
			numerator += dotProd * confidence
		}

		// Denominator: φ(Q) · Σφ(K)
		denominator := DotProduct(query, sumFeatures)
		if denominator == 0 {
			denominator = 1.0
		}

		scores[i] = numerator / denominator
	}

	// Normalize scores
	normalizedScores := Softmax(scores)

	// Map to agent IDs
	weights := make(map[string]float64)
	for i, output := range outputs {
		weights[output.AgentID] = normalizedScores[i]
	}

	return weights
}

// buildConsensus builds consensus output from weighted agent outputs.
func (la *LinearAttention) buildConsensus(outputs []shared.AttentionAgentOutput, weights map[string]float64) *shared.AttentionCoordinationResult {
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

// GetConfig returns the Linear Attention configuration.
func (la *LinearAttention) GetConfig() shared.LinearAttentionConfig {
	return la.config
}

// EstimateComplexity returns the complexity comparison.
func (la *LinearAttention) EstimateComplexity(n int, d int) (linear, quadratic int64) {
	// Linear attention: O(n × d)
	linear = int64(n * d)

	// Standard attention: O(n² × d)
	quadratic = int64(n * n * d)

	return linear, quadratic
}
