// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"
	"time"

	appNeural "github.com/anthropics/claude-flow-go/internal/application/neural"
	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ReasoningTools provides MCP tools for reasoning pattern operations.
type ReasoningTools struct {
	bank *appNeural.ReasoningBank
}

// NewReasoningTools creates a new ReasoningTools instance.
func NewReasoningTools(bank *appNeural.ReasoningBank) *ReasoningTools {
	return &ReasoningTools{
		bank: bank,
	}
}

// GetTools returns available reasoning tools.
func (t *ReasoningTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "reasoning_store",
			Description: "Store a reasoning pattern",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Pattern name",
					},
					"domain": map[string]interface{}{
						"type":        "string",
						"description": "Pattern domain (code, creative, logical, math, planning, general)",
					},
					"strategy": map[string]interface{}{
						"type":        "string",
						"description": "Strategy pattern description",
					},
					"keyLearnings": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Key learnings from this pattern",
					},
					"embedding": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "number"},
						"description": "Optional embedding vector for similarity search",
					},
				},
				"required": []string{"name", "strategy"},
			},
		},
		{
			Name:        "reasoning_retrieve",
			Description: "Retrieve reasoning patterns by similarity search",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"embedding": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "number"},
						"description": "Query embedding vector",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Query content for text-based search (if no embedding)",
					},
					"k": map[string]interface{}{
						"type":        "number",
						"description": "Number of patterns to retrieve (default: 5)",
					},
					"minRelevance": map[string]interface{}{
						"type":        "number",
						"description": "Minimum relevance threshold (default: 0.6)",
					},
				},
			},
		},
		{
			Name:        "reasoning_learn",
			Description: "Learn from a trajectory outcome",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"patternId": map[string]interface{}{
						"type":        "string",
						"description": "Pattern ID to update (optional)",
					},
					"success": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the outcome was successful",
					},
					"qualityScore": map[string]interface{}{
						"type":        "number",
						"description": "Quality score (0-1)",
					},
					"feedback": map[string]interface{}{
						"type":        "string",
						"description": "Optional feedback text",
					},
				},
				"required": []string{"success", "qualityScore"},
			},
		},
		{
			Name:        "reasoning_optimize",
			Description: "Optimize patterns through consolidation (deduplicate, merge, prune)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "Force consolidation even if recently run",
					},
				},
			},
		},
	}
}

// Execute executes a reasoning tool.
func (t *ReasoningTools) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	switch toolName {
	case "reasoning_store":
		return t.storePattern(params)
	case "reasoning_retrieve":
		return t.retrievePatterns(params)
	case "reasoning_learn":
		return t.learnFromOutcome(params)
	case "reasoning_optimize":
		return t.optimizePatterns(params)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (t *ReasoningTools) storePattern(params map[string]interface{}) (*shared.MCPToolResult, error) {
	name, _ := params["name"].(string)
	strategy, _ := params["strategy"].(string)

	if name == "" || strategy == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: name and strategy are required",
		}, nil
	}

	pattern := domainNeural.ReasoningPattern{
		PatternID:   fmt.Sprintf("pat_%d", time.Now().UnixNano()),
		Name:        name,
		Domain:      domainNeural.ReasoningDomainGeneral,
		Strategy:    strategy,
		SuccessRate: 0,
		UsageCount:  0,
		Version:     1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		LastUsed:    time.Now(),
	}

	// Set domain if provided
	if domain, ok := params["domain"].(string); ok && domain != "" {
		pattern.Domain = domainNeural.ReasoningDomain(domain)
	}

	// Set key learnings if provided
	if learnings, ok := params["keyLearnings"].([]interface{}); ok {
		pattern.KeyLearnings = make([]string, 0, len(learnings))
		for _, l := range learnings {
			if s, ok := l.(string); ok {
				pattern.KeyLearnings = append(pattern.KeyLearnings, s)
			}
		}
	}

	// Set embedding if provided
	if embeddingRaw, ok := params["embedding"].([]interface{}); ok {
		pattern.Embedding = make([]float32, len(embeddingRaw))
		for i, v := range embeddingRaw {
			if f, ok := v.(float64); ok {
				pattern.Embedding[i] = float32(f)
			}
		}
	}

	// Add creation evolution
	pattern.EvolutionHistory = []domainNeural.PatternEvolution{
		{
			EvolutionID:  fmt.Sprintf("evo_%d", time.Now().UnixNano()),
			PatternID:    pattern.PatternID,
			Type:         domainNeural.EvolutionCreate,
			NewVersion:   1,
			QualityAfter: 0,
			Reason:       "Created via MCP tool",
			Timestamp:    time.Now(),
		},
	}

	if err := t.bank.SavePattern(pattern); err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"patternId": pattern.PatternID,
			"version":   pattern.Version,
			"name":      pattern.Name,
			"domain":    string(pattern.Domain),
		},
	}, nil
}

