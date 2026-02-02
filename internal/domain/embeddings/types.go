// Package embeddings provides domain types for the embeddings service.
package embeddings

import (
	"time"
)

// ProviderType represents the type of embedding provider.
type ProviderType string

const (
	ProviderOpenAI ProviderType = "openai"
	ProviderLocal  ProviderType = "local"
	ProviderMock   ProviderType = "mock"
)

// NormalizationType represents the type of normalization to apply.
type NormalizationType string

const (
	NormalizationL2     NormalizationType = "l2"
	NormalizationL1     NormalizationType = "l1"
	NormalizationMinMax NormalizationType = "minmax"
	NormalizationZScore NormalizationType = "zscore"
	NormalizationNone   NormalizationType = "none"
)

// SimilarityMetric represents the type of similarity metric.
type SimilarityMetric string

const (
	SimilarityCosine    SimilarityMetric = "cosine"
	SimilarityEuclidean SimilarityMetric = "euclidean"
	SimilarityDot       SimilarityMetric = "dot"
)

// ChunkingStrategy represents the strategy for chunking documents.
type ChunkingStrategy string

const (
	ChunkingCharacter ChunkingStrategy = "character"
	ChunkingSentence  ChunkingStrategy = "sentence"
	ChunkingParagraph ChunkingStrategy = "paragraph"
	ChunkingToken     ChunkingStrategy = "token"
)

// ========================================================================
// Configuration Types
// ========================================================================

// CacheConfig configures the persistent cache.
type CacheConfig struct {
	// Enabled enables persistent disk cache.
	Enabled bool `json:"enabled"`

	// DBPath is the path to SQLite database file.
	DBPath string `json:"dbPath,omitempty"`

	// MaxSize is the maximum entries in cache (default: 10000).
	MaxSize int `json:"maxSize,omitempty"`

	// TTL is the time-to-live in milliseconds (default: 7 days).
	TTLMs int64 `json:"ttlMs,omitempty"`
}

// DefaultCacheConfig returns the default cache configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled: true,
		DBPath:  ".cache/embeddings.db",
		MaxSize: 10000,
		TTLMs:   7 * 24 * 60 * 60 * 1000, // 7 days
	}
}

// BaseConfig contains common configuration for all providers.
type BaseConfig struct {
	// Provider is the provider identifier.
	Provider ProviderType `json:"provider"`

	// Dimensions is the embedding dimensions.
	Dimensions int `json:"dimensions,omitempty"`

	// CacheSize is the in-memory cache size.
	CacheSize int `json:"cacheSize,omitempty"`

	// EnableCache enables caching.
	EnableCache bool `json:"enableCache,omitempty"`

	// Normalization is the normalization type.
	Normalization NormalizationType `json:"normalization,omitempty"`

	// PersistentCache is the persistent cache configuration.
	PersistentCache *CacheConfig `json:"persistentCache,omitempty"`
}

// OpenAIConfig configures the OpenAI provider.
type OpenAIConfig struct {
	BaseConfig

	// APIKey is the OpenAI API key.
	APIKey string `json:"apiKey"`

	// Model is the embedding model to use.
	Model string `json:"model,omitempty"`

	// BaseURL is the base URL override (for Azure OpenAI).
	BaseURL string `json:"baseURL,omitempty"`

	// Timeout is the request timeout in milliseconds.
	TimeoutMs int `json:"timeoutMs,omitempty"`

	// MaxRetries is the maximum number of retries.
	MaxRetries int `json:"maxRetries,omitempty"`
}

// DefaultOpenAIConfig returns the default OpenAI configuration.
func DefaultOpenAIConfig(apiKey string) OpenAIConfig {
	return OpenAIConfig{
		BaseConfig: BaseConfig{
			Provider:      ProviderOpenAI,
			Dimensions:    1536,
			CacheSize:     1000,
			EnableCache:   true,
			Normalization: NormalizationNone,
		},
		APIKey:     apiKey,
		Model:      "text-embedding-3-small",
		TimeoutMs:  30000,
		MaxRetries: 3,
	}
}

