// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	domainMemory "github.com/anthropics/claude-flow-go/internal/domain/memory"
	appMemory "github.com/anthropics/claude-flow-go/internal/application/memory"
	infraMemory "github.com/anthropics/claude-flow-go/internal/infrastructure/memory"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ========================================================================
// Memory Drift Tool
// ========================================================================

// MemoryDriftTool checks memory drift status.
type MemoryDriftTool struct {
	driftService *appMemory.DriftService
}

// MemoryDriftInput is the input for the memory drift tool.
type MemoryDriftInput struct {
	// Action is the action to perform.
	Action string `json:"action"` // status, check, alerts, history, acknowledge, resolve, refresh
	// AlertID is the alert ID for acknowledge/resolve actions.
	AlertID string `json:"alertId,omitempty"`
}

// MemoryDriftOutput is the output for the memory drift tool.
type MemoryDriftOutput struct {
	// Status is the current drift status.
	Status *domainMemory.DriftStatus `json:"status,omitempty"`
	// Alerts are the active alerts.
	Alerts []*domainMemory.DriftAlert `json:"alerts,omitempty"`
	// Metrics are the drift metrics.
	Metrics *domainMemory.DriftMetrics `json:"metrics,omitempty"`
	// History is the drift history.
	History *domainMemory.DriftHistory `json:"history,omitempty"`
	// Baseline is the current baseline.
	Baseline *domainMemory.EmbeddingBaseline `json:"baseline,omitempty"`
	// Success indicates if the action was successful.
	Success bool `json:"success"`
	// Message is a status message.
	Message string `json:"message,omitempty"`
}

// NewMemoryDriftTool creates a new memory drift tool.
func NewMemoryDriftTool(driftService *appMemory.DriftService) *MemoryDriftTool {
	return &MemoryDriftTool{
		driftService: driftService,
	}
}

// Name returns the tool name.
func (t *MemoryDriftTool) Name() string {
	return "memory_drift"
}

// Description returns the tool description.
func (t *MemoryDriftTool) Description() string {
	return "Check memory embedding drift status and manage drift alerts"
}

// InputSchema returns the tool input schema.
func (t *MemoryDriftTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: status, check, alerts, history, acknowledge, resolve, refresh",
				"enum":        []string{"status", "check", "alerts", "history", "acknowledge", "resolve", "refresh"},
			},
			"alertId": map[string]interface{}{
				"type":        "string",
				"description": "Alert ID for acknowledge/resolve actions",
			},
		},
		"required": []string{"action"},
	}
}

// Execute executes the tool.
func (t *MemoryDriftTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params MemoryDriftInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	output := &MemoryDriftOutput{Success: true}

	switch params.Action {
	case "status":
		status, err := t.driftService.GetStatus(ctx)
		if err != nil {
			return nil, err
		}
		output.Status = status
		output.Baseline = t.driftService.GetBaseline()
		output.Message = fmt.Sprintf("Drift level: %.2f%%, Healthy: %v", status.CurrentLevel*100, status.IsHealthy)

	case "check":
		status, alerts, err := t.driftService.CheckDriftNow(ctx)
		if err != nil {
			return nil, err
		}
		output.Status = status
		output.Alerts = alerts
		if len(alerts) > 0 {
			output.Message = fmt.Sprintf("Detected %d drift alerts", len(alerts))
		} else {
			output.Message = "No drift detected"
		}

	case "alerts":
		output.Alerts = t.driftService.GetAlerts()
		output.Message = fmt.Sprintf("Found %d active alerts", len(output.Alerts))

	case "history":
		output.History = t.driftService.GetHistory()
		output.Message = fmt.Sprintf("History contains %d entries", len(output.History.Entries))

	case "acknowledge":
		if params.AlertID == "" {
			return nil, fmt.Errorf("alertId is required for acknowledge action")
		}
		output.Success = t.driftService.AcknowledgeAlert(params.AlertID)
		if output.Success {
			output.Message = "Alert acknowledged"
		} else {
			output.Message = "Alert not found or already acknowledged"
		}

	case "resolve":
		if params.AlertID == "" {
			return nil, fmt.Errorf("alertId is required for resolve action")
		}
		output.Success = t.driftService.ResolveAlert(params.AlertID)
		if output.Success {
			output.Message = "Alert resolved"
		} else {
			output.Message = "Alert not found or already resolved"
		}

	case "refresh":
		if err := t.driftService.RefreshBaseline(ctx); err != nil {
			return nil, err
		}
		output.Baseline = t.driftService.GetBaseline()
		output.Message = fmt.Sprintf("Baseline refreshed with %d samples", output.Baseline.SampleCount)

	default:
		return nil, fmt.Errorf("unknown action: %s", params.Action)
	}

	return output, nil
}

// ========================================================================
// Memory Optimize Tool
// ========================================================================

// MemoryOptimizeTool optimizes memory storage.
type MemoryOptimizeTool struct {
	optimizationService *appMemory.OptimizationService
}

// MemoryOptimizeInput is the input for the memory optimize tool.
type MemoryOptimizeInput struct {
	// Action is the action to perform.
	Action string `json:"action"` // optimize, migrate, cleanup, stats
}

