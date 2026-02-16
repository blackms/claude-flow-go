// Package sampling provides MCP sampling API implementation.
package sampling

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// LLMProvider represents an LLM provider that can create messages.
type LLMProvider interface {
	Name() string
	CreateMessage(ctx context.Context, request *shared.CreateMessageRequest) (*shared.CreateMessageResult, error)
	IsAvailable() bool
}

// SamplingManager manages LLM providers and sampling requests.
type SamplingManager struct {
	mu              sync.RWMutex
	providers       map[string]LLMProvider
	defaultProvider string
	config          shared.SamplingConfig
	stats           *shared.SamplingStats
}

func normalizeSamplingConfig(config shared.SamplingConfig) shared.SamplingConfig {
	defaults := shared.DefaultSamplingConfig()

	if config.DefaultMaxTokens <= 0 {
		config.DefaultMaxTokens = defaults.DefaultMaxTokens
	}
	if config.DefaultTemperature == 0 {
		config.DefaultTemperature = defaults.DefaultTemperature
	}
	if config.TimeoutMs <= 0 {
		config.TimeoutMs = defaults.TimeoutMs
	}

	return config
}

func cloneCreateMessageRequest(request *shared.CreateMessageRequest) *shared.CreateMessageRequest {
	if request == nil {
		return nil
	}

	cloned := *request

	if request.Messages != nil {
		cloned.Messages = make([]shared.SamplingMessage, len(request.Messages))
		for i := range request.Messages {
			cloned.Messages[i] = request.Messages[i]
			if request.Messages[i].Content != nil {
				cloned.Messages[i].Content = append([]shared.PromptContent(nil), request.Messages[i].Content...)
			}
		}
	}

	if request.ModelPreferences != nil {
		prefs := *request.ModelPreferences
		if request.ModelPreferences.Hints != nil {
			prefs.Hints = append([]shared.ModelHint(nil), request.ModelPreferences.Hints...)
		}
		cloned.ModelPreferences = &prefs
	}

	if request.StopSequences != nil {
		cloned.StopSequences = append([]string(nil), request.StopSequences...)
	}

	if request.Metadata != nil {
		cloned.Metadata = make(map[string]interface{}, len(request.Metadata))
		for key, value := range request.Metadata {
			cloned.Metadata[key] = value
		}
	}

	return &cloned
}

func safeProviderName(provider LLMProvider) (name string, ok bool) {
	if provider == nil {
		return "", false
	}

	defer func() {
		if recover() != nil {
			name = ""
			ok = false
		}
	}()

	name = strings.TrimSpace(provider.Name())
	if name == "" {
		return "", false
	}
	return name, true
}

func safeProviderIsAvailable(provider LLMProvider) (available bool) {
	if provider == nil {
		return false
	}

	defer func() {
		if recover() != nil {
			available = false
		}
	}()

	return provider.IsAvailable()
}

func safeProviderCreateMessage(provider LLMProvider, ctx context.Context, request *shared.CreateMessageRequest) (result *shared.CreateMessageResult, err error) {
	name, _ := safeProviderName(provider)
	if name == "" {
		name = "<unknown>"
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("provider %s panicked: %v", name, recovered)
			result = nil
		}
	}()

	return provider.CreateMessage(ctx, request)
}

// NewSamplingManager creates a new SamplingManager.
func NewSamplingManager(config shared.SamplingConfig) *SamplingManager {
	config = normalizeSamplingConfig(config)
	return &SamplingManager{
		providers: make(map[string]LLMProvider),
		config:    config,
		stats:     &shared.SamplingStats{},
	}
}

// NewSamplingManagerWithDefaults creates a SamplingManager with default configuration.
func NewSamplingManagerWithDefaults() *SamplingManager {
	return NewSamplingManager(shared.DefaultSamplingConfig())
}

// RegisterProvider registers an LLM provider.
func (sm *SamplingManager) RegisterProvider(provider LLMProvider, isDefault bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	name, ok := safeProviderName(provider)
	if !ok {
		return
	}

	sm.providers[name] = provider

	if isDefault || sm.defaultProvider == "" {
		sm.defaultProvider = name
	}
}

// UnregisterProvider removes an LLM provider.
func (sm *SamplingManager) UnregisterProvider(name string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	name = strings.TrimSpace(name)
	if _, exists := sm.providers[name]; !exists {
		return false
	}

	delete(sm.providers, name)

	if sm.defaultProvider == name {
		sm.defaultProvider = ""
		// Set new default if any provider exists
		for n := range sm.providers {
			sm.defaultProvider = n
			break
		}
	}

	return true
}

