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

// LocalProvider provides embeddings using local hash-based generation.
// This is a deterministic provider that generates embeddings without external APIs.
type LocalProvider struct {
	mu         sync.RWMutex
	config     domainEmbeddings.LocalConfig
	closed     bool
	normalize  func([]float32) []float32
}

// NewLocalProvider creates a new local hash-based provider.
func NewLocalProvider(config domainEmbeddings.LocalConfig) (*LocalProvider, error) {
	if config.Dimensions <= 0 {
		config.Dimensions = 256
	}

	p := &LocalProvider{
		config: config,
	}

	// Set up normalization (default L2 for local)
	if config.Normalization == "" {
		config.Normalization = domainEmbeddings.NormalizationL2
	}
	p.normalize = getNormalizer(config.Normalization)

	return p, nil
}

// Embed generates an embedding for a single text.
func (p *LocalProvider) Embed(ctx context.Context, text string) (*domainEmbeddings.EmbeddingResult, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrProviderClosed
	}
	p.mu.RUnlock()

	if text == "" {
		return nil, ErrEmptyInput
	}

	startTime := time.Now()

	embedding := p.generateEmbedding(text)

	result := &domainEmbeddings.EmbeddingResult{
		Embedding:  embedding,
		LatencyMs:  float64(time.Since(startTime).Microseconds()) / 1000.0,
		Normalized: p.config.Normalization != domainEmbeddings.NormalizationNone,
	}

	return result, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (p *LocalProvider) EmbedBatch(ctx context.Context, texts []string) (*domainEmbeddings.BatchEmbeddingResult, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrProviderClosed
	}
	p.mu.RUnlock()

	if len(texts) == 0 {
		return nil, ErrEmptyInput
	}

	startTime := time.Now()

	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		if text == "" {
			embeddings[i] = make([]float32, p.config.Dimensions)
		} else {
			embeddings[i] = p.generateEmbedding(text)
		}
	}

	totalLatency := float64(time.Since(startTime).Microseconds()) / 1000.0

	result := &domainEmbeddings.BatchEmbeddingResult{
		Embeddings:     embeddings,
		TotalLatencyMs: totalLatency,
		AvgLatencyMs:   totalLatency / float64(len(texts)),
	}

	return result, nil
}

// generateEmbedding generates an embedding using FNV-1a hash.
func (p *LocalProvider) generateEmbedding(text string) []float32 {
	dims := p.config.Dimensions
	embedding := make([]float32, dims)

	// Generate embedding using multiple hash passes
	// This creates a more distributed representation
	for pass := 0; pass < 4; pass++ {
		h := fnv.New64a()
		
		// Include seed and pass number for variation
		h.Write([]byte{byte(p.config.Seed >> 56), byte(p.config.Seed >> 48),
			byte(p.config.Seed >> 40), byte(p.config.Seed >> 32),
			byte(p.config.Seed >> 24), byte(p.config.Seed >> 16),
			byte(p.config.Seed >> 8), byte(p.config.Seed)})
		h.Write([]byte{byte(pass)})
		h.Write([]byte(text))

		hash := h.Sum64()

		// Distribute hash across embedding dimensions
		for i := 0; i < dims; i++ {
			// Create a unique value for each dimension
			h2 := fnv.New64a()
			h2.Write([]byte{byte(hash >> 56), byte(hash >> 48),
				byte(hash >> 40), byte(hash >> 32),
				byte(hash >> 24), byte(hash >> 16),
				byte(hash >> 8), byte(hash)})
			h2.Write([]byte{byte(i >> 8), byte(i)})
			dimHash := h2.Sum64()

			// Convert to float in range [-1, 1]
			val := float32(dimHash) / float32(math.MaxUint64) * 2.0 - 1.0
			embedding[i] += val / 4.0 // Divide by number of passes
		}
	}

	// Apply normalization
	if p.normalize != nil {
		embedding = p.normalize(embedding)
	}

	return embedding
}

// Dimensions returns the embedding dimensions.
func (p *LocalProvider) Dimensions() int {
	return p.config.Dimensions
}

// Name returns the provider name.
func (p *LocalProvider) Name() domainEmbeddings.ProviderType {
	return domainEmbeddings.ProviderLocal
}

// Close closes the provider.
func (p *LocalProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}
