// Package transfer provides domain types for pattern transfer and sharing.
package transfer

import (
	"sort"
	"strings"
	"time"
)

// PatternEntry represents a pattern in the registry.
type PatternEntry struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	CID         string    `json:"cid"` // IPFS CID or gs:// URI
	Categories  []string  `json:"categories"`
	Tags        []string  `json:"tags"`
	Downloads   int       `json:"downloads"`
	Rating      float64   `json:"rating"`
	Reviews     int       `json:"reviews"`
	Verified    bool      `json:"verified"`
	Featured    bool      `json:"featured"`
	Trending    bool      `json:"trending"`
	Author      Author    `json:"author"`
	License     string    `json:"license"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	Language    string    `json:"language,omitempty"`
	Framework   string    `json:"framework,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// PatternRegistry holds the pattern registry.
type PatternRegistry struct {
	Name      string         `json:"name"`
	Version   string         `json:"version"`
	UpdatedAt time.Time      `json:"updatedAt"`
	Patterns  []PatternEntry `json:"patterns"`
	Signature string         `json:"signature,omitempty"`
}

// RegistryConfig holds registry configuration.
type RegistryConfig struct {
	Name          string `json:"name"`
	Gateway       string `json:"gateway"`
	CacheDir      string `json:"cacheDir"`
	CacheExpiry   int64  `json:"cacheExpiry"` // seconds
	Verification  bool   `json:"verification"`
	MinTrustLevel string `json:"minTrustLevel"`
}

// DefaultRegistryConfig returns default registry configuration.
func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		Name:          "claude-flow-official",
		Gateway:       "https://w3s.link",
		CacheDir:      ".claude-flow/patterns/cache",
		CacheExpiry:   3600, // 1 hour
		Verification:  false,
		MinTrustLevel: "unverified",
	}
}

// NewPatternRegistry creates a new pattern registry.
func NewPatternRegistry(name string) *PatternRegistry {
	return &PatternRegistry{
		Name:      name,
		Version:   "1.0.0",
		UpdatedAt: time.Now(),
		Patterns:  make([]PatternEntry, 0),
	}
}

// AddPattern adds a pattern to the registry.
func (r *PatternRegistry) AddPattern(entry PatternEntry) {
	r.Patterns = append(r.Patterns, entry)
	r.UpdatedAt = time.Now()
}

// FindByName finds a pattern by name.
func (r *PatternRegistry) FindByName(name string) *PatternEntry {
	nameLower := strings.ToLower(name)
	for i := range r.Patterns {
		if strings.ToLower(r.Patterns[i].Name) == nameLower ||
			strings.ToLower(r.Patterns[i].DisplayName) == nameLower {
			return &r.Patterns[i]
		}
	}
	return nil
}

// FindByID finds a pattern by ID.
func (r *PatternRegistry) FindByID(id string) *PatternEntry {
	for i := range r.Patterns {
		if r.Patterns[i].ID == id {
			return &r.Patterns[i]
		}
	}
	return nil
}

// FindByCID finds a pattern by CID.
func (r *PatternRegistry) FindByCID(cid string) *PatternEntry {
	for i := range r.Patterns {
		if r.Patterns[i].CID == cid {
			return &r.Patterns[i]
		}
	}
	return nil
}

// ListFiltered returns patterns matching the filter.
func (r *PatternRegistry) ListFiltered(filter PatternFilter) []PatternEntry {
	results := make([]PatternEntry, 0)

	for _, p := range r.Patterns {
		if filter.Match(p) {
			results = append(results, p)
		}
	}

	// Sort based on filter
	switch filter.SortBy {
	case "downloads":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Downloads > results[j].Downloads
		})
	case "rating":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Rating > results[j].Rating
		})
	case "newest":
		sort.Slice(results, func(i, j int) bool {
			return results[i].CreatedAt.After(results[j].CreatedAt)
		})
	default:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})
	}

	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results
}

// Search searches patterns by query.
func (r *PatternRegistry) Search(query string, filter PatternFilter) []PatternEntry {
	queryLower := strings.ToLower(query)
	results := make([]PatternEntry, 0)

	for _, p := range r.Patterns {
		// Search in name, displayName, description, tags
		if strings.Contains(strings.ToLower(p.Name), queryLower) ||
			strings.Contains(strings.ToLower(p.DisplayName), queryLower) ||
			strings.Contains(strings.ToLower(p.Description), queryLower) ||
			containsTag(p.Tags, queryLower) {
			if filter.Match(p) {
				results = append(results, p)
			}
		}
	}

	// Sort by relevance (name match first, then downloads)
	sort.Slice(results, func(i, j int) bool {
		iNameMatch := strings.Contains(strings.ToLower(results[i].Name), queryLower)
		jNameMatch := strings.Contains(strings.ToLower(results[j].Name), queryLower)
		if iNameMatch != jNameMatch {
			return iNameMatch
		}
		return results[i].Downloads > results[j].Downloads
	})

	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results
}

// PatternFilter defines filtering options for patterns.
type PatternFilter struct {
	Category    string
	Language    string
	Framework   string
	Tags        []string
	MinRating   float64
	Verified    bool
	Featured    bool
	Trending    bool
	Newest      bool
	Limit       int
	SortBy      string
}

// Match checks if a pattern matches the filter.
func (f PatternFilter) Match(p PatternEntry) bool {
	// Category filter
	if f.Category != "" && !containsIgnoreCase(p.Categories, f.Category) {
		return false
	}

	// Language filter
	if f.Language != "" && !strings.EqualFold(p.Language, f.Language) {
		return false
	}

	// Framework filter
	if f.Framework != "" && !strings.EqualFold(p.Framework, f.Framework) {
		return false
	}

	// Tags filter
	if len(f.Tags) > 0 {
		hasAllTags := true
		for _, tag := range f.Tags {
			if !containsIgnoreCase(p.Tags, tag) {
				hasAllTags = false
				break
			}
		}
		if !hasAllTags {
			return false
		}
	}

	// Rating filter
	if f.MinRating > 0 && p.Rating < f.MinRating {
		return false
	}

	// Verified filter
	if f.Verified && !p.Verified {
		return false
	}

	// Featured filter
	if f.Featured && !p.Featured {
		return false
	}

	// Trending filter
	if f.Trending && !p.Trending {
		return false
	}

	return true
}

// containsIgnoreCase checks if a slice contains a string (case-insensitive).
func containsIgnoreCase(slice []string, s string) bool {
	sLower := strings.ToLower(s)
	for _, item := range slice {
		if strings.ToLower(item) == sLower {
			return true
		}
	}
	return false
}

// containsTag checks if tags contain a query.
func containsTag(tags []string, query string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
