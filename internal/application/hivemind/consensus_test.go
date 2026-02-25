package hivemind

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func newTestEngine() *ConsensusEngine {
	return NewConsensusEngine(shared.HiveMindConfig{
		ConsensusAlgorithm: shared.ConsensusTypeMajority,
		VoteTimeout:        30000,
		MaxProposals:       100,
		EnableLearning:     true,
		DefaultQuorum:      0.5,
		MinVoteWeight:      0.1,
	})
}

func makeVotes(approve, reject int, weight float64) []shared.WeightedVote {
	votes := make([]shared.WeightedVote, 0, approve+reject)
	for i := 0; i < approve; i++ {
		votes = append(votes, shared.WeightedVote{
			AgentID:    shared.GenerateID("agent"),
			Vote:       true,
			Weight:     weight,
			Confidence: 0.9,
		})
	}
	for i := 0; i < reject; i++ {
		votes = append(votes, shared.WeightedVote{
			AgentID:    shared.GenerateID("agent"),
			Vote:       false,
			Weight:     weight,
			Confidence: 0.9,
		})
	}
	return votes
}

func TestCalculateResultMajority(t *testing.T) {
	engine := newTestEngine()

	votes := makeVotes(10, 5, 1.0)
	result := engine.CalculateResult(votes, shared.ConsensusTypeMajority, 0.5)

	if result.ApprovalVotes != 10 {
		t.Fatalf("expected 10 approvals, got %d", result.ApprovalVotes)
	}
	if result.RejectionVotes != 5 {
		t.Fatalf("expected 5 rejections, got %d", result.RejectionVotes)
	}
	if !result.ConsensusReached {
		t.Fatal("majority should be reached with 10/15 votes")
	}
}

func TestCalculateResultMajorityNotReached(t *testing.T) {
	engine := newTestEngine()

	votes := makeVotes(5, 10, 1.0)
	result := engine.CalculateResult(votes, shared.ConsensusTypeMajority, 0.5)

	if result.ConsensusReached {
		t.Fatal("majority should not be reached with 5/15 votes")
	}
}

func TestCalculateResultSuperMajority(t *testing.T) {
	engine := newTestEngine()

	votes := makeVotes(11, 4, 1.0)
	result := engine.CalculateResult(votes, shared.ConsensusTypeSuperMajority, 0.5)

	if !result.ConsensusReached {
		t.Fatal("supermajority should be reached with 11/15 (~73%)")
	}
}

func TestCalculateResultSuperMajorityNotReached(t *testing.T) {
	engine := newTestEngine()

	votes := makeVotes(9, 6, 1.0)
	result := engine.CalculateResult(votes, shared.ConsensusTypeSuperMajority, 0.5)

	if result.ConsensusReached {
		t.Fatal("supermajority should not be reached with 9/15 (60%)")
	}
}

func TestCalculateResultUnanimous(t *testing.T) {
	engine := newTestEngine()

	votes := makeVotes(15, 0, 1.0)
	result := engine.CalculateResult(votes, shared.ConsensusTypeUnanimous, 0.5)

	if !result.ConsensusReached {
		t.Fatal("unanimous should be reached with 15/15")
	}
}

func TestCalculateResultUnanimousNotReached(t *testing.T) {
	engine := newTestEngine()

	votes := makeVotes(14, 1, 1.0)
	result := engine.CalculateResult(votes, shared.ConsensusTypeUnanimous, 0.5)

	if result.ConsensusReached {
		t.Fatal("unanimous should not be reached with 1 rejection")
	}
}

func TestCalculateResultQueenOverride(t *testing.T) {
	engine := newTestEngine()

	votes := makeVotes(1, 14, 1.0)
	result := engine.CalculateResult(votes, shared.ConsensusTypeQueenOverride, 0.0)

	if !result.ConsensusReached {
		t.Fatal("queen override should always pass")
	}
}

func TestCalculateResultEmptyVotes(t *testing.T) {
	engine := newTestEngine()

	result := engine.CalculateResult(nil, shared.ConsensusTypeMajority, 0.5)

	if result.TotalVotes != 0 {
		t.Fatal("empty votes should have 0 total")
	}
	if result.ConsensusReached {
		t.Fatal("empty votes should not reach consensus")
	}
}

