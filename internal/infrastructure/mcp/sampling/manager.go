// Package sampling provides MCP sampling API implementation.
package sampling

import (
	"context"
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

// NewSamplingManager creates a new SamplingManager.
func NewSamplingManager(config shared.SamplingConfig) *SamplingManager {
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

	name := provider.Name()
	sm.providers[name] = provider

	if isDefault || sm.defaultProvider == "" {
		sm.defaultProvider = name
	}
}

// UnregisterProvider removes an LLM provider.
func (sm *SamplingManager) UnregisterProvider(name string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

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

	// Apply defaults
	if request.MaxTokens <= 0 {
		request.MaxTokens = sm.config.DefaultMaxTokens
	}
	if request.Temperature == 0 {
		request.Temperature = sm.config.DefaultTemperature
	}

	// Select provider
	provider, err := sm.selectProvider(request.ModelPreferences)
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
	result, err := provider.CreateMessage(ctx, request)
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
			if provider, exists := sm.providers[hint.Name]; exists {
				if provider.IsAvailable() {
					return provider, nil
				}
			}
		}
	}

	// Use default provider
	if sm.defaultProvider != "" {
		if provider, exists := sm.providers[sm.defaultProvider]; exists {
			if provider.IsAvailable() {
				return provider, nil
			}
		}
	}

	// Find any available provider
	for _, provider := range sm.providers {
		if provider.IsAvailable() {
			return provider, nil
		}
	}

	return nil, shared.ErrNoProvidersAvailable
}

// GetProvider returns a provider by name.
func (sm *SamplingManager) GetProvider(name string) LLMProvider {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.providers[name]
}

// GetProviders returns all registered providers.
func (sm *SamplingManager) GetProviders() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	names := make([]string, 0, len(sm.providers))
	for name := range sm.providers {
		names = append(names, name)
	}
	return names
}

// IsAvailable checks if any provider is available.
func (sm *SamplingManager) IsAvailable() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, provider := range sm.providers {
		if provider.IsAvailable() {
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
