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

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long:  `Start the MCP server to handle Model Context Protocol requests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Starting Claude Flow MCP Server on %s:%d...\n", serveHost, servePort)

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

// ============================================================================
// Hive Mind Commands
// ============================================================================

var hiveMindCmd = &cobra.Command{
	Use:   "hive-mind",
	Short: "Hive Mind consensus system commands",
	Long: `Commands for managing the Hive Mind multi-agent consensus system.

The Hive Mind system provides Queen-led consensus coordination across
the 15-agent domain architecture with 5 consensus types:
  - majority (>50%)
  - supermajority (>=67%)
  - unanimous (100%)
  - weighted (performance-based)
  - queen-override (emergency)`,
}

var hiveMindAlgorithm string

var hiveMindInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the Hive Mind system",
	Long:  `Initialize the Hive Mind with the specified consensus algorithm.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Initializing Hive Mind with consensus algorithm: %s\n", hiveMindAlgorithm)

		// Create coordinator
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyHierarchical,
		})
		if err != nil {
			return fmt.Errorf("failed to create coordinator: %w", err)
		}
		defer coordinator.Shutdown()

		// Create Queen Coordinator
		queen, err := claudeflow.NewQueenCoordinator(coordinator, claudeflow.DefaultQueenConfig())
		if err != nil {
			return fmt.Errorf("failed to create queen coordinator: %w", err)
		}
		queen.Start()
		defer queen.Stop()

		// Create Hive Mind Manager
		config := claudeflow.DefaultHiveMindConfig()
		config.ConsensusAlgorithm = claudeflow.ConsensusType(hiveMindAlgorithm)

		hm, err := claudeflow.NewHiveMindManager(queen, config)
		if err != nil {
			return fmt.Errorf("failed to create hive mind: %w", err)
		}

		if err := hm.Initialize(context.Background()); err != nil {
			return fmt.Errorf("failed to initialize hive mind: %w", err)
		}

		state := hm.GetState()
		output, _ := json.MarshalIndent(state, "", "  ")
		fmt.Println("Hive Mind initialized:")
		fmt.Println(string(output))

		return nil
	},
}

var hiveMindSpawnCmd = &cobra.Command{
	Use:   "spawn",
	Short: "Spawn the full 15-agent hierarchy",
	Long:  `Spawn the complete 15-agent domain hierarchy with Queen coordination.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Spawning 15-agent hierarchy...")

		// Create coordinator
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyHierarchical,
		})
		if err != nil {
			return fmt.Errorf("failed to create coordinator: %w", err)
		}
		defer coordinator.Shutdown()

		// Create Queen Coordinator
		queen, err := claudeflow.NewQueenCoordinator(coordinator, claudeflow.DefaultQueenConfig())
		if err != nil {
			return fmt.Errorf("failed to create queen coordinator: %w", err)
		}
		queen.Start()
		defer queen.Stop()

		// Spawn full hierarchy
		if err := queen.SpawnFullHierarchy(context.Background()); err != nil {
			return fmt.Errorf("failed to spawn hierarchy: %w", err)
		}

		// Show domain health
		health := queen.GetDomainHealth()
		fmt.Printf("\nSpawned 15 agents across %d domains:\n", len(health))
		for domain, h := range health {
			fmt.Printf("  %s: %d agents (health: %.2f)\n", domain, h.TotalAgents, h.HealthScore)
		}

		return nil
	},
}

var hiveMindStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Hive Mind status",
	Long:  `Show the current status of the Hive Mind system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create coordinator
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyHierarchical,
		})
		if err != nil {
			return fmt.Errorf("failed to create coordinator: %w", err)
		}
		defer coordinator.Shutdown()

		// Create Queen Coordinator
		queen, err := claudeflow.NewQueenCoordinator(coordinator, claudeflow.DefaultQueenConfig())
		if err != nil {
			return fmt.Errorf("failed to create queen coordinator: %w", err)
		}
		queen.Start()
		defer queen.Stop()

		// Create Hive Mind Manager
		hm, err := claudeflow.NewHiveMindManager(queen, claudeflow.DefaultHiveMindConfig())
		if err != nil {
			return fmt.Errorf("failed to create hive mind: %w", err)
		}

		state := hm.GetState()
		output, _ := json.MarshalIndent(state, "", "  ")
		fmt.Println(string(output))

		return nil
	},
}

var hiveMindTaskDescription string
var hiveMindTaskType string

var hiveMindTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Submit a task for consensus",
	Long:  `Submit a task that requires consensus from the Hive Mind.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if hiveMindTaskDescription == "" {
			return fmt.Errorf("task description is required")
		}

		fmt.Printf("Submitting task for consensus: %s\n", hiveMindTaskDescription)

		// Create coordinator
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyHierarchical,
		})
		if err != nil {
			return fmt.Errorf("failed to create coordinator: %w", err)
		}
		defer coordinator.Shutdown()

		// Create Queen Coordinator and spawn hierarchy
		queen, err := claudeflow.NewQueenCoordinator(coordinator, claudeflow.DefaultQueenConfig())
		if err != nil {
			return fmt.Errorf("failed to create queen coordinator: %w", err)
		}
		queen.Start()
		defer queen.Stop()

		if err := queen.SpawnFullHierarchy(context.Background()); err != nil {
			return fmt.Errorf("failed to spawn hierarchy: %w", err)
		}

		// Create Hive Mind Manager
		hm, err := claudeflow.NewHiveMindManager(queen, claudeflow.DefaultHiveMindConfig())
		if err != nil {
			return fmt.Errorf("failed to create hive mind: %w", err)
		}
		if err := hm.Initialize(context.Background()); err != nil {
			return err
		}
		defer hm.Shutdown()

		// Create proposal
		proposal := claudeflow.Proposal{
			Type:        hiveMindTaskType,
			Title:       "Task Proposal",
			Description: hiveMindTaskDescription,
			Priority:    claudeflow.PriorityMedium,
		}

		created, err := hm.CreateProposal(context.Background(), proposal)
		if err != nil {
			return fmt.Errorf("failed to create proposal: %w", err)
		}

		fmt.Printf("Created proposal: %s\n", created.ID)

		// Collect votes synchronously
		result, err := hm.CollectVotesSync(context.Background(), created.ID)
		if err != nil {
			return fmt.Errorf("failed to collect votes: %w", err)
		}

		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println("\nConsensus Result:")
		fmt.Println(string(output))

		return nil
	},
}

var consensusProposalTitle string
var consensusProposalType string
var consensusAlgorithm string

var hiveMindConsensusCmd = &cobra.Command{
	Use:   "consensus",
	Short: "Create and manage consensus proposals",
	Long:  `Create consensus proposals and manage voting in the Hive Mind.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if consensusProposalTitle == "" {
			return fmt.Errorf("proposal title is required")
		}

		fmt.Printf("Creating consensus proposal: %s\n", consensusProposalTitle)

		// Create coordinator
		coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
			Topology: claudeflow.TopologyHierarchical,
		})
		if err != nil {
			return fmt.Errorf("failed to create coordinator: %w", err)
		}
		defer coordinator.Shutdown()

		// Create Queen Coordinator and spawn hierarchy
		queen, err := claudeflow.NewQueenCoordinator(coordinator, claudeflow.DefaultQueenConfig())
		if err != nil {
			return fmt.Errorf("failed to create queen coordinator: %w", err)
		}
		queen.Start()
		defer queen.Stop()

		if err := queen.SpawnFullHierarchy(context.Background()); err != nil {
			return fmt.Errorf("failed to spawn hierarchy: %w", err)
		}

		// Create Hive Mind Manager
		config := claudeflow.DefaultHiveMindConfig()
		config.ConsensusAlgorithm = claudeflow.ConsensusType(consensusAlgorithm)

		hm, err := claudeflow.NewHiveMindManager(queen, config)
		if err != nil {
			return fmt.Errorf("failed to create hive mind: %w", err)
		}
		if err := hm.Initialize(context.Background()); err != nil {
			return err
		}
		defer hm.Shutdown()

		// Create proposal
		proposal := claudeflow.Proposal{
			Type:         consensusProposalType,
			Title:        consensusProposalTitle,
			Description:  consensusProposalTitle,
			RequiredType: claudeflow.ConsensusType(consensusAlgorithm),
		}

		created, err := hm.CreateProposal(context.Background(), proposal)
		if err != nil {
			return fmt.Errorf("failed to create proposal: %w", err)
		}

		fmt.Printf("Created proposal: %s (algorithm: %s)\n", created.ID, created.RequiredType)

		// Collect votes
		result, err := hm.CollectVotesSync(context.Background(), created.ID)
		if err != nil {
			return fmt.Errorf("failed to collect votes: %w", err)
		}

		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println("\nConsensus Result:")
		fmt.Println(string(output))

		if result.ConsensusReached {
			fmt.Println("\n✓ Consensus reached!")
		} else {
			fmt.Println("\n✗ Consensus not reached")
		}

		return nil
	},
}

func init() {
	// Serve command
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 3000, "Port to listen on")
	serveCmd.Flags().StringVarP(&serveHost, "host", "H", "localhost", "Host to listen on")
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

	// Hive Mind commands
	hiveMindInitCmd.Flags().StringVarP(&hiveMindAlgorithm, "algorithm", "a", "majority", "Consensus algorithm (majority, supermajority, unanimous, weighted, queen-override)")
	hiveMindCmd.AddCommand(hiveMindInitCmd)
	hiveMindCmd.AddCommand(hiveMindSpawnCmd)
	hiveMindCmd.AddCommand(hiveMindStatusCmd)
	
	hiveMindTaskCmd.Flags().StringVarP(&hiveMindTaskDescription, "description", "d", "", "Task description (required)")
	hiveMindTaskCmd.Flags().StringVarP(&hiveMindTaskType, "type", "t", "task", "Task type")
	hiveMindTaskCmd.MarkFlagRequired("description")
	hiveMindCmd.AddCommand(hiveMindTaskCmd)

	hiveMindConsensusCmd.Flags().StringVarP(&consensusProposalTitle, "title", "t", "", "Proposal title (required)")
	hiveMindConsensusCmd.Flags().StringVarP(&consensusProposalType, "type", "T", "general", "Proposal type")
	hiveMindConsensusCmd.Flags().StringVarP(&consensusAlgorithm, "algorithm", "a", "majority", "Consensus algorithm")
	hiveMindConsensusCmd.MarkFlagRequired("title")
	hiveMindCmd.AddCommand(hiveMindConsensusCmd)

	rootCmd.AddCommand(hiveMindCmd)

	// Neural commands
	rootCmd.AddCommand(commands.NeuralCmd)

	// RuVector commands
	rootCmd.AddCommand(commands.RuVectorCmd)
}
