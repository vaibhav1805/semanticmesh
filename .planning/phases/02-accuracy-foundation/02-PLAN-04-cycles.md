# Phase 2, Plan 4: Cycle Detection & Safety (Wave 3)

**Frontmatter:**
```
phase: 2
plan: 04-cycles
wave: 3
depends_on: [02-01]
blocks: [02-05]
autonomous: true
files_modified:
  - internal/knowledge/graph.go
  - internal/knowledge/traversal.go (NEW)
  - internal/knowledge/types.go
  - internal/knowledge/*_test.go
```

---

## Plan Summary

**Requirement:** REL-03 (Handle circular dependencies without infinite loops using visited set tracking in traversal)

**Goal:** Implement cycle-safe graph traversal ensuring all queries (crawl, impact) complete without infinite loops, even with complex circular dependencies (A→B→C→A).

**Scope:**
- Design TraversalState struct with visited set + path tracking
- Implement DFS-based traversal with cycle detection
- Mark cyclic edges with relationship_type field
- Ensure queries complete in < 1 second on test data

**Depends on:** Plan 1 (aggregation foundation)

---

## Tasks

### Task 1: Design TraversalState & Cycle Detection

**Action:** Create cycle detection infrastructure in new `traversal.go` file.

**Specific steps:**
1. Create TraversalState struct:
   ```go
   type TraversalState struct {
       Visited map[string]bool  // node ID → visited
       Path    []string         // current DFS path
       Cycles  [][]string       // detected cycles (path rings)
   }

   func NewTraversalState() *TraversalState {
       return &TraversalState{
           Visited: make(map[string]bool),
           Path:    []string{},
           Cycles:  [][]string{},
       }
   }
   ```

2. Implement cycle detection methods:
   ```go
   // WouldCreateCycle checks if adding edge (from → to) closes a cycle
   func (ts *TraversalState) WouldCreateCycle(from, to string) bool {
       for _, node := range ts.Path {
           if node == to {
               return true  // 'to' is ancestor in current path
           }
       }
       return false
   }

   // VisitNode marks node visited and adds to path
   func (ts *TraversalState) VisitNode(nodeID string) {
       ts.Visited[nodeID] = true
       ts.Path = append(ts.Path, nodeID)
   }

   // UnvisitNode removes from path (backtrack in DFS)
   func (ts *TraversalState) UnvisitNode(nodeID string) {
       if len(ts.Path) > 0 && ts.Path[len(ts.Path)-1] == nodeID {
           ts.Path = ts.Path[:len(ts.Path)-1]
       }
   }

   // RecordCycle stores detected cycle
   func (ts *TraversalState) RecordCycle(cycle []string) {
       ts.Cycles = append(ts.Cycles, cycle)
   }
   ```

3. Document cycle detection semantics:
   - Cycles are valid real-world patterns (not errors)
   - Include cyclic edges in all results
   - Mark edges as cyclic-dependency for visibility

**Verification:**
- Code compiles
- WouldCreateCycle correctly identifies ancestors in path
- Visited set prevents revisiting nodes in same traversal

---

### Task 2: Extend Edge Struct with Relationship Type

**Action:** Modify Edge struct in `types.go` to track cyclic vs. direct relationships.

**Specific steps:**
1. Update Edge struct:
   ```go
   type Edge struct {
       // existing fields...
       RelationshipType string  // "direct-dependency" (default), "cyclic-dependency"
   }
   ```

2. Add enum for RelationshipType:
   ```go
   type RelationshipType string

   const (
       RelDirect RelationshipType = "direct-dependency"
       RelCyclic RelationshipType = "cyclic-dependency"
   )
   ```

3. Update Edge validation:
   - Default RelationshipType = RelDirect
   - Only valid values: direct-dependency, cyclic-dependency

4. Update NewEdge():
   - Accept optional RelationshipType parameter
   - Default to RelDirect

**Verification:**
- Code compiles
- Edge.RelationshipType defaults correctly
- Invalid types rejected

---

### Task 3: Implement Cycle-Safe Traversal in Graph

**Action:** Update Graph.TraverseBFS() in `graph.go` to use TraversalState and detect cycles.

**Specific steps:**
1. Rename/extend TraverseBFS to TraverseWithCycleDetection:
   ```go
   func (g *Graph) TraverseWithCycleDetection(startNode string, maxDepth int) (*TraversalState, []*Edge, error) {
       ts := NewTraversalState()
       edges := []*Edge{}

       // BFS/DFS traversal with cycle detection
       // For each edge from startNode:
       //   if edge.Target in ts.Path → mark as cyclic-dependency
       //   else → recurse with depth limit

       return ts, edges, nil
   }
   ```

2. Implement traversal logic:
   ```go
   func (g *Graph) traverse(state *TraversalState, nodeID string, depth int, maxDepth int, results *[]*Edge) {
       if depth > maxDepth {
           return
       }

       state.VisitNode(nodeID)

       for _, edge := range g.BySource[nodeID] {
           newEdge := *edge

           if state.WouldCreateCycle(nodeID, edge.Target) {
               newEdge.RelationshipType = RelCyclic
           }

           *results = append(*results, &newEdge)

           if !state.Visited[edge.Target] {
               g.traverse(state, edge.Target, depth+1, maxDepth, results)
           }
       }

       state.UnvisitNode(nodeID)
   }
   ```

