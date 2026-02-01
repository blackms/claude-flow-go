// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/mcp/tasks"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// TaskTools provides MCP tools for task management operations.
type TaskTools struct {
	manager *tasks.TaskManager
}

// NewTaskTools creates a new TaskTools instance.
func NewTaskTools(manager *tasks.TaskManager) *TaskTools {
	return &TaskTools{
		manager: manager,
	}
}

// GetTools returns available task tools.
func (t *TaskTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "tasks/create",
			Description: "Create a new task with type, description, priority, and optional dependencies",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Task type: code, test, review, design, deploy, workflow",
						"enum":        []string{"code", "test", "review", "design", "deploy", "workflow"},
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Task description",
					},
					"priority": map[string]interface{}{
						"type":        "number",
						"description": "Priority level (1-10, higher is more important)",
						"minimum":     1,
						"maximum":     10,
					},
					"dependencies": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "List of task IDs this task depends on",
					},
					"assignToAgent": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID to assign the task to",
					},
					"input": map[string]interface{}{
						"type":        "object",
						"description": "Input data for the task",
					},
					"timeoutMs": map[string]interface{}{
						"type":        "number",
						"description": "Timeout in milliseconds (default: 300000)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Additional metadata",
					},
				},
				"required": []string{"type", "description"},
			},
		},
		{
			Name:        "tasks/list",
			Description: "List tasks with optional filtering and pagination",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status",
						"enum":        []string{"pending", "queued", "running", "completed", "failed", "cancelled"},
					},
					"agentId": map[string]interface{}{
						"type":        "string",
						"description": "Filter by assigned agent ID",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by task type",
					},
					"priority": map[string]interface{}{
						"type":        "number",
						"description": "Filter by priority level",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of tasks to return (default: 100, max: 1000)",
					},
					"offset": map[string]interface{}{
						"type":        "number",
						"description": "Offset for pagination",
					},
					"sortBy": map[string]interface{}{
						"type":        "string",
						"description": "Sort field: created, priority, status, updated",
						"enum":        []string{"created", "priority", "status", "updated"},
					},
					"sortOrder": map[string]interface{}{
						"type":        "string",
						"description": "Sort order: asc, desc",
						"enum":        []string{"asc", "desc"},
					},
				},
			},
		},
		{
			Name:        "tasks/status",
			Description: "Get detailed status of a task including progress and timing information",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"taskId": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
					"includeMetrics": map[string]interface{}{
						"type":        "boolean",
						"description": "Include execution metrics",
					},
					"includeHistory": map[string]interface{}{
						"type":        "boolean",
						"description": "Include task history",
					},
				},
				"required": []string{"taskId"},
			},
		},
		{
			Name:        "tasks/cancel",
			Description: "Cancel a pending or running task",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"taskId": map[string]interface{}{
						"type":        "string",
						"description": "Task ID to cancel",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Reason for cancellation",
					},
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "Force cancellation even if task is in a non-cancellable state",
					},
				},
				"required": []string{"taskId"},
			},
		},
		{
			Name:        "tasks/assign",
			Description: "Assign a task to an agent",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"taskId": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
					"agentId": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID to assign the task to",
					},
					"reassign": map[string]interface{}{
						"type":        "boolean",
						"description": "Allow reassignment if already assigned",
					},
				},
				"required": []string{"taskId", "agentId"},
			},
		},
		{
			Name:        "tasks/update",
			Description: "Update task properties like priority, description, or timeout",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"taskId": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
					"priority": map[string]interface{}{
						"type":        "number",
						"description": "New priority (1-10)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description",
					},
					"timeoutMs": map[string]interface{}{
						"type":        "number",
						"description": "New timeout in milliseconds",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Metadata to merge",
					},
				},
				"required": []string{"taskId"},
			},
		},
		{
			Name:        "tasks/dependencies",
			Description: "Manage task dependencies (add, remove, list, or clear)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"taskId": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform",
						"enum":        []string{"add", "remove", "list", "clear"},
					},
					"dependencies": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Task IDs for add/remove actions",
					},
				},
				"required": []string{"taskId", "action"},
			},
		},
		{
			Name:        "tasks/results",
			Description: "Get the results of a completed task",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"taskId": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": "Result format: summary, detailed, raw",
						"enum":        []string{"summary", "detailed", "raw"},
					},
					"includeArtifacts": map[string]interface{}{
						"type":        "boolean",
						"description": "Include task artifacts in the result",
					},
				},
				"required": []string{"taskId"},
			},
		},
	}
}

