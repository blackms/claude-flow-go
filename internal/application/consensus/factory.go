// Package consensus provides distributed consensus algorithms for multi-agent coordination.
package consensus

import (
	"context"
	"fmt"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Engine is a unified consensus engine that wraps different consensus algorithms.
type Engine struct {
	nodeID         string
	algorithmType  shared.ConsensusAlgorithmType
	implementation Algorithm
}

// NewEngine creates a new consensus engine with the specified algorithm.
func NewEngine(nodeID string, algorithmType shared.ConsensusAlgorithmType) (*Engine, error) {
	engine := &Engine{
		nodeID:        nodeID,
		algorithmType: algorithmType,
	}

	switch algorithmType {
	case shared.AlgorithmRaft:
		engine.implementation = NewRaftConsensus(nodeID, shared.DefaultRaftConfig())
	case shared.AlgorithmByzantine:
		engine.implementation = NewByzantineConsensus(nodeID, shared.DefaultByzantineConfig())
	case shared.AlgorithmGossip:
		engine.implementation = NewGossipConsensus(nodeID, shared.DefaultGossipConfig())
	case shared.AlgorithmPaxos:
		// Fall back to Raft for Paxos (similar guarantees)
		engine.implementation = NewRaftConsensus(nodeID, shared.DefaultRaftConfig())
		engine.algorithmType = shared.AlgorithmRaft
	default:
		return nil, fmt.Errorf("unknown consensus algorithm: %s", algorithmType)
	}

	return engine, nil
}

// NewEngineWithConfig creates a new consensus engine with custom configuration.
func NewEngineWithConfig(nodeID string, algorithmType shared.ConsensusAlgorithmType, config interface{}) (*Engine, error) {
	engine := &Engine{
		nodeID:        nodeID,
		algorithmType: algorithmType,
	}

	switch algorithmType {
	case shared.AlgorithmRaft:
		if cfg, ok := config.(shared.RaftConfig); ok {
			engine.implementation = NewRaftConsensus(nodeID, cfg)
		} else {
			engine.implementation = NewRaftConsensus(nodeID, shared.DefaultRaftConfig())
		}
	case shared.AlgorithmByzantine:
		if cfg, ok := config.(shared.ByzantineConfig); ok {
			engine.implementation = NewByzantineConsensus(nodeID, cfg)
		} else {
			engine.implementation = NewByzantineConsensus(nodeID, shared.DefaultByzantineConfig())
		}
	case shared.AlgorithmGossip:
		if cfg, ok := config.(shared.GossipConfig); ok {
			engine.implementation = NewGossipConsensus(nodeID, cfg)
		} else {
			engine.implementation = NewGossipConsensus(nodeID, shared.DefaultGossipConfig())
		}
	default:
		return nil, fmt.Errorf("unknown consensus algorithm: %s", algorithmType)
	}

	return engine, nil
}

// Initialize initializes the consensus engine.
func (e *Engine) Initialize(ctx context.Context) error {
	return e.implementation.Initialize(ctx)
}

// Shutdown shuts down the consensus engine.
func (e *Engine) Shutdown() error {
	return e.implementation.Shutdown()
}

// AddNode adds a node to the consensus cluster.
func (e *Engine) AddNode(nodeID string, opts ...NodeOption) error {
	return e.implementation.AddNode(nodeID, opts...)
}

// RemoveNode removes a node from the consensus cluster.
func (e *Engine) RemoveNode(nodeID string) error {
	return e.implementation.RemoveNode(nodeID)
}

// Propose proposes a value for consensus.
func (e *Engine) Propose(ctx context.Context, value interface{}) (*shared.ConsensusProposal, error) {
	return e.implementation.Propose(ctx, value)
}

// Vote submits a vote for a proposal.
func (e *Engine) Vote(ctx context.Context, proposalID string, vote shared.ConsensusVote) error {
	return e.implementation.Vote(ctx, proposalID, vote)
}

// AwaitConsensus waits for consensus on a proposal.
func (e *Engine) AwaitConsensus(ctx context.Context, proposalID string) (*shared.DistributedConsensusResult, error) {
	return e.implementation.AwaitConsensus(ctx, proposalID)
}

// GetProposal returns a proposal by ID.
func (e *Engine) GetProposal(proposalID string) (*shared.ConsensusProposal, bool) {
	return e.implementation.GetProposal(proposalID)
}

// GetActiveProposals returns all active proposals.
func (e *Engine) GetActiveProposals() []*shared.ConsensusProposal {
	return e.implementation.GetActiveProposals()
}

// GetStats returns algorithm statistics.
func (e *Engine) GetStats() shared.AlgorithmStats {
	return e.implementation.GetStats()
}

// GetAlgorithmType returns the algorithm type.
func (e *Engine) GetAlgorithmType() shared.ConsensusAlgorithmType {
	return e.algorithmType
}

// IsLeader returns true if this node is the leader (for Raft/BFT).
func (e *Engine) IsLeader() bool {
	if raft, ok := e.implementation.(*RaftConsensus); ok {
		return raft.IsLeader()
	}
	if bft, ok := e.implementation.(*ByzantineConsensus); ok {
		return bft.IsPrimary()
	}
	return false // Gossip has no leader
}

// GetLeaderID returns the leader ID (for Raft).
func (e *Engine) GetLeaderID() string {
	if raft, ok := e.implementation.(*RaftConsensus); ok {
		return raft.GetLeaderID()
	}
	return ""
}

// GetImplementation returns the underlying implementation.
func (e *Engine) GetImplementation() Algorithm {
	return e.implementation
}

// SelectOptimalAlgorithm selects the optimal consensus algorithm based on requirements.
func SelectOptimalAlgorithm(opts shared.AlgorithmSelectionOptions) shared.ConsensusAlgorithmType {
	// Byzantine fault tolerance required
	if opts.FaultTolerance == shared.FaultToleranceByzantine {
		return shared.AlgorithmByzantine
	}

	// Eventual consistency acceptable and large scale
	if opts.Consistency == shared.ConsistencyEventual && opts.NetworkScale == shared.NetworkScaleLarge {
		return shared.AlgorithmGossip
	}

	// High latency priority with small/medium scale
	if opts.LatencyPriority == shared.LatencyPriorityHigh && opts.NetworkScale != shared.NetworkScaleLarge {
		return shared.AlgorithmRaft
	}

	// Strong consistency with any scale
	if opts.Consistency == shared.ConsistencyStrong {
		return shared.AlgorithmRaft
	}

	// Default to Raft
	return shared.AlgorithmRaft
}

// AlgorithmInfo provides information about a consensus algorithm.
type AlgorithmInfo struct {
	Type            shared.ConsensusAlgorithmType `json:"type"`
	Name            string                        `json:"name"`
	Description     string                        `json:"description"`
	FaultTolerance  string                        `json:"faultTolerance"`
	Consistency     string                        `json:"consistency"`
	Latency         string                        `json:"latency"`
	RecommendedFor  string                        `json:"recommendedFor"`
}

// GetAlgorithmInfo returns information about all available algorithms.
func GetAlgorithmInfo() []AlgorithmInfo {
	return []AlgorithmInfo{
		{
			Type:           shared.AlgorithmRaft,
			Name:           "Raft",
			Description:    "Leader-based consensus with strong consistency",
			FaultTolerance: "Crash faults (f failures with 2f+1 nodes)",
			Consistency:    "Strong (linearizable)",
			Latency:        "Low (<100ms)",
			RecommendedFor: "Small to medium clusters requiring strong consistency",
		},
		{
			Type:           shared.AlgorithmByzantine,
			Name:           "Byzantine (PBFT)",
			Description:    "Byzantine fault-tolerant consensus using PBFT",
			FaultTolerance: "Byzantine faults (f failures with 3f+1 nodes)",
			Consistency:    "Strong (linearizable)",
			Latency:        "Medium (<200ms)",
			RecommendedFor: "Adversarial environments with untrusted nodes",
		},
		{
			Type:           shared.AlgorithmGossip,
			Name:           "Gossip",
			Description:    "Epidemic protocol for eventual consistency",
			FaultTolerance: "Partition tolerant",
			Consistency:    "Eventual",
			Latency:        "Variable (<500ms for convergence)",
			RecommendedFor: "Large-scale systems (100+ nodes) where eventual consistency is acceptable",
		},
	}
}

// CreateRaft creates a Raft consensus instance with the given config.
func CreateRaft(nodeID string, config shared.RaftConfig) *RaftConsensus {
	return NewRaftConsensus(nodeID, config)
}

// CreateByzantine creates a Byzantine consensus instance with the given config.
func CreateByzantine(nodeID string, config shared.ByzantineConfig) *ByzantineConsensus {
	return NewByzantineConsensus(nodeID, config)
}

// CreateGossip creates a Gossip consensus instance with the given config.
func CreateGossip(nodeID string, config shared.GossipConfig) *GossipConsensus {
	return NewGossipConsensus(nodeID, config)
}
