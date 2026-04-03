# Project State: graphmd v2.0

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-29)

**Core value:** AI agents can answer "if this fails, what breaks?" by querying a pre-computed dependency graph
**Current focus:** Phase 11 - Connection Strings + Comment Analysis

## Current Position

Phase: 11 of 13 (Connection Strings + Comment Analysis)
Plan: 3 of 3 complete
Status: Phase 11 complete
Last activity: 2026-04-01 — Completed 11-03-PLAN.md (parser integration)

Progress: [█████████████████████] 96% (26/~27 plans)

## Performance Metrics

**Velocity:**
- Total plans completed: 26 (v1.0: 15, v1.1: 4, v2.0: 7)
- Total execution time: see milestone records

**By Phase (v1.1 — most recent):**

| Phase | Plans | Status |
|-------|-------|--------|
| 6. Dead Code Removal | 1 | Complete |
| 7. Silent Loss Reporting | 2 | Complete |
| 8. Provenance Access | 1 | Complete |
| 9. Code Analysis Foundation | 2/2 | Complete |
| 10. Python/JS/TS Parsers | 3/3 | Complete |
| 11. Connection Strings + Comment Analysis | 3/3 | Complete |

## Accumulated Context

### Decisions

- v1.0 shipped 2026-03-28: 18 requirements, 5 phases, 15 plans
- v1.1 shipped 2026-03-29: 4 requirements, 3 phases, 4 plans
- Code flows (function call chains) deferred to v2.1
- Go parser uses stdlib go/ast (no CGo) — proves LanguageParser interface before Python/JS
- Python/JS use regex-first approach; tree-sitter deferred to v2.1 if needed
- Signal merging isolated in Phase 12 (highest risk)
- MCP server last (Phase 13) — queries enriched hybrid graph
- Schema v6: source_type column + code_signals table
- CGo-free build maintained throughout v2.0
- Version-aware import path resolution for Go modules (vN -> parent package name)
- Pattern table keyed by importPath.Function for O(1) lookup during AST walk
- Comment hints at 0.4 confidence to distinguish from code-level detection
- Variadic parser args on RunCodeAnalysis to avoid import cycle (code <-> goparser)
- Code signals printed to stderr as diagnostic output; graph integration deferred to Phase 12
- Python parser uses regex two-phase scan: import resolution then call pattern matching
- host:port extraction added for bootstrap_servers-style targets (kafka, etc.)
- Bare call matching for from-imports handled separately from obj.fn matching
- JS/TS parser uses two-pass line scanning: import map first, pattern detection second
- Global bare call support for fetch() (no import required)
- Constructor patterns separated from method call patterns for new X() syntax
- Manifest check order for InferSourceComponent: go.mod > pyproject.toml > setup.py > package.json
- Regex for pyproject.toml/setup.py name extraction (no TOML parser dependency)
- All three parsers registered at all CLI call sites (index, export, crawl)
- Comment analyzer uses non-greedy regex to avoid consuming trailing punctuation in docstrings
- URL filtering in comments uses static blocklist of documentation/example domains
- Block comment state tracking shared across Go/JS /* */ and Python triple-quote docstrings
- Consolidated three extractURLHost implementations into shared connstring.Parse with richer type mapping
- Host:port regex requires leading alpha char to avoid false matches on version strings
- Go parser extractTarget returns (target, targetType) tuple for connstring-enriched type info
- boostKnownComponents runs as two-pass in-place mutation in RunCodeAnalysis
- inferEnvVarTargetType duplicated per parser rather than shared (trivial function, avoids cross-package dep)

### Pending Todos

None yet.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-04-01
Stopped at: Completed 11-03-PLAN.md (parser integration) — Phase 11 complete
Resume file: None