func (t *ReasoningTools) retrievePatterns(params map[string]interface{}) (*shared.MCPToolResult, error) {
	k := 5
	if kVal, ok := params["k"].(float64); ok {
		k = int(kVal)
	}

	var results []domainNeural.RetrievalResult

	// Try embedding-based search first
	if embeddingRaw, ok := params["embedding"].([]interface{}); ok && len(embeddingRaw) > 0 {
		embedding := make([]float32, len(embeddingRaw))
		for i, v := range embeddingRaw {
			if f, ok := v.(float64); ok {
				embedding[i] = float32(f)
			}
		}
		results = t.bank.Retrieve(embedding, k)
	} else if content, ok := params["content"].(string); ok && content != "" {
		// Fall back to content-based search
		results = t.bank.RetrieveByContent(content, k)
	} else {
		// No query provided, list recent patterns
		patterns, err := t.bank.ListPatterns(nil, k)
		if err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		results = make([]domainNeural.RetrievalResult, len(patterns))
		for i, p := range patterns {
			results[i] = domainNeural.RetrievalResult{
				Pattern:        p,
				RelevanceScore: 1.0,
				DiversityScore: 1.0,
				CombinedScore:  1.0,
			}
		}
	}

	// Convert to response format
	patternsData := make([]map[string]interface{}, len(results))
	for i, r := range results {
		patternsData[i] = map[string]interface{}{
			"patternId":      r.Pattern.PatternID,
			"name":           r.Pattern.Name,
			"domain":         string(r.Pattern.Domain),
			"strategy":       r.Pattern.Strategy,
			"successRate":    r.Pattern.SuccessRate,
			"usageCount":     r.Pattern.UsageCount,
			"keyLearnings":   r.Pattern.KeyLearnings,
			"relevanceScore": r.RelevanceScore,
			"diversityScore": r.DiversityScore,
			"combinedScore":  r.CombinedScore,
		}
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"patterns": patternsData,
			"count":    len(results),
		},
	}, nil
}

func (t *ReasoningTools) learnFromOutcome(params map[string]interface{}) (*shared.MCPToolResult, error) {
	success, _ := params["success"].(bool)
	qualityScore, ok := params["qualityScore"].(float64)
	if !ok {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: qualityScore is required",
		}, nil
	}

	feedback, _ := params["feedback"].(string)

	outcome := domainNeural.LearningOutcome{
		Success:      success,
		QualityScore: qualityScore,
		Feedback:     feedback,
	}

	// If patternId provided, evolve that pattern
	if patternID, ok := params["patternId"].(string); ok && patternID != "" {
		evolution, err := t.bank.EvolvePattern(patternID, outcome)
		if err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"patternId":       patternID,
				"evolutionId":     evolution.EvolutionID,
				"newVersion":      evolution.NewVersion,
				"qualityBefore":   evolution.QualityBefore,
				"qualityAfter":    evolution.QualityAfter,
				"successRateBefore": evolution.SuccessRateBefore,
				"successRateAfter":  evolution.SuccessRateAfter,
			},
		}, nil
	}

	// No pattern specified, return stats
	stats := t.bank.GetStats()

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"message":        "Learning recorded. No pattern specified to update.",
			"outcome":        outcome,
			"totalPatterns":  stats.PatternCount,
			"avgSuccessRate": stats.AvgSuccessRate,
		},
	}, nil
}

func (t *ReasoningTools) optimizePatterns(params map[string]interface{}) (*shared.MCPToolResult, error) {
	// Run consolidation
	result := t.bank.Consolidate()

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"deduplicated":        result.Deduplicated,
			"merged":              result.Merged,
			"pruned":              result.Pruned,
			"contradictionsFound": result.ContradictionsFound,
			"durationMs":          result.DurationMs,
			"timestamp":           result.Timestamp.Format(time.RFC3339),
		},
	}, nil
}

// RegisterReasoningTools registers reasoning tools with the MCP server.
func RegisterReasoningTools(config domainNeural.ReasoningConfig) (*ReasoningTools, error) {
	bank, err := appNeural.NewReasoningBank(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create reasoning bank: %w", err)
	}

	return NewReasoningTools(bank), nil
}

// GetReasoningBank returns the underlying ReasoningBank.
func (t *ReasoningTools) GetReasoningBank() *appNeural.ReasoningBank {
	return t.bank
}

// Export exports all patterns to CFP format.
func (t *ReasoningTools) Export(metadata infraNeural.ExportMetadata) (interface{}, error) {
	return t.bank.Export(metadata)
}

// GetStats returns ReasoningBank statistics.
func (t *ReasoningTools) GetStats() domainNeural.ReasoningBankStats {
	return t.bank.GetStats()
}
