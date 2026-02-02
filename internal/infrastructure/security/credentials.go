// Package security provides security infrastructure implementations.
package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Credential errors.
var (
	ErrInvalidPasswordLength = errors.New("password length must be at least 16 characters")
	ErrInvalidAPIKeyLength   = errors.New("API key length must be at least 32 characters")
	ErrInvalidSecretLength   = errors.New("secret length must be at least 32 characters")
)

// Character sets for credential generation.
const (
	CharsUppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	CharsLowercase = "abcdefghijklmnopqrstuvwxyz"
	CharsDigits    = "0123456789"
	CharsSpecial   = "!@#$%^&*()_+-=[]{}|;:,.<>?"
	CharsURLSafe   = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	CharsHex       = "0123456789abcdef"
)

// CredentialGeneratorConfig configures the credential generator.
type CredentialGeneratorConfig struct {
	// PasswordLength is the length of generated passwords (default: 32).
	PasswordLength int

	// APIKeyLength is the length of generated API keys (default: 48).
	APIKeyLength int

	// SecretLength is the length of generated secrets (default: 64).
	SecretLength int

	// PasswordCharset is the character set for passwords.
	PasswordCharset string

	// APIKeyCharset is the character set for API keys.
	APIKeyCharset string
}

// DefaultCredentialGeneratorConfig returns the default configuration.
func DefaultCredentialGeneratorConfig() CredentialGeneratorConfig {
	return CredentialGeneratorConfig{
		PasswordLength:  32,
		APIKeyLength:    48,
		SecretLength:    64,
		PasswordCharset: CharsUppercase + CharsLowercase + CharsDigits + CharsSpecial,
		APIKeyCharset:   CharsURLSafe,
	}
}

