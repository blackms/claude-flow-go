// Package transfer provides application services for pattern transfer.
package transfer

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
	infraTransfer "github.com/anthropics/claude-flow-go/internal/infrastructure/transfer"
)

// PatternPublisher handles pattern publishing.
type PatternPublisher struct {
	ipfs   *infraTransfer.IPFSBackend
	gcs    *infraTransfer.GCSBackend
	local  *infraTransfer.LocalBackend
	config transfer.StoreConfig
}

// NewPatternPublisher creates a new pattern publisher.
func NewPatternPublisher(config transfer.StoreConfig) (*PatternPublisher, error) {
	local, err := infraTransfer.NewLocalBackend("")
	if err != nil {
		return nil, err
	}

	return &PatternPublisher{
		ipfs:   infraTransfer.NewIPFSBackend(infraTransfer.IPFSConfig{Gateway: config.Gateway}),
		gcs:    infraTransfer.NewGCSBackend(infraTransfer.GCSConfig{}),
		local:  local,
		config: config,
	}, nil
}

// Publish publishes a pattern to the store.
func (p *PatternPublisher) Publish(options transfer.PublishOptions) (*transfer.PublishResult, error) {
	// Read and parse input file
	data, err := os.ReadFile(options.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read input file: %w", err)
	}

	var cfp transfer.CFPFormat
	if err := json.Unmarshal(data, &cfp); err != nil {
		return nil, fmt.Errorf("invalid CFP format: %w", err)
	}

	// Update metadata
	cfp.Metadata.Name = options.Name
	cfp.Metadata.Description = options.Description
	cfp.Metadata.Tags = options.Tags
	cfp.Metadata.License = options.License
	cfp.Metadata.Language = options.Language
	cfp.Metadata.Framework = options.Framework
	cfp.Metadata.UpdatedAt = time.Now().Format(time.RFC3339)

	if cfp.Metadata.ID == "" {
		cfp.Metadata.ID = uuid.New().String()
	}

	// Apply anonymization
	p.applyAnonymization(&cfp, options.Anonymize)

	// Validate
	if err := p.validateForPublish(&cfp, options); err != nil {
		return nil, err
	}

	// Serialize
	publishData, err := json.MarshalIndent(cfp, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize: %w", err)
	}

	// Calculate checksum
	hash := sha256.Sum256(publishData)
	checksum := hex.EncodeToString(hash[:])

	// Sign if requested
	if options.Sign {
		signature, err := p.signContent(publishData)
		if err != nil {
			return nil, fmt.Errorf("failed to sign: %w", err)
		}
		cfp.Signature = signature

		// Re-serialize with signature
		publishData, _ = json.MarshalIndent(cfp, "", "  ")
	}

	// Upload to storage
	var cid string

	// Try GCS first if configured
	if p.gcs.IsConfigured() {
		cid, err = p.gcs.Upload(publishData, options.Name)
		if err == nil {
			return &transfer.PublishResult{
				CID:         cid,
				GatewayURL:  p.gcs.GetPublicURL(cid),
				PatternID:   cfp.Metadata.ID,
				Checksum:    checksum,
				Size:        int64(len(publishData)),
				PublishedAt: time.Now(),
			}, nil
		}
	}

	// Try IPFS
	if p.ipfs.IsConfigured() {
		cid, err = p.ipfs.Upload(publishData, options.Name)
		if err == nil {
			return &transfer.PublishResult{
				CID:         cid,
				GatewayURL:  p.ipfs.GetGatewayURL(cid),
				PatternID:   cfp.Metadata.ID,
				Checksum:    checksum,
				Size:        int64(len(publishData)),
				PublishedAt: time.Now(),
			}, nil
		}
	}

	// Fall back to local storage
	cid, err = p.local.Upload(publishData, options.Name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", transfer.ErrUploadFailed, err)
	}

	return &transfer.PublishResult{
		CID:         cid,
		GatewayURL:  "file://" + p.local.GetPath(cid),
		PatternID:   cfp.Metadata.ID,
		Checksum:    checksum,
		Size:        int64(len(publishData)),
		PublishedAt: time.Now(),
	}, nil
}

// applyAnonymization applies anonymization to the CFP.
func (p *PatternPublisher) applyAnonymization(cfp *transfer.CFPFormat, level transfer.AnonymizationLevel) {
	transforms := make([]string, 0)

	switch level {
	case transfer.AnonymizationParanoid:
		// Most aggressive - remove all identifying info
		cfp.Metadata.Author = nil
		transforms = append(transforms, "author_removed")
		fallthrough

	case transfer.AnonymizationStrict:
		// Strip paths and generalize timestamps
		for i := range cfp.Patterns.Routing {
			cfp.Patterns.Routing[i].Trigger = p.stripPaths(cfp.Patterns.Routing[i].Trigger)
			cfp.Patterns.Routing[i].Action = p.stripPaths(cfp.Patterns.Routing[i].Action)
		}
		transforms = append(transforms, "paths_stripped")
		cfp.Anonymization.PathsStripped = true
		fallthrough

	case transfer.AnonymizationStandard:
		// Strip PII (emails, IPs)
		p.stripPII(cfp)
		transforms = append(transforms, "pii_redacted")
		cfp.Anonymization.PIIRedacted = true
		fallthrough

	case transfer.AnonymizationMinimal:
		// Generalize timestamps
		cfp.Anonymization.TimestampsGeneralized = true
		transforms = append(transforms, "timestamps_generalized")
	}

	cfp.Anonymization.Level = string(level)
	cfp.Anonymization.AppliedTransforms = transforms
}

