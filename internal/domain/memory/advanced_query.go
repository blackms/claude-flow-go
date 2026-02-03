// Package memory provides the Memory domain entity and related types.
package memory

import (
	"time"
)

// DecayFunction represents temporal decay functions.
type DecayFunction string

const (
	// DecayLinear is linear decay over time.
	DecayLinear DecayFunction = "linear"
	// DecayExponential is exponential decay.
	DecayExponential DecayFunction = "exponential"
	// DecayGaussian is Gaussian decay.
	DecayGaussian DecayFunction = "gaussian"
	// DecayNone indicates no decay.
	DecayNone DecayFunction = "none"
)

// GraphDirection represents graph traversal direction.
type GraphDirection string

const (
	// DirectionOutgoing traverses outgoing edges.
	DirectionOutgoing GraphDirection = "outgoing"
	// DirectionIncoming traverses incoming edges.
	DirectionIncoming GraphDirection = "incoming"
	// DirectionBoth traverses both directions.
	DirectionBoth GraphDirection = "both"
)

// SemanticQuery defines a semantic search query.
type SemanticQuery struct {
	// Embedding is the query embedding vector.
	Embedding []float64 `json:"embedding,omitempty"`

	// Text is the text to embed and search.
	Text string `json:"text,omitempty"`

	// TopK is the number of results to return.
	TopK int `json:"topK,omitempty"`

	// MinScore is the minimum similarity score.
	MinScore float64 `json:"minScore,omitempty"`

	// Filters are metadata filters to apply.
	Filters map[string]interface{} `json:"filters,omitempty"`

	// AgentID filters by agent.
	AgentID string `json:"agentId,omitempty"`

	// MemoryTypes filters by memory types.
	MemoryTypes []string `json:"memoryTypes,omitempty"`

	// BoostFactors boosts specific fields.
	BoostFactors map[string]float64 `json:"boostFactors,omitempty"`

	// IncludeEmbeddings includes embeddings in results.
	IncludeEmbeddings bool `json:"includeEmbeddings,omitempty"`
}

// DefaultSemanticQuery returns a default semantic query.
func DefaultSemanticQuery() SemanticQuery {
	return SemanticQuery{
		TopK:     10,
		MinScore: 0.5,
	}
}

// TemporalQuery defines a temporal search query.
type TemporalQuery struct {
	// StartTime is the start of the time range.
	StartTime *time.Time `json:"startTime,omitempty"`

	// EndTime is the end of the time range.
	EndTime *time.Time `json:"endTime,omitempty"`

	// RecencyBias weights recent results higher.
	RecencyBias float64 `json:"recencyBias,omitempty"`

	// DecayFunction is the temporal decay function.
	DecayFunction DecayFunction `json:"decayFunction,omitempty"`

	// DecayRate is the decay rate (for exponential decay).
	DecayRate float64 `json:"decayRate,omitempty"`

	// ReferenceTime is the reference time for decay calculation.
	ReferenceTime *time.Time `json:"referenceTime,omitempty"`

	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`

	// OrderByTime orders by timestamp.
	OrderByTime bool `json:"orderByTime,omitempty"`

	// Descending orders descending (newest first).
	Descending bool `json:"descending,omitempty"`
}

// DefaultTemporalQuery returns a default temporal query.
func DefaultTemporalQuery() TemporalQuery {
	return TemporalQuery{
		DecayFunction: DecayExponential,
		DecayRate:     0.1,
		Limit:         100,
		OrderByTime:   true,
		Descending:    true,
	}
}

// GraphQuery defines a graph-based search query.
type GraphQuery struct {
	// StartNodeID is the starting node.
	StartNodeID string `json:"startNodeId"`

	// NodeType filters by node type.
	NodeType string `json:"nodeType,omitempty"`

	// RelationshipType filters by relationship type.
	RelationshipType string `json:"relationshipType,omitempty"`

	// Direction is the traversal direction.
	Direction GraphDirection `json:"direction,omitempty"`

	// MaxDepth is the maximum traversal depth.
	MaxDepth int `json:"maxDepth,omitempty"`

	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`

	// IncludePath includes the traversal path.
	IncludePath bool `json:"includePath,omitempty"`

	// NodeFilters are filters for nodes.
	NodeFilters map[string]interface{} `json:"nodeFilters,omitempty"`

	// EdgeFilters are filters for edges.
	EdgeFilters map[string]interface{} `json:"edgeFilters,omitempty"`
}

// DefaultGraphQuery returns a default graph query.
func DefaultGraphQuery() GraphQuery {
	return GraphQuery{
		Direction: DirectionBoth,
		MaxDepth:  3,
		Limit:     100,
	}
}

