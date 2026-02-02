// Package embeddings provides application services for embedding generation.
package embeddings

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	domainEmbeddings "github.com/anthropics/claude-flow-go/internal/domain/embeddings"
	infraEmbeddings "github.com/anthropics/claude-flow-go/internal/infrastructure/embeddings"
)

// EmbeddingService provides high-level embedding operations.
type EmbeddingService struct {
	mu              sync.RWMutex
	provider        infraEmbeddings.Provider
	cache           *infraEmbeddings.Cache
	memoryCache     *infraEmbeddings.MemoryCache
	chunker         *infraEmbeddings.Chunker
	eventHandlers   []domainEmbeddings.EventHandler
	
	// Statistics
	totalEmbeddings int64
	cacheHits       int64
	cacheMisses     int64
	totalLatencyMs  float64

	// Configuration
	enableCache       bool
	enableMemoryCache bool
	normalization     domainEmbeddings.NormalizationType
}

// ServiceConfig configures the embedding service.
type ServiceConfig struct {
	// Provider configuration (one of these should be set)
	OpenAI *domainEmbeddings.OpenAIConfig
	Local  *domainEmbeddings.LocalConfig
	Mock   *domainEmbeddings.MockConfig

	// Cache configuration
	EnableCache       bool
	EnableMemoryCache bool
	MemoryCacheSize   int
	PersistentCache   *domainEmbeddings.CacheConfig

	// Chunking configuration
	ChunkingConfig *domainEmbeddings.ChunkingConfig

	// Normalization
	Normalization domainEmbeddings.NormalizationType
}

