// Package neural provides neural network infrastructure.
package neural

import (
	"math"
	"math/rand"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// DecisionTransformer implements sequence modeling for trajectory learning.
// Models trajectories as sequences: (return-to-go, state, action, ...)
type DecisionTransformer struct {
	mu     sync.RWMutex
	config domainNeural.DecisionTransformerConfig
	rng    *rand.Rand

	// Embeddings
	stateEmbed  []float64
	actionEmbed []float64
	returnEmbed []float64
	posEmbed    []float64

	// Transformer layers
	attentionWeights [][][]float64 // [layer][Q,K,V,O][weights]
	ffnWeights       [][][]float64 // [layer][up,down][weights]

	// Output head
	actionHead []float64

	// Training buffer
	trajectoryBuffer []*domainNeural.ExtendedTrajectory

	// Statistics
	updateCount int64
	avgLoss     float64
	totalTime   float64
}

// NewDecisionTransformer creates a new Decision Transformer.
func NewDecisionTransformer(config domainNeural.DecisionTransformerConfig) *DecisionTransformer {
	dt := &DecisionTransformer{
		config:           config,
		rng:              rand.New(rand.NewSource(time.Now().UnixNano())),
		trajectoryBuffer: make([]*domainNeural.ExtendedTrajectory, 0),
	}

	dt.initWeights()
	return dt
}

// initWeights initializes all model weights.
func (dt *DecisionTransformer) initWeights() {
	cfg := dt.config

	// State embedding: stateDim -> embeddingDim
	dt.stateEmbed = dt.initEmbedding(cfg.StateDim, cfg.EmbeddingDim)

	// Action embedding: numActions -> embeddingDim
	dt.actionEmbed = dt.initEmbedding(cfg.NumActions, cfg.EmbeddingDim)

	// Return embedding: 1 -> embeddingDim
	dt.returnEmbed = dt.initEmbedding(1, cfg.EmbeddingDim)

	// Positional embedding: contextLength * 3 -> embeddingDim
	dt.posEmbed = dt.initEmbedding(cfg.ContextLength*3, cfg.EmbeddingDim)

	// Transformer layers
	dt.attentionWeights = make([][][]float64, cfg.NumLayers)
	dt.ffnWeights = make([][][]float64, cfg.NumLayers)

	for l := 0; l < cfg.NumLayers; l++ {
		// Attention: Q, K, V, O projections
		dt.attentionWeights[l] = [][]float64{
			dt.initWeight(cfg.EmbeddingDim, cfg.HiddenDim), // Q
			dt.initWeight(cfg.EmbeddingDim, cfg.HiddenDim), // K
			dt.initWeight(cfg.EmbeddingDim, cfg.HiddenDim), // V
			dt.initWeight(cfg.HiddenDim, cfg.EmbeddingDim), // O
		}

		// FFN: up and down projections
		dt.ffnWeights[l] = [][]float64{
			dt.initWeight(cfg.EmbeddingDim, cfg.HiddenDim*4),
			dt.initWeight(cfg.HiddenDim*4, cfg.EmbeddingDim),
		}
	}

	// Action prediction head
	dt.actionHead = dt.initWeight(cfg.EmbeddingDim, cfg.NumActions)
}

// initEmbedding initializes an embedding matrix.
func (dt *DecisionTransformer) initEmbedding(inputDim, outputDim int) []float64 {
	embed := make([]float64, inputDim*outputDim)
	scale := math.Sqrt(2.0 / float64(inputDim))
	for i := range embed {
		embed[i] = (dt.rng.Float64() - 0.5) * scale
	}
	return embed
}

// initWeight initializes a weight matrix.
func (dt *DecisionTransformer) initWeight(inputDim, outputDim int) []float64 {
	weight := make([]float64, inputDim*outputDim)
	scale := math.Sqrt(2.0 / float64(inputDim))
	for i := range weight {
		weight[i] = (dt.rng.Float64() - 0.5) * scale
	}
	return weight
}

// AddTrajectory adds a complete trajectory to the training buffer.
func (dt *DecisionTransformer) AddTrajectory(traj *domainNeural.ExtendedTrajectory) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if !traj.IsComplete || len(traj.Steps) == 0 {
		return
	}

	dt.trajectoryBuffer = append(dt.trajectoryBuffer, traj)

	// Keep buffer bounded
	if len(dt.trajectoryBuffer) > dt.config.MaxBufferSize {
		dt.trajectoryBuffer = dt.trajectoryBuffer[len(dt.trajectoryBuffer)-dt.config.MaxBufferSize:]
	}
}

