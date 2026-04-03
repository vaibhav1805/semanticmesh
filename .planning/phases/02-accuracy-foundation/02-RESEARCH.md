# Phase 2: Accuracy Foundation - Research Report

**Compiled:** 2026-03-19
**Status:** Ready for planning
**Scope:** Technical approach, implementation order, and validation strategy

---

## 1. Current State

### 1.1 Existing Infrastructure

#### Pageindex Integration
- **File:** `pageindex.go` (95 lines)
- **Status:** Subprocess runner exists, not yet integrated into relationship tracking
- **Capability:** Calls `pageindex index --file <filePath>` CLI, returns hierarchical `FileTree` JSON
- **Gap:** No connection to Edge/Graph or algo_aggregator; no deduplication logic
- **What's needed:** Wrapper to map pageindex output (file:line references) into relationship locations for deduplication

#### Algorithm Aggregator
- **File:** `algo_aggregator.go` (139 lines)
- **Current behavior:** Groups DiscoverySignals by (source, target) pair; takes **highest-confidence signal** as final edge
- **Limitation:** Does NOT implement weighted averaging; does NOT use pageindex location data
- **Signals structure:** Includes algorithm name (e.g., "semantic", "cooccurrence") but no location/provenance fields
- **Gap:** Need to extend DiscoverySignal with location metadata and implement aggregation by location

#### Edge and Graph Structures
- **Edge struct:** Currently holds (Source, Target, Type, Confidence, Evidence)
- **Gap:** No provenance fields (source_file, extraction_method, evidence_pointer)
- **Graph struct:** In-memory representation with Nodes, Edges, BySource, ByTarget maps
- **Schema:** graph_edges table stores (id, source_id, target_id, type, confidence, evidence)
- **Gap:** No columns for source_file, extraction_method, last_modified

#### Discovery Pipeline
- **Orchestration file:** `discovery_orchestration.go` (200+ lines)
- **Current:** Implements DiscoveryFilterConfig with confidence thresholds for signal filtering
- **Signal types:** Recognized (SignalLLM, plus co-occurrence, structural, NER, semantic)
- **Filter logic:** Supports multi-signal quality gates (1x, 2x, 3x signal tiers)
- **Gap:** Filter operates on raw Confidence values; no tier-based filtering

#### SQLite Schema
- **Current version:** 3
- **graph_nodes table:** id, type, file, title, content, metadata, component_type (TEXT NOT NULL DEFAULT 'unknown')
- **graph_edges table:** id, source_id, target_id, type, confidence (REAL CHECK [0.0, 1.0]), evidence
- **component_mentions table:** Exists (from Phase 1) but only tracks component detection (component_id, file_path, detected_by, confidence)
- **Gap:** No relationship_provenance or equivalent table; edges lack source_file, extraction_method, last_modified

#### Migration Infrastructure
- **Current version constant:** SchemaVersion = 3
- **Migration pattern:** Idempotent ALTER statements; version tracked in metadata table
- **Existing migrations:** v1→v2 (chunk metadata), v2→v3 (component_type column + component_mentions table)
- **How it works:** Migrate() checks stored version, runs applicable migrations, updates version
- **Ready to use:** Framework is proven and works; next migration is v3→v4

#### Test Data
- **Location:** `test-data/` (multiple service directories: payment-service, recommendation-service, deployment-service, notification-service, sms-service)
- **Size:** ~62 markdown documents with service architectures, API specs, README files
- **Existing coverage:** Services with cross-references, component types, dependency mentions
- **Usable for Phase 2:** Provides real graph with cycles and complex relationships

#### SaveGraph Function
- **Location:** db.go lines 804-875
- **Current:** Saves nodes and edges; filters dangling references but does NOT save provenance
- **Batch processing:** Operates in batches of 1000 for performance
- **Transaction safety:** Wrapped in transaction() helper
- **What SaveGraph would need to change:** Accept new provenance fields (source_file, extraction_method, etc.) from Edge struct

### 1.2 What's NOT Yet Done

