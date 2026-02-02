// Package security provides security infrastructure implementations.
package security

import (
	"errors"
	"fmt"
	"regexp"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher errors.
var (
	ErrInvalidRounds     = errors.New("bcrypt rounds must be between 10 and 20")
	ErrInvalidMinLength  = errors.New("minimum password length must be at least 8")
	ErrPasswordTooShort  = errors.New("password is too short")
	ErrPasswordTooLong   = errors.New("password is too long")
	ErrPasswordNoUpper   = errors.New("password must contain uppercase letter")
	ErrPasswordNoLower   = errors.New("password must contain lowercase letter")
	ErrPasswordNoDigit   = errors.New("password must contain digit")
	ErrPasswordNoSpecial = errors.New("password must contain special character")
	ErrPasswordRequired  = errors.New("password is required")
	ErrHashFailed        = errors.New("failed to hash password")
	ErrInvalidHash       = errors.New("invalid bcrypt hash format")
)

// PasswordHasherConfig configures the password hasher.
type PasswordHasherConfig struct {
	// Rounds is the bcrypt cost factor (10-20, default: 12).
	Rounds int

	// MinLength is the minimum password length (default: 8).
	MinLength int

	// MaxLength is the maximum password length (default: 128).
	MaxLength int

	// RequireUppercase requires at least one uppercase letter (default: true).
	RequireUppercase bool

	// RequireLowercase requires at least one lowercase letter (default: true).
	RequireLowercase bool

	// RequireDigit requires at least one digit (default: true).
	RequireDigit bool

	// RequireSpecial requires at least one special character (default: false).
	RequireSpecial bool
}

// DefaultPasswordHasherConfig returns the default configuration.
func DefaultPasswordHasherConfig() PasswordHasherConfig {
	return PasswordHasherConfig{
		Rounds:           12,
		MinLength:        8,
		MaxLength:        128,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   false,
	}
}

// PasswordValidationResult contains the result of password validation.
type PasswordValidationResult struct {
	IsValid bool
	Errors  []error
}

// PasswordHasher provides secure password hashing using bcrypt.
type PasswordHasher struct {
	config PasswordHasherConfig
}

// NewPasswordHasher creates a new password hasher.
func NewPasswordHasher(config PasswordHasherConfig) (*PasswordHasher, error) {
	if config.Rounds < 10 || config.Rounds > 20 {
		return nil, ErrInvalidRounds
	}

	if config.MinLength < 8 {
		return nil, ErrInvalidMinLength
	}

	return &PasswordHasher{
		config: config,
	}, nil
}

// NewDefaultPasswordHasher creates a password hasher with default config.
func NewDefaultPasswordHasher() *PasswordHasher {
	hasher, _ := NewPasswordHasher(DefaultPasswordHasherConfig())
	return hasher
}

// Validate validates a password against configured requirements.
func (h *PasswordHasher) Validate(password string) PasswordValidationResult {
	result := PasswordValidationResult{
		IsValid: true,
		Errors:  make([]error, 0),
	}

	if password == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrPasswordRequired)
		return result
	}

	if len(password) < h.config.MinLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: minimum %d characters", ErrPasswordTooShort, h.config.MinLength))
	}

	if len(password) > h.config.MaxLength {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: maximum %d characters", ErrPasswordTooLong, h.config.MaxLength))
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if h.config.RequireUppercase && !hasUpper {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrPasswordNoUpper)
	}

	if h.config.RequireLowercase && !hasLower {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrPasswordNoLower)
	}

	if h.config.RequireDigit && !hasDigit {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrPasswordNoDigit)
	}

	if h.config.RequireSpecial && !hasSpecial {
		result.IsValid = false
		result.Errors = append(result.Errors, ErrPasswordNoSpecial)
	}

	return result
}

// Hash hashes a password using bcrypt.
func (h *PasswordHasher) Hash(password string) (string, error) {
	validation := h.Validate(password)
	if !validation.IsValid {
		if len(validation.Errors) > 0 {
			return "", validation.Errors[0]
		}
		return "", ErrPasswordRequired
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.config.Rounds)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrHashFailed, err)
	}

	return string(hash), nil
}

// Verify verifies a password against a bcrypt hash.
// Uses timing-safe comparison internally.
func (h *PasswordHasher) Verify(password, hash string) bool {
	if password == "" || hash == "" {
		return false
	}

	if !h.isValidBcryptHash(hash) {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// NeedsRehash checks if a hash needs to be rehashed with updated parameters.
func (h *PasswordHasher) NeedsRehash(hash string) bool {
	if !h.isValidBcryptHash(hash) {
		return true
	}

	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return true
	}

	return cost < h.config.Rounds
}

// isValidBcryptHash validates bcrypt hash format.
func (h *PasswordHasher) isValidBcryptHash(hash string) bool {
	// bcrypt hash format: $2a$XX$22charsSalt31charsHash
	// Total length: 60 characters
	matched, _ := regexp.MatchString(`^\$2[aby]\$\d{2}\$[./A-Za-z0-9]{53}$`, hash)
	return matched
}

// GetConfig returns the current configuration.
func (h *PasswordHasher) GetConfig() PasswordHasherConfig {
	return h.config
}

// HashWithoutValidation hashes a password without validation.
// Use with caution - prefer Hash() for user passwords.
func (h *PasswordHasher) HashWithoutValidation(password string) (string, error) {
	if password == "" {
		return "", ErrPasswordRequired
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.config.Rounds)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrHashFailed, err)
	}

	return string(hash), nil
}

// GetHashCost returns the cost factor used for a hash.
func GetHashCost(hash string) (int, error) {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return 0, ErrInvalidHash
	}
	return cost, nil
}
