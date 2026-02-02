// Package commands provides CLI command implementations.
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	appNeural "github.com/anthropics/claude-flow-go/internal/application/neural"
	"github.com/anthropics/claude-flow-go/internal/domain/neural"
)

// Flag variables for neural commands
var (
	// train flags
	trainPattern      string
	trainEpochs       int
	trainData         string
	trainLearningRate float64
	trainBatchSize    int
	trainDim          int

	// learn flags
	learnAgentID string
	learnType    string
	learnInput   string

	// patterns flags
	patternsAction string
	patternsQuery  string
	patternsLimit  int
	patternsFormat string

	// optimize flags
	optimizeMethod  string
	optimizeVerbose bool

	// export flags
	exportOutput   string
	exportModel    string
	exportSign     bool
	exportStripPII bool

	// import flags
	importFile     string
	importVerify   bool
	importMerge    bool
	importCategory string

	// benchmark flags
	benchmarkDim        int
	benchmarkIterations int

	// status flags
	statusVerbose bool
)

// NeuralCmd is the parent command for all neural subcommands.
var NeuralCmd = &cobra.Command{
	Use:   "neural",
	Short: "Neural pattern learning and training commands",
	Long: `Commands for neural pattern learning, training, and management.

The neural system provides:
  - Pattern training with contrastive learning
  - Pattern search and management
  - Import/export for sharing patterns
  - Optimization and benchmarking`,
}

// trainCmd handles neural training
var trainCmd = &cobra.Command{
	Use:   "train",
	Short: "Train neural patterns",
	Long:  `Train neural patterns using contrastive learning with configurable hyperparameters.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appNeural.NewNeuralService("")
		if err != nil {
			return fmt.Errorf("failed to initialize neural service: %w", err)
		}

		config := neural.TrainingConfig{
			PatternType:  trainPattern,
			Epochs:       trainEpochs,
			LearningRate: trainLearningRate,
			BatchSize:    trainBatchSize,
			Dimension:    trainDim,
		}

		// Parse training data if provided
		var data appNeural.TrainingData
		if trainData != "" {
			// Try to parse as JSON first
			if err := json.Unmarshal([]byte(trainData), &data); err != nil {
				// If not JSON, try to read as file
				fileData, fileErr := os.ReadFile(trainData)
				if fileErr != nil {
					// Treat as comma-separated texts
					data.Texts = strings.Split(trainData, ",")
				} else {
					if err := json.Unmarshal(fileData, &data); err != nil {
						// Treat file content as newline-separated texts
						data.Texts = strings.Split(string(fileData), "\n")
					}
				}
			}
		}

		fmt.Printf("Training neural patterns (type: %s, epochs: %d)\n", config.PatternType, config.Epochs)
		fmt.Println(strings.Repeat("-", 50))

		// Progress callback
		progressFn := func(epoch int, loss float64) {
			bar := strings.Repeat("█", epoch*30/config.Epochs)
			spaces := strings.Repeat("░", 30-len(bar))
			fmt.Printf("\rEpoch %3d/%d [%s%s] Loss: %.4f", epoch, config.Epochs, bar, spaces, loss)
		}

		metrics, err := service.Train(config, data, progressFn)
		if err != nil {
			return fmt.Errorf("training failed: %w", err)
		}

		fmt.Println() // New line after progress bar
		fmt.Println(strings.Repeat("-", 50))
		fmt.Println("\nTraining Complete!")
		fmt.Printf("  Patterns created: %d\n", metrics.TotalPatterns)
		fmt.Printf("  Final loss:       %.4f\n", metrics.FinalLoss)
		fmt.Printf("  Adaptations:      %d\n", metrics.Adaptations)
		fmt.Printf("  Training time:    %dms\n", metrics.TrainingTimeMs)

		return nil
	},
}

// learnCmd handles learning from outcomes
var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Learn from outcomes",
	Long:  `Learn from outcomes to extract patterns and consolidate memory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appNeural.NewNeuralService("")
		if err != nil {
			return fmt.Errorf("failed to initialize neural service: %w", err)
		}

		// Read input from file if path provided
		input := learnInput
		if fileData, err := os.ReadFile(learnInput); err == nil {
			input = string(fileData)
		}

		pattern, err := service.Learn(learnAgentID, neural.LearningType(learnType), input)
		if err != nil {
			return fmt.Errorf("learning failed: %w", err)
		}

		fmt.Println("Pattern learned successfully!")
		fmt.Printf("  ID:         %s\n", pattern.ID)
		fmt.Printf("  Type:       %s\n", pattern.Type)
		fmt.Printf("  Confidence: %.2f\n", pattern.Confidence)

		return nil
	},
}

