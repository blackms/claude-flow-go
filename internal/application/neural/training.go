// Package neural provides neural application services.
package neural

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/neural"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
)

// TrainingEngine provides neural pattern training capabilities.
type TrainingEngine struct {
	mu           sync.RWMutex
	generator    *infraNeural.EmbeddingGenerator
	trajectories []*neural.Trajectory
	config       neural.TrainingConfig
	rng          *rand.Rand

	// LoRA and EWC integration
	loraEngine   *infraNeural.LoRAEngine
	ewcEngine    *infraNeural.EWCEngine
	loraConfig   *neural.LoRAConfig
	ewcConfig    *neural.EWCConfig
	enableLoRA   bool
	enableEWC    bool
}

// NewTrainingEngine creates a new training engine.
func NewTrainingEngine(dimension int) *TrainingEngine {
	return &TrainingEngine{
		generator:    infraNeural.NewEmbeddingGenerator(dimension),
		trajectories: make([]*neural.Trajectory, 0),
		config:       neural.DefaultTrainingConfig(),
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
		enableLoRA:   false,
		enableEWC:    false,
	}
}

// NewTrainingEngineWithLoRAEWC creates a training engine with LoRA and EWC support.
func NewTrainingEngineWithLoRAEWC(dimension int, loraConfig *neural.LoRAConfig, ewcConfig *neural.EWCConfig) *TrainingEngine {
	engine := NewTrainingEngine(dimension)

	if loraConfig != nil {
		engine.loraConfig = loraConfig
		engine.enableLoRA = true
		engine.loraEngine = infraNeural.NewLoRAEngine(infraNeural.LoRAEngineConfig{
			DefaultConfig: *loraConfig,
			MaxAdapters:   100,
			EnableCaching: true,
			BatchSize:     32,
		})
	}

	if ewcConfig != nil {
		engine.ewcConfig = ewcConfig
		engine.enableEWC = true
		engine.ewcEngine = infraNeural.NewEWCEngine(*ewcConfig)
	}

	return engine
}

// EnableLoRA enables LoRA adaptation with the given configuration.
func (e *TrainingEngine) EnableLoRA(config neural.LoRAConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.loraConfig = &config
	e.enableLoRA = true
	e.loraEngine = infraNeural.NewLoRAEngine(infraNeural.LoRAEngineConfig{
		DefaultConfig: config,
		MaxAdapters:   100,
		EnableCaching: true,
		BatchSize:     32,
	})
}

// DisableLoRA disables LoRA adaptation.
func (e *TrainingEngine) DisableLoRA() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enableLoRA = false
}

// EnableEWC enables EWC regularization with the given configuration.
func (e *TrainingEngine) EnableEWC(config neural.EWCConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.ewcConfig = &config
	e.enableEWC = true
	e.ewcEngine = infraNeural.NewEWCEngine(config)
}

// DisableEWC disables EWC regularization.
func (e *TrainingEngine) DisableEWC() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enableEWC = false
}

// IsLoRAEnabled returns whether LoRA is enabled.
func (e *TrainingEngine) IsLoRAEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enableLoRA
}

// IsEWCEnabled returns whether EWC is enabled.
func (e *TrainingEngine) IsEWCEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enableEWC
}

// GetLoRAEngine returns the LoRA engine.
func (e *TrainingEngine) GetLoRAEngine() *infraNeural.LoRAEngine {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loraEngine
}

// GetEWCEngine returns the EWC engine.
func (e *TrainingEngine) GetEWCEngine() *infraNeural.EWCEngine {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ewcEngine
}

// SetConfig sets the training configuration.
func (e *TrainingEngine) SetConfig(config neural.TrainingConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}

// GetConfig returns the current training configuration.
func (e *TrainingEngine) GetConfig() neural.TrainingConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// TrainingData represents input data for training.
type TrainingData struct {
	Texts    []string `json:"texts"`
	Labels   []string `json:"labels,omitempty"`
}

