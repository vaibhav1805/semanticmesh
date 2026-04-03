package knowledge

import (
	"fmt"
	"testing"
	"time"
)

// --- TraversalState unit tests -----------------------------------------------

func TestTraversalState_MarkVisited(t *testing.T) {
	ts := NewTraversalState()
	if ts.HasVisited("A") {
		t.Fatal("expected A not visited initially")
	}
	ts.MarkVisited("A")
	if !ts.HasVisited("A") {
		t.Fatal("expected A visited after MarkVisited")
	}
	if ts.HasVisited("B") {
		t.Fatal("expected B not visited")
	}
}

func TestTraversalState_IsInPath(t *testing.T) {
	ts := NewTraversalState()
	ts.AddPathNode("A")
	ts.AddPathNode("B")

	if !ts.IsInPath("A") {
		t.Fatal("A should be in path")
	}
	if !ts.IsInPath("B") {
		t.Fatal("B should be in path")
	}
	if ts.IsInPath("C") {
		t.Fatal("C should not be in path")
	}
}

func TestTraversalState_PathManagement(t *testing.T) {
	ts := NewTraversalState()
	ts.AddPathNode("A")
	ts.AddPathNode("B")
	ts.AddPathNode("C")

	if len(ts.Path) != 3 {
		t.Fatalf("expected path length 3, got %d", len(ts.Path))
	}

	ts.RemovePathNode()
	if ts.IsInPath("C") {
		t.Fatal("C should have been removed from path")
	}
	if !ts.IsInPath("B") {
		t.Fatal("B should still be in path")
	}
	if len(ts.Path) != 2 {
		t.Fatalf("expected path length 2, got %d", len(ts.Path))
	}
}

func TestTraversalState_DepthLimiting(t *testing.T) {
	ts := NewTraversalState()

	// depth=0, maxDepth=0 → at max (0 >= 0).
	if !ts.AtMaxDepth(0) {
		t.Fatal("depth 0, maxDepth 0 should be at max")
	}

	// depth=0, maxDepth=3 → not at max.
	ts.Depth = 0
	if ts.AtMaxDepth(3) {
		t.Fatal("depth 0, maxDepth 3 should not be at max")
	}

	// depth=3, maxDepth=3 → at max (3 >= 3).
	ts.Depth = 3
	if !ts.AtMaxDepth(3) {
		t.Fatal("depth 3, maxDepth 3 should be at max")
	}

	// depth=5, maxDepth=3 → at max (5 >= 3).
	ts.Depth = 5
	if !ts.AtMaxDepth(3) {
		t.Fatal("depth 5, maxDepth 3 should be at max")
	}
}

func TestTraversalState_RecordCycle(t *testing.T) {
	ts := NewTraversalState()
	ts.RecordCycle([]string{"A", "B", "C", "A"})
	ts.RecordCycle([]string{"X", "Y", "X"})

	if len(ts.Cycles) != 2 {
		t.Fatalf("expected 2 cycles, got %d", len(ts.Cycles))
	}
	if ts.Cycles[0][0] != "A" || ts.Cycles[0][3] != "A" {
		t.Fatal("first cycle should start and end with A")
	}
}

func TestTraversalState_RemovePathNode_Empty(t *testing.T) {
	ts := NewTraversalState()
	// Should not panic on empty path.
	ts.RemovePathNode()
	if len(ts.Path) != 0 {
		t.Fatal("path should remain empty")
	}
}

// --- Helper: build test graphs -----------------------------------------------

// buildGraph creates a graph from a list of node IDs and edge tuples (source, target).
func buildGraph(nodeIDs []string, edges [][2]string) *Graph {
	g := NewGraph()
	for _, id := range nodeIDs {
		_ = g.AddNode(&Node{ID: id, Title: id, Type: "document"})
	}
	for _, e := range edges {
		edge, err := NewEdge(e[0], e[1], EdgeReferences, 0.8, "test")
		if err != nil {
			panic(fmt.Sprintf("buildGraph: %v", err))
		}
		_ = g.AddEdge(edge)
	}
	return g
}

// --- TraverseDFS cycle detection tests ---------------------------------------

