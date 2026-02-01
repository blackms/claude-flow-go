// Package agent provides the Agent domain entity.
package agent

import (
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ModelTier represents the model tier for an agent type.
type ModelTier string

const (
	ModelTierOpus   ModelTier = "opus"
	ModelTierSonnet ModelTier = "sonnet"
	ModelTierHaiku  ModelTier = "haiku"
)

// AgentTypeSpec defines the specification for an agent type.
type AgentTypeSpec struct {
	Type           shared.AgentType `json:"type"`
	Capabilities   []string         `json:"capabilities"`
	Description    string           `json:"description"`
	ModelTier      ModelTier        `json:"modelTier"`
	NeuralPatterns []string         `json:"neuralPatterns,omitempty"`
	Tags           []string         `json:"tags,omitempty"`
	Domain         shared.AgentDomain `json:"domain,omitempty"`
}

// AgentTypeRegistry manages all agent type specifications.
type AgentTypeRegistry struct {
	mu    sync.RWMutex
	specs map[shared.AgentType]*AgentTypeSpec

	// Indexes for O(1) lookups
	byCapability map[string][]shared.AgentType
	byTag        map[string][]shared.AgentType
	byModelTier  map[ModelTier][]shared.AgentType
	byDomain     map[shared.AgentDomain][]shared.AgentType
}

// NewAgentTypeRegistry creates a new AgentTypeRegistry with all default specs.
func NewAgentTypeRegistry() *AgentTypeRegistry {
	r := &AgentTypeRegistry{
		specs:        make(map[shared.AgentType]*AgentTypeSpec),
		byCapability: make(map[string][]shared.AgentType),
		byTag:        make(map[string][]shared.AgentType),
		byModelTier:  make(map[ModelTier][]shared.AgentType),
		byDomain:     make(map[shared.AgentDomain][]shared.AgentType),
	}

	// Register all default specs
	r.registerDefaults()

	return r
}

// registerDefaults registers all default agent type specifications.
func (r *AgentTypeRegistry) registerDefaults() {
	specs := []AgentTypeSpec{
		// Basic agent types
		{
			Type:         shared.AgentTypeCoder,
			Capabilities: []string{"code-generation", "refactoring", "debugging", "testing"},
			Description:  "Code development with neural patterns",
			ModelTier:    ModelTierSonnet,
			Tags:         []string{"development", "core"},
		},
		{
			Type:         shared.AgentTypeTester,
			Capabilities: []string{"unit-testing", "integration-testing", "coverage-analysis", "automation"},
			Description:  "Comprehensive testing",
			ModelTier:    ModelTierSonnet,
			Tags:         []string{"testing", "quality"},
		},
		{
			Type:         shared.AgentTypeReviewer,
			Capabilities: []string{"code-review", "security-audit", "quality-check", "documentation"},
			Description:  "Code review with security checks",
			ModelTier:    ModelTierSonnet,
			Tags:         []string{"review", "quality"},
		},
		{
			Type:         shared.AgentTypeCoordinator,
			Capabilities: []string{"orchestration", "delegation", "monitoring", "consensus"},
			Description:  "Multi-agent orchestration",
			ModelTier:    ModelTierOpus,
			Tags:         []string{"coordination", "management"},
		},
		{
			Type:         shared.AgentTypeDesigner,
			Capabilities: []string{"system-design", "architecture", "prototyping"},
			Description:  "System design",
			ModelTier:    ModelTierOpus,
			Tags:         []string{"design", "architecture"},
		},
		{
			Type:         shared.AgentTypeDeployer,
			Capabilities: []string{"deployment", "release", "rollback", "monitoring"},
			Description:  "Deployment and release management",
			ModelTier:    ModelTierSonnet,
			Tags:         []string{"deployment", "operations"},
		},

		// Queen Domain
		{
			Type:           shared.AgentTypeQueen,
			Capabilities:   []string{"coordination", "planning", "oversight", "consensus", "delegation"},
			Description:    "Top-level swarm coordination and orchestration",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"hierarchical-attention", "consensus-building"},
			Tags:           []string{"queen", "coordination"},
			Domain:         shared.DomainQueen,
		},

		// Security Domain
		{
			Type:           shared.AgentTypeSecurityArchitect,
			Capabilities:   []string{"security-architecture", "threat-modeling", "security-design"},
			Description:    "Security architecture",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"threat-detection", "vulnerability-analysis"},
			Tags:           []string{"security", "architecture"},
			Domain:         shared.DomainSecurity,
		},
		{
			Type:           shared.AgentTypeCVERemediation,
			Capabilities:   []string{"cve-remediation", "vulnerability-fix", "security-patch"},
			Description:    "CVE remediation",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"pattern-matching", "fix-generation"},
			Tags:           []string{"security", "remediation"},
			Domain:         shared.DomainSecurity,
		},
		{
			Type:           shared.AgentTypeThreatModeler,
			Capabilities:   []string{"threat-modeling", "risk-assessment", "security-analysis"},
			Description:    "Threat modeling and risk assessment",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"risk-prediction", "threat-classification"},
			Tags:           []string{"security", "analysis"},
			Domain:         shared.DomainSecurity,
		},

		// Core Domain
		{
			Type:           shared.AgentTypeDDDDesigner,
			Capabilities:   []string{"ddd-design", "domain-modeling", "architecture", "bounded-contexts"},
			Description:    "Domain-driven design",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"domain-extraction", "context-mapping"},
			Tags:           []string{"architecture", "ddd"},
			Domain:         shared.DomainCore,
		},
		{
			Type:           shared.AgentTypeMemorySpecialist,
			Capabilities:   []string{"memory-unification", "vector-search", "agentdb", "caching"},
			Description:    "AgentDB unification (150x-12,500x faster)",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"memory-optimization", "retrieval-patterns"},
			Tags:           []string{"memory", "performance"},
			Domain:         shared.DomainCore,
		},
		{
			Type:           shared.AgentTypeTypeModernizer,
			Capabilities:   []string{"type-modernization", "refactor", "code-quality"},
			Description:    "Type system modernization",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"type-inference", "migration-patterns"},
			Tags:           []string{"refactoring", "types"},
			Domain:         shared.DomainCore,
		},
		{
			Type:           shared.AgentTypeSwarmSpecialist,
			Capabilities:   []string{"swarm-coordination", "topology", "consensus", "federation"},
			Description:    "Unified coordination engine",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"swarm-optimization", "topology-adaptation"},
			Tags:           []string{"swarm", "coordination"},
			Domain:         shared.DomainCore,
		},
		{
			Type:           shared.AgentTypeMCPOptimizer,
			Capabilities:   []string{"mcp-optimization", "protocol", "integration", "tool-management"},
			Description:    "MCP protocol optimization",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"protocol-optimization", "tool-routing"},
			Tags:           []string{"mcp", "protocol"},
			Domain:         shared.DomainCore,
		},

		// Integration Domain
		{
			Type:           shared.AgentTypeAgenticFlow,
			Capabilities:   []string{"agentic-flow-integration", "workflow", "automation"},
			Description:    "Agentic flow integration",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"flow-optimization", "step-prediction"},
			Tags:           []string{"integration", "workflow"},
			Domain:         shared.DomainIntegration,
		},
		{
			Type:           shared.AgentTypeCLIDeveloper,
			Capabilities:   []string{"cli-modernization", "command-line", "user-interface"},
			Description:    "CLI development",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"command-generation", "ux-optimization"},
			Tags:           []string{"cli", "development"},
			Domain:         shared.DomainIntegration,
		},
		{
			Type:           shared.AgentTypeNeuralIntegrator,
			Capabilities:   []string{"neural-integration", "learning", "patterns", "hooks"},
			Description:    "Neural pattern integration",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"pattern-learning", "integration-optimization"},
			Tags:           []string{"neural", "integration"},
			Domain:         shared.DomainIntegration,
		},

		// Support Domain
		{
			Type:           shared.AgentTypeTDDTester,
			Capabilities:   []string{"tdd-testing", "unit-test", "test-driven", "london-school"},
			Description:    "TDD London School methodology",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"test-generation", "coverage-optimization"},
			Tags:           []string{"testing", "tdd"},
			Domain:         shared.DomainSupport,
		},
		{
			Type:           shared.AgentTypePerformanceEngineer,
			Capabilities:   []string{"performance-benchmarking", "optimization", "profiling"},
			Description:    "2.49x-7.47x optimization targets",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"bottleneck-detection", "optimization-patterns"},
			Tags:           []string{"performance", "optimization"},
			Domain:         shared.DomainSupport,
		},
		{
			Type:           shared.AgentTypeReleaseManager,
			Capabilities:   []string{"deployment", "release-management", "ci-cd", "versioning"},
			Description:    "Release management and CI/CD",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"release-planning", "risk-assessment"},
			Tags:           []string{"release", "deployment"},
			Domain:         shared.DomainSupport,
		},

		// Extended Agent Types
		{
			Type:           shared.AgentTypeResearcher,
			Capabilities:   []string{"web-search", "data-analysis", "summarization", "citation"},
			Description:    "Research with web access",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"information-extraction", "relevance-scoring"},
			Tags:           []string{"research", "analysis"},
		},
		{
			Type:           shared.AgentTypeArchitect,
			Capabilities:   []string{"system-design", "pattern-analysis", "scalability", "documentation"},
			Description:    "System design",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"architecture-patterns", "system-modeling"},
			Tags:           []string{"architecture", "design"},
		},
		{
			Type:           shared.AgentTypeAnalyst,
			Capabilities:   []string{"performance-analysis", "metrics", "reporting", "insights"},
			Description:    "Performance analysis",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"trend-detection", "anomaly-detection"},
			Tags:           []string{"analysis", "metrics"},
		},
		{
			Type:           shared.AgentTypeOptimizer,
			Capabilities:   []string{"performance-optimization", "resource-management", "efficiency"},
			Description:    "Performance optimization",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"optimization-search", "resource-allocation"},
			Tags:           []string{"optimization", "performance"},
		},
		{
			Type:           shared.AgentTypeSecurityAuditor,
			Capabilities:   []string{"security-audit", "vulnerability-scan", "compliance", "cve-detection"},
			Description:    "CVE remediation and security auditing",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"vulnerability-detection", "compliance-checking"},
			Tags:           []string{"security", "audit"},
			Domain:         shared.DomainSecurity,
		},
		{
			Type:           shared.AgentTypeCoreArchitect,
			Capabilities:   []string{"ddd-design", "core-architecture", "domain-modeling", "clean-architecture"},
			Description:    "Domain-driven design",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"domain-extraction", "architecture-optimization"},
			Tags:           []string{"architecture", "ddd"},
			Domain:         shared.DomainCore,
		},
		{
			Type:           shared.AgentTypeTestArchitect,
			Capabilities:   []string{"test-architecture", "tdd-london", "test-strategy", "coverage-planning"},
			Description:    "TDD London School methodology",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"test-strategy-generation", "coverage-optimization"},
			Tags:           []string{"testing", "architecture"},
			Domain:         shared.DomainSupport,
		},
		{
			Type:           shared.AgentTypeIntegrationArchitect,
			Capabilities:   []string{"external-integration", "api-design", "protocol-design", "interoperability"},
			Description:    "External integration",
			ModelTier:      ModelTierOpus,
			NeuralPatterns: []string{"integration-patterns", "api-optimization"},
			Tags:           []string{"integration", "architecture"},
			Domain:         shared.DomainIntegration,
		},
		{
			Type:           shared.AgentTypeHooksDeveloper,
			Capabilities:   []string{"hooks-development", "self-learning", "event-handling", "extensibility"},
			Description:    "Self-learning hooks",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"hook-optimization", "learning-integration"},
			Tags:           []string{"hooks", "development"},
			Domain:         shared.DomainIntegration,
		},
		{
			Type:           shared.AgentTypeMCPSpecialist,
			Capabilities:   []string{"mcp-protocol", "tool-development", "server-management", "resource-handling"},
			Description:    "MCP protocol specialist",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"protocol-optimization", "tool-generation"},
			Tags:           []string{"mcp", "protocol"},
			Domain:         shared.DomainCore,
		},
		{
			Type:           shared.AgentTypeDocumentationLead,
			Capabilities:   []string{"documentation", "technical-writing", "api-docs", "tutorials"},
			Description:    "Documentation",
			ModelTier:      ModelTierHaiku,
			NeuralPatterns: []string{"content-generation", "clarity-optimization"},
			Tags:           []string{"documentation", "writing"},
		},
		{
			Type:           shared.AgentTypeDevOpsEngineer,
			Capabilities:   []string{"ci-cd", "infrastructure", "deployment", "monitoring", "containerization"},
			Description:    "Deployment & CI/CD",
			ModelTier:      ModelTierSonnet,
			NeuralPatterns: []string{"pipeline-optimization", "infrastructure-patterns"},
			Tags:           []string{"devops", "infrastructure"},
			Domain:         shared.DomainSupport,
		},
	}

	for i := range specs {
		r.Register(&specs[i])
	}
}

