// Package consensus provides distributed consensus algorithms for multi-agent coordination.
package consensus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
	"github.com/google/uuid"
)

// ByzantineConsensus implements PBFT-style Byzantine fault-tolerant consensus.
type ByzantineConsensus struct {
	*BaseAlgorithm
	config shared.ByzantineConfig
	node   *shared.ByzantineNode
	nodes  map[string]*shared.ByzantineNode
	mu     sync.RWMutex

	// Message logs for each sequence number
	messageLog        map[string][]shared.ByzantineMessage
	preparedMessages  map[string][]shared.ByzantineMessage
	committedMessages map[string][]shared.ByzantineMessage

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Counters
	proposalCounter int64
}

// NewByzantineConsensus creates a new Byzantine consensus instance.
func NewByzantineConsensus(nodeID string, config shared.ByzantineConfig) *ByzantineConsensus {
	ctx, cancel := context.WithCancel(context.Background())

	bc := &ByzantineConsensus{
		BaseAlgorithm: NewBaseAlgorithm(nodeID, shared.AlgorithmByzantine),
		config:        config,
		node: &shared.ByzantineNode{
			ID:             nodeID,
			IsPrimary:      false,
			ViewNumber:     0,
			SequenceNumber: 0,
		},
		nodes:             make(map[string]*shared.ByzantineNode),
		messageLog:        make(map[string][]shared.ByzantineMessage),
		preparedMessages:  make(map[string][]shared.ByzantineMessage),
		committedMessages: make(map[string][]shared.ByzantineMessage),
		ctx:               ctx,
		cancel:            cancel,
	}

	return bc
}

// Initialize initializes the Byzantine consensus.
func (bc *ByzantineConsensus) Initialize(ctx context.Context) error {
	return nil
}

// Shutdown shuts down the Byzantine consensus.
func (bc *ByzantineConsensus) Shutdown() error {
	bc.cancel()
	return nil
}

// AddNode adds a node to the Byzantine cluster.
func (bc *ByzantineConsensus) AddNode(nodeID string, opts ...NodeOption) error {
	options := ApplyNodeOptions(opts...)

	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.nodes[nodeID] = &shared.ByzantineNode{
		ID:             nodeID,
		IsPrimary:      options.IsPrimary,
		ViewNumber:     0,
		SequenceNumber: 0,
	}

	if options.IsPrimary && nodeID == bc.node.ID {
		bc.node.IsPrimary = true
	}

	return nil
}

// RemoveNode removes a node from the Byzantine cluster.
func (bc *ByzantineConsensus) RemoveNode(nodeID string) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	delete(bc.nodes, nodeID)
	return nil
}

// ElectPrimary elects a primary based on view number.
func (bc *ByzantineConsensus) ElectPrimary() string {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	nodeIDs := []string{bc.node.ID}
	for id := range bc.nodes {
		nodeIDs = append(nodeIDs, id)
	}

	primaryIndex := int(bc.node.ViewNumber) % len(nodeIDs)
	primaryID := nodeIDs[primaryIndex]

	bc.node.IsPrimary = primaryID == bc.node.ID

	for id, node := range bc.nodes {
		node.IsPrimary = id == primaryID
	}

	return primaryID
}

// Propose proposes a value for consensus.
func (bc *ByzantineConsensus) Propose(ctx context.Context, value interface{}) (*shared.ConsensusProposal, error) {
	bc.mu.Lock()

	if !bc.node.IsPrimary {
		bc.mu.Unlock()
		return nil, fmt.Errorf("only primary can propose values")
	}

	bc.proposalCounter++
	bc.node.SequenceNumber++
	sequenceNumber := bc.node.SequenceNumber
	digest := bc.computeDigest(value)
	proposalID := fmt.Sprintf("bft_%d_%d", bc.node.ViewNumber, sequenceNumber)

	proposal := &shared.ConsensusProposal{
		ID:         proposalID,
		ProposerID: bc.node.ID,
		Value:      value,
		Term:       bc.node.ViewNumber,
		Timestamp:  shared.Now(),
		Status:     "pending",
		Votes:      make(map[string]interface{}),
	}

	bc.Proposals[proposalID] = proposal

	// Phase 1: Pre-prepare
	prePrepareMsg := shared.ByzantineMessage{
		Type:           shared.ByzantinePhasePrePrepare,
		ViewNumber:     bc.node.ViewNumber,
		SequenceNumber: sequenceNumber,
		Digest:         digest,
		SenderID:       bc.node.ID,
		Timestamp:      shared.Now(),
		Payload:        value,
	}

	bc.mu.Unlock()

	// Broadcast pre-prepare
	bc.broadcastMessage(prePrepareMsg)

	// Self-prepare
	bc.handlePrepare(shared.ByzantineMessage{
		Type:           shared.ByzantinePhasePrepare,
		ViewNumber:     bc.node.ViewNumber,
		SequenceNumber: sequenceNumber,
		Digest:         digest,
		SenderID:       bc.node.ID,
		Timestamp:      shared.Now(),
	})

	return proposal, nil
}