// GeneratedCredentials contains a complete set of installation credentials.
type GeneratedCredentials struct {
	AdminPassword   string     `json:"adminPassword"`
	ServicePassword string     `json:"servicePassword"`
	JWTSecret       string     `json:"jwtSecret"`
	SessionSecret   string     `json:"sessionSecret"`
	EncryptionKey   string     `json:"encryptionKey"`
	GeneratedAt     time.Time  `json:"generatedAt"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
}

// APIKeyCredential contains API key information.
type APIKeyCredential struct {
	Key       string    `json:"key"`
	Prefix    string    `json:"prefix"`
	KeyID     string    `json:"keyId"`
	CreatedAt time.Time `json:"createdAt"`
}

// CredentialGenerator provides secure credential generation.
type CredentialGenerator struct {
	config CredentialGeneratorConfig
}

// NewCredentialGenerator creates a new credential generator.
func NewCredentialGenerator(config CredentialGeneratorConfig) (*CredentialGenerator, error) {
	if config.PasswordLength < 16 {
		return nil, ErrInvalidPasswordLength
	}

	if config.APIKeyLength < 32 {
		return nil, ErrInvalidAPIKeyLength
	}

	if config.SecretLength < 32 {
		return nil, ErrInvalidSecretLength
	}

	return &CredentialGenerator{
		config: config,
	}, nil
}

// NewDefaultCredentialGenerator creates a credential generator with default config.
func NewDefaultCredentialGenerator() *CredentialGenerator {
	gen, _ := NewCredentialGenerator(DefaultCredentialGeneratorConfig())
	return gen
}

// generateSecureString generates a cryptographically secure random string
// using rejection sampling to eliminate modulo bias.
func (g *CredentialGenerator) generateSecureString(length int, charset string) (string, error) {
	charsetLen := len(charset)
	if charsetLen == 0 {
		return "", errors.New("charset cannot be empty")
	}

	result := make([]byte, length)

	// Calculate rejection threshold to eliminate modulo bias
	maxValidValue := 256 - (256 % charsetLen)

	i := 0
	for i < length {
		// Generate random bytes
		bufSize := length - i
		if bufSize < 16 {
			bufSize = 16
		}
		randomBuffer := make([]byte, bufSize)
		if _, err := rand.Read(randomBuffer); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}

		for j := 0; j < len(randomBuffer) && i < length; j++ {
			randomValue := int(randomBuffer[j])

			// Rejection sampling: only accept values below threshold
			if randomValue < maxValidValue {
				result[i] = charset[randomValue%charsetLen]
				i++
			}
		}
	}

	return string(result), nil
}

// GeneratePassword generates a secure random password.
func (g *CredentialGenerator) GeneratePassword(length int) (string, error) {
	if length <= 0 {
		length = g.config.PasswordLength
	}

	for attempts := 0; attempts < 10; attempts++ {
		password, err := g.generateSecureString(length, g.config.PasswordCharset)
		if err != nil {
			return "", err
		}

		if g.hasRequiredCharacterTypes(password) {
			return password, nil
		}
	}

	// Fallback: generate with guaranteed character types
	return g.generatePasswordWithRequirements(length)
}

// generatePasswordWithRequirements generates a password guaranteed to have all requirements.
func (g *CredentialGenerator) generatePasswordWithRequirements(length int) (string, error) {
	if length < 4 {
		length = g.config.PasswordLength
	}

	result := make([]byte, length)

	// Guarantee at least one of each type
	upper, _ := g.generateSecureString(1, CharsUppercase)
	lower, _ := g.generateSecureString(1, CharsLowercase)
	digit, _ := g.generateSecureString(1, CharsDigits)
	special, _ := g.generateSecureString(1, CharsSpecial)

	result[0] = upper[0]
	result[1] = lower[0]
	result[2] = digit[0]
	result[3] = special[0]

	// Fill the rest
	remaining, err := g.generateSecureString(length-4, g.config.PasswordCharset)
	if err != nil {
		return "", err
	}

	for i := 4; i < length; i++ {
		result[i] = remaining[i-4]
	}

	// Shuffle the result
	return g.shuffleString(string(result))
}

// shuffleString shuffles a string using Fisher-Yates algorithm.
func (g *CredentialGenerator) shuffleString(s string) (string, error) {
	runes := []rune(s)
	n := len(runes)

	for i := n - 1; i > 0; i-- {
		randBytes := make([]byte, 1)
		if _, err := rand.Read(randBytes); err != nil {
			return "", err
		}
		j := int(randBytes[0]) % (i + 1)
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes), nil
}

// hasRequiredCharacterTypes checks if password has required character types.
func (g *CredentialGenerator) hasRequiredCharacterTypes(password string) bool {
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

	return hasUpper && hasLower && hasDigit && hasSpecial
}

// GenerateAPIKey generates a secure API key.
func (g *CredentialGenerator) GenerateAPIKey(prefix string) (*APIKeyCredential, error) {
	if prefix == "" {
		prefix = "cf_"
	}

	bodyLength := g.config.APIKeyLength - len(prefix)
	if bodyLength < 20 {
		bodyLength = 20
	}

	keyBody, err := g.generateSecureString(bodyLength, g.config.APIKeyCharset)
	if err != nil {
		return nil, err
	}

	return &APIKeyCredential{
		Key:       prefix + keyBody,
		Prefix:    prefix,
		KeyID:     uuid.New().String(),
		CreatedAt: time.Now(),
	}, nil
}

// GenerateSecret generates a secure secret (hex-encoded).
func (g *CredentialGenerator) GenerateSecret(length int) (string, error) {
	if length <= 0 {
		length = g.config.SecretLength
	}

	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return hex.EncodeToString(bytes)[:length], nil
}

// GenerateEncryptionKey generates a 32-byte key for AES-256 (64 hex chars).
func (g *CredentialGenerator) GenerateEncryptionKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateInstallationCredentials generates a complete set of installation credentials.
func (g *CredentialGenerator) GenerateInstallationCredentials(expirationDays int) (*GeneratedCredentials, error) {
	adminPassword, err := g.GeneratePassword(0)
	if err != nil {
		return nil, fmt.Errorf("failed to generate admin password: %w", err)
	}

	servicePassword, err := g.GeneratePassword(0)
	if err != nil {
		return nil, fmt.Errorf("failed to generate service password: %w", err)
	}

	jwtSecret, err := g.GenerateSecret(64)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
	}

	sessionSecret, err := g.GenerateSecret(64)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session secret: %w", err)
	}

	encryptionKey, err := g.GenerateEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	now := time.Now()
	var expiresAt *time.Time
	if expirationDays > 0 {
		t := now.Add(time.Duration(expirationDays) * 24 * time.Hour)
		expiresAt = &t
	}

	return &GeneratedCredentials{
		AdminPassword:   adminPassword,
		ServicePassword: servicePassword,
		JWTSecret:       jwtSecret,
		SessionSecret:   sessionSecret,
		EncryptionKey:   encryptionKey,
		GeneratedAt:     now,
		ExpiresAt:       expiresAt,
	}, nil
}

// GenerateSessionToken generates a URL-safe session token.
func (g *CredentialGenerator) GenerateSessionToken() (string, error) {
	return g.generateSecureString(64, CharsURLSafe)
}

// GenerateCSRFToken generates a CSRF token.
func (g *CredentialGenerator) GenerateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateNonce generates a unique nonce.
func (g *CredentialGenerator) GenerateNonce() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateEnvScript creates an environment variable export script.
func (g *CredentialGenerator) CreateEnvScript(creds *GeneratedCredentials) string {
	script := fmt.Sprintf(`# Claude Flow - Generated Credentials
# Generated: %s
# IMPORTANT: Store these securely and delete this file after use

export CLAUDE_FLOW_ADMIN_PASSWORD="%s"
export CLAUDE_FLOW_SERVICE_PASSWORD="%s"
export CLAUDE_FLOW_JWT_SECRET="%s"
export CLAUDE_FLOW_SESSION_SECRET="%s"
export CLAUDE_FLOW_ENCRYPTION_KEY="%s"
`,
		creds.GeneratedAt.Format(time.RFC3339),
		creds.AdminPassword,
		creds.ServicePassword,
		creds.JWTSecret,
		creds.SessionSecret,
		creds.EncryptionKey,
	)
	return script
}

// CreateJSONConfig creates a JSON configuration for secrets manager.
func (g *CredentialGenerator) CreateJSONConfig(creds *GeneratedCredentials) string {
	expiresAt := ""
	if creds.ExpiresAt != nil {
		expiresAt = creds.ExpiresAt.Format(time.RFC3339)
	}

	return fmt.Sprintf(`{
  "claude-flow/admin-password": "%s",
  "claude-flow/service-password": "%s",
  "claude-flow/jwt-secret": "%s",
  "claude-flow/session-secret": "%s",
  "claude-flow/encryption-key": "%s",
  "claude-flow/generated-at": "%s",
  "claude-flow/expires-at": "%s"
}`,
		creds.AdminPassword,
		creds.ServicePassword,
		creds.JWTSecret,
		creds.SessionSecret,
		creds.EncryptionKey,
		creds.GeneratedAt.Format(time.RFC3339),
		expiresAt,
	)
}