| Item | Status | Impact |
|------|--------|--------|
| Confidence tier system (7 levels) | Not defined | REL-01 blocker |
| Relationship provenance fields | Not in Edge struct | REL-02 blocker |
| Relationship provenance in schema | Not in graph_edges | REL-02 blocker |
| Pageindex location deduplication | Not implemented | REL-05 blocker |
| Cycle detection in traversal | Partial (BFS traversal exists) | REL-03 blocker |
| Direct-edge vs transitive semantics | Not enforced | REL-04 blocker |
| Weighted confidence aggregation | Not implemented | REL-01 depends on this |
| Graph traversal with visited set | Not implemented | REL-03 blocker |

---

## 2. Technical Approach

### 2.1 Confidence Tier System (REL-01)

**Where to define:** `confidence.go` (new file) or extend `types.go`

**Structure:**
```go
type ConfidenceTier string
type ConfidenceScore float64  // [0.4, 1.0]

const (
    TierExplicit        ConfidenceTier = "explicit"         // 1.0
    TierStrongInference ConfidenceTier = "strong-inference" // 0.8
    TierModerate        ConfidenceTier = "moderate"         // 0.6
    TierWeak            ConfidenceTier = "weak"             // 0.5
    TierSemantic        ConfidenceTier = "semantic"         // 0.45
    TierThreshold       ConfidenceTier = "threshold"        // 0.4
)

// Map confidence score [0.4, 1.0] → tier
func ScoreToTier(score ConfidenceScore) ConfidenceTier { ... }

// Map tier → canonical score for aggregation
func TierToScore(tier ConfidenceTier) ConfidenceScore { ... }
```

**Algorithm weights for aggregation (when same relationship detected by multiple algorithms):**
- Co-occurrence: 0.3x
- NER: 0.5x
- Structural: 0.6x
- Semantic: 0.7x
- LLM: 1.0x

**Aggregation formula:**
```
weighted_sum = Σ(confidence_i * weight_i)
weighted_count = Σ(weight_i)
aggregated_confidence = weighted_sum / weighted_count
```

### 2.2 Provenance Schema Extension (REL-02)

**Edge struct addition:**
```go
type Edge struct {
    // existing fields
    ID         string
    Source     string
    Target     string
    Type       EdgeType
    Confidence float64
    Evidence   string

    // NEW provenance fields
    SourceFile       string  // path to where relationship was detected
    ExtractionMethod string  // "explicit-link", "co-occurrence", "structural", "NER", "semantic", "LLM"
    DetectionEvidence string // ~200 char contextual snippet (example: "calls primary-db for transaction storage")
    EvidencePointer  string  // file:line or byte offset (optional; "service.yaml:42")
    LastModified     time.Time // file mtime or detection timestamp
}
```

**SQLite schema migration (v3→v4):**
```sql
ALTER TABLE graph_edges ADD COLUMN source_file TEXT;
ALTER TABLE graph_edges ADD COLUMN extraction_method TEXT;
ALTER TABLE graph_edges ADD COLUMN detection_evidence TEXT;
ALTER TABLE graph_edges ADD COLUMN evidence_pointer TEXT;
ALTER TABLE graph_edges ADD COLUMN last_modified INTEGER;

-- Add NOT NULL constraint with default (or make nullable, depending on data availability)
-- For safe migration, start nullable, populate from existing data, then add constraint
```

**SaveGraph changes:**
```go
// In SaveGraph loop, extend INSERT statement:
INSERT OR REPLACE INTO graph_edges
(id, source_id, target_id, type, confidence, evidence,
 source_file, extraction_method, detection_evidence, evidence_pointer, last_modified)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
```

### 2.3 Pageindex Integration (REL-05 - Hard Dependency)

**Approach:** Create a relationship deduplication layer that uses pageindex location data

**New struct:**
```go
// RelationshipLocation identifies where in source file(s) a relationship was detected
type RelationshipLocation struct {
    File      string  // relative path
    Line      int     // line number (from pageindex or detection algorithm)
    ByteOffset int    // optional
    Evidence  string  // snippet
}

// RelationshipLocationKey deduplicates on file:line
func RelationshipLocationKey(loc RelationshipLocation) string {
    return fmt.Sprintf("%s:%d", loc.File, loc.Line)
}
```

