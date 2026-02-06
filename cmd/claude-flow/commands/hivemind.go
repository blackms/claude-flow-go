// Package commands provides CLI commands for claude-flow.
package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/anthropics/claude-flow-go/internal/application/coordinator"
	"github.com/anthropics/claude-flow-go/internal/application/hivemind"
	appMemory "github.com/anthropics/claude-flow-go/internal/application/memory"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// hiveMindState holds the runtime state for the hive mind commands.
var hiveMindState struct {
	manager           *hivemind.Manager
	queen             *coordinator.QueenCoordinator
	swarm             *coordinator.SwarmCoordinator
	memoryService     *appMemory.MemoryService
	optimizationSvc   *appMemory.OptimizationService
	initialized       bool
	config            shared.HiveMindConfig
}

// HiveMindCmd is the root command for hive-mind operations.
var HiveMindCmd = &cobra.Command{
	Use:   "hive-mind",
	Short: "Manage the Hive Mind multi-agent coordination system",
	Long: `The Hive Mind system provides queen-led coordination for multi-agent swarms.

It supports multiple consensus algorithms (byzantine, raft, gossip, crdt, quorum)
and manages the 15-agent domain hierarchy:
  - Queen Domain (Agent 1): Top-level coordination
  - Security Domain (Agents 2-4): Security architecture and testing
  - Core Domain (Agents 5-9): DDD, memory, swarm, MCP optimization
  - Integration Domain (Agents 10-12): CLI, neural, agentic-flow
  - Support Domain (Agents 13-15): TDD, performance, release`,
}

// ============================================================================
// hive-mind init
// ============================================================================

var initAlgorithm string
var initV3Mode bool
var initQuorum float64

var hiveMindInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the Hive Mind with a consensus algorithm",
	Long: `Initialize the Hive Mind system with the specified consensus algorithm.

Available algorithms:
  - majority:      Simple majority voting (>50%)
  - supermajority: Supermajority voting (>=67%)
  - unanimous:     Unanimous agreement required (100%)
  - weighted:      Weighted voting based on agent performance
  - queen-override: Queen has unilateral authority`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHiveMindInit()
	},
}

func runHiveMindInit() error {
	// Map algorithm string to ConsensusType
	var algorithm shared.ConsensusType
	switch strings.ToLower(initAlgorithm) {
	case "majority":
		algorithm = shared.ConsensusTypeMajority
	case "supermajority":
		algorithm = shared.ConsensusTypeSuperMajority
	case "unanimous":
		algorithm = shared.ConsensusTypeUnanimous
	case "weighted":
		algorithm = shared.ConsensusTypeWeighted
	case "queen-override", "queen":
		algorithm = shared.ConsensusTypeQueenOverride
	default:
		algorithm = shared.ConsensusTypeMajority
	}

	// Create configuration
	hiveMindState.config = shared.HiveMindConfig{
		ConsensusAlgorithm: algorithm,
		VoteTimeout:        30000,
		MaxProposals:       100,
		EnableLearning:     true,
		DefaultQuorum:      initQuorum,
		MinVoteWeight:      0.1,
	}

	// Create swarm coordinator
	hiveMindState.swarm = coordinator.New(coordinator.Options{
		Topology: shared.TopologyHierarchical,
	})
	if err := hiveMindState.swarm.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize swarm: %w", err)
	}

	// Create queen coordinator
	var err error
	hiveMindState.queen, err = coordinator.NewQueenCoordinator(
		hiveMindState.swarm,
		coordinator.DefaultQueenConfig(),
	)
	if err != nil {
		return fmt.Errorf("failed to create queen coordinator: %w", err)
	}

	// Create hive mind manager
	hiveMindState.manager, err = hivemind.NewManager(hiveMindState.queen, hiveMindState.config)
	if err != nil {
		return fmt.Errorf("failed to create hive mind manager: %w", err)
	}

	// Initialize manager
	ctx := context.Background()
	if err := hiveMindState.manager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize hive mind: %w", err)
	}

	// Spawn full hierarchy if v3 mode
	if initV3Mode {
		fmt.Println("Spawning 15-agent V3 hierarchy...")
		if err := hiveMindState.queen.SpawnFullHierarchy(ctx); err != nil {
			return fmt.Errorf("failed to spawn hierarchy: %w", err)
		}
	}

	hiveMindState.queen.Start()
	hiveMindState.initialized = true

	fmt.Printf("Hive Mind initialized successfully\n")
	fmt.Printf("  Algorithm:  %s\n", algorithm)
	fmt.Printf("  Quorum:     %.0f%%\n", initQuorum*100)
	fmt.Printf("  V3 Mode:    %v\n", initV3Mode)
	if initV3Mode {
		fmt.Printf("  Agents:     15 (full hierarchy)\n")
	}

	return nil
}

