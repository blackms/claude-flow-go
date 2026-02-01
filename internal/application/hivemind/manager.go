// Package hivemind provides the Hive Mind consensus system for multi-agent coordination.
package hivemind

import (
	"context"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/shared"
	"github.com/google/uuid"
)

// Manager is the Hive Mind Manager that coordinates consensus across agents.
type Manager struct {
	queen        *coordinator.QueenCoordinator
	proposals    map[string]*shared.Proposal
	votes        map[string][]shared.WeightedVote
	results      map[string]*shared.ProposalResult
	config       shared.HiveMindConfig
	consensus    *ConsensusEngine
	learning     *LearningModule
	initialized  bool
	mu           sync.RWMutex

	// Channels for async operations
	votesChan    chan shared.WeightedVote
	proposalChan chan *shared.Proposal
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewManager creates a new Hive Mind Manager.
func NewManager(queen *coordinator.QueenCoordinator, config shared.HiveMindConfig) (*Manager, error) {
	if queen == nil {
		return nil, shared.NewValidationError("queen coordinator is required", nil)
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		queen:        queen,
		proposals:    make(map[string]*shared.Proposal),
		votes:        make(map[string][]shared.WeightedVote),
		results:      make(map[string]*shared.ProposalResult),
		config:       config,
		consensus:    NewConsensusEngine(config),
		learning:     NewLearningModule(config.EnableLearning),
		votesChan:    make(chan shared.WeightedVote, 100),
		proposalChan: make(chan *shared.Proposal, 10),
		ctx:          ctx,
		cancel:       cancel,
	}

	return m, nil
}

// Initialize initializes the Hive Mind system.
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	// Start background workers
	m.wg.Add(2)
	go m.voteProcessor()
	go m.proposalExpiryChecker()

	m.initialized = true
	return nil
}

// Shutdown shuts down the Hive Mind Manager.
func (m *Manager) Shutdown() error {
	m.cancel()
	close(m.votesChan)
	close(m.proposalChan)
	m.wg.Wait()
	return nil
}

// voteProcessor processes incoming votes asynchronously.
func (m *Manager) voteProcessor() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case vote, ok := <-m.votesChan:
			if !ok {
				return
			}
			m.processVote(vote)
		}
	}
}

// proposalExpiryChecker checks for expired proposals.
func (m *Manager) proposalExpiryChecker() {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkExpiredProposals()
		}
	}
}

// processVote processes a single vote.
func (m *Manager) processVote(vote shared.WeightedVote) {
	m.mu.Lock()
	defer m.mu.Unlock()

	proposal, exists := m.proposals[vote.ProposalID]
	if !exists || proposal.Status != shared.ProposalStatusVoting {
		return
	}

	// Add vote
	m.votes[vote.ProposalID] = append(m.votes[vote.ProposalID], vote)

	// Check if we have enough votes to reach consensus
	votes := m.votes[vote.ProposalID]
	result := m.consensus.CalculateResult(votes, proposal.RequiredType, proposal.RequiredQuorum)

	// Check if consensus is reached
	if result.ConsensusReached || result.QuorumReached {
		m.finalizeProposal(proposal.ID, result)
	}
}

// checkExpiredProposals checks and expires old proposals.
func (m *Manager) checkExpiredProposals() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := shared.Now()

	for id, proposal := range m.proposals {
		if proposal.Status == shared.ProposalStatusPending || proposal.Status == shared.ProposalStatusVoting {
			if now > proposal.ExpiresAt {
				// Finalize with current votes
				votes := m.votes[id]
				result := m.consensus.CalculateResult(votes, proposal.RequiredType, proposal.RequiredQuorum)
				result.Status = shared.ProposalStatusExpired
				m.finalizeProposal(id, result)
			}
		}
	}
}

// finalizeProposal finalizes a proposal with the given result.
func (m *Manager) finalizeProposal(proposalID string, result *shared.ProposalResult) {
	proposal, exists := m.proposals[proposalID]
	if !exists {
		return
	}

	// Update proposal status
	if result.ConsensusReached {
		if result.WeightedApproval > result.WeightedRejection {
			proposal.Status = shared.ProposalStatusApproved
			result.Status = shared.ProposalStatusApproved
		} else {
			proposal.Status = shared.ProposalStatusRejected
			result.Status = shared.ProposalStatusRejected
		}
	} else if result.Status != shared.ProposalStatusExpired {
		proposal.Status = shared.ProposalStatusRejected
		result.Status = shared.ProposalStatusRejected
	} else {
		proposal.Status = shared.ProposalStatusExpired
	}

	result.CompletedAt = shared.Now()
	result.Duration = result.CompletedAt - proposal.CreatedAt
	m.results[proposalID] = result

	// Record outcome for learning
	if m.learning != nil {
		outcome := shared.ProposalOutcome{
			ProposalID:    proposalID,
			ProposalType:  proposal.Type,
			ConsensusType: proposal.RequiredType,
			WasApproved:   proposal.Status == shared.ProposalStatusApproved,
			VoteCount:     result.TotalVotes,
			WeightedScore: result.WeightedApproval,
			Duration:      result.Duration,
			Timestamp:     result.CompletedAt,
		}
		m.learning.RecordOutcome(outcome)
	}
}

