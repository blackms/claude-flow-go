// Package coordinator provides swarm coordination functionality.
package coordinator

import (
	"context"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
	"github.com/google/uuid"
)

// HealthMonitor provides continuous health monitoring for agents and domains.
type HealthMonitor struct {
	agentHealth  map[string]*shared.AgentHealth
	domainHealth map[shared.AgentDomain]*shared.DomainHealth
	alerts       []shared.HealthAlert
	config       HealthMonitorConfig
	mu           sync.RWMutex

	// Background monitoring
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// HealthMonitorConfig holds configuration for the health monitor.
type HealthMonitorConfig struct {
	CheckInterval      time.Duration // How often to check health
	HeartbeatTimeout   time.Duration // How long before an agent is considered unhealthy
	LoadThreshold      float64       // Load threshold for warnings (0-1)
	ErrorRateThreshold float64       // Error rate threshold for warnings (0-1)
	MaxAlerts          int           // Maximum number of alerts to keep
}

// DefaultHealthMonitorConfig returns the default health monitor configuration.
func DefaultHealthMonitorConfig() HealthMonitorConfig {
	return HealthMonitorConfig{
		CheckInterval:      5 * time.Second,
		HeartbeatTimeout:   30 * time.Second,
		LoadThreshold:      0.8,
		ErrorRateThreshold: 0.2,
		MaxAlerts:          100,
	}
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor(config HealthMonitorConfig) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &HealthMonitor{
		agentHealth:  make(map[string]*shared.AgentHealth),
		domainHealth: make(map[shared.AgentDomain]*shared.DomainHealth),
		alerts:       make([]shared.HealthAlert, 0),
		config:       config,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins background health monitoring.
func (hm *HealthMonitor) Start() {
	hm.wg.Add(1)
	go hm.monitorLoop()
}

// Stop stops the health monitor.
func (hm *HealthMonitor) Stop() {
	hm.cancel()
	hm.wg.Wait()
}

// monitorLoop runs the background health monitoring loop.
func (hm *HealthMonitor) monitorLoop() {
	defer hm.wg.Done()

	ticker := time.NewTicker(hm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hm.ctx.Done():
			return
		case <-ticker.C:
			hm.checkAgentHealth()
		}
	}
}

// checkAgentHealth checks all agent health and generates alerts.
func (hm *HealthMonitor) checkAgentHealth() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	now := time.Now().UnixMilli()

	for agentID, health := range hm.agentHealth {
		// Check for heartbeat timeout
		if now-health.LastHeartbeat > hm.config.HeartbeatTimeout.Milliseconds() {
			health.IsAvailable = false
			health.HealthScore = 0.0
			hm.addAlertLocked(shared.HealthAlert{
				ID:        uuid.New().String(),
				Level:     shared.AlertLevelCritical,
				AgentID:   agentID,
				Message:   "Agent heartbeat timeout - agent may be unresponsive",
				Timestamp: now,
			})
		}

		// Check for high error rate
		if health.ErrorRate > hm.config.ErrorRateThreshold {
			hm.addAlertLocked(shared.HealthAlert{
				ID:        uuid.New().String(),
				Level:     shared.AlertLevelWarning,
				AgentID:   agentID,
				Message:   "High error rate detected",
				Timestamp: now,
			})
		}

		// Check for high load
		if health.CurrentLoad > hm.config.LoadThreshold {
			hm.addAlertLocked(shared.HealthAlert{
				ID:        uuid.New().String(),
				Level:     shared.AlertLevelWarning,
				AgentID:   agentID,
				Message:   "Agent load exceeds threshold",
				Timestamp: now,
			})
		}
	}
}

// addAlertLocked adds an alert (caller must hold lock).
func (hm *HealthMonitor) addAlertLocked(alert shared.HealthAlert) {
	hm.alerts = append(hm.alerts, alert)

	// Trim old alerts if over limit
	if len(hm.alerts) > hm.config.MaxAlerts {
		hm.alerts = hm.alerts[len(hm.alerts)-hm.config.MaxAlerts:]
	}
}

// RegisterAgent registers an agent for health monitoring.
func (hm *HealthMonitor) RegisterAgent(agentID string, domain shared.AgentDomain) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.agentHealth[agentID] = &shared.AgentHealth{
		AgentID:       agentID,
		HealthScore:   1.0,
		CurrentLoad:   0.0,
		TasksInQueue:  0,
		AvgResponseTime: 0,
		ErrorRate:     0.0,
		LastHeartbeat: time.Now().UnixMilli(),
		IsAvailable:   true,
	}
}

