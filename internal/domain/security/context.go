// Package security provides domain types for the security module.
package security

import (
	"sync"
	"time"
)

// SecurityContext represents the security context for a user or session.
type SecurityContext struct {
	mu sync.RWMutex

	// Identity
	UserID      string `json:"userId"`
	SessionID   string `json:"sessionId"`
	Username    string `json:"username,omitempty"`
	Email       string `json:"email,omitempty"`
	Role        Role   `json:"role"`
	IsAuthenticated bool `json:"isAuthenticated"`

	// Permissions
	Permissions PermissionSet `json:"permissions"`

	// Session info
	CreatedAt     time.Time  `json:"createdAt"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	LastActivityAt time.Time `json:"lastActivityAt"`

	// Security state
	ThreatLevel     ThreatLevel `json:"threatLevel"`
	FailedAttempts  int         `json:"failedAttempts"`
	LockedUntil     *time.Time  `json:"lockedUntil,omitempty"`
	MFAVerified     bool        `json:"mfaVerified"`
	IPAddress       string      `json:"ipAddress,omitempty"`
	UserAgent       string      `json:"userAgent,omitempty"`

	// Audit trail
	AuditLog []AuditEntry `json:"auditLog,omitempty"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AuditEntry represents an entry in the audit log.
type AuditEntry struct {
	Timestamp   time.Time   `json:"timestamp"`
	Action      string      `json:"action"`
	Resource    string      `json:"resource,omitempty"`
	Result      string      `json:"result"`
	ThreatLevel ThreatLevel `json:"threatLevel,omitempty"`
	Details     string      `json:"details,omitempty"`
	IPAddress   string      `json:"ipAddress,omitempty"`
}

// NewSecurityContext creates a new security context.
func NewSecurityContext(userID, sessionID string, role Role) *SecurityContext {
	now := time.Now()
	return &SecurityContext{
		UserID:          userID,
		SessionID:       sessionID,
		Role:            role,
		IsAuthenticated: false,
		Permissions:     GetRolePermissions(role),
		CreatedAt:       now,
		LastActivityAt:  now,
		ThreatLevel:     ThreatLevelNone,
		FailedAttempts:  0,
		MFAVerified:     false,
		AuditLog:        make([]AuditEntry, 0),
		Metadata:        make(map[string]interface{}),
	}
}

// NewAnonymousContext creates an unauthenticated context.
func NewAnonymousContext(sessionID string) *SecurityContext {
	return NewSecurityContext("", sessionID, RoleViewer)
}

// Authenticate marks the context as authenticated.
func (sc *SecurityContext) Authenticate(userID, username string, role Role) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.UserID = userID
	sc.Username = username
	sc.Role = role
	sc.IsAuthenticated = true
	sc.Permissions = GetRolePermissions(role)
	sc.LastActivityAt = time.Now()
	sc.FailedAttempts = 0

	sc.addAuditEntry("authenticate", "", "success", "")
}

// Logout invalidates the context.
func (sc *SecurityContext) Logout() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.addAuditEntry("logout", "", "success", "")

	sc.IsAuthenticated = false
	sc.MFAVerified = false
}

// HasPermission checks if the context has a specific permission.
func (sc *SecurityContext) HasPermission(permission Permission) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if !sc.IsAuthenticated {
		return false
	}

	if sc.IsLocked() {
		return false
	}

	return sc.Permissions.Has(permission)
}

// CanPerform checks if the context can perform an action and logs the attempt.
func (sc *SecurityContext) CanPerform(permission Permission, resource string) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.IsAuthenticated {
		sc.addAuditEntry("access_denied", resource, "unauthenticated", "")
		return false
	}

	if sc.IsLockedUnsafe() {
		sc.addAuditEntry("access_denied", resource, "locked", "")
		return false
	}

	if sc.Permissions.Has(permission) {
		sc.addAuditEntry("access_granted", resource, "success", string(permission))
		sc.LastActivityAt = time.Now()
		return true
	}

	sc.addAuditEntry("access_denied", resource, "insufficient_permissions", string(permission))
	return false
}

// AddPermission adds a permission to the context.
func (sc *SecurityContext) AddPermission(permission Permission) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Permissions.Add(permission)
}

// RemovePermission removes a permission from the context.
func (sc *SecurityContext) RemovePermission(permission Permission) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Permissions.Remove(permission)
}