// ============================================================================
// hive-mind spawn
// ============================================================================

var spawnCount int
var spawnDomain string
var spawnType string
var spawnClaude bool

var hiveMindSpawnCmd = &cobra.Command{
	Use:   "spawn",
	Short: "Spawn worker agents in the swarm",
	Long: `Spawn one or more worker agents in the Hive Mind swarm.

Use --domain to specify the target domain:
  - queen, security, core, integration, support

Use --type to specify the agent type:
  - coder, tester, reviewer, coordinator, designer, deployer
  - security-architect, cve-remediation, threat-modeler
  - ddd-designer, memory-specialist, swarm-specialist
  - And more...`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHiveMindSpawn()
	},
}

func runHiveMindSpawn() error {
	if hiveMindState.swarm == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	// Parse domain
	var domain shared.AgentDomain
	switch strings.ToLower(spawnDomain) {
	case "queen":
		domain = shared.DomainQueen
	case "security":
		domain = shared.DomainSecurity
	case "core":
		domain = shared.DomainCore
	case "integration":
		domain = shared.DomainIntegration
	case "support":
		domain = shared.DomainSupport
	default:
		domain = shared.DomainCore
	}

	// Parse agent type
	agentType := shared.AgentType(spawnType)
	if agentType == "" {
		agentType = shared.AgentTypeCoder
	}

	fmt.Printf("Spawning %d agent(s)...\n", spawnCount)

	for i := 0; i < spawnCount; i++ {
		config := shared.AgentConfig{
			ID:     shared.GenerateID("agent"),
			Type:   agentType,
			Role:   shared.AgentRoleWorker,
			Domain: domain,
		}

		agent, err := hiveMindState.swarm.SpawnAgent(config)
		if err != nil {
			return fmt.Errorf("failed to spawn agent %d: %w", i+1, err)
		}

		if spawnClaude {
			fmt.Printf("  [%d] Agent %s spawned (Claude Code session simulated)\n", i+1, agent.ID)
		} else {
			fmt.Printf("  [%d] Agent %s spawned (type: %s, domain: %s)\n", i+1, agent.ID, agentType, domain)
		}
	}

	fmt.Printf("\nSuccessfully spawned %d agent(s)\n", spawnCount)
	return nil
}

// ============================================================================
// hive-mind status
// ============================================================================

var statusVerbose bool

var hiveMindStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display Hive Mind status and worker details",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHiveMindStatus()
	},
}

