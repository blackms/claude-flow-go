// Package commands provides CLI command implementations.
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	appHooks "github.com/anthropics/claude-flow-go/internal/application/hooks"
	domainHooks "github.com/anthropics/claude-flow-go/internal/domain/hooks"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Hooks command flags
var (
	// List flags
	hooksListType     string
	hooksListEnabled  bool
	hooksListDisabled bool
	hooksListFormat   string

	// Enable/Disable flags
	hooksEnableAll  bool
	hooksDisableAll bool

	// Config flags
	hooksConfigTimeout      int64
	hooksConfigMaxPatterns  int
	hooksConfigLearningRate float64
	hooksConfigLearning     bool
	hooksConfigNoLearning   bool

	// Stats flags
	hooksStatsHook   string
	hooksStatsPeriod string
	hooksStatsFormat string

	// Test flags
	hooksTestDryRun  bool
	hooksTestInput   string
	hooksTestVerbose bool

	// Reset flags
	hooksResetHook     string
	hooksResetPatterns bool
	hooksResetRouting  bool
	hooksResetAll      bool
	hooksResetConfirm  bool
)

// HooksCmd is the parent command for hooks operations.
var HooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage self-learning hooks system",
	Long: `Commands for managing the self-learning hooks system.

The hooks system provides:
  - Pre/post hooks for file edits, commands, and routing
  - Pattern learning from successful operations
  - Intelligent task routing based on learned patterns
  - Configuration and statistics management`,
}

// hooksListCmd lists all hooks
var hooksListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all registered hooks",
	Long:    `List all registered hooks with optional filtering by type and status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appHooks.NewService("")
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		var hookType shared.HookEvent
		if hooksListType != "" {
			hookType = shared.HookEvent(hooksListType)
		}

		hooks := service.ListHooks(hookType, hooksListEnabled, hooksListDisabled)

		if len(hooks) == 0 {
			fmt.Println("No hooks found")
			return nil
		}

		if hooksListFormat == "json" {
			output, _ := json.MarshalIndent(hooks, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		// Table format
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tTYPE\tPRIORITY\tENABLED\tEXECUTIONS")
		fmt.Fprintln(w, strings.Repeat("-", 80))

		for _, hook := range hooks {
			status := "yes"
			if !hook.Enabled {
				status = "no"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%d\n",
				hook.ID, hook.Name, hook.Type, hook.Priority, status, hook.Stats.ExecutionCount)
		}
		w.Flush()

		fmt.Printf("\nTotal: %d hooks\n", len(hooks))

		return nil
	},
}

// hooksEnableCmd enables a hook
var hooksEnableCmd = &cobra.Command{
	Use:   "enable [hook-id]",
	Short: "Enable a hook",
	Long:  `Enable a specific hook by ID or name, or all hooks with --all.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appHooks.NewService("")
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		if hooksEnableAll {
			if err := service.EnableAllHooks(); err != nil {
				return err
			}
			fmt.Println("All hooks enabled")
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("hook ID or name required (or use --all)")
		}

		hookID := args[0]
		if err := service.EnableHook(hookID); err != nil {
			return err
		}

		fmt.Printf("Hook '%s' enabled\n", hookID)
		return nil
	},
}

