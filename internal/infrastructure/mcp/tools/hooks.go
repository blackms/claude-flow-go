// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/hooks"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// HooksTools provides MCP tools for hooks operations.
type HooksTools struct {
	manager *hooks.HooksManager
}

// NewHooksTools creates a new HooksTools instance.
func NewHooksTools(manager *hooks.HooksManager) *HooksTools {
	return &HooksTools{
		manager: manager,
	}
}

// GetTools returns available hooks tools.
func (t *HooksTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "hooks/pre-edit",
			Description: "Pre-edit hook for file validation and context retrieval",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filePath": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file being edited",
					},
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Type of edit operation (create, modify, delete)",
					},
					"includeContext": map[string]interface{}{
						"type":        "boolean",
						"description": "Include file context in response",
					},
					"includeSuggestions": map[string]interface{}{
						"type":        "boolean",
						"description": "Include suggestions based on similar patterns",
					},
				},
				"required": []string{"filePath", "operation"},
			},
		},
		{
			Name:        "hooks/post-edit",
			Description: "Post-edit hook for learning from edit outcomes",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filePath": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file that was edited",
					},
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Type of edit operation performed",
					},
					"success": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the edit was successful",
					},
					"outcome": map[string]interface{}{
						"type":        "string",
						"description": "Description of the outcome",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Additional metadata about the edit",
					},
				},
				"required": []string{"filePath", "operation", "success"},
			},
		},
		{
			Name:        "hooks/pre-command",
			Description: "Pre-command hook for risk assessment",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Command to be executed",
					},
					"workingDirectory": map[string]interface{}{
						"type":        "string",
						"description": "Working directory for the command",
					},
					"includeRiskAssessment": map[string]interface{}{
						"type":        "boolean",
						"description": "Include risk assessment in response",
					},
					"includeSuggestions": map[string]interface{}{
						"type":        "boolean",
						"description": "Include safer alternatives",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "hooks/post-command",
			Description: "Post-command hook for learning from command outcomes",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Command that was executed",
					},
					"exitCode": map[string]interface{}{
						"type":        "number",
						"description": "Exit code of the command",
					},
					"success": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the command was successful",
					},
					"output": map[string]interface{}{
						"type":        "string",
						"description": "Command output (optional)",
					},
					"error": map[string]interface{}{
						"type":        "string",
						"description": "Error message if failed",
					},
					"executionTimeMs": map[string]interface{}{
						"type":        "number",
						"description": "Execution time in milliseconds",
					},
				},
				"required": []string{"command", "exitCode", "success"},
			},
		},
		{
			Name:        "hooks/route",
			Description: "Route a task to the optimal agent based on learning",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
						"type":        "string",
						"description": "Task description",
					},
					"context": map[string]interface{}{
						"type":        "object",
						"description": "Additional context for routing",
					},
					"preferredAgents": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "List of preferred agent types",
					},
					"constraints": map[string]interface{}{
						"type":        "object",
						"description": "Routing constraints",
					},
					"includeExplanation": map[string]interface{}{
						"type":        "boolean",
						"description": "Include detailed explanation",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			Name:        "hooks/explain",
			Description: "Explain routing decisions for a task",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
						"type":        "string",
						"description": "Task to explain routing for",
					},
					"context": map[string]interface{}{
						"type":        "object",
						"description": "Additional context",
					},
					"verbose": map[string]interface{}{
						"type":        "boolean",
						"description": "Include detailed historical data",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			Name:        "hooks/pretrain",
			Description: "Bootstrap intelligence from repository patterns",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repositoryPath": map[string]interface{}{
						"type":        "string",
						"description": "Path to repository to analyze",
					},
					"maxPatterns": map[string]interface{}{
						"type":        "number",
						"description": "Maximum patterns to extract",
					},
					"includeGitHistory": map[string]interface{}{
						"type":        "boolean",
						"description": "Include git commit history analysis",
					},
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "Force re-extraction of patterns",
					},
				},
				"required": []string{"repositoryPath"},
			},
		},
		{
			Name:        "hooks/metrics",
			Description: "Get learning and hook execution metrics",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Metrics category: all, routing, patterns, hooks",
						"enum":        []string{"all", "routing", "patterns", "hooks"},
					},
					"timeRange": map[string]interface{}{
						"type":        "string",
						"description": "Time range: hour, day, week, all",
						"enum":        []string{"hour", "day", "week", "all"},
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": "Output format: summary, detailed",
						"enum":        []string{"summary", "detailed"},
					},
				},
			},
		},
		{
			Name:        "hooks/list",
			Description: "List registered hooks",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Filter by hook event category",
					},
					"includeDisabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Include disabled hooks",
					},
					"includeMetadata": map[string]interface{}{
						"type":        "boolean",
						"description": "Include hook metadata and stats",
					},
				},
			},
		},
	}
}

