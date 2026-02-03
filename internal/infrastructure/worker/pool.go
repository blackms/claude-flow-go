// Package worker provides worker infrastructure.
package worker

import (
	"fmt"
	"sync"
	"time"

	domainWorker "github.com/anthropics/claude-flow-go/internal/domain/worker"
)

// PoolManager manages a pool of workers.
type PoolManager struct {
	mu sync.RWMutex

	// Pool configuration
	config domainWorker.WorkerPool

	// Dispatcher reference
	dispatcher *WorkerDispatchService

	// Last scale action
	lastScaleUp   *time.Time
	lastScaleDown *time.Time

	// Scale cooldown
	scaleCooldown time.Duration
}

// PoolConfig contains pool configuration options.
type PoolConfig struct {
	// MinWorkers is the minimum number of workers.
	MinWorkers int

	// MaxWorkers is the maximum number of workers.
	MaxWorkers int

	// InitialSize is the initial pool size.
	InitialSize int

	// AutoScale enables auto-scaling.
	AutoScale bool

	// ScaleUpThreshold is the load threshold for scaling up (0-1).
	ScaleUpThreshold float64

	// ScaleDownThreshold is the load threshold for scaling down (0-1).
	ScaleDownThreshold float64

	// ScaleCooldown is the cooldown between scale actions.
	ScaleCooldown time.Duration
}

// DefaultPoolConfig returns the default pool configuration.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MinWorkers:         2,
		MaxWorkers:         10,
		InitialSize:        4,
		AutoScale:          true,
		ScaleUpThreshold:   0.8,
		ScaleDownThreshold: 0.2,
		ScaleCooldown:      30 * time.Second,
	}
}

// NewPoolManager creates a new pool manager.
func NewPoolManager(dispatcher *WorkerDispatchService, config PoolConfig) *PoolManager {
	return &PoolManager{
		config: domainWorker.WorkerPool{
			Size:               config.InitialSize,
			MinWorkers:         config.MinWorkers,
			MaxWorkers:         config.MaxWorkers,
			ActiveWorkers:      0,
			IdleWorkers:        config.InitialSize,
			PendingTasks:       0,
			AutoScale:          config.AutoScale,
			ScaleUpThreshold:   config.ScaleUpThreshold,
			ScaleDownThreshold: config.ScaleDownThreshold,
		},
		dispatcher:    dispatcher,
		scaleCooldown: config.ScaleCooldown,
	}
}

// GetPoolInfo returns the current pool status.
func (p *PoolManager) GetPoolInfo() domainWorker.WorkerPool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Update active/idle counts from dispatcher
	pool := p.config
	if p.dispatcher != nil {
		stats := p.dispatcher.GetStats()
		pool.ActiveWorkers = stats.Running
		pool.PendingTasks = stats.Pending
		pool.IdleWorkers = pool.Size - stats.Running
		if pool.IdleWorkers < 0 {
			pool.IdleWorkers = 0
		}
	}

	return pool
}

// SetPoolSize sets the pool size.
func (p *PoolManager) SetPoolSize(size int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if size < p.config.MinWorkers {
		return fmt.Errorf("size %d is below minimum %d", size, p.config.MinWorkers)
	}
	if size > p.config.MaxWorkers {
		return fmt.Errorf("size %d exceeds maximum %d", size, p.config.MaxWorkers)
	}

	p.config.Size = size
	p.config.IdleWorkers = size - p.config.ActiveWorkers
	if p.config.IdleWorkers < 0 {
		p.config.IdleWorkers = 0
	}

	return nil
}

// ScaleUp increases the pool size.
func (p *PoolManager) ScaleUp(delta int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if delta <= 0 {
		return fmt.Errorf("delta must be positive")
	}

	// Check cooldown
	if p.lastScaleUp != nil && time.Since(*p.lastScaleUp) < p.scaleCooldown {
		return fmt.Errorf("scale up cooldown not elapsed")
	}

	newSize := p.config.Size + delta
	if newSize > p.config.MaxWorkers {
		newSize = p.config.MaxWorkers
	}

	p.config.Size = newSize
	p.config.IdleWorkers += delta

	now := time.Now()
	p.lastScaleUp = &now

	return nil
}

// ScaleDown decreases the pool size.
func (p *PoolManager) ScaleDown(delta int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if delta <= 0 {
		return fmt.Errorf("delta must be positive")
	}

	// Check cooldown
	if p.lastScaleDown != nil && time.Since(*p.lastScaleDown) < p.scaleCooldown {
		return fmt.Errorf("scale down cooldown not elapsed")
	}

	newSize := p.config.Size - delta
	if newSize < p.config.MinWorkers {
		newSize = p.config.MinWorkers
	}

	// Cannot scale below active workers
	if p.dispatcher != nil {
		stats := p.dispatcher.GetStats()
		if newSize < stats.Running {
			return fmt.Errorf("cannot scale below active worker count %d", stats.Running)
		}
	}

	p.config.Size = newSize
	p.config.IdleWorkers = newSize - p.config.ActiveWorkers
	if p.config.IdleWorkers < 0 {
		p.config.IdleWorkers = 0
	}

	now := time.Now()
	p.lastScaleDown = &now

	return nil
}

// SetAutoScale enables or disables auto-scaling.
func (p *PoolManager) SetAutoScale(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.AutoScale = enabled
}