// NewEmbeddingService creates a new embedding service.
func NewEmbeddingService(config ServiceConfig) (*EmbeddingService, error) {
	factory := infraEmbeddings.NewProviderFactory()

	// Create provider
	var provider infraEmbeddings.Provider
	var err error

	switch {
	case config.OpenAI != nil:
		provider, err = factory.CreateOpenAI(*config.OpenAI)
	case config.Local != nil:
		provider, err = factory.CreateLocal(*config.Local)
	case config.Mock != nil:
		provider, err = factory.CreateMock(*config.Mock)
	default:
		// Default to local provider
		localConfig := domainEmbeddings.DefaultLocalConfig()
		provider, err = factory.CreateLocal(localConfig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	svc := &EmbeddingService{
		provider:          provider,
		enableCache:       config.EnableCache,
		enableMemoryCache: config.EnableMemoryCache,
		normalization:     config.Normalization,
	}

	// Create persistent cache
	if config.EnableCache && config.PersistentCache != nil {
		cache, err := infraEmbeddings.NewCache(*config.PersistentCache)
		if err != nil {
			provider.Close()
			return nil, fmt.Errorf("failed to create cache: %w", err)
		}
		svc.cache = cache
	}

	// Create memory cache
	if config.EnableMemoryCache {
		cacheSize := config.MemoryCacheSize
		if cacheSize <= 0 {
			cacheSize = 1000
		}
		svc.memoryCache = infraEmbeddings.NewMemoryCache(cacheSize)
	}

	// Create chunker
	if config.ChunkingConfig != nil {
		svc.chunker = infraEmbeddings.NewChunker(*config.ChunkingConfig)
	} else {
		svc.chunker = infraEmbeddings.NewChunker(domainEmbeddings.DefaultChunkingConfig())
	}

	return svc, nil
}

// CreateOpenAIService creates an embedding service with OpenAI provider.
func CreateOpenAIService(apiKey string, model string) (*EmbeddingService, error) {
	openAIConfig := domainEmbeddings.DefaultOpenAIConfig(apiKey)
	if model != "" {
		openAIConfig.Model = model
	}

	cacheConfig := domainEmbeddings.DefaultCacheConfig()

	return NewEmbeddingService(ServiceConfig{
		OpenAI:            &openAIConfig,
		EnableCache:       true,
		EnableMemoryCache: true,
		MemoryCacheSize:   1000,
		PersistentCache:   &cacheConfig,
	})
}

// CreateLocalService creates an embedding service with local provider.
func CreateLocalService(dimensions int) (*EmbeddingService, error) {
	localConfig := domainEmbeddings.DefaultLocalConfig()
	if dimensions > 0 {
		localConfig.Dimensions = dimensions
	}

	cacheConfig := domainEmbeddings.DefaultCacheConfig()

	return NewEmbeddingService(ServiceConfig{
		Local:             &localConfig,
		EnableCache:       true,
		EnableMemoryCache: true,
		MemoryCacheSize:   1000,
		PersistentCache:   &cacheConfig,
	})
}

// CreateMockService creates an embedding service with mock provider.
func CreateMockService() (*EmbeddingService, error) {
	mockConfig := domainEmbeddings.DefaultMockConfig()

	return NewEmbeddingService(ServiceConfig{
		Mock:              &mockConfig,
		EnableCache:       false,
		EnableMemoryCache: true,
		MemoryCacheSize:   100,
	})
}

// Embed generates an embedding for a single text.
func (s *EmbeddingService) Embed(ctx context.Context, text string) (*domainEmbeddings.EmbeddingResult, error) {
	startTime := time.Now()

	s.emitEvent(domainEmbeddings.EmbeddingEvent{
		Type: domainEmbeddings.EventEmbedStart,
		Text: text,
	})

	// Check memory cache first
	cacheKey := infraEmbeddings.GenerateCacheKey(text, s.provider.Name())
	
	if s.enableMemoryCache && s.memoryCache != nil {
		if embedding, ok := s.memoryCache.Get(cacheKey); ok {
			atomic.AddInt64(&s.cacheHits, 1)
			atomic.AddInt64(&s.totalEmbeddings, 1)
			
			result := &domainEmbeddings.EmbeddingResult{
				Embedding: embedding,
				LatencyMs: float64(time.Since(startTime).Microseconds()) / 1000.0,
				Cached:    true,
			}
			
			s.emitEvent(domainEmbeddings.EmbeddingEvent{
				Type:      domainEmbeddings.EventCacheHit,
				Text:      text,
				LatencyMs: result.LatencyMs,
			})
			
			return result, nil
		}
	}

	// Check persistent cache
	if s.enableCache && s.cache != nil {
		entry, err := s.cache.Get(cacheKey)
		if err == nil {
			atomic.AddInt64(&s.cacheHits, 1)
			atomic.AddInt64(&s.totalEmbeddings, 1)

			// Store in memory cache
			if s.memoryCache != nil {
				s.memoryCache.Set(cacheKey, entry.Embedding)
			}

			result := &domainEmbeddings.EmbeddingResult{
				Embedding:        entry.Embedding,
				LatencyMs:        float64(time.Since(startTime).Microseconds()) / 1000.0,
				Cached:           true,
				PersistentCached: true,
			}

			s.emitEvent(domainEmbeddings.EmbeddingEvent{
				Type:      domainEmbeddings.EventCacheHit,
				Text:      text,
				LatencyMs: result.LatencyMs,
			})

			return result, nil
		}
	}

	atomic.AddInt64(&s.cacheMisses, 1)

	// Generate embedding from provider
	result, err := s.provider.Embed(ctx, text)
	if err != nil {
		s.emitEvent(domainEmbeddings.EmbeddingEvent{
			Type:  domainEmbeddings.EventEmbedError,
			Text:  text,
			Error: err.Error(),
		})
		return nil, err
	}

	atomic.AddInt64(&s.totalEmbeddings, 1)

	// Apply normalization if configured
	if s.normalization != "" && s.normalization != domainEmbeddings.NormalizationNone {
		result.Embedding = infraEmbeddings.Normalize(result.Embedding, s.normalization)
		result.Normalized = true
	}

	// Store in caches
	if s.memoryCache != nil {
		s.memoryCache.Set(cacheKey, result.Embedding)
	}
	if s.cache != nil {
		s.cache.Set(cacheKey, result.Embedding, s.provider.Name())
	}

	s.mu.Lock()
	s.totalLatencyMs += result.LatencyMs
	s.mu.Unlock()

	s.emitEvent(domainEmbeddings.EmbeddingEvent{
		Type:      domainEmbeddings.EventEmbedComplete,
		Text:      text,
		LatencyMs: result.LatencyMs,
	})

	return result, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (s *EmbeddingService) EmbedBatch(ctx context.Context, texts []string) (*domainEmbeddings.BatchEmbeddingResult, error) {
	startTime := time.Now()

	s.emitEvent(domainEmbeddings.EmbeddingEvent{
		Type:  domainEmbeddings.EventBatchStart,
		Count: len(texts),
	})

	// Separate cached and uncached texts
	cachedEmbeddings := make(map[int][]float32)
	uncachedIndices := make([]int, 0)
	uncachedTexts := make([]string, 0)
	cacheHits := 0
	cacheMisses := 0

	for i, text := range texts {
		cacheKey := infraEmbeddings.GenerateCacheKey(text, s.provider.Name())

		// Check memory cache
		if s.enableMemoryCache && s.memoryCache != nil {
			if embedding, ok := s.memoryCache.Get(cacheKey); ok {
				cachedEmbeddings[i] = embedding
				cacheHits++
				continue
			}
		}

		// Check persistent cache
		if s.enableCache && s.cache != nil {
			entry, err := s.cache.Get(cacheKey)
			if err == nil {
				cachedEmbeddings[i] = entry.Embedding
				if s.memoryCache != nil {
					s.memoryCache.Set(cacheKey, entry.Embedding)
				}
				cacheHits++
				continue
			}
		}

		cacheMisses++
		uncachedIndices = append(uncachedIndices, i)
		uncachedTexts = append(uncachedTexts, text)
	}

	// Generate embeddings for uncached texts
	var batchResult *domainEmbeddings.BatchEmbeddingResult
	if len(uncachedTexts) > 0 {
		var err error
		batchResult, err = s.provider.EmbedBatch(ctx, uncachedTexts)
		if err != nil {
			s.emitEvent(domainEmbeddings.EmbeddingEvent{
				Type:  domainEmbeddings.EventEmbedError,
				Count: len(texts),
				Error: err.Error(),
			})
			return nil, err
		}

		// Apply normalization and cache
		for i, embedding := range batchResult.Embeddings {
			origIdx := uncachedIndices[i]
			text := uncachedTexts[i]
			cacheKey := infraEmbeddings.GenerateCacheKey(text, s.provider.Name())

			if s.normalization != "" && s.normalization != domainEmbeddings.NormalizationNone {
				embedding = infraEmbeddings.Normalize(embedding, s.normalization)
			}

			cachedEmbeddings[origIdx] = embedding

			if s.memoryCache != nil {
				s.memoryCache.Set(cacheKey, embedding)
			}
			if s.cache != nil {
				s.cache.Set(cacheKey, embedding, s.provider.Name())
			}
		}
	}

	// Assemble final result
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = cachedEmbeddings[i]
	}

	totalLatency := float64(time.Since(startTime).Milliseconds())

	atomic.AddInt64(&s.totalEmbeddings, int64(len(texts)))
	atomic.AddInt64(&s.cacheHits, int64(cacheHits))
	atomic.AddInt64(&s.cacheMisses, int64(cacheMisses))

	s.mu.Lock()
	s.totalLatencyMs += totalLatency
	s.mu.Unlock()

	result := &domainEmbeddings.BatchEmbeddingResult{
		Embeddings:     embeddings,
		TotalLatencyMs: totalLatency,
		AvgLatencyMs:   totalLatency / float64(len(texts)),
		CacheStats: &domainEmbeddings.CacheStats{
			Hits:   cacheHits,
			Misses: cacheMisses,
		},
	}

	if batchResult != nil && batchResult.Usage != nil {
		result.Usage = batchResult.Usage
	}

	s.emitEvent(domainEmbeddings.EmbeddingEvent{
		Type:      domainEmbeddings.EventBatchComplete,
		Count:     len(texts),
		LatencyMs: totalLatency,
	})

	return result, nil
}

// EmbedDocument chunks and embeds a document.
func (s *EmbeddingService) EmbedDocument(ctx context.Context, text string) ([][]float32, *domainEmbeddings.ChunkedDocument, error) {
	// Chunk the document
	chunked := s.chunker.Chunk(text)

	if len(chunked.Chunks) == 0 {
		return nil, chunked, nil
	}

	// Extract chunk texts
	texts := infraEmbeddings.GetChunkTexts(chunked.Chunks)

	// Embed all chunks
	result, err := s.EmbedBatch(ctx, texts)
	if err != nil {
		return nil, chunked, err
	}

	return result.Embeddings, chunked, nil
}

// ComputeSimilarity computes similarity between two embeddings.
func (s *EmbeddingService) ComputeSimilarity(a, b []float32, metric domainEmbeddings.SimilarityMetric) *domainEmbeddings.SimilarityResult {
	return infraEmbeddings.ComputeSimilarity(a, b, metric)
}

// FindMostSimilar finds the most similar embedding from candidates.
func (s *EmbeddingService) FindMostSimilar(query []float32, candidates [][]float32, metric domainEmbeddings.SimilarityMetric) (int, float64) {
	return infraEmbeddings.FindMostSimilar(query, candidates, metric)
}

// FindTopK finds the top K most similar embeddings.
func (s *EmbeddingService) FindTopK(query []float32, candidates [][]float32, k int, metric domainEmbeddings.SimilarityMetric) ([]int, []float64) {
	return infraEmbeddings.FindTopK(query, candidates, k, metric)
}

// AddEventHandler adds an event handler.
func (s *EmbeddingService) AddEventHandler(handler domainEmbeddings.EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventHandlers = append(s.eventHandlers, handler)
}

// emitEvent emits an event to all handlers.
func (s *EmbeddingService) emitEvent(event domainEmbeddings.EmbeddingEvent) {
	s.mu.RLock()
	handlers := s.eventHandlers
	s.mu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}

// GetStats returns service statistics.
func (s *EmbeddingService) GetStats() domainEmbeddings.ServiceStats {
	s.mu.RLock()
	totalLatency := s.totalLatencyMs
	s.mu.RUnlock()

	total := atomic.LoadInt64(&s.totalEmbeddings)
	hits := atomic.LoadInt64(&s.cacheHits)
	misses := atomic.LoadInt64(&s.cacheMisses)

	avgLatency := 0.0
	if total > 0 {
		avgLatency = totalLatency / float64(total)
	}

	cacheSize := 0
	cacheMaxSize := 0
	if s.memoryCache != nil {
		cacheSize = s.memoryCache.Size()
		cacheMaxSize = 1000 // Default
	}

	return domainEmbeddings.ServiceStats{
		Provider:        s.provider.Name(),
		TotalEmbeddings: total,
		CacheHits:       hits,
		CacheMisses:     misses,
		TotalLatencyMs:  totalLatency,
		AvgLatencyMs:    avgLatency,
		CacheSize:       cacheSize,
		CacheMaxSize:    cacheMaxSize,
	}
}

// GetProvider returns the underlying provider.
func (s *EmbeddingService) GetProvider() infraEmbeddings.Provider {
	return s.provider
}

// Dimensions returns the embedding dimensions.
func (s *EmbeddingService) Dimensions() int {
	return s.provider.Dimensions()
}

// ClearCache clears all caches.
func (s *EmbeddingService) ClearCache() error {
	if s.memoryCache != nil {
		s.memoryCache.Clear()
	}
	if s.cache != nil {
		return s.cache.Clear()
	}
	return nil
}

// Close closes the service and releases resources.
func (s *EmbeddingService) Close() error {
	var errs []error

	if s.cache != nil {
		if err := s.cache.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := s.provider.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