// Execute executes a hooks tool.
func (t *HooksTools) Execute(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	switch method {
	case "hooks/pre-edit":
		return t.preEdit(ctx, params)
	case "hooks/post-edit":
		return t.postEdit(ctx, params)
	case "hooks/pre-command":
		return t.preCommand(ctx, params)
	case "hooks/post-command":
		return t.postCommand(ctx, params)
	case "hooks/route":
		return t.route(ctx, params)
	case "hooks/explain":
		return t.explain(ctx, params)
	case "hooks/pretrain":
		return t.pretrain(ctx, params)
	case "hooks/metrics":
		return t.metrics(ctx, params)
	case "hooks/list":
		return t.list(ctx, params)
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

func (t *HooksTools) preEdit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	filePath, _ := params["filePath"].(string)
	operation, _ := params["operation"].(string)
	includeContext, _ := params["includeContext"].(bool)
	includeSuggestions, _ := params["includeSuggestions"].(bool)

	if filePath == "" {
		return nil, fmt.Errorf("filePath is required")
	}
	if operation == "" {
		return nil, fmt.Errorf("operation is required")
	}

	// Find similar patterns
	patternStore := t.manager.GetPatternStore()
	query := filePath + " " + operation
	similarPatterns := patternStore.FindSimilar(query, shared.PatternTypeEdit, 5)

	result := &shared.PreEditResult{
		FilePath:        filePath,
		SimilarPatterns: similarPatterns,
	}

	// Generate warnings based on patterns
	for _, pattern := range similarPatterns {
		if pattern.GetSuccessRate() < 0.5 {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("Similar edit pattern '%s' has low success rate (%.0f%%)", 
					pattern.Content, pattern.GetSuccessRate()*100))
		}
	}

	// Generate suggestions
	if includeSuggestions {
		for _, pattern := range similarPatterns {
			if pattern.GetSuccessRate() > 0.8 {
				result.Suggestions = append(result.Suggestions,
					fmt.Sprintf("Previous successful pattern: %s", pattern.Content))
			}
		}
	}

	// Add context
	if includeContext {
		result.Context = map[string]interface{}{
			"operation":      operation,
			"patternCount":   len(similarPatterns),
			"hasHighRiskPatterns": len(result.Warnings) > 0,
		}
	}

	// Execute pre-edit hooks
	hookCtx := &shared.HookContext{
		Event:     shared.HookEventPreEdit,
		Data:      result,
		Timestamp: shared.Now(),
	}
	t.manager.Execute(ctx, hookCtx)

	return result, nil
}

func (t *HooksTools) postEdit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	filePath, _ := params["filePath"].(string)
	operation, _ := params["operation"].(string)
	success, _ := params["success"].(bool)
	outcome, _ := params["outcome"].(string)
	metadata, _ := params["metadata"].(map[string]interface{})

	if filePath == "" {
		return nil, fmt.Errorf("filePath is required")
	}
	if operation == "" {
		return nil, fmt.Errorf("operation is required")
	}

	// Create and store pattern
	pattern := hooks.CreateEditPattern(filePath, operation, success, metadata)
	
	patternStore := t.manager.GetPatternStore()
	if err := patternStore.Store(pattern); err != nil {
		return nil, err
	}

	result := &shared.PostEditResult{
		Recorded:  true,
		PatternID: pattern.ID,
		Message:   fmt.Sprintf("Edit pattern recorded: %s", operation),
	}

	// Execute post-edit hooks
	hookCtx := &shared.HookContext{
		Event: shared.HookEventPostEdit,
		Data: map[string]interface{}{
			"filePath":  filePath,
			"operation": operation,
			"success":   success,
			"outcome":   outcome,
			"patternId": pattern.ID,
		},
		Timestamp: shared.Now(),
	}
	t.manager.Execute(ctx, hookCtx)

	return result, nil
}

