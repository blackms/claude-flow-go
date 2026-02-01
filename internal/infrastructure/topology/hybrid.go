// Package topology provides topology management for swarm coordination.
package topology

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// Hybrid topology implementation
// Combines mesh workers + hierarchical coordinators.
// Workers form limited mesh (max 3 connections to other workers).
// All workers also connect to at least one coordinator/queen.
// Best for: Large-scale enterprise deployments
// Scalability: O(n), up to 200 agents
// Latency: 20-50ms

const (
	maxWorkerMeshConnections = 3
)

// connectHybrid connects a new node in hybrid topology.
func (tm *TopologyManager) connectHybrid(agentID string) {
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
		return
	}

	// Connect based on role
	switch node.Role {
	case shared.TopologyRoleQueen, shared.TopologyRoleCoordinator:
		tm.connectHybridCoordinator(agentID)
	default:
		tm.connectHybridWorker(agentID)
	}
}

// connectHybridCoordinator connects a coordinator/queen in hybrid topology.
func (tm *TopologyManager) connectHybridCoordinator(agentID string) {
	// Coordinators connect to queen and other coordinators
	if tm.queenNode != nil && tm.queenNode.AgentID != agentID {
		tm.addEdge(agentID, tm.queenNode.AgentID, true)
	}

	// Connect to other coordinators
	for otherID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
		if otherID != agentID {
			tm.addEdge(agentID, otherID, true)
		}
	}
}

// connectHybridWorker connects a worker in hybrid topology.
func (tm *TopologyManager) connectHybridWorker(agentID string) {
	// Workers connect to a coordinator or queen
	connectedToLeader := false

	// Try to connect to queen first
	if tm.queenNode != nil {
		tm.addEdge(agentID, tm.queenNode.AgentID, false)
		tm.addEdge(tm.queenNode.AgentID, agentID, false)
		connectedToLeader = true
	}

	// If no queen, connect to a coordinator
	if !connectedToLeader {
		for coordID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
			if node, exists := tm.nodeIndex[coordID]; exists && node.Status == shared.TopologyStatusActive {
				tm.addEdge(agentID, coordID, false)
				tm.addEdge(coordID, agentID, false)
				connectedToLeader = true
				break
			}
		}
	}

	// Connect to limited number of other workers (mesh within workers)
	workerConnections := 0
	for otherID := range tm.roleIndex[shared.TopologyRoleWorker] {
		if otherID == agentID {
			continue
		}
		if workerConnections >= maxWorkerMeshConnections {
			break
		}
		// Only connect if the other worker also has capacity
		otherConnections := tm.countWorkerConnections(otherID)
		if otherConnections < maxWorkerMeshConnections {
			tm.addEdge(agentID, otherID, true)
			workerConnections++
		}
	}

	// Also connect to peers
	for otherID := range tm.roleIndex[shared.TopologyRolePeer] {
		if otherID == agentID {
			continue
		}
		if workerConnections >= maxWorkerMeshConnections {
			break
		}
		otherConnections := tm.countWorkerConnections(otherID)
		if otherConnections < maxWorkerMeshConnections {
			tm.addEdge(agentID, otherID, true)
			workerConnections++
		}
	}
}

// countWorkerConnections counts connections to other workers/peers.
func (tm *TopologyManager) countWorkerConnections(agentID string) int {
	count := 0
	for neighborID := range tm.adjacencyList[agentID] {
		if neighbor, exists := tm.nodeIndex[neighborID]; exists {
			if neighbor.Role == shared.TopologyRoleWorker || neighbor.Role == shared.TopologyRolePeer {
				count++
			}
		}
	}
	return count
}