// MultiModalQuery combines multiple query types.
type MultiModalQuery struct {
	// Semantic is the semantic search component.
	Semantic *SemanticQuery `json:"semantic,omitempty"`

	// Temporal is the temporal search component.
	Temporal *TemporalQuery `json:"temporal,omitempty"`

	// Metadata is metadata filters.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// AgentID filters by agent.
	AgentID string `json:"agentId,omitempty"`

	// MemoryTypes filters by memory types.
	MemoryTypes []string `json:"memoryTypes,omitempty"`

	// Combine is how to combine results.
	Combine CombineStrategy `json:"combine,omitempty"`

	// Weights for each query type.
	Weights QueryWeights `json:"weights,omitempty"`

	// TopK is the final number of results.
	TopK int `json:"topK,omitempty"`
}

// CombineStrategy defines how to combine multi-modal results.
type CombineStrategy string

const (
	// CombineIntersection returns intersection of results.
	CombineIntersection CombineStrategy = "intersection"
	// CombineUnion returns union of results.
	CombineUnion CombineStrategy = "union"
	// CombineWeighted uses weighted scoring.
	CombineWeighted CombineStrategy = "weighted"
	// CombineRerank reranks results.
	CombineRerank CombineStrategy = "rerank"
)

// QueryWeights defines weights for multi-modal queries.
type QueryWeights struct {
	// Semantic is the weight for semantic scores.
	Semantic float64 `json:"semantic,omitempty"`

	// Temporal is the weight for temporal scores.
	Temporal float64 `json:"temporal,omitempty"`

	// Metadata is the weight for metadata matches.
	Metadata float64 `json:"metadata,omitempty"`
}

// DefaultQueryWeights returns default query weights.
func DefaultQueryWeights() QueryWeights {
	return QueryWeights{
		Semantic: 0.6,
		Temporal: 0.3,
		Metadata: 0.1,
	}
}

// SearchResult represents a search result with scores.
type SearchResult struct {
	// Memory is the matched memory.
	Memory *Memory `json:"memory"`

	// Score is the combined score.
	Score float64 `json:"score"`

	// SemanticScore is the semantic similarity score.
	SemanticScore float64 `json:"semanticScore,omitempty"`

	// TemporalScore is the temporal relevance score.
	TemporalScore float64 `json:"temporalScore,omitempty"`

	// MetadataScore is the metadata match score.
	MetadataScore float64 `json:"metadataScore,omitempty"`

	// Highlights are highlighted text portions.
	Highlights []string `json:"highlights,omitempty"`

	// Path is the graph traversal path (for graph queries).
	Path []string `json:"path,omitempty"`

	// Explanation explains the score.
	Explanation string `json:"explanation,omitempty"`
}

// SearchResults contains search results with metadata.
type SearchResults struct {
	// Results is the list of results.
	Results []*SearchResult `json:"results"`

	// TotalCount is the total matching count (before limit).
	TotalCount int `json:"totalCount"`

	// QueryTimeMs is the query execution time.
	QueryTimeMs float64 `json:"queryTimeMs"`

	// FromCache indicates if results are from cache.
	FromCache bool `json:"fromCache,omitempty"`
}

// QueryPlan represents a query execution plan.
type QueryPlan struct {
	// Steps is the list of execution steps.
	Steps []QueryStep `json:"steps"`

	// EstimatedCost is the estimated execution cost.
	EstimatedCost float64 `json:"estimatedCost"`

	// UseIndex indicates which indexes will be used.
	UseIndex []string `json:"useIndex,omitempty"`

	// Filters to push down.
	PushedFilters []string `json:"pushedFilters,omitempty"`
}

// QueryStep represents a step in query execution.
type QueryStep struct {
	// Operation is the step operation.
	Operation string `json:"operation"`

	// Description describes the step.
	Description string `json:"description"`

	// EstimatedRows is the estimated output rows.
	EstimatedRows int `json:"estimatedRows"`

	// Cost is the step cost.
	Cost float64 `json:"cost"`
}

// ContextAwareQuery supports context-aware retrieval.
type ContextAwareQuery struct {
	// CurrentContext is the current conversation context.
	CurrentContext string `json:"currentContext"`

	// ContextEmbedding is the context embedding.
	ContextEmbedding []float64 `json:"contextEmbedding,omitempty"`

	// SessionID is the session for context.
	SessionID string `json:"sessionId,omitempty"`

	// RecentMemoryIDs are recently accessed memories.
	RecentMemoryIDs []string `json:"recentMemoryIds,omitempty"`

	// ContextWindow is the number of recent memories to consider.
	ContextWindow int `json:"contextWindow,omitempty"`

	// TopK is the number of results.
	TopK int `json:"topK,omitempty"`
}

// DefaultContextAwareQuery returns a default context-aware query.
func DefaultContextAwareQuery() ContextAwareQuery {
	return ContextAwareQuery{
		ContextWindow: 10,
		TopK:          5,
	}
}
