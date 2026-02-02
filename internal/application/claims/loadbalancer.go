// Package claims provides application services for the claims system.
package claims

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	domainClaims "github.com/anthropics/claude-flow-go/internal/domain/claims"
	infraClaims "github.com/anthropics/claude-flow-go/internal/infrastructure/claims"
)

// LoadBalancer provides load balancing operations for claims.
type LoadBalancer struct {
	mu            sync.RWMutex
	eventStore    infraClaims.EventStore
	claimRepo     infraClaims.ClaimRepository
	claimantRepo  infraClaims.ClaimantRepository
	config        domainClaims.LoadConfig
	swarms        map[string]*Swarm
	correlationID string
}

// Swarm represents a group of agents that can share work.
type Swarm struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	AgentIDs  []string `json:"agentIds"`
	CreatedAt time.Time `json:"createdAt"`
}

// AgentLoad represents the load information for an agent.
type AgentLoad struct {
	AgentID         string  `json:"agentId"`
	AgentName       string  `json:"agentName"`
	CurrentWorkload int     `json:"currentWorkload"`
	MaxConcurrent   int     `json:"maxConcurrent"`
	Utilization     float64 `json:"utilization"`
	ActiveClaims    int     `json:"activeClaims"`
	IsOverloaded    bool    `json:"isOverloaded"`
	IsUnderloaded   bool    `json:"isUnderloaded"`
}

// SwarmLoad represents the load information for a swarm.
type SwarmLoad struct {
	SwarmID           string       `json:"swarmId"`
	SwarmName         string       `json:"swarmName"`
	AgentCount        int          `json:"agentCount"`
	TotalCapacity     int          `json:"totalCapacity"`
	TotalWorkload     int          `json:"totalWorkload"`
	AverageUtilization float64     `json:"averageUtilization"`
	OverloadedAgents  int          `json:"overloadedAgents"`
	UnderloadedAgents int          `json:"underloadedAgents"`
	NeedsRebalancing  bool         `json:"needsRebalancing"`
	AgentLoads        []*AgentLoad `json:"agentLoads"`
}

// RebalanceResult represents the result of a rebalancing operation.
type RebalanceResult struct {
	SwarmID       string                       `json:"swarmId"`
	MovesExecuted int                          `json:"movesExecuted"`
	MovesFailed   int                          `json:"movesFailed"`
	Moves         []domainClaims.RebalanceMove `json:"moves"`
	Duration      time.Duration                `json:"duration"`
	Error         error                        `json:"error,omitempty"`
}

// NewLoadBalancer creates a new load balancer.
func NewLoadBalancer(
	eventStore infraClaims.EventStore,
	claimRepo infraClaims.ClaimRepository,
	claimantRepo infraClaims.ClaimantRepository,
) *LoadBalancer {
	return &LoadBalancer{
		eventStore:   eventStore,
		claimRepo:    claimRepo,
		claimantRepo: claimantRepo,
		config:       domainClaims.DefaultLoadConfig(),
		swarms:       make(map[string]*Swarm),
	}
}

// SetConfig sets the load configuration.
func (lb *LoadBalancer) SetConfig(config domainClaims.LoadConfig) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.config = config
}

// SetCorrelationID sets the correlation ID for operations.
func (lb *LoadBalancer) SetCorrelationID(correlationID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.correlationID = correlationID
}

// CreateSwarm creates a new swarm.
func (lb *LoadBalancer) CreateSwarm(id, name string, agentIDs []string) (*Swarm, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if _, exists := lb.swarms[id]; exists {
		return nil, fmt.Errorf("swarm already exists: %s", id)
	}

	swarm := &Swarm{
		ID:        id,
		Name:      name,
		AgentIDs:  agentIDs,
		CreatedAt: time.Now(),
	}

	lb.swarms[id] = swarm
	return swarm, nil
}

// AddAgentToSwarm adds an agent to a swarm.
func (lb *LoadBalancer) AddAgentToSwarm(swarmID, agentID string) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	swarm, exists := lb.swarms[swarmID]
	if !exists {
		return fmt.Errorf("swarm not found: %s", swarmID)
	}

	// Check if already in swarm
	for _, id := range swarm.AgentIDs {
		if id == agentID {
			return nil
		}
	}

	swarm.AgentIDs = append(swarm.AgentIDs, agentID)
	return nil
}

