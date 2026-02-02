// Package embeddings provides infrastructure for the embeddings service.
package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	domainEmbeddings "github.com/anthropics/claude-flow-go/internal/domain/embeddings"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	maxBatchSize         = 2048
)

// OpenAI API request/response types.
type openAIEmbeddingRequest struct {
	Input          interface{} `json:"input"`
	Model          string      `json:"model"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
	Dimensions     int         `json:"dimensions,omitempty"`
}

type openAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// OpenAIProvider provides embeddings using OpenAI API.
type OpenAIProvider struct {
	mu         sync.RWMutex
	config     domainEmbeddings.OpenAIConfig
	client     *http.Client
	baseURL    string
	closed     bool
	normalize  func([]float32) []float32
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(config domainEmbeddings.OpenAIConfig) (*OpenAIProvider, error) {
	if config.APIKey == "" {
		return nil, ErrAPIKeyRequired
	}

	if config.Model == "" {
		config.Model = domainEmbeddings.ModelTextEmbedding3Small
	}

	if config.TimeoutMs <= 0 {
		config.TimeoutMs = 30000
	}

	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}

	client := &http.Client{
		Timeout: time.Duration(config.TimeoutMs) * time.Millisecond,
	}

	p := &OpenAIProvider{
		config:  config,
		client:  client,
		baseURL: baseURL,
	}

	// Set up normalization
	p.normalize = getNormalizer(config.Normalization)

	return p, nil
}

// Embed generates an embedding for a single text.
func (p *OpenAIProvider) Embed(ctx context.Context, text string) (*domainEmbeddings.EmbeddingResult, error) {
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

	embedding, usage, err := p.callAPI(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	result := &domainEmbeddings.EmbeddingResult{
		Embedding: embedding[0],
		LatencyMs: float64(time.Since(startTime).Milliseconds()),
		Usage: &domainEmbeddings.TokenUsage{
			PromptTokens: usage.PromptTokens,
			TotalTokens:  usage.TotalTokens,
		},
		Normalized: p.config.Normalization != domainEmbeddings.NormalizationNone,
	}

	return result, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) (*domainEmbeddings.BatchEmbeddingResult, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrProviderClosed
	}
	p.mu.RUnlock()

	if len(texts) == 0 {
		return nil, ErrEmptyInput
	}

	if len(texts) > maxBatchSize {
		return nil, fmt.Errorf("%w: maximum %d, got %d", ErrBatchTooLarge, maxBatchSize, len(texts))
	}

	startTime := time.Now()

	embeddings, usage, err := p.callAPI(ctx, texts)
	if err != nil {
		return nil, err
	}

	totalLatency := float64(time.Since(startTime).Milliseconds())

	result := &domainEmbeddings.BatchEmbeddingResult{
		Embeddings:     embeddings,
		TotalLatencyMs: totalLatency,
		AvgLatencyMs:   totalLatency / float64(len(texts)),
		Usage: &domainEmbeddings.TokenUsage{
			PromptTokens: usage.PromptTokens,
			TotalTokens:  usage.TotalTokens,
		},
	}

	return result, nil
}

// callAPI calls the OpenAI embeddings API with retry logic.
func (p *OpenAIProvider) callAPI(ctx context.Context, texts []string) ([][]float32, *domainEmbeddings.TokenUsage, error) {
	var input interface{}
	if len(texts) == 1 {
		input = texts[0]
	} else {
		input = texts
	}

	reqBody := openAIEmbeddingRequest{
		Input: input,
		Model: p.config.Model,
	}

	// Set dimensions for v3 models
	if p.config.Dimensions > 0 && (p.config.Model == domainEmbeddings.ModelTextEmbedding3Small ||
		p.config.Model == domainEmbeddings.ModelTextEmbedding3Large) {
		reqBody.Dimensions = p.config.Dimensions
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, ...
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			if backoff > 32*time.Second {
				backoff = 32 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		embeddings, usage, err := p.doRequest(ctx, jsonBody)
		if err == nil {
			return embeddings, usage, nil
		}

		lastErr = err

		// Don't retry on non-retryable errors
		if err == ErrAPIKeyRequired || err == ErrEmptyInput {
			return nil, nil, err
		}
	}

	return nil, nil, lastErr
}

// doRequest performs a single API request.
func (p *OpenAIProvider) doRequest(ctx context.Context, jsonBody []byte) ([][]float32, *domainEmbeddings.TokenUsage, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ErrTimeout
		}
		return nil, nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == 429 {
		return nil, nil, ErrRateLimited
	}

	if resp.StatusCode != 200 {
		var errResp openAIErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return nil, nil, fmt.Errorf("%w: %s", ErrRequestFailed, errResp.Error.Message)
		}
		return nil, nil, fmt.Errorf("%w: status %d", ErrRequestFailed, resp.StatusCode)
	}

	var apiResp openAIEmbeddingResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	if len(apiResp.Data) == 0 {
		return nil, nil, ErrInvalidResponse
	}

	// Extract embeddings and apply normalization
	embeddings := make([][]float32, len(apiResp.Data))
	for _, item := range apiResp.Data {
		embedding := item.Embedding
		if p.normalize != nil {
			embedding = p.normalize(embedding)
		}
		embeddings[item.Index] = embedding
	}

	usage := &domainEmbeddings.TokenUsage{
		PromptTokens: apiResp.Usage.PromptTokens,
		TotalTokens:  apiResp.Usage.TotalTokens,
	}

	return embeddings, usage, nil
}

// Dimensions returns the embedding dimensions.
func (p *OpenAIProvider) Dimensions() int {
	if p.config.Dimensions > 0 {
		return p.config.Dimensions
	}

	// Default dimensions per model
	switch p.config.Model {
	case domainEmbeddings.ModelTextEmbedding3Small:
		return 1536
	case domainEmbeddings.ModelTextEmbedding3Large:
		return 3072
	case domainEmbeddings.ModelTextEmbeddingAda002:
		return 1536
	default:
		return 1536
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() domainEmbeddings.ProviderType {
	return domainEmbeddings.ProviderOpenAI
}

// Close closes the provider.
func (p *OpenAIProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}
