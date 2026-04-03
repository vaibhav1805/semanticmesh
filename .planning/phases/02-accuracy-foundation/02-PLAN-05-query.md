# Phase 2, Plan 5: Query Interface (Wave 3)

**Frontmatter:**
```
phase: 2
plan: 05-query
wave: 3
depends_on: [02-01, 02-02, 02-03, 02-04]
blocks: []
autonomous: true
files_modified:
  - internal/knowledge/query.go (NEW)
  - internal/knowledge/graph.go
  - cmd/graphmd/main.go
  - internal/knowledge/*_test.go
```

---

## Plan Summary

**Requirement:** REL-04 (Avoid transitive closure misinterpretation; direct edges only in impact queries by default)

**Goal:** Implement query interface supporting both exploratory (crawl) and focused (impact) traversal modes with configurable depth/tier filtering. All queries return JSON with complete subgraph topology.

**Scope:**
- Implement crawl query (exploratory, all relationships, no filtering)
- Implement impact query (focused, depth-limited, confidence filtering)
- Support --depth, --traverse-mode (direct|cascade|full), --min-tier, --min-confidence flags
- Return consistent JSON schema with subgraph nodes + edges

**Depends on:** Plans 1–4 (aggregation, tiers, provenance, cycles)

---

## Tasks

### Task 1: Define Query Enums & Structs

**Action:** Create query types in new `query.go` file.

**Specific steps:**
1. Define TraverseMode enum:
   ```go
   type TraverseMode string

   const (
       TraverseDirect  TraverseMode = "direct"   // distance=1 only
       TraverseCascade TraverseMode = "cascade"  // distance <= depth
       TraverseFull    TraverseMode = "full"     // full transitive closure
   )
   ```

2. Define QueryResult struct:
   ```go
   type QueryResult struct {
       Query         string                 `json:"query"`      // "impact", "crawl", "dependencies", "path"
       Root          string                 `json:"root"`       // starting component
       Depth         int                    `json:"depth,omitempty"`
       TraverseMode  TraverseMode           `json:"traverse_mode,omitempty"`
       AffectedNodes []QueryNode            `json:"affected_nodes"`
       Edges         []QueryEdge            `json:"edges"`
       Metadata      map[string]interface{} `json:"metadata"`   // query stats
   }

   type QueryNode struct {
       Name              string  `json:"name"`
       Type              string  `json:"type"`
       Confidence        float64 `json:"confidence"`
       RelationshipType  string  `json:"relationship_type"`
       Distance          int     `json:"distance"`
   }

   type QueryEdge struct {
       From              string  `json:"from"`
       To                string  `json:"to"`
       Confidence        float64 `json:"confidence"`
       Type              string  `json:"type"`
       RelationshipType  string  `json:"relationship_type"`
       Evidence          string  `json:"evidence"`
       SourceFile        string  `json:"source_file"`
       ExtractionMethod  string  `json:"extraction_method"`
       EvidencePointer   string  `json:"evidence_pointer,omitempty"`
       SignalsCount      int     `json:"signals_count"`
   }
   ```

3. Add QueryOptions struct:
   ```go
   type QueryOptions struct {
       MinConfidenceScore float64
       MinConfidenceTier  ConfidenceTier
       MaxDepth           int
       TraverseMode       TraverseMode
       IncludeMetadata    bool
   }
   ```

**Verification:**
- Code compiles
- Structs are JSON-serializable
- Enum values are documented

---

### Task 2: Implement Impact Query Logic

**Action:** Add Impact() method to Graph in `query.go`.

**Specific steps:**
1. Implement Impact query:
   ```go
   func (g *Graph) Impact(root string, opts QueryOptions) (*QueryResult, error) {
       // Validate root node exists
       if _, ok := g.Nodes[root]; !ok {
           return nil, fmt.Errorf("node not found: %s", root)
       }

       // Use TraverseWithCycleDetection with depth limit
       maxDepth := opts.MaxDepth
       if maxDepth == 0 {
           maxDepth = 2  // default depth=2 (direct + 1 hop)
       }

       state, edges, _ := g.TraverseWithCycleDetection(root, maxDepth)

       // Filter by confidence tier/score
       filtered := g.filterEdgesByConfidence(edges, opts)

       // Apply traverse mode (direct only vs cascade vs full)
       modeFiltered := g.filterEdgesByTraverseMode(filtered, opts.TraverseMode, maxDepth)

       // Build result
       return g.buildQueryResult("impact", root, modeFiltered, state, opts), nil
   }
   ```