// RemoveAgentFromSwarm removes an agent from a swarm.
func (lb *LoadBalancer) RemoveAgentFromSwarm(swarmID, agentID string) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	swarm, exists := lb.swarms[swarmID]
	if !exists {
		return fmt.Errorf("swarm not found: %s", swarmID)
	}

	for i, id := range swarm.AgentIDs {
		if id == agentID {
			swarm.AgentIDs = append(swarm.AgentIDs[:i], swarm.AgentIDs[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("agent not in swarm: %s", agentID)
}

// GetAgentLoad returns load information for an agent.
func (lb *LoadBalancer) GetAgentLoad(agentID string) (*AgentLoad, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	claimant, err := lb.claimantRepo.FindByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	activeClaims := lb.claimRepo.CountByClaimant(agentID)

	// Calculate average utilization for comparison
	allClaimants, _ := lb.claimantRepo.FindAll()
	avgUtilization := lb.calculateAverageUtilization(allClaimants)

	load := &AgentLoad{
		AgentID:         agentID,
		AgentName:       claimant.Name,
		CurrentWorkload: claimant.CurrentWorkload,
		MaxConcurrent:   claimant.MaxConcurrent,
		Utilization:     claimant.Utilization(),
		ActiveClaims:    activeClaims,
		IsOverloaded:    domainClaims.IsAgentOverloaded(claimant, avgUtilization, lb.config),
		IsUnderloaded:   domainClaims.IsAgentUnderloaded(claimant, avgUtilization, lb.config),
	}

	return load, nil
}

// GetSwarmLoad returns load information for a swarm.
func (lb *LoadBalancer) GetSwarmLoad(swarmID string) (*SwarmLoad, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	swarm, exists := lb.swarms[swarmID]
	if !exists {
		return nil, fmt.Errorf("swarm not found: %s", swarmID)
	}

	agentLoads := make([]*AgentLoad, 0, len(swarm.AgentIDs))
	totalCapacity := 0
	totalWorkload := 0
	overloaded := 0
	underloaded := 0
	utilizations := make([]float64, 0, len(swarm.AgentIDs))

	for _, agentID := range swarm.AgentIDs {
		claimant, err := lb.claimantRepo.FindByID(agentID)
		if err != nil {
			continue
		}

		totalCapacity += claimant.MaxConcurrent
		totalWorkload += claimant.CurrentWorkload
		utilizations = append(utilizations, claimant.Utilization())
	}

	avgUtilization := 0.0
	if len(utilizations) > 0 {
		sum := 0.0
		for _, u := range utilizations {
			sum += u
		}
		avgUtilization = sum / float64(len(utilizations))
	}

	for _, agentID := range swarm.AgentIDs {
		claimant, err := lb.claimantRepo.FindByID(agentID)
		if err != nil {
			continue
		}

		activeClaims := lb.claimRepo.CountByClaimant(agentID)

		isOverloaded := domainClaims.IsAgentOverloaded(claimant, avgUtilization, lb.config)
		isUnderloaded := domainClaims.IsAgentUnderloaded(claimant, avgUtilization, lb.config)

		if isOverloaded {
			overloaded++
		}
		if isUnderloaded {
			underloaded++
		}

		agentLoads = append(agentLoads, &AgentLoad{
			AgentID:         agentID,
			AgentName:       claimant.Name,
			CurrentWorkload: claimant.CurrentWorkload,
			MaxConcurrent:   claimant.MaxConcurrent,
			Utilization:     claimant.Utilization(),
			ActiveClaims:    activeClaims,
			IsOverloaded:    isOverloaded,
			IsUnderloaded:   isUnderloaded,
		})
	}

	needsRebalancing := domainClaims.NeedsRebalancing(utilizations, lb.config)

	return &SwarmLoad{
		SwarmID:            swarmID,
		SwarmName:          swarm.Name,
		AgentCount:         len(swarm.AgentIDs),
		TotalCapacity:      totalCapacity,
		TotalWorkload:      totalWorkload,
		AverageUtilization: avgUtilization,
		OverloadedAgents:   overloaded,
		UnderloadedAgents:  underloaded,
		NeedsRebalancing:   needsRebalancing,
		AgentLoads:         agentLoads,
	}, nil
}

// Rebalance triggers rebalancing for a swarm.
func (lb *LoadBalancer) Rebalance(swarmID string) (*RebalanceResult, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	startTime := time.Now()

	swarm, exists := lb.swarms[swarmID]
	if !exists {
		return nil, fmt.Errorf("swarm not found: %s", swarmID)
	}

	result := &RebalanceResult{
		SwarmID: swarmID,
		Moves:   make([]domainClaims.RebalanceMove, 0),
	}

	// Get all agents in swarm
	agents := make([]*domainClaims.Claimant, 0, len(swarm.AgentIDs))
	for _, agentID := range swarm.AgentIDs {
		claimant, err := lb.claimantRepo.FindByID(agentID)
		if err != nil {
			continue
		}
		agents = append(agents, claimant)
	}

	if len(agents) < 2 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Calculate average utilization
	avgUtilization := lb.calculateAverageUtilization(agents)

	// Find overloaded and underloaded agents
	overloaded := make([]*domainClaims.Claimant, 0)
	underloaded := make([]*domainClaims.Claimant, 0)

	for _, agent := range agents {
		if domainClaims.IsAgentOverloaded(agent, avgUtilization, lb.config) {
			overloaded = append(overloaded, agent)
		} else if domainClaims.IsAgentUnderloaded(agent, avgUtilization, lb.config) {
			underloaded = append(underloaded, agent)
		}
	}

	// Sort overloaded by utilization (highest first)
	sort.Slice(overloaded, func(i, j int) bool {
		return overloaded[i].Utilization() > overloaded[j].Utilization()
	})

	// Sort underloaded by utilization (lowest first)
	sort.Slice(underloaded, func(i, j int) bool {
		return underloaded[i].Utilization() < underloaded[j].Utilization()
	})

	// Move claims from overloaded to underloaded
	for _, from := range overloaded {
		if len(underloaded) == 0 {
			break
		}

		// Get claims for this agent
		claims, err := lb.claimRepo.FindByClaimant(from.ID)
		if err != nil {
			continue
		}

		// Filter movable claims
		movable := make([]*domainClaims.Claim, 0)
		for _, claim := range claims {
			if claim.IsActive() {
				canMove, _ := domainClaims.CanMoveClaim(claim, lb.config)
				if canMove {
					movable = append(movable, claim)
				}
			}
		}

		// Sort by progress (lowest first)
		sort.Slice(movable, func(i, j int) bool {
			return movable[i].Progress < movable[j].Progress
		})

		// Move claims until agent is no longer overloaded
		for _, claim := range movable {
			if !domainClaims.IsAgentOverloaded(from, avgUtilization, lb.config) {
				break
			}

			// Find best target
			var target *domainClaims.Claimant
			for _, to := range underloaded {
				if to.CanAcceptMoreWork() {
					target = to
					break
				}
			}

			if target == nil {
				break
			}

			// Execute move
			if err := lb.executeMove(claim, from, target); err != nil {
				result.MovesFailed++
				continue
			}

			result.MovesExecuted++
			result.Moves = append(result.Moves, domainClaims.RebalanceMove{
				ClaimID:     claim.ID,
				FromAgentID: from.ID,
				ToAgentID:   target.ID,
			})

			// Update local state
			from.DecrementWorkload()
			target.IncrementWorkload()

			// Re-evaluate underloaded
			if !domainClaims.IsAgentUnderloaded(target, avgUtilization, lb.config) {
				// Remove from underloaded
				for i, u := range underloaded {
					if u.ID == target.ID {
						underloaded = append(underloaded[:i], underloaded[i+1:]...)
						break
					}
				}
			}
		}
	}

	result.Duration = time.Since(startTime)

	// Emit rebalance event
	if len(result.Moves) > 0 {
		event := domainClaims.NewSwarmRebalancedEvent(swarmID, result.Moves)
		if lb.correlationID != "" {
			event.WithCorrelation(lb.correlationID)
		}
		_ = lb.eventStore.Append(event)
	}

	return result, nil
}

// executeMove moves a claim from one agent to another.
func (lb *LoadBalancer) executeMove(claim *domainClaims.Claim, from, to *domainClaims.Claimant) error {
	// Update claim
	claim.Claimant = *to
	claim.LastActivityAt = time.Now()
	claim.Version++
	claim.AddNote(fmt.Sprintf("Moved from %s to %s by load balancer", from.Name, to.Name))

	if err := lb.claimRepo.Save(claim); err != nil {
		return fmt.Errorf("failed to save claim: %w", err)
	}

	// Update claimant workloads
	fromUpdated, err := lb.claimantRepo.FindByID(from.ID)
	if err == nil {
		fromUpdated.DecrementWorkload()
		_ = lb.claimantRepo.Save(fromUpdated)
	}

	toUpdated, err := lb.claimantRepo.FindByID(to.ID)
	if err == nil {
		toUpdated.IncrementWorkload()
		_ = lb.claimantRepo.Save(toUpdated)
	}

	return nil
}

// DetectImbalance checks if a swarm needs rebalancing.
func (lb *LoadBalancer) DetectImbalance(swarmID string) (bool, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	swarm, exists := lb.swarms[swarmID]
	if !exists {
		return false, fmt.Errorf("swarm not found: %s", swarmID)
	}

	utilizations := make([]float64, 0, len(swarm.AgentIDs))
	for _, agentID := range swarm.AgentIDs {
		claimant, err := lb.claimantRepo.FindByID(agentID)
		if err != nil {
			continue
		}
		utilizations = append(utilizations, claimant.Utilization())
	}

	return domainClaims.NeedsRebalancing(utilizations, lb.config), nil
}

// calculateAverageUtilization calculates the average utilization of agents.
func (lb *LoadBalancer) calculateAverageUtilization(agents []*domainClaims.Claimant) float64 {
	if len(agents) == 0 {
		return 0
	}

	sum := 0.0
	for _, agent := range agents {
		sum += agent.Utilization()
	}

	return sum / float64(len(agents))
}

// GetSwarm returns a swarm by ID.
func (lb *LoadBalancer) GetSwarm(swarmID string) (*Swarm, bool) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	swarm, exists := lb.swarms[swarmID]
	if !exists {
		return nil, false
	}
	swarmCopy := *swarm
	return &swarmCopy, true
}

// GetAllSwarms returns all swarms.
func (lb *LoadBalancer) GetAllSwarms() []*Swarm {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]*Swarm, 0, len(lb.swarms))
	for _, swarm := range lb.swarms {
		swarmCopy := *swarm
		result = append(result, &swarmCopy)
	}
	return result
}

