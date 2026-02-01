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

// RaftConsensus implements the Raft consensus algorithm.
type RaftConsensus struct {
	*BaseAlgorithm
	config   shared.RaftConfig
	node     *shared.RaftNode
	peers    map[string]*shared.RaftNode
	mu       sync.RWMutex

	// Timers and channels
	electionTimer   *time.Timer
	heartbeatTicker *time.Ticker
	ctx             context.Context
	cancel          context.CancelFunc

	// Event channels
	voteChan     chan voteRequest
	appendChan   chan appendRequest
	proposalChan chan *shared.ConsensusProposal

	// Proposal counter
	proposalCounter int64
}

type voteRequest struct {
	candidateID  string
	term         int64
	lastLogIndex int64
	lastLogTerm  int64
	response     chan bool
}

type appendRequest struct {
	leaderID     string
	term         int64
	entries      []shared.RaftLogEntry
	leaderCommit int64
	response     chan bool
}

// NewRaftConsensus creates a new Raft consensus instance.
func NewRaftConsensus(nodeID string, config shared.RaftConfig) *RaftConsensus {
	ctx, cancel := context.WithCancel(context.Background())

	rc := &RaftConsensus{
		BaseAlgorithm: NewBaseAlgorithm(nodeID, shared.AlgorithmRaft),
		config:        config,
		node: &shared.RaftNode{
			ID:          nodeID,
			State:       shared.RaftStateFollower,
			CurrentTerm: 0,
			Log:         make([]shared.RaftLogEntry, 0),
			CommitIndex: 0,
			LastApplied: 0,
		},
		peers:        make(map[string]*shared.RaftNode),
		ctx:          ctx,
		cancel:       cancel,
		voteChan:     make(chan voteRequest, 10),
		appendChan:   make(chan appendRequest, 10),
		proposalChan: make(chan *shared.ConsensusProposal, 10),
	}

	return rc
}

// Initialize initializes the Raft consensus.
func (rc *RaftConsensus) Initialize(ctx context.Context) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Start election timer
	rc.resetElectionTimer()

	// Start background goroutines
	go rc.run()

	return nil
}

// Shutdown shuts down the Raft consensus.
func (rc *RaftConsensus) Shutdown() error {
	rc.cancel()

	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.electionTimer != nil {
		rc.electionTimer.Stop()
	}
	if rc.heartbeatTicker != nil {
		rc.heartbeatTicker.Stop()
	}

	return nil
}

// AddNode adds a peer to the Raft cluster.
func (rc *RaftConsensus) AddNode(nodeID string, opts ...NodeOption) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.peers[nodeID] = &shared.RaftNode{
		ID:          nodeID,
		State:       shared.RaftStateFollower,
		CurrentTerm: 0,
		Log:         make([]shared.RaftLogEntry, 0),
		CommitIndex: 0,
		LastApplied: 0,
	}

	return nil
}

// RemoveNode removes a peer from the Raft cluster.
func (rc *RaftConsensus) RemoveNode(nodeID string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	delete(rc.peers, nodeID)
	return nil
}

// Propose proposes a value for consensus.
func (rc *RaftConsensus) Propose(ctx context.Context, value interface{}) (*shared.ConsensusProposal, error) {
	rc.mu.Lock()

	if rc.node.State != shared.RaftStateLeader {
		rc.mu.Unlock()
		return nil, fmt.Errorf("only leader can propose values (current state: %s)", rc.node.State)
	}

	rc.proposalCounter++
	proposalID := fmt.Sprintf("raft_%s_%d", rc.node.ID, rc.proposalCounter)

	proposal := &shared.ConsensusProposal{
		ID:         proposalID,
		ProposerID: rc.node.ID,
		Value:      value,
		Term:       rc.node.CurrentTerm,
		Timestamp:  shared.Now(),
		Status:     "pending",
		Votes:      make(map[string]interface{}),
	}

	// Add to local log
	logEntry := shared.RaftLogEntry{
		Term:      rc.node.CurrentTerm,
		Index:     int64(len(rc.node.Log) + 1),
		Command:   map[string]interface{}{"proposalId": proposalID, "value": value},
		Timestamp: shared.Now(),
	}
	rc.node.Log = append(rc.node.Log, logEntry)

	// Store proposal
	rc.Proposals[proposalID] = proposal

	// Leader votes for itself
	proposal.Votes[rc.node.ID] = map[string]interface{}{
		"voterId":    rc.node.ID,
		"approve":    true,
		"confidence": 1.0,
		"timestamp":  shared.Now(),
	}

	rc.mu.Unlock()

	// Replicate to followers
	go rc.replicateToFollowers(logEntry)

	return proposal, nil
}

