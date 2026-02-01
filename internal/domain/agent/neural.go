// Package agent provides the Agent domain entity.
package agent

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// NeuralPatternConfig holds neural pattern configuration for an agent type.
type NeuralPatternConfig struct {
	Patterns          []string `json:"patterns"`
	ActivationThresh  float64  `json:"activationThreshold"`
	LearningRate      float64  `json:"learningRate"`
	SignificanceThresh float64 `json:"significanceThreshold"`
	MaxPatterns       int      `json:"maxPatterns"`
}

// DefaultNeuralPatternConfig returns the default neural pattern configuration.
func DefaultNeuralPatternConfig() NeuralPatternConfig {
	return NeuralPatternConfig{
		Patterns:          []string{},
		ActivationThresh:  0.7,
		LearningRate:      0.01,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	}
}

// neuralPatternConfigs maps agent types to their neural pattern configurations.
var neuralPatternConfigs = map[shared.AgentType]NeuralPatternConfig{
	// Queen - hierarchical coordination patterns
	shared.AgentTypeQueen: {
		Patterns:          []string{"hierarchical-attention", "consensus-building", "delegation-optimization"},
		ActivationThresh:  0.6,
		LearningRate:      0.02,
		SignificanceThresh: 0.7,
		MaxPatterns:       15,
	},

	// Coder - code generation patterns
	shared.AgentTypeCoder: {
		Patterns:          []string{"code-generation", "refactoring-patterns", "debugging-heuristics"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       12,
	},

	// Security Domain
	shared.AgentTypeSecurityArchitect: {
		Patterns:          []string{"threat-detection", "vulnerability-analysis", "security-patterns"},
		ActivationThresh:  0.8,
		LearningRate:      0.01,
		SignificanceThresh: 0.8,
		MaxPatterns:       10,
	},
	shared.AgentTypeCVERemediation: {
		Patterns:          []string{"pattern-matching", "fix-generation", "patch-optimization"},
		ActivationThresh:  0.75,
		LearningRate:      0.015,
		SignificanceThresh: 0.75,
		MaxPatterns:       10,
	},
	shared.AgentTypeThreatModeler: {
		Patterns:          []string{"risk-prediction", "threat-classification", "attack-path-analysis"},
		ActivationThresh:  0.8,
		LearningRate:      0.01,
		SignificanceThresh: 0.8,
		MaxPatterns:       10,
	},
	shared.AgentTypeSecurityAuditor: {
		Patterns:          []string{"vulnerability-detection", "compliance-checking", "audit-patterns"},
		ActivationThresh:  0.8,
		LearningRate:      0.01,
		SignificanceThresh: 0.85,
		MaxPatterns:       10,
	},

	// Core Domain
	shared.AgentTypeDDDDesigner: {
		Patterns:          []string{"domain-extraction", "context-mapping", "aggregate-design"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       12,
	},
	shared.AgentTypeMemorySpecialist: {
		Patterns:          []string{"memory-optimization", "retrieval-patterns", "caching-strategies"},
		ActivationThresh:  0.7,
		LearningRate:      0.02,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeTypeModernizer: {
		Patterns:          []string{"type-inference", "migration-patterns", "type-safety"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeSwarmSpecialist: {
		Patterns:          []string{"swarm-optimization", "topology-adaptation", "consensus-patterns"},
		ActivationThresh:  0.65,
		LearningRate:      0.02,
		SignificanceThresh: 0.7,
		MaxPatterns:       12,
	},
	shared.AgentTypeMCPOptimizer: {
		Patterns:          []string{"protocol-optimization", "tool-routing", "resource-management"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeMCPSpecialist: {
		Patterns:          []string{"protocol-optimization", "tool-generation", "server-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeCoreArchitect: {
		Patterns:          []string{"domain-extraction", "architecture-optimization", "clean-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.75,
		MaxPatterns:       12,
	},

	// Integration Domain
	shared.AgentTypeAgenticFlow: {
		Patterns:          []string{"flow-optimization", "step-prediction", "workflow-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeCLIDeveloper: {
		Patterns:          []string{"command-generation", "ux-optimization", "cli-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeNeuralIntegrator: {
		Patterns:          []string{"pattern-learning", "integration-optimization", "neural-adaptation"},
		ActivationThresh:  0.6,
		LearningRate:      0.025,
		SignificanceThresh: 0.65,
		MaxPatterns:       15,
	},
	shared.AgentTypeHooksDeveloper: {
		Patterns:          []string{"hook-optimization", "learning-integration", "event-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.02,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeIntegrationArchitect: {
		Patterns:          []string{"integration-patterns", "api-optimization", "protocol-design"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.75,
		MaxPatterns:       12,
	},

	// Support Domain
	shared.AgentTypeTDDTester: {
		Patterns:          []string{"test-generation", "coverage-optimization", "test-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypePerformanceEngineer: {
		Patterns:          []string{"bottleneck-detection", "optimization-patterns", "profiling-heuristics"},
		ActivationThresh:  0.7,
		LearningRate:      0.02,
		SignificanceThresh: 0.7,
		MaxPatterns:       12,
	},
	shared.AgentTypeReleaseManager: {
		Patterns:          []string{"release-planning", "risk-assessment", "deployment-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.01,
		SignificanceThresh: 0.75,
		MaxPatterns:       10,
	},
	shared.AgentTypeTestArchitect: {
		Patterns:          []string{"test-strategy-generation", "coverage-optimization", "tdd-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.75,
		MaxPatterns:       12,
	},
	shared.AgentTypeDevOpsEngineer: {
		Patterns:          []string{"pipeline-optimization", "infrastructure-patterns", "deployment-automation"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},

	// Extended Types
	shared.AgentTypeResearcher: {
		Patterns:          []string{"information-extraction", "relevance-scoring", "summarization-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeArchitect: {
		Patterns:          []string{"architecture-patterns", "system-modeling", "scalability-analysis"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.75,
		MaxPatterns:       12,
	},
	shared.AgentTypeAnalyst: {
		Patterns:          []string{"trend-detection", "anomaly-detection", "metric-analysis"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeOptimizer: {
		Patterns:          []string{"optimization-search", "resource-allocation", "efficiency-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.02,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeDocumentationLead: {
		Patterns:          []string{"content-generation", "clarity-optimization", "documentation-patterns"},
		ActivationThresh:  0.7,
		LearningRate:      0.01,
		SignificanceThresh: 0.7,
		MaxPatterns:       8,
	},

	// Basic Types
	shared.AgentTypeTester: {
		Patterns:          []string{"test-generation", "validation-patterns", "coverage-analysis"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeReviewer: {
		Patterns:          []string{"review-patterns", "quality-detection", "security-analysis"},
		ActivationThresh:  0.75,
		LearningRate:      0.01,
		SignificanceThresh: 0.75,
		MaxPatterns:       10,
	},
	shared.AgentTypeCoordinator: {
		Patterns:          []string{"orchestration-patterns", "delegation-optimization", "consensus-building"},
		ActivationThresh:  0.65,
		LearningRate:      0.02,
		SignificanceThresh: 0.7,
		MaxPatterns:       12,
	},
	shared.AgentTypeDesigner: {
		Patterns:          []string{"design-patterns", "prototyping-heuristics", "architecture-modeling"},
		ActivationThresh:  0.7,
		LearningRate:      0.015,
		SignificanceThresh: 0.7,
		MaxPatterns:       10,
	},
	shared.AgentTypeDeployer: {
		Patterns:          []string{"deployment-patterns", "rollback-strategies", "monitoring-integration"},
		ActivationThresh:  0.7,
		LearningRate:      0.01,
		SignificanceThresh: 0.75,
		MaxPatterns:       10,
	},
}

// GetNeuralPatternsForType returns the neural pattern configuration for an agent type.
func GetNeuralPatternsForType(t shared.AgentType) *NeuralPatternConfig {
	config, exists := neuralPatternConfigs[t]
	if !exists {
		defaultConfig := DefaultNeuralPatternConfig()
		return &defaultConfig
	}
	return &config
}

// GetAllNeuralPatterns returns all neural pattern configurations.
func GetAllNeuralPatterns() map[shared.AgentType]NeuralPatternConfig {
	result := make(map[shared.AgentType]NeuralPatternConfig)
	for k, v := range neuralPatternConfigs {
		result[k] = v
	}
	return result
}

// GetPatternNames returns just the pattern names for an agent type.
func GetPatternNames(t shared.AgentType) []string {
	config := GetNeuralPatternsForType(t)
	result := make([]string, len(config.Patterns))
	copy(result, config.Patterns)
	return result
}

// IsPatternSignificant checks if a pattern score meets the significance threshold.
func IsPatternSignificant(t shared.AgentType, score float64) bool {
	config := GetNeuralPatternsForType(t)
	return score >= config.SignificanceThresh
}

// IsPatternActivated checks if a pattern score meets the activation threshold.
func IsPatternActivated(t shared.AgentType, score float64) bool {
	config := GetNeuralPatternsForType(t)
	return score >= config.ActivationThresh
}