// DeleteSwarm deletes a swarm.
func (lb *LoadBalancer) DeleteSwarm(swarmID string) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if _, exists := lb.swarms[swarmID]; !exists {
		return fmt.Errorf("swarm not found: %s", swarmID)
	}

	delete(lb.swarms, swarmID)
	return nil
}

// GetStats returns load balancer statistics.
func (lb *LoadBalancer) GetStats() LoadBalancerStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	totalSwarms := len(lb.swarms)
	totalAgents := 0
	swarmsNeedingRebalance := 0

	for _, swarm := range lb.swarms {
		totalAgents += len(swarm.AgentIDs)

		utilizations := make([]float64, 0, len(swarm.AgentIDs))
		for _, agentID := range swarm.AgentIDs {
			claimant, err := lb.claimantRepo.FindByID(agentID)
			if err == nil {
				utilizations = append(utilizations, claimant.Utilization())
			}
		}

		if domainClaims.NeedsRebalancing(utilizations, lb.config) {
			swarmsNeedingRebalance++
		}
	}

	return LoadBalancerStats{
		TotalSwarms:            totalSwarms,
		TotalAgents:            totalAgents,
		SwarmsNeedingRebalance: swarmsNeedingRebalance,
		Config:                 lb.config,
	}
}

// LoadBalancerStats holds load balancer statistics.
type LoadBalancerStats struct {
	TotalSwarms            int                      `json:"totalSwarms"`
	TotalAgents            int                      `json:"totalAgents"`
	SwarmsNeedingRebalance int                      `json:"swarmsNeedingRebalance"`
	Config                 domainClaims.LoadConfig  `json:"config"`
}

