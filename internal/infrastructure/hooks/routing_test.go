// Package hooks provides the hooks system for self-learning operations.
package hooks

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestNewRoutingEngine(t *testing.T) {
	tests := []struct {
		name         string
		learningRate float64
		expected     float64
	}{
		{"positive learning rate", 0.2, 0.2},
		{"zero learning rate defaults to 0.1", 0, 0.1},
		{"negative learning rate defaults to 0.1", -0.5, 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := NewRoutingEngine(tt.learningRate)
			if re.learningRate != tt.expected {
				t.Errorf("expected learningRate %v, got %v", tt.expected, re.learningRate)
			}
			if re.maxHistory != 10000 {
				t.Errorf("expected maxHistory 10000, got %d", re.maxHistory)
			}
			if re.agentScores == nil {
				t.Error("agentScores should be initialized")
			}
			if re.agentStats == nil {
				t.Error("agentStats should be initialized")
			}
			if re.history == nil {
				t.Error("history should be initialized")
			}
		})
	}
}

func TestRoutingEngine_Route(t *testing.T) {
	re := NewRoutingEngine(0.1)

	tests := []struct {
		name           string
		task           string
		preferredAgent string
		expectedAgent  string
	}{
		{"code task routes to coder", "implement a new API endpoint", "", "coder"},
		{"test task routes to tester", "write unit tests for the auth module", "", "tester"},
		{"review task routes to reviewer", "review the pull request for security issues", "", "reviewer"},
		{"deploy task routes to deployer", "deploy the application to production", "", "deployer"},
		{"design task routes to designer", "design the database schema", "", "designer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var preferredAgents []string
			if tt.preferredAgent != "" {
				preferredAgents = []string{tt.preferredAgent}
			}

			result := re.Route(tt.task, nil, preferredAgents, nil)

			if result.ID == "" {
				t.Error("result should have an ID")
			}
			if result.RecommendedAgent != tt.expectedAgent {
				t.Errorf("expected agent %s, got %s", tt.expectedAgent, result.RecommendedAgent)
			}
			if result.Confidence < 0 || result.Confidence > 1 {
				t.Errorf("confidence should be between 0 and 1, got %v", result.Confidence)
			}
			if result.Explanation == "" {
				t.Error("explanation should not be empty")
			}
		})
	}
}

func TestRoutingEngine_RouteWithPreference(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Route a task with a preferred agent
	result := re.Route("do some work", nil, []string{"tester"}, nil)

	// The preferred agent should get a bonus, potentially affecting the result
	found := false
	for _, factor := range result.Factors {
		if factor.Name == "preferred" {
			found = true
			if factor.Score != 1.0 {
				t.Errorf("preferred factor score should be 1.0, got %v", factor.Score)
			}
		}
	}

	// If tester was chosen, the preferred factor should be present
	if result.RecommendedAgent == "tester" && !found {
		t.Error("preferred factor should be present when preferred agent is selected")
	}
}

func TestRoutingEngine_RecordOutcome(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// First, route a task
	result := re.Route("implement a feature", nil, nil, nil)

	// Record a successful outcome
	err := re.RecordOutcome(result.ID, true, 100)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that success was recorded
	if re.successCount != 1 {
		t.Errorf("expected successCount 1, got %d", re.successCount)
	}

	// Check that agent stats were updated
	stats := re.GetAgentStats()
	agentStats, exists := stats[result.RecommendedAgent]
	if !exists {
		t.Errorf("stats for agent %s should exist", result.RecommendedAgent)
	} else {
		if agentStats.TasksRouted != 1 {
			t.Errorf("expected TasksRouted 1, got %d", agentStats.TasksRouted)
		}
		if agentStats.SuccessRate != 1.0 {
			t.Errorf("expected SuccessRate 1.0, got %v", agentStats.SuccessRate)
		}
	}
}

func TestRoutingEngine_RecordOutcomeFailure(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Route a task
	result := re.Route("implement a feature", nil, nil, nil)

	// Record a failed outcome
	err := re.RecordOutcome(result.ID, false, 200)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Success count should not increase
	if re.successCount != 0 {
		t.Errorf("expected successCount 0, got %d", re.successCount)
	}

	// Check agent stats
	stats := re.GetAgentStats()
	agentStats := stats[result.RecommendedAgent]
	if agentStats.SuccessRate != 0.0 {
		t.Errorf("expected SuccessRate 0.0, got %v", agentStats.SuccessRate)
	}
}

