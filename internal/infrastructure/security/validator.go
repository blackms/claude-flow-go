// Package security provides security infrastructure implementations.
package security

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// Validation errors.
var (
	ErrInputEmpty           = errors.New("input is empty")
	ErrInputTooLong         = errors.New("input exceeds maximum length")
	ErrInputTooShort        = errors.New("input below minimum length")
	ErrInvalidFormat        = errors.New("invalid format")
	ErrInvalidEmail         = errors.New("invalid email format")
	ErrInvalidURL           = errors.New("invalid URL format")
	ErrInvalidIdentifier    = errors.New("invalid identifier format")
	ErrInvalidFilename      = errors.New("invalid filename format")
	ErrShellMetacharacters  = errors.New("input contains shell metacharacters")
	ErrPathTraversal        = errors.New("input contains path traversal pattern")
	ErrNullByte             = errors.New("input contains null byte")
	ErrHTTPSRequired        = errors.New("URL must use HTTPS")
)

// Validation patterns.
var (
	// Safe identifier: alphanumeric with underscore/hyphen, starts with letter
	patternSafeIdentifier = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

	// Safe filename: alphanumeric with dot, underscore, hyphen
	patternSafeFilename = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

	// Safe path segment: no control characters or special chars
	patternSafePathSegment = regexp.MustCompile(`^[^<>:"|?*\x00-\x1f]+$`)

	// Shell metacharacters
	patternShellChars = regexp.MustCompile(`[;&|` + "`" + `$(){}><\n\r\x00]`)

	// Semantic version
	patternSemver = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

	// UUID
	patternUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

	// IPv4
	patternIPv4 = regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)
)

// Validation limits.
const (
	LimitMinPasswordLength  = 8
	LimitMaxPasswordLength  = 128
	LimitMaxEmailLength     = 254
	LimitMaxIdentifierLength = 64
	LimitMaxPathLength      = 4096
	LimitMaxContentLength   = 1024 * 1024 // 1MB
	LimitMaxArrayLength     = 1000
	LimitMaxFilenameLength  = 255
)

// ValidationResult contains the result of validation.
type ValidationResult struct {
	IsValid bool
	Errors  []error
	Value   string // Sanitized value if applicable
}

// InputValidator provides input validation capabilities.
type InputValidator struct {
	maxContentLength int
	maxPathLength    int
}

// NewInputValidator creates a new input validator.
func NewInputValidator() *InputValidator {
	return &InputValidator{
		maxContentLength: LimitMaxContentLength,
		maxPathLength:    LimitMaxPathLength,
	}
}

// ValidateSafeString validates a string doesn't contain shell metacharacters.
func (v *InputValidator) ValidateSafeString(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	if len(input) > v.maxContentLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: maximum %d characters", ErrInputTooLong, v.maxContentLength))
	}

	if patternShellChars.MatchString(input) {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrShellMetacharacters)
	}

	return result
}

// ValidateIdentifier validates an identifier (alphanumeric with underscore/hyphen).
func (v *InputValidator) ValidateIdentifier(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	if len(input) > LimitMaxIdentifierLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: maximum %d characters", ErrInputTooLong, LimitMaxIdentifierLength))
	}

	if !patternSafeIdentifier.MatchString(input) {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInvalidIdentifier)
	}

	return result
}

// ValidateFilename validates a filename.
func (v *InputValidator) ValidateFilename(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	if len(input) > LimitMaxFilenameLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: maximum %d characters", ErrInputTooLong, LimitMaxFilenameLength))
	}

	if !patternSafeFilename.MatchString(input) {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInvalidFilename)
	}

	// Check for traversal
	if strings.Contains(input, "..") {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrPathTraversal)
	}

	return result
}

// ValidateEmail validates an email address.
func (v *InputValidator) ValidateEmail(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: strings.ToLower(strings.TrimSpace(input))}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	if len(input) > LimitMaxEmailLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: maximum %d characters", ErrInputTooLong, LimitMaxEmailLength))
	}

	_, err := mail.ParseAddress(input)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInvalidEmail)
	}

	return result
}

// ValidatePassword validates a password.
func (v *InputValidator) ValidatePassword(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	if len(input) < LimitMinPasswordLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: minimum %d characters", ErrInputTooShort, LimitMinPasswordLength))
	}

	if len(input) > LimitMaxPasswordLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: maximum %d characters", ErrInputTooLong, LimitMaxPasswordLength))
	}

	hasUpper := false
	hasLower := false
	hasDigit := false

	for _, r := range input {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper {
		result.IsValid = false
		result.Errors = append(result.Errors, errors.New("password must contain uppercase letter"))
	}

	if !hasLower {
		result.IsValid = false
		result.Errors = append(result.Errors, errors.New("password must contain lowercase letter"))
	}

	if !hasDigit {
		result.IsValid = false
		result.Errors = append(result.Errors, errors.New("password must contain digit"))
	}

	return result
}

