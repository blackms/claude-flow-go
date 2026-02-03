// Package worker provides worker infrastructure.
package worker

import (
	"regexp"
	"strings"
	"time"

	domainWorker "github.com/anthropics/claude-flow-go/internal/domain/worker"
)

// TriggerRegistry contains all trigger definitions.
type TriggerRegistry struct {
	triggers map[domainWorker.WorkerTrigger]domainWorker.TriggerConfig
}

// NewTriggerRegistry creates a new trigger registry with all triggers.
func NewTriggerRegistry() *TriggerRegistry {
	registry := &TriggerRegistry{
		triggers: make(map[domainWorker.WorkerTrigger]domainWorker.TriggerConfig),
	}

	registry.initTriggers()
	return registry
}

func (r *TriggerRegistry) initTriggers() {
	r.triggers[domainWorker.TriggerUltralearn] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerUltralearn,
		Description:       "Deep learning from trajectories - analyzes execution patterns",
		Priority:          domainWorker.PriorityHigh,
		EstimatedDuration: 30 * time.Second,
		Capabilities:      []string{"neural", "trajectory", "learning"},
		Keywords:          []string{"ultralearn", "deep learn", "trajectory learning", "pattern learning"},
	}

	r.triggers[domainWorker.TriggerOptimize] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerOptimize,
		Description:       "Pattern optimization - improves stored patterns",
		Priority:          domainWorker.PriorityNormal,
		EstimatedDuration: 15 * time.Second,
		Capabilities:      []string{"neural", "pattern", "optimization"},
		Keywords:          []string{"optimize", "optimization", "improve patterns", "tune"},
	}

	r.triggers[domainWorker.TriggerConsolidate] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerConsolidate,
		Description:       "Memory consolidation - merges and cleans memory",
		Priority:          domainWorker.PriorityNormal,
		EstimatedDuration: 20 * time.Second,
		Capabilities:      []string{"memory", "consolidation", "cleanup"},
		Keywords:          []string{"consolidate", "consolidation", "merge memory", "clean memory"},
	}

	r.triggers[domainWorker.TriggerPredict] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerPredict,
		Description:       "Predictive analysis - predicts outcomes and behaviors",
		Priority:          domainWorker.PriorityNormal,
		EstimatedDuration: 10 * time.Second,
		Capabilities:      []string{"prediction", "analysis", "inference"},
		Keywords:          []string{"predict", "prediction", "forecast", "anticipate"},
	}

	r.triggers[domainWorker.TriggerAudit] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerAudit,
		Description:       "Security and quality audit - comprehensive codebase review",
		Priority:          domainWorker.PriorityHigh,
		EstimatedDuration: 25 * time.Second,
		Capabilities:      []string{"security", "audit", "review", "quality"},
		Keywords:          []string{"audit", "security audit", "code review", "quality check"},
	}

	r.triggers[domainWorker.TriggerMap] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerMap,
		Description:       "Codebase mapping - creates structure and dependency maps",
		Priority:          domainWorker.PriorityNormal,
		EstimatedDuration: 45 * time.Second,
		Capabilities:      []string{"mapping", "analysis", "structure"},
		Keywords:          []string{"map", "codebase map", "dependency map", "structure analysis"},
	}

	r.triggers[domainWorker.TriggerPreload] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerPreload,
		Description:       "Preload embeddings - generates and caches embeddings",
		Priority:          domainWorker.PriorityLow,
		EstimatedDuration: 60 * time.Second,
		Capabilities:      []string{"embeddings", "caching", "preload"},
		Keywords:          []string{"preload", "preload embeddings", "cache embeddings", "warm cache"},
	}

	r.triggers[domainWorker.TriggerDeepdive] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerDeepdive,
		Description:       "Deep code analysis - thorough investigation of code",
		Priority:          domainWorker.PriorityNormal,
		EstimatedDuration: 90 * time.Second,
		Capabilities:      []string{"analysis", "deep", "investigation"},
		Keywords:          []string{"deepdive", "deep dive", "deep analysis", "thorough analysis"},
	}

	r.triggers[domainWorker.TriggerDocument] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerDocument,
		Description:       "Auto-documentation - generates documentation from code",
		Priority:          domainWorker.PriorityNormal,
		EstimatedDuration: 30 * time.Second,
		Capabilities:      []string{"documentation", "generation", "writing"},
		Keywords:          []string{"document", "documentation", "auto-doc", "generate docs"},
	}

	r.triggers[domainWorker.TriggerRefactor] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerRefactor,
		Description:       "Refactoring suggestions - identifies improvement opportunities",
		Priority:          domainWorker.PriorityNormal,
		EstimatedDuration: 40 * time.Second,
		Capabilities:      []string{"refactoring", "improvement", "suggestions"},
		Keywords:          []string{"refactor", "refactoring", "improve code", "code improvement"},
	}

	r.triggers[domainWorker.TriggerBenchmark] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerBenchmark,
		Description:       "Performance benchmarking - measures performance metrics",
		Priority:          domainWorker.PriorityNormal,
		EstimatedDuration: 120 * time.Second,
		Capabilities:      []string{"benchmarking", "performance", "metrics"},
		Keywords:          []string{"benchmark", "benchmarking", "performance test", "perf test"},
	}

	r.triggers[domainWorker.TriggerTestgaps] = domainWorker.TriggerConfig{
		Name:              domainWorker.TriggerTestgaps,
		Description:       "Test coverage gaps - identifies missing test coverage",
		Priority:          domainWorker.PriorityNormal,
		EstimatedDuration: 35 * time.Second,
		Capabilities:      []string{"testing", "coverage", "analysis"},
		Keywords:          []string{"testgaps", "test gaps", "coverage gaps", "missing tests"},
	}
}

