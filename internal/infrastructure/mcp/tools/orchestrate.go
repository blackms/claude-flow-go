// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/infrastructure/hooks"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// orchestrationStatus represents the status of an orchestration.
type orchestrationStatus string

const (
	orchestrationRunning    orchestrationStatus = "running"
	orchestrationCompleted  orchestrationStatus = "completed"
	orchestrationFailed     orchestrationStatus = "failed"

	// maxOrchestrations is the maximum number of orchestrations tracked in memory.
	// When exceeded, the oldest completed orchestrations are evicted.
	maxOrchestrations = 1000
)

// subtaskCategory defines a category of work derived from keyword matching.
type subtaskCategory struct {
	Name      string
	Keywords  []string
	AgentType string
	Phase     int
	DependsOn []string
}

// defaultCategories defines the keyword → subtask mapping table.
var defaultCategories = []subtaskCategory{
	{Name: "design", Keywords: []string{"design", "architect", "structure", "plan", "schema", "model", "api"}, AgentType: "architect", Phase: 0, DependsOn: nil},
	{Name: "implement", Keywords: []string{"build", "implement", "create", "develop", "code", "write", "add", "feature", "endpoint", "handler", "rest", "crud"}, AgentType: "coder", Phase: 1, DependsOn: []string{"design"}},
	{Name: "test", Keywords: []string{"test", "spec", "coverage", "unit", "integration", "e2e", "validate"}, AgentType: "tdd-tester", Phase: 2, DependsOn: []string{"implement"}},
	{Name: "security", Keywords: []string{"security", "vulnerability", "threat", "auth", "encrypt"}, AgentType: "security-auditor", Phase: 2, DependsOn: []string{"implement"}},
	{Name: "review", Keywords: []string{"review", "audit", "inspect", "quality", "check"}, AgentType: "reviewer", Phase: 3, DependsOn: []string{"implement", "test"}},
	{Name: "docs", Keywords: []string{"document", "readme", "docs", "swagger", "openapi"}, AgentType: "documentation-lead", Phase: 2, DependsOn: []string{"implement"}},
	{Name: "optimize", Keywords: []string{"optimize", "performance", "benchmark", "speed", "cache"}, AgentType: "optimizer", Phase: 3, DependsOn: []string{"implement", "test"}},
	{Name: "deploy", Keywords: []string{"deploy", "release", "docker", "kubernetes", "ci", "cd", "pipeline"}, AgentType: "deployer", Phase: 4, DependsOn: []string{"review"}},
}

// defaultPipeline is used when no keywords match.
var defaultPipeline = []string{"design", "implement", "test", "review"}

// subSpec defines a sub-decomposition rule within a parent category.
type subSpec struct {
	Name      string   // sub-task name suffix, e.g. "models"
	Keywords  []string // keywords that trigger this sub-task
	AgentType string   // agent type override (empty = inherit parent)
	TaskLabel string   // label injected into the sub-task description
}