**Integration into algo_aggregator:**
```go
// Extend DiscoverySignal with location
type DiscoverySignal struct {
    Source      string
    Target      string
    Type        EdgeType
    Confidence  float64
    Evidence    string
    Algorithm   string
    // NEW:
    Location    RelationshipLocation  // file:line from pageindex
}

// Modified aggregation: group by (source, target, location),
// then aggregate by weighted average within each location group
```

**Workflow:**
1. Each discovery algorithm (co-occurrence, NER, semantic, LLM) calls pageindex to map detected relationship to file:line
2. DiscoverySignal includes RelationshipLocation
3. AggregateSignals now groups by (source, target, location) instead of just (source, target)
4. For same (source, target, location), compute weighted average confidence
5. Result: Same relationship detected by 3 algorithms at same location = 1 edge with aggregated confidence

**Implementation location:** Extend algo_aggregator.go

### 2.4 Cycle Detection & Safety (REL-03)

**Approach:** Implement cycle detection in graph traversal using visited set

**New struct:**
```go
type TraversalState struct {
    Visited map[string]bool  // node ID → visited
    Path    []string         // current DFS path for cycle detection
    Cycles  [][]string       // detected cycles (path rings)
}

// Detect if adding edge (source → target) would create a cycle
func (ts *TraversalState) WouldCreateCycle(source, target string) bool {
    // Check if target is already in current path (ancestor)
    for _, node := range ts.Path {
        if node == target {
            return true  // target is ancestor; adding edge creates cycle
        }
    }
    return false
}
```

**Marking cyclic edges:**
```go
// Add relationship_type field to Edge (or new EdgeMarker struct)
type Edge struct {
    // existing fields...
    RelationshipType string  // "direct-dependency" (default), "cyclic-dependency"
}
```

