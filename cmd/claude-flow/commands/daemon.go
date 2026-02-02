// Package commands provides CLI command implementations.
package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/anthropics/claude-flow-go/internal/application/utility"
)

// Daemon command flags
var (
	daemonBackground bool
	daemonForeground bool
	daemonLogFile    string
	daemonVerbose    bool
	daemonFollow     bool
	daemonLines      int
)

// DaemonCmd is the parent command for daemon operations.
var DaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background daemon",
	Long: `Commands for managing the Claude Flow background daemon.

The daemon runs background tasks such as:
  - Codebase mapping
  - Memory consolidation
  - Pattern learning
  - Performance optimization`,
}

// daemonStartCmd starts the daemon
var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the background daemon",
	Long:  `Start the Claude Flow background daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := utility.NewDaemonService()
		if err != nil {
			return err
		}

		foreground := daemonForeground || !daemonBackground

		if err := service.Start(foreground, daemonLogFile); err != nil {
			return err
		}

		if !foreground {
			fmt.Println("Daemon started successfully")
		}

		return nil
	},
}

// daemonStopCmd stops the daemon
var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background daemon",
	Long:  `Stop the Claude Flow background daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := utility.NewDaemonService()
		if err != nil {
			return err
		}

		if err := service.Stop(); err != nil {
			return err
		}

		fmt.Println("Daemon stopped")
		return nil
	},
}

// daemonRestartCmd restarts the daemon
var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the background daemon",
	Long:  `Restart the Claude Flow background daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := utility.NewDaemonService()
		if err != nil {
			return err
		}

		if err := service.Restart(); err != nil {
			return err
		}

		fmt.Println("Daemon restarted")
		return nil
	},
}

// daemonStatusCmd shows daemon status
var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long:  `Show the current status of the Claude Flow background daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := utility.NewDaemonService()
		if err != nil {
			return err
		}

		state, err := service.Status()
		if err != nil {
			return err
		}

		if cmd.Flags().Changed("format") {
			output, _ := json.MarshalIndent(state, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Print(utility.FormatStatus(state, daemonVerbose))
		}

		return nil
	},
}

// daemonLogsCmd shows daemon logs
var daemonLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View daemon logs",
	Long:  `View the Claude Flow daemon log file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, err := utility.NewDaemonService()
		if err != nil {
			return err
		}

		if daemonFollow {
			return service.TailLogs(os.Stdout, true)
		}

		lines, err := service.GetLogs(daemonLines)
		if err != nil {
			return err
		}

		if len(lines) == 0 {
			fmt.Println("No log entries found")
			return nil
		}

		for _, line := range lines {
			fmt.Println(line)
		}

		return nil
	},
}

func init() {
	// Start command flags
	daemonStartCmd.Flags().BoolVarP(&daemonBackground, "background", "b", true, "Run in background")
	daemonStartCmd.Flags().BoolVarP(&daemonForeground, "foreground", "f", false, "Run in foreground")
	daemonStartCmd.Flags().StringVar(&daemonLogFile, "log-file", "", "Log file path")

	// Status command flags
	daemonStatusCmd.Flags().BoolVarP(&daemonVerbose, "verbose", "v", false, "Show detailed statistics")
	daemonStatusCmd.Flags().String("format", "text", "Output format (text|json)")

	// Logs command flags
	daemonLogsCmd.Flags().BoolVarP(&daemonFollow, "follow", "f", false, "Follow log output")
	daemonLogsCmd.Flags().IntVarP(&daemonLines, "lines", "n", 50, "Number of lines to show")

	// Add subcommands
	DaemonCmd.AddCommand(daemonStartCmd)
	DaemonCmd.AddCommand(daemonStopCmd)
	DaemonCmd.AddCommand(daemonRestartCmd)
	DaemonCmd.AddCommand(daemonStatusCmd)
	DaemonCmd.AddCommand(daemonLogsCmd)
}
