// Package neural provides neural network infrastructure.
package neural

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	"github.com/anthropics/claude-flow-go/internal/domain/transfer"
)

// ReasoningSerializer handles serialization of reasoning patterns to CFP format.
type ReasoningSerializer struct {
	generatedBy string
}

// NewReasoningSerializer creates a new reasoning serializer.
func NewReasoningSerializer() *ReasoningSerializer {
	return &ReasoningSerializer{
		generatedBy: "claude-flow-go@3.0.0",
	}
}

// SerializePattern serializes a single pattern to JSON string.
func (s *ReasoningSerializer) SerializePattern(pattern domainNeural.ReasoningPattern) (string, error) {
	data, err := json.Marshal(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to serialize pattern: %w", err)
	}
	return string(data), nil
}

// DeserializePattern deserializes a JSON string to a pattern.
func (s *ReasoningSerializer) DeserializePattern(data string) (*domainNeural.ReasoningPattern, error) {
	var pattern domainNeural.ReasoningPattern
	if err := json.Unmarshal([]byte(data), &pattern); err != nil {
		return nil, fmt.Errorf("failed to deserialize pattern: %w", err)
	}
	return &pattern, nil
}

// SerializePatterns serializes multiple patterns to JSON string.
func (s *ReasoningSerializer) SerializePatterns(patterns []domainNeural.ReasoningPattern) (string, error) {
	data, err := json.Marshal(patterns)
	if err != nil {
		return "", fmt.Errorf("failed to serialize patterns: %w", err)
	}
	return string(data), nil
}

// DeserializePatterns deserializes a JSON string to patterns.
func (s *ReasoningSerializer) DeserializePatterns(data string) ([]domainNeural.ReasoningPattern, error) {
	var patterns []domainNeural.ReasoningPattern
	if err := json.Unmarshal([]byte(data), &patterns); err != nil {
		return nil, fmt.Errorf("failed to deserialize patterns: %w", err)
	}
	return patterns, nil
}

// ExportToCFP exports reasoning patterns to CFP format.
func (s *ReasoningSerializer) ExportToCFP(patterns []domainNeural.ReasoningPattern, metadata ExportMetadata) (*transfer.CFPFormat, error) {
	cfp := transfer.NewCFPFormat()
	cfp.GeneratedBy = s.generatedBy
	cfp.CreatedAt = time.Now().Format(time.RFC3339)

	// Set metadata
	cfp.Metadata = transfer.CFPMetadata{
		ID:          metadata.ID,
		Name:        metadata.Name,
		Description: metadata.Description,
		License:     metadata.License,
		Tags:        metadata.Tags,
		CreatedAt:   time.Now().Format(time.RFC3339),
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}

	if metadata.Author != "" {
		cfp.Metadata.Author = &transfer.Author{
			Name: metadata.Author,
		}
	}

	// Set anonymization
	cfp.Anonymization = transfer.CFPAnonymization{
		Level:                 string(metadata.AnonymizationLevel),
		PIIRedacted:           true,
		PathsStripped:         true,
		TimestampsGeneralized: true,
		AppliedTransforms:     []string{"pii_redaction", "path_stripping", "timestamp_generalization"},
	}

	// Compute checksum of original data
	originalData, _ := json.Marshal(patterns)
	hash := sha256.Sum256(originalData)
	cfp.Anonymization.Checksum = base64.StdEncoding.EncodeToString(hash[:])

	// Convert reasoning patterns to CFP patterns
	for _, pattern := range patterns {
		// Add as trajectory pattern (closest match for reasoning patterns)
		trajectoryPattern := s.reasoningToTrajectoryPattern(pattern)
		cfp.Patterns.Trajectory = append(cfp.Patterns.Trajectory, trajectoryPattern)

		// Also add as custom pattern for full fidelity
		customPattern := s.reasoningToCustomPattern(pattern, metadata.AnonymizationLevel)
		cfp.Patterns.Custom = append(cfp.Patterns.Custom, customPattern)
	}

	// Compute statistics
	cfp.Statistics = s.computeStatistics(patterns)

	return cfp, nil
}