2. Helper: confidence filtering
   ```go
   func (g *Graph) filterEdgesByConfidence(edges []*Edge, opts QueryOptions) []*Edge {
       minScore := opts.MinConfidenceScore
       if opts.MinConfidenceTier != "" {
           tierScore := TierToScore(opts.MinConfidenceTier)
           if tierScore > minScore {
               minScore = tierScore
           }
       }

       filtered := []*Edge{}
       for _, e := range edges {
           if e.Confidence >= minScore {
               filtered = append(filtered, e)
           }
       }
       return filtered
   }
   ```

3. Helper: traverse mode filtering
   ```go
   func (g *Graph) filterEdgesByTraverseMode(edges []*Edge, mode TraverseMode, maxDepth int) []*Edge {
       // For cascade (default), include all edges up to maxDepth
       // For direct, keep only distance=1 edges
       // For full, include all (no filtering by mode)
       // Note: distance calculated during traversal
   }
   ```

4. Document defaults:
   - Default depth=2 (direct + 1 hop, balances completeness vs. info overload)
   - Default traverse_mode=cascade
   - Default min_confidence=threshold (include all)

**Verification:**
- Impact() compiles and runs
- Filters correctly by confidence, traverse mode, depth
- Returns results with proper structure

---

### Task 3: Implement Crawl Query Logic

**Action:** Add Crawl() method to Graph in `query.go`.

**Specific steps:**
1. Implement Crawl query (exploratory, no filtering):
   ```go
   func (g *Graph) Crawl(root string, opts QueryOptions) (*QueryResult, error) {
       // Crawl: all relationships, full depth, no confidence filtering
       // Agents see everything, judge trustworthiness themselves

       maxDepth := opts.MaxDepth
       if maxDepth == 0 {
           maxDepth = 10  // deep exploration (crawl more than impact)
       }

       state, edges, _ := g.TraverseWithCycleDetection(root, maxDepth)

       // NO filtering; return all edges with full provenance/confidence

       return g.buildQueryResult("crawl", root, edges, state, opts), nil
   }
   ```

2. Semantic difference from Impact:
   - Crawl: exploratory, returns all relationships with full confidence/provenance
   - Impact: focused incident response, filtered by confidence threshold
   - Both return JSON with subgraph topology

3. Document use case:
   - Crawl: "show me everything related to this component"
   - Impact: "if this component fails, what breaks? (high confidence only)"

**Verification:**
- Crawl() compiles and runs
- Returns unfiltered results
- Includes all provenance fields

---

### Task 4: Implement JSON Result Builder

**Action:** Add buildQueryResult() helper to create consistent JSON output.

**Specific steps:**
1. Implement result builder:
   ```go
   func (g *Graph) buildQueryResult(queryType, root string, edges []*Edge, state *TraversalState, opts QueryOptions) *QueryResult {
       // Extract unique affected nodes from edges
       affected := g.extractAffectedNodes(edges, state)

       // Build QueryEdges with full provenance
       queryEdges := g.edgesToQueryEdges(edges)

       // Add metadata
       meta := map[string]interface{}{
           "total_edges": len(queryEdges),
           "total_nodes": len(affected),
           "cycles_detected": len(state.Cycles),
       }

       return &QueryResult{
           Query:         queryType,
           Root:          root,
           Depth:         opts.MaxDepth,
           TraverseMode:  opts.TraverseMode,
           AffectedNodes: affected,
           Edges:         queryEdges,
           Metadata:      meta,
       }
   }
   ```

2. Helper: extract affected nodes with distance:
   ```go
   func (g *Graph) extractAffectedNodes(edges []*Edge, state *TraversalState) []QueryNode {
       // Calculate distance for each node based on edge traversal
       // distance=1 for direct edges, 2 for 2-hop, etc.
   }
   ```

3. Helper: convert Edge to QueryEdge with full provenance:
   ```go
   func (g *Graph) edgesToQueryEdges(edges []*Edge) []QueryEdge {
       // Ensure all provenance fields included
       // Include aggregation_count from Plan 1
   }
   ```

**Verification:**
- JSON output is valid and parseable by jq
- All provenance fields included
- Metadata section accurate

---

### Task 5: Add CLI Commands for Queries

**Action:** Wire query interface to CLI in `cmd/graphmd/main.go`.

**Specific steps:**
1. Add impact subcommand:
   ```bash
   graphmd query impact --component payment-api [--depth N] [--min-tier TIER] [--output json]
   ```

2. Add crawl subcommand:
   ```bash
   graphmd query crawl --component payment-api [--max-depth N] [--output json]
   ```

