// Package security provides application services for the security module.
package security

import (
	"context"
	"fmt"
	"sync"
	"time"

	domainSecurity "github.com/anthropics/claude-flow-go/internal/domain/security"
	infraSecurity "github.com/anthropics/claude-flow-go/internal/infrastructure/security"
)

// SecurityServiceConfig configures the security service.
type SecurityServiceConfig struct {
	// BCryptRounds is the bcrypt cost factor (default: 12).
	BCryptRounds int

	// JWTExpiration is the JWT token expiration in seconds (default: 3600).
	JWTExpiration int

	// SessionTimeout is the session timeout in seconds (default: 3600).
	SessionTimeout int

	// MaxLoginAttempts is the maximum failed login attempts (default: 5).
	MaxLoginAttempts int

	// LockoutDuration is the account lockout duration (default: 15 minutes).
	LockoutDuration time.Duration

	// HMACSecret is the secret for signed tokens.
	HMACSecret string

	// AllowedPaths are the allowed path prefixes.
	AllowedPaths []string

	// AllowedCommands are the allowed commands for execution.
	AllowedCommands []string
}

// DefaultSecurityServiceConfig returns the default configuration.
func DefaultSecurityServiceConfig() SecurityServiceConfig {
	return SecurityServiceConfig{
		BCryptRounds:     12,
		JWTExpiration:    3600,
		SessionTimeout:   3600,
		MaxLoginAttempts: 5,
		LockoutDuration:  15 * time.Minute,
		HMACSecret:       "",
		AllowedPaths:     []string{},
		AllowedCommands:  []string{"git", "npm", "node", "go"},
	}
}

// AuditEvent represents a security audit event.
type AuditEvent struct {
	Timestamp   time.Time                  `json:"timestamp"`
	EventType   string                     `json:"eventType"`
	UserID      string                     `json:"userId,omitempty"`
	SessionID   string                     `json:"sessionId,omitempty"`
	Action      string                     `json:"action"`
	Resource    string                     `json:"resource,omitempty"`
	Result      string                     `json:"result"`
	ThreatLevel domainSecurity.ThreatLevel `json:"threatLevel,omitempty"`
	IPAddress   string                     `json:"ipAddress,omitempty"`
	Details     map[string]interface{}     `json:"details,omitempty"`
}

// SecurityService provides a unified API for security operations.
type SecurityService struct {
	mu sync.RWMutex

	config           SecurityServiceConfig
	hasher           *infraSecurity.PasswordHasher
	tokenGenerator   *infraSecurity.TokenGenerator
	credentialGen    *infraSecurity.CredentialGenerator
	inputValidator   *infraSecurity.InputValidator
	pathValidator    *infraSecurity.PathValidator
	executor         *infraSecurity.SafeExecutor
	threatDetector   *domainSecurity.ThreatDetector

	// Context storage
	contexts map[string]*domainSecurity.SecurityContext

	// Audit log
	auditLog []AuditEvent
}