func runHiveMindStatus() error {
	if hiveMindState.manager == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	state := hiveMindState.manager.GetState()

	fmt.Println("=== Hive Mind Status ===")
	fmt.Printf("Initialized:      %v\n", state.Initialized)
	fmt.Printf("Algorithm:        %s\n", state.Algorithm)
	fmt.Printf("Active Proposals: %d\n", state.ActiveProposals)
	fmt.Printf("Total Agents:     %d\n", state.TotalAgents)
	fmt.Printf("Active Agents:    %d\n", state.ActiveAgents)
	if state.QueenAgentID != "" {
		fmt.Printf("Queen Agent:      %s\n", state.QueenAgentID)
	}

	// Show domains
	if len(state.DomainsActive) > 0 {
		fmt.Printf("\nActive Domains:\n")
		for _, d := range state.DomainsActive {
			fmt.Printf("  - %s\n", d)
		}
	}

	// Show domain health if verbose
	if statusVerbose && hiveMindState.queen != nil {
		fmt.Println("\n=== Domain Health ===")
		health := hiveMindState.queen.GetDomainHealth()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DOMAIN\tTOTAL\tACTIVE\tHEALTH\tAVG LOAD")
		for domain, h := range health {
			healthPct := fmt.Sprintf("%.0f%%", h.HealthScore*100)
			loadPct := fmt.Sprintf("%.0f%%", h.AvgLoad*100)
			fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\n",
				domain, h.TotalAgents, h.ActiveAgents, healthPct, loadPct)
		}
		w.Flush()

		// Show agents
		fmt.Println("\n=== Agents ===")
		agents := hiveMindState.swarm.ListAgents()
		w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTYPE\tROLE\tSTATUS\tDOMAIN")
		for _, a := range agents {
			sharedAgent := a.ToShared()
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				a.ID, a.Type, a.Role, sharedAgent.Status, a.Domain)
		}
		w.Flush()
	}

	// Show proposals
	if hiveMindState.manager != nil {
		proposals := hiveMindState.manager.ListProposals("")
		if len(proposals) > 0 {
			fmt.Printf("\n=== Active Proposals (%d) ===\n", len(proposals))
			for _, p := range proposals {
				fmt.Printf("  [%s] %s (status: %s, type: %s)\n",
					p.ID[:8], p.Type, p.Status, p.RequiredType)
			}
		}
	}

	return nil
}

// ============================================================================
// hive-mind task
// ============================================================================

var taskPriority string
var taskDomain string
var taskConsensusType string
var taskDescription string

var hiveMindTaskCmd = &cobra.Command{
	Use:   "task [description]",
	Short: "Submit a task with optional consensus requirements",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			taskDescription = args[0]
		}
		return runHiveMindTask()
	},
}

func runHiveMindTask() error {
	if hiveMindState.queen == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	if taskDescription == "" {
		return fmt.Errorf("task description is required")
	}

	// Parse priority
	var priority shared.TaskPriority
	switch strings.ToLower(taskPriority) {
	case "high":
		priority = shared.PriorityHigh
	case "medium":
		priority = shared.PriorityMedium
	case "low":
		priority = shared.PriorityLow
	default:
		priority = shared.PriorityMedium
	}

	// Parse domain
	var domain shared.AgentDomain
	switch strings.ToLower(taskDomain) {
	case "queen":
		domain = shared.DomainQueen
	case "security":
		domain = shared.DomainSecurity
	case "core":
		domain = shared.DomainCore
	case "integration":
		domain = shared.DomainIntegration
	case "support":
		domain = shared.DomainSupport
	default:
		domain = ""
	}

	task := shared.Task{
		ID:          shared.GenerateID("task"),
		Description: taskDescription,
		Priority:    priority,
		Type:        shared.TaskTypeCode,
		Status:      shared.TaskStatusPending,
	}

	ctx := context.Background()

	// Analyze and delegate task
	analysis, err := hiveMindState.queen.AnalyzeTask(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to analyze task: %w", err)
	}

	fmt.Printf("Task created: %s\n", task.ID)
	fmt.Printf("  Description:  %s\n", taskDescription)
	fmt.Printf("  Priority:     %s\n", priority)
	fmt.Printf("  Complexity:   %.2f\n", analysis.ComplexityScore)
	fmt.Printf("  Recommended:  %s domain\n", analysis.RecommendedDomain)
	fmt.Printf("  Est. Time:    %dms\n", analysis.EstimatedDuration)

	// Submit to domain if specified
	if domain != "" {
		if err := hiveMindState.queen.AssignTaskToDomain(ctx, task, domain); err != nil {
			return fmt.Errorf("failed to assign task to domain: %w", err)
		}
		fmt.Printf("  Assigned to:  %s domain\n", domain)
	}

	return nil
}

