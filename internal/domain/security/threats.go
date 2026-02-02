// Package security provides domain types for the security module.
package security

import (
	"regexp"
	"strings"
)

// ThreatType represents a type of security threat.
type ThreatType string

const (
	ThreatXSS                ThreatType = "xss"
	ThreatSQLInjection       ThreatType = "sql_injection"
	ThreatCommandInjection   ThreatType = "command_injection"
	ThreatPathTraversal      ThreatType = "path_traversal"
	ThreatCredentialExposure ThreatType = "credential_exposure"
	ThreatCSRF               ThreatType = "csrf"
	ThreatSSRF               ThreatType = "ssrf"
	ThreatXXE                ThreatType = "xxe"
	ThreatInsecureDeserialize ThreatType = "insecure_deserialize"
)

// ThreatLevel represents the severity level of a threat.
type ThreatLevel string

const (
	ThreatLevelNone     ThreatLevel = "none"
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// ThreatDetection represents a detected threat.
type ThreatDetection struct {
	Type        ThreatType  `json:"type"`
	Level       ThreatLevel `json:"level"`
	Description string      `json:"description"`
	Pattern     string      `json:"pattern"`
	Input       string      `json:"input"`
	Position    int         `json:"position,omitempty"`
}

// ThreatPattern represents a pattern for detecting threats.
type ThreatPattern struct {
	Type        ThreatType
	Level       ThreatLevel
	Pattern     *regexp.Regexp
	Description string
}

// threatPatterns contains all threat detection patterns.
var threatPatterns = []ThreatPattern{
	// XSS patterns
	{
		Type:        ThreatXSS,
		Level:       ThreatLevelHigh,
		Pattern:     regexp.MustCompile(`(?i)<script[^>]*>`),
		Description: "Script tag detected",
	},
	{
		Type:        ThreatXSS,
		Level:       ThreatLevelHigh,
		Pattern:     regexp.MustCompile(`(?i)javascript:`),
		Description: "JavaScript protocol detected",
	},
	{
		Type:        ThreatXSS,
		Level:       ThreatLevelMedium,
		Pattern:     regexp.MustCompile(`(?i)on\w+\s*=`),
		Description: "Event handler attribute detected",
	},
	{
		Type:        ThreatXSS,
		Level:       ThreatLevelMedium,
		Pattern:     regexp.MustCompile(`(?i)data:`),
		Description: "Data protocol detected",
	},

	// SQL Injection patterns
	{
		Type:        ThreatSQLInjection,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`(?i)('|")\s*(or|and)\s*('|"|\d)`),
		Description: "SQL boolean injection pattern",
	},
	{
		Type:        ThreatSQLInjection,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`(?i)(union\s+select|select\s+.*\s+from|insert\s+into|delete\s+from|drop\s+table)`),
		Description: "SQL statement injection",
	},
	{
		Type:        ThreatSQLInjection,
		Level:       ThreatLevelHigh,
		Pattern:     regexp.MustCompile(`(?i)(--|#|/\*)`),
		Description: "SQL comment injection",
	},

	// Command Injection patterns
	{
		Type:        ThreatCommandInjection,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`[;&|` + "`" + `$(){}]`),
		Description: "Shell metacharacter detected",
	},
	{
		Type:        ThreatCommandInjection,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`\$\([^)]+\)`),
		Description: "Command substitution detected",
	},
	{
		Type:        ThreatCommandInjection,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`\$\{[^}]+\}`),
		Description: "Variable expansion detected",
	},

	// Path Traversal patterns
	{
		Type:        ThreatPathTraversal,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`\.\.(/|\\)`),
		Description: "Directory traversal pattern",
	},
	{
		Type:        ThreatPathTraversal,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`(?i)%2e%2e`),
		Description: "URL-encoded traversal pattern",
	},
	{
		Type:        ThreatPathTraversal,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`(?i)%252e%252e`),
		Description: "Double URL-encoded traversal pattern",
	},
	{
		Type:        ThreatPathTraversal,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`\x00`),
		Description: "Null byte injection",
	},

	// Credential Exposure patterns
	{
		Type:        ThreatCredentialExposure,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*['"]?[^\s'"]+`),
		Description: "Password in plaintext",
	},
	{
		Type:        ThreatCredentialExposure,
		Level:       ThreatLevelCritical,
		Pattern:     regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret[_-]?key)\s*[=:]\s*['"]?[a-zA-Z0-9]+`),
		Description: "API key in plaintext",
	},
	{
		Type:        ThreatCredentialExposure,
		Level:       ThreatLevelHigh,
		Pattern:     regexp.MustCompile(`(?i)(bearer|token)\s+[a-zA-Z0-9._-]+`),
		Description: "Token in plaintext",
	},
}

