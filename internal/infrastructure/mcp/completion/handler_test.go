package completion

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/prompts"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/resources"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestCompletionHandler_CompleteHandlesNilReceiverAndInputs(t *testing.T) {
	var handler *CompletionHandler
	result := handler.Complete(nil, nil)
	if result == nil {
		t.Fatal("expected non-nil result for nil receiver/input")
	}
	if len(result.Values) != 0 {
		t.Fatalf("expected empty completion values, got %d", len(result.Values))
	}
}

func TestCompletionHandler_CompletePromptWithNilArgumentAndTrimmedName(t *testing.T) {
	promptRegistry := prompts.NewPromptRegistryWithDefaults()
	err := promptRegistry.Register(&shared.MCPPrompt{
		Name: "deploy",
		Arguments: []shared.PromptArgument{
			{Name: "environment"},
			{Name: "region"},
			{Name: "version"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("failed to register prompt: %v", err)
	}

	handler := NewCompletionHandler(nil, promptRegistry)
	result := handler.Complete(&shared.CompletionReference{
		Type: shared.CompletionRefPrompt,
		Name: " deploy ",
	}, nil)
	if result == nil {
		t.Fatal("expected non-nil completion result")
	}

	expected := []string{"environment", "region", "version"}
	if len(result.Values) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(result.Values))
	}
	for i, value := range expected {
		if result.Values[i] != value {
			t.Fatalf("expected values[%d] = %q, got %q", i, value, result.Values[i])
		}
	}
}

func TestCompletionHandler_CompleteResourceReturnsSortedURIs(t *testing.T) {
	resourceRegistry := resources.NewResourceRegistryWithDefaults()
	register := func(uri string) {
		err := resourceRegistry.RegisterResource(&shared.MCPResource{
			URI:  uri,
			Name: uri,
		}, resources.CreateTextResource("ok", "text/plain"))
		if err != nil {
			t.Fatalf("failed to register resource %q: %v", uri, err)
		}
	}
	register("resource://zeta")
	register("resource://alpha")
	register("resource://beta")

	handler := NewCompletionHandler(resourceRegistry, nil)
	result := handler.Complete(&shared.CompletionReference{
		Type: shared.CompletionRefResource,
		URI:  " resource:// ",
	}, nil)
	if result == nil {
		t.Fatal("expected non-nil completion result")
	}

	expected := []string{"resource://alpha", "resource://beta", "resource://zeta"}
	if len(result.Values) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(result.Values))
	}
	for i, value := range expected {
		if result.Values[i] != value {
			t.Fatalf("expected values[%d] = %q, got %q", i, value, result.Values[i])
		}
	}
}

func TestCompletionHandler_BuildResultReturnsNonNilEmptyValues(t *testing.T) {
	result := buildResult(nil)
	if result == nil {
		t.Fatal("expected non-nil completion result")
	}
	if result.Values == nil {
		t.Fatal("expected non-nil values slice for empty result")
	}
	if len(result.Values) != 0 {
		t.Fatalf("expected empty values slice, got %d", len(result.Values))
	}
}
