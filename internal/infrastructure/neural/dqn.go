// Package neural provides neural network infrastructure.
package neural

import (
	"math"
	"math/rand"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// DQNAlgorithm implements Deep Q-Network.
type DQNAlgorithm struct {
	mu     sync.RWMutex
	config domainNeural.DQNConfig
	rng    *rand.Rand

	// Q-network weights (2 layers)
	qWeights     [][]float64
	targetWeights [][]float64

	// Optimizer state
	qMomentum [][]float64

	// Replay buffer (circular)
	buffer    []dqnExperience
	bufferIdx int

	// Exploration
	epsilon   float64
	stepCount int64

	// Statistics
	updateCount int64
	avgLoss     float64
	avgLatency  float64
}

// dqnExperience is a DQN-specific experience.
type dqnExperience struct {
	state     []float32
	action    int
	reward    float64
	nextState []float32
	done      bool
}

// NewDQNAlgorithm creates a new DQN algorithm.
func NewDQNAlgorithm(config domainNeural.DQNConfig) *DQNAlgorithm {
	dqn := &DQNAlgorithm{
		config:  config,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
		buffer:  make([]dqnExperience, 0, config.BufferSize),
		epsilon: config.ExplorationInitial,
	}

	// Initialize networks
	dqn.qWeights = dqn.initializeNetwork()
	dqn.targetWeights = dqn.copyNetwork(dqn.qWeights)
	dqn.qMomentum = make([][]float64, len(dqn.qWeights))
	for i := range dqn.qWeights {
		dqn.qMomentum[i] = make([]float64, len(dqn.qWeights[i]))
	}

	return dqn
}

// AddExperience adds experience to the replay buffer.
func (d *DQNAlgorithm) AddExperience(exp domainNeural.RLExperience) {
	d.mu.Lock()
	defer d.mu.Unlock()

	dqnExp := dqnExperience{
		state:     exp.State,
		action:    exp.Action,
		reward:    exp.Reward,
		nextState: exp.NextState,
		done:      exp.Done,
	}

	// Add to circular buffer
	if len(d.buffer) < d.config.BufferSize {
		d.buffer = append(d.buffer, dqnExp)
	} else {
		d.buffer[d.bufferIdx] = dqnExp
	}
	d.bufferIdx = (d.bufferIdx + 1) % d.config.BufferSize
}

// Update performs a DQN update.
// Target: <10ms
func (d *DQNAlgorithm) Update() domainNeural.RLUpdateResult {
	startTime := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.buffer) < d.config.MiniBatchSize {
		return domainNeural.RLUpdateResult{Epsilon: d.epsilon}
	}

	// Sample mini-batch
	batch := d.sampleBatch()

	// Compute TD targets and update
	var totalLoss float64
	gradients := make([][]float64, len(d.qWeights))
	for i := range gradients {
		gradients[i] = make([]float64, len(d.qWeights[i]))
	}

	for _, exp := range batch {
		// Current Q-values
		qValues := d.forward(exp.state, d.qWeights)
		currentQ := qValues[exp.action]

		// Target Q-value
		var targetQ float64
		if exp.done {
			targetQ = exp.reward
		} else {
			if d.config.DoubleDQN {
				// Double DQN: use online to select, target to evaluate
				nextQOnline := d.forward(exp.nextState, d.qWeights)
				bestAction := argmax64(nextQOnline)
				nextQTarget := d.forward(exp.nextState, d.targetWeights)
				targetQ = exp.reward + d.config.Gamma*nextQTarget[bestAction]
			} else {
				// Standard DQN
				nextQ := d.forward(exp.nextState, d.targetWeights)
				targetQ = exp.reward + d.config.Gamma*max64(nextQ)
			}
		}

		// TD error
		tdError := targetQ - currentQ
		loss := tdError * tdError
		totalLoss += loss

		// Accumulate gradients
		d.accumulateGradients(gradients, exp.state, exp.action, tdError)
	}

	// Apply gradients
	d.applyGradients(gradients, len(batch))

	// Update target network periodically
	d.stepCount++
	if d.stepCount%int64(d.config.TargetUpdateFreq) == 0 {
		d.targetWeights = d.copyNetwork(d.qWeights)
	}

	// Decay exploration
	d.epsilon = math.Max(
		d.config.ExplorationFinal,
		d.config.ExplorationInitial-float64(d.stepCount)/float64(d.config.ExplorationDecay),
	)

	d.updateCount++
	d.avgLoss = totalLoss / float64(len(batch))

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	d.avgLatency = (d.avgLatency*float64(d.updateCount-1) + elapsed) / float64(d.updateCount)

	return domainNeural.RLUpdateResult{
		TotalLoss: d.avgLoss,
		Epsilon:   d.epsilon,
		LatencyMs: elapsed,
	}
}

// GetAction returns an action using epsilon-greedy.
func (d *DQNAlgorithm) GetAction(state []float32, explore bool) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if explore && d.rng.Float64() < d.epsilon {
		return d.rng.Intn(d.config.NumActions)
	}

	qValues := d.forward(state, d.qWeights)
	return argmax64(qValues)
}

// GetQValues returns Q-values for a state.
func (d *DQNAlgorithm) GetQValues(state []float32) []float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.forward(state, d.qWeights)
}

