// Package neural provides neural network infrastructure.
package neural

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// MultiAgentRL implements Multi-Agent Reinforcement Learning.
type MultiAgentRL struct {
	mu     sync.RWMutex
	config domainNeural.MultiAgentConfig
	rng    *rand.Rand

	// Per-agent policies
	agentPolicies map[string]*agentPolicy

	// Shared value function (if shared reward)
	sharedValueWeights []float64

	// Coordination metrics
	coordinationHistory []float64

	// Statistics
	updateCount    int64
	avgLoss        float64
	avgCoordination float64
	avgLatency     float64
}

// agentPolicy represents an individual agent's policy.
type agentPolicy struct {
	id             string
	policyWeights  [][]float64
	policyMomentum [][]float64
	valueWeights   []float64
	valueMomentum  []float64
	buffer         []multiAgentExperience
	updateCount    int64
	avgReward      float64
}

// multiAgentExperience is a multi-agent experience.
type multiAgentExperience struct {
	agentID   string
	state     []float32
	action    int
	reward    float64
	nextState []float32
	done      bool
	jointState  [][]float32 // States of all agents
	jointAction []int       // Actions of all agents
}

// NewMultiAgentRL creates a new Multi-Agent RL system.
func NewMultiAgentRL(config domainNeural.MultiAgentConfig) *MultiAgentRL {
	inputDim := config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	marl := &MultiAgentRL{
		config:              config,
		rng:                 rand.New(rand.NewSource(time.Now().UnixNano())),
		agentPolicies:       make(map[string]*agentPolicy),
		sharedValueWeights:  make([]float64, inputDim),
		coordinationHistory: make([]float64, 0),
	}

	// Initialize shared value weights
	scale := math.Sqrt(2.0 / float64(inputDim))
	for i := range marl.sharedValueWeights {
		marl.sharedValueWeights[i] = (marl.rng.Float64() - 0.5) * scale
	}

	// Create initial agents
	for i := 0; i < config.NumAgents; i++ {
		agentID := fmt.Sprintf("agent_%d", i)
		marl.agentPolicies[agentID] = marl.createAgentPolicy(agentID)
	}

	return marl
}

// RegisterAgent registers a new agent.
func (m *MultiAgentRL) RegisterAgent(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agentPolicies[agentID]; !exists {
		m.agentPolicies[agentID] = m.createAgentPolicy(agentID)
	}
}

// RemoveAgent removes an agent.
func (m *MultiAgentRL) RemoveAgent(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.agentPolicies, agentID)
}

// AddExperience adds experience for a specific agent.
func (m *MultiAgentRL) AddExperience(agentID string, exp domainNeural.RLExperience) {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agentPolicies[agentID]
	if !exists {
		agent = m.createAgentPolicy(agentID)
		m.agentPolicies[agentID] = agent
	}

	maExp := multiAgentExperience{
		agentID:   agentID,
		state:     exp.State,
		action:    exp.Action,
		reward:    exp.Reward,
		nextState: exp.NextState,
		done:      exp.Done,
	}

	agent.buffer = append(agent.buffer, maExp)
}

// AddJointExperience adds experience with joint state/action information.
func (m *MultiAgentRL) AddJointExperience(agentID string, state []float32, action int, reward float64,
	nextState []float32, done bool, jointState [][]float32, jointAction []int) {

	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agentPolicies[agentID]
	if !exists {
		agent = m.createAgentPolicy(agentID)
		m.agentPolicies[agentID] = agent
	}

	exp := multiAgentExperience{
		agentID:     agentID,
		state:       state,
		action:      action,
		reward:      reward,
		nextState:   nextState,
		done:        done,
		jointState:  jointState,
		jointAction: jointAction,
	}

	agent.buffer = append(agent.buffer, exp)
}

// Update performs updates for all agents.
func (m *MultiAgentRL) Update() domainNeural.RLUpdateResult {
	startTime := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	var totalLoss float64
	var agentsUpdated int

	for _, agent := range m.agentPolicies {
		if len(agent.buffer) < m.config.MiniBatchSize {
			continue
		}

		loss := m.updateAgent(agent)
		totalLoss += loss
		agentsUpdated++

		// Clear buffer after update
		agent.buffer = make([]multiAgentExperience, 0)
	}

	// Compute coordination score
	coordination := m.computeCoordinationScore()
	m.coordinationHistory = append(m.coordinationHistory, coordination)
	if len(m.coordinationHistory) > 100 {
		m.coordinationHistory = m.coordinationHistory[1:]
	}

	m.updateCount++
	if agentsUpdated > 0 {
		m.avgLoss = totalLoss / float64(agentsUpdated)
	}
	m.avgCoordination = coordination

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	m.avgLatency = (m.avgLatency*float64(m.updateCount-1) + elapsed) / float64(m.updateCount)

	return domainNeural.RLUpdateResult{
		TotalLoss: m.avgLoss,
		LatencyMs: elapsed,
	}
}

