// Package review provides review application services.
package review

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	domainReview "github.com/anthropics/claude-flow-go/internal/domain/review"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ReviewSwarmConfig contains configuration for review swarms.
type ReviewSwarmConfig struct {
	// MinReviewers is the minimum reviewers per type.
	MinReviewers int

	// MaxReviewers is the maximum reviewers per type.
	MaxReviewers int

	// Timeout is the review timeout.
	Timeout time.Duration

	// ParallelExecution enables parallel reviews.
	ParallelExecution bool

	// RequireUnanimity requires unanimous approval.
	RequireUnanimity bool

	// IncludeAdversarial includes adversarial reviewers.
	IncludeAdversarial bool
}

// DefaultReviewSwarmConfig returns default configuration.
func DefaultReviewSwarmConfig() ReviewSwarmConfig {
	return ReviewSwarmConfig{
		MinReviewers:       1,
		MaxReviewers:       3,
		Timeout:            5 * time.Minute,
		ParallelExecution:  true,
		RequireUnanimity:   false,
		IncludeAdversarial: true,
	}
}

// ReviewSwarmMember represents a member of a review swarm.
type ReviewSwarmMember struct {
	// AgentID is the agent identifier.
	AgentID string `json:"agentId"`

	// AgentType is the type of agent.
	AgentType shared.AgentType `json:"agentType"`

	// ReviewType is the review type assigned.
	ReviewType domainReview.ReviewType `json:"reviewType"`

	// Role is the adversarial role.
	Role domainReview.AdversarialRole `json:"role"`

	// Status is the member status.
	Status string `json:"status"` // pending, reviewing, completed

	// Verdict is the submitted verdict.
	Verdict *domainReview.ReviewVerdict `json:"verdict,omitempty"`
}

// ReviewSwarm represents a swarm of reviewers.
type ReviewSwarm struct {
	// SwarmID is the unique swarm identifier.
	SwarmID string `json:"swarmId"`

	// WorkID is the work being reviewed.
	WorkID string `json:"workId"`

	// Config is the swarm configuration.
	Config ReviewSwarmConfig `json:"config"`

	// Members are the swarm members.
	Members []ReviewSwarmMember `json:"members"`

	// Status is the swarm status.
	Status domainReview.WorkflowStatus `json:"status"`

	// FinalVerdict is the aggregated verdict.
	FinalVerdict *domainReview.FinalVerdict `json:"finalVerdict,omitempty"`

	// CreatedAt is when the swarm was created.
	CreatedAt time.Time `json:"createdAt"`

	// CompletedAt is when the swarm completed.
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// AdversarialSwarm manages multi-agent adversarial review swarms.
type AdversarialSwarm struct {
	mu sync.RWMutex

	// Active swarms
	swarms map[string]*ReviewSwarm

	// Aggregator for combining verdicts
	aggregator *VerdictAggregator

	// Agent pool for spawning reviewers
	agentPool map[shared.AgentType][]string
}

// NewAdversarialSwarm creates a new adversarial swarm manager.
func NewAdversarialSwarm() *AdversarialSwarm {
	return &AdversarialSwarm{
		swarms:     make(map[string]*ReviewSwarm),
		aggregator: NewVerdictAggregator(DefaultAggregatorConfig()),
		agentPool:  make(map[shared.AgentType][]string),
	}
}

// RegisterAgent registers an agent in the pool.
func (as *AdversarialSwarm) RegisterAgent(agentID string, agentType shared.AgentType) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.agentPool[agentType] = append(as.agentPool[agentType], agentID)
}

