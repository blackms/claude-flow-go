// Package transfer provides domain types for pattern transfer and sharing.
package transfer

import (
	"time"
)

// CFPFormat represents the Claude Flow Pattern format.
type CFPFormat struct {
	Magic         string           `json:"magic"`       // "CFP1"
	Version       string           `json:"version"`     // "1.0.0"
	CreatedAt     string           `json:"createdAt"`
	GeneratedBy   string           `json:"generatedBy"`
	Metadata      CFPMetadata      `json:"metadata"`
	Anonymization CFPAnonymization `json:"anonymization"`
	Patterns      CFPPatterns      `json:"patterns"`
	Statistics    CFPStatistics    `json:"statistics"`
	Signature     *CFPSignature    `json:"signature,omitempty"`
	IPFS          *CFPIPFSInfo     `json:"ipfs,omitempty"`
}

// CFPMetadata holds pattern metadata.
type CFPMetadata struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      *Author  `json:"author,omitempty"`
	License     string   `json:"license"`
	Tags        []string `json:"tags"`
	Language    string   `json:"language,omitempty"`
	Framework   string   `json:"framework,omitempty"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// Author represents a pattern author.
type Author struct {
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	URL         string `json:"url,omitempty"`
	Anonymous   bool   `json:"anonymous,omitempty"`
}

// CFPAnonymization holds anonymization configuration.
type CFPAnonymization struct {
	Level                  string   `json:"level"` // minimal, standard, strict, paranoid
	AppliedTransforms      []string `json:"appliedTransforms"`
	PIIRedacted            bool     `json:"piiRedacted"`
	PathsStripped          bool     `json:"pathsStripped"`
	TimestampsGeneralized  bool     `json:"timestampsGeneralized"`
	Checksum               string   `json:"checksum"` // SHA256 of original
}

// AnonymizationLevel defines anonymization levels.
type AnonymizationLevel string

const (
	AnonymizationMinimal  AnonymizationLevel = "minimal"
	AnonymizationStandard AnonymizationLevel = "standard"
	AnonymizationStrict   AnonymizationLevel = "strict"
	AnonymizationParanoid AnonymizationLevel = "paranoid"
)

// CFPPatterns holds the pattern collections.
type CFPPatterns struct {
	Routing    []RoutingPattern    `json:"routing"`
	Complexity []ComplexityPattern `json:"complexity"`
	Coverage   []CoveragePattern   `json:"coverage"`
	Trajectory []TrajectoryPattern `json:"trajectory"`
	Custom     []CustomPattern     `json:"custom"`
}

// RoutingPattern represents a task routing pattern.
type RoutingPattern struct {
	ID          string  `json:"id"`
	Trigger     string  `json:"trigger"`
	Action      string  `json:"action"`
	Confidence  float64 `json:"confidence"`
	UsageCount  int     `json:"usageCount"`
	SuccessRate float64 `json:"successRate"`
}

// ComplexityPattern represents a code complexity pattern.
type ComplexityPattern struct {
	ID              string  `json:"id"`
	Pattern         string  `json:"pattern"`
	ComplexityScore float64 `json:"complexityScore"`
	Tokens          int     `json:"tokens"`
	Frequency       int     `json:"frequency"`
}

// CoveragePattern represents a test coverage pattern.
type CoveragePattern struct {
	ID               string   `json:"id"`
	Domain           string   `json:"domain"`
	CoveragePercent  float64  `json:"coveragePercent"`
	Gaps             []string `json:"gaps"`
}

// TrajectoryPattern represents an execution trajectory pattern.
type TrajectoryPattern struct {
	ID        string   `json:"id"`
	Steps     []string `json:"steps"`
	Outcome   string   `json:"outcome"`
	Duration  int64    `json:"duration"`
	Learnings []string `json:"learnings"`
}

// CustomPattern represents a custom pattern type.
type CustomPattern struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata"`
}

// CFPStatistics holds pattern statistics with differential privacy.
type CFPStatistics struct {
	TotalPatterns int                `json:"totalPatterns"`
	AvgConfidence float64            `json:"avgConfidence"`
	PatternTypes  map[string]int     `json:"patternTypes"`
	TimeRange     CFPTimeRange       `json:"timeRange"`
}

// CFPTimeRange represents a time range.
type CFPTimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// CFPSignature holds the pattern signature.
type CFPSignature struct {
	Algorithm string `json:"algorithm"` // ed25519
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
}

// CFPIPFSInfo holds IPFS metadata.
type CFPIPFSInfo struct {
	CID      string   `json:"cid"`
	PinnedAt []string `json:"pinnedAt"`
	Gateway  string   `json:"gateway"`
	Size     int64    `json:"size"`
}

// NewCFPFormat creates a new CFP format with defaults.
func NewCFPFormat() *CFPFormat {
	return &CFPFormat{
		Magic:       "CFP1",
		Version:     "1.0.0",
		CreatedAt:   time.Now().Format(time.RFC3339),
		GeneratedBy: "claude-flow-go@3.0.0",
		Metadata: CFPMetadata{
			Tags: make([]string, 0),
		},
		Anonymization: CFPAnonymization{
			Level:             string(AnonymizationStrict),
			AppliedTransforms: make([]string, 0),
		},
		Patterns: CFPPatterns{
			Routing:    make([]RoutingPattern, 0),
			Complexity: make([]ComplexityPattern, 0),
			Coverage:   make([]CoveragePattern, 0),
			Trajectory: make([]TrajectoryPattern, 0),
			Custom:     make([]CustomPattern, 0),
		},
		Statistics: CFPStatistics{
			PatternTypes: make(map[string]int),
		},
	}
}

// TotalPatternCount returns the total number of patterns.
func (c *CFPFormat) TotalPatternCount() int {
	return len(c.Patterns.Routing) +
		len(c.Patterns.Complexity) +
		len(c.Patterns.Coverage) +
		len(c.Patterns.Trajectory) +
		len(c.Patterns.Custom)
}

// Validate validates the CFP format.
func (c *CFPFormat) Validate() error {
	if c.Magic != "CFP1" {
		return ErrInvalidMagic
	}
	if c.Version == "" {
		return ErrMissingVersion
	}
	if c.Metadata.Name == "" {
		return ErrMissingName
	}
	return nil
}
