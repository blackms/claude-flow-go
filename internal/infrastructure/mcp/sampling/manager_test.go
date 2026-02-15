package sampling

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

type testProvider struct {
	name             string
	available        bool
	panicOnName      bool
	panicOnAvailable bool
	panicOnCreate    bool
	createFn         func(ctx context.Context, request *shared.CreateMessageRequest) (*shared.CreateMessageResult, error)
}

func (tp *testProvider) Name() string {
	if tp.panicOnName {
		panic("name panic")
	}
	return tp.name
}

func (tp *testProvider) CreateMessage(ctx context.Context, request *shared.CreateMessageRequest) (*shared.CreateMessageResult, error) {
	if tp.panicOnCreate {
		panic("create panic")
	}
	if tp.createFn != nil {
		return tp.createFn(ctx, request)
	}
	return &shared.CreateMessageResult{
		Role:       "assistant",
		Content:    "ok",
		Model:      tp.name,
		StopReason: "end_turn",
	}, nil
}

func (tp *testProvider) IsAvailable() bool {
	if tp.panicOnAvailable {
		panic("available panic")
	}
	return tp.available
}

func TestSamplingManager_NormalizesConfigDefaults(t *testing.T) {
	manager := NewSamplingManager(shared.SamplingConfig{})
	config := manager.GetConfig()
	defaults := shared.DefaultSamplingConfig()

	if config.DefaultMaxTokens != defaults.DefaultMaxTokens {
		t.Fatalf("expected default max tokens %d, got %d", defaults.DefaultMaxTokens, config.DefaultMaxTokens)
	}
	if config.DefaultTemperature != defaults.DefaultTemperature {
		t.Fatalf("expected default temperature %.2f, got %.2f", defaults.DefaultTemperature, config.DefaultTemperature)
	}
	if config.TimeoutMs != defaults.TimeoutMs {
		t.Fatalf("expected default timeout %d, got %d", defaults.TimeoutMs, config.TimeoutMs)
	}
}

func TestSamplingManager_RegisterProviderIgnoresNilAndInvalidNames(t *testing.T) {
	manager := NewSamplingManagerWithDefaults()

	manager.RegisterProvider(nil, true)
	manager.RegisterProvider(&testProvider{name: "   ", available: true}, true)
	manager.RegisterProvider(&testProvider{panicOnName: true, available: true}, true)

	if count := manager.ProviderCount(); count != 0 {
		t.Fatalf("expected invalid providers to be ignored, got count %d", count)
	}
}

func TestSamplingManager_GetProvidersReturnsSortedNames(t *testing.T) {
	manager := NewSamplingManagerWithDefaults()
	manager.RegisterProvider(&testProvider{name: "charlie", available: true}, false)
	manager.RegisterProvider(&testProvider{name: "alpha", available: true}, false)
	manager.RegisterProvider(&testProvider{name: "bravo", available: true}, false)

	providers := manager.GetProviders()
	expected := []string{"alpha", "bravo", "charlie"}
	if len(providers) != len(expected) {
		t.Fatalf("expected %d providers, got %d", len(expected), len(providers))
	}
	for i, name := range expected {
		if providers[i] != name {
			t.Fatalf("expected providers[%d] == %q, got %q", i, name, providers[i])
		}
	}
}

func TestSamplingManager_CreateMessageWithContextDoesNotMutateInputRequest(t *testing.T) {
	manager := NewSamplingManagerWithDefaults()

	var captured *shared.CreateMessageRequest
	manager.RegisterProvider(&testProvider{
		name:      "primary",
		available: true,
		createFn: func(ctx context.Context, request *shared.CreateMessageRequest) (*shared.CreateMessageResult, error) {
			captured = request
			return &shared.CreateMessageResult{
				Role:       "assistant",
				Content:    "ok",
				Model:      "primary",
				StopReason: "end_turn",
			}, nil
		},
	}, true)

	request := &shared.CreateMessageRequest{
		Messages: []shared.SamplingMessage{
			{
				Role: "user",
				Content: []shared.PromptContent{
					{Type: shared.PromptContentTypeText, Text: "hello"},
				},
			},
		},
		MaxTokens:   0,
		Temperature: 0,
		Metadata: map[string]interface{}{
			"traceId": "abc",
		},
	}

	_, err := manager.CreateMessageWithContext(context.Background(), request)
	if err != nil {
		t.Fatalf("expected CreateMessageWithContext success, got %v", err)
	}
	if request.MaxTokens != 0 {
		t.Fatalf("expected original MaxTokens to remain unchanged, got %d", request.MaxTokens)
	}
	if request.Temperature != 0 {
		t.Fatalf("expected original Temperature to remain unchanged, got %f", request.Temperature)
	}
	if captured == nil {
		t.Fatal("expected provider request capture")
	}
	defaults := shared.DefaultSamplingConfig()
	if captured.MaxTokens != defaults.DefaultMaxTokens {
		t.Fatalf("expected provider to receive default max tokens %d, got %d", defaults.DefaultMaxTokens, captured.MaxTokens)
	}
	if captured.Temperature != defaults.DefaultTemperature {
		t.Fatalf("expected provider to receive default temperature %.2f, got %.2f", defaults.DefaultTemperature, captured.Temperature)
	}
}

func TestSamplingManager_CreateMessageWithContextNilRequestReturnsError(t *testing.T) {
	manager := NewSamplingManagerWithDefaults()
	_, err := manager.CreateMessageWithContext(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil sampling request")
	}
	if err.Error() != "sampling request is required" {
		t.Fatalf("expected nil request error message, got %q", err.Error())
	}

	stats := manager.GetStats()
	if stats.TotalRequests != 1 {
		t.Fatalf("expected total requests 1, got %d", stats.TotalRequests)
	}
	if stats.FailedRequests != 1 {
		t.Fatalf("expected failed requests 1, got %d", stats.FailedRequests)
	}
}

func TestSamplingManager_SelectProviderSkipsPanickingAvailability(t *testing.T) {
	manager := NewSamplingManagerWithDefaults()

	manager.RegisterProvider(&testProvider{
		name:             "panic-default",
		available:        true,
		panicOnAvailable: true,
	}, true)
	manager.RegisterProvider(&testProvider{
		name:      "healthy",
		available: true,
	}, false)

	result, err := manager.CreateMessageWithContext(context.Background(), &shared.CreateMessageRequest{})
	if err != nil {
		t.Fatalf("expected CreateMessageWithContext success, got %v", err)
	}
	if result.Model != "healthy" {
		t.Fatalf("expected fallback to healthy provider, got model %q", result.Model)
	}
}

func TestSamplingManager_CreateMessageWithContextRecoversProviderPanics(t *testing.T) {
	manager := NewSamplingManagerWithDefaults()
	manager.RegisterProvider(&testProvider{
		name:          "panic-create",
		available:     true,
		panicOnCreate: true,
	}, true)

	_, err := manager.CreateMessageWithContext(context.Background(), &shared.CreateMessageRequest{})
	if err == nil {
		t.Fatal("expected provider panic to be converted into error")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("expected panic-derived error message, got %q", err.Error())
	}

	stats := manager.GetStats()
	if stats.FailedRequests != 1 {
		t.Fatalf("expected failed requests 1 after panic recovery, got %d", stats.FailedRequests)
	}
}
