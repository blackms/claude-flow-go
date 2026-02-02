// Package security provides security infrastructure implementations.
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Token errors.
var (
	ErrTokenLengthTooShort = errors.New("token length must be at least 16 bytes")
	ErrNoHMACSecret        = errors.New("HMAC secret required for signed tokens")
	ErrInvalidToken        = errors.New("invalid token")
	ErrTokenExpired        = errors.New("token has expired")
	ErrTokenMalformed      = errors.New("malformed token")
)

// TokenEncoding represents token encoding format.
type TokenEncoding string

const (
	TokenEncodingHex       TokenEncoding = "hex"
	TokenEncodingBase64    TokenEncoding = "base64"
	TokenEncodingBase64URL TokenEncoding = "base64url"
)

// TokenGeneratorConfig configures the token generator.
type TokenGeneratorConfig struct {
	// DefaultLength is the default token length in bytes (default: 32).
	DefaultLength int

	// Encoding is the token encoding format (default: base64url).
	Encoding TokenEncoding

	// HMACSecret is the secret for signed tokens.
	HMACSecret string

	// DefaultExpiration is the default expiration in seconds (default: 3600).
	DefaultExpiration int
}

// DefaultTokenGeneratorConfig returns the default configuration.
func DefaultTokenGeneratorConfig() TokenGeneratorConfig {
	return TokenGeneratorConfig{
		DefaultLength:     32,
		Encoding:          TokenEncodingBase64URL,
		HMACSecret:        "",
		DefaultExpiration: 3600,
	}
}

