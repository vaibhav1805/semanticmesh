---
phase: 02-accuracy-foundation
plan: 04
type: execute
wave: 2
depends_on:
  - 02-03
files_modified:
  - internal/knowledge/types.go
  - internal/knowledge/graph.go
  - internal/knowledge/graph_test.go
autonomous: true
requirements:
  - REL-03
  - REL-04

must_haves:
  truths:
    - "Graph traversal completes without infinite loops on cyclic graphs (A→B→C→A)"
    - "Cyclic edges are detected and marked with relationship_type field"
    - "Every traversal uses visited set to track already-processed nodes"
    - "Query results include all edges (direct and cyclic), not filtered out"
    - "Impact query returns direct edges by default; transitive traversal optional"
  artifacts:
    - path: "internal/knowledge/types.go"
      provides: "TraversalState struct; updated Edge struct with RelationshipType field"
      exports: ["TraversalState", "EdgeType", "EdgeDirectDependency", "EdgeCyclicDependency"]
      min_lines: 30
    - path: "internal/knowledge/graph.go"
      provides: "Updated Graph.TraverseBFS() with visited set; cycle detection logic"
      exports: ["Graph.TraverseBFS", "Graph.TraverseDFS", "Graph.GetImpact"]
      min_lines: 100
    - path: "internal/knowledge/graph_test.go"
      provides: "Cycle detection and traversal tests"
      exports: ["TestTraversalCyclicGraph", "TestTraversalVisitedSet"]
  key_links:
    - from: "Graph.TraverseBFS() or TraverseDFS()"
      to: "TraversalState visited set"
      via: "visited map tracks processed nodes to prevent infinite loops"
      pattern: "visited\\[node\\]|TraversalState.*visited"
    - from: "Cycle detection"
      to: "Edge.RelationshipType field"
      via: "Mark edges with EdgeCyclicDependency when path would create cycle"
      pattern: "RelationshipType.*cyclic|WouldCreateCycle"
    - from: "Query interface (future)"
      to: "direct-edge-only default"
      via: "--depth 1 returns only distance=1 edges; depth > 1 enables transitive"
      pattern: "depth.*==.*1|direct.*only"

---

<objective>
Implement cycle detection and visited-set tracking in graph traversal to prevent infinite loops while preserving cyclic edges in results. Mark cyclic relationships explicitly and support depth-controlled traversal (direct edges by default, transitive with opt-in).

Purpose: REL-03 and REL-04 requirements; ensures safety on real-world cyclic graphs and enforces direct-edge-only results by default (preventing false cascade reporting).

Output: TraversalState struct with visited tracking, updated Graph traversal methods, cycle marking in edges, depth-controlled impact queries.
</objective>

<execution_context>
@/Users/flurryhead/.claude/get-shit-done/workflows/execute-plan.md
@/Users/flurryhead/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/phases/02-accuracy-foundation/02-CONTEXT.md
@.planning/phases/02-accuracy-foundation/02-RESEARCH.md
@internal/knowledge/types.go
@internal/knowledge/graph.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Define TraversalState and cycle detection helpers</name>
  <files>internal/knowledge/types.go</files>
  <action>
Add to types.go:

1. Define TraversalState struct:
   ```go
   type TraversalState struct {
       Visited map[string]bool  // node ID → true if visited
       Path    []string         // current DFS path (for cycle detection)
       Cycles  [][]string       // detected cycles (slices of node IDs)
       Depth   int              // current traversal depth (for depth limiting)
   }
   ```

2. Add methods to TraversalState:
   - func (ts *TraversalState) HasVisited(nodeID string) bool — returns true if node already in visited set
   - func (ts *TraversalState) MarkVisited(nodeID string) — adds node to visited
   - func (ts *TraversalState) IsInPath(nodeID string) bool — checks if node is in current DFS path (ancestor)
   - func (ts *TraversalState) AddPathNode(nodeID string) — appends to current path
   - func (ts *TraversalState) RemovePathNode(nodeID string) — pops from current path
   - func (ts *TraversalState) RecordCycle(cycle []string) — appends to cycles list
   - func (ts *TraversalState) AtMaxDepth(maxDepth int) bool — returns true if Depth >= maxDepth

3. Add EdgeType constants for relationship marking (if not already exist):
   - const EdgeDirectDependency EdgeType = "direct-dependency"
   - const EdgeCyclicDependency EdgeType = "cyclic-dependency"

4. Update Edge struct (from Plan 3) to include RelationshipType field:
   ```go
   type Edge struct {
       // existing fields including provenance
       RelationshipType string  // "direct-dependency" (default) or "cyclic-dependency"
   }
   ```

5. Update NewEdge() to initialize RelationshipType = "direct-dependency" (default)

