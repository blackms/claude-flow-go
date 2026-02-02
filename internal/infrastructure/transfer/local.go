// Package transfer provides infrastructure for pattern transfer and sharing.
package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
)

// LocalBackend implements storage using local filesystem.
type LocalBackend struct {
	basePath string
}

// NewLocalBackend creates a new local filesystem backend.
func NewLocalBackend(basePath string) (*LocalBackend, error) {
	if basePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		basePath = filepath.Join(home, ".claude-flow", "patterns", "local")
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &LocalBackend{basePath: basePath}, nil
}

// Upload stores data locally and returns a pseudo-CID.
func (b *LocalBackend) Upload(data []byte, name string) (string, error) {
	// Generate CID-like hash
	hash := sha256.Sum256(data)
	cid := "local-" + hex.EncodeToString(hash[:16])

	// Write file
	path := filepath.Join(b.basePath, cid+".cfp")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return cid, nil
}

// Download retrieves data from local storage.
func (b *LocalBackend) Download(cid string) ([]byte, error) {
	path := filepath.Join(b.basePath, cid+".cfp")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, transfer.ErrPatternNotFound
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// Exists checks if a pattern exists locally.
func (b *LocalBackend) Exists(cid string) (bool, error) {
	path := filepath.Join(b.basePath, cid+".cfp")
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete removes a pattern from local storage.
func (b *LocalBackend) Delete(cid string) error {
	path := filepath.Join(b.basePath, cid+".cfp")
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetPath returns the full path for a CID.
func (b *LocalBackend) GetPath(cid string) string {
	return filepath.Join(b.basePath, cid+".cfp")
}

// List lists all locally stored patterns.
func (b *LocalBackend) List() ([]string, error) {
	entries, err := os.ReadDir(b.basePath)
	if err != nil {
		return nil, err
	}

	var cids []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".cfp" {
			cid := entry.Name()[:len(entry.Name())-4]
			cids = append(cids, cid)
		}
	}

	return cids, nil
}

// ReadFile reads a file by path (not CID).
func (b *LocalBackend) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes a file by path.
func (b *LocalBackend) WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// BasePath returns the base path.
func (b *LocalBackend) BasePath() string {
	return b.basePath
}