// CreateProposal creates a new proposal for voting.
func (m *Manager) CreateProposal(ctx context.Context, proposal shared.Proposal) (*shared.Proposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check max proposals limit
	activeCount := 0
	for _, p := range m.proposals {
		if p.Status == shared.ProposalStatusPending || p.Status == shared.ProposalStatusVoting {
			activeCount++
		}
	}
	if activeCount >= m.config.MaxProposals {
		return nil, shared.NewValidationError("maximum proposals limit reached", map[string]interface{}{
			"max":     m.config.MaxProposals,
			"current": activeCount,
		})
	}

	// Generate ID if not provided
	if proposal.ID == "" {
		proposal.ID = uuid.New().String()
	}

	// Set defaults
	now := shared.Now()
	proposal.CreatedAt = now
	if proposal.ExpiresAt == 0 {
		proposal.ExpiresAt = now + m.config.VoteTimeout
	}
	if proposal.RequiredQuorum == 0 {
		proposal.RequiredQuorum = m.config.DefaultQuorum
	}
	if proposal.RequiredType == "" {
		proposal.RequiredType = m.config.ConsensusAlgorithm
	}
	proposal.Status = shared.ProposalStatusVoting

	// Store proposal
	m.proposals[proposal.ID] = &proposal
	m.votes[proposal.ID] = make([]shared.WeightedVote, 0)

	return &proposal, nil
}

// Vote submits a vote for a proposal.
func (m *Manager) Vote(ctx context.Context, vote shared.WeightedVote) error {
	m.mu.RLock()
	proposal, exists := m.proposals[vote.ProposalID]
	if !exists {
		m.mu.RUnlock()
		return shared.NewValidationError("proposal not found", map[string]interface{}{
			"proposalId": vote.ProposalID,
		})
	}
	if proposal.Status != shared.ProposalStatusVoting {
		m.mu.RUnlock()
		return shared.NewValidationError("proposal is not accepting votes", map[string]interface{}{
			"proposalId": vote.ProposalID,
			"status":     proposal.Status,
		})
	}
	m.mu.RUnlock()

	// Set timestamp
	if vote.Timestamp == 0 {
		vote.Timestamp = shared.Now()
	}

	// Calculate weight if not provided
	if vote.Weight == 0 {
		vote.Weight = m.calculateVoteWeight(vote.AgentID)
	}

	// Validate minimum weight
	if vote.Weight < m.config.MinVoteWeight {
		vote.Weight = m.config.MinVoteWeight
	}

	// Send to async processor
	select {
	case m.votesChan <- vote:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// calculateVoteWeight calculates the weight for an agent's vote.
func (m *Manager) calculateVoteWeight(agentID string) float64 {
	// Get agent health from Queen Coordinator
	healthMonitor := m.queen.GetHealthMonitor()
	if healthMonitor == nil {
		return 1.0
	}

	health, exists := healthMonitor.GetAgentHealth(agentID)
	if !exists {
		return 1.0
	}

	// Weight based on health score
	return health.HealthScore
}

// GetProposal returns a proposal by ID.
func (m *Manager) GetProposal(proposalID string) (*shared.Proposal, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proposal, exists := m.proposals[proposalID]
	return proposal, exists
}

// GetProposalResult returns the result of a proposal.
func (m *Manager) GetProposalResult(proposalID string) (*shared.ProposalResult, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, exists := m.results[proposalID]
	return result, exists
}

// GetProposalVotes returns the votes for a proposal.
func (m *Manager) GetProposalVotes(proposalID string) []shared.WeightedVote {
	m.mu.RLock()
	defer m.mu.RUnlock()

	votes, exists := m.votes[proposalID]
	if !exists {
		return nil
	}

	// Return a copy
	result := make([]shared.WeightedVote, len(votes))
	copy(result, votes)
	return result
}

// ListProposals returns all proposals with optional status filter.
func (m *Manager) ListProposals(status shared.ProposalStatus) []*shared.Proposal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*shared.Proposal, 0)
	for _, p := range m.proposals {
		if status == "" || p.Status == status {
			result = append(result, p)
		}
	}
	return result
}