// CreateMessage creates a message using an LLM provider.
func (sm *SamplingManager) CreateMessage(request *shared.CreateMessageRequest) (*shared.CreateMessageResult, error) {
	return sm.CreateMessageWithContext(context.Background(), request)
}

// CreateMessageWithContext creates a message with context.
func (sm *SamplingManager) CreateMessageWithContext(ctx context.Context, request *shared.CreateMessageRequest) (*shared.CreateMessageResult, error) {
	startTime := time.Now()

	sm.mu.Lock()
	sm.stats.TotalRequests++
	sm.mu.Unlock()

	if request == nil {
		sm.mu.Lock()
		sm.stats.FailedRequests++
		sm.mu.Unlock()
		return nil, fmt.Errorf("sampling request is required")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	requestCopy := cloneCreateMessageRequest(request)

	// Apply defaults
	if requestCopy.MaxTokens <= 0 {
		requestCopy.MaxTokens = sm.config.DefaultMaxTokens
	}
	if requestCopy.Temperature == 0 {
		requestCopy.Temperature = sm.config.DefaultTemperature
	}

	// Select provider
	provider, err := sm.selectProvider(requestCopy.ModelPreferences)
	if err != nil {
		sm.mu.Lock()
		sm.stats.FailedRequests++
		sm.mu.Unlock()
		return nil, err
	}

	// Create timeout context
	timeout := time.Duration(sm.config.TimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute request
	result, err := safeProviderCreateMessage(provider, ctx, requestCopy)
	if err != nil {
		sm.mu.Lock()
		sm.stats.FailedRequests++
		sm.mu.Unlock()

		if ctx.Err() == context.DeadlineExceeded {
			return nil, shared.ErrSamplingTimeout
		}
		return nil, err
	}

	// Update stats
	latency := float64(time.Since(startTime).Milliseconds())
	sm.mu.Lock()
	sm.stats.SuccessfulRequests++
	n := float64(sm.stats.SuccessfulRequests)
	sm.stats.AvgLatencyMs = (sm.stats.AvgLatencyMs*(n-1) + latency) / n
	sm.mu.Unlock()

	return result, nil
}

// selectProvider selects the best provider based on preferences.
func (sm *SamplingManager) selectProvider(prefs *shared.ModelPreferences) (LLMProvider, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.providers) == 0 {
		return nil, shared.ErrNoProvidersAvailable
	}

	// Check if specific model hint is provided
	if prefs != nil && len(prefs.Hints) > 0 {
		for _, hint := range prefs.Hints {
			name := strings.TrimSpace(hint.Name)
			if provider, exists := sm.providers[name]; exists {
				if safeProviderIsAvailable(provider) {
					return provider, nil
				}
			}
		}
	}

	// Use default provider
	if sm.defaultProvider != "" {
		if provider, exists := sm.providers[sm.defaultProvider]; exists {
			if safeProviderIsAvailable(provider) {
				return provider, nil
			}
		}
	}

	// Find any available provider
	names := make([]string, 0, len(sm.providers))
	for name := range sm.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		provider := sm.providers[name]
		if safeProviderIsAvailable(provider) {
			return provider, nil
		}
	}

	return nil, shared.ErrNoProvidersAvailable
}

// GetProvider returns a provider by name.
func (sm *SamplingManager) GetProvider(name string) LLMProvider {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.providers[strings.TrimSpace(name)]
}

// GetProviders returns all registered providers.
func (sm *SamplingManager) GetProviders() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	names := make([]string, 0, len(sm.providers))
	for name := range sm.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsAvailable checks if any provider is available.
func (sm *SamplingManager) IsAvailable() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, provider := range sm.providers {
		if safeProviderIsAvailable(provider) {
			return true
		}
	}
	return false
}

// GetStats returns sampling statistics.
func (sm *SamplingManager) GetStats() shared.SamplingStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return *sm.stats
}

// GetConfig returns the sampling configuration.
func (sm *SamplingManager) GetConfig() shared.SamplingConfig {
	return sm.config
}

// GetDefaultProvider returns the name of the default provider.
func (sm *SamplingManager) GetDefaultProvider() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.defaultProvider
}

// SetDefaultProvider sets the default provider.
func (sm *SamplingManager) SetDefaultProvider(name string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	name = strings.TrimSpace(name)
	if _, exists := sm.providers[name]; !exists {
		return false
	}
	sm.defaultProvider = name
	return true
}

// ProviderCount returns the number of registered providers.
func (sm *SamplingManager) ProviderCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.providers)
}

// ResetStats resets the statistics.
func (sm *SamplingManager) ResetStats() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.stats = &shared.SamplingStats{}
}
