---
phase: 12-signal-integration
plan: 01
subsystem: database, graph
tags: [sqlite, schema-migration, code-signals, probabilistic-OR, confidence-merging]

# Dependency graph
requires:
  - phase: 09-code-analysis-foundation
    provides: CodeSignal type and code analysis pipeline
  - phase: 02-accuracy-foundation
    provides: Edge struct, provenance columns, schema v5
provides:
  - Schema v6 with source_type column on graph_edges
  - code_signals provenance table for raw code analysis data
  - Signal conversion functions (CodeSignal to DiscoveredEdge)
  - Probabilistic OR confidence merging for multi-source edges
  - SaveCodeSignals for bulk provenance persistence
affects: [12-signal-integration plan 02, 13-mcp-server]

# Tech tracking
tech-stack:
  added: []
  patterns: [probabilistic-OR confidence merging, code signal to graph edge conversion]

key-files:
  created:
    - internal/knowledge/signal_convert.go
    - internal/knowledge/signal_convert_test.go
  modified:
    - internal/knowledge/db.go
    - internal/knowledge/edge.go
    - internal/knowledge/registry.go
    - internal/knowledge/relationship_types.go
    - internal/knowledge/algo_aggregator.go
    - internal/knowledge/export_test.go

key-decisions:
  - "All code detection kinds map to EdgeDependsOn for simplicity; fine-grained type info lives in code_signals table"
  - "Probabilistic OR formula: merged = 1-(1-a)*(1-b) for multi-source confidence"
  - "SourceType field on Edge defaults to 'markdown' for backward compatibility"
  - "code weight = 0.85 in AlgorithmWeight (between structural 0.6 and llm 1.0)"

patterns-established:
  - "Signal conversion: group by (source, target, edgeType), deduplicate, sort deterministically"
  - "Multi-source confidence: probabilistic OR only when both markdown and code signals present"

requirements-completed: [SIG-02]

# Metrics
duration: 18min
completed: 2026-04-02
---

# Phase 12 Plan 01: Signal Integration Infrastructure Summary

**Schema v6 migration with source_type tracking, code_signals provenance table, and probabilistic OR confidence merging for multi-source edges**

## Performance

- **Duration:** 18 min
- **Started:** 2026-04-01T06:32:29Z
- **Completed:** 2026-04-02T00:00:00Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Schema v6 migration adds source_type column to graph_edges (defaults to "markdown") and creates code_signals provenance table
- Edge.SourceType field round-trips through SaveGraph/LoadGraph with backward compatibility
- Signal conversion pipeline: convertCodeSignalsToDiscovered groups, deduplicates, and sorts code signals into DiscoveredEdge values
- Probabilistic OR formula (1-(1-a)*(1-b)) merges markdown + code confidence correctly (~0.84 for two 0.6 inputs)
- 12 TDD tests covering conversion, dedup, self-loop/empty-target filtering, confidence merging, source type determination, and stub node creation

## Task Commits

Each task was committed atomically:

1. **Task 1: Schema v6 migration and Edge.SourceType field** - `ed5a885` (feat)
2. **Task 2a: Failing TDD tests for signal conversion** - `c447e39` (test)
3. **Task 2b: Signal conversion function implementation** - `c819c0a` (feat)

## Files Created/Modified
- `internal/knowledge/signal_convert.go` - Code signal to DiscoveredEdge conversion, probabilistic OR, source type determination, stub node creation
- `internal/knowledge/signal_convert_test.go` - 12 TDD tests for all signal conversion functions
- `internal/knowledge/db.go` - Schema v6 migration, SaveCodeSignals, source_type in SaveGraph/LoadGraph, code_signals DDL
- `internal/knowledge/edge.go` - SourceType field on Edge, "code-analysis" extraction method, "markdown" default
- `internal/knowledge/registry.go` - SignalCode constant for code analysis signals
- `internal/knowledge/relationship_types.go` - SignalNER constant for completeness
- `internal/knowledge/algo_aggregator.go` - "code" algorithm weight (0.85)
- `internal/knowledge/export_test.go` - Updated schema version assertion from v5 to v6

## Decisions Made
- All code detection kinds (http_call, db_connection, cache_client, etc.) map to EdgeDependsOn; fine-grained type info preserved in code_signals table
- Probabilistic OR formula: merged = 1-(1-a)*(1-b) produces ~0.84 for two 0.6 confidence inputs
- SourceType defaults to "markdown" for backward compatibility with all existing edges
- Code weight set to 0.85 in AlgorithmWeight map (higher than semantic 0.7, lower than LLM 1.0)
- ensureCodeTargetNodes creates "infrastructure" type stubs for code-detected targets not in markdown graph

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed schema DDL for fresh databases**
- **Found during:** Task 1
- **Issue:** Fresh databases created with Initialize() would not have source_type column because schemaSQL DDL lacked it. Migrate() would not run because version was already set to 6.
- **Fix:** Added source_type column and code_signals table DDL to schemaSQL constant
- **Files modified:** internal/knowledge/db.go
- **Verification:** All SaveGraph/LoadGraph tests pass on fresh databases
- **Committed in:** ed5a885 (Task 1 commit)

**2. [Rule 1 - Bug] Fixed export test schema version assertion**
- **Found during:** Task 1
- **Issue:** TestExportPipelineEndToEnd asserted schema_version == "5", now fails with v6
- **Fix:** Updated assertion to expect "6"
- **Files modified:** internal/knowledge/export_test.go
- **Verification:** Test passes
- **Committed in:** ed5a885 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both fixes necessary for correctness. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Schema v6 infrastructure ready for Plan 02 to wire into export/crawl pipeline
- convertCodeSignalsToDiscovered and applyMultiSourceConfidence are pure functions ready for integration
- ensureCodeTargetNodes prevents dangling references when code signals introduce new components

---
## Self-Check: PASSED

All 7 key files verified present. All 3 task commits verified in git log.

*Phase: 12-signal-integration*
*Completed: 2026-04-02*