// hooksDisableCmd disables a hook
var hooksDisableCmd = &cobra.Command{
	Use:   "disable [hook-id]",
	Short: "Disable a hook",
	Long:  `Disable a specific hook by ID or name, or all hooks with --all.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appHooks.NewService("")
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		if hooksDisableAll {
			if err := service.DisableAllHooks(); err != nil {
				return err
			}
			fmt.Println("All hooks disabled")
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("hook ID or name required (or use --all)")
		}

		hookID := args[0]
		if err := service.DisableHook(hookID); err != nil {
			return err
		}

		fmt.Printf("Hook '%s' disabled\n", hookID)
		return nil
	},
}

// hooksConfigCmd manages hook configuration
var hooksConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage hooks configuration",
	Long:  `Get or set hooks configuration values.`,
}

// hooksConfigGetCmd gets configuration
var hooksConfigGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appHooks.NewService("")
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		config := service.GetConfig()

		fmt.Println("Hooks Configuration:")
		fmt.Printf("  Default Timeout:   %d ms\n", config.DefaultTimeoutMs)
		fmt.Printf("  Max Patterns:      %d\n", config.MaxPatterns)
		fmt.Printf("  Max Hooks/Event:   %d\n", config.MaxHooksPerEvent)
		fmt.Printf("  Learning Rate:     %.2f\n", config.LearningRate)
		fmt.Printf("  Learning Enabled:  %v\n", config.EnableLearning)

		return nil
	},
}

// hooksConfigSetCmd sets configuration
var hooksConfigSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set configuration values",
	Long: `Set hooks configuration values.

Examples:
  hooks config set --timeout 10000
  hooks config set --learning-rate 0.15
  hooks config set --no-learning`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appHooks.NewService("")
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		update := domainHooks.ConfigUpdate{}
		changed := false

		if cmd.Flags().Changed("timeout") {
			update.Timeout = &hooksConfigTimeout
			changed = true
		}
		if cmd.Flags().Changed("max-patterns") {
			update.MaxPatterns = &hooksConfigMaxPatterns
			changed = true
		}
		if cmd.Flags().Changed("learning-rate") {
			if hooksConfigLearningRate < 0 || hooksConfigLearningRate > 1 {
				return fmt.Errorf("learning rate must be between 0 and 1")
			}
			update.LearningRate = &hooksConfigLearningRate
			changed = true
		}
		if cmd.Flags().Changed("learning") {
			update.Learning = &hooksConfigLearning
			changed = true
		}
		if cmd.Flags().Changed("no-learning") {
			noLearning := false
			update.Learning = &noLearning
			changed = true
		}

		if !changed {
			return fmt.Errorf("no configuration values specified")
		}

		if err := service.UpdateConfig(update); err != nil {
			return err
		}

		fmt.Println("Configuration updated")

		// Show new config
		config := service.GetConfig()
		fmt.Printf("\nNew configuration:\n")
		fmt.Printf("  Default Timeout:   %d ms\n", config.DefaultTimeoutMs)
		fmt.Printf("  Max Patterns:      %d\n", config.MaxPatterns)
		fmt.Printf("  Learning Rate:     %.2f\n", config.LearningRate)
		fmt.Printf("  Learning Enabled:  %v\n", config.EnableLearning)

		return nil
	},
}

// hooksStatsCmd shows hook statistics
var hooksStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show hooks statistics",
	Long:  `Display execution statistics for hooks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := appHooks.NewService("")
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		stats, err := service.GetStats(hooksStatsHook)
		if err != nil {
			return err
		}

		if hooksStatsFormat == "json" {
			output, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(output))
			return nil
		}

		fmt.Println("Hooks Statistics")
		fmt.Println(strings.Repeat("=", 50))

		fmt.Printf("\nExecution Summary:\n")
		fmt.Printf("  Total Executions:    %d\n", stats.TotalExecutions)
		fmt.Printf("  Successful:          %d\n", stats.SuccessfulExecutions)
		fmt.Printf("  Failed:              %d\n", stats.FailedExecutions)
		fmt.Printf("  Avg Execution Time:  %.2f ms\n", stats.AvgExecutionMs)

		fmt.Printf("\nPattern Learning:\n")
		fmt.Printf("  Total Patterns:      %d\n", stats.PatternCount)
		fmt.Printf("  Edit Patterns:       %d\n", stats.EditPatterns)
		fmt.Printf("  Command Patterns:    %d\n", stats.CommandPatterns)

		fmt.Printf("\nRouting:\n")
		fmt.Printf("  Total Routings:      %d\n", stats.RoutingCount)
		fmt.Printf("  Success Rate:        %.1f%%\n", stats.RoutingSuccessRate*100)

		if stats.HookStats != nil {
			fmt.Printf("\nHook-Specific Stats:\n")
			fmt.Printf("  Executions:          %d\n", stats.HookStats.ExecutionCount)
			fmt.Printf("  Successes:           %d\n", stats.HookStats.SuccessCount)
			fmt.Printf("  Failures:            %d\n", stats.HookStats.FailureCount)
			fmt.Printf("  Success Rate:        %.1f%%\n", stats.HookStats.GetSuccessRate())
			fmt.Printf("  Avg Latency:         %.2f ms\n", stats.HookStats.AvgLatencyMs)
		}

		if len(stats.HooksByEvent) > 0 {
			fmt.Printf("\nExecutions by Event:\n")
			for event, count := range stats.HooksByEvent {
				fmt.Printf("  %-20s %d\n", event, count)
			}
		}

		return nil
	},
}