// OpenAI models
const (
	ModelTextEmbedding3Small = "text-embedding-3-small"
	ModelTextEmbedding3Large = "text-embedding-3-large"
	ModelTextEmbeddingAda002 = "text-embedding-ada-002"
)

// LocalConfig configures the local hash-based provider.
type LocalConfig struct {
	BaseConfig

	// Seed is the hash seed for reproducibility.
	Seed uint64 `json:"seed,omitempty"`
}

// DefaultLocalConfig returns the default local configuration.
func DefaultLocalConfig() LocalConfig {
	return LocalConfig{
		BaseConfig: BaseConfig{
			Provider:      ProviderLocal,
			Dimensions:    256,
			CacheSize:     1000,
			EnableCache:   true,
			Normalization: NormalizationL2,
		},
		Seed: 0,
	}
}

// MockConfig configures the mock provider.
type MockConfig struct {
	BaseConfig

	// SimulatedLatencyMs is the simulated latency in milliseconds.
	SimulatedLatencyMs int `json:"simulatedLatencyMs,omitempty"`

	// Deterministic makes outputs reproducible.
	Deterministic bool `json:"deterministic,omitempty"`
}

// DefaultMockConfig returns the default mock configuration.
func DefaultMockConfig() MockConfig {
	return MockConfig{
		BaseConfig: BaseConfig{
			Provider:      ProviderMock,
			Dimensions:    384,
			CacheSize:     1000,
			EnableCache:   true,
			Normalization: NormalizationL2,
		},
		SimulatedLatencyMs: 10,
		Deterministic:      true,
	}
}

// ChunkingConfig configures document chunking.
type ChunkingConfig struct {
	// MaxChunkSize is the maximum chunk size in characters (default: 512).
	MaxChunkSize int `json:"maxChunkSize,omitempty"`

	// Overlap is the overlap between chunks in characters (default: 50).
	Overlap int `json:"overlap,omitempty"`

	// Strategy is the chunking strategy (default: sentence).
	Strategy ChunkingStrategy `json:"strategy,omitempty"`

	// MinChunkSize is the minimum chunk size (default: 100).
	MinChunkSize int `json:"minChunkSize,omitempty"`

	// IncludeMetadata includes metadata with chunks.
	IncludeMetadata bool `json:"includeMetadata,omitempty"`
}

// DefaultChunkingConfig returns the default chunking configuration.
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{
		MaxChunkSize:    512,
		Overlap:         50,
		Strategy:        ChunkingSentence,
		MinChunkSize:    100,
		IncludeMetadata: true,
	}
}

// ========================================================================
// Result Types
// ========================================================================

// TokenUsage contains token usage information.
type TokenUsage struct {
	PromptTokens int `json:"promptTokens"`
	TotalTokens  int `json:"totalTokens"`
}

// EmbeddingResult contains the result of a single embedding.
type EmbeddingResult struct {
	// Embedding is the embedding vector.
	Embedding []float32 `json:"embedding"`

	// LatencyMs is the latency in milliseconds.
	LatencyMs float64 `json:"latencyMs"`

	// Usage is the token usage (for API providers).
	Usage *TokenUsage `json:"usage,omitempty"`

	// Cached indicates if the result was from cache.
	Cached bool `json:"cached,omitempty"`

	// PersistentCached indicates if from persistent cache.
	PersistentCached bool `json:"persistentCached,omitempty"`

	// Normalized indicates if the embedding was normalized.
	Normalized bool `json:"normalized,omitempty"`
}

// CacheStats contains cache statistics.
type CacheStats struct {
	Hits   int `json:"hits"`
	Misses int `json:"misses"`
}

// BatchEmbeddingResult contains the result of batch embedding.
type BatchEmbeddingResult struct {
	// Embeddings is the array of embeddings.
	Embeddings [][]float32 `json:"embeddings"`

	// TotalLatencyMs is the total latency in milliseconds.
	TotalLatencyMs float64 `json:"totalLatencyMs"`

	// AvgLatencyMs is the average latency per embedding.
	AvgLatencyMs float64 `json:"avgLatencyMs"`

	// Usage is the token usage (for API providers).
	Usage *TokenUsage `json:"usage,omitempty"`

	// CacheStats contains cache statistics.
	CacheStats *CacheStats `json:"cacheStats,omitempty"`
}

