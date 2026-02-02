// Package neural provides neural application services.
package neural

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/domain/neural"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
)

// NeuralService provides the main orchestration for neural operations.
type NeuralService struct {
	mu       sync.RWMutex
	store    *PatternStore
	engine   *TrainingEngine
	basePath string
}

// NewNeuralService creates a new neural service.
func NewNeuralService(basePath string) (*NeuralService, error) {
	if basePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		basePath = filepath.Join(home, ".claude-flow", "neural")
	}

	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create neural directory: %w", err)
	}

	storagePath := filepath.Join(basePath, "patterns.json")
	store, err := NewPatternStore(storagePath, 256)
	if err != nil {
		return nil, fmt.Errorf("failed to create pattern store: %w", err)
	}

	engine := NewTrainingEngine(256)

	return &NeuralService{
		store:    store,
		engine:   engine,
		basePath: basePath,
	}, nil
}

// Train trains neural patterns with the given configuration.
func (s *NeuralService) Train(config neural.TrainingConfig, data TrainingData, progressFn func(epoch int, loss float64)) (*neural.TrainingMetrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Set config
	s.engine.SetConfig(config)

	// If no data provided, generate synthetic data
	if len(data.Texts) == 0 {
		data = s.engine.GenerateSyntheticData(config.PatternType, 50)
	}

	// Run training
	metrics, patterns, err := s.engine.Train(data, progressFn)
	if err != nil {
		return nil, err
	}

	// Store learned patterns
	for _, p := range patterns {
		if err := s.store.Add(p); err != nil {
			return nil, fmt.Errorf("failed to store pattern: %w", err)
		}
	}

	// Save to disk
	if err := s.store.Save(); err != nil {
		return nil, fmt.Errorf("failed to save patterns: %w", err)
	}

	return metrics, nil
}

// Learn learns from an outcome and creates a new pattern.
func (s *NeuralService) Learn(agentID string, learningType neural.LearningType, input string) (*neural.Pattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Determine success based on learning type
	success := learningType == neural.LearningTypePattern || learningType == neural.LearningTypeConsolidation

	pattern := s.engine.LearnFromOutcome(input, success)

	if err := s.store.Add(pattern); err != nil {
		return nil, fmt.Errorf("failed to store pattern: %w", err)
	}

	if err := s.store.Save(); err != nil {
		return nil, fmt.Errorf("failed to save patterns: %w", err)
	}

	return pattern, nil
}

// ListPatterns returns all patterns, optionally filtered by type.
func (s *NeuralService) ListPatterns(patternType string, limit int) []*neural.Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var patterns []*neural.Pattern
	if patternType != "" {
		patterns = s.store.GetByType(patternType)
	} else {
		patterns = s.store.GetAll()
	}

	if limit > 0 && len(patterns) > limit {
		patterns = patterns[:limit]
	}

	return patterns
}

// SearchPatterns searches for patterns similar to the query.
func (s *NeuralService) SearchPatterns(query string, limit int) []*neural.Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.Search(query, limit)
}

// Optimize optimizes the pattern store using the specified method.
func (s *NeuralService) Optimize(method neural.OptimizationMethod, verbose bool) (*neural.OptimizationMetrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	startTime := time.Now()
	metrics := &neural.OptimizationMetrics{
		Method: string(method),
	}

	// Get original size
	info, err := os.Stat(s.store.GetStoragePath())
	if err == nil {
		metrics.OriginalSize = info.Size()
	}

	switch method {
	case neural.OptimizeQuantize:
		// Quantize embeddings (simulation - actual quantization happens in-memory)
		patterns := s.store.GetAll()
		for _, p := range patterns {
			// Quantize and immediately dequantize to simulate the process
			quantized := infraNeural.QuantizeInt8(p.Embedding)
			_ = quantized // In a real implementation, we'd store quantized versions
		}
		metrics.MemoryReduction = "4x (simulated)"

	case neural.OptimizeCompact:
		removed := s.store.Compact(0.95)
		metrics.PatternsRemoved = removed
		if err := s.store.Save(); err != nil {
			return nil, fmt.Errorf("failed to save after compact: %w", err)
		}

	case neural.OptimizeAnalyze:
		// Just gather stats
		stats := s.store.GetStats()
		metrics.PatternsRemoved = 0
		metrics.MemoryReduction = fmt.Sprintf("%d patterns, %.2f avg confidence", stats.PatternCount, stats.AverageConfidence)
	}

	// Get optimized size
	if err := s.store.Save(); err == nil {
		info, err := os.Stat(s.store.GetStoragePath())
		if err == nil {
			metrics.OptimizedSize = info.Size()
			if metrics.OriginalSize > 0 {
				metrics.CompressionRatio = float64(metrics.OriginalSize) / float64(metrics.OptimizedSize)
			}
		}
	}

	metrics.DurationMs = time.Since(startTime).Milliseconds()

	return metrics, nil
}

