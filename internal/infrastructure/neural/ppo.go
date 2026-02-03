// Package neural provides neural network infrastructure.
package neural

import (
	"math"
	"math/rand"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// PPOAlgorithm implements Proximal Policy Optimization.
type PPOAlgorithm struct {
	mu     sync.RWMutex
	config domainNeural.PPOConfig
	rng    *rand.Rand

	// Network weights
	policyWeights []float64
	valueWeights  []float64

	// Optimizer state (momentum)
	policyMomentum []float64
	valueMomentum  []float64

	// Experience buffer
	buffer []ppoExperience

	// Statistics
	updateCount  int64
	totalLoss    float64
	approxKL     float64
	clipFraction float64
	avgLatency   float64
}

// ppoExperience is a PPO-specific experience.
type ppoExperience struct {
	state     []float32
	action    int
	reward    float64
	value     float64
	logProb   float64
	advantage float64
	return_   float64
}

// NewPPOAlgorithm creates a new PPO algorithm.
func NewPPOAlgorithm(config domainNeural.PPOConfig) *PPOAlgorithm {
	dim := config.InputDim
	if dim == 0 {
		dim = 256
	}

	ppo := &PPOAlgorithm{
		config:         config,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
		policyWeights:  make([]float64, dim),
		valueWeights:   make([]float64, dim),
		policyMomentum: make([]float64, dim),
		valueMomentum:  make([]float64, dim),
		buffer:         make([]ppoExperience, 0),
	}

	// Xavier initialization
	scale := math.Sqrt(2.0 / float64(dim))
	for i := 0; i < dim; i++ {
		ppo.policyWeights[i] = (ppo.rng.Float64() - 0.5) * scale
		ppo.valueWeights[i] = (ppo.rng.Float64() - 0.5) * scale
	}

	return ppo
}

// AddExperience adds experience to the buffer.
func (p *PPOAlgorithm) AddExperience(exp domainNeural.RLExperience) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Compute value and log probability
	value := p.computeValue(exp.State)
	logProb := p.computeLogProb(exp.State, exp.Action)

	p.buffer = append(p.buffer, ppoExperience{
		state:   exp.State,
		action:  exp.Action,
		reward:  exp.Reward,
		value:   value,
		logProb: logProb,
	})
}

// AddTrajectoryExperiences adds experiences from a trajectory with computed advantages.
func (p *PPOAlgorithm) AddTrajectoryExperiences(experiences []domainNeural.RLExperience) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(experiences) == 0 {
		return
	}

	// Compute values for all experiences
	values := make([]float64, len(experiences))
	for i, exp := range experiences {
		values[i] = p.computeValue(exp.State)
	}

	// Compute GAE advantages
	rewards := make([]float64, len(experiences))
	for i, exp := range experiences {
		rewards[i] = exp.Reward
	}
	advantages := p.computeGAE(rewards, values)

	// Compute returns
	returns := p.computeReturns(rewards)

	// Add to buffer
	for i, exp := range experiences {
		p.buffer = append(p.buffer, ppoExperience{
			state:     exp.State,
			action:    exp.Action,
			reward:    exp.Reward,
			value:     values[i],
			logProb:   p.computeLogProb(exp.State, exp.Action),
			advantage: advantages[i],
			return_:   returns[i],
		})
	}
}

