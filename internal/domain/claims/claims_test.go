package claims

import (
	"testing"
	"time"
)

func TestNewClaim(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claim := NewClaim("claim-1", "issue-1", *claimant)

	if claim.ID != "claim-1" {
		t.Fatalf("expected ID claim-1, got %s", claim.ID)
	}
	if claim.IssueID != "issue-1" {
		t.Fatalf("expected IssueID issue-1, got %s", claim.IssueID)
	}
	if claim.Status != StatusActive {
		t.Fatalf("expected status active, got %s", claim.Status)
	}
	if claim.Progress != 0.0 {
		t.Fatalf("expected progress 0.0, got %f", claim.Progress)
	}
	if claim.Version != 1 {
		t.Fatalf("expected version 1, got %d", claim.Version)
	}
}

func TestClaimUpdateStatus(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claim := NewClaim("claim-1", "issue-1", *claimant)

	if err := claim.UpdateStatus(StatusPaused); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claim.Status != StatusPaused {
		t.Fatalf("expected paused, got %s", claim.Status)
	}
	if claim.Version != 2 {
		t.Fatalf("expected version 2, got %d", claim.Version)
	}
}

func TestClaimUpdateStatusTerminal(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claim := NewClaim("claim-1", "issue-1", *claimant)
	claim.Complete()

	if err := claim.UpdateStatus(StatusActive); err == nil {
		t.Fatal("expected error updating terminal claim")
	}
}

func TestClaimUpdateProgress(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claim := NewClaim("claim-1", "issue-1", *claimant)

	claim.UpdateProgress(0.5)
	if claim.Progress != 0.5 {
		t.Fatalf("expected 0.5, got %f", claim.Progress)
	}

	claim.UpdateProgress(-1.0)
	if claim.Progress != 0.0 {
		t.Fatal("expected 0.0 for negative progress")
	}

	claim.UpdateProgress(2.0)
	if claim.Progress != 1.0 {
		t.Fatal("expected 1.0 for over-max progress")
	}
}

func TestClaimLifecycle(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claim := NewClaim("claim-1", "issue-1", *claimant)

	if !claim.IsActive() {
		t.Fatal("new claim should be active")
	}
	if claim.IsTerminal() {
		t.Fatal("new claim should not be terminal")
	}

	claim.Complete()
	if !claim.IsTerminal() {
		t.Fatal("completed claim should be terminal")
	}
	if claim.Progress != 1.0 {
		t.Fatal("completed claim should have progress 1.0")
	}
}

func TestClaimRelease(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claim := NewClaim("claim-1", "issue-1", *claimant)

	claim.Release()
	if claim.Status != StatusReleased {
		t.Fatalf("expected released, got %s", claim.Status)
	}
	if !claim.IsTerminal() {
		t.Fatal("released claim should be terminal")
	}
}

func TestClaimExpire(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claim := NewClaim("claim-1", "issue-1", *claimant)

	claim.Expire()
	if claim.Status != StatusExpired {
		t.Fatalf("expected expired, got %s", claim.Status)
	}
}

func TestClaimIsOwnedBy(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claim := NewClaim("claim-1", "issue-1", *claimant)

	if !claim.IsOwnedBy("agent-1") {
		t.Fatal("should be owned by agent-1")
	}
	if claim.IsOwnedBy("agent-2") {
		t.Fatal("should not be owned by agent-2")
	}
}

func TestClaimAddNote(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claim := NewClaim("claim-1", "issue-1", *claimant)

	claim.AddNote("started work")
	if len(claim.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(claim.Notes))
	}
	if claim.Notes[0] != "started work" {
		t.Fatalf("unexpected note: %s", claim.Notes[0])
	}
}

func TestCanClaimIssue(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claimant.Capabilities = []string{"code", "debug"}
	issue := NewIssue("issue-1", "Fix bug", "Fix a critical bug")

	ok, reason := CanClaimIssue(claimant, issue, nil)
	if !ok {
		t.Fatalf("should be able to claim: %s", reason)
	}
}

func TestCanClaimIssueAtCapacity(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	claimant.CurrentWorkload = claimant.MaxConcurrent
	issue := NewIssue("issue-1", "Fix bug", "Fix a bug")

	ok, _ := CanClaimIssue(claimant, issue, nil)
	if ok {
		t.Fatal("should not be able to claim at capacity")
	}
}

func TestCanClaimIssueAlreadyClaimed(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	issue := NewIssue("issue-1", "Fix bug", "Fix a bug")
	other := NewClaimant("agent-2", "Tester", ClaimantTypeAgent)
	existing := []*Claim{NewClaim("c1", "issue-1", *other)}

	ok, _ := CanClaimIssue(claimant, issue, existing)
	if ok {
		t.Fatal("should not be able to claim already-claimed issue")
	}
}