// GetTrigger returns the config for a trigger.
func (r *TriggerRegistry) GetTrigger(trigger domainWorker.WorkerTrigger) (domainWorker.TriggerConfig, bool) {
	config, ok := r.triggers[trigger]
	return config, ok
}

// GetAllTriggers returns all trigger configs.
func (r *TriggerRegistry) GetAllTriggers() map[domainWorker.WorkerTrigger]domainWorker.TriggerConfig {
	result := make(map[domainWorker.WorkerTrigger]domainWorker.TriggerConfig)
	for k, v := range r.triggers {
		result[k] = v
	}
	return result
}

// GetTriggerList returns all triggers as a list.
func (r *TriggerRegistry) GetTriggerList() []domainWorker.TriggerConfig {
	result := make([]domainWorker.TriggerConfig, 0, len(r.triggers))
	for _, trigger := range domainWorker.AllTriggers() {
		if config, ok := r.triggers[trigger]; ok {
			result = append(result, config)
		}
	}
	return result
}

// TriggerDetector detects triggers in text.
type TriggerDetector struct {
	registry *TriggerRegistry
	patterns map[domainWorker.WorkerTrigger]*regexp.Regexp
}

// NewTriggerDetector creates a new trigger detector.
func NewTriggerDetector(registry *TriggerRegistry) *TriggerDetector {
	detector := &TriggerDetector{
		registry: registry,
		patterns: make(map[domainWorker.WorkerTrigger]*regexp.Regexp),
	}

	// Build regex patterns for each trigger
	for trigger, config := range registry.GetAllTriggers() {
		if len(config.Keywords) > 0 {
			// Escape and join keywords
			escapedKeywords := make([]string, len(config.Keywords))
			for i, kw := range config.Keywords {
				escapedKeywords[i] = regexp.QuoteMeta(kw)
			}
			pattern := `(?i)\b(` + strings.Join(escapedKeywords, "|") + `)\b`
			detector.patterns[trigger] = regexp.MustCompile(pattern)
		}
	}

	return detector
}

// DetectTriggers detects triggers in the given text.
func (d *TriggerDetector) DetectTriggers(text string) domainWorker.TriggerDetectionResult {
	result := domainWorker.TriggerDetectionResult{
		Detected: false,
		Triggers: make([]domainWorker.DetectedTrigger, 0),
	}

	textLower := strings.ToLower(text)

	for trigger, pattern := range d.patterns {
		matches := pattern.FindAllStringSubmatch(textLower, -1)
		if len(matches) > 0 {
			// Use the first match
			keyword := matches[0][1]

			// Calculate confidence based on keyword length and specificity
			confidence := d.calculateConfidence(keyword, trigger)

			// Extract context around the match
			context := d.extractContext(text, keyword)

			result.Triggers = append(result.Triggers, domainWorker.DetectedTrigger{
				Trigger:    trigger,
				Confidence: confidence,
				Keyword:    keyword,
				Context:    context,
			})
			result.Detected = true
		}
	}

	// Sort by confidence (higher first)
	d.sortByConfidence(result.Triggers)

	return result
}

// calculateConfidence calculates detection confidence.
func (d *TriggerDetector) calculateConfidence(keyword string, trigger domainWorker.WorkerTrigger) float64 {
	// Base confidence
	confidence := 0.5

	// Longer keywords are more specific
	if len(keyword) > 8 {
		confidence += 0.2
	} else if len(keyword) > 5 {
		confidence += 0.1
	}

	// Exact trigger name match
	if strings.EqualFold(keyword, string(trigger)) {
		confidence += 0.3
	}

	// Multi-word keywords are more specific
	if strings.Contains(keyword, " ") {
		confidence += 0.1
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// extractContext extracts surrounding context for a keyword.
func (d *TriggerDetector) extractContext(text, keyword string) string {
	textLower := strings.ToLower(text)
	keywordLower := strings.ToLower(keyword)

	idx := strings.Index(textLower, keywordLower)
	if idx == -1 {
		return ""
	}

	// Get 50 characters before and after
	start := idx - 50
	if start < 0 {
		start = 0
	}
	end := idx + len(keyword) + 50
	if end > len(text) {
		end = len(text)
	}

	context := text[start:end]

	// Add ellipsis if truncated
	if start > 0 {
		context = "..." + context
	}
	if end < len(text) {
		context = context + "..."
	}

	return strings.TrimSpace(context)
}

// sortByConfidence sorts triggers by confidence (descending).
func (d *TriggerDetector) sortByConfidence(triggers []domainWorker.DetectedTrigger) {
	for i := 0; i < len(triggers); i++ {
		for j := i + 1; j < len(triggers); j++ {
			if triggers[j].Confidence > triggers[i].Confidence {
				triggers[i], triggers[j] = triggers[j], triggers[i]
			}
		}
	}
}

// GetRegistry returns the trigger registry.
func (d *TriggerDetector) GetRegistry() *TriggerRegistry {
	return d.registry
}
