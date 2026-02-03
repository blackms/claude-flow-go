// Package memory provides infrastructure for memory management.
package memory

import (
	"sync"
	"time"

	domainMemory "github.com/anthropics/claude-flow-go/internal/domain/memory"
)

// QueryOptimizer optimizes memory queries.
type QueryOptimizer struct {
	mu          sync.RWMutex
	planCache   map[string]*cachedPlan
	stats       *QueryOptimizerStats
	config      QueryOptimizerConfig
}

// cachedPlan is a cached query plan.
type cachedPlan struct {
	Plan      *domainMemory.QueryPlan
	Query     string
	HitCount  int
	CreatedAt time.Time
	LastUsed  time.Time
}

// QueryOptimizerConfig configures the optimizer.
type QueryOptimizerConfig struct {
	// CacheSize is the plan cache size.
	CacheSize int `json:"cacheSize"`

	// CacheTTLMs is the cache TTL in milliseconds.
	CacheTTLMs int64 `json:"cacheTTLMs"`

	// EnableFilterPushdown enables filter pushdown.
	EnableFilterPushdown bool `json:"enableFilterPushdown"`

	// EnableIndexSelection enables automatic index selection.
	EnableIndexSelection bool `json:"enableIndexSelection"`

	// ParallelThreshold is the row count threshold for parallel execution.
	ParallelThreshold int `json:"parallelThreshold"`
}

// DefaultQueryOptimizerConfig returns the default configuration.
func DefaultQueryOptimizerConfig() QueryOptimizerConfig {
	return QueryOptimizerConfig{
		CacheSize:            100,
		CacheTTLMs:           60 * 60 * 1000, // 1 hour
		EnableFilterPushdown: true,
		EnableIndexSelection: true,
		ParallelThreshold:    1000,
	}
}

// QueryOptimizerStats contains optimizer statistics.
type QueryOptimizerStats struct {
	// PlanCacheHits is the plan cache hit count.
	PlanCacheHits int64 `json:"planCacheHits"`

	// PlanCacheMisses is the plan cache miss count.
	PlanCacheMisses int64 `json:"planCacheMisses"`

	// OptimizedQueries is the number of optimized queries.
	OptimizedQueries int64 `json:"optimizedQueries"`

	// AvgPlanTimeMs is the average planning time.
	AvgPlanTimeMs float64 `json:"avgPlanTimeMs"`

	// FiltersPushedDown is the number of filters pushed down.
	FiltersPushedDown int64 `json:"filtersPushedDown"`

	// IndexesUsed is the number of times indexes were used.
	IndexesUsed int64 `json:"indexesUsed"`
}

// NewQueryOptimizer creates a new query optimizer.
func NewQueryOptimizer(config QueryOptimizerConfig) *QueryOptimizer {
	return &QueryOptimizer{
		planCache: make(map[string]*cachedPlan),
		stats:     &QueryOptimizerStats{},
		config:    config,
	}
}

