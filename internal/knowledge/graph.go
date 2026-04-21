package knowledge

import (
	"fmt"
)

// Node represents a single document vertex in the knowledge graph.
// Its ID is the document's relative path (forward slashes, no leading slash)
// — identical to Document.ID so the two types can be correlated without extra
// bookkeeping.
type Node struct {
	// ID uniquely identifies the node within the graph.  It matches the
	// corresponding Document.ID (relative path, forward slashes).
	ID string

	// Title is the human-readable label extracted from the first H1 heading,
	// or the file-name stem when no heading is present.
	Title string

	// Type describes what kind of entity the node represents.
	// For documents loaded via the scanner the type is always "document".
	Type string

	// ComponentType classifies this node using the 12-type taxonomy
	// (service, database, cache, etc.).  Defaults to "unknown".
	ComponentType ComponentType

	// Description is an LLM-generated description of the component.
	// Empty for non-enriched nodes.
	Description string

	// Tags is a list of LLM-extracted tags (e.g., ["authentication", "api"]).
	// Empty for non-enriched nodes.
	Tags []string
}

// Graph is a directed knowledge graph where nodes represent documents and edges
// represent typed, confidence-scored relationships between them.
//
// Internals use four maps to support O(1) node/edge lookup and O(degree)
// adjacency traversal without O(n²) scans.
//
// Zero value is NOT valid; always create via NewGraph.
type Graph struct {
	// Nodes maps node ID → *Node.
	Nodes map[string]*Node

	// Edges maps edge ID → *Edge.
	Edges map[string]*Edge

	// BySource maps source node ID → outgoing edges from that node.
	BySource map[string][]*Edge

	// ByTarget maps target node ID → incoming edges to that node.
	ByTarget map[string][]*Edge
}

// NewGraph returns an empty, ready-to-use Graph.
func NewGraph() *Graph {
	return &Graph{
		Nodes:    make(map[string]*Node),
		Edges:    make(map[string]*Edge),
		BySource: make(map[string][]*Edge),
		ByTarget: make(map[string][]*Edge),
	}
}

// AddNode adds node to the graph.
//
// If a node with the same ID already exists it is replaced in-place (last
// write wins).  Returns an error only when node is nil or its ID is empty.
func (g *Graph) AddNode(node *Node) error {
	if node == nil {
		return fmt.Errorf("knowledge.Graph.AddNode: node must not be nil")
	}
	if node.ID == "" {
		return fmt.Errorf("knowledge.Graph.AddNode: node.ID must not be empty")
	}
	g.Nodes[node.ID] = node
	return nil
}

// AddEdge adds edge to the graph and maintains the BySource / ByTarget indices.
//
// Rules:
//   - Self-loops (source == target) are rejected.
//   - Duplicate edges (same ID) are silently dropped; callers should prefer
//     the higher-confidence edge before adding.
//
// Returns an error when edge is nil, the edge ID is empty, or source == target.
func (g *Graph) AddEdge(edge *Edge) error {
	if edge == nil {
		return fmt.Errorf("knowledge.Graph.AddEdge: edge must not be nil")
	}
	if edge.ID == "" {
		return fmt.Errorf("knowledge.Graph.AddEdge: edge.ID must not be empty")
	}
	if edge.Source == edge.Target {
		return fmt.Errorf("knowledge.Graph.AddEdge: self-loop not allowed (source == target == %q)", edge.Source)
	}

	// Idempotent: do not add duplicate edges.
	if _, exists := g.Edges[edge.ID]; exists {
		return nil
	}

	g.Edges[edge.ID] = edge
	g.BySource[edge.Source] = append(g.BySource[edge.Source], edge)
	g.ByTarget[edge.Target] = append(g.ByTarget[edge.Target], edge)
	return nil
}

// GetOutgoing returns all edges whose Source is nodeID.
// Returns nil (not an error) when the node has no outgoing edges.
func (g *Graph) GetOutgoing(nodeID string) []*Edge {
	return g.BySource[nodeID]
}

// GetIncoming returns all edges whose Target is nodeID.
// Returns nil (not an error) when the node has no incoming edges.
func (g *Graph) GetIncoming(nodeID string) []*Edge {
	return g.ByTarget[nodeID]
}

