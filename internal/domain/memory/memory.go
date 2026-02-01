// Package memory provides the Memory domain entity.
package memory

import (
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Memory represents a memory entry in the V3 system.
type Memory struct {
	mu        sync.RWMutex
	ID        string
	AgentID   string
	Content   string
	Type      shared.MemoryType
	Timestamp int64
	Embedding []float64
	Metadata  map[string]interface{}
}

// Config holds configuration for creating a memory entry.
type Config struct {
	ID        string
	AgentID   string
	Content   string
	Type      shared.MemoryType
	Timestamp int64
	Embedding []float64
	Metadata  map[string]interface{}
}

// New creates a new Memory from the given configuration.
func New(config Config) *Memory {
	timestamp := config.Timestamp
	if timestamp == 0 {
		timestamp = shared.Now()
	}

	metadata := config.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &Memory{
		ID:        config.ID,
		AgentID:   config.AgentID,
		Content:   config.Content,
		Type:      config.Type,
		Timestamp: timestamp,
		Embedding: config.Embedding,
		Metadata:  metadata,
	}
}

// FromShared creates a Memory from a shared.Memory.
func FromShared(m shared.Memory) *Memory {
	timestamp := m.Timestamp
	if timestamp == 0 {
		timestamp = shared.Now()
	}

	metadata := m.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &Memory{
		ID:        m.ID,
		AgentID:   m.AgentID,
		Content:   m.Content,
		Type:      m.Type,
		Timestamp: timestamp,
		Embedding: m.Embedding,
		Metadata:  metadata,
	}
}

// HasEmbedding checks if the memory has an embedding.
func (m *Memory) HasEmbedding() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.Embedding) > 0
}

// GetEmbeddingDimension returns the embedding dimension.
func (m *Memory) GetEmbeddingDimension() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.Embedding)
}

// UpdateContent updates the memory content.
func (m *Memory) UpdateContent(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Content = content
}

// SetEmbedding sets the memory embedding.
func (m *Memory) SetEmbedding(embedding []float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Embedding = embedding
}

// UpdateMetadata updates the memory metadata.
func (m *Memory) UpdateMetadata(metadata map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for k, v := range metadata {
		m.Metadata[k] = v
	}
}

// Matches checks if the memory matches a query.
func (m *Memory) Matches(query shared.MemoryQuery) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if query.AgentID != "" && m.AgentID != query.AgentID {
		return false
	}
	if query.Type != "" && m.Type != query.Type {
		return false
	}
	if query.TimeRange != nil {
		if m.Timestamp < query.TimeRange.Start || m.Timestamp > query.TimeRange.End {
			return false
		}
	}
	if query.Metadata != nil {
		for k, v := range query.Metadata {
			if m.Metadata[k] != v {
				return false
			}
		}
	}
	return true
}

// GetAge returns the age of the memory in milliseconds.
func (m *Memory) GetAge() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return time.Now().UnixMilli() - m.Timestamp
}

// ToShared converts the Memory to a shared.Memory.
func (m *Memory) ToShared() shared.Memory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metadata := make(map[string]interface{})
	for k, v := range m.Metadata {
		metadata[k] = v
	}

	return shared.Memory{
		ID:        m.ID,
		AgentID:   m.AgentID,
		Content:   m.Content,
		Type:      m.Type,
		Timestamp: m.Timestamp,
		Embedding: m.Embedding,
		Metadata:  metadata,
	}
}

// ============================================================================
// Factory Methods
// ============================================================================

// CreateTaskMemory creates a task memory entry.
func CreateTaskMemory(agentID, content, taskID string) *Memory {
	return New(Config{
		ID:        fmt.Sprintf("task-%s-%d", taskID, shared.Now()),
		AgentID:   agentID,
		Content:   content,
		Type:      shared.MemoryTypeTask,
		Timestamp: shared.Now(),
		Metadata: map[string]interface{}{
			"taskId": taskID,
		},
	})
}

// CreateContextMemory creates a context memory entry.
func CreateContextMemory(agentID, content string) *Memory {
	return New(Config{
		ID:        fmt.Sprintf("context-%d", shared.Now()),
		AgentID:   agentID,
		Content:   content,
		Type:      shared.MemoryTypeContext,
		Timestamp: shared.Now(),
	})
}

// CreateEventMemory creates an event memory entry.
func CreateEventMemory(agentID, eventType, content string) *Memory {
	return New(Config{
		ID:        fmt.Sprintf("event-%d", shared.Now()),
		AgentID:   agentID,
		Content:   content,
		Type:      shared.MemoryTypeEvent,
		Timestamp: shared.Now(),
		Metadata: map[string]interface{}{
			"eventType": eventType,
		},
	})
}

// CreateTaskStartMemory creates a task start memory entry.
func CreateTaskStartMemory(agentID, taskID string) *Memory {
	return New(Config{
		ID:        fmt.Sprintf("task-start-%s", taskID),
		AgentID:   agentID,
		Content:   fmt.Sprintf("Task %s started", taskID),
		Type:      shared.MemoryTypeTaskStart,
		Timestamp: shared.Now(),
		Metadata: map[string]interface{}{
			"taskId":  taskID,
			"agentId": agentID,
		},
	})
}

// CreateTaskCompleteMemory creates a task complete memory entry.
func CreateTaskCompleteMemory(agentID, taskID string, status shared.TaskStatus, duration int64) *Memory {
	return New(Config{
		ID:        fmt.Sprintf("task-complete-%s", taskID),
		AgentID:   agentID,
		Content:   fmt.Sprintf("Task %s %s", taskID, status),
		Type:      shared.MemoryTypeTaskComplete,
		Timestamp: shared.Now(),
		Metadata: map[string]interface{}{
			"taskId":   taskID,
			"agentId":  agentID,
			"status":   string(status),
			"duration": duration,
		},
	})
}

// CreateWorkflowStateMemory creates a workflow state memory entry.
func CreateWorkflowStateMemory(workflowID string, state shared.WorkflowState) *Memory {
	return New(Config{
		ID:        fmt.Sprintf("workflow-state-%s", workflowID),
		AgentID:   "system",
		Content:   fmt.Sprintf("Workflow %s state: %s", workflowID, state.Status),
		Type:      shared.MemoryTypeWorkflow,
		Timestamp: shared.Now(),
		Metadata: map[string]interface{}{
			"workflowId": workflowID,
			"status":     string(state.Status),
		},
	})
}

// ConvertFromShared converts a slice of shared.Memory to []*Memory.
func ConvertFromShared(memories []shared.Memory) []*Memory {
	result := make([]*Memory, len(memories))
	for i, m := range memories {
		result[i] = FromShared(m)
	}
	return result
}

// ConvertToShared converts a slice of *Memory to []shared.Memory.
func ConvertToShared(memories []*Memory) []shared.Memory {
	result := make([]shared.Memory, len(memories))
	for i, m := range memories {
		result[i] = m.ToShared()
	}
	return result
}
