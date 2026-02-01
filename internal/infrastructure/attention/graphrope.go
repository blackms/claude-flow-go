// Package attention provides attention mechanisms for agent coordination.
package attention

import (
	"math"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// GraphRoPE implements Graph-aware Rotary Position Encoding.
// Uses BFS-based distance computation for topology-aware coordination.
// Best for: Mesh and hybrid topologies.
type GraphRoPE struct {
	config    shared.GraphRoPEConfig
	adjacency map[string][]string // agentID -> connected agents
	distances map[string]map[string]int // cached BFS distances
}

// NewGraphRoPE creates a new GraphRoPE instance.
func NewGraphRoPE(config shared.GraphRoPEConfig) *GraphRoPE {
	return &GraphRoPE{
		config:    config,
		adjacency: make(map[string][]string),
		distances: make(map[string]map[string]int),
	}
}

// NewGraphRoPEWithDefaults creates a GraphRoPE with default configuration.
func NewGraphRoPEWithDefaults() *GraphRoPE {
	return NewGraphRoPE(shared.DefaultGraphRoPEConfig())
}

// SetTopology sets the graph topology from adjacency information.
func (gr *GraphRoPE) SetTopology(adjacency map[string][]string) {
	gr.adjacency = adjacency
	gr.distances = make(map[string]map[string]int) // Clear cache
}

// SetTopologyFromEdges sets the topology from a list of edges.
func (gr *GraphRoPE) SetTopologyFromEdges(edges []shared.TopologyEdge) {
	gr.adjacency = make(map[string][]string)

	for _, edge := range edges {
		gr.adjacency[edge.From] = append(gr.adjacency[edge.From], edge.To)
		if edge.Bidirectional {
			gr.adjacency[edge.To] = append(gr.adjacency[edge.To], edge.From)
		}
	}

	gr.distances = make(map[string]map[string]int) // Clear cache
}

// Coordinate performs GraphRoPE-based coordination on agent outputs.
func (gr *GraphRoPE) Coordinate(outputs []shared.AttentionAgentOutput) *shared.AttentionCoordinationResult {
	startTime := shared.Now()

	n := len(outputs)
	if n == 0 {
		return &shared.AttentionCoordinationResult{
			Mechanism: shared.AttentionGraphRoPE,
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

	// Compute position encodings based on graph distances
	positionEncodings := gr.computePositionEncodings(outputs)

	// Compute attention with rotary encoding
	weights := gr.computeGraphAttention(embeddings, positionEncodings, outputs)

	// Build consensus
	result := gr.buildConsensus(outputs, weights)

	// Calculate memory usage
	memoryBytes := int64(n * embeddingDim * 8)
	memoryBytes += int64(n * gr.config.EncodingDim * 8) // Position encodings

	latency := float64(shared.Now() - startTime)

	result.LatencyMs = latency
	result.MemoryBytes = memoryBytes
	result.Mechanism = shared.AttentionGraphRoPE

	return result
}

// computePositionEncodings computes position encodings based on graph distances.
func (gr *GraphRoPE) computePositionEncodings(outputs []shared.AttentionAgentOutput) map[string][]float64 {
	encodings := make(map[string][]float64)

	// For each agent, compute its position encoding
	for _, output := range outputs {
		encoding := gr.computeAgentPositionEncoding(output.AgentID, outputs)
		encodings[output.AgentID] = encoding
	}

	return encodings
}

// computeAgentPositionEncoding computes the position encoding for a single agent.
func (gr *GraphRoPE) computeAgentPositionEncoding(agentID string, outputs []shared.AttentionAgentOutput) []float64 {
	dim := gr.config.EncodingDim

	// Compute distances to all other agents
	distances := make([]int, len(outputs))
	for i, output := range outputs {
		if output.AgentID == agentID {
			distances[i] = 0
		} else {
			distances[i] = gr.bfsDistance(agentID, output.AgentID)
		}
	}

	// Create sinusoidal encoding based on average distance
	avgDistance := 0.0
	for _, d := range distances {
		avgDistance += float64(d)
	}
	if len(distances) > 0 {
		avgDistance /= float64(len(distances))
	}

	// Scale distance
	scaledPos := avgDistance * gr.config.DistanceScale

	// Generate sinusoidal encoding
	return SinusoidalEncoding(int(scaledPos), dim)
}

// bfsDistance computes the BFS distance between two agents.
func (gr *GraphRoPE) bfsDistance(from, to string) int {
	if from == to {
		return 0
	}

	// Check cache
	if gr.distances[from] != nil {
		if d, exists := gr.distances[from][to]; exists {
			return d
		}
	}

	// BFS
	visited := make(map[string]bool)
	queue := []struct {
		node     string
		distance int
	}{{from, 0}}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.node == to {
			// Cache and return
			gr.cacheDistance(from, to, current.distance)
			return current.distance
		}

		// Don't go beyond max distance
		if current.distance >= gr.config.MaxDistance {
			continue
		}

		// Explore neighbors
		for _, neighbor := range gr.adjacency[current.node] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, struct {
					node     string
					distance int
				}{neighbor, current.distance + 1})
			}
		}
	}

	// No path found - return max distance
	return gr.config.MaxDistance
}