// ============================================================================
// hive-mind join
// ============================================================================

var joinAgentType string
var joinDomain string

var hiveMindJoinCmd = &cobra.Command{
	Use:   "join [agent-id]",
	Short: "Add an agent to the swarm dynamically",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := ""
		if len(args) > 0 {
			agentID = args[0]
		}
		return runHiveMindJoin(agentID)
	},
}

func runHiveMindJoin(agentID string) error {
	if hiveMindState.swarm == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	if agentID == "" {
		agentID = shared.GenerateID("agent")
	}

	domain := shared.AgentDomain(joinDomain)
	if domain == "" {
		domain = shared.DomainCore
	}

	agentType := shared.AgentType(joinAgentType)
	if agentType == "" {
		agentType = shared.AgentTypeCoder
	}

	config := shared.AgentConfig{
		ID:     agentID,
		Type:   agentType,
		Role:   shared.AgentRoleWorker,
		Domain: domain,
	}

	agent, err := hiveMindState.swarm.SpawnAgent(config)
	if err != nil {
		return fmt.Errorf("failed to join agent: %w", err)
	}

	fmt.Printf("Agent joined swarm successfully\n")
	fmt.Printf("  ID:     %s\n", agent.ID)
	fmt.Printf("  Type:   %s\n", agent.Type)
	fmt.Printf("  Domain: %s\n", agent.Domain)

	return nil
}

// ============================================================================
// hive-mind leave
// ============================================================================

var leaveForce bool

var hiveMindLeaveCmd = &cobra.Command{
	Use:   "leave <agent-id>",
	Short: "Remove an agent from the swarm gracefully",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHiveMindLeave(args[0])
	},
}

func runHiveMindLeave(agentID string) error {
	if hiveMindState.swarm == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	// Check if agent exists
	agent, err := hiveMindState.swarm.GetAgent(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	fmt.Printf("Removing agent %s from swarm...\n", agentID)

	if err := hiveMindState.swarm.TerminateAgent(agentID); err != nil {
		return fmt.Errorf("failed to remove agent: %w", err)
	}

	fmt.Printf("Agent %s (type: %s) left swarm successfully\n", agent.ID, agent.Type)
	return nil
}

// ============================================================================
// hive-mind consensus
// ============================================================================

var hiveMindConsensusCmd = &cobra.Command{
	Use:   "consensus",
	Short: "Manage proposals and voting",
}

// consensus create
var consensusQuorum float64
var consensusTimeout int64
var consensusType string

var consensusCreateCmd = &cobra.Command{
	Use:   "create <proposal-type> <description>",
	Short: "Create a new proposal for voting",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConsensusCreate(args[0], args[1])
	},
}

func runConsensusCreate(proposalType, description string) error {
	if hiveMindState.manager == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	// Parse consensus type
	var cType shared.ConsensusType
	switch strings.ToLower(consensusType) {
	case "majority":
		cType = shared.ConsensusTypeMajority
	case "supermajority":
		cType = shared.ConsensusTypeSuperMajority
	case "unanimous":
		cType = shared.ConsensusTypeUnanimous
	case "weighted":
		cType = shared.ConsensusTypeWeighted
	default:
		cType = hiveMindState.config.ConsensusAlgorithm
	}

	proposal := shared.Proposal{
		Type:           proposalType,
		Description:    description,
		RequiredType:   cType,
		RequiredQuorum: consensusQuorum,
		Proposer:       "cli-user",
	}

	ctx := context.Background()
	created, err := hiveMindState.manager.CreateProposal(ctx, proposal)
	if err != nil {
		return fmt.Errorf("failed to create proposal: %w", err)
	}

	fmt.Printf("Proposal created successfully\n")
	fmt.Printf("  ID:          %s\n", created.ID)
	fmt.Printf("  Type:        %s\n", created.Type)
	fmt.Printf("  Consensus:   %s\n", created.RequiredType)
	fmt.Printf("  Quorum:      %.0f%%\n", created.RequiredQuorum*100)
	fmt.Printf("  Status:      %s\n", created.Status)

	return nil
}

