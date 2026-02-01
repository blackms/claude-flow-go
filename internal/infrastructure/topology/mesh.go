// Package topology provides topology management for swarm coordination.
package topology

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Mesh topology implementation
// Peer-to-peer, bidirectional edges.
// Each node connects to multiple other nodes (target: 5 connections or all if < 5 nodes).
// Best for: Resilient communication, moderate scale
// Scalability: O(n), up to 100 agents

const (
	targetMeshConnections = 5
)

// connectMesh connects a new node in mesh topology.
func (tm *TopologyManager) connectMesh(agentID string) {
	// Connect to existing nodes up to target connections
	connections := 0
	for otherID := range tm.nodeIndex {
		if otherID == agentID {
			continue
		}
		if connections >= targetMeshConnections {
			break
		}
		tm.addEdge(agentID, otherID, true)
		connections++
	}
}

// rebalanceMesh rebalances the mesh topology.
// Ensures each node has at least targetMeshConnections (or all if fewer nodes).
func (tm *TopologyManager) rebalanceMesh() {
	nodeCount := len(tm.nodeIndex)
	if nodeCount == 0 {
		return
	}

	targetConns := targetMeshConnections
	if nodeCount-1 < targetConns {
		targetConns = nodeCount - 1
	}

	// Collect all nodes
	nodes := make([]*shared.TopologyNode, 0, nodeCount)
	for _, node := range tm.nodeIndex {
		nodes = append(nodes, node)
	}

	// Ensure minimum connections for each node
	for _, node := range nodes {
		currentConns := len(tm.adjacencyList[node.AgentID])
		if currentConns < targetConns {
			needed := targetConns - currentConns
			for _, other := range nodes {
				if other.AgentID == node.AgentID {
					continue
				}
				if tm.adjacencyList[node.AgentID][other.AgentID] {
					continue // Already connected
				}
				if needed <= 0 {
					break
				}
				tm.addEdge(node.AgentID, other.AgentID, true)
				needed--
			}
		}
	}

	// Elect leader if needed
	if tm.state.Leader == "" && tm.config.FailoverEnabled {
		tm.electLeaderInternal()
	}
}

// GetMeshConnections returns all mesh connections as edges.
func (tm *TopologyManager) GetMeshConnections() []shared.MeshConnection {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	connections := make([]shared.MeshConnection, 0)
	seen := make(map[string]bool)

	for _, edge := range tm.state.Edges {
		// Avoid duplicates for bidirectional edges
		key := edge.From + "-" + edge.To
		reverseKey := edge.To + "-" + edge.From
		if !seen[key] && !seen[reverseKey] {
			connections = append(connections, shared.MeshConnection{
				From:   edge.From,
				To:     edge.To,
				Type:   "mesh",
				Weight: edge.Weight,
			})
			seen[key] = true
		}
	}

	return connections
}
