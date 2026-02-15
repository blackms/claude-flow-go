package resources

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestResourceRegistry_ListCursorBeyondLastReturnsEmptyPage(t *testing.T) {
	registry := NewResourceRegistryWithDefaults()

	register := func(uri string) {
		err := registry.RegisterResource(&shared.MCPResource{
			URI:  uri,
			Name: uri,
		}, func(uri string) (*shared.ResourceContent, error) {
			return &shared.ResourceContent{
				URI:  uri,
				Text: "ok",
			}, nil
		})
		if err != nil {
			t.Fatalf("failed to register resource %s: %v", uri, err)
		}
	}
	register("resource://a")
	register("resource://b")

	result := registry.List("resource://z", 10)
	if len(result.Resources) != 0 {
		t.Fatalf("expected empty page for cursor beyond last item, got %d items", len(result.Resources))
	}
	if result.NextCursor != "" {
		t.Fatalf("expected empty next cursor for terminal page, got %q", result.NextCursor)
	}
}