// MemoryOptimizeOutput is the output for the memory optimize tool.
type MemoryOptimizeOutput struct {
	// Result is the optimization result.
	Result *appMemory.OptimizationResult `json:"result,omitempty"`
	// Migration is the migration result.
	Migration *appMemory.MigrationResult `json:"migration,omitempty"`
	// Cleanup is the cleanup result.
	Cleanup *appMemory.CleanupResult `json:"cleanup,omitempty"`
	// Stats are the optimization statistics.
	Stats *domainMemory.OptimizationStats `json:"stats,omitempty"`
	// Message is a status message.
	Message string `json:"message"`
}

// NewMemoryOptimizeTool creates a new memory optimize tool.
func NewMemoryOptimizeTool(optimizationService *appMemory.OptimizationService) *MemoryOptimizeTool {
	return &MemoryOptimizeTool{
		optimizationService: optimizationService,
	}
}

// Name returns the tool name.
func (t *MemoryOptimizeTool) Name() string {
	return "memory_optimize"
}

// Description returns the tool description.
func (t *MemoryOptimizeTool) Description() string {
	return "Optimize memory storage with tier migration and cleanup"
}

// InputSchema returns the tool input schema.
func (t *MemoryOptimizeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: optimize, migrate, cleanup, stats",
				"enum":        []string{"optimize", "migrate", "cleanup", "stats"},
			},
		},
		"required": []string{"action"},
	}
}

// Execute executes the tool.
func (t *MemoryOptimizeTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params MemoryOptimizeInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	output := &MemoryOptimizeOutput{}

	switch params.Action {
	case "optimize":
		output.Result = t.optimizationService.Optimize(ctx)
		output.Message = fmt.Sprintf("Optimization completed in %dms", output.Result.DurationMs())

	case "migrate":
		output.Migration = t.optimizationService.RunMigration(ctx)
		output.Message = fmt.Sprintf("Migrated %d memories (hot->warm: %d, warm->cold: %d)",
			output.Migration.TotalMigrated, output.Migration.HotToWarm, output.Migration.WarmToCold)

	case "cleanup":
		output.Cleanup = t.optimizationService.RunCleanup(ctx)
		output.Message = fmt.Sprintf("Cleaned up %d memories (expired: %d, excess: %d)",
			output.Cleanup.DeletedCount, output.Cleanup.ExpiredCount, output.Cleanup.ExcessCount)

	case "stats":
		output.Stats = t.optimizationService.GetStats()
		output.Message = fmt.Sprintf("Total memories: %d, Reduction: %.1f%%",
			output.Stats.TotalMemories, output.Stats.MemoryReductionPercent)

	default:
		return nil, fmt.Errorf("unknown action: %s", params.Action)
	}

	return output, nil
}

// ========================================================================
// Memory Sync Tool
// ========================================================================

// MemorySyncTool synchronizes memory across swarm.
type MemorySyncTool struct {
	swarmSync *infraMemory.SwarmSynchronizer
}

// MemorySyncInput is the input for the memory sync tool.
type MemorySyncInput struct {
	// Action is the action to perform.
	Action string `json:"action"` // status, peers, pending, register, remove
	// PeerID is the peer ID for register/remove actions.
	PeerID string `json:"peerId,omitempty"`
	// Address is the peer address for register action.
	Address string `json:"address,omitempty"`
}

// MemorySyncOutput is the output for the memory sync tool.
type MemorySyncOutput struct {
	// Stats are the sync statistics.
	Stats *infraMemory.SwarmSyncStats `json:"stats,omitempty"`
	// Peers are the registered peers.
	Peers []*infraMemory.PeerNode `json:"peers,omitempty"`
	// PendingOps is the pending operation count.
	PendingOps int `json:"pendingOps,omitempty"`
	// Success indicates if the action was successful.
	Success bool `json:"success"`
	// Message is a status message.
	Message string `json:"message"`
}

// NewMemorySyncTool creates a new memory sync tool.
func NewMemorySyncTool(swarmSync *infraMemory.SwarmSynchronizer) *MemorySyncTool {
	return &MemorySyncTool{
		swarmSync: swarmSync,
	}
}

// Name returns the tool name.
func (t *MemorySyncTool) Name() string {
	return "memory_sync"
}

// Description returns the tool description.
func (t *MemorySyncTool) Description() string {
	return "Synchronize memory across swarm nodes"
}

// InputSchema returns the tool input schema.
func (t *MemorySyncTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: status, peers, pending, register, remove",
				"enum":        []string{"status", "peers", "pending", "register", "remove"},
			},
			"peerId": map[string]interface{}{
				"type":        "string",
				"description": "Peer ID for register/remove actions",
			},
			"address": map[string]interface{}{
				"type":        "string",
				"description": "Peer address for register action",
			},
		},
		"required": []string{"action"},
	}
}

