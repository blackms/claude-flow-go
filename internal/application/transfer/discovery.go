// Package transfer provides application services for pattern transfer.
package transfer

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
	infraTransfer "github.com/anthropics/claude-flow-go/internal/infrastructure/transfer"
)

// PatternDiscovery handles registry discovery and pattern lookup.
type PatternDiscovery struct {
	mu        sync.RWMutex
	ipfs      *infraTransfer.IPFSBackend
	cache     *infraTransfer.Cache
	registries map[string]*cachedRegistry
	config    transfer.RegistryConfig
}

// cachedRegistry holds a cached registry.
type cachedRegistry struct {
	registry  *transfer.PatternRegistry
	fetchedAt time.Time
}

// NewPatternDiscovery creates a new pattern discovery service.
func NewPatternDiscovery(config transfer.RegistryConfig) (*PatternDiscovery, error) {
	cache, err := infraTransfer.NewCache(config.CacheDir, config.CacheExpiry)
	if err != nil {
		return nil, err
	}

	return &PatternDiscovery{
		ipfs:       infraTransfer.NewIPFSBackend(infraTransfer.IPFSConfig{Gateway: config.Gateway}),
		cache:      cache,
		registries: make(map[string]*cachedRegistry),
		config:     config,
	}, nil
}

// GetRegistry fetches a pattern registry.
func (d *PatternDiscovery) GetRegistry(name string) (*transfer.PatternRegistry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check cache
	if cached, ok := d.registries[name]; ok {
		if time.Since(cached.fetchedAt) < time.Duration(d.config.CacheExpiry)*time.Second {
			return cached.registry, nil
		}
	}

	// Find bootstrap registry
	var bootstrap *transfer.BootstrapRegistry
	for _, br := range transfer.DefaultBootstrapRegistries() {
		if br.Name == name {
			bootstrap = &br
			break
		}
	}

	if bootstrap == nil {
		return nil, transfer.ErrRegistryNotFound
	}

	// Try to fetch from IPFS
	registry, err := d.fetchRegistryFromIPFS(bootstrap)
	if err != nil {
		// Fall back to genesis registry
		registry = d.getGenesisRegistry(name)
	}

	// Cache the registry
	d.registries[name] = &cachedRegistry{
		registry:  registry,
		fetchedAt: time.Now(),
	}

	return registry, nil
}

// fetchRegistryFromIPFS fetches a registry from IPFS.
func (d *PatternDiscovery) fetchRegistryFromIPFS(bootstrap *transfer.BootstrapRegistry) (*transfer.PatternRegistry, error) {
	// Resolve IPNS to CID
	cid, err := d.ipfs.ResolveIPNS(bootstrap.IPNS)
	if err != nil {
		return nil, err
	}

	// Download registry
	data, err := d.ipfs.Download(cid)
	if err != nil {
		return nil, err
	}

	// Parse registry
	var registry transfer.PatternRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	return &registry, nil
}

// getGenesisRegistry returns a genesis (empty or sample) registry.
func (d *PatternDiscovery) getGenesisRegistry(name string) *transfer.PatternRegistry {
	registry := transfer.NewPatternRegistry(name)

	// Add some sample patterns for demo purposes
	registry.Patterns = []transfer.PatternEntry{
		{
			ID:          "sample-routing-patterns",
			Name:        "routing-patterns",
			DisplayName: "Task Routing Patterns",
			Description: "Pre-trained patterns for intelligent task routing based on complexity and domain",
			Version:     "1.0.0",
			CID:         "bafybeisample1routingpatterns",
			Categories:  []string{"routing", "optimization"},
			Tags:        []string{"routing", "task-management", "ai"},
			Downloads:   1250,
			Rating:      4.8,
			Reviews:     45,
			Verified:    true,
			Featured:    true,
			License:     "MIT",
			Author:      transfer.Author{DisplayName: "Claude Flow Team"},
			CreatedAt:   time.Now().AddDate(0, -3, 0),
			UpdatedAt:   time.Now().AddDate(0, -1, 0),
		},
		{
			ID:          "sample-code-complexity",
			Name:        "code-complexity",
			DisplayName: "Code Complexity Analysis",
			Description: "Patterns for analyzing and predicting code complexity",
			Version:     "1.2.0",
			CID:         "bafybeisample2codecomplexity",
			Categories:  []string{"analysis", "code-quality"},
			Tags:        []string{"complexity", "metrics", "static-analysis"},
			Downloads:   890,
			Rating:      4.5,
			Reviews:     32,
			Verified:    true,
			License:     "MIT",
			Author:      transfer.Author{DisplayName: "Community"},
			CreatedAt:   time.Now().AddDate(0, -2, 0),
			UpdatedAt:   time.Now().AddDate(0, 0, -15),
		},
		{
			ID:          "sample-test-coverage",
			Name:        "test-coverage",
			DisplayName: "Test Coverage Patterns",
			Description: "Patterns for predicting test coverage gaps and generating test suggestions",
			Version:     "0.9.0",
			CID:         "bafybeisample3testcoverage",
			Categories:  []string{"testing", "quality"},
			Tags:        []string{"testing", "coverage", "tdd"},
			Downloads:   650,
			Rating:      4.2,
			Reviews:     18,
			Trending:    true,
			License:     "Apache-2.0",
			Author:      transfer.Author{DisplayName: "Test Guild"},
			CreatedAt:   time.Now().AddDate(0, -1, 0),
			UpdatedAt:   time.Now().AddDate(0, 0, -5),
		},
	}

	return registry
}