// OptimizeSemanticQuery optimizes a semantic query.
func (o *QueryOptimizer) OptimizeSemanticQuery(query *domainMemory.SemanticQuery) *domainMemory.QueryPlan {
	cacheKey := o.generateCacheKey("semantic", query)

	// Check cache
	if plan := o.getCachedPlan(cacheKey); plan != nil {
		return plan
	}

	startTime := time.Now()
	plan := &domainMemory.QueryPlan{
		Steps:         make([]domainMemory.QueryStep, 0),
		UseIndex:      make([]string, 0),
		PushedFilters: make([]string, 0),
	}

	// Step 1: Filter pushdown
	if o.config.EnableFilterPushdown && len(query.Filters) > 0 {
		for key := range query.Filters {
			plan.PushedFilters = append(plan.PushedFilters, key)
		}
		plan.Steps = append(plan.Steps, domainMemory.QueryStep{
			Operation:     "filter_pushdown",
			Description:   "Push metadata filters to storage layer",
			EstimatedRows: 0, // Will be estimated
			Cost:          0.1,
		})
		o.stats.FiltersPushedDown++
	}

	// Step 2: Index selection for embedding search
	if o.config.EnableIndexSelection && len(query.Embedding) > 0 {
		plan.UseIndex = append(plan.UseIndex, "embedding_hnsw")
		plan.Steps = append(plan.Steps, domainMemory.QueryStep{
			Operation:     "vector_search",
			Description:   "Use HNSW index for approximate nearest neighbor search",
			EstimatedRows: query.TopK,
			Cost:          1.0,
		})
		o.stats.IndexesUsed++
	}

	// Step 3: Type filter
	if len(query.MemoryTypes) > 0 {
		plan.UseIndex = append(plan.UseIndex, "type_index")
		plan.Steps = append(plan.Steps, domainMemory.QueryStep{
			Operation:     "type_filter",
			Description:   "Filter by memory types",
			EstimatedRows: 0,
			Cost:          0.2,
		})
	}

	// Step 4: Agent filter
	if query.AgentID != "" {
		plan.UseIndex = append(plan.UseIndex, "agent_index")
		plan.Steps = append(plan.Steps, domainMemory.QueryStep{
			Operation:     "agent_filter",
			Description:   "Filter by agent ID",
			EstimatedRows: 0,
			Cost:          0.1,
		})
	}

	// Step 5: Score and rank
	plan.Steps = append(plan.Steps, domainMemory.QueryStep{
		Operation:     "score_rank",
		Description:   "Calculate similarity scores and rank results",
		EstimatedRows: query.TopK,
		Cost:          0.5,
	})

	// Calculate total cost
	for _, step := range plan.Steps {
		plan.EstimatedCost += step.Cost
	}

	o.cachePlan(cacheKey, plan)
	o.stats.OptimizedQueries++
	o.updateAvgPlanTime(time.Since(startTime))

	return plan
}

// OptimizeTemporalQuery optimizes a temporal query.
func (o *QueryOptimizer) OptimizeTemporalQuery(query *domainMemory.TemporalQuery) *domainMemory.QueryPlan {
	cacheKey := o.generateCacheKey("temporal", query)

	if plan := o.getCachedPlan(cacheKey); plan != nil {
		return plan
	}

	startTime := time.Now()
	plan := &domainMemory.QueryPlan{
		Steps:         make([]domainMemory.QueryStep, 0),
		UseIndex:      make([]string, 0),
		PushedFilters: make([]string, 0),
	}

	// Step 1: Time range filter
	if query.StartTime != nil || query.EndTime != nil {
		plan.UseIndex = append(plan.UseIndex, "timestamp_index")
		plan.Steps = append(plan.Steps, domainMemory.QueryStep{
			Operation:     "time_range_scan",
			Description:   "Scan timestamp index for time range",
			EstimatedRows: 0,
			Cost:          0.5,
		})
		o.stats.IndexesUsed++
	}

	// Step 2: Apply decay function
	if query.DecayFunction != domainMemory.DecayNone {
		plan.Steps = append(plan.Steps, domainMemory.QueryStep{
			Operation:     "apply_decay",
			Description:   "Apply temporal decay function",
			EstimatedRows: 0,
			Cost:          0.3,
		})
	}

	// Step 3: Sort and limit
	plan.Steps = append(plan.Steps, domainMemory.QueryStep{
		Operation:     "sort_limit",
		Description:   "Sort by timestamp and apply limit",
		EstimatedRows: query.Limit,
		Cost:          0.2,
	})

	for _, step := range plan.Steps {
		plan.EstimatedCost += step.Cost
	}

	o.cachePlan(cacheKey, plan)
	o.stats.OptimizedQueries++
	o.updateAvgPlanTime(time.Since(startTime))

	return plan
}

