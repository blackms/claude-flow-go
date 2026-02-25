package topology

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func newTestManager(topoType shared.SwarmTopology) *TopologyManager {
	config := shared.TopologyConfig{
		Type:              topoType,
		MaxAgents:         50,
		AutoRebalance:     false,
		FailoverEnabled:   true,
		ReplicationFactor: 2,
	}
	tm := NewTopologyManager(config)
	_ = tm.Initialize()
	return tm
}

func TestAddNode(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	node, err := tm.AddNode("agent-1", shared.TopologyRoleWorker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.AgentID != "agent-1" {
		t.Fatalf("expected agent-1, got %s", node.AgentID)
	}
	if node.Status != shared.TopologyStatusActive {
		t.Fatalf("expected active, got %s", node.Status)
	}
}

func TestAddDuplicateNode(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, err := tm.AddNode("agent-1", shared.TopologyRoleWorker)
	if err != nil {
		t.Fatal(err)
	}

	_, err = tm.AddNode("agent-1", shared.TopologyRoleWorker)
	if err == nil {
		t.Fatal("should reject duplicate node")
	}
}

func TestAddNodeMaxCapacity(t *testing.T) {
	config := shared.TopologyConfig{
		Type:      shared.TopologyMesh,
		MaxAgents: 2,
	}
	tm := NewTopologyManager(config)
	_ = tm.Initialize()

	_, _ = tm.AddNode("agent-1", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("agent-2", shared.TopologyRoleWorker)

	_, err := tm.AddNode("agent-3", shared.TopologyRoleWorker)
	if err == nil {
		t.Fatal("should reject when at max capacity")
	}
}

func TestRemoveNode(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("agent-1", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("agent-2", shared.TopologyRoleWorker)

	err := tm.RemoveNode("agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := tm.GetNode("agent-1")
	if exists {
		t.Fatal("removed node should not exist")
	}
}

func TestRemoveNonexistentNode(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	err := tm.RemoveNode("nonexistent")
	if err == nil {
		t.Fatal("should error on removing nonexistent node")
	}
}

func TestGetNode(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("agent-1", shared.TopologyRoleWorker)

	node, exists := tm.GetNode("agent-1")
	if !exists {
		t.Fatal("node should exist")
	}
	if node.AgentID != "agent-1" {
		t.Fatal("wrong node returned")
	}

	_, exists = tm.GetNode("nonexistent")
	if exists {
		t.Fatal("nonexistent node should not exist")
	}
}

func TestGetNeighborsMesh(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("agent-1", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("agent-2", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("agent-3", shared.TopologyRoleWorker)

	neighbors := tm.GetNeighbors("agent-1")
	if len(neighbors) == 0 {
		t.Fatal("mesh nodes should have neighbors")
	}
}

func TestGetNodesByRole(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("queen-1", shared.TopologyRoleQueen)
	_, _ = tm.AddNode("worker-1", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("worker-2", shared.TopologyRoleWorker)

	queens := tm.GetNodesByRole(shared.TopologyRoleQueen)
	if len(queens) != 1 {
		t.Fatalf("expected 1 queen, got %d", len(queens))
	}

	workers := tm.GetNodesByRole(shared.TopologyRoleWorker)
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(workers))
	}
}

func TestQueenNode(t *testing.T) {
	tm := newTestManager(shared.TopologyHierarchical)

	if tm.GetQueen() != nil {
		t.Fatal("should be nil before adding queen")
	}

	_, _ = tm.AddNode("queen-1", shared.TopologyRoleQueen)
	queen := tm.GetQueen()
	if queen == nil {
		t.Fatal("queen should be set")
	}
	if queen.AgentID != "queen-1" {
		t.Fatal("wrong queen")
	}
}

func TestElectLeader(t *testing.T) {
	tm := newTestManager(shared.TopologyHierarchical)

	_, _ = tm.AddNode("queen-1", shared.TopologyRoleQueen)
	_, _ = tm.AddNode("worker-1", shared.TopologyRoleWorker)

	leader := tm.ElectLeader()
	if leader != "queen-1" {
		t.Fatalf("expected queen as leader, got %s", leader)
	}
}

func TestElectLeaderFallback(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("worker-1", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("worker-2", shared.TopologyRoleWorker)

	leader := tm.ElectLeader()
	if leader == "" {
		t.Fatal("should elect a leader from active nodes")
	}
}

func TestGetStats(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("agent-1", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("agent-2", shared.TopologyRoleWorker)

	stats := tm.GetStats()
	if stats.NodeCount != 2 {
		t.Fatalf("expected 2 nodes, got %d", stats.NodeCount)
	}
}

func TestUpdateNode(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("agent-1", shared.TopologyRoleWorker)

	err := tm.UpdateNode("agent-1", map[string]interface{}{
		"status": shared.TopologyStatusInactive,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	node, _ := tm.GetNode("agent-1")
	if node.Status != shared.TopologyStatusInactive {
		t.Fatalf("expected inactive, got %s", node.Status)
	}
}

func TestUpdateNodeNotFound(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	err := tm.UpdateNode("nonexistent", map[string]interface{}{})
	if err == nil {
		t.Fatal("should error on nonexistent node")
	}
}

func TestShutdown(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("agent-1", shared.TopologyRoleWorker)

	err := tm.Shutdown()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := tm.GetNode("agent-1")
	if exists {
		t.Fatal("all nodes should be cleared after shutdown")
	}
}

func TestLeaderRemovalElectsNew(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("queen-1", shared.TopologyRoleQueen)
	_, _ = tm.AddNode("worker-1", shared.TopologyRoleWorker)

	tm.ElectLeader()
	if tm.GetLeader() != "queen-1" {
		t.Fatal("queen should be leader")
	}

	_ = tm.RemoveNode("queen-1")
	if tm.GetLeader() == "queen-1" {
		t.Fatal("removed leader should trigger new election")
	}
}

func TestStarTopology(t *testing.T) {
	tm := newTestManager(shared.TopologyStar)

	_, _ = tm.AddNode("hub", shared.TopologyRoleCoordinator)
	_, _ = tm.AddNode("spoke-1", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("spoke-2", shared.TopologyRoleWorker)

	coord := tm.GetCoordinator()
	if coord == nil || coord.AgentID != "hub" {
		t.Fatal("coordinator should be set")
	}

	hubNeighbors := tm.GetNeighbors("hub")
	if len(hubNeighbors) != 2 {
		t.Fatalf("hub should connect to 2 spokes, got %d", len(hubNeighbors))
	}
}

func TestHierarchicalTopology(t *testing.T) {
	tm := newTestManager(shared.TopologyHierarchical)

	_, _ = tm.AddNode("queen", shared.TopologyRoleQueen)
	_, _ = tm.AddNode("worker-1", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("worker-2", shared.TopologyRoleWorker)

	queenNeighbors := tm.GetNeighbors("queen")
	if len(queenNeighbors) != 2 {
		t.Fatalf("queen should connect to 2 workers, got %d", len(queenNeighbors))
	}
}

func TestRebalance(t *testing.T) {
	tm := newTestManager(shared.TopologyMesh)

	_, _ = tm.AddNode("agent-1", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("agent-2", shared.TopologyRoleWorker)
	_, _ = tm.AddNode("agent-3", shared.TopologyRoleWorker)

	tm.Rebalance()

	stats := tm.GetStats()
	if stats.NodeCount != 3 {
		t.Fatal("rebalance should not lose nodes")
	}
}