// UnregisterAgent removes an agent from health monitoring.
func (hm *HealthMonitor) UnregisterAgent(agentID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	delete(hm.agentHealth, agentID)
}

// RecordHeartbeat records a heartbeat from an agent.
func (hm *HealthMonitor) RecordHeartbeat(agentID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if health, ok := hm.agentHealth[agentID]; ok {
		health.LastHeartbeat = time.Now().UnixMilli()
		health.IsAvailable = true
	}
}

// UpdateAgentLoad updates the current load for an agent.
func (hm *HealthMonitor) UpdateAgentLoad(agentID string, load float64, tasksInQueue int) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if health, ok := hm.agentHealth[agentID]; ok {
		health.CurrentLoad = load
		health.TasksInQueue = tasksInQueue
		health.LastHeartbeat = time.Now().UnixMilli()
	}
}

// RecordTaskCompletion records a task completion for health tracking.
func (hm *HealthMonitor) RecordTaskCompletion(agentID string, duration int64, success bool) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	health, ok := hm.agentHealth[agentID]
	if !ok {
		return
	}

	// Update response time with exponential moving average
	alpha := 0.2 // Smoothing factor
	if health.AvgResponseTime == 0 {
		health.AvgResponseTime = duration
	} else {
		health.AvgResponseTime = int64(float64(health.AvgResponseTime)*(1-alpha) + float64(duration)*alpha)
	}

	// Update error rate with exponential moving average
	errorValue := 0.0
	if !success {
		errorValue = 1.0
	}
	health.ErrorRate = health.ErrorRate*(1-alpha) + errorValue*alpha

	// Update health score based on error rate and response time
	health.HealthScore = hm.calculateHealthScore(health)
	health.LastHeartbeat = time.Now().UnixMilli()
}

// calculateHealthScore calculates the overall health score for an agent.
func (hm *HealthMonitor) calculateHealthScore(health *shared.AgentHealth) float64 {
	score := 1.0

	// Penalize for high error rate
	if health.ErrorRate > 0.1 {
		score -= health.ErrorRate * 0.5
	}

	// Penalize for high load
	if health.CurrentLoad > 0.8 {
		score -= (health.CurrentLoad - 0.8) * 0.5
	}

	// Penalize for slow response time (>5s is considered slow)
	if health.AvgResponseTime > 5000 {
		penalty := float64(health.AvgResponseTime-5000) / 10000.0
		if penalty > 0.3 {
			penalty = 0.3
		}
		score -= penalty
	}

	// Clamp between 0 and 1
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// GetAgentHealth returns the health status of an agent.
func (hm *HealthMonitor) GetAgentHealth(agentID string) (*shared.AgentHealth, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	health, ok := hm.agentHealth[agentID]
	if !ok {
		return nil, false
	}

	// Return a copy
	copy := *health
	return &copy, true
}

// GetAllAgentHealth returns health status for all agents.
func (hm *HealthMonitor) GetAllAgentHealth() map[string]*shared.AgentHealth {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make(map[string]*shared.AgentHealth)
	for id, health := range hm.agentHealth {
		copy := *health
		result[id] = &copy
	}
	return result
}

// UpdateDomainHealth updates the health status of a domain.
func (hm *HealthMonitor) UpdateDomainHealth(domain shared.AgentDomain, health shared.DomainHealth) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.domainHealth[domain] = &health

	// Generate alerts for domain issues
	if health.HealthScore < 0.5 {
		hm.addAlertLocked(shared.HealthAlert{
			ID:        uuid.New().String(),
			Level:     shared.AlertLevelCritical,
			Domain:    domain,
			Message:   "Domain health critically low",
			Timestamp: time.Now().UnixMilli(),
		})
	}

	for _, bottleneck := range health.Bottlenecks {
		hm.addAlertLocked(shared.HealthAlert{
			ID:        uuid.New().String(),
			Level:     shared.AlertLevelWarning,
			Domain:    domain,
			Message:   "Bottleneck detected: " + bottleneck,
			Timestamp: time.Now().UnixMilli(),
		})
	}
}