// subDecompositions maps parent category → sub-specs.
// When ≥2 sub-specs match, the parent is replaced by parallel sub-tasks.
var subDecompositions = map[string][]subSpec{
	"design": {
		{Name: "api-contract", Keywords: []string{"api", "rest", "graphql", "grpc", "endpoint", "contract", "openapi"}, TaskLabel: "API contract design"},
		{Name: "data-model", Keywords: []string{"model", "schema", "entity", "struct", "table", "relation", "erd"}, TaskLabel: "data model design"},
		{Name: "architecture", Keywords: []string{"architect", "system", "microservice", "monolith", "layer", "hexagonal", "clean"}, TaskLabel: "system architecture"},
		{Name: "ui-ux", Keywords: []string{"ui", "ux", "wireframe", "mockup", "component", "page", "screen", "layout"}, AgentType: "designer", TaskLabel: "UI/UX design"},
	},
	"implement": {
		{Name: "models", Keywords: []string{"model", "schema", "struct", "type", "entity", "domain"}, TaskLabel: "models & data structures"},
		{Name: "handlers", Keywords: []string{"handler", "endpoint", "route", "controller", "api", "rest", "crud", "http"}, TaskLabel: "handlers & endpoints"},
		{Name: "store", Keywords: []string{"store", "repository", "database", "db", "persistence", "storage"}, TaskLabel: "data store & persistence"},
		{Name: "service", Keywords: []string{"service", "usecase", "business", "logic", "workflow", "process"}, TaskLabel: "service & business logic"},
		{Name: "middleware", Keywords: []string{"middleware", "cors", "rate", "limit", "compress"}, TaskLabel: "middleware"},
		{Name: "validation", Keywords: []string{"validation", "validate", "sanitize", "constraint", "rule"}, TaskLabel: "input validation"},
		{Name: "migration", Keywords: []string{"migration", "migrate", "seed", "fixture", "ddl"}, TaskLabel: "database migrations"},
		{Name: "events", Keywords: []string{"event", "message", "queue", "kafka", "rabbitmq", "pubsub", "async", "webhook"}, TaskLabel: "events & messaging"},
		{Name: "worker", Keywords: []string{"worker", "job", "cron", "scheduler", "background", "batch"}, TaskLabel: "background workers"},
		{Name: "client", Keywords: []string{"client", "sdk", "consumer", "fetch", "request"}, TaskLabel: "client & SDK"},
		{Name: "config", Keywords: []string{"config", "setup", "init", "bootstrap", "wire", "env"}, TaskLabel: "configuration & setup"},
		{Name: "auth", Keywords: []string{"auth", "login", "signup", "session", "oauth", "jwt", "token", "password"}, TaskLabel: "authentication & authorization"},
	},
	"test": {
		{Name: "unit", Keywords: []string{"unit", "function", "method", "handler", "isolated"}, TaskLabel: "unit tests"},
		{Name: "integration", Keywords: []string{"integration", "e2e", "end-to-end", "acceptance", "api-test"}, TaskLabel: "integration tests"},
		{Name: "benchmark", Keywords: []string{"benchmark", "perf", "performance", "load", "stress"}, AgentType: "performance-engineer", TaskLabel: "benchmark tests"},
		{Name: "contract", Keywords: []string{"contract", "pact", "consumer", "provider", "compatibility"}, TaskLabel: "contract tests"},
		{Name: "fuzz", Keywords: []string{"fuzz", "fuzzing", "random", "property", "generative"}, TaskLabel: "fuzz & property tests"},
	},
	"security": {
		{Name: "auth", Keywords: []string{"auth", "authentication", "authorization", "oauth", "jwt", "token", "rbac", "acl"}, TaskLabel: "authentication & authorization"},
		{Name: "scan", Keywords: []string{"vulnerability", "cve", "scan", "dependency", "sast", "dast"}, AgentType: "cve-remediation", TaskLabel: "vulnerability scanning"},
		{Name: "threat", Keywords: []string{"threat", "model", "attack", "surface", "stride"}, AgentType: "threat-modeler", TaskLabel: "threat modeling"},
		{Name: "crypto", Keywords: []string{"encrypt", "decrypt", "hash", "secret", "certificate", "tls", "ssl"}, TaskLabel: "cryptography & secrets"},
		{Name: "input", Keywords: []string{"injection", "xss", "csrf", "sanitize", "escape", "input"}, TaskLabel: "input security & sanitization"},
	},
	"docs": {
		{Name: "api-docs", Keywords: []string{"swagger", "openapi", "redoc", "postman", "endpoint"}, TaskLabel: "API documentation"},
		{Name: "readme", Keywords: []string{"readme", "getting-started", "quickstart", "install", "setup"}, TaskLabel: "README & getting started"},
		{Name: "architecture", Keywords: []string{"architect", "adr", "decision", "diagram", "c4", "system"}, TaskLabel: "architecture documentation"},
		{Name: "runbook", Keywords: []string{"runbook", "ops", "incident", "troubleshoot", "monitor"}, TaskLabel: "runbooks & operations"},
		{Name: "changelog", Keywords: []string{"changelog", "release-notes", "migration-guide", "upgrade"}, TaskLabel: "changelog & release notes"},
	},
	"deploy": {
		{Name: "container", Keywords: []string{"docker", "dockerfile", "container", "image", "compose", "podman"}, AgentType: "devops-engineer", TaskLabel: "containerization"},
		{Name: "kubernetes", Keywords: []string{"kubernetes", "k8s", "helm", "kustomize", "manifest", "pod", "service"}, AgentType: "devops-engineer", TaskLabel: "Kubernetes deployment"},
		{Name: "ci", Keywords: []string{"ci", "github-actions", "gitlab-ci", "jenkins", "pipeline", "lint"}, AgentType: "devops-engineer", TaskLabel: "CI pipeline"},
		{Name: "cd", Keywords: []string{"cd", "deploy", "release", "rollback", "canary", "blue-green", "argocd"}, AgentType: "release-manager", TaskLabel: "CD & release"},
		{Name: "infra", Keywords: []string{"terraform", "pulumi", "cloudformation", "infra", "iac", "provision"}, AgentType: "devops-engineer", TaskLabel: "infrastructure as code"},
		{Name: "monitoring", Keywords: []string{"monitor", "alert", "prometheus", "grafana", "datadog", "observ", "trace", "log"}, AgentType: "devops-engineer", TaskLabel: "monitoring & observability"},
	},
	"review": {
		{Name: "code", Keywords: []string{"code", "implementation", "logic", "clean", "refactor", "dry", "solid"}, TaskLabel: "code quality review"},
		{Name: "architecture", Keywords: []string{"architect", "design", "pattern", "structure", "coupling", "cohesion"}, TaskLabel: "architecture review"},
		{Name: "security", Keywords: []string{"security", "vulnerability", "owasp", "injection", "auth"}, AgentType: "security-auditor", TaskLabel: "security review"},
		{Name: "performance", Keywords: []string{"performance", "bottleneck", "profil", "memory", "cpu", "latency"}, AgentType: "performance-engineer", TaskLabel: "performance review"},
	},
	"optimize": {
		{Name: "query", Keywords: []string{"query", "sql", "index", "database", "n+1", "join"}, TaskLabel: "query optimization"},
		{Name: "caching", Keywords: []string{"cache", "redis", "memcache", "cdn", "invalidat", "ttl"}, TaskLabel: "caching strategy"},
		{Name: "algorithm", Keywords: []string{"algorithm", "complexity", "sort", "search", "hash", "tree"}, TaskLabel: "algorithm optimization"},
		{Name: "memory", Keywords: []string{"memory", "leak", "gc", "allocation", "pool", "buffer"}, TaskLabel: "memory optimization"},
		{Name: "network", Keywords: []string{"network", "latency", "bandwidth", "compress", "batch", "connection"}, TaskLabel: "network optimization"},
		{Name: "concurrency", Keywords: []string{"concurrency", "parallel", "goroutine", "thread", "lock", "contention", "async"}, TaskLabel: "concurrency optimization"},
	},
}

