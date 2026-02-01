// Package hooks provides the hooks system for self-learning operations.
package hooks

import (
	"sort"
	"strings"
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// RoutingDecision represents a historical routing decision.
type RoutingDecision struct {
	ID             string  `json:"id"`
	Task           string  `json:"task"`
	TaskType       string  `json:"taskType"`
	SelectedAgent  string  `json:"selectedAgent"`
	AgentType      string  `json:"agentType"`
	Confidence     float64 `json:"confidence"`
	Success        bool    `json:"success"`
	Timestamp      int64   `json:"timestamp"`
	ExecutionTimeMs int64  `json:"executionTimeMs,omitempty"`
}

// RoutingEngine provides intelligent routing of tasks to agents.
type RoutingEngine struct {
	mu             sync.RWMutex
	agentScores    map[string]map[string]float64 // agentType -> taskType -> score
	agentStats     map[string]*shared.AgentRoutingStats
	history        []*RoutingDecision
	learningRate   float64
	maxHistory     int
	totalRoutings  int64
	successCount   int64
}

// NewRoutingEngine creates a new RoutingEngine.
func NewRoutingEngine(learningRate float64) *RoutingEngine {
	if learningRate <= 0 {
		learningRate = 0.1
	}
	return &RoutingEngine{
		agentScores:  make(map[string]map[string]float64),
		agentStats:   make(map[string]*shared.AgentRoutingStats),
		history:      make([]*RoutingDecision, 0),
		learningRate: learningRate,
		maxHistory:   10000,
	}
}

// Route routes a task to the optimal agent.
func (re *RoutingEngine) Route(task string, context map[string]interface{}, preferredAgents []string, constraints map[string]interface{}) *shared.RoutingResult {
	re.mu.Lock()
	defer re.mu.Unlock()

	result := &shared.RoutingResult{
		ID:        shared.GenerateID("route"),
		Timestamp: shared.Now(),
	}

	// Extract task type from task description
	taskType := re.extractTaskType(task)

	// Score all agent types
	type agentScore struct {
		agentType  string
		score      float64
		confidence float64
		factors    []shared.RoutingFactor
	}

	var scores []agentScore

	// Get all known agent types
	agentTypes := re.getKnownAgentTypes()

	// Add default agent types if none known
	if len(agentTypes) == 0 {
		agentTypes = []string{"coder", "tester", "reviewer", "coordinator", "designer", "deployer"}
	}

	for _, agentType := range agentTypes {
		score, factors := re.scoreAgentForTask(agentType, taskType, task, preferredAgents)
		scores = append(scores, agentScore{
			agentType:  agentType,
			score:      score,
			confidence: re.calculateConfidence(agentType, taskType),
			factors:    factors,
		})
	}

	// Sort by score
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if len(scores) == 0 {
		result.RecommendedAgent = "coder"
		result.Confidence = 0.5
		result.Explanation = "Default routing due to no historical data"
		return result
	}

	// Set recommended agent
	best := scores[0]
	result.RecommendedAgent = best.agentType
	result.Confidence = best.confidence
	result.Factors = best.factors

	// Generate explanation
	result.Explanation = re.generateExplanation(task, taskType, best.agentType, best.score, best.factors)

	// Set alternatives (next best options)
	for i := 1; i < len(scores) && i <= 3; i++ {
		alt := scores[i]
		result.Alternatives = append(result.Alternatives, shared.RoutingAlternative{
			AgentType:  alt.agentType,
			Confidence: alt.confidence,
			Reason:     re.getAlternativeReason(alt.agentType, taskType),
		})
	}

	// Record this routing
	decision := &RoutingDecision{
		ID:            result.ID,
		Task:          task,
		TaskType:      taskType,
		SelectedAgent: result.RecommendedAgent,
		AgentType:     result.RecommendedAgent,
		Confidence:    result.Confidence,
		Timestamp:     result.Timestamp,
	}
	re.history = append(re.history, decision)
	re.totalRoutings++

	// Trim history if needed
	if len(re.history) > re.maxHistory {
		re.history = re.history[len(re.history)-re.maxHistory:]
	}

	return result
}

// RecordOutcome records the outcome of a routing decision.
func (re *RoutingEngine) RecordOutcome(routingID string, success bool, executionTimeMs int64) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	// Find the decision
	var decision *RoutingDecision
	for _, d := range re.history {
		if d.ID == routingID {
			decision = d
			break
		}
	}

	if decision == nil {
		return nil // Not found, but not an error
	}

	decision.Success = success
	decision.ExecutionTimeMs = executionTimeMs

	// Update success count
	if success {
		re.successCount++
	}

	// Update agent scores using Q-learning update rule
	agentType := decision.AgentType
	taskType := decision.TaskType

	if re.agentScores[agentType] == nil {
		re.agentScores[agentType] = make(map[string]float64)
	}

	currentScore := re.agentScores[agentType][taskType]
	reward := 0.0
	if success {
		reward = 1.0
	} else {
		reward = -0.5
	}

	// Q-learning update: Q(s,a) = Q(s,a) + α * (reward - Q(s,a))
	newScore := currentScore + re.learningRate*(reward-currentScore)
	re.agentScores[agentType][taskType] = newScore

	// Update agent stats
	if re.agentStats[agentType] == nil {
		re.agentStats[agentType] = &shared.AgentRoutingStats{
			AgentType: agentType,
		}
	}
	stats := re.agentStats[agentType]
	stats.TasksRouted++
	if success {
		n := float64(stats.TasksRouted)
		stats.SuccessRate = (stats.SuccessRate*(n-1) + 1.0) / n
	} else {
		n := float64(stats.TasksRouted)
		stats.SuccessRate = (stats.SuccessRate * (n - 1)) / n
	}
	stats.AvgLatencyMs = (stats.AvgLatencyMs*float64(stats.TasksRouted-1) + float64(executionTimeMs)) / float64(stats.TasksRouted)

	return nil
}

