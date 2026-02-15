// Package resources provides MCP resource registry implementation.
package resources

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ResourceHandler is a function that reads a resource.
type ResourceHandler func(uri string) (*shared.ResourceContent, error)

// Subscription represents a resource subscription.
type Subscription struct {
	ID       string
	URI      string
	Callback func(uri string, content *shared.ResourceContent)
}

// ResourceRegistry manages MCP resources.
type ResourceRegistry struct {
	mu            sync.RWMutex
	resources     map[string]*shared.MCPResource
	templates     map[string]*TemplateEntry
	handlers      map[string]ResourceHandler
	subscriptions map[string][]*Subscription
	subByID       map[string]*Subscription
	cache         *ResourceCache
	nextSubID     int64
}

// TemplateEntry holds a template with its compiled regex.
type TemplateEntry struct {
	Template *shared.ResourceTemplate
	Pattern  *regexp.Regexp
	Handler  ResourceHandler
}

// NewResourceRegistry creates a new ResourceRegistry.
func NewResourceRegistry(cacheConfig shared.ResourceCacheConfig) *ResourceRegistry {
	return &ResourceRegistry{
		resources:     make(map[string]*shared.MCPResource),
		templates:     make(map[string]*TemplateEntry),
		handlers:      make(map[string]ResourceHandler),
		subscriptions: make(map[string][]*Subscription),
		subByID:       make(map[string]*Subscription),
		cache:         NewResourceCache(cacheConfig),
	}
}

// NewResourceRegistryWithDefaults creates a ResourceRegistry with default configuration.
func NewResourceRegistryWithDefaults() *ResourceRegistry {
	return NewResourceRegistry(shared.DefaultResourceCacheConfig())
}

// RegisterResource registers a static resource with its handler.
func (rr *ResourceRegistry) RegisterResource(resource *shared.MCPResource, handler ResourceHandler) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	rr.resources[resource.URI] = resource
	rr.handlers[resource.URI] = handler

	return nil
}

// RegisterTemplate registers a resource template with its handler.
func (rr *ResourceRegistry) RegisterTemplate(template *shared.ResourceTemplate, handler ResourceHandler) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	// Convert URI template to regex pattern
	// Simple conversion: {name} -> ([^/]+)
	pattern := rr.templateToRegex(template.URITemplate)

	regex, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		return err
	}

	rr.templates[template.URITemplate] = &TemplateEntry{
		Template: template,
		Pattern:  regex,
		Handler:  handler,
	}

	return nil
}

// templateToRegex converts a URI template to a regex pattern.
func (rr *ResourceRegistry) templateToRegex(template string) string {
	// Escape special regex characters first
	pattern := regexp.QuoteMeta(template)
	// Replace escaped {name} patterns with capturing groups
	re := regexp.MustCompile(`\\\{[^}]+\\\}`)
	pattern = re.ReplaceAllString(pattern, `([^/]+)`)
	return pattern
}

// UnregisterResource removes a resource.
func (rr *ResourceRegistry) UnregisterResource(uri string) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	delete(rr.resources, uri)
	delete(rr.handlers, uri)
	rr.cache.Invalidate(uri)
}

// UnregisterTemplate removes a template.
func (rr *ResourceRegistry) UnregisterTemplate(uriTemplate string) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	delete(rr.templates, uriTemplate)
}

// List returns a paginated list of resources.
func (rr *ResourceRegistry) List(cursor string, pageSize int) *shared.ResourceListResult {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if pageSize <= 0 {
		pageSize = 100
	}

	// Collect all resources
	allResources := make([]*shared.MCPResource, 0, len(rr.resources))
	for _, res := range rr.resources {
		allResources = append(allResources, res)
	}

	// Sort by URI for consistent pagination
	sort.Slice(allResources, func(i, j int) bool {
		return allResources[i].URI < allResources[j].URI
	})

	// Find starting point
	startIdx := 0
	if cursor != "" {
		startIdx = len(allResources)
		for i, res := range allResources {
			if res.URI > cursor {
				startIdx = i
				break
			}
		}
	}

	// Get page
	endIdx := startIdx + pageSize
	if endIdx > len(allResources) {
		endIdx = len(allResources)
	}

	page := allResources[startIdx:endIdx]
	resources := make([]shared.MCPResource, len(page))
	for i, res := range page {
		resources[i] = *res
	}

	result := &shared.ResourceListResult{
		Resources: resources,
	}

	if endIdx < len(allResources) {
		result.NextCursor = page[len(page)-1].URI
	}

	return result
}

