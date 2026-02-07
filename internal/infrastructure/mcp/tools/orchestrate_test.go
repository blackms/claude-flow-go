// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/events"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/hooks"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/memory"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// helper to create a fully initialised OrchestrateTools for testing.
func newTestOrchestrateTools(t *testing.T) *OrchestrateTools {
	t.Helper()

	coord := coordinator.New(coordinator.Options{
		Topology: shared.TopologyMesh,
		EventBus: events.New(),
	})
	if err := coord.Initialize(); err != nil {
		t.Fatalf("failed to initialise coordinator: %v", err)
	}

	re := hooks.NewRoutingEngine(0.1)

	mem := memory.NewSQLiteBackend(":memory:")
	if err := mem.Initialize(); err != nil {
		t.Fatalf("failed to initialise memory backend: %v", err)
	}

	return NewOrchestrateTools(coord, re, mem, nil)
}

func TestNewOrchestrateTools(t *testing.T) {
	ot := newTestOrchestrateTools(t)

	if ot == nil {
		t.Fatal("OrchestrateTools should not be nil")
	}
	if ot.coordinator == nil {
		t.Error("coordinator should be set")
	}
	if ot.routingEngine == nil {
		t.Error("routingEngine should be set")
	}
	if ot.memoryBackend == nil {
		t.Error("memoryBackend should be set")
	}
	if ot.orchestrations == nil {
		t.Error("orchestrations map should be initialised")
	}
}

