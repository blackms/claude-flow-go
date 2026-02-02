// Package embeddings provides infrastructure for the embeddings service.
package embeddings

import (
	"math"

	domainEmbeddings "github.com/anthropics/claude-flow-go/internal/domain/embeddings"
)

// CosineSimilarity calculates the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical direction.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (normA * normB)
}

// EuclideanDistance calculates the Euclidean distance between two vectors.
// Returns a non-negative value where 0 means identical vectors.
func EuclideanDistance(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var sum float64
	for i := 0; i < len(a); i++ {
		diff := float64(a[i]) - float64(b[i])
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

// DotProduct calculates the dot product of two vectors.
func DotProduct(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var sum float64
	for i := 0; i < len(a); i++ {
		sum += float64(a[i]) * float64(b[i])
	}

	return sum
}

// ManhattanDistance calculates the Manhattan (L1) distance between two vectors.
func ManhattanDistance(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var sum float64
	for i := 0; i < len(a); i++ {
		sum += math.Abs(float64(a[i]) - float64(b[i]))
	}

	return sum
}

// ComputeSimilarity computes similarity using the specified metric.
func ComputeSimilarity(a, b []float32, metric domainEmbeddings.SimilarityMetric) *domainEmbeddings.SimilarityResult {
	var score float64

	switch metric {
	case domainEmbeddings.SimilarityCosine:
		score = CosineSimilarity(a, b)
	case domainEmbeddings.SimilarityEuclidean:
		// Convert distance to similarity (inverse)
		dist := EuclideanDistance(a, b)
		score = 1.0 / (1.0 + dist)
	case domainEmbeddings.SimilarityDot:
		score = DotProduct(a, b)
	default:
		score = CosineSimilarity(a, b)
		metric = domainEmbeddings.SimilarityCosine
	}

	return &domainEmbeddings.SimilarityResult{
		Score:  score,
		Metric: metric,
	}
}

// FindMostSimilar finds the most similar embedding from a list.
// Returns the index and similarity score.
func FindMostSimilar(query []float32, candidates [][]float32, metric domainEmbeddings.SimilarityMetric) (int, float64) {
	if len(candidates) == 0 {
		return -1, 0
	}

	bestIndex := 0
	bestScore := math.Inf(-1)

	for i, candidate := range candidates {
		result := ComputeSimilarity(query, candidate, metric)
		if result.Score > bestScore {
			bestScore = result.Score
			bestIndex = i
		}
	}

	return bestIndex, bestScore
}

// FindTopK finds the top K most similar embeddings.
// Returns indices and scores in descending order of similarity.
func FindTopK(query []float32, candidates [][]float32, k int, metric domainEmbeddings.SimilarityMetric) ([]int, []float64) {
	if len(candidates) == 0 || k <= 0 {
		return nil, nil
	}

	if k > len(candidates) {
		k = len(candidates)
	}

	// Calculate all similarities
	type scored struct {
		index int
		score float64
	}
	scores := make([]scored, len(candidates))
	for i, candidate := range candidates {
		result := ComputeSimilarity(query, candidate, metric)
		scores[i] = scored{i, result.Score}
	}

	// Simple selection sort for top K (efficient for small K)
	for i := 0; i < k; i++ {
		maxIdx := i
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[maxIdx].score {
				maxIdx = j
			}
		}
		scores[i], scores[maxIdx] = scores[maxIdx], scores[i]
	}

	indices := make([]int, k)
	similarities := make([]float64, k)
	for i := 0; i < k; i++ {
		indices[i] = scores[i].index
		similarities[i] = scores[i].score
	}

	return indices, similarities
}

// AverageEmbedding calculates the average of multiple embeddings.
func AverageEmbedding(embeddings [][]float32) []float32 {
	if len(embeddings) == 0 {
		return nil
	}

	dims := len(embeddings[0])
	if dims == 0 {
		return nil
	}

	result := make([]float32, dims)
	for _, emb := range embeddings {
		for i, val := range emb {
			result[i] += val
		}
	}

	n := float32(len(embeddings))
	for i := range result {
		result[i] /= n
	}

	return result
}

// WeightedAverageEmbedding calculates the weighted average of embeddings.
func WeightedAverageEmbedding(embeddings [][]float32, weights []float32) []float32 {
	if len(embeddings) == 0 || len(embeddings) != len(weights) {
		return nil
	}

	dims := len(embeddings[0])
	if dims == 0 {
		return nil
	}

	result := make([]float32, dims)
	var totalWeight float32

	for i, emb := range embeddings {
		weight := weights[i]
		totalWeight += weight
		for j, val := range emb {
			result[j] += val * weight
		}
	}

	if totalWeight == 0 {
		return result
	}

	for i := range result {
		result[i] /= totalWeight
	}

	return result
}