3. Performance:
   - Use visited set to avoid O(n²) revisits
   - Limit depth to prevent exponential traversal
   - Test on 62-document corpus; expect < 1 second for depth=3

**Verification:**
- Traversal completes without infinite loops
- Cycles correctly marked as cyclic-dependency
- Performance meets < 1 second target

---

### Task 4: Add Cyclic Edge Detection Tests

**Action:** Create `traversal_test.go` with cycle detection unit tests.

**Specific steps:**
1. Test functions:
   ```go
   func TestTraversalState_WouldCreateCycle_Simple()      // A→B→A
   func TestTraversalState_WouldCreateCycle_Complex()     // A→B→C→A
   func TestTraversalState_WouldCreateCycle_NoFalsePos()  // Unrelated nodes
   func TestTraversalState_VisitUnvisit()                 // Path tracking
   func TestTraverse_MarksCyclicEdges()                   // Cycle marked in results
   func TestTraverse_IncludesCyclicEdges()                // Cyclic edges not skipped
   func TestTraverse_CompletesWithComplexCycles()         // Large cyclic graph
   ```

2. Example test:
   ```go
   // Create graph: A→B, B→C, C→A
   g := NewGraph()
   g.AddNode("A", "service")
   g.AddNode("B", "service")
   g.AddNode("C", "service")
   g.AddEdge("A", "B", "depends", 0.8, "")
   g.AddEdge("B", "C", "depends", 0.8, "")
   g.AddEdge("C", "A", "depends", 0.8, "")

   state, edges, _ := g.TraverseWithCycleDetection("A", 3)

   // Verify: A→B is direct, B→C is direct, C→A is cyclic
   cyclic := edges[2]
   assert.Equal(t, RelCyclic, cyclic.RelationshipType)
   ```

3. Benchmark test:
   ```go
   func BenchmarkTraverse_ComplexCycles(b *testing.B) {
       // Load test-data corpus, measure traversal time
       // Expected: < 1 second per query
   }
   ```

**Verification:**
- All tests pass
- Coverage >= 85% for traversal.go
- Benchmark shows < 1 second on test corpus

---

### Task 5: Integration Test with Cyclic Test-Data

**Action:** Create integration test using real cyclic patterns from test-data.

**Specific steps:**
1. Create test in `graph_test.go`:
   ```go
   func TestIntegration_CycleHandling_WithTestData()
   ```

2. Test flow:
   - Load test-data corpus
   - Build graph from relationships
   - Identify natural cycles (A→B→C→A patterns)
   - Run TraverseWithCycleDetection on cyclic node
   - Measure execution time (assert < 1 second)
   - Verify cyclic edges marked
   - Verify all edges included in results

3. Report:
   - Number of cycles detected
   - Longest cycle length
   - Traversal execution time
   - Memory usage

**Verification:**
- Integration test passes
- Cycles detected in test-data
- Performance acceptable (< 1 second)
- All edges included (not skipped)

---

### Task 6: Document Cycle Handling for Agents

**Action:** Add comments and documentation explaining cycle semantics.

**Specific steps:**
1. Add code comments:
   ```go
   // Cycles are valid real-world patterns. Example: A→B→C→A represents
   // mutual dependencies where C's response calls back to A.
   // Every edge is included in results, marked by RelationshipType field.
   ```

2. Add to CONTEXT.md or user docs:
   ```markdown
   ## Cycle Handling

   Circular dependencies are supported and marked as `relationship_type: cyclic-dependency`.
   Agents see all edges including cycles, allowing them to understand mutual dependencies.
   ```

3. Update CLI help:
   ```
   Cyclic dependencies (A→B→C→A) are included in all query results
   and marked with relationship_type=cyclic-dependency.
   ```

**Verification:**
- Comments clear and complete
- Documentation explains cycle patterns
- Help text visible to users

---

## Verification Criteria

**What "done" looks like:**

1. ✓ **TraversalState implemented:** Visited set + path tracking. WouldCreateCycle works correctly.

2. ✓ **Cycle-safe traversal:** TraverseWithCycleDetection completes without infinite loops. Cyclic edges marked.

3. ✓ **Performance target met:** Traversal on test-data completes in < 1 second. Benchmark shows acceptable time.

4. ✓ **Unit tests pass:** traversal_test.go covers simple, complex, and edge cases. Coverage >= 85%.

5. ✓ **Integration test validates:** Test-data corpus produces cycles correctly. All edges included. Execution time < 1 second.

6. ✓ **Edge struct updated:** RelationshipType field added. Cyclic edges marked in results.

7. ✓ **No regressions:** Phase 1 tests still pass. Plans 1–3 still work.

8. ✓ **Documentation complete:** Code comments explain cycle semantics. Help text visible. User docs describe cycle handling.

---

## Must-Haves (Phase Goal Backward)

From phase goal: "prevents false edges from misleading agents during incident response"

This plan delivers:
- ✓ Cycle-safe traversal (agents never stuck in infinite loops)
- ✓ Cycle visibility (cyclic edges marked and included)
- ✓ Performance guarantee (< 1 second for realistic graphs)

---

## Dependency Notes

**Depends on:** Plan 1 (aggregation foundation)

**Enables:** Plan 5 (query interface uses cycle-safe traversal)

---

*Plan created: 2026-03-19*
*Autonomous execution: Yes*
