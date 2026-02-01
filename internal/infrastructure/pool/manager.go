// Package pool provides agent pool management with per-type pools and scaling.
package pool

import (
	"context"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/agent"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// TypePoolConfig holds configuration for a type-specific agent pool.
type TypePoolConfig struct {
	MinAgents      int           `json:"minAgents"`
	MaxAgents      int           `json:"maxAgents"`
	WarmPoolSize   int           `json:"warmPoolSize"`
	ScaleUpDelay   time.Duration `json:"scaleUpDelay"`
	ScaleDownDelay time.Duration `json:"scaleDownDelay"`
	IdleTimeout    time.Duration `json:"idleTimeout"`
}

// DefaultTypePoolConfig returns the default pool configuration.
func DefaultTypePoolConfig() TypePoolConfig {
	return TypePoolConfig{
		MinAgents:      0,
		MaxAgents:      10,
		WarmPoolSize:   2,
		ScaleUpDelay:   100 * time.Millisecond,
		ScaleDownDelay: 30 * time.Second,
		IdleTimeout:    5 * time.Minute,
	}
}

// TypePool manages agents of a specific type.
type TypePool struct {
	mu            sync.RWMutex
	agentType     shared.AgentType
	config        TypePoolConfig
	agents        map[string]*agent.Agent
	warmPool      []*agent.Agent // Pre-spawned agents ready for use
	waitQueue     []chan *agent.Agent
	totalSpawned  int64
	totalAcquired int64
	totalReleased int64
	lastScaleUp   time.Time
	lastScaleDown time.Time
}

// newTypePool creates a new TypePool for the specified agent type.
func newTypePool(agentType shared.AgentType, config TypePoolConfig) *TypePool {
	return &TypePool{
		agentType: agentType,
		config:    config,
		agents:    make(map[string]*agent.Agent),
		warmPool:  make([]*agent.Agent, 0, config.WarmPoolSize),
		waitQueue: make([]chan *agent.Agent, 0),
	}
}

// AgentPoolManager manages pools of agents by type.
type AgentPoolManager struct {
	mu       sync.RWMutex
	pools    map[shared.AgentType]*TypePool
	configs  map[shared.AgentType]TypePoolConfig
	registry *agent.AgentTypeRegistry

	// Callbacks
	onSpawn     func(a *agent.Agent)
	onTerminate func(agentID string)

	// Background tasks
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAgentPoolManager creates a new AgentPoolManager.
func NewAgentPoolManager(registry *agent.AgentTypeRegistry) *AgentPoolManager {
	ctx, cancel := context.WithCancel(context.Background())

	pm := &AgentPoolManager{
		pools:    make(map[shared.AgentType]*TypePool),
		configs:  make(map[shared.AgentType]TypePoolConfig),
		registry: registry,
		ctx:      ctx,
		cancel:   cancel,
	}

	return pm
}

// Initialize initializes the pool manager and starts background tasks.
func (pm *AgentPoolManager) Initialize() error {
	pm.wg.Add(1)
	go pm.maintenanceLoop()
	return nil
}

// Shutdown shuts down the pool manager.
func (pm *AgentPoolManager) Shutdown() error {
	pm.cancel()
	pm.wg.Wait()

	// Terminate all agents
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, pool := range pm.pools {
		pool.mu.Lock()
		for _, a := range pool.agents {
			a.Terminate()
		}
		for _, a := range pool.warmPool {
			a.Terminate()
		}
		pool.mu.Unlock()
	}

	return nil
}

// SetConfig sets the configuration for a specific agent type.
func (pm *AgentPoolManager) SetConfig(agentType shared.AgentType, config TypePoolConfig) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.configs[agentType] = config

	// Update existing pool if present
	if pool, exists := pm.pools[agentType]; exists {
		pool.mu.Lock()
		pool.config = config
		pool.mu.Unlock()
	}
}

// GetConfig returns the configuration for a specific agent type.
func (pm *AgentPoolManager) GetConfig(agentType shared.AgentType) TypePoolConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if config, exists := pm.configs[agentType]; exists {
		return config
	}
	return DefaultTypePoolConfig()
}