// GetAction returns an action for a specific agent.
func (m *MultiAgentRL) GetAction(agentID string, state []float32, explore bool) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agentPolicies[agentID]
	if !exists {
		return m.rng.Intn(m.config.NumActions)
	}

	logits := m.forwardAgent(agent, state)
	probs := softmax64(logits)

	if explore {
		return m.sampleAction(probs)
	}

	return argmax64(probs)
}

// GetJointAction returns actions for all agents given joint state.
func (m *MultiAgentRL) GetJointAction(jointState [][]float32, explore bool) map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	actions := make(map[string]int)
	i := 0

	for agentID, agent := range m.agentPolicies {
		var state []float32
		if i < len(jointState) {
			state = jointState[i]
		} else {
			state = make([]float32, m.config.InputDim)
		}

		logits := m.forwardAgent(agent, state)
		probs := softmax64(logits)

		if explore {
			actions[agentID] = m.sampleAction(probs)
		} else {
			actions[agentID] = argmax64(probs)
		}
		i++
	}

	return actions
}

// DistributeReward distributes a team reward among agents.
func (m *MultiAgentRL) DistributeReward(teamReward float64) map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rewards := make(map[string]float64)
	numAgents := len(m.agentPolicies)
	if numAgents == 0 {
		return rewards
	}

	if m.config.SharedReward {
		// Equal distribution
		perAgent := teamReward / float64(numAgents)
		for agentID := range m.agentPolicies {
			rewards[agentID] = perAgent
		}
	} else {
		// Performance-based distribution
		var totalPerf float64
		perfs := make(map[string]float64)

		for agentID, agent := range m.agentPolicies {
			perf := math.Max(0.1, agent.avgReward) // Avoid zero
			perfs[agentID] = perf
			totalPerf += perf
		}

		for agentID, perf := range perfs {
			rewards[agentID] = teamReward * (perf / totalPerf)
		}
	}

	// Add coordination bonus
	if m.config.CoordinationBonus > 0 {
		coordination := m.computeCoordinationScore()
		bonus := teamReward * m.config.CoordinationBonus * coordination
		for agentID := range rewards {
			rewards[agentID] += bonus / float64(numAgents)
		}
	}

	return rewards
}

// GetStats returns multi-agent statistics.
func (m *MultiAgentRL) GetStats() domainNeural.RLStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalBuffer int
	for _, agent := range m.agentPolicies {
		totalBuffer += len(agent.buffer)
	}

	return domainNeural.RLStats{
		Algorithm:    domainNeural.AlgorithmMultiAgent,
		UpdateCount:  m.updateCount,
		BufferSize:   totalBuffer,
		AvgLoss:      m.avgLoss,
		AvgLatencyMs: m.avgLatency,
		LastUpdate:   time.Now(),
	}
}

// GetAgentStats returns statistics for a specific agent.
func (m *MultiAgentRL) GetAgentStats(agentID string) map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agentPolicies[agentID]
	if !exists {
		return nil
	}

	return map[string]float64{
		"updateCount": float64(agent.updateCount),
		"bufferSize":  float64(len(agent.buffer)),
		"avgReward":   agent.avgReward,
	}
}

// GetCoordinationScore returns the current coordination score.
func (m *MultiAgentRL) GetCoordinationScore() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.computeCoordinationScore()
}

// GetNumAgents returns the number of active agents.
func (m *MultiAgentRL) GetNumAgents() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.agentPolicies)
}

// Reset resets all agents.
func (m *MultiAgentRL) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, agent := range m.agentPolicies {
		m.resetAgentPolicy(agent)
	}

	m.coordinationHistory = make([]float64, 0)
	m.updateCount = 0
	m.avgLoss = 0
	m.avgCoordination = 0
}

// Private methods

