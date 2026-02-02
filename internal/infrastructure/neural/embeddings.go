// Package neural provides neural infrastructure components.
package neural

import (
	"hash/fnv"
	"math"
)

// EmbeddingGenerator generates embeddings using hash-based approach.
// This is a fallback when ONNX models are not available.
type EmbeddingGenerator struct {
	dimension int
}

// NewEmbeddingGenerator creates a new embedding generator.
func NewEmbeddingGenerator(dimension int) *EmbeddingGenerator {
	if dimension <= 0 {
		dimension = 256
	}
	return &EmbeddingGenerator{
		dimension: dimension,
	}
}

// Generate generates a hash-based embedding for the given text.
// Uses FNV-1a hashing with multiple passes for pseudo-random distribution.
func (e *EmbeddingGenerator) Generate(text string) []float32 {
	embedding := make([]float32, e.dimension)
	
	if text == "" {
		return embedding
	}
	
	// Generate multiple hashes for different regions of the embedding
	for i := 0; i < e.dimension; i++ {
		h := fnv.New64a()
		// Mix the index into the hash for different values per dimension
		h.Write([]byte{byte(i), byte(i >> 8)})
		h.Write([]byte(text))
		hashVal := h.Sum64()
		
		// Convert to float32 in range [-1, 1]
		embedding[i] = float32((float64(hashVal)/float64(math.MaxUint64))*2.0 - 1.0)
	}
	
	// Normalize the embedding
	return normalize(embedding)
}

// normalize normalizes a vector to unit length.
func normalize(v []float32) []float32 {
	var sum float64
	for _, val := range v {
		sum += float64(val * val)
	}
	
	if sum == 0 {
		return v
	}
	
	norm := float32(math.Sqrt(sum))
	result := make([]float32, len(v))
	for i, val := range v {
		result[i] = val / norm
	}
	
	return result
}

// CosineSimilarity calculates cosine similarity between two embeddings.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	
	if normA == 0 || normB == 0 {
		return 0.0
	}
	
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// EuclideanDistance calculates Euclidean distance between two embeddings.
func EuclideanDistance(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}
	
	var sum float64
	for i := range a {
		diff := float64(a[i]) - float64(b[i])
		sum += diff * diff
	}
	
	return math.Sqrt(sum)
}

// QuantizeInt8 quantizes float32 embeddings to int8 for 4x memory reduction.
func QuantizeInt8(embedding []float32) []int8 {
	result := make([]int8, len(embedding))
	
	// Find min and max for scaling
	var minVal, maxVal float32 = embedding[0], embedding[0]
	for _, v := range embedding {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	
	// Scale to [-127, 127]
	scale := maxVal - minVal
	if scale == 0 {
		return result
	}
	
	for i, v := range embedding {
		normalized := (v - minVal) / scale // [0, 1]
		result[i] = int8((normalized * 254) - 127)
	}
	
	return result
}

// DequantizeInt8 dequantizes int8 back to float32.
func DequantizeInt8(quantized []int8, minVal, maxVal float32) []float32 {
	result := make([]float32, len(quantized))
	scale := maxVal - minVal
	
	for i, v := range quantized {
		normalized := (float32(v) + 127) / 254 // [0, 1]
		result[i] = normalized*scale + minVal
	}
	
	return result
}