// NewSecurityService creates a new security service.
func NewSecurityService(config SecurityServiceConfig) (*SecurityService, error) {
	// Create password hasher
	hasherConfig := infraSecurity.PasswordHasherConfig{
		Rounds:           config.BCryptRounds,
		MinLength:        8,
		MaxLength:        128,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   false,
	}
	hasher, err := infraSecurity.NewPasswordHasher(hasherConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create password hasher: %w", err)
	}

	// Create token generator
	tokenConfig := infraSecurity.TokenGeneratorConfig{
		DefaultLength:     32,
		Encoding:          infraSecurity.TokenEncodingBase64URL,
		HMACSecret:        config.HMACSecret,
		DefaultExpiration: config.JWTExpiration,
	}
	tokenGen, err := infraSecurity.NewTokenGenerator(tokenConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create token generator: %w", err)
	}

	// Create credential generator
	credentialGen := infraSecurity.NewDefaultCredentialGenerator()

	// Create input validator
	inputValidator := infraSecurity.NewInputValidator()

	// Create path validator if paths are configured
	var pathValidator *infraSecurity.PathValidator
	if len(config.AllowedPaths) > 0 {
		pathConfig := infraSecurity.DefaultPathValidatorConfig(config.AllowedPaths)
		pv, err := infraSecurity.NewPathValidator(pathConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create path validator: %w", err)
		}
		pathValidator = pv
	}

	// Create executor if commands are configured
	var executor *infraSecurity.SafeExecutor
	if len(config.AllowedCommands) > 0 {
		execConfig := infraSecurity.DefaultSafeExecutorConfig(config.AllowedCommands)
		ex, err := infraSecurity.NewSafeExecutor(execConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create executor: %w", err)
		}
		executor = ex
	}

	// Create threat detector
	threatDetector := domainSecurity.NewThreatDetector()

	return &SecurityService{
		config:          config,
		hasher:          hasher,
		tokenGenerator:  tokenGen,
		credentialGen:   credentialGen,
		inputValidator:  inputValidator,
		pathValidator:   pathValidator,
		executor:        executor,
		threatDetector:  threatDetector,
		contexts:        make(map[string]*domainSecurity.SecurityContext),
		auditLog:        make([]AuditEvent, 0),
	}, nil
}

// NewDefaultSecurityService creates a security service with default config.
func NewDefaultSecurityService() (*SecurityService, error) {
	return NewSecurityService(DefaultSecurityServiceConfig())
}

// ========================================================================
// Password Operations
// ========================================================================

// HashPassword hashes a password.
func (s *SecurityService) HashPassword(password string) (string, error) {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		s.audit("password_hash", "", "", "failure", nil)
		return "", err
	}
	s.audit("password_hash", "", "", "success", nil)
	return hash, nil
}

// VerifyPassword verifies a password against a hash.
func (s *SecurityService) VerifyPassword(password, hash string) bool {
	result := s.hasher.Verify(password, hash)
	if result {
		s.audit("password_verify", "", "", "success", nil)
	} else {
		s.audit("password_verify", "", "", "failure", nil)
	}
	return result
}

// NeedsRehash checks if a password hash needs to be rehashed.
func (s *SecurityService) NeedsRehash(hash string) bool {
	return s.hasher.NeedsRehash(hash)
}

// ValidatePassword validates a password against requirements.
func (s *SecurityService) ValidatePassword(password string) infraSecurity.PasswordValidationResult {
	return s.hasher.Validate(password)
}

// ========================================================================
// Token Operations
// ========================================================================

// GenerateSessionToken generates a session token.
func (s *SecurityService) GenerateSessionToken() (*infraSecurity.Token, error) {
	token, err := s.tokenGenerator.GenerateSessionToken()
	if err != nil {
		s.audit("token_generate", "", "", "failure", map[string]interface{}{"type": "session"})
		return nil, err
	}
	s.audit("token_generate", "", "", "success", map[string]interface{}{"type": "session"})
	return token, nil
}

// GenerateAPIToken generates an API token.
func (s *SecurityService) GenerateAPIToken(prefix string) (*infraSecurity.Token, error) {
	token, err := s.tokenGenerator.GenerateAPIToken(prefix)
	if err != nil {
		s.audit("token_generate", "", "", "failure", map[string]interface{}{"type": "api"})
		return nil, err
	}
	s.audit("token_generate", "", "", "success", map[string]interface{}{"type": "api"})
	return token, nil
}

// GenerateCSRFToken generates a CSRF token.
func (s *SecurityService) GenerateCSRFToken() (*infraSecurity.Token, error) {
	return s.tokenGenerator.GenerateCSRFToken()
}

// GenerateSignedToken generates a signed token with payload.
func (s *SecurityService) GenerateSignedToken(payload map[string]interface{}, expirationSeconds int) (*infraSecurity.SignedToken, error) {
	return s.tokenGenerator.GenerateSignedToken(payload, expirationSeconds)
}

