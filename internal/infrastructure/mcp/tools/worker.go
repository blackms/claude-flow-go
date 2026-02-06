// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"
	"time"

	domainWorker "github.com/anthropics/claude-flow-go/internal/domain/worker"
	infraWorker "github.com/anthropics/claude-flow-go/internal/infrastructure/worker"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// WorkerTools provides MCP tools for worker management.
type WorkerTools struct {
	dispatcher *infraWorker.WorkerDispatchService
	poolMgr    *infraWorker.PoolManager
}

// NewWorkerTools creates a new WorkerTools instance.
func NewWorkerTools(dispatcher *infraWorker.WorkerDispatchService, poolMgr *infraWorker.PoolManager) *WorkerTools {
	return &WorkerTools{
		dispatcher: dispatcher,
		poolMgr:    poolMgr,
	}
}

// NewWorkerToolsDefault creates a WorkerTools with default components.
func NewWorkerToolsDefault() *WorkerTools {
	dispatcher := infraWorker.NewWorkerDispatchService()
	poolMgr := infraWorker.NewPoolManager(dispatcher, infraWorker.DefaultPoolConfig())
	return &WorkerTools{
		dispatcher: dispatcher,
		poolMgr:    poolMgr,
	}
}

// GetTools returns available worker tools.
func (t *WorkerTools) GetTools() []shared.MCPTool {
	triggerEnums := make([]string, 0)
	for _, trigger := range domainWorker.AllTriggers() {
		triggerEnums = append(triggerEnums, string(trigger))
	}

	return []shared.MCPTool{
		{
			Name:        "worker_dispatch",
			Description: "Dispatch a background worker for analysis or optimization tasks",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"trigger": map[string]interface{}{
						"type":        "string",
						"enum":        triggerEnums,
						"description": "Worker trigger type (ultralearn, optimize, consolidate, predict, audit, map, preload, deepdive, document, refactor, benchmark, testgaps)",
					},
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Context for the worker (e.g., file path, topic)",
					},
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "Session identifier (auto-generated if not provided)",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"low", "normal", "high", "critical"},
						"description": "Worker priority (default: normal)",
					},
					"timeout": map[string]interface{}{
						"type":        "number",
						"description": "Timeout in milliseconds",
					},
				},
				"required": []string{"trigger", "context"},
			},
		},
		{
			Name:        "worker_status",
			Description: "Get the status of a background worker",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workerId": map[string]interface{}{
						"type":        "string",
						"description": "Worker ID to get status for",
					},
				},
				"required": []string{"workerId"},
			},
		},
		{
			Name:        "worker_cancel",
			Description: "Cancel a running background worker",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workerId": map[string]interface{}{
						"type":        "string",
						"description": "Worker ID to cancel",
					},
				},
				"required": []string{"workerId"},
			},
		},
		{
			Name:        "worker_triggers",
			Description: "List all available worker trigger types",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "worker_results",
			Description: "Get results from completed workers",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "Filter by session ID",
					},
					"trigger": map[string]interface{}{
						"type":        "string",
						"enum":        triggerEnums,
						"description": "Filter by trigger type",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"pending", "running", "completed", "failed", "cancelled"},
						"description": "Filter by status",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum results to return (default: 10)",
					},
				},
			},
		},
		{
			Name:        "worker_stats",
			Description: "Get aggregated worker statistics",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "worker_context",
			Description: "Get worker results formatted for prompt injection",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "Session ID to get context for",
					},
				},
				"required": []string{"sessionId"},
			},
		},
		{
			Name:        "worker_pool",
			Description: "Manage the worker pool (get status, resize, scale)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"get", "resize", "scale_up", "scale_down", "auto_scale"},
						"description": "Action to perform",
					},
					"size": map[string]interface{}{
						"type":        "number",
						"description": "New pool size (for resize action)",
					},
					"delta": map[string]interface{}{
						"type":        "number",
						"description": "Number of workers to add/remove (for scale_up/scale_down actions)",
					},
					"enabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable/disable auto-scaling (for auto_scale action)",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "worker_health",
			Description: "Check worker or pool health status",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workerId": map[string]interface{}{
						"type":        "string",
						"description": "Worker ID to check (omit for pool health)",
					},
				},
			},
		},
	}
}