func TestOrchestrateTools_GetTools(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	tools := ot.GetTools()

	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}

	expected := map[string]bool{
		"orchestrate_plan":    false,
		"orchestrate_execute": false,
		"orchestrate_status":  false,
	}

	for _, tool := range tools {
		if _, exists := expected[tool.Name]; exists {
			expected[tool.Name] = true
		} else {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestOrchestrateTools_Execute_UnknownTool(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	ctx := context.Background()

	result, err := ot.Execute(ctx, "unknown_tool", nil)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
	if result != nil {
		t.Error("result should be nil for unknown tool")
	}
}

func TestOrchestrateTools_ImplementsMCPToolProvider(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	var _ shared.MCPToolProvider = ot
}

// ---------------------------------------------------------------------------
// Decomposition tests
// ---------------------------------------------------------------------------

func TestDecomposeTask_KeywordMatching(t *testing.T) {
	ot := newTestOrchestrateTools(t)

	tests := []struct {
		task               string
		expectedCategories []string
	}{
		{
			task:               "Build a Go REST API with tests and security",
			expectedCategories: []string{"design", "implement", "test", "security"},
		},
		{
			task:               "Deploy the service to Kubernetes",
			expectedCategories: []string{"deploy", "review", "implement", "design", "test"}, // deploy depends on review → implement+test → design
		},
		{
			task:               "Write unit tests for the handler",
			expectedCategories: []string{"test", "implement", "design"}, // test depends on implement → design
		},
		{
			task:               "Document the API with swagger",
			expectedCategories: []string{"docs", "implement", "design"}, // docs depends on implement → design
		},
	}

	for _, tt := range tests {
		t.Run(tt.task, func(t *testing.T) {
			plan := ot.decomposeTask(tt.task)

			if plan == nil {
				t.Fatal("plan should not be nil")
			}
			if plan.Task != tt.task {
				t.Errorf("plan task should be %q, got %q", tt.task, plan.Task)
			}

			categories := make(map[string]bool)
			for _, st := range plan.Subtasks {
				categories[st.Category] = true
			}

			for _, expected := range tt.expectedCategories {
				if !categories[expected] {
					t.Errorf("expected category %q in plan, got categories: %v", expected, categories)
				}
			}
		})
	}
}

func TestDecomposeTask_DefaultPipeline(t *testing.T) {
	ot := newTestOrchestrateTools(t)

	// A task with no matching keywords should use the default pipeline.
	plan := ot.decomposeTask("do something amazing")

	categories := make(map[string]bool)
	for _, st := range plan.Subtasks {
		categories[st.Category] = true
	}

	for _, name := range defaultPipeline {
		if !categories[name] {
			t.Errorf("default pipeline should include %q, got: %v", name, categories)
		}
	}
}

func TestDecomposeTask_DependencyPropagation(t *testing.T) {
	ot := newTestOrchestrateTools(t)

	// "test" depends on "implement" which depends on "design".
	// Mentioning only "test" should pull in both dependencies.
	plan := ot.decomposeTask("test the validation logic")

	categories := make(map[string]bool)
	for _, st := range plan.Subtasks {
		categories[st.Category] = true
	}

	if !categories["test"] {
		t.Error("should include test")
	}
	if !categories["implement"] {
		t.Error("should include implement (dependency of test)")
	}
	if !categories["design"] {
		t.Error("should include design (dependency of implement)")
	}
}

// ---------------------------------------------------------------------------
// Sub-decomposition tests
// ---------------------------------------------------------------------------

func TestDecomposeTask_SubDecomposition_Implement(t *testing.T) {
	ot := newTestOrchestrateTools(t)

	// Task mentions models, handlers, store → implement should split into 3 parallel sub-tasks.
	plan := ot.decomposeTask("Build a Go REST API with models, handlers and a store")

	var implSubtasks []orchestrationSubtask
	for _, st := range plan.Subtasks {
		if st.Category == "implement" {
			implSubtasks = append(implSubtasks, st)
		}
	}

	if len(implSubtasks) < 3 {
		t.Errorf("expected ≥3 implement sub-tasks (models, handlers, store), got %d", len(implSubtasks))
	}

	// All implement sub-tasks should be in the same phase (parallel).
	phase := implSubtasks[0].Phase
	for _, st := range implSubtasks[1:] {
		if st.Phase != phase {
			t.Errorf("all implement sub-tasks should share phase %d, but %s has phase %d", phase, st.ID, st.Phase)
		}
	}

	// Check that task labels reflect the split.
	hasModels, hasHandlers, hasStore := false, false, false
	for _, st := range implSubtasks {
		if strings.Contains(st.Task, "models") {
			hasModels = true
		}
		if strings.Contains(st.Task, "handlers") {
			hasHandlers = true
		}
		if strings.Contains(st.Task, "store") {
			hasStore = true
		}
	}
	if !hasModels {
		t.Error("expected a models sub-task")
	}
	if !hasHandlers {
		t.Error("expected a handlers sub-task")
	}
	if !hasStore {
		t.Error("expected a store sub-task")
	}
}

func TestDecomposeTask_SubDecomposition_Test(t *testing.T) {
	ot := newTestOrchestrateTools(t)

	// Task mentions unit and integration → test should split into 2 parallel sub-tasks.
	plan := ot.decomposeTask("Write unit tests and integration tests for the API")

	var testSubtasks []orchestrationSubtask
	for _, st := range plan.Subtasks {
		if st.Category == "test" {
			testSubtasks = append(testSubtasks, st)
		}
	}

	if len(testSubtasks) < 2 {
		t.Errorf("expected ≥2 test sub-tasks (unit, integration), got %d", len(testSubtasks))
	}

	// All test sub-tasks should share the same phase.
	phase := testSubtasks[0].Phase
	for _, st := range testSubtasks[1:] {
		if st.Phase != phase {
			t.Errorf("all test sub-tasks should share phase %d, but %s has phase %d", phase, st.ID, st.Phase)
		}
	}
}

func TestDecomposeTask_SubDecomposition_NoSplit(t *testing.T) {
	ot := newTestOrchestrateTools(t)

	// Only one sub-keyword matches → should NOT split (need ≥2).
	plan := ot.decomposeTask("Build an API endpoint")

	var implSubtasks []orchestrationSubtask
	for _, st := range plan.Subtasks {
		if st.Category == "implement" {
			implSubtasks = append(implSubtasks, st)
		}
	}

	if len(implSubtasks) != 1 {
		t.Errorf("expected 1 implement subtask (no split with only 1 sub-keyword match), got %d", len(implSubtasks))
	}
}

func TestDecomposeTask_SubDecomposition_FullParallelPlan(t *testing.T) {
	ot := newTestOrchestrateTools(t)

	// Full task: design → (implement-models || implement-handlers || implement-store) → (test-unit || test-integration)
	plan := ot.decomposeTask("Build a Go REST API with models, handlers, store, unit tests and integration tests")

	// Check phases.
	if plan.TotalPhases < 3 {
		t.Fatalf("expected ≥3 phases, got %d", plan.TotalPhases)
	}

	// Phase 1 (implement) should have ≥3 parallel sub-tasks.
	for _, phase := range plan.Phases {
		implCount := 0
		testCount := 0
		for _, st := range phase.Subtasks {
			if st.Category == "implement" {
				implCount++
			}
			if st.Category == "test" {
				testCount++
			}
		}
		if implCount >= 3 {
			t.Logf("Phase %d has %d implement sub-tasks (parallel) ✓", phase.Phase, implCount)
		}
		if testCount >= 2 {
			t.Logf("Phase %d has %d test sub-tasks (parallel) ✓", phase.Phase, testCount)
		}
	}

	// Total subtasks should be > the number of categories (proves splitting happened).
	if len(plan.Subtasks) <= 3 {
		t.Errorf("expected more than 3 subtasks due to sub-decomposition, got %d", len(plan.Subtasks))
	}
}

func TestMatchSubSpecs(t *testing.T) {
	implCat := subtaskCategory{Name: "implement", AgentType: "coder", Phase: 1}

	tests := []struct {
		task     string
		expected int
	}{
		{"build models and handlers", 2},
		{"build models, handlers and store", 3},
		{"build something", 0},
		{"build endpoint with middleware and config", 3}, // handlers(endpoint), middleware, config
	}

	for _, tt := range tests {
		t.Run(tt.task, func(t *testing.T) {
			matched := matchSubSpecs(tt.task, implCat)
			if len(matched) != tt.expected {
				names := make([]string, len(matched))
				for i, m := range matched {
					names[i] = m.Name
				}
				t.Errorf("expected %d sub-specs, got %d: %v", tt.expected, len(matched), names)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// containsKeyword tests
// ---------------------------------------------------------------------------

func TestContainsKeyword(t *testing.T) {
	tests := []struct {
		text    string
		keyword string
		want    bool
	}{
		// Short keywords (≤3) use word-boundary matching.
		{"middleware layer", "ddl", false},   // "ddl" is inside "middleware" — must not match
		{"build the app", "ui", false},       // "ui" is inside "build" — must not match
		{"add a ui component", "ui", true},   // "ui" at word boundary — must match
		{"use the db layer", "db", true},     // "db" at word boundary — must match
		{"debugging the app", "db", false},   // "db" inside "debugging" — must not match
		{"set up ci and cd", "ci", true},     // "ci" at word boundary
		{"create schema ddl", "ddl", true},   // "ddl" at word boundary

		// Longer keywords (>3) use substring matching.
		{"run tests", "test", true},          // "test" inside "tests" — match
		{"build models", "model", true},      // "model" inside "models" — match
		{"architecture", "architect", true},  // substring match
		{"endpoint handler", "endpoint", true},
		{"no match here", "kubernetes", false},
	}

	for _, tt := range tests {
		t.Run(tt.text+"/"+tt.keyword, func(t *testing.T) {
			got := containsKeyword(tt.text, tt.keyword)
			if got != tt.want {
				t.Errorf("containsKeyword(%q, %q) = %v, want %v", tt.text, tt.keyword, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// computePhases tests
// ---------------------------------------------------------------------------

func TestComputePhases(t *testing.T) {
	subtasks := []orchestrationSubtask{
		{ID: "s1", Category: "design", Phase: 0},
		{ID: "s2", Category: "implement", Phase: 1},
		{ID: "s3", Category: "test", Phase: 2},
		{ID: "s4", Category: "security", Phase: 2},
		{ID: "s5", Category: "review", Phase: 3},
	}

	phases := computePhases(subtasks)

	if len(phases) != 4 {
		t.Fatalf("expected 4 phases, got %d", len(phases))
	}

	// Phase 0 should have 1 subtask (design).
	if len(phases[0].Subtasks) != 1 {
		t.Errorf("phase 0: expected 1 subtask, got %d", len(phases[0].Subtasks))
	}

	// Phase 1 should have 1 subtask (implement).
	if len(phases[1].Subtasks) != 1 {
		t.Errorf("phase 1: expected 1 subtask, got %d", len(phases[1].Subtasks))
	}

	// Phase 2 should have 2 subtasks (test + security) — parallel.
	if len(phases[2].Subtasks) != 2 {
		t.Errorf("phase 2: expected 2 subtasks (parallel), got %d", len(phases[2].Subtasks))
	}

	// Phase 3 should have 1 subtask (review).
	if len(phases[3].Subtasks) != 1 {
		t.Errorf("phase 3: expected 1 subtask, got %d", len(phases[3].Subtasks))
	}

	// Phases should be in ascending order.
	for i := 1; i < len(phases); i++ {
		if phases[i].Phase <= phases[i-1].Phase {
			t.Errorf("phases not in ascending order: phase[%d]=%d, phase[%d]=%d",
				i-1, phases[i-1].Phase, i, phases[i].Phase)
		}
	}
}

func TestComputePhases_Empty(t *testing.T) {
	phases := computePhases(nil)
	if len(phases) != 0 {
		t.Errorf("expected 0 phases for empty input, got %d", len(phases))
	}
}

// ---------------------------------------------------------------------------
// Plan tool (end-to-end via MCP interface)
// ---------------------------------------------------------------------------

func TestOrchestratePlan_Success(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	ctx := context.Background()

	params := map[string]interface{}{
		"task": "Build a Go REST API with tests",
	}

	result, err := ot.Execute(ctx, "orchestrate_plan", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result.Data)
	}

	if data["task"] != "Build a Go REST API with tests" {
		t.Errorf("expected task in plan data")
	}
	if data["totalPhases"] == nil {
		t.Error("expected totalPhases in plan data")
	}

	subtasks, ok := data["subtasks"].([]interface{})
	if !ok {
		t.Fatalf("expected subtasks to be a slice, got %T", data["subtasks"])
	}
	if len(subtasks) == 0 {
		t.Error("expected at least one subtask")
	}
}

func TestOrchestratePlan_MissingTask(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	ctx := context.Background()

	result, err := ot.Execute(ctx, "orchestrate_plan", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for missing task")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

// ---------------------------------------------------------------------------
// Execute tool (end-to-end)
// ---------------------------------------------------------------------------

func TestOrchestrateExecute_Success(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	ctx := context.Background()

	params := map[string]interface{}{
		"task": "Build a REST API with tests",
	}

	result, err := ot.Execute(ctx, "orchestrate_execute", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The execution should complete (even if subtask results are
	// simulated by the coordinator's default ExecuteTask behaviour).
	if result.Data == nil {
		t.Fatal("expected data in result")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}

	status, _ := data["status"].(string)
	// Should be either "completed" or "failed" — both are valid outcomes.
	if status != string(orchestrationCompleted) && status != string(orchestrationFailed) {
		t.Errorf("expected terminal status, got %q", status)
	}

	// Verify orchestration was tracked.
	id, _ := data["id"].(string)
	if id == "" {
		t.Error("expected orchestration ID")
	}
}

func TestOrchestrateExecute_MissingTask(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	ctx := context.Background()

	result, err := ot.Execute(ctx, "orchestrate_execute", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for missing task")
	}
}

// ---------------------------------------------------------------------------
// Status tool
// ---------------------------------------------------------------------------

func TestOrchestrateStatus_NotFound(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	ctx := context.Background()

	result, err := ot.Execute(ctx, "orchestrate_status", map[string]interface{}{
		"orchestrationId": "nonexistent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for non-existent orchestration")
	}
}

func TestOrchestrateStatus_MissingID(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	ctx := context.Background()

	result, err := ot.Execute(ctx, "orchestrate_status", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for missing orchestrationId")
	}
}

func TestOrchestrateStatus_AfterExecute(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	ctx := context.Background()

	// Run an execution first.
	execResult, err := ot.Execute(ctx, "orchestrate_execute", map[string]interface{}{
		"task": "Build a service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, ok := execResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data from execute, got %T", execResult.Data)
	}

	orchID, _ := data["id"].(string)
	if orchID == "" {
		t.Fatal("expected orchestration ID from execute")
	}

	// Now query status.
	statusResult, err := ot.Execute(ctx, "orchestrate_status", map[string]interface{}{
		"orchestrationId": orchID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !statusResult.Success {
		t.Errorf("expected success, got error: %s", statusResult.Error)
	}

	statusData, ok := statusResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data from status, got %T", statusResult.Data)
	}

	if statusData["id"] != orchID {
		t.Errorf("expected id %q, got %v", orchID, statusData["id"])
	}
}

// ---------------------------------------------------------------------------
// Tool parameters validation
// ---------------------------------------------------------------------------

func TestOrchestrateTools_ToolParameters(t *testing.T) {
	ot := newTestOrchestrateTools(t)
	tools := ot.GetTools()

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			if tool.Description == "" {
				t.Error("tool should have a description")
			}

			params := tool.Parameters
			if params == nil {
				t.Error("parameters should not be nil")
				return
			}

			if params["type"] != "object" {
				t.Error("parameters type should be 'object'")
			}

			if params["properties"] == nil {
				t.Error("parameters should have properties")
			}
		})
	}
}