6. Add comment explaining: "RelationshipType is marked cyclic-dependency when edge would create cycle in DFS traversal"
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestTraversalState -v
Expected:
  - TestTraversalState_MarkVisited: HasVisited returns true after MarkVisited
  - TestTraversalState_IsInPath: IsInPath correctly tracks DFS path
  - TestTraversalState_PathManagement: AddPathNode and RemovePathNode work correctly
  - TestTraversalState_DepthLimiting: AtMaxDepth returns true at or above max

Also run: go build ./cmd/graphmd — no syntax errors
  </verify>
  <done>
- TraversalState struct defined with visited map, path tracking, cycles list, depth counter
- All helper methods implemented: HasVisited, MarkVisited, IsInPath, AddPathNode, RemovePathNode, RecordCycle, AtMaxDepth
- Edge.RelationshipType field added for cycle marking
- EdgeDirectDependency and EdgeCyclicDependency constants defined
- Default RelationshipType = "direct-dependency"
  </done>
</task>

<task type="auto">
  <name>Task 2: Implement cycle detection in graph traversal</name>
  <files>internal/knowledge/graph.go</files>
  <action>
Update Graph traversal methods:

1. Find existing Graph.TraverseBFS() or Graph.TraverseDFS() method (research shows BFS exists)

2. Replace or enhance with new traversal logic:
   ```go
   // Sketch of updated TraverseDFS with cycle detection
   func (g *Graph) TraverseDFS(startNodeID string, maxDepth int) (*TraversalState, []Edge) {
       ts := &TraversalState{
           Visited: make(map[string]bool),
           Path:    []string{},
           Cycles:  [][]string{},
           Depth:   0,
       }
       resultEdges := []*Edge{}

       var dfs func(nodeID string)
       dfs = func(nodeID string) {
           if ts.HasVisited(nodeID) {
               // Already processed from different path; skip to avoid infinite loop
               return
           }
           ts.MarkVisited(nodeID)
           ts.AddPathNode(nodeID)

           if ts.AtMaxDepth(maxDepth) {
               ts.RemovePathNode(nodeID)
               return
           }

           // Traverse outgoing edges
           for _, edge := range g.BySource[nodeID] {
               if ts.IsInPath(edge.Target) {
                   // Would create cycle
                   edge.RelationshipType = EdgeCyclicDependency
                   resultEdges = append(resultEdges, edge)
                   ts.RecordCycle(append(ts.Path, edge.Target))
               } else {
                   // Normal edge
                   edge.RelationshipType = EdgeDirectDependency
                   resultEdges = append(resultEdges, edge)
                   ts.Depth++
                   dfs(edge.Target)
                   ts.Depth--
               }
           }

           ts.RemovePathNode(nodeID)
       }

       dfs(startNodeID)
       return ts, resultEdges
   }
   ```

3. Add documentation comment explaining:
   - Visited set prevents re-processing same node from different paths
   - Path tracking detects cycles (if target is in current path, adding edge creates cycle)
   - Cyclic edges are included in results with RelationshipType = cyclic-dependency
   - maxDepth limits traversal depth (1 = direct only, 2 = direct + 1 hop, etc.)

4. Ensure existing Graph methods (TraverseBFS, BySource, ByTarget maps) are compatible; no breaking changes

5. If TraverseBFS exists, update it similarly; or implement TraverseDFS as new primary method

Reference RESEARCH.md section 2.4 for algorithm details.
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestTraversal -v
Expected:
  - TestTraversalAcyclic: Linear path A→B→C completes without cycles
  - TestTraversalCyclic: A→B→C→A marks edges correctly, completes in < 1 second
  - TestTraversalVisitedSet: No infinite loops; visited set prevents reprocessing
  - TestTraversalDepthLimiting: maxDepth=1 returns only direct edges; maxDepth=2 includes one hop

Manual check: go build ./cmd/graphmd — no syntax errors
  </verify>
  <done>
- Graph.TraverseDFS (or updated TraverseBFS) uses TraversalState with visited set
- Cycle detection identifies when edge would close a cycle
- Cyclic edges marked with RelationshipType = cyclic-dependency
- Direct edges marked with RelationshipType = direct-dependency (default)
- Traversal completes without infinite loops even on cyclic graphs
- Depth limiting enforced via maxDepth parameter
  </done>
</task>

<task type="auto">
  <name>Task 3: Unit and integration tests for cycle detection and depth-limited traversal</name>
  <files>internal/knowledge/graph_test.go</files>
  <action>
Create comprehensive test coverage:

