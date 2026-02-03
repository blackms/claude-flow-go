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

// SARSAAlgorithm implements SARSA (State-Action-Reward-State-Action).
type SARSAAlgorithm struct {
	mu     sync.RWMutex
	config domainNeural.SARSAConfig
	rng    *rand.Rand

	// Q-table: stateHash -> action -> value
	qTable map[uint64][]float64

	// LRU tracking for pruning
	stateAccessTime map[uint64]time.Time

	// Eligibility traces
	traces map[uint64][]float64

	// Previous step for SARSA update
	prevState     []float32
	prevAction    int
	prevReward    float64
	hasPrevious   bool

	// Exploration
	epsilon   float64
	stepCount int64

	// Statistics
	updateCount  int64
	avgTDError   float64
	avgLatency   float64
}

// NewSARSAAlgorithm creates a new SARSA algorithm.
func NewSARSAAlgorithm(config domainNeural.SARSAConfig) *SARSAAlgorithm {
	return &SARSAAlgorithm{
		config:          config,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
		qTable:          make(map[uint64][]float64),
		stateAccessTime: make(map[uint64]time.Time),
		traces:          make(map[uint64][]float64),
		epsilon:         config.ExplorationInitial,
	}
}

// AddExperience adds experience (redirects to step-based update).
func (s *SARSAAlgorithm) AddExperience(exp domainNeural.RLExperience) {
	s.Step(exp.State, exp.Action, exp.Reward, exp.Done)
}

// Step processes a single SARSA step.
// Target: <1ms
func (s *SARSAAlgorithm) Step(state []float32, action int, reward float64, done bool) domainNeural.RLUpdateResult {
	startTime := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	stateHash := s.hashState(state)
	s.ensureState(stateHash)
	s.stateAccessTime[stateHash] = time.Now()

	var tdError float64

	// SARSA update from previous step
	if s.hasPrevious {
		prevStateHash := s.hashState(s.prevState)
		s.ensureState(prevStateHash)

		// Get Q(s, a)
		currentQ := s.qTable[prevStateHash][s.prevAction]

		// Compute target based on variant
		var targetQ float64
		if done {
			targetQ = s.prevReward
		} else if s.config.UseExpectedSARSA {
			// Expected SARSA: use expected value over policy
			targetQ = s.prevReward + s.config.Gamma*s.expectedQ(stateHash)
		} else {
			// Standard SARSA: use next action
			targetQ = s.prevReward + s.config.Gamma*s.qTable[stateHash][action]
		}

		// TD error
		tdError = targetQ - currentQ

		// Update Q-value
		if s.config.UseEligibilityTraces {
			s.updateWithTraces(prevStateHash, s.prevAction, tdError)
		} else {
			s.qTable[prevStateHash][s.prevAction] += s.config.LearningRate * tdError
		}

		s.updateCount++
		s.avgTDError = (s.avgTDError*float64(s.updateCount-1) + math.Abs(tdError)) / float64(s.updateCount)
	}

	// Store current step for next SARSA update
	if done {
		s.hasPrevious = false
		// Reset traces on episode end
		if s.config.UseEligibilityTraces {
			s.traces = make(map[uint64][]float64)
		}
	} else {
		s.prevState = state
		s.prevAction = action
		s.prevReward = reward
		s.hasPrevious = true
	}

	// Prune if needed
	if len(s.qTable) > s.config.MaxStates {
		s.pruneOldStates()
	}

	// Decay exploration
	s.stepCount++
	s.epsilon = math.Max(
		s.config.ExplorationFinal,
		s.config.ExplorationInitial-float64(s.stepCount)/float64(s.config.ExplorationDecay),
	)

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	s.avgLatency = (s.avgLatency*float64(s.updateCount) + elapsed) / float64(s.updateCount+1)

	return domainNeural.RLUpdateResult{
		TDError:   tdError,
		Epsilon:   s.epsilon,
		LatencyMs: elapsed,
	}
}

// Update returns current stats (for interface compliance).
func (s *SARSAAlgorithm) Update() domainNeural.RLUpdateResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return domainNeural.RLUpdateResult{
		TDError:   s.avgTDError,
		Epsilon:   s.epsilon,
		LatencyMs: s.avgLatency,
	}
}

// GetAction returns an action using epsilon-greedy.
func (s *SARSAAlgorithm) GetAction(state []float32, explore bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	stateHash := s.hashState(state)
	s.ensureState(stateHash)
	s.stateAccessTime[stateHash] = time.Now()

	if explore && s.rng.Float64() < s.epsilon {
		return s.rng.Intn(s.config.NumActions)
	}

	return s.argmaxQ(stateHash)
}

