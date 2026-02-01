// Package consensus provides distributed consensus algorithms for multi-agent coordination.
package consensus

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
	"github.com/google/uuid"
)

// GossipConsensus implements gossip protocol for eventually consistent consensus.
type GossipConsensus struct {
	*BaseAlgorithm
	config shared.GossipConfig
	node   *gossipNodeState
	nodes  map[string]*gossipNodeState
	mu     sync.RWMutex

	// Message queue and seen messages
	messageQueue []shared.GossipMessage
	seenMessages map[string]bool

	// Context and control
	ctx           context.Context
	cancel        context.CancelFunc
	gossipTicker  *time.Ticker

	// Counters
	proposalCounter int64
}

// gossipNodeState holds the internal state of a gossip node.
type gossipNodeState struct {
	ID           string
	State        map[string]interface{}
	Version      int64
	Neighbors    map[string]bool
	SeenMessages map[string]bool
	LastSync     int64
}

// NewGossipConsensus creates a new Gossip consensus instance.
func NewGossipConsensus(nodeID string, config shared.GossipConfig) *GossipConsensus {
	ctx, cancel := context.WithCancel(context.Background())

	gc := &GossipConsensus{
		BaseAlgorithm: NewBaseAlgorithm(nodeID, shared.AlgorithmGossip),
		config:        config,
		node: &gossipNodeState{
			ID:           nodeID,
			State:        make(map[string]interface{}),
			Version:      0,
			Neighbors:    make(map[string]bool),
			SeenMessages: make(map[string]bool),
			LastSync:     shared.Now(),
		},
		nodes:        make(map[string]*gossipNodeState),
		messageQueue: make([]shared.GossipMessage, 0),
		seenMessages: make(map[string]bool),
		ctx:          ctx,
		cancel:       cancel,
	}

	return gc
}

// Initialize initializes the Gossip consensus.
func (gc *GossipConsensus) Initialize(ctx context.Context) error {
	gc.startGossipLoop()
	return nil
}

// Shutdown shuts down the Gossip consensus.
func (gc *GossipConsensus) Shutdown() error {
	gc.cancel()
	if gc.gossipTicker != nil {
		gc.gossipTicker.Stop()
	}
	return nil
}

// AddNode adds a node to the gossip network.
func (gc *GossipConsensus) AddNode(nodeID string, opts ...NodeOption) error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	node := &gossipNodeState{
		ID:           nodeID,
		State:        make(map[string]interface{}),
		Version:      0,
		Neighbors:    make(map[string]bool),
		SeenMessages: make(map[string]bool),
		LastSync:     shared.Now(),
	}

	gc.nodes[nodeID] = node

	// Add as neighbor with some probability (random mesh)
	if rand.Float64() < 0.5 {
		gc.node.Neighbors[nodeID] = true
		node.Neighbors[gc.node.ID] = true
	}

	return nil
}

// RemoveNode removes a node from the gossip network.
func (gc *GossipConsensus) RemoveNode(nodeID string) error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	delete(gc.nodes, nodeID)
	delete(gc.node.Neighbors, nodeID)

	for _, node := range gc.nodes {
		delete(node.Neighbors, nodeID)
	}

	return nil
}

// AddNeighbor explicitly adds a neighbor.
func (gc *GossipConsensus) AddNeighbor(nodeID string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if _, exists := gc.nodes[nodeID]; exists {
		gc.node.Neighbors[nodeID] = true
	}
}

// RemoveNeighbor removes a neighbor.
func (gc *GossipConsensus) RemoveNeighbor(nodeID string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	delete(gc.node.Neighbors, nodeID)
}

// Propose proposes a value for consensus.
func (gc *GossipConsensus) Propose(ctx context.Context, value interface{}) (*shared.ConsensusProposal, error) {
	gc.mu.Lock()

	gc.proposalCounter++
	proposalID := fmt.Sprintf("gossip_%s_%d", gc.node.ID, gc.proposalCounter)

	proposal := &shared.ConsensusProposal{
		ID:         proposalID,
		ProposerID: gc.node.ID,
		Value:      value,
		Term:       gc.node.Version,
		Timestamp:  shared.Now(),
		Status:     "pending",
		Votes:      make(map[string]interface{}),
	}

	gc.Proposals[proposalID] = proposal

	// Self-vote
	proposal.Votes[gc.node.ID] = map[string]interface{}{
		"voterId":    gc.node.ID,
		"approve":    true,
		"confidence": 1.0,
		"timestamp":  shared.Now(),
	}

	// Create gossip message
	gc.node.Version++
	message := shared.GossipMessage{
		ID:        fmt.Sprintf("msg_%s", proposalID),
		Type:      shared.GossipMessageProposal,
		SenderID:  gc.node.ID,
		Version:   gc.node.Version,
		Payload:   map[string]interface{}{"proposalId": proposalID, "value": value},
		Timestamp: shared.Now(),
		TTL:       gc.config.MaxHops,
		Hops:      0,
		Path:      []string{gc.node.ID},
	}

	gc.queueMessage(message)
	gc.mu.Unlock()

	return proposal, nil
}

