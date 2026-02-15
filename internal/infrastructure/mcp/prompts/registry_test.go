package prompts

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestPromptRegistry_ListCursorBeyondLastReturnsEmptyPage(t *testing.T) {
	registry := NewPromptRegistryWithDefaults()

	register := func(name string) {
		err := registry.Register(&shared.MCPPrompt{
			Name: name,
		}, func(args map[string]string) ([]shared.PromptMessage, error) {
			return []shared.PromptMessage{
				{
					Role: "user",
					Content: []shared.PromptContent{
						{
							Type: shared.PromptContentTypeText,
							Text: "ok",
						},
					},
				},
			}, nil
		})
		if err != nil {
			t.Fatalf("failed to register prompt %s: %v", name, err)
		}
	}
	register("prompt-a")
	register("prompt-b")

	result := registry.List("prompt-z", 10)
	if len(result.Prompts) != 0 {
		t.Fatalf("expected empty page for cursor beyond last prompt, got %d prompts", len(result.Prompts))
	}
	if result.NextCursor != "" {
		t.Fatalf("expected empty next cursor for terminal page, got %q", result.NextCursor)
	}
}

func TestPromptRegistry_ListReturnsDefensiveCopies(t *testing.T) {
	registry := NewPromptRegistryWithDefaults()

	input := &shared.MCPPrompt{
		Name: "prompt-defensive-copy",
		Arguments: []shared.PromptArgument{
			{Name: "arg-one", Required: true},
		},
	}
	err := registry.Register(input, func(args map[string]string) ([]shared.PromptMessage, error) {
		return []shared.PromptMessage{
			{
				Role: "user",
				Content: []shared.PromptContent{
					{
						Type: shared.PromptContentTypeText,
						Text: "ok",
					},
				},
			},
		}, nil
	})
	if err != nil {
		t.Fatalf("failed to register prompt: %v", err)
	}

	// Mutate caller-owned input after registration; registry should be isolated.
	input.Arguments[0].Name = "mutated"

	find := func(prompts []shared.MCPPrompt, name string) *shared.MCPPrompt {
		for i := range prompts {
			if prompts[i].Name == name {
				return &prompts[i]
			}
		}
		return nil
	}

	first := registry.List("", 10)
	firstPrompt := find(first.Prompts, "prompt-defensive-copy")
	if firstPrompt == nil {
		t.Fatal("expected prompt in first list response")
	}
	if len(firstPrompt.Arguments) != 1 || firstPrompt.Arguments[0].Name != "arg-one" {
		t.Fatalf("expected stored prompt arguments unchanged, got %+v", firstPrompt.Arguments)
	}

	// Mutate returned prompt arguments and verify second read remains unchanged.
	firstPrompt.Arguments[0].Name = "list-mutated"

	second := registry.List("", 10)
	secondPrompt := find(second.Prompts, "prompt-defensive-copy")
	if secondPrompt == nil {
		t.Fatal("expected prompt in second list response")
	}
	if len(secondPrompt.Arguments) != 1 || secondPrompt.Arguments[0].Name != "arg-one" {
		t.Fatalf("expected defensive copy for prompt arguments, got %+v", secondPrompt.Arguments)
	}
}

func TestPromptRegistry_GetPromptReturnsDefensiveCopy(t *testing.T) {
	registry := NewPromptRegistryWithDefaults()

	err := registry.Register(&shared.MCPPrompt{
		Name: "prompt-get-copy",
		Arguments: []shared.PromptArgument{
			{Name: "arg-one", Required: true},
		},
	}, func(args map[string]string) ([]shared.PromptMessage, error) {
		return []shared.PromptMessage{
			{
				Role: "user",
				Content: []shared.PromptContent{
					{
						Type: shared.PromptContentTypeText,
						Text: "ok",
					},
				},
			},
		}, nil
	})
	if err != nil {
		t.Fatalf("failed to register prompt: %v", err)
	}

	first := registry.GetPrompt("prompt-get-copy")
	if first == nil {
		t.Fatal("expected first GetPrompt result")
	}
	first.Arguments[0].Name = "mutated"

	second := registry.GetPrompt("prompt-get-copy")
	if second == nil {
		t.Fatal("expected second GetPrompt result")
	}
	if len(second.Arguments) != 1 || second.Arguments[0].Name != "arg-one" {
		t.Fatalf("expected defensive copy for GetPrompt arguments, got %+v", second.Arguments)
	}
}
