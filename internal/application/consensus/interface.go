// Package consensus provides distributed consensus algorithms for multi-agent coordination.
package consensus

import (
	"context"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Algorithm defines the interface for consensus algorithms.
type Algorithm interface {
	// Initialize initializes the consensus algorithm.
	Initialize(ctx context.Context) error

	// Shutdown shuts down the consensus algorithm.
	Shutdown() error

	// AddNode adds a node to the consensus cluster.
	AddNode(nodeID string, opts ...NodeOption) error

	// RemoveNode removes a node from the consensus cluster.
	RemoveNode(nodeID string) error

	// Propose proposes a value for consensus.
	Propose(ctx context.Context, value interface{}) (*shared.ConsensusProposal, error)

	// Vote submits a vote for a proposal.
	Vote(ctx context.Context, proposalID string, vote shared.ConsensusVote) error

	// AwaitConsensus waits for consensus on a proposal.
	AwaitConsensus(ctx context.Context, proposalID string) (*shared.DistributedConsensusResult, error)

	// GetProposal returns a proposal by ID.
	GetProposal(proposalID string) (*shared.ConsensusProposal, bool)

	// GetActiveProposals returns all active proposals.
	GetActiveProposals() []*shared.ConsensusProposal

	// GetStats returns algorithm statistics.
	GetStats() shared.AlgorithmStats

	// GetAlgorithmType returns the algorithm type.
	GetAlgorithmType() shared.ConsensusAlgorithmType
}

// NodeOption is a functional option for adding nodes.
type NodeOption func(*NodeOptions)

// NodeOptions holds options for adding a node.
type NodeOptions struct {
	IsPrimary bool
	Weight    float64
}

// WithPrimary sets the node as primary (for Byzantine consensus).
func WithPrimary(isPrimary bool) NodeOption {
	return func(o *NodeOptions) {
		o.IsPrimary = isPrimary
	}
}

// WithWeight sets the node weight.
func WithWeight(weight float64) NodeOption {
	return func(o *NodeOptions) {
		o.Weight = weight
	}
}

// ApplyNodeOptions applies node options.
func ApplyNodeOptions(opts ...NodeOption) NodeOptions {
	options := NodeOptions{
		IsPrimary: false,
		Weight:    1.0,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// BaseAlgorithm provides common functionality for consensus algorithms.
type BaseAlgorithm struct {
	NodeID    string
	Proposals map[string]*shared.ConsensusProposal
	Stats     shared.AlgorithmStats
}

// NewBaseAlgorithm creates a new base algorithm.
func NewBaseAlgorithm(nodeID string, algorithmType shared.ConsensusAlgorithmType) *BaseAlgorithm {
	return &BaseAlgorithm{
		NodeID:    nodeID,
		Proposals: make(map[string]*shared.ConsensusProposal),
		Stats: shared.AlgorithmStats{
			Algorithm: algorithmType,
		},
	}
}

// GetProposal returns a proposal by ID.
func (ba *BaseAlgorithm) GetProposal(proposalID string) (*shared.ConsensusProposal, bool) {
	proposal, exists := ba.Proposals[proposalID]
	return proposal, exists
}

// GetActiveProposals returns all active proposals.
func (ba *BaseAlgorithm) GetActiveProposals() []*shared.ConsensusProposal {
	result := make([]*shared.ConsensusProposal, 0)
	for _, p := range ba.Proposals {
		if p.Status == "pending" {
			result = append(result, p)
		}
	}
	return result
}

// GetStats returns algorithm statistics.
func (ba *BaseAlgorithm) GetStats() shared.AlgorithmStats {
	return ba.Stats
}

// UpdateStats updates the statistics based on proposals.
func (ba *BaseAlgorithm) UpdateStats() {
	ba.Stats.TotalProposals = len(ba.Proposals)
	ba.Stats.PendingProposals = 0
	ba.Stats.AcceptedProposals = 0
	ba.Stats.RejectedProposals = 0
	ba.Stats.ExpiredProposals = 0

	for _, p := range ba.Proposals {
		switch p.Status {
		case "pending":
			ba.Stats.PendingProposals++
		case "accepted":
			ba.Stats.AcceptedProposals++
		case "rejected":
			ba.Stats.RejectedProposals++
		case "expired":
			ba.Stats.ExpiredProposals++
		}
	}
}

// CreateResult creates a consensus result from a proposal.
func (ba *BaseAlgorithm) CreateResult(proposal *shared.ConsensusProposal, durationMs int64, totalNodes int, rounds int) *shared.DistributedConsensusResult {
	approvingVotes := 0
	if proposal.Votes != nil {
		for _, v := range proposal.Votes {
			if vote, ok := v.(map[string]interface{}); ok {
				if approve, ok := vote["approve"].(bool); ok && approve {
					approvingVotes++
				}
			}
		}
	}

	voteCount := len(proposal.Votes)
	approvalRate := 0.0
	if voteCount > 0 {
		approvalRate = float64(approvingVotes) / float64(voteCount)
	}

	participationRate := 0.0
	if totalNodes > 0 {
		participationRate = float64(voteCount) / float64(totalNodes)
	}

	return &shared.DistributedConsensusResult{
		ProposalID:        proposal.ID,
		Approved:          proposal.Status == "accepted",
		ApprovalRate:      approvalRate,
		ParticipationRate: participationRate,
		FinalValue:        proposal.Value,
		Rounds:            rounds,
		DurationMs:        durationMs,
	}
}