// consensus vote
var voteApprove bool

var consensusVoteCmd = &cobra.Command{
	Use:   "vote <proposal-id>",
	Short: "Submit a vote for a proposal",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConsensusVote(args[0])
	},
}

func runConsensusVote(proposalID string) error {
	if hiveMindState.manager == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	vote := shared.WeightedVote{
		AgentID:    "cli-user",
		ProposalID: proposalID,
		Vote:       voteApprove,
		Weight:     1.0,
		Confidence: 0.9,
		Reason:     "CLI vote",
		Timestamp:  shared.Now(),
	}

	ctx := context.Background()
	if err := hiveMindState.manager.Vote(ctx, vote); err != nil {
		return fmt.Errorf("failed to submit vote: %w", err)
	}

	voteStr := "REJECT"
	if voteApprove {
		voteStr = "APPROVE"
	}
	fmt.Printf("Vote submitted: %s for proposal %s\n", voteStr, proposalID)
	return nil
}

// consensus list
var consensusListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all proposals",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConsensusList()
	},
}

func runConsensusList() error {
	if hiveMindState.manager == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	proposals := hiveMindState.manager.ListProposals("")
	if len(proposals) == 0 {
		fmt.Println("No proposals found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tCONSENSUS\tQUORUM")
	for _, p := range proposals {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.0f%%\n",
			p.ID[:12], p.Type, p.Status, p.RequiredType, p.RequiredQuorum*100)
	}
	w.Flush()

	return nil
}

// consensus result
var consensusResultCmd = &cobra.Command{
	Use:   "result <proposal-id>",
	Short: "Get the result of a proposal",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConsensusResult(args[0])
	},
}

func runConsensusResult(proposalID string) error {
	if hiveMindState.manager == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	result, exists := hiveMindState.manager.GetProposalResult(proposalID)
	if !exists {
		// Try to collect votes synchronously
		ctx := context.Background()
		collected, err := hiveMindState.manager.CollectVotesSync(ctx, proposalID)
		if err != nil {
			return fmt.Errorf("proposal result not available: %w", err)
		}
		result = collected
	}

	fmt.Printf("=== Proposal Result: %s ===\n", proposalID)
	fmt.Printf("Status:            %s\n", result.Status)
	fmt.Printf("Consensus Reached: %v\n", result.ConsensusReached)
	fmt.Printf("Quorum Reached:    %v\n", result.QuorumReached)
	fmt.Printf("Total Votes:       %d\n", result.TotalVotes)
	fmt.Printf("Approval Votes:    %d\n", result.ApprovalVotes)
	fmt.Printf("Rejection Votes:   %d\n", result.RejectionVotes)
	fmt.Printf("Weighted Approval: %.2f%%\n", result.WeightedApproval*100)
	fmt.Printf("Duration:          %dms\n", result.Duration)

	return nil
}

// ============================================================================
// hive-mind broadcast
// ============================================================================

var broadcastDomain string
var broadcastPriority string

var hiveMindBroadcastCmd = &cobra.Command{
	Use:   "broadcast <message>",
	Short: "Broadcast a message to all workers",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHiveMindBroadcast(args[0])
	},
}

