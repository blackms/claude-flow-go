// Package commands provides CLI command implementations.
package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/anthropics/claude-flow-go/internal/application/utility"
)

// Benchmark command flags
var (
	benchmarkIterations int
	benchmarkWarmup     int
	benchmarkOutput     string
	benchmarkSave       bool
	benchmarkVerbose    bool
)

// BenchmarkCmd is the parent command for benchmark operations.
var BenchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run performance benchmarks",
	Long: `Commands for running performance benchmarks.

Available benchmark suites:
  - neural: Embedding generation, similarity search
  - memory: Store, query, vector search
  - cli: Cold start time
  - all: Run all benchmark suites

Target thresholds:
  - CLI cold start: < 500ms
  - Vector search: < 1ms
  - Memory write: < 5ms
  - Embedding generation: < 5ms`,
}

// benchmarkNeuralCmd runs neural benchmarks
var benchmarkNeuralCmd = &cobra.Command{
	Use:   "neural",
	Short: "Run neural operation benchmarks",
	Long:  `Benchmark neural operations including embedding generation and similarity search.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config := utility.BenchmarkConfig{
			Iterations: benchmarkIterations,
			Warmup:     benchmarkWarmup,
			Verbose:    benchmarkVerbose,
		}

		service, err := utility.NewBenchmarkService(config)
		if err != nil {
			return err
		}

		fmt.Println("Running neural benchmarks...")

		report, err := service.RunNeuralBenchmarks()
		if err != nil {
			return err
		}

		return outputBenchmarkReport(service, report)
	},
}

// benchmarkMemoryCmd runs memory benchmarks
var benchmarkMemoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Run memory operation benchmarks",
	Long:  `Benchmark memory operations including store, query, and vector search.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config := utility.BenchmarkConfig{
			Iterations: benchmarkIterations,
			Warmup:     benchmarkWarmup,
			Verbose:    benchmarkVerbose,
		}

		service, err := utility.NewBenchmarkService(config)
		if err != nil {
			return err
		}

		fmt.Println("Running memory benchmarks...")

		report, err := service.RunMemoryBenchmarks()
		if err != nil {
			return err
		}

		return outputBenchmarkReport(service, report)
	},
}

// benchmarkCLICmd runs CLI benchmarks
var benchmarkCLICmd = &cobra.Command{
	Use:   "cli",
	Short: "Run CLI benchmarks",
	Long:  `Benchmark CLI operations including cold start time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config := utility.BenchmarkConfig{
			Iterations: benchmarkIterations,
			Warmup:     benchmarkWarmup,
			Verbose:    benchmarkVerbose,
		}

		service, err := utility.NewBenchmarkService(config)
		if err != nil {
			return err
		}

		fmt.Println("Running CLI benchmarks...")

		report, err := service.RunCLIBenchmarks()
		if err != nil {
			return err
		}

		return outputBenchmarkReport(service, report)
	},
}

// benchmarkAllCmd runs all benchmarks
var benchmarkAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Run all benchmark suites",
	Long:  `Run all available benchmark suites (neural, memory, cli).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config := utility.BenchmarkConfig{
			Iterations: benchmarkIterations,
			Warmup:     benchmarkWarmup,
			Verbose:    benchmarkVerbose,
		}

		service, err := utility.NewBenchmarkService(config)
		if err != nil {
			return err
		}

		fmt.Println("Running all benchmarks...")

		report, err := service.RunAllBenchmarks()
		if err != nil {
			return err
		}

		return outputBenchmarkReport(service, report)
	},
}

// outputBenchmarkReport outputs the benchmark report in the requested format.
func outputBenchmarkReport(service *utility.BenchmarkService, report *utility.BenchmarkReport) error {
	if benchmarkOutput == "json" {
		output, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(output))
	} else {
		fmt.Println()
		fmt.Print(utility.FormatBenchmarkReport(report))
	}

	if benchmarkSave {
		path, err := service.SaveReport(report)
		if err != nil {
			return fmt.Errorf("failed to save report: %w", err)
		}
		fmt.Printf("\nReport saved to: %s\n", path)
	}

	// Return error if any benchmarks failed
	if report.Summary.Failed > 0 {
		return fmt.Errorf("%d benchmark(s) failed to meet target", report.Summary.Failed)
	}

	return nil
}

func init() {
	// Common flags for all benchmark commands
	addBenchmarkFlags := func(cmd *cobra.Command) {
		cmd.Flags().IntVarP(&benchmarkIterations, "iterations", "i", 100, "Number of benchmark iterations")
		cmd.Flags().IntVarP(&benchmarkWarmup, "warmup", "w", 10, "Number of warmup iterations")
		cmd.Flags().StringVarP(&benchmarkOutput, "output", "o", "text", "Output format (text|json)")
		cmd.Flags().BoolVarP(&benchmarkSave, "save", "s", false, "Save results to file")
		cmd.Flags().BoolVarP(&benchmarkVerbose, "verbose", "v", false, "Verbose output")
	}

	addBenchmarkFlags(benchmarkNeuralCmd)
	addBenchmarkFlags(benchmarkMemoryCmd)
	addBenchmarkFlags(benchmarkCLICmd)
	addBenchmarkFlags(benchmarkAllCmd)

	// Add subcommands
	BenchmarkCmd.AddCommand(benchmarkNeuralCmd)
	BenchmarkCmd.AddCommand(benchmarkMemoryCmd)
	BenchmarkCmd.AddCommand(benchmarkCLICmd)
	BenchmarkCmd.AddCommand(benchmarkAllCmd)
}