// Update performs a PPO update.
// Target: <10ms
func (p *PPOAlgorithm) Update() domainNeural.RLUpdateResult {
	startTime := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.buffer) < p.config.MiniBatchSize {
		return domainNeural.RLUpdateResult{}
	}

	// Normalize advantages
	advantages := make([]float64, len(p.buffer))
	for i, exp := range p.buffer {
		advantages[i] = exp.advantage
	}
	advMean, advStd := meanStd(advantages)
	for i := range p.buffer {
		p.buffer[i].advantage = (p.buffer[i].advantage - advMean) / (advStd + 1e-8)
	}

	var totalPolicyLoss, totalValueLoss, totalEntropy float64
	var totalClipFrac, totalKL float64
	var numUpdates int

	// Multiple epochs
	for epoch := 0; epoch < p.config.Epochs; epoch++ {
		// Shuffle buffer
		p.shuffleBuffer()

		// Process mini-batches
		for i := 0; i < len(p.buffer); i += p.config.MiniBatchSize {
			end := i + p.config.MiniBatchSize
			if end > len(p.buffer) {
				end = len(p.buffer)
			}
			batch := p.buffer[i:end]
			if len(batch) < p.config.MiniBatchSize/2 {
				continue
			}

			result := p.updateMiniBatch(batch)
			totalPolicyLoss += result.policyLoss
			totalValueLoss += result.valueLoss
			totalEntropy += result.entropy
			totalClipFrac += result.clipFrac
			totalKL += result.kl
			numUpdates++

			// Early stopping if KL too high
			if result.kl > p.config.TargetKL*1.5 {
				break
			}
		}
	}

	// Clear buffer
	p.buffer = make([]ppoExperience, 0)
	p.updateCount++

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	p.avgLatency = (p.avgLatency*float64(p.updateCount-1) + elapsed) / float64(p.updateCount)

	if numUpdates == 0 {
		numUpdates = 1
	}

	p.approxKL = totalKL / float64(numUpdates)
	p.clipFraction = totalClipFrac / float64(numUpdates)
	p.totalLoss = (totalPolicyLoss + totalValueLoss) / float64(numUpdates)

	return domainNeural.RLUpdateResult{
		PolicyLoss:   totalPolicyLoss / float64(numUpdates),
		ValueLoss:    totalValueLoss / float64(numUpdates),
		Entropy:      totalEntropy / float64(numUpdates),
		TotalLoss:    p.totalLoss,
		ClipFraction: p.clipFraction,
		ApproxKL:     p.approxKL,
		LatencyMs:    elapsed,
	}
}

// GetAction returns an action for the given state.
func (p *PPOAlgorithm) GetAction(state []float32, explore bool) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	logits := p.computeLogits(state)
	probs := softmax64(logits)

	if explore {
		return p.sampleAction(probs)
	}

	return argmax64(probs)
}

// GetActionWithInfo returns action with value and log probability.
func (p *PPOAlgorithm) GetActionWithInfo(state []float32) (action int, logProb, value float64) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	logits := p.computeLogits(state)
	probs := softmax64(logits)
	action = p.sampleAction(probs)

	return action, math.Log(probs[action] + 1e-8), p.computeValue(state)
}

// GetStats returns algorithm statistics.
func (p *PPOAlgorithm) GetStats() domainNeural.RLStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return domainNeural.RLStats{
		Algorithm:    domainNeural.AlgorithmPPO,
		UpdateCount:  p.updateCount,
		BufferSize:   len(p.buffer),
		AvgLoss:      p.totalLoss,
		AvgLatencyMs: p.avgLatency,
		LastUpdate:   time.Now(),
	}
}

// Reset resets the algorithm state.
func (p *PPOAlgorithm) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buffer = make([]ppoExperience, 0)
	p.updateCount = 0
	p.totalLoss = 0
	p.approxKL = 0
	p.clipFraction = 0

	// Reinitialize weights
	dim := len(p.policyWeights)
	scale := math.Sqrt(2.0 / float64(dim))
	for i := 0; i < dim; i++ {
		p.policyWeights[i] = (p.rng.Float64() - 0.5) * scale
		p.valueWeights[i] = (p.rng.Float64() - 0.5) * scale
		p.policyMomentum[i] = 0
		p.valueMomentum[i] = 0
	}
}

// Private methods

func (p *PPOAlgorithm) computeValue(state []float32) float64 {
	var value float64
	for i := 0; i < len(state) && i < len(p.valueWeights); i++ {
		value += float64(state[i]) * p.valueWeights[i]
	}
	return value
}

func (p *PPOAlgorithm) computeLogits(state []float32) []float64 {
	numActions := p.config.NumActions
	if numActions == 0 {
		numActions = 4
	}
	logits := make([]float64, numActions)

	for a := 0; a < numActions; a++ {
		for i := 0; i < len(state) && i < len(p.policyWeights); i++ {
			logits[a] += float64(state[i]) * p.policyWeights[i] * (1 + float64(a)*0.1)
		}
	}

	return logits
}

func (p *PPOAlgorithm) computeLogProb(state []float32, action int) float64 {
	logits := p.computeLogits(state)
	probs := softmax64(logits)
	if action < len(probs) {
		return math.Log(probs[action] + 1e-8)
	}
	return math.Log(1e-8)
}

func (p *PPOAlgorithm) computeGAE(rewards, values []float64) []float64 {
	advantages := make([]float64, len(rewards))
	var lastGae float64

	for t := len(rewards) - 1; t >= 0; t-- {
		nextValue := 0.0
		if t < len(rewards)-1 {
			nextValue = values[t+1]
		}
		delta := rewards[t] + p.config.Gamma*nextValue - values[t]
		lastGae = delta + p.config.Gamma*p.config.GAELambda*lastGae
		advantages[t] = lastGae
	}

	return advantages
}

