// Package memory provides infrastructure for memory management.
package memory

import (
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	domainMemory "github.com/anthropics/claude-flow-go/internal/domain/memory"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// DriftDetector detects embedding drift in memories.
type DriftDetector struct {
	mu       sync.RWMutex
	config   domainMemory.DriftConfig
	baseline *domainMemory.EmbeddingBaseline
	history  *domainMemory.DriftHistory
	handlers []domainMemory.DriftHandler
	alerts   []*domainMemory.DriftAlert
}

// NewDriftDetector creates a new drift detector.
func NewDriftDetector(config domainMemory.DriftConfig) *DriftDetector {
	return &DriftDetector{
		config:   config,
		history:  &domainMemory.DriftHistory{},
		handlers: make([]domainMemory.DriftHandler, 0),
		alerts:   make([]*domainMemory.DriftAlert, 0),
	}
}

// SetBaseline sets the baseline from a set of memories.
func (d *DriftDetector) SetBaseline(memories []shared.Memory) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(memories) == 0 {
		return
	}

	// Filter memories with embeddings
	embeddings := make([][]float64, 0)
	for _, m := range memories {
		if len(m.Embedding) > 0 {
			embeddings = append(embeddings, m.Embedding)
		}
	}

	if len(embeddings) == 0 {
		return
	}

	dims := len(embeddings[0])
	baseline := &domainMemory.EmbeddingBaseline{
		Centroid:    make([]float64, dims),
		Variance:    make([]float64, dims),
		Mean:        make([]float64, dims),
		StdDev:      make([]float64, dims),
		SampleCount: len(embeddings),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Calculate mean per dimension
	for _, emb := range embeddings {
		for i, val := range emb {
			baseline.Mean[i] += val
		}
	}
	for i := range baseline.Mean {
		baseline.Mean[i] /= float64(len(embeddings))
		baseline.Centroid[i] = baseline.Mean[i]
	}

	// Calculate variance per dimension
	for _, emb := range embeddings {
		for i, val := range emb {
			diff := val - baseline.Mean[i]
			baseline.Variance[i] += diff * diff
		}
	}
	for i := range baseline.Variance {
		baseline.Variance[i] /= float64(len(embeddings))
		baseline.StdDev[i] = math.Sqrt(baseline.Variance[i])
	}

	d.baseline = baseline
}

// DetectDrift detects drift from current memories.
func (d *DriftDetector) DetectDrift(memories []shared.Memory) *domainMemory.DriftMetrics {
	d.mu.RLock()
	baseline := d.baseline
	config := d.config
	d.mu.RUnlock()

	if baseline == nil || len(memories) < config.MinSamples {
		return nil
	}

	// Filter memories with embeddings
	embeddings := make([][]float64, 0)
	for _, m := range memories {
		if len(m.Embedding) > 0 {
			embeddings = append(embeddings, m.Embedding)
		}
	}

	if len(embeddings) == 0 {
		return nil
	}

	dims := len(baseline.Centroid)
	if len(embeddings[0]) != dims {
		return nil
	}

	// Calculate current centroid
	currentCentroid := make([]float64, dims)
	for _, emb := range embeddings {
		for i, val := range emb {
			currentCentroid[i] += val
		}
	}
	for i := range currentCentroid {
		currentCentroid[i] /= float64(len(embeddings))
	}

	// Calculate current variance
	currentVariance := make([]float64, dims)
	for _, emb := range embeddings {
		for i, val := range emb {
			diff := val - currentCentroid[i]
			currentVariance[i] += diff * diff
		}
	}
	for i := range currentVariance {
		currentVariance[i] /= float64(len(embeddings))
	}

	// Calculate drift metrics
	metrics := &domainMemory.DriftMetrics{
		MeasuredAt: time.Now(),
	}

	// Centroid shift (Euclidean distance)
	var centroidShift float64
	for i := range baseline.Centroid {
		diff := currentCentroid[i] - baseline.Centroid[i]
		centroidShift += diff * diff
	}
	metrics.CentroidShift = math.Sqrt(centroidShift)

	// Variance change
	var varianceChange float64
	for i := range baseline.Variance {
		if baseline.Variance[i] > 0 {
			change := math.Abs(currentVariance[i]-baseline.Variance[i]) / baseline.Variance[i]
			varianceChange += change
		}
	}
	metrics.VarianceChange = varianceChange / float64(dims)

	// Embedding drift (normalized centroid shift)
	maxShift := math.Sqrt(float64(dims)) // Maximum possible shift
	metrics.EmbeddingDrift = metrics.CentroidShift / maxShift

	// Statistical drift (simplified KL divergence approximation)
	var klDivergence float64
	for i := range baseline.Variance {
		if baseline.Variance[i] > 0 && currentVariance[i] > 0 {
			meanDiff := currentCentroid[i] - baseline.Mean[i]
			kl := math.Log(math.Sqrt(currentVariance[i])/math.Sqrt(baseline.Variance[i])) +
				(baseline.Variance[i]+meanDiff*meanDiff)/(2*currentVariance[i]) - 0.5
			klDivergence += math.Abs(kl)
		}
	}
	metrics.StatisticalDrift = klDivergence / float64(dims)

	// Distribution distance (simplified Wasserstein)
	var wasserstein float64
	for i := range baseline.Mean {
		wasserstein += math.Abs(currentCentroid[i]-baseline.Mean[i]) +
			math.Abs(math.Sqrt(currentVariance[i])-baseline.StdDev[i])
	}
	metrics.DistributionDistance = wasserstein / float64(dims)

	// Record history
	d.mu.Lock()
	d.history.Entries = append(d.history.Entries, *metrics)
	if len(d.history.Entries) > 100 {
		d.history.Entries = d.history.Entries[1:]
	}
	d.mu.Unlock()

	return metrics
}

// CheckDrift checks for drift and generates alerts.
func (d *DriftDetector) CheckDrift(memories []shared.Memory) (*domainMemory.DriftStatus, []*domainMemory.DriftAlert) {
	metrics := d.DetectDrift(memories)
	if metrics == nil {
		return nil, nil
	}

	d.mu.RLock()
	config := d.config
	d.mu.RUnlock()

	alerts := make([]*domainMemory.DriftAlert, 0)

	// Check each drift type
	for _, driftType := range config.Types {
		var driftLevel float64
		switch driftType {
		case domainMemory.DriftTypeEmbedding:
			driftLevel = metrics.EmbeddingDrift
		case domainMemory.DriftTypeStatistical:
			driftLevel = metrics.StatisticalDrift
		case domainMemory.DriftTypeSemantic:
			driftLevel = metrics.SemanticDrift
		case domainMemory.DriftTypeConcept:
			driftLevel = metrics.ConceptDrift
		}

		threshold := config.GetThreshold(driftType)
		if driftLevel > threshold && config.AlertOnDrift {
			severity := d.getSeverity(driftLevel, threshold)
			alert := &domainMemory.DriftAlert{
				ID:             uuid.New().String(),
				Type:           driftType,
				Severity:       severity,
				DriftLevel:     driftLevel,
				Threshold:      threshold,
				AffectedCount:  len(memories),
				Recommendation: d.getRecommendation(driftType, severity),
				CreatedAt:      time.Now(),
			}
			alerts = append(alerts, alert)
		}
	}

	// Store alerts and notify handlers
	if len(alerts) > 0 {
		d.mu.Lock()
		d.alerts = append(d.alerts, alerts...)
		d.history.AlertCount += len(alerts)
		handlers := d.handlers
		d.mu.Unlock()

		for _, alert := range alerts {
			for _, handler := range handlers {
				go handler(alert)
			}
		}
	}

	status := &domainMemory.DriftStatus{
		Type:                domainMemory.DriftTypeEmbedding,
		CurrentLevel:        metrics.EmbeddingDrift,
		Threshold:           config.GetThreshold(domainMemory.DriftTypeEmbedding),
		Severity:            d.getSeverity(metrics.EmbeddingDrift, config.GetThreshold(domainMemory.DriftTypeEmbedding)),
		LastCheckAt:         time.Now(),
		SampleCount:         len(memories),
		BaselineSampleCount: d.baseline.SampleCount,
		IsHealthy:           metrics.EmbeddingDrift <= config.GetThreshold(domainMemory.DriftTypeEmbedding),
	}

	return status, alerts
}

// getSeverity determines severity from drift level and threshold.
func (d *DriftDetector) getSeverity(level, threshold float64) domainMemory.DriftSeverity {
	if level <= threshold {
		return domainMemory.DriftSeverityNone
	}

	ratio := level / threshold
	switch {
	case ratio < 1.25:
		return domainMemory.DriftSeverityLow
	case ratio < 1.5:
		return domainMemory.DriftSeverityMedium
	case ratio < 2.0:
		return domainMemory.DriftSeverityHigh
	default:
		return domainMemory.DriftSeverityCritical
	}
}

// getRecommendation provides a recommendation based on drift type and severity.
func (d *DriftDetector) getRecommendation(driftType domainMemory.DriftType, severity domainMemory.DriftSeverity) string {
	switch severity {
	case domainMemory.DriftSeverityLow:
		return "Monitor drift levels; no immediate action required"
	case domainMemory.DriftSeverityMedium:
		return "Consider updating baseline or reindexing affected memories"
	case domainMemory.DriftSeverityHigh:
		return "Recommend reindexing memories and updating embedding model"
	case domainMemory.DriftSeverityCritical:
		return "Immediate reindexing required; consider refreshing all embeddings"
	default:
		return ""
	}
}

// AddHandler adds a drift alert handler.
func (d *DriftDetector) AddHandler(handler domainMemory.DriftHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, handler)
}