3. Add flags:
   ```
   --depth N                  (impact only) traversal depth; default 2
   --max-depth N              (crawl only) max exploration depth; default 10
   --min-confidence FLOAT     numeric threshold [0.4, 1.0]
   --min-tier TIER            semantic tier: explicit, strong-inference, moderate, weak, semantic, threshold
   --traverse-mode MODE       direct|cascade|full; default cascade
   --output FORMAT            json|table; default json
   ```

4. Wire to Graph methods:
   ```go
   case "impact":
       opts := QueryOptions{
           MinConfidenceScore: minConf,
           MaxDepth: depth,
           TraverseMode: mode,
       }
       result, err := graph.Impact(root, opts)
       fmt.Println(result.ToJSON())
   ```

**Verification:**
- CLI parses flags correctly
- Help text visible (`graphmd query --help`)
- Commands produce JSON output

---

### Task 6: Unit Tests for Query Interface

**Action:** Create `query_test.go` with comprehensive query tests.

**Specific steps:**
1. Test functions:
   ```go
   func TestQuery_Impact_DirectOnly()
   func TestQuery_Impact_WithDepth()
   func TestQuery_Impact_ConfidenceFilter()
   func TestQuery_Impact_TraverseModeControl()
   func TestQuery_Crawl_NoFiltering()
   func TestQuery_Crawl_IncludesAllEdges()
   func TestQuery_JSONSerialization()
   func TestQuery_NodeDistance()
   func TestQuery_IncludesCyclicEdges()
   ```

2. Example test:
   ```go
   // Build simple graph: A→B (0.9), B→C (0.6), A→D (0.3)
   // Impact from A with --depth 1 should return only A→B
   // Impact from A with --depth 2 should return A→B, B→C, A→D
   // Impact from A with --min-tier strong should filter C (0.6 < strong)
   ```

3. JSON validation test:
   ```go
   func TestQuery_JSONOutput_Parseable() {
       result := g.Impact("A", opts)
       data := result.ToJSON()
       err := json.Unmarshal(data, &QueryResult{})
       assert.Nil(t, err)
   }
   ```

**Verification:**
- All tests pass
- Coverage >= 85% for query.go
- JSON output valid

---

### Task 7: Integration Test with Full Phase 2 Stack

**Action:** Create end-to-end test using all Phase 2 components.

**Specific steps:**
1. Create test in `graph_test.go`:
   ```go
   func TestIntegration_QueryInterface_FullStack()
   ```

2. Test flow:
   - Load test-data corpus
   - Run discovery with pageindex (Plan 1)
   - Apply confidence tiers (Plan 2)
   - Save with provenance (Plan 3)
   - Load graph
   - Run impact query with depth/confidence filters
   - Verify JSON structure
   - Run crawl query
   - Verify all edges included

3. Validate:
   - Impact and crawl return different result counts (impact filtered, crawl not)
   - Cyclic edges marked in both
   - Confidence tiers visible in output
   - Provenance fields populated

**Verification:**
- Integration test passes
- Full stack works end-to-end
- JSON output complete and correct

---

## Verification Criteria

**What "done" looks like:**

1. ✓ **Query enums & structs defined:** TraverseMode, QueryResult, QueryOptions all structured. JSON-serializable.

2. ✓ **Impact query implemented:** Direct edges by default. Depth control works. Confidence filtering works.

3. ✓ **Crawl query implemented:** No filtering. All edges included. Exploratory use case supported.

4. ✓ **JSON output consistent:** Both queries return same schema. Fields include provenance. Valid JSON.

5. ✓ **CLI commands wired:** `graphmd query impact` and `graphmd query crawl` work. Flags accepted. Help text visible.

6. ✓ **Unit tests pass:** query_test.go covers impact, crawl, filtering, modes, JSON output. Coverage >= 85%.

7. ✓ **Integration test validates:** Full stack (Plans 1–4) integrated. Impact returns fewer results than crawl. Provenance fields present.

8. ✓ **Performance acceptable:** Queries complete in < 2 seconds on test-data corpus.

9. ✓ **No regressions:** All Phase 2 plans (1–4) tests still pass.

---

## Must-Haves (Phase Goal Backward)

From phase goal: "prevents false edges from misleading agents during incident response"

This plan delivers:
- ✓ Direct-edge-only default (depth=1 for impact; agents start focused)
- ✓ Confidence filtering (high-confidence edges returned first; low-confidence visible in crawl)
- ✓ JSON subgraph topology (agents reconstruct full graph; queryable with jq)
- ✓ Provenance in results (agents assess trustworthiness)

---

## Dependency Notes

**Depends on:** Plans 1–4 (all foundational work must complete)

**Enables:** Phase 3 (export pipeline uses these queries to build output)

---

*Plan created: 2026-03-19*
*Autonomous execution: Yes*