1. Unit tests for cycle detection:
   - TestTraversalCyclicGraph_Simple: A→B→C→A cycles correctly
     - Setup: Create nodes A, B, C; edges A→B, B→C, C→A
     - Call TraverseDFS(A, maxDepth=10)
     - Assert: Result includes all 3 edges; C→A marked as cyclic-dependency
     - Assert: Execution time < 1 second (no infinite loop)

   - TestTraversalCyclicGraph_SelfLoop: A→A self-loop
     - Setup: Node A with edge A→A
     - Call TraverseDFS(A)
     - Assert: Edge marked as cyclic-dependency; traversal completes

   - TestTraversalCyclicGraph_ComplexCycle: A→B→D→A, C→D (diamond with cycle)
     - Setup: 4 nodes, edges forming complex cycle
     - Assert: All edges included; A→... paths include cyclic edge D→A marked correctly

2. Depth-limiting tests:
   - TestTraversalDepthOne: maxDepth=1 returns only direct edges
     - Setup: A→B→C→D chain
     - Call TraverseDFS(A, maxDepth=1)
     - Assert: Result contains only A→B edge; B→C not included

   - TestTraversalDepthTwo: maxDepth=2 includes one hop
     - Setup: Same chain A→B→C→D
     - Call TraverseDFS(A, maxDepth=2)
     - Assert: Result contains A→B and B→C; C→D not included

   - TestTraversalUnlimitedDepth: maxDepth=999 traverses entire graph
     - Setup: Chain A→B→C→D
     - Call TraverseDFS(A, maxDepth=999)
     - Assert: All 3 edges included

3. Visited set tests:
   - TestTraversalVisitedSet_Diamond: A→B→D, A→C→D (diamond graph)
     - Setup: Node D reachable from A via two paths
     - Call TraverseDFS(A)
     - Assert: D visited once (not twice); traversal efficient

4. Integration test with test-data corpus:
   - TestTraversalOnCorpus: Load test-data, run traversal on real service graph
   - Assert: No crashes, all nodes reachable, cycles (if any) properly marked

Run tests: go test ./internal/knowledge -run TestTraversal -v -count=3
Expected: All tests pass, including repetition (deterministic)
Benchmark: go test ./internal/knowledge -bench BenchmarkTraversal -benchmem
Expected: Large cyclic graphs (1000+ nodes) traverse in < 500ms

Run with: go test ./internal/knowledge -run TestTraversal -v -cover
Expected: graph.go traversal code >85% coverage
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestTraversal -v
Expected: All tests pass
Run: go test ./internal/knowledge -run TestCyclic -v
Expected: Cyclic graph tests pass, execution < 1 second per graph
Run coverage: go test ./internal/knowledge -cover | grep "graph.go"
Expected: >85% coverage on cycle detection and traversal code
  </verify>
  <done>
- Cycle detection tests pass (simple, self-loop, complex cycles)
- Depth limiting tests pass (maxDepth=1 → direct only, higher → transitive)
- Visited set prevents infinite loops and ensures efficiency
- Real corpus traversal works without crashes
- Execution time for cyclic graphs < 1 second
- >85% code coverage on traversal implementation
  </done>
</task>

</tasks>

<verification>
**Comprehensive checks before marking complete:**

1. **Cycle Completeness:** Simple cycle A→B→C→A is detected; all edges included in results; execution completes in < 1 second.

2. **Visited Set Correctness:** Diamond graph A→{B,C}→D: D is visited once, not twice. Traversal is efficient and correct.

3. **Depth Limiting:**
   - maxDepth=1 returns only direct edges (distance=1)
   - maxDepth=2 returns direct + 1 hop (distance ≤ 2)
   - No false edges at distances beyond maxDepth

4. **Relationship Type Marking:** Cyclic edges marked with RelationshipType = cyclic-dependency; direct edges marked as direct-dependency. No unmarked edges.

5. **No Breaking Changes:** Existing Graph methods still work; TraversalState is new structure (additive); Edge.RelationshipType has default value.

6. **Performance:** Cyclic graph with 100+ nodes traverses in < 500ms; scales to 1000+ nodes in < 2 seconds.
</verification>

<success_criteria>
- REL-03 and REL-04 satisfied: Cycle detection and direct-edge-only semantics
- TraversalState struct tracks visited nodes, current DFS path, detected cycles, depth
- Graph traversal uses visited set to prevent infinite loops on cyclic graphs
- Cyclic edges marked with RelationshipType = cyclic-dependency
- Direct edges marked with RelationshipType = direct-dependency (default)
- maxDepth parameter controls traversal scope (1 = direct only, higher = transitive)
- Cyclic graphs complete traversal in < 1 second without hangs
- Unit tests pass with 85%+ coverage on cycle detection logic
- No breaking changes to Phase 1/3 Edge or Graph structures
</success_criteria>

<output>
After completion, create `.planning/phases/02-accuracy-foundation/02-04-SUMMARY.md` documenting:
- TraversalState struct and cycle detection algorithm
- Depth limiting in traversal (maxDepth parameter)
- Relationship type marking (direct vs cyclic)
- Test results for cyclic and acyclic graphs
- Performance characteristics on large graphs
</output>
