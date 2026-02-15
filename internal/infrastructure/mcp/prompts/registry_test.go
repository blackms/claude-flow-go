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
