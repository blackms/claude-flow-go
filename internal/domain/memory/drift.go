// Package memory provides the Memory domain entity and related types.
package memory

import (
	"time"
)

// DriftType represents the type of drift detected.
type DriftType string

const (
	// DriftTypeEmbedding indicates embedding distribution drift.
	DriftTypeEmbedding DriftType = "embedding"
	// DriftTypeStatistical indicates statistical drift (data distribution).
	DriftTypeStatistical DriftType = "statistical"
	// DriftTypeSemantic indicates semantic meaning drift.
	DriftTypeSemantic DriftType = "semantic"
	// DriftTypeConcept indicates concept/topic drift.
	DriftTypeConcept DriftType = "concept"
)

// DriftSeverity represents the severity of drift.
type DriftSeverity string

const (
	// DriftSeverityNone indicates no drift detected.
	DriftSeverityNone DriftSeverity = "none"
	// DriftSeverityLow indicates minor drift.
	DriftSeverityLow DriftSeverity = "low"
	// DriftSeverityMedium indicates moderate drift requiring attention.
	DriftSeverityMedium DriftSeverity = "medium"
	// DriftSeverityHigh indicates significant drift requiring action.
	DriftSeverityHigh DriftSeverity = "high"
	// DriftSeverityCritical indicates critical drift requiring immediate action.
	DriftSeverityCritical DriftSeverity = "critical"
)

// DriftStatus contains the current drift status.
type DriftStatus struct {
	// Type is the drift type being tracked.
	Type DriftType `json:"type"`

	// CurrentLevel is the current drift level (0.0 to 1.0).
	CurrentLevel float64 `json:"currentLevel"`

	// Threshold is the threshold for alerting.
	Threshold float64 `json:"threshold"`

	// Severity is the current severity level.
	Severity DriftSeverity `json:"severity"`

	// LastCheckAt is when the last check was performed.
	LastCheckAt time.Time `json:"lastCheckAt"`

	// WindowStart is the start of the monitoring window.
	WindowStart time.Time `json:"windowStart"`

	// WindowEnd is the end of the monitoring window.
	WindowEnd time.Time `json:"windowEnd"`

	// SampleCount is the number of samples analyzed.
	SampleCount int `json:"sampleCount"`

	// BaselineSampleCount is the baseline sample count.
	BaselineSampleCount int `json:"baselineSampleCount"`

	// IsHealthy indicates if drift is within acceptable bounds.
	IsHealthy bool `json:"isHealthy"`
}