// Vote submits a vote for a proposal.
func (gc *GossipConsensus) Vote(ctx context.Context, proposalID string, vote shared.ConsensusVote) error {
	gc.mu.Lock()

	proposal, exists := gc.Proposals[proposalID]
	if !exists {
		gc.mu.Unlock()
		return nil
	}

	proposal.Votes[vote.VoterID] = map[string]interface{}{
		"voterId":    vote.VoterID,
		"approve":    vote.Approve,
		"confidence": vote.Confidence,
		"timestamp":  vote.Timestamp,
	}

	// Create vote gossip message
	gc.node.Version++
	message := shared.GossipMessage{
		ID:        fmt.Sprintf("vote_%s_%s", proposalID, vote.VoterID),
		Type:      shared.GossipMessageVote,
		SenderID:  gc.node.ID,
		Version:   gc.node.Version,
		Payload:   map[string]interface{}{"proposalId": proposalID, "vote": vote},
		Timestamp: shared.Now(),
		TTL:       gc.config.MaxHops,
		Hops:      0,
		Path:      []string{gc.node.ID},
	}

	gc.queueMessage(message)
	gc.mu.Unlock()

	// Check convergence
	gc.checkConvergence(proposalID)

	return nil
}

// AwaitConsensus waits for consensus on a proposal.
func (gc *GossipConsensus) AwaitConsensus(ctx context.Context, proposalID string) (*shared.DistributedConsensusResult, error) {
	startTime := time.Now()
	timeout := time.Duration(gc.config.TimeoutMs) * time.Millisecond

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			gc.mu.RLock()
			proposal, exists := gc.Proposals[proposalID]
			if !exists {
				gc.mu.RUnlock()
				return nil, fmt.Errorf("proposal %s not found", proposalID)
			}

			if proposal.Status != "pending" {
				totalNodes := len(gc.nodes) + 1
				result := gc.CreateResult(proposal, time.Since(startTime).Milliseconds(), totalNodes, int(gc.node.Version))
				gc.mu.RUnlock()
				return result, nil
			}
			gc.mu.RUnlock()

			// Check convergence
			gc.checkConvergence(proposalID)

			if time.Since(startTime) > timeout {
				gc.mu.Lock()
				// Gossip is eventually consistent, so mark as accepted if threshold met
				totalNodes := len(gc.nodes) + 1
				votes := len(proposal.Votes)
				threshold := gc.config.ConvergenceThreshold

				if float64(votes)/float64(totalNodes) >= threshold {
					proposal.Status = "accepted"
				} else {
					proposal.Status = "expired"
				}

				result := gc.CreateResult(proposal, time.Since(startTime).Milliseconds(), totalNodes, int(gc.node.Version))
				gc.mu.Unlock()
				return result, nil
			}
		}
	}
}

// GetAlgorithmType returns the algorithm type.
func (gc *GossipConsensus) GetAlgorithmType() shared.ConsensusAlgorithmType {
	return shared.AlgorithmGossip
}

// GetConvergence returns the convergence rate for a proposal.
func (gc *GossipConsensus) GetConvergence(proposalID string) float64 {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	proposal, exists := gc.Proposals[proposalID]
	if !exists {
		return 0
	}

	totalNodes := len(gc.nodes) + 1
	return float64(len(proposal.Votes)) / float64(totalNodes)
}

// GetVersion returns the current version.
func (gc *GossipConsensus) GetVersion() int64 {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.node.Version
}

// GetNeighborCount returns the number of neighbors.
func (gc *GossipConsensus) GetNeighborCount() int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return len(gc.node.Neighbors)
}