// GetDomainHealth returns the health status of a domain.
func (hm *HealthMonitor) GetDomainHealth(domain shared.AgentDomain) (*shared.DomainHealth, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	health, ok := hm.domainHealth[domain]
	if !ok {
		return nil, false
	}

	copy := *health
	return &copy, true
}

// GetAllDomainHealth returns health status for all domains.
func (hm *HealthMonitor) GetAllDomainHealth() map[shared.AgentDomain]*shared.DomainHealth {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make(map[shared.AgentDomain]*shared.DomainHealth)
	for domain, health := range hm.domainHealth {
		copy := *health
		result[domain] = &copy
	}
	return result
}

// GetAlerts returns recent alerts.
func (hm *HealthMonitor) GetAlerts(limit int) []shared.HealthAlert {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if limit <= 0 || limit > len(hm.alerts) {
		limit = len(hm.alerts)
	}

	// Return most recent alerts
	start := len(hm.alerts) - limit
	result := make([]shared.HealthAlert, limit)
	copy(result, hm.alerts[start:])
	return result
}

// GetAlertsByLevel returns alerts filtered by level.
func (hm *HealthMonitor) GetAlertsByLevel(level shared.AlertLevel) []shared.HealthAlert {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make([]shared.HealthAlert, 0)
	for _, alert := range hm.alerts {
		if alert.Level == level {
			result = append(result, alert)
		}
	}
	return result
}

// ClearAlerts clears all alerts.
func (hm *HealthMonitor) ClearAlerts() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.alerts = make([]shared.HealthAlert, 0)
}

// DetectBottlenecks analyzes the system and returns detected bottlenecks.
func (hm *HealthMonitor) DetectBottlenecks() []BottleneckInfo {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	bottlenecks := make([]BottleneckInfo, 0)

	// Check for agent-level bottlenecks
	for agentID, health := range hm.agentHealth {
		if !health.IsAvailable {
			bottlenecks = append(bottlenecks, BottleneckInfo{
				Type:        BottleneckTypeAgent,
				ID:          agentID,
				Severity:    SeverityCritical,
				Description: "Agent is unavailable",
				Suggestion:  "Check agent status and restart if necessary",
			})
		} else if health.CurrentLoad > 0.9 {
			bottlenecks = append(bottlenecks, BottleneckInfo{
				Type:        BottleneckTypeAgent,
				ID:          agentID,
				Severity:    SeverityHigh,
				Description: "Agent is overloaded",
				Suggestion:  "Redistribute tasks or spawn additional agents",
			})
		} else if health.TasksInQueue > 10 {
			bottlenecks = append(bottlenecks, BottleneckInfo{
				Type:        BottleneckTypeAgent,
				ID:          agentID,
				Severity:    SeverityMedium,
				Description: "Agent has high task queue",
				Suggestion:  "Consider task redistribution",
			})
		}
	}

	// Check for domain-level bottlenecks
	for domain, health := range hm.domainHealth {
		if health.ActiveAgents == 0 {
			bottlenecks = append(bottlenecks, BottleneckInfo{
				Type:        BottleneckTypeDomain,
				ID:          string(domain),
				Severity:    SeverityCritical,
				Description: "Domain has no active agents",
				Suggestion:  "Spawn agents in this domain",
			})
		} else if health.AvgLoad > 0.85 {
			bottlenecks = append(bottlenecks, BottleneckInfo{
				Type:        BottleneckTypeDomain,
				ID:          string(domain),
				Severity:    SeverityHigh,
				Description: "Domain is overloaded",
				Suggestion:  "Scale up domain or redistribute workload",
			})
		}
	}

	return bottlenecks
}

// BottleneckInfo represents information about a detected bottleneck.
type BottleneckInfo struct {
	Type        BottleneckType
	ID          string
	Severity    Severity
	Description string
	Suggestion  string
}

// BottleneckType represents the type of bottleneck.
type BottleneckType string

const (
	BottleneckTypeAgent  BottleneckType = "agent"
	BottleneckTypeDomain BottleneckType = "domain"
	BottleneckTypeQueue  BottleneckType = "queue"
)

// Severity represents the severity of a bottleneck.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)