// SpawnReviewSwarm spawns a new review swarm for a work item.
func (as *AdversarialSwarm) SpawnReviewSwarm(
	ctx context.Context,
	work domainReview.Work,
	config ReviewSwarmConfig,
) (*ReviewSwarm, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	swarmID := uuid.New().String()
	now := time.Now()

	swarm := &ReviewSwarm{
		SwarmID:   swarmID,
		WorkID:    work.WorkID,
		Config:    config,
		Members:   make([]ReviewSwarmMember, 0),
		Status:    domainReview.WorkflowStatusPending,
		CreatedAt: now,
	}

	// Assign reviewers for each review type
	reviewTypes := []struct {
		reviewType domainReview.ReviewType
		agentTypes []shared.AgentType
		role       domainReview.AdversarialRole
	}{
		{
			domainReview.ReviewTypeSecurity,
			[]shared.AgentType{shared.AgentTypeRedTeam, shared.AgentTypeSecurityAuditor},
			domainReview.RoleChallenger,
		},
		{
			domainReview.ReviewTypeQuality,
			[]shared.AgentType{shared.AgentTypeCritic, shared.AgentTypeReviewer},
			domainReview.RoleChallenger,
		},
		{
			domainReview.ReviewTypePerformance,
			[]shared.AgentType{shared.AgentTypePerformanceEngineer},
			domainReview.RoleChallenger,
		},
		{
			domainReview.ReviewTypeArchitecture,
			[]shared.AgentType{shared.AgentTypeDevilsAdvocate, shared.AgentTypeArchitect},
			domainReview.RoleChallenger,
		},
	}

	for _, rt := range reviewTypes {
		for _, agentType := range rt.agentTypes {
			agents := as.agentPool[agentType]
			for _, agentID := range agents {
				// Skip if this is the author
				if agentID == work.AuthorID {
					continue
				}

				swarm.Members = append(swarm.Members, ReviewSwarmMember{
					AgentID:    agentID,
					AgentType:  agentType,
					ReviewType: rt.reviewType,
					Role:       rt.role,
					Status:     "pending",
				})

				if len(swarm.Members) >= config.MaxReviewers*4 { // 4 review types
					break
				}
			}
			if len(swarm.Members) >= config.MaxReviewers*4 {
				break
			}
		}
	}

	// Add a quality gate judge if we have adversarial reviewers
	if config.IncludeAdversarial {
		judges := as.agentPool[shared.AgentTypeQualityGate]
		if len(judges) > 0 {
			swarm.Members = append(swarm.Members, ReviewSwarmMember{
				AgentID:    judges[0],
				AgentType:  shared.AgentTypeQualityGate,
				ReviewType: domainReview.ReviewTypeFull,
				Role:       domainReview.RoleJudge,
				Status:     "pending",
			})
		}
	}

	if len(swarm.Members) == 0 {
		return nil, fmt.Errorf("no reviewers available for swarm")
	}

	swarm.Status = domainReview.WorkflowStatusInProgress
	as.swarms[swarmID] = swarm

	return swarm, nil
}

// SubmitMemberVerdict submits a verdict from a swarm member.
func (as *AdversarialSwarm) SubmitMemberVerdict(
	swarmID string,
	agentID string,
	verdict domainReview.ReviewVerdict,
) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	swarm, ok := as.swarms[swarmID]
	if !ok {
		return fmt.Errorf("swarm not found: %s", swarmID)
	}

	// Find and update member
	found := false
	for i, member := range swarm.Members {
		if member.AgentID == agentID {
			if member.Status == "completed" {
				return fmt.Errorf("member already submitted verdict")
			}
			swarm.Members[i].Status = "completed"
			swarm.Members[i].Verdict = &verdict
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("agent not a member of swarm: %s", agentID)
	}

	// Check if all members have submitted
	allComplete := true
	for _, member := range swarm.Members {
		if member.Status != "completed" {
			allComplete = false
			break
		}
	}

	if allComplete {
		as.finalizeSwarm(swarm)
	}

	return nil
}

func (as *AdversarialSwarm) finalizeSwarm(swarm *ReviewSwarm) {
	// Collect all verdicts
	verdicts := make([]domainReview.ReviewVerdict, 0)
	for _, member := range swarm.Members {
		if member.Verdict != nil {
			verdicts = append(verdicts, *member.Verdict)
		}
	}

	// Aggregate verdicts
	finalVerdict, _ := as.aggregator.AggregateVerdicts(verdicts, swarm.Config.RequireUnanimity)
	swarm.FinalVerdict = finalVerdict

	now := time.Now()
	swarm.CompletedAt = &now
	swarm.Status = domainReview.WorkflowStatusCompleted
}

// GetSwarm returns a swarm by ID.
func (as *AdversarialSwarm) GetSwarm(swarmID string) (*ReviewSwarm, bool) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	swarm, ok := as.swarms[swarmID]
	return swarm, ok
}

// GetActiveSwarms returns all active swarms.
func (as *AdversarialSwarm) GetActiveSwarms() []*ReviewSwarm {
	as.mu.RLock()
	defer as.mu.RUnlock()

	result := make([]*ReviewSwarm, 0)
	for _, swarm := range as.swarms {
		if swarm.Status == domainReview.WorkflowStatusInProgress {
			result = append(result, swarm)
		}
	}

	return result
}

// GetSwarmStats returns swarm statistics.
func (as *AdversarialSwarm) GetSwarmStats() map[string]interface{} {
	as.mu.RLock()
	defer as.mu.RUnlock()

	total := len(as.swarms)
	active := 0
	completed := 0

	for _, swarm := range as.swarms {
		if swarm.Status == domainReview.WorkflowStatusInProgress {
			active++
		} else if swarm.Status == domainReview.WorkflowStatusCompleted {
			completed++
		}
	}

	agentCounts := make(map[string]int)
	for agentType, agents := range as.agentPool {
		agentCounts[string(agentType)] = len(agents)
	}

	return map[string]interface{}{
		"totalSwarms":     total,
		"activeSwarms":    active,
		"completedSwarms": completed,
		"agentPool":       agentCounts,
	}
}

// GetAggregator returns the verdict aggregator.
func (as *AdversarialSwarm) GetAggregator() *VerdictAggregator {
	return as.aggregator
}
