---
phase: 02-accuracy-foundation
plan: 05
type: execute
wave: 4
depends_on:
  - 02-01
  - 02-02
  - 02-03
  - 02-04
files_modified:
  - internal/knowledge/query.go
  - internal/knowledge/query_test.go
  - internal/knowledge/types.go
autonomous: true
requirements:
  - REL-04

must_haves:
  truths:
    - "Crawl command returns all relationships with full confidence/provenance, no filtering"
    - "Impact command returns direct edges only by default; transitive traversal with --depth flag"
    - "Query results include JSON subgraph: affected_nodes array and edges array with provenance"
    - "Confidence filtering supports both --min-confidence (numeric) and --min-tier (semantic) flags"
    - "Agents can parse JSON output with jq or JSON libraries; programmatically filter/analyze"
  artifacts:
    - path: "internal/knowledge/query.go"
      provides: "CrawlQuery, ImpactQuery, QueryResult structs; traversal and formatting logic"
      exports: ["CrawlQuery", "ImpactQuery", "QueryResult", "ExecuteImpact", "ExecuteCrawl"]
      min_lines: 250
    - path: "internal/knowledge/types.go"
      provides: "QueryResult, AffectedNode, QueryEdge structs for JSON serialization"
      exports: ["QueryResult", "AffectedNode", "QueryEdge"]
      min_lines: 50
    - path: "internal/knowledge/query_test.go"
      provides: "Query execution and JSON output tests"
      exports: ["TestImpactQuery_DirectOnly", "TestCrawlQuery_Full"]
  key_links:
    - from: "ImpactQuery command"
      to: "TraversalState depth limiting"
      via: "--depth parameter controls traversal scope; default=1 (direct only)"
      pattern: "depth.*1|direct.*only|maxDepth"
    - from: "Query filters"
      to: "Confidence tier system (Plan 2)"
      via: "--min-confidence 0.7 or --min-tier strong filters edges"
      pattern: "min-confidence|min-tier|FilterByTier"
    - from: "QueryResult JSON"
      to: "Agent parseable format"
      via: "json.Marshal(result) produces valid JSON with subgraph structure"
      pattern: "json:.*|JSON|json\\.Marshal"

---

<objective>
Implement two query patterns that agents need for incident response: Crawl (exploratory, all relationships) and Impact (focused incident response, direct edges only by default). Support confidence filtering, depth-limited traversal, and JSON output with full provenance and subgraph topology.

Purpose: REL-04 requirement; completes the accuracy foundation by providing agent-usable query interface. Crawl for exploration, Impact for incident response decisions.

Output: QueryResult struct with JSON schema, CrawlQuery and ImpactQuery implementations, confidence-filtered traversal, depth-controlled results.
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
  <name>Task 1: Define QueryResult and related structures for JSON output</name>
  <files>internal/knowledge/types.go</files>
  <action>
Add to types.go:

1. Define AffectedNode struct for JSON results:
   ```go
   type AffectedNode struct {
       Name             string  `json:"name"`
       Type             string  `json:"type"`
       Confidence       float64 `json:"confidence"`
       RelationshipType string  `json:"relationship_type"` // "direct-dependency" or "cyclic-dependency"
       Distance         int     `json:"distance"`          // distance from root node
   }
   ```

2. Define QueryEdge struct for individual edges in result:
   ```go
   type QueryEdge struct {
       From              string  `json:"from"`
       To                string  `json:"to"`
       Confidence        float64 `json:"confidence"`
       Type              string  `json:"type"`
       RelationshipType  string  `json:"relationship_type"`
       Evidence          string  `json:"evidence"`
       SourceFile        string  `json:"source_file"`
       ExtractionMethod  string  `json:"extraction_method"`
       EvidencePointer   string  `json:"evidence_pointer"`
       SignalsCount      int     `json:"signals_count"`   // number of algorithms detecting this edge
   }
   ```

3. Define QueryResult struct (top-level JSON structure):
   ```go
   type QueryResult struct {
       Query          string         `json:"query"`           // "impact" or "crawl"
       Root           string         `json:"root"`            // root component name
       Depth          int            `json:"depth"`           // traversal depth used
       TraverseMode   string         `json:"traverse_mode"`   // "direct", "cascade", "full"
       MinConfidence  float64        `json:"min_confidence"`  // filtering threshold applied
       MinTier        string         `json:"min_tier"`        // tier name or null
       AffectedNodes  []AffectedNode `json:"affected_nodes"`  // all reached nodes
       Edges          []QueryEdge    `json:"edges"`           // all edges in result
       Metadata       map[string]interface{} `json:"metadata"` // query metadata (execution time, node count, etc.)
   }
   ```

4. Add String() method to QueryResult for pretty-printing (for debugging)