// orchestrationSubtask represents a single unit of work within an orchestration.
type orchestrationSubtask struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"`
	AgentType string   `json:"agentType"`
	Phase     int      `json:"phase"`
	DependsOn []string `json:"dependsOn"`
	Task      string   `json:"task"`
}

// orchestrationPhase groups subtasks that can run in parallel.
type orchestrationPhase struct {
	Phase    int                    `json:"phase"`
	Subtasks []orchestrationSubtask `json:"subtasks"`
}

// orchestrationPlan describes the full execution plan.
type orchestrationPlan struct {
	ID          string                `json:"id"`
	Task        string                `json:"task"`
	Subtasks    []orchestrationSubtask `json:"subtasks"`
	Phases      []orchestrationPhase  `json:"phases"`
	TotalPhases int                   `json:"totalPhases"`
}

// orchestrationState tracks the runtime state of an orchestration.
type orchestrationState struct {
	ID           string                 `json:"id"`
	Task         string                 `json:"task"`
	Status       orchestrationStatus    `json:"status"`
	Plan         *orchestrationPlan     `json:"plan"`
	PhaseResults map[int][]phaseResult  `json:"phaseResults"`
	StartedAt    int64                  `json:"startedAt"`
	CompletedAt  int64                  `json:"completedAt,omitempty"`
	Error        string                 `json:"error,omitempty"`
	SpawnedAgents []string              `json:"spawnedAgents"`
}