// Vote submits a vote for a proposal.
func (rc *RaftConsensus) Vote(ctx context.Context, proposalID string, vote shared.ConsensusVote) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	proposal, exists := rc.Proposals[proposalID]
	if !exists {
		return fmt.Errorf("proposal %s not found", proposalID)
	}

	if proposal.Status != "pending" {
		return nil
	}

	proposal.Votes[vote.VoterID] = map[string]interface{}{
		"voterId":    vote.VoterID,
		"approve":    vote.Approve,
		"confidence": vote.Confidence,
		"timestamp":  vote.Timestamp,
	}

	// Check consensus
	rc.checkConsensus(proposal)

	return nil
}

// AwaitConsensus waits for consensus on a proposal.
func (rc *RaftConsensus) AwaitConsensus(ctx context.Context, proposalID string) (*shared.DistributedConsensusResult, error) {
	startTime := time.Now()
	timeout := time.Duration(rc.config.TimeoutMs) * time.Millisecond

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			rc.mu.RLock()
			proposal, exists := rc.Proposals[proposalID]
			if !exists {
				rc.mu.RUnlock()
				return nil, fmt.Errorf("proposal %s not found", proposalID)
			}

			if proposal.Status != "pending" {
				totalNodes := len(rc.peers) + 1
				result := rc.CreateResult(proposal, time.Since(startTime).Milliseconds(), totalNodes, 1)
				rc.mu.RUnlock()
				return result, nil
			}
			rc.mu.RUnlock()

			if time.Since(startTime) > timeout {
				rc.mu.Lock()
				proposal.Status = "expired"
				totalNodes := len(rc.peers) + 1
				result := rc.CreateResult(proposal, time.Since(startTime).Milliseconds(), totalNodes, 1)
				rc.mu.Unlock()
				return result, nil
			}
		}
	}
}

// GetAlgorithmType returns the algorithm type.
func (rc *RaftConsensus) GetAlgorithmType() shared.ConsensusAlgorithmType {
	return shared.AlgorithmRaft
}

// IsLeader returns true if this node is the leader.
func (rc *RaftConsensus) IsLeader() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.node.State == shared.RaftStateLeader
}

// GetLeaderID returns the leader ID (or empty if unknown).
func (rc *RaftConsensus) GetLeaderID() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if rc.node.State == shared.RaftStateLeader {
		return rc.node.ID
	}
	return rc.node.VotedFor
}

// GetState returns the current Raft state.
func (rc *RaftConsensus) GetState() shared.RaftState {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.node.State
}

// GetTerm returns the current term.
func (rc *RaftConsensus) GetTerm() int64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.node.CurrentTerm
}

// run is the main event loop.
func (rc *RaftConsensus) run() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		case req := <-rc.voteChan:
			granted := rc.handleVoteRequest(req.candidateID, req.term, req.lastLogIndex, req.lastLogTerm)
			req.response <- granted
		case req := <-rc.appendChan:
			success := rc.handleAppendEntries(req.leaderID, req.term, req.entries, req.leaderCommit)
			req.response <- success
		}
	}
}

// resetElectionTimer resets the election timer with a random timeout.
func (rc *RaftConsensus) resetElectionTimer() {
	if rc.electionTimer != nil {
		rc.electionTimer.Stop()
	}

	timeout := rc.randomElectionTimeout()
	rc.electionTimer = time.AfterFunc(timeout, func() {
		rc.startElection()
	})
}

