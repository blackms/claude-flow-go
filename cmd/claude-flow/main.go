// Package main provides the CLI entry point for claude-flow-go.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/anthropics/claude-flow-go/cmd/claude-flow/commands"
	claudeflow "github.com/anthropics/claude-flow-go/pkg/claude-flow"
)

var (
	version = "3.0.0-alpha.1"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "claude-flow",
	Short: "Claude Flow - Multi-agent orchestration framework",
	Long: `Claude Flow is a production-ready enterprise AI orchestration platform
for deploying coordinated multi-agent systems.

It provides:
  - Multi-agent coordination with swarm topologies
  - Workflow execution with dependency resolution
  - Memory backends with SQLite and vector search
  - MCP Server for Model Context Protocol integration
  - Plugin system for extensibility`,
	Version: version,
}

// ============================================================================
// Serve Command
// ============================================================================

var servePort int
var serveHost string
var serveStdio bool

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long:  `Start the MCP server to handle Model Context Protocol requests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create coordinator
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyMesh,
		})
		if err != nil {
			return fmt.Errorf("failed to create coordinator: %w", err)
		}
		defer coordinator.Shutdown()

		// Create memory backend
		memory, err := claudeflow.NewMemoryBackend(claudeflow.MemoryBackendConfig{
			SQLitePath: ":memory:",
			Dimensions: 768,
		})
		if err != nil {
			return fmt.Errorf("failed to create memory backend: %w", err)
		}

		// Create MCP server
		server := claudeflow.NewMCPServer(claudeflow.MCPServerConfig{
			Port:        servePort,
			Host:        serveHost,
			Coordinator: coordinator,
			Memory:      memory,
		})

		// Stdio mode for Claude Code / Claude Desktop integration
		if serveStdio {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				cancel()
			}()

			return server.RunStdio(ctx, os.Stdin, os.Stdout)
		}

		// HTTP mode
		fmt.Printf("Starting Claude Flow MCP Server on %s:%d...\n", serveHost, servePort)

		if err := server.Start(); err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}

		fmt.Printf("MCP Server running on http://%s:%d\n", serveHost, servePort)
		fmt.Println("Press Ctrl+C to stop")

		// Wait for interrupt
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\nShutting down...")
		return server.Stop()
	},
}

// ============================================================================
// Agent Commands
// ============================================================================

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent management commands",
	Long:  `Commands for managing agents in the swarm.`,
}

var agentSpawnID string
var agentSpawnType string
var agentSpawnCaps []string

var agentSpawnCmd = &cobra.Command{
	Use:   "spawn",
	Short: "Spawn a new agent",
	Long:  `Spawn a new agent in the swarm with the specified type and capabilities.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyMesh,
		})
		if err != nil {
			return err
		}
		defer coordinator.Shutdown()

		caps := agentSpawnCaps
		if len(caps) == 0 {
			caps = claudeflow.GetDefaultCapabilities(claudeflow.AgentType(agentSpawnType))
		}

		agent, err := coordinator.SpawnAgent(claudeflow.AgentConfig{
			ID:           agentSpawnID,
			Type:         claudeflow.AgentType(agentSpawnType),
			Capabilities: caps,
		})
		if err != nil {
			return err
		}

		output, _ := json.MarshalIndent(agent, "", "  ")
		fmt.Println(string(output))
		return nil
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents",
	Long:  `List all agents currently in the swarm.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyMesh,
		})
		if err != nil {
			return err
		}
		defer coordinator.Shutdown()

		agents := coordinator.ListAgents()

		if len(agents) == 0 {
			fmt.Println("No agents in swarm")
			return nil
		}

		output, _ := json.MarshalIndent(agents, "", "  ")
		fmt.Println(string(output))
		return nil
	},
}

// ============================================================================
// Workflow Commands
// ============================================================================

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Workflow management commands",
	Long:  `Commands for managing and executing workflows.`,
}

var workflowRunFile string

var workflowRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a workflow",
	Long:  `Run a workflow from a definition file or inline.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyMesh,
		})
		if err != nil {
			return err
		}
		defer coordinator.Shutdown()

		engine, err := claudeflow.NewWorkflowEngine(claudeflow.WorkflowEngineConfig{
			Coordinator: coordinator,
		})
		if err != nil {
			return err
		}
		defer engine.Shutdown()

		// Spawn a default agent for the workflow
		_, err = coordinator.SpawnAgent(claudeflow.AgentConfig{
			ID:           "workflow-agent",
			Type:         claudeflow.AgentTypeCoder,
			Capabilities: claudeflow.GetDefaultCapabilities(claudeflow.AgentTypeCoder),
		})
		if err != nil {
			return err
		}

		// Create a sample workflow if no file provided
		workflow := claudeflow.WorkflowDefinition{
			ID:   "sample-workflow",
			Name: "Sample Workflow",
			Tasks: []claudeflow.Task{
				{
					ID:          "task-1",
					Type:        claudeflow.TaskTypeCode,
					Description: "First task",
					Priority:    claudeflow.PriorityHigh,
				},
				{
					ID:           "task-2",
					Type:         claudeflow.TaskTypeTest,
					Description:  "Second task",
					Priority:     claudeflow.PriorityMedium,
					Dependencies: []string{"task-1"},
				},
			},
		}

		fmt.Printf("Executing workflow: %s\n", workflow.Name)

		result, err := engine.ExecuteWorkflow(context.Background(), workflow)
		if err != nil {
			return err
		}

		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
		return nil
	},
}