// OptimizeMultiModalQuery optimizes a multi-modal query.
func (o *QueryOptimizer) OptimizeMultiModalQuery(query *domainMemory.MultiModalQuery) *domainMemory.QueryPlan {
	cacheKey := o.generateCacheKey("multimodal", query)

	if plan := o.getCachedPlan(cacheKey); plan != nil {
		return plan
	}

	startTime := time.Now()
	plan := &domainMemory.QueryPlan{
		Steps:         make([]domainMemory.QueryStep, 0),
		UseIndex:      make([]string, 0),
		PushedFilters: make([]string, 0),
	}

	// Step 1: Semantic component
	if query.Semantic != nil {
		plan.Steps = append(plan.Steps, domainMemory.QueryStep{
			Operation:     "semantic_search",
			Description:   "Execute semantic vector search",
			EstimatedRows: query.Semantic.TopK,
			Cost:          1.0,
		})
		plan.UseIndex = append(plan.UseIndex, "embedding_hnsw")
	}

	// Step 2: Temporal component
	if query.Temporal != nil {
		plan.Steps = append(plan.Steps, domainMemory.QueryStep{
			Operation:     "temporal_filter",
			Description:   "Apply temporal filters and decay",
			EstimatedRows: query.Temporal.Limit,
			Cost:          0.5,
		})
		plan.UseIndex = append(plan.UseIndex, "timestamp_index")
	}

	// Step 3: Metadata filters
	if len(query.Metadata) > 0 {
		for key := range query.Metadata {
			plan.PushedFilters = append(plan.PushedFilters, key)
		}
		plan.Steps = append(plan.Steps, domainMemory.QueryStep{
			Operation:     "metadata_filter",
			Description:   "Apply metadata filters",
			EstimatedRows: 0,
			Cost:          0.2,
		})
	}

	// Step 4: Combine results
	plan.Steps = append(plan.Steps, domainMemory.QueryStep{
		Operation:     "combine_results",
		Description:   "Combine and score multi-modal results",
		EstimatedRows: query.TopK,
		Cost:          0.4,
	})

	for _, step := range plan.Steps {
		plan.EstimatedCost += step.Cost
	}

	o.cachePlan(cacheKey, plan)
	o.stats.OptimizedQueries++
	o.updateAvgPlanTime(time.Since(startTime))

	return plan
}

// generateCacheKey generates a cache key for a query.
func (o *QueryOptimizer) generateCacheKey(queryType string, query interface{}) string {
	// Simple key generation - in production would use hash
	return queryType + "_cached"
}

// getCachedPlan retrieves a cached plan.
func (o *QueryOptimizer) getCachedPlan(key string) *domainMemory.QueryPlan {
	o.mu.RLock()
	defer o.mu.RUnlock()

	cached, ok := o.planCache[key]
	if !ok {
		o.stats.PlanCacheMisses++
		return nil
	}

	// Check TTL
	if time.Since(cached.CreatedAt).Milliseconds() > o.config.CacheTTLMs {
		o.stats.PlanCacheMisses++
		return nil
	}

	cached.HitCount++
	cached.LastUsed = time.Now()
	o.stats.PlanCacheHits++

	return cached.Plan
}

// cachePlan caches a query plan.
func (o *QueryOptimizer) cachePlan(key string, plan *domainMemory.QueryPlan) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Evict if cache is full
	if len(o.planCache) >= o.config.CacheSize {
		o.evictOldest()
	}

	o.planCache[key] = &cachedPlan{
		Plan:      plan,
		Query:     key,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}
}

// evictOldest evicts the oldest cached plan.
func (o *QueryOptimizer) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, cached := range o.planCache {
		if oldestKey == "" || cached.LastUsed.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.LastUsed
		}
	}

	if oldestKey != "" {
		delete(o.planCache, oldestKey)
	}
}

// updateAvgPlanTime updates average planning time.
func (o *QueryOptimizer) updateAvgPlanTime(duration time.Duration) {
	// Simple moving average
	o.mu.Lock()
	defer o.mu.Unlock()

	ms := float64(duration.Microseconds()) / 1000.0
	if o.stats.OptimizedQueries == 1 {
		o.stats.AvgPlanTimeMs = ms
	} else {
		o.stats.AvgPlanTimeMs = (o.stats.AvgPlanTimeMs*float64(o.stats.OptimizedQueries-1) + ms) / float64(o.stats.OptimizedQueries)
	}
}

// GetStats returns optimizer statistics.
func (o *QueryOptimizer) GetStats() *QueryOptimizerStats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.stats
}

// ClearCache clears the plan cache.
func (o *QueryOptimizer) ClearCache() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.planCache = make(map[string]*cachedPlan)
}