func TestRoutingEngine_QLearningUpdate(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Route multiple tasks and record outcomes
	for i := 0; i < 10; i++ {
		result := re.Route("implement a feature", nil, nil, nil)
		re.RecordOutcome(result.ID, true, 100)
	}

	// Check that the score for coder->code has increased
	if re.agentScores["coder"] == nil {
		t.Error("coder scores should exist")
	}

	// After 10 successful outcomes, the score should be positive
	codeScore := re.agentScores["coder"]["code"]
	if codeScore <= 0 {
		t.Errorf("expected positive score for coder->code, got %v", codeScore)
	}
}

func TestRoutingEngine_Explain(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Route some tasks first to build history
	for i := 0; i < 5; i++ {
		result := re.Route("implement feature", nil, nil, nil)
		re.RecordOutcome(result.ID, true, 100)
	}

	// Get explanation
	explanation := re.Explain("implement a new feature", nil, true)

	if explanation.Task != "implement a new feature" {
		t.Errorf("expected task in explanation, got %s", explanation.Task)
	}

	if explanation.Analysis == "" {
		t.Error("analysis should not be empty")
	}

	// Verbose should include historical data
	if explanation.HistoricalData == nil {
		t.Error("historical data should be present in verbose mode")
	}

	if explanation.HistoricalData.TotalRoutings != 5 {
		t.Errorf("expected 5 total routings, got %d", explanation.HistoricalData.TotalRoutings)
	}
}

func TestRoutingEngine_ExplainNonVerbose(t *testing.T) {
	re := NewRoutingEngine(0.1)

	explanation := re.Explain("implement a feature", nil, false)

	// Non-verbose should not include historical data
	if explanation.HistoricalData != nil {
		t.Error("historical data should not be present in non-verbose mode")
	}
}

func TestRoutingEngine_GetRoutingCount(t *testing.T) {
	re := NewRoutingEngine(0.1)

	if re.GetRoutingCount() != 0 {
		t.Error("initial routing count should be 0")
	}

	// Route some tasks
	for i := 0; i < 3; i++ {
		re.Route("task", nil, nil, nil)
	}

	if re.GetRoutingCount() != 3 {
		t.Errorf("expected 3 routings, got %d", re.GetRoutingCount())
	}
}

func TestRoutingEngine_GetSuccessRate(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Initial success rate should be 0
	if re.GetSuccessRate() != 0 {
		t.Error("initial success rate should be 0")
	}

	// Route 4 tasks, 3 successful
	for i := 0; i < 4; i++ {
		result := re.Route("task", nil, nil, nil)
		re.RecordOutcome(result.ID, i < 3, 100)
	}

	expectedRate := 0.75 // 3/4
	if re.GetSuccessRate() != expectedRate {
		t.Errorf("expected success rate %v, got %v", expectedRate, re.GetSuccessRate())
	}
}

func TestRoutingEngine_GetHistory(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Route some tasks
	for i := 0; i < 5; i++ {
		re.Route("task", nil, nil, nil)
	}

	// Get all history
	history := re.GetHistory(0)
	if len(history) != 5 {
		t.Errorf("expected 5 history entries, got %d", len(history))
	}

	// Get limited history
	history = re.GetHistory(3)
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}
}

