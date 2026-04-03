---
phase: 11-connection-strings-comment-analysis
plan: 03
subsystem: code-analysis
tags: [parser-integration, connstring, comments, env-var, boost, refactor]

# Dependency graph
requires:
  - phase: 11-connection-strings-comment-analysis
    provides: Shared connstring package (11-01) and comments package (11-02)
  - phase: 09-code-analysis-foundation
    provides: CodeSignal struct, LanguageParser interface, RunCodeAnalysis
provides:
  - All three parsers (Go, Python, JS/TS) using shared connstring and comments packages
  - Per-parser extractURLHost and comment hint code removed (deduplication)
  - Env var reference detection in all parsers
  - Known-component boost pass in RunCodeAnalysis
affects: [12-signal-merging]

# Tech tracking
tech-stack:
  added: []
  patterns: [shared-utility-integration, known-component-boosting, env-var-ref-detection]

key-files:
  created: []
  modified:
    - internal/code/goparser/parser.go
    - internal/code/goparser/patterns.go
    - internal/code/pyparser/parser.go
    - internal/code/pyparser/patterns.go
    - internal/code/jsparser/parser.go
    - internal/code/jsparser/patterns.go
    - internal/code/integration.go
    - internal/code/integration_test.go

key-decisions:
  - "Go parser extractTarget returns (target, targetType) tuple to pass connstring-enriched type info"
  - "extractURLHost in Python and JS parsers delegates to connstring.Parse instead of net/url directly"
  - "inferEnvVarTargetType uses prefix-based heuristic (DATABASE_->database, REDIS_->cache, etc.)"
  - "boostKnownComponents runs as two-pass in-place mutation on signals slice (no interface changes)"

patterns-established:
  - "Parser integration: import shared package, call after main loop, set Language/SourceFile on returned signals"
  - "Env var detection: ParseEnvVarRef + IsConnectionEnvVar filter + inferEnvVarTargetType for type classification"

requirements-completed: [CODE-04, CODE-05]

# Metrics
duration: 6min
completed: 2026-04-01
---

# Phase 11 Plan 03: Parser Integration Summary

**All three parsers migrated to shared connstring/comments packages with env var detection and known-component confidence boosting**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-01T03:30:59Z
- **Completed:** 2026-04-01T03:37:25Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Replaced per-parser extractURLHost with shared connstring.Parse in Go, Python, and JS/TS parsers
- Replaced per-parser commentHintPattern/commentHintRe with shared comments.Analyze
- Added env_var_ref signal detection (confidence 0.7) using connstring.ParseEnvVarRef across all languages
- Implemented boostKnownComponents two-pass in RunCodeAnalysis: comment hints referencing code-detected components get boosted to 0.5
- All 80+ existing tests pass with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Integrate shared utilities into all parsers** - `39fca7e` (feat)
2. **Task 2: Add known-component boost pass in RunCodeAnalysis** - `3b0845d` (feat)

## Files Created/Modified
- `internal/code/goparser/parser.go` - Uses connstring.Parse and comments.Analyze, env var detection
- `internal/code/goparser/patterns.go` - Removed commentHintPattern and regexp import
- `internal/code/pyparser/parser.go` - Uses connstring.Parse and comments.Analyze, env var detection
- `internal/code/pyparser/patterns.go` - Removed commentHintRe
- `internal/code/jsparser/parser.go` - Uses connstring.Parse and comments.Analyze, env var detection
- `internal/code/jsparser/patterns.go` - Removed commentHintPattern
- `internal/code/integration.go` - Added boostKnownComponents function and call in RunCodeAnalysis
- `internal/code/integration_test.go` - Added TestBoostKnownComponents

## Decisions Made
- Go parser's extractTarget returns a (target, targetType) tuple so connstring.Parse can enrich the TargetType field when the URL scheme provides type information
- Python/JS extractURLHost functions now delegate to connstring.Parse, which handles URLs, DSNs, and host:port -- a superset of the old net/url.Parse approach
- inferEnvVarTargetType is duplicated per parser (3 identical copies) rather than shared, to avoid adding a cross-package dependency for a trivial function; could be consolidated later if needed
- boostKnownComponents modifies signals in-place (no copy), keeping it zero-allocation

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated TestCommentLineSkipped expectation in Python parser**
- **Found during:** Task 1
- **Issue:** The test expected 0 signals from `# requests.get("http://old-service/api")`, but the shared comment analyzer correctly detects the URL as a comment_hint signal
- **Fix:** Changed test to verify no http_call signals are produced (the intended behavior), while allowing comment_hint signals from URL detection
- **Files modified:** internal/code/pyparser/parser_test.go
- **Verification:** All pyparser tests pass
- **Committed in:** 39fca7e (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 test expectation update)
**Impact on plan:** Test assertion adjusted to match correct new behavior. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 11 complete: shared connstring, comments, and parser integration all done
- All code analysis parsers produce enriched signals with connstring types, comment analysis, and env var refs
- Ready for Phase 12 (signal merging) which combines code signals with markdown-derived graph

---
*Phase: 11-connection-strings-comment-analysis*
*Completed: 2026-04-01*