// GetAlerts returns unresolved alerts.
func (d *DriftDetector) GetAlerts() []*domainMemory.DriftAlert {
	d.mu.RLock()
	defer d.mu.RUnlock()

	unresolved := make([]*domainMemory.DriftAlert, 0)
	for _, alert := range d.alerts {
		if !alert.IsResolved() {
			unresolved = append(unresolved, alert)
		}
	}
	return unresolved
}

// GetHistory returns drift history.
func (d *DriftDetector) GetHistory() *domainMemory.DriftHistory {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.history
}

// GetBaseline returns the current baseline.
func (d *DriftDetector) GetBaseline() *domainMemory.EmbeddingBaseline {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.baseline
}

// UpdateBaseline updates the baseline with new memories.
func (d *DriftDetector) UpdateBaseline(memories []shared.Memory) {
	d.SetBaseline(memories)
	d.mu.Lock()
	if d.baseline != nil {
		d.baseline.UpdatedAt = time.Now()
	}
	d.mu.Unlock()
}

// AcknowledgeAlert acknowledges an alert.
func (d *DriftDetector) AcknowledgeAlert(alertID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, alert := range d.alerts {
		if alert.ID == alertID && !alert.IsAcknowledged() {
			now := time.Now()
			alert.AcknowledgedAt = &now
			return true
		}
	}
	return false
}

// ResolveAlert resolves an alert.
func (d *DriftDetector) ResolveAlert(alertID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, alert := range d.alerts {
		if alert.ID == alertID && !alert.IsResolved() {
			now := time.Now()
			alert.ResolvedAt = &now
			return true
		}
	}
	return false
}