// Execute executes a task tool.
func (t *TaskTools) Execute(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	switch method {
	case "tasks/create":
		return t.createTask(ctx, params)
	case "tasks/list":
		return t.listTasks(ctx, params)
	case "tasks/status":
		return t.getTaskStatus(ctx, params)
	case "tasks/cancel":
		return t.cancelTask(ctx, params)
	case "tasks/assign":
		return t.assignTask(ctx, params)
	case "tasks/update":
		return t.updateTask(ctx, params)
	case "tasks/dependencies":
		return t.manageDependencies(ctx, params)
	case "tasks/results":
		return t.getResults(ctx, params)
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

func (t *TaskTools) createTask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	req := &shared.TaskCreateRequest{}

	if taskType, ok := params["type"].(string); ok {
		req.Type = shared.TaskType(taskType)
	} else {
		return nil, fmt.Errorf("type is required")
	}

	if desc, ok := params["description"].(string); ok {
		req.Description = desc
	} else {
		return nil, fmt.Errorf("description is required")
	}

	if priority, ok := params["priority"].(float64); ok {
		req.Priority = int(priority)
	}

	if deps, ok := params["dependencies"].([]interface{}); ok {
		for _, d := range deps {
			if depID, ok := d.(string); ok {
				req.Dependencies = append(req.Dependencies, depID)
			}
		}
	}

	if agentID, ok := params["assignToAgent"].(string); ok {
		req.AssignToAgent = agentID
	}

	if input, ok := params["input"]; ok {
		req.Input = input
	}

	if timeout, ok := params["timeoutMs"].(float64); ok {
		req.TimeoutMs = int64(timeout)
	}

	if metadata, ok := params["metadata"].(map[string]interface{}); ok {
		req.Metadata = metadata
	}

	task, err := t.manager.CreateTask(req)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"taskId":        task.ID,
		"status":        task.Status,
		"createdAt":     task.CreatedAt,
		"queuePosition": task.QueuePosition,
	}, nil
}

func (t *TaskTools) listTasks(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	filter := &shared.TaskFilter{}

	if status, ok := params["status"].(string); ok {
		filter.Status = shared.ManagedTaskStatus(status)
	}

	if agentID, ok := params["agentId"].(string); ok {
		filter.AgentID = agentID
	}

	if taskType, ok := params["type"].(string); ok {
		filter.Type = shared.TaskType(taskType)
	}

	if priority, ok := params["priority"].(float64); ok {
		filter.Priority = int(priority)
	}

	if limit, ok := params["limit"].(float64); ok {
		filter.Limit = int(limit)
		if filter.Limit > 1000 {
			filter.Limit = 1000
		}
	}

	if offset, ok := params["offset"].(float64); ok {
		filter.Offset = int(offset)
	}

	if sortBy, ok := params["sortBy"].(string); ok {
		filter.SortBy = sortBy
	}

	if sortOrder, ok := params["sortOrder"].(string); ok {
		filter.SortOrder = sortOrder
	}

	result := t.manager.ListTasks(filter)

	// Convert to response format
	taskList := make([]map[string]interface{}, len(result.Tasks))
	for i, task := range result.Tasks {
		taskList[i] = t.taskToMap(task, false, false)
	}

	return map[string]interface{}{
		"tasks":  taskList,
		"total":  result.Total,
		"limit":  result.Limit,
		"offset": result.Offset,
	}, nil
}

func (t *TaskTools) getTaskStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, ok := params["taskId"].(string)
	if !ok {
		return nil, fmt.Errorf("taskId is required")
	}

	includeMetrics := false
	if m, ok := params["includeMetrics"].(bool); ok {
		includeMetrics = m
	}

	includeHistory := false
	if h, ok := params["includeHistory"].(bool); ok {
		includeHistory = h
	}

	task, err := t.manager.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	return t.taskToMap(task, includeMetrics, includeHistory), nil
}

func (t *TaskTools) cancelTask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, ok := params["taskId"].(string)
	if !ok {
		return nil, fmt.Errorf("taskId is required")
	}

	reason := ""
	if r, ok := params["reason"].(string); ok {
		reason = r
	}

	force := false
	if f, ok := params["force"].(bool); ok {
		force = f
	}

	// Get previous status
	task, err := t.manager.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	prevStatus := task.Status

	if err := t.manager.CancelTask(taskID, reason, force); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"taskId":         taskID,
		"cancelled":      true,
		"previousStatus": prevStatus,
		"reason":         reason,
	}, nil
}

func (t *TaskTools) assignTask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, ok := params["taskId"].(string)
	if !ok {
		return nil, fmt.Errorf("taskId is required")
	}

	agentID, ok := params["agentId"].(string)
	if !ok {
		return nil, fmt.Errorf("agentId is required")
	}

	reassign := false
	if r, ok := params["reassign"].(bool); ok {
		reassign = r
	}

	// Get previous assignment
	task, err := t.manager.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	prevAgent := task.AssignedTo

	if err := t.manager.AssignTask(taskID, agentID, reassign); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"taskId":        taskID,
		"assigned":      true,
		"agentId":       agentID,
		"previousAgent": prevAgent,
	}, nil
}

