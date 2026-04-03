---
phase: 12-signal-integration
plan: 02
subsystem: graph, cli
tags: [code-signals, source-type, pipeline-integration, query-filter, probabilistic-OR]

# Dependency graph
requires:
  - phase: 12-signal-integration plan 01
    provides: Schema v6, signal conversion functions, probabilistic OR, SaveCodeSignals
  - phase: 09-code-analysis-foundation
    provides: CodeSignal type, RunCodeAnalysis, InferSourceComponent
provides:
  - Code signal integration into export and crawl pipelines (full graph integration, not print-and-discard)
  - source_type field on every EnrichedRelationship in query JSON output
  - --source-type filter flag on all query subcommands (impact, dependencies, path, list)
  - Shared integrateCodeSignals() helper preventing export/crawl divergence
affects: [13-mcp-server]

# Tech tracking
tech-stack:
  added: []
  patterns: [shared pipeline integration helper, source-type filter semantics]

key-files:
  created: []
  modified:
    - internal/knowledge/export.go
    - internal/knowledge/crawl_cmd.go
    - internal/knowledge/signal_convert.go
    - internal/knowledge/query_cli.go
    - internal/knowledge/query_cli_test.go

key-decisions:
  - "Shared integrateCodeSignals() function prevents export/crawl pipeline divergence"
  - "Source type filter semantics: code matches code+both, markdown matches markdown+both, both matches only both"
  - "EnrichedRelationship.SourceType has no omitempty - always present in JSON output"
  - "Edges without explicit SourceType default to 'markdown' via edgeSourceType() helper"
  - "Code edges added to graph inside integrateCodeSignals; markdown edges already added in Step 7"

patterns-established:
  - "Filter composition: --source-type and --min-confidence compose independently (edge must pass both)"
  - "edgeSourceType() helper for consistent default-to-markdown behavior"

requirements-completed: [SIG-01]

# Metrics
duration: 8min
completed: 2026-04-02
---

# Phase 12 Plan 02: Pipeline Integration and Query Filter Summary

**Code signals wired into export/crawl pipelines with source_type on all query output and --source-type filter for agent-driven source provenance queries**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-02T04:37:30Z
- **Completed:** 2026-04-02T04:45:30Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Export and crawl pipelines now integrate code signals into the graph instead of printing and discarding them
- Shared integrateCodeSignals() function converts signals, ensures stub nodes, merges with markdown edges, applies probabilistic OR confidence, and sets source_type
- SaveCodeSignals called after DB save for provenance persistence in export pipeline
- EnrichedRelationship always includes source_type field in JSON query output (defaults to "markdown" for backward compatibility)
- --source-type filter available on all 4 query subcommands with correct semantics
- 7 new tests covering all filter modes, JSON output, default behavior, and both traversal directions

## Task Commits

Each task was committed atomically:

1. **Task 1: Integrate code signals into CmdExport and CmdCrawl pipelines** - `93e7b38` (feat)
2. **Task 2: Add --source-type filter and source_type to query JSON output** - `b9ce776` (feat)

## Files Created/Modified
- `internal/knowledge/export.go` - Replaced print-and-discard Step 7b with full integration; SaveCodeSignals after DB save
- `internal/knowledge/crawl_cmd.go` - Same integration using shared integrateCodeSignals()
- `internal/knowledge/signal_convert.go` - Added integrateCodeSignals() shared helper
- `internal/knowledge/query_cli.go` - SourceType on EnrichedRelationship, --source-type flag on all queries, filter helpers, updated BFS traversals
- `internal/knowledge/query_cli_test.go` - 7 new tests for source type output and filtering

## Decisions Made
- Extracted shared integrateCodeSignals() to prevent export/crawl divergence (as recommended by research)
- Filter semantics: "code" matches code+both, "markdown" matches markdown+both, "both" matches only both -- this means "show me everything code was involved in" or "show me everything markdown was involved in"
- EnrichedRelationship.SourceType never has omitempty, ensuring agents always see the field
- Code edges are added to the graph inside integrateCodeSignals; markdown edges were already added in Step 7 before code integration runs

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 12 (Signal Integration) is now complete
- All code analysis signals flow through the full pipeline: detect -> convert -> merge -> save -> query
- Query output enriched with source_type on every relationship
- Agents can filter by detection source provenance
- Ready for Phase 13 (MCP Server) which will expose enriched hybrid graph via MCP protocol

---
## Self-Check: PASSED

All 5 modified files verified present. Both task commits verified in git log.

*Phase: 12-signal-integration*
*Completed: 2026-04-02*