// GetQValue returns the Q-value for a state-action pair.
func (s *SARSAAlgorithm) GetQValue(state []float32, action int) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stateHash := s.hashState(state)
	if qValues, ok := s.qTable[stateHash]; ok && action < len(qValues) {
		return qValues[action]
	}
	return 0
}

// GetQValues returns all Q-values for a state.
func (s *SARSAAlgorithm) GetQValues(state []float32) []float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stateHash := s.hashState(state)
	if qValues, ok := s.qTable[stateHash]; ok {
		result := make([]float64, len(qValues))
		copy(result, qValues)
		return result
	}

	result := make([]float64, s.config.NumActions)
	return result
}

// GetStats returns algorithm statistics.
func (s *SARSAAlgorithm) GetStats() domainNeural.RLStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return domainNeural.RLStats{
		Algorithm:    domainNeural.AlgorithmSARSA,
		UpdateCount:  s.updateCount,
		Epsilon:      s.epsilon,
		AvgLoss:      s.avgTDError,
		QTableSize:   len(s.qTable),
		StepCount:    s.stepCount,
		AvgLatencyMs: s.avgLatency,
		LastUpdate:   time.Now(),
	}
}

// Reset resets the algorithm state.
func (s *SARSAAlgorithm) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.qTable = make(map[uint64][]float64)
	s.stateAccessTime = make(map[uint64]time.Time)
	s.traces = make(map[uint64][]float64)
	s.prevState = nil
	s.prevAction = 0
	s.prevReward = 0
	s.hasPrevious = false
	s.epsilon = s.config.ExplorationInitial
	s.stepCount = 0
	s.updateCount = 0
	s.avgTDError = 0
}

// StartEpisode resets eligibility traces for a new episode.
func (s *SARSAAlgorithm) StartEpisode() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.traces = make(map[uint64][]float64)
	s.prevState = nil
	s.prevAction = 0
	s.prevReward = 0
	s.hasPrevious = false
}

// Private methods

func (s *SARSAAlgorithm) hashState(state []float32) uint64 {
	h := fnv.New64a()
	for _, v := range state {
		discreteVal := int(v * 100)
		h.Write([]byte(fmt.Sprintf("%d,", discreteVal)))
	}
	return h.Sum64()
}

func (s *SARSAAlgorithm) ensureState(stateHash uint64) {
	if _, ok := s.qTable[stateHash]; !ok {
		numActions := s.config.NumActions
		if numActions == 0 {
			numActions = 4
		}
		qValues := make([]float64, numActions)
		for i := range qValues {
			qValues[i] = (s.rng.Float64() - 0.5) * 0.01
		}
		s.qTable[stateHash] = qValues

		if s.config.UseEligibilityTraces {
			s.traces[stateHash] = make([]float64, numActions)
		}
	}
}

func (s *SARSAAlgorithm) expectedQ(stateHash uint64) float64 {
	qValues := s.qTable[stateHash]
	if qValues == nil {
		return 0
	}

	// Under epsilon-greedy policy
	numActions := len(qValues)
	bestAction := s.argmaxQ(stateHash)
	var expected float64

	for a, q := range qValues {
		var prob float64
		if a == bestAction {
			prob = 1.0 - s.epsilon + s.epsilon/float64(numActions)
		} else {
			prob = s.epsilon / float64(numActions)
		}
		expected += prob * q
	}

	return expected
}

func (s *SARSAAlgorithm) argmaxQ(stateHash uint64) int {
	if qValues, ok := s.qTable[stateHash]; ok {
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

func (s *SARSAAlgorithm) updateWithTraces(stateHash uint64, action int, tdError float64) {
	// Set trace for current state-action to 1 (replacing traces)
	if _, ok := s.traces[stateHash]; !ok {
		s.traces[stateHash] = make([]float64, s.config.NumActions)
	}
	s.traces[stateHash][action] = 1.0

	// Update all states in traces
	for st, traces := range s.traces {
		for a, trace := range traces {
			if trace > 0 {
				s.qTable[st][a] += s.config.LearningRate * tdError * trace
				// Decay trace
				s.traces[st][a] *= s.config.Gamma * s.config.TraceDecay
			}
		}
	}
}

func (s *SARSAAlgorithm) pruneOldStates() {
	type stateTime struct {
		hash uint64
		t    time.Time
	}
	states := make([]stateTime, 0, len(s.stateAccessTime))
	for hash, t := range s.stateAccessTime {
		states = append(states, stateTime{hash, t})
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].t.Before(states[j].t)
	})

	toRemove := len(states) / 10
	if toRemove < 1 {
		toRemove = 1
	}
	for i := 0; i < toRemove; i++ {
		delete(s.qTable, states[i].hash)
		delete(s.stateAccessTime, states[i].hash)
		delete(s.traces, states[i].hash)
	}
}
