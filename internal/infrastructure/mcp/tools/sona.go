// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"

	appNeural "github.com/anthropics/claude-flow-go/internal/application/neural"
	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// SONATools provides MCP tools for SONA operations.
type SONATools struct {
	manager *appNeural.SONAManager
}

// NewSONATools creates a new SONATools instance.
func NewSONATools(manager *appNeural.SONAManager) *SONATools {
	return &SONATools{
		manager: manager,
	}
}

// NewSONAToolsDefault creates a new SONATools with default manager.
func NewSONAToolsDefault() *SONATools {
	return &SONATools{
		manager: appNeural.NewSONAManager(),
	}
}

// GetTools returns available SONA tools.
func (t *SONATools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "sona_mode",
			Description: "Get or set the SONA learning mode (real-time, balanced, research, edge, batch)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"get", "set"},
						"description": "Action to perform: 'get' returns current mode, 'set' changes mode",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"real-time", "balanced", "research", "edge", "batch"},
						"description": "Mode to set (required for 'set' action)",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Reason for mode change (optional)",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "sona_adapt",
			Description: "Apply SONA adaptation to an embedding vector",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"embedding": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "number"},
						"description": "Embedding vector to adapt",
					},
				},
				"required": []string{"embedding"},
			},
		},
		{
			Name:        "sona_metrics",
			Description: "Get SONA learning metrics and statistics",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"detailed": map[string]interface{}{
						"type":        "boolean",
						"description": "Include detailed mode history (default: false)",
					},
				},
			},
		},
		{
			Name:        "sona_optimize",
			Description: "Optimize SONA patterns and trigger learning",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"learn", "optimize", "suggest"},
						"description": "Action: 'learn' triggers learning, 'optimize' optimizes patterns, 'suggest' suggests optimal mode",
					},
					"constraints": map[string]interface{}{
						"type":        "object",
						"description": "Constraints for mode suggestion",
						"properties": map[string]interface{}{
							"maxLatencyMs": map[string]interface{}{
								"type":        "number",
								"description": "Maximum allowed latency in milliseconds",
							},
							"maxMemoryMb": map[string]interface{}{
								"type":        "number",
								"description": "Maximum allowed memory in megabytes",
							},
							"minQuality": map[string]interface{}{
								"type":        "number",
								"description": "Minimum required quality (0-1)",
							},
							"batchProcessing": map[string]interface{}{
								"type":        "boolean",
								"description": "Enable batch processing mode",
							},
						},
					},
				},
				"required": []string{"action"},
			},
		},
	}
}

// Execute executes a SONA tool.
func (t *SONATools) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	switch toolName {
	case "sona_mode":
		return t.handleMode(ctx, params)
	case "sona_adapt":
		return t.handleAdapt(ctx, params)
	case "sona_metrics":
		return t.handleMetrics(ctx, params)
	case "sona_optimize":
		return t.handleOptimize(ctx, params)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (t *SONATools) handleMode(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	action, _ := params["action"].(string)

	switch action {
	case "get":
		mode := t.manager.GetMode()
		config := t.manager.GetConfig()

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"mode":   string(mode),
				"config": configToMap(config),
			},
		}, nil

	case "set":
		modeStr, ok := params["mode"].(string)
		if !ok || modeStr == "" {
			return &shared.MCPToolResult{
				Success: false,
				Error:   "validation: mode is required for 'set' action",
			}, nil
		}

		mode := domainNeural.SONAMode(modeStr)

		// Validate mode
		validModes := domainNeural.AllSONAModes()
		isValid := false
		for _, m := range validModes {
			if m == mode {
				isValid = true
				break
			}
		}
		if !isValid {
			return &shared.MCPToolResult{
				Success: false,
				Error:   fmt.Sprintf("validation: invalid mode '%s'. Valid modes: real-time, balanced, research, edge, batch", modeStr),
			}, nil
		}

		reason, _ := params["reason"].(string)
		if reason == "" {
			reason = "manual mode switch via MCP tool"
		}

		if err := t.manager.SetModeWithReason(ctx, mode, reason); err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		config := t.manager.GetConfig()

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"mode":           string(mode),
				"config":         configToMap(config),
				"previousMode":   "", // Would need to track this
				"message":        fmt.Sprintf("Successfully switched to %s mode", mode),
			},
		}, nil

	default:
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: action must be 'get' or 'set'",
		}, nil
	}
}

func (t *SONATools) handleAdapt(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	embeddingRaw, ok := params["embedding"].([]interface{})
	if !ok || len(embeddingRaw) == 0 {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: embedding is required",
		}, nil
	}

	embedding := make([]float64, len(embeddingRaw))
	for i, v := range embeddingRaw {
		if f, ok := v.(float64); ok {
			embedding[i] = f
		}
	}

	result, err := t.manager.Adapt(ctx, embedding)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"embedding": result.Embedding,
			"latencyMs": result.LatencyMs,
			"mode":      string(result.Mode),
			"cacheHit":  result.CacheHit,
		},
	}, nil
}

