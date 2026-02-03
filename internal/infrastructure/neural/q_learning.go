// Package neural provides neural network infrastructure.
package neural

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// QLearningAlgorithm implements tabular Q-Learning.
type QLearningAlgorithm struct {
	mu     sync.RWMutex
	config domainNeural.QLearningConfig
	rng    *rand.Rand

	// Q-table: stateHash -> action -> value
	qTable map[uint64][]float64

	// LRU tracking for pruning
	stateAccessTime map[uint64]time.Time

	// Eligibility traces
	traces map[uint64][]float64

	// Last state-action for TD update
	lastState  []float32
	lastAction int

	// Exploration
	epsilon   float64
	stepCount int64

	// Statistics
	updateCount  int64
	avgTDError   float64
	avgLatency   float64
}

// NewQLearningAlgorithm creates a new Q-Learning algorithm.
func NewQLearningAlgorithm(config domainNeural.QLearningConfig) *QLearningAlgorithm {
	return &QLearningAlgorithm{
		config:          config,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
		qTable:          make(map[uint64][]float64),
		stateAccessTime: make(map[uint64]time.Time),
		traces:          make(map[uint64][]float64),
		epsilon:         config.ExplorationInitial,
	}
}

// AddExperience adds experience (not used for tabular methods, use Update instead).
func (q *QLearningAlgorithm) AddExperience(exp domainNeural.RLExperience) {
	// Tabular methods don't use buffer
	q.UpdateFromExperience(exp)
}

// UpdateFromExperience updates Q-values from a single experience.
// Target: <1ms
func (q *QLearningAlgorithm) UpdateFromExperience(exp domainNeural.RLExperience) domainNeural.RLUpdateResult {
	startTime := time.Now()

	q.mu.Lock()
	defer q.mu.Unlock()

	stateHash := q.hashState(exp.State)
	nextStateHash := q.hashState(exp.NextState)

	// Ensure states exist in Q-table
	q.ensureState(stateHash)
	q.ensureState(nextStateHash)

	// Get current Q-value
	currentQ := q.qTable[stateHash][exp.Action]

	// Compute target
	var targetQ float64
	if exp.Done {
		targetQ = exp.Reward
	} else {
		// Q-Learning: max over next state actions
		maxNextQ := q.maxQ(nextStateHash)
		targetQ = exp.Reward + q.config.Gamma*maxNextQ
	}

	// TD error
	tdError := targetQ - currentQ

	// Update Q-value
	if q.config.UseEligibilityTraces {
		// Update with eligibility traces
		q.updateWithTraces(stateHash, exp.Action, tdError)
	} else {
		// Standard Q-learning update
		q.qTable[stateHash][exp.Action] += q.config.LearningRate * tdError
	}

	// Update access time for LRU
	q.stateAccessTime[stateHash] = time.Now()
	q.stateAccessTime[nextStateHash] = time.Now()

	// Prune if needed
	if len(q.qTable) > q.config.MaxStates {
		q.pruneOldStates()
	}

	// Decay exploration
	q.stepCount++
	q.epsilon = math.Max(
		q.config.ExplorationFinal,
		q.config.ExplorationInitial-float64(q.stepCount)/float64(q.config.ExplorationDecay),
	)

	q.updateCount++
	q.avgTDError = (q.avgTDError*float64(q.updateCount-1) + math.Abs(tdError)) / float64(q.updateCount)

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	q.avgLatency = (q.avgLatency*float64(q.updateCount-1) + elapsed) / float64(q.updateCount)

	return domainNeural.RLUpdateResult{
		TDError:   tdError,
		Epsilon:   q.epsilon,
		LatencyMs: elapsed,
	}
}

// Update performs batch update (for interface compliance).
func (q *QLearningAlgorithm) Update() domainNeural.RLUpdateResult {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return domainNeural.RLUpdateResult{
		TDError:   q.avgTDError,
		Epsilon:   q.epsilon,
		LatencyMs: q.avgLatency,
	}
}

// GetAction returns an action using epsilon-greedy.
func (q *QLearningAlgorithm) GetAction(state []float32, explore bool) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	stateHash := q.hashState(state)
	q.ensureState(stateHash)
	q.stateAccessTime[stateHash] = time.Now()

	if explore && q.rng.Float64() < q.epsilon {
		return q.rng.Intn(q.config.NumActions)
	}

	return q.argmaxQ(stateHash)
}

// GetQValue returns the Q-value for a state-action pair.
func (q *QLearningAlgorithm) GetQValue(state []float32, action int) float64 {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stateHash := q.hashState(state)
	if qValues, ok := q.qTable[stateHash]; ok && action < len(qValues) {
		return qValues[action]
	}
	return 0
}