// GetSeenMessageCount returns the number of seen messages.
func (gc *GossipConsensus) GetSeenMessageCount() int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return len(gc.seenMessages)
}

// GetQueueDepth returns the message queue depth.
func (gc *GossipConsensus) GetQueueDepth() int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return len(gc.messageQueue)
}

// startGossipLoop starts the gossip loop.
func (gc *GossipConsensus) startGossipLoop() {
	gc.gossipTicker = time.NewTicker(time.Duration(gc.config.GossipIntervalMs) * time.Millisecond)

	go func() {
		for {
			select {
			case <-gc.ctx.Done():
				return
			case <-gc.gossipTicker.C:
				gc.gossipRound()
			}
		}
	}()
}

// gossipRound performs one round of gossip.
func (gc *GossipConsensus) gossipRound() {
	gc.mu.Lock()

	if len(gc.messageQueue) == 0 {
		gc.mu.Unlock()
		return
	}

	// Select random neighbors (fanout)
	fanout := gc.config.Fanout
	if fanout > len(gc.node.Neighbors) {
		fanout = len(gc.node.Neighbors)
	}
	neighbors := gc.selectRandomNeighbors(fanout)

	// Process up to 10 messages per round
	messagesToSend := 10
	if messagesToSend > len(gc.messageQueue) {
		messagesToSend = len(gc.messageQueue)
	}
	messages := gc.messageQueue[:messagesToSend]
	gc.messageQueue = gc.messageQueue[messagesToSend:]

	gc.node.LastSync = shared.Now()
	gc.mu.Unlock()

	// Send to neighbors
	for _, message := range messages {
		for _, neighborID := range neighbors {
			gc.sendToNeighbor(neighborID, message)
		}
	}
}

// selectRandomNeighbors selects random neighbors.
func (gc *GossipConsensus) selectRandomNeighbors(count int) []string {
	neighbors := make([]string, 0, len(gc.node.Neighbors))
	for id := range gc.node.Neighbors {
		neighbors = append(neighbors, id)
	}

	// Shuffle
	rand.Shuffle(len(neighbors), func(i, j int) {
		neighbors[i], neighbors[j] = neighbors[j], neighbors[i]
	})

	if count > len(neighbors) {
		count = len(neighbors)
	}

	return neighbors[:count]
}

// sendToNeighbor sends a message to a neighbor.
func (gc *GossipConsensus) sendToNeighbor(neighborID string, message shared.GossipMessage) {
	gc.mu.Lock()

	neighbor, exists := gc.nodes[neighborID]
	if !exists {
		gc.mu.Unlock()
		return
	}

	// Check if already seen
	if neighbor.SeenMessages[message.ID] {
		gc.mu.Unlock()
		return
	}

	// Create delivered message
	deliveredMessage := message
	deliveredMessage.Hops++
	deliveredMessage.Path = append(append([]string{}, message.Path...), neighborID)

	gc.mu.Unlock()

	// Process at neighbor
	gc.processReceivedMessage(neighbor, deliveredMessage)
}

// processReceivedMessage processes a received message.
func (gc *GossipConsensus) processReceivedMessage(node *gossipNodeState, message shared.GossipMessage) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	// Mark as seen
	node.SeenMessages[message.ID] = true

	// Check TTL
	if message.TTL <= 0 || message.Hops >= gc.config.MaxHops {
		return
	}

	switch message.Type {
	case shared.GossipMessageProposal:
		gc.handleProposalMessage(node, message)
	case shared.GossipMessageVote:
		gc.handleVoteMessage(node, message)
	case shared.GossipMessageState:
		gc.handleStateMessage(node, message)
	}

	// Propagate to neighbors (gossip)
	if message.Hops < gc.config.MaxHops {
		propagateMessage := message
		propagateMessage.TTL--

		// Add to queue if this is our node
		if node.ID == gc.node.ID {
			gc.queueMessageLocked(propagateMessage)
		}
	}
}