// RemoveEdge deletes the edge with edgeID from the graph and removes it from
// both BySource and ByTarget indices.
//
// Returns an error if no edge with that ID exists.
func (g *Graph) RemoveEdge(edgeID string) error {
	edge, ok := g.Edges[edgeID]
	if !ok {
		return fmt.Errorf("knowledge.Graph.RemoveEdge: edge %q not found", edgeID)
	}

	delete(g.Edges, edgeID)

	g.BySource[edge.Source] = removeEdgeFromSlice(g.BySource[edge.Source], edgeID)
	g.ByTarget[edge.Target] = removeEdgeFromSlice(g.ByTarget[edge.Target], edgeID)
	return nil
}

// TraverseBFS performs a breadth-first traversal starting from start, visiting
// nodes up to maxDepth hops away.  The starting node itself is NOT included in
// the result.
//
// Returns all reachable Node pointers (unique, stable-order via BFS queue).
// Returns nil when start is unknown or maxDepth < 1.
func (g *Graph) TraverseBFS(start string, maxDepth int) []*Node {
	if maxDepth < 1 {
		return nil
	}
	if _, ok := g.Nodes[start]; !ok {
		return nil
	}

	type entry struct {
		id    string
		depth int
	}

	visited := make(map[string]struct{})
	visited[start] = struct{}{}

	queue := []entry{{id: start, depth: 0}}
	var result []*Node

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxDepth {
			continue
		}

		for _, edge := range g.BySource[cur.id] {
			if _, seen := visited[edge.Target]; seen {
				continue
			}
			visited[edge.Target] = struct{}{}
			if node, ok := g.Nodes[edge.Target]; ok {
				result = append(result, node)
			}
			queue = append(queue, entry{id: edge.Target, depth: cur.depth + 1})
		}
	}

	return result
}

// NodeCount returns the number of nodes currently in the graph.
func (g *Graph) NodeCount() int { return len(g.Nodes) }

// EdgeCount returns the number of edges currently in the graph.
func (g *Graph) EdgeCount() int { return len(g.Edges) }

// RemoveNode deletes the node with the given id from the graph and removes all
// edges where the node is either the source or the target. Both the Edges map
// and the BySource/ByTarget adjacency indices are updated atomically.
//
// It is a no-op if no node with that ID exists.
func (g *Graph) RemoveNode(id string) {
	if _, ok := g.Nodes[id]; !ok {
		return
	}
	delete(g.Nodes, id)

	// Collect edge IDs to remove to avoid mutating the map during iteration.
	var toRemove []string
	for edgeID, e := range g.Edges {
		if e.Source == id || e.Target == id {
			toRemove = append(toRemove, edgeID)
		}
	}

	for _, edgeID := range toRemove {
		_ = g.RemoveEdge(edgeID)
	}
}

// --- graph traversal algorithms (Task 6) ------------------------------------

// TransitiveDeps returns the IDs of all nodes reachable from nodeID by
// following outgoing edges (BFS, no depth limit).
//
// The starting node itself is NOT included.  Returns nil when nodeID is
// unknown or has no outgoing edges.
func (g *Graph) TransitiveDeps(nodeID string) []string {
	if _, ok := g.Nodes[nodeID]; !ok {
		return nil
	}

	visited := make(map[string]struct{})
	visited[nodeID] = struct{}{}

	queue := []string{nodeID}
	var result []string

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, edge := range g.BySource[cur] {
			if _, seen := visited[edge.Target]; seen {
				continue
			}
			visited[edge.Target] = struct{}{}
			result = append(result, edge.Target)
			queue = append(queue, edge.Target)
		}
	}

	return result
}

// FindPaths returns all simple paths between from and to, limited to a maximum
// path length of maxDepth edges.  Paths longer than maxDepth are silently
// truncated / not explored further, preventing combinatorial explosion.
//
// Each path is represented as a slice of node IDs starting with from and
// ending with to.  Returns nil when no path exists or either node is unknown.
func (g *Graph) FindPaths(from, to string, maxDepth int) [][]string {
	if maxDepth < 1 {
		return nil
	}
	if _, ok := g.Nodes[from]; !ok {
		return nil
	}
	if _, ok := g.Nodes[to]; !ok {
		return nil
	}
	if from == to {
		return nil
	}

	var results [][]string
	visited := make(map[string]bool)

	var dfs func(current string, path []string)
	dfs = func(current string, path []string) {
		if len(path)-1 >= maxDepth {
			return
		}
		for _, edge := range g.BySource[current] {
			next := edge.Target
			if visited[next] {
				continue
			}
			newPath := append(append([]string{}, path...), next)
			if next == to {
				results = append(results, newPath)
				continue
			}
			visited[next] = true
			dfs(next, newPath)
			visited[next] = false
		}
	}

	visited[from] = true
	dfs(from, []string{from})
	return results
}