// Token represents a token with metadata.
type Token struct {
	Value     string                 `json:"value"`
	CreatedAt time.Time              `json:"createdAt"`
	ExpiresAt time.Time              `json:"expiresAt"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SignedToken represents an HMAC-signed token.
type SignedToken struct {
	Token     string    `json:"token"`
	Signature string    `json:"signature"`
	Combined  string    `json:"combined"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// VerificationCode represents a numeric verification code.
type VerificationCode struct {
	Code        string    `json:"code"`
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"maxAttempts"`
}

// TokenPair represents an access and refresh token pair.
type TokenPair struct {
	AccessToken  Token `json:"accessToken"`
	RefreshToken Token `json:"refreshToken"`
}

// TokenGenerator provides secure token generation.
type TokenGenerator struct {
	config TokenGeneratorConfig
}

// NewTokenGenerator creates a new token generator.
func NewTokenGenerator(config TokenGeneratorConfig) (*TokenGenerator, error) {
	if config.DefaultLength < 16 {
		return nil, ErrTokenLengthTooShort
	}

	return &TokenGenerator{
		config: config,
	}, nil
}

// NewDefaultTokenGenerator creates a token generator with default config.
func NewDefaultTokenGenerator() *TokenGenerator {
	gen, _ := NewTokenGenerator(DefaultTokenGeneratorConfig())
	return gen
}

// Generate generates a random token.
func (g *TokenGenerator) Generate(length int) (string, error) {
	if length <= 0 {
		length = g.config.DefaultLength
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return g.encode(bytes), nil
}

// GenerateWithExpiration generates a token with expiration.
func (g *TokenGenerator) GenerateWithExpiration(expirationSeconds int) (*Token, error) {
	if expirationSeconds <= 0 {
		expirationSeconds = g.config.DefaultExpiration
	}

	value, err := g.Generate(g.config.DefaultLength)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Token{
		Value:     value,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(expirationSeconds) * time.Second),
		Metadata:  make(map[string]interface{}),
	}, nil
}

// GenerateSessionToken generates a session token.
func (g *TokenGenerator) GenerateSessionToken() (*Token, error) {
	return g.GenerateWithExpiration(g.config.DefaultExpiration)
}

// GenerateCSRFToken generates a CSRF token (30 minutes expiration).
func (g *TokenGenerator) GenerateCSRFToken() (*Token, error) {
	return g.GenerateWithExpiration(1800) // 30 minutes
}

// GenerateAPIToken generates an API token with prefix.
func (g *TokenGenerator) GenerateAPIToken(prefix string) (*Token, error) {
	if prefix == "" {
		prefix = "cf_"
	}

	tokenBody, err := g.Generate(32)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Token{
		Value:     prefix + tokenBody,
		CreatedAt: now,
		ExpiresAt: now.Add(365 * 24 * time.Hour), // 1 year
		Metadata:  make(map[string]interface{}),
	}, nil
}

// GenerateVerificationCode generates a numeric verification code.
func (g *TokenGenerator) GenerateVerificationCode(length, expirationMinutes, maxAttempts int) (*VerificationCode, error) {
	if length <= 0 {
		length = 6
	}
	if expirationMinutes <= 0 {
		expirationMinutes = 10
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	code := ""
	for i := 0; i < length; i++ {
		code += fmt.Sprintf("%d", bytes[i]%10)
	}

	now := time.Now()
	return &VerificationCode{
		Code:        code,
		CreatedAt:   now,
		ExpiresAt:   now.Add(time.Duration(expirationMinutes) * time.Minute),
		Attempts:    0,
		MaxAttempts: maxAttempts,
	}, nil
}

// GenerateSignedToken generates an HMAC-signed token.
func (g *TokenGenerator) GenerateSignedToken(payload map[string]interface{}, expirationSeconds int) (*SignedToken, error) {
	if g.config.HMACSecret == "" {
		return nil, ErrNoHMACSecret
	}

	if expirationSeconds <= 0 {
		expirationSeconds = g.config.DefaultExpiration
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(expirationSeconds) * time.Second)

	// Create nonce
	nonce, err := g.Generate(8)
	if err != nil {
		return nil, err
	}

	tokenData := map[string]interface{}{
		"iat":   now.Unix(),
		"exp":   expiresAt.Unix(),
		"nonce": nonce,
	}

	// Merge payload
	for k, v := range payload {
		tokenData[k] = v
	}

	// Encode token
	tokenJSON, err := json.Marshal(tokenData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(tokenJSON)
	signature := g.sign(token)

	return &SignedToken{
		Token:     token,
		Signature: signature,
		Combined:  token + "." + signature,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

// VerifySignedToken verifies a signed token and returns the payload.
func (g *TokenGenerator) VerifySignedToken(combined string) (map[string]interface{}, error) {
	if g.config.HMACSecret == "" {
		return nil, ErrNoHMACSecret
	}

	// Split token and signature
	var token, signature string
	for i := len(combined) - 1; i >= 0; i-- {
		if combined[i] == '.' {
			token = combined[:i]
			signature = combined[i+1:]
			break
		}
	}

	if token == "" || signature == "" {
		return nil, ErrTokenMalformed
	}

	// Verify signature
	expectedSignature := g.sign(token)
	if !g.timingSafeCompare(signature, expectedSignature) {
		return nil, ErrInvalidToken
	}

	// Decode and parse payload
	tokenJSON, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrTokenMalformed
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(tokenJSON, &payload); err != nil {
		return nil, ErrTokenMalformed
	}

	// Check expiration
	if exp, ok := payload["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, ErrTokenExpired
		}
	}

	return payload, nil
}

// GenerateTokenPair generates an access and refresh token pair.
func (g *TokenGenerator) GenerateTokenPair() (*TokenPair, error) {
	accessToken, err := g.GenerateWithExpiration(900) // 15 minutes
	if err != nil {
		return nil, err
	}

	refreshToken, err := g.GenerateWithExpiration(604800) // 7 days
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  *accessToken,
		RefreshToken: *refreshToken,
	}, nil
}

// GeneratePasswordResetToken generates a password reset token (30 min).
func (g *TokenGenerator) GeneratePasswordResetToken() (*Token, error) {
	return g.GenerateWithExpiration(1800) // 30 minutes
}

// GenerateEmailVerificationToken generates an email verification token (24h).
func (g *TokenGenerator) GenerateEmailVerificationToken() (*Token, error) {
	return g.GenerateWithExpiration(86400) // 24 hours
}

// GenerateRequestID generates a unique request ID.
func (g *TokenGenerator) GenerateRequestID() (string, error) {
	return g.Generate(8)
}

// GenerateCorrelationID generates a correlation ID for distributed tracing.
func (g *TokenGenerator) GenerateCorrelationID() (string, error) {
	random, err := g.Generate(8)
	if err != nil {
		return "", err
	}

	timestamp := fmt.Sprintf("%x", time.Now().UnixNano())
	return timestamp + "-" + random, nil
}

// IsExpired checks if a token has expired.
func (g *TokenGenerator) IsExpired(token *Token) bool {
	return time.Now().After(token.ExpiresAt)
}

// IsVerificationCodeExpired checks if a verification code has expired.
func (g *TokenGenerator) IsVerificationCodeExpired(code *VerificationCode) bool {
	return time.Now().After(code.ExpiresAt)
}

// Compare compares two tokens in constant time.
func (g *TokenGenerator) Compare(a, b string) bool {
	return g.timingSafeCompare(a, b)
}

// sign creates an HMAC-SHA256 signature.
func (g *TokenGenerator) sign(data string) string {
	h := hmac.New(sha256.New, []byte(g.config.HMACSecret))
	h.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// timingSafeCompare compares two strings in constant time.
func (g *TokenGenerator) timingSafeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// encode encodes bytes according to the configured encoding.
func (g *TokenGenerator) encode(data []byte) string {
	switch g.config.Encoding {
	case TokenEncodingHex:
		return hex.EncodeToString(data)
	case TokenEncodingBase64:
		return base64.StdEncoding.EncodeToString(data)
	case TokenEncodingBase64URL:
		fallthrough
	default:
		return base64.RawURLEncoding.EncodeToString(data)
	}
}

// SetHMACSecret sets the HMAC secret for signed tokens.
func (g *TokenGenerator) SetHMACSecret(secret string) {
	g.config.HMACSecret = secret
}

// GenerateRandomBytes generates cryptographically secure random bytes.
func GenerateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return bytes, nil
}

// GenerateRandomHex generates a random hex string.
func GenerateRandomHex(length int) (string, error) {
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateRandomBase64URL generates a random base64url string.
func GenerateRandomBase64URL(length int) (string, error) {
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