// Execute executes a worker tool.
func (t *WorkerTools) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	switch toolName {
	case "worker_dispatch":
		return t.handleDispatch(ctx, params)
	case "worker_status":
		return t.handleStatus(ctx, params)
	case "worker_cancel":
		return t.handleCancel(ctx, params)
	case "worker_triggers":
		return t.handleTriggers(ctx, params)
	case "worker_results":
		return t.handleResults(ctx, params)
	case "worker_stats":
		return t.handleStats(ctx, params)
	case "worker_context":
		return t.handleContext(ctx, params)
	case "worker_pool":
		return t.handlePool(ctx, params)
	case "worker_health":
		return t.handleHealth(ctx, params)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (t *WorkerTools) handleDispatch(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	triggerStr, ok := params["trigger"].(string)
	if !ok || triggerStr == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: trigger is required",
		}, nil
	}

	trigger := domainWorker.WorkerTrigger(triggerStr)
	if !trigger.IsValid() {
		return &shared.MCPToolResult{
			Success: false,
			Error:   fmt.Sprintf("validation: invalid trigger '%s'", triggerStr),
		}, nil
	}

	workerContext, ok := params["context"].(string)
	if !ok || workerContext == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: context is required",
		}, nil
	}

	sessionID, _ := params["sessionId"].(string)

	options := &domainWorker.DispatchOptions{}

	if priorityStr, ok := params["priority"].(string); ok {
		options.Priority = domainWorker.WorkerPriority(priorityStr)
	}

	if timeoutMs, ok := params["timeout"].(float64); ok {
		options.Timeout = time.Duration(timeoutMs) * time.Millisecond
	}

	workerID, err := t.dispatcher.Dispatch(ctx, trigger, workerContext, sessionID, options)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Get trigger config for estimated duration
	triggers := t.dispatcher.GetTriggers()
	triggerConfig := triggers[trigger]

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"workerId":          workerID,
			"trigger":           string(trigger),
			"status":            string(domainWorker.StatusPending),
			"startedAt":         "now",
			"estimatedDuration": fmt.Sprintf("%ds", int(triggerConfig.EstimatedDuration.Seconds())),
		},
	}, nil
}

func (t *WorkerTools) handleStatus(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	workerID, ok := params["workerId"].(string)
	if !ok || workerID == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: workerId is required",
		}, nil
	}

	worker, found := t.dispatcher.GetWorker(workerID)
	if !found {
		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"found":    false,
				"workerId": workerID,
			},
		}, nil
	}

	data := map[string]interface{}{
		"found":     true,
		"workerId":  worker.ID,
		"trigger":   string(worker.Trigger),
		"status":    string(worker.Status),
		"progress":  worker.Progress,
		"phase":     worker.Phase,
		"startedAt": worker.StartedAt.String(),
	}

	if worker.CompletedAt != nil {
		data["completedAt"] = worker.CompletedAt.String()
	}

	if worker.Result != nil {
		data["result"] = worker.Result
	}

	if worker.Error != "" {
		data["error"] = worker.Error
	}

	return &shared.MCPToolResult{
		Success: true,
		Data:    data,
	}, nil
}

func (t *WorkerTools) handleCancel(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	workerID, ok := params["workerId"].(string)
	if !ok || workerID == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: workerId is required",
		}, nil
	}

	cancelled, err := t.dispatcher.Cancel(workerID)
	reason := "Cancelled by user"
	if err != nil {
		reason = err.Error()
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"cancelled": cancelled,
			"workerId":  workerID,
			"reason":    reason,
		},
	}, nil
}

func (t *WorkerTools) handleTriggers(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	triggerList := t.dispatcher.GetTriggerList()

	triggers := make([]map[string]interface{}, 0, len(triggerList))
	for _, config := range triggerList {
		triggers = append(triggers, map[string]interface{}{
			"name":              string(config.Name),
			"description":       config.Description,
			"priority":          string(config.Priority),
			"estimatedDuration": fmt.Sprintf("%ds", int(config.EstimatedDuration.Seconds())),
			"capabilities":      config.Capabilities,
		})
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"triggers": triggers,
		},
	}, nil
}

func (t *WorkerTools) handleResults(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	sessionID, _ := params["sessionId"].(string)

	var trigger *domainWorker.WorkerTrigger
	if triggerStr, ok := params["trigger"].(string); ok && triggerStr != "" {
		t := domainWorker.WorkerTrigger(triggerStr)
		trigger = &t
	}

	var status *domainWorker.WorkerStatus
	if statusStr, ok := params["status"].(string); ok && statusStr != "" {
		s := domainWorker.WorkerStatus(statusStr)
		status = &s
	}

	limit := 10
	if limitVal, ok := params["limit"].(float64); ok {
		limit = int(limitVal)
	}

	workers := t.dispatcher.FilterWorkers(sessionID, trigger, status, limit)

	results := make([]map[string]interface{}, 0, len(workers))
	for _, w := range workers {
		result := map[string]interface{}{
			"workerId":  w.ID,
			"trigger":   string(w.Trigger),
			"status":    string(w.Status),
			"progress":  w.Progress,
			"startedAt": w.StartedAt.String(),
		}

		if w.CompletedAt != nil {
			result["completedAt"] = w.CompletedAt.String()
		}

		if w.Result != nil && w.Result.Summary != "" {
			result["summary"] = w.Result.Summary
		}

		results = append(results, result)
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"results": results,
			"total":   len(results),
		},
	}, nil
}