func (m *MultiAgentRL) createAgentPolicy(agentID string) *agentPolicy {
	inputDim := m.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	hiddenDim := m.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := m.config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	agent := &agentPolicy{
		id:            agentID,
		policyWeights: make([][]float64, 2),
		policyMomentum: make([][]float64, 2),
		valueWeights:  make([]float64, hiddenDim),
		valueMomentum: make([]float64, hiddenDim),
		buffer:        make([]multiAgentExperience, 0),
	}

	// Layer 1
	scale1 := math.Sqrt(2.0 / float64(inputDim))
	agent.policyWeights[0] = make([]float64, inputDim*hiddenDim)
	agent.policyMomentum[0] = make([]float64, inputDim*hiddenDim)
	for i := range agent.policyWeights[0] {
		agent.policyWeights[0][i] = (m.rng.Float64() - 0.5) * scale1
	}

	// Layer 2
	scale2 := math.Sqrt(2.0 / float64(hiddenDim))
	agent.policyWeights[1] = make([]float64, hiddenDim*numActions)
	agent.policyMomentum[1] = make([]float64, hiddenDim*numActions)
	for i := range agent.policyWeights[1] {
		agent.policyWeights[1][i] = (m.rng.Float64() - 0.5) * scale2
	}

	// Value head
	for i := range agent.valueWeights {
		agent.valueWeights[i] = (m.rng.Float64() - 0.5) * 0.1
	}

	return agent
}

func (m *MultiAgentRL) resetAgentPolicy(agent *agentPolicy) {
	inputDim := m.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	hiddenDim := m.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}

	agent.buffer = make([]multiAgentExperience, 0)
	agent.updateCount = 0
	agent.avgReward = 0

	scale1 := math.Sqrt(2.0 / float64(inputDim))
	for i := range agent.policyWeights[0] {
		agent.policyWeights[0][i] = (m.rng.Float64() - 0.5) * scale1
		agent.policyMomentum[0][i] = 0
	}

	scale2 := math.Sqrt(2.0 / float64(hiddenDim))
	for i := range agent.policyWeights[1] {
		agent.policyWeights[1][i] = (m.rng.Float64() - 0.5) * scale2
		agent.policyMomentum[1][i] = 0
	}

	for i := range agent.valueWeights {
		agent.valueWeights[i] = (m.rng.Float64() - 0.5) * 0.1
		agent.valueMomentum[i] = 0
	}
}

func (m *MultiAgentRL) forwardAgent(agent *agentPolicy, state []float32) []float64 {
	hiddenDim := m.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := m.config.NumActions
	if numActions == 0 {
		numActions = 4
	}
	inputDim := m.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	// Layer 1: ReLU
	hidden := make([]float64, hiddenDim)
	for h := 0; h < hiddenDim; h++ {
		var sum float64
		for i := 0; i < len(state) && i < inputDim; i++ {
			sum += float64(state[i]) * agent.policyWeights[0][i*hiddenDim+h]
		}
		hidden[h] = math.Max(0, sum)
	}

	// Layer 2: Output logits
	logits := make([]float64, numActions)
	for a := 0; a < numActions; a++ {
		var sum float64
		for h := 0; h < hiddenDim; h++ {
			sum += hidden[h] * agent.policyWeights[1][h*numActions+a]
		}
		logits[a] = sum
	}

	return logits
}

func (m *MultiAgentRL) getValueAgent(agent *agentPolicy, hidden []float64) float64 {
	var value float64
	for h := 0; h < len(hidden) && h < len(agent.valueWeights); h++ {
		value += hidden[h] * agent.valueWeights[h]
	}
	return value
}