// findShortestPathExcluding finds the single shortest path from `from` to `to`
// using BFS with parent tracking. Edges in excludedEdges and nodes in
// excludedNodes are skipped during traversal.
//
// Time complexity: O(V + E). Returns nil when no path exists.
func (g *Graph) findShortestPathExcluding(from, to string, excludedEdges map[string]bool, excludedNodes map[string]bool) []string {
	if excludedNodes[from] || excludedNodes[to] {
		return nil
	}

	parent := make(map[string]string, len(g.Nodes))
	visited := make(map[string]bool, len(g.Nodes))
	visited[from] = true
	queue := []string{from}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, edge := range g.BySource[cur] {
			next := edge.Target
			if visited[next] || excludedNodes[next] {
				continue
			}
			if excludedEdges[cur+"|"+next] {
				continue
			}
			parent[next] = cur
			if next == to {
				// Reconstruct path from parent chain.
				path := []string{to}
				for node := to; node != from; {
					node = parent[node]
					path = append(path, node)
				}
				// Reverse to get from→to order.
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path
			}
			visited[next] = true
			queue = append(queue, next)
		}
	}

	return nil
}

// FindShortestPath returns the single shortest path from `from` to `to`.
// Uses standard BFS with parent tracking — O(V + E) time, no combinatorial
// explosion regardless of graph density.
//
// Returns nil when no path exists or either node is unknown.
func (g *Graph) FindShortestPath(from, to string) []string {
	if _, ok := g.Nodes[from]; !ok {
		return nil
	}
	if _, ok := g.Nodes[to]; !ok {
		return nil
	}
	if from == to {
		return nil
	}
	return g.findShortestPathExcluding(from, to, nil, nil)
}

// FindKShortestPaths returns up to k shortest simple (loopless) paths between
// from and to using Yen's algorithm. Each iteration finds one more path by
// systematically deviating from previously found paths.
//
// Time complexity: O(k · V · (V + E)) worst case, but typically much faster
// due to early termination. Returns paths in order of increasing length.
//
// Returns nil when no path exists or either node is unknown.
func (g *Graph) FindKShortestPaths(from, to string, k int) [][]string {
	if k < 1 {
		k = 1
	}
	if _, ok := g.Nodes[from]; !ok {
		return nil
	}
	if _, ok := g.Nodes[to]; !ok {
		return nil
	}
	if from == to {
		return nil
	}

	// A[0] = shortest path.
	first := g.findShortestPathExcluding(from, to, nil, nil)
	if first == nil {
		return nil
	}
	results := [][]string{first}

	// B = candidate pool, kept sorted by path length.
	type candidate struct {
		path []string
	}
	var candidates []candidate
	seen := make(map[string]bool)

	joinPath := func(p []string) string {
		key := p[0]
		for _, n := range p[1:] {
			key += "|" + n
		}
		return key
	}
	seen[joinPath(first)] = true

	for i := 1; i < k; i++ {
		prevPath := results[i-1]

		for j := 0; j < len(prevPath)-1; j++ {
			spurNode := prevPath[j]
			rootPath := make([]string, j+1)
			copy(rootPath, prevPath[:j+1])

			// Exclude edges from spurNode that overlap with existing result roots.
			excludedEdges := make(map[string]bool)
			for _, result := range results {
				if len(result) > j && sharesPrefix(rootPath, result) {
					excludedEdges[result[j]+"|"+result[j+1]] = true
				}
			}

			// Exclude root path nodes (except the spur node itself).
			excludedNodes := make(map[string]bool)
			for _, node := range rootPath[:j] {
				excludedNodes[node] = true
			}

			spurPath := g.findShortestPathExcluding(spurNode, to, excludedEdges, excludedNodes)
			if spurPath == nil {
				continue
			}

			// Combine: rootPath + spurPath[1:] (spurNode appears in both).
			totalPath := make([]string, len(rootPath)+len(spurPath)-1)
			copy(totalPath, rootPath)
			copy(totalPath[len(rootPath):], spurPath[1:])

			key := joinPath(totalPath)
			if !seen[key] {
				seen[key] = true
				candidates = append(candidates, candidate{path: totalPath})
			}
		}

		if len(candidates) == 0 {
			break
		}

		// Pick the shortest candidate (ties broken by first found).
		bestIdx := 0
		for ci := 1; ci < len(candidates); ci++ {
			if len(candidates[ci].path) < len(candidates[bestIdx].path) {
				bestIdx = ci
			}
		}

		results = append(results, candidates[bestIdx].path)
		// Remove chosen candidate.
		candidates[bestIdx] = candidates[len(candidates)-1]
		candidates = candidates[:len(candidates)-1]
	}

	return results
}

