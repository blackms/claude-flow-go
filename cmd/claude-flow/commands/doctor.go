// Package commands provides CLI command implementations.
package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/anthropics/claude-flow-go/internal/application/utility"
)

// Doctor command flags
var (
	doctorFix       bool
	doctorComponent string
	doctorVerbose   bool
	doctorFormat    string
)

// Version is set at build time
var Version = "3.0.0-alpha.1"

// DoctorCmd is the doctor command for system diagnostics.
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run system diagnostics",
	Long: `Run system diagnostics to check the health of your Claude Flow installation.

The doctor command checks:
  - Version and updates
  - Go runtime version
  - Configuration file validity
  - Daemon status
  - Memory database
  - Disk space
  - Environment variables (API keys)
  - Git installation

Use --fix to see suggested fixes for any issues found.
Use --component to check a specific component only.`,
	Example: `  # Run all diagnostic checks
  claude-flow doctor

  # Show fix suggestions
  claude-flow doctor --fix

  # Check specific component
  claude-flow doctor --component config

  # Output as JSON
  claude-flow doctor --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service := utility.NewDoctorService(Version)

		if doctorComponent != "" {
			// Run single check
			result, err := service.RunCheck(doctorComponent)
			if err != nil {
				return err
			}

			if doctorFormat == "json" {
				output, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(output))
			} else {
				icon := getCheckIcon(result.Status)
				fmt.Printf("%s %s: %s\n", icon, result.Name, result.Message)
				if doctorFix && result.Fix != "" && result.Status != utility.CheckStatusPass {
					fmt.Printf("  Fix: %s\n", result.Fix)
				}
			}

			return nil
		}

		// Run all checks
		report := service.RunAllChecks(doctorVerbose)

		if doctorFormat == "json" {
			output, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Print(utility.FormatReport(report, doctorFix))
		}

		// Return error if any checks failed
		if report.Summary.Failed > 0 {
			return fmt.Errorf("%d check(s) failed", report.Summary.Failed)
		}

		return nil
	},
}

// getCheckIcon returns an icon for the check status.
func getCheckIcon(status utility.CheckStatus) string {
	switch status {
	case utility.CheckStatusPass:
		return "[OK]"
	case utility.CheckStatusWarn:
		return "[WARN]"
	case utility.CheckStatusFail:
		return "[FAIL]"
	default:
		return "[?]"
	}
}

func init() {
	DoctorCmd.Flags().BoolVarP(&doctorFix, "fix", "f", false, "Show fix commands for issues")
	DoctorCmd.Flags().StringVarP(&doctorComponent, "component", "c", "", "Check specific component (version|go|config|daemon|memory|disk|env|git)")
	DoctorCmd.Flags().BoolVarP(&doctorVerbose, "verbose", "v", false, "Verbose output")
	DoctorCmd.Flags().StringVar(&doctorFormat, "format", "text", "Output format (text|json)")
}