func (m *MultiAgentRL) updateAgent(agent *agentPolicy) float64 {
	inputDim := m.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	hiddenDim := m.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := m.config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	// Shuffle buffer
	for i := len(agent.buffer) - 1; i > 0; i-- {
		j := m.rng.Intn(i + 1)
		agent.buffer[i], agent.buffer[j] = agent.buffer[j], agent.buffer[i]
	}

	var totalLoss float64
	var totalReward float64

	// Initialize gradients
	grad0 := make([]float64, len(agent.policyWeights[0]))
	grad1 := make([]float64, len(agent.policyWeights[1]))
	valueGrad := make([]float64, len(agent.valueWeights))

	for _, exp := range agent.buffer {
		// Forward pass
		hidden := make([]float64, hiddenDim)
		for h := 0; h < hiddenDim; h++ {
			var sum float64
			for i := 0; i < len(exp.state) && i < inputDim; i++ {
				sum += float64(exp.state[i]) * agent.policyWeights[0][i*hiddenDim+h]
			}
			hidden[h] = math.Max(0, sum)
		}

		logits := make([]float64, numActions)
		for a := 0; a < numActions; a++ {
			var sum float64
			for h := 0; h < hiddenDim; h++ {
				sum += hidden[h] * agent.policyWeights[1][h*numActions+a]
			}
			logits[a] = sum
		}

		probs := softmax64(logits)
		value := m.getValueAgent(agent, hidden)

		// Compute advantage
		advantage := exp.reward - value
		totalLoss += advantage * advantage
		totalReward += exp.reward

		// Policy gradient
		for a := 0; a < numActions; a++ {
			target := 0.0
			if a == exp.action {
				target = 1.0
			}
			error := (probs[a] - target) * (-advantage)

			for h := 0; h < hiddenDim; h++ {
				grad1[h*numActions+a] += hidden[h] * error
			}
		}

		// Value gradient
		valueError := value - exp.reward
		for h := 0; h < len(hidden) && h < len(valueGrad); h++ {
			valueGrad[h] += hidden[h] * valueError
		}

		// Backward through hidden
		for h := 0; h < hiddenDim; h++ {
			if hidden[h] > 0 {
				var signal float64
				// From policy
				for a := 0; a < numActions; a++ {
					target := 0.0
					if a == exp.action {
						target = 1.0
					}
					signal += (probs[a] - target) * (-advantage) * agent.policyWeights[1][h*numActions+a]
				}
				// From value
				signal += valueError * agent.valueWeights[h]

				for i := 0; i < len(exp.state) && i < inputDim; i++ {
					grad0[i*hiddenDim+h] += float64(exp.state[i]) * signal
				}
			}
		}
	}

	// Apply gradients
	lr := m.config.LearningRate / float64(len(agent.buffer))
	beta := 0.9

	for i := range agent.policyWeights[0] {
		grad := math.Max(math.Min(grad0[i], m.config.MaxGradNorm), -m.config.MaxGradNorm)
		agent.policyMomentum[0][i] = beta*agent.policyMomentum[0][i] + (1-beta)*grad
		agent.policyWeights[0][i] -= lr * agent.policyMomentum[0][i]
	}

	for i := range agent.policyWeights[1] {
		grad := math.Max(math.Min(grad1[i], m.config.MaxGradNorm), -m.config.MaxGradNorm)
		agent.policyMomentum[1][i] = beta*agent.policyMomentum[1][i] + (1-beta)*grad
		agent.policyWeights[1][i] -= lr * agent.policyMomentum[1][i]
	}

	for i := range agent.valueWeights {
		grad := math.Max(math.Min(valueGrad[i], m.config.MaxGradNorm), -m.config.MaxGradNorm)
		agent.valueMomentum[i] = beta*agent.valueMomentum[i] + (1-beta)*grad
		agent.valueWeights[i] -= lr * agent.valueMomentum[i]
	}

	// Update stats
	agent.updateCount++
	agent.avgReward = totalReward / float64(len(agent.buffer))

	return totalLoss / float64(len(agent.buffer))
}

func (m *MultiAgentRL) computeCoordinationScore() float64 {
	if len(m.agentPolicies) < 2 {
		return 1.0
	}

	// Measure policy similarity/diversity balance
	// Good coordination = agents are different enough to cover state space
	// but similar enough to not conflict

	var policies [][]float64
	for _, agent := range m.agentPolicies {
		policy := make([]float64, 0)
		policy = append(policy, agent.policyWeights[1]...)
		policies = append(policies, policy)
	}

	if len(policies) < 2 {
		return 1.0
	}

	// Compute average pairwise cosine similarity
	var totalSim float64
	var pairs int

	for i := 0; i < len(policies); i++ {
		for j := i + 1; j < len(policies); j++ {
			sim := cosineSimilarity64(policies[i], policies[j])
			totalSim += sim
			pairs++
		}
	}

	if pairs == 0 {
		return 1.0
	}

	avgSim := totalSim / float64(pairs)

	// Optimal coordination is moderate similarity (not too same, not too different)
	// Peak at 0.5 similarity
	return 1.0 - math.Abs(avgSim-0.5)*2
}

func (m *MultiAgentRL) sampleAction(probs []float64) int {
	r := m.rng.Float64()
	var cumSum float64
	for i, p := range probs {
		cumSum += p
		if r < cumSum {
			return i
		}
	}
	return len(probs) - 1
}

func cosineSimilarity64(a, b []float64) float64 {
	if len(a) != len(b) {
		minLen := len(a)
		if len(b) < minLen {
			minLen = len(b)
		}
		a = a[:minLen]
		b = b[:minLen]
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