// GetQValues returns all Q-values for a state.
func (q *QLearningAlgorithm) GetQValues(state []float32) []float64 {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stateHash := q.hashState(state)
	if qValues, ok := q.qTable[stateHash]; ok {
		result := make([]float64, len(qValues))
		copy(result, qValues)
		return result
	}

	// Return zeros if not in table
	result := make([]float64, q.config.NumActions)
	return result
}

// GetStats returns algorithm statistics.
func (q *QLearningAlgorithm) GetStats() domainNeural.RLStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return domainNeural.RLStats{
		Algorithm:    domainNeural.AlgorithmQLearning,
		UpdateCount:  q.updateCount,
		Epsilon:      q.epsilon,
		AvgLoss:      q.avgTDError,
		QTableSize:   len(q.qTable),
		StepCount:    q.stepCount,
		AvgLatencyMs: q.avgLatency,
		LastUpdate:   time.Now(),
	}
}

// Reset resets the algorithm state.
func (q *QLearningAlgorithm) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.qTable = make(map[uint64][]float64)
	q.stateAccessTime = make(map[uint64]time.Time)
	q.traces = make(map[uint64][]float64)
	q.lastState = nil
	q.lastAction = 0
	q.epsilon = q.config.ExplorationInitial
	q.stepCount = 0
	q.updateCount = 0
	q.avgTDError = 0
}

// StartEpisode resets eligibility traces for a new episode.
func (q *QLearningAlgorithm) StartEpisode() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.traces = make(map[uint64][]float64)
	q.lastState = nil
	q.lastAction = 0
}

// Private methods

func (q *QLearningAlgorithm) hashState(state []float32) uint64 {
	h := fnv.New64a()
	for _, v := range state {
		// Discretize to reduce state space
		discreteVal := int(v * 100)
		h.Write([]byte(fmt.Sprintf("%d,", discreteVal)))
	}
	return h.Sum64()
}

func (q *QLearningAlgorithm) ensureState(stateHash uint64) {
	if _, ok := q.qTable[stateHash]; !ok {
		numActions := q.config.NumActions
		if numActions == 0 {
			numActions = 4
		}
		// Initialize with small random values for exploration
		qValues := make([]float64, numActions)
		for i := range qValues {
			qValues[i] = (q.rng.Float64() - 0.5) * 0.01
		}
		q.qTable[stateHash] = qValues

		if q.config.UseEligibilityTraces {
			q.traces[stateHash] = make([]float64, numActions)
		}
	}
}

func (q *QLearningAlgorithm) maxQ(stateHash uint64) float64 {
	if qValues, ok := q.qTable[stateHash]; ok {
		maxVal := qValues[0]
		for _, v := range qValues[1:] {
			if v > maxVal {
				maxVal = v
			}
		}
		return maxVal
	}
	return 0
}

func (q *QLearningAlgorithm) argmaxQ(stateHash uint64) int {
	if qValues, ok := q.qTable[stateHash]; ok {
		maxIdx := 0
		maxVal := qValues[0]
		for i, v := range qValues[1:] {
			if v > maxVal {
				maxVal = v
				maxIdx = i + 1
			}
		}
		return maxIdx
	}
	return 0
}

func (q *QLearningAlgorithm) updateWithTraces(stateHash uint64, action int, tdError float64) {
	// Set trace for current state-action to 1 (replacing traces)
	if _, ok := q.traces[stateHash]; !ok {
		q.traces[stateHash] = make([]float64, q.config.NumActions)
	}
	q.traces[stateHash][action] = 1.0

	// Update all states in traces
	for s, traces := range q.traces {
		for a, trace := range traces {
			if trace > 0 {
				q.qTable[s][a] += q.config.LearningRate * tdError * trace
				// Decay trace
				q.traces[s][a] *= q.config.Gamma * q.config.TraceDecay
			}
		}
	}
}

func (q *QLearningAlgorithm) pruneOldStates() {
	// Get states sorted by access time
	type stateTime struct {
		hash uint64
		t    time.Time
	}
	states := make([]stateTime, 0, len(q.stateAccessTime))
	for hash, t := range q.stateAccessTime {
		states = append(states, stateTime{hash, t})
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].t.Before(states[j].t)
	})

	// Remove oldest 10%
	toRemove := len(states) / 10
	if toRemove < 1 {
		toRemove = 1
	}
	for i := 0; i < toRemove; i++ {
		delete(q.qTable, states[i].hash)
		delete(q.stateAccessTime, states[i].hash)
		delete(q.traces, states[i].hash)
	}
}