// ExportPackage represents an exported pattern package.
type ExportPackage struct {
	Type       string            `json:"type"`
	Version    string            `json:"version"`
	Name       string            `json:"name"`
	ExportedAt string            `json:"exportedAt"`
	ModelID    string            `json:"modelId"`
	Patterns   []ExportedPattern `json:"patterns"`
	Metadata   ExportMetadata    `json:"metadata"`
	Signature  string            `json:"signature,omitempty"`
	PublicKey  string            `json:"publicKey,omitempty"`
}

// ExportedPattern represents a pattern in export format.
type ExportedPattern struct {
	ID         string  `json:"id"`
	Trigger    string  `json:"trigger"`
	Action     string  `json:"action"`
	Confidence float64 `json:"confidence"`
	UsageCount int     `json:"usageCount"`
}

// ExportMetadata holds export metadata.
type ExportMetadata struct {
	SourceVersion string  `json:"sourceVersion"`
	PIIStripped   bool    `json:"piiStripped"`
	Signed        bool    `json:"signed"`
	Accuracy      float64 `json:"accuracy"`
	TotalUsage    int     `json:"totalUsage"`
}

// Export exports patterns to a file with optional signing.
func (s *NeuralService) Export(config neural.ExportConfig) (*ExportPackage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	patterns := s.store.GetAll()
	if config.ModelID != "" {
		filtered := make([]*neural.Pattern, 0)
		for _, p := range patterns {
			if p.Type == config.ModelID {
				filtered = append(filtered, p)
			}
		}
		patterns = filtered
	}

	exportedPatterns := make([]ExportedPattern, len(patterns))
	var totalUsage int
	var totalConfidence float64

	for i, p := range patterns {
		content := p.Content
		if config.StripPII {
			content = stripPII(content)
		}

		exportedPatterns[i] = ExportedPattern{
			ID:         p.ID,
			Trigger:    content,
			Action:     p.Type,
			Confidence: p.Confidence,
			UsageCount: p.UsageCount,
		}
		totalUsage += p.UsageCount
		totalConfidence += p.Confidence
	}

	avgAccuracy := 0.0
	if len(patterns) > 0 {
		avgAccuracy = totalConfidence / float64(len(patterns))
	}

	pkg := &ExportPackage{
		Type:       "claude-flow-patterns",
		Version:    "1.0.0",
		Name:       config.ModelID,
		ExportedAt: time.Now().Format(time.RFC3339),
		ModelID:    config.ModelID,
		Patterns:   exportedPatterns,
		Metadata: ExportMetadata{
			SourceVersion: "3.0.0",
			PIIStripped:   config.StripPII,
			Signed:        config.Sign,
			Accuracy:      avgAccuracy,
			TotalUsage:    totalUsage,
		},
	}

	// Sign if requested
	if config.Sign {
		pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate signing key: %w", err)
		}

		// Sign the patterns data
		patternsJSON, _ := json.Marshal(pkg.Patterns)
		signature := ed25519.Sign(privKey, patternsJSON)

		pkg.Signature = base64.StdEncoding.EncodeToString(signature)
		pkg.PublicKey = base64.StdEncoding.EncodeToString(pubKey)
	}

	// Write to file if output path specified
	if config.OutputPath != "" {
		data, err := json.MarshalIndent(pkg, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal export: %w", err)
		}

		if err := os.WriteFile(config.OutputPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write export file: %w", err)
		}
	}

	return pkg, nil
}

// Import imports patterns from a file.
func (s *NeuralService) Import(config neural.ImportConfig) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(config.FilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read import file: %w", err)
	}

	var pkg ExportPackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return 0, fmt.Errorf("failed to parse import file: %w", err)
	}

	// Verify signature if requested
	if config.Verify && pkg.Signature != "" && pkg.PublicKey != "" {
		pubKeyBytes, err := base64.StdEncoding.DecodeString(pkg.PublicKey)
		if err != nil {
			return 0, fmt.Errorf("failed to decode public key: %w", err)
		}

		sigBytes, err := base64.StdEncoding.DecodeString(pkg.Signature)
		if err != nil {
			return 0, fmt.Errorf("failed to decode signature: %w", err)
		}

		patternsJSON, _ := json.Marshal(pkg.Patterns)
		if !ed25519.Verify(pubKeyBytes, patternsJSON, sigBytes) {
			return 0, fmt.Errorf("signature verification failed")
		}
	}

	// Clear existing if not merging
	if !config.Merge {
		s.store.Clear()
	}

	// Import patterns
	generator := infraNeural.NewEmbeddingGenerator(256)
	imported := 0

	for _, ep := range pkg.Patterns {
		// Filter by category if specified
		if config.Category != "" && ep.Action != config.Category {
			continue
		}

		// Skip suspicious patterns
		if isSuspicious(ep.Trigger) {
			continue
		}

		// Create pattern
		embedding := generator.Generate(ep.Trigger)
		pattern := &neural.Pattern{
			ID:         ep.ID,
			Type:       ep.Action,
			Content:    ep.Trigger,
			Embedding:  embedding,
			Confidence: ep.Confidence,
			UsageCount: ep.UsageCount,
			CreatedAt:  time.Now(),
			LastUsedAt: time.Now(),
		}

		if err := s.store.Add(pattern); err != nil {
			continue
		}
		imported++
	}

	if err := s.store.Save(); err != nil {
		return imported, fmt.Errorf("failed to save after import: %w", err)
	}

	return imported, nil
}

