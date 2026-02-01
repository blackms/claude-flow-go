// Package sampling provides MCP sampling API implementation.
package sampling

import (
	"context"
	"strings"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// MockProvider is a mock LLM provider for testing.
type MockProvider struct {
	name      string
	available bool
	responses map[string]string // Map of prompt prefixes to responses
}

// NewMockProvider creates a new mock provider.
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:      name,
		available: true,
		responses: make(map[string]string),
	}
}

// NewMockProviderWithDefaults creates a mock provider with default responses.
func NewMockProviderWithDefaults() *MockProvider {
	mp := NewMockProvider("mock")
	mp.SetResponse("hello", "Hello! How can I help you today?")
	mp.SetResponse("help", "I'm here to assist you with any questions or tasks.")
	mp.SetResponse("", "I understand. Let me help you with that.")
	return mp
}

// Name returns the provider name.
func (mp *MockProvider) Name() string {
	return mp.name
}

// CreateMessage creates a mock response.
func (mp *MockProvider) CreateMessage(ctx context.Context, request *shared.CreateMessageRequest) (*shared.CreateMessageResult, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Find matching response
	response := "This is a mock response."
	if len(request.Messages) > 0 {
		lastMsg := request.Messages[len(request.Messages)-1]
		for _, content := range lastMsg.Content {
			if content.Type == shared.PromptContentTypeText {
				text := strings.ToLower(content.Text)
				for prefix, resp := range mp.responses {
					if prefix == "" || strings.HasPrefix(text, prefix) {
						response = resp
						break
					}
				}
			}
		}
	}

	return &shared.CreateMessageResult{
		Role:       "assistant",
		Content:    response,
		Model:      mp.name,
		StopReason: "end_turn",
	}, nil
}

// IsAvailable returns whether the provider is available.
func (mp *MockProvider) IsAvailable() bool {
	return mp.available
}

// SetAvailable sets the availability of the provider.
func (mp *MockProvider) SetAvailable(available bool) {
	mp.available = available
}

// SetResponse sets a response for a prompt prefix.
func (mp *MockProvider) SetResponse(prefix, response string) {
	mp.responses[prefix] = response
}

// ClearResponses clears all custom responses.
func (mp *MockProvider) ClearResponses() {
	mp.responses = make(map[string]string)
}