func TestTraversalCyclicGraph_Simple(t *testing.T) {
	// A → B → C → A (simple cycle)
	g := buildGraph(
		[]string{"A", "B", "C"},
		[][2]string{{"A", "B"}, {"B", "C"}, {"C", "A"}},
	)

	start := time.Now()
	ts, edges := g.TraverseDFS("A", 10)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("traversal took %v, expected < 1s (possible infinite loop)", elapsed)
	}

	if len(edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(edges))
	}

	// C→A should be marked cyclic.
	var foundCyclic bool
	for _, e := range edges {
		if e.Source == "C" && e.Target == "A" {
			if e.RelationshipType != EdgeCyclicDependency {
				t.Fatalf("C→A should be cyclic-dependency, got %s", e.RelationshipType)
			}
			foundCyclic = true
		} else {
			if e.RelationshipType != EdgeDirectDependency {
				t.Fatalf("%s→%s should be direct-dependency, got %s", e.Source, e.Target, e.RelationshipType)
			}
		}
	}
	if !foundCyclic {
		t.Fatal("expected C→A cyclic edge in results")
	}

	if len(ts.Cycles) != 1 {
		t.Fatalf("expected 1 cycle detected, got %d", len(ts.Cycles))
	}
}

func TestTraversalCyclicGraph_ComplexCycle(t *testing.T) {
	// A → B → D → A (cycle), A → C → D (diamond with cycle back)
	g := buildGraph(
		[]string{"A", "B", "C", "D"},
		[][2]string{{"A", "B"}, {"A", "C"}, {"B", "D"}, {"C", "D"}, {"D", "A"}},
	)

	start := time.Now()
	ts, edges := g.TraverseDFS("A", 10)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("traversal took %v, expected < 1s", elapsed)
	}

	// D→A should be cyclic (back-edge to ancestor).
	var cyclicCount int
	for _, e := range edges {
		if e.RelationshipType == EdgeCyclicDependency {
			cyclicCount++
			if e.Target != "A" {
				t.Fatalf("expected cyclic edge target to be A, got %s", e.Target)
			}
		}
	}
	if cyclicCount == 0 {
		t.Fatal("expected at least one cyclic edge (D→A)")
	}

	if len(ts.Cycles) == 0 {
		t.Fatal("expected at least one cycle detected")
	}
}

func TestTraversalAcyclic(t *testing.T) {
	// Linear: A → B → C (no cycles)
	g := buildGraph(
		[]string{"A", "B", "C"},
		[][2]string{{"A", "B"}, {"B", "C"}},
	)

	ts, edges := g.TraverseDFS("A", 10)

	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
	for _, e := range edges {
		if e.RelationshipType != EdgeDirectDependency {
			t.Fatalf("expected all edges direct-dependency, got %s", e.RelationshipType)
		}
	}
	if len(ts.Cycles) != 0 {
		t.Fatalf("expected 0 cycles, got %d", len(ts.Cycles))
	}
}

// --- Depth limiting tests ----------------------------------------------------

func TestTraversalDepthOne(t *testing.T) {
	// Chain: A → B → C → D; maxDepth=1 returns only A→B.
	g := buildGraph(
		[]string{"A", "B", "C", "D"},
		[][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}},
	)

	_, edges := g.TraverseDFS("A", 1)

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge at depth 1, got %d", len(edges))
	}
	if edges[0].Source != "A" || edges[0].Target != "B" {
		t.Fatalf("expected A→B, got %s→%s", edges[0].Source, edges[0].Target)
	}
}

func TestTraversalDepthTwo(t *testing.T) {
	// Chain: A → B → C → D; maxDepth=2 returns A→B and B→C.
	g := buildGraph(
		[]string{"A", "B", "C", "D"},
		[][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}},
	)

	_, edges := g.TraverseDFS("A", 2)

	if len(edges) != 2 {
		t.Fatalf("expected 2 edges at depth 2, got %d", len(edges))
	}
}

func TestTraversalUnlimitedDepth(t *testing.T) {
	// Chain: A → B → C → D; maxDepth=999 returns all 3 edges.
	g := buildGraph(
		[]string{"A", "B", "C", "D"},
		[][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}},
	)

	_, edges := g.TraverseDFS("A", 999)

	if len(edges) != 3 {
		t.Fatalf("expected 3 edges with unlimited depth, got %d", len(edges))
	}
}