5. Add Validate() method to ensure:
   - Root component exists
   - AffectedNodes and Edges non-empty for valid query
   - All edges have corresponding from/to nodes in AffectedNodes

Reference from CONTEXT.md decision examples (lines 89-116) for expected JSON structure.
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestQueryResult -v
Expected:
  - TestQueryResult_JSONMarshal: QueryResult serializes to valid JSON
  - TestQueryResult_Validation: Validate() catches missing nodes or invalid structure

Also run: go build ./cmd/graphmd — no syntax errors
  </verify>
  <done>
- AffectedNode struct defined with all required JSON fields
- QueryEdge struct includes provenance fields (source_file, extraction_method, evidence_pointer)
- QueryResult top-level struct with query metadata
- Validate() method ensures structural integrity
- JSON marshaling produces valid JSON per CONTEXT.md example
  </done>
</task>

<task type="auto">
  <name>Task 2: Implement ImpactQuery and CrawlQuery execution logic</name>
  <files>internal/knowledge/query.go</files>
  <action>
Create new file internal/knowledge/query.go with:

1. Define query request types:
   ```go
   type ImpactQuery struct {
       Root              string  // root component name
       Depth             int     // traversal depth (default 2: direct + 1 hop)
       MinConfidence     *float64 // numeric threshold (optional)
       MinTier           *string  // tier name (optional)
       TraverseMode      string  // "direct", "cascade" (default), "full"
   }

   type CrawlQuery struct {
       Root              string  // root component name
       MaxDepth          int     // max traversal depth (optional; unbounded if 0)
   }
   ```

2. Implement ExecuteImpact(g *Graph, q *ImpactQuery) (*QueryResult, error):
   - Validate root node exists in graph
   - Call g.TraverseDFS(root, maxDepth=q.Depth) from Plan 4
   - Filter edges by confidence:
     - If MinTier set: use ScoreToTier() from Plan 2 to filter
     - If MinConfidence set: filter confidence >= threshold
   - Apply traverse mode:
     - "direct": include only distance=1 edges
     - "cascade": include edges up to depth (default)
     - "full": include all edges (transitive closure)
   - Build QueryResult:
     - AffectedNodes: all nodes reached during traversal
     - Edges: all edges in traversal result (filtered)
     - Metadata: execution time, node count, edge count
   - Return QueryResult

3. Implement ExecuteCrawl(g *Graph, q *CrawlQuery) (*QueryResult, error):
   - Similar to ImpactQuery but:
     - No confidence filtering (return all relationships)
     - MaxDepth unbounded (if 0) or limited
     - Exploratory intent: agents see everything
   - Build QueryResult with same structure

4. Add helper function BuildQueryResult(root string, nodes []string, edges []*Edge, queryType string) *QueryResult

5. Add comment explaining difference: "Impact for incident response (filtered by confidence, focused); Crawl for exploration (all relationships visible)"

Reference RESEARCH.md section 2.5 for query semantics; CONTEXT.md section on JSON output for structure.
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestQuery -v
Expected:
  - TestExecuteImpact_DirectOnly: depth=1 returns only direct edges
  - TestExecuteImpact_WithHop: depth=2 includes one hop
  - TestExecuteImpact_ConfidenceFilter: MinConfidence filters edges
  - TestExecuteImpact_TierFilter: MinTier filters edges by semantic tier
  - TestExecuteCrawl_NoFilter: Returns all relationships regardless of confidence

Also run: go build ./cmd/graphmd — no syntax errors
  </verify>
  <done>
- ImpactQuery and CrawlQuery types defined with all parameters
- ExecuteImpact() implements depth limiting, confidence filtering, traverse mode
- ExecuteCrawl() implements full traversal with no filtering
- BuildQueryResult() assembles JSON response structure
- Confidence filtering supports both numeric and tier-based thresholds
  </done>
</task>

<task type="auto">
  <name>Task 3: Unit and integration tests for query execution and JSON output</name>
  <files>internal/knowledge/query_test.go</files>
  <action>
Create comprehensive test coverage:

1. Impact query tests:
   - TestImpactQuery_DirectOnly:
     - Setup: Graph A→B→C→D chain
     - Query: root=A, depth=1
     - Assert: Result.Edges contains only A→B; B→C not included
     - Assert: AffectedNodes contains A, B only

   - TestImpactQuery_DepthTwo:
     - Setup: Same chain
     - Query: root=A, depth=2
     - Assert: Result.Edges contains A→B, B→C
     - Assert: AffectedNodes contains A, B, C

   - TestImpactQuery_ConfidenceFilter:
     - Setup: Multiple edges with different confidences (0.8, 0.6, 0.5)
     - Query: root=A, MinConfidence=0.7
     - Assert: Only edge with 0.8 confidence returned; others filtered

   - TestImpactQuery_TierFilter:
     - Setup: Edges with different tiers
     - Query: root=A, MinTier="strong-inference" (0.8)
     - Assert: Only edges with tier >= strong-inference returned

   - TestImpactQuery_TraverseMode:
     - Setup: Multiple paths to same node
     - Query: root=A, depth=3, TraverseMode="direct"
     - Assert: Only direct (distance=1) edges returned despite depth=3