// Train trains neural patterns using contrastive learning.
func (e *TrainingEngine) Train(data TrainingData, progressFn func(epoch int, loss float64)) (*neural.TrainingMetrics, []*neural.Pattern, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(data.Texts) == 0 {
		return nil, nil, fmt.Errorf("no training data provided")
	}

	config := e.config
	startTime := time.Now()
	lossHistory := make([]float64, 0, config.Epochs)
	patterns := make([]*neural.Pattern, 0)

	// Generate embeddings for all training texts
	embeddings := make([][]float32, len(data.Texts))
	for i, text := range data.Texts {
		embeddings[i] = e.generator.Generate(text)
	}

	var totalAdaptations int
	var ewcRegLoss float64

	// Collect gradients for EWC if enabled
	var gradientBatch [][]float64
	if e.enableEWC {
		gradientBatch = make([][]float64, 0)
	}

	// Training loop
	for epoch := 0; epoch < config.Epochs; epoch++ {
		epochLoss := 0.0

		// Process in batches
		for batchStart := 0; batchStart < len(embeddings); batchStart += config.BatchSize {
			batchEnd := batchStart + config.BatchSize
			if batchEnd > len(embeddings) {
				batchEnd = len(embeddings)
			}

			batchEmbeddings := embeddings[batchStart:batchEnd]
			batchTexts := data.Texts[batchStart:batchEnd]

			// Compute contrastive loss for this batch
			loss := e.computeInfoNCELoss(batchEmbeddings)
			epochLoss += loss

			// Apply gradient update (simulated MicroLoRA adaptation)
			for i := range batchEmbeddings {
				gradient := e.computeGradient(batchEmbeddings[i], loss)

				// Apply EWC regularization if enabled
				if e.enableEWC && e.ewcEngine != nil {
					currentParams := float32ToFloat64(embeddings[batchStart+i])
					ewcLoss := e.ewcEngine.ComputeRegularizationLoss(currentParams)
					ewcRegLoss += ewcLoss.RegularizationLoss

					// Apply EWC gradient
					regularizedGrad := e.ewcEngine.ApplyRegularization(float32ToFloat64(gradient), currentParams)
					gradient = float64ToFloat32(regularizedGrad)

					// Collect gradients for Fisher estimation
					if len(gradientBatch) < 100 {
						gradientBatch = append(gradientBatch, float32ToFloat64(gradient))
					}
				}

				embeddings[batchStart+i] = e.applyGradient(embeddings[batchStart+i], gradient, config.LearningRate)
				totalAdaptations++
			}

			// Create patterns from the updated embeddings on last epoch
			if epoch == config.Epochs-1 {
				for i, emb := range batchEmbeddings {
					// Apply LoRA adaptation if enabled
					adaptedEmb := emb
					if e.enableLoRA && e.loraEngine != nil {
						adapterIDs := e.loraEngine.ListAdapters()
						if len(adapterIDs) > 0 {
							delta, err := e.loraEngine.Forward(adapterIDs[0], float32ToFloat64(emb))
							if err == nil {
								for j := range adaptedEmb {
									if j < len(delta) {
										adaptedEmb[j] += float32(delta[j])
									}
								}
							}
						}
					}

					pattern := neural.NewPattern(config.PatternType, batchTexts[i], adaptedEmb)
					pattern.Confidence = 1.0 - (loss / 10.0) // Convert loss to confidence
					if pattern.Confidence < 0 {
						pattern.Confidence = 0.1
					}
					patterns = append(patterns, pattern)
				}
			}
		}

		avgLoss := epochLoss / float64(len(embeddings)/config.BatchSize+1)
		lossHistory = append(lossHistory, avgLoss)

		if progressFn != nil {
			progressFn(epoch+1, avgLoss)
		}
	}

	// Record training trajectory
	trajectory := neural.NewTrajectory()
	trajectory.AddStep(*neural.NewTrajectoryStep(neural.StepTypeObservation, fmt.Sprintf("Training %d samples", len(data.Texts))))
	trajAction := fmt.Sprintf("Ran %d epochs with batch size %d", config.Epochs, config.BatchSize)
	if e.enableLoRA {
		trajAction += " [LoRA enabled]"
	}
	if e.enableEWC {
		trajAction += " [EWC enabled]"
	}
	trajectory.AddStep(*neural.NewTrajectoryStep(neural.StepTypeAction, trajAction))
	trajectory.AddStep(*neural.NewTrajectoryStep(neural.StepTypeResult, fmt.Sprintf("Generated %d patterns", len(patterns))))
	trajectory.SetVerdict("success")
	e.trajectories = append(e.trajectories, trajectory)

	finalLoss := 0.0
	if len(lossHistory) > 0 {
		finalLoss = lossHistory[len(lossHistory)-1]
	}

	metrics := &neural.TrainingMetrics{
		Epochs:         config.Epochs,
		FinalLoss:      finalLoss,
		TotalPatterns:  len(patterns),
		Adaptations:    totalAdaptations,
		TrainingTimeMs: time.Since(startTime).Milliseconds(),
		LossHistory:    lossHistory,
		PatternType:    config.PatternType,
		LearningRate:   config.LearningRate,
	}

	return metrics, patterns, nil
}

// float32ToFloat64 converts a float32 slice to float64.
func float32ToFloat64(input []float32) []float64 {
	output := make([]float64, len(input))
	for i, v := range input {
		output[i] = float64(v)
	}
	return output
}

// float64ToFloat32 converts a float64 slice to float32.
func float64ToFloat32(input []float64) []float32 {
	output := make([]float32, len(input))
	for i, v := range input {
		output[i] = float32(v)
	}
	return output
}

