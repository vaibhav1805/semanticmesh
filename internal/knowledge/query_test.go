package knowledge

import (
	"encoding/json"
	"testing"
)

// buildTestGraph creates a linear chain: A -> B -> C -> D with varying confidences.
func buildTestGraph() *Graph {
	g := NewGraph()
	for _, id := range []string{"A", "B", "C", "D"} {
		_ = g.AddNode(&Node{ID: id, Title: id, Type: "document", ComponentType: ComponentTypeService})
	}

	edges := []struct {
		src, dst   string
		t          EdgeType
		confidence float64
	}{
		{"A", "B", EdgeDependsOn, 0.9},
		{"B", "C", EdgeDependsOn, 0.7},
		{"C", "D", EdgeDependsOn, 0.5},
	}
	for _, e := range edges {
		edge, _ := NewEdge(e.src, e.dst, e.t, e.confidence, "test evidence")
		edge.SourceFile = "test.md"
		edge.ExtractionMethod = "structural"
		_ = g.AddEdge(edge)
	}
	return g
}

func TestImpactQuery_DirectOnly(t *testing.T) {
	g := buildTestGraph()

	result, err := ExecuteImpact(g, &ImpactQuery{Root: "A", Depth: 1})
	if err != nil {
		t.Fatalf("ExecuteImpact: %v", err)
	}

	// Depth=1: only A->B edge should appear.
	if len(result.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(result.Edges))
	}
	if result.Edges[0].From != "A" || result.Edges[0].To != "B" {
		t.Errorf("expected A->B, got %s->%s", result.Edges[0].From, result.Edges[0].To)
	}

	// AffectedNodes should contain A and B.
	nodeNames := make(map[string]bool)
	for _, n := range result.AffectedNodes {
		nodeNames[n.Name] = true
	}
	if !nodeNames["A"] || !nodeNames["B"] {
		t.Errorf("expected A and B in affected nodes, got %v", nodeNames)
	}
	if nodeNames["C"] || nodeNames["D"] {
		t.Error("C and D should not appear at depth=1")
	}
}

func TestImpactQuery_DepthTwo(t *testing.T) {
	g := buildTestGraph()

	result, err := ExecuteImpact(g, &ImpactQuery{Root: "A", Depth: 2})
	if err != nil {
		t.Fatalf("ExecuteImpact: %v", err)
	}

	if len(result.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(result.Edges))
	}

	nodeNames := make(map[string]bool)
	for _, n := range result.AffectedNodes {
		nodeNames[n.Name] = true
	}
	if !nodeNames["A"] || !nodeNames["B"] || !nodeNames["C"] {
		t.Errorf("expected A, B, C in affected nodes, got %v", nodeNames)
	}
}

func TestImpactQuery_ConfidenceFilter(t *testing.T) {
	g := buildTestGraph()

	minConf := 0.7
	result, err := ExecuteImpact(g, &ImpactQuery{
		Root:          "A",
		Depth:         3,
		MinConfidence: &minConf,
	})
	if err != nil {
		t.Fatalf("ExecuteImpact: %v", err)
	}

	// Only edges with confidence >= 0.7 should appear (A->B at 0.9, B->C at 0.7).
	// C->D at 0.5 should be filtered out.
	for _, e := range result.Edges {
		if e.Confidence < 0.7 {
			t.Errorf("edge %s->%s has confidence %.2f below threshold 0.7", e.From, e.To, e.Confidence)
		}
	}
	if len(result.Edges) != 2 {
		t.Errorf("expected 2 edges after filtering, got %d", len(result.Edges))
	}
}

func TestImpactQuery_TierFilter(t *testing.T) {
	g := buildTestGraph()

	tier := "strong-inference" // >= 0.75
	result, err := ExecuteImpact(g, &ImpactQuery{
		Root:    "A",
		Depth:   3,
		MinTier: &tier,
	})
	if err != nil {
		t.Fatalf("ExecuteImpact: %v", err)
	}

	// Only A->B (0.9, strong-inference) should pass. B->C (0.7, moderate) and C->D (0.5, weak) filtered.
	if len(result.Edges) != 1 {
		t.Fatalf("expected 1 edge after tier filter, got %d", len(result.Edges))
	}
	if result.Edges[0].From != "A" || result.Edges[0].To != "B" {
		t.Errorf("expected A->B, got %s->%s", result.Edges[0].From, result.Edges[0].To)
	}
}

func TestImpactQuery_TraverseModeDirect(t *testing.T) {
	g := buildTestGraph()

	result, err := ExecuteImpact(g, &ImpactQuery{
		Root:         "A",
		Depth:        3,
		TraverseMode: "direct",
	})
	if err != nil {
		t.Fatalf("ExecuteImpact: %v", err)
	}

	// "direct" mode: only edges where source=root (A->B).
	if len(result.Edges) != 1 {
		t.Fatalf("expected 1 edge in direct mode, got %d", len(result.Edges))
	}
	if result.Edges[0].From != "A" {
		t.Errorf("expected edge from root A, got from %s", result.Edges[0].From)
	}
}