// cacheDistance caches BFS distance (bidirectional).
func (gr *GraphRoPE) cacheDistance(from, to string, distance int) {
	if gr.distances[from] == nil {
		gr.distances[from] = make(map[string]int)
	}
	gr.distances[from][to] = distance

	if gr.distances[to] == nil {
		gr.distances[to] = make(map[string]int)
	}
	gr.distances[to][from] = distance
}

// computeGraphAttention computes attention with rotary position encoding.
func (gr *GraphRoPE) computeGraphAttention(embeddings [][]float64, posEncodings map[string][]float64, outputs []shared.AttentionAgentOutput) map[string]float64 {
	n := len(embeddings)
	scores := make([]float64, n)

	for i := 0; i < n; i++ {
		totalScore := 0.0
		agentI := outputs[i].AgentID
		posI := posEncodings[agentI]

		for j := 0; j < n; j++ {
			if i == j {
				continue
			}

			agentJ := outputs[j].AgentID
			posJ := posEncodings[agentJ]

			// Base similarity
			similarity := CosineSimilarity(embeddings[i], embeddings[j])

			// Apply rotary encoding factor
			// Closer nodes (similar position encodings) get higher weight
			positionSimilarity := CosineSimilarity(posI, posJ)

			// Combine base similarity with position-aware factor
			rotaryFactor := 0.5 + 0.5*positionSimilarity // Range [0, 1]
			score := similarity * rotaryFactor

			// Scale by confidence
			confidence := (outputs[i].Confidence + outputs[j].Confidence) / 2
			if confidence == 0 {
				confidence = 0.5
			}

			score *= confidence

			// Apply distance-based decay
			distance := gr.bfsDistance(agentI, agentJ)
			distanceDecay := math.Exp(-float64(distance) * 0.1)
			score *= distanceDecay

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

// buildConsensus builds consensus output from weighted agent outputs.
func (gr *GraphRoPE) buildConsensus(outputs []shared.AttentionAgentOutput, weights map[string]float64) *shared.AttentionCoordinationResult {
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

// GetConfig returns the GraphRoPE configuration.
func (gr *GraphRoPE) GetConfig() shared.GraphRoPEConfig {
	return gr.config
}

// GetDistanceMatrix returns the computed distance matrix.
func (gr *GraphRoPE) GetDistanceMatrix(agentIDs []string) [][]int {
	n := len(agentIDs)
	matrix := make([][]int, n)

	for i := 0; i < n; i++ {
		matrix[i] = make([]int, n)
		for j := 0; j < n; j++ {
			if i == j {
				matrix[i][j] = 0
			} else {
				matrix[i][j] = gr.bfsDistance(agentIDs[i], agentIDs[j])
			}
		}
	}

	return matrix
}

// ClearDistanceCache clears the cached distances.
func (gr *GraphRoPE) ClearDistanceCache() {
	gr.distances = make(map[string]map[string]int)
}