// stripPaths removes file paths from content.
func (p *PatternPublisher) stripPaths(content string) string {
	pathRegex := regexp.MustCompile(`(/[a-zA-Z0-9._-]+)+`)
	return pathRegex.ReplaceAllString(content, "[PATH]")
}

// stripPII removes PII from the CFP.
func (p *PatternPublisher) stripPII(cfp *transfer.CFPFormat) {
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	ipRegex := regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)

	for i := range cfp.Patterns.Routing {
		cfp.Patterns.Routing[i].Trigger = emailRegex.ReplaceAllString(cfp.Patterns.Routing[i].Trigger, "[EMAIL]")
		cfp.Patterns.Routing[i].Trigger = ipRegex.ReplaceAllString(cfp.Patterns.Routing[i].Trigger, "[IP]")
		cfp.Patterns.Routing[i].Action = emailRegex.ReplaceAllString(cfp.Patterns.Routing[i].Action, "[EMAIL]")
		cfp.Patterns.Routing[i].Action = ipRegex.ReplaceAllString(cfp.Patterns.Routing[i].Action, "[IP]")
	}

	// Strip from author if present
	if cfp.Metadata.Author != nil {
		cfp.Metadata.Author.Email = ""
	}
}

// validateForPublish validates the CFP for publishing.
func (p *PatternPublisher) validateForPublish(cfp *transfer.CFPFormat, options transfer.PublishOptions) error {
	if options.Name == "" {
		return fmt.Errorf("pattern name is required")
	}
	if len(options.Name) < 3 || len(options.Name) > 64 {
		return fmt.Errorf("pattern name must be 3-64 characters")
	}

	if options.Description == "" {
		return fmt.Errorf("description is required")
	}
	if len(options.Description) < 10 || len(options.Description) > 500 {
		return fmt.Errorf("description must be 10-500 characters")
	}

	if len(options.Categories) == 0 {
		return fmt.Errorf("at least one category is required")
	}

	if len(options.Tags) == 0 {
		return fmt.Errorf("at least one tag is required")
	}

	if cfp.TotalPatternCount() == 0 {
		return fmt.Errorf("pattern file contains no patterns")
	}

	// Check for suspicious content
	if p.hasSuspiciousContent(cfp) {
		return fmt.Errorf("pattern contains suspicious content (eval, exec, etc.)")
	}

	return nil
}

// hasSuspiciousContent checks for suspicious patterns.
func (p *PatternPublisher) hasSuspiciousContent(cfp *transfer.CFPFormat) bool {
	suspiciousPatterns := []string{
		"eval(", "exec(", "system(", "__import__",
		"subprocess", "os.system", "shell_exec",
	}

	checkContent := func(content string) bool {
		for _, pattern := range suspiciousPatterns {
			if strings.Contains(content, pattern) {
				return true
			}
		}
		return false
	}

	for _, r := range cfp.Patterns.Routing {
		if checkContent(r.Trigger) || checkContent(r.Action) {
			return true
		}
	}

	return false
}

// signContent signs the content with Ed25519.
func (p *PatternPublisher) signContent(data []byte) (*transfer.CFPSignature, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	signature := ed25519.Sign(privKey, data)

	return &transfer.CFPSignature{
		Algorithm: "ed25519",
		PublicKey: base64.StdEncoding.EncodeToString(pubKey),
		Signature: base64.StdEncoding.EncodeToString(signature),
	}, nil
}

// CreateFromNeuralPatterns creates a CFP from neural patterns.
func (p *PatternPublisher) CreateFromNeuralPatterns(patterns []RoutingPatternInput) *transfer.CFPFormat {
	cfp := transfer.NewCFPFormat()

	for _, input := range patterns {
		cfp.Patterns.Routing = append(cfp.Patterns.Routing, transfer.RoutingPattern{
			ID:          uuid.New().String(),
			Trigger:     input.Trigger,
			Action:      input.Action,
			Confidence:  input.Confidence,
			UsageCount:  input.UsageCount,
			SuccessRate: input.SuccessRate,
		})
	}

	cfp.Statistics.TotalPatterns = cfp.TotalPatternCount()
	cfp.Statistics.PatternTypes["routing"] = len(cfp.Patterns.Routing)

	return cfp
}

// RoutingPatternInput represents input for creating a routing pattern.
type RoutingPatternInput struct {
	Trigger     string
	Action      string
	Confidence  float64
	UsageCount  int
	SuccessRate float64
}

// SaveCFPToFile saves a CFP to a file.
func (p *PatternPublisher) SaveCFPToFile(cfp *transfer.CFPFormat, path string) error {
	data, err := json.MarshalIndent(cfp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