// GetStats returns algorithm statistics.
func (d *DQNAlgorithm) GetStats() domainNeural.RLStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return domainNeural.RLStats{
		Algorithm:    domainNeural.AlgorithmDQN,
		UpdateCount:  d.updateCount,
		BufferSize:   len(d.buffer),
		Epsilon:      d.epsilon,
		AvgLoss:      d.avgLoss,
		AvgLatencyMs: d.avgLatency,
		StepCount:    d.stepCount,
		LastUpdate:   time.Now(),
	}
}

// Reset resets the algorithm state.
func (d *DQNAlgorithm) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.buffer = make([]dqnExperience, 0, d.config.BufferSize)
	d.bufferIdx = 0
	d.epsilon = d.config.ExplorationInitial
	d.stepCount = 0
	d.updateCount = 0
	d.avgLoss = 0

	// Reinitialize networks
	d.qWeights = d.initializeNetwork()
	d.targetWeights = d.copyNetwork(d.qWeights)
	for i := range d.qMomentum {
		for j := range d.qMomentum[i] {
			d.qMomentum[i][j] = 0
		}
	}
}

// Private methods

func (d *DQNAlgorithm) initializeNetwork() [][]float64 {
	inputDim := d.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	hiddenDim := d.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := d.config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	weights := make([][]float64, 2)

	// Layer 1: inputDim -> hiddenDim
	w1 := make([]float64, inputDim*hiddenDim)
	scale1 := math.Sqrt(2.0 / float64(inputDim))
	for i := range w1 {
		w1[i] = (d.rng.Float64() - 0.5) * scale1
	}
	weights[0] = w1

	// Layer 2: hiddenDim -> numActions
	w2 := make([]float64, hiddenDim*numActions)
	scale2 := math.Sqrt(2.0 / float64(hiddenDim))
	for i := range w2 {
		w2[i] = (d.rng.Float64() - 0.5) * scale2
	}
	weights[1] = w2

	return weights
}

func (d *DQNAlgorithm) copyNetwork(weights [][]float64) [][]float64 {
	copied := make([][]float64, len(weights))
	for i := range weights {
		copied[i] = make([]float64, len(weights[i]))
		copy(copied[i], weights[i])
	}
	return copied
}

func (d *DQNAlgorithm) forward(state []float32, weights [][]float64) []float64 {
	hiddenDim := d.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := d.config.NumActions
	if numActions == 0 {
		numActions = 4
	}
	inputDim := d.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	// Layer 1: ReLU(W1 * x)
	hidden := make([]float64, hiddenDim)
	for h := 0; h < hiddenDim; h++ {
		var sum float64
		for i := 0; i < len(state) && i < inputDim; i++ {
			sum += float64(state[i]) * weights[0][i*hiddenDim+h]
		}
		hidden[h] = math.Max(0, sum) // ReLU
	}

	// Layer 2: W2 * hidden
	output := make([]float64, numActions)
	for a := 0; a < numActions; a++ {
		var sum float64
		for h := 0; h < hiddenDim; h++ {
			sum += hidden[h] * weights[1][h*numActions+a]
		}
		output[a] = sum
	}

	return output
}

func (d *DQNAlgorithm) accumulateGradients(gradients [][]float64, state []float32, action int, tdError float64) {
	hiddenDim := d.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := d.config.NumActions
	if numActions == 0 {
		numActions = 4
	}
	inputDim := d.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	// Forward pass to get hidden activations
	hidden := make([]float64, hiddenDim)
	for h := 0; h < hiddenDim; h++ {
		var sum float64
		for i := 0; i < len(state) && i < inputDim; i++ {
			sum += float64(state[i]) * d.qWeights[0][i*hiddenDim+h]
		}
		hidden[h] = math.Max(0, sum)
	}

	// Gradient for layer 2 (only for selected action)
	for h := 0; h < hiddenDim; h++ {
		gradients[1][h*numActions+action] += hidden[h] * tdError
	}

	// Gradient for layer 1 (backprop through ReLU)
	for h := 0; h < hiddenDim; h++ {
		if hidden[h] > 0 { // ReLU gradient
			grad := tdError * d.qWeights[1][h*numActions+action]
			for i := 0; i < len(state) && i < inputDim; i++ {
				gradients[0][i*hiddenDim+h] += float64(state[i]) * grad
			}
		}
	}
}

func (d *DQNAlgorithm) applyGradients(gradients [][]float64, batchSize int) {
	lr := d.config.LearningRate / float64(batchSize)
	beta := 0.9

	for layer := 0; layer < len(gradients); layer++ {
		for i := 0; i < len(gradients[layer]); i++ {
			// Gradient clipping
			grad := math.Max(math.Min(gradients[layer][i], d.config.MaxGradNorm), -d.config.MaxGradNorm)

			// Momentum update
			d.qMomentum[layer][i] = beta*d.qMomentum[layer][i] + (1-beta)*grad
			d.qWeights[layer][i] += lr * d.qMomentum[layer][i]
		}
	}
}

func (d *DQNAlgorithm) sampleBatch() []dqnExperience {
	batch := make([]dqnExperience, 0, d.config.MiniBatchSize)
	indices := make(map[int]bool)

	for len(batch) < d.config.MiniBatchSize && len(batch) < len(d.buffer) {
		idx := d.rng.Intn(len(d.buffer))
		if !indices[idx] {
			indices[idx] = true
			batch = append(batch, d.buffer[idx])
		}
	}

	return batch
}

func max64(values []float64) float64 {
	maxVal := values[0]
	for _, v := range values[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}
