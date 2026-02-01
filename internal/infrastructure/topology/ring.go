// Package topology provides topology management for swarm coordination.
package topology

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Ring topology implementation
// Each node connects to exactly 2 neighbors forming a circular pattern.
// Best for: Sequential processing pipelines
// Scalability: O(n), moderate latency (15-35ms)

// connectRing connects a new node in ring topology.
func (tm *TopologyManager) connectRing(agentID string) {
	nodeCount := len(tm.nodeIndex)

	if nodeCount == 1 {
		// First node - no connections yet
		return
	}

	if nodeCount == 2 {
		// Second node - connect to first
		for otherID := range tm.nodeIndex {
			if otherID != agentID {
				tm.addEdge(agentID, otherID, true)
				break
			}
		}
		return
	}

	// Find two nodes to insert between
	// Get ordered list of nodes in the ring
	ringOrder := tm.getRingOrder()

	// Find the last node in the ring (before the new node)
	lastIdx := len(ringOrder) - 2 // -2 because new node is already added
	if lastIdx < 0 {
		lastIdx = 0
	}

	// Insert new node between last and first
	lastNode := ringOrder[lastIdx]
	firstNode := ringOrder[0]

	// Remove edge between last and first
	tm.removeEdge(lastNode, firstNode)

	// Add edges: last -> new -> first
	tm.addEdge(lastNode, agentID, true)
	tm.addEdge(agentID, firstNode, true)
}

// getRingOrder returns the ordered list of agent IDs in the ring.
func (tm *TopologyManager) getRingOrder() []string {
	if len(tm.nodeIndex) == 0 {
		return []string{}
	}

	order := make([]string, 0, len(tm.nodeIndex))
	visited := make(map[string]bool)

	// Start from any node
	var startID string
	for id := range tm.nodeIndex {
		startID = id
		break
	}

	// Traverse the ring
	currentID := startID
	for {
		if visited[currentID] {
			break
		}
		order = append(order, currentID)
		visited[currentID] = true

		// Find next unvisited neighbor
		found := false
		for neighborID := range tm.adjacencyList[currentID] {
			if !visited[neighborID] {
				currentID = neighborID
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	return order
}

// rebalanceRing rebalances the ring topology.
// Ensures each node has exactly 2 connections (except single node).
func (tm *TopologyManager) rebalanceRing() {
	nodeCount := len(tm.nodeIndex)

	if nodeCount == 0 {
		return
	}

	if nodeCount == 1 {
		// Single node - no connections
		for _, node := range tm.nodeIndex {
			node.Connections = []string{}
		}
		tm.state.Edges = []shared.TopologyEdge{}
		return
	}

	// Collect all nodes
	nodes := make([]*shared.TopologyNode, 0, nodeCount)
	for _, node := range tm.nodeIndex {
		nodes = append(nodes, node)
	}

	// Clear existing edges
	tm.state.Edges = []shared.TopologyEdge{}
	for id := range tm.adjacencyList {
		tm.adjacencyList[id] = make(map[string]bool)
	}
	for _, node := range nodes {
		node.Connections = []string{}
	}

	// Create ring: node[0] -> node[1] -> ... -> node[n-1] -> node[0]
	for i := 0; i < nodeCount; i++ {
		nextIdx := (i + 1) % nodeCount
		tm.addEdge(nodes[i].AgentID, nodes[nextIdx].AgentID, true)
	}

	// Elect leader if needed
	if tm.state.Leader == "" && tm.config.FailoverEnabled {
		tm.electLeaderInternal()
	}
}

// GetRingSuccessor returns the next node in the ring.
func (tm *TopologyManager) GetRingSuccessor(agentID string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	neighbors := tm.adjacencyList[agentID]
	ringOrder := tm.getRingOrder()

	// Find current position
	currentIdx := -1
	for i, id := range ringOrder {
		if id == agentID {
			currentIdx = i
			break
		}
	}

	if currentIdx < 0 {
		return ""
	}

	// Return next in ring
	nextIdx := (currentIdx + 1) % len(ringOrder)

	// Verify it's actually a neighbor
	if neighbors[ringOrder[nextIdx]] {
		return ringOrder[nextIdx]
	}

	return ""
}

// GetRingPredecessor returns the previous node in the ring.
func (tm *TopologyManager) GetRingPredecessor(agentID string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	neighbors := tm.adjacencyList[agentID]
	ringOrder := tm.getRingOrder()

	// Find current position
	currentIdx := -1
	for i, id := range ringOrder {
		if id == agentID {
			currentIdx = i
			break
		}
	}

	if currentIdx < 0 {
		return ""
	}

	// Return previous in ring
	prevIdx := (currentIdx - 1 + len(ringOrder)) % len(ringOrder)

	// Verify it's actually a neighbor
	if neighbors[ringOrder[prevIdx]] {
		return ringOrder[prevIdx]
	}

	return ""
}

// GetRingSize returns the number of nodes in the ring.
func (tm *TopologyManager) GetRingSize() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.nodeIndex)
}

// PassMessageAroundRing simulates message passing around the ring.
// Returns the path the message would take.
func (tm *TopologyManager) PassMessageAroundRing(startID string) []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if _, exists := tm.nodeIndex[startID]; !exists {
		return []string{}
	}

	ringOrder := tm.getRingOrder()

	// Find start position
	startIdx := -1
	for i, id := range ringOrder {
		if id == startID {
			startIdx = i
			break
		}
	}

	if startIdx < 0 {
		return []string{}
	}

	// Build path starting from startID
	path := make([]string, len(ringOrder))
	for i := 0; i < len(ringOrder); i++ {
		path[i] = ringOrder[(startIdx+i)%len(ringOrder)]
	}

	return path
}