func (p *PPOAlgorithm) computeReturns(rewards []float64) []float64 {
	returns := make([]float64, len(rewards))
	var cumReturn float64

	for t := len(rewards) - 1; t >= 0; t-- {
		cumReturn = rewards[t] + p.config.Gamma*cumReturn
		returns[t] = cumReturn
	}

	return returns
}

func (p *PPOAlgorithm) shuffleBuffer() {
	for i := len(p.buffer) - 1; i > 0; i-- {
		j := p.rng.Intn(i + 1)
		p.buffer[i], p.buffer[j] = p.buffer[j], p.buffer[i]
	}
}

func (p *PPOAlgorithm) updateMiniBatch(batch []ppoExperience) struct {
	policyLoss, valueLoss, entropy, clipFrac, kl float64
} {
	var policyLoss, valueLoss, entropy, clipFrac, kl float64

	policyGrad := make([]float64, len(p.policyWeights))
	valueGrad := make([]float64, len(p.valueWeights))

	for _, exp := range batch {
		// Current policy
		logits := p.computeLogits(exp.state)
		probs := softmax64(logits)
		newLogProb := math.Log(probs[exp.action] + 1e-8)
		currentValue := p.computeValue(exp.state)

		// Ratio for PPO
		ratio := math.Exp(newLogProb - exp.logProb)

		// Clipped surrogate objective
		surr1 := ratio * exp.advantage
		clippedRatio := math.Max(math.Min(ratio, 1+p.config.ClipRange), 1-p.config.ClipRange)
		surr2 := clippedRatio * exp.advantage

		policyLossI := -math.Min(surr1, surr2)
		policyLoss += policyLossI

		// Track clipping
		if math.Abs(ratio-1) > p.config.ClipRange {
			clipFrac++
		}

		// KL divergence approximation
		kl += exp.logProb - newLogProb

		// Value loss
		valueLossI := (currentValue - exp.return_) * (currentValue - exp.return_)
		valueLoss += valueLossI

		// Entropy
		var entropyI float64
		for _, prob := range probs {
			if prob > 0 {
				entropyI -= prob * math.Log(prob)
			}
		}
		entropy += entropyI

		// Compute gradients (simplified)
		for i := 0; i < len(exp.state) && i < len(policyGrad); i++ {
			policyGrad[i] += float64(exp.state[i]) * policyLossI * 0.01
			valueGrad[i] += float64(exp.state[i]) * valueLossI * 0.01
		}
	}

	// Apply gradients with momentum
	lr := p.config.LearningRate
	beta := 0.9

	for i := 0; i < len(p.policyWeights); i++ {
		p.policyMomentum[i] = beta*p.policyMomentum[i] + (1-beta)*policyGrad[i]
		p.policyWeights[i] -= lr * p.policyMomentum[i]

		p.valueMomentum[i] = beta*p.valueMomentum[i] + (1-beta)*valueGrad[i]
		p.valueWeights[i] -= lr * p.valueMomentum[i]
	}

	n := float64(len(batch))
	return struct {
		policyLoss, valueLoss, entropy, clipFrac, kl float64
	}{
		policyLoss: policyLoss / n,
		valueLoss:  valueLoss / n,
		entropy:    entropy / n,
		clipFrac:   clipFrac / n,
		kl:         kl / n,
	}
}

func (p *PPOAlgorithm) sampleAction(probs []float64) int {
	r := p.rng.Float64()
	var cumSum float64
	for i, prob := range probs {
		cumSum += prob
		if r < cumSum {
			return i
		}
	}
	return len(probs) - 1
}

// Helper functions

func meanStd(values []float64) (mean, std float64) {
	if len(values) == 0 {
		return 0, 1
	}

	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	for _, v := range values {
		std += (v - mean) * (v - mean)
	}
	std = math.Sqrt(std / float64(len(values)))

	if std == 0 {
		std = 1
	}

	return mean, std
}

func softmax64(logits []float64) []float64 {
	maxVal := logits[0]
	for _, v := range logits {
		if v > maxVal {
			maxVal = v
		}
	}

	exps := make([]float64, len(logits))
	var sum float64
	for i, v := range logits {
		exps[i] = math.Exp(v - maxVal)
		sum += exps[i]
	}

	for i := range exps {
		exps[i] /= sum
	}

	return exps
}

func argmax64(values []float64) int {
	maxIdx := 0
	maxVal := values[0]
	for i := 1; i < len(values); i++ {
		if values[i] > maxVal {
			maxVal = values[i]
			maxIdx = i
		}
	}
	return maxIdx
}
