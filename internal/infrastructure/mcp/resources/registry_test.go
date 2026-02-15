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

func TestResourceRegistry_ListReturnsDefensiveCopies(t *testing.T) {
	registry := NewResourceRegistryWithDefaults()

	input := &shared.MCPResource{
		URI:  "resource://defensive-copy",
		Name: "defensive-copy",
		Annotations: map[string]interface{}{
			"owner": "alpha",
		},
	}
	err := registry.RegisterResource(input, func(uri string) (*shared.ResourceContent, error) {
		return &shared.ResourceContent{
			URI:  uri,
			Text: "ok",
		}, nil
	})
	if err != nil {
		t.Fatalf("failed to register test resource: %v", err)
	}

	// Mutate caller-owned input after registration; registry should be isolated.
	input.Name = "mutated"
	input.Annotations["owner"] = "mutated"

	find := func(resources []shared.MCPResource, uri string) *shared.MCPResource {
		for i := range resources {
			if resources[i].URI == uri {
				return &resources[i]
			}
		}
		return nil
	}

	first := registry.List("", 10)
	firstResource := find(first.Resources, "resource://defensive-copy")
	if firstResource == nil {
		t.Fatal("expected registered resource in first list result")
	}
	if firstResource.Name != "defensive-copy" {
		t.Fatalf("expected stored resource name to remain unchanged, got %q", firstResource.Name)
	}
	if got := firstResource.Annotations["owner"]; got != "alpha" {
		t.Fatalf("expected stored annotations owner to remain unchanged, got %v", got)
	}

	// Mutate returned resource map and verify second read is unaffected.
	firstResource.Annotations["owner"] = "list-mutated"
	firstResource.Annotations["new"] = true

	second := registry.List("", 10)
	secondResource := find(second.Resources, "resource://defensive-copy")
	if secondResource == nil {
		t.Fatal("expected registered resource in second list result")
	}
	if got := secondResource.Annotations["owner"]; got != "alpha" {
		t.Fatalf("expected defensive copy for annotations owner, got %v", got)
	}
	if _, ok := secondResource.Annotations["new"]; ok {
		t.Fatalf("expected defensive copy to exclude injected annotation key, got %v", secondResource.Annotations["new"])
	}
}

func TestResourceRegistry_GetResourceReturnsDefensiveCopy(t *testing.T) {
	registry := NewResourceRegistryWithDefaults()

	err := registry.RegisterResource(&shared.MCPResource{
		URI:  "resource://get-copy",
		Name: "get-copy",
		Annotations: map[string]interface{}{
			"owner": "alpha",
		},
	}, func(uri string) (*shared.ResourceContent, error) {
		return &shared.ResourceContent{URI: uri, Text: "ok"}, nil
	})
	if err != nil {
		t.Fatalf("failed to register resource: %v", err)
	}

	first := registry.GetResource("resource://get-copy")
	if first == nil {
		t.Fatal("expected first GetResource result")
	}
	first.Name = "mutated"
	first.Annotations["owner"] = "mutated"

	second := registry.GetResource("resource://get-copy")
	if second == nil {
		t.Fatal("expected second GetResource result")
	}
	if second.Name != "get-copy" {
		t.Fatalf("expected defensive copy for resource name, got %q", second.Name)
	}
	if got := second.Annotations["owner"]; got != "alpha" {
		t.Fatalf("expected defensive copy for annotations owner, got %v", got)
	}
}

func TestResourceRegistry_ReadReturnsDefensiveBlobCopy(t *testing.T) {
	registry := NewResourceRegistryWithDefaults()

	err := registry.RegisterResource(&shared.MCPResource{
		URI:  "resource://blob-copy",
		Name: "blob-copy",
	}, func(uri string) (*shared.ResourceContent, error) {
		return &shared.ResourceContent{
			URI:  uri,
			Blob: []byte("abc"),
		}, nil
	})
	if err != nil {
		t.Fatalf("failed to register resource: %v", err)
	}

	first, err := registry.Read("resource://blob-copy")
	if err != nil {
		t.Fatalf("failed to read first resource snapshot: %v", err)
	}
	if len(first.Contents) != 1 {
		t.Fatalf("expected one content in first read, got %d", len(first.Contents))
	}
	first.Contents[0].Blob[0] = 'z'

	second, err := registry.Read("resource://blob-copy")
	if err != nil {
		t.Fatalf("failed to read second resource snapshot: %v", err)
	}
	if len(second.Contents) != 1 {
		t.Fatalf("expected one content in second read, got %d", len(second.Contents))
	}
	if string(second.Contents[0].Blob) != "abc" {
		t.Fatalf("expected defensive copy for blob contents, got %q", string(second.Contents[0].Blob))
	}
}

func TestResourceRegistry_RegisterTemplateClonesTemplateInput(t *testing.T) {
	registry := NewResourceRegistryWithDefaults()

	template := &shared.ResourceTemplate{
		URITemplate: "resource://template/{id}",
		Name:        "template-original",
	}
	err := registry.RegisterTemplate(template, func(uri string) (*shared.ResourceContent, error) {
		return &shared.ResourceContent{
			URI:  uri,
			Text: "ok",
		}, nil
	})
	if err != nil {
		t.Fatalf("failed to register template: %v", err)
	}

	template.Name = "template-mutated"

	templates := registry.GetTemplates()
	if len(templates) != 1 {
		t.Fatalf("expected one registered template, got %d", len(templates))
	}
	if templates[0].Name != "template-original" {
		t.Fatalf("expected template name to remain unchanged after caller mutation, got %q", templates[0].Name)
	}
}
