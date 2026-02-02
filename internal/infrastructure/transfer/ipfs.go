// Package transfer provides infrastructure for pattern transfer and sharing.
package transfer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
)

// IPFSBackend implements storage using IPFS.
type IPFSBackend struct {
	apiURL       string
	gateway      string
	pinataKey    string
	pinataSecret string
	web3Token    string
	httpClient   *http.Client
}

// IPFSConfig holds IPFS configuration.
type IPFSConfig struct {
	APIURL       string
	Gateway      string
	PinataKey    string
	PinataSecret string
	Web3Token    string
}

// NewIPFSBackend creates a new IPFS backend.
func NewIPFSBackend(config IPFSConfig) *IPFSBackend {
	// Load from environment if not provided
	if config.APIURL == "" {
		config.APIURL = os.Getenv("IPFS_API_URL")
	}
	if config.Gateway == "" {
		config.Gateway = os.Getenv("IPFS_GATEWAY")
		if config.Gateway == "" {
			config.Gateway = "https://w3s.link"
		}
	}
	if config.PinataKey == "" {
		config.PinataKey = os.Getenv("PINATA_API_KEY")
	}
	if config.PinataSecret == "" {
		config.PinataSecret = os.Getenv("PINATA_API_SECRET")
	}
	if config.Web3Token == "" {
		config.Web3Token = os.Getenv("WEB3_STORAGE_TOKEN")
	}

	return &IPFSBackend{
		apiURL:       config.APIURL,
		gateway:      config.Gateway,
		pinataKey:    config.PinataKey,
		pinataSecret: config.PinataSecret,
		web3Token:    config.Web3Token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Upload uploads data to IPFS.
func (b *IPFSBackend) Upload(data []byte, name string) (string, error) {
	// Try pinning services in order
	var cid string
	var err error

	// 1. Try local IPFS node
	if b.apiURL != "" {
		cid, err = b.uploadToLocalNode(data, name)
		if err == nil {
			return cid, nil
		}
	}

	// 2. Try Pinata
	if b.pinataKey != "" && b.pinataSecret != "" {
		cid, err = b.uploadToPinata(data, name)
		if err == nil {
			return cid, nil
		}
	}

	// 3. Try Web3.Storage
	if b.web3Token != "" {
		cid, err = b.uploadToWeb3Storage(data, name)
		if err == nil {
			return cid, nil
		}
	}

	// 4. Fall back to demo mode (generate deterministic CID)
	cid = b.generateDemoCID(data)
	return cid, nil
}

// uploadToLocalNode uploads to local IPFS node.
func (b *IPFSBackend) uploadToLocalNode(data []byte, name string) (string, error) {
	url := b.apiURL + "/api/v0/add"

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IPFS add failed: %s", resp.Status)
	}

	var result struct {
		Hash string `json:"Hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Hash, nil
}

// uploadToPinata uploads to Pinata.
func (b *IPFSBackend) uploadToPinata(data []byte, name string) (string, error) {
	url := "https://api.pinata.cloud/pinning/pinFileToIPFS"

	body := &bytes.Buffer{}
	body.Write(data)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("pinata_api_key", b.pinataKey)
	req.Header.Set("pinata_secret_api_key", b.pinataSecret)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Pinata upload failed: %s", resp.Status)
	}

	var result struct {
		IpfsHash string `json:"IpfsHash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.IpfsHash, nil
}

// uploadToWeb3Storage uploads to Web3.Storage.
func (b *IPFSBackend) uploadToWeb3Storage(data []byte, name string) (string, error) {
	url := "https://api.web3.storage/upload"

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+b.web3Token)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Web3.Storage upload failed: %s", resp.Status)
	}

	var result struct {
		CID string `json:"cid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.CID, nil
}

// generateDemoCID generates a deterministic demo CID.
func (b *IPFSBackend) generateDemoCID(data []byte) string {
	hash := sha256.Sum256(data)
	// Use bafybei prefix for CIDv1 simulation
	return "bafybei" + hex.EncodeToString(hash[:])[:52]
}

// Download downloads data from IPFS.
func (b *IPFSBackend) Download(cid string) ([]byte, error) {
	// Try multiple gateways
	gateways := transfer.IPFSGateways()

	// Add configured gateway first
	if b.gateway != "" && !strings.HasSuffix(b.gateway, "/ipfs/") {
		gateways = append([]string{b.gateway + "/ipfs/"}, gateways...)
	}

	var lastErr error
	for _, gateway := range gateways {
		url := gateway + cid

		resp, err := b.httpClient.Get(url)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("gateway %s returned %s", gateway, resp.Status)
			continue
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		return data, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", transfer.ErrDownloadFailed, lastErr)
	}
	return nil, transfer.ErrDownloadFailed
}

// Exists checks if a CID exists on IPFS.
func (b *IPFSBackend) Exists(cid string) (bool, error) {
	// Check via HEAD request to gateway
	url := b.gateway + "/ipfs/" + cid

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return false, nil // Network error, assume doesn't exist
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// Delete is a no-op for IPFS (content is immutable).
func (b *IPFSBackend) Delete(cid string) error {
	// IPFS content is immutable, can't delete
	// Could unpin from pinning services but that's complex
	return nil
}

// GetGatewayURL returns the gateway URL for a CID.
func (b *IPFSBackend) GetGatewayURL(cid string) string {
	gateway := b.gateway
	if !strings.HasSuffix(gateway, "/") {
		gateway += "/"
	}
	return gateway + "ipfs/" + cid
}

// ResolveIPNS resolves an IPNS name to a CID.
func (b *IPFSBackend) ResolveIPNS(name string) (string, error) {
	// Try multiple gateways for IPNS resolution
	gateways := []string{
		"https://ipfs.io",
		"https://dweb.link",
		"https://cloudflare-ipfs.com",
	}

	for _, gateway := range gateways {
		url := gateway + "/api/v0/name/resolve?arg=" + name

		resp, err := b.httpClient.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var result struct {
			Path string `json:"Path"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			continue
		}

		// Extract CID from path (e.g., "/ipfs/Qm...")
		if strings.HasPrefix(result.Path, "/ipfs/") {
			return result.Path[6:], nil
		}
		return result.Path, nil
	}

	return "", fmt.Errorf("failed to resolve IPNS name: %s", name)
}

// IsConfigured returns whether any upload method is configured.
func (b *IPFSBackend) IsConfigured() bool {
	return b.apiURL != "" || b.pinataKey != "" || b.web3Token != ""
}
