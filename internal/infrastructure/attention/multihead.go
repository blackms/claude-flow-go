// Package attention provides attention mechanisms for agent coordination.
package attention

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// MultiHeadAttention implements multi-head attention mechanism.
// Default: 8 parallel attention heads, 64-dim head dimension.
type MultiHeadAttention struct {
	config shared.MultiHeadAttentionConfig
}

// NewMultiHeadAttention creates a new MultiHeadAttention instance.
func NewMultiHeadAttention(config shared.MultiHeadAttentionConfig) *MultiHeadAttention {
	return &MultiHeadAttention{config: config}
}

// NewMultiHeadAttentionWithDefaults creates a MultiHeadAttention with default configuration.
func NewMultiHeadAttentionWithDefaults() *MultiHeadAttention {
	return NewMultiHeadAttention(shared.DefaultMultiHeadAttentionConfig())
}

// Coordinate performs Multi-Head Attention coordination on agent outputs.
func (mha *MultiHeadAttention) Coordinate(outputs []shared.AttentionAgentOutput) *shared.AttentionCoordinationResult {
	startTime := shared.Now()

	n := len(outputs)
	if n == 0 {
		return &shared.AttentionCoordinationResult{
			Mechanism: shared.AttentionMultiHead,
		}
	}

	// Ensure all outputs have embeddings
	embeddingDim := mha.config.NumHeads * mha.config.HeadDimension
	embeddings := make([][]float64, n)
	for i, output := range outputs {
		if len(output.Embedding) > 0 {
			embeddings[i] = output.Embedding
		} else {
			embeddings[i] = GenerateEmbedding(output.Content, embeddingDim)
		}
		// Pad or truncate to expected dimension
		embeddings[i] = mha.padOrTruncate(embeddings[i], embeddingDim)
	}

	// Compute attention for each head
	numHeads := mha.config.NumHeads
	headDim := mha.config.HeadDimension

	// Store per-head weights
	headWeights := make([]map[string]float64, numHeads)

	for h := 0; h < numHeads; h++ {
		// Extract head-specific embeddings
		headEmbeddings := make([][]float64, n)
		for i := range embeddings {
			start := h * headDim
			end := start + headDim
			if end > len(embeddings[i]) {
				end = len(embeddings[i])
			}
			if start < len(embeddings[i]) {
				headEmbeddings[i] = embeddings[i][start:end]
			} else {
				headEmbeddings[i] = make([]float64, headDim)
			}
		}

		// Compute attention for this head
		headWeights[h] = mha.computeHeadAttention(headEmbeddings, outputs)
	}

	// Combine heads by averaging weights
	combinedWeights := mha.combineHeads(headWeights, outputs)

	// Build consensus
	result := mha.buildConsensus(outputs, combinedWeights)

	// Calculate memory usage (N² × heads × 8 bytes)
	memoryBytes := int64(n * n * numHeads * 8)

	latency := float64(shared.Now() - startTime)

	result.LatencyMs = latency
	result.MemoryBytes = memoryBytes
	result.Mechanism = shared.AttentionMultiHead

	return result
}

// padOrTruncate pads or truncates an embedding to the target dimension.
func (mha *MultiHeadAttention) padOrTruncate(embedding []float64, targetDim int) []float64 {
	if len(embedding) == targetDim {
		return embedding
	}

	result := make([]float64, targetDim)
	copy(result, embedding)
	return result
}

// computeHeadAttention computes attention weights for a single head.
func (mha *MultiHeadAttention) computeHeadAttention(embeddings [][]float64, outputs []shared.AttentionAgentOutput) map[string]float64 {
	n := len(embeddings)
	scores := make([]float64, n)

	// Compute attention scores using scaled dot-product
	scale := 1.0 / float64(mha.config.HeadDimension)

	for i := 0; i < n; i++ {
		totalScore := 0.0
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}

			// Scaled dot product
			dotProd := DotProduct(embeddings[i], embeddings[j])
			score := dotProd * scale

			// Scale by confidence
			confidence := (outputs[i].Confidence + outputs[j].Confidence) / 2
			if confidence == 0 {
				confidence = 0.5
			}

			totalScore += score * confidence
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

// combineHeads combines attention weights from all heads by averaging.
func (mha *MultiHeadAttention) combineHeads(headWeights []map[string]float64, outputs []shared.AttentionAgentOutput) map[string]float64 {
	numHeads := len(headWeights)
	if numHeads == 0 {
		return make(map[string]float64)
	}

	combined := make(map[string]float64)

	for _, output := range outputs {
		agentID := output.AgentID
		totalWeight := 0.0

		for h := 0; h < numHeads; h++ {
			if weight, exists := headWeights[h][agentID]; exists {
				totalWeight += weight
			}
		}

		combined[agentID] = totalWeight / float64(numHeads)
	}

	return combined
}

// buildConsensus builds consensus output from weighted agent outputs.
func (mha *MultiHeadAttention) buildConsensus(outputs []shared.AttentionAgentOutput, weights map[string]float64) *shared.AttentionCoordinationResult {
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

// GetConfig returns the Multi-Head Attention configuration.
func (mha *MultiHeadAttention) GetConfig() shared.MultiHeadAttentionConfig {
	return mha.config
}

// GetHeadWeights returns the per-head attention weights for debugging.
func (mha *MultiHeadAttention) GetHeadWeights(outputs []shared.AttentionAgentOutput) []map[string]float64 {
	n := len(outputs)
	if n == 0 {
		return nil
	}

	embeddingDim := mha.config.NumHeads * mha.config.HeadDimension
	embeddings := make([][]float64, n)
	for i, output := range outputs {
		if len(output.Embedding) > 0 {
			embeddings[i] = output.Embedding
		} else {
			embeddings[i] = GenerateEmbedding(output.Content, embeddingDim)
		}
		embeddings[i] = mha.padOrTruncate(embeddings[i], embeddingDim)
	}

	numHeads := mha.config.NumHeads
	headDim := mha.config.HeadDimension
	headWeights := make([]map[string]float64, numHeads)

	for h := 0; h < numHeads; h++ {
		headEmbeddings := make([][]float64, n)
		for i := range embeddings {
			start := h * headDim
			end := start + headDim
			if end > len(embeddings[i]) {
				end = len(embeddings[i])
			}
			if start < len(embeddings[i]) {
				headEmbeddings[i] = embeddings[i][start:end]
			} else {
				headEmbeddings[i] = make([]float64, headDim)
			}
		}
		headWeights[h] = mha.computeHeadAttention(headEmbeddings, outputs)
	}

	return headWeights
}
