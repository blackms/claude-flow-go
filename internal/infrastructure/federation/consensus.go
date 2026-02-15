// Package federation provides cross-swarm coordination and ephemeral agent management.
package federation

import (
	"fmt"
	"math"
	"strings"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ============================================================================
// Federation Consensus
// ============================================================================

// Propose creates a new consensus proposal.
func (fh *FederationHub) Propose(proposerID, proposalType string, value interface{}) (*shared.FederationProposal, error) {
	if !fh.config.EnableConsensus {
		return nil, fmt.Errorf("consensus is disabled")
	}

	fh.mu.Lock()
	defer fh.mu.Unlock()

	proposerID = strings.TrimSpace(proposerID)
	proposalType = strings.TrimSpace(proposalType)
	if proposerID == "" {
		return nil, fmt.Errorf("proposerId is required")
	}
	if proposalType == "" {
		return nil, fmt.Errorf("proposalType is required")
	}
	if value == nil {
		return nil, fmt.Errorf("value is required")
	}
	if fh.config.ConsensusQuorum <= 0 || fh.config.ConsensusQuorum > 1 {
		return nil, fmt.Errorf("consensus quorum must be between 0 and 1")
	}
	if fh.config.ProposalTimeout <= 0 {
		return nil, fmt.Errorf("proposal timeout must be greater than 0")
	}

	// Validate proposer swarm
	proposerSwarm, exists := fh.swarms[proposerID]
	if !exists {
		return nil, fmt.Errorf("proposer swarm %s not found", proposerID)
	}
	if proposerSwarm.Status != shared.SwarmStatusActive {
		return nil, fmt.Errorf("proposer swarm %s is not active", proposerID)
	}

	now := shared.Now()
	if fh.config.ProposalTimeout > math.MaxInt64-now {
		return nil, fmt.Errorf("proposal timeout is out of range")
	}
	proposalID := generateID("proposal")

	proposal := &shared.FederationProposal{
		ID:         proposalID,
		ProposerID: proposerID,
		Type:       proposalType,
		Value:      cloneInterfaceValue(value),
		Votes:      make(map[string]bool),
		Status:     shared.FederationProposalPending,
		CreatedAt:  now,
		ExpiresAt:  now + fh.config.ProposalTimeout,
	}

	// Proposer auto-votes in favor
	proposal.Votes[proposerID] = true

	// Add to proposals
	fh.proposals[proposalID] = proposal
	fh.stats.PendingProposals++

	// Emit event
	fh.emitEvent(shared.FederationEvent{
		Type:      shared.FederationEventConsensusStarted,
		SwarmID:   proposerID,
		Data:      map[string]interface{}{"proposalId": proposalID, "type": proposalType},
		Timestamp: now,
	})

	// Broadcast vote request to all active swarms
	fh.broadcastConsensusRequest(proposal)

	// Check if quorum already reached (e.g., single swarm)
	fh.checkQuorum(proposal)

	return cloneFederationProposal(proposal), nil
}

// Vote submits a vote on a proposal.
func (fh *FederationHub) Vote(voterID, proposalID string, approve bool) error {
	if !fh.config.EnableConsensus {
		return fmt.Errorf("consensus is disabled")
	}

	fh.mu.Lock()
	defer fh.mu.Unlock()

	voterID = strings.TrimSpace(voterID)
	proposalID = strings.TrimSpace(proposalID)
	if voterID == "" {
		return fmt.Errorf("voterId is required")
	}
	if proposalID == "" {
		return fmt.Errorf("proposalId is required")
	}

	// Validate voter swarm
	voterSwarm, exists := fh.swarms[voterID]
	if !exists {
		return fmt.Errorf("voter swarm %s not found", voterID)
	}
	if voterSwarm.Status != shared.SwarmStatusActive {
		return fmt.Errorf("voter swarm %s is not active", voterID)
	}

	// Get proposal
	proposal, exists := fh.proposals[proposalID]
	if !exists {
		return fmt.Errorf("proposal %s not found", proposalID)
	}

	// Check proposal status
	if proposal.Status != shared.FederationProposalPending {
		return fmt.Errorf("proposal %s is no longer pending", proposalID)
	}

	// Check if already expired
	if shared.Now() > proposal.ExpiresAt {
		proposal.Status = shared.FederationProposalRejected
		fh.stats.PendingProposals--
		fh.stats.RejectedProposals++
		return fmt.Errorf("proposal %s has expired", proposalID)
	}

	if _, hasVoted := proposal.Votes[voterID]; hasVoted {
		return fmt.Errorf("voter %s has already voted on proposal %s", voterID, proposalID)
	}

	// Record vote
	proposal.Votes[voterID] = approve

	// Check quorum
	fh.checkQuorum(proposal)

	return nil
}

// checkQuorum checks if a proposal has reached quorum.
func (fh *FederationHub) checkQuorum(proposal *shared.FederationProposal) {
	if proposal.Status != shared.FederationProposalPending {
		return
	}

	// Count active swarms (eligible voters)
	activeSwarms := 0
	for _, swarm := range fh.swarms {
		if swarm.Status == shared.SwarmStatusActive {
			activeSwarms++
		}
	}

	if activeSwarms == 0 {
		return
	}

	// Count votes
	approvals := 0
	rejections := 0
	for _, approve := range proposal.Votes {
		if approve {
			approvals++
		} else {
			rejections++
		}
	}

	// Calculate required votes for quorum
	requiredVotes := int(math.Ceil(float64(activeSwarms) * fh.config.ConsensusQuorum))
	if requiredVotes < 1 {
		requiredVotes = 1
	}

	now := shared.Now()

	// Check if approved
	if approvals >= requiredVotes {
		proposal.Status = shared.FederationProposalAccepted
		fh.stats.PendingProposals--
		fh.stats.AcceptedProposals++

		fh.emitEvent(shared.FederationEvent{
			Type:    shared.FederationEventConsensusCompleted,
			SwarmID: proposal.ProposerID,
			Data: map[string]interface{}{
				"proposalId": proposal.ID,
				"status":     "accepted",
				"approvals":  approvals,
				"rejections": rejections,
			},
			Timestamp: now,
		})
		return
	}

	// Check if impossible to reach quorum (too many rejections)
	remainingVotes := activeSwarms - len(proposal.Votes)
	if approvals+remainingVotes < requiredVotes {
		proposal.Status = shared.FederationProposalRejected
		fh.stats.PendingProposals--
		fh.stats.RejectedProposals++

		fh.emitEvent(shared.FederationEvent{
			Type:    shared.FederationEventConsensusCompleted,
			SwarmID: proposal.ProposerID,
			Data: map[string]interface{}{
				"proposalId": proposal.ID,
				"status":     "rejected",
				"approvals":  approvals,
				"rejections": rejections,
			},
			Timestamp: now,
		})
	}
}

// broadcastConsensusRequest broadcasts a vote request to all active swarms.
func (fh *FederationHub) broadcastConsensusRequest(proposal *shared.FederationProposal) {
	for swarmID, swarm := range fh.swarms {
		if swarmID == proposal.ProposerID {
			continue // Skip proposer
		}
		if swarm.Status != shared.SwarmStatusActive {
			continue
		}

		// Send consensus message
		msg := &shared.FederationMessage{
			ID:            generateID("consensus_req"),
			Type:          shared.FederationMsgConsensus,
			SourceSwarmID: proposal.ProposerID,
			TargetSwarmID: swarmID,
			Payload: map[string]interface{}{
				"action":     "vote_request",
				"proposalId": proposal.ID,
				"type":       proposal.Type,
				"value":      cloneInterfaceValue(proposal.Value),
				"expiresAt":  proposal.ExpiresAt,
			},
			Timestamp: shared.Now(),
		}
		fh.addMessage(msg)
	}
}

// GetProposal returns a proposal by ID.
func (fh *FederationHub) GetProposal(proposalID string) (*shared.FederationProposal, bool) {
	fh.mu.RLock()
	defer fh.mu.RUnlock()
	proposalID = strings.TrimSpace(proposalID)
	if proposalID == "" {
		return nil, false
	}
	proposal, exists := fh.proposals[proposalID]
	if !exists {
		return nil, false
	}
	return cloneFederationProposal(proposal), true
}

// GetProposals returns all proposals.
func (fh *FederationHub) GetProposals() []*shared.FederationProposal {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	proposals := make([]*shared.FederationProposal, 0, len(fh.proposals))
	for _, proposal := range fh.proposals {
		proposals = append(proposals, cloneFederationProposal(proposal))
	}
	sortFederationProposalsByID(proposals)
	return proposals
}

// GetPendingProposals returns all pending proposals.
func (fh *FederationHub) GetPendingProposals() []*shared.FederationProposal {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	proposals := make([]*shared.FederationProposal, 0)
	for _, proposal := range fh.proposals {
		if proposal.Status == shared.FederationProposalPending {
			proposals = append(proposals, cloneFederationProposal(proposal))
		}
	}
	sortFederationProposalsByID(proposals)
	return proposals
}

// GetProposalsByStatus returns proposals by status.
func (fh *FederationHub) GetProposalsByStatus(status shared.FederationProposalStatus) []*shared.FederationProposal {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	proposals := make([]*shared.FederationProposal, 0)
	for _, proposal := range fh.proposals {
		if proposal.Status == status {
			proposals = append(proposals, cloneFederationProposal(proposal))
		}
	}
	sortFederationProposalsByID(proposals)
	return proposals
}

// GetProposalVotes returns the vote breakdown for a proposal.
func (fh *FederationHub) GetProposalVotes(proposalID string) (approvals, rejections int, err error) {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	proposalID = strings.TrimSpace(proposalID)
	if proposalID == "" {
		return 0, 0, fmt.Errorf("proposalId is required")
	}

	proposal, exists := fh.proposals[proposalID]
	if !exists {
		return 0, 0, fmt.Errorf("proposal %s not found", proposalID)
	}

	for _, approve := range proposal.Votes {
		if approve {
			approvals++
		} else {
			rejections++
		}
	}

	return approvals, rejections, nil
}

// GetQuorumInfo returns information about quorum requirements.
func (fh *FederationHub) GetQuorumInfo() (activeSwarms int, requiredVotes int, quorumPercentage float64) {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	for _, swarm := range fh.swarms {
		if swarm.Status == shared.SwarmStatusActive {
			activeSwarms++
		}
	}

	requiredVotes = int(math.Ceil(float64(activeSwarms) * fh.config.ConsensusQuorum))
	if requiredVotes < 1 && activeSwarms > 0 {
		requiredVotes = 1
	}

	return activeSwarms, requiredVotes, fh.config.ConsensusQuorum
}
