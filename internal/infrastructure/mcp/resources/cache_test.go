package resources

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestResourceCache_SetGetUsesDefensiveCopies(t *testing.T) {
	cache := NewResourceCacheWithDefaults()

	original := &shared.ResourceContent{
		URI:  "resource://cache-copy",
		Text: "ok",
		Blob: []byte("abc"),
	}
	cache.Set("resource://cache-copy", original)

	// Mutate caller-owned original after Set; cache value should remain unchanged.
	original.Blob[0] = 'z'
	original.Text = "mutated"

	first, found := cache.Get("resource://cache-copy")
	if !found || first == nil {
		t.Fatal("expected cached resource")
	}
	if got := string(first.Blob); got != "abc" {
		t.Fatalf("expected cached blob to remain unchanged after caller mutation, got %q", got)
	}
	if first.Text != "ok" {
		t.Fatalf("expected cached text to remain unchanged after caller mutation, got %q", first.Text)
	}

	// Mutate returned snapshot; subsequent reads should be unaffected.
	first.Blob[1] = 'y'
	first.Text = "returned-mutation"

	second, found := cache.Get("resource://cache-copy")
	if !found || second == nil {
		t.Fatal("expected cached resource on second read")
	}
	if got := string(second.Blob); got != "abc" {
		t.Fatalf("expected defensive copy on cache Get, got %q", got)
	}
	if second.Text != "ok" {
		t.Fatalf("expected defensive copy for text field on cache Get, got %q", second.Text)
	}
}