// Explain provides a detailed explanation for routing a task.
func (re *RoutingEngine) Explain(task string, context map[string]interface{}, verbose bool) *shared.RoutingExplanation {
	re.mu.RLock()
	defer re.mu.RUnlock()

	taskType := re.extractTaskType(task)

	explanation := &shared.RoutingExplanation{
		Task:     task,
		Analysis: re.analyzeTask(task, taskType),
	}

	// Collect factors for each agent type
	agentTypes := re.getKnownAgentTypes()
	if len(agentTypes) == 0 {
		agentTypes = []string{"coder", "tester", "reviewer"}
	}

	for _, agentType := range agentTypes {
		_, factors := re.scoreAgentForTask(agentType, taskType, task, nil)
		explanation.Factors = append(explanation.Factors, factors...)
	}

	// Add historical data
	if verbose {
		explanation.HistoricalData = &shared.RoutingHistory{
			TotalRoutings: re.totalRoutings,
			SuccessRate:   re.GetSuccessRate(),
			AgentStats:    make(map[string]shared.AgentRoutingStats),
		}
		for agentType, stats := range re.agentStats {
			explanation.HistoricalData.AgentStats[agentType] = *stats
		}
	}

	return explanation
}

// scoreAgentForTask calculates a score for an agent handling a task.
func (re *RoutingEngine) scoreAgentForTask(agentType, taskType, task string, preferredAgents []string) (float64, []shared.RoutingFactor) {
	var factors []shared.RoutingFactor
	score := 0.5 // Base score

	// Factor 1: Historical performance for this task type
	if re.agentScores[agentType] != nil {
		histScore := re.agentScores[agentType][taskType]
		if histScore != 0 {
			factor := shared.RoutingFactor{
				Name:   "historical_performance",
				Weight: 0.4,
				Score:  (histScore + 1) / 2, // Normalize to 0-1
				Reason: "Based on past success with similar tasks",
			}
			score += factor.Weight * factor.Score
			factors = append(factors, factor)
		}
	}

	// Factor 2: Agent specialization match
	specScore := re.getSpecializationScore(agentType, taskType)
	if specScore > 0 {
		factor := shared.RoutingFactor{
			Name:   "specialization",
			Weight: 0.3,
			Score:  specScore,
			Reason: "Agent type matches task requirements",
		}
		score += factor.Weight * factor.Score
		factors = append(factors, factor)
	}

	// Factor 3: Keyword match
	keywordScore := re.getKeywordMatchScore(agentType, task)
	if keywordScore > 0 {
		factor := shared.RoutingFactor{
			Name:   "keyword_match",
			Weight: 0.2,
			Score:  keywordScore,
			Reason: "Task description matches agent capabilities",
		}
		score += factor.Weight * factor.Score
		factors = append(factors, factor)
	}

	// Factor 4: Preferred agent bonus
	for _, preferred := range preferredAgents {
		if strings.EqualFold(preferred, agentType) {
			factor := shared.RoutingFactor{
				Name:   "preferred",
				Weight: 0.1,
				Score:  1.0,
				Reason: "Agent is in preferred list",
			}
			score += factor.Weight * factor.Score
			factors = append(factors, factor)
			break
		}
	}

	return score, factors
}

// getSpecializationScore returns a score based on agent-task type match.
func (re *RoutingEngine) getSpecializationScore(agentType, taskType string) float64 {
	specializations := map[string][]string{
		"coder":       {"code", "implement", "develop", "fix", "feature"},
		"tester":      {"test", "qa", "verify", "validate"},
		"reviewer":    {"review", "audit", "check", "inspect"},
		"coordinator": {"coordinate", "manage", "plan", "organize"},
		"designer":    {"design", "architect", "structure", "plan"},
		"deployer":    {"deploy", "release", "publish", "ship"},
		"researcher":  {"research", "investigate", "analyze", "study"},
		"optimizer":   {"optimize", "improve", "performance", "speed"},
	}

	specs, exists := specializations[agentType]
	if !exists {
		return 0.3 // Default score for unknown types
	}

	for _, spec := range specs {
		if strings.Contains(strings.ToLower(taskType), spec) {
			return 1.0
		}
	}

	return 0.2
}

