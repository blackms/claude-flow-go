// Package transfer provides infrastructure for pattern transfer and sharing.
package transfer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
)

// GCSBackend implements storage using Google Cloud Storage.
type GCSBackend struct {
	bucket     string
	prefix     string
	projectID  string
	credsFile  string
	httpClient *http.Client
}

// GCSConfig holds GCS configuration.
type GCSConfig struct {
	Bucket      string
	Prefix      string
	ProjectID   string
	Credentials string
}

// NewGCSBackend creates a new GCS backend.
func NewGCSBackend(config GCSConfig) *GCSBackend {
	// Load from environment if not provided
	if config.Bucket == "" {
		config.Bucket = os.Getenv("GCS_BUCKET")
		if config.Bucket == "" {
			config.Bucket = os.Getenv("GOOGLE_CLOUD_BUCKET")
		}
	}
	if config.Prefix == "" {
		config.Prefix = os.Getenv("GCS_PREFIX")
		if config.Prefix == "" {
			config.Prefix = "claude-flow-patterns"
		}
	}
	if config.ProjectID == "" {
		config.ProjectID = os.Getenv("GCS_PROJECT_ID")
		if config.ProjectID == "" {
			config.ProjectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
	}
	if config.Credentials == "" {
		config.Credentials = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}

	return &GCSBackend{
		bucket:    config.Bucket,
		prefix:    config.Prefix,
		projectID: config.ProjectID,
		credsFile: config.Credentials,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Upload uploads data to GCS.
func (b *GCSBackend) Upload(data []byte, name string) (string, error) {
	if b.bucket == "" {
		return "", fmt.Errorf("GCS bucket not configured")
	}

	// Generate object path
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:16])
	objectPath := filepath.Join(b.prefix, hashStr+".cfp")
	gsURI := fmt.Sprintf("gs://%s/%s", b.bucket, objectPath)

	// Try gcloud CLI first
	if b.hasGcloud() {
		if err := b.uploadViaGcloud(data, objectPath); err == nil {
			return gsURI, nil
		}
	}

	// Fall back to HTTP API (requires authentication)
	if err := b.uploadViaHTTP(data, objectPath); err != nil {
		return "", fmt.Errorf("%w: %v", transfer.ErrUploadFailed, err)
	}

	return gsURI, nil
}

// uploadViaGcloud uploads using gcloud CLI.
func (b *GCSBackend) uploadViaGcloud(data []byte, objectPath string) error {
	// Write to temp file
	tmpFile, err := os.CreateTemp("", "cfp-upload-*.cfp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// Upload via gcloud
	gsURI := fmt.Sprintf("gs://%s/%s", b.bucket, objectPath)
	cmd := exec.Command("gcloud", "storage", "cp", tmpFile.Name(), gsURI)
	if b.projectID != "" {
		cmd.Env = append(os.Environ(), "CLOUDSDK_CORE_PROJECT="+b.projectID)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gcloud upload failed: %s", string(output))
	}

	return nil
}

// uploadViaHTTP uploads using HTTP API.
func (b *GCSBackend) uploadViaHTTP(data []byte, objectPath string) error {
	url := fmt.Sprintf("https://storage.googleapis.com/upload/storage/v1/b/%s/o?uploadType=media&name=%s",
		b.bucket, objectPath)

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GCS upload failed: %s - %s", resp.Status, string(body))
	}

	return nil
}

// Download downloads data from GCS.
func (b *GCSBackend) Download(uri string) ([]byte, error) {
	// Parse gs:// URI
	if !strings.HasPrefix(uri, "gs://") {
		return nil, fmt.Errorf("invalid GCS URI: %s", uri)
	}

	parts := strings.SplitN(uri[5:], "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid GCS URI: %s", uri)
	}
	bucket := parts[0]
	objectPath := parts[1]

	// Try public URL first
	publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, objectPath)
	resp, err := b.httpClient.Get(publicURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Try gcloud CLI
	if b.hasGcloud() {
		data, err := b.downloadViaGcloud(uri)
		if err == nil {
			return data, nil
		}
	}

	return nil, transfer.ErrDownloadFailed
}

// downloadViaGcloud downloads using gcloud CLI.
func (b *GCSBackend) downloadViaGcloud(uri string) ([]byte, error) {
	tmpFile, err := os.CreateTemp("", "cfp-download-*.cfp")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cmd := exec.Command("gcloud", "storage", "cp", uri, tmpFile.Name())
	if b.projectID != "" {
		cmd.Env = append(os.Environ(), "CLOUDSDK_CORE_PROJECT="+b.projectID)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gcloud download failed: %s", string(output))
	}

	return os.ReadFile(tmpFile.Name())
}

// Exists checks if an object exists in GCS.
func (b *GCSBackend) Exists(uri string) (bool, error) {
	if !strings.HasPrefix(uri, "gs://") {
		return false, fmt.Errorf("invalid GCS URI: %s", uri)
	}

	parts := strings.SplitN(uri[5:], "/", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid GCS URI: %s", uri)
	}
	bucket := parts[0]
	objectPath := parts[1]

	// Check via HEAD request
	publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, objectPath)
	req, err := http.NewRequest("HEAD", publicURL, nil)
	if err != nil {
		return false, err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// Delete deletes an object from GCS.
func (b *GCSBackend) Delete(uri string) error {
	if !b.hasGcloud() {
		return fmt.Errorf("gcloud CLI required for delete operations")
	}

	cmd := exec.Command("gcloud", "storage", "rm", uri)
	if b.projectID != "" {
		cmd.Env = append(os.Environ(), "CLOUDSDK_CORE_PROJECT="+b.projectID)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gcloud delete failed: %s", string(output))
	}

	return nil
}

// GetPublicURL returns the public URL for a GCS URI.
func (b *GCSBackend) GetPublicURL(uri string) string {
	if !strings.HasPrefix(uri, "gs://") {
		return uri
	}

	parts := strings.SplitN(uri[5:], "/", 2)
	if len(parts) != 2 {
		return uri
	}

	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", parts[0], parts[1])
}

// hasGcloud checks if gcloud CLI is available.
func (b *GCSBackend) hasGcloud() bool {
	_, err := exec.LookPath("gcloud")
	return err == nil
}

// IsConfigured returns whether GCS is configured.
func (b *GCSBackend) IsConfigured() bool {
	return b.bucket != ""
}

// IsGCSURI checks if a URI is a GCS URI.
func IsGCSURI(uri string) bool {
	return strings.HasPrefix(uri, "gs://")
}