// Train trains on buffered trajectories.
// Target: <10ms per batch.
func (dt *DecisionTransformer) Train() domainNeural.TrainingResult {
	startTime := time.Now()

	dt.mu.Lock()
	defer dt.mu.Unlock()

	if len(dt.trajectoryBuffer) == 0 {
		return domainNeural.TrainingResult{}
	}

	// Sample mini-batch of trajectories
	batchSize := dt.config.MiniBatchSize
	if batchSize > len(dt.trajectoryBuffer) {
		batchSize = len(dt.trajectoryBuffer)
	}

	batch := make([]*domainNeural.ExtendedTrajectory, batchSize)
	for i := 0; i < batchSize; i++ {
		idx := dt.rng.Intn(len(dt.trajectoryBuffer))
		batch[i] = dt.trajectoryBuffer[idx]
	}

	var totalLoss float64
	var correct, total int

	for _, traj := range batch {
		// Create sequence from trajectory
		sequence := dt.createSequence(traj)

		if len(sequence) < 2 {
			continue
		}

		// Forward pass and compute loss
		for t := 1; t < len(sequence); t++ {
			// Use context up to position t
			contextStart := t - dt.config.ContextLength
			if contextStart < 0 {
				contextStart = 0
			}
			context := sequence[contextStart:t]
			target := sequence[t]

			// Predict action
			predicted := dt.forward(context)
			predictedAction := dt.argmax(predicted)

			// Cross-entropy loss
			targetAction := target.Action
			if targetAction >= 0 && targetAction < len(predicted) {
				loss := -math.Log(predicted[targetAction] + 1e-8)
				totalLoss += loss
			}

			if predictedAction == targetAction {
				correct++
			}
			total++

			// Gradient update (simplified)
			dt.updateWeights(context, targetAction, predicted)
		}
	}

	dt.updateCount++
	if total > 0 {
		dt.avgLoss = totalLoss / float64(total)
	}

	elapsed := float64(time.Since(startTime).Microseconds()) / 1000.0
	dt.totalTime += elapsed

	return domainNeural.TrainingResult{
		Loss:        dt.avgLoss,
		Accuracy:    float64(correct) / float64(total+1),
		UpdateCount: dt.updateCount,
		LatencyMs:   elapsed,
	}
}

// GetAction returns the predicted action conditioned on target return.
func (dt *DecisionTransformer) GetAction(states [][]float32, actions []int, targetReturn float64) int {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	// Build sequence
	sequence := make([]domainNeural.SequenceEntry, 0, len(states))
	returnToGo := targetReturn

	for i := 0; i < len(states); i++ {
		action := 0
		if i < len(actions) {
			action = actions[i]
		}

		sequence = append(sequence, domainNeural.SequenceEntry{
			ReturnToGo: returnToGo,
			State:      states[i],
			Action:     action,
			Timestep:   i,
		})

		// Decrease return-to-go
		if i > 0 {
			returnToGo -= 0.1
		}
	}

	// Forward pass
	logits := dt.forward(sequence)
	return dt.argmax(logits)
}