// DriftAlert represents a drift alert.
type DriftAlert struct {
	// ID is the unique alert identifier.
	ID string `json:"id"`

	// Type is the drift type that triggered the alert.
	Type DriftType `json:"type"`

	// Severity is the alert severity.
	Severity DriftSeverity `json:"severity"`

	// DriftLevel is the detected drift level.
	DriftLevel float64 `json:"driftLevel"`

	// Threshold is the threshold that was exceeded.
	Threshold float64 `json:"threshold"`

	// AffectedMemories is the list of affected memory IDs.
	AffectedMemories []string `json:"affectedMemories,omitempty"`

	// AffectedCount is the number of affected memories.
	AffectedCount int `json:"affectedCount"`

	// Recommendation is the recommended action.
	Recommendation string `json:"recommendation"`

	// CreatedAt is when the alert was created.
	CreatedAt time.Time `json:"createdAt"`

	// AcknowledgedAt is when the alert was acknowledged.
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`

	// ResolvedAt is when the alert was resolved.
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

// IsResolved returns true if the alert is resolved.
func (a *DriftAlert) IsResolved() bool {
	return a.ResolvedAt != nil
}

// IsAcknowledged returns true if the alert is acknowledged.
func (a *DriftAlert) IsAcknowledged() bool {
	return a.AcknowledgedAt != nil
}

// DriftConfig configures drift detection.
type DriftConfig struct {
	// Enabled enables drift detection.
	Enabled bool `json:"enabled"`

	// Types specifies which drift types to monitor.
	Types []DriftType `json:"types,omitempty"`

	// WindowSizeMs is the monitoring window size in milliseconds.
	WindowSizeMs int64 `json:"windowSizeMs,omitempty"`

	// CheckIntervalMs is the check interval in milliseconds.
	CheckIntervalMs int64 `json:"checkIntervalMs,omitempty"`

	// Thresholds maps drift types to their thresholds.
	Thresholds map[DriftType]float64 `json:"thresholds,omitempty"`

	// MinSamples is the minimum samples required for detection.
	MinSamples int `json:"minSamples,omitempty"`

	// AutoReindex triggers automatic reindexing on drift.
	AutoReindex bool `json:"autoReindex,omitempty"`

	// AlertOnDrift enables alerting on drift detection.
	AlertOnDrift bool `json:"alertOnDrift,omitempty"`
}

// DefaultDriftConfig returns the default drift configuration.
func DefaultDriftConfig() DriftConfig {
	return DriftConfig{
		Enabled: true,
		Types: []DriftType{
			DriftTypeEmbedding,
			DriftTypeStatistical,
		},
		WindowSizeMs:    24 * 60 * 60 * 1000, // 24 hours
		CheckIntervalMs: 60 * 60 * 1000,       // 1 hour
		Thresholds: map[DriftType]float64{
			DriftTypeEmbedding:   0.15,
			DriftTypeStatistical: 0.20,
			DriftTypeSemantic:    0.25,
			DriftTypeConcept:     0.30,
		},
		MinSamples:   100,
		AutoReindex:  false,
		AlertOnDrift: true,
	}
}

// GetThreshold returns the threshold for a drift type.
func (c DriftConfig) GetThreshold(driftType DriftType) float64 {
	if threshold, ok := c.Thresholds[driftType]; ok {
		return threshold
	}
	return 0.20 // Default threshold
}

// DriftMetrics contains drift detection metrics.
type DriftMetrics struct {
	// EmbeddingDrift is the embedding distribution drift.
	EmbeddingDrift float64 `json:"embeddingDrift"`

	// StatisticalDrift is the statistical drift (KL divergence).
	StatisticalDrift float64 `json:"statisticalDrift"`

	// SemanticDrift is the semantic meaning drift.
	SemanticDrift float64 `json:"semanticDrift"`

	// ConceptDrift is the concept/topic drift.
	ConceptDrift float64 `json:"conceptDrift"`

	// CentroidShift is the embedding centroid shift distance.
	CentroidShift float64 `json:"centroidShift"`

	// VarianceChange is the variance change percentage.
	VarianceChange float64 `json:"varianceChange"`

	// DistributionDistance is the distribution distance (Wasserstein).
	DistributionDistance float64 `json:"distributionDistance"`

	// MeasuredAt is when the metrics were measured.
	MeasuredAt time.Time `json:"measuredAt"`
}

// DriftHistory contains historical drift data.
type DriftHistory struct {
	// Entries is the list of historical metrics.
	Entries []DriftMetrics `json:"entries"`

	// AlertCount is the total alert count.
	AlertCount int `json:"alertCount"`

	// ReindexCount is the number of reindexing operations.
	ReindexCount int `json:"reindexCount"`

	// LastReindexAt is when the last reindex occurred.
	LastReindexAt *time.Time `json:"lastReindexAt,omitempty"`
}

// EmbeddingBaseline stores baseline embedding statistics.
type EmbeddingBaseline struct {
	// Centroid is the centroid of embeddings.
	Centroid []float64 `json:"centroid"`

	// Variance is the variance per dimension.
	Variance []float64 `json:"variance"`

	// Mean is the mean per dimension.
	Mean []float64 `json:"mean"`

	// StdDev is the standard deviation per dimension.
	StdDev []float64 `json:"stdDev"`

	// SampleCount is the number of samples in baseline.
	SampleCount int `json:"sampleCount"`

	// CreatedAt is when the baseline was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the baseline was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// DriftHandler handles drift alerts.
type DriftHandler func(alert *DriftAlert) error