func TestImpactQuery_RootNotFound(t *testing.T) {
	g := buildTestGraph()

	_, err := ExecuteImpact(g, &ImpactQuery{Root: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent root")
	}
}

func TestCrawlQuery_NoFilter(t *testing.T) {
	g := buildTestGraph()

	result, err := ExecuteCrawl(g, &CrawlQuery{Root: "A"})
	if err != nil {
		t.Fatalf("ExecuteCrawl: %v", err)
	}

	// All edges should be returned regardless of confidence.
	if len(result.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(result.Edges))
	}

	// All four nodes should be affected.
	if len(result.AffectedNodes) != 4 {
		t.Errorf("expected 4 affected nodes, got %d", len(result.AffectedNodes))
	}
}

func TestCrawlQuery_MaxDepth(t *testing.T) {
	g := buildTestGraph()

	result, err := ExecuteCrawl(g, &CrawlQuery{Root: "A", MaxDepth: 2})
	if err != nil {
		t.Fatalf("ExecuteCrawl: %v", err)
	}

	// MaxDepth=2: A->B, B->C only. C->D is at depth 3.
	if len(result.Edges) != 2 {
		t.Fatalf("expected 2 edges at MaxDepth=2, got %d", len(result.Edges))
	}
}

func TestCrawlQuery_RootNotFound(t *testing.T) {
	g := buildTestGraph()

	_, err := ExecuteCrawl(g, &CrawlQuery{Root: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent root")
	}
}

func TestQueryResult_JSONSchema(t *testing.T) {
	g := buildTestGraph()

	result, err := ExecuteImpact(g, &ImpactQuery{Root: "A", Depth: 2})
	if err != nil {
		t.Fatalf("ExecuteImpact: %v", err)
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// Top-level fields required by CONTEXT.md spec.
	required := []string{"query", "root", "depth", "traverse_mode", "affected_nodes", "edges", "metadata"}
	for _, key := range required {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing required key %q in JSON", key)
		}
	}

	// Verify edge structure has provenance fields.
	edgesRaw := parsed["edges"].([]interface{})
	if len(edgesRaw) > 0 {
		edge := edgesRaw[0].(map[string]interface{})
		edgeFields := []string{"from", "to", "confidence", "type", "relationship_type", "evidence", "source_file", "extraction_method"}
		for _, f := range edgeFields {
			if _, ok := edge[f]; !ok {
				t.Errorf("edge missing field %q", f)
			}
		}
	}

	// Verify node structure.
	nodesRaw := parsed["affected_nodes"].([]interface{})
	if len(nodesRaw) > 0 {
		node := nodesRaw[0].(map[string]interface{})
		nodeFields := []string{"name", "type", "confidence", "relationship_type", "distance"}
		for _, f := range nodeFields {
			if _, ok := node[f]; !ok {
				t.Errorf("node missing field %q", f)
			}
		}
	}
}

func TestQueryResult_ConfidenceFilterComparison(t *testing.T) {
	g := buildTestGraph()

	lowConf := 0.4
	highConf := 0.7

	lowResult, err := ExecuteImpact(g, &ImpactQuery{Root: "A", Depth: 3, MinConfidence: &lowConf})
	if err != nil {
		t.Fatalf("low confidence query: %v", err)
	}

	highResult, err := ExecuteImpact(g, &ImpactQuery{Root: "A", Depth: 3, MinConfidence: &highConf})
	if err != nil {
		t.Fatalf("high confidence query: %v", err)
	}

	if len(highResult.Edges) >= len(lowResult.Edges) {
		t.Errorf("higher MinConfidence should return fewer edges: high=%d, low=%d",
			len(highResult.Edges), len(lowResult.Edges))
	}
}

func TestQueryResult_CycleHandling(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"A", "B", "C"} {
		_ = g.AddNode(&Node{ID: id, Title: id, Type: "document"})
	}
	e1, _ := NewEdge("A", "B", EdgeDependsOn, 0.8, "test")
	e2, _ := NewEdge("B", "C", EdgeDependsOn, 0.8, "test")
	e3, _ := NewEdge("C", "A", EdgeDependsOn, 0.8, "test") // cycle
	_ = g.AddEdge(e1)
	_ = g.AddEdge(e2)
	_ = g.AddEdge(e3)

	result, err := ExecuteCrawl(g, &CrawlQuery{Root: "A"})
	if err != nil {
		t.Fatalf("ExecuteCrawl with cycle: %v", err)
	}

	// Should complete without infinite loop.
	if len(result.Edges) < 3 {
		t.Errorf("expected at least 3 edges (including cyclic), got %d", len(result.Edges))
	}

	// At least one edge should be marked cyclic.
	hasCyclic := false
	for _, e := range result.Edges {
		if e.RelationshipType == string(EdgeCyclicDependency) {
			hasCyclic = true
			break
		}
	}
	if !hasCyclic {
		t.Error("expected at least one cyclic-dependency edge in cycle graph")
	}
}
