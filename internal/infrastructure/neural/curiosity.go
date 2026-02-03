// Package neural provides neural network infrastructure.
package neural

import (
	"math"
	"math/rand"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// CuriosityModule implements Intrinsic Curiosity Module (ICM).
type CuriosityModule struct {
	mu     sync.RWMutex
	config domainNeural.CuriosityConfig
	rng    *rand.Rand

	// Feature encoder: state -> feature
	encoderWeights []float64

	// Forward dynamics model: (feature, action) -> predicted next feature
	forwardWeights []float64

	// Inverse dynamics model: (feature, next feature) -> action
	inverseWeights []float64

	// Random network for RND (if enabled)
	randomNetFixed  []float64
	randomNetTrain  []float64

	// Optimizer state
	encoderMomentum []float64
	forwardMomentum []float64
	inverseMomentum []float64
	rndMomentum     []float64

	// Visit counts for novelty
	visitCounts map[uint64]int

	// Statistics
	updateCount       int64
	avgIntrinsicReward float64
	avgForwardLoss    float64
	avgInverseLoss    float64
	avgNovelty        float64
	avgLatency        float64
}

// NewCuriosityModule creates a new Curiosity module.
func NewCuriosityModule(config domainNeural.CuriosityConfig) *CuriosityModule {
	inputDim := config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	featureDim := config.FeatureDim
	if featureDim == 0 {
		featureDim = 64
	}
	numActions := config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	cm := &CuriosityModule{
		config:          config,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
		encoderWeights:  make([]float64, inputDim*featureDim),
		forwardWeights:  make([]float64, (featureDim+numActions)*featureDim),
		inverseWeights:  make([]float64, 2*featureDim*numActions),
		encoderMomentum: make([]float64, inputDim*featureDim),
		forwardMomentum: make([]float64, (featureDim+numActions)*featureDim),
		inverseMomentum: make([]float64, 2*featureDim*numActions),
		visitCounts:     make(map[uint64]int),
	}

	// Initialize weights
	scale := math.Sqrt(2.0 / float64(inputDim))
	for i := range cm.encoderWeights {
		cm.encoderWeights[i] = (cm.rng.Float64() - 0.5) * scale
	}
	for i := range cm.forwardWeights {
		cm.forwardWeights[i] = (cm.rng.Float64() - 0.5) * 0.1
	}
	for i := range cm.inverseWeights {
		cm.inverseWeights[i] = (cm.rng.Float64() - 0.5) * 0.1
	}

	// Initialize RND if enabled
	if config.UseRND {
		cm.randomNetFixed = make([]float64, inputDim*featureDim)
		cm.randomNetTrain = make([]float64, inputDim*featureDim)
		cm.rndMomentum = make([]float64, inputDim*featureDim)

		for i := range cm.randomNetFixed {
			cm.randomNetFixed[i] = (cm.rng.Float64() - 0.5) * scale
			cm.randomNetTrain[i] = (cm.rng.Float64() - 0.5) * scale
		}
	}

	return cm
}

// ComputeIntrinsicReward computes the intrinsic curiosity reward.
func (cm *CuriosityModule) ComputeIntrinsicReward(state, nextState []float32, action int) float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.config.UseRND {
		return cm.computeRNDReward(nextState)
	}

	// ICM: prediction error as intrinsic reward
	feature := cm.encodeState(state)
	nextFeature := cm.encodeState(nextState)
	predictedNextFeature := cm.forwardPredict(feature, action)

	// L2 distance between predicted and actual next feature
	var mse float64
	for i := range nextFeature {
		diff := nextFeature[i] - predictedNextFeature[i]
		mse += diff * diff
	}
	mse /= float64(len(nextFeature))

	return cm.config.IntrinsicCoef * mse
}

// Update trains the curiosity module.
func (cm *CuriosityModule) Update(state, nextState []float32, action int) domainNeural.RLUpdateResult {
	startTime := time.Now()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Encode states
	feature := cm.encodeState(state)
	nextFeature := cm.encodeState(nextState)

	// Forward model loss
	predictedNext := cm.forwardPredict(feature, action)
	var forwardLoss float64
	for i := range nextFeature {
		diff := nextFeature[i] - predictedNext[i]
		forwardLoss += diff * diff
	}
	forwardLoss /= float64(len(nextFeature))

	// Inverse model loss
	predictedAction := cm.inversePredictLogits(feature, nextFeature)
	inverseProbs := softmax64(predictedAction)
	inverseLoss := -math.Log(inverseProbs[action] + 1e-8)

	// Update forward model
	cm.updateForwardModel(feature, nextFeature, predictedNext, action)

	// Update inverse model
	cm.updateInverseModel(feature, nextFeature, action, predictedAction)

	// Update RND if enabled
	if cm.config.UseRND {
		cm.updateRND(nextState)
	}

	// Update novelty tracking
	stateHash := hashStateFloat32(state)
	cm.visitCounts[stateHash]++
	novelty := 1.0 / math.Sqrt(float64(cm.visitCounts[stateHash]))

	cm.updateCount++
	cm.avgForwardLoss = (cm.avgForwardLoss*float64(cm.updateCount-1) + forwardLoss) / float64(cm.updateCount)
	cm.avgInverseLoss = (cm.avgInverseLoss*float64(cm.updateCount-1) + inverseLoss) / float64(cm.updateCount)
	cm.avgNovelty = (cm.avgNovelty*float64(cm.updateCount-1) + novelty) / float64(cm.updateCount)
	cm.avgIntrinsicReward = (cm.avgIntrinsicReward*float64(cm.updateCount-1) + cm.config.IntrinsicCoef*forwardLoss) / float64(cm.updateCount)

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	cm.avgLatency = (cm.avgLatency*float64(cm.updateCount-1) + elapsed) / float64(cm.updateCount)

	return domainNeural.RLUpdateResult{
		TotalLoss: forwardLoss + inverseLoss,
		LatencyMs: elapsed,
	}
}