// hooksTestCmd tests a hook
var hooksTestCmd = &cobra.Command{
	Use:   "test [hook-id]",
	Short: "Test a hook execution",
	Long: `Test a hook with optional input data.

Examples:
  hooks test pre-edit --dry-run
  hooks test pre-command --input '{"command": "npm install"}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("hook ID or name required")
		}

		service, err := appHooks.NewService("")
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		hookID := args[0]

		// Parse input data
		var inputData map[string]interface{}
		if hooksTestInput != "" {
			if err := json.Unmarshal([]byte(hooksTestInput), &inputData); err != nil {
				return fmt.Errorf("invalid input JSON: %w", err)
			}
		}

		input := domainHooks.TestInput{
			Data:    inputData,
			DryRun:  hooksTestDryRun,
			Verbose: hooksTestVerbose,
		}

		fmt.Printf("Testing hook: %s\n", hookID)
		if hooksTestDryRun {
			fmt.Println("Mode: dry-run")
		} else {
			fmt.Println("Mode: live execution")
		}
		fmt.Println()

		result, err := service.TestHook(hookID, input)
		if err != nil {
			return err
		}

		if result.Success {
			fmt.Println("Result: SUCCESS")
		} else {
			fmt.Println("Result: FAILED")
			if result.Error != "" {
				fmt.Printf("Error: %s\n", result.Error)
			}
		}

		fmt.Printf("Execution Time: %d ms\n", result.ExecutionMs)

		if hooksTestVerbose && len(result.Logs) > 0 {
			fmt.Println("\nLogs:")
			for _, log := range result.Logs {
				fmt.Printf("  %s\n", log)
			}
		}

		if result.Result != nil && hooksTestVerbose {
			fmt.Println("\nResult Data:")
			output, _ := json.MarshalIndent(result.Result, "  ", "  ")
			fmt.Println("  " + string(output))
		}

		return nil
	},
}

// hooksResetCmd resets hook state
var hooksResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset hooks state",
	Long: `Reset hooks state, patterns, or routing history.

Examples:
  hooks reset --patterns          # Clear learned patterns
  hooks reset --routing           # Clear routing history
  hooks reset --hook pre-edit     # Reset specific hook stats
  hooks reset --all --confirm     # Factory reset everything`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !hooksResetPatterns && !hooksResetRouting && !hooksResetAll && hooksResetHook == "" {
			return fmt.Errorf("specify what to reset: --patterns, --routing, --hook, or --all")
		}

		if hooksResetAll && !hooksResetConfirm {
			fmt.Println("WARNING: This will reset all hooks state including patterns and routing history.")
			fmt.Println("Use --confirm to proceed.")
			return nil
		}

		service, err := appHooks.NewService("")
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
		defer service.Close()

		options := domainHooks.ResetOptions{
			HookID:   hooksResetHook,
			Patterns: hooksResetPatterns,
			Routing:  hooksResetRouting,
			All:      hooksResetAll,
		}

		if err := service.Reset(options); err != nil {
			return err
		}

		if hooksResetAll {
			fmt.Println("All hooks state has been reset to defaults")
		} else {
			if hooksResetHook != "" {
				fmt.Printf("Hook '%s' stats have been reset\n", hooksResetHook)
			}
			if hooksResetPatterns {
				fmt.Println("Learned patterns have been cleared")
			}
			if hooksResetRouting {
				fmt.Println("Routing history has been cleared")
			}
		}

		return nil
	},
}