func (t *HooksTools) preCommand(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command, _ := params["command"].(string)
	workingDir, _ := params["workingDirectory"].(string)
	includeRiskAssessment, _ := params["includeRiskAssessment"].(bool)
	includeSuggestions, _ := params["includeSuggestions"].(bool)

	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Find similar patterns
	patternStore := t.manager.GetPatternStore()
	similarPatterns := patternStore.FindSimilar(command, shared.PatternTypeCommand, 5)

	result := &shared.PreCommandResult{
		Command:        command,
		SimilarPatterns: similarPatterns,
	}

	// Perform risk assessment
	if includeRiskAssessment {
		result.RiskAssessment = t.assessCommandRisk(command, workingDir)
	}

	// Generate suggestions
	if includeSuggestions {
		result.Suggestions = t.generateCommandSuggestions(command, similarPatterns)
	}

	// Execute pre-command hooks
	hookCtx := &shared.HookContext{
		Event: shared.HookEventPreCommand,
		Data: map[string]interface{}{
			"command":          command,
			"workingDirectory": workingDir,
			"riskAssessment":   result.RiskAssessment,
		},
		Timestamp: shared.Now(),
	}
	t.manager.Execute(ctx, hookCtx)

	return result, nil
}

func (t *HooksTools) postCommand(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command, _ := params["command"].(string)
	exitCode := 0
	if ec, ok := params["exitCode"].(float64); ok {
		exitCode = int(ec)
	}
	success, _ := params["success"].(bool)
	executionTime := int64(0)
	if et, ok := params["executionTimeMs"].(float64); ok {
		executionTime = int64(et)
	}

	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Create and store pattern
	pattern := hooks.CreateCommandPattern(command, success, exitCode, executionTime)
	
	patternStore := t.manager.GetPatternStore()
	if err := patternStore.Store(pattern); err != nil {
		return nil, err
	}

	result := &shared.PostCommandResult{
		Recorded:  true,
		PatternID: pattern.ID,
		Message:   fmt.Sprintf("Command pattern recorded: exit code %d", exitCode),
	}

	// Execute post-command hooks
	hookCtx := &shared.HookContext{
		Event: shared.HookEventPostCommand,
		Data: map[string]interface{}{
			"command":       command,
			"exitCode":      exitCode,
			"success":       success,
			"executionTime": executionTime,
			"patternId":     pattern.ID,
		},
		Timestamp: shared.Now(),
	}
	t.manager.Execute(ctx, hookCtx)

	return result, nil
}

func (t *HooksTools) route(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	taskContext, _ := params["context"].(map[string]interface{})
	constraints, _ := params["constraints"].(map[string]interface{})
	includeExplanation, _ := params["includeExplanation"].(bool)

	if task == "" {
		return nil, fmt.Errorf("task is required")
	}

	var preferredAgents []string
	if pa, ok := params["preferredAgents"].([]interface{}); ok {
		for _, p := range pa {
			if s, ok := p.(string); ok {
				preferredAgents = append(preferredAgents, s)
			}
		}
	}

	routingEngine := t.manager.GetRoutingEngine()
	result := routingEngine.Route(task, taskContext, preferredAgents, constraints)

	// Execute pre-route hooks
	hookCtx := &shared.HookContext{
		Event: shared.HookEventPreRoute,
		Data: map[string]interface{}{
			"task":           task,
			"recommendedAgent": result.RecommendedAgent,
			"confidence":     result.Confidence,
		},
		Timestamp: shared.Now(),
	}
	t.manager.Execute(ctx, hookCtx)

	response := map[string]interface{}{
		"routingId":        result.ID,
		"recommendedAgent": result.RecommendedAgent,
		"confidence":       result.Confidence,
		"alternatives":     result.Alternatives,
	}

	if includeExplanation {
		response["explanation"] = result.Explanation
		response["factors"] = result.Factors
	}

	return response, nil
}