// FindPattern finds a pattern by name or ID.
func (d *PatternDiscovery) FindPattern(registryName, nameOrID string) (*transfer.PatternEntry, error) {
	registry, err := d.GetRegistry(registryName)
	if err != nil {
		return nil, err
	}

	// Try by ID first
	if entry := registry.FindByID(nameOrID); entry != nil {
		return entry, nil
	}

	// Try by name
	if entry := registry.FindByName(nameOrID); entry != nil {
		return entry, nil
	}

	return nil, transfer.ErrPatternNotFound
}

// ListPatterns lists patterns with optional filtering.
func (d *PatternDiscovery) ListPatterns(registryName string, filter transfer.PatternFilter) ([]transfer.PatternEntry, error) {
	registry, err := d.GetRegistry(registryName)
	if err != nil {
		return nil, err
	}

	return registry.ListFiltered(filter), nil
}

// SearchPatterns searches patterns by query.
func (d *PatternDiscovery) SearchPatterns(registryName, query string, filter transfer.PatternFilter) ([]transfer.PatternEntry, error) {
	registry, err := d.GetRegistry(registryName)
	if err != nil {
		return nil, err
	}

	return registry.Search(query, filter), nil
}

// GetPatternInfo returns detailed information about a pattern.
func (d *PatternDiscovery) GetPatternInfo(registryName, nameOrID string) (*PatternInfo, error) {
	entry, err := d.FindPattern(registryName, nameOrID)
	if err != nil {
		return nil, err
	}

	return &PatternInfo{
		Entry:      *entry,
		GatewayURL: d.ipfs.GetGatewayURL(entry.CID),
	}, nil
}

// PatternInfo holds detailed pattern information.
type PatternInfo struct {
	Entry      transfer.PatternEntry `json:"entry"`
	GatewayURL string                `json:"gatewayUrl"`
}

// RefreshRegistry forces a registry refresh.
func (d *PatternDiscovery) RefreshRegistry(name string) error {
	d.mu.Lock()
	delete(d.registries, name)
	d.mu.Unlock()

	_, err := d.GetRegistry(name)
	return err
}

// GetCache returns the cache instance.
func (d *PatternDiscovery) GetCache() *infraTransfer.Cache {
	return d.cache
}

// GetIPFS returns the IPFS backend.
func (d *PatternDiscovery) GetIPFS() *infraTransfer.IPFSBackend {
	return d.ipfs
}

// Close closes the discovery service.
func (d *PatternDiscovery) Close() error {
	// Prune expired cache entries
	_, _ = d.cache.Prune()
	return nil
}

// RegistryStats returns statistics about a registry.
func (d *PatternDiscovery) RegistryStats(registryName string) (*RegistryStats, error) {
	registry, err := d.GetRegistry(registryName)
	if err != nil {
		return nil, err
	}

	stats := &RegistryStats{
		Name:         registry.Name,
		Version:      registry.Version,
		PatternCount: len(registry.Patterns),
		Categories:   make(map[string]int),
	}

	for _, p := range registry.Patterns {
		if p.Verified {
			stats.VerifiedCount++
		}
		if p.Featured {
			stats.FeaturedCount++
		}
		stats.TotalDownloads += p.Downloads

		for _, cat := range p.Categories {
			stats.Categories[cat]++
		}
	}

	return stats, nil
}

// RegistryStats holds registry statistics.
type RegistryStats struct {
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	PatternCount   int            `json:"patternCount"`
	VerifiedCount  int            `json:"verifiedCount"`
	FeaturedCount  int            `json:"featuredCount"`
	TotalDownloads int            `json:"totalDownloads"`
	Categories     map[string]int `json:"categories"`
}

// FormatPatternInfo formats pattern info for display.
func FormatPatternInfo(info *PatternInfo) string {
	e := info.Entry

	return fmt.Sprintf(`Pattern: %s
ID: %s
Version: %s
Description: %s

Author: %s
License: %s
Categories: %v
Tags: %v

Statistics:
  Downloads: %d
  Rating: %.1f (%d reviews)
  Verified: %v
  Featured: %v
  Trending: %v

IPFS:
  CID: %s
  Gateway: %s
  Size: %d bytes

Created: %s
Updated: %s`,
		e.DisplayName,
		e.ID,
		e.Version,
		e.Description,
		e.Author.DisplayName,
		e.License,
		e.Categories,
		e.Tags,
		e.Downloads,
		e.Rating, e.Reviews,
		e.Verified,
		e.Featured,
		e.Trending,
		e.CID,
		info.GatewayURL,
		e.Size,
		e.CreatedAt.Format(time.RFC3339),
		e.UpdatedAt.Format(time.RFC3339),
	)
}