// VerifySignedToken verifies a signed token.
func (s *SecurityService) VerifySignedToken(combined string) (map[string]interface{}, error) {
	payload, err := s.tokenGenerator.VerifySignedToken(combined)
	if err != nil {
		s.audit("token_verify", "", "", "failure", nil)
		return nil, err
	}
	s.audit("token_verify", "", "", "success", nil)
	return payload, nil
}

// GenerateVerificationCode generates a verification code.
func (s *SecurityService) GenerateVerificationCode(length, expirationMinutes, maxAttempts int) (*infraSecurity.VerificationCode, error) {
	return s.tokenGenerator.GenerateVerificationCode(length, expirationMinutes, maxAttempts)
}

// CompareTokens compares two tokens in constant time.
func (s *SecurityService) CompareTokens(a, b string) bool {
	return s.tokenGenerator.Compare(a, b)
}

// ========================================================================
// Credential Operations
// ========================================================================

// GenerateCredentials generates installation credentials.
func (s *SecurityService) GenerateCredentials(expirationDays int) (*infraSecurity.GeneratedCredentials, error) {
	creds, err := s.credentialGen.GenerateInstallationCredentials(expirationDays)
	if err != nil {
		s.audit("credentials_generate", "", "", "failure", nil)
		return nil, err
	}
	s.audit("credentials_generate", "", "", "success", nil)
	return creds, nil
}

// GenerateAPIKey generates an API key.
func (s *SecurityService) GenerateAPIKey(prefix string) (*infraSecurity.APIKeyCredential, error) {
	return s.credentialGen.GenerateAPIKey(prefix)
}

// GeneratePassword generates a secure password.
func (s *SecurityService) GeneratePassword(length int) (string, error) {
	return s.credentialGen.GeneratePassword(length)
}

// GenerateSecret generates a secure secret.
func (s *SecurityService) GenerateSecret(length int) (string, error) {
	return s.credentialGen.GenerateSecret(length)
}

// ========================================================================
// Input Validation
// ========================================================================

// ValidateEmail validates an email address.
func (s *SecurityService) ValidateEmail(email string) infraSecurity.ValidationResult {
	return s.inputValidator.ValidateEmail(email)
}

// ValidateIdentifier validates an identifier.
func (s *SecurityService) ValidateIdentifier(id string) infraSecurity.ValidationResult {
	return s.inputValidator.ValidateIdentifier(id)
}

// ValidatePath validates a path.
func (s *SecurityService) ValidatePath(path string) infraSecurity.ValidationResult {
	return s.inputValidator.ValidatePath(path)
}

// ValidateURL validates a URL.
func (s *SecurityService) ValidateURL(url string) infraSecurity.ValidationResult {
	return s.inputValidator.ValidateURL(url)
}

// ValidateCommandArg validates a command argument.
func (s *SecurityService) ValidateCommandArg(arg string) infraSecurity.ValidationResult {
	return s.inputValidator.ValidateCommandArgument(arg)
}

// SanitizeString sanitizes a string.
func (s *SecurityService) SanitizeString(input string) string {
	return infraSecurity.SanitizeString(input)
}

// SanitizeHTML sanitizes HTML.
func (s *SecurityService) SanitizeHTML(input string) string {
	return infraSecurity.SanitizeHTML(input)
}

// SanitizePath sanitizes a path.
func (s *SecurityService) SanitizePath(input string) string {
	return infraSecurity.SanitizePath(input)
}

// ========================================================================
// Path Validation
// ========================================================================

// ValidateFilePath validates a file path against allowed prefixes.
func (s *SecurityService) ValidateFilePath(path string) (string, error) {
	if s.pathValidator == nil {
		return "", fmt.Errorf("path validator not configured")
	}

	result := s.pathValidator.Validate(path)
	if !result.IsValid {
		s.audit("path_validate", "", "", "failure", map[string]interface{}{"path": path})
		if len(result.Errors) > 0 {
			return "", result.Errors[0]
		}
		return "", infraSecurity.ErrPathOutsideAllowed
	}

	s.audit("path_validate", "", "", "success", map[string]interface{}{"path": path})
	return result.ResolvedPath, nil
}

