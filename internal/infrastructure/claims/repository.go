// Package claims provides infrastructure for the claims system.
package claims

import (
	"fmt"
	"sync"
	"time"

	domainClaims "github.com/anthropics/claude-flow-go/internal/domain/claims"
)

// ClaimRepository defines the interface for claim persistence.
type ClaimRepository interface {
	Save(claim *domainClaims.Claim) error
	FindByID(id string) (*domainClaims.Claim, error)
	FindByIssueID(issueID string) (*domainClaims.Claim, error)
	FindByClaimant(claimantID string) ([]*domainClaims.Claim, error)
	FindActive() ([]*domainClaims.Claim, error)
	FindStealable() ([]*domainClaims.Claim, error)
	FindStale(threshold time.Duration) ([]*domainClaims.Claim, error)
	FindByStatus(status domainClaims.ClaimStatus) ([]*domainClaims.Claim, error)
	Delete(id string) error
	Count() int
	CountByStatus(status domainClaims.ClaimStatus) int
	CountByClaimant(claimantID string) int
}

// IssueRepository defines the interface for issue persistence.
type IssueRepository interface {
	Save(issue *domainClaims.Issue) error
	FindByID(id string) (*domainClaims.Issue, error)
	FindAll() ([]*domainClaims.Issue, error)
	FindByPriority(priority domainClaims.Priority) ([]*domainClaims.Issue, error)
	FindUnclaimed(claims []*domainClaims.Claim) ([]*domainClaims.Issue, error)
	Delete(id string) error
	Count() int
}

// ClaimantRepository defines the interface for claimant persistence.
type ClaimantRepository interface {
	Save(claimant *domainClaims.Claimant) error
	FindByID(id string) (*domainClaims.Claimant, error)
	FindAll() ([]*domainClaims.Claimant, error)
	FindByType(claimantType domainClaims.ClaimantType) ([]*domainClaims.Claimant, error)
	FindAvailable() ([]*domainClaims.Claimant, error)
	Delete(id string) error
	Count() int
}

// InMemoryClaimRepository provides an in-memory claim repository.
type InMemoryClaimRepository struct {
	mu     sync.RWMutex
	claims map[string]*domainClaims.Claim
}

// NewInMemoryClaimRepository creates a new in-memory claim repository.
func NewInMemoryClaimRepository() *InMemoryClaimRepository {
	return &InMemoryClaimRepository{
		claims: make(map[string]*domainClaims.Claim),
	}
}

// Save saves a claim.
func (r *InMemoryClaimRepository) Save(claim *domainClaims.Claim) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Make a copy to avoid mutation issues
	claimCopy := *claim
	r.claims[claim.ID] = &claimCopy
	return nil
}

// FindByID finds a claim by ID.
func (r *InMemoryClaimRepository) FindByID(id string) (*domainClaims.Claim, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	claim, exists := r.claims[id]
	if !exists {
		return nil, fmt.Errorf("claim not found: %s", id)
	}

	claimCopy := *claim
	return &claimCopy, nil
}

// FindByIssueID finds the active claim for an issue.
func (r *InMemoryClaimRepository) FindByIssueID(issueID string) (*domainClaims.Claim, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, claim := range r.claims {
		if claim.IssueID == issueID && claim.IsActive() {
			claimCopy := *claim
			return &claimCopy, nil
		}
	}

	return nil, fmt.Errorf("no active claim for issue: %s", issueID)
}

// FindByClaimant finds all claims for a claimant.
func (r *InMemoryClaimRepository) FindByClaimant(claimantID string) ([]*domainClaims.Claim, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Claim, 0)
	for _, claim := range r.claims {
		if claim.Claimant.ID == claimantID {
			claimCopy := *claim
			result = append(result, &claimCopy)
		}
	}

	return result, nil
}

// FindActive finds all active claims.
func (r *InMemoryClaimRepository) FindActive() ([]*domainClaims.Claim, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Claim, 0)
	for _, claim := range r.claims {
		if claim.IsActive() {
			claimCopy := *claim
			result = append(result, &claimCopy)
		}
	}

	return result, nil
}

// FindStealable finds all stealable claims.
func (r *InMemoryClaimRepository) FindStealable() ([]*domainClaims.Claim, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Claim, 0)
	for _, claim := range r.claims {
		if claim.Status == domainClaims.StatusStealable {
			claimCopy := *claim
			result = append(result, &claimCopy)
		}
	}

	return result, nil
}

// FindStale finds claims that are stale.
func (r *InMemoryClaimRepository) FindStale(threshold time.Duration) ([]*domainClaims.Claim, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Claim, 0)
	for _, claim := range r.claims {
		if claim.IsActive() && claim.IsStale(threshold) {
			claimCopy := *claim
			result = append(result, &claimCopy)
		}
	}

	return result, nil
}

// FindByStatus finds claims by status.
func (r *InMemoryClaimRepository) FindByStatus(status domainClaims.ClaimStatus) ([]*domainClaims.Claim, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Claim, 0)
	for _, claim := range r.claims {
		if claim.Status == status {
			claimCopy := *claim
			result = append(result, &claimCopy)
		}
	}

	return result, nil
}

// Delete deletes a claim.
func (r *InMemoryClaimRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.claims[id]; !exists {
		return fmt.Errorf("claim not found: %s", id)
	}

	delete(r.claims, id)
	return nil
}

// Count returns the total number of claims.
func (r *InMemoryClaimRepository) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.claims)
}

