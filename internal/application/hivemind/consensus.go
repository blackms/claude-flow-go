// Package hivemind provides the Hive Mind consensus system for multi-agent coordination.
package hivemind

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ConsensusEngine handles consensus calculation and voting.
type ConsensusEngine struct {
	config shared.HiveMindConfig
}

// NewConsensusEngine creates a new Consensus Engine.
func NewConsensusEngine(config shared.HiveMindConfig) *ConsensusEngine {
	return &ConsensusEngine{
		config: config,
	}
}

// CalculateResult calculates the consensus result from votes.
func (ce *ConsensusEngine) CalculateResult(votes []shared.WeightedVote, consensusType shared.ConsensusType, requiredQuorum float64) *shared.ProposalResult {
	result := &shared.ProposalResult{
		TotalVotes:        len(votes),
		ApprovalVotes:     0,
		RejectionVotes:    0,
		WeightedApproval:  0,
		WeightedRejection: 0,
		QuorumReached:     false,
		ConsensusReached:  false,
		Votes:             votes,
	}

	if len(votes) == 0 {
		return result
	}

	// Calculate vote counts and weighted scores
	var totalWeight float64
	var approvalWeight float64
	var rejectionWeight float64

	for _, vote := range votes {
		weight := vote.Weight
		if weight < ce.config.MinVoteWeight {
			weight = ce.config.MinVoteWeight
		}
		totalWeight += weight

		if vote.Vote {
			result.ApprovalVotes++
			approvalWeight += weight
		} else {
			result.RejectionVotes++
			rejectionWeight += weight
		}
	}

	// Calculate weighted percentages
	if totalWeight > 0 {
		result.WeightedApproval = approvalWeight / totalWeight
		result.WeightedRejection = rejectionWeight / totalWeight
	}

	// Check quorum
	result.QuorumReached = float64(result.TotalVotes) >= requiredQuorum*float64(ce.getExpectedVoters())

	// Calculate consensus based on type
	result.ConsensusReached = ce.isConsensusReached(result, consensusType)

	return result
}

// getExpectedVoters returns the expected number of voters (15 for full hierarchy).
func (ce *ConsensusEngine) getExpectedVoters() int {
	return 15 // 15-agent hierarchy
}

// isConsensusReached checks if consensus is reached based on the consensus type.
func (ce *ConsensusEngine) isConsensusReached(result *shared.ProposalResult, consensusType shared.ConsensusType) bool {
	switch consensusType {
	case shared.ConsensusTypeMajority:
		// Simple majority: > 50%
		return result.WeightedApproval > 0.5

	case shared.ConsensusTypeSuperMajority:
		// Supermajority: >= 67%
		return result.WeightedApproval >= 0.67

	case shared.ConsensusTypeUnanimous:
		// Unanimous: 100% approval
		return result.RejectionVotes == 0 && result.ApprovalVotes > 0

	case shared.ConsensusTypeWeighted:
		// Weighted: Based on weighted approval > 50%
		return result.WeightedApproval > 0.5

	case shared.ConsensusTypeQueenOverride:
		// Queen override: Always passes (Queen has authority)
		return true

	default:
		// Default to majority
		return result.WeightedApproval > 0.5
	}
}

// GetThreshold returns the required threshold for a consensus type.
func (ce *ConsensusEngine) GetThreshold(consensusType shared.ConsensusType) float64 {
	switch consensusType {
	case shared.ConsensusTypeMajority:
		return 0.51
	case shared.ConsensusTypeSuperMajority:
		return 0.67
	case shared.ConsensusTypeUnanimous:
		return 1.0
	case shared.ConsensusTypeWeighted:
		return 0.51
	case shared.ConsensusTypeQueenOverride:
		return 0.0 // No threshold for queen override
	default:
		return 0.51
	}
}

// CalculateAgentWeight calculates the weight for an agent based on performance.
func (ce *ConsensusEngine) CalculateAgentWeight(healthScore, performanceScore, successRate float64) float64 {
	// Weighted average of factors
	// Health: 40%, Performance: 35%, Success Rate: 25%
	weight := healthScore*0.4 + performanceScore*0.35 + successRate*0.25

	// Clamp to valid range
	if weight < ce.config.MinVoteWeight {
		weight = ce.config.MinVoteWeight
	}
	if weight > 1.0 {
		weight = 1.0
	}

	return weight
}

