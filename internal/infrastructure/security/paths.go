// Package security provides security infrastructure implementations.
package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Path validation errors.
var (
	ErrPathEmpty            = errors.New("path is empty")
	ErrPathTooLong          = errors.New("path exceeds maximum length")
	ErrPathTraversalPattern = errors.New("path traversal pattern detected")
	ErrPathOutsideAllowed   = errors.New("path is outside allowed directories")
	ErrBlockedExtension     = errors.New("file extension is blocked")
	ErrBlockedFilename      = errors.New("file name is blocked")
	ErrHiddenNotAllowed     = errors.New("hidden files/directories are not allowed")
	ErrPathNotExist         = errors.New("path does not exist")
	ErrPathResolveError     = errors.New("failed to resolve path")
)

// Dangerous traversal patterns.
var traversalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\.\.(/|\\)`),         // ../ or ..\
	regexp.MustCompile(`\.\.`),               // .. anywhere
	regexp.MustCompile(`(?i)%2e%2e`),         // URL-encoded ..
	regexp.MustCompile(`(?i)%252e%252e`),     // Double URL-encoded ..
	regexp.MustCompile(`(?i)\.%2e`),          // Mixed encoding
	regexp.MustCompile(`(?i)%2e\.`),          // Mixed encoding
	regexp.MustCompile(`\x00`),               // Null byte
	regexp.MustCompile(`(?i)%00`),            // URL-encoded null
}

// Default blocked file extensions (sensitive files).
var defaultBlockedExtensions = []string{
	".env",
	".pem",
	".key",
	".crt",
	".pfx",
	".p12",
	".jks",
	".keystore",
	".secret",
	".credentials",
}

// Default blocked file names (sensitive files).
var defaultBlockedNames = []string{
	"id_rsa",
	"id_dsa",
	"id_ecdsa",
	"id_ed25519",
	".htpasswd",
	".htaccess",
	"shadow",
	"passwd",
	"authorized_keys",
	"known_hosts",
	".git",
	".gitconfig",
	".npmrc",
	".docker",
}

// PathValidatorConfig configures the path validator.
type PathValidatorConfig struct {
	// AllowedPrefixes are the allowed directory prefixes.
	AllowedPrefixes []string

	// BlockedExtensions are blocked file extensions.
	BlockedExtensions []string

	// BlockedNames are blocked file names.
	BlockedNames []string

	// MaxPathLength is the maximum path length (default: 4096).
	MaxPathLength int

	// ResolveSymlinks resolves symlinks (default: true).
	ResolveSymlinks bool

	// AllowNonExistent allows paths that don't exist (default: true).
	AllowNonExistent bool

	// AllowHidden allows hidden files/directories (default: false).
	AllowHidden bool
}

// DefaultPathValidatorConfig returns the default configuration.
func DefaultPathValidatorConfig(allowedPrefixes []string) PathValidatorConfig {
	return PathValidatorConfig{
		AllowedPrefixes:   allowedPrefixes,
		BlockedExtensions: defaultBlockedExtensions,
		BlockedNames:      defaultBlockedNames,
		MaxPathLength:     4096,
		ResolveSymlinks:   true,
		AllowNonExistent:  true,
		AllowHidden:       false,
	}
}

// PathValidationResult contains the result of path validation.
type PathValidationResult struct {
	IsValid       bool
	ResolvedPath  string
	RelativePath  string
	MatchedPrefix string
	Errors        []error
}

// PathValidator provides path traversal prevention.
type PathValidator struct {
	config           PathValidatorConfig
	resolvedPrefixes []string
}

// NewPathValidator creates a new path validator.
func NewPathValidator(config PathValidatorConfig) (*PathValidator, error) {
	if len(config.AllowedPrefixes) == 0 {
		return nil, errors.New("at least one allowed prefix must be specified")
	}

	// Pre-resolve all prefixes
	resolvedPrefixes := make([]string, 0, len(config.AllowedPrefixes))
	for _, prefix := range config.AllowedPrefixes {
		resolved, err := filepath.Abs(prefix)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve prefix %s: %w", prefix, err)
		}
		resolvedPrefixes = append(resolvedPrefixes, resolved)
	}

	return &PathValidator{
		config:           config,
		resolvedPrefixes: resolvedPrefixes,
	}, nil
}

// Validate validates a path against security rules.
func (v *PathValidator) Validate(inputPath string) PathValidationResult {
	result := PathValidationResult{
		IsValid: true,
		Errors:  make([]error, 0),
	}

	// Check for empty path
	if inputPath == "" || strings.TrimSpace(inputPath) == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrPathEmpty)
		return result
	}

	// Check path length
	if len(inputPath) > v.config.MaxPathLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: maximum %d characters", ErrPathTooLong, v.config.MaxPathLength))
		return result
	}

	// Check for traversal patterns
	for _, pattern := range traversalPatterns {
		if pattern.MatchString(inputPath) {
			result.IsValid = false
			result.Errors = append(result.Errors, ErrPathTraversalPattern)
			return result
		}
	}

	// Resolve the path
	resolvedPath, err := filepath.Abs(inputPath)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: %v", ErrPathResolveError, err))
		return result
	}

	// Optionally resolve symlinks
	if v.config.ResolveSymlinks {
		realPath, err := filepath.EvalSymlinks(resolvedPath)
		if err != nil {
			if os.IsNotExist(err) {
				if !v.config.AllowNonExistent {
					result.IsValid = false
					result.Errors = append(result.Errors, ErrPathNotExist)
					return result
				}
				// Path doesn't exist yet, use resolved path
			} else {
				result.IsValid = false
				result.Errors = append(result.Errors, fmt.Errorf("%w: %v", ErrPathResolveError, err))
				return result
			}
		} else {
			resolvedPath = realPath
		}
	}

	result.ResolvedPath = resolvedPath

	// Check against allowed prefixes
	prefixMatched := false
	for _, prefix := range v.resolvedPrefixes {
		if resolvedPath == prefix || strings.HasPrefix(resolvedPath, prefix+string(filepath.Separator)) {
			prefixMatched = true
			result.MatchedPrefix = prefix
			result.RelativePath = strings.TrimPrefix(resolvedPath, prefix)
			result.RelativePath = strings.TrimPrefix(result.RelativePath, string(filepath.Separator))
			break
		}
	}

	if !prefixMatched {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrPathOutsideAllowed)
		return result
	}

	// Check for hidden files
	if !v.config.AllowHidden {
		parts := strings.Split(resolvedPath, string(filepath.Separator))
		for _, part := range parts {
			if strings.HasPrefix(part, ".") && part != "." && part != ".." {
				result.IsValid = false
				result.Errors = append(result.Errors, ErrHiddenNotAllowed)
				break
			}
		}
	}

	// Check blocked file names
	basename := filepath.Base(resolvedPath)
	for _, blocked := range v.config.BlockedNames {
		if basename == blocked {
			result.IsValid = false
			result.Errors = append(result.Errors, fmt.Errorf("%w: %s", ErrBlockedFilename, basename))
			break
		}
	}

	// Check blocked extensions
	ext := strings.ToLower(filepath.Ext(resolvedPath))
	for _, blocked := range v.config.BlockedExtensions {
		if ext == blocked {
			result.IsValid = false
			result.Errors = append(result.Errors, fmt.Errorf("%w: %s", ErrBlockedExtension, ext))
			break
		}
	}

	// Also check for double extensions (e.g., .config.json)
	lowerBasename := strings.ToLower(basename)
	for _, blocked := range v.config.BlockedExtensions {
		if strings.HasSuffix(lowerBasename, blocked) && ext != blocked {
			result.IsValid = false
			result.Errors = append(result.Errors, fmt.Errorf("%w: %s", ErrBlockedExtension, blocked))
			break
		}
	}

	return result
}

// ValidateOrError validates and returns the resolved path, or an error.
func (v *PathValidator) ValidateOrError(inputPath string) (string, error) {
	result := v.Validate(inputPath)
	if !result.IsValid {
		if len(result.Errors) > 0 {
			return "", result.Errors[0]
		}
		return "", ErrPathOutsideAllowed
	}
	return result.ResolvedPath, nil
}

// IsWithinAllowed quickly checks if a path is within allowed prefixes.
func (v *PathValidator) IsWithinAllowed(inputPath string) bool {
	resolved, err := filepath.Abs(inputPath)
	if err != nil {
		return false
	}

	for _, prefix := range v.resolvedPrefixes {
		if resolved == prefix || strings.HasPrefix(resolved, prefix+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// SecureJoin securely joins path segments within allowed directories.
func (v *PathValidator) SecureJoin(prefix string, segments ...string) (string, error) {
	joined := filepath.Join(append([]string{prefix}, segments...)...)
	return v.ValidateOrError(joined)
}

// AddPrefix adds a prefix to the allowed list.
func (v *PathValidator) AddPrefix(prefix string) error {
	resolved, err := filepath.Abs(prefix)
	if err != nil {
		return fmt.Errorf("failed to resolve prefix: %w", err)
	}

	// Check if already present
	for _, p := range v.resolvedPrefixes {
		if p == resolved {
			return nil
		}
	}

	v.config.AllowedPrefixes = append(v.config.AllowedPrefixes, prefix)
	v.resolvedPrefixes = append(v.resolvedPrefixes, resolved)
	return nil
}

// GetAllowedPrefixes returns the resolved allowed prefixes.
func (v *PathValidator) GetAllowedPrefixes() []string {
	result := make([]string, len(v.resolvedPrefixes))
	copy(result, v.resolvedPrefixes)
	return result
}

// HasTraversalPattern checks if a path contains traversal patterns.
func HasTraversalPattern(path string) bool {
	for _, pattern := range traversalPatterns {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

// CleanPath cleans a path and removes dangerous patterns.
func CleanPath(inputPath string) string {
	// Remove null bytes
	result := strings.ReplaceAll(inputPath, "\x00", "")

	// Normalize slashes
	result = filepath.Clean(result)

	// Remove URL-encoded patterns
	result = regexp.MustCompile(`(?i)%2e`).ReplaceAllString(result, ".")
	result = regexp.MustCompile(`(?i)%2f`).ReplaceAllString(result, "/")
	result = regexp.MustCompile(`(?i)%5c`).ReplaceAllString(result, "\\")

	// Clean again after normalization
	result = filepath.Clean(result)

	return result
}

// CreateProjectPathValidator creates a path validator for a project directory.
func CreateProjectPathValidator(projectRoot string) (*PathValidator, error) {
	srcDir := filepath.Join(projectRoot, "src")
	testDir := filepath.Join(projectRoot, "tests")
	docsDir := filepath.Join(projectRoot, "docs")

	config := PathValidatorConfig{
		AllowedPrefixes:   []string{srcDir, testDir, docsDir},
		BlockedExtensions: defaultBlockedExtensions,
		BlockedNames:      defaultBlockedNames,
		MaxPathLength:     4096,
		ResolveSymlinks:   true,
		AllowNonExistent:  true,
		AllowHidden:       false,
	}

	return NewPathValidator(config)
}

// CreateFullProjectPathValidator creates a path validator for an entire project.
func CreateFullProjectPathValidator(projectRoot string) (*PathValidator, error) {
	blockedNames := append([]string{}, defaultBlockedNames...)
	blockedNames = append(blockedNames, "node_modules")

	config := PathValidatorConfig{
		AllowedPrefixes:   []string{projectRoot},
		BlockedExtensions: defaultBlockedExtensions,
		BlockedNames:      blockedNames,
		MaxPathLength:     4096,
		ResolveSymlinks:   true,
		AllowNonExistent:  true,
		AllowHidden:       true, // Allow .gitignore, etc.
	}

	return NewPathValidator(config)
}