2. Crawl query tests:
   - TestCrawlQuery_NoFilter:
     - Setup: Graph with edges of varying confidences
     - Query: root=A, no confidence filter
     - Assert: All edges returned regardless of confidence

   - TestCrawlQuery_MaxDepth:
     - Setup: Graph with 5 levels
     - Query: root=A, MaxDepth=2
     - Assert: Only edges at distance <= 2 returned

3. JSON output tests:
   - TestQueryResult_JSONMarshal:
     - Execute query, marshal QueryResult to JSON
     - Parse JSON with json.Unmarshal
     - Assert: All fields present and correct types
     - Assert: Compatible with jq parsing (test: `jq .affected_nodes[].name`)

   - TestQueryResult_JSONSchema:
     - Verify QueryResult JSON matches schema from CONTEXT.md (lines 89-116)
     - Assert: query, root, depth, affected_nodes, edges fields present
     - Assert: Each edge has confidence, source_file, extraction_method, evidence

4. Integration test with test-data:
   - TestQueryOnCorpus:
     - Load test-data, run ImpactQuery and CrawlQuery on various roots
     - Assert: All queries complete successfully
     - Assert: Results are valid JSON
     - Assert: No crashes or infinite loops

Run tests: go test ./internal/knowledge -run TestQuery -v
Expected: All tests pass
Verify JSON: Test results parseable with jq (e.g., `echo "$json" | jq .affected_nodes`)
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestQuery -v
Expected: All tests pass
Run: go test ./internal/knowledge -run TestQuery -v | jq .
Expected: Test JSON output is valid and parseable

Also validate with: jq -e '.affected_nodes | length > 0' <<< "$test_json"
Expected: JSON validation succeeds
  </verify>
  <done>
- ImpactQuery execution tests pass (depth limiting, confidence filtering, traverse modes)
- CrawlQuery execution tests pass (no filtering, max depth respected)
- JSON output marshaling works correctly
- QueryResult JSON is valid and parseable with standard tools
- Integration tests verify queries work on real corpus without crashes
- Test coverage >85% on query execution logic
  </done>
</task>

</tasks>

<verification>
**Comprehensive checks before marking complete:**

1. **Direct-Edge-Only Default:** Impact query with depth=1 returns ONLY direct edges (distance=1); no transitive edges leaked through.

2. **Confidence Filtering:** Same graph queried with MinConfidence=0.7 returns fewer edges than MinConfidence=0.4. Correctness verified by manual inspection of test data.

3. **Tier-Based Filtering:** Query with MinTier="strong-inference" produces same or fewer edges than MinTier="weak". Tier ordering correctly implemented.

4. **JSON Schema Validation:** QueryResult JSON structure matches CONTEXT.md example (lines 89-116):
   - Top-level fields: query, root, depth, affected_nodes, edges, metadata
   - Each node: name, type, confidence, relationship_type, distance
   - Each edge: from, to, confidence, type, relationship_type, evidence, source_file, extraction_method, evidence_pointer, signals_count

5. **jq Compatibility:** Sample JSON output parseable with standard jq filters (`.affected_nodes[].name`, `.edges[] | select(.confidence > 0.7)`)

6. **Traverse Mode Semantics:** TraverseMode="direct" returns only distance=1 regardless of depth parameter. Modes affect result scope correctly.

7. **Graph Coverage:** Test data corpus queries complete without crashes. Results are reasonable (not empty, not overly large).
</verification>

<success_criteria>
- REL-04 satisfied: Direct-edge-only semantics in impact queries
- QueryResult JSON structure matches CONTEXT.md specification
- ImpactQuery supports depth limiting, confidence filtering, traverse modes
- CrawlQuery supports full traversal without filtering
- Both query types produce valid JSON with full subgraph topology
- Confidence filtering supports both numeric (--min-confidence) and semantic (--min-tier) thresholds
- All query results include provenance fields (source_file, extraction_method, evidence_pointer)
- JSON output parseable by agents with standard tools (jq, JSON libraries)
- Unit tests pass with 85%+ coverage on query execution logic
- No breaking changes to Phase 1-4 structures
</success_criteria>

<output>
After completion, create `.planning/phases/02-accuracy-foundation/02-05-SUMMARY.md` documenting:
- QueryResult struct and JSON schema
- ImpactQuery execution with depth limiting and confidence filtering
- CrawlQuery execution for full graph exploration
- Traverse mode semantics (direct, cascade, full)
- JSON output examples and jq-compatible filtering
- Test coverage and performance characteristics
</output>