func (t *HooksTools) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	taskContext, _ := params["context"].(map[string]interface{})
	verbose, _ := params["verbose"].(bool)

	if task == "" {
		return nil, fmt.Errorf("task is required")
	}

	routingEngine := t.manager.GetRoutingEngine()
	explanation := routingEngine.Explain(task, taskContext, verbose)

	return map[string]interface{}{
		"task":           explanation.Task,
		"analysis":       explanation.Analysis,
		"factors":        explanation.Factors,
		"historicalData": explanation.HistoricalData,
	}, nil
}

func (t *HooksTools) pretrain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	repoPath, _ := params["repositoryPath"].(string)
	maxPatterns := 1000
	if mp, ok := params["maxPatterns"].(float64); ok {
		maxPatterns = int(mp)
	}
	includeGitHistory, _ := params["includeGitHistory"].(bool)
	force, _ := params["force"].(bool)

	if repoPath == "" {
		return nil, fmt.Errorf("repositoryPath is required")
	}

	// For now, we simulate pattern extraction
	// In a real implementation, this would scan the repository
	patternStore := t.manager.GetPatternStore()
	
	stats := map[string]interface{}{
		"repositoryPath":    repoPath,
		"maxPatterns":       maxPatterns,
		"includeGitHistory": includeGitHistory,
		"force":             force,
		"patternsExtracted": 0,
		"existingPatterns":  patternStore.Count(),
		"message":           "Pretrain simulation complete. Implement repository scanning for full functionality.",
	}

	return stats, nil
}

func (t *HooksTools) metrics(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	category := "all"
	if c, ok := params["category"].(string); ok && c != "" {
		category = c
	}
	format := "summary"
	if f, ok := params["format"].(string); ok && f != "" {
		format = f
	}

	metrics := t.manager.GetMetrics()
	routingEngine := t.manager.GetRoutingEngine()
	patternStore := t.manager.GetPatternStore()

	response := map[string]interface{}{}

	switch category {
	case "routing":
		response["routing"] = map[string]interface{}{
			"totalRoutings":  metrics.RoutingCount,
			"successRate":    metrics.RoutingSuccessRate,
			"agentStats":     routingEngine.GetAgentStats(),
		}
	case "patterns":
		response["patterns"] = map[string]interface{}{
			"total":    metrics.PatternCount,
			"edit":     metrics.EditPatterns,
			"command":  metrics.CommandPatterns,
		}
	case "hooks":
		response["hooks"] = map[string]interface{}{
			"totalExecutions":    metrics.TotalExecutions,
			"successfulExecutions": metrics.SuccessfulExecutions,
			"failedExecutions":   metrics.FailedExecutions,
			"avgExecutionMs":     metrics.AvgExecutionMs,
			"byEvent":            metrics.HooksByEvent,
		}
	default: // "all"
		response = map[string]interface{}{
			"summary": map[string]interface{}{
				"totalHookExecutions": metrics.TotalExecutions,
				"totalPatterns":       metrics.PatternCount,
				"totalRoutings":       metrics.RoutingCount,
				"routingSuccessRate":  metrics.RoutingSuccessRate,
			},
		}
		if format == "detailed" {
			response["routing"] = map[string]interface{}{
				"totalRoutings": metrics.RoutingCount,
				"successRate":   metrics.RoutingSuccessRate,
				"agentStats":    routingEngine.GetAgentStats(),
			}
			response["patterns"] = map[string]interface{}{
				"total":   metrics.PatternCount,
				"edit":    patternStore.CountByType(shared.PatternTypeEdit),
				"command": patternStore.CountByType(shared.PatternTypeCommand),
			}
			response["hooks"] = map[string]interface{}{
				"totalExecutions":    metrics.TotalExecutions,
				"successfulExecutions": metrics.SuccessfulExecutions,
				"failedExecutions":   metrics.FailedExecutions,
				"avgExecutionMs":     metrics.AvgExecutionMs,
				"byEvent":            metrics.HooksByEvent,
			}
		}
	}

	return response, nil
}

