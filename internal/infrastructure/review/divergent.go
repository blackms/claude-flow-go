// Package review provides review infrastructure.
package review

import (
	"math"
	"sync"
	"time"

	domainReview "github.com/anthropics/claude-flow-go/internal/domain/review"
)

// Vote represents a vote in a consensus decision.
type Vote struct {
	// AgentID is who cast the vote.
	AgentID string `json:"agentId"`

	// Decision is the vote decision.
	Decision bool `json:"decision"`

	// Confidence is the confidence level.
	Confidence float64 `json:"confidence"`

	// Embedding is the agent's embedding (for similarity).
	Embedding []float64 `json:"embedding,omitempty"`

	// Timestamp is when the vote was cast.
	Timestamp time.Time `json:"timestamp"`
}

// DivergentDetectorConfig contains configuration for divergent detection.
type DivergentDetectorConfig struct {
	// DistanceThreshold is the threshold for divergence.
	DistanceThreshold float64

	// MinVotesForAnalysis is minimum votes needed.
	MinVotesForAnalysis int

	// HistoryWindow is how many votes to track.
	HistoryWindow int

	// DisagreementThreshold is when to flag as divergent.
	DisagreementThreshold float64
}

// DefaultDivergentDetectorConfig returns default configuration.
func DefaultDivergentDetectorConfig() DivergentDetectorConfig {
	return DivergentDetectorConfig{
		DistanceThreshold:     0.3,
		MinVotesForAnalysis:   3,
		HistoryWindow:         20,
		DisagreementThreshold: 0.4,
	}
}

// DivergentDetector detects agents that diverge from consensus.
type DivergentDetector struct {
	mu     sync.RWMutex
	config DivergentDetectorConfig

	// Vote history per agent
	voteHistory map[string][]bool

	// Embedding history per agent
	embeddings map[string][][]float64

	// Disagreement counts
	disagreements map[string]int
	totalVotes    map[string]int

	// Reputation scores
	reputations map[string]float64
}

// NewDivergentDetector creates a new divergent detector.
func NewDivergentDetector(config DivergentDetectorConfig) *DivergentDetector {
	return &DivergentDetector{
		config:        config,
		voteHistory:   make(map[string][]bool),
		embeddings:    make(map[string][][]float64),
		disagreements: make(map[string]int),
		totalVotes:    make(map[string]int),
		reputations:   make(map[string]float64),
	}
}

// DetectDivergent finds agents that voted against the majority.
func (d *DivergentDetector) DetectDivergent(votes []Vote, threshold float64) []domainReview.DivergentAgent {
	if len(votes) < d.config.MinVotesForAnalysis {
		return nil
	}

	// Count votes
	approvals := 0
	rejections := 0
	for _, vote := range votes {
		if vote.Decision {
			approvals++
		} else {
			rejections++
		}
	}

	// Determine majority
	majorityApproved := approvals > rejections
	majorityCount := approvals
	if !majorityApproved {
		majorityCount = rejections
	}

	// Consensus level
	consensusLevel := float64(majorityCount) / float64(len(votes))

	// Find divergent agents
	divergent := make([]domainReview.DivergentAgent, 0)
	for _, vote := range votes {
		isDivergent := vote.Decision != majorityApproved

		if isDivergent {
			// Calculate distance from majority
			distance := 1.0 - consensusLevel

			d.mu.Lock()
			// Update history
			d.voteHistory[vote.AgentID] = append(d.voteHistory[vote.AgentID], vote.Decision)
			if len(d.voteHistory[vote.AgentID]) > d.config.HistoryWindow {
				d.voteHistory[vote.AgentID] = d.voteHistory[vote.AgentID][1:]
			}

			// Update disagreement count
			d.disagreements[vote.AgentID]++
			d.totalVotes[vote.AgentID]++

			// Calculate disagreement rate
			disagreementRate := float64(d.disagreements[vote.AgentID]) / float64(d.totalVotes[vote.AgentID])
			d.mu.Unlock()

			if distance >= threshold {
				divergent = append(divergent, domainReview.DivergentAgent{
					AgentID:          vote.AgentID,
					Distance:         distance,
					VoteHistory:      d.getVoteHistory(vote.AgentID),
					DisagreementRate: disagreementRate,
					Reason:           "Voted against majority consensus",
				})
			}
		} else {
			d.mu.Lock()
			d.totalVotes[vote.AgentID]++
			d.mu.Unlock()
		}
	}

	return divergent
}

