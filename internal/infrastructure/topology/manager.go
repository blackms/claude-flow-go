// Package topology provides topology management for swarm coordination.
package topology

import (
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// TopologyManager manages the topology of a swarm with O(1) lookups.
type TopologyManager struct {
	config shared.TopologyConfig
	state  shared.TopologyState

	// O(1) lookup indexes
	nodeIndex     map[string]*shared.TopologyNode    // agentId -> node
	adjacencyList map[string]map[string]bool         // agentId -> set of connected agentIds
	roleIndex     map[shared.TopologyNodeRole]map[string]bool // role -> set of agentIds

	// Cached special nodes for O(1) access
	queenNode       *shared.TopologyNode
	coordinatorNode *shared.TopologyNode

	// Rebalancing control
	lastRebalance    time.Time
	minRebalanceGap  time.Duration

	// Event callbacks
	onNodeAdded      func(node *shared.TopologyNode)
	onNodeRemoved    func(agentID string)
	onLeaderElected  func(leaderID string)
	onRebalanced     func()

	mu sync.RWMutex
}

// NewTopologyManager creates a new TopologyManager with the given configuration.
func NewTopologyManager(config shared.TopologyConfig) *TopologyManager {
	return &TopologyManager{
		config:          config,
		nodeIndex:       make(map[string]*shared.TopologyNode),
		adjacencyList:   make(map[string]map[string]bool),
		roleIndex:       make(map[shared.TopologyNodeRole]map[string]bool),
		minRebalanceGap: 5 * time.Second,
	}
}

// NewTopologyManagerWithDefaults creates a new TopologyManager with default configuration.
func NewTopologyManagerWithDefaults() *TopologyManager {
	return NewTopologyManager(shared.DefaultTopologyConfig())
}

// Initialize initializes the topology manager.
func (tm *TopologyManager) Initialize() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Initialize role index for all roles
	tm.roleIndex[shared.TopologyRoleQueen] = make(map[string]bool)
	tm.roleIndex[shared.TopologyRoleWorker] = make(map[string]bool)
	tm.roleIndex[shared.TopologyRoleCoordinator] = make(map[string]bool)
	tm.roleIndex[shared.TopologyRolePeer] = make(map[string]bool)

	return nil
}

// Shutdown shuts down the topology manager.
func (tm *TopologyManager) Shutdown() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.nodeIndex = make(map[string]*shared.TopologyNode)
	tm.adjacencyList = make(map[string]map[string]bool)
	tm.roleIndex = make(map[shared.TopologyNodeRole]map[string]bool)
	tm.queenNode = nil
	tm.coordinatorNode = nil
	tm.state = shared.TopologyState{}

	return nil
}

// AddNode adds a node to the topology with the given role.
func (tm *TopologyManager) AddNode(agentID string, role shared.TopologyNodeRole) (*shared.TopologyNode, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if node already exists
	if _, exists := tm.nodeIndex[agentID]; exists {
		return nil, fmt.Errorf("node %s already exists", agentID)
	}

	// Check max agents
	if len(tm.nodeIndex) >= tm.config.MaxAgents {
		return nil, fmt.Errorf("max agents (%d) reached", tm.config.MaxAgents)
	}

	// Create node
	node := &shared.TopologyNode{
		ID:          fmt.Sprintf("node_%s", agentID),
		AgentID:     agentID,
		Role:        role,
		Status:      shared.TopologyStatusActive,
		Connections: []string{},
		Metadata: map[string]interface{}{
			"joinedAt": shared.Now(),
		},
	}

	// Add to indexes
	tm.nodeIndex[agentID] = node
	tm.adjacencyList[agentID] = make(map[string]bool)

	if tm.roleIndex[role] == nil {
		tm.roleIndex[role] = make(map[string]bool)
	}
	tm.roleIndex[role][agentID] = true

	// Update cached special nodes
	if role == shared.TopologyRoleQueen && tm.queenNode == nil {
		tm.queenNode = node
	}
	if role == shared.TopologyRoleCoordinator && tm.coordinatorNode == nil {
		tm.coordinatorNode = node
	}

	// Add to state
	tm.state.Nodes = append(tm.state.Nodes, *node)

	// Connect based on topology
	tm.connectNode(agentID)

	// Auto-rebalance if enabled
	if tm.config.AutoRebalance {
		tm.rebalanceIfNeeded()
	}

	// Callback
	if tm.onNodeAdded != nil {
		tm.onNodeAdded(node)
	}

	return node, nil
}

