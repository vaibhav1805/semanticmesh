# Phase 2: Accuracy Foundation - Plan Summary

**Phase:** 02-accuracy-foundation
**Status:** Ready for execution
**Created:** 2026-03-19

---

## Phase Goal

Establish the confidence and provenance infrastructure that prevents false edges from misleading AI agents during incident response.

## Requirements Coverage

| ID | Plan | Requirement | Status |
|----|------|-------------|--------|
| REL-05 | 02-PLAN-01-pageindex | Pageindex as hard dependency (location tracking & deduplication) | Planned (Wave 1) |
| REL-01 | 02-PLAN-02-confidence | 7-tier confidence system | Planned (Wave 2) |
| REL-02 | 02-PLAN-03-provenance | Provenance metadata (source file, extraction method, timestamps) | Planned (Wave 2) |
| REL-03 | 02-PLAN-04-cycles | Cycle-safe traversal (visited set tracking) | Planned (Wave 3) |
| REL-04 | 02-PLAN-05-query | Direct-edge-only default in queries; JSON output | Planned (Wave 3) |

**Coverage:** 5/5 requirements mapped to 5 executable plans

---

## Execution Waves

### Wave 1 (Hard Dependency - Must Run First)

**Plan 02-PLAN-01-pageindex** (REL-05)
- Integrate pageindex subprocess for relationship location tracking
- Implement location-based deduplication in algo_aggregator
- Extend DiscoverySignal with RelationshipLocation
- Implement weighted average confidence aggregation
- **Blocks:** All other Phase 2 plans
- **Deliverable:** Aggregated signals with location metadata, single edges from multi-algorithm detections

**Autonomy:** Autonomous (no decisions required)

---

### Wave 2 (Parallel Execution)

**Plan 02-PLAN-02-confidence** (REL-01)
- Define 7-tier confidence system (explicit, strong-inference, moderate, weak, semantic, threshold)
- Implement ScoreToTier / TierToScore mapping functions
- Update discovery algorithms to use tier-aware confidence
- Support CLI filtering by --min-tier and --min-confidence
- **Depends on:** Plan 1 (aggregation must exist first)
- **Deliverable:** Tier definitions, mapping functions, tier-filtered query support

**Autonomy:** Autonomous

---

**Plan 02-PLAN-03-provenance** (REL-02)
- Extend Edge struct with provenance fields (source_file, extraction_method, detection_evidence, evidence_pointer, last_modified)
- Create SQLite v3→v4 schema migration
- Update SaveGraph/LoadGraph to persist and retrieve provenance
- Add provenance validation (non-null constraints)
- **Depends on:** Plan 1 (Location metadata from pageindex flows here)
- **Deliverable:** Provenance-enriched Edge struct, v3→v4 migration, round-trip persistence

**Autonomy:** Autonomous

---

### Wave 3 (Parallel Execution)

**Plan 02-PLAN-04-cycles** (REL-03)
- Implement TraversalState with visited set + path tracking
- Add cycle detection to DFS/BFS traversal
- Mark cyclic edges with relationship_type field
- Ensure queries complete < 1 second with cyclic graphs
- **Depends on:** Plan 1 (aggregation foundation)
- **Deliverable:** Cycle-safe traversal, TraversalState, cyclic edge marking

**Autonomy:** Autonomous

---

**Plan 02-PLAN-05-query** (REL-04)
- Implement Impact query (focused incident response: direct edges by default, depth-limited, confidence-filtered)
- Implement Crawl query (exploratory: all relationships, no filtering)
- Support --depth, --traverse-mode, --min-tier, --min-confidence flags
- Return consistent JSON schema (nodes + edges) for agent parsing
- **Depends on:** Plans 1–4 (all foundational work must complete)
- **Deliverable:** Query router (impact/crawl), JSON output formatter, CLI commands

**Autonomy:** Autonomous

---

## Dependency Graph

```
Wave 1:
  02-PLAN-01-pageindex (REL-05)
       ↓ BLOCKS
Wave 2:
  02-PLAN-02-confidence (REL-01) ─────┐
  02-PLAN-03-provenance (REL-02) ─────┼─ PARALLEL
       ↓ DEPEND ON Wave 1              │
Wave 3:
  02-PLAN-04-cycles (REL-03) ─────────┼─ PARALLEL
  02-PLAN-05-query (REL-04) ──────────┤
       ↓ DEPEND ON Plans 1-4           │
       ✓ PHASE COMPLETE ────────────────┘
```

**Critical path:**
1. Plan 1 (pageindex) — 1 day
2. Plans 2–3 in parallel — 2–3 days
3. Plans 4–5 in parallel — 2–3 days
4. **Total: 5–7 days estimated**