// SimulateVote simulates a vote for an agent based on proposal analysis.
// In a real implementation, this would query the agent for their actual vote.
func (ce *ConsensusEngine) SimulateVote(agentID string, proposal *shared.Proposal, weight float64) shared.WeightedVote {
	// Default to approval with high confidence
	// In real implementation, agent would analyze proposal and vote accordingly
	return shared.WeightedVote{
		AgentID:    agentID,
		ProposalID: proposal.ID,
		Vote:       true,
		Weight:     weight,
		Confidence: 0.8,
		Reason:     "Automated approval based on proposal analysis",
		Timestamp:  shared.Now(),
	}
}

// VoteResult represents the result of a single vote calculation.
type VoteResult struct {
	Approved   bool
	Weight     float64
	Confidence float64
}

// AnalyzeVotes analyzes votes and provides detailed breakdown.
func (ce *ConsensusEngine) AnalyzeVotes(votes []shared.WeightedVote) VoteAnalysis {
	analysis := VoteAnalysis{
		TotalVotes:       len(votes),
		ApprovalCount:    0,
		RejectionCount:   0,
		TotalWeight:      0,
		ApprovalWeight:   0,
		RejectionWeight:  0,
		AverageWeight:    0,
		AverageConfidence: 0,
		WeightDistribution: make(map[string]float64),
	}

	if len(votes) == 0 {
		return analysis
	}

	var totalConfidence float64

	for _, vote := range votes {
		analysis.TotalWeight += vote.Weight
		totalConfidence += vote.Confidence

		if vote.Vote {
			analysis.ApprovalCount++
			analysis.ApprovalWeight += vote.Weight
		} else {
			analysis.RejectionCount++
			analysis.RejectionWeight += vote.Weight
		}

		// Track by agent
		analysis.WeightDistribution[vote.AgentID] = vote.Weight
	}

	analysis.AverageWeight = analysis.TotalWeight / float64(len(votes))
	analysis.AverageConfidence = totalConfidence / float64(len(votes))

	return analysis
}

// VoteAnalysis provides detailed analysis of votes.
type VoteAnalysis struct {
	TotalVotes         int                `json:"totalVotes"`
	ApprovalCount      int                `json:"approvalCount"`
	RejectionCount     int                `json:"rejectionCount"`
	TotalWeight        float64            `json:"totalWeight"`
	ApprovalWeight     float64            `json:"approvalWeight"`
	RejectionWeight    float64            `json:"rejectionWeight"`
	AverageWeight      float64            `json:"averageWeight"`
	AverageConfidence  float64            `json:"averageConfidence"`
	WeightDistribution map[string]float64 `json:"weightDistribution"`
}

// ConsensusTypeInfo provides information about a consensus type.
type ConsensusTypeInfo struct {
	Type        shared.ConsensusType `json:"type"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Threshold   float64              `json:"threshold"`
}

// GetConsensusTypeInfo returns information about all consensus types.
func GetConsensusTypeInfo() []ConsensusTypeInfo {
	return []ConsensusTypeInfo{
		{
			Type:        shared.ConsensusTypeMajority,
			Name:        "Majority",
			Description: "Simple majority voting (>50%)",
			Threshold:   0.51,
		},
		{
			Type:        shared.ConsensusTypeSuperMajority,
			Name:        "Supermajority",
			Description: "Supermajority voting (>=67%)",
			Threshold:   0.67,
		},
		{
			Type:        shared.ConsensusTypeUnanimous,
			Name:        "Unanimous",
			Description: "Unanimous agreement required (100%)",
			Threshold:   1.0,
		},
		{
			Type:        shared.ConsensusTypeWeighted,
			Name:        "Weighted",
			Description: "Weighted voting based on agent performance",
			Threshold:   0.51,
		},
		{
			Type:        shared.ConsensusTypeQueenOverride,
			Name:        "Queen Override",
			Description: "Queen has unilateral authority for emergencies",
			Threshold:   0.0,
		},
	}
}