// AutoRebalanceAll automatically rebalances all swarms that need it.
func (lb *LoadBalancer) AutoRebalanceAll() ([]*RebalanceResult, error) {
	lb.mu.Lock()
	swarmIDs := make([]string, 0, len(lb.swarms))
	for id := range lb.swarms {
		swarmIDs = append(swarmIDs, id)
	}
	lb.mu.Unlock()

	results := make([]*RebalanceResult, 0)

	for _, swarmID := range swarmIDs {
		needsRebalance, err := lb.DetectImbalance(swarmID)
		if err != nil {
			continue
		}

		if needsRebalance {
			result, err := lb.Rebalance(swarmID)
			if err != nil {
				result = &RebalanceResult{SwarmID: swarmID, Error: err}
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// GetGiniCoefficient calculates the Gini coefficient for workload distribution.
func (lb *LoadBalancer) GetGiniCoefficient(swarmID string) (float64, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	swarm, exists := lb.swarms[swarmID]
	if !exists {
		return 0, fmt.Errorf("swarm not found: %s", swarmID)
	}

	workloads := make([]float64, 0, len(swarm.AgentIDs))
	for _, agentID := range swarm.AgentIDs {
		claimant, err := lb.claimantRepo.FindByID(agentID)
		if err != nil {
			continue
		}
		workloads = append(workloads, float64(claimant.CurrentWorkload))
	}

	if len(workloads) < 2 {
		return 0, nil
	}

	// Sort workloads
	sort.Float64s(workloads)

	// Calculate Gini coefficient
	n := float64(len(workloads))
	var sumOfDifferences float64
	var sumOfWorkloads float64

	for i, wi := range workloads {
		sumOfWorkloads += wi
		for j, wj := range workloads {
			if i != j {
				sumOfDifferences += math.Abs(wi - wj)
			}
		}
	}

	if sumOfWorkloads == 0 {
		return 0, nil
	}

	gini := sumOfDifferences / (2 * n * sumOfWorkloads)
	return gini, nil
}