// RemoveNode removes a node from the topology.
func (tm *TopologyManager) RemoveNode(agentID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	node, exists := tm.nodeIndex[agentID]
	if !exists {
		return fmt.Errorf("node %s not found", agentID)
	}

	// Remove from adjacency list and disconnect neighbors
	for neighborID := range tm.adjacencyList[agentID] {
		delete(tm.adjacencyList[neighborID], agentID)
		// Update neighbor's connections
		if neighbor, ok := tm.nodeIndex[neighborID]; ok {
			neighbor.Connections = tm.removeFromSlice(neighbor.Connections, agentID)
		}
	}
	delete(tm.adjacencyList, agentID)

	// Remove from role index
	delete(tm.roleIndex[node.Role], agentID)

	// Update cached special nodes
	if tm.queenNode != nil && tm.queenNode.AgentID == agentID {
		tm.queenNode = nil
	}
	if tm.coordinatorNode != nil && tm.coordinatorNode.AgentID == agentID {
		tm.coordinatorNode = nil
	}

	// Remove from node index
	delete(tm.nodeIndex, agentID)

	// Remove from state
	tm.state.Nodes = tm.removeNodeFromSlice(tm.state.Nodes, agentID)
	tm.state.Edges = tm.removeEdgesForNode(tm.state.Edges, agentID)

	// Handle leader removal
	if tm.state.Leader == agentID {
		tm.state.Leader = ""
		if tm.config.FailoverEnabled {
			tm.electLeaderInternal()
		}
	}

	// Update partitions
	tm.removeFromPartitions(agentID)

	// Auto-rebalance if enabled
	if tm.config.AutoRebalance {
		tm.rebalanceIfNeeded()
	}

	// Callback
	if tm.onNodeRemoved != nil {
		tm.onNodeRemoved(agentID)
	}

	return nil
}

// UpdateNode updates a node's properties.
func (tm *TopologyManager) UpdateNode(agentID string, updates map[string]interface{}) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	node, exists := tm.nodeIndex[agentID]
	if !exists {
		return fmt.Errorf("node %s not found", agentID)
	}

	// Update role if provided
	if newRole, ok := updates["role"].(shared.TopologyNodeRole); ok {
		oldRole := node.Role
		delete(tm.roleIndex[oldRole], agentID)
		if tm.roleIndex[newRole] == nil {
			tm.roleIndex[newRole] = make(map[string]bool)
		}
		tm.roleIndex[newRole][agentID] = true
		node.Role = newRole

		// Update cached special nodes
		if newRole == shared.TopologyRoleQueen {
			tm.queenNode = node
		} else if oldRole == shared.TopologyRoleQueen && tm.queenNode == node {
			tm.queenNode = nil
		}
		if newRole == shared.TopologyRoleCoordinator {
			tm.coordinatorNode = node
		} else if oldRole == shared.TopologyRoleCoordinator && tm.coordinatorNode == node {
			tm.coordinatorNode = nil
		}
	}

	// Update status if provided
	if newStatus, ok := updates["status"].(shared.TopologyNodeStatus); ok {
		node.Status = newStatus
	}

	// Update metadata if provided
	if metadata, ok := updates["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			node.Metadata[k] = v
		}
	}

	return nil
}