// Register registers an agent type specification.
func (r *AgentTypeRegistry) Register(spec *AgentTypeSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.specs[spec.Type] = spec

	// Index by capability
	for _, cap := range spec.Capabilities {
		r.byCapability[cap] = append(r.byCapability[cap], spec.Type)
	}

	// Index by tag
	for _, tag := range spec.Tags {
		r.byTag[tag] = append(r.byTag[tag], spec.Type)
	}

	// Index by model tier
	r.byModelTier[spec.ModelTier] = append(r.byModelTier[spec.ModelTier], spec.Type)

	// Index by domain
	if spec.Domain != "" {
		r.byDomain[spec.Domain] = append(r.byDomain[spec.Domain], spec.Type)
	}
}

// GetSpec returns the specification for an agent type.
func (r *AgentTypeRegistry) GetSpec(t shared.AgentType) *AgentTypeSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.specs[t]
}

// ListAll returns all registered agent types.
func (r *AgentTypeRegistry) ListAll() []shared.AgentType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]shared.AgentType, 0, len(r.specs))
	for t := range r.specs {
		types = append(types, t)
	}
	return types
}

// ListByCapability returns agent types that have the specified capability.
func (r *AgentTypeRegistry) ListByCapability(capability string) []shared.AgentType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := r.byCapability[capability]
	if types == nil {
		return []shared.AgentType{}
	}

	// Return a copy
	result := make([]shared.AgentType, len(types))
	copy(result, types)
	return result
}