// forward performs the transformer forward pass.
func (dt *DecisionTransformer) forward(sequence []domainNeural.SequenceEntry) []float64 {
	cfg := dt.config
	seqLen := len(sequence)
	if seqLen > cfg.ContextLength {
		seqLen = cfg.ContextLength
	}
	embedDim := cfg.EmbeddingDim

	// Initialize hidden states (stack all modalities)
	hidden := make([]float64, seqLen*3*embedDim)

	for t := 0; t < seqLen; t++ {
		entry := sequence[len(sequence)-seqLen+t]
		baseIdx := t * 3 * embedDim

		// Embed return
		for d := 0; d < embedDim; d++ {
			hidden[baseIdx+d] = entry.ReturnToGo * dt.returnEmbed[d]
		}

		// Embed state
		for d := 0; d < embedDim; d++ {
			var stateSum float64
			for s := 0; s < len(entry.State) && s < cfg.StateDim; s++ {
				hidden[baseIdx+embedDim+d] += float64(entry.State[s]) * dt.stateEmbed[s*embedDim+d]
			}
			_ = stateSum
		}

		// Embed action
		actionIdx := entry.Action
		if actionIdx >= 0 && actionIdx < cfg.NumActions {
			for d := 0; d < embedDim; d++ {
				hidden[baseIdx+2*embedDim+d] = dt.actionEmbed[actionIdx*embedDim+d]
			}
		}

		// Add positional embedding
		for d := 0; d < 3*embedDim && t*3*embedDim+d < len(dt.posEmbed); d++ {
			hidden[baseIdx+d] += dt.posEmbed[t*3*embedDim+d]
		}
	}

	// Apply transformer layers
	for l := 0; l < cfg.NumLayers; l++ {
		hidden = dt.transformerLayer(hidden, seqLen*3, l)
	}

	// Extract last state position embedding for action prediction
	lastStateIdx := (seqLen*3 - 2) * embedDim
	lastState := hidden[lastStateIdx : lastStateIdx+embedDim]

	// Action prediction
	logits := make([]float64, cfg.NumActions)
	for a := 0; a < cfg.NumActions; a++ {
		var sum float64
		for d := 0; d < embedDim; d++ {
			sum += lastState[d] * dt.actionHead[d*cfg.NumActions+a]
		}
		logits[a] = sum
	}

	return dt.softmax(logits)
}

// transformerLayer applies one transformer layer.
func (dt *DecisionTransformer) transformerLayer(hidden []float64, seqLen, layerIdx int) []float64 {
	cfg := dt.config
	embedDim := cfg.EmbeddingDim
	hiddenDim := cfg.HiddenDim
	headDim := hiddenDim / cfg.NumHeads

	output := make([]float64, len(hidden))

	// Get layer weights
	Wq := dt.attentionWeights[layerIdx][0]
	Wk := dt.attentionWeights[layerIdx][1]
	Wv := dt.attentionWeights[layerIdx][2]
	Wo := dt.attentionWeights[layerIdx][3]

	// Compute Q, K, V for all positions
	Q := make([]float64, seqLen*hiddenDim)
	K := make([]float64, seqLen*hiddenDim)
	V := make([]float64, seqLen*hiddenDim)

	for pos := 0; pos < seqLen; pos++ {
		for h := 0; h < hiddenDim; h++ {
			var qSum, kSum, vSum float64
			for d := 0; d < embedDim; d++ {
				hiddenVal := hidden[pos*embedDim+d]
				qSum += hiddenVal * Wq[d*hiddenDim+h]
				kSum += hiddenVal * Wk[d*hiddenDim+h]
				vSum += hiddenVal * Wv[d*hiddenDim+h]
			}
			Q[pos*hiddenDim+h] = qSum
			K[pos*hiddenDim+h] = kSum
			V[pos*hiddenDim+h] = vSum
		}
	}

	// Causal attention
	for pos := 0; pos < seqLen; pos++ {
		// Compute attention scores for current position
		scores := make([]float64, pos+1)
		for k := 0; k <= pos; k++ {
			var score float64
			for h := 0; h < hiddenDim; h++ {
				score += Q[pos*hiddenDim+h] * K[k*hiddenDim+h]
			}
			scores[k] = score / math.Sqrt(float64(headDim))
		}

		// Softmax
		maxScore := scores[0]
		for _, s := range scores {
			if s > maxScore {
				maxScore = s
			}
		}
		var sumExp float64
		for k := range scores {
			scores[k] = math.Exp(scores[k] - maxScore)
			sumExp += scores[k]
		}
		for k := range scores {
			scores[k] /= sumExp
		}

		// Weighted sum of values
		attnOut := make([]float64, hiddenDim)
		for k := 0; k <= pos; k++ {
			for h := 0; h < hiddenDim; h++ {
				attnOut[h] += scores[k] * V[k*hiddenDim+h]
			}
		}

		// Output projection with residual
		for d := 0; d < embedDim; d++ {
			sum := hidden[pos*embedDim+d] // Residual
			for h := 0; h < hiddenDim; h++ {
				sum += attnOut[h] * Wo[h*embedDim+d]
			}
			output[pos*embedDim+d] = sum
		}
	}

	// FFN with residual
	Wup := dt.ffnWeights[layerIdx][0]
	Wdown := dt.ffnWeights[layerIdx][1]
	ffnHiddenDim := hiddenDim * 4

	for pos := 0; pos < seqLen; pos++ {
		// Up projection + GELU
		ffnHidden := make([]float64, ffnHiddenDim)
		for h := 0; h < ffnHiddenDim; h++ {
			var sum float64
			for d := 0; d < embedDim; d++ {
				sum += output[pos*embedDim+d] * Wup[d*ffnHiddenDim+h]
			}
			// GELU approximation
			ffnHidden[h] = sum * 0.5 * (1 + math.Tanh(0.7978845608*(sum+0.044715*sum*sum*sum)))
		}

		// Down projection with residual
		for d := 0; d < embedDim; d++ {
			sum := output[pos*embedDim+d] // Residual
			for h := 0; h < ffnHiddenDim; h++ {
				sum += ffnHidden[h] * Wdown[h*embedDim+d]
			}
			output[pos*embedDim+d] = sum
		}
	}

	return output
}