func TestCanClaimIssueMissingCapabilities(t *testing.T) {
	claimant := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	issue := NewIssue("issue-1", "Security audit", "Audit")
	issue.RequiredCapabilities = []string{"security-audit"}

	ok, _ := CanClaimIssue(claimant, issue, nil)
	if ok {
		t.Fatal("should not claim without required capabilities")
	}
}

func TestCanTransitionStatus(t *testing.T) {
	tests := []struct {
		from, to ClaimStatus
		valid    bool
	}{
		{StatusActive, StatusPaused, true},
		{StatusActive, StatusCompleted, true},
		{StatusActive, StatusExpired, false},
		{StatusPaused, StatusActive, true},
		{StatusPaused, StatusCompleted, false},
		{StatusCompleted, StatusActive, false},
		{StatusReleased, StatusActive, false},
		{StatusBlocked, StatusStealable, true},
	}

	for _, tt := range tests {
		result := CanTransitionStatus(tt.from, tt.to)
		if result != tt.valid {
			t.Errorf("CanTransitionStatus(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.valid)
		}
	}
}

func TestCanStealClaim(t *testing.T) {
	owner := NewClaimant("owner", "Owner", ClaimantTypeAgent)
	stealer := NewClaimant("stealer", "Stealer", ClaimantTypeAgent)
	config := DefaultStealConfig()

	claim := NewClaim("c1", "issue-1", *owner)
	claim.Status = StatusStealable

	ok, _ := CanStealClaim(claim, stealer, config)
	if !ok {
		t.Fatal("should be able to steal stealable claim")
	}
}

func TestCanStealClaimNotStealable(t *testing.T) {
	owner := NewClaimant("owner", "Owner", ClaimantTypeAgent)
	stealer := NewClaimant("stealer", "Stealer", ClaimantTypeAgent)
	config := DefaultStealConfig()

	claim := NewClaim("c1", "issue-1", *owner)

	ok, _ := CanStealClaim(claim, stealer, config)
	if ok {
		t.Fatal("should not steal non-stealable claim")
	}
}

func TestCanStealOwnClaim(t *testing.T) {
	owner := NewClaimant("owner", "Owner", ClaimantTypeAgent)
	config := DefaultStealConfig()

	claim := NewClaim("c1", "issue-1", *owner)
	claim.Status = StatusStealable

	ok, _ := CanStealClaim(claim, owner, config)
	if ok {
		t.Fatal("should not steal own claim")
	}
}

func TestCanMarkAsStealable(t *testing.T) {
	owner := NewClaimant("owner", "Owner", ClaimantTypeAgent)
	config := DefaultStealConfig()
	config.GracePeriod = 0

	claim := NewClaim("c1", "issue-1", *owner)
	claim.ClaimedAt = time.Now().Add(-10 * time.Minute)

	ok, _ := CanMarkAsStealable(claim, config)
	if !ok {
		t.Fatal("should be able to mark as stealable")
	}
}

func TestCanMarkAsStealableWithinGracePeriod(t *testing.T) {
	owner := NewClaimant("owner", "Owner", ClaimantTypeAgent)
	config := DefaultStealConfig()

	claim := NewClaim("c1", "issue-1", *owner)

	ok, _ := CanMarkAsStealable(claim, config)
	if ok {
		t.Fatal("should not mark as stealable within grace period")
	}
}

func TestCanMarkAsStealableHighProgress(t *testing.T) {
	owner := NewClaimant("owner", "Owner", ClaimantTypeAgent)
	config := DefaultStealConfig()
	config.GracePeriod = 0

	claim := NewClaim("c1", "issue-1", *owner)
	claim.ClaimedAt = time.Now().Add(-10 * time.Minute)
	claim.Progress = 0.9

	ok, _ := CanMarkAsStealable(claim, config)
	if ok {
		t.Fatal("should not mark as stealable with high progress")
	}
}

func TestCanInitiateHandoff(t *testing.T) {
	from := NewClaimant("from", "From", ClaimantTypeAgent)
	to := NewClaimant("to", "To", ClaimantTypeAgent)
	claim := NewClaim("c1", "issue-1", *from)

	ok, _ := CanInitiateHandoff(claim, from, to)
	if !ok {
		t.Fatal("should be able to initiate handoff")
	}
}

func TestCanInitiateHandoffNotOwner(t *testing.T) {
	owner := NewClaimant("owner", "Owner", ClaimantTypeAgent)
	other := NewClaimant("other", "Other", ClaimantTypeAgent)
	to := NewClaimant("to", "To", ClaimantTypeAgent)
	claim := NewClaim("c1", "issue-1", *owner)

	ok, _ := CanInitiateHandoff(claim, other, to)
	if ok {
		t.Fatal("non-owner should not initiate handoff")
	}
}

func TestCanInitiateHandoffToSelf(t *testing.T) {
	from := NewClaimant("from", "From", ClaimantTypeAgent)
	claim := NewClaim("c1", "issue-1", *from)

	ok, _ := CanInitiateHandoff(claim, from, from)
	if ok {
		t.Fatal("should not handoff to self")
	}
}

func TestIsAgentOverloaded(t *testing.T) {
	config := DefaultLoadConfig()
	agent := NewClaimant("agent-1", "Coder", ClaimantTypeAgent)
	agent.CurrentWorkload = 9
	agent.MaxConcurrent = 10

	if !IsAgentOverloaded(agent, 0.5, config) {
		t.Fatal("agent at 90% utilization should be overloaded when avg is 50%")
	}

	agent.CurrentWorkload = 3
	if IsAgentOverloaded(agent, 0.5, config) {
		t.Fatal("agent at 30% utilization should not be overloaded when avg is 50%")
	}
}

func TestNeedsRebalancing(t *testing.T) {
	config := DefaultLoadConfig()

	balanced := []float64{0.5, 0.5, 0.5, 0.5}
	if NeedsRebalancing(balanced, config) {
		t.Fatal("balanced workload should not need rebalancing")
	}

	imbalanced := []float64{0.1, 0.1, 0.9, 0.9}
	if !NeedsRebalancing(imbalanced, config) {
		t.Fatal("imbalanced workload should need rebalancing")
	}
}

func TestIsValidPriority(t *testing.T) {
	if !IsValidPriority(PriorityCritical) {
		t.Fatal("critical should be valid")
	}
	if !IsValidPriority(PriorityHigh) {
		t.Fatal("high should be valid")
	}
	if IsValidPriority("invalid") {
		t.Fatal("invalid should not be valid")
	}
}

func TestIsValidComplexity(t *testing.T) {
	if !IsValidComplexity(ComplexityEpic) {
		t.Fatal("epic should be valid")
	}
	if IsValidComplexity("unknown") {
		t.Fatal("unknown should not be valid")
	}
}

func TestIsValidStatus(t *testing.T) {
	if !IsValidStatus(StatusActive) {
		t.Fatal("active should be valid")
	}
	if IsValidStatus("unknown") {
		t.Fatal("unknown should not be valid")
	}
}

func TestClaimantUtilization(t *testing.T) {
	c := NewClaimant("a1", "Agent", ClaimantTypeAgent)
	if c.Utilization() != 0.0 {
		t.Fatal("empty workload should be 0 utilization")
	}

	c.IncrementWorkload()
	expected := 1.0 / float64(c.MaxConcurrent)
	if c.Utilization() != expected {
		t.Fatalf("expected %f, got %f", expected, c.Utilization())
	}
}

func TestIssue(t *testing.T) {
	issue := NewIssue("i1", "Bug", "Fix it")
	if issue.IsCritical() {
		t.Fatal("default priority should not be critical")
	}

	issue.Priority = PriorityCritical
	if !issue.IsCritical() {
		t.Fatal("critical issue should be critical")
	}
	if !issue.IsHighPriority() {
		t.Fatal("critical should also be high priority")
	}

	issue.AddLabel("bug")
	if !issue.HasLabel("bug") {
		t.Fatal("should have bug label")
	}
	issue.RemoveLabel("bug")
	if issue.HasLabel("bug") {
		t.Fatal("should not have bug label after removal")
	}
}

func TestPriorityWeight(t *testing.T) {
	if PriorityCritical.Weight() <= PriorityHigh.Weight() {
		t.Fatal("critical should outweigh high")
	}
	if PriorityHigh.Weight() <= PriorityMedium.Weight() {
		t.Fatal("high should outweigh medium")
	}
	if PriorityMedium.Weight() <= PriorityLow.Weight() {
		t.Fatal("medium should outweigh low")
	}
}

func TestComplexityEstimatedHours(t *testing.T) {
	if ComplexityTrivial.EstimatedHours() >= ComplexitySimple.EstimatedHours() {
		t.Fatal("trivial should be fewer hours than simple")
	}
	if ComplexityEpic.EstimatedHours() <= ComplexityComplex.EstimatedHours() {
		t.Fatal("epic should be more hours than complex")
	}
}