// RecordFailedAttempt records a failed authentication attempt.
func (sc *SecurityContext) RecordFailedAttempt(maxAttempts int, lockoutDuration time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.FailedAttempts++
	sc.addAuditEntry("failed_attempt", "", "failure", "")

	if sc.FailedAttempts >= maxAttempts {
		lockUntil := time.Now().Add(lockoutDuration)
		sc.LockedUntil = &lockUntil
		sc.ThreatLevel = ThreatLevelHigh
		sc.addAuditEntry("account_locked", "", "locked", "")
	}
}

// ResetFailedAttempts resets the failed attempt counter.
func (sc *SecurityContext) ResetFailedAttempts() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.FailedAttempts = 0
	sc.LockedUntil = nil
}

// IsLocked returns true if the context is locked.
func (sc *SecurityContext) IsLocked() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.IsLockedUnsafe()
}

// IsLockedUnsafe returns true if the context is locked (without lock).
func (sc *SecurityContext) IsLockedUnsafe() bool {
	if sc.LockedUntil == nil {
		return false
	}
	if time.Now().After(*sc.LockedUntil) {
		sc.LockedUntil = nil
		return false
	}
	return true
}

// IsExpired returns true if the context has expired.
func (sc *SecurityContext) IsExpired() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if sc.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*sc.ExpiresAt)
}

// SetExpiration sets the expiration time.
func (sc *SecurityContext) SetExpiration(duration time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	expiresAt := time.Now().Add(duration)
	sc.ExpiresAt = &expiresAt
}

// UpdateActivity updates the last activity timestamp.
func (sc *SecurityContext) UpdateActivity() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.LastActivityAt = time.Now()
}

// SetThreatLevel sets the threat level.
func (sc *SecurityContext) SetThreatLevel(level ThreatLevel) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.ThreatLevel = level

	if compareThreatLevel(level, ThreatLevelMedium) > 0 {
		sc.addAuditEntry("threat_detected", "", string(level), "")
	}
}

// VerifyMFA marks MFA as verified.
func (sc *SecurityContext) VerifyMFA() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.MFAVerified = true
	sc.addAuditEntry("mfa_verified", "", "success", "")
}

// SetClientInfo sets the client information.
func (sc *SecurityContext) SetClientInfo(ipAddress, userAgent string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.IPAddress = ipAddress
	sc.UserAgent = userAgent
}

// addAuditEntry adds an audit entry (must be called with lock held).
func (sc *SecurityContext) addAuditEntry(action, resource, result, details string) {
	entry := AuditEntry{
		Timestamp:   time.Now(),
		Action:      action,
		Resource:    resource,
		Result:      result,
		ThreatLevel: sc.ThreatLevel,
		Details:     details,
		IPAddress:   sc.IPAddress,
	}

	sc.AuditLog = append(sc.AuditLog, entry)

	// Keep only last 100 entries
	if len(sc.AuditLog) > 100 {
		sc.AuditLog = sc.AuditLog[len(sc.AuditLog)-100:]
	}
}

// GetAuditLog returns a copy of the audit log.
func (sc *SecurityContext) GetAuditLog() []AuditEntry {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make([]AuditEntry, len(sc.AuditLog))
	copy(result, sc.AuditLog)
	return result
}

// Clone creates a copy of the security context.
func (sc *SecurityContext) Clone() *SecurityContext {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	clone := &SecurityContext{
		UserID:          sc.UserID,
		SessionID:       sc.SessionID,
		Username:        sc.Username,
		Email:           sc.Email,
		Role:            sc.Role,
		IsAuthenticated: sc.IsAuthenticated,
		Permissions:     make(PermissionSet),
		CreatedAt:       sc.CreatedAt,
		LastActivityAt:  sc.LastActivityAt,
		ThreatLevel:     sc.ThreatLevel,
		FailedAttempts:  sc.FailedAttempts,
		MFAVerified:     sc.MFAVerified,
		IPAddress:       sc.IPAddress,
		UserAgent:       sc.UserAgent,
		AuditLog:        make([]AuditEntry, len(sc.AuditLog)),
		Metadata:        make(map[string]interface{}),
	}

	for p := range sc.Permissions {
		clone.Permissions[p] = true
	}

	copy(clone.AuditLog, sc.AuditLog)

	for k, v := range sc.Metadata {
		clone.Metadata[k] = v
	}

	if sc.ExpiresAt != nil {
		t := *sc.ExpiresAt
		clone.ExpiresAt = &t
	}

	if sc.LockedUntil != nil {
		t := *sc.LockedUntil
		clone.LockedUntil = &t
	}

	return clone
}