// --- Visited set tests -------------------------------------------------------

func TestTraversalVisitedSet_Diamond(t *testing.T) {
	// Diamond: A → B → D, A → C → D. D should be visited once.
	g := buildGraph(
		[]string{"A", "B", "C", "D"},
		[][2]string{{"A", "B"}, {"A", "C"}, {"B", "D"}, {"C", "D"}},
	)

	ts, _ := g.TraverseDFS("A", 10)

	if !ts.HasVisited("D") {
		t.Fatal("D should be visited")
	}
	// All 4 nodes should be visited exactly once.
	visitedCount := 0
	for _, v := range ts.Visited {
		if v {
			visitedCount++
		}
	}
	if visitedCount != 4 {
		t.Fatalf("expected 4 visited nodes, got %d", visitedCount)
	}
}

// --- GetImpact tests ---------------------------------------------------------

func TestGetImpact_DirectOnly(t *testing.T) {
	g := buildGraph(
		[]string{"A", "B", "C"},
		[][2]string{{"A", "B"}, {"B", "C"}},
	)

	edges := g.GetImpact("A", 1)
	if len(edges) != 1 {
		t.Fatalf("expected 1 direct impact edge, got %d", len(edges))
	}
	if edges[0].Target != "B" {
		t.Fatalf("expected direct impact to B, got %s", edges[0].Target)
	}
}

func TestGetImpact_Transitive(t *testing.T) {
	g := buildGraph(
		[]string{"A", "B", "C"},
		[][2]string{{"A", "B"}, {"B", "C"}},
	)

	edges := g.GetImpact("A", 3)
	if len(edges) != 2 {
		t.Fatalf("expected 2 transitive impact edges, got %d", len(edges))
	}
}

func TestGetImpact_DefaultMinDepth(t *testing.T) {
	// maxDepth=0 should be treated as 1.
	g := buildGraph(
		[]string{"A", "B"},
		[][2]string{{"A", "B"}},
	)

	edges := g.GetImpact("A", 0)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge with default depth, got %d", len(edges))
	}
}

// --- Edge mutation safety ----------------------------------------------------

func TestTraverseDFS_DoesNotMutateOriginalEdges(t *testing.T) {
	g := buildGraph(
		[]string{"A", "B"},
		[][2]string{{"A", "B"}},
	)

	// Original edge should have default RelationshipType from NewEdge.
	origEdge := g.BySource["A"][0]
	origRT := origEdge.RelationshipType

	g.TraverseDFS("A", 10)

	if origEdge.RelationshipType != origRT {
		t.Fatal("TraverseDFS mutated the original graph edge's RelationshipType")
	}
}

// --- Unknown start node ------------------------------------------------------

func TestTraverseDFS_UnknownStart(t *testing.T) {
	g := buildGraph([]string{"A"}, nil)

	ts, edges := g.TraverseDFS("Z", 10)
	if ts != nil || edges != nil {
		t.Fatal("expected nil results for unknown start node")
	}
}

// --- Performance: large cyclic graph -----------------------------------------

func TestTraversalLargeCyclicGraph(t *testing.T) {
	// Build a ring of 500 nodes: 0→1→2→...→499→0.
	nodes := make([]string, 500)
	edges := make([][2]string, 500)
	for i := 0; i < 500; i++ {
		nodes[i] = fmt.Sprintf("node-%d", i)
	}
	for i := 0; i < 500; i++ {
		edges[i] = [2]string{nodes[i], nodes[(i+1)%500]}
	}
	g := buildGraph(nodes, edges)

	start := time.Now()
	ts, resultEdges := g.TraverseDFS("node-0", 1000)
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Fatalf("large cyclic graph took %v, expected < 500ms", elapsed)
	}

	// Should have 500 edges (499 direct + 1 cyclic back to node-0).
	if len(resultEdges) != 500 {
		t.Fatalf("expected 500 edges, got %d", len(resultEdges))
	}
	if len(ts.Cycles) != 1 {
		t.Fatalf("expected 1 cycle in ring graph, got %d", len(ts.Cycles))
	}
}
