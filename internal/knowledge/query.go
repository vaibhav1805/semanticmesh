package knowledge

import (
	"fmt"
	"time"
)

// ImpactQuery describes a focused incident-response query: "if this component
// fails, what breaks?" Supports depth limiting, confidence filtering, and
// traverse mode control.
type ImpactQuery struct {
	Root          string   // root component name
	Depth         int      // traversal depth (default 1: direct only)
	MinConfidence *float64 // numeric threshold (optional)
	MinTier       *string  // tier name (optional)
	TraverseMode  string   // "direct", "cascade" (default), "full"
}

// CrawlQuery describes an exploratory query: "show me everything connected to
// this component." No confidence filtering — agents see all relationships.
type CrawlQuery struct {
	Root     string // root component name
	MaxDepth int    // max traversal depth (0 = unbounded, uses 100 as safety limit)
}

// ExecuteImpact runs an impact query against the graph, returning a QueryResult
// with depth-limited, confidence-filtered edges and the full affected subgraph.
//
// Impact for incident response (filtered by confidence, focused).
func ExecuteImpact(g *Graph, q *ImpactQuery) (*QueryResult, error) {
	start := time.Now()

	if _, ok := g.Nodes[q.Root]; !ok {
		return nil, fmt.Errorf("ExecuteImpact: root node %q not found in graph", q.Root)
	}

	depth := q.Depth
	if depth < 1 {
		depth = 1
	}

	traverseMode := q.TraverseMode
	if traverseMode == "" {
		traverseMode = "cascade"
	}

	// Run DFS traversal from root with depth limit.
	_, edges := g.TraverseDFS(q.Root, depth)

	// Compute distance for each node reached via BFS (for AffectedNode.Distance).
	distances := computeDistances(g, q.Root, depth)

	// Filter edges by confidence.
	var filtered []*Edge
	for _, e := range edges {
		if !passesConfidenceFilter(e, q.MinConfidence, q.MinTier) {
			continue
		}
		// Apply traverse mode filtering.
		switch traverseMode {
		case "direct":
			// Only include edges where source is the root (distance=1).
			if e.Source != q.Root {
				continue
			}
		case "cascade":
			// Include all edges within depth (default DFS behavior).
		case "full":
			// Include all edges (transitive closure) — same as cascade for DFS.
		}
		filtered = append(filtered, e)
	}

	result := buildQueryResult(g, q.Root, filtered, distances, "impact")
	result.Depth = depth
	result.TraverseMode = traverseMode
	if q.MinConfidence != nil {
		result.MinConfidence = *q.MinConfidence
	}
	if q.MinTier != nil {
		result.MinTier = *q.MinTier
	}
	result.Metadata["execution_time_ms"] = time.Since(start).Milliseconds()

	return result, nil
}

// ExecuteCrawl runs an exploratory query against the graph, returning all
// relationships reachable from root without confidence filtering.
//
// Crawl for exploration (all relationships visible).
func ExecuteCrawl(g *Graph, q *CrawlQuery) (*QueryResult, error) {
	start := time.Now()

	if _, ok := g.Nodes[q.Root]; !ok {
		return nil, fmt.Errorf("ExecuteCrawl: root node %q not found in graph", q.Root)
	}

	maxDepth := q.MaxDepth
	if maxDepth < 1 {
		maxDepth = 100 // safety limit for unbounded traversal
	}

	_, edges := g.TraverseDFS(q.Root, maxDepth)
	distances := computeDistances(g, q.Root, maxDepth)

	result := buildQueryResult(g, q.Root, edges, distances, "crawl")
	result.Depth = maxDepth
	result.TraverseMode = "full"
	result.Metadata["execution_time_ms"] = time.Since(start).Milliseconds()

	return result, nil
}

// buildQueryResult assembles a QueryResult from raw traversal edges.
func buildQueryResult(g *Graph, root string, edges []*Edge, distances map[string]int, queryType string) *QueryResult {
	// Collect unique nodes from edges.
	nodeSet := make(map[string]bool)
	nodeSet[root] = true
	for _, e := range edges {
		nodeSet[e.Source] = true
		nodeSet[e.Target] = true
	}

	var affectedNodes []AffectedNode
	for name := range nodeSet {
		node := g.Nodes[name]
		an := AffectedNode{
			Name:     name,
			Distance: distances[name],
		}
		if node != nil {
			an.Type = string(node.ComponentType)
			if an.Type == "" {
				an.Type = "unknown"
			}
		} else {
			an.Type = "unknown"
		}
		// Set confidence and relationship type from the edge that reached this node.
		an.Confidence = 1.0 // root node
		an.RelationshipType = string(EdgeDirectDependency)
		for _, e := range edges {
			if e.Target == name {
				an.Confidence = e.Confidence
				an.RelationshipType = string(e.RelationshipType)
				break
			}
		}
		affectedNodes = append(affectedNodes, an)
	}

	var queryEdges []QueryEdge
	for _, e := range edges {
		qe := QueryEdge{
			From:             e.Source,
			To:               e.Target,
			Confidence:       e.Confidence,
			Type:             string(e.Type),
			RelationshipType: string(e.RelationshipType),
			Evidence:         e.Evidence,
			SourceFile:       e.SourceFile,
			ExtractionMethod: e.ExtractionMethod,
			EvidencePointer:  e.EvidencePointer,
		}
		queryEdges = append(queryEdges, qe)
	}

	return &QueryResult{
		Query:         queryType,
		Root:          root,
		AffectedNodes: affectedNodes,
		Edges:         queryEdges,
		Metadata: map[string]interface{}{
			"node_count": len(affectedNodes),
			"edge_count": len(queryEdges),
		},
	}
}

// computeDistances returns BFS distances from root to all reachable nodes
// within maxDepth.
func computeDistances(g *Graph, root string, maxDepth int) map[string]int {
	distances := map[string]int{root: 0}

	type entry struct {
		id    string
		depth int
	}
	queue := []entry{{id: root, depth: 0}}
	visited := map[string]bool{root: true}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= maxDepth {
			continue
		}
		for _, edge := range g.BySource[cur.id] {
			if !visited[edge.Target] {
				visited[edge.Target] = true
				distances[edge.Target] = cur.depth + 1
				queue = append(queue, entry{id: edge.Target, depth: cur.depth + 1})
			}
		}
	}
	return distances
}

// passesConfidenceFilter returns true when the edge meets optional confidence
// thresholds (numeric and/or tier-based).
func passesConfidenceFilter(e *Edge, minConfidence *float64, minTier *string) bool {
	if minConfidence != nil && e.Confidence < *minConfidence {
		return false
	}
	if minTier != nil {
		tier := ConfidenceTier(*minTier)
		edgeTier := ScoreToTier(e.Confidence)
		if !TierAtLeast(edgeTier, tier) {
			return false
		}
	}
	return true
}