func (t *SONATools) handleMetrics(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	detailed, _ := params["detailed"].(bool)

	stats := t.manager.GetStats()

	data := map[string]interface{}{
		"mode": string(stats.Mode),
		"config": map[string]interface{}{
			"loraRank":           stats.Config.LoRARank,
			"loraAlpha":          stats.Config.LoRAAlpha,
			"learningRate":       stats.Config.LearningRate,
			"maxLatencyMs":       stats.Config.MaxLatencyMs,
			"memoryBudgetMb":     stats.Config.MemoryBudgetMB,
			"qualityThreshold":   stats.Config.QualityThreshold,
			"trajectoryCapacity": stats.Config.TrajectoryCapacity,
		},
		"trajectories": map[string]interface{}{
			"total":       stats.Trajectories.Total,
			"active":      stats.Trajectories.Active,
			"completed":   stats.Trajectories.Completed,
			"utilization": stats.Trajectories.Utilization,
		},
		"performance": map[string]interface{}{
			"avgQualityScore":  stats.Performance.AvgQualityScore,
			"opsPerSecond":     stats.Performance.OpsPerSecond,
			"learningCycles":   stats.Performance.LearningCycles,
			"avgLatencyMs":     stats.Performance.AvgLatencyMs,
			"totalAdaptations": stats.Performance.TotalAdaptations,
		},
		"patterns": map[string]interface{}{
			"totalPatterns":  stats.Patterns.TotalPatterns,
			"avgMatchTimeMs": stats.Patterns.AvgMatchTimeMs,
			"cacheHitRate":   stats.Patterns.CacheHitRate,
			"evolutionCount": stats.Patterns.EvolutionCount,
		},
		"memory": map[string]interface{}{
			"usedMb":          stats.Memory.UsedMB,
			"budgetMb":        stats.Memory.BudgetMB,
			"trajectoryBytes": stats.Memory.TrajectoryBytes,
			"patternBytes":    stats.Memory.PatternBytes,
		},
		"uptime": stats.StartTime.String(),
	}

	if stats.LastModeChange != nil {
		data["lastModeChange"] = stats.LastModeChange.String()
	}
	if stats.LastLearning != nil {
		data["lastLearning"] = stats.LastLearning.String()
	}

	if detailed && len(stats.ModeHistory) > 0 {
		history := make([]map[string]interface{}, len(stats.ModeHistory))
		for i, event := range stats.ModeHistory {
			history[i] = map[string]interface{}{
				"fromMode":  string(event.FromMode),
				"toMode":    string(event.ToMode),
				"timestamp": event.Timestamp.String(),
				"reason":    event.Reason,
			}
		}
		data["modeHistory"] = history
	}

	return &shared.MCPToolResult{
		Success: true,
		Data:    data,
	}, nil
}

func (t *SONATools) handleOptimize(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	action, _ := params["action"].(string)

	switch action {
	case "learn":
		result, err := t.manager.TriggerLearning(ctx)
		if err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"triggered":             result.Triggered,
				"trajectoriesProcessed": result.TrajectoriesProcessed,
				"patternsLearned":       result.PatternsLearned,
				"avgLoss":               result.AvgLoss,
				"durationMs":            result.DurationMs,
				"timestamp":             result.Timestamp.String(),
			},
		}, nil

	case "optimize":
		result, err := t.manager.OptimizePatterns(ctx)
		if err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"patternsOptimized": result.PatternsOptimized,
				"patternsPruned":    result.PatternsPruned,
				"patternsMerged":    result.PatternsMerged,
				"memoryFreedMb":     result.MemoryFreedMB,
				"durationMs":        result.DurationMs,
				"timestamp":         result.Timestamp.String(),
			},
		}, nil

	case "suggest":
		constraints := appNeural.ModeConstraints{}

		if constraintsMap, ok := params["constraints"].(map[string]interface{}); ok {
			if maxLatency, ok := constraintsMap["maxLatencyMs"].(float64); ok {
				constraints.MaxLatencyMs = maxLatency
			}
			if maxMemory, ok := constraintsMap["maxMemoryMb"].(float64); ok {
				constraints.MaxMemoryMB = int(maxMemory)
			}
			if minQuality, ok := constraintsMap["minQuality"].(float64); ok {
				constraints.MinQuality = minQuality
			}
			if batchProcessing, ok := constraintsMap["batchProcessing"].(bool); ok {
				constraints.BatchProcessing = batchProcessing
			}
		}

		suggestedMode := t.manager.SuggestMode(ctx, constraints)
		config := domainNeural.DefaultModeConfig(suggestedMode)

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"suggestedMode": string(suggestedMode),
				"config":        configToMap(config),
				"constraints":   constraints,
				"currentMode":   string(t.manager.GetMode()),
			},
		}, nil

	default:
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: action must be 'learn', 'optimize', or 'suggest'",
		}, nil
	}
}

// GetSONAManager returns the underlying SONA manager.
func (t *SONATools) GetSONAManager() *appNeural.SONAManager {
	return t.manager
}

// Helper function to convert ModeConfig to map
func configToMap(config domainNeural.ModeConfig) map[string]interface{} {
	return map[string]interface{}{
		"mode":               string(config.Mode),
		"loraRank":           config.LoRARank,
		"loraAlpha":          config.LoRAAlpha,
		"learningRate":       config.LearningRate,
		"maxLatencyMs":       config.MaxLatencyMs,
		"memoryBudgetMb":     config.MemoryBudgetMB,
		"qualityThreshold":   config.QualityThreshold,
		"trajectoryCapacity": config.TrajectoryCapacity,
		"patternClusters":    config.PatternClusters,
		"ewcLambda":          config.EWCLambda,
		"optimizations": map[string]interface{}{
			"enableSimd":              config.Optimizations.EnableSIMD,
			"useMicroLora":            config.Optimizations.UseMicroLoRA,
			"gradientCheckpointing":   config.Optimizations.GradientCheckpointing,
			"useHalfPrecision":        config.Optimizations.UseHalfPrecision,
			"patternCaching":          config.Optimizations.PatternCaching,
			"asyncUpdates":            config.Optimizations.AsyncUpdates,
			"useQuantization":         config.Optimizations.UseQuantization,
			"compressEmbeddings":      config.Optimizations.CompressEmbeddings,
			"gradientAccumulation":    config.Optimizations.GradientAccumulation,
			"gradientAccumulationSteps": config.Optimizations.GradientAccumulationSteps,
		},
	}
}
