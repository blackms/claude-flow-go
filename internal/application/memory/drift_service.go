// Package memory provides application services for memory management.
package memory

import (
	"context"
	"sync"
	"time"

	domainMemory "github.com/anthropics/claude-flow-go/internal/domain/memory"
	infraMemory "github.com/anthropics/claude-flow-go/internal/infrastructure/memory"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// DriftService provides drift detection and management.
type DriftService struct {
	mu            sync.RWMutex
	detector      *infraMemory.DriftDetector
	backend       shared.MemoryBackend
	config        DriftServiceConfig
	running       bool
	stopChan      chan struct{}
	handlers      []domainMemory.DriftHandler
	alertHistory  []*domainMemory.DriftAlert
}

// DriftServiceConfig configures the drift service.
type DriftServiceConfig struct {
	// DriftConfig is the drift detection configuration.
	DriftConfig domainMemory.DriftConfig `json:"driftConfig"`

	// CheckIntervalMs is the drift check interval.
	CheckIntervalMs int64 `json:"checkIntervalMs"`

	// BaselineRefreshMs is the baseline refresh interval.
	BaselineRefreshMs int64 `json:"baselineRefreshMs"`

	// MaxAlertHistory is the maximum alert history size.
	MaxAlertHistory int `json:"maxAlertHistory"`

	// AutoRemediate enables automatic remediation.
	AutoRemediate bool `json:"autoRemediate"`
}

// DefaultDriftServiceConfig returns the default configuration.
func DefaultDriftServiceConfig() DriftServiceConfig {
	return DriftServiceConfig{
		DriftConfig:       domainMemory.DefaultDriftConfig(),
		CheckIntervalMs:   60 * 60 * 1000, // 1 hour
		BaselineRefreshMs: 24 * 60 * 60 * 1000, // 24 hours
		MaxAlertHistory:   100,
		AutoRemediate:     false,
	}
}

// NewDriftService creates a new drift service.
func NewDriftService(backend shared.MemoryBackend, config DriftServiceConfig) *DriftService {
	return &DriftService{
		detector:     infraMemory.NewDriftDetector(config.DriftConfig),
		backend:      backend,
		config:       config,
		handlers:     make([]domainMemory.DriftHandler, 0),
		alertHistory: make([]*domainMemory.DriftAlert, 0),
	}
}

// Start starts the drift monitoring service.
func (s *DriftService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	// Initialize baseline
	if err := s.refreshBaseline(ctx); err != nil {
		return err
	}

	// Start background monitoring
	go s.monitorLoop(ctx)

	return nil
}

// Stop stops the drift monitoring service.
func (s *DriftService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)
}

// monitorLoop runs the drift monitoring loop.
func (s *DriftService) monitorLoop(ctx context.Context) {
	checkTicker := time.NewTicker(time.Duration(s.config.CheckIntervalMs) * time.Millisecond)
	refreshTicker := time.NewTicker(time.Duration(s.config.BaselineRefreshMs) * time.Millisecond)

	defer checkTicker.Stop()
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-checkTicker.C:
			s.checkDrift(ctx)
		case <-refreshTicker.C:
			s.refreshBaseline(ctx)
		}
	}
}

// checkDrift performs a drift check.
func (s *DriftService) checkDrift(ctx context.Context) {
	// Get recent memories
	memories, err := s.backend.Query(shared.MemoryQuery{
		Limit: 1000,
	})
	if err != nil {
		return
	}

	// Check for drift
	status, alerts := s.detector.CheckDrift(memories)
	if status == nil {
		return
	}

	// Handle alerts
	if len(alerts) > 0 {
		s.mu.Lock()
		s.alertHistory = append(s.alertHistory, alerts...)
		if len(s.alertHistory) > s.config.MaxAlertHistory {
			s.alertHistory = s.alertHistory[len(s.alertHistory)-s.config.MaxAlertHistory:]
		}
		handlers := s.handlers
		s.mu.Unlock()

		// Notify handlers
		for _, alert := range alerts {
			for _, handler := range handlers {
				go handler(alert)
			}

			// Auto-remediate if enabled
			if s.config.AutoRemediate {
				s.remediate(ctx, alert)
			}
		}
	}
}

// refreshBaseline refreshes the baseline embeddings.
func (s *DriftService) refreshBaseline(ctx context.Context) error {
	// Get memories for baseline
	memories, err := s.backend.Query(shared.MemoryQuery{
		Limit: 10000,
	})
	if err != nil {
		return err
	}

	s.detector.SetBaseline(memories)
	return nil
}

// remediate performs automatic remediation for drift.
func (s *DriftService) remediate(ctx context.Context, alert *domainMemory.DriftAlert) {
	switch alert.Severity {
	case domainMemory.DriftSeverityHigh, domainMemory.DriftSeverityCritical:
		// Refresh baseline for high severity
		s.refreshBaseline(ctx)
	default:
		// No automatic action for lower severity
	}
}

// CheckDriftNow performs an immediate drift check.
func (s *DriftService) CheckDriftNow(ctx context.Context) (*domainMemory.DriftStatus, []*domainMemory.DriftAlert, error) {
	memories, err := s.backend.Query(shared.MemoryQuery{
		Limit: 1000,
	})
	if err != nil {
		return nil, nil, err
	}

	status, alerts := s.detector.CheckDrift(memories)
	return status, alerts, nil
}

// GetStatus returns the current drift status.
func (s *DriftService) GetStatus(ctx context.Context) (*domainMemory.DriftStatus, error) {
	status, _, err := s.CheckDriftNow(ctx)
	return status, err
}

// GetAlerts returns unresolved alerts.
func (s *DriftService) GetAlerts() []*domainMemory.DriftAlert {
	return s.detector.GetAlerts()
}

// GetAlertHistory returns alert history.
func (s *DriftService) GetAlertHistory() []*domainMemory.DriftAlert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]*domainMemory.DriftAlert, len(s.alertHistory))
	copy(history, s.alertHistory)
	return history
}

// AcknowledgeAlert acknowledges an alert.
func (s *DriftService) AcknowledgeAlert(alertID string) bool {
	return s.detector.AcknowledgeAlert(alertID)
}

// ResolveAlert resolves an alert.
func (s *DriftService) ResolveAlert(alertID string) bool {
	return s.detector.ResolveAlert(alertID)
}

// GetBaseline returns the current baseline.
func (s *DriftService) GetBaseline() *domainMemory.EmbeddingBaseline {
	return s.detector.GetBaseline()
}

// RefreshBaseline manually refreshes the baseline.
func (s *DriftService) RefreshBaseline(ctx context.Context) error {
	return s.refreshBaseline(ctx)
}

// GetHistory returns drift history.
func (s *DriftService) GetHistory() *domainMemory.DriftHistory {
	return s.detector.GetHistory()
}

// AddHandler adds a drift alert handler.
func (s *DriftService) AddHandler(handler domainMemory.DriftHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, handler)
	s.detector.AddHandler(handler)
}

// GetMetrics returns drift metrics.
func (s *DriftService) GetMetrics(ctx context.Context) (*domainMemory.DriftMetrics, error) {
	memories, err := s.backend.Query(shared.MemoryQuery{
		Limit: 1000,
	})
	if err != nil {
		return nil, err
	}

	return s.detector.DetectDrift(memories), nil
}

// IsRunning returns true if the service is running.
func (s *DriftService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetConfig returns the service configuration.
func (s *DriftService) GetConfig() DriftServiceConfig {
	return s.config
}