// handleProposalMessage handles a proposal gossip message.
func (gc *GossipConsensus) handleProposalMessage(node *gossipNodeState, message shared.GossipMessage) {
	payload, ok := message.Payload.(map[string]interface{})
	if !ok {
		return
	}

	proposalID, ok := payload["proposalId"].(string)
	if !ok {
		return
	}

	value := payload["value"]

	if _, exists := gc.Proposals[proposalID]; !exists {
		proposal := &shared.ConsensusProposal{
			ID:         proposalID,
			ProposerID: message.SenderID,
			Value:      value,
			Term:       message.Version,
			Timestamp:  message.Timestamp,
			Status:     "pending",
			Votes:      make(map[string]interface{}),
		}
		gc.Proposals[proposalID] = proposal

		// Auto-vote if this is our node
		if node.ID == gc.node.ID {
			proposal.Votes[gc.node.ID] = map[string]interface{}{
				"voterId":    gc.node.ID,
				"approve":    true,
				"confidence": 0.9,
				"timestamp":  shared.Now(),
			}
		}
	}
}

// handleVoteMessage handles a vote gossip message.
func (gc *GossipConsensus) handleVoteMessage(node *gossipNodeState, message shared.GossipMessage) {
	payload, ok := message.Payload.(map[string]interface{})
	if !ok {
		return
	}

	proposalID, ok := payload["proposalId"].(string)
	if !ok {
		return
	}

	voteData, ok := payload["vote"].(map[string]interface{})
	if !ok {
		return
	}

	proposal, exists := gc.Proposals[proposalID]
	if !exists {
		return
	}

	voterID, ok := voteData["voterId"].(string)
	if !ok {
		return
	}

	if _, hasVote := proposal.Votes[voterID]; !hasVote {
		proposal.Votes[voterID] = voteData
	}
}

// handleStateMessage handles a state gossip message.
func (gc *GossipConsensus) handleStateMessage(node *gossipNodeState, message shared.GossipMessage) {
	state, ok := message.Payload.(map[string]interface{})
	if !ok {
		return
	}

	// Merge state (last-writer-wins)
	if message.Version > node.Version {
		for key, value := range state {
			node.State[key] = value
		}
		node.Version = message.Version
	}
}

// queueMessage queues a message for gossip.
func (gc *GossipConsensus) queueMessage(message shared.GossipMessage) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.queueMessageLocked(message)
}

// queueMessageLocked queues a message (caller must hold lock).
func (gc *GossipConsensus) queueMessageLocked(message shared.GossipMessage) {
	if !gc.seenMessages[message.ID] {
		gc.seenMessages[message.ID] = true
		gc.messageQueue = append(gc.messageQueue, message)
	}
}

// checkConvergence checks if consensus has converged.
func (gc *GossipConsensus) checkConvergence(proposalID string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	proposal, exists := gc.Proposals[proposalID]
	if !exists || proposal.Status != "pending" {
		return
	}

	totalNodes := len(gc.nodes) + 1
	votes := len(proposal.Votes)
	convergenceThreshold := gc.config.ConvergenceThreshold
	approvalThreshold := gc.config.Threshold

	// Check if we've converged (enough nodes have voted)
	if float64(votes)/float64(totalNodes) >= convergenceThreshold {
		approvingVotes := 0
		for _, v := range proposal.Votes {
			if voteMap, ok := v.(map[string]interface{}); ok {
				if approve, ok := voteMap["approve"].(bool); ok && approve {
					approvingVotes++
				}
			}
		}

		if float64(approvingVotes)/float64(votes) >= approvalThreshold {
			proposal.Status = "accepted"
		} else {
			proposal.Status = "rejected"
		}
	}

	gc.UpdateStats()
}

// AntiEntropy performs anti-entropy state sync with a random neighbor.
func (gc *GossipConsensus) AntiEntropy() error {
	gc.mu.Lock()

	if len(gc.node.Neighbors) == 0 {
		gc.mu.Unlock()
		return nil
	}

	neighbors := make([]string, 0, len(gc.node.Neighbors))
	for id := range gc.node.Neighbors {
		neighbors = append(neighbors, id)
	}

	randomNeighbor := neighbors[rand.Intn(len(neighbors))]

	stateMessage := shared.GossipMessage{
		ID:        fmt.Sprintf("state_%s_%d", gc.node.ID, shared.Now()),
		Type:      shared.GossipMessageState,
		SenderID:  gc.node.ID,
		Version:   gc.node.Version,
		Payload:   gc.node.State,
		Timestamp: shared.Now(),
		TTL:       1,
		Hops:      0,
		Path:      []string{gc.node.ID},
	}

	gc.mu.Unlock()

	gc.sendToNeighbor(randomNeighbor, stateMessage)

	return nil
}

// NewGossipProposalID generates a new proposal ID.
func NewGossipProposalID() string {
	return "gossip_" + uuid.New().String()
}