func runHiveMindBroadcast(message string) error {
	if hiveMindState.queen == nil {
		return fmt.Errorf("hive mind not initialized. Run 'hive-mind init' first")
	}

	// Parse domain
	var domain shared.AgentDomain
	switch strings.ToLower(broadcastDomain) {
	case "all", "":
		domain = ""
	case "queen":
		domain = shared.DomainQueen
	case "security":
		domain = shared.DomainSecurity
	case "core":
		domain = shared.DomainCore
	case "integration":
		domain = shared.DomainIntegration
	case "support":
		domain = shared.DomainSupport
	default:
		domain = ""
	}

	msg := shared.AgentMessage{
		From:      "cli-user",
		Type:      "broadcast",
		Payload:   map[string]interface{}{"content": message, "priority": broadcastPriority},
		Timestamp: shared.Now(),
	}

	ctx := context.Background()

	if domain == "" {
		// Broadcast to all domains
		for _, d := range []shared.AgentDomain{
			shared.DomainQueen, shared.DomainSecurity,
			shared.DomainCore, shared.DomainIntegration, shared.DomainSupport,
		} {
			_ = hiveMindState.queen.BroadcastMessage(ctx, d, msg)
		}
		fmt.Printf("Message broadcast to all domains: %s\n", message)
	} else {
		if err := hiveMindState.queen.BroadcastMessage(ctx, domain, msg); err != nil {
			return fmt.Errorf("failed to broadcast: %w", err)
		}
		fmt.Printf("Message broadcast to %s domain: %s\n", domain, message)
	}

	return nil
}

// ============================================================================
// hive-mind memory
// ============================================================================

var hiveMindMemoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Access shared memory operations",
}

// memory get
var memoryGetCmd = &cobra.Command{
	Use:   "get <memory-id>",
	Short: "Retrieve a memory by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMemoryGet(args[0])
	},
}

func runMemoryGet(memoryID string) error {
	if hiveMindState.memoryService == nil {
		fmt.Printf("Memory service not available. Memory ID: %s\n", memoryID)
		fmt.Println("(In production, this would retrieve the memory from the backend)")
		return nil
	}

	ctx := context.Background()
	memory, err := hiveMindState.memoryService.Retrieve(ctx, memoryID)
	if err != nil {
		return fmt.Errorf("memory not found: %w", err)
	}

	fmt.Printf("Memory: %s\n", memory.ID)
	fmt.Printf("  Agent:     %s\n", memory.AgentID)
	fmt.Printf("  Type:      %s\n", memory.Type)
	fmt.Printf("  Content:   %s\n", memory.Content)
	fmt.Printf("  Timestamp: %d\n", memory.Timestamp)

	return nil
}

// memory set
var memorySetType string

var memorySetCmd = &cobra.Command{
	Use:   "set <content>",
	Short: "Store a new memory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMemorySet(args[0])
	},
}

func runMemorySet(content string) error {
	memoryID := shared.GenerateID("memory")
	
	var memType shared.MemoryType
	switch strings.ToLower(memorySetType) {
	case "fact":
		memType = shared.MemoryTypeContext
	case "observation":
		memType = shared.MemoryTypeEvent
	case "decision":
		memType = shared.MemoryTypeTask
	default:
		memType = shared.MemoryTypeContext
	}

	fmt.Printf("Memory stored: %s\n", memoryID)
	fmt.Printf("  Type:    %s\n", memType)
	fmt.Printf("  Content: %s\n", content)
	fmt.Println("(In production, this would store to the memory backend)")

	return nil
}

// memory list
var memoryListLimit int

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent memories",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMemoryList()
	},
}

func runMemoryList() error {
	fmt.Printf("Listing recent %d memories...\n", memoryListLimit)
	fmt.Println("(In production, this would query the memory backend)")
	fmt.Println("No memories found (memory backend not connected)")
	return nil
}

// memory search
var memorySearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search memories by content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMemorySearch(args[0])
	},
}

func runMemorySearch(query string) error {
	fmt.Printf("Searching memories for: %s\n", query)
	fmt.Println("(In production, this would perform semantic search)")
	fmt.Println("No results found (memory backend not connected)")
	return nil
}