// getOrCreatePool gets or creates a pool for the specified agent type.
func (pm *AgentPoolManager) getOrCreatePool(agentType shared.AgentType) *TypePool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pool, exists := pm.pools[agentType]
	if !exists {
		config := pm.configs[agentType]
		if config.MaxAgents == 0 {
			config = DefaultTypePoolConfig()
		}
		pool = newTypePool(agentType, config)
		pm.pools[agentType] = pool
	}
	return pool
}

// Spawn spawns a new agent of the specified type.
func (pm *AgentPoolManager) Spawn(agentType shared.AgentType, config shared.AgentConfig) (*agent.Agent, error) {
	pool := pm.getOrCreatePool(agentType)

	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Check if we can spawn more agents
	currentCount := len(pool.agents) + len(pool.warmPool)
	if currentCount >= pool.config.MaxAgents {
		return nil, shared.ErrMaxAgentsReached
	}

	// Try to get from warm pool first
	if len(pool.warmPool) > 0 {
		a := pool.warmPool[len(pool.warmPool)-1]
		pool.warmPool = pool.warmPool[:len(pool.warmPool)-1]
		pool.agents[a.ID] = a
		pool.totalAcquired++
		a.Activate()
		return a, nil
	}

	// Create new agent
	config.Type = agentType
	if config.ID == "" {
		config.ID = shared.GenerateID("agent")
	}
	a := agent.FromConfig(config)
	pool.agents[a.ID] = a
	pool.totalSpawned++
	pool.totalAcquired++
	pool.lastScaleUp = time.Now()

	if pm.onSpawn != nil {
		pm.onSpawn(a)
	}

	return a, nil
}

// Acquire acquires an available agent of the specified type.
// If no agent is available, it spawns a new one.
func (pm *AgentPoolManager) Acquire(agentType shared.AgentType) (*agent.Agent, error) {
	pool := pm.getOrCreatePool(agentType)

	pool.mu.Lock()

	// Try to find an idle agent
	for _, a := range pool.agents {
		if a.GetStatus() == shared.AgentStatusIdle {
			a.Activate()
			pool.totalAcquired++
			pool.mu.Unlock()
			return a, nil
		}
	}

	// Try warm pool
	if len(pool.warmPool) > 0 {
		a := pool.warmPool[len(pool.warmPool)-1]
		pool.warmPool = pool.warmPool[:len(pool.warmPool)-1]
		pool.agents[a.ID] = a
		pool.totalAcquired++
		a.Activate()
		pool.mu.Unlock()
		return a, nil
	}

	// Check if we can spawn more
	currentCount := len(pool.agents) + len(pool.warmPool)
	if currentCount >= pool.config.MaxAgents {
		pool.mu.Unlock()
		return nil, shared.ErrMaxAgentsReached
	}

	pool.mu.Unlock()

	// Spawn a new agent
	return pm.Spawn(agentType, shared.AgentConfig{Type: agentType})
}

// Release releases an agent back to the pool.
func (pm *AgentPoolManager) Release(a *agent.Agent) {
	pool := pm.getOrCreatePool(a.Type)

	pool.mu.Lock()
	defer pool.mu.Unlock()

	if _, exists := pool.agents[a.ID]; exists {
		a.SetIdle()
		pool.totalReleased++
	}
}

// Terminate terminates an agent and removes it from the pool.
func (pm *AgentPoolManager) Terminate(agentID string, agentType shared.AgentType) {
	pool := pm.getOrCreatePool(agentType)

	pool.mu.Lock()
	defer pool.mu.Unlock()

	if a, exists := pool.agents[agentID]; exists {
		a.Terminate()
		delete(pool.agents, agentID)
		pool.lastScaleDown = time.Now()

		if pm.onTerminate != nil {
			pm.onTerminate(agentID)
		}
	}
}

// GetAgent returns an agent by ID from any pool.
func (pm *AgentPoolManager) GetAgent(agentID string) *agent.Agent {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, pool := range pm.pools {
		pool.mu.RLock()
		if a, exists := pool.agents[agentID]; exists {
			pool.mu.RUnlock()
			return a
		}
		pool.mu.RUnlock()
	}
	return nil
}