// sharesPrefix returns true if path starts with the same nodes as prefix.
func sharesPrefix(prefix, path []string) bool {
	if len(path) < len(prefix) {
		return false
	}
	for i := range prefix {
		if prefix[i] != path[i] {
			return false
		}
	}
	return true
}

// DetectCycles returns all cycles in the graph using iterative DFS with
// three-colour marking (white/gray/black).
//
// Each cycle is represented as a slice of node IDs forming a loop; the first
// and last elements are the same node.  Returns nil when the graph is acyclic.
func (g *Graph) DetectCycles() [][]string {
	// Colour constants.
	const (
		white = 0 // unvisited
		gray  = 1 // in current DFS path
		black = 2 // fully processed
	)

	colour := make(map[string]int, len(g.Nodes))
	parent := make(map[string]string, len(g.Nodes))

	var cycles [][]string

	var dfs func(u string)
	dfs = func(u string) {
		colour[u] = gray

		for _, edge := range g.BySource[u] {
			v := edge.Target
			if colour[v] == white {
				parent[v] = u
				dfs(v)
			} else if colour[v] == gray {
				// Back edge found — reconstruct cycle.
				cycle := reconstructCycle(parent, v, u)
				cycles = append(cycles, cycle)
			}
		}

		colour[u] = black
	}

	for id := range g.Nodes {
		if colour[id] == white {
			dfs(id)
		}
	}

	return cycles
}