// GetNode returns a node by agentID. O(1).
func (tm *TopologyManager) GetNode(agentID string) (*shared.TopologyNode, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	node, exists := tm.nodeIndex[agentID]
	return node, exists
}

// GetNeighbors returns the neighbors of a node. O(1).
func (tm *TopologyManager) GetNeighbors(agentID string) []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	neighbors := make([]string, 0)
	if adjacency, exists := tm.adjacencyList[agentID]; exists {
		for neighborID := range adjacency {
			neighbors = append(neighbors, neighborID)
		}
	}
	return neighbors
}

// GetNodesByRole returns all nodes with the given role. O(1).
func (tm *TopologyManager) GetNodesByRole(role shared.TopologyNodeRole) []*shared.TopologyNode {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	nodes := make([]*shared.TopologyNode, 0)
	if agentIDs, exists := tm.roleIndex[role]; exists {
		for agentID := range agentIDs {
			if node, ok := tm.nodeIndex[agentID]; ok {
				nodes = append(nodes, node)
			}
		}
	}
	return nodes
}

// GetQueen returns the queen node. O(1).
func (tm *TopologyManager) GetQueen() *shared.TopologyNode {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.queenNode
}

// GetCoordinator returns the coordinator node. O(1).
func (tm *TopologyManager) GetCoordinator() *shared.TopologyNode {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.coordinatorNode
}

// GetLeader returns the current leader agentID.
func (tm *TopologyManager) GetLeader() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.state.Leader
}

// ElectLeader performs leader election based on role priority.
func (tm *TopologyManager) ElectLeader() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.electLeaderInternal()
}

// electLeaderInternal performs leader election (must be called with lock held).
func (tm *TopologyManager) electLeaderInternal() string {
	var leader *shared.TopologyNode

	// Priority: Queen > Coordinator > First active node
	switch tm.config.Type {
	case shared.TopologyHierarchical, shared.TopologyHybrid:
		// Look for queen first
		if tm.queenNode != nil && tm.queenNode.Status == shared.TopologyStatusActive {
			leader = tm.queenNode
		}
	case shared.TopologyStar:
		// Look for coordinator (hub)
		if tm.coordinatorNode != nil && tm.coordinatorNode.Status == shared.TopologyStatusActive {
			leader = tm.coordinatorNode
		}
	}

	// Fall back to any active queen
	if leader == nil {
		for agentID := range tm.roleIndex[shared.TopologyRoleQueen] {
			if node, ok := tm.nodeIndex[agentID]; ok && node.Status == shared.TopologyStatusActive {
				leader = node
				tm.queenNode = node
				break
			}
		}
	}

	// Fall back to any active coordinator
	if leader == nil {
		for agentID := range tm.roleIndex[shared.TopologyRoleCoordinator] {
			if node, ok := tm.nodeIndex[agentID]; ok && node.Status == shared.TopologyStatusActive {
				leader = node
				tm.coordinatorNode = node
				break
			}
		}
	}

	// Fall back to first active node
	if leader == nil {
		for _, node := range tm.nodeIndex {
			if node.Status == shared.TopologyStatusActive {
				leader = node
				break
			}
		}
	}

	if leader != nil {
		tm.state.Leader = leader.AgentID
		if tm.onLeaderElected != nil {
			tm.onLeaderElected(leader.AgentID)
		}
		return leader.AgentID
	}

	return ""
}

// Rebalance rebalances the topology.
func (tm *TopologyManager) Rebalance() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.rebalanceInternal()
}

// rebalanceIfNeeded performs rebalancing if enough time has passed.
func (tm *TopologyManager) rebalanceIfNeeded() {
	if time.Since(tm.lastRebalance) < tm.minRebalanceGap {
		return
	}
	tm.rebalanceInternal()
}