// randomElectionTimeout returns a random election timeout.
func (rc *RaftConsensus) randomElectionTimeout() time.Duration {
	min := rc.config.ElectionTimeoutMin
	max := rc.config.ElectionTimeoutMax
	timeout := min + rand.Int63n(max-min+1)
	return time.Duration(timeout) * time.Millisecond
}

// startElection starts a leader election.
func (rc *RaftConsensus) startElection() {
	rc.mu.Lock()

	rc.node.State = shared.RaftStateCandidate
	rc.node.CurrentTerm++
	rc.node.VotedFor = rc.node.ID

	term := rc.node.CurrentTerm
	lastLogIndex := int64(len(rc.node.Log))
	lastLogTerm := int64(0)
	if len(rc.node.Log) > 0 {
		lastLogTerm = rc.node.Log[len(rc.node.Log)-1].Term
	}

	peers := make([]string, 0, len(rc.peers))
	for peerID := range rc.peers {
		peers = append(peers, peerID)
	}

	rc.mu.Unlock()

	// Vote for self
	votesReceived := 1
	votesNeeded := (len(peers)+1)/2 + 1

	// Request votes from peers
	var wg sync.WaitGroup
	var votesMu sync.Mutex

	for _, peerID := range peers {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()

			granted := rc.requestVote(pid, term, lastLogIndex, lastLogTerm)
			if granted {
				votesMu.Lock()
				votesReceived++
				if votesReceived >= votesNeeded {
					rc.becomeLeader()
				}
				votesMu.Unlock()
			}
		}(peerID)
	}

	// Wait for votes with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Duration(rc.config.ElectionTimeoutMax) * time.Millisecond):
	}

	// If not leader, reset to follower
	rc.mu.Lock()
	if rc.node.State == shared.RaftStateCandidate {
		rc.node.State = shared.RaftStateFollower
		rc.resetElectionTimer()
	}
	rc.mu.Unlock()
}

// requestVote requests a vote from a peer.
func (rc *RaftConsensus) requestVote(peerID string, term, lastLogIndex, lastLogTerm int64) bool {
	rc.mu.Lock()
	peer, exists := rc.peers[peerID]
	if !exists {
		rc.mu.Unlock()
		return false
	}

	// Simulate vote request - in real implementation this would be RPC
	if term > peer.CurrentTerm {
		peer.VotedFor = rc.node.ID
		peer.CurrentTerm = term
		rc.mu.Unlock()
		return true
	}

	rc.mu.Unlock()
	return false
}

// becomeLeader transitions to leader state.
func (rc *RaftConsensus) becomeLeader() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.node.State != shared.RaftStateCandidate {
		return
	}

	rc.node.State = shared.RaftStateLeader

	if rc.electionTimer != nil {
		rc.electionTimer.Stop()
	}

	// Start heartbeat ticker
	rc.heartbeatTicker = time.NewTicker(time.Duration(rc.config.HeartbeatInterval) * time.Millisecond)
	go rc.sendHeartbeats()
}

// sendHeartbeats sends heartbeats to followers.
func (rc *RaftConsensus) sendHeartbeats() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		case <-rc.heartbeatTicker.C:
			rc.mu.RLock()
			if rc.node.State != shared.RaftStateLeader {
				rc.mu.RUnlock()
				return
			}
			peers := make([]string, 0, len(rc.peers))
			for peerID := range rc.peers {
				peers = append(peers, peerID)
			}
			rc.mu.RUnlock()

			for _, peerID := range peers {
				go rc.appendEntries(peerID, nil)
			}
		}
	}
}

// appendEntries sends AppendEntries to a peer.
func (rc *RaftConsensus) appendEntries(peerID string, entries []shared.RaftLogEntry) bool {
	rc.mu.Lock()
	peer, exists := rc.peers[peerID]
	if !exists {
		rc.mu.Unlock()
		return false
	}

	// Simulate AppendEntries - in real implementation this would be RPC
	if rc.node.CurrentTerm >= peer.CurrentTerm {
		peer.CurrentTerm = rc.node.CurrentTerm
		peer.State = shared.RaftStateFollower
		if entries != nil {
			peer.Log = append(peer.Log, entries...)
		}
		rc.mu.Unlock()
		return true
	}

	rc.mu.Unlock()
	return false
}