// CancelProposal cancels a pending proposal.
func (m *Manager) CancelProposal(ctx context.Context, proposalID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proposal, exists := m.proposals[proposalID]
	if !exists {
		return shared.NewValidationError("proposal not found", map[string]interface{}{
			"proposalId": proposalID,
		})
	}

	if proposal.Status != shared.ProposalStatusPending && proposal.Status != shared.ProposalStatusVoting {
		return shared.NewValidationError("proposal cannot be cancelled", map[string]interface{}{
			"proposalId": proposalID,
			"status":     proposal.Status,
		})
	}

	proposal.Status = shared.ProposalStatusCancelled

	// Create result
	m.results[proposalID] = &shared.ProposalResult{
		ProposalID:       proposalID,
		Status:           shared.ProposalStatusCancelled,
		ConsensusReached: false,
		CompletedAt:      shared.Now(),
	}

	return nil
}

// CollectVotesSync synchronously collects votes from all agents.
func (m *Manager) CollectVotesSync(ctx context.Context, proposalID string) (*shared.ProposalResult, error) {
	m.mu.RLock()
	proposal, exists := m.proposals[proposalID]
	if !exists {
		m.mu.RUnlock()
		return nil, shared.NewValidationError("proposal not found", nil)
	}
	m.mu.RUnlock()

	// Get all available agents
	agents := make([]string, 0)
	for _, domain := range []shared.AgentDomain{
		shared.DomainQueen, shared.DomainSecurity, shared.DomainCore,
		shared.DomainIntegration, shared.DomainSupport,
	} {
		domainAgents := m.queen.GetAgentsByDomain(domain)
		for _, a := range domainAgents {
			agents = append(agents, a.ID)
		}
	}

	// Collect votes with timeout
	timeout := time.Duration(m.config.VoteTimeout) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	voteChan := make(chan shared.WeightedVote, len(agents))

	for _, agentID := range agents {
		wg.Add(1)
		go func(aid string) {
			defer wg.Done()

			// Simulate agent voting (in real implementation, this would query the agent)
			weight := m.calculateVoteWeight(aid)
			vote := shared.WeightedVote{
				AgentID:    aid,
				ProposalID: proposalID,
				Vote:       true, // Default to approval (real impl would have agent logic)
				Weight:     weight,
				Confidence: 0.8,
				Timestamp:  shared.Now(),
			}

			select {
			case voteChan <- vote:
			case <-ctx.Done():
			}
		}(agentID)
	}

	// Wait for all votes or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
	close(voteChan)

	// Collect all votes
	votes := make([]shared.WeightedVote, 0)
	for vote := range voteChan {
		votes = append(votes, vote)
	}

	// Store votes
	m.mu.Lock()
	m.votes[proposalID] = votes
	m.mu.Unlock()

	// Calculate result
	result := m.consensus.CalculateResult(votes, proposal.RequiredType, proposal.RequiredQuorum)
	result.ProposalID = proposalID

	// Finalize
	m.mu.Lock()
	m.finalizeProposal(proposalID, result)
	m.mu.Unlock()

	return result, nil
}

// GetState returns the current state of the Hive Mind.
func (m *Manager) GetState() shared.HiveMindState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeProposals := 0
	for _, p := range m.proposals {
		if p.Status == shared.ProposalStatusPending || p.Status == shared.ProposalStatusVoting {
			activeProposals++
		}
	}

	// Get agent counts from Queen
	domainHealth := m.queen.GetDomainHealth()
	totalAgents := 0
	activeAgents := 0
	domainsActive := make([]shared.AgentDomain, 0)

	for domain, health := range domainHealth {
		totalAgents += health.TotalAgents
		activeAgents += health.ActiveAgents
		if health.ActiveAgents > 0 {
			domainsActive = append(domainsActive, domain)
		}
	}

	queenAgentID := ""
	if queen := m.queen.GetQueen(); queen != nil {
		queenAgentID = queen.ID
	}

	return shared.HiveMindState{
		Initialized:     m.initialized,
		Algorithm:       m.config.ConsensusAlgorithm,
		ActiveProposals: activeProposals,
		TotalAgents:     totalAgents,
		ActiveAgents:    activeAgents,
		DomainsActive:   domainsActive,
		QueenAgentID:    queenAgentID,
	}
}

// GetQueen returns the Queen Coordinator.
func (m *Manager) GetQueen() *coordinator.QueenCoordinator {
	return m.queen
}

// GetLearning returns the Learning Module.
func (m *Manager) GetLearning() *LearningModule {
	return m.learning
}

// GetConfig returns the current configuration.
func (m *Manager) GetConfig() shared.HiveMindConfig {
	return m.config
}