---

## Success Criteria Summary

### Phase-Level Verification

1. ✓ **Confidence tiers:** Every relationship has confidence ∈ [0.4, 1.0] mapped to one of 7 tiers
2. ✓ **Provenance complete:** Every relationship includes non-null source_file, extraction_method, last_modified
3. ✓ **Cycle safety:** Cyclic graph (A→B→C→A) completes impact analysis in < 1 second
4. ✓ **Direct edges default:** Impact query returns only direct dependents by default (depth=1)
5. ✓ **Deduplication:** Same relationship from 3 algorithms at same location = 1 edge with aggregated confidence
6. ✓ **JSON output:** Queries return valid JSON with complete subgraph topology (nodes + edges)
7. ✓ **No regressions:** All Phase 1 tests (component types) still pass

### Per-Plan Verification

See individual PLAN files for detailed verification criteria.

---

## File Changes Summary

| File | Status | Lines | Impact |
|------|--------|-------|--------|
| `confidence.go` | NEW | ~100 | Tier system implementation |
| `query.go` | NEW | ~200 | Query interface implementation |
| `traversal.go` | NEW | ~150 | Cycle detection |
| `edge.go` | MODIFY | +40 | Provenance fields + validation |
| `algo_aggregator.go` | MODIFY | +80 | Location-based grouping, weighted avg |
| `discovery_orchestration.go` | MODIFY | +50 | Pageindex integration |
| `db.go` | MODIFY | +120 | v3→v4 migration, SaveGraph/LoadGraph updates |
| `graph.go` | MODIFY | +60 | Traversal updates, cycle detection |
| `types.go` | MODIFY | +20 | RelationshipType, ConfidenceTier enums |
| `cmd/graphmd/main.go` | MODIFY | +100 | Query CLI commands |
| Test files (`*_test.go`) | NEW/MODIFY | ~400 | Unit + integration tests |

**Total new/modified code:** ~1,120 lines (estimated)

---

## Risk Mitigation

| Risk | Mitigation | Plan |
|------|-----------|------|
| Pageindex availability (external subprocess) | Test graceful degradation; make optional for testing | 1 |
| Schema migration atomicity | Use transaction wrapper; test on existing test-data DB | 3 |
| Weighted aggregation correctness | Unit test with known signal combinations | 1 |
| Cycle detection performance | Benchmark on large cyclic graphs; aim for < 1 sec | 4 |
| Backward compatibility with Phase 1 | Test import of old graphs; handle NULL provenance | 3 |
| JSON output correctness | Validate with jq parser; test all query types | 5 |

---

## Testing Strategy

### Unit Testing
- Confidence tier mapping (ScoreToTier, TierToScore)
- Aggregation formula (weighted average with algorithm weights)
- Cycle detection (WouldCreateCycle, path tracking)
- Provenance validation (non-null constraints)
- Query filtering (confidence, depth, traverse mode)

**Target:** Coverage >= 85% for each new/modified module

### Integration Testing
- Full pipeline: discovery → aggregation → save → load
- Cycle handling with synthetic and real test data
- Confidence aggregation correctness (multiple algorithms → single edge)
- Provenance round-trip (save → load preserves all fields)
- Query interface with test-data corpus

### Validation Testing
- Performance: Queries complete < 2 seconds on test-data (62 docs)
- Backward compatibility: Old exports (Phase 1) can be imported
- JSON schema: Output parseable by jq
- Confidence distribution: Test data produces relationships across 7 tiers

---

## Phase Completion Checklist

- [ ] Plan 02-PLAN-01-pageindex executed and verified
- [ ] Plan 02-PLAN-02-confidence executed and verified
- [ ] Plan 02-PLAN-03-provenance executed and verified
- [ ] Plan 02-PLAN-04-cycles executed and verified
- [ ] Plan 02-PLAN-05-query executed and verified
- [ ] All phase-level verification criteria met
- [ ] No regressions from Phase 1
- [ ] Integration test passes (full stack with test-data)
- [ ] Performance target met (< 2 sec per query)
- [ ] STATE.md updated with Phase 2 completion

---

## Next Phase

**Phase 3: Extract & Export Pipeline**

After Phase 2 completion:
1. Wire pageindex-aggregated signals into export pipeline
2. Apply confidence tiers and provenance to exported edges
3. Create SQLite database with indexed schema
4. Package as ZIP with metadata
5. Validate end-to-end: markdown → graph → ZIP

**Dependencies:** Requires Phase 2 completion (confidence + provenance + aggregation)

---

*Plan summary created: 2026-03-19*
*Autonomous execution: All 5 plans*
*Estimated duration: 5–7 days*