// DetectDivergentByEmbedding finds agents whose embeddings differ from the group.
func (d *DivergentDetector) DetectDivergentByEmbedding(votes []Vote) []domainReview.DivergentAgent {
	if len(votes) < d.config.MinVotesForAnalysis {
		return nil
	}

	// Filter votes with embeddings
	votesWithEmbeddings := make([]Vote, 0)
	for _, v := range votes {
		if len(v.Embedding) > 0 {
			votesWithEmbeddings = append(votesWithEmbeddings, v)
		}
	}

	if len(votesWithEmbeddings) < 2 {
		return nil
	}

	// Compute centroid
	centroid := d.computeCentroid(votesWithEmbeddings)

	// Find agents far from centroid
	divergent := make([]domainReview.DivergentAgent, 0)
	for _, vote := range votesWithEmbeddings {
		distance := d.computeDistance(vote.Embedding, centroid)

		if distance > d.config.DistanceThreshold {
			divergent = append(divergent, domainReview.DivergentAgent{
				AgentID:  vote.AgentID,
				Distance: distance,
				Reason:   "Embedding significantly differs from group centroid",
			})
		}
	}

	return divergent
}

// computeCentroid computes the centroid of vote embeddings.
func (d *DivergentDetector) computeCentroid(votes []Vote) []float64 {
	if len(votes) == 0 {
		return nil
	}

	dim := len(votes[0].Embedding)
	centroid := make([]float64, dim)

	for _, vote := range votes {
		for i := 0; i < dim && i < len(vote.Embedding); i++ {
			centroid[i] += vote.Embedding[i]
		}
	}

	n := float64(len(votes))
	for i := range centroid {
		centroid[i] /= n
	}

	return centroid
}

// computeDistance computes Euclidean distance between two vectors.
func (d *DivergentDetector) computeDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return 1.0
	}

	var sum float64
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

// getVoteHistory returns an agent's vote history.
func (d *DivergentDetector) getVoteHistory(agentID string) []bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if history, ok := d.voteHistory[agentID]; ok {
		result := make([]bool, len(history))
		copy(result, history)
		return result
	}
	return nil
}

// GetDisagreementRate returns an agent's disagreement rate.
func (d *DivergentDetector) GetDisagreementRate(agentID string) float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	total := d.totalVotes[agentID]
	if total == 0 {
		return 0
	}

	return float64(d.disagreements[agentID]) / float64(total)
}

// GetReputation returns an agent's reputation score.
func (d *DivergentDetector) GetReputation(agentID string) float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if rep, ok := d.reputations[agentID]; ok {
		return rep
	}
	return 0.5 // Default neutral reputation
}

// UpdateReputation updates an agent's reputation based on outcome.
func (d *DivergentDetector) UpdateReputation(agentID string, wasCorrect bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	current := d.reputations[agentID]
	if current == 0 {
		current = 0.5
	}

	// Exponential moving average
	alpha := 0.1
	if wasCorrect {
		d.reputations[agentID] = current + alpha*(1.0-current)
	} else {
		d.reputations[agentID] = current - alpha*current
	}

	// Clamp to [0, 1]
	if d.reputations[agentID] < 0 {
		d.reputations[agentID] = 0
	}
	if d.reputations[agentID] > 1 {
		d.reputations[agentID] = 1
	}
}

// GetChronicDisagreers returns agents who frequently disagree.
func (d *DivergentDetector) GetChronicDisagreers() []domainReview.DivergentAgent {
	d.mu.RLock()
	defer d.mu.RUnlock()

	divergent := make([]domainReview.DivergentAgent, 0)

	for agentID, total := range d.totalVotes {
		if total < d.config.MinVotesForAnalysis {
			continue
		}

		rate := float64(d.disagreements[agentID]) / float64(total)
		if rate >= d.config.DisagreementThreshold {
			divergent = append(divergent, domainReview.DivergentAgent{
				AgentID:          agentID,
				Distance:         rate,
				VoteHistory:      d.getVoteHistoryUnlocked(agentID),
				DisagreementRate: rate,
				Reason:           "Chronically disagrees with majority",
			})
		}
	}

	return divergent
}

func (d *DivergentDetector) getVoteHistoryUnlocked(agentID string) []bool {
	if history, ok := d.voteHistory[agentID]; ok {
		result := make([]bool, len(history))
		copy(result, history)
		return result
	}
	return nil
}

// Reset clears all history.
func (d *DivergentDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.voteHistory = make(map[string][]bool)
	d.embeddings = make(map[string][][]float64)
	d.disagreements = make(map[string]int)
	d.totalVotes = make(map[string]int)
}

// GetStats returns detector statistics.
func (d *DivergentDetector) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"trackedAgents":    len(d.totalVotes),
		"totalVotesTracked": d.sumVotes(),
		"chronicDisagreers": len(d.GetChronicDisagreers()),
	}
}

func (d *DivergentDetector) sumVotes() int {
	total := 0
	for _, v := range d.totalVotes {
		total += v
	}
	return total
}
