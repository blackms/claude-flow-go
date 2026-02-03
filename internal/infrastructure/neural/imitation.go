// Package neural provides neural network infrastructure.
package neural

import (
	"math"
	"math/rand"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// ImitationLearner implements Imitation Learning / Behavioral Cloning.
type ImitationLearner struct {
	mu     sync.RWMutex
	config domainNeural.ImitationConfig
	rng    *rand.Rand

	// Policy network
	policyWeights [][]float64
	policyMomentum [][]float64

	// Expert trajectory buffer
	expertBuffer []imitationExperience

	// Agent trajectory buffer (for DAgger)
	agentBuffer []imitationExperience

	// Statistics
	updateCount  int64
	avgLoss      float64
	avgAccuracy  float64
	avgLatency   float64
}

// imitationExperience is an experience for imitation learning.
type imitationExperience struct {
	state      []float32
	action     int
	isExpert   bool
	confidence float64
}

// NewImitationLearner creates a new Imitation Learner.
func NewImitationLearner(config domainNeural.ImitationConfig) *ImitationLearner {
	inputDim := config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	hiddenDim := config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	il := &ImitationLearner{
		config:       config,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
		expertBuffer: make([]imitationExperience, 0),
		agentBuffer:  make([]imitationExperience, 0),
	}

	// Initialize policy network (2 layers)
	il.policyWeights = make([][]float64, 2)
	il.policyMomentum = make([][]float64, 2)

	// Layer 1: inputDim -> hiddenDim
	scale1 := math.Sqrt(2.0 / float64(inputDim))
	il.policyWeights[0] = make([]float64, inputDim*hiddenDim)
	il.policyMomentum[0] = make([]float64, inputDim*hiddenDim)
	for i := range il.policyWeights[0] {
		il.policyWeights[0][i] = (il.rng.Float64() - 0.5) * scale1
	}

	// Layer 2: hiddenDim -> numActions
	scale2 := math.Sqrt(2.0 / float64(hiddenDim))
	il.policyWeights[1] = make([]float64, hiddenDim*numActions)
	il.policyMomentum[1] = make([]float64, hiddenDim*numActions)
	for i := range il.policyWeights[1] {
		il.policyWeights[1][i] = (il.rng.Float64() - 0.5) * scale2
	}

	return il
}

// AddExpertTrajectory adds expert demonstrations.
func (il *ImitationLearner) AddExpertTrajectory(states [][]float32, actions []int) {
	il.mu.Lock()
	defer il.mu.Unlock()

	for i := range states {
		if i >= len(actions) {
			break
		}

		exp := imitationExperience{
			state:      states[i],
			action:     actions[i],
			isExpert:   true,
			confidence: 1.0,
		}

		// Check buffer limit
		if len(il.expertBuffer) >= il.config.MaxExpertTrajectories {
			// Remove oldest
			il.expertBuffer = il.expertBuffer[1:]
		}
		il.expertBuffer = append(il.expertBuffer, exp)
	}
}

// AddExpertExperience adds a single expert experience.
func (il *ImitationLearner) AddExpertExperience(exp domainNeural.RLExperience) {
	il.mu.Lock()
	defer il.mu.Unlock()

	imExp := imitationExperience{
		state:      exp.State,
		action:     exp.Action,
		isExpert:   true,
		confidence: 1.0,
	}

	if len(il.expertBuffer) >= il.config.MaxExpertTrajectories {
		il.expertBuffer = il.expertBuffer[1:]
	}
	il.expertBuffer = append(il.expertBuffer, imExp)
}

// AddAgentExperience adds agent experience with expert label (for DAgger).
func (il *ImitationLearner) AddAgentExperience(state []float32, agentAction, expertAction int) {
	il.mu.Lock()
	defer il.mu.Unlock()

	exp := imitationExperience{
		state:      state,
		action:     expertAction, // Use expert label
		isExpert:   false,
		confidence: il.config.ExpertWeight,
	}

	il.agentBuffer = append(il.agentBuffer, exp)
}

// AddExperience adds experience (for interface compliance).
func (il *ImitationLearner) AddExperience(exp domainNeural.RLExperience) {
	il.AddExpertExperience(exp)
}

// Update performs a behavioral cloning update.
func (il *ImitationLearner) Update() domainNeural.RLUpdateResult {
	startTime := time.Now()

	il.mu.Lock()
	defer il.mu.Unlock()

	// Combine expert and agent buffers
	var trainData []imitationExperience
	if il.config.DAgger && len(il.agentBuffer) > 0 {
		// DAgger: mix expert and agent data
		trainData = append(trainData, il.expertBuffer...)
		trainData = append(trainData, il.agentBuffer...)
	} else {
		// Pure behavioral cloning
		trainData = il.expertBuffer
	}

	if len(trainData) < il.config.MiniBatchSize {
		return domainNeural.RLUpdateResult{}
	}

	var totalLoss float64
	var correctCount int
	var totalCount int

	// Multiple epochs
	for epoch := 0; epoch < il.config.Epochs; epoch++ {
		// Shuffle data
		il.shuffle(trainData)

		// Process mini-batches
		for i := 0; i < len(trainData); i += il.config.MiniBatchSize {
			end := i + il.config.MiniBatchSize
			if end > len(trainData) {
				end = len(trainData)
			}
			batch := trainData[i:end]

			loss, correct := il.updateBatch(batch)
			totalLoss += loss
			correctCount += correct
			totalCount += len(batch)
		}
	}

	il.updateCount++
	il.avgLoss = totalLoss / float64(max(totalCount, 1))
	il.avgAccuracy = float64(correctCount) / float64(max(totalCount, 1))

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	il.avgLatency = (il.avgLatency*float64(il.updateCount-1) + elapsed) / float64(il.updateCount)

	return domainNeural.RLUpdateResult{
		TotalLoss: il.avgLoss,
		Accuracy:  il.avgAccuracy,
		LatencyMs: elapsed,
	}
}

// GetAction returns an action from the learned policy.
func (il *ImitationLearner) GetAction(state []float32, explore bool) int {
	il.mu.RLock()
	defer il.mu.RUnlock()

	logits := il.forward(state)
	probs := softmax64(logits)

	if explore {
		return il.sampleAction(probs)
	}

	return argmax64(probs)
}

// GetStats returns learner statistics.
func (il *ImitationLearner) GetStats() domainNeural.RLStats {
	il.mu.RLock()
	defer il.mu.RUnlock()

	return domainNeural.RLStats{
		Algorithm:    domainNeural.AlgorithmImitation,
		UpdateCount:  il.updateCount,
		BufferSize:   len(il.expertBuffer) + len(il.agentBuffer),
		AvgLoss:      il.avgLoss,
		AvgLatencyMs: il.avgLatency,
		LastUpdate:   time.Now(),
	}
}

// Reset resets the learner state.
func (il *ImitationLearner) Reset() {
	il.mu.Lock()
	defer il.mu.Unlock()

	il.expertBuffer = make([]imitationExperience, 0)
	il.agentBuffer = make([]imitationExperience, 0)
	il.updateCount = 0
	il.avgLoss = 0
	il.avgAccuracy = 0

	// Reinitialize weights
	inputDim := il.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	hiddenDim := il.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}

	scale1 := math.Sqrt(2.0 / float64(inputDim))
	for i := range il.policyWeights[0] {
		il.policyWeights[0][i] = (il.rng.Float64() - 0.5) * scale1
		il.policyMomentum[0][i] = 0
	}

	scale2 := math.Sqrt(2.0 / float64(hiddenDim))
	for i := range il.policyWeights[1] {
		il.policyWeights[1][i] = (il.rng.Float64() - 0.5) * scale2
		il.policyMomentum[1][i] = 0
	}
}