func init() {
	// List command flags
	hooksListCmd.Flags().StringVarP(&hooksListType, "type", "t", "", "Filter by hook type (pre-edit|post-edit|pre-command|post-command|pre-route|post-route)")
	hooksListCmd.Flags().BoolVar(&hooksListEnabled, "enabled", false, "Show only enabled hooks")
	hooksListCmd.Flags().BoolVar(&hooksListDisabled, "disabled", false, "Show only disabled hooks")
	hooksListCmd.Flags().StringVarP(&hooksListFormat, "format", "f", "table", "Output format (table|json)")

	// Enable command flags
	hooksEnableCmd.Flags().BoolVar(&hooksEnableAll, "all", false, "Enable all hooks")

	// Disable command flags
	hooksDisableCmd.Flags().BoolVar(&hooksDisableAll, "all", false, "Disable all hooks")

	// Config set command flags
	hooksConfigSetCmd.Flags().Int64Var(&hooksConfigTimeout, "timeout", 5000, "Default timeout in ms")
	hooksConfigSetCmd.Flags().IntVar(&hooksConfigMaxPatterns, "max-patterns", 10000, "Maximum patterns to store")
	hooksConfigSetCmd.Flags().Float64Var(&hooksConfigLearningRate, "learning-rate", 0.1, "Learning rate (0.0-1.0)")
	hooksConfigSetCmd.Flags().BoolVar(&hooksConfigLearning, "learning", true, "Enable learning")
	hooksConfigSetCmd.Flags().BoolVar(&hooksConfigNoLearning, "no-learning", false, "Disable learning")

	// Stats command flags
	hooksStatsCmd.Flags().StringVarP(&hooksStatsHook, "hook", "H", "", "Stats for specific hook")
	hooksStatsCmd.Flags().StringVar(&hooksStatsPeriod, "period", "all", "Time period (hour|day|week|all)")
	hooksStatsCmd.Flags().StringVarP(&hooksStatsFormat, "format", "f", "table", "Output format (table|json)")

	// Test command flags
	hooksTestCmd.Flags().BoolVar(&hooksTestDryRun, "dry-run", true, "Don't actually execute")
	hooksTestCmd.Flags().StringVarP(&hooksTestInput, "input", "i", "", "JSON input data")
	hooksTestCmd.Flags().BoolVarP(&hooksTestVerbose, "verbose", "v", false, "Show detailed output")

	// Reset command flags
	hooksResetCmd.Flags().StringVarP(&hooksResetHook, "hook", "H", "", "Reset specific hook stats")
	hooksResetCmd.Flags().BoolVar(&hooksResetPatterns, "patterns", false, "Clear learned patterns")
	hooksResetCmd.Flags().BoolVar(&hooksResetRouting, "routing", false, "Clear routing history")
	hooksResetCmd.Flags().BoolVar(&hooksResetAll, "all", false, "Factory reset everything")
	hooksResetCmd.Flags().BoolVar(&hooksResetConfirm, "confirm", false, "Confirm destructive operation")

	// Add config subcommands
	hooksConfigCmd.AddCommand(hooksConfigGetCmd)
	hooksConfigCmd.AddCommand(hooksConfigSetCmd)

	// Add subcommands to hooks
	HooksCmd.AddCommand(hooksListCmd)
	HooksCmd.AddCommand(hooksEnableCmd)
	HooksCmd.AddCommand(hooksDisableCmd)
	HooksCmd.AddCommand(hooksConfigCmd)
	HooksCmd.AddCommand(hooksStatsCmd)
	HooksCmd.AddCommand(hooksTestCmd)
	HooksCmd.AddCommand(hooksResetCmd)
}
