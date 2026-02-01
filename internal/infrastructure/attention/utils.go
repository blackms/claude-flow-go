// Package attention provides attention mechanisms for agent coordination.
package attention

import (
	"hash/fnv"
	"math"
)

// Softmax applies softmax normalization to a slice of scores.
// Returns a probability distribution that sums to 1.
func Softmax(scores []float64) []float64 {
	if len(scores) == 0 {
		return []float64{}
	}

	// Find max for numerical stability
	maxScore := scores[0]
	for _, s := range scores[1:] {
		if s > maxScore {
			maxScore = s
		}
	}

	// Compute exp(x - max) and sum
	result := make([]float64, len(scores))
	sum := 0.0
	for i, s := range scores {
		result[i] = math.Exp(s - maxScore)
		sum += result[i]
	}

	// Normalize
	if sum > 0 {
		for i := range result {
			result[i] /= sum
		}
	}

	return result
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 and 1.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (normA * normB)
}

// DotProduct computes the dot product of two vectors.
func DotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// L2Norm computes the L2 norm (Euclidean norm) of a vector.
func L2Norm(v []float64) float64 {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	return math.Sqrt(sum)
}

// NormalizeEmbedding normalizes an embedding to unit length (L2 normalization).
func NormalizeEmbedding(embedding []float64) []float64 {
	norm := L2Norm(embedding)
	if norm == 0 {
		return embedding
	}

	result := make([]float64, len(embedding))
	for i, x := range embedding {
		result[i] = x / norm
	}
	return result
}

// GenerateEmbedding generates a hash-based embedding from content.
// This is a fallback when no embedding is provided.
func GenerateEmbedding(content string, dim int) []float64 {
	if dim <= 0 {
		dim = 64
	}

	embedding := make([]float64, dim)

	// Use FNV hash to generate pseudo-random but deterministic values
	h := fnv.New64a()

	for i := 0; i < dim; i++ {
		h.Reset()
		h.Write([]byte(content))
		h.Write([]byte{byte(i), byte(i >> 8)})
		hashVal := h.Sum64()

		// Convert to float64 in range [-1, 1]
		embedding[i] = (float64(hashVal)/float64(^uint64(0)))*2 - 1
	}

	return NormalizeEmbedding(embedding)
}

// ReLU applies the ReLU activation function.
func ReLU(x float64) float64 {
	if x < 0 {
		return 0
	}
	return x
}

// ReLUVector applies ReLU to each element of a vector.
func ReLUVector(v []float64) []float64 {
	result := make([]float64, len(v))
	for i, x := range v {
		result[i] = ReLU(x)
	}
	return result
}

// ELU applies the ELU activation function with alpha=1.0.
func ELU(x float64) float64 {
	if x < 0 {
		return math.Exp(x) - 1
	}
	return x
}

// ELUVector applies ELU to each element of a vector.
func ELUVector(v []float64) []float64 {
	result := make([]float64, len(v))
	for i, x := range v {
		result[i] = ELU(x)
	}
	return result
}

// MatMul performs matrix multiplication between two 2D matrices.
// A is m x n, B is n x p, result is m x p.
func MatMul(A, B [][]float64) [][]float64 {
	if len(A) == 0 || len(B) == 0 || len(A[0]) != len(B) {
		return nil
	}

	m := len(A)
	n := len(B)
	p := len(B[0])

	result := make([][]float64, m)
	for i := range result {
		result[i] = make([]float64, p)
		for j := 0; j < p; j++ {
			for k := 0; k < n; k++ {
				result[i][j] += A[i][k] * B[k][j]
			}
		}
	}

	return result
}

// Transpose transposes a 2D matrix.
func Transpose(A [][]float64) [][]float64 {
	if len(A) == 0 {
		return nil
	}

	m := len(A)
	n := len(A[0])

	result := make([][]float64, n)
	for i := range result {
		result[i] = make([]float64, m)
		for j := 0; j < m; j++ {
			result[i][j] = A[j][i]
		}
	}

	return result
}

// VectorAdd adds two vectors element-wise.
func VectorAdd(a, b []float64) []float64 {
	if len(a) != len(b) {
		return nil
	}

	result := make([]float64, len(a))
	for i := range a {
		result[i] = a[i] + b[i]
	}
	return result
}

// VectorScale multiplies a vector by a scalar.
func VectorScale(v []float64, scale float64) []float64 {
	result := make([]float64, len(v))
	for i, x := range v {
		result[i] = x * scale
	}
	return result
}

// SinusoidalEncoding generates sinusoidal position encoding.
func SinusoidalEncoding(position int, dim int) []float64 {
	encoding := make([]float64, dim)

	for i := 0; i < dim; i++ {
		denominator := math.Pow(10000.0, float64(2*(i/2))/float64(dim))
		if i%2 == 0 {
			encoding[i] = math.Sin(float64(position) / denominator)
		} else {
			encoding[i] = math.Cos(float64(position) / denominator)
		}
	}

	return encoding
}

// Acosh computes the inverse hyperbolic cosine (arccosh).
func Acosh(x float64) float64 {
	if x < 1 {
		return 0
	}
	return math.Log(x + math.Sqrt(x*x-1))
}

// Clamp clamps a value between min and max.
func Clamp(x, min, max float64) float64 {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}

// ArgMax returns the index of the maximum value in a slice.
func ArgMax(values []float64) int {
	if len(values) == 0 {
		return -1
	}

	maxIdx := 0
	maxVal := values[0]
	for i, v := range values[1:] {
		if v > maxVal {
			maxVal = v
			maxIdx = i + 1
		}
	}
	return maxIdx
}

// TopKIndices returns the indices of the top-k values in descending order.
func TopKIndices(values []float64, k int) []int {
	if k <= 0 || len(values) == 0 {
		return []int{}
	}
	if k > len(values) {
		k = len(values)
	}

	// Create index-value pairs
	type pair struct {
		index int
		value float64
	}
	pairs := make([]pair, len(values))
	for i, v := range values {
		pairs[i] = pair{i, v}
	}

	// Simple selection sort for top-k (efficient for small k)
	for i := 0; i < k; i++ {
		maxIdx := i
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].value > pairs[maxIdx].value {
				maxIdx = j
			}
		}
		pairs[i], pairs[maxIdx] = pairs[maxIdx], pairs[i]
	}

	result := make([]int, k)
	for i := 0; i < k; i++ {
		result[i] = pairs[i].index
	}
	return result
}

// Mean computes the mean of a slice.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// Sum computes the sum of a slice.
func Sum(values []float64) float64 {
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum
}