// ClearAgentBuffer clears the agent buffer.
func (il *ImitationLearner) ClearAgentBuffer() {
	il.mu.Lock()
	defer il.mu.Unlock()
	il.agentBuffer = make([]imitationExperience, 0)
}

// GetExpertBufferSize returns the expert buffer size.
func (il *ImitationLearner) GetExpertBufferSize() int {
	il.mu.RLock()
	defer il.mu.RUnlock()
	return len(il.expertBuffer)
}

// Private methods

func (il *ImitationLearner) forward(state []float32) []float64 {
	inputDim := il.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	hiddenDim := il.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := il.config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	// Layer 1: ReLU
	hidden := make([]float64, hiddenDim)
	for h := 0; h < hiddenDim; h++ {
		var sum float64
		for i := 0; i < len(state) && i < inputDim; i++ {
			sum += float64(state[i]) * il.policyWeights[0][i*hiddenDim+h]
		}
		hidden[h] = math.Max(0, sum)
	}

	// Layer 2: Output logits
	logits := make([]float64, numActions)
	for a := 0; a < numActions; a++ {
		var sum float64
		for h := 0; h < hiddenDim; h++ {
			sum += hidden[h] * il.policyWeights[1][h*numActions+a]
		}
		logits[a] = sum
	}

	return logits
}