// patternsCmd handles pattern listing and search
var patternsCmd = &cobra.Command{
	Use:   "patterns",
	Short: "List and search patterns",
	Long:  `List, search, and analyze neural patterns.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appNeural.NewNeuralService("")
		if err != nil {
			return fmt.Errorf("failed to initialize neural service: %w", err)
		}

		var patterns []*neural.Pattern

		switch patternsAction {
		case "search":
			if patternsQuery == "" {
				return fmt.Errorf("query is required for search action")
			}
			patterns = service.SearchPatterns(patternsQuery, patternsLimit)
		case "analyze":
			patterns = service.ListPatterns("", patternsLimit)
			// Additional analysis could be added here
		default: // list
			patterns = service.ListPatterns("", patternsLimit)
		}

		if len(patterns) == 0 {
			fmt.Println("No patterns found")
			return nil
		}

		if patternsFormat == "json" {
			output, _ := json.MarshalIndent(patterns, "", "  ")
			fmt.Println(string(output))
		} else {
			// Table format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTYPE\tCONFIDENCE\tUSAGE\tCONTENT")
			fmt.Fprintln(w, strings.Repeat("-", 80))
			for _, p := range patterns {
				content := p.Content
				if len(content) > 40 {
					content = content[:37] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%.2f\t%d\t%s\n",
					p.ID[:8]+"...", p.Type, p.Confidence, p.UsageCount, content)
			}
			w.Flush()
		}

		return nil
	},
}

// optimizeCmd handles pattern optimization
var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Optimize neural patterns",
	Long:  `Optimize neural patterns using quantization, compaction, or analysis.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appNeural.NewNeuralService("")
		if err != nil {
			return fmt.Errorf("failed to initialize neural service: %w", err)
		}

		fmt.Printf("Running optimization (method: %s)...\n", optimizeMethod)

		metrics, err := service.Optimize(neural.OptimizationMethod(optimizeMethod), optimizeVerbose)
		if err != nil {
			return fmt.Errorf("optimization failed: %w", err)
		}

		fmt.Println("\nOptimization Complete!")
		fmt.Printf("  Method:           %s\n", metrics.Method)
		fmt.Printf("  Original size:    %d bytes\n", metrics.OriginalSize)
		fmt.Printf("  Optimized size:   %d bytes\n", metrics.OptimizedSize)
		if metrics.CompressionRatio > 0 {
			fmt.Printf("  Compression:      %.2fx\n", metrics.CompressionRatio)
		}
		if metrics.PatternsRemoved > 0 {
			fmt.Printf("  Patterns removed: %d\n", metrics.PatternsRemoved)
		}
		if metrics.MemoryReduction != "" {
			fmt.Printf("  Memory reduction: %s\n", metrics.MemoryReduction)
		}
		fmt.Printf("  Duration:         %dms\n", metrics.DurationMs)

		return nil
	},
}

// exportCmd handles pattern export
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export neural patterns",
	Long:  `Export neural patterns to a file with optional signing and PII stripping.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appNeural.NewNeuralService("")
		if err != nil {
			return fmt.Errorf("failed to initialize neural service: %w", err)
		}

		config := neural.ExportConfig{
			OutputPath: exportOutput,
			ModelID:    exportModel,
			Sign:       exportSign,
			StripPII:   exportStripPII,
		}

		pkg, err := service.Export(config)
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		if exportOutput != "" {
			fmt.Printf("Exported %d patterns to %s\n", len(pkg.Patterns), exportOutput)
		} else {
			output, _ := json.MarshalIndent(pkg, "", "  ")
			fmt.Println(string(output))
		}

		if pkg.Signature != "" {
			fmt.Println("\nExport signed with Ed25519")
			fmt.Printf("  Public key: %s...\n", pkg.PublicKey[:32])
		}

		return nil
	},
}

// importCmd handles pattern import
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import neural patterns",
	Long:  `Import neural patterns from a file with optional signature verification.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if importFile == "" {
			return fmt.Errorf("file path is required")
		}

		service, err := appNeural.NewNeuralService("")
		if err != nil {
			return fmt.Errorf("failed to initialize neural service: %w", err)
		}

		config := neural.ImportConfig{
			FilePath: importFile,
			Verify:   importVerify,
			Merge:    importMerge,
			Category: importCategory,
		}

		count, err := service.Import(config)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		fmt.Printf("Successfully imported %d patterns\n", count)
		if importMerge {
			fmt.Println("Patterns merged with existing store")
		} else {
			fmt.Println("Existing patterns replaced")
		}

		return nil
	},
}

