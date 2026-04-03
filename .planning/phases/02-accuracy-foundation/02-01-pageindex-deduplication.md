---
phase: 02-accuracy-foundation
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/knowledge/algo_aggregator.go
  - internal/knowledge/types.go
  - internal/knowledge/pageindex.go
autonomous: true
requirements:
  - REL-05

must_haves:
  truths:
    - "Same relationship detected by multiple discovery algorithms at same file:line location produces single edge with aggregated confidence"
    - "DiscoverySignal includes location metadata (file, line, evidence)"
    - "AggregateSignals groups by (source, target, location) and computes weighted average"
    - "Deduplication keys are deterministic (file:line format)"
  artifacts:
    - path: "internal/knowledge/types.go"
      provides: "RelationshipLocation struct with File, Line, ByteOffset, Evidence fields"
      exports: ["RelationshipLocation", "RelationshipLocationKey"]
    - path: "internal/knowledge/algo_aggregator.go"
      provides: "Extended DiscoverySignal with Location; updated AggregateSignals logic"
      exports: ["AggregateSignals", "AggregateSignalsByLocation"]
      min_lines: 200
    - path: "internal/knowledge/pageindex.go"
      provides: "Calls to pageindex CLI for location mapping (already exists; verify integration)"
  key_links:
    - from: "discovery algorithms (cooccurrence, NER, semantic, LLM)"
      to: "pageindex.go"
      via: "CallPageindex() to map detected relationship to file:line"
      pattern: "CallPageindex.*file.*line"
    - from: "DiscoverySignal"
      to: "RelationshipLocation"
      via: "struct embedding or field"
      pattern: "Location.*RelationshipLocation|RelationshipLocation.*Location"
    - from: "AggregateSignals"
      to: "weighted average aggregation"
      via: "signature (source, target, location) → single edge with aggregated confidence"
      pattern: "aggregated_confidence.*=.*weighted_sum.*weighted_count"

---

<objective>
Integrate pageindex location data into the discovery pipeline so that identical relationships detected by multiple algorithms at the same file location are deduplicated and confidence scores are aggregated using weighted averaging.

Purpose: REL-05 is a hard dependency for all other Phase 2 requirements. Pageindex provides location tracking that enables:
- Deduplication (same relationship at same location = 1 edge, not 3)
- Aggregation (combine confidence scores from multiple algorithms)
- Traceability (agents see evidence location)

Output: Updated DiscoverySignal with location, new AggregateSignals logic grouping by (source, target, location), deterministic deduplication keys.
</objective>

<execution_context>
@/Users/flurryhead/.claude/get-shit-done/workflows/execute-plan.md
@/Users/flurryhead/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/phases/02-accuracy-foundation/02-RESEARCH.md
@.planning/phases/02-accuracy-foundation/02-CONTEXT.md
@internal/knowledge/types.go
@internal/knowledge/algo_aggregator.go
@internal/knowledge/pageindex.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Define RelationshipLocation struct and helper functions</name>
  <files>internal/knowledge/types.go</files>
  <action>
Add to types.go:

1. Define RelationshipLocation struct with fields: File (string), Line (int), ByteOffset (int), Evidence (string)
2. Implement RelationshipLocationKey(loc RelationshipLocation) string that returns fmt.Sprintf("%s:%d", loc.File, loc.Line) — deterministic dedup key
3. Add location field validation: File must be non-empty and relative path (no leading /); Line must be >= 0
4. Add method Location.String() for logging
5. Update DiscoverySignal struct to include Location field of type RelationshipLocation (add as new field, not replacing existing Evidence)

