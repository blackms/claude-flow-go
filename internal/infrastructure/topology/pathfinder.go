// Package topology provides topology management for swarm coordination.
package topology

// FindOptimalPath finds the shortest path between two nodes using BFS.
// Returns the path as a slice of agentIDs, or empty slice if no path exists.
func (tm *TopologyManager) FindOptimalPath(from, to string) []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Check if nodes exist
	if _, exists := tm.nodeIndex[from]; !exists {
		return []string{}
	}
	if _, exists := tm.nodeIndex[to]; !exists {
		return []string{}
	}

	// Self-loop
	if from == to {
		return []string{from}
	}

	// BFS
	visited := make(map[string]bool)
	parent := make(map[string]string)
	queue := []string{from}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check if we reached the destination
		if current == to {
			// Reconstruct path
			path := []string{}
			node := to
			for node != "" {
				path = append([]string{node}, path...)
				node = parent[node]
			}
			return path
		}

		// Explore neighbors
		for neighbor := range tm.adjacencyList[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				parent[neighbor] = current
				queue = append(queue, neighbor)
			}
		}
	}

	// No path found
	return []string{}
}

// FindAllPaths finds all paths between two nodes (up to maxPaths).
// Uses DFS with backtracking.
func (tm *TopologyManager) FindAllPaths(from, to string, maxPaths int) [][]string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Check if nodes exist
	if _, exists := tm.nodeIndex[from]; !exists {
		return [][]string{}
	}
	if _, exists := tm.nodeIndex[to]; !exists {
		return [][]string{}
	}

	// Self-loop
	if from == to {
		return [][]string{{from}}
	}

	paths := make([][]string, 0)
	visited := make(map[string]bool)
	currentPath := []string{from}
	visited[from] = true

	tm.dfsAllPaths(from, to, visited, currentPath, &paths, maxPaths)

	return paths
}

// dfsAllPaths is a helper for FindAllPaths using DFS.
func (tm *TopologyManager) dfsAllPaths(current, target string, visited map[string]bool, path []string, paths *[][]string, maxPaths int) {
	if len(*paths) >= maxPaths {
		return
	}

	if current == target {
		// Found a path - make a copy
		pathCopy := make([]string, len(path))
		copy(pathCopy, path)
		*paths = append(*paths, pathCopy)
		return
	}

	for neighbor := range tm.adjacencyList[current] {
		if !visited[neighbor] {
			visited[neighbor] = true
			tm.dfsAllPaths(neighbor, target, visited, append(path, neighbor), paths, maxPaths)
			visited[neighbor] = false
		}
	}
}

// GetShortestDistance returns the shortest distance (hops) between two nodes.
// Returns -1 if no path exists.
func (tm *TopologyManager) GetShortestDistance(from, to string) int {
	path := tm.FindOptimalPath(from, to)
	if len(path) == 0 {
		return -1
	}
	return len(path) - 1 // Number of edges = nodes - 1
}

// GetConnectedComponents returns all connected components in the topology.
func (tm *TopologyManager) GetConnectedComponents() [][]string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	visited := make(map[string]bool)
	components := make([][]string, 0)

	for agentID := range tm.nodeIndex {
		if !visited[agentID] {
			component := tm.bfsComponent(agentID, visited)
			components = append(components, component)
		}
	}

	return components
}

// bfsComponent finds all nodes in the same component using BFS.
func (tm *TopologyManager) bfsComponent(start string, visited map[string]bool) []string {
	component := []string{}
	queue := []string{start}
	visited[start] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		component = append(component, current)

		for neighbor := range tm.adjacencyList[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return component
}

// IsConnected checks if the topology is fully connected.
func (tm *TopologyManager) IsConnected() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if len(tm.nodeIndex) <= 1 {
		return true
	}

	// Start from any node and try to reach all others
	var startID string
	for id := range tm.nodeIndex {
		startID = id
		break
	}

	visited := make(map[string]bool)
	queue := []string{startID}
	visited[startID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for neighbor := range tm.adjacencyList[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return len(visited) == len(tm.nodeIndex)
}

// GetNodeDegree returns the degree (number of connections) of a node.
func (tm *TopologyManager) GetNodeDegree(agentID string) int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if adjacency, exists := tm.adjacencyList[agentID]; exists {
		return len(adjacency)
	}
	return 0
}

// GetMaxDegreeNode returns the node with the highest degree.
func (tm *TopologyManager) GetMaxDegreeNode() (string, int) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	maxDegree := -1
	maxNode := ""

	for agentID, adjacency := range tm.adjacencyList {
		degree := len(adjacency)
		if degree > maxDegree {
			maxDegree = degree
			maxNode = agentID
		}
	}

	return maxNode, maxDegree
}

// GetMinDegreeNode returns the node with the lowest degree.
func (tm *TopologyManager) GetMinDegreeNode() (string, int) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	minDegree := -1
	minNode := ""

	for agentID, adjacency := range tm.adjacencyList {
		degree := len(adjacency)
		if minDegree < 0 || degree < minDegree {
			minDegree = degree
			minNode = agentID
		}
	}

	if minDegree < 0 {
		minDegree = 0
	}

	return minNode, minDegree
}