func (t *HooksTools) list(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	category, _ := params["category"].(string)
	includeDisabled, _ := params["includeDisabled"].(bool)
	includeMetadata, _ := params["includeMetadata"].(bool)

	var event shared.HookEvent
	if category != "" {
		event = shared.HookEvent(category)
	}

	hookslist := t.manager.ListHooks(event, includeDisabled)

	var result []map[string]interface{}
	for _, hook := range hookslist {
		hookMap := map[string]interface{}{
			"id":          hook.ID,
			"name":        hook.Name,
			"event":       hook.Event,
			"priority":    hook.Priority,
			"enabled":     hook.Enabled,
			"description": hook.Description,
		}

		if includeMetadata {
			hookMap["createdAt"] = hook.CreatedAt
			hookMap["executionCount"] = hook.ExecutionCount
			hookMap["lastExecutedAt"] = hook.LastExecutedAt
			hookMap["avgExecutionMs"] = hook.AvgExecutionMs
			hookMap["metadata"] = hook.Metadata
		}

		result = append(result, hookMap)
	}

	return map[string]interface{}{
		"hooks": result,
		"total": len(result),
	}, nil
}

// assessCommandRisk assesses the risk of a command.
func (t *HooksTools) assessCommandRisk(command, workingDir string) *shared.RiskAssessment {
	assessment := &shared.RiskAssessment{
		Level:         shared.RiskLevelLow,
		Score:         0.1,
		ShouldProceed: true,
	}

	commandLower := strings.ToLower(command)

	// Check for dangerous patterns
	dangerousPatterns := map[string]string{
		"rm -rf":      "Recursive force delete",
		"rm -r /":     "Recursive delete from root",
		"dd if=":      "Low-level disk write",
		":(){":        "Fork bomb pattern",
		"> /dev/sda":  "Direct disk write",
		"chmod 777":   "Overly permissive permissions",
		"--no-verify": "Skipping verification",
		"--force":     "Force operation",
		"DROP TABLE":  "Database table deletion",
		"DELETE FROM": "Database record deletion",
	}

	for pattern, concern := range dangerousPatterns {
		if strings.Contains(commandLower, strings.ToLower(pattern)) {
			assessment.Concerns = append(assessment.Concerns, concern)
			assessment.Score += 0.3
		}
	}

	// Determine risk level
	if assessment.Score >= 0.7 {
		assessment.Level = shared.RiskLevelCritical
		assessment.ShouldProceed = false
		assessment.Recommendations = append(assessment.Recommendations, "Review command carefully before proceeding")
	} else if assessment.Score >= 0.5 {
		assessment.Level = shared.RiskLevelHigh
		assessment.Recommendations = append(assessment.Recommendations, "Consider using a safer alternative")
	} else if assessment.Score >= 0.3 {
		assessment.Level = shared.RiskLevelMedium
	}

	// Cap score at 1.0
	if assessment.Score > 1.0 {
		assessment.Score = 1.0
	}

	return assessment
}

// generateCommandSuggestions generates suggestions for a command.
func (t *HooksTools) generateCommandSuggestions(command string, patterns []*shared.Pattern) []string {
	var suggestions []string

	// Add suggestions from successful patterns
	for _, pattern := range patterns {
		if pattern.GetSuccessRate() > 0.8 {
			suggestions = append(suggestions, 
				fmt.Sprintf("Similar successful command: %s (%.0f%% success rate)", 
					pattern.Content, pattern.GetSuccessRate()*100))
		}
	}

	// Add general safety suggestions
	commandLower := strings.ToLower(command)
	if strings.Contains(commandLower, "rm") && !strings.Contains(commandLower, "-i") {
		suggestions = append(suggestions, "Consider using 'rm -i' for interactive deletion")
	}
	if strings.Contains(commandLower, "git push") && strings.Contains(commandLower, "--force") {
		suggestions = append(suggestions, "Consider using 'git push --force-with-lease' instead")
	}

	return suggestions
}