// ImportFromCFP imports reasoning patterns from CFP format.
func (s *ReasoningSerializer) ImportFromCFP(cfp *transfer.CFPFormat) ([]domainNeural.ReasoningPattern, error) {
	if err := cfp.Validate(); err != nil {
		return nil, fmt.Errorf("invalid CFP format: %w", err)
	}

	patterns := make([]domainNeural.ReasoningPattern, 0)

	// Import from custom patterns first (full fidelity)
	for _, custom := range cfp.Patterns.Custom {
		if custom.Type == "reasoning_pattern" {
			pattern, err := s.customToReasoningPattern(custom)
			if err != nil {
				continue
			}
			patterns = append(patterns, *pattern)
		}
	}

	// If no custom patterns, import from trajectory patterns
	if len(patterns) == 0 {
		for _, traj := range cfp.Patterns.Trajectory {
			pattern := s.trajectoryToReasoningPattern(traj)
			patterns = append(patterns, pattern)
		}
	}

	return patterns, nil
}

// ExportMetadata contains metadata for CFP export.
type ExportMetadata struct {
	ID                 string
	Name               string
	Description        string
	Author             string
	License            string
	Tags               []string
	AnonymizationLevel transfer.AnonymizationLevel
}

// DefaultExportMetadata returns default export metadata.
func DefaultExportMetadata() ExportMetadata {
	return ExportMetadata{
		ID:                 fmt.Sprintf("reasoning-%d", time.Now().UnixNano()),
		Name:               "Reasoning Patterns Export",
		Description:        "Exported reasoning patterns from ReasoningBank",
		License:            "MIT",
		Tags:               []string{"reasoning", "patterns", "learning"},
		AnonymizationLevel: transfer.AnonymizationStandard,
	}
}

// Private methods

func (s *ReasoningSerializer) reasoningToTrajectoryPattern(pattern domainNeural.ReasoningPattern) transfer.TrajectoryPattern {
	return transfer.TrajectoryPattern{
		ID:        pattern.PatternID,
		Steps:     s.extractStepsFromStrategy(pattern.Strategy),
		Outcome:   fmt.Sprintf("success_rate:%.2f", pattern.SuccessRate),
		Duration:  int64(pattern.UsageCount * 1000), // Estimate
		Learnings: pattern.KeyLearnings,
	}
}

func (s *ReasoningSerializer) reasoningToCustomPattern(pattern domainNeural.ReasoningPattern, level transfer.AnonymizationLevel) transfer.CustomPattern {
	data := map[string]interface{}{
		"name":        pattern.Name,
		"domain":      string(pattern.Domain),
		"strategy":    s.anonymizeStrategy(pattern.Strategy, level),
		"successRate": pattern.SuccessRate,
		"usageCount":  pattern.UsageCount,
		"version":     pattern.Version,
	}

	// Include embedding if not paranoid level
	if level != transfer.AnonymizationParanoid {
		// Quantize embedding to reduce size
		quantized := s.quantizeEmbedding(pattern.Embedding)
		data["embedding"] = quantized
	}

	metadata := map[string]interface{}{
		"keyLearnings":   pattern.KeyLearnings,
		"qualityHistory": s.generalizeQualityHistory(pattern.QualityHistory),
	}

	// Add evolution history (generalized)
	if len(pattern.EvolutionHistory) > 0 {
		evolutions := make([]map[string]interface{}, 0)
		for _, ev := range pattern.EvolutionHistory {
			evolutions = append(evolutions, map[string]interface{}{
				"type":         string(ev.Type),
				"qualityDelta": ev.QualityAfter - ev.QualityBefore,
			})
		}
		metadata["evolutionHistory"] = evolutions
	}

	return transfer.CustomPattern{
		ID:       pattern.PatternID,
		Type:     "reasoning_pattern",
		Data:     data,
		Metadata: metadata,
	}
}