// rebalanceInternal performs the actual rebalancing (must be called with lock held).
func (tm *TopologyManager) rebalanceInternal() {
	tm.lastRebalance = time.Now()

	switch tm.config.Type {
	case shared.TopologyMesh:
		tm.rebalanceMesh()
	case shared.TopologyHierarchical:
		tm.rebalanceHierarchical()
	case shared.TopologyRing:
		tm.rebalanceRing()
	case shared.TopologyStar:
		tm.rebalanceStar()
	case shared.TopologyHybrid:
		tm.rebalanceHybrid()
	}

	// Update partitions for mesh/hybrid
	if tm.config.Type == shared.TopologyMesh || tm.config.Type == shared.TopologyHybrid {
		tm.updatePartitions()
	}

	if tm.onRebalanced != nil {
		tm.onRebalanced()
	}
}

// GetState returns the current topology state.
func (tm *TopologyManager) GetState() shared.TopologyState {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.state
}

// GetConfig returns the current configuration.
func (tm *TopologyManager) GetConfig() shared.TopologyConfig {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.config
}

// GetStats returns topology statistics.
func (tm *TopologyManager) GetStats() shared.TopologyStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	totalConnections := 0
	for _, adjacency := range tm.adjacencyList {
		totalConnections += len(adjacency)
	}

	avgConnections := 0.0
	if len(tm.nodeIndex) > 0 {
		avgConnections = float64(totalConnections) / float64(len(tm.nodeIndex))
	}

	return shared.TopologyStats{
		NodeCount:      len(tm.nodeIndex),
		EdgeCount:      len(tm.state.Edges),
		PartitionCount: len(tm.state.Partitions),
		AvgConnections: avgConnections,
		LeaderID:       tm.state.Leader,
	}
}

// SetOnNodeAdded sets the callback for when a node is added.
func (tm *TopologyManager) SetOnNodeAdded(callback func(node *shared.TopologyNode)) {
	tm.onNodeAdded = callback
}

// SetOnNodeRemoved sets the callback for when a node is removed.
func (tm *TopologyManager) SetOnNodeRemoved(callback func(agentID string)) {
	tm.onNodeRemoved = callback
}

// SetOnLeaderElected sets the callback for when a leader is elected.
func (tm *TopologyManager) SetOnLeaderElected(callback func(leaderID string)) {
	tm.onLeaderElected = callback
}

// SetOnRebalanced sets the callback for when the topology is rebalanced.
func (tm *TopologyManager) SetOnRebalanced(callback func()) {
	tm.onRebalanced = callback
}

// connectNode connects a new node based on the topology type.
func (tm *TopologyManager) connectNode(agentID string) {
	switch tm.config.Type {
	case shared.TopologyMesh:
		tm.connectMesh(agentID)
	case shared.TopologyHierarchical:
		tm.connectHierarchical(agentID)
	case shared.TopologyRing:
		tm.connectRing(agentID)
	case shared.TopologyStar:
		tm.connectStar(agentID)
	case shared.TopologyHybrid:
		tm.connectHybrid(agentID)
	}
}

// addEdge adds an edge between two nodes.
func (tm *TopologyManager) addEdge(from, to string, bidirectional bool) {
	if from == to {
		return
	}

	// Update adjacency list
	if tm.adjacencyList[from] == nil {
		tm.adjacencyList[from] = make(map[string]bool)
	}
	tm.adjacencyList[from][to] = true

	if bidirectional {
		if tm.adjacencyList[to] == nil {
			tm.adjacencyList[to] = make(map[string]bool)
		}
		tm.adjacencyList[to][from] = true
	}

	// Update node connections
	if node, exists := tm.nodeIndex[from]; exists {
		if !tm.containsString(node.Connections, to) {
			node.Connections = append(node.Connections, to)
		}
	}
	if bidirectional {
		if node, exists := tm.nodeIndex[to]; exists {
			if !tm.containsString(node.Connections, from) {
				node.Connections = append(node.Connections, from)
			}
		}
	}

	// Add to state edges
	edge := shared.TopologyEdge{
		From:          from,
		To:            to,
		Weight:        1.0,
		Bidirectional: bidirectional,
	}
	tm.state.Edges = append(tm.state.Edges, edge)
}