// Vote submits a vote for a proposal.
func (bc *ByzantineConsensus) Vote(ctx context.Context, proposalID string, vote shared.ConsensusVote) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	proposal, exists := bc.Proposals[proposalID]
	if !exists || proposal.Status != "pending" {
		return nil
	}

	proposal.Votes[vote.VoterID] = map[string]interface{}{
		"voterId":    vote.VoterID,
		"approve":    vote.Approve,
		"confidence": vote.Confidence,
		"timestamp":  vote.Timestamp,
	}

	// Check BFT consensus (2f+1 votes)
	f := bc.config.MaxFaultyNodes
	n := len(bc.nodes) + 1
	requiredVotes := 2*f + 1

	approvingVotes := 0
	for _, v := range proposal.Votes {
		if voteMap, ok := v.(map[string]interface{}); ok {
			if approve, ok := voteMap["approve"].(bool); ok && approve {
				approvingVotes++
			}
		}
	}

	if approvingVotes >= requiredVotes {
		proposal.Status = "accepted"
	} else if len(proposal.Votes) >= n && approvingVotes < requiredVotes {
		proposal.Status = "rejected"
	}

	bc.UpdateStats()
	return nil
}

// AwaitConsensus waits for consensus on a proposal.
func (bc *ByzantineConsensus) AwaitConsensus(ctx context.Context, proposalID string) (*shared.DistributedConsensusResult, error) {
	startTime := time.Now()
	timeout := time.Duration(bc.config.TimeoutMs) * time.Millisecond

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			bc.mu.RLock()
			proposal, exists := bc.Proposals[proposalID]
			if !exists {
				bc.mu.RUnlock()
				return nil, fmt.Errorf("proposal %s not found", proposalID)
			}

			if proposal.Status != "pending" {
				totalNodes := len(bc.nodes) + 1
				result := bc.CreateResult(proposal, time.Since(startTime).Milliseconds(), totalNodes, 3) // 3 phases
				bc.mu.RUnlock()
				return result, nil
			}
			bc.mu.RUnlock()

			if time.Since(startTime) > timeout {
				bc.mu.Lock()
				proposal.Status = "expired"
				totalNodes := len(bc.nodes) + 1
				result := bc.CreateResult(proposal, time.Since(startTime).Milliseconds(), totalNodes, 3)
				bc.mu.Unlock()
				return result, nil
			}
		}
	}
}

// GetAlgorithmType returns the algorithm type.
func (bc *ByzantineConsensus) GetAlgorithmType() shared.ConsensusAlgorithmType {
	return shared.AlgorithmByzantine
}

// IsPrimary returns true if this node is the primary.
func (bc *ByzantineConsensus) IsPrimary() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.node.IsPrimary
}

// GetViewNumber returns the current view number.
func (bc *ByzantineConsensus) GetViewNumber() int64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.node.ViewNumber
}

// GetSequenceNumber returns the current sequence number.
func (bc *ByzantineConsensus) GetSequenceNumber() int64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.node.SequenceNumber
}

// GetMaxFaultyNodes returns the maximum number of faulty nodes that can be tolerated.
func (bc *ByzantineConsensus) GetMaxFaultyNodes() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	n := len(bc.nodes) + 1
	return (n - 1) / 3
}

// CanTolerate returns true if the cluster can tolerate the given number of faulty nodes.
func (bc *ByzantineConsensus) CanTolerate(faultyCount int) bool {
	return faultyCount <= bc.GetMaxFaultyNodes()
}

// HandlePrePrepare handles a pre-prepare message.
func (bc *ByzantineConsensus) HandlePrePrepare(message shared.ByzantineMessage) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if message.ViewNumber != bc.node.ViewNumber {
		return nil
	}

	proposalID := fmt.Sprintf("bft_%d_%d", message.ViewNumber, message.SequenceNumber)

	if _, exists := bc.Proposals[proposalID]; !exists && message.Payload != nil {
		proposal := &shared.ConsensusProposal{
			ID:         proposalID,
			ProposerID: message.SenderID,
			Value:      message.Payload,
			Term:       message.ViewNumber,
			Timestamp:  message.Timestamp,
			Status:     "pending",
			Votes:      make(map[string]interface{}),
		}
		bc.Proposals[proposalID] = proposal
	}

	// Send prepare message
	prepareMsg := shared.ByzantineMessage{
		Type:           shared.ByzantinePhasePrepare,
		ViewNumber:     message.ViewNumber,
		SequenceNumber: message.SequenceNumber,
		Digest:         message.Digest,
		SenderID:       bc.node.ID,
		Timestamp:      shared.Now(),
	}

	bc.mu.Unlock()
	bc.broadcastMessage(prepareMsg)
	bc.handlePrepare(prepareMsg)
	bc.mu.Lock()

	return nil
}