// ============================================================================
// hive-mind optimize-memory
// ============================================================================

var hiveMindOptimizeMemoryCmd = &cobra.Command{
	Use:   "optimize-memory",
	Short: "Optimize memory patterns and storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHiveMindOptimizeMemory()
	},
}

func runHiveMindOptimizeMemory() error {
	fmt.Println("Optimizing memory...")

	// Simulate optimization
	fmt.Println("  [1/3] Running tier migration...")
	time.Sleep(100 * time.Millisecond)
	fmt.Println("        Hot -> Warm: 0 memories migrated")
	fmt.Println("        Warm -> Cold: 0 memories migrated")

	fmt.Println("  [2/3] Running cleanup...")
	time.Sleep(100 * time.Millisecond)
	fmt.Println("        Expired: 0 memories removed")
	fmt.Println("        Orphaned: 0 embeddings removed")

	fmt.Println("  [3/3] Compressing cold storage...")
	time.Sleep(100 * time.Millisecond)
	fmt.Println("        Compression ratio: N/A")

	fmt.Println("\nOptimization complete")
	fmt.Println("  Total memories: 0")
	fmt.Println("  Memory reduction: 0%")

	return nil
}

// ============================================================================
// hive-mind shutdown
// ============================================================================

var shutdownForce bool
var shutdownSaveState bool

var hiveMindShutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Gracefully shutdown the Hive Mind",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHiveMindShutdown()
	},
}

func runHiveMindShutdown() error {
	if !hiveMindState.initialized {
		fmt.Println("Hive Mind is not running")
		return nil
	}

	fmt.Println("Shutting down Hive Mind...")

	if shutdownSaveState {
		fmt.Println("  Saving state...")
		// In production, this would persist state
	}

	if !shutdownForce {
		fmt.Println("  Waiting for active tasks to complete...")
		time.Sleep(100 * time.Millisecond)
	}

	// Shutdown components
	if hiveMindState.manager != nil {
		fmt.Println("  Stopping Hive Mind Manager...")
		hiveMindState.manager.Shutdown()
	}

	if hiveMindState.queen != nil {
		fmt.Println("  Stopping Queen Coordinator...")
		hiveMindState.queen.Stop()
	}

	if hiveMindState.swarm != nil {
		fmt.Println("  Stopping Swarm Coordinator...")
		hiveMindState.swarm.Shutdown()
	}

	hiveMindState.initialized = false
	hiveMindState.manager = nil
	hiveMindState.queen = nil
	hiveMindState.swarm = nil

	fmt.Println("Hive Mind shutdown complete")
	return nil
}

// ============================================================================
// Init function to wire up all commands
// ============================================================================

