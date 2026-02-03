// Package neural provides neural network infrastructure.
package neural

import (
	"math"
	"math/rand"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// A2CAlgorithm implements Advantage Actor-Critic.
type A2CAlgorithm struct {
	mu     sync.RWMutex
	config domainNeural.A2CConfig
	rng    *rand.Rand

	// Shared network weights
	sharedWeights []float64
	policyHead    []float64
	valueHead     []float64

	// Optimizer state
	sharedMomentum []float64
	policyMomentum []float64
	valueMomentum  []float64

	// Experience buffer for n-step
	buffer []a2cExperience

	// Statistics
	updateCount    int64
	avgPolicyLoss  float64
	avgValueLoss   float64
	avgEntropy     float64
	avgLatency     float64
}

// a2cExperience is an A2C-specific experience.
type a2cExperience struct {
	state   []float32
	action  int
	reward  float64
	value   float64
	logProb float64
	entropy float64
}

// NewA2CAlgorithm creates a new A2C algorithm.
func NewA2CAlgorithm(config domainNeural.A2CConfig) *A2CAlgorithm {
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

	a2c := &A2CAlgorithm{
		config:         config,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
		sharedWeights:  make([]float64, inputDim*hiddenDim),
		policyHead:     make([]float64, hiddenDim*numActions),
		valueHead:      make([]float64, hiddenDim),
		sharedMomentum: make([]float64, inputDim*hiddenDim),
		policyMomentum: make([]float64, hiddenDim*numActions),
		valueMomentum:  make([]float64, hiddenDim),
		buffer:         make([]a2cExperience, 0),
	}

	// Initialize weights
	scale := math.Sqrt(2.0 / float64(inputDim))
	for i := range a2c.sharedWeights {
		a2c.sharedWeights[i] = (a2c.rng.Float64() - 0.5) * scale
	}
	for i := range a2c.policyHead {
		a2c.policyHead[i] = (a2c.rng.Float64() - 0.5) * 0.1
	}
	for i := range a2c.valueHead {
		a2c.valueHead[i] = (a2c.rng.Float64() - 0.5) * 0.1
	}

	return a2c
}

// AddExperience adds experience to the buffer.
func (a *A2CAlgorithm) AddExperience(exp domainNeural.RLExperience) {
	a.mu.Lock()
	defer a.mu.Unlock()

	probs, value, entropy := a.evaluate(exp.State)
	action := exp.Action
	if action >= len(probs) {
		action = 0
	}

	a.buffer = append(a.buffer, a2cExperience{
		state:   exp.State,
		action:  action,
		reward:  exp.Reward,
		value:   value,
		logProb: math.Log(probs[action] + 1e-8),
		entropy: entropy,
	})
}

// Update performs an A2C update.
// Target: <10ms
func (a *A2CAlgorithm) Update() domainNeural.RLUpdateResult {
	startTime := time.Now()

	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.buffer) < a.config.NSteps {
		return domainNeural.RLUpdateResult{}
	}

	// Compute returns and advantages
	returns := a.computeReturns()
	advantages := a.computeAdvantages(returns)

	// Initialize gradients
	hiddenDim := a.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := a.config.NumActions
	if numActions == 0 {
		numActions = 4
	}
	inputDim := a.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	sharedGrad := make([]float64, inputDim*hiddenDim)
	policyGrad := make([]float64, hiddenDim*numActions)
	valueGrad := make([]float64, hiddenDim)

	var totalPolicyLoss, totalValueLoss, totalEntropy float64

	// Process all experiences
	for i, exp := range a.buffer {
		advantage := advantages[i]
		return_ := returns[i]

		// Get current policy and value
		probs, value, hidden := a.forwardWithHidden(exp.state)
		logProb := math.Log(probs[exp.action] + 1e-8)

		// Policy loss
		policyLoss := -logProb * advantage
		totalPolicyLoss += policyLoss

		// Value loss
		valueLoss := (value - return_) * (value - return_)
		totalValueLoss += valueLoss

		// Entropy
		var entropy float64
		for _, p := range probs {
			if p > 0 {
				entropy -= p * math.Log(p)
			}
		}
		totalEntropy += entropy

		// Accumulate gradients
		a.accumulateGradients(sharedGrad, policyGrad, valueGrad, exp.state, hidden, exp.action, advantage, value-return_)
	}

	// Add entropy bonus to policy gradient
	for i := range policyGrad {
		policyGrad[i] -= a.config.EntropyCoef * totalEntropy / float64(len(a.buffer))
	}

	// Apply gradients
	a.applyGradients(sharedGrad, policyGrad, valueGrad, len(a.buffer))

	n := float64(len(a.buffer))

	// Clear buffer
	a.buffer = make([]a2cExperience, 0)
	a.updateCount++

	a.avgPolicyLoss = totalPolicyLoss / n
	a.avgValueLoss = totalValueLoss / n
	a.avgEntropy = totalEntropy / n

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	a.avgLatency = (a.avgLatency*float64(a.updateCount-1) + elapsed) / float64(a.updateCount)

	return domainNeural.RLUpdateResult{
		PolicyLoss: a.avgPolicyLoss,
		ValueLoss:  a.avgValueLoss,
		Entropy:    a.avgEntropy,
		TotalLoss:  a.avgPolicyLoss + a.config.ValueLossCoef*a.avgValueLoss,
		LatencyMs:  elapsed,
	}
}

