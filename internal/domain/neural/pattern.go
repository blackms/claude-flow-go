// Package neural provides neural pattern domain entities for the neural CLI commands.
package neural

import (
	"time"

	"github.com/google/uuid"
)

// PatternType represents the type of neural pattern.
type PatternType string

const (
	PatternTypeCoordination PatternType = "coordination"
	PatternTypeOptimization PatternType = "optimization"
	PatternTypePrediction   PatternType = "prediction"
	PatternTypeSecurity     PatternType = "security"
	PatternTypeTesting      PatternType = "testing"
)

// Pattern represents a learned neural pattern.
type Pattern struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Embedding  []float32 `json:"embedding"`
	Content    string    `json:"content"`
	Confidence float64   `json:"confidence"`
	UsageCount int       `json:"usageCount"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
}

// NewPattern creates a new pattern with generated ID.
func NewPattern(patternType string, content string, embedding []float32) *Pattern {
	now := time.Now()
	return &Pattern{
		ID:         uuid.New().String(),
		Type:       patternType,
		Content:    content,
		Embedding:  embedding,
		Confidence: 0.5,
		UsageCount: 0,
		CreatedAt:  now,
		LastUsedAt: now,
	}
}

// IncrementUsage increments the usage count and updates last used time.
func (p *Pattern) IncrementUsage() {
	p.UsageCount++
	p.LastUsedAt = time.Now()
}

// UpdateConfidence updates the pattern confidence score.
func (p *Pattern) UpdateConfidence(delta float64) {
	p.Confidence += delta
	if p.Confidence > 1.0 {
		p.Confidence = 1.0
	}
	if p.Confidence < 0.0 {
		p.Confidence = 0.0
	}
}

// StepType represents the type of a trajectory step.
type StepType string

const (
	StepTypeObservation StepType = "observation"
	StepTypeThought     StepType = "thought"
	StepTypeAction      StepType = "action"
	StepTypeResult      StepType = "result"
)

// TrajectoryStep represents a step in a learning trajectory.
type TrajectoryStep struct {
	Type      StepType               `json:"type"`
	Content   string                 `json:"content"`
	Embedding []float32              `json:"embedding,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewTrajectoryStep creates a new trajectory step.
func NewTrajectoryStep(stepType StepType, content string) *TrajectoryStep {
	return &TrajectoryStep{
		Type:      stepType,
		Content:   content,
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// Trajectory represents a complete learning trajectory.
type Trajectory struct {
	ID        string           `json:"id"`
	Steps     []TrajectoryStep `json:"steps"`
	Verdict   string           `json:"verdict"` // success, failure, partial
	CreatedAt time.Time        `json:"createdAt"`
}

// NewTrajectory creates a new trajectory.
func NewTrajectory() *Trajectory {
	return &Trajectory{
		ID:        uuid.New().String(),
		Steps:     make([]TrajectoryStep, 0),
		CreatedAt: time.Now(),
	}
}

// AddStep adds a step to the trajectory.
func (t *Trajectory) AddStep(step TrajectoryStep) {
	t.Steps = append(t.Steps, step)
}

// SetVerdict sets the final verdict of the trajectory.
func (t *Trajectory) SetVerdict(verdict string) {
	t.Verdict = verdict
}