// SimilarityResult contains the result of a similarity calculation.
type SimilarityResult struct {
	// Score is the similarity score.
	Score float64 `json:"score"`

	// Metric is the metric used.
	Metric SimilarityMetric `json:"metric"`
}

// ========================================================================
// Chunk Types
// ========================================================================

// Chunk represents a chunk of text with metadata.
type Chunk struct {
	// Text is the chunk text content.
	Text string `json:"text"`

	// Index is the original index in document.
	Index int `json:"index"`

	// StartPos is the start position in original text.
	StartPos int `json:"startPos"`

	// EndPos is the end position in original text.
	EndPos int `json:"endPos"`

	// Length is the character count.
	Length int `json:"length"`

	// TokenCount is the approximate token count.
	TokenCount int `json:"tokenCount"`
}

// ChunkedDocument contains the result of document chunking.
type ChunkedDocument struct {
	// Chunks is the array of chunks.
	Chunks []Chunk `json:"chunks"`

	// OriginalLength is the original text length.
	OriginalLength int `json:"originalLength"`

	// TotalChunks is the total chunks created.
	TotalChunks int `json:"totalChunks"`

	// Config is the configuration used.
	Config ChunkingConfig `json:"config"`
}

// ========================================================================
// Cache Entry Types
// ========================================================================

// CacheEntry represents a cached embedding.
type CacheEntry struct {
	// Key is the cache key (hash of text).
	Key string `json:"key"`

	// Embedding is the cached embedding.
	Embedding []float32 `json:"embedding"`

	// Dimensions is the embedding dimensions.
	Dimensions int `json:"dimensions"`

	// Provider is the provider that created it.
	Provider ProviderType `json:"provider"`

	// CreatedAt is when the entry was created.
	CreatedAt time.Time `json:"createdAt"`

	// AccessedAt is when the entry was last accessed.
	AccessedAt time.Time `json:"accessedAt"`

	// AccessCount is the number of times accessed.
	AccessCount int `json:"accessCount"`
}

// ========================================================================
// Event Types
// ========================================================================

// EventType represents the type of embedding event.
type EventType string

const (
	EventEmbedStart     EventType = "embed_start"
	EventEmbedComplete  EventType = "embed_complete"
	EventEmbedError     EventType = "embed_error"
	EventBatchStart     EventType = "batch_start"
	EventBatchComplete  EventType = "batch_complete"
	EventCacheHit       EventType = "cache_hit"
	EventCacheEviction  EventType = "cache_eviction"
)

// EmbeddingEvent represents an embedding service event.
type EmbeddingEvent struct {
	// Type is the event type.
	Type EventType `json:"type"`

	// Text is the text being embedded (for single operations).
	Text string `json:"text,omitempty"`

	// Count is the batch count.
	Count int `json:"count,omitempty"`

	// LatencyMs is the operation latency.
	LatencyMs float64 `json:"latencyMs,omitempty"`

	// Error is the error message (for error events).
	Error string `json:"error,omitempty"`

	// Size is the cache size (for eviction events).
	Size int `json:"size,omitempty"`
}

// EventHandler handles embedding events.
type EventHandler func(event EmbeddingEvent)

// ========================================================================
// Service Statistics
// ========================================================================

// ServiceStats contains embedding service statistics.
type ServiceStats struct {
	// Provider is the active provider.
	Provider ProviderType `json:"provider"`

	// TotalEmbeddings is the total embeddings generated.
	TotalEmbeddings int64 `json:"totalEmbeddings"`

	// CacheHits is the cache hit count.
	CacheHits int64 `json:"cacheHits"`

	// CacheMisses is the cache miss count.
	CacheMisses int64 `json:"cacheMisses"`

	// TotalLatencyMs is the cumulative latency.
	TotalLatencyMs float64 `json:"totalLatencyMs"`

	// AvgLatencyMs is the average latency.
	AvgLatencyMs float64 `json:"avgLatencyMs"`

	// CacheSize is the current cache size.
	CacheSize int `json:"cacheSize"`

	// CacheMaxSize is the maximum cache size.
	CacheMaxSize int `json:"cacheMaxSize"`
}