// GetAction returns an action from the policy.
func (a *A2CAlgorithm) GetAction(state []float32, explore bool) int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	probs, _, _ := a.evaluate(state)

	if explore {
		return a.sampleAction(probs)
	}

	return argmax64(probs)
}

// GetActionWithValue returns action with value estimate.
func (a *A2CAlgorithm) GetActionWithValue(state []float32) (action int, value float64) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	probs, value, _ := a.evaluate(state)
	action = a.sampleAction(probs)
	return action, value
}

// GetStats returns algorithm statistics.
func (a *A2CAlgorithm) GetStats() domainNeural.RLStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return domainNeural.RLStats{
		Algorithm:    domainNeural.AlgorithmA2C,
		UpdateCount:  a.updateCount,
		BufferSize:   len(a.buffer),
		AvgLoss:      a.avgPolicyLoss + a.avgValueLoss,
		AvgLatencyMs: a.avgLatency,
		LastUpdate:   time.Now(),
	}
}

// Reset resets the algorithm state.
func (a *A2CAlgorithm) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.buffer = make([]a2cExperience, 0)
	a.updateCount = 0
	a.avgPolicyLoss = 0
	a.avgValueLoss = 0
	a.avgEntropy = 0

	// Reinitialize weights
	inputDim := a.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	scale := math.Sqrt(2.0 / float64(inputDim))

	for i := range a.sharedWeights {
		a.sharedWeights[i] = (a.rng.Float64() - 0.5) * scale
		a.sharedMomentum[i] = 0
	}
	for i := range a.policyHead {
		a.policyHead[i] = (a.rng.Float64() - 0.5) * 0.1
		a.policyMomentum[i] = 0
	}
	for i := range a.valueHead {
		a.valueHead[i] = (a.rng.Float64() - 0.5) * 0.1
		a.valueMomentum[i] = 0
	}
}

// Private methods

func (a *A2CAlgorithm) evaluate(state []float32) (probs []float64, value float64, entropy float64) {
	probs, value, _ = a.forwardWithHidden(state)

	for _, p := range probs {
		if p > 0 {
			entropy -= p * math.Log(p)
		}
	}

	return probs, value, entropy
}

func (a *A2CAlgorithm) forwardWithHidden(state []float32) (probs []float64, value float64, hidden []float64) {
	hiddenDim := a.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := a.config.NumActions
	if numActions == 0 {
		numActions = 4
	}
	inputDim := a.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	// Shared hidden layer
	hidden = make([]float64, hiddenDim)
	for h := 0; h < hiddenDim; h++ {
		var sum float64
		for i := 0; i < len(state) && i < inputDim; i++ {
			sum += float64(state[i]) * a.sharedWeights[i*hiddenDim+h]
		}
		hidden[h] = math.Max(0, sum) // ReLU
	}

	// Policy head
	logits := make([]float64, numActions)
	for act := 0; act < numActions; act++ {
		var sum float64
		for h := 0; h < hiddenDim; h++ {
			sum += hidden[h] * a.policyHead[h*numActions+act]
		}
		logits[act] = sum
	}
	probs = softmax64(logits)

	// Value head
	for h := 0; h < hiddenDim; h++ {
		value += hidden[h] * a.valueHead[h]
	}

	return probs, value, hidden
}