// GetAgentsByType returns all agents of the specified type.
func (pm *AgentPoolManager) GetAgentsByType(agentType shared.AgentType) []*agent.Agent {
	pool := pm.getOrCreatePool(agentType)

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	agents := make([]*agent.Agent, 0, len(pool.agents))
	for _, a := range pool.agents {
		agents = append(agents, a)
	}
	return agents
}

// GetActiveAgentsByType returns active agents of the specified type.
func (pm *AgentPoolManager) GetActiveAgentsByType(agentType shared.AgentType) []*agent.Agent {
	pool := pm.getOrCreatePool(agentType)

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	agents := make([]*agent.Agent, 0)
	for _, a := range pool.agents {
		status := a.GetStatus()
		if status == shared.AgentStatusActive || status == shared.AgentStatusBusy {
			agents = append(agents, a)
		}
	}
	return agents
}

// GetPoolStats returns statistics for a specific agent type pool.
func (pm *AgentPoolManager) GetPoolStats(agentType shared.AgentType) PoolStats {
	pool := pm.getOrCreatePool(agentType)

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	activeCount := 0
	idleCount := 0
	busyCount := 0

	for _, a := range pool.agents {
		switch a.GetStatus() {
		case shared.AgentStatusActive:
			activeCount++
		case shared.AgentStatusIdle:
			idleCount++
		case shared.AgentStatusBusy:
			busyCount++
		}
	}

	totalAgents := len(pool.agents)
	utilization := 0.0
	if totalAgents > 0 {
		utilization = float64(busyCount) / float64(totalAgents)
	}

	return PoolStats{
		AgentType:     agentType,
		TotalAgents:   totalAgents,
		ActiveAgents:  activeCount,
		IdleAgents:    idleCount,
		BusyAgents:    busyCount,
		WarmPoolSize:  len(pool.warmPool),
		TotalSpawned:  pool.totalSpawned,
		TotalAcquired: pool.totalAcquired,
		TotalReleased: pool.totalReleased,
		Utilization:   utilization,
		MaxAgents:     pool.config.MaxAgents,
		MinAgents:     pool.config.MinAgents,
	}
}

// PoolStats holds statistics for an agent pool.
type PoolStats struct {
	AgentType     shared.AgentType `json:"agentType"`
	TotalAgents   int              `json:"totalAgents"`
	ActiveAgents  int              `json:"activeAgents"`
	IdleAgents    int              `json:"idleAgents"`
	BusyAgents    int              `json:"busyAgents"`
	WarmPoolSize  int              `json:"warmPoolSize"`
	TotalSpawned  int64            `json:"totalSpawned"`
	TotalAcquired int64            `json:"totalAcquired"`
	TotalReleased int64            `json:"totalReleased"`
	Utilization   float64          `json:"utilization"`
	MaxAgents     int              `json:"maxAgents"`
	MinAgents     int              `json:"minAgents"`
}

// GetAllPoolStats returns statistics for all pools.
func (pm *AgentPoolManager) GetAllPoolStats() []PoolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := make([]PoolStats, 0, len(pm.pools))
	for agentType := range pm.pools {
		pm.mu.RUnlock()
		stat := pm.GetPoolStats(agentType)
		pm.mu.RLock()
		stats = append(stats, stat)
	}
	return stats
}

// Scale scales a pool to the specified target size.
func (pm *AgentPoolManager) Scale(agentType shared.AgentType, targetSize int) error {
	pool := pm.getOrCreatePool(agentType)

	pool.mu.Lock()
	defer pool.mu.Unlock()

	currentSize := len(pool.agents) + len(pool.warmPool)

	if targetSize > pool.config.MaxAgents {
		targetSize = pool.config.MaxAgents
	}
	if targetSize < pool.config.MinAgents {
		targetSize = pool.config.MinAgents
	}

	if targetSize > currentSize {
		// Scale up - add to warm pool
		toAdd := targetSize - currentSize
		for i := 0; i < toAdd; i++ {
			config := shared.AgentConfig{
				Type: agentType,
				ID:   shared.GenerateID("agent"),
			}
			a := agent.FromConfig(config)
			a.SetIdle()
			pool.warmPool = append(pool.warmPool, a)
			pool.totalSpawned++
		}
		pool.lastScaleUp = time.Now()
	} else if targetSize < currentSize {
		// Scale down - terminate idle agents first
		toRemove := currentSize - targetSize

		// Remove from warm pool first
		for toRemove > 0 && len(pool.warmPool) > 0 {
			a := pool.warmPool[len(pool.warmPool)-1]
			pool.warmPool = pool.warmPool[:len(pool.warmPool)-1]
			a.Terminate()
			toRemove--
		}

		// Remove idle agents
		for id, a := range pool.agents {
			if toRemove <= 0 {
				break
			}
			if a.GetStatus() == shared.AgentStatusIdle {
				a.Terminate()
				delete(pool.agents, id)
				toRemove--
			}
		}
		pool.lastScaleDown = time.Now()
	}

	return nil
}

