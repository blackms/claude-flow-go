// Package embeddings provides infrastructure for the embeddings service.
package embeddings

import (
	"context"

	domainEmbeddings "github.com/anthropics/claude-flow-go/internal/domain/embeddings"
)

// Provider defines the interface for embedding providers.
type Provider interface {
	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) (*domainEmbeddings.EmbeddingResult, error)

	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) (*domainEmbeddings.BatchEmbeddingResult, error)

	// Dimensions returns the embedding dimensions.
	Dimensions() int

	// Name returns the provider name.
	Name() domainEmbeddings.ProviderType

	// Close closes the provider and releases resources.
	Close() error
}

// ProviderFactory creates embedding providers.
type ProviderFactory struct{}

// NewProviderFactory creates a new provider factory.
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{}
}

// CreateOpenAI creates an OpenAI provider.
func (f *ProviderFactory) CreateOpenAI(config domainEmbeddings.OpenAIConfig) (Provider, error) {
	return NewOpenAIProvider(config)
}

// CreateLocal creates a local hash-based provider.
func (f *ProviderFactory) CreateLocal(config domainEmbeddings.LocalConfig) (Provider, error) {
	return NewLocalProvider(config)
}

// CreateMock creates a mock provider.
func (f *ProviderFactory) CreateMock(config domainEmbeddings.MockConfig) (Provider, error) {
	return NewMockProvider(config)
}

// CreateProvider creates a provider based on the provider type.
func (f *ProviderFactory) CreateProvider(providerType domainEmbeddings.ProviderType, config interface{}) (Provider, error) {
	switch providerType {
	case domainEmbeddings.ProviderOpenAI:
		if cfg, ok := config.(domainEmbeddings.OpenAIConfig); ok {
			return f.CreateOpenAI(cfg)
		}
	case domainEmbeddings.ProviderLocal:
		if cfg, ok := config.(domainEmbeddings.LocalConfig); ok {
			return f.CreateLocal(cfg)
		}
	case domainEmbeddings.ProviderMock:
		if cfg, ok := config.(domainEmbeddings.MockConfig); ok {
			return f.CreateMock(cfg)
		}
	}
	return nil, ErrUnsupportedProvider
}