// IsPathAllowed checks if a path is within allowed prefixes.
func (s *SecurityService) IsPathAllowed(path string) bool {
	if s.pathValidator == nil {
		return false
	}
	return s.pathValidator.IsWithinAllowed(path)
}

// SecureJoin securely joins path segments.
func (s *SecurityService) SecureJoin(prefix string, segments ...string) (string, error) {
	if s.pathValidator == nil {
		return "", fmt.Errorf("path validator not configured")
	}
	return s.pathValidator.SecureJoin(prefix, segments...)
}

// ========================================================================
// Command Execution
// ========================================================================

// ExecuteCommand executes a command safely.
func (s *SecurityService) ExecuteCommand(ctx context.Context, command string, args ...string) (*infraSecurity.ExecutionResult, error) {
	if s.executor == nil {
		return nil, fmt.Errorf("executor not configured")
	}

	result, err := s.executor.ExecuteWithContext(ctx, command, args...)
	if err != nil {
		s.audit("command_execute", "", "", "failure", map[string]interface{}{
			"command": command,
			"error":   err.Error(),
		})
		return nil, err
	}

	s.audit("command_execute", "", "", "success", map[string]interface{}{
		"command":  command,
		"exitCode": result.ExitCode,
	})
	return result, nil
}

// IsCommandAllowed checks if a command is allowed.
func (s *SecurityService) IsCommandAllowed(command string) bool {
	if s.executor == nil {
		return false
	}
	return s.executor.IsCommandAllowed(command)
}

// ========================================================================
// Threat Detection
// ========================================================================

// DetectThreats scans input for threats.
func (s *SecurityService) DetectThreats(input string) []domainSecurity.ThreatDetection {
	detections := s.threatDetector.Detect(input)
	if len(detections) > 0 {
		s.audit("threat_detected", "", "", "alert", map[string]interface{}{
			"count":    len(detections),
			"input":    domainSecurity.SanitizeForLogging(input),
		})
	}
	return detections
}

// HasThreats checks if input contains threats.
func (s *SecurityService) HasThreats(input string) bool {
	return s.threatDetector.HasThreats(input)
}

// GetHighestThreatLevel returns the highest threat level in input.
func (s *SecurityService) GetHighestThreatLevel(input string) domainSecurity.ThreatLevel {
	return s.threatDetector.GetHighestThreatLevel(input)
}

// IsSafe checks if input is safe up to the given threat level.
func (s *SecurityService) IsSafe(input string, maxLevel domainSecurity.ThreatLevel) bool {
	return s.threatDetector.IsSafe(input, maxLevel)
}

// ========================================================================
// Security Context Management
// ========================================================================

// CreateContext creates a new security context.
func (s *SecurityService) CreateContext(userID, sessionID string, role domainSecurity.Role) *domainSecurity.SecurityContext {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := domainSecurity.NewSecurityContext(userID, sessionID, role)
	ctx.SetExpiration(time.Duration(s.config.SessionTimeout) * time.Second)
	s.contexts[sessionID] = ctx

	s.auditWithContext("context_create", ctx, "success", nil)
	return ctx
}

// GetContext retrieves a security context by session ID.
func (s *SecurityService) GetContext(sessionID string) (*domainSecurity.SecurityContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx, ok := s.contexts[sessionID]
	return ctx, ok
}

