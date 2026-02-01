// Package topology provides topology management for swarm coordination.
package topology

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Hierarchical topology implementation
// Queen-worker pattern with single leader.
// First node becomes queen, all others connect to queen.
// Best for: Clear command structure, simple coordination
// Scalability: O(n), moderate scale

// connectHierarchical connects a new node in hierarchical topology.
func (tm *TopologyManager) connectHierarchical(agentID string) {
	node := tm.nodeIndex[agentID]
	if node == nil {
		return
	}

	// First node becomes queen
	if len(tm.nodeIndex) == 1 {
		node.Role = shared.TopologyRoleQueen
		tm.queenNode = node
		tm.roleIndex[shared.TopologyRoleQueen][agentID] = true
		delete(tm.roleIndex[shared.TopologyRoleWorker], agentID)
		delete(tm.roleIndex[shared.TopologyRolePeer], agentID)
		tm.state.Leader = agentID
		return
	}

	// Connect to queen
	if tm.queenNode != nil && tm.queenNode.AgentID != agentID {
		tm.addEdge(agentID, tm.queenNode.AgentID, false)
		tm.addEdge(tm.queenNode.AgentID, agentID, false)
	}
}

// rebalanceHierarchical rebalances the hierarchical topology.
// Ensures queen exists and all workers connect to queen.
func (tm *TopologyManager) rebalanceHierarchical() {
	if len(tm.nodeIndex) == 0 {
		return
	}

	// Ensure we have a queen
	if tm.queenNode == nil {
		// Look for existing queen
		for agentID := range tm.roleIndex[shared.TopologyRoleQueen] {
			if node, exists := tm.nodeIndex[agentID]; exists && node.Status == shared.TopologyStatusActive {
				tm.queenNode = node
				break
			}
		}
	}

	// If still no queen, elect first active node
	if tm.queenNode == nil {
		for _, node := range tm.nodeIndex {
			if node.Status == shared.TopologyStatusActive {
				node.Role = shared.TopologyRoleQueen
				tm.roleIndex[shared.TopologyRoleQueen][node.AgentID] = true
				delete(tm.roleIndex[shared.TopologyRoleWorker], node.AgentID)
				delete(tm.roleIndex[shared.TopologyRolePeer], node.AgentID)
				tm.queenNode = node
				break
			}
		}
	}

	if tm.queenNode == nil {
		return
	}

	// Clear existing edges
	tm.state.Edges = []shared.TopologyEdge{}
	for id := range tm.adjacencyList {
		tm.adjacencyList[id] = make(map[string]bool)
	}
	for _, node := range tm.nodeIndex {
		node.Connections = []string{}
	}

	// Connect all workers to queen
	for agentID, node := range tm.nodeIndex {
		if agentID == tm.queenNode.AgentID {
			continue
		}

		// Ensure non-queen nodes are workers
		if node.Role == shared.TopologyRoleQueen {
			node.Role = shared.TopologyRoleWorker
			tm.roleIndex[shared.TopologyRoleWorker][agentID] = true
			delete(tm.roleIndex[shared.TopologyRoleQueen], agentID)
		}

		// Connect to queen
		tm.addEdge(agentID, tm.queenNode.AgentID, false)
		tm.addEdge(tm.queenNode.AgentID, agentID, false)
	}

	// Set leader
	tm.state.Leader = tm.queenNode.AgentID
}

// GetHierarchy returns the hierarchy structure.
func (tm *TopologyManager) GetHierarchy() shared.SwarmHierarchy {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	hierarchy := shared.SwarmHierarchy{
		Workers: make([]shared.WorkerInfo, 0),
	}

	if tm.queenNode != nil {
		hierarchy.Leader = tm.queenNode.AgentID
	}

	// Get workers
	for agentID := range tm.roleIndex[shared.TopologyRoleWorker] {
		hierarchy.Workers = append(hierarchy.Workers, shared.WorkerInfo{
			ID:     agentID,
			Parent: hierarchy.Leader,
		})
	}

	// Also include peers as workers
	for agentID := range tm.roleIndex[shared.TopologyRolePeer] {
		if agentID != hierarchy.Leader {
			hierarchy.Workers = append(hierarchy.Workers, shared.WorkerInfo{
				ID:     agentID,
				Parent: hierarchy.Leader,
			})
		}
	}

	return hierarchy
}

// PromoteToQueen promotes a worker to queen role.
func (tm *TopologyManager) PromoteToQueen(agentID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	node, exists := tm.nodeIndex[agentID]
	if !exists {
		return nil
	}

	// Demote current queen
	if tm.queenNode != nil && tm.queenNode.AgentID != agentID {
		oldQueen := tm.queenNode
		oldQueen.Role = shared.TopologyRoleWorker
		tm.roleIndex[shared.TopologyRoleWorker][oldQueen.AgentID] = true
		delete(tm.roleIndex[shared.TopologyRoleQueen], oldQueen.AgentID)
	}

	// Promote new queen
	node.Role = shared.TopologyRoleQueen
	tm.roleIndex[shared.TopologyRoleQueen][agentID] = true
	delete(tm.roleIndex[shared.TopologyRoleWorker], agentID)
	delete(tm.roleIndex[shared.TopologyRolePeer], agentID)
	tm.queenNode = node

	// Rebalance
	tm.rebalanceHierarchical()

	return nil
}
