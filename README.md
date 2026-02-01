# Claude Flow Go

A Go implementation of the Claude Flow v3 multi-agent orchestration framework.

## Overview

Claude Flow Go is a production-ready enterprise AI orchestration platform for deploying coordinated multi-agent systems. It provides:

- **Multi-agent coordination** with swarm topologies (hierarchical, mesh, adaptive)
- **Workflow execution** with dependency resolution and parallel task execution
- **Memory backends** with SQLite for persistence and vector search capabilities
- **MCP Server** for Model Context Protocol integration
- **Plugin system** for extensibility

## Architecture

The project follows Clean Architecture / DDD patterns:

```
├── cmd/claude-flow/      # CLI entry point
├── internal/
│   ├── domain/           # Domain entities (Agent, Task, Memory)
│   ├── application/      # Application services (SwarmCoordinator, WorkflowEngine)
│   ├── infrastructure/   # Infrastructure (MCP, Memory backends, Plugins)
│   └── shared/           # Shared types and utilities
└── pkg/claude-flow/      # Public API
```

## Installation

```bash
go install github.com/anthropics/claude-flow-go/cmd/claude-flow@latest
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    claudeflow "github.com/anthropics/claude-flow-go/pkg/claude-flow"
)

func main() {
    // Create a new swarm coordinator
    coordinator, err := claudeflow.NewSwarmCoordinator(claudeflow.SwarmConfig{
        Topology: claudeflow.TopologyMesh,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer coordinator.Shutdown()

    // Spawn agents
    agent, err := coordinator.SpawnAgent(claudeflow.AgentConfig{
        ID:           "coder-1",
        Type:         claudeflow.AgentTypeCoder,
        Capabilities: []string{"code", "refactor", "debug"},
    })
    if err != nil {
        log.Fatal(err)
    }

    // Execute tasks
    result, err := coordinator.ExecuteTask(agent.ID, claudeflow.Task{
        ID:          "task-1",
        Type:        claudeflow.TaskTypeCode,
        Description: "Implement feature X",
        Priority:    claudeflow.PriorityHigh,
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Task completed: %s", result.Status)
}
```

## CLI Usage

```bash
# Start MCP server
claude-flow serve --port 3000

# Spawn an agent
claude-flow agent spawn --id coder-1 --type coder

# List agents
claude-flow agent list

# Execute a workflow
claude-flow workflow run --file workflow.yaml
```

## Features

### Agent Types

- `coordinator` - Orchestrates other agents
- `coder` - Code generation and modification
- `tester` - Test creation and execution
- `reviewer` - Code review and analysis
- `designer` - System design
- `deployer` - Deployment operations

### Swarm Topologies

- **Hierarchical** - Leader-worker hierarchy
- **Mesh** - Peer-to-peer connections
- **Simple** - Flat structure
- **Adaptive** - Dynamic topology based on workload

### Memory Backends

- **SQLite** - Persistent storage with SQL queries
- **AgentDB** - Vector search with HNSW algorithm
- **Hybrid** - Combines SQLite + AgentDB

## Development

```bash
# Build
go build ./...

# Test
go test ./...

# Run
go run ./cmd/claude-flow
```

## License

MIT License - see LICENSE file for details.