// benchmarkCmd handles performance benchmarking
var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Benchmark neural performance",
	Long:  `Run performance benchmarks on neural operations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appNeural.NewNeuralService("")
		if err != nil {
			return fmt.Errorf("failed to initialize neural service: %w", err)
		}

		config := neural.BenchmarkConfig{
			Dimension:  benchmarkDim,
			Iterations: benchmarkIterations,
		}

		fmt.Printf("Running benchmarks (dim: %d, iterations: %d)...\n\n", config.Dimension, config.Iterations)

		metrics, err := service.Benchmark(config)
		if err != nil {
			return fmt.Errorf("benchmark failed: %w", err)
		}

		// Table format
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "OPERATION\tITERATIONS\tTOTAL (ms)\tAVG (μs)\tOPS/SEC")
		fmt.Fprintln(w, strings.Repeat("-", 70))
		for _, r := range metrics.Results {
			fmt.Fprintf(w, "%s\t%d\t%.2f\t%.2f\t%.0f\n",
				r.Operation, r.Iterations, r.TotalTimeMs, r.AvgTimeUs, r.OpsPerSecond)
		}
		w.Flush()

		fmt.Printf("\n%s\n", metrics.Summary)

		return nil
	},
}

// statusCmd handles neural system status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show neural system status",
	Long:  `Show the current status of the neural system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appNeural.NewNeuralService("")
		if err != nil {
			return fmt.Errorf("failed to initialize neural service: %w", err)
		}

		status := service.GetStatus()

		fmt.Println("Neural System Status")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("  Initialized:    %v\n", status.Initialized)
		fmt.Printf("  Pattern count:  %d\n", status.PatternCount)
		fmt.Printf("  Total usage:    %d\n", status.TotalUsage)
		fmt.Printf("  Avg confidence: %.2f\n", status.AverageConfidence)
		fmt.Printf("  Storage path:   %s\n", status.StoragePath)
		if status.LastUpdated != "" {
			fmt.Printf("  Last updated:   %s\n", status.LastUpdated)
		}

		if statusVerbose {
			fmt.Println("\nStorage Info:")
			fmt.Printf("  Base path: %s\n", service.GetBasePath())
		}

		return nil
	},
}

func init() {
	// train command flags
	trainCmd.Flags().StringVarP(&trainPattern, "pattern", "p", "coordination", "Pattern type (coordination|optimization|prediction|security|testing)")
	trainCmd.Flags().IntVarP(&trainEpochs, "epochs", "e", 50, "Training epochs")
	trainCmd.Flags().StringVarP(&trainData, "data", "d", "", "Training data file or inline JSON")
	trainCmd.Flags().Float64VarP(&trainLearningRate, "learning-rate", "l", 0.01, "Learning rate")
	trainCmd.Flags().IntVarP(&trainBatchSize, "batch-size", "b", 32, "Batch size")
	trainCmd.Flags().IntVar(&trainDim, "dim", 256, "Embedding dimension (max 256)")

	// learn command flags
	learnCmd.Flags().StringVarP(&learnAgentID, "agent-id", "a", "", "Agent ID to learn from")
	learnCmd.Flags().StringVarP(&learnType, "type", "t", "outcome", "Learning type (outcome|pattern|consolidation)")
	learnCmd.Flags().StringVarP(&learnInput, "input", "i", "", "Input data or file")

	// patterns command flags
	patternsCmd.Flags().StringVarP(&patternsAction, "action", "a", "list", "Action (list|search|analyze)")
	patternsCmd.Flags().StringVarP(&patternsQuery, "query", "q", "", "Search query")
	patternsCmd.Flags().IntVarP(&patternsLimit, "limit", "l", 10, "Max results")
	patternsCmd.Flags().StringVarP(&patternsFormat, "format", "f", "table", "Output format (table|json)")

	// optimize command flags
	optimizeCmd.Flags().StringVar(&optimizeMethod, "method", "quantize", "Method (quantize|analyze|compact)")
	optimizeCmd.Flags().BoolVarP(&optimizeVerbose, "verbose", "v", false, "Show detailed metrics")

	// export command flags
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path")
	exportCmd.Flags().StringVarP(&exportModel, "model", "m", "", "Model ID or category to export")
	exportCmd.Flags().BoolVarP(&exportSign, "sign", "s", true, "Sign with Ed25519")
	exportCmd.Flags().BoolVar(&exportStripPII, "strip-pii", true, "Strip potential PII from export")

	// import command flags
	importCmd.Flags().StringVarP(&importFile, "file", "f", "", "File to import")
	importCmd.Flags().BoolVarP(&importVerify, "verify", "v", true, "Verify Ed25519 signature")
	importCmd.Flags().BoolVar(&importMerge, "merge", true, "Merge with existing patterns")
	importCmd.Flags().StringVar(&importCategory, "category", "", "Only import patterns from specific category")

	// benchmark command flags
	benchmarkCmd.Flags().IntVarP(&benchmarkDim, "dim", "d", 256, "Embedding dimension")
	benchmarkCmd.Flags().IntVarP(&benchmarkIterations, "iterations", "i", 1000, "Number of iterations")

	// status command flags
	statusCmd.Flags().BoolVarP(&statusVerbose, "verbose", "v", false, "Show detailed metrics")

	// Add subcommands to neural
	NeuralCmd.AddCommand(trainCmd)
	NeuralCmd.AddCommand(learnCmd)
	NeuralCmd.AddCommand(patternsCmd)
	NeuralCmd.AddCommand(optimizeCmd)
	NeuralCmd.AddCommand(exportCmd)
	NeuralCmd.AddCommand(importCmd)
	NeuralCmd.AddCommand(benchmarkCmd)
	NeuralCmd.AddCommand(statusCmd)
}
