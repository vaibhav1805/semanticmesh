---
phase: 09-code-analysis-foundation
plan: 01
subsystem: code-analysis
tags: [go-ast, static-analysis, infrastructure-detection, parser]

# Dependency graph
requires: []
provides:
  - CodeSignal struct with 8 fields for infrastructure dependency signals
  - LanguageParser interface for multi-language code analysis
  - CodeAnalyzer orchestrator with file dispatch and directory walking
  - GoParser with HTTP/DB/cache/queue detection patterns
  - InferSourceComponent from go.mod module path
affects: [09-02, 10-python-js-parsers, 12-signal-merging]

# Tech tracking
tech-stack:
  added: [golang.org/x/mod]
  patterns: [go/ast visitor, pattern-table lookup, versioned import resolution]

key-files:
  created:
    - internal/code/signal.go
    - internal/code/analyzer.go
    - internal/code/analyzer_test.go
    - internal/code/goparser/parser.go
    - internal/code/goparser/patterns.go
    - internal/code/goparser/parser_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Version-aware import path resolution for Go modules (v9 -> parent package name)"
  - "Hyphenated package name heuristic (go-redis -> redis) for local name inference"
  - "Pattern table keyed by importPath.Function for O(1) lookup during AST walk"
  - "Comment hints at 0.4 confidence to distinguish from code-level detection"

patterns-established:
  - "LanguageParser interface: Name/Extensions/ParseFile contract for all language parsers"
  - "DetectionPattern table: declarative patterns vs imperative AST matching"
  - "CodeSignal as universal signal type across all languages"

requirements-completed: [CODE-01]

# Metrics
duration: 4min
completed: 2026-03-30
---

# Phase 9 Plan 1: Code Analysis Foundation Summary

**Go AST-based parser detecting HTTP/DB/cache/queue infrastructure calls with pattern-table lookup, comment hints, and language-agnostic LanguageParser interface**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-30T07:20:17Z
- **Completed:** 2026-03-30T07:24:40Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- CodeSignal struct, LanguageParser interface, and CodeAnalyzer orchestrator defined with full test coverage
- GoParser detects HTTP calls (net/http), DB connections (database/sql, sqlx, gorm, mongo), cache clients (redis, memcache), and queue producers/consumers (sarama, amqp, nats, SQS)
- Comment hint detection (// Calls X, // Depends on Y) at 0.4 confidence
- URL hostname extraction from string literal arguments for precise target identification
- 23 tests total: 5 for CodeAnalyzer, 18 for GoParser covering all detection categories and edge cases

## Task Commits

Each task was committed atomically:

1. **Task 1: Define CodeSignal, LanguageParser, CodeAnalyzer** - `78c2f04` (feat)
2. **Task 2: Implement Go parser with detection patterns** - `16c32e4` (feat)

## Files Created/Modified
- `internal/code/signal.go` - CodeSignal struct with 8 JSON-tagged fields
- `internal/code/analyzer.go` - LanguageParser interface, CodeAnalyzer orchestrator, InferSourceComponent
- `internal/code/analyzer_test.go` - Tests for CodeAnalyzer dispatch, InferSourceComponent
- `internal/code/goparser/patterns.go` - DetectionPattern table with 21 patterns across 4 categories
- `internal/code/goparser/parser.go` - GoParser with AST walking, import resolution, URL extraction
- `internal/code/goparser/parser_test.go` - 18 tests with inline Go source fixtures
- `go.mod` - Added golang.org/x/mod dependency
- `go.sum` - Updated checksums

## Decisions Made
- Version-aware import path resolution: paths ending in vN (e.g., go-redis/v9) resolve to the parent segment, then strip hyphens for the Go local name
- Pattern table uses "importPath.Function" composite key for O(1) lookup during AST visitor traversal
- Comment hints produce signals at 0.4 confidence to clearly distinguish from code-level 0.85-0.9 detections
- Import-only files (import without function call) correctly produce zero signals

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed versioned import path resolution for Redis/cache packages**
- **Found during:** Task 2 (Go parser implementation)
- **Issue:** `filepath.Base("github.com/redis/go-redis/v9")` returns "v9", not the expected "redis" local name, causing cache detection to miss all redis.NewClient calls
- **Fix:** Added `defaultPackageName()` function that handles version segments (vN -> parent), .go suffixes (nats.go -> nats), and hyphenated names (go-redis -> redis)
- **Files modified:** internal/code/goparser/parser.go
- **Verification:** TestCacheDetection and TestCacheRedisGenericTarget pass
- **Committed in:** 16c32e4 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential fix for cache detection correctness. No scope creep.

## Issues Encountered
None beyond the auto-fixed import resolution bug.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- LanguageParser interface ready for Python and JavaScript parsers (Phase 10)
- CodeAnalyzer.RegisterParser accepts any LanguageParser implementation
- Pattern table approach established as the convention for future parsers
- GoParser proves the abstraction works end-to-end before committing more parsers

---
*Phase: 09-code-analysis-foundation*
*Completed: 2026-03-30*