// getKeywordMatchScore returns a score based on keyword matching.
func (re *RoutingEngine) getKeywordMatchScore(agentType, task string) float64 {
	keywords := map[string][]string{
		"coder":       {"code", "function", "class", "method", "implement", "bug", "fix", "feature", "api", "endpoint"},
		"tester":      {"test", "spec", "coverage", "unit", "integration", "e2e", "mock", "assert"},
		"reviewer":    {"review", "pr", "pull request", "feedback", "comment", "approve"},
		"coordinator": {"coordinate", "schedule", "assign", "delegate", "prioritize"},
		"designer":    {"design", "architecture", "structure", "diagram", "uml", "pattern"},
		"deployer":    {"deploy", "release", "docker", "kubernetes", "ci", "cd", "pipeline"},
	}

	taskLower := strings.ToLower(task)
	agentKeywords, exists := keywords[agentType]
	if !exists {
		return 0.1
	}

	matches := 0
	for _, kw := range agentKeywords {
		if strings.Contains(taskLower, kw) {
			matches++
		}
	}

	if matches == 0 {
		return 0
	}
	return float64(matches) / float64(len(agentKeywords))
}

// extractTaskType extracts a task type from the task description.
func (re *RoutingEngine) extractTaskType(task string) string {
	taskLower := strings.ToLower(task)

	typeKeywords := map[string][]string{
		"code":     {"implement", "code", "develop", "create", "build", "add", "feature"},
		"test":     {"test", "spec", "coverage", "unit test", "integration test"},
		"review":   {"review", "check", "inspect", "audit", "examine"},
		"deploy":   {"deploy", "release", "ship", "publish"},
		"design":   {"design", "architect", "structure", "plan"},
		"fix":      {"fix", "bug", "patch", "repair", "resolve"},
		"refactor": {"refactor", "clean", "improve", "optimize"},
	}

	for taskType, keywords := range typeKeywords {
		for _, kw := range keywords {
			if strings.Contains(taskLower, kw) {
				return taskType
			}
		}
	}

	return "general"
}

// analyzeTask provides analysis of a task.
func (re *RoutingEngine) analyzeTask(task, taskType string) string {
	return "Task type: " + taskType + ". " +
		"Routing based on historical performance, agent specialization, and keyword matching."
}

// calculateConfidence calculates confidence for a routing decision.
func (re *RoutingEngine) calculateConfidence(agentType, taskType string) float64 {
	// Base confidence
	confidence := 0.5

	// Increase confidence based on historical data
	if re.agentStats[agentType] != nil {
		stats := re.agentStats[agentType]
		if stats.TasksRouted > 10 {
			confidence += 0.2
		}
		confidence += stats.SuccessRate * 0.3
	}

	// Cap at 0.95
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

// generateExplanation generates a human-readable explanation.
func (re *RoutingEngine) generateExplanation(task, taskType, agentType string, score float64, factors []shared.RoutingFactor) string {
	explanation := "Routing to " + agentType + " based on: "
	
	factorNames := make([]string, 0, len(factors))
	for _, f := range factors {
		factorNames = append(factorNames, f.Name)
	}
	
	if len(factorNames) > 0 {
		explanation += strings.Join(factorNames, ", ")
	} else {
		explanation += "default routing rules"
	}

	return explanation
}

// getAlternativeReason returns a reason for an alternative routing.
func (re *RoutingEngine) getAlternativeReason(agentType, taskType string) string {
	if re.agentStats[agentType] != nil {
		stats := re.agentStats[agentType]
		if stats.SuccessRate > 0.7 {
			return "High historical success rate"
		}
	}
	return "Alternative specialization"
}

// getKnownAgentTypes returns all agent types we have data for.
func (re *RoutingEngine) getKnownAgentTypes() []string {
	seen := make(map[string]bool)
	
	for agentType := range re.agentScores {
		seen[agentType] = true
	}
	for agentType := range re.agentStats {
		seen[agentType] = true
	}

	result := make([]string, 0, len(seen))
	for agentType := range seen {
		result = append(result, agentType)
	}

	return result
}

// GetRoutingCount returns the total number of routing decisions.
func (re *RoutingEngine) GetRoutingCount() int64 {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.totalRoutings
}

// GetSuccessRate returns the overall routing success rate.
func (re *RoutingEngine) GetSuccessRate() float64 {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if re.totalRoutings == 0 {
		return 0
	}
	return float64(re.successCount) / float64(re.totalRoutings)
}

// GetAgentStats returns routing stats for all agents.
func (re *RoutingEngine) GetAgentStats() map[string]*shared.AgentRoutingStats {
	re.mu.RLock()
	defer re.mu.RUnlock()

	result := make(map[string]*shared.AgentRoutingStats)
	for k, v := range re.agentStats {
		statsCopy := *v
		result[k] = &statsCopy
	}
	return result
}

// GetHistory returns recent routing decisions.
func (re *RoutingEngine) GetHistory(limit int) []*RoutingDecision {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if limit <= 0 || limit > len(re.history) {
		limit = len(re.history)
	}

	start := len(re.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*RoutingDecision, limit)
	copy(result, re.history[start:])
	return result
}
