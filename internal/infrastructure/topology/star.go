// Package topology provides topology management for swarm coordination.
package topology

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Star topology implementation
// Central coordinator (hub) with all nodes connecting only to it.
// Best for: Simple coordination, small to medium swarms
// Scalability: O(n), up to 50 agents
// Latency: 10-20ms

// connectStar connects a new node in star topology.
func (tm *TopologyManager) connectStar(agentID string) {
	node := tm.nodeIndex[agentID]
	if node == nil {
		return
	}

	// First node becomes the hub (coordinator)
	if len(tm.nodeIndex) == 1 {
		node.Role = shared.TopologyRoleCoordinator
		tm.coordinatorNode = node
		tm.roleIndex[shared.TopologyRoleCoordinator][agentID] = true
		delete(tm.roleIndex[shared.TopologyRoleWorker], agentID)
		delete(tm.roleIndex[shared.TopologyRolePeer], agentID)
		return
	}

	// Find the hub
	hub := tm.getHub()
	if hub == nil {
		// No hub found, elect one
		hub = tm.electHub()
	}

	if hub != nil && hub.AgentID != agentID {
		// Connect to hub
		tm.addEdge(agentID, hub.AgentID, false) // Not bidirectional - spokes connect to hub
		tm.addEdge(hub.AgentID, agentID, false) // Hub connects back to spoke
	}
}

// getHub returns the current hub (coordinator) node.
func (tm *TopologyManager) getHub() *shared.TopologyNode {
	// Check cached coordinator
	if tm.coordinatorNode != nil && tm.coordinatorNode.Status == shared.TopologyStatusActive {
		return tm.coordinatorNode
	}

	// Look for coordinator in role index
	for agentID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
		if node, exists := tm.nodeIndex[agentID]; exists && node.Status == shared.TopologyStatusActive {
			tm.coordinatorNode = node
			return node
		}
	}

	return nil
}

// electHub elects a new hub from available nodes.
func (tm *TopologyManager) electHub() *shared.TopologyNode {
	// Priority: existing coordinator > queen > first active node
	for agentID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
		if node, exists := tm.nodeIndex[agentID]; exists && node.Status == shared.TopologyStatusActive {
			tm.coordinatorNode = node
			return node
		}
	}

	for agentID := range tm.roleIndex[shared.TopologyRoleQueen] {
		if node, exists := tm.nodeIndex[agentID]; exists && node.Status == shared.TopologyStatusActive {
			// Promote to coordinator
			node.Role = shared.TopologyRoleCoordinator
			tm.roleIndex[shared.TopologyRoleCoordinator][agentID] = true
			delete(tm.roleIndex[shared.TopologyRoleQueen], agentID)
			tm.coordinatorNode = node
			return node
		}
	}

	// Elect first active node
	for _, node := range tm.nodeIndex {
		if node.Status == shared.TopologyStatusActive {
			node.Role = shared.TopologyRoleCoordinator
			tm.roleIndex[shared.TopologyRoleCoordinator][node.AgentID] = true
			delete(tm.roleIndex[shared.TopologyRoleWorker], node.AgentID)
			delete(tm.roleIndex[shared.TopologyRolePeer], node.AgentID)
			tm.coordinatorNode = node
			return node
		}
	}

	return nil
}

// rebalanceStar rebalances the star topology.
// Ensures all non-hub nodes connect only to the hub.
func (tm *TopologyManager) rebalanceStar() {
	if len(tm.nodeIndex) == 0 {
		return
	}

	// Ensure we have a hub
	hub := tm.getHub()
	if hub == nil {
		hub = tm.electHub()
	}

	if hub == nil {
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

	// Connect all nodes to hub
	for agentID, node := range tm.nodeIndex {
		if agentID == hub.AgentID {
			continue
		}

		// Ensure non-hub nodes are not coordinators
		if node.Role == shared.TopologyRoleCoordinator {
			node.Role = shared.TopologyRoleWorker
			tm.roleIndex[shared.TopologyRoleWorker][agentID] = true
			delete(tm.roleIndex[shared.TopologyRoleCoordinator], agentID)
		}

		// Connect spoke to hub
		tm.addEdge(agentID, hub.AgentID, false)
		tm.addEdge(hub.AgentID, agentID, false)
	}

	// Set leader
	tm.state.Leader = hub.AgentID
}

// GetHub returns the hub (coordinator) node.
func (tm *TopologyManager) GetHub() *shared.TopologyNode {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.getHub()
}

// GetSpokes returns all non-hub nodes.
func (tm *TopologyManager) GetSpokes() []*shared.TopologyNode {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	hub := tm.getHub()
	spokes := make([]*shared.TopologyNode, 0)

	for _, node := range tm.nodeIndex {
		if hub == nil || node.AgentID != hub.AgentID {
			spokes = append(spokes, node)
		}
	}

	return spokes
}

// IsHub checks if the given agent is the hub.
func (tm *TopologyManager) IsHub(agentID string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	hub := tm.getHub()
	return hub != nil && hub.AgentID == agentID
}

// PromoteToHub promotes a node to be the new hub.
func (tm *TopologyManager) PromoteToHub(agentID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	node, exists := tm.nodeIndex[agentID]
	if !exists {
		return nil
	}

	// Demote current hub
	if tm.coordinatorNode != nil && tm.coordinatorNode.AgentID != agentID {
		oldHub := tm.coordinatorNode
		oldHub.Role = shared.TopologyRoleWorker
		tm.roleIndex[shared.TopologyRoleWorker][oldHub.AgentID] = true
		delete(tm.roleIndex[shared.TopologyRoleCoordinator], oldHub.AgentID)
	}

	// Promote new hub
	node.Role = shared.TopologyRoleCoordinator
	tm.roleIndex[shared.TopologyRoleCoordinator][agentID] = true
	delete(tm.roleIndex[shared.TopologyRoleWorker], agentID)
	delete(tm.roleIndex[shared.TopologyRolePeer], agentID)
	tm.coordinatorNode = node

	// Rebalance to update connections
	tm.rebalanceStar()

	return nil
}

// GetHubLoad returns the number of spokes connected to the hub.
func (tm *TopologyManager) GetHubLoad() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	hub := tm.getHub()
	if hub == nil {
		return 0
	}

	return len(tm.adjacencyList[hub.AgentID])
}