// ListByTag returns agent types that have the specified tag.
func (r *AgentTypeRegistry) ListByTag(tag string) []shared.AgentType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := r.byTag[tag]
	if types == nil {
		return []shared.AgentType{}
	}

	result := make([]shared.AgentType, len(types))
	copy(result, types)
	return result
}

// ListByModelTier returns agent types for the specified model tier.
func (r *AgentTypeRegistry) ListByModelTier(tier ModelTier) []shared.AgentType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := r.byModelTier[tier]
	if types == nil {
		return []shared.AgentType{}
	}

	result := make([]shared.AgentType, len(types))
	copy(result, types)
	return result
}

// ListByDomain returns agent types in the specified domain.
func (r *AgentTypeRegistry) ListByDomain(domain shared.AgentDomain) []shared.AgentType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := r.byDomain[domain]
	if types == nil {
		return []shared.AgentType{}
	}

	result := make([]shared.AgentType, len(types))
	copy(result, types)
	return result
}

// GetCapabilities returns the capabilities for an agent type.
func (r *AgentTypeRegistry) GetCapabilities(t shared.AgentType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec := r.specs[t]
	if spec == nil {
		return []string{}
	}

	result := make([]string, len(spec.Capabilities))
	copy(result, spec.Capabilities)
	return result
}

