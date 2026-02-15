// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/hooks"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestNewHooksTools(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	if ht == nil {
		t.Error("HooksTools should not be nil")
	}
	if ht.manager != manager {
		t.Error("manager should be set correctly")
	}
}

func TestHooksTools_GetTools(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	tools := ht.GetTools()

	// Should have 9 tools
	if len(tools) != 9 {
		t.Errorf("expected 9 tools, got %d", len(tools))
	}

	// Verify all expected tool names
	expectedTools := map[string]bool{
		"hooks/pre-edit":     false,
		"hooks/post-edit":    false,
		"hooks/pre-command":  false,
		"hooks/post-command": false,
		"hooks/route":        false,
		"hooks/explain":      false,
		"hooks/pretrain":     false,
		"hooks/metrics":      false,
		"hooks/list":         false,
	}

	for _, tool := range tools {
		if _, exists := expectedTools[tool.Name]; exists {
			expectedTools[tool.Name] = true
		} else {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestHooksTools_Execute_UnknownMethod(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()
	result, err := ht.Execute(ctx, "unknown/method", nil)

	if err == nil {
		t.Error("expected error for unknown method")
	}
	if result != nil {
		t.Error("result should be nil for unknown method")
	}
}

func TestHooksTools_PreEdit(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()
	params := map[string]interface{}{
		"filePath":           "/path/to/file.go",
		"operation":          "modify",
		"includeContext":     true,
		"includeSuggestions": true,
	}

	mcpResult, err := ht.Execute(ctx, "hooks/pre-edit", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	preEditResult, ok := mcpResult.Data.(*shared.PreEditResult)
	if !ok {
		t.Fatalf("expected *shared.PreEditResult, got %T", mcpResult.Data)
	}

	if preEditResult.FilePath != "/path/to/file.go" {
		t.Errorf("expected filePath '/path/to/file.go', got '%s'", preEditResult.FilePath)
	}

	// Context should be present since includeContext=true
	if preEditResult.Context == nil {
		t.Error("expected context to be present")
	}
}

func TestHooksTools_PreEdit_MissingParams(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	// Missing filePath
	result, err := ht.Execute(ctx, "hooks/pre-edit", map[string]interface{}{
		"operation": "modify",
	})
	if err == nil {
		t.Error("expected error for missing filePath")
	}
	if result != nil && result.Success {
		t.Error("result should indicate failure")
	}

	// Missing operation
	result, err = ht.Execute(ctx, "hooks/pre-edit", map[string]interface{}{
		"filePath": "/path/to/file.go",
	})
	if err == nil {
		t.Error("expected error for missing operation")
	}
	if result != nil && result.Success {
		t.Error("result should indicate failure")
	}
}

func TestHooksTools_PostEdit(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()
	params := map[string]interface{}{
		"filePath":  "/path/to/file.go",
		"operation": "modify",
		"success":   true,
		"outcome":   "File modified successfully",
		"metadata": map[string]interface{}{
			"linesChanged": 10,
		},
	}

	mcpResult, err := ht.Execute(ctx, "hooks/post-edit", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	postEditResult, ok := mcpResult.Data.(*shared.PostEditResult)
	if !ok {
		t.Fatalf("expected *shared.PostEditResult, got %T", mcpResult.Data)
	}

	if !postEditResult.Recorded {
		t.Error("expected Recorded to be true")
	}

	if postEditResult.PatternID == "" {
		t.Error("expected PatternID to be set")
	}

	// Verify pattern was stored
	patternStore := manager.GetPatternStore()
	if patternStore.Count() != 1 {
		t.Errorf("expected 1 pattern stored, got %d", patternStore.Count())
	}
}

func TestHooksTools_PostEdit_MissingParams(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	// Missing filePath
	result, err := ht.Execute(ctx, "hooks/post-edit", map[string]interface{}{
		"operation": "modify",
		"success":   true,
	})
	if err == nil {
		t.Error("expected error for missing filePath")
	}
	if result != nil && result.Success {
		t.Error("result should indicate failure")
	}

	// Missing operation
	result, err = ht.Execute(ctx, "hooks/post-edit", map[string]interface{}{
		"filePath": "/path/to/file.go",
		"success":  true,
	})
	if err == nil {
		t.Error("expected error for missing operation")
	}
	if result != nil && result.Success {
		t.Error("result should indicate failure")
	}
}

func TestHooksTools_PreCommand(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()
	params := map[string]interface{}{
		"command":               "rm -rf /tmp/test",
		"workingDirectory":      "/home/user",
		"includeRiskAssessment": true,
		"includeSuggestions":    true,
	}

	mcpResult, err := ht.Execute(ctx, "hooks/pre-command", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	preCommandResult, ok := mcpResult.Data.(*shared.PreCommandResult)
	if !ok {
		t.Fatalf("expected *shared.PreCommandResult, got %T", mcpResult.Data)
	}

	if preCommandResult.Command != "rm -rf /tmp/test" {
		t.Errorf("expected command 'rm -rf /tmp/test', got '%s'", preCommandResult.Command)
	}

	// Risk assessment should be present
	if preCommandResult.RiskAssessment == nil {
		t.Error("expected risk assessment to be present")
	}
}

func TestHooksTools_PreCommand_MissingParams(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	// Missing command
	result, err := ht.Execute(ctx, "hooks/pre-command", map[string]interface{}{
		"workingDirectory": "/home/user",
	})
	if err == nil {
		t.Error("expected error for missing command")
	}
	if result != nil && result.Success {
		t.Error("result should indicate failure")
	}
}

func TestHooksTools_PreCommand_RiskAssessment(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	tests := []struct {
		command       string
		expectedLevel shared.RiskLevel
	}{
		{"ls -la", shared.RiskLevelLow},
		{"rm -rf /", shared.RiskLevelCritical},
		{"chmod 777 file", shared.RiskLevelHigh},
		{"echo hello", shared.RiskLevelLow},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			params := map[string]interface{}{
				"command":               tt.command,
				"includeRiskAssessment": true,
			}

			mcpResult, err := ht.Execute(ctx, "hooks/pre-command", params)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			preCommandResult := mcpResult.Data.(*shared.PreCommandResult)
			if preCommandResult.RiskAssessment.Level != tt.expectedLevel {
				t.Errorf("expected risk level %s for '%s', got %s",
					tt.expectedLevel, tt.command, preCommandResult.RiskAssessment.Level)
			}
		})
	}
}

func TestHooksTools_PostCommand(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()
	params := map[string]interface{}{
		"command":         "go test ./...",
		"exitCode":        float64(0),
		"success":         true,
		"output":          "PASS",
		"executionTimeMs": float64(1500),
	}

	mcpResult, err := ht.Execute(ctx, "hooks/post-command", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	postCommandResult, ok := mcpResult.Data.(*shared.PostCommandResult)
	if !ok {
		t.Fatalf("expected *shared.PostCommandResult, got %T", mcpResult.Data)
	}

	if !postCommandResult.Recorded {
		t.Error("expected Recorded to be true")
	}

	if postCommandResult.PatternID == "" {
		t.Error("expected PatternID to be set")
	}

	// Verify pattern was stored
	patternStore := manager.GetPatternStore()
	if patternStore.CountByType(shared.PatternTypeCommand) != 1 {
		t.Errorf("expected 1 command pattern, got %d", patternStore.CountByType(shared.PatternTypeCommand))
	}
}

func TestHooksTools_PostCommand_MissingParams(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	// Missing command
	result, err := ht.Execute(ctx, "hooks/post-command", map[string]interface{}{
		"exitCode": float64(0),
		"success":  true,
	})
	if err == nil {
		t.Error("expected error for missing command")
	}
	if result != nil && result.Success {
		t.Error("result should indicate failure")
	}
}

func TestHooksTools_Route(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()
	params := map[string]interface{}{
		"task": "implement a new API endpoint",
		"context": map[string]interface{}{
			"language": "go",
		},
		"preferredAgents":    []interface{}{"coder"},
		"includeExplanation": true,
	}

	mcpResult, err := ht.Execute(ctx, "hooks/route", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	routeResult, ok := mcpResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", mcpResult.Data)
	}

	if routeResult["routingId"] == "" {
		t.Error("expected routingId to be set")
	}

	if routeResult["recommendedAgent"] == "" {
		t.Error("expected recommendedAgent to be set")
	}

	if routeResult["confidence"] == nil {
		t.Error("expected confidence to be set")
	}

	// Explanation should be present since includeExplanation=true
	if routeResult["explanation"] == nil {
		t.Error("expected explanation to be present")
	}
}

func TestHooksTools_Route_MissingParams(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	// Missing task
	result, err := ht.Execute(ctx, "hooks/route", map[string]interface{}{
		"context": map[string]interface{}{},
	})
	if err == nil {
		t.Error("expected error for missing task")
	}
	if result != nil && result.Success {
		t.Error("result should indicate failure")
	}
}

func TestHooksTools_Explain(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	// First route some tasks to build history
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		params := map[string]interface{}{
			"task": "implement feature",
		}
		_, _ = ht.Execute(ctx, "hooks/route", params)
	}

	// Now explain
	params := map[string]interface{}{
		"task":    "implement a new feature",
		"verbose": true,
	}

	mcpResult, err := ht.Execute(ctx, "hooks/explain", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	explainResult, ok := mcpResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", mcpResult.Data)
	}

	if explainResult["task"] == "" {
		t.Error("expected task in result")
	}

	if explainResult["analysis"] == "" {
		t.Error("expected analysis in result")
	}

	// Verbose should include historical data
	if explainResult["historicalData"] == nil {
		t.Error("expected historicalData in verbose mode")
	}
}

func TestHooksTools_Explain_MissingParams(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	// Missing task
	result, err := ht.Execute(ctx, "hooks/explain", map[string]interface{}{
		"verbose": true,
	})
	if err == nil {
		t.Error("expected error for missing task")
	}
	if result != nil && result.Success {
		t.Error("result should indicate failure")
	}
}

func TestHooksTools_Pretrain(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()
	params := map[string]interface{}{
		"repositoryPath":    "/path/to/repo",
		"maxPatterns":       float64(500),
		"includeGitHistory": true,
		"force":             false,
	}

	mcpResult, err := ht.Execute(ctx, "hooks/pretrain", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	pretrainResult, ok := mcpResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", mcpResult.Data)
	}

	if pretrainResult["repositoryPath"] != "/path/to/repo" {
		t.Errorf("expected repositoryPath '/path/to/repo', got '%v'", pretrainResult["repositoryPath"])
	}

	if pretrainResult["maxPatterns"] != 500 {
		t.Errorf("expected maxPatterns 500, got %v", pretrainResult["maxPatterns"])
	}
}

func TestHooksTools_Pretrain_MissingParams(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	// Missing repositoryPath
	result, err := ht.Execute(ctx, "hooks/pretrain", map[string]interface{}{
		"maxPatterns": float64(500),
	})
	if err == nil {
		t.Error("expected error for missing repositoryPath")
	}
	if result != nil && result.Success {
		t.Error("result should indicate failure")
	}
}

func TestHooksTools_Metrics(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	_ = manager.Initialize()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	// Record some activity first
	for i := 0; i < 3; i++ {
		params := map[string]interface{}{
			"task": "test task",
		}
		_, _ = ht.Execute(ctx, "hooks/route", params)
	}

	// Get all metrics
	params := map[string]interface{}{
		"category": "all",
		"format":   "detailed",
	}

	mcpResult, err := ht.Execute(ctx, "hooks/metrics", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	metricsResult, ok := mcpResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", mcpResult.Data)
	}

	if metricsResult["summary"] == nil {
		t.Error("expected summary in result")
	}

	if metricsResult["routing"] == nil {
		t.Error("expected routing in detailed format")
	}
}

func TestHooksTools_Metrics_Categories(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	categories := []string{"routing", "patterns", "hooks"}

	for _, category := range categories {
		t.Run(category, func(t *testing.T) {
			params := map[string]interface{}{
				"category": category,
			}

			mcpResult, err := ht.Execute(ctx, "hooks/metrics", params)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !mcpResult.Success {
				t.Errorf("expected success, got error: %s", mcpResult.Error)
			}

			metricsResult := mcpResult.Data.(map[string]interface{})
			if metricsResult[category] == nil {
				t.Errorf("expected %s in result", category)
			}
		})
	}
}

func TestHooksTools_List(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	// Register some hooks
	for i := 0; i < 3; i++ {
		hook := &shared.HookRegistration{
			ID:          shared.GenerateID("hook"),
			Name:        "Test Hook",
			Event:       shared.HookEventPreEdit,
			Priority:    shared.HookPriorityNormal,
			Description: "A test hook",
		}
		_ = manager.Register(hook)
	}

	ctx := context.Background()
	params := map[string]interface{}{
		"includeDisabled": true,
		"includeMetadata": true,
	}

	mcpResult, err := ht.Execute(ctx, "hooks/list", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	listResult, ok := mcpResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", mcpResult.Data)
	}

	hooksList := listResult["hooks"].([]map[string]interface{})
	if len(hooksList) != 3 {
		t.Errorf("expected 3 hooks, got %d", len(hooksList))
	}

	if listResult["total"].(int) != 3 {
		t.Errorf("expected total 3, got %v", listResult["total"])
	}
}

func TestHooksTools_List_FilterByCategory(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	// Register hooks for different events
	events := []shared.HookEvent{shared.HookEventPreEdit, shared.HookEventPreEdit, shared.HookEventPreCommand}
	for _, event := range events {
		hook := &shared.HookRegistration{
			ID:    shared.GenerateID("hook"),
			Event: event,
		}
		_ = manager.Register(hook)
	}

	ctx := context.Background()
	params := map[string]interface{}{
		"category": string(shared.HookEventPreEdit),
	}

	mcpResult, err := ht.Execute(ctx, "hooks/list", params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mcpResult.Success {
		t.Errorf("expected success, got error: %s", mcpResult.Error)
	}

	listResult := mcpResult.Data.(map[string]interface{})
	if listResult["total"].(int) != 2 {
		t.Errorf("expected 2 pre-edit hooks, got %v", listResult["total"])
	}
}

func TestHooksTools_AssessCommandRisk(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	tests := []struct {
		command       string
		expectedLevel shared.RiskLevel
		shouldProceed bool
	}{
		{"ls -la", shared.RiskLevelLow, true},
		{"rm -rf /", shared.RiskLevelCritical, false},
		{"dd if=/dev/zero of=/dev/sda", shared.RiskLevelCritical, false},
		{"chmod 777 sensitive_file", shared.RiskLevelHigh, true},
		{"git push --force", shared.RiskLevelMedium, true},
		{"echo hello", shared.RiskLevelLow, true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			assessment := ht.assessCommandRisk(tt.command, "/home/user")

			if assessment.Level != tt.expectedLevel {
				t.Errorf("expected level %s, got %s", tt.expectedLevel, assessment.Level)
			}

			if assessment.ShouldProceed != tt.shouldProceed {
				t.Errorf("expected shouldProceed %v, got %v", tt.shouldProceed, assessment.ShouldProceed)
			}
		})
	}
}

func TestHooksTools_GenerateCommandSuggestions(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	// Store some successful patterns
	patternStore := manager.GetPatternStore()
	pattern := &shared.Pattern{
		ID:           "cmd-1",
		Type:         shared.PatternTypeCommand,
		Content:      "rm -i file.txt",
		Keywords:     []string{"rm", "file"},
		SuccessCount: 10,
	}
	_ = patternStore.Store(pattern)

	patterns := patternStore.FindSimilar("rm file", shared.PatternTypeCommand, 5)
	suggestions := ht.generateCommandSuggestions("rm file.txt", patterns)

	if len(suggestions) == 0 {
		t.Error("expected at least one suggestion")
	}
}

func TestHooksTools_Integration(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	_ = manager.Initialize()
	ht := NewHooksTools(manager)

	ctx := context.Background()

	// 1. Pre-edit
	preEditParams := map[string]interface{}{
		"filePath":  "main.go",
		"operation": "modify",
	}
	mcpResult, err := ht.Execute(ctx, "hooks/pre-edit", preEditParams)
	if err != nil {
		t.Errorf("pre-edit failed: %v", err)
	}
	if !mcpResult.Success {
		t.Errorf("pre-edit should succeed: %s", mcpResult.Error)
	}

	// 2. Post-edit
	postEditParams := map[string]interface{}{
		"filePath":  "main.go",
		"operation": "modify",
		"success":   true,
	}
	mcpResult, err = ht.Execute(ctx, "hooks/post-edit", postEditParams)
	if err != nil {
		t.Errorf("post-edit failed: %v", err)
	}
	if !mcpResult.Success {
		t.Errorf("post-edit should succeed: %s", mcpResult.Error)
	}

	// 3. Pre-command
	preCommandParams := map[string]interface{}{
		"command": "go build",
	}
	mcpResult, err = ht.Execute(ctx, "hooks/pre-command", preCommandParams)
	if err != nil {
		t.Errorf("pre-command failed: %v", err)
	}
	if !mcpResult.Success {
		t.Errorf("pre-command should succeed: %s", mcpResult.Error)
	}

	// 4. Post-command
	postCommandParams := map[string]interface{}{
		"command":  "go build",
		"exitCode": float64(0),
		"success":  true,
	}
	mcpResult, err = ht.Execute(ctx, "hooks/post-command", postCommandParams)
	if err != nil {
		t.Errorf("post-command failed: %v", err)
	}
	if !mcpResult.Success {
		t.Errorf("post-command should succeed: %s", mcpResult.Error)
	}

	// 5. Route
	routeParams := map[string]interface{}{
		"task": "implement feature",
	}
	mcpResult, err = ht.Execute(ctx, "hooks/route", routeParams)
	if err != nil {
		t.Errorf("route failed: %v", err)
	}
	if !mcpResult.Success {
		t.Errorf("route should succeed: %s", mcpResult.Error)
	}
	routeMap := mcpResult.Data.(map[string]interface{})

	// 6. Explain
	explainParams := map[string]interface{}{
		"task": "implement feature",
	}
	mcpResult, err = ht.Execute(ctx, "hooks/explain", explainParams)
	if err != nil {
		t.Errorf("explain failed: %v", err)
	}
	if !mcpResult.Success {
		t.Errorf("explain should succeed: %s", mcpResult.Error)
	}

	// 7. Pretrain (simulation)
	pretrainParams := map[string]interface{}{
		"repositoryPath": "/tmp/test",
	}
	mcpResult, err = ht.Execute(ctx, "hooks/pretrain", pretrainParams)
	if err != nil {
		t.Errorf("pretrain failed: %v", err)
	}
	if !mcpResult.Success {
		t.Errorf("pretrain should succeed: %s", mcpResult.Error)
	}

	// 8. Metrics
	metricsParams := map[string]interface{}{
		"category": "all",
	}
	mcpResult, err = ht.Execute(ctx, "hooks/metrics", metricsParams)
	if err != nil {
		t.Errorf("metrics failed: %v", err)
	}
	if !mcpResult.Success {
		t.Errorf("metrics should succeed: %s", mcpResult.Error)
	}
	metricsMap := mcpResult.Data.(map[string]interface{})
	summary := metricsMap["summary"].(map[string]interface{})

	// Verify metrics show our activity
	if summary["totalPatterns"].(int64) < 2 {
		t.Errorf("expected at least 2 patterns, got %v", summary["totalPatterns"])
	}
	if summary["totalRoutings"].(int64) < 1 {
		t.Errorf("expected at least 1 routing, got %v", summary["totalRoutings"])
	}

	// 9. List
	listParams := map[string]interface{}{}
	mcpResult, err = ht.Execute(ctx, "hooks/list", listParams)
	if err != nil {
		t.Errorf("list failed: %v", err)
	}
	if !mcpResult.Success {
		t.Errorf("list should succeed: %s", mcpResult.Error)
	}

	// Verify routing ID was returned
	if routeMap["routingId"] == "" {
		t.Error("route should return a routingId")
	}
}

func TestHooksTools_ToolParameters(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	tools := ht.GetTools()

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			if tool.Description == "" {
				t.Error("tool should have a description")
			}

			params := tool.Parameters

			if params["type"] != "object" {
				t.Error("parameters type should be 'object'")
			}

			if params["properties"] == nil {
				t.Error("parameters should have properties")
			}
		})
	}
}

func TestHooksTools_ImplementsMCPToolProvider(t *testing.T) {
	manager := hooks.NewHooksManagerWithDefaults()
	ht := NewHooksTools(manager)

	// This test verifies that HooksTools implements MCPToolProvider interface
	var _ shared.MCPToolProvider = ht
}