func TestCalculateResultWeightedVoting(t *testing.T) {
	engine := newTestEngine()

	votes := []shared.WeightedVote{
		{AgentID: "queen", Vote: true, Weight: 5.0, Confidence: 1.0},
		{AgentID: "worker-1", Vote: false, Weight: 1.0, Confidence: 0.8},
		{AgentID: "worker-2", Vote: false, Weight: 1.0, Confidence: 0.8},
	}

	result := engine.CalculateResult(votes, shared.ConsensusTypeWeighted, 0.0)

	if !result.ConsensusReached {
		t.Fatal("weighted majority should pass when queen has high weight")
	}
	if result.WeightedApproval <= 0.5 {
		t.Fatalf("weighted approval should be > 50%%, got %.2f", result.WeightedApproval)
	}
}

func TestGetThreshold(t *testing.T) {
	engine := newTestEngine()

	tests := []struct {
		cType     shared.ConsensusType
		threshold float64
	}{
		{shared.ConsensusTypeMajority, 0.51},
		{shared.ConsensusTypeSuperMajority, 0.67},
		{shared.ConsensusTypeUnanimous, 1.0},
		{shared.ConsensusTypeQueenOverride, 0.0},
	}

	for _, tt := range tests {
		threshold := engine.GetThreshold(tt.cType)
		if threshold != tt.threshold {
			t.Errorf("GetThreshold(%s) = %f, want %f", tt.cType, threshold, tt.threshold)
		}
	}
}

func TestCalculateAgentWeight(t *testing.T) {
	engine := newTestEngine()

	weight := engine.CalculateAgentWeight(1.0, 1.0, 1.0)
	if weight != 1.0 {
		t.Fatalf("perfect scores should give weight 1.0, got %f", weight)
	}

	weight = engine.CalculateAgentWeight(0.0, 0.0, 0.0)
	if weight != engine.config.MinVoteWeight {
		t.Fatalf("zero scores should give minimum weight, got %f", weight)
	}

	weight = engine.CalculateAgentWeight(0.8, 0.6, 0.9)
	if weight <= 0 || weight > 1.0 {
		t.Fatalf("weight out of range: %f", weight)
	}
}

func TestAnalyzeVotes(t *testing.T) {
	engine := newTestEngine()

	votes := makeVotes(8, 7, 1.0)
	analysis := engine.AnalyzeVotes(votes)

	if analysis.TotalVotes != 15 {
		t.Fatalf("expected 15 total votes, got %d", analysis.TotalVotes)
	}
	if analysis.ApprovalCount != 8 {
		t.Fatalf("expected 8 approvals, got %d", analysis.ApprovalCount)
	}
	if analysis.RejectionCount != 7 {
		t.Fatalf("expected 7 rejections, got %d", analysis.RejectionCount)
	}
	if analysis.AverageWeight != 1.0 {
		t.Fatalf("expected average weight 1.0, got %f", analysis.AverageWeight)
	}
	if analysis.AverageConfidence < 0.89 || analysis.AverageConfidence > 0.91 {
		t.Fatalf("expected average confidence ~0.9, got %f", analysis.AverageConfidence)
	}
}

func TestAnalyzeVotesEmpty(t *testing.T) {
	engine := newTestEngine()

	analysis := engine.AnalyzeVotes(nil)
	if analysis.TotalVotes != 0 {
		t.Fatal("empty votes should have 0 total")
	}
	if analysis.AverageWeight != 0 {
		t.Fatal("empty votes should have 0 average weight")
	}
}

func TestGetConsensusTypeInfo(t *testing.T) {
	infos := GetConsensusTypeInfo()
	if len(infos) != 5 {
		t.Fatalf("expected 5 consensus types, got %d", len(infos))
	}

	types := make(map[shared.ConsensusType]bool)
	for _, info := range infos {
		types[info.Type] = true
		if info.Name == "" {
			t.Fatal("consensus type info should have a name")
		}
		if info.Description == "" {
			t.Fatal("consensus type info should have a description")
		}
	}

	if !types[shared.ConsensusTypeMajority] {
		t.Fatal("should include majority type")
	}
	if !types[shared.ConsensusTypeQueenOverride] {
		t.Fatal("should include queen override type")
	}
}

func TestQuorumCheck(t *testing.T) {
	engine := newTestEngine()

	votes := makeVotes(8, 0, 1.0)
	result := engine.CalculateResult(votes, shared.ConsensusTypeMajority, 0.5)

	if !result.QuorumReached {
		t.Fatal("8/15 voters should meet 50% quorum")
	}

	smallVotes := makeVotes(2, 0, 1.0)
	result = engine.CalculateResult(smallVotes, shared.ConsensusTypeMajority, 0.5)

	if result.QuorumReached {
		t.Fatal("2/15 voters should not meet 50% quorum")
	}
}