// GetModelTier returns the model tier for an agent type.
func (r *AgentTypeRegistry) GetModelTier(t shared.AgentType) ModelTier {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec := r.specs[t]
	if spec == nil {
		return ModelTierSonnet // Default
	}
	return spec.ModelTier
}

// GetNeuralPatterns returns the neural patterns for an agent type.
func (r *AgentTypeRegistry) GetNeuralPatterns(t shared.AgentType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec := r.specs[t]
	if spec == nil {
		return []string{}
	}

	result := make([]string, len(spec.NeuralPatterns))
	copy(result, spec.NeuralPatterns)
	return result
}

// FindBestMatch finds the best agent type for the given capabilities.
func (r *AgentTypeRegistry) FindBestMatch(requiredCapabilities []string) (shared.AgentType, float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var bestType shared.AgentType
	bestScore := 0.0

	for agentType, spec := range r.specs {
		score := r.calculateCapabilityScore(spec.Capabilities, requiredCapabilities)
		if score > bestScore {
			bestScore = score
			bestType = agentType
		}
	}

	return bestType, bestScore
}

// calculateCapabilityScore calculates how well capabilities match requirements.
func (r *AgentTypeRegistry) calculateCapabilityScore(capabilities, requirements []string) float64 {
	if len(requirements) == 0 {
		return 0
	}

	capSet := make(map[string]bool)
	for _, cap := range capabilities {
		capSet[cap] = true
	}

	matches := 0
	for _, req := range requirements {
		if capSet[req] {
			matches++
		}
	}

	return float64(matches) / float64(len(requirements))
}

// Count returns the total number of registered agent types.
func (r *AgentTypeRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.specs)
}

// HasType checks if an agent type is registered.
func (r *AgentTypeRegistry) HasType(t shared.AgentType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.specs[t]
	return exists
}

// GetAllSpecs returns all registered specifications.
func (r *AgentTypeRegistry) GetAllSpecs() []*AgentTypeSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs := make([]*AgentTypeSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		specs = append(specs, spec)
	}
	return specs
}
