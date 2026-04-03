---
phase: 09-code-analysis-foundation
plan: 02
subsystem: code-analysis
tags: [cli-integration, analyze-code-flag, end-to-end-test, go-parser]

# Dependency graph
requires:
  - phase: 09-01
    provides: CodeSignal struct, LanguageParser interface, CodeAnalyzer, GoParser
provides:
  - RunCodeAnalysis integration function for CLI commands
  - PrintCodeSignalsSummary diagnostic output for stderr
  - --analyze-code opt-in flag on export, crawl, and index commands
  - End-to-end test validating full Go source -> CodeSignal pipeline
affects: [10-python-js-parsers, 12-signal-merging, 13-mcp-server]

# Tech tracking
tech-stack:
  added: []
  patterns: [variadic parser injection to avoid import cycles, opt-in CLI flag pattern]

key-files:
  created:
    - internal/code/integration.go
    - internal/code/integration_test.go
  modified:
    - internal/knowledge/export.go
    - internal/knowledge/crawl_cmd.go
    - cmd/graphmd/main.go

key-decisions:
  - "Variadic parser args on RunCodeAnalysis to avoid import cycle between code and goparser packages"
  - "Code signals printed to stderr as diagnostic output -- not yet integrated into graph (Phase 12)"
  - "External test package (code_test) for integration tests to allow importing both code and goparser"

patterns-established:
  - "CLI opt-in flag pattern: --analyze-code enables new behavior with zero default-path impact"
  - "Parser injection via function args rather than global registration for testability"

requirements-completed: [CODE-01]

# Metrics
duration: 4min
completed: 2026-03-30
---

# Phase 9 Plan 2: CLI Integration Summary

**RunCodeAnalysis integration layer with --analyze-code opt-in flag on export/crawl/index and end-to-end test proving Go source to CodeSignal pipeline**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-30T07:27:10Z
- **Completed:** 2026-03-30T07:31:24Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- RunCodeAnalysis orchestrates the full pipeline: infer source component, register parsers, analyze directory, return signals
- PrintCodeSignalsSummary provides concise detection kind counts for stderr diagnostic output
- All three CLI commands (export, crawl, index) accept --analyze-code flag with zero default-path impact
- End-to-end test validates HTTP call, DB connection, and comment hint detection with test file exclusion
- 4 new integration tests pass alongside all 23 existing code analysis tests (27 total)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create integration layer and end-to-end test** - `49d92ee` (feat)
2. **Task 2: Add --analyze-code flag to export, crawl, and index commands** - `a5f6cc1` (feat)

## Files Created/Modified
- `internal/code/integration.go` - RunCodeAnalysis and PrintCodeSignalsSummary functions
- `internal/code/integration_test.go` - End-to-end test with temp Go source fixtures
- `internal/knowledge/export.go` - Added AnalyzeCode field and --analyze-code flag to export command
- `internal/knowledge/crawl_cmd.go` - Added AnalyzeCode field and --analyze-code flag to crawl command
- `cmd/graphmd/main.go` - Added --analyze-code flag to index command

## Decisions Made
- Used variadic parser args (`RunCodeAnalysis(dir, parsers...)`) instead of global registration to avoid import cycle between `internal/code` and `internal/code/goparser` packages. This is cleaner than the originally planned direct import approach.
- External test package (`package code_test`) for integration tests since they need to import both `code` and `goparser` without creating cycles.
- Code signals are printed to stderr only -- no graph integration yet (deferred to Phase 12 signal merging).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Restructured RunCodeAnalysis to avoid import cycle**
- **Found during:** Task 1 (integration layer creation)
- **Issue:** Plan specified `RunCodeAnalysis` in `internal/code/integration.go` directly importing `goparser`, but goparser already imports `code` for the CodeSignal type, creating a circular dependency
- **Fix:** Changed RunCodeAnalysis to accept parsers as variadic arguments; CLI commands pass `goparser.NewGoParser()` at call sites. Test uses external test package.
- **Files modified:** internal/code/integration.go, internal/code/integration_test.go
- **Verification:** All tests pass, `go vet ./...` clean, no import cycles
- **Committed in:** 49d92ee (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Essential fix for Go import cycle. API surface matches plan intent (single RunCodeAnalysis call from CLI commands). No scope creep.

## Issues Encountered
None beyond the auto-fixed import cycle.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 9 complete: Go code analysis foundation fully integrated into CLI
- Phase 10 (Python/JS parsers) can add parsers and pass them alongside GoParser in CLI commands
- Phase 12 (signal merging) will consume the CodeSignal output to enrich the graph
- All existing tests continue to pass (no regressions)

---
*Phase: 09-code-analysis-foundation*
*Completed: 2026-03-30*