// Read reads a resource by URI.
func (rr *ResourceRegistry) Read(uri string) (*shared.ResourceReadResult, error) {
	// Check cache first
	if content, found := rr.cache.Get(uri); found {
		return &shared.ResourceReadResult{
			Contents: []shared.ResourceContent{*content},
		}, nil
	}

	rr.mu.RLock()
	handler, exists := rr.handlers[uri]
	if !exists {
		// Try template matching
		handler = rr.findTemplateHandler(uri)
	}
	rr.mu.RUnlock()

	if handler == nil {
		return nil, shared.ErrResourceNotFound
	}

	content, err := handler(uri)
	if err != nil {
		return nil, err
	}

	// Cache the result
	rr.cache.Set(uri, content)

	return &shared.ResourceReadResult{
		Contents: []shared.ResourceContent{*content},
	}, nil
}

// findTemplateHandler finds a handler that matches the URI via template.
func (rr *ResourceRegistry) findTemplateHandler(uri string) ResourceHandler {
	for _, entry := range rr.templates {
		if entry.Pattern.MatchString(uri) {
			return entry.Handler
		}
	}
	return nil
}

// Subscribe subscribes to updates for a resource URI.
func (rr *ResourceRegistry) Subscribe(uri string, callback func(string, *shared.ResourceContent)) string {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	rr.nextSubID++
	subID := shared.GenerateID("sub")

	sub := &Subscription{
		ID:       subID,
		URI:      uri,
		Callback: callback,
	}

	rr.subscriptions[uri] = append(rr.subscriptions[uri], sub)
	rr.subByID[subID] = sub

	return subID
}

// Unsubscribe removes a subscription.
func (rr *ResourceRegistry) Unsubscribe(subscriptionID string) bool {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	sub, exists := rr.subByID[subscriptionID]
	if !exists {
		return false
	}

	// Remove from URI subscriptions
	subs := rr.subscriptions[sub.URI]
	for i, s := range subs {
		if s.ID == subscriptionID {
			rr.subscriptions[sub.URI] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	delete(rr.subByID, subscriptionID)
	return true
}

// NotifyUpdate notifies subscribers of a resource update.
func (rr *ResourceRegistry) NotifyUpdate(uri string) {
	rr.mu.RLock()
	subs := make([]*Subscription, len(rr.subscriptions[uri]))
	copy(subs, rr.subscriptions[uri])
	rr.mu.RUnlock()

	// Invalidate cache
	rr.cache.Invalidate(uri)

	// Read the updated content
	result, err := rr.Read(uri)
	if err != nil {
		return
	}

	var content *shared.ResourceContent
	if len(result.Contents) > 0 {
		content = &result.Contents[0]
	}

	// Notify subscribers
	for _, sub := range subs {
		go sub.Callback(uri, content)
	}
}

// GetResource returns a resource by URI.
func (rr *ResourceRegistry) GetResource(uri string) *shared.MCPResource {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return rr.resources[uri]
}

// HasResource checks if a resource exists.
func (rr *ResourceRegistry) HasResource(uri string) bool {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if _, exists := rr.resources[uri]; exists {
		return true
	}

	// Check templates
	for _, entry := range rr.templates {
		if entry.Pattern.MatchString(uri) {
			return true
		}
	}

	return false
}

// GetTemplates returns all registered templates.
func (rr *ResourceRegistry) GetTemplates() []shared.ResourceTemplate {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	templates := make([]shared.ResourceTemplate, 0, len(rr.templates))
	for _, entry := range rr.templates {
		templates = append(templates, *entry.Template)
	}
	return templates
}

// GetSubscriptionCount returns the number of subscriptions for a URI.
func (rr *ResourceRegistry) GetSubscriptionCount(uri string) int {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return len(rr.subscriptions[uri])
}

// GetCache returns the resource cache.
func (rr *ResourceRegistry) GetCache() *ResourceCache {
	return rr.cache
}

// Count returns the number of registered resources.
func (rr *ResourceRegistry) Count() int {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return len(rr.resources)
}

// TemplateCount returns the number of registered templates.
func (rr *ResourceRegistry) TemplateCount() int {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return len(rr.templates)
}

// ListResourcesByPrefix returns resources matching a URI prefix.
func (rr *ResourceRegistry) ListResourcesByPrefix(prefix string) []shared.MCPResource {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	result := make([]shared.MCPResource, 0)
	for _, res := range rr.resources {
		if strings.HasPrefix(res.URI, prefix) {
			result = append(result, *res)
		}
	}
	return result
}

// Helper functions for creating resources

// CreateTextResource creates a text resource handler.
func CreateTextResource(text, mimeType string) ResourceHandler {
	return func(uri string) (*shared.ResourceContent, error) {
		return &shared.ResourceContent{
			URI:      uri,
			MimeType: mimeType,
			Text:     text,
		}, nil
	}
}

// CreateDynamicResource creates a dynamic resource handler.
func CreateDynamicResource(fn func(uri string) (string, error), mimeType string) ResourceHandler {
	return func(uri string) (*shared.ResourceContent, error) {
		text, err := fn(uri)
		if err != nil {
			return nil, err
		}
		return &shared.ResourceContent{
			URI:      uri,
			MimeType: mimeType,
			Text:     text,
		}, nil
	}
}
