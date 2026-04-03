# Project State: graphmd v1

**Last updated:** 2026-03-19
**Current phase:** Phase 2 - Accuracy Foundation (5/5 plans complete)
**Status:** Phase 2 complete

## Phase Progress

| Phase | Name | Status | Requirements | Completed |
|-------|------|--------|-------------|-----------|
| 1 | Component Model | Complete (3/3 plans) | COMP-01, COMP-02, COMP-03 | 3/3 |
| 2 | Accuracy Foundation | Complete (5/5 plans) | REL-01, REL-02, REL-03, REL-04, REL-05 | 5/5 |
| 3 | Extract & Export Pipeline | Not started | EXTRACT-01, EXTRACT-02, EXTRACT-03, EXPORT-01, EXPORT-02 | 0/5 |
| 4 | Import & Query Pipeline | Not started | IMPORT-01, IMPORT-02, IMPORT-03 | 0/3 |
| 5 | Crawl Exploration | Not started | CRAWL-01, CRAWL-02 | 0/2 |

## Overall Progress

- **Total requirements:** 18
- **Completed:** 8 (COMP-01, COMP-02, COMP-03, REL-05, REL-01, REL-02, REL-03, REL-04)
- **In progress:** 0
- **Not started:** 10
- **Completion:** 44%

## Current Focus

Phase 2 complete (5/5 plans). All REL requirements satisfied.

### Next Actions

1. Phase 3, Plan 1: Extract & Export Pipeline

## Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-03-16 | 5-phase roadmap derived from requirements | Standard depth; phases follow natural dependency order |
| 2026-03-16 | Pageindex as hard dependency in Phase 2 | Required for deduplication and location tracking before export pipeline |
| 2026-03-16 | Phases 4 and 5 can parallelize | Crawl exploration has no dependency on import/query pipeline |
| 2026-03-16 | 12-type taxonomy based on Backstage/Cartography patterns | Covers common infrastructure component categories |
| 2026-03-16 | Longest-match pattern strategy for type inference | Handles ambiguous names correctly; 3-tier confidence |
| 2026-03-16 | SeedConfig with glob patterns for user extensibility | Override auto-detection at confidence 1.0 |
| 2026-03-16 | ComponentTypeAPI/Config as backward-compatible aliases | Avoids breaking existing registry data |
| 2026-03-19 | Weighted average for signal aggregation (not max) | Better reflects combined evidence strength from multiple algorithms |
| 2026-03-19 | Algorithm weights: cooccurrence=0.3, ner=0.5, structural=0.6, semantic=0.7, llm=1.0 | Higher-quality algorithms weighted more heavily |
| 2026-03-19 | Preserved existing AggregateSignals() alongside new AggregateSignalsByLocation() | Backward compatibility for callers without location data |
| 2026-03-19 | 6 confidence tiers (explicit through threshold) with rank-based comparison | Enables AI agents to filter by semantic tier name or numeric score |
| 2026-03-19 | MinTier/MinScore as optional pointer fields in DiscoveryFilterConfig | Backward compatible; nil means no additional filtering |
| 2026-03-19 | Edge copies returned from TraverseDFS (not mutating originals) | Traversal results independent of shared graph state |
| 2026-03-19 | GetImpact defaults maxDepth=1 when caller passes 0 | Safe direct-only default for impact queries |
| 2026-03-19 | Lenient provenance validation (zero-value passes ValidateEdge) | Backward compatible; legacy edges without provenance still valid |
| 2026-03-19 | 6 extraction methods: explicit-link, co-occurrence, structural, NER, semantic, LLM | Covers all current discovery algorithms |
| 2026-03-19 | Provenance stored as SQL NULL when empty (not empty string) | Enables proper IS NULL queries for filtering |
| 2026-03-19 | BFS distance computation separate from DFS traversal | Accurate hop distances for AffectedNode in query results |
| 2026-03-19 | TraverseMode direct/cascade/full for ImpactQuery | Controls edge inclusion scope for different query intents |
| 2026-03-19 | CrawlQuery safety limit of 100 for unbounded depth | Prevents runaway queries on large graphs |

## Blockers

None.

## Notes

- Brownfield project: scanner, extractor, discovery algorithms, and export logic exist
- cmdExport stub in main.go needs wiring to CmdExport (Phase 3)
- importtar.go has implementation but no CLI command (Phase 4)
- All code in single package `internal/knowledge` -- sub-packaging may be needed as complexity grows
- MCP server deferred to v2
- Index command now creates graph nodes from documents (was missing before Plan 1)
- SaveGraph skips edges with dangling node references (FK safety)

## Execution Metrics

| Phase | Plan | Duration | Tasks | Files |
|-------|------|----------|-------|-------|
| 01-01 | Component Type System | 11 min | 5 | 7 |
| 01-02 | User-Facing Documentation & QA | 32 min | 7 | 10 |
| 01-01-closure | Documentation & Runtime Validation | 25 min | 4 | 2 |
| 02-01 | Pageindex Integration & Deduplication | 15 min | 3 | 4 |
| 02-02 | Confidence Tier System | 17 min | 3 | 4 |
| 02-04 | Cycle Detection & Traversal | 6 min | 3 | 4 |
| 02-03 | Edge Provenance Schema | 15 min | 4 | 4 |
| 02-05 | Query Interface | 8 min | 3 | 4 |

---
*Initialized: 2026-03-16*
*Last plan completed: 02-05 Query Interface (2026-03-19)*
