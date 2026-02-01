// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// MemoryTools provides MCP tools for memory operations.
type MemoryTools struct {
	backend shared.MemoryBackend
}

// NewMemoryTools creates a new MemoryTools instance.
func NewMemoryTools(backend shared.MemoryBackend) *MemoryTools {
	return &MemoryTools{
		backend: backend,
	}
}

// GetTools returns available memory tools.
func (t *MemoryTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "memory_store",
			Description: "Store a memory entry",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Unique memory identifier",
					},
					"agentId": map[string]interface{}{
						"type":        "string",
						"description": "Agent ID associated with the memory",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Memory content",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Memory type",
					},
				},
				"required": []string{"id", "agentId", "content", "type"},
			},
		},
		{
			Name:        "memory_retrieve",
			Description: "Retrieve a memory entry by ID",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Memory ID to retrieve",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "memory_query",
			Description: "Query memory entries",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agentId": map[string]interface{}{
						"type":        "string",
						"description": "Filter by agent ID",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by memory type",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results",
					},
				},
			},
		},
		{
			Name:        "memory_search",
			Description: "Search memories by vector similarity",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"embedding": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "number"},
						"description": "Query embedding vector",
					},
					"k": map[string]interface{}{
						"type":        "number",
						"description": "Number of results to return",
					},
				},
				"required": []string{"embedding"},
			},
		},
		{
			Name:        "memory_delete",
			Description: "Delete a memory entry",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Memory ID to delete",
					},
				},
				"required": []string{"id"},
			},
		},
	}
}

// Execute executes a memory tool.
func (t *MemoryTools) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	switch toolName {
	case "memory_store":
		return t.storeMemory(params)
	case "memory_retrieve":
		return t.retrieveMemory(params)
	case "memory_query":
		return t.queryMemory(params)
	case "memory_search":
		return t.searchMemory(params)
	case "memory_delete":
		return t.deleteMemory(params)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (t *MemoryTools) storeMemory(params map[string]interface{}) (*shared.MCPToolResult, error) {
	id, _ := params["id"].(string)
	agentID, _ := params["agentId"].(string)
	content, _ := params["content"].(string)
	memType, _ := params["type"].(string)

	if id == "" || agentID == "" || content == "" || memType == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: id, agentId, content, and type are required",
		}, nil
	}

	memory := shared.Memory{
		ID:        id,
		AgentID:   agentID,
		Content:   content,
		Type:      shared.MemoryType(memType),
		Timestamp: shared.Now(),
	}

	if metadata, ok := params["metadata"].(map[string]interface{}); ok {
		memory.Metadata = metadata
	}

	result, err := t.backend.Store(memory)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success:  true,
		Memories: []shared.Memory{result},
	}, nil
}

func (t *MemoryTools) retrieveMemory(params map[string]interface{}) (*shared.MCPToolResult, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: id is required",
		}, nil
	}

	memory, err := t.backend.Retrieve(id)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if memory == nil {
		return &shared.MCPToolResult{
			Success:  true,
			Memories: []shared.Memory{},
		}, nil
	}

	return &shared.MCPToolResult{
		Success:  true,
		Memories: []shared.Memory{*memory},
	}, nil
}

func (t *MemoryTools) queryMemory(params map[string]interface{}) (*shared.MCPToolResult, error) {
	query := shared.MemoryQuery{}

	if agentID, ok := params["agentId"].(string); ok {
		query.AgentID = agentID
	}
	if memType, ok := params["type"].(string); ok {
		query.Type = shared.MemoryType(memType)
	}
	if limit, ok := params["limit"].(float64); ok {
		query.Limit = int(limit)
	}
	if offset, ok := params["offset"].(float64); ok {
		query.Offset = int(offset)
	}

	memories, err := t.backend.Query(query)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success:  true,
		Memories: memories,
	}, nil
}

func (t *MemoryTools) searchMemory(params map[string]interface{}) (*shared.MCPToolResult, error) {
	embeddingRaw, ok := params["embedding"].([]interface{})
	if !ok {
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

	k := 10
	if kVal, ok := params["k"].(float64); ok {
		k = int(kVal)
	}

	results, err := t.backend.VectorSearch(embedding, k)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
		Results: results,
	}, nil
}

func (t *MemoryTools) deleteMemory(params map[string]interface{}) (*shared.MCPToolResult, error) {
	id, _ := params["id"].(string)
	if id == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: id is required",
		}, nil
	}

	if err := t.backend.Delete(id); err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
	}, nil
}