// removeEdge removes an edge between two nodes.
func (tm *TopologyManager) removeEdge(from, to string) {
	delete(tm.adjacencyList[from], to)
	delete(tm.adjacencyList[to], from)

	// Update node connections
	if node, exists := tm.nodeIndex[from]; exists {
		node.Connections = tm.removeFromSlice(node.Connections, to)
	}
	if node, exists := tm.nodeIndex[to]; exists {
		node.Connections = tm.removeFromSlice(node.Connections, from)
	}

	// Remove from state edges
	tm.state.Edges = tm.removeEdgeBetween(tm.state.Edges, from, to)
}

// Helper functions

func (tm *TopologyManager) containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (tm *TopologyManager) removeFromSlice(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

func (tm *TopologyManager) removeNodeFromSlice(nodes []shared.TopologyNode, agentID string) []shared.TopologyNode {
	result := make([]shared.TopologyNode, 0, len(nodes))
	for _, node := range nodes {
		if node.AgentID != agentID {
			result = append(result, node)
		}
	}
	return result
}

func (tm *TopologyManager) removeEdgesForNode(edges []shared.TopologyEdge, agentID string) []shared.TopologyEdge {
	result := make([]shared.TopologyEdge, 0, len(edges))
	for _, edge := range edges {
		if edge.From != agentID && edge.To != agentID {
			result = append(result, edge)
		}
	}
	return result
}

func (tm *TopologyManager) removeEdgeBetween(edges []shared.TopologyEdge, from, to string) []shared.TopologyEdge {
	result := make([]shared.TopologyEdge, 0, len(edges))
	for _, edge := range edges {
		if !((edge.From == from && edge.To == to) || (edge.From == to && edge.To == from)) {
			result = append(result, edge)
		}
	}
	return result
}

func (tm *TopologyManager) removeFromPartitions(agentID string) {
	for i := range tm.state.Partitions {
		partition := &tm.state.Partitions[i]
		partition.Nodes = tm.removeFromSlice(partition.Nodes, agentID)

		// Re-elect partition leader if needed
		if partition.Leader == agentID && len(partition.Nodes) > 0 {
			partition.Leader = partition.Nodes[0]
		}
	}

	// Remove empty partitions
	newPartitions := make([]shared.TopologyPartition, 0)
	for _, p := range tm.state.Partitions {
		if len(p.Nodes) > 0 {
			newPartitions = append(newPartitions, p)
		}
	}
	tm.state.Partitions = newPartitions
}

// updatePartitions updates partitions for mesh/hybrid topologies.
func (tm *TopologyManager) updatePartitions() {
	if len(tm.nodeIndex) == 0 {
		return
	}

	// Calculate partition size
	partitionSize := (tm.config.MaxAgents + 9) / 10 // ceil(maxAgents / 10)
	if partitionSize < 1 {
		partitionSize = 1
	}

	// Collect all agent IDs
	agentIDs := make([]string, 0, len(tm.nodeIndex))
	for agentID := range tm.nodeIndex {
		agentIDs = append(agentIDs, agentID)
	}

	// Create partitions using round-robin strategy
	tm.state.Partitions = make([]shared.TopologyPartition, 0)
	for i := 0; i < len(agentIDs); i += partitionSize {
		end := i + partitionSize
		if end > len(agentIDs) {
			end = len(agentIDs)
		}

		partitionNodes := agentIDs[i:end]
		partition := shared.TopologyPartition{
			ID:           fmt.Sprintf("partition_%d", len(tm.state.Partitions)),
			Nodes:        partitionNodes,
			Leader:       partitionNodes[0],
			ReplicaCount: min(len(partitionNodes), tm.config.ReplicationFactor),
		}
		tm.state.Partitions = append(tm.state.Partitions, partition)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
