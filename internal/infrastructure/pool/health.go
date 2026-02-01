// Package pool provides agent pool management with per-type pools and scaling.
package pool

import (
	"context"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// TypeMetrics holds health metrics for an agent type.
type TypeMetrics struct {
	AgentType       shared.AgentType `json:"agentType"`
	TotalTasks      int64            `json:"totalTasks"`
	SuccessfulTasks int64            `json:"successfulTasks"`
	FailedTasks     int64            `json:"failedTasks"`
	SuccessRate     float64          `json:"successRate"`
	AvgLatencyMs    float64          `json:"avgLatencyMs"`
	MinLatencyMs    float64          `json:"minLatencyMs"`
	MaxLatencyMs    float64          `json:"maxLatencyMs"`
	ActiveAgents    int              `json:"activeAgents"`
	IdleAgents      int              `json:"idleAgents"`
	BusyAgents      int              `json:"busyAgents"`
	PoolUtilization float64          `json:"poolUtilization"`
	LastUpdated     int64            `json:"lastUpdated"`
	HealthStatus    HealthStatus     `json:"healthStatus"`
}

// HealthStatus represents the health status of an agent type.
type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "healthy"
	HealthStatusDegraded HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown  HealthStatus = "unknown"
)

// HealthAlert represents a health alert for an agent type.
type HealthAlert struct {
	AgentType   shared.AgentType `json:"agentType"`
	AlertType   string           `json:"alertType"`
	Message     string           `json:"message"`
	Severity    string           `json:"severity"` // info, warning, critical
	Timestamp   int64            `json:"timestamp"`
	Resolved    bool             `json:"resolved"`
	ResolvedAt  int64            `json:"resolvedAt,omitempty"`
}

// HealthConfig holds configuration for health monitoring.
type HealthConfig struct {
	SuccessRateThreshold  float64       `json:"successRateThreshold"`  // Below this is degraded
	CriticalSuccessRate   float64       `json:"criticalSuccessRate"`   // Below this is unhealthy
	MaxLatencyMs          float64       `json:"maxLatencyMs"`          // Above this is degraded
	CriticalLatencyMs     float64       `json:"criticalLatencyMs"`     // Above this is unhealthy
	UtilizationThreshold  float64       `json:"utilizationThreshold"`  // Above this is warning
	CheckInterval         time.Duration `json:"checkInterval"`
	AlertRetentionPeriod  time.Duration `json:"alertRetentionPeriod"`
}

// DefaultHealthConfig returns the default health configuration.
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		SuccessRateThreshold: 0.9,  // 90% success rate
		CriticalSuccessRate:  0.7,  // 70% critical threshold
		MaxLatencyMs:         5000, // 5 seconds
		CriticalLatencyMs:    10000, // 10 seconds
		UtilizationThreshold: 0.8,  // 80% utilization
		CheckInterval:        10 * time.Second,
		AlertRetentionPeriod: 1 * time.Hour,
	}
}