// createSequence creates a sequence from a trajectory.
func (dt *DecisionTransformer) createSequence(traj *domainNeural.ExtendedTrajectory) []domainNeural.SequenceEntry {
	sequence := make([]domainNeural.SequenceEntry, 0, len(traj.Steps))

	// Compute returns-to-go
	rewards := make([]float64, len(traj.Steps))
	for i, step := range traj.Steps {
		rewards[i] = step.Reward
	}

	returnsToGo := make([]float64, len(rewards))
	var cumReturn float64
	for t := len(rewards) - 1; t >= 0; t-- {
		cumReturn = rewards[t] + dt.config.Gamma*cumReturn
		returnsToGo[t] = cumReturn
	}

	// Create sequence entries
	for t, step := range traj.Steps {
		sequence = append(sequence, domainNeural.SequenceEntry{
			ReturnToGo: returnsToGo[t],
			State:      step.StateAfter,
			Action:     dt.hashAction(step.Action),
			Timestep:   t,
		})
	}

	return sequence
}

// updateWeights performs a gradient update (simplified).
func (dt *DecisionTransformer) updateWeights(context []domainNeural.SequenceEntry, targetAction int, predicted []float64) {
	lr := dt.config.LearningRate
	embedDim := dt.config.EmbeddingDim
	numActions := dt.config.NumActions

	// Gradient of cross-entropy
	grad := make([]float64, numActions)
	for a := 0; a < numActions; a++ {
		if a == targetAction {
			grad[a] = predicted[a] - 1
		} else {
			grad[a] = predicted[a]
		}
	}

	// Update action head (simplified)
	for d := 0; d < embedDim; d++ {
		for a := 0; a < numActions; a++ {
			dt.actionHead[d*numActions+a] -= lr * grad[a] * 0.1
		}
	}
}

// softmax computes softmax of logits.
func (dt *DecisionTransformer) softmax(logits []float64) []float64 {
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

// argmax returns the index of the maximum value.
func (dt *DecisionTransformer) argmax(values []float64) int {
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

// hashAction hashes an action string to an action index.
func (dt *DecisionTransformer) hashAction(action string) int {
	var hash int
	for i := 0; i < len(action); i++ {
		hash = (hash*31 + int(action[i])) % dt.config.NumActions
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// GetStats returns transformer statistics.
func (dt *DecisionTransformer) GetStats() map[string]interface{} {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	return map[string]interface{}{
		"updateCount":   dt.updateCount,
		"bufferSize":    len(dt.trajectoryBuffer),
		"avgLoss":       dt.avgLoss,
		"contextLength": dt.config.ContextLength,
		"numLayers":     dt.config.NumLayers,
		"avgTimeMs":     dt.totalTime / float64(dt.updateCount+1),
	}
}

// ClearBuffer clears the trajectory buffer.
func (dt *DecisionTransformer) ClearBuffer() {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.trajectoryBuffer = make([]*domainNeural.ExtendedTrajectory, 0)
}

// GetBufferSize returns the current buffer size.
func (dt *DecisionTransformer) GetBufferSize() int {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return len(dt.trajectoryBuffer)
}