// GetNovelty returns novelty score for a state.
func (cm *CuriosityModule) GetNovelty(state []float32) float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stateHash := hashStateFloat32(state)
	count := cm.visitCounts[stateHash]
	if count == 0 {
		return 1.0 // Never visited
	}
	return 1.0 / math.Sqrt(float64(count))
}

// IsNovel returns true if state novelty exceeds threshold.
func (cm *CuriosityModule) IsNovel(state []float32) bool {
	return cm.GetNovelty(state) > cm.config.NoveltyThreshold
}

// GetStats returns module statistics.
func (cm *CuriosityModule) GetStats() map[string]float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return map[string]float64{
		"updateCount":       float64(cm.updateCount),
		"avgIntrinsicReward": cm.avgIntrinsicReward,
		"avgForwardLoss":    cm.avgForwardLoss,
		"avgInverseLoss":    cm.avgInverseLoss,
		"avgNovelty":        cm.avgNovelty,
		"avgLatencyMs":      cm.avgLatency,
		"statesVisited":     float64(len(cm.visitCounts)),
	}
}

// Reset resets the module state.
func (cm *CuriosityModule) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.visitCounts = make(map[uint64]int)
	cm.updateCount = 0
	cm.avgIntrinsicReward = 0
	cm.avgForwardLoss = 0
	cm.avgInverseLoss = 0
	cm.avgNovelty = 0

	// Reinitialize weights
	inputDim := cm.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}
	scale := math.Sqrt(2.0 / float64(inputDim))

	for i := range cm.encoderWeights {
		cm.encoderWeights[i] = (cm.rng.Float64() - 0.5) * scale
		cm.encoderMomentum[i] = 0
	}
	for i := range cm.forwardWeights {
		cm.forwardWeights[i] = (cm.rng.Float64() - 0.5) * 0.1
		cm.forwardMomentum[i] = 0
	}
	for i := range cm.inverseWeights {
		cm.inverseWeights[i] = (cm.rng.Float64() - 0.5) * 0.1
		cm.inverseMomentum[i] = 0
	}
}

// Private methods

func (cm *CuriosityModule) encodeState(state []float32) []float64 {
	featureDim := cm.config.FeatureDim
	if featureDim == 0 {
		featureDim = 64
	}
	inputDim := cm.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	feature := make([]float64, featureDim)
	for f := 0; f < featureDim; f++ {
		var sum float64
		for i := 0; i < len(state) && i < inputDim; i++ {
			sum += float64(state[i]) * cm.encoderWeights[i*featureDim+f]
		}
		feature[f] = math.Tanh(sum) // Tanh activation
	}
	return feature
}

func (cm *CuriosityModule) forwardPredict(feature []float64, action int) []float64 {
	featureDim := cm.config.FeatureDim
	if featureDim == 0 {
		featureDim = 64
	}
	numActions := cm.config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	// Concatenate feature and one-hot action
	inputDim := featureDim + numActions
	input := make([]float64, inputDim)
	copy(input, feature)
	if action < numActions {
		input[featureDim+action] = 1.0
	}

	// Forward pass
	output := make([]float64, featureDim)
	for f := 0; f < featureDim; f++ {
		var sum float64
		for i := 0; i < inputDim; i++ {
			sum += input[i] * cm.forwardWeights[i*featureDim+f]
		}
		output[f] = sum
	}
	return output
}

func (cm *CuriosityModule) inversePredictLogits(feature, nextFeature []float64) []float64 {
	featureDim := cm.config.FeatureDim
	if featureDim == 0 {
		featureDim = 64
	}
	numActions := cm.config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	// Concatenate feature and next feature
	inputDim := 2 * featureDim
	input := make([]float64, inputDim)
	copy(input[:featureDim], feature)
	copy(input[featureDim:], nextFeature)

	// Forward pass
	logits := make([]float64, numActions)
	for a := 0; a < numActions; a++ {
		var sum float64
		for i := 0; i < inputDim; i++ {
			sum += input[i] * cm.inverseWeights[i*numActions+a]
		}
		logits[a] = sum
	}
	return logits
}