// CountByStatus returns the count of claims with a given status.
func (r *InMemoryClaimRepository) CountByStatus(status domainClaims.ClaimStatus) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, claim := range r.claims {
		if claim.Status == status {
			count++
		}
	}
	return count
}

// CountByClaimant returns the count of active claims for a claimant.
func (r *InMemoryClaimRepository) CountByClaimant(claimantID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, claim := range r.claims {
		if claim.Claimant.ID == claimantID && claim.IsActive() {
			count++
		}
	}
	return count
}

// Clear clears all claims (for testing).
func (r *InMemoryClaimRepository) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claims = make(map[string]*domainClaims.Claim)
}

// InMemoryIssueRepository provides an in-memory issue repository.
type InMemoryIssueRepository struct {
	mu     sync.RWMutex
	issues map[string]*domainClaims.Issue
}

// NewInMemoryIssueRepository creates a new in-memory issue repository.
func NewInMemoryIssueRepository() *InMemoryIssueRepository {
	return &InMemoryIssueRepository{
		issues: make(map[string]*domainClaims.Issue),
	}
}

// Save saves an issue.
func (r *InMemoryIssueRepository) Save(issue *domainClaims.Issue) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	issueCopy := *issue
	r.issues[issue.ID] = &issueCopy
	return nil
}

// FindByID finds an issue by ID.
func (r *InMemoryIssueRepository) FindByID(id string) (*domainClaims.Issue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	issue, exists := r.issues[id]
	if !exists {
		return nil, fmt.Errorf("issue not found: %s", id)
	}

	issueCopy := *issue
	return &issueCopy, nil
}

// FindAll finds all issues.
func (r *InMemoryIssueRepository) FindAll() ([]*domainClaims.Issue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Issue, 0, len(r.issues))
	for _, issue := range r.issues {
		issueCopy := *issue
		result = append(result, &issueCopy)
	}

	return result, nil
}

// FindByPriority finds issues by priority.
func (r *InMemoryIssueRepository) FindByPriority(priority domainClaims.Priority) ([]*domainClaims.Issue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Issue, 0)
	for _, issue := range r.issues {
		if issue.Priority == priority {
			issueCopy := *issue
			result = append(result, &issueCopy)
		}
	}

	return result, nil
}

// FindUnclaimed finds issues that are not claimed.
func (r *InMemoryIssueRepository) FindUnclaimed(claims []*domainClaims.Claim) ([]*domainClaims.Issue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	claimedIssues := make(map[string]bool)
	for _, claim := range claims {
		if claim.IsActive() {
			claimedIssues[claim.IssueID] = true
		}
	}

	result := make([]*domainClaims.Issue, 0)
	for _, issue := range r.issues {
		if !claimedIssues[issue.ID] {
			issueCopy := *issue
			result = append(result, &issueCopy)
		}
	}

	return result, nil
}

// Delete deletes an issue.
func (r *InMemoryIssueRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.issues[id]; !exists {
		return fmt.Errorf("issue not found: %s", id)
	}

	delete(r.issues, id)
	return nil
}

// Count returns the total number of issues.
func (r *InMemoryIssueRepository) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.issues)
}

// InMemoryClaimantRepository provides an in-memory claimant repository.
type InMemoryClaimantRepository struct {
	mu        sync.RWMutex
	claimants map[string]*domainClaims.Claimant
}

// NewInMemoryClaimantRepository creates a new in-memory claimant repository.
func NewInMemoryClaimantRepository() *InMemoryClaimantRepository {
	return &InMemoryClaimantRepository{
		claimants: make(map[string]*domainClaims.Claimant),
	}
}

// Save saves a claimant.
func (r *InMemoryClaimantRepository) Save(claimant *domainClaims.Claimant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	claimantCopy := *claimant
	r.claimants[claimant.ID] = &claimantCopy
	return nil
}

// FindByID finds a claimant by ID.
func (r *InMemoryClaimantRepository) FindByID(id string) (*domainClaims.Claimant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	claimant, exists := r.claimants[id]
	if !exists {
		return nil, fmt.Errorf("claimant not found: %s", id)
	}

	claimantCopy := *claimant
	return &claimantCopy, nil
}

// FindAll finds all claimants.
func (r *InMemoryClaimantRepository) FindAll() ([]*domainClaims.Claimant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Claimant, 0, len(r.claimants))
	for _, claimant := range r.claimants {
		claimantCopy := *claimant
		result = append(result, &claimantCopy)
	}

	return result, nil
}

// FindByType finds claimants by type.
func (r *InMemoryClaimantRepository) FindByType(claimantType domainClaims.ClaimantType) ([]*domainClaims.Claimant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Claimant, 0)
	for _, claimant := range r.claimants {
		if claimant.Type == claimantType {
			claimantCopy := *claimant
			result = append(result, &claimantCopy)
		}
	}

	return result, nil
}

// FindAvailable finds claimants that can accept more work.
func (r *InMemoryClaimantRepository) FindAvailable() ([]*domainClaims.Claimant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainClaims.Claimant, 0)
	for _, claimant := range r.claimants {
		if claimant.CanAcceptMoreWork() {
			claimantCopy := *claimant
			result = append(result, &claimantCopy)
		}
	}

	return result, nil
}

// Delete deletes a claimant.
func (r *InMemoryClaimantRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.claimants[id]; !exists {
		return fmt.Errorf("claimant not found: %s", id)
	}

	delete(r.claimants, id)
	return nil
}

// Count returns the total number of claimants.
func (r *InMemoryClaimantRepository) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.claimants)
}