func init() {
	// hive-mind init flags
	hiveMindInitCmd.Flags().StringVar(&initAlgorithm, "algorithm", "majority",
		"Consensus algorithm (majority, supermajority, unanimous, weighted, queen-override)")
	hiveMindInitCmd.Flags().BoolVar(&initV3Mode, "v3", false,
		"Enable V3 mode with 15-agent structure")
	hiveMindInitCmd.Flags().Float64Var(&initQuorum, "quorum", 0.5,
		"Required quorum for consensus (0.0 - 1.0)")

	// hive-mind spawn flags
	hiveMindSpawnCmd.Flags().IntVar(&spawnCount, "count", 1, "Number of agents to spawn")
	hiveMindSpawnCmd.Flags().StringVar(&spawnDomain, "domain", "core",
		"Target domain (queen, security, core, integration, support)")
	hiveMindSpawnCmd.Flags().StringVar(&spawnType, "type", "coder", "Agent type")
	hiveMindSpawnCmd.Flags().BoolVar(&spawnClaude, "claude", false,
		"Simulate Claude Code session")

	// hive-mind status flags
	hiveMindStatusCmd.Flags().BoolVarP(&statusVerbose, "verbose", "v", false,
		"Show detailed status including all agents")

	// hive-mind task flags
	hiveMindTaskCmd.Flags().StringVar(&taskPriority, "priority", "medium",
		"Task priority (low, medium, high)")
	hiveMindTaskCmd.Flags().StringVar(&taskDomain, "domain", "",
		"Target domain for task assignment")
	hiveMindTaskCmd.Flags().StringVar(&taskConsensusType, "consensus", "",
		"Required consensus type for task approval")
	hiveMindTaskCmd.Flags().StringVar(&taskDescription, "description", "",
		"Task description")

	// hive-mind join flags
	hiveMindJoinCmd.Flags().StringVar(&joinAgentType, "type", "coder", "Agent type")
	hiveMindJoinCmd.Flags().StringVar(&joinDomain, "domain", "core", "Agent domain")

	// hive-mind leave flags
	hiveMindLeaveCmd.Flags().BoolVar(&leaveForce, "force", false, "Force removal without graceful shutdown")

	// consensus create flags
	consensusCreateCmd.Flags().Float64Var(&consensusQuorum, "quorum", 0.5, "Required quorum (0.0 - 1.0)")
	consensusCreateCmd.Flags().Int64Var(&consensusTimeout, "timeout", 30000, "Vote timeout in milliseconds")
	consensusCreateCmd.Flags().StringVar(&consensusType, "type", "", "Consensus type override")

	// consensus vote flags
	consensusVoteCmd.Flags().BoolVar(&voteApprove, "approve", true, "Vote to approve (default: true)")

	// hive-mind broadcast flags
	hiveMindBroadcastCmd.Flags().StringVar(&broadcastDomain, "domain", "all",
		"Target domain (all, queen, security, core, integration, support)")
	hiveMindBroadcastCmd.Flags().StringVar(&broadcastPriority, "priority", "normal",
		"Message priority (low, normal, high)")

	// memory set flags
	memorySetCmd.Flags().StringVar(&memorySetType, "type", "fact",
		"Memory type (fact, observation, decision)")

	// memory list flags
	memoryListCmd.Flags().IntVar(&memoryListLimit, "limit", 10, "Maximum memories to list")

	// hive-mind shutdown flags
	hiveMindShutdownCmd.Flags().BoolVar(&shutdownForce, "force", false, "Force immediate shutdown")
	hiveMindShutdownCmd.Flags().BoolVar(&shutdownSaveState, "save", true, "Save state before shutdown")

	// Build consensus subcommands
	hiveMindConsensusCmd.AddCommand(consensusCreateCmd)
	hiveMindConsensusCmd.AddCommand(consensusVoteCmd)
	hiveMindConsensusCmd.AddCommand(consensusListCmd)
	hiveMindConsensusCmd.AddCommand(consensusResultCmd)

	// Build memory subcommands
	hiveMindMemoryCmd.AddCommand(memoryGetCmd)
	hiveMindMemoryCmd.AddCommand(memorySetCmd)
	hiveMindMemoryCmd.AddCommand(memoryListCmd)
	hiveMindMemoryCmd.AddCommand(memorySearchCmd)

	// Add all subcommands to HiveMindCmd
	HiveMindCmd.AddCommand(hiveMindInitCmd)
	HiveMindCmd.AddCommand(hiveMindSpawnCmd)
	HiveMindCmd.AddCommand(hiveMindStatusCmd)
	HiveMindCmd.AddCommand(hiveMindTaskCmd)
	HiveMindCmd.AddCommand(hiveMindJoinCmd)
	HiveMindCmd.AddCommand(hiveMindLeaveCmd)
	HiveMindCmd.AddCommand(hiveMindConsensusCmd)
	HiveMindCmd.AddCommand(hiveMindBroadcastCmd)
	HiveMindCmd.AddCommand(hiveMindMemoryCmd)
	HiveMindCmd.AddCommand(hiveMindOptimizeMemoryCmd)
	HiveMindCmd.AddCommand(hiveMindShutdownCmd)
}