func TestRoutingEngine_ExtractTaskType(t *testing.T) {
	re := NewRoutingEngine(0.1)

	tests := []struct {
		task     string
		expected string
	}{
		{"implement a new feature", "code"},
		{"write unit tests", "test"},
		{"review the code", "review"},
		{"deploy to production", "deploy"},
		{"design the architecture", "design"},
		{"fix the bug", "fix"},
		{"refactor the module", "refactor"},
		{"something random", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.task, func(t *testing.T) {
			result := re.extractTaskType(tt.task)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestRoutingEngine_ScoreAgentForTask(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Score a coder for a code task
	score, factors := re.scoreAgentForTask("coder", "code", "implement a new API", nil)

	if score < 0.5 {
		t.Error("base score should be at least 0.5")
	}

	// Should have specialization factor
	found := false
	for _, f := range factors {
		if f.Name == "specialization" {
			found = true
		}
	}
	if !found {
		t.Error("should have specialization factor for matching agent-task type")
	}
}

func TestRoutingEngine_HistoryTrimming(t *testing.T) {
	// Create engine with small max history for testing
	re := NewRoutingEngine(0.1)
	re.maxHistory = 5

	// Route more tasks than max history
	for i := 0; i < 10; i++ {
		re.Route("task", nil, nil, nil)
	}

	// History should be trimmed to max
	history := re.GetHistory(0)
	if len(history) != 5 {
		t.Errorf("expected history to be trimmed to 5, got %d", len(history))
	}
}

func TestRoutingEngine_RecordOutcomeNotFound(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Recording outcome for non-existent routing should not error
	err := re.RecordOutcome("non-existent-id", true, 100)
	if err != nil {
		t.Errorf("unexpected error for non-existent routing: %v", err)
	}
}

func TestRoutingEngine_Alternatives(t *testing.T) {
	re := NewRoutingEngine(0.1)

	result := re.Route("implement a feature", nil, nil, nil)

	// Should have alternatives
	if len(result.Alternatives) == 0 {
		t.Error("should have at least one alternative")
	}

	// Alternatives should have required fields
	for _, alt := range result.Alternatives {
		if alt.AgentType == "" {
			t.Error("alternative should have agent type")
		}
		if alt.Confidence < 0 || alt.Confidence > 1 {
			t.Errorf("alternative confidence should be between 0 and 1, got %v", alt.Confidence)
		}
	}
}

func TestRoutingEngine_ConfidenceIncreases(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Initial confidence should be around 0.5 (base)
	result1 := re.Route("implement feature", nil, nil, nil)
	initialConfidence := result1.Confidence

	// Record many successful outcomes for the same agent type
	for i := 0; i < 20; i++ {
		result := re.Route("implement feature", nil, nil, nil)
		re.RecordOutcome(result.ID, true, 100)
	}

	// Route again and check confidence increased
	result2 := re.Route("implement feature", nil, nil, nil)

	if result2.Confidence <= initialConfidence {
		t.Errorf("expected confidence to increase from %v, got %v", initialConfidence, result2.Confidence)
	}

	// Confidence should be capped at 0.95
	if result2.Confidence > 0.95 {
		t.Errorf("confidence should be capped at 0.95, got %v", result2.Confidence)
	}
}

func TestRoutingEngine_ConcurrentAccess(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Run concurrent routing and outcome recording
	done := make(chan bool, 100)

	for i := 0; i < 50; i++ {
		go func() {
			result := re.Route("task", nil, nil, nil)
			re.RecordOutcome(result.ID, true, 100)
			done <- true
		}()
	}

	for i := 0; i < 50; i++ {
		go func() {
			re.Explain("task", nil, true)
			re.GetRoutingCount()
			re.GetSuccessRate()
			re.GetAgentStats()
			re.GetHistory(10)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Should have processed all routing requests
	count := re.GetRoutingCount()
	if count != 50 {
		t.Errorf("expected 50 routings, got %d", count)
	}
}

func TestRoutingEngine_FactorsWeight(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Score an agent with all factors
	_, factors := re.scoreAgentForTask("coder", "code", "implement a new function", []string{"coder"})

	// Check that we have multiple factors
	if len(factors) < 2 {
		t.Errorf("expected at least 2 factors, got %d", len(factors))
	}

	// Check total weight doesn't exceed 1.0
	totalWeight := 0.0
	for _, f := range factors {
		totalWeight += f.Weight
	}
	if totalWeight > 1.0 {
		t.Errorf("total weight should not exceed 1.0, got %v", totalWeight)
	}
}

func TestRoutingEngine_GetAgentStats(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Route and record some outcomes
	result := re.Route("implement feature", nil, nil, nil)
	re.RecordOutcome(result.ID, true, 150)

	stats := re.GetAgentStats()

	// Should have stats for the routed agent
	agentStats, exists := stats[result.RecommendedAgent]
	if !exists {
		t.Errorf("expected stats for agent %s", result.RecommendedAgent)
	}

	// Stats should be a copy (modifying shouldn't affect original)
	agentStats.TasksRouted = 999
	originalStats := re.GetAgentStats()
	if originalStats[result.RecommendedAgent].TasksRouted == 999 {
		t.Error("GetAgentStats should return a copy")
	}
}

func TestRoutingEngine_IntegrationWithTypes(t *testing.T) {
	re := NewRoutingEngine(0.1)

	// Route a task
	result := re.Route("implement feature", nil, nil, nil)

	// Result should be a proper shared.RoutingResult
	var _ *shared.RoutingResult = result

	// Explanation should be a proper shared.RoutingExplanation
	explanation := re.Explain("task", nil, true)
	var _ *shared.RoutingExplanation = explanation

	// Agent stats should be proper shared.AgentRoutingStats
	stats := re.GetAgentStats()
	for _, s := range stats {
		var _ *shared.AgentRoutingStats = s
	}
}