// Execute executes the tool.
func (t *MemorySyncTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params MemorySyncInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	output := &MemorySyncOutput{Success: true}

	switch params.Action {
	case "status":
		output.Stats = t.swarmSync.GetStats()
		output.Message = fmt.Sprintf("Syncs: %d successful, %d failed, %d conflicts",
			output.Stats.SuccessfulSyncs, output.Stats.FailedSyncs, output.Stats.ConflictsDetected)

	case "peers":
		output.Peers = t.swarmSync.GetPeers()
		output.Message = fmt.Sprintf("Found %d registered peers", len(output.Peers))

	case "pending":
		ops := t.swarmSync.GetPendingOperations()
		output.PendingOps = len(ops)
		output.Message = fmt.Sprintf("Found %d pending sync operations", output.PendingOps)

	case "register":
		if params.PeerID == "" || params.Address == "" {
			return nil, fmt.Errorf("peerId and address are required for register action")
		}
		t.swarmSync.RegisterPeer(params.PeerID, params.Address)
		output.Message = fmt.Sprintf("Registered peer %s at %s", params.PeerID, params.Address)

	case "remove":
		if params.PeerID == "" {
			return nil, fmt.Errorf("peerId is required for remove action")
		}
		t.swarmSync.RemovePeer(params.PeerID)
		output.Message = fmt.Sprintf("Removed peer %s", params.PeerID)

	default:
		return nil, fmt.Errorf("unknown action: %s", params.Action)
	}

	return output, nil
}

// ========================================================================
// Memory Stats Tool
// ========================================================================

// MemoryStatsTool provides memory statistics.
type MemoryStatsTool struct {
	memoryService *appMemory.MemoryService
	backend       shared.MemoryBackend
}

// MemoryStatsInput is the input for the memory stats tool.
type MemoryStatsInput struct {
	// Detailed returns detailed statistics.
	Detailed bool `json:"detailed,omitempty"`
}

// MemoryStatsOutput is the output for the memory stats tool.
type MemoryStatsOutput struct {
	// TotalMemories is the total memory count.
	TotalMemories int64 `json:"totalMemories"`
	// TotalSizeBytes is the total storage size.
	TotalSizeBytes int64 `json:"totalSizeBytes,omitempty"`
	// ServiceStats are the service statistics.
	ServiceStats *appMemory.MemoryServiceStats `json:"serviceStats,omitempty"`
	// TierStats are tier statistics.
	TierStats map[domainMemory.StorageTier]*domainMemory.TierStats `json:"tierStats,omitempty"`
	// Message is a status message.
	Message string `json:"message"`
}

// NewMemoryStatsTool creates a new memory stats tool.
func NewMemoryStatsTool(memoryService *appMemory.MemoryService, backend shared.MemoryBackend) *MemoryStatsTool {
	return &MemoryStatsTool{
		memoryService: memoryService,
		backend:       backend,
	}
}

// Name returns the tool name.
func (t *MemoryStatsTool) Name() string {
	return "memory_stats"
}

// Description returns the tool description.
func (t *MemoryStatsTool) Description() string {
	return "Get memory system statistics"
}

// InputSchema returns the tool input schema.
func (t *MemoryStatsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"detailed": map[string]interface{}{
				"type":        "boolean",
				"description": "Return detailed statistics",
			},
		},
	}
}

// Execute executes the tool.
func (t *MemoryStatsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params MemoryStatsInput
	if len(input) > 0 {
		json.Unmarshal(input, &params)
	}

	output := &MemoryStatsOutput{}

	// Get memory count from backend
	memories, err := t.backend.Query(shared.MemoryQuery{})
	if err == nil {
		output.TotalMemories = int64(len(memories))
	}

	// Get service stats
	if t.memoryService != nil {
		output.ServiceStats = t.memoryService.GetStats()

		if output.ServiceStats.TieredStats != nil {
			output.TotalSizeBytes = output.ServiceStats.TieredStats.TotalSizeBytes
			if params.Detailed {
				output.TierStats = output.ServiceStats.TieredStats.TierStats
			}
		}
	}

	output.Message = fmt.Sprintf("Total memories: %d, Stores: %d, Retrieves: %d, Searches: %d",
		output.TotalMemories,
		output.ServiceStats.StoreCount,
		output.ServiceStats.RetrieveCount,
		output.ServiceStats.SearchCount)

	return output, nil
}

// ========================================================================
// Tool Registry
// ========================================================================

// RegisterAdvancedMemoryTools registers all advanced memory tools.
func RegisterAdvancedMemoryTools(
	driftService *appMemory.DriftService,
	optimizationService *appMemory.OptimizationService,
	swarmSync *infraMemory.SwarmSynchronizer,
	memoryService *appMemory.MemoryService,
	backend shared.MemoryBackend,
) []interface{} {
	tools := make([]interface{}, 0)

	if driftService != nil {
		tools = append(tools, NewMemoryDriftTool(driftService))
	}

	if optimizationService != nil {
		tools = append(tools, NewMemoryOptimizeTool(optimizationService))
	}

	if swarmSync != nil {
		tools = append(tools, NewMemorySyncTool(swarmSync))
	}

	if memoryService != nil && backend != nil {
		tools = append(tools, NewMemoryStatsTool(memoryService, backend))
	}

	return tools
}
