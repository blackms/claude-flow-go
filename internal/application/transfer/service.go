// Package transfer provides application services for pattern transfer.
package transfer

import (
	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
)

// Service orchestrates all transfer operations.
type Service struct {
	discovery  *PatternDiscovery
	downloader *PatternDownloader
	publisher  *PatternPublisher
	config     transfer.StoreConfig
}

// NewService creates a new transfer service.
func NewService(config transfer.StoreConfig) (*Service, error) {
	regConfig := transfer.RegistryConfig{
		Name:        config.Registry,
		Gateway:     config.Gateway,
		CacheDir:    config.CacheDir,
		CacheExpiry: config.CacheExpiry,
	}

	discovery, err := NewPatternDiscovery(regConfig)
	if err != nil {
		return nil, err
	}

	downloader, err := NewPatternDownloader(discovery, config)
	if err != nil {
		return nil, err
	}

	publisher, err := NewPatternPublisher(config)
	if err != nil {
		return nil, err
	}

	return &Service{
		discovery:  discovery,
		downloader: downloader,
		publisher:  publisher,
		config:     config,
	}, nil
}

// List lists patterns from the registry.
func (s *Service) List(options transfer.ListOptions) ([]transfer.PatternEntry, error) {
	registry := options.Registry
	if registry == "" {
		registry = s.config.Registry
	}

	filter := transfer.PatternFilter{
		Category: options.Category,
		Featured: options.Featured,
		Trending: options.Trending,
		Newest:   options.Newest,
		Limit:    options.Limit,
	}

	if options.Featured {
		filter.SortBy = "downloads"
	} else if options.Trending {
		filter.SortBy = "downloads"
	} else if options.Newest {
		filter.SortBy = "newest"
	}

	return s.discovery.ListPatterns(registry, filter)
}

// Search searches patterns in the registry.
func (s *Service) Search(options transfer.SearchOptions) ([]transfer.PatternEntry, error) {
	filter := transfer.PatternFilter{
		Category:  options.Category,
		Language:  options.Language,
		Framework: options.Framework,
		Tags:      options.Tags,
		MinRating: options.MinRating,
		Verified:  options.Verified,
		Limit:     options.Limit,
		SortBy:    "downloads",
	}

	return s.discovery.SearchPatterns(s.config.Registry, options.Query, filter)
}

// Download downloads a pattern.
func (s *Service) Download(options transfer.DownloadOptions) (*transfer.DownloadResult, error) {
	return s.downloader.Download(options)
}

// Publish publishes a pattern.
func (s *Service) Publish(options transfer.PublishOptions) (*transfer.PublishResult, error) {
	return s.publisher.Publish(options)
}

// Info returns information about a pattern.
func (s *Service) Info(options transfer.InfoOptions) (*PatternInfo, error) {
	return s.discovery.GetPatternInfo(s.config.Registry, options.Name)
}

// GetRegistryStats returns registry statistics.
func (s *Service) GetRegistryStats() (*RegistryStats, error) {
	return s.discovery.RegistryStats(s.config.Registry)
}

// Close closes the service.
func (s *Service) Close() error {
	return s.discovery.Close()
}

// Discovery returns the discovery service.
func (s *Service) Discovery() *PatternDiscovery {
	return s.discovery
}

// Downloader returns the downloader service.
func (s *Service) Downloader() *PatternDownloader {
	return s.downloader
}

// Publisher returns the publisher service.
func (s *Service) Publisher() *PatternPublisher {
	return s.publisher
}

// Config returns the service configuration.
func (s *Service) Config() transfer.StoreConfig {
	return s.config
}

// RefreshRegistry forces a registry refresh.
func (s *Service) RefreshRegistry() error {
	return s.discovery.RefreshRegistry(s.config.Registry)
}

// ClearCache clears the download cache.
func (s *Service) ClearCache() error {
	return s.downloader.ClearCache()
}

// GetCacheStats returns cache statistics.
func (s *Service) GetCacheStats() CacheStats {
	stats := s.downloader.GetCacheStats()
	return CacheStats{
		EntryCount: stats.EntryCount,
		TotalSize:  stats.TotalSize,
		TotalHits:  stats.TotalHits,
	}
}

// CacheStats holds cache statistics (re-exported for convenience).
type CacheStats struct {
	EntryCount int   `json:"entryCount"`
	TotalSize  int64 `json:"totalSize"`
	TotalHits  int   `json:"totalHits"`
}