// TypeHealthMonitor monitors health metrics per agent type.
type TypeHealthMonitor struct {
	mu      sync.RWMutex
	metrics map[shared.AgentType]*TypeMetrics
	alerts  []HealthAlert
	config  HealthConfig
	pools   *AgentPoolManager

	// Callbacks
	onAlert func(alert HealthAlert)

	// Background tasks
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewTypeHealthMonitor creates a new TypeHealthMonitor.
func NewTypeHealthMonitor(pools *AgentPoolManager, config HealthConfig) *TypeHealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &TypeHealthMonitor{
		metrics: make(map[shared.AgentType]*TypeMetrics),
		alerts:  make([]HealthAlert, 0),
		config:  config,
		pools:   pools,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// NewTypeHealthMonitorWithDefaults creates a TypeHealthMonitor with default config.
func NewTypeHealthMonitorWithDefaults(pools *AgentPoolManager) *TypeHealthMonitor {
	return NewTypeHealthMonitor(pools, DefaultHealthConfig())
}

// Initialize starts the health monitoring.
func (thm *TypeHealthMonitor) Initialize() error {
	thm.wg.Add(1)
	go thm.monitorLoop()
	return nil
}

// Shutdown stops the health monitoring.
func (thm *TypeHealthMonitor) Shutdown() error {
	thm.cancel()
	thm.wg.Wait()
	return nil
}

// RecordTaskCompletion records a task completion for an agent type.
func (thm *TypeHealthMonitor) RecordTaskCompletion(agentType shared.AgentType, success bool, latencyMs float64) {
	thm.mu.Lock()
	defer thm.mu.Unlock()

	metrics := thm.getOrCreateMetrics(agentType)

	metrics.TotalTasks++
	if success {
		metrics.SuccessfulTasks++
	} else {
		metrics.FailedTasks++
	}

	// Update success rate
	metrics.SuccessRate = float64(metrics.SuccessfulTasks) / float64(metrics.TotalTasks)

	// Update latency stats
	if metrics.MinLatencyMs == 0 || latencyMs < metrics.MinLatencyMs {
		metrics.MinLatencyMs = latencyMs
	}
	if latencyMs > metrics.MaxLatencyMs {
		metrics.MaxLatencyMs = latencyMs
	}

	// Incremental average
	n := float64(metrics.TotalTasks)
	metrics.AvgLatencyMs = (metrics.AvgLatencyMs*(n-1) + latencyMs) / n

	metrics.LastUpdated = shared.Now()

	// Check health status
	thm.updateHealthStatusLocked(agentType, metrics)
}

// getOrCreateMetrics gets or creates metrics for an agent type.
func (thm *TypeHealthMonitor) getOrCreateMetrics(agentType shared.AgentType) *TypeMetrics {
	metrics, exists := thm.metrics[agentType]
	if !exists {
		metrics = &TypeMetrics{
			AgentType:    agentType,
			HealthStatus: HealthStatusUnknown,
		}
		thm.metrics[agentType] = metrics
	}
	return metrics
}

// updateHealthStatusLocked updates health status (caller must hold lock).
func (thm *TypeHealthMonitor) updateHealthStatusLocked(agentType shared.AgentType, metrics *TypeMetrics) {
	oldStatus := metrics.HealthStatus
	newStatus := HealthStatusHealthy

	// Check success rate
	if metrics.TotalTasks > 10 { // Require minimum samples
		if metrics.SuccessRate < thm.config.CriticalSuccessRate {
			newStatus = HealthStatusUnhealthy
		} else if metrics.SuccessRate < thm.config.SuccessRateThreshold {
			newStatus = HealthStatusDegraded
		}
	}

	// Check latency
	if metrics.AvgLatencyMs > thm.config.CriticalLatencyMs {
		if newStatus != HealthStatusUnhealthy {
			newStatus = HealthStatusUnhealthy
		}
	} else if metrics.AvgLatencyMs > thm.config.MaxLatencyMs {
		if newStatus == HealthStatusHealthy {
			newStatus = HealthStatusDegraded
		}
	}

	metrics.HealthStatus = newStatus

	// Emit alert on status change
	if oldStatus != newStatus && oldStatus != HealthStatusUnknown {
		thm.emitAlertLocked(agentType, oldStatus, newStatus)
	}
}

// emitAlertLocked emits a health alert (caller must hold lock).
func (thm *TypeHealthMonitor) emitAlertLocked(agentType shared.AgentType, oldStatus, newStatus HealthStatus) {
	severity := "info"
	if newStatus == HealthStatusDegraded {
		severity = "warning"
	} else if newStatus == HealthStatusUnhealthy {
		severity = "critical"
	}

	alert := HealthAlert{
		AgentType: agentType,
		AlertType: "status_change",
		Message:   "Health status changed from " + string(oldStatus) + " to " + string(newStatus),
		Severity:  severity,
		Timestamp: shared.Now(),
		Resolved:  newStatus == HealthStatusHealthy,
	}

	if alert.Resolved {
		alert.ResolvedAt = shared.Now()
	}

	thm.alerts = append(thm.alerts, alert)

	// Trim old alerts
	thm.trimAlerts()

	if thm.onAlert != nil {
		go thm.onAlert(alert)
	}
}

// trimAlerts removes old alerts beyond retention period.
func (thm *TypeHealthMonitor) trimAlerts() {
	cutoff := shared.Now() - int64(thm.config.AlertRetentionPeriod.Milliseconds())
	newAlerts := make([]HealthAlert, 0, len(thm.alerts))

	for _, alert := range thm.alerts {
		if alert.Timestamp > cutoff {
			newAlerts = append(newAlerts, alert)
		}
	}

	thm.alerts = newAlerts
}

// GetMetrics returns metrics for an agent type.
func (thm *TypeHealthMonitor) GetMetrics(agentType shared.AgentType) *TypeMetrics {
	thm.mu.RLock()
	defer thm.mu.RUnlock()

	metrics, exists := thm.metrics[agentType]
	if !exists {
		return nil
	}

	// Return a copy
	copy := *metrics
	return &copy
}

// GetAllMetrics returns metrics for all agent types.
func (thm *TypeHealthMonitor) GetAllMetrics() []*TypeMetrics {
	thm.mu.RLock()
	defer thm.mu.RUnlock()

	result := make([]*TypeMetrics, 0, len(thm.metrics))
	for _, m := range thm.metrics {
		copy := *m
		result = append(result, &copy)
	}
	return result
}

// GetHealthStatus returns the health status for an agent type.
func (thm *TypeHealthMonitor) GetHealthStatus(agentType shared.AgentType) HealthStatus {
	thm.mu.RLock()
	defer thm.mu.RUnlock()

	metrics, exists := thm.metrics[agentType]
	if !exists {
		return HealthStatusUnknown
	}
	return metrics.HealthStatus
}

// GetAlerts returns recent alerts.
func (thm *TypeHealthMonitor) GetAlerts(limit int) []HealthAlert {
	thm.mu.RLock()
	defer thm.mu.RUnlock()

	if limit <= 0 || limit > len(thm.alerts) {
		limit = len(thm.alerts)
	}

	// Return most recent
	start := len(thm.alerts) - limit
	if start < 0 {
		start = 0
	}

	result := make([]HealthAlert, limit)
	copy(result, thm.alerts[start:])
	return result
}

// GetUnresolvedAlerts returns unresolved alerts.
func (thm *TypeHealthMonitor) GetUnresolvedAlerts() []HealthAlert {
	thm.mu.RLock()
	defer thm.mu.RUnlock()

	result := make([]HealthAlert, 0)
	for _, alert := range thm.alerts {
		if !alert.Resolved {
			result = append(result, alert)
		}
	}
	return result
}

// GetAlertsByType returns alerts for a specific agent type.
func (thm *TypeHealthMonitor) GetAlertsByType(agentType shared.AgentType) []HealthAlert {
	thm.mu.RLock()
	defer thm.mu.RUnlock()

	result := make([]HealthAlert, 0)
	for _, alert := range thm.alerts {
		if alert.AgentType == agentType {
			result = append(result, alert)
		}
	}
	return result
}

// SetOnAlert sets the callback for health alerts.
func (thm *TypeHealthMonitor) SetOnAlert(handler func(alert HealthAlert)) {
	thm.mu.Lock()
	defer thm.mu.Unlock()
	thm.onAlert = handler
}

// GetConfig returns the health configuration.
func (thm *TypeHealthMonitor) GetConfig() HealthConfig {
	thm.mu.RLock()
	defer thm.mu.RUnlock()
	return thm.config
}

// UpdateConfig updates the health configuration.
func (thm *TypeHealthMonitor) UpdateConfig(config HealthConfig) {
	thm.mu.Lock()
	defer thm.mu.Unlock()
	thm.config = config
}

// monitorLoop runs periodic health checks.
func (thm *TypeHealthMonitor) monitorLoop() {
	defer thm.wg.Done()

	ticker := time.NewTicker(thm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-thm.ctx.Done():
			return
		case <-ticker.C:
			thm.performHealthCheck()
		}
	}
}

// performHealthCheck performs a health check on all pools.
func (thm *TypeHealthMonitor) performHealthCheck() {
	if thm.pools == nil {
		return
	}

	allStats := thm.pools.GetAllPoolStats()

	thm.mu.Lock()
	defer thm.mu.Unlock()

	for _, stats := range allStats {
		metrics := thm.getOrCreateMetrics(stats.AgentType)

		// Update pool-related metrics
		metrics.ActiveAgents = stats.ActiveAgents
		metrics.IdleAgents = stats.IdleAgents
		metrics.BusyAgents = stats.BusyAgents
		metrics.PoolUtilization = stats.Utilization
		metrics.LastUpdated = shared.Now()

		// Check utilization
		if stats.Utilization > thm.config.UtilizationThreshold {
			// Emit utilization warning if not already unhealthy
			if metrics.HealthStatus == HealthStatusHealthy {
				thm.emitAlertLocked(stats.AgentType, HealthStatusHealthy, HealthStatusDegraded)
				metrics.HealthStatus = HealthStatusDegraded
			}
		}
	}
}

// GetOverallHealth returns the overall health of all agent types.
func (thm *TypeHealthMonitor) GetOverallHealth() HealthStatus {
	thm.mu.RLock()
	defer thm.mu.RUnlock()

	hasUnhealthy := false
	hasDegraded := false

	for _, metrics := range thm.metrics {
		switch metrics.HealthStatus {
		case HealthStatusUnhealthy:
			hasUnhealthy = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return HealthStatusUnhealthy
	}
	if hasDegraded {
		return HealthStatusDegraded
	}
	return HealthStatusHealthy
}

// Reset resets all metrics for an agent type.
func (thm *TypeHealthMonitor) Reset(agentType shared.AgentType) {
	thm.mu.Lock()
	defer thm.mu.Unlock()

	delete(thm.metrics, agentType)
}

// ResetAll resets all metrics.
func (thm *TypeHealthMonitor) ResetAll() {
	thm.mu.Lock()
	defer thm.mu.Unlock()

	thm.metrics = make(map[shared.AgentType]*TypeMetrics)
	thm.alerts = make([]HealthAlert, 0)
}