Validation:
- RelationshipLocation with empty File is invalid (will be caught in next task's aggregation logic)
- Location keys for same file:line are identical across multiple calls
- Evidence is preserved per-location

Reference existing code: DiscoverySignal already in types.go (review current struct definition); Edge struct has Evidence field (existing pattern to follow for provenance fields).
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestRelationshipLocation -v
Expected: Tests pass for LocationKey determinism, Location validation, String() output
Also verify: go build ./cmd/graphmd — no compilation errors for new types.go changes
  </verify>
  <done>
- RelationshipLocation struct exists with File, Line, ByteOffset, Evidence fields
- RelationshipLocationKey() function returns deterministic keys (e.g., "docs/service.yaml:42")
- DiscoverySignal includes Location field without breaking existing field order
- NewDiscoverySignal() or constructor updated to accept location parameter
  </done>
</task>

<task type="auto">
  <name>Task 2: Extend DiscoverySignal integration in discovery algorithms</name>
  <files>internal/knowledge/algo_aggregator.go</files>
  <action>
Modify algo_aggregator.go to wire pageindex location into discovery signals:

1. In existing discovery algorithm calls (wherever DiscoverySignal is created: co-occurrence, NER, structural, semantic, LLM), add pageindex lookup:
   - After detecting relationship (source → target), call existing pageindex.CallPageindex() with the markdown file path
   - Extract file:line from pageindex response
   - Populate DiscoverySignal.Location with returned file:line and evidence snippet

2. Update AggregateSignals() signature: currently groups by (source, target), must now group by (source, target, location)
   - Create grouping key: fmt.Sprintf("%s|%s|%s", source, target, RelationshipLocationKey(location))
   - Group all signals with same key

3. For each group with same (source, target, location):
   - Compute weighted average: Σ(confidence_i * weight_i) / Σ(weight_i)
   - Algorithm weights (from CONTEXT.md): Co-occurrence=0.3x, NER=0.5x, Structural=0.6x, Semantic=0.7x, LLM=1.0x
   - Result confidence is the aggregated score
   - Result Location is first signal's location (all in group have same location by definition)
   - Result Evidence is concatenated or summary of evidence from all signals

4. Update function comment to document: "AggregateSignals groups by (source, target, location) and returns deduplicated edges with aggregated confidence scores using weighted averaging per algorithm."

Reference research section 2.3 for RelationshipLocation struct and workflow.
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestAggregateSignals -v
Expected:
  - TestAggregateSignalsWithLocation passes (same source/target/location from 3 algorithms → 1 edge)
  - TestWeightedAggregation passes (sample calculation matches expected average)
  - TestDifferentLocations passes (same source/target but different lines → multiple edges)

Sampling: Run on existing test-data corpus to ensure no regressions
  </verify>
  <done>
- DiscoverySignal includes location metadata from pageindex
- AggregateSignals groups by (source, target, location) triple
- Weighted averaging implemented per algorithm weights
- Same relationship at same location produces single edge with aggregated confidence
- Different locations produce separate edges (even if same source/target pair)
  </done>
</task>

<task type="auto">
  <name>Task 3: Integration test — pageindex deduplication with test-data corpus</name>
  <files>internal/knowledge/algo_aggregator_test.go</files>
  <action>
Create integration test TestPageindexDeduplication_E2E:

1. Load test-data corpus (e.g., test-data/payment-service/)
2. Run discovery algorithms (at minimum: co-occurrence, NER; LLM optional if available)
3. Collect all DiscoverySignals with location metadata populated via pageindex
4. Call AggregateSignals() to deduplicate
5. Assert:
   - Same relationship from multiple algorithms at same file:line → single edge in result
   - Confidence of aggregated edge is weighted average (sample: 2 signals at 0.6 and 0.8 with weights 0.3 and 0.6 → (0.6*0.3 + 0.8*0.6) / (0.3+0.6) ≈ 0.73)
   - Different file:line references for same source/target → separate edges in result
   - All locations are non-null and deterministic

Run test with verbose output to confirm signal grouping logic. No false positives (should NOT merge edges with same source/target but different file:line).

Benchmark: Aggregation should complete on 62-document corpus in <500ms.
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestPageindexDeduplication_E2E -v
Expected: Test passes, verbose output shows signal grouping and aggregation steps
Benchmark: go test ./internal/knowledge -bench BenchmarkAggregateSignals -benchmem
Expected: Processing 500+ signals completes in <500ms
  </verify>
  <done>
- Integration test verifies full pipeline: discovery → location mapping → aggregation → deduplication
- Weighted averaging produces correct confidence scores
- Multiple algorithms for same location result in one deduplicated edge
- All edges have non-null, deterministic location keys
- Performance acceptable for corpus size
  </done>
</task>

</tasks>

<verification>
**Comprehensive checks before marking complete:**

1. **Location Key Determinism:** Run same discovery twice on same corpus; verify AggregateSignals produces identical grouping keys both times (reproducibility)

2. **Weighted Average Correctness:** Create unit test with known signal combination:
   - 3 signals for (payment-api → primary-db) at docs/service.yaml:42
   - Confidences: 0.6 (weight 0.3), 0.8 (weight 0.6), 0.5 (weight 0.7)
   - Expected: (0.6*0.3 + 0.8*0.6 + 0.5*0.7) / (0.3+0.6+0.7) = 0.6265
   - Verify aggregated confidence ≈ 0.6265

3. **No False Merging:** Create two relationships at different file:line:
   - (A → B) at file1.md:10
   - (A → B) at file2.md:20
   - Verify AggregateSignals produces 2 separate edges (not merged)

4. **Corpus Round-Trip:** Load test-data, run discovery, aggregate, verify no crashes; relationships have location data populated

5. **Backward Compatibility:** Ensure existing Edge/Graph code not broken by DiscoverySignal.Location addition (new field is additive)
</verification>

<success_criteria>
- REL-05 satisfied: Pageindex location deduplication working end-to-end
- Same relationship from 3 discovery algorithms at same file:line → 1 edge with aggregated confidence
- Weighted aggregation formula correct (unit + integration tests pass)
- All DiscoverySignal instances include non-null location metadata
- AggregateSignals groups deterministically by (source, target, location)
- Performance: <500ms for 62-doc corpus with 500+ signals
- No false merging of different-location edges
- Backward compatible with Phase 1 Edge/Graph structures
</success_criteria>

<output>
After completion, create `.planning/phases/02-accuracy-foundation/02-01-SUMMARY.md` documenting:
- RelationshipLocation struct and helper functions added
- AggregateSignals updated to group by (source, target, location)
- Weighted averaging implemented per algorithm
- Integration test verifies deduplication and aggregation
- Performance characteristics on test-data corpus
</output>
