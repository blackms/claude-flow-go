// Package neural provides neural pattern domain entities for the neural CLI commands.
package neural

// TrainingConfig holds training hyperparameters.
type TrainingConfig struct {
	PatternType  string  `json:"patternType"`
	Epochs       int     `json:"epochs"`
	LearningRate float64 `json:"learningRate"`
	BatchSize    int     `json:"batchSize"`
	Dimension    int     `json:"dimension"`
}

// DefaultTrainingConfig returns sensible defaults for training.
func DefaultTrainingConfig() TrainingConfig {
	return TrainingConfig{
		PatternType:  string(PatternTypeCoordination),
		Epochs:       50,
		LearningRate: 0.01,
		BatchSize:    32,
		Dimension:    256,
	}
}

// OptimizationMethod represents methods for optimizing patterns.
type OptimizationMethod string

const (
	OptimizeQuantize OptimizationMethod = "quantize"
	OptimizeAnalyze  OptimizationMethod = "analyze"
	OptimizeCompact  OptimizationMethod = "compact"
)

// LearningType represents types of learning operations.
type LearningType string

const (
	LearningTypeOutcome       LearningType = "outcome"
	LearningTypePattern       LearningType = "pattern"
	LearningTypeConsolidation LearningType = "consolidation"
)

// ExportConfig holds configuration for pattern export.
type ExportConfig struct {
	OutputPath string `json:"outputPath"`
	ModelID    string `json:"modelId"`
	Sign       bool   `json:"sign"`
	StripPII   bool   `json:"stripPii"`
}

// DefaultExportConfig returns sensible defaults for export.
func DefaultExportConfig() ExportConfig {
	return ExportConfig{
		Sign:     true,
		StripPII: true,
	}
}

// ImportConfig holds configuration for pattern import.
type ImportConfig struct {
	FilePath string `json:"filePath"`
	Verify   bool   `json:"verify"`
	Merge    bool   `json:"merge"`
	Category string `json:"category"`
}

// DefaultImportConfig returns sensible defaults for import.
func DefaultImportConfig() ImportConfig {
	return ImportConfig{
		Verify: true,
		Merge:  true,
	}
}

// BenchmarkConfig holds configuration for benchmarking.
type BenchmarkConfig struct {
	Dimension  int `json:"dimension"`
	Iterations int `json:"iterations"`
}

// DefaultBenchmarkConfig returns sensible defaults for benchmarking.
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		Dimension:  256,
		Iterations: 1000,
	}
}

// NeuralSystemStatus holds the status of the neural system.
type NeuralSystemStatus struct {
	Initialized       bool    `json:"initialized"`
	PatternCount      int     `json:"patternCount"`
	TotalUsage        int     `json:"totalUsage"`
	AverageConfidence float64 `json:"averageConfidence"`
	StoragePath       string  `json:"storagePath"`
	LastUpdated       string  `json:"lastUpdated"`
}

// TrainingMetrics holds metrics from a training run.
type TrainingMetrics struct {
	Epochs          int       `json:"epochs"`
	FinalLoss       float64   `json:"finalLoss"`
	TotalPatterns   int       `json:"totalPatterns"`
	Adaptations     int       `json:"adaptations"`
	TrainingTimeMs  int64     `json:"trainingTimeMs"`
	LossHistory     []float64 `json:"lossHistory"`
	PatternType     string    `json:"patternType"`
	LearningRate    float64   `json:"learningRate"`
}

// OptimizationMetrics holds metrics from an optimization run.
type OptimizationMetrics struct {
	Method            string  `json:"method"`
	OriginalSize      int64   `json:"originalSize"`
	OptimizedSize     int64   `json:"optimizedSize"`
	CompressionRatio  float64 `json:"compressionRatio"`
	PatternsRemoved   int     `json:"patternsRemoved"`
	MemoryReduction   string  `json:"memoryReduction"`
	DurationMs        int64   `json:"durationMs"`
}

// BenchmarkResult holds results from a benchmark run.
type BenchmarkResult struct {
	Operation      string  `json:"operation"`
	Iterations     int     `json:"iterations"`
	TotalTimeMs    float64 `json:"totalTimeMs"`
	AvgTimeUs      float64 `json:"avgTimeUs"`
	OpsPerSecond   float64 `json:"opsPerSecond"`
}

// BenchmarkMetrics holds all benchmark results.
type BenchmarkMetrics struct {
	Dimension  int               `json:"dimension"`
	Results    []BenchmarkResult `json:"results"`
	Summary    string            `json:"summary"`
}