// GetSubgraph extracts the subgraph reachable from nodeID within maxDepth hops.
// The result is a new Graph containing the start node, all reachable nodes, and
// only the edges that connect nodes within the subgraph.
//
// Returns an empty graph when nodeID is not found.
func (g *Graph) GetSubgraph(nodeID string, maxDepth int) *Graph {
	sub := NewGraph()

	startNode, ok := g.Nodes[nodeID]
	if !ok {
		return sub
	}
	_ = sub.AddNode(startNode)

	if maxDepth < 1 {
		return sub
	}

	type entry struct {
		id    string
		depth int
	}

	visited := make(map[string]struct{})
	visited[nodeID] = struct{}{}
	queue := []entry{{id: nodeID, depth: 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, edge := range g.BySource[cur.id] {
			// Always add the edge if both endpoints will be in the subgraph.
			if _, seen := visited[edge.Target]; !seen {
				if cur.depth < maxDepth {
					visited[edge.Target] = struct{}{}
					if targetNode, ok := g.Nodes[edge.Target]; ok {
						_ = sub.AddNode(targetNode)
					}
					queue = append(queue, entry{id: edge.Target, depth: cur.depth + 1})
				}
			}
			// Only add the edge when the target is already in the subgraph.
			if _, inSub := visited[edge.Target]; inSub {
				_ = sub.AddEdge(edge)
			}
		}
	}

	return sub
}

// --- Cycle-aware traversal ---------------------------------------------------

// TraverseDFS performs a depth-first traversal starting from startNodeID,
// descending at most maxDepth levels.
//
// It uses a visited set to prevent re-processing nodes and an ancestor path
// to detect back-edges (cycles). Cyclic edges are included in results with
// RelationshipType set to EdgeCyclicDependency; all other edges are marked
// EdgeDirectDependency.
//
// Returns the TraversalState (with detected cycles) and a slice of *Edge
// copies representing every edge encountered during the walk.
//
// Returns (nil, nil) when startNodeID is not in the graph.
func (g *Graph) TraverseDFS(startNodeID string, maxDepth int) (*TraversalState, []*Edge) {
	if _, ok := g.Nodes[startNodeID]; !ok {
		return nil, nil
	}

	ts := NewTraversalState()
	var resultEdges []*Edge

	var dfs func(nodeID string)
	dfs = func(nodeID string) {
		ts.MarkVisited(nodeID)
		ts.AddPathNode(nodeID)

		if !ts.AtMaxDepth(maxDepth) {
			for _, edge := range g.BySource[nodeID] {
				// Copy the edge so we don't mutate shared graph state.
				eCopy := *edge
				if ts.IsInPath(edge.Target) {
					// Back-edge: would close a cycle.
					eCopy.RelationshipType = EdgeCyclicDependency
					resultEdges = append(resultEdges, &eCopy)
					// Record the cycle path.
					cyclePath := make([]string, len(ts.Path)+1)
					copy(cyclePath, ts.Path)
					cyclePath[len(cyclePath)-1] = edge.Target
					ts.RecordCycle(cyclePath)
				} else {
					eCopy.RelationshipType = EdgeDirectDependency
					resultEdges = append(resultEdges, &eCopy)
					if !ts.HasVisited(edge.Target) {
						ts.Depth++
						dfs(edge.Target)
						ts.Depth--
					}
				}
			}
		}

		ts.RemovePathNode()
	}

	dfs(startNodeID)
	return ts, resultEdges
}

// GetImpact returns the edges reachable from nodeID, defaulting to direct
// edges only (maxDepth=1). Set maxDepth > 1 for transitive impact analysis.
//
// This is the primary entry point for impact queries. Each edge in the result
// has its RelationshipType set to indicate whether it is a direct or cyclic
// dependency.
func (g *Graph) GetImpact(nodeID string, maxDepth int) []*Edge {
	if maxDepth < 1 {
		maxDepth = 1
	}
	_, edges := g.TraverseDFS(nodeID, maxDepth)
	return edges
}

// --- GraphBuilder ------------------------------------------------------------

// GraphBuilder constructs a Graph from a collection of Documents by running
// the Extractor on each document and merging the resulting edges.
//
// Deduplication rule: when two edges share the same source, target, and type,
// the one with the higher Confidence score is kept.
type GraphBuilder struct {
	extractor *Extractor
}

// NewGraphBuilder creates a GraphBuilder whose Extractor is rooted at root.
// root must be the same directory passed to ScanDirectory so that relative
// link paths are resolved correctly.
func NewGraphBuilder(root string) *GraphBuilder {
	return &GraphBuilder{extractor: NewExtractor(root)}
}

// Build extracts relationships from each document in documents, creates a Node
// for every document, and returns the populated Graph.
//
// The method is safe to call with an empty slice; it returns an empty Graph.
func (gb *GraphBuilder) Build(documents []Document) *Graph {
	g := NewGraph()

	for i := range documents {
		doc := &documents[i]

		// Create a node for each document.
		node := &Node{
			ID:    doc.ID,
			Title: doc.Title,
			Type:  "document",
		}
		_ = g.AddNode(node) // error only on nil/empty ID which cannot happen here

		// Extract edges from this document.
		edges := gb.extractor.Extract(doc)

		for _, edge := range edges {
			gb.mergeEdge(g, edge)
		}
	}

	return g
}

// mergeEdge adds edge to g, replacing any existing edge with a lower
// confidence score.  If an edge with the same ID already exists and has a
// higher or equal confidence, the new edge is discarded.
func (gb *GraphBuilder) mergeEdge(g *Graph, edge *Edge) {
	if existing, ok := g.Edges[edge.ID]; ok {
		if existing.Confidence >= edge.Confidence {
			return // keep the higher-confidence version
		}
		// Replace: remove the old edge first.
		_ = g.RemoveEdge(edge.ID)
	}
	_ = g.AddEdge(edge)
}

// --- internal helpers --------------------------------------------------------

// removeEdgeFromSlice returns a new slice with the edge identified by edgeID
// removed.  The original slice is not modified.
func removeEdgeFromSlice(edges []*Edge, edgeID string) []*Edge {
	result := edges[:0:0]
	for _, e := range edges {
		if e.ID != edgeID {
			result = append(result, e)
		}
	}
	return result
}

// reconstructCycle builds a cycle path starting and ending at cycleRoot by
// following the parent map backwards from tail.
func reconstructCycle(parent map[string]string, cycleRoot, tail string) []string {
	// Walk back from tail to cycleRoot.
	path := []string{cycleRoot}
	cur := tail
	for cur != cycleRoot {
		path = append([]string{cur}, path...)
		p, ok := parent[cur]
		if !ok {
			break
		}
		cur = p
	}
	// Close the cycle.
	path = append(path, cycleRoot)
	return path
}