// SetScaleThresholds sets the scaling thresholds.
func (p *PoolManager) SetScaleThresholds(scaleUp, scaleDown float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if scaleUp < 0 || scaleUp > 1 {
		return fmt.Errorf("scale up threshold must be between 0 and 1")
	}
	if scaleDown < 0 || scaleDown > 1 {
		return fmt.Errorf("scale down threshold must be between 0 and 1")
	}
	if scaleDown >= scaleUp {
		return fmt.Errorf("scale down threshold must be less than scale up threshold")
	}

	p.config.ScaleUpThreshold = scaleUp
	p.config.ScaleDownThreshold = scaleDown

	return nil
}

// CheckAndScale checks if scaling is needed and performs it.
func (p *PoolManager) CheckAndScale() (string, error) {
	pool := p.GetPoolInfo()

	if !pool.AutoScale {
		return "auto-scale disabled", nil
	}

	if pool.ShouldScaleUp() {
		// Calculate how many workers to add
		delta := 1
		if pool.PendingTasks > 2 {
			delta = 2
		}
		if err := p.ScaleUp(delta); err != nil {
			return "", err
		}
		return fmt.Sprintf("scaled up by %d workers", delta), nil
	}

	if pool.ShouldScaleDown() {
		if err := p.ScaleDown(1); err != nil {
			return "", err
		}
		return "scaled down by 1 worker", nil
	}

	return "no scaling needed", nil
}

// GetActiveWorkers returns active workers.
func (p *PoolManager) GetActiveWorkers() []*domainWorker.WorkerInstance {
	if p.dispatcher == nil {
		return nil
	}

	status := domainWorker.StatusRunning
	return p.dispatcher.FilterWorkers("", nil, &status, 0)
}

// GetHealth returns the pool health status.
func (p *PoolManager) GetHealth() domainWorker.WorkerHealth {
	pool := p.GetPoolInfo()
	now := time.Now()

	health := domainWorker.WorkerHealth{
		Status:    domainWorker.HealthHealthy,
		CheckedAt: now,
		Diagnostics: map[string]interface{}{
			"size":          pool.Size,
			"activeWorkers": pool.ActiveWorkers,
			"idleWorkers":   pool.IdleWorkers,
			"pendingTasks":  pool.PendingTasks,
			"load":          pool.Load(),
			"autoScale":     pool.AutoScale,
		},
	}

	// Check for degraded conditions
	if pool.Load() > 0.9 {
		health.Status = domainWorker.HealthDegraded
		health.Message = "pool is under high load"
	}

	if pool.PendingTasks > pool.Size {
		health.Status = domainWorker.HealthDegraded
		health.Message = "many pending tasks waiting"
	}

	if pool.Size == 0 {
		health.Status = domainWorker.HealthUnhealthy
		health.Message = "no workers in pool"
	}

	if health.Status == domainWorker.HealthHealthy {
		health.Message = "pool is operating normally"
	}

	return health
}

// GetWorkerHealth returns health for a specific worker.
func (p *PoolManager) GetWorkerHealth(workerID string) domainWorker.WorkerHealth {
	now := time.Now()

	if p.dispatcher == nil {
		return domainWorker.WorkerHealth{
			WorkerID:  workerID,
			Status:    domainWorker.HealthUnknown,
			Message:   "dispatcher not available",
			CheckedAt: now,
		}
	}

	worker, ok := p.dispatcher.GetWorker(workerID)
	if !ok {
		return domainWorker.WorkerHealth{
			WorkerID:  workerID,
			Status:    domainWorker.HealthUnknown,
			Message:   "worker not found",
			CheckedAt: now,
		}
	}

	health := domainWorker.WorkerHealth{
		WorkerID:  workerID,
		Status:    domainWorker.HealthHealthy,
		Uptime:    time.Since(worker.StartedAt),
		CheckedAt: now,
		Diagnostics: map[string]interface{}{
			"trigger":   string(worker.Trigger),
			"status":    string(worker.Status),
			"progress":  worker.Progress,
			"phase":     worker.Phase,
			"sessionId": worker.SessionID,
		},
	}

	lastActivity := worker.StartedAt
	if worker.CompletedAt != nil {
		lastActivity = *worker.CompletedAt
	}
	health.LastActivity = &lastActivity

	// Check worker status
	switch worker.Status {
	case domainWorker.StatusRunning:
		// Check if worker might be stuck
		if worker.Progress < 10 && time.Since(worker.StartedAt) > 60*time.Second {
			health.Status = domainWorker.HealthDegraded
			health.Message = "worker may be stuck"
		} else {
			health.Message = fmt.Sprintf("worker running: %s (%d%%)", worker.Phase, worker.Progress)
		}

	case domainWorker.StatusCompleted:
		health.Message = "worker completed successfully"

	case domainWorker.StatusFailed:
		health.Status = domainWorker.HealthUnhealthy
		health.Message = fmt.Sprintf("worker failed: %s", worker.Error)

	case domainWorker.StatusCancelled:
		health.Status = domainWorker.HealthDegraded
		health.Message = "worker was cancelled"

	case domainWorker.StatusPending:
		health.Message = "worker pending execution"
	}

	return health
}

// Reset resets the pool to initial configuration.
func (p *PoolManager) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	defaultConfig := DefaultPoolConfig()
	p.config.Size = defaultConfig.InitialSize
	p.config.IdleWorkers = defaultConfig.InitialSize
	p.config.ActiveWorkers = 0
	p.config.PendingTasks = 0
	p.lastScaleUp = nil
	p.lastScaleDown = nil
}