// replicateToFollowers replicates a log entry to followers.
func (rc *RaftConsensus) replicateToFollowers(entry shared.RaftLogEntry) {
	rc.mu.RLock()
	peers := make([]string, 0, len(rc.peers))
	for peerID := range rc.peers {
		peers = append(peers, peerID)
	}
	rc.mu.RUnlock()

	successCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, peerID := range peers {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()
			if rc.appendEntries(pid, []shared.RaftLogEntry{entry}) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(peerID)
	}

	wg.Wait()

	// Check if majority replicated
	majority := (len(peers)+1)/2 + 1
	if successCount+1 >= majority {
		rc.mu.Lock()
		rc.node.CommitIndex = entry.Index
		rc.mu.Unlock()
	}
}

// handleVoteRequest handles a vote request from a candidate.
func (rc *RaftConsensus) handleVoteRequest(candidateID string, term, lastLogIndex, lastLogTerm int64) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if term < rc.node.CurrentTerm {
		return false
	}

	if term > rc.node.CurrentTerm {
		rc.node.CurrentTerm = term
		rc.node.State = shared.RaftStateFollower
		rc.node.VotedFor = ""
	}

	if rc.node.VotedFor == "" || rc.node.VotedFor == candidateID {
		// Check log is at least as up-to-date
		myLastTerm := int64(0)
		myLastIndex := int64(0)
		if len(rc.node.Log) > 0 {
			myLastTerm = rc.node.Log[len(rc.node.Log)-1].Term
			myLastIndex = rc.node.Log[len(rc.node.Log)-1].Index
		}

		if lastLogTerm > myLastTerm || (lastLogTerm == myLastTerm && lastLogIndex >= myLastIndex) {
			rc.node.VotedFor = candidateID
			rc.resetElectionTimer()
			return true
		}
	}

	return false
}

// handleAppendEntries handles AppendEntries from a leader.
func (rc *RaftConsensus) handleAppendEntries(leaderID string, term int64, entries []shared.RaftLogEntry, leaderCommit int64) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if term < rc.node.CurrentTerm {
		return false
	}

	rc.resetElectionTimer()

	if term > rc.node.CurrentTerm {
		rc.node.CurrentTerm = term
		rc.node.State = shared.RaftStateFollower
	}

	rc.node.VotedFor = leaderID

	// Append entries
	if entries != nil {
		rc.node.Log = append(rc.node.Log, entries...)
	}

	// Update commit index
	if leaderCommit > rc.node.CommitIndex {
		if leaderCommit < int64(len(rc.node.Log)) {
			rc.node.CommitIndex = leaderCommit
		} else {
			rc.node.CommitIndex = int64(len(rc.node.Log))
		}
	}

	return true
}

// checkConsensus checks if consensus is reached for a proposal.
func (rc *RaftConsensus) checkConsensus(proposal *shared.ConsensusProposal) {
	totalVoters := len(rc.peers) + 1
	approvingVotes := 0

	for _, v := range proposal.Votes {
		if vote, ok := v.(map[string]interface{}); ok {
			if approve, ok := vote["approve"].(bool); ok && approve {
				approvingVotes++
			}
		}
	}

	threshold := rc.config.Threshold
	quorum := int(float64(totalVoters) * threshold)

	if approvingVotes >= quorum {
		proposal.Status = "accepted"
	} else if len(proposal.Votes)-approvingVotes > totalVoters-quorum {
		proposal.Status = "rejected"
	}

	rc.UpdateStats()
}

// ForceLeader forces this node to become leader (for testing).
func (rc *RaftConsensus) ForceLeader() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.node.State = shared.RaftStateLeader
	rc.node.CurrentTerm++

	if rc.electionTimer != nil {
		rc.electionTimer.Stop()
	}

	rc.heartbeatTicker = time.NewTicker(time.Duration(rc.config.HeartbeatInterval) * time.Millisecond)
	go rc.sendHeartbeats()
}

// NewRaftProposalID generates a new proposal ID.
func NewRaftProposalID() string {
	return "raft_" + uuid.New().String()
}