// phaseResult captures the outcome of a subtask execution.
type phaseResult struct {
	SubtaskID string `json:"subtaskId"`
	AgentID   string `json:"agentId"`
	AgentType string `json:"agentType"`
	Success   bool   `json:"success"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	DurationMs int64 `json:"durationMs"`
}

// OrchestrateTools provides MCP tools for smart orchestration.
type OrchestrateTools struct {
	coordinator     *coordinator.SwarmCoordinator
	routingEngine   *hooks.RoutingEngine
	memoryBackend   shared.MemoryBackend
	samplingManager SamplingManagerInterface
	mu              sync.RWMutex
	orchestrations  map[string]*orchestrationState
}

// SamplingManagerInterface abstracts the sampling manager for testability.
type SamplingManagerInterface interface {
	CreateMessageWithContext(ctx context.Context, request *shared.CreateMessageRequest) (*shared.CreateMessageResult, error)
	IsAvailable() bool
}

// NewOrchestrateTools creates a new OrchestrateTools instance.
func NewOrchestrateTools(coord *coordinator.SwarmCoordinator, re *hooks.RoutingEngine, mem shared.MemoryBackend, sm SamplingManagerInterface) *OrchestrateTools {
	return &OrchestrateTools{
		coordinator:     coord,
		routingEngine:   re,
		memoryBackend:   mem,
		samplingManager: sm,
		orchestrations:  make(map[string]*orchestrationState),
	}
}

// GetTools returns available orchestration tools.
func (o *OrchestrateTools) GetTools() []shared.MCPTool {
	return []shared.MCPTool{
		{
			Name:        "orchestrate_plan",
			Description: "Analyze a high-level task and produce an execution plan with phases and agent assignments (dry-run, no execution)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
						"type":        "string",
						"description": "High-level task description to decompose and plan",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			Name:        "orchestrate_execute",
			Description: "Analyze a high-level task, spawn agents, and execute the full orchestration pipeline with parallel phases",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
						"type":        "string",
						"description": "High-level task description to decompose and execute",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			Name:        "orchestrate_status",
			Description: "Get the status of an orchestration by ID",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"orchestrationId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the orchestration to check",
					},
				},
				"required": []string{"orchestrationId"},
			},
		},
	}
}

// Execute executes an orchestration tool.
func (o *OrchestrateTools) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	switch toolName {
	case "orchestrate_plan":
		return o.plan(params)
	case "orchestrate_execute":
		return o.execute(ctx, params)
	case "orchestrate_status":
		return o.status(params)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// plan creates an execution plan without running it.
func (o *OrchestrateTools) plan(params map[string]interface{}) (*shared.MCPToolResult, error) {
	task, ok := params["task"].(string)
	if !ok || task == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: task is required",
		}, nil
	}

	plan := o.smartDecompose(context.Background(), task)

	data, _ := json.Marshal(plan)
	var result interface{}
	json.Unmarshal(data, &result)

	return &shared.MCPToolResult{
		Success: true,
		Data:    result,
	}, nil
}

// execute creates a plan and runs it to completion.
func (o *OrchestrateTools) execute(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	task, ok := params["task"].(string)
	if !ok || task == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: task is required",
		}, nil
	}

	plan := o.smartDecompose(ctx, task)

	state := &orchestrationState{
		ID:            plan.ID,
		Task:          task,
		Status:        orchestrationRunning,
		Plan:          plan,
		PhaseResults:  make(map[int][]phaseResult),
		StartedAt:     shared.Now(),
		SpawnedAgents: make([]string, 0),
	}

	o.mu.Lock()
	o.orchestrations[state.ID] = state
	o.evictOldOrchestrations()
	o.mu.Unlock()

	// Execute phases sequentially; subtasks within a phase run concurrently.
	for _, phase := range plan.Phases {
		// Check for context cancellation between phases.
		select {
		case <-ctx.Done():
			state.Status = orchestrationFailed
			state.Error = fmt.Sprintf("cancelled before phase %d: %v", phase.Phase, ctx.Err())
			state.CompletedAt = shared.Now()

			data, _ := json.Marshal(state)
			var result interface{}
			json.Unmarshal(data, &result)

			return &shared.MCPToolResult{
				Success: false,
				Error:   state.Error,
				Data:    result,
			}, nil
		default:
		}

		phaseResults, err := o.executePhase(ctx, state, phase)
		if err != nil {
			state.Status = orchestrationFailed
			state.Error = fmt.Sprintf("phase %d failed: %v", phase.Phase, err)
			state.CompletedAt = shared.Now()

			data, _ := json.Marshal(state)
			var result interface{}
			json.Unmarshal(data, &result)

			return &shared.MCPToolResult{
				Success: false,
				Error:   state.Error,
				Data:    result,
			}, nil
		}

		o.mu.Lock()
		state.PhaseResults[phase.Phase] = phaseResults
		o.mu.Unlock()

		// Store phase results in memory for subsequent phases.
		if o.memoryBackend != nil {
			o.storePhaseContext(state.ID, phase.Phase, phaseResults)
		}
	}

	// Cleanup: terminate spawned agents.
	o.cleanupAgents(state)

	state.Status = orchestrationCompleted
	state.CompletedAt = shared.Now()

	// Synthesize final result from all phase outputs.
	synthesized := o.synthesizeResults(ctx, state)

	data, _ := json.Marshal(state)
	var result interface{}
	json.Unmarshal(data, &result)

	// Attach synthesized output to result map.
	if resultMap, ok := result.(map[string]interface{}); ok {
		resultMap["synthesizedResult"] = synthesized
	}

	return &shared.MCPToolResult{
		Success: true,
		Data:    result,
	}, nil
}

// status returns the current state of an orchestration.
func (o *OrchestrateTools) status(params map[string]interface{}) (*shared.MCPToolResult, error) {
	id, ok := params["orchestrationId"].(string)
	if !ok || id == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: orchestrationId is required",
		}, nil
	}

	o.mu.RLock()
	state, exists := o.orchestrations[id]
	o.mu.RUnlock()

	if !exists {
		return &shared.MCPToolResult{
			Success: false,
			Error:   fmt.Sprintf("orchestration not found: %s", id),
		}, nil
	}

	data, _ := json.Marshal(state)
	var result interface{}
	json.Unmarshal(data, &result)

	return &shared.MCPToolResult{
		Success: true,
		Data:    result,
	}, nil
}

// decomposeTask analyzes a task description and produces an execution plan.
func (o *OrchestrateTools) decomposeTask(task string) *orchestrationPlan {
	lower := strings.ToLower(task)

	// Match categories by keywords.
	matched := make(map[string]subtaskCategory)
	for _, cat := range defaultCategories {
		for _, kw := range cat.Keywords {
			if containsKeyword(lower, kw) {
				matched[cat.Name] = cat
				break
			}
		}
	}

	// If no matches, use default pipeline.
	if len(matched) == 0 {
		for _, name := range defaultPipeline {
			for _, cat := range defaultCategories {
				if cat.Name == name {
					matched[cat.Name] = cat
					break
				}
			}
		}
	}

	// Ensure dependency categories are present.
	// E.g. if "test" is matched and depends on "implement", ensure "implement" is there.
	changed := true
	for changed {
		changed = false
		for _, cat := range matched {
			for _, dep := range cat.DependsOn {
				if _, exists := matched[dep]; !exists {
					for _, dc := range defaultCategories {
						if dc.Name == dep {
							matched[dep] = dc
							changed = true
							break
						}
					}
				}
			}
		}
	}

	// Sort matched categories by phase (then name) for deterministic ordering.
	sortedCats := make([]subtaskCategory, 0, len(matched))
	for _, cat := range matched {
		sortedCats = append(sortedCats, cat)
	}
	sort.Slice(sortedCats, func(i, j int) bool {
		if sortedCats[i].Phase != sortedCats[j].Phase {
			return sortedCats[i].Phase < sortedCats[j].Phase
		}
		return sortedCats[i].Name < sortedCats[j].Name
	})

	// Build subtasks, apply sub-decomposition, then refine agent types.
	subtasks := make([]orchestrationSubtask, 0, len(matched))
	for _, cat := range sortedCats {
		// Try sub-decomposition: split a monolithic category into parallel sub-tasks.
		subs := matchSubSpecs(lower, cat)
		if len(subs) >= 2 {
			for _, sub := range subs {
				agentType := cat.AgentType
				if sub.AgentType != "" {
					agentType = sub.AgentType
				}
				agentType = o.refineAgentType(fmt.Sprintf("[%s/%s] %s", cat.Name, sub.Name, task), agentType)

				subtasks = append(subtasks, orchestrationSubtask{
					ID:        fmt.Sprintf("%s-subtask-%s-%s", shared.GenerateID("orch"), cat.Name, sub.Name),
					Category:  cat.Name,
					AgentType: agentType,
					Phase:     cat.Phase,
					DependsOn: cat.DependsOn,
					Task:      fmt.Sprintf("[%s · %s] %s", cat.Name, sub.TaskLabel, task),
				})
			}
		} else {
			// No sub-decomposition — keep as single subtask.
			agentType := o.refineAgentType(fmt.Sprintf("[%s] %s", cat.Name, task), cat.AgentType)
			subtasks = append(subtasks, orchestrationSubtask{
				ID:        fmt.Sprintf("%s-subtask-%s", shared.GenerateID("orch"), cat.Name),
				Category:  cat.Name,
				AgentType: agentType,
				Phase:     cat.Phase,
				DependsOn: cat.DependsOn,
				Task:      fmt.Sprintf("[%s] %s", cat.Name, task),
			})
		}
	}

	// Compute phases (topological sort into parallel groups).
	phases := computePhases(subtasks)

	return &orchestrationPlan{
		ID:          shared.GenerateID("orch"),
		Task:        task,
		Subtasks:    subtasks,
		Phases:      phases,
		TotalPhases: len(phases),
	}
}

// computePhases groups subtasks into phases based on their Phase field.
// Subtasks with the same phase number run in parallel.
func computePhases(subtasks []orchestrationSubtask) []orchestrationPhase {
	phaseMap := make(map[int][]orchestrationSubtask)
	for _, st := range subtasks {
		phaseMap[st.Phase] = append(phaseMap[st.Phase], st)
	}

	// Collect and sort phase numbers.
	phaseNums := make([]int, 0, len(phaseMap))
	for p := range phaseMap {
		phaseNums = append(phaseNums, p)
	}
	sort.Ints(phaseNums)

	phases := make([]orchestrationPhase, 0, len(phaseNums))
	for _, p := range phaseNums {
		phases = append(phases, orchestrationPhase{
			Phase:    p,
			Subtasks: phaseMap[p],
		})
	}
	return phases
}

// containsWord checks if keyword appears in text at a word boundary.
// A keyword matches if the characters immediately before and after it (if any)
// are not letters or digits. This prevents "ddl" matching inside "middleware"
// and "ui" matching inside "build".
func containsWord(text, keyword string) bool {
	idx := 0
	for {
		pos := strings.Index(text[idx:], keyword)
		if pos < 0 {
			return false
		}
		pos += idx // absolute position

		// Check left boundary.
		if pos > 0 {
			c := text[pos-1]
			if isAlphaNum(c) {
				idx = pos + 1
				continue
			}
		}

		// Check right boundary.
		end := pos + len(keyword)
		if end < len(text) {
			c := text[end]
			if isAlphaNum(c) {
				idx = pos + 1
				continue
			}
		}

		return true
	}
}

func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// containsKeyword returns true if the keyword matches the text.
// Keywords ≤3 characters use word-boundary matching to avoid false positives
// (e.g. "ddl" inside "middleware", "ui" inside "build", "db" inside "debug").
// Longer keywords use substring matching so that "test" matches "tests",
// "model" matches "models", "architect" matches "architecture", etc.
func containsKeyword(text, keyword string) bool {
	if len(keyword) <= 3 {
		return containsWord(text, keyword)
	}
	return strings.Contains(text, keyword)
}

// matchSubSpecs returns the sub-specs that match the task description for a given category.
func matchSubSpecs(lowerTask string, cat subtaskCategory) []subSpec {
	specs, ok := subDecompositions[cat.Name]
	if !ok {
		return nil
	}

	var matched []subSpec
	for _, s := range specs {
		for _, kw := range s.Keywords {
			if containsKeyword(lowerTask, kw) {
				matched = append(matched, s)
				break
			}
		}
	}
	return matched
}

// refineAgentType uses the routing engine to potentially improve the agent type selection.
func (o *OrchestrateTools) refineAgentType(taskDesc string, defaultType string) string {
	if o.routingEngine == nil {
		return defaultType
	}
	routingResult := o.routingEngine.Route(taskDesc, nil, []string{defaultType}, nil)
	if routingResult != nil && routingResult.RecommendedAgent != "" {
		return routingResult.RecommendedAgent
	}
	return defaultType
}

// executePhase spawns agents and executes subtasks for a single phase.
func (o *OrchestrateTools) executePhase(ctx context.Context, state *orchestrationState, phase orchestrationPhase) ([]phaseResult, error) {
	results := make([]phaseResult, len(phase.Subtasks))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, subtask := range phase.Subtasks {
		wg.Add(1)
		go func(idx int, st orchestrationSubtask) {
			defer wg.Done()
			pr := o.executeSubtask(ctx, state, st)
			mu.Lock()
			results[idx] = pr
			mu.Unlock()
		}(i, subtask)
	}

	wg.Wait()

	// Check for failures.
	for _, r := range results {
		if !r.Success {
			return results, fmt.Errorf("subtask %s failed: %s", r.SubtaskID, r.Error)
		}
	}

	return results, nil
}

// executeSubtask spawns an agent, executes the subtask, and records the outcome.
func (o *OrchestrateTools) executeSubtask(ctx context.Context, state *orchestrationState, st orchestrationSubtask) phaseResult {
	start := time.Now()

	// Spawn agent for this subtask. Use st.ID (unique) to avoid collisions
	// when multiple subtasks share the same category (sub-decomposition).
	agentID := fmt.Sprintf("orch-%s-%s", state.ID, st.ID)
	config := shared.AgentConfig{
		ID:   agentID,
		Type: shared.AgentType(st.AgentType),
	}

	agent, err := o.coordinator.SpawnAgent(config)
	if err != nil {
		return phaseResult{
			SubtaskID:  st.ID,
			AgentType:  st.AgentType,
			Success:    false,
			Error:      fmt.Sprintf("failed to spawn agent: %v", err),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	agentData := agent.ToShared()

	o.mu.Lock()
	state.SpawnedAgents = append(state.SpawnedAgents, agentData.ID)
	o.mu.Unlock()

	// Gather phase context from prior phases.
	phaseCtx := o.gatherPhaseContext(state.ID, st.Phase)

	// Execute the task on the agent.
	taskObj := shared.Task{
		ID:          st.ID,
		Type:        shared.TaskTypeCode,
		Description: st.Task,
		Priority:    shared.PriorityHigh,
		Metadata: map[string]interface{}{
			"phaseContext": phaseCtx,
		},
	}

	taskResult, err := o.coordinator.ExecuteTask(ctx, agentData.ID, taskObj)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		// Record outcome for Q-learning.
		if o.routingEngine != nil {
			o.routingEngine.RecordOutcome(st.ID, false, durationMs)
		}
		return phaseResult{
			SubtaskID:  st.ID,
			AgentID:    agentData.ID,
			AgentType:  st.AgentType,
			Success:    false,
			Error:      fmt.Sprintf("execution failed: %v", err),
			DurationMs: durationMs,
		}
	}

	success := taskResult.Status == shared.TaskStatusCompleted

	// Record outcome for Q-learning.
	if o.routingEngine != nil {
		o.routingEngine.RecordOutcome(st.ID, success, durationMs)
	}

	output := ""
	if taskResult.Result != nil {
		if s, ok := taskResult.Result.(string); ok {
			output = s
		}
	}

	return phaseResult{
		SubtaskID:  st.ID,
		AgentID:    agentData.ID,
		AgentType:  st.AgentType,
		Success:    success,
		Output:     output,
		DurationMs: durationMs,
	}
}

// storePhaseContext stores the results of a phase in memory so subsequent phases can access them.
func (o *OrchestrateTools) storePhaseContext(orchID string, phase int, results []phaseResult) {
	content, _ := json.Marshal(results)
	mem := shared.Memory{
		ID:        fmt.Sprintf("orch-%s-phase-%d", orchID, phase),
		AgentID:   "orchestrator",
		Content:   string(content),
		Type:      shared.MemoryType("orchestration-context"),
		Timestamp: shared.Now(),
	}
	if _, err := o.memoryBackend.Store(mem); err != nil {
		log.Printf("orchestrate: failed to store phase %d context for %s: %v", phase, orchID, err)
	}
}

// evictOldOrchestrations removes the oldest completed orchestrations when the map exceeds maxOrchestrations.
// Must be called with o.mu held.
func (o *OrchestrateTools) evictOldOrchestrations() {
	if len(o.orchestrations) <= maxOrchestrations {
		return
	}

	// Find completed orchestrations, sorted by completion time.
	type entry struct {
		id          string
		completedAt int64
	}
	var completed []entry
	for id, s := range o.orchestrations {
		if s.Status == orchestrationCompleted || s.Status == orchestrationFailed {
			completed = append(completed, entry{id, s.CompletedAt})
		}
	}
	sort.Slice(completed, func(i, j int) bool {
		return completed[i].completedAt < completed[j].completedAt
	})

	// Evict oldest completed until we're under the limit.
	toRemove := len(o.orchestrations) - maxOrchestrations
	for i := 0; i < toRemove && i < len(completed); i++ {
		delete(o.orchestrations, completed[i].id)
	}
}

// cleanupAgents terminates all agents spawned during the orchestration.
func (o *OrchestrateTools) cleanupAgents(state *orchestrationState) {
	o.mu.RLock()
	agents := make([]string, len(state.SpawnedAgents))
	copy(agents, state.SpawnedAgents)
	o.mu.RUnlock()

	for _, agentID := range agents {
		o.coordinator.TerminateAgent(agentID)
	}
}

// smartDecompose uses LLM decomposition when available, falling back to keyword-based.
func (o *OrchestrateTools) smartDecompose(ctx context.Context, task string) *orchestrationPlan {
	if o.samplingManager != nil && o.samplingManager.IsAvailable() {
		plan, err := o.decomposeTaskLLM(ctx, task)
		if err == nil && plan != nil && len(plan.Subtasks) > 0 {
			return plan
		}
		log.Printf("orchestrate: LLM decomposition failed, falling back to keyword: %v", err)
	}
	return o.decomposeTask(task)
}

// decomposeTaskLLM uses an LLM to decompose a task into subtasks.
func (o *OrchestrateTools) decomposeTaskLLM(ctx context.Context, task string) (*orchestrationPlan, error) {
	prompt := fmt.Sprintf(`Decompose the following task into subtasks for a multi-agent system.

Task: %s

Respond with ONLY a JSON array of subtasks. Each subtask has:
- "category": one of "design", "implement", "test", "security", "review", "docs", "optimize", "deploy"
- "agentType": the type of agent (e.g. "architect", "coder", "tdd-tester", "reviewer", "security-auditor", "documentation-lead", "optimizer", "deployer")
- "phase": integer phase number (0-based, tasks in the same phase run in parallel)
- "dependsOn": array of category names this depends on
- "task": specific task description for the agent

Example response:
[
  {"category":"design","agentType":"architect","phase":0,"dependsOn":[],"task":"Design the API structure"},
  {"category":"implement","agentType":"coder","phase":1,"dependsOn":["design"],"task":"Implement the endpoints"}
]`, task)

	result, err := o.samplingManager.CreateMessageWithContext(ctx, &shared.CreateMessageRequest{
		SystemPrompt: "You are a task decomposition engine. Output valid JSON only, no markdown fences.",
		Messages: []shared.SamplingMessage{
			{
				Role: "user",
				Content: []shared.PromptContent{
					{Type: "text", Text: prompt},
				},
			},
		},
		ModelPreferences: &shared.ModelPreferences{
			Hints: []shared.ModelHint{{Name: "haiku"}},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse JSON from response (strip markdown fences if present).
	content := strings.TrimSpace(result.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var rawSubtasks []orchestrationSubtask
	if err := json.Unmarshal([]byte(content), &rawSubtasks); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if len(rawSubtasks) == 0 {
		return nil, fmt.Errorf("LLM returned empty subtask list")
	}

	// Assign IDs and refine agent types.
	orchID := shared.GenerateID("orch")
	for i := range rawSubtasks {
		rawSubtasks[i].ID = fmt.Sprintf("%s-subtask-%s-%d", orchID, rawSubtasks[i].Category, i)
		rawSubtasks[i].AgentType = o.refineAgentType(rawSubtasks[i].Task, rawSubtasks[i].AgentType)
	}

	phases := computePhases(rawSubtasks)

	return &orchestrationPlan{
		ID:          orchID,
		Task:        task,
		Subtasks:    rawSubtasks,
		Phases:      phases,
		TotalPhases: len(phases),
	}, nil
}

// gatherPhaseContext reads memory for completed phases 0..currentPhase-1.
func (o *OrchestrateTools) gatherPhaseContext(orchID string, currentPhase int) string {
	if o.memoryBackend == nil || currentPhase == 0 {
		return ""
	}

	var parts []string
	for p := 0; p < currentPhase; p++ {
		mem, err := o.memoryBackend.Retrieve(fmt.Sprintf("orch-%s-phase-%d", orchID, p))
		if err != nil || mem == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("Phase %d results: %s", p, mem.Content))
	}

	return strings.Join(parts, "\n\n")
}

// synthesizeResults aggregates all phase outputs into a coherent final result.
func (o *OrchestrateTools) synthesizeResults(ctx context.Context, state *orchestrationState) string {
	// Collect all outputs.
	var outputs []string
	for phase := 0; phase < len(state.Plan.Phases); phase++ {
		results, ok := state.PhaseResults[state.Plan.Phases[phase].Phase]
		if !ok {
			continue
		}
		for _, r := range results {
			if r.Output != "" {
				outputs = append(outputs, fmt.Sprintf("[%s/%s] %s", r.AgentType, r.SubtaskID, r.Output))
			}
		}
	}

	if len(outputs) == 0 {
		return "All phases completed successfully."
	}

	concatenated := strings.Join(outputs, "\n\n")

	// Try LLM synthesis if available.
	if o.samplingManager != nil && o.samplingManager.IsAvailable() {
		prompt := fmt.Sprintf("The following outputs were produced by specialized agents working on the task: %q\n\nAgent outputs:\n%s\n\nSynthesize these into a coherent, unified final result.", state.Task, concatenated)

		result, err := o.samplingManager.CreateMessageWithContext(ctx, &shared.CreateMessageRequest{
			SystemPrompt: "You are a synthesis agent. Combine the provided agent outputs into a coherent final deliverable. Be concise but comprehensive.",
			Messages: []shared.SamplingMessage{
				{
					Role: "user",
					Content: []shared.PromptContent{
						{Type: "text", Text: prompt},
					},
				},
			},
			ModelPreferences: &shared.ModelPreferences{
				Hints: []shared.ModelHint{{Name: "sonnet"}},
			},
		})
		if err == nil {
			return result.Content
		}
		log.Printf("orchestrate: synthesis LLM call failed, using concatenation: %v", err)
	}

	return concatenated
}