// ThreatDetector provides threat detection capabilities.
type ThreatDetector struct {
	patterns []ThreatPattern
}

// NewThreatDetector creates a new threat detector.
func NewThreatDetector() *ThreatDetector {
	return &ThreatDetector{
		patterns: threatPatterns,
	}
}

// Detect scans input for threats and returns all detections.
func (td *ThreatDetector) Detect(input string) []ThreatDetection {
	detections := make([]ThreatDetection, 0)

	for _, pattern := range td.patterns {
		if loc := pattern.Pattern.FindStringIndex(input); loc != nil {
			detections = append(detections, ThreatDetection{
				Type:        pattern.Type,
				Level:       pattern.Level,
				Description: pattern.Description,
				Pattern:     pattern.Pattern.String(),
				Input:       input,
				Position:    loc[0],
			})
		}
	}

	return detections
}

// DetectType scans input for a specific threat type.
func (td *ThreatDetector) DetectType(input string, threatType ThreatType) []ThreatDetection {
	detections := make([]ThreatDetection, 0)

	for _, pattern := range td.patterns {
		if pattern.Type != threatType {
			continue
		}
		if loc := pattern.Pattern.FindStringIndex(input); loc != nil {
			detections = append(detections, ThreatDetection{
				Type:        pattern.Type,
				Level:       pattern.Level,
				Description: pattern.Description,
				Pattern:     pattern.Pattern.String(),
				Input:       input,
				Position:    loc[0],
			})
		}
	}

	return detections
}

// HasThreats returns true if any threats are detected.
func (td *ThreatDetector) HasThreats(input string) bool {
	for _, pattern := range td.patterns {
		if pattern.Pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// GetHighestThreatLevel returns the highest threat level detected.
func (td *ThreatDetector) GetHighestThreatLevel(input string) ThreatLevel {
	highestLevel := ThreatLevelNone

	for _, pattern := range td.patterns {
		if pattern.Pattern.MatchString(input) {
			if compareThreatLevel(pattern.Level, highestLevel) > 0 {
				highestLevel = pattern.Level
			}
		}
	}

	return highestLevel
}

// compareThreatLevel compares two threat levels.
// Returns 1 if a > b, -1 if a < b, 0 if equal.
func compareThreatLevel(a, b ThreatLevel) int {
	levels := map[ThreatLevel]int{
		ThreatLevelNone:     0,
		ThreatLevelLow:      1,
		ThreatLevelMedium:   2,
		ThreatLevelHigh:     3,
		ThreatLevelCritical: 4,
	}

	aLevel := levels[a]
	bLevel := levels[b]

	if aLevel > bLevel {
		return 1
	}
	if aLevel < bLevel {
		return -1
	}
	return 0
}

// IsSafe returns true if no threats are detected above the given level.
func (td *ThreatDetector) IsSafe(input string, maxLevel ThreatLevel) bool {
	for _, pattern := range td.patterns {
		if pattern.Pattern.MatchString(input) {
			if compareThreatLevel(pattern.Level, maxLevel) > 0 {
				return false
			}
		}
	}
	return true
}

// SanitizeForLogging sanitizes input for safe logging.
func SanitizeForLogging(input string) string {
	// Truncate long inputs
	if len(input) > 200 {
		input = input[:200] + "..."
	}

	// Remove control characters
	result := strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' {
			return -1
		}
		return r
	}, input)

	// Mask potential credentials
	result = regexp.MustCompile(`(?i)(password|secret|key|token)\s*[=:]\s*['"]?[^\s'"]+`).
		ReplaceAllString(result, "$1=***REDACTED***")

	return result
}
