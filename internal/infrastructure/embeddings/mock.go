// Package embeddings provides infrastructure for the embeddings service.
package embeddings

import (
	"context"
	"hash/fnv"
	"math"
	"sync"
	"time"

	domainEmbeddings "github.com/anthropics/claude-flow-go/internal/domain/embeddings"
)

// MockProvider provides deterministic mock embeddings for testing.
type MockProvider struct {
	mu         sync.RWMutex
	config     domainEmbeddings.MockConfig
	closed     bool
	normalize  func([]float32) []float32
	callCount  int
}

// NewMockProvider creates a new mock provider.
func NewMockProvider(config domainEmbeddings.MockConfig) (*MockProvider, error) {
	if config.Dimensions <= 0 {
		config.Dimensions = 384
	}

	if config.SimulatedLatencyMs <= 0 {
		config.SimulatedLatencyMs = 10
	}

	p := &MockProvider{
		config: config,
	}

	// Set up normalization
	if config.Normalization == "" {
		config.Normalization = domainEmbeddings.NormalizationL2
	}
	p.normalize = getNormalizer(config.Normalization)

	return p, nil
}

// Embed generates a mock embedding for a single text.
func (p *MockProvider) Embed(ctx context.Context, text string) (*domainEmbeddings.EmbeddingResult, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrProviderClosed
	}
	p.callCount++
	p.mu.Unlock()

	if text == "" {
		return nil, ErrEmptyInput
	}

	startTime := time.Now()

	// Simulate latency
	if p.config.SimulatedLatencyMs > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(p.config.SimulatedLatencyMs) * time.Millisecond):
		}
	}

	embedding := p.generateMockEmbedding(text)

	result := &domainEmbeddings.EmbeddingResult{
		Embedding: embedding,
		LatencyMs: float64(time.Since(startTime).Milliseconds()),
		Usage: &domainEmbeddings.TokenUsage{
			PromptTokens: len(text) / 4, // Approximate
			TotalTokens:  len(text) / 4,
		},
		Normalized: p.config.Normalization != domainEmbeddings.NormalizationNone,
	}

	return result, nil
}

// EmbedBatch generates mock embeddings for multiple texts.
func (p *MockProvider) EmbedBatch(ctx context.Context, texts []string) (*domainEmbeddings.BatchEmbeddingResult, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrProviderClosed
	}
	p.callCount++
	p.mu.Unlock()

	if len(texts) == 0 {
		return nil, ErrEmptyInput
	}

	startTime := time.Now()

	// Simulate latency (proportional to batch size)
	if p.config.SimulatedLatencyMs > 0 {
		latency := time.Duration(p.config.SimulatedLatencyMs*len(texts)/2) * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(latency):
		}
	}

	embeddings := make([][]float32, len(texts))
	totalTokens := 0
	for i, text := range texts {
		if text == "" {
			embeddings[i] = make([]float32, p.config.Dimensions)
		} else {
			embeddings[i] = p.generateMockEmbedding(text)
			totalTokens += len(text) / 4
		}
	}

	totalLatency := float64(time.Since(startTime).Milliseconds())

	result := &domainEmbeddings.BatchEmbeddingResult{
		Embeddings:     embeddings,
		TotalLatencyMs: totalLatency,
		AvgLatencyMs:   totalLatency / float64(len(texts)),
		Usage: &domainEmbeddings.TokenUsage{
			PromptTokens: totalTokens,
			TotalTokens:  totalTokens,
		},
	}

	return result, nil
}

// generateMockEmbedding generates a deterministic mock embedding.
func (p *MockProvider) generateMockEmbedding(text string) []float32 {
	dims := p.config.Dimensions
	embedding := make([]float32, dims)

	if p.config.Deterministic {
		// Generate deterministic embedding based on text hash
		h := fnv.New64a()
		h.Write([]byte(text))
		seed := h.Sum64()

		for i := 0; i < dims; i++ {
			// Use a simple LCG to generate pseudo-random values
			seed = seed*6364136223846793005 + 1442695040888963407
			embedding[i] = float32(seed)/float32(math.MaxUint64)*2.0 - 1.0
		}
	} else {
		// Generate semi-random embedding (still consistent per text)
		h := fnv.New64a()
		h.Write([]byte(text))
		h.Write([]byte{byte(p.callCount >> 8), byte(p.callCount)})
		seed := h.Sum64()

		for i := 0; i < dims; i++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			embedding[i] = float32(seed)/float32(math.MaxUint64)*2.0 - 1.0
		}
	}

	// Apply normalization
	if p.normalize != nil {
		embedding = p.normalize(embedding)
	}

	return embedding
}

// Dimensions returns the embedding dimensions.
func (p *MockProvider) Dimensions() int {
	return p.config.Dimensions
}

// Name returns the provider name.
func (p *MockProvider) Name() domainEmbeddings.ProviderType {
	return domainEmbeddings.ProviderMock
}

// Close closes the provider.
func (p *MockProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

// GetCallCount returns the number of embed calls made.
func (p *MockProvider) GetCallCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.callCount
}

// Reset resets the mock provider state.
func (p *MockProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callCount = 0
}
