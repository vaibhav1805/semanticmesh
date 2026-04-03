# Phase 2, Plan 1: Pageindex Integration (Hard Dependency - Wave 1)

**Frontmatter:**
```
phase: 2
plan: 01-pageindex
wave: 1
depends_on: []
blocks: [02-02, 02-03, 02-04, 02-05]
autonomous: true
files_modified:
  - internal/knowledge/algo_aggregator.go
  - internal/knowledge/pageindex.go
  - internal/knowledge/discovery_orchestration.go
  - internal/knowledge/graph.go
  - internal/knowledge/*_test.go (multiple test files)
```

---

## Plan Summary

**Requirement:** REL-05 (Pageindex as hard dependency for relationship location tracking and deduplication)

**Goal:** Integrate pageindex subprocess runner into relationship deduplication pipeline so that duplicate relationships detected by multiple algorithms at the same file:line location are merged with aggregated confidence.

**Why Wave 1:** This plan must complete first. All other Phase 2 plans (confidence tiers, provenance, cycle detection, query interface) depend on the deduplication foundation provided here.

**Scope:**
- Extend DiscoverySignal struct with location metadata
- Modify AggregateSignals to group by (source, target, location)
- Implement weighted average confidence aggregation
- Update discovery pipeline to populate location data via pageindex

**Out of scope:**
- Confidence tier definitions (Plan 2)
- Provenance schema changes (Plan 3)
- Cycle detection (Plan 4)

---

## Tasks

### Task 1: Extend DiscoverySignal with Location Metadata

**Action:** Modify `DiscoverySignal` struct in `internal/knowledge/discovery_orchestration.go` to include source file location.

**Specific steps:**
1. Add new struct `RelationshipLocation` to `discovery_orchestration.go`:
   ```go
   type RelationshipLocation struct {
       File       string // relative path (e.g., "services/payment.md")
       Line       int    // line number where relationship detected
       ByteOffset int    // optional byte offset
       Evidence   string // snippet context (~200 chars)
   }
   ```

2. Add `Location` field to `DiscoverySignal`:
   ```go
   type DiscoverySignal struct {
       Source      string
       Target      string
       Type        EdgeType
       Confidence  float64
       Evidence    string
       Algorithm   string
       // NEW FIELD:
       Location    *RelationshipLocation // nil if location unavailable
   }
   ```

3. Update `NewDiscoverySignal()` constructor to accept location parameter (optional, nil-safe)

4. Document field semantics in struct comments (file paths are relative to scan root)

**Verification:**
- Code compiles
- Struct fields match documentation
- Nil location doesn't break signal creation

---

### Task 2: Implement Deduplication Key & Aggregation Function

**Action:** Add aggregation logic to `algo_aggregator.go` that groups signals by (source, target, location).

**Specific steps:**
1. Add helper function `SignalDeduplicationKey()`:
   ```go
   func SignalDeduplicationKey(sig DiscoverySignal) string {
       locKey := "unknown:0"
       if sig.Location != nil {
           locKey = fmt.Sprintf("%s:%d", sig.Location.File, sig.Location.Line)
       }
       return fmt.Sprintf("%s|%s|%s", sig.Source, sig.Target, locKey)
   }
   ```

2. Modify `AggregateSignals()` to:
   - Group signals by deduplication key instead of just (source, target)
   - Within each group, compute weighted average confidence using algorithm weights
   - Algorithm weights: co-occurrence=0.3, NER=0.5, structural=0.6, semantic=0.7, LLM=1.0
   - Formula: `aggregated = Σ(confidence_i * weight_i) / Σ(weight_i)`
   - Return single Edge per key with aggregated confidence + location metadata

3. Add `aggregation_count` or equivalent metadata to Edge struct (for debugging; used in queries to show "signals_count")

**Verification:**
- Unit test: Three signals for same (source, target, location) with different algorithms aggregate correctly
- Unit test: Same (source, target) at different locations stay separate
- Unit test: Nil location signals don't crash aggregation

---

### Task 3: Wire Pageindex Calls into Discovery Pipeline

**Action:** Update discovery algorithms to call pageindex and populate RelationshipLocation.

**Specific steps:**
1. In `discovery_orchestration.go`, before returning DiscoverySignals from each algorithm:
   - For each detected relationship, call `pageindex.IndexFile(filename)` to get location data
   - Map relationship mention to file:line using pageindex output
   - Populate RelationshipLocation with file, line, evidence snippet
   - Attach to DiscoverySignal before returning

2. Handle pageindex failures gracefully:
   - If pageindex subprocess unavailable, set Location to nil
   - Log warning but don't fail pipeline (graceful degradation)
   - Continue with unlocated signals (no deduplication benefit, but functional)

3. Update `RunDiscoveryPipeline()` to pass discovered relationships through location enrichment before aggregation

**Verification:**
- Relationships have Location field populated (non-nil for detected relationships)
- Pageindex subprocess calls succeed (or degrade gracefully)
- Pipeline still produces edges even if pageindex unavailable

---

