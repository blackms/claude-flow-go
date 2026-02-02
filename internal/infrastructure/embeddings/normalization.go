// Package embeddings provides infrastructure for the embeddings service.
package embeddings

import (
	"math"

	domainEmbeddings "github.com/anthropics/claude-flow-go/internal/domain/embeddings"
)

// getNormalizer returns the appropriate normalizer function for the given type.
func getNormalizer(normType domainEmbeddings.NormalizationType) func([]float32) []float32 {
	switch normType {
	case domainEmbeddings.NormalizationL2:
		return L2Normalize
	case domainEmbeddings.NormalizationL1:
		return L1Normalize
	case domainEmbeddings.NormalizationMinMax:
		return MinMaxNormalize
	case domainEmbeddings.NormalizationZScore:
		return ZScoreNormalize
	case domainEmbeddings.NormalizationNone:
		return nil
	default:
		return nil
	}
}

// L2Normalize normalizes a vector to unit length (L2 norm = 1).
// This is the most common normalization for cosine similarity.
func L2Normalize(v []float32) []float32 {
	if len(v) == 0 {
		return v
	}

	norm := L2Norm(v)
	if norm == 0 {
		return v
	}

	result := make([]float32, len(v))
	for i, val := range v {
		result[i] = val / float32(norm)
	}
	return result
}

// L1Normalize normalizes a vector using L1 norm (Manhattan normalization).
func L1Normalize(v []float32) []float32 {
	if len(v) == 0 {
		return v
	}

	norm := L1Norm(v)
	if norm == 0 {
		return v
	}

	result := make([]float32, len(v))
	for i, val := range v {
		result[i] = val / float32(norm)
	}
	return result
}

// MinMaxNormalize scales vector values to [0, 1] range.
func MinMaxNormalize(v []float32) []float32 {
	if len(v) == 0 {
		return v
	}

	minVal := v[0]
	maxVal := v[0]
	for _, val := range v {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	rangeVal := maxVal - minVal
	if rangeVal == 0 {
		result := make([]float32, len(v))
		for i := range result {
			result[i] = 0.5 // All values are the same
		}
		return result
	}

	result := make([]float32, len(v))
	for i, val := range v {
		result[i] = (val - minVal) / rangeVal
	}
	return result
}

// ZScoreNormalize normalizes to mean=0, std=1 (standard score).
func ZScoreNormalize(v []float32) []float32 {
	if len(v) == 0 {
		return v
	}

	// Calculate mean
	var sum float64
	for _, val := range v {
		sum += float64(val)
	}
	mean := sum / float64(len(v))

	// Calculate standard deviation
	var variance float64
	for _, val := range v {
		diff := float64(val) - mean
		variance += diff * diff
	}
	std := math.Sqrt(variance / float64(len(v)))

	if std == 0 {
		result := make([]float32, len(v))
		return result
	}

	result := make([]float32, len(v))
	for i, val := range v {
		result[i] = float32((float64(val) - mean) / std)
	}
	return result
}

// L2Norm calculates the L2 norm (Euclidean length) of a vector.
func L2Norm(v []float32) float64 {
	var sum float64
	for _, val := range v {
		sum += float64(val) * float64(val)
	}
	return math.Sqrt(sum)
}

// L1Norm calculates the L1 norm (Manhattan distance from origin) of a vector.
func L1Norm(v []float32) float64 {
	var sum float64
	for _, val := range v {
		sum += math.Abs(float64(val))
	}
	return sum
}

// IsNormalized checks if a vector is approximately L2-normalized.
func IsNormalized(v []float32, tolerance float64) bool {
	if tolerance <= 0 {
		tolerance = 1e-6
	}
	norm := L2Norm(v)
	return math.Abs(norm-1.0) < tolerance
}

// CenterEmbeddings subtracts the batch mean from each embedding.
func CenterEmbeddings(embeddings [][]float32) [][]float32 {
	if len(embeddings) == 0 {
		return embeddings
	}

	dims := len(embeddings[0])
	if dims == 0 {
		return embeddings
	}

	// Calculate mean per dimension
	mean := make([]float64, dims)
	for _, emb := range embeddings {
		for i, val := range emb {
			mean[i] += float64(val)
		}
	}
	for i := range mean {
		mean[i] /= float64(len(embeddings))
	}

	// Subtract mean
	result := make([][]float32, len(embeddings))
	for i, emb := range embeddings {
		centered := make([]float32, dims)
		for j, val := range emb {
			centered[j] = val - float32(mean[j])
		}
		result[i] = centered
	}

	return result
}

// NormalizeBatch normalizes a batch of embeddings.
func NormalizeBatch(embeddings [][]float32, normType domainEmbeddings.NormalizationType) [][]float32 {
	normalizer := getNormalizer(normType)
	if normalizer == nil {
		return embeddings
	}

	result := make([][]float32, len(embeddings))
	for i, emb := range embeddings {
		result[i] = normalizer(emb)
	}
	return result
}

// Normalize applies the specified normalization to a vector.
func Normalize(v []float32, normType domainEmbeddings.NormalizationType) []float32 {
	normalizer := getNormalizer(normType)
	if normalizer == nil {
		return v
	}
	return normalizer(v)
}