// WarmUp pre-spawns agents into the warm pool.
func (pm *AgentPoolManager) WarmUp(agentType shared.AgentType, count int) error {
	pool := pm.getOrCreatePool(agentType)

	pool.mu.Lock()
	defer pool.mu.Unlock()

	currentTotal := len(pool.agents) + len(pool.warmPool)
	available := pool.config.MaxAgents - currentTotal

	if count > available {
		count = available
	}

	for i := 0; i < count; i++ {
		config := shared.AgentConfig{
			Type: agentType,
			ID:   shared.GenerateID("agent"),
		}
		a := agent.FromConfig(config)
		a.SetIdle()
		pool.warmPool = append(pool.warmPool, a)
		pool.totalSpawned++
	}

	return nil
}

// SetOnSpawn sets the callback for agent spawn events.
func (pm *AgentPoolManager) SetOnSpawn(handler func(a *agent.Agent)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.onSpawn = handler
}

// SetOnTerminate sets the callback for agent termination events.
func (pm *AgentPoolManager) SetOnTerminate(handler func(agentID string)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.onTerminate = handler
}

// maintenanceLoop runs periodic maintenance tasks.
func (pm *AgentPoolManager) maintenanceLoop() {
	defer pm.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.performMaintenance()
		}
	}
}

// performMaintenance performs pool maintenance tasks.
func (pm *AgentPoolManager) performMaintenance() {
	pm.mu.RLock()
	pools := make([]*TypePool, 0, len(pm.pools))
	for _, pool := range pm.pools {
		pools = append(pools, pool)
	}
	pm.mu.RUnlock()

	now := time.Now()

	for _, pool := range pools {
		pool.mu.Lock()

		// Terminate idle agents that have exceeded idle timeout
		for id, a := range pool.agents {
			if a.GetStatus() == shared.AgentStatusIdle {
				idleTime := time.Duration(now.UnixMilli()-a.LastActive) * time.Millisecond
				if idleTime > pool.config.IdleTimeout {
					// Keep minimum agents
					currentCount := len(pool.agents)
					if currentCount > pool.config.MinAgents {
						a.Terminate()
						delete(pool.agents, id)
					}
				}
			}
		}

		// Ensure warm pool is filled
		currentWarm := len(pool.warmPool)
		targetWarm := pool.config.WarmPoolSize
		currentTotal := len(pool.agents) + currentWarm

		if currentWarm < targetWarm && currentTotal < pool.config.MaxAgents {
			toAdd := targetWarm - currentWarm
			available := pool.config.MaxAgents - currentTotal
			if toAdd > available {
				toAdd = available
			}

			for i := 0; i < toAdd; i++ {
				config := shared.AgentConfig{
					Type: pool.agentType,
					ID:   shared.GenerateID("agent"),
				}
				a := agent.FromConfig(config)
				a.SetIdle()
				pool.warmPool = append(pool.warmPool, a)
				pool.totalSpawned++
			}
		}

		pool.mu.Unlock()
	}
}

// ListPoolTypes returns all pool types that have been created.
func (pm *AgentPoolManager) ListPoolTypes() []shared.AgentType {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	types := make([]shared.AgentType, 0, len(pm.pools))
	for t := range pm.pools {
		types = append(types, t)
	}
	return types
}

// GetTotalAgentCount returns the total number of agents across all pools.
func (pm *AgentPoolManager) GetTotalAgentCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	total := 0
	for _, pool := range pm.pools {
		pool.mu.RLock()
		total += len(pool.agents) + len(pool.warmPool)
		pool.mu.RUnlock()
	}
	return total
}