// ============================================================================
// Memory Commands
// ============================================================================

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Memory management commands",
	Long:  `Commands for managing the memory backend.`,
}

var memoryStoreAgentID string
var memoryStoreContent string
var memoryStoreType string

var memoryStoreCmd = &cobra.Command{
	Use:   "store",
	Short: "Store a memory entry",
	Long:  `Store a new memory entry in the backend.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		backend, err := claudeflow.NewMemoryBackend(claudeflow.MemoryBackendConfig{
			SQLitePath: ":memory:",
		})
		if err != nil {
			return err
		}

		memory := claudeflow.Memory{
			ID:        fmt.Sprintf("mem-%d", claudeflow.Now()),
			AgentID:   memoryStoreAgentID,
			Content:   memoryStoreContent,
			Type:      claudeflow.MemoryType(memoryStoreType),
			Timestamp: claudeflow.Now(),
		}

		result, err := backend.Store(memory)
		if err != nil {
			return err
		}

		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
		return nil
	},
}

var memoryQueryAgentID string
var memoryQueryType string
var memoryQueryLimit int

var memoryQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query memory entries",
	Long:  `Query memory entries with optional filters.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		backend, err := claudeflow.NewMemoryBackend(claudeflow.MemoryBackendConfig{
			SQLitePath: ":memory:",
		})
		if err != nil {
			return err
		}

		query := claudeflow.MemoryQuery{
			AgentID: memoryQueryAgentID,
			Type:    claudeflow.MemoryType(memoryQueryType),
			Limit:   memoryQueryLimit,
		}

		memories, err := backend.Query(query)
		if err != nil {
			return err
		}

		if len(memories) == 0 {
			fmt.Println("No memories found")
			return nil
		}

		output, _ := json.MarshalIndent(memories, "", "  ")
		fmt.Println(string(output))
		return nil
	},
}

// ============================================================================
// Status Command
// ============================================================================

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system status",
	Long:  `Show the current status of the Claude Flow system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyMesh,
		})
		if err != nil {
			return err
		}
		defer coordinator.Shutdown()

		state := coordinator.GetSwarmState()

		status := map[string]interface{}{
			"version":           version,
			"topology":          state.Topology,
			"agentCount":        len(state.Agents),
			"leader":            state.Leader,
			"activeConnections": state.ActiveConnections,
		}

		output, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(output))
		return nil
	},
}

// Note: Hive Mind commands have been moved to commands/hivemind.go
// with comprehensive support for init, spawn, status, task, join, leave,
// consensus, broadcast, memory, optimize-memory, and shutdown.

func init() {
	// Serve command
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 3000, "Port to listen on")
	serveCmd.Flags().StringVarP(&serveHost, "host", "H", "localhost", "Host to listen on")
	serveCmd.Flags().BoolVar(&serveStdio, "stdio", false, "Use stdio transport (for Claude Code/Desktop integration)")
	rootCmd.AddCommand(serveCmd)

	// Agent commands
	agentSpawnCmd.Flags().StringVarP(&agentSpawnID, "id", "i", "", "Agent ID (required)")
	agentSpawnCmd.Flags().StringVarP(&agentSpawnType, "type", "t", "coder", "Agent type")
	agentSpawnCmd.Flags().StringSliceVarP(&agentSpawnCaps, "capabilities", "c", []string{}, "Agent capabilities")
	agentSpawnCmd.MarkFlagRequired("id")
	agentCmd.AddCommand(agentSpawnCmd)
	agentCmd.AddCommand(agentListCmd)
	rootCmd.AddCommand(agentCmd)

	// Workflow commands
	workflowRunCmd.Flags().StringVarP(&workflowRunFile, "file", "f", "", "Workflow definition file")
	workflowCmd.AddCommand(workflowRunCmd)
	rootCmd.AddCommand(workflowCmd)

	// Memory commands
	memoryStoreCmd.Flags().StringVarP(&memoryStoreAgentID, "agent-id", "a", "", "Agent ID")
	memoryStoreCmd.Flags().StringVarP(&memoryStoreContent, "content", "c", "", "Memory content")
	memoryStoreCmd.Flags().StringVarP(&memoryStoreType, "type", "t", "context", "Memory type")
	memoryQueryCmd.Flags().StringVarP(&memoryQueryAgentID, "agent-id", "a", "", "Filter by agent ID")
	memoryQueryCmd.Flags().StringVarP(&memoryQueryType, "type", "t", "", "Filter by type")
	memoryQueryCmd.Flags().IntVarP(&memoryQueryLimit, "limit", "l", 10, "Maximum results")
	memoryCmd.AddCommand(memoryStoreCmd)
	memoryCmd.AddCommand(memoryQueryCmd)
	rootCmd.AddCommand(memoryCmd)

	// Status command
	rootCmd.AddCommand(statusCmd)

	// Hive Mind commands (from commands package)
	rootCmd.AddCommand(commands.HiveMindCmd)

	// Neural commands
	rootCmd.AddCommand(commands.NeuralCmd)

	// RuVector commands
	rootCmd.AddCommand(commands.RuVectorCmd)

	// Store commands
	rootCmd.AddCommand(commands.StoreCmd)

	// Hooks commands
	rootCmd.AddCommand(commands.HooksCmd)

	// Utility commands
	rootCmd.AddCommand(commands.DoctorCmd)
	rootCmd.AddCommand(commands.DaemonCmd)
	rootCmd.AddCommand(commands.BenchmarkCmd)
}