func (t *TaskTools) updateTask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, ok := params["taskId"].(string)
	if !ok {
		return nil, fmt.Errorf("taskId is required")
	}

	update := &shared.TaskUpdate{}
	changes := make(map[string]bool)

	if priority, ok := params["priority"].(float64); ok {
		p := int(priority)
		update.Priority = &p
		changes["priority"] = true
	}

	if desc, ok := params["description"].(string); ok {
		update.Description = &desc
		changes["description"] = true
	}

	if timeout, ok := params["timeoutMs"].(float64); ok {
		to := int64(timeout)
		update.TimeoutMs = &to
		changes["timeoutMs"] = true
	}

	if metadata, ok := params["metadata"].(map[string]interface{}); ok {
		update.Metadata = metadata
		changes["metadata"] = true
	}

	if err := t.manager.UpdateTask(taskID, update); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"taskId":  taskID,
		"updated": true,
		"changes": changes,
	}, nil
}

func (t *TaskTools) manageDependencies(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, ok := params["taskId"].(string)
	if !ok {
		return nil, fmt.Errorf("taskId is required")
	}

	actionStr, ok := params["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action is required")
	}

	action := shared.TaskDependencyAction(actionStr)

	var deps []string
	if d, ok := params["dependencies"].([]interface{}); ok {
		for _, dep := range d {
			if depID, ok := dep.(string); ok {
				deps = append(deps, depID)
			}
		}
	}

	result, err := t.manager.UpdateDependencies(taskID, action, deps)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"taskId":       taskID,
		"action":       action,
		"dependencies": result,
	}, nil
}

func (t *TaskTools) getResults(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, ok := params["taskId"].(string)
	if !ok {
		return nil, fmt.Errorf("taskId is required")
	}

	format := shared.TaskResultFormatSummary
	if f, ok := params["format"].(string); ok {
		format = shared.TaskResultFormat(f)
	}

	includeArtifacts := false
	if a, ok := params["includeArtifacts"].(bool); ok {
		includeArtifacts = a
	}

	result, err := t.manager.GetResults(taskID, format, includeArtifacts)
	if err != nil {
		return nil, err
	}

	response := map[string]interface{}{
		"taskId":          result.TaskID,
		"status":          result.Status,
		"executionTimeMs": result.ExecutionTime,
		"completedAt":     result.CompletedAt,
	}

	if result.Output != nil {
		response["output"] = result.Output
	}

	if result.Error != "" {
		response["error"] = result.Error
	}

	if len(result.Artifacts) > 0 {
		artifacts := make([]map[string]interface{}, len(result.Artifacts))
		for i, a := range result.Artifacts {
			artifacts[i] = map[string]interface{}{
				"name":     a.Name,
				"type":     a.Type,
				"path":     a.Path,
				"size":     a.Size,
				"mimeType": a.MimeType,
			}
		}
		response["artifacts"] = artifacts
	}

	return response, nil
}

func (t *TaskTools) taskToMap(task *shared.ManagedTask, includeMetrics, includeHistory bool) map[string]interface{} {
	result := map[string]interface{}{
		"id":            task.ID,
		"type":          task.Type,
		"description":   task.Description,
		"priority":      task.Priority,
		"status":        task.Status,
		"progress":      task.Progress,
		"createdAt":     task.CreatedAt,
		"queuePosition": task.QueuePosition,
	}

	if task.AssignedTo != "" {
		result["assignedTo"] = task.AssignedTo
	}

	if len(task.Dependencies) > 0 {
		result["dependencies"] = task.Dependencies
	}

	if task.StartedAt > 0 {
		result["startedAt"] = task.StartedAt
	}

	if task.CompletedAt > 0 {
		result["completedAt"] = task.CompletedAt
	}

	if task.Error != "" {
		result["error"] = task.Error
	}

	if task.TimeoutMs > 0 {
		result["timeoutMs"] = task.TimeoutMs
	}

	if includeMetrics && task.Metrics != nil {
		result["metrics"] = map[string]interface{}{
			"executionTimeMs": task.Metrics.ExecutionTimeMs,
			"waitTimeMs":      task.Metrics.WaitTimeMs,
			"retryCount":      task.Metrics.RetryCount,
		}
	}

	if includeHistory && len(task.History) > 0 {
		history := make([]map[string]interface{}, len(task.History))
		for i, h := range task.History {
			history[i] = map[string]interface{}{
				"timestamp": h.Timestamp,
				"event":     h.Event,
				"details":   h.Details,
			}
		}
		result["history"] = history
	}

	return result
}