// Authenticate authenticates a context.
func (s *SecurityService) Authenticate(sessionID, userID, username string, role domainSecurity.Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, ok := s.contexts[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	ctx.Authenticate(userID, username, role)
	s.auditWithContext("authenticate", ctx, "success", nil)
	return nil
}

// RecordFailedLogin records a failed login attempt.
func (s *SecurityService) RecordFailedLogin(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, ok := s.contexts[sessionID]
	if !ok {
		return
	}

	ctx.RecordFailedAttempt(s.config.MaxLoginAttempts, s.config.LockoutDuration)
	s.auditWithContext("failed_login", ctx, "failure", nil)
}

// Logout logs out a context.
func (s *SecurityService) Logout(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, ok := s.contexts[sessionID]
	if !ok {
		return
	}

	ctx.Logout()
	s.auditWithContext("logout", ctx, "success", nil)
	delete(s.contexts, sessionID)
}

// CheckPermission checks if a context has a permission.
func (s *SecurityService) CheckPermission(sessionID string, permission domainSecurity.Permission, resource string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ctx, ok := s.contexts[sessionID]
	if !ok {
		return false
	}

	return ctx.CanPerform(permission, resource)
}

// ========================================================================
// Audit Operations
// ========================================================================

// audit logs an audit event.
func (s *SecurityService) audit(eventType, userID, sessionID, result string, details map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		UserID:    userID,
		SessionID: sessionID,
		Result:    result,
		Details:   details,
	}

	s.auditLog = append(s.auditLog, event)

	// Keep only last 1000 entries
	if len(s.auditLog) > 1000 {
		s.auditLog = s.auditLog[len(s.auditLog)-1000:]
	}
}

// auditWithContext logs an audit event with context.
func (s *SecurityService) auditWithContext(eventType string, ctx *domainSecurity.SecurityContext, result string, details map[string]interface{}) {
	event := AuditEvent{
		Timestamp:   time.Now(),
		EventType:   eventType,
		UserID:      ctx.UserID,
		SessionID:   ctx.SessionID,
		Result:      result,
		ThreatLevel: ctx.ThreatLevel,
		IPAddress:   ctx.IPAddress,
		Details:     details,
	}

	s.auditLog = append(s.auditLog, event)

	if len(s.auditLog) > 1000 {
		s.auditLog = s.auditLog[len(s.auditLog)-1000:]
	}
}

// GetAuditLog returns the audit log.
func (s *SecurityService) GetAuditLog() []AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]AuditEvent, len(s.auditLog))
	copy(result, s.auditLog)
	return result
}

// GetAuditLogByType returns audit events of a specific type.
func (s *SecurityService) GetAuditLogByType(eventType string) []AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]AuditEvent, 0)
	for _, event := range s.auditLog {
		if event.EventType == eventType {
			result = append(result, event)
		}
	}
	return result
}

// ClearAuditLog clears the audit log.
func (s *SecurityService) ClearAuditLog() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditLog = make([]AuditEvent, 0)
}

// ========================================================================
// Configuration
// ========================================================================

// SetHMACSecret sets the HMAC secret for signed tokens.
func (s *SecurityService) SetHMACSecret(secret string) {
	s.tokenGenerator.SetHMACSecret(secret)
	s.config.HMACSecret = secret
}

// AddAllowedPath adds an allowed path prefix.
func (s *SecurityService) AddAllowedPath(path string) error {
	if s.pathValidator == nil {
		return fmt.Errorf("path validator not configured")
	}
	return s.pathValidator.AddPrefix(path)
}

// AddAllowedCommand adds an allowed command.
func (s *SecurityService) AddAllowedCommand(command string) error {
	if s.executor == nil {
		return fmt.Errorf("executor not configured")
	}
	return s.executor.AllowCommand(command)
}

// GetStats returns security service statistics.
func (s *SecurityService) GetStats() SecurityStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return SecurityStats{
		ActiveContexts: len(s.contexts),
		AuditLogSize:   len(s.auditLog),
		BCryptRounds:   s.config.BCryptRounds,
	}
}

// SecurityStats contains security service statistics.
type SecurityStats struct {
	ActiveContexts int `json:"activeContexts"`
	AuditLogSize   int `json:"auditLogSize"`
	BCryptRounds   int `json:"bcryptRounds"`
}