func (cm *CuriosityModule) updateForwardModel(feature, nextFeature, predictedNext []float64, action int) {
	featureDim := cm.config.FeatureDim
	if featureDim == 0 {
		featureDim = 64
	}
	numActions := cm.config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	// Input
	inputDim := featureDim + numActions
	input := make([]float64, inputDim)
	copy(input, feature)
	if action < numActions {
		input[featureDim+action] = 1.0
	}

	// Compute gradients and update
	lr := cm.config.ForwardLR
	beta := 0.9

	for f := 0; f < featureDim; f++ {
		error := predictedNext[f] - nextFeature[f]
		for i := 0; i < inputDim; i++ {
			grad := error * input[i]
			idx := i*featureDim + f
			cm.forwardMomentum[idx] = beta*cm.forwardMomentum[idx] + (1-beta)*grad
			cm.forwardWeights[idx] -= lr * cm.forwardMomentum[idx]
		}
	}
}

func (cm *CuriosityModule) updateInverseModel(feature, nextFeature []float64, action int, logits []float64) {
	featureDim := cm.config.FeatureDim
	if featureDim == 0 {
		featureDim = 64
	}
	numActions := cm.config.NumActions
	if numActions == 0 {
		numActions = 4
	}

	// Input
	inputDim := 2 * featureDim
	input := make([]float64, inputDim)
	copy(input[:featureDim], feature)
	copy(input[featureDim:], nextFeature)

	// Softmax and gradient
	probs := softmax64(logits)
	lr := cm.config.InverseLR
	beta := 0.9

	for a := 0; a < numActions; a++ {
		target := 0.0
		if a == action {
			target = 1.0
		}
		error := probs[a] - target

		for i := 0; i < inputDim; i++ {
			grad := error * input[i]
			idx := i*numActions + a
			cm.inverseMomentum[idx] = beta*cm.inverseMomentum[idx] + (1-beta)*grad
			cm.inverseWeights[idx] -= lr * cm.inverseMomentum[idx]
		}
	}
}

func (cm *CuriosityModule) computeRNDReward(state []float32) float64 {
	featureDim := cm.config.FeatureDim
	if featureDim == 0 {
		featureDim = 64
	}
	inputDim := cm.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	// Fixed network output
	fixedOutput := make([]float64, featureDim)
	for f := 0; f < featureDim; f++ {
		var sum float64
		for i := 0; i < len(state) && i < inputDim; i++ {
			sum += float64(state[i]) * cm.randomNetFixed[i*featureDim+f]
		}
		fixedOutput[f] = sum
	}

	// Trainable network output
	trainOutput := make([]float64, featureDim)
	for f := 0; f < featureDim; f++ {
		var sum float64
		for i := 0; i < len(state) && i < inputDim; i++ {
			sum += float64(state[i]) * cm.randomNetTrain[i*featureDim+f]
		}
		trainOutput[f] = sum
	}

	// Prediction error as reward
	var mse float64
	for f := 0; f < featureDim; f++ {
		diff := fixedOutput[f] - trainOutput[f]
		mse += diff * diff
	}
	mse /= float64(featureDim)

	return cm.config.IntrinsicCoef * mse
}

func (cm *CuriosityModule) updateRND(state []float32) {
	featureDim := cm.config.FeatureDim
	if featureDim == 0 {
		featureDim = 64
	}
	inputDim := cm.config.InputDim
	if inputDim == 0 {
		inputDim = 256
	}

	// Compute outputs
	fixedOutput := make([]float64, featureDim)
	trainOutput := make([]float64, featureDim)
	for f := 0; f < featureDim; f++ {
		var sumFixed, sumTrain float64
		for i := 0; i < len(state) && i < inputDim; i++ {
			sumFixed += float64(state[i]) * cm.randomNetFixed[i*featureDim+f]
			sumTrain += float64(state[i]) * cm.randomNetTrain[i*featureDim+f]
		}
		fixedOutput[f] = sumFixed
		trainOutput[f] = sumTrain
	}

	// Update trainable network towards fixed network
	lr := cm.config.LearningRate
	beta := 0.9

	for f := 0; f < featureDim; f++ {
		error := trainOutput[f] - fixedOutput[f]
		for i := 0; i < len(state) && i < inputDim; i++ {
			grad := error * float64(state[i])
			idx := i*featureDim + f
			cm.rndMomentum[idx] = beta*cm.rndMomentum[idx] + (1-beta)*grad
			cm.randomNetTrain[idx] -= lr * cm.rndMomentum[idx]
		}
	}
}

// hashStateFloat32 hashes a float32 state for novelty tracking.
func hashStateFloat32(state []float32) uint64 {
	var h uint64 = 14695981039346656037
	for _, v := range state {
		discreteVal := int32(v * 1000)
		h ^= uint64(discreteVal)
		h *= 1099511628211
	}
	return h
}