// ValidateURL validates a URL.
func (v *InputValidator) ValidateURL(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	parsed, err := url.Parse(input)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInvalidURL)
	}

	return result
}

// ValidateHTTPSURL validates a URL requiring HTTPS.
func (v *InputValidator) ValidateHTTPSURL(input string) ValidationResult {
	result := v.ValidateURL(input)
	if !result.IsValid {
		return result
	}

	if !strings.HasPrefix(strings.ToLower(input), "https://") {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrHTTPSRequired)
	}

	return result
}

// ValidateUUID validates a UUID.
func (v *InputValidator) ValidateUUID(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	if !patternUUID.MatchString(input) {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInvalidFormat)
	}

	return result
}

// ValidateSemver validates a semantic version.
func (v *InputValidator) ValidateSemver(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	if !patternSemver.MatchString(input) {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInvalidFormat)
	}

	return result
}

// ValidateIPv4 validates an IPv4 address.
func (v *InputValidator) ValidateIPv4(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	if !patternIPv4.MatchString(input) {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInvalidFormat)
	}

	return result
}

// ValidatePort validates a port number.
func (v *InputValidator) ValidatePort(port int) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0)}

	if port < 1 || port > 65535 {
		result.IsValid = false
		result.Errors = append(result.Errors, errors.New("port must be between 1 and 65535"))
	}

	return result
}

// ValidateCommandArgument validates a command argument.
func (v *InputValidator) ValidateCommandArgument(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if len(input) > 1024 {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: maximum 1024 characters", ErrInputTooLong))
	}

	if strings.Contains(input, "\x00") {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrNullByte)
	}

	if patternShellChars.MatchString(input) {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrShellMetacharacters)
	}

	return result
}

// ValidatePath validates a file path.
func (v *InputValidator) ValidatePath(input string) ValidationResult {
	result := ValidationResult{IsValid: true, Errors: make([]error, 0), Value: input}

	if input == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrInputEmpty)
		return result
	}

	if len(input) > v.maxPathLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: maximum %d characters", ErrInputTooLong, v.maxPathLength))
	}

	if strings.Contains(input, "\x00") {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrNullByte)
	}

	if strings.Contains(input, "..") {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrPathTraversal)
	}

	return result
}

// Sanitization functions

// SanitizeString removes dangerous characters from a string.
func SanitizeString(input string) string {
	result := strings.ReplaceAll(input, "\x00", "")
	result = strings.ReplaceAll(result, "<", "")
	result = strings.ReplaceAll(result, ">", "")
	result = regexp.MustCompile(`(?i)javascript:`).ReplaceAllString(result, "")
	result = regexp.MustCompile(`(?i)data:`).ReplaceAllString(result, "")
	return strings.TrimSpace(result)
}

// SanitizeHTML escapes HTML entities.
func SanitizeHTML(input string) string {
	result := strings.ReplaceAll(input, "&", "&amp;")
	result = strings.ReplaceAll(result, "<", "&lt;")
	result = strings.ReplaceAll(result, ">", "&gt;")
	result = strings.ReplaceAll(result, "\"", "&quot;")
	result = strings.ReplaceAll(result, "'", "&#x27;")
	return result
}

// SanitizePath removes traversal patterns from a path.
func SanitizePath(input string) string {
	result := strings.ReplaceAll(input, "\x00", "")
	result = strings.ReplaceAll(result, "..", "")
	result = regexp.MustCompile(`/+`).ReplaceAllString(result, "/")
	result = strings.TrimPrefix(result, "/")
	return strings.TrimSpace(result)
}

// SanitizeCommandArg removes shell metacharacters from a command argument.
func SanitizeCommandArg(input string) string {
	result := strings.ReplaceAll(input, "\x00", "")
	result = patternShellChars.ReplaceAllString(result, "")
	return result
}

// HasShellMetacharacters checks if input contains shell metacharacters.
func HasShellMetacharacters(input string) bool {
	return patternShellChars.MatchString(input)
}

// HasPathTraversal checks if input contains path traversal patterns.
func HasPathTraversal(input string) bool {
	if strings.Contains(input, "..") {
		return true
	}
	if strings.Contains(strings.ToLower(input), "%2e%2e") {
		return true
	}
	if strings.Contains(strings.ToLower(input), "%252e%252e") {
		return true
	}
	return false
}

// HasNullByte checks if input contains null bytes.
func HasNullByte(input string) bool {
	return strings.Contains(input, "\x00") || strings.Contains(strings.ToLower(input), "%00")
}