**Traversal implementation (impact query):**
- Use DFS with visited set to traverse outgoing edges
- When traversal would close a cycle, mark edge as cyclic-dependency
- Continue traversal (don't skip); cyclic edges are included in results
- Track visited set per query (not global) to handle multiple independent queries

### 2.5 Query Interface: Crawl vs Impact (REL-04)

**Query types:**

1. **Crawl command (exploratory):**
   - Input: component name
   - Output: All relationships with full confidence/provenance, no filtering
   - Depth: Unbounded (or configurable --max-depth)
   - Format: JSON with all relationships, agents see everything and judge

2. **Impact command (focused incident response):**
   - Input: component name, --depth (default 2), --min-confidence/--min-tier, --traverse-mode
   - Output: Filtered relationships (direct + 1 hop by default)
   - Depth control:
     - `--depth 1`: Direct edges only (distance=1)
     - `--depth 2`: Direct + 1 hop (distance ≤ 2, default)
     - `--depth N`: Transitive closure up to N hops
   - Traverse mode:
     - `--traverse-mode direct`: distance=1 only
     - `--traverse-mode cascade`: distance ≤ depth (default)
     - `--traverse-mode full`: full transitive closure
   - Min confidence: `--min-confidence 0.7` or `--min-tier strong` filters edges

**JSON output format (both commands):**
```json
{
  "query": "impact",
  "root": "payment-api",
  "depth": 2,
  "affected_nodes": [
    {
      "name": "primary-db",
      "type": "database",
      "confidence": 1.0,
      "relationship_type": "cyclic-dependency",
      "distance": 1
    }
  ],
  "edges": [
    {
      "from": "payment-api",
      "to": "primary-db",
      "confidence": 1.0,
      "type": "direct-dependency",
      "relationship_type": "direct-dependency",
      "evidence": "calls primary-db for transaction storage",
      "source_file": "service.yaml",
      "extraction_method": "explicit-link",
      "evidence_pointer": {"file": "service.yaml", "line": 42},
      "signals_count": 1
    }
  ]
}
```

**Where to implement:** New `query.go` module with TraversalMode enum and traversal logic

---

## 3. Implementation Plan

### 3.1 Dependency Order (REL-05 must be first)

```
1. Confidence tier system (types.go / confidence.go)
   ├─ Define 7 tiers + scoring functions
   └─ Update discovery algorithms to assign tier-aware confidence

2. Edge struct provenance extension
   ├─ Add source_file, extraction_method, detection_evidence, evidence_pointer, last_modified
   └─ Update NewEdge() validation

3. Pageindex integration (REL-05 hard dependency)
   ├─ Extend DiscoverySignal with location metadata
   ├─ Modify AggregateSignals to group by (source, target, location)
   └─ Implement weighted average aggregation

4. SQLite schema migration (v3→v4)
   ├─ Add provenance columns to graph_edges
   ├─ Update SaveGraph() and LoadGraph()
   └─ Test round-trip persistence

5. Cycle detection & traversal (REL-03)
   ├─ Implement TraversalState with visited set
   ├─ Update Graph.TraverseBFS() to track cycles
   └─ Mark cyclic edges with relationship_type field

6. Query interface (REL-04)
   ├─ Implement crawl vs impact traversal logic
   ├─ Add depth/traverse-mode/min-confidence filtering
   └─ Create JSON output formatter

7. Integration & testing
   ├─ Wire into export pipeline (Phase 3 prep)
   ├─ Add integration tests with test-data corpus
   └─ Validate cycle handling, confidence aggregation, provenance round-trip
```

### 3.2 Risk Areas

| Risk | Mitigation |
|------|-----------|
| Pageindex availability (external tool) | Test graceful fallback; make optional for testing |
| Schema migration atomicity | Use transaction() wrapper; test on existing test-data database |
| Weighted aggregation correctness | Unit test with known signal combinations |
| Cycle detection performance | Benchmark on large cyclic graphs (test-data has ~62 docs) |
| Backward compatibility with Phase 1 exports | Test import of old graphs; handle missing provenance fields |

### 3.3 File Changes Summary

| File | Change | Lines |
|------|--------|-------|
| `confidence.go` (NEW) | Tier system, scoring functions | ~100 |
| `edge.go` | Add provenance fields | +30 |
| `algo_aggregator.go` | Location-based grouping, weighted avg | +80 |
| `db.go` | Add v3→v4 migration, update SaveGraph/LoadGraph | +100 |
| `graph.go` | Update traversal for cycles | +50 |
| `query.go` (NEW) | Crawl/impact traversal logic | ~200 |
| `types.go` | Add relationship_type field to Edge | +20 |
| Test files | Unit + integration tests | ~300 |

**Total new/modified code:** ~880 lines

---

## 4. Testing Strategy

### 4.1 Unit Tests

**Confidence tier system:**
```go
TestScoreToTier_BoundaryValues()  // 0.4→threshold, 1.0→explicit, etc.
TestTierToScore_Reversible()      // tier→score→tier is identity
TestWeightedAggregation()         // [0.3sig@0.6, 0.6sig@0.8] → aggregated score
```

**Pageindex integration:**
```go
TestRelationshipLocationKey()     // Deterministic deduplication key
TestAggregateSignalsWithLocation() // Same location merged, different locations kept separate
```

**Cycle detection:**
```go
TestTraversalStateWouldCreateCycle()
TestTraversalStatePathTracking()
```

**Provenance persistence:**
```go
TestSaveLoadGraph_PreservesProvenance()  // Round-trip: edge → DB → edge
TestEdgeProvenance_Validation()  // NewEdge rejects invalid extraction_method
```

**Query interface:**
```go
TestImpactQuery_DirectEdgesOnly()    // Default depth=1 returns only immediate deps
TestImpactQuery_DepthControl()       // depth=2 includes one hop
TestImpactQuery_MinConfidenceFilter()
TestCrawlQuery_ReturnsAll()         // No filtering
```

### 4.2 Integration Tests

**Full pipeline with test-data:**
1. Load test-data corpus (62 documents, services with cross-refs)
2. Run discovery algorithms (co-occurrence, NER, semantic, LLM if available)
3. Aggregate signals with pageindex location grouping
4. Save to SQLite with provenance
5. Load graph and verify:
   - All relationships preserved
   - Provenance fields non-null
   - Confidence scores in [0.4, 1.0]
   - Multiple signals from same location → single edge with aggregated confidence

**Cycle test:**
1. Construct test graph: A→B, B→C, C→A
2. Run impact query on A
3. Verify:
   - Completes in < 1 second
   - Returns A→B (confidence 1.0)
   - Marks B→C→A as cyclic-dependency
   - All edges included (not infinite loop)

**Confidence aggregation test:**
1. Create three signals for same (source, target, location):
   - Algorithm 1 (weight 0.3): confidence 0.6
   - Algorithm 2 (weight 0.6): confidence 0.8
   - Algorithm 3 (weight 0.7): confidence 0.5
2. Compute: (0.6×0.3 + 0.8×0.6 + 0.5×0.7) / (0.3+0.6+0.7) = 1.31 / 1.6 = 0.8188
3. Verify aggregated edge has confidence ≈ 0.82

**Container scenario:**
1. Export graph to SQLite (Phase 3)
2. ZIP with database
3. In fresh container: import ZIP
4. Query relationships
5. Verify evidence snippets are complete and usable without file access

### 4.3 Validation Approach

**Success criteria (from ROADMAP):**
1. ✓ Every relationship has confidence ∈ [0.4, 1.0] mapped to 7 tiers
2. ✓ Every relationship includes non-null source_file, extraction_method, last_modified
3. ✓ Cyclic graph A→B→C→A completes impact analysis in < 1 second
4. ✓ Impact query returns direct edges by default (depth=1); transitive is opt-in
5. ✓ Same relationship from 3 algorithms at same location = 1 edge with aggregated confidence

---

## 5. Open Questions

### 5.1 Clarifications Needed Before Implementation

| Question | Impact | Default Assumption |
|----------|--------|-------------------|
| Should `last_modified` be file mtime or detection timestamp? | Provenance accuracy | Use file mtime from scan; fallback to detection timestamp |
| How to handle relationships detected in code vs markdown (different file types)? | Extraction method coverage | Track source_file as-is (both types valid); extraction_method indicates discovery approach |
| When multiple detection methods find same relationship at different lines, keep one or multiple edges? | Deduplication scope | Keep multiple edges if different lines; pageindex location is dedup key |
| Should evidence_pointer be mandatory or optional? | Schema design | Optional; some algorithms don't produce precise line numbers |
| For LLM discovery without explicit file location, what goes in source_file and evidence_pointer? | Edge case handling | source_file = document analyzed; evidence_pointer = null; detection_evidence = LLM summary |
| Should confidence tier names be used in CLI output or only numeric scores? | UX clarity | Both: `--min-tier strong` (human-readable input), output shows `tier: "strong-inference"` + numeric |
| How to handle relationships in container-mounted volumes (bind mounts, no direct /host path)? | Portability | Store evidence_snippet instead of relying on file paths; source_file is audit metadata |
| Should pagination be needed for large impact queries (e.g., 10k+ edges returned)? | Scale handling | Not in Phase 2; assume < 5k edges per query |

### 5.2 Test Data Gaps

- **Cycle coverage:** test-data may not have A→B→C→A cycles naturally. Plan to add synthetic test case or construct in test.
- **Confidence distribution:** Need to verify test data produces relationships across all 7 tiers (not just high-confidence explicit links).
- **Provenance variety:** Test data should exercise multiple extraction methods; may need to enhance LLM discovery integration for Phase 2.

---

## 6. Success Criteria Checklist

When planning concludes, verify:

- [ ] Confidence tier system (7 levels) defined and integrated into DiscoverySignal
- [ ] Edge struct extended with provenance fields; NewEdge() validation updated
- [ ] Pageindex location deduplication approach documented (for future implementation)
- [ ] SQLite schema migration plan (v3→v4) written; no data loss
- [ ] Cycle detection algorithm designed (visited set + path tracking)
- [ ] Query interface (crawl vs impact) design documented with JSON schema
- [ ] Implementation order confirmed with dependencies noted
- [ ] Risk areas identified and mitigations planned
- [ ] Test strategy covers unit, integration, and container scenarios
- [ ] Test data gaps documented; synthetic test cases planned

---

*Research compiled: 2026-03-19*
*Confidence: High (existing code reviewed; no blockers identified)*