### Task 4: Update Aggregation Output & Graph Persistence

**Action:** Ensure aggregated signals flow through to Edge creation and SaveGraph persistence.

**Specific steps:**
1. In `AggregateSignals()`, ensure returned Edges include:
   - `source_file` = aggregated signal's Location.File (or blank if nil)
   - `evidence` = aggregated signal's Evidence (keep best/longest)
   - `confidence` = aggregated score [0.4, 1.0]
   - `aggregation_count` = number of signals merged (1 if no merging)

2. In `SaveGraph()` in `db.go`:
   - Verify Edge struct fields are written to graph_edges table
   - No schema changes yet (happens in Plan 3)
   - Preserve evidence field integrity

3. Document aggregation in code comments (why duplicate detection matters for confidence)

**Verification:**
- Edges created from aggregated signals have correct confidence values
- Multiple algorithms for same relationship at same location produce single edge
- SaveGraph doesn't error on extended Edge struct

---

### Task 5: Add Unit Tests for Deduplication & Aggregation

**Action:** Add tests to `algo_aggregator_test.go` (create if doesn't exist).

**Specific steps:**
1. Create test cases:
   ```go
   func TestSignalDeduplicationKey_Deterministic()
   func TestSignalDeduplicationKey_DifferentLocations()
   func TestAggregateSignals_WeightedAverage()
   func TestAggregateSignals_MultipleAlgorithmsMergeSame()
   func TestAggregateSignals_DifferentLocationsStaySeparate()
   func TestAggregateSignals_NilLocationHandling()
   func TestAggregateSignals_AggregationCountMetadata()
   ```

2. Example test: weighted average aggregation
   ```go
   // Input: Same (source, target, location) from 3 algorithms
   // sig1: co-occurrence, confidence 0.6
   // sig2: structural, confidence 0.8
   // sig3: semantic, confidence 0.5
   // Expected: (0.6*0.3 + 0.8*0.6 + 0.5*0.7) / (0.3+0.6+0.7) ≈ 0.6625
   ```

3. Use table-driven tests for multiple algorithm weight combinations

**Verification:**
- All tests pass
- Coverage > 85% for algo_aggregator.go

---

### Task 6: Integration Test with Test-Data Corpus

**Action:** Create integration test using test-data/ to validate end-to-end deduplication.

**Specific steps:**
1. Create test in `graph_test.go` or `integration_test.go`:
   ```go
   func TestIntegration_PageindexDeduplication_WithTestData()
   ```

2. Test flow:
   - Load test-data corpus (62 documents)
   - Run discovery pipeline with pageindex enabled
   - Aggregate signals
   - Verify:
     - Same relationship detected by multiple algorithms → single edge
     - aggregation_count >= 1 for merged edges
     - All edges have Location populated (or nil with graceful degradation)
     - Confidence values in [0.4, 1.0]

3. Measure and report:
   - Total signals discovered before aggregation
   - Total edges after aggregation (should be < signal count)
   - Deduplication ratio (signals/edges)

**Verification:**
- Integration test passes with test-data
- Deduplication produces measurable reduction (e.g., 150 signals → 120 edges)
- No data loss; all relationships represented

---

## Verification Criteria

**What "done" looks like:**

1. ✓ **Code compiles:** All changes build without errors. `go build ./...` succeeds.

2. ✓ **Deduplication key works:** Signals with identical (source, target, file, line) produce one deduplication key. Different locations produce different keys.

3. ✓ **Weighted average aggregation:** Test signal set with known weights produces correct aggregated confidence (within 0.01 tolerance).

4. ✓ **Pageindex integration:** Discovery algorithms populate RelationshipLocation from pageindex output. If pageindex unavailable, signals still created but Location is nil (graceful degradation).

5. ✓ **Unit tests pass:** All test cases in Task 5 pass. Coverage >= 85% for algo_aggregator.go.

6. ✓ **Integration test validates:** With test-data corpus, deduplication produces measurable reduction (signals > edges). All edges have confidence in [0.4, 1.0].

7. ✓ **No regressions:** Existing Phase 1 tests still pass. Component type system still works.

8. ✓ **Documentation:** Code comments explain aggregation formula, weight values, and location semantics.

---

## Must-Haves (Phase Goal Backward)

**From phase goal:** "Establish confidence and provenance infrastructure"

This plan must deliver:
- ✓ Deduplication foundation using pageindex location data (required for confidence aggregation accuracy)
- ✓ Weighted average confidence aggregation (multiple signals → single trustworthy score)
- ✓ RelationshipLocation metadata (foundation for provenance in Plan 3)
- ✓ Graceful degradation if pageindex unavailable (robustness for Phase 3+ deployment)

---

## Dependency Notes

**Blocks:** Plans 02-02 (confidence tiers), 02-03 (provenance schema), 02-04 (cycle detection), 02-05 (query interface) all depend on aggregation foundation.

**Unblocks Phase 3:** Export pipeline can use aggregated signals with location metadata.

---

*Plan created: 2026-03-19*
*Autonomous execution: Yes (no decisions required from leadership)*