func (a *A2CAlgorithm) computeReturns() []float64 {
	returns := make([]float64, len(a.buffer))
	var cumReturn float64

	// Bootstrap from last value if not terminal
	if len(a.buffer) > 0 {
		cumReturn = a.buffer[len(a.buffer)-1].value
	}

	for t := len(a.buffer) - 1; t >= 0; t-- {
		cumReturn = a.buffer[t].reward + a.config.Gamma*cumReturn
		returns[t] = cumReturn
	}

	return returns
}

func (a *A2CAlgorithm) computeAdvantages(returns []float64) []float64 {
	if a.config.UseGAE {
		return a.computeGAE()
	}

	// Simple advantage: return - value
	advantages := make([]float64, len(a.buffer))
	for i := range a.buffer {
		advantages[i] = returns[i] - a.buffer[i].value
	}

	// Normalize
	mean, std := meanStd(advantages)
	for i := range advantages {
		advantages[i] = (advantages[i] - mean) / (std + 1e-8)
	}

	return advantages
}

func (a *A2CAlgorithm) computeGAE() []float64 {
	advantages := make([]float64, len(a.buffer))
	var lastGae float64

	for t := len(a.buffer) - 1; t >= 0; t-- {
		nextValue := 0.0
		if t < len(a.buffer)-1 {
			nextValue = a.buffer[t+1].value
		}
		delta := a.buffer[t].reward + a.config.Gamma*nextValue - a.buffer[t].value
		lastGae = delta + a.config.Gamma*a.config.GAELambda*lastGae
		advantages[t] = lastGae
	}

	// Normalize
	mean, std := meanStd(advantages)
	for i := range advantages {
		advantages[i] = (advantages[i] - mean) / (std + 1e-8)
	}

	return advantages
}

func (a *A2CAlgorithm) accumulateGradients(sharedGrad, policyGrad, valueGrad []float64, state []float32, hidden []float64, action int, advantage, valueError float64) {
	hiddenDim := a.config.HiddenDim
	if hiddenDim == 0 {
		hiddenDim = 64
	}
	numActions := a.config.NumActions
	if numActions == 0 {
		numActions = 4
	}
	inputDim := a.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	// Policy gradient
	for h := 0; h < hiddenDim; h++ {
		policyGrad[h*numActions+action] += hidden[h] * advantage
	}

	// Value gradient
	for h := 0; h < hiddenDim; h++ {
		valueGrad[h] += hidden[h] * valueError * a.config.ValueLossCoef
	}

	// Shared layer gradient
	for h := 0; h < hiddenDim; h++ {
		if hidden[h] > 0 { // ReLU gradient
			policySignal := advantage * a.policyHead[h*numActions+action]
			valueSignal := valueError * a.valueHead[h] * a.config.ValueLossCoef
			totalSignal := policySignal + valueSignal

			for i := 0; i < len(state) && i < inputDim; i++ {
				sharedGrad[i*hiddenDim+h] += float64(state[i]) * totalSignal
			}
		}
	}
}

func (a *A2CAlgorithm) applyGradients(sharedGrad, policyGrad, valueGrad []float64, batchSize int) {
	lr := a.config.LearningRate / float64(batchSize)
	beta := 0.9

	// Apply to shared weights
	for i := range a.sharedWeights {
		grad := math.Max(math.Min(sharedGrad[i], a.config.MaxGradNorm), -a.config.MaxGradNorm)
		a.sharedMomentum[i] = beta*a.sharedMomentum[i] + (1-beta)*grad
		a.sharedWeights[i] -= lr * a.sharedMomentum[i]
	}

	// Apply to policy head
	for i := range a.policyHead {
		grad := math.Max(math.Min(policyGrad[i], a.config.MaxGradNorm), -a.config.MaxGradNorm)
		a.policyMomentum[i] = beta*a.policyMomentum[i] + (1-beta)*grad
		a.policyHead[i] -= lr * a.policyMomentum[i]
	}

	// Apply to value head
	for i := range a.valueHead {
		grad := math.Max(math.Min(valueGrad[i], a.config.MaxGradNorm), -a.config.MaxGradNorm)
		a.valueMomentum[i] = beta*a.valueMomentum[i] + (1-beta)*grad
		a.valueHead[i] -= lr * a.valueMomentum[i]
	}
}

func (a *A2CAlgorithm) sampleAction(probs []float64) int {
	r := a.rng.Float64()
	var cumSum float64
	for i, p := range probs {
		cumSum += p
		if r < cumSum {
			return i
		}
	}
	return len(probs) - 1
}