// Benchmark runs performance benchmarks.
func (s *NeuralService) Benchmark(config neural.BenchmarkConfig) (*neural.BenchmarkMetrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]neural.BenchmarkResult, 0)
	generator := infraNeural.NewEmbeddingGenerator(config.Dimension)

	// Benchmark embedding generation
	start := time.Now()
	for i := 0; i < config.Iterations; i++ {
		generator.Generate(fmt.Sprintf("test embedding %d", i))
	}
	embedTime := time.Since(start)
	results = append(results, neural.BenchmarkResult{
		Operation:    "Embedding Generation",
		Iterations:   config.Iterations,
		TotalTimeMs:  float64(embedTime.Milliseconds()),
		AvgTimeUs:    float64(embedTime.Microseconds()) / float64(config.Iterations),
		OpsPerSecond: float64(config.Iterations) / embedTime.Seconds(),
	})

	// Benchmark similarity computation
	emb1 := generator.Generate("test embedding 1")
	emb2 := generator.Generate("test embedding 2")
	start = time.Now()
	for i := 0; i < config.Iterations; i++ {
		infraNeural.CosineSimilarity(emb1, emb2)
	}
	simTime := time.Since(start)
	results = append(results, neural.BenchmarkResult{
		Operation:    "Cosine Similarity",
		Iterations:   config.Iterations,
		TotalTimeMs:  float64(simTime.Milliseconds()),
		AvgTimeUs:    float64(simTime.Microseconds()) / float64(config.Iterations),
		OpsPerSecond: float64(config.Iterations) / simTime.Seconds(),
	})

	// Benchmark quantization
	start = time.Now()
	for i := 0; i < config.Iterations; i++ {
		infraNeural.QuantizeInt8(emb1)
	}
	quantTime := time.Since(start)
	results = append(results, neural.BenchmarkResult{
		Operation:    "Int8 Quantization",
		Iterations:   config.Iterations,
		TotalTimeMs:  float64(quantTime.Milliseconds()),
		AvgTimeUs:    float64(quantTime.Microseconds()) / float64(config.Iterations),
		OpsPerSecond: float64(config.Iterations) / quantTime.Seconds(),
	})

	// Benchmark pattern search (if we have patterns)
	if s.store.Count() > 0 {
		start = time.Now()
		searchIterations := config.Iterations / 10 // Fewer iterations for search
		if searchIterations < 1 {
			searchIterations = 1
		}
		for i := 0; i < searchIterations; i++ {
			s.store.Search("test query", 10)
		}
		searchTime := time.Since(start)
		results = append(results, neural.BenchmarkResult{
			Operation:    "Pattern Search",
			Iterations:   searchIterations,
			TotalTimeMs:  float64(searchTime.Milliseconds()),
			AvgTimeUs:    float64(searchTime.Microseconds()) / float64(searchIterations),
			OpsPerSecond: float64(searchIterations) / searchTime.Seconds(),
		})
	}

	return &neural.BenchmarkMetrics{
		Dimension: config.Dimension,
		Results:   results,
		Summary:   fmt.Sprintf("Benchmarked %d operations at dimension %d", len(results), config.Dimension),
	}, nil
}

// GetStatus returns the current status of the neural system.
func (s *NeuralService) GetStatus() neural.NeuralSystemStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.GetStats()
}

// GetBasePath returns the base storage path.
func (s *NeuralService) GetBasePath() string {
	return s.basePath
}

// stripPII removes potential PII from content.
func stripPII(content string) string {
	// Remove email addresses
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	content = emailRegex.ReplaceAllString(content, "[EMAIL]")

	// Remove file paths
	pathRegex := regexp.MustCompile(`(/[a-zA-Z0-9._-]+)+`)
	content = pathRegex.ReplaceAllString(content, "[PATH]")

	// Remove IP addresses
	ipRegex := regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	content = ipRegex.ReplaceAllString(content, "[IP]")

	return content
}

// isSuspicious checks if content contains suspicious patterns.
func isSuspicious(content string) bool {
	suspiciousPatterns := []string{
		"eval(", "exec(", "system(", "__import__",
		"subprocess", "os.system", "shell_exec",
	}

	for _, pattern := range suspiciousPatterns {
		if regexp.MustCompile(regexp.QuoteMeta(pattern)).MatchString(content) {
			return true
		}
	}

	return false
}