// handlePrepare handles a prepare message.
func (bc *ByzantineConsensus) handlePrepare(message shared.ByzantineMessage) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	key := fmt.Sprintf("%d_%d", message.ViewNumber, message.SequenceNumber)

	if _, exists := bc.messageLog[key]; !exists {
		bc.messageLog[key] = make([]shared.ByzantineMessage, 0)
	}

	// Check if already have prepare from this sender
	messages := bc.messageLog[key]
	for _, m := range messages {
		if m.Type == shared.ByzantinePhasePrepare && m.SenderID == message.SenderID {
			return
		}
	}

	bc.messageLog[key] = append(bc.messageLog[key], message)

	// Check if prepared (2f + 1 prepare messages)
	f := bc.config.MaxFaultyNodes
	prepareCount := 0
	for _, m := range bc.messageLog[key] {
		if m.Type == shared.ByzantinePhasePrepare {
			prepareCount++
		}
	}

	if prepareCount >= 2*f+1 {
		proposalID := fmt.Sprintf("bft_%d_%d", message.ViewNumber, message.SequenceNumber)
		bc.preparedMessages[key] = bc.messageLog[key]

		// Send commit message
		commitMsg := shared.ByzantineMessage{
			Type:           shared.ByzantinePhaseCommit,
			ViewNumber:     message.ViewNumber,
			SequenceNumber: message.SequenceNumber,
			Digest:         message.Digest,
			SenderID:       bc.node.ID,
			Timestamp:      shared.Now(),
		}

		bc.mu.Unlock()
		bc.broadcastMessage(commitMsg)
		bc.handleCommit(commitMsg)
		bc.mu.Lock()

		// Record vote
		if proposal, exists := bc.Proposals[proposalID]; exists {
			proposal.Votes[bc.node.ID] = map[string]interface{}{
				"voterId":    bc.node.ID,
				"approve":    true,
				"confidence": 1.0,
				"timestamp":  shared.Now(),
			}
		}
	}
}

// handleCommit handles a commit message.
func (bc *ByzantineConsensus) handleCommit(message shared.ByzantineMessage) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	key := fmt.Sprintf("%d_%d", message.ViewNumber, message.SequenceNumber)

	if _, exists := bc.messageLog[key]; !exists {
		bc.messageLog[key] = make([]shared.ByzantineMessage, 0)
	}

	// Check if already have commit from this sender
	messages := bc.messageLog[key]
	for _, m := range messages {
		if m.Type == shared.ByzantinePhaseCommit && m.SenderID == message.SenderID {
			return
		}
	}

	bc.messageLog[key] = append(bc.messageLog[key], message)

	// Check if committed (2f + 1 commit messages)
	f := bc.config.MaxFaultyNodes
	commitCount := 0
	for _, m := range bc.messageLog[key] {
		if m.Type == shared.ByzantinePhaseCommit {
			commitCount++
		}
	}

	if commitCount >= 2*f+1 {
		proposalID := fmt.Sprintf("bft_%d_%d", message.ViewNumber, message.SequenceNumber)
		bc.committedMessages[key] = bc.messageLog[key]

		if proposal, exists := bc.Proposals[proposalID]; exists && proposal.Status == "pending" {
			proposal.Status = "accepted"
		}
	}
}

// InitiateViewChange initiates a view change.
func (bc *ByzantineConsensus) InitiateViewChange() {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.node.ViewNumber++

	// Elect new primary
	bc.mu.Unlock()
	bc.ElectPrimary()
	bc.mu.Lock()
}

// broadcastMessage broadcasts a message to all nodes.
func (bc *ByzantineConsensus) broadcastMessage(message shared.ByzantineMessage) {
	bc.mu.RLock()
	nodes := make([]string, 0, len(bc.nodes))
	for id := range bc.nodes {
		nodes = append(nodes, id)
	}
	bc.mu.RUnlock()

	// In real implementation, this would send messages over network
	// For now, we simulate local message handling
	for range nodes {
		// Simulate message delivery
	}
}

// computeDigest computes a digest of a value.
func (bc *ByzantineConsensus) computeDigest(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ForcePrimary forces this node to become primary (for testing).
func (bc *ByzantineConsensus) ForcePrimary() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.node.IsPrimary = true
}

// NewByzantineProposalID generates a new proposal ID.
func NewByzantineProposalID() string {
	return "bft_" + uuid.New().String()
}