// computeInfoNCELoss computes InfoNCE (Noise Contrastive Estimation) loss.
// This is a simplified version of contrastive learning loss.
func (e *TrainingEngine) computeInfoNCELoss(embeddings [][]float32) float64 {
	if len(embeddings) < 2 {
		return 0.0
	}

	temperature := 0.07 // Standard temperature for InfoNCE
	totalLoss := 0.0

	for i, anchor := range embeddings {
		// Positive: similar embeddings (next in sequence for simplicity)
		positiveIdx := (i + 1) % len(embeddings)
		positive := embeddings[positiveIdx]

		// Compute similarity scores
		posSim := infraNeural.CosineSimilarity(anchor, positive) / temperature

		// Negatives: all other embeddings
		negSum := 0.0
		for j, neg := range embeddings {
			if j != i && j != positiveIdx {
				negSim := infraNeural.CosineSimilarity(anchor, neg) / temperature
				negSum += math.Exp(negSim)
			}
		}

		// InfoNCE loss: -log(exp(pos) / (exp(pos) + sum(exp(neg))))
		if negSum > 0 {
			loss := -math.Log(math.Exp(posSim) / (math.Exp(posSim) + negSum))
			totalLoss += loss
		}
	}

	return totalLoss / float64(len(embeddings))
}

// computeGradient computes a pseudo-gradient based on loss.
func (e *TrainingEngine) computeGradient(embedding []float32, loss float64) []float32 {
	gradient := make([]float32, len(embedding))
	
	// Simple gradient approximation based on loss magnitude
	scale := float32(loss * 0.01) // Scale gradient by loss
	
	for i := range gradient {
		// Add small random perturbation weighted by loss
		gradient[i] = scale * (e.rng.Float32()*2 - 1)
	}
	
	return gradient
}

// applyGradient applies gradient update with learning rate.
func (e *TrainingEngine) applyGradient(embedding []float32, gradient []float32, lr float64) []float32 {
	result := make([]float32, len(embedding))
	
	for i := range embedding {
		result[i] = embedding[i] - float32(lr)*gradient[i]
	}
	
	// Renormalize
	var sum float64
	for _, v := range result {
		sum += float64(v * v)
	}
	
	if sum > 0 {
		norm := float32(math.Sqrt(sum))
		for i := range result {
			result[i] /= norm
		}
	}
	
	return result
}

// LearnFromOutcome learns from an outcome and updates patterns.
func (e *TrainingEngine) LearnFromOutcome(content string, success bool) *neural.Pattern {
	embedding := e.generator.Generate(content)
	pattern := neural.NewPattern(e.config.PatternType, content, embedding)
	
	if success {
		pattern.Confidence = 0.8
	} else {
		pattern.Confidence = 0.3
	}
	
	// Record trajectory
	trajectory := neural.NewTrajectory()
	trajectory.AddStep(*neural.NewTrajectoryStep(neural.StepTypeObservation, content))
	verdict := "failure"
	if success {
		verdict = "success"
	}
	trajectory.SetVerdict(verdict)
	
	e.mu.Lock()
	e.trajectories = append(e.trajectories, trajectory)
	e.mu.Unlock()
	
	return pattern
}

// GetTrajectories returns recorded training trajectories.
func (e *TrainingEngine) GetTrajectories() []*neural.Trajectory {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	result := make([]*neural.Trajectory, len(e.trajectories))
	copy(result, e.trajectories)
	return result
}

// ClearTrajectories clears all recorded trajectories.
func (e *TrainingEngine) ClearTrajectories() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.trajectories = make([]*neural.Trajectory, 0)
}

// GenerateSyntheticData generates synthetic training data for a pattern type.
func (e *TrainingEngine) GenerateSyntheticData(patternType string, count int) TrainingData {
	texts := make([]string, count)
	labels := make([]string, count)
	
	templates := map[string][]string{
		string(neural.PatternTypeCoordination): {
			"coordinate task assignment for %s",
			"delegate responsibility to agent %s",
			"synchronize work between %s and %s",
			"establish consensus on %s",
			"orchestrate workflow for %s",
		},
		string(neural.PatternTypeOptimization): {
			"optimize performance of %s",
			"reduce latency in %s",
			"improve throughput for %s",
			"minimize resource usage in %s",
			"enhance efficiency of %s",
		},
		string(neural.PatternTypePrediction): {
			"predict outcome for %s",
			"forecast behavior of %s",
			"estimate probability of %s",
			"anticipate changes in %s",
			"project future state of %s",
		},
		string(neural.PatternTypeSecurity): {
			"validate security of %s",
			"detect threats in %s",
			"enforce access control for %s",
			"audit permissions on %s",
			"protect data integrity of %s",
		},
		string(neural.PatternTypeTesting): {
			"test functionality of %s",
			"verify behavior of %s",
			"validate output of %s",
			"check edge cases in %s",
			"assert correctness of %s",
		},
	}
	
	entities := []string{"agent", "task", "workflow", "module", "service", "component", "system", "pipeline"}
	
	tmpl, ok := templates[patternType]
	if !ok {
		tmpl = templates[string(neural.PatternTypeCoordination)]
	}
	
	for i := 0; i < count; i++ {
		template := tmpl[i%len(tmpl)]
		entity := entities[i%len(entities)]
		texts[i] = fmt.Sprintf(template, entity)
		labels[i] = patternType
	}
	
	return TrainingData{Texts: texts, Labels: labels}
}