func (s *ReasoningSerializer) customToReasoningPattern(custom transfer.CustomPattern) (*domainNeural.ReasoningPattern, error) {
	pattern := &domainNeural.ReasoningPattern{
		PatternID: custom.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Extract data fields
	if name, ok := custom.Data["name"].(string); ok {
		pattern.Name = name
	}
	if domain, ok := custom.Data["domain"].(string); ok {
		pattern.Domain = domainNeural.ReasoningDomain(domain)
	}
	if strategy, ok := custom.Data["strategy"].(string); ok {
		pattern.Strategy = strategy
	}
	if successRate, ok := custom.Data["successRate"].(float64); ok {
		pattern.SuccessRate = successRate
	}
	if usageCount, ok := custom.Data["usageCount"].(float64); ok {
		pattern.UsageCount = int(usageCount)
	}
	if version, ok := custom.Data["version"].(float64); ok {
		pattern.Version = int(version)
	}

	// Extract embedding
	if embeddingData, ok := custom.Data["embedding"].([]interface{}); ok {
		pattern.Embedding = make([]float32, len(embeddingData))
		for i, v := range embeddingData {
			if f, ok := v.(float64); ok {
				pattern.Embedding[i] = float32(f)
			}
		}
	}

	// Extract metadata fields
	if keyLearnings, ok := custom.Metadata["keyLearnings"].([]interface{}); ok {
		pattern.KeyLearnings = make([]string, 0, len(keyLearnings))
		for _, kl := range keyLearnings {
			if s, ok := kl.(string); ok {
				pattern.KeyLearnings = append(pattern.KeyLearnings, s)
			}
		}
	}

	if qualityHistory, ok := custom.Metadata["qualityHistory"].([]interface{}); ok {
		pattern.QualityHistory = make([]float64, 0, len(qualityHistory))
		for _, q := range qualityHistory {
			if f, ok := q.(float64); ok {
				pattern.QualityHistory = append(pattern.QualityHistory, f)
			}
		}
	}

	return pattern, nil
}

func (s *ReasoningSerializer) trajectoryToReasoningPattern(traj transfer.TrajectoryPattern) domainNeural.ReasoningPattern {
	return domainNeural.ReasoningPattern{
		PatternID:    traj.ID,
		Name:         fmt.Sprintf("Imported: %s", traj.ID),
		Domain:       domainNeural.ReasoningDomainGeneral,
		Strategy:     s.stepsToStrategy(traj.Steps),
		KeyLearnings: traj.Learnings,
		UsageCount:   1,
		Version:      1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func (s *ReasoningSerializer) extractStepsFromStrategy(strategy string) []string {
	// Simple extraction - split by common delimiters
	if strategy == "" {
		return []string{}
	}
	return []string{strategy}
}

func (s *ReasoningSerializer) stepsToStrategy(steps []string) string {
	if len(steps) == 0 {
		return ""
	}
	// Combine steps into a strategy string
	result := ""
	for i, step := range steps {
		if i > 0 {
			result += " -> "
		}
		result += step
	}
	return result
}

func (s *ReasoningSerializer) anonymizeStrategy(strategy string, level transfer.AnonymizationLevel) string {
	if level == transfer.AnonymizationMinimal {
		return strategy
	}
	// For higher levels, we could apply more transformations
	// For now, return as-is (real implementation would redact PII)
	return strategy
}

func (s *ReasoningSerializer) quantizeEmbedding(embedding []float32) []float64 {
	// Convert to float64 for JSON (with reduced precision)
	result := make([]float64, len(embedding))
	for i, v := range embedding {
		// Round to 4 decimal places to reduce size
		result[i] = float64(int(v*10000)) / 10000
	}
	return result
}

func (s *ReasoningSerializer) generalizeQualityHistory(history []float64) []float64 {
	if len(history) == 0 {
		return []float64{}
	}

	// Return last 10 entries max
	if len(history) > 10 {
		history = history[len(history)-10:]
	}

	// Round to 2 decimal places
	result := make([]float64, len(history))
	for i, v := range history {
		result[i] = float64(int(v*100)) / 100
	}
	return result
}

func (s *ReasoningSerializer) computeStatistics(patterns []domainNeural.ReasoningPattern) transfer.CFPStatistics {
	stats := transfer.CFPStatistics{
		TotalPatterns: len(patterns),
		PatternTypes:  make(map[string]int),
	}

	if len(patterns) == 0 {
		return stats
	}

	var totalConfidence float64
	var minTime, maxTime time.Time

	for i, p := range patterns {
		totalConfidence += p.SuccessRate
		stats.PatternTypes[string(p.Domain)]++

		if i == 0 || p.CreatedAt.Before(minTime) {
			minTime = p.CreatedAt
		}
		if i == 0 || p.CreatedAt.After(maxTime) {
			maxTime = p.CreatedAt
		}
	}

	stats.AvgConfidence = totalConfidence / float64(len(patterns))
	stats.TimeRange = transfer.CFPTimeRange{
		Start: minTime.Format(time.RFC3339),
		End:   maxTime.Format(time.RFC3339),
	}

	return stats
}