func (il *ImitationLearner) updateBatch(batch []imitationExperience) (loss float64, correct int) {
	inputDim := il.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	hiddenDim := il.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := il.config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	// Initialize gradients
	grad0 := make([]float64, len(il.policyWeights[0]))
	grad1 := make([]float64, len(il.policyWeights[1]))

	for _, exp := range batch {
		// Forward pass
		hidden := make([]float64, hiddenDim)
		for h := 0; h < hiddenDim; h++ {
			var sum float64
			for i := 0; i < len(exp.state) && i < inputDim; i++ {
				sum += float64(exp.state[i]) * il.policyWeights[0][i*hiddenDim+h]
			}
			hidden[h] = math.Max(0, sum)
		}

		logits := make([]float64, numActions)
		for a := 0; a < numActions; a++ {
			var sum float64
			for h := 0; h < hiddenDim; h++ {
				sum += hidden[h] * il.policyWeights[1][h*numActions+a]
			}
			logits[a] = sum
		}

		probs := softmax64(logits)

		// Cross-entropy loss
		if exp.action < len(probs) {
			loss -= math.Log(probs[exp.action]+1e-8) * exp.confidence
		}

		// Accuracy
		if argmax64(probs) == exp.action {
			correct++
		}

		// Backward pass
		for a := 0; a < numActions; a++ {
			target := 0.0
			if a == exp.action {
				target = 1.0
			}
			error := (probs[a] - target) * exp.confidence

			// Layer 2 gradient
			for h := 0; h < hiddenDim; h++ {
				grad1[h*numActions+a] += hidden[h] * error
			}

			// Layer 1 gradient (through ReLU)
			for h := 0; h < hiddenDim; h++ {
				if hidden[h] > 0 {
					signal := error * il.policyWeights[1][h*numActions+a]
					for i := 0; i < len(exp.state) && i < inputDim; i++ {
						grad0[i*hiddenDim+h] += float64(exp.state[i]) * signal
					}
				}
			}
		}
	}

	// Apply gradients
	lr := il.config.LearningRate / float64(len(batch))
	beta := 0.9

	for i := range il.policyWeights[0] {
		grad := math.Max(math.Min(grad0[i], il.config.MaxGradNorm), -il.config.MaxGradNorm)
		il.policyMomentum[0][i] = beta*il.policyMomentum[0][i] + (1-beta)*grad
		il.policyWeights[0][i] -= lr * il.policyMomentum[0][i]
	}

	for i := range il.policyWeights[1] {
		grad := math.Max(math.Min(grad1[i], il.config.MaxGradNorm), -il.config.MaxGradNorm)
		il.policyMomentum[1][i] = beta*il.policyMomentum[1][i] + (1-beta)*grad
		il.policyWeights[1][i] -= lr * il.policyMomentum[1][i]
	}

	return loss, correct
}

func (il *ImitationLearner) shuffle(data []imitationExperience) {
	for i := len(data) - 1; i > 0; i-- {
		j := il.rng.Intn(i + 1)
		data[i], data[j] = data[j], data[i]
	}
}

func (il *ImitationLearner) sampleAction(probs []float64) int {
	r := il.rng.Float64()
	var cumSum float64
	for i, p := range probs {
		cumSum += p
		if r < cumSum {
			return i
		}
	}
	return len(probs) - 1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
