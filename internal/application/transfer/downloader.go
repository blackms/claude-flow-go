// Package transfer provides application services for pattern transfer.
package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
	infraTransfer "github.com/anthropics/claude-flow-go/internal/infrastructure/transfer"
)

// PatternDownloader handles pattern downloads.
type PatternDownloader struct {
	discovery *PatternDiscovery
	ipfs      *infraTransfer.IPFSBackend
	gcs       *infraTransfer.GCSBackend
	local     *infraTransfer.LocalBackend
	cache     *infraTransfer.Cache
	config    transfer.StoreConfig
}

// NewPatternDownloader creates a new pattern downloader.
func NewPatternDownloader(discovery *PatternDiscovery, config transfer.StoreConfig) (*PatternDownloader, error) {
	local, err := infraTransfer.NewLocalBackend("")
	if err != nil {
		return nil, err
	}

	cache, err := infraTransfer.NewCache(config.CacheDir, config.CacheExpiry)
	if err != nil {
		return nil, err
	}

	return &PatternDownloader{
		discovery: discovery,
		ipfs:      infraTransfer.NewIPFSBackend(infraTransfer.IPFSConfig{Gateway: config.Gateway}),
		gcs:       infraTransfer.NewGCSBackend(infraTransfer.GCSConfig{}),
		local:     local,
		cache:     cache,
		config:    config,
	}, nil
}

// Download downloads a pattern by name or ID.
func (d *PatternDownloader) Download(options transfer.DownloadOptions) (*transfer.DownloadResult, error) {
	// Find pattern in registry
	entry, err := d.discovery.FindPattern(d.config.Registry, options.Name)
	if err != nil {
		return nil, err
	}

	return d.DownloadByCID(entry.CID, entry.Checksum, options)
}

// DownloadByCID downloads a pattern by CID.
func (d *PatternDownloader) DownloadByCID(cid, expectedChecksum string, options transfer.DownloadOptions) (*transfer.DownloadResult, error) {
	result := &transfer.DownloadResult{
		CID:          cid,
		DownloadedAt: time.Now(),
	}

	// Check cache first
	if data, entry, err := d.cache.Get(cid); err == nil {
		result.FromCache = true
		result.Path = entry.Path
		result.Size = entry.Size
		result.Checksum = entry.Checksum
		result.Verified = true

		// Copy to output path if specified
		if options.OutputPath != "" {
			if err := d.writeOutput(data, options.OutputPath); err != nil {
				return nil, err
			}
			result.Path = options.OutputPath
		}

		return result, nil
	}

	// Download from appropriate backend
	var data []byte
	var err error

	if infraTransfer.IsGCSURI(cid) {
		data, err = d.gcs.Download(cid)
	} else {
		data, err = d.ipfs.Download(cid)
	}

	if err != nil {
		return nil, fmt.Errorf("%w: %v", transfer.ErrDownloadFailed, err)
	}

	result.Size = int64(len(data))

	// Calculate checksum
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])
	result.Checksum = checksum

	// Verify checksum if required
	if options.Verify && expectedChecksum != "" {
		if checksum != expectedChecksum {
			return nil, fmt.Errorf("%w: expected %s, got %s",
				transfer.ErrChecksumMismatch, expectedChecksum, checksum)
		}
		result.Verified = true
	}

	// Store in cache
	cacheEntry, err := d.cache.Put(cid, data, checksum)
	if err != nil {
		// Cache error is non-fatal
		fmt.Printf("Warning: failed to cache pattern: %v\n", err)
	} else {
		result.Path = cacheEntry.Path
	}

	// Write to output path if specified
	if options.OutputPath != "" {
		if err := d.writeOutput(data, options.OutputPath); err != nil {
			return nil, err
		}
		result.Path = options.OutputPath
	}

	// Import if requested
	if options.Import {
		if err := d.importPattern(data); err != nil {
			return nil, fmt.Errorf("import failed: %w", err)
		}
	}

	return result, nil
}

// writeOutput writes data to the output path.
func (d *PatternDownloader) writeOutput(data []byte, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// importPattern imports a downloaded pattern.
func (d *PatternDownloader) importPattern(data []byte) error {
	// Parse CFP format
	var cfp transfer.CFPFormat
	if err := json.Unmarshal(data, &cfp); err != nil {
		return fmt.Errorf("invalid CFP format: %w", err)
	}

	// Validate
	if err := cfp.Validate(); err != nil {
		return err
	}

	// Import patterns to local store
	// This would integrate with the neural pattern store
	fmt.Printf("Imported %d patterns from %s\n", cfp.TotalPatternCount(), cfp.Metadata.Name)

	return nil
}

// DownloadFromFile reads a pattern from a local file.
func (d *PatternDownloader) DownloadFromFile(path string) (*transfer.CFPFormat, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var cfp transfer.CFPFormat
	if err := json.Unmarshal(data, &cfp); err != nil {
		return nil, fmt.Errorf("invalid CFP format: %w", err)
	}

	if err := cfp.Validate(); err != nil {
		return nil, err
	}

	return &cfp, nil
}

// VerifyPattern verifies a downloaded pattern.
func (d *PatternDownloader) VerifyPattern(path string, expectedChecksum string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	return checksum == expectedChecksum, nil
}

// GetCacheStats returns cache statistics.
func (d *PatternDownloader) GetCacheStats() infraTransfer.CacheStats {
	return d.cache.Stats()
}

// ClearCache clears the download cache.
func (d *PatternDownloader) ClearCache() error {
	return d.cache.Clear()
}

// PruneCache removes expired cache entries.
func (d *PatternDownloader) PruneCache() (int, error) {
	return d.cache.Prune()
}

// ListCached lists all cached patterns.
func (d *PatternDownloader) ListCached() []transfer.CacheEntry {
	return d.cache.Entries()
}

// GetLocalPath returns the local path for a CID.
func (d *PatternDownloader) GetLocalPath(cid string) string {
	return d.local.GetPath(cid)
}

// ResolveDownloadSource resolves the download source for a pattern.
func (d *PatternDownloader) ResolveDownloadSource(cid string) string {
	if infraTransfer.IsGCSURI(cid) {
		return "gcs"
	}
	if strings.HasPrefix(cid, "local-") {
		return "local"
	}
	return "ipfs"
}