// rebalanceHybrid rebalances the hybrid topology.
func (tm *TopologyManager) rebalanceHybrid() {
	if len(tm.nodeIndex) == 0 {
		return
	}

	// Ensure we have a queen
	if tm.queenNode == nil {
		for agentID := range tm.roleIndex[shared.TopologyRoleQueen] {
			if node, exists := tm.nodeIndex[agentID]; exists && node.Status == shared.TopologyStatusActive {
				tm.queenNode = node
				break
			}
		}
	}

	// If still no queen, promote first coordinator or first active node
	if tm.queenNode == nil {
		for agentID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
			if node, exists := tm.nodeIndex[agentID]; exists && node.Status == shared.TopologyStatusActive {
				node.Role = shared.TopologyRoleQueen
				tm.roleIndex[shared.TopologyRoleQueen][agentID] = true
				delete(tm.roleIndex[shared.TopologyRoleCoordinator], agentID)
				tm.queenNode = node
				break
			}
		}
	}

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

	// Clear existing edges
	tm.state.Edges = []shared.TopologyEdge{}
	for id := range tm.adjacencyList {
		tm.adjacencyList[id] = make(map[string]bool)
	}
	for _, node := range tm.nodeIndex {
		node.Connections = []string{}
	}

	// Connect coordinators to queen
	for coordID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
		if tm.queenNode != nil && coordID != tm.queenNode.AgentID {
			tm.addEdge(coordID, tm.queenNode.AgentID, true)
		}
		// Connect coordinators to each other
		for otherCoordID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
			if coordID != otherCoordID {
				tm.addEdge(coordID, otherCoordID, true)
			}
		}
	}

	// Collect workers and peers
	workers := make([]string, 0)
	for agentID := range tm.roleIndex[shared.TopologyRoleWorker] {
		workers = append(workers, agentID)
	}
	for agentID := range tm.roleIndex[shared.TopologyRolePeer] {
		workers = append(workers, agentID)
	}

	// Connect workers to queen or coordinator
	leader := tm.queenNode
	if leader == nil {
		for coordID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
			if node, exists := tm.nodeIndex[coordID]; exists && node.Status == shared.TopologyStatusActive {
				leader = node
				break
			}
		}
	}

	for _, workerID := range workers {
		if leader != nil {
			tm.addEdge(workerID, leader.AgentID, false)
			tm.addEdge(leader.AgentID, workerID, false)
		}
	}

	// Create limited mesh among workers (max 3 connections per worker)
	for i, workerID := range workers {
		connections := 0
		for j := i + 1; j < len(workers) && connections < maxWorkerMeshConnections; j++ {
			otherID := workers[j]
			otherConnections := tm.countWorkerConnections(otherID)
			if otherConnections < maxWorkerMeshConnections {
				tm.addEdge(workerID, otherID, true)
				connections++
			}
		}
	}

	// Set leader
	if tm.queenNode != nil {
		tm.state.Leader = tm.queenNode.AgentID
	}
}

// GetHybridCoordinators returns all coordinator nodes (queen + coordinators).
func (tm *TopologyManager) GetHybridCoordinators() []*shared.TopologyNode {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	coordinators := make([]*shared.TopologyNode, 0)

	// Add queen
	for agentID := range tm.roleIndex[shared.TopologyRoleQueen] {
		if node, exists := tm.nodeIndex[agentID]; exists {
			coordinators = append(coordinators, node)
		}
	}

	// Add coordinators
	for agentID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
		if node, exists := tm.nodeIndex[agentID]; exists {
			coordinators = append(coordinators, node)
		}
	}

	return coordinators
}

// GetHybridWorkers returns all worker nodes.
func (tm *TopologyManager) GetHybridWorkers() []*shared.TopologyNode {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	workers := make([]*shared.TopologyNode, 0)

	for agentID := range tm.roleIndex[shared.TopologyRoleWorker] {
		if node, exists := tm.nodeIndex[agentID]; exists {
			workers = append(workers, node)
		}
	}

	for agentID := range tm.roleIndex[shared.TopologyRolePeer] {
		if node, exists := tm.nodeIndex[agentID]; exists {
			workers = append(workers, node)
		}
	}

	return workers
}

// PromoteToCoordinator promotes a worker to coordinator role.
func (tm *TopologyManager) PromoteToCoordinator(agentID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	node, exists := tm.nodeIndex[agentID]
	if !exists {
		return nil
	}

	// Update role
	oldRole := node.Role
	node.Role = shared.TopologyRoleCoordinator

	// Update role indexes
	delete(tm.roleIndex[oldRole], agentID)
	tm.roleIndex[shared.TopologyRoleCoordinator][agentID] = true

	// Update cached coordinator
	if tm.coordinatorNode == nil {
		tm.coordinatorNode = node
	}

	// Rebalance
	tm.rebalanceHybrid()

	return nil
}

// GetWorkerMeshConnections returns worker-to-worker connections.
func (tm *TopologyManager) GetWorkerMeshConnections() []shared.TopologyEdge {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	edges := make([]shared.TopologyEdge, 0)
	seen := make(map[string]bool)

	for _, edge := range tm.state.Edges {
		fromNode, fromExists := tm.nodeIndex[edge.From]
		toNode, toExists := tm.nodeIndex[edge.To]

		if !fromExists || !toExists {
			continue
		}

		// Check if both are workers/peers
		fromIsWorker := fromNode.Role == shared.TopologyRoleWorker || fromNode.Role == shared.TopologyRolePeer
		toIsWorker := toNode.Role == shared.TopologyRoleWorker || toNode.Role == shared.TopologyRolePeer

		if fromIsWorker && toIsWorker {
			// Avoid duplicates for bidirectional edges
			key := edge.From + "-" + edge.To
			reverseKey := edge.To + "-" + edge.From
			if !seen[key] && !seen[reverseKey] {
				edges = append(edges, edge)
				seen[key] = true
			}
		}
	}

	return edges
}