func (t *WorkerTools) handleStats(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	stats := t.dispatcher.GetStats()

	byTrigger := make(map[string]int)
	for trigger, count := range stats.ByTrigger {
		byTrigger[string(trigger)] = count
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"total":         stats.Total,
			"pending":       stats.Pending,
			"running":       stats.Running,
			"completed":     stats.Completed,
			"failed":        stats.Failed,
			"cancelled":     stats.Cancelled,
			"byTrigger":     byTrigger,
			"avgDurationMs": stats.AvgDurationMs,
			"successRate":   stats.SuccessRate,
		},
	}, nil
}

func (t *WorkerTools) handleContext(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	sessionID, ok := params["sessionId"].(string)
	if !ok || sessionID == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: sessionId is required",
		}, nil
	}

	context := t.dispatcher.GetContextForInjection(sessionID)
	workers := t.dispatcher.GetSessionWorkers(sessionID)

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"context":     context,
			"workerCount": len(workers),
			"hasResults":  len(context) > 0,
		},
	}, nil
}

func (t *WorkerTools) handlePool(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	action, ok := params["action"].(string)
	if !ok || action == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: action is required",
		}, nil
	}

	switch action {
	case "get":
		pool := t.poolMgr.GetPoolInfo()
		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"size":               pool.Size,
				"minWorkers":         pool.MinWorkers,
				"maxWorkers":         pool.MaxWorkers,
				"activeWorkers":      pool.ActiveWorkers,
				"idleWorkers":        pool.IdleWorkers,
				"pendingTasks":       pool.PendingTasks,
				"autoScale":          pool.AutoScale,
				"scaleUpThreshold":   pool.ScaleUpThreshold,
				"scaleDownThreshold": pool.ScaleDownThreshold,
				"load":               pool.Load(),
			},
		}, nil

	case "resize":
		sizeVal, ok := params["size"].(float64)
		if !ok {
			return &shared.MCPToolResult{
				Success: false,
				Error:   "validation: size is required for resize action",
			}, nil
		}

		if err := t.poolMgr.SetPoolSize(int(sizeVal)); err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		pool := t.poolMgr.GetPoolInfo()
		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"action":  "resize",
				"newSize": pool.Size,
				"message": fmt.Sprintf("Pool resized to %d workers", pool.Size),
			},
		}, nil

	case "scale_up":
		delta := 1
		if deltaVal, ok := params["delta"].(float64); ok {
			delta = int(deltaVal)
		}

		if err := t.poolMgr.ScaleUp(delta); err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		pool := t.poolMgr.GetPoolInfo()
		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"action":  "scale_up",
				"delta":   delta,
				"newSize": pool.Size,
				"message": fmt.Sprintf("Scaled up by %d workers to %d total", delta, pool.Size),
			},
		}, nil

	case "scale_down":
		delta := 1
		if deltaVal, ok := params["delta"].(float64); ok {
			delta = int(deltaVal)
		}

		if err := t.poolMgr.ScaleDown(delta); err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		pool := t.poolMgr.GetPoolInfo()
		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"action":  "scale_down",
				"delta":   delta,
				"newSize": pool.Size,
				"message": fmt.Sprintf("Scaled down by %d workers to %d total", delta, pool.Size),
			},
		}, nil

	case "auto_scale":
		enabled, ok := params["enabled"].(bool)
		if !ok {
			// Toggle if not specified
			pool := t.poolMgr.GetPoolInfo()
			enabled = !pool.AutoScale
		}

		t.poolMgr.SetAutoScale(enabled)

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"action":    "auto_scale",
				"enabled":   enabled,
				"message":   fmt.Sprintf("Auto-scaling %s", boolToEnabledStr(enabled)),
			},
		}, nil

	default:
		return &shared.MCPToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

func (t *WorkerTools) handleHealth(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	workerID, hasWorkerID := params["workerId"].(string)

	if hasWorkerID && workerID != "" {
		// Check specific worker health
		health := t.poolMgr.GetWorkerHealth(workerID)

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"workerId":     health.WorkerID,
				"status":       string(health.Status),
				"message":      health.Message,
				"uptimeMs":     health.Uptime.Milliseconds(),
				"lastActivity": health.LastActivity,
				"checkedAt":    health.CheckedAt.String(),
				"diagnostics":  health.Diagnostics,
			},
		}, nil
	}

	// Check pool health
	health := t.poolMgr.GetHealth()

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"status":      string(health.Status),
			"message":     health.Message,
			"checkedAt":   health.CheckedAt.String(),
			"diagnostics": health.Diagnostics,
		},
	}, nil
}

// GetDispatcher returns the worker dispatcher.
func (t *WorkerTools) GetDispatcher() *infraWorker.WorkerDispatchService {
	return t.dispatcher
}

// GetPoolManager returns the pool manager.
func (t *WorkerTools) GetPoolManager() *infraWorker.PoolManager {
	return t.poolMgr
}

// Helper function
func boolToEnabledStr(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}
